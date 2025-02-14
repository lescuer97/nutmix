package admin

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lescuer97/nutmix/internal/mint"
	"github.com/lescuer97/nutmix/internal/utils"
	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip19"
)

const AdminAuthKey = "admin-cookie"

func AuthMiddleware(logger *slog.Logger, secret []byte) gin.HandlerFunc {
	return func(c *gin.Context) {
		if cookie, err := c.Cookie(AdminAuthKey); err == nil {

			token, err := jwt.Parse(cookie, func(token *jwt.Token) (interface{}, error) {

				// Don't forget to validate the alg is what you expect:
				if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
					logger.Warn(fmt.Sprintf("Unexpected signing method: %v", token.Header["alg"]))
					return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
				}
				return secret, nil
			})

			if err != nil {
				logger.Debug(
					"jwt.Parse(cookie)",
					slog.String(utils.LogExtraInfo, err.Error()),
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

func Login(mint *mint.Mint, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// parse data for login
		logger.Debug("Attempting log in")
		var nostrEvent nostr.Event
		err := c.BindJSON(&nostrEvent)

		if err != nil {
			logger.Debug(
				"Incorrect body",
				slog.String(utils.LogExtraInfo, err.Error()),
			)
			c.JSON(400, "Malformed body request")
			return
		}
		ctx := context.Background()

		tx, err := mint.MintDB.GetTx(ctx)
		if err != nil {
			c.Error(fmt.Errorf("mint.MintDB.GetTx(). %w", err))
			return
		}

		defer func() {
			if p := recover(); p != nil {
				c.Error(fmt.Errorf("\n Rolling back  because of failure %+v\n", err))
				mint.MintDB.Rollback(ctx, tx)

			} else if err != nil {
				c.Error(fmt.Errorf("\n Rolling back  because of failure %+v\n", err))
				mint.MintDB.Rollback(ctx, tx)
			} else {
				err = mint.MintDB.Commit(context.Background(), tx)
				if err != nil {
					c.Error(fmt.Errorf("\n Failed to commit transaction: %+v \n", err))
				}
			}
		}()

		nostrLogin, err := mint.MintDB.GetNostrAuth(tx, nostrEvent.Content)

		if err != nil {
			logger.Error(
				"database.GetNostrLogin(pool, nostrEvent.Content )",
				slog.String(utils.LogExtraInfo, err.Error()),
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
			logger.Info("nostrEvent.CheckSignature()", slog.String(utils.LogExtraInfo, err.Error()))
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
			logger.Info("hex.DecodeString(nostrEvent.Sig)", slog.String(utils.LogExtraInfo, err.Error()))
			c.JSON(500, "Something happend!")
			return
		}

		sig, err := schnorr.ParseSignature(sigBytes)
		if err != nil {
			logger.Info("schnorr.ParseSignature(sigBytes)", slog.String(utils.LogExtraInfo, err.Error()))
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
			logger.Info("nip19.Decode(adminPubkey)", slog.String(utils.LogExtraInfo, err.Error()))
			c.JSON(500, "Something happend!")
			return
		}

		decodedKey, err := hex.DecodeString(value.(string))
		if err != nil {
			logger.Info("hex.DecodeString(value.(string))", slog.String(utils.LogExtraInfo, err.Error()))
			c.JSON(500, "Something happend!")
			return
		}

		pubkey, err := schnorr.ParsePubKey(decodedKey)

		if err != nil {
			logger.Info("schnorr.ParsePubKey(decodedKey)", slog.String(utils.LogExtraInfo, err.Error()))
			c.JSON(500, "Something happend!")
			return
		}

		eventHash := sha256.Sum256(nostrEvent.Serialize())

		verified := sig.Verify(eventHash[:], pubkey)

		if !verified {
			if c.ContentType() == gin.MIMEJSON {
				c.JSON(400, "Private key used is not correct")
				return
			} else {
				logger.Warn("Private key used is not correct")
				c.Header("HX-RETARGET", "error-message")
				c.HTML(400, "incorrect-key-error", nil)
				return
			}
		}

		nostrLogin.Activated = verified

		err = mint.MintDB.UpdateNostrAuthActivation(tx, nostrLogin.Nonce, nostrLogin.Activated)

		if err != nil {
			logger.Error("database.UpdateNostrLoginActivation(pool, nostrLogin)", slog.String(utils.LogExtraInfo, err.Error()))
			c.JSON(500, "Opps!, something wrong happened")
			return
		}

		token, err := makeJWTToken(mint.ActiveKeysets[cashu.Sat.String()][1].PrivKey.Serialize())

		if err != nil {
			logger.Warn("Could not makeJWTToken", slog.String(utils.LogExtraInfo, err.Error()))
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
