package admin

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lescuer97/nutmix/internal/database"
	"github.com/lescuer97/nutmix/internal/mint"
	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip19"
	"log/slog"
	"net/http"
	"os"
)

const AdminAuthKey = "admin-cookie"

func AuthMiddleware(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		if cookie, err := c.Cookie(AdminAuthKey); err == nil {
			key := []byte(os.Getenv(JWT_SECRET))

			token, err := jwt.Parse(cookie, func(token *jwt.Token) (interface{}, error) {

				// Don't forget to validate the alg is what you expect:
				if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
					logger.Warn(fmt.Sprintf("Unexpected signing method: %v", token.Header["alg"]))
					return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
				}
				return key, nil
			})

			if err != nil {
				logger.Debug(
					"jwt.Parse(cookie)",
					slog.String("extra-info", err.Error()),
				)

				c.SetCookie(AdminAuthKey, "", -1, "/", "", false, true)
				c.Redirect(http.StatusTemporaryRedirect, "/admin/login")
				return
			}

			// if token is valid an navigating /admin/login redirect to /admin
			switch token.Valid {
			case true:
				if c.Request.URL.Path == "/admin/login" {
					c.Header("HX-Redirect", "/admin")
					c.Redirect(http.StatusTemporaryRedirect, "/admin")

					return
				}
				return
			case false:
				if c.Request.URL.Path == "/admin/login" {
					return
				}
				c.SetCookie(AdminAuthKey, "", -1, "/", "", false, true)
				c.Header("HX-Redirect", "/admin/login")
				return

			}
		}

		switch {
		case c.Request.URL.Path == "/admin/login":
			return
		default:
			c.Redirect(http.StatusTemporaryRedirect, "/admin/login")
			c.Header("HX-Location", "/admin/login")
			c.Abort()
			c.JSON(200, nil)

		}
	}
}

func Login(pool *pgxpool.Pool, mint *mint.Mint, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {

		logger.Debug("Attempting log in")
		var nostrEvent nostr.Event
		err := c.BindJSON(&nostrEvent)

		if err != nil {
			logger.Debug(
				"Incorrect body",
				slog.String("extra-info", err.Error()),
			)
			c.JSON(400, "Malformed body request")
			return
		}

		nostrLogin, err := database.GetNostrLogin(pool, nostrEvent.Content)

		if err != nil {
			logger.Error(
				"database.GetNostrLogin(pool, nostrEvent.Content )",
				slog.String("extra-info", err.Error()),
			)
			c.JSON(500, "Opps!, something wrong happened")
			return
		}

		if nostrLogin.Activated {
			c.JSON(403, "This login value was already used, please reload the page")
			return

		}

		// check valid signature
		validSig, err := nostrEvent.CheckSignature()
		if err != nil {
			logger.Info("nostrEvent.CheckSignature()", slog.String("extra-info", err.Error()))
			c.JSON(400, "Invalid signature")
			return
		}

		if !validSig {
			logger.Warn("Invalid Signature")
			c.JSON(403, "Invalid signature")
			return
		}

		// check signature happened with the correct private key.
		sigBytes, err := hex.DecodeString(nostrEvent.Sig)
		if err != nil {
			logger.Info("hex.DecodeString(nostrEvent.Sig)", slog.String("extra-info", err.Error()))
			c.JSON(500, "Something happend!")
			return
		}

		sig, err := schnorr.ParseSignature(sigBytes)
		if err != nil {
			logger.Info("schnorr.ParseSignature(sigBytes)", slog.String("extra-info", err.Error()))
			c.JSON(500, "Something happend!")
			return
		}

		adminPubkey := os.Getenv("ADMIN_NOSTR_NPUB")

		if adminPubkey == "" {
			logger.Error("ERROR: NO ADMIN PUBKEY PRESENT")
			c.JSON(500, "Something happend!")
			return

		}

		_, value, err := nip19.Decode(adminPubkey)
		if err != nil {
			logger.Info("nip19.Decode(adminPubkey)", slog.String("extra-info", err.Error()))
			c.JSON(500, "Something happend!")
			return
		}

		decodedKey, err := hex.DecodeString(value.(string))
		if err != nil {
			logger.Info("hex.DecodeString(value.(string))", slog.String("extra-info", err.Error()))
			c.JSON(500, "Something happend!")
			return
		}

		pubkey, err := schnorr.ParsePubKey(decodedKey)

		if err != nil {
			logger.Info("schnorr.ParsePubKey(decodedKey)", slog.String("extra-info", err.Error()))
			c.JSON(500, "Something happend!")
			return
		}

		eventHash := sha256.Sum256(nostrEvent.Serialize())

		verified := sig.Verify(eventHash[:], pubkey)

		if !verified {
			logger.Warn("Private key used is not correct")
			c.Header("HX-RETARGET", "error-message")
			c.HTML(400, "incorrect-key-error", nil)
			return
		}

		nostrLogin.Activated = verified

		err = database.UpdateNostrLoginActivation(pool, nostrLogin)

		if err != nil {
			logger.Error("database.UpdateNostrLoginActivation(pool, nostrLogin)", slog.String("extra-info", err.Error()))
			c.JSON(500, "Opps!, something wrong happened")
			return
		}

		jwtsecret := []byte(os.Getenv(JWT_SECRET))

		token, err := makeJWTToken(jwtsecret)

		if err != nil {
			logger.Warn("Could not makeJWTToken ", slog.String("extra-info", err.Error()))
			c.JSON(500, nil)
			return
		}

		c.SetCookie(AdminAuthKey, token, 3600, "/", "", false, true)
		c.Header("HX-Redirect", "/admin")
		c.JSON(200, nil)
	}
}

func makeJWTToken(secret []byte) (string, error) {

	token := jwt.New(jwt.SigningMethodHS256)
	string, err := token.SignedString(secret)
	if err != nil {
		return "", fmt.Errorf("token.SignedString(secret) %v", err)

	}
	return string, nil
}
