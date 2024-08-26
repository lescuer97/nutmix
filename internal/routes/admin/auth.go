package admin

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lescuer97/nutmix/internal/database"
	"github.com/lescuer97/nutmix/internal/mint"
	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip19"
)

const AdminAuthKey = "admin-cookie"

func AuthMiddleware(ctx context.Context) gin.HandlerFunc {
	return func(c *gin.Context) {
		if cookie, err := c.Cookie(AdminAuthKey); err == nil {
			key := os.Getenv(JWTSECRET)
			token, err := jwt.Parse(cookie, func(token *jwt.Token) (interface{}, error) {
				return key, nil
			})

			if err != nil {
				log.Printf("jwt.Parse(cookie) %+v", err)
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
			c.Next()

		}
	}
}

func Login(ctx context.Context, pool *pgxpool.Pool, mint *mint.Mint) gin.HandlerFunc {
	return func(c *gin.Context) {

		var nostrEvent nostr.Event
		err := c.BindJSON(&nostrEvent)

		if err != nil {
			log.Printf("Incorrect body: %+v", err)
			c.JSON(400, "Malformed body request")
			return
		}

		nostrLogin, err := database.GetNostrLogin(pool, nostrEvent.Content)

		if err != nil {
			log.Printf("database.GetNostrLogin(pool, nostrEvent.Content ): %+v", err)
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
			log.Printf("nostrEvent.CheckSignature(): %+v", err)
			c.JSON(400, "Invalid signature")
			return
		}

		if !validSig {
			log.Printf("Invalid Signature: %+v", err)
			c.JSON(403, "Invalid signature")
			return
		}

		// check signature happened with the correct private key.
		sigBytes, err := hex.DecodeString(nostrEvent.Sig)
		if err != nil {
			log.Printf("hex.DecodeString(nostrEvent.Sig): %+v", err)
			c.JSON(500, "Something happend!")
			return
		}

		sig, err := schnorr.ParseSignature(sigBytes)
		if err != nil {
			log.Printf("schnorr.ParseSignature(sigBytes): %+v", err)
			c.JSON(500, "Something happend!")
			return
		}

		adminPubkey := os.Getenv("ADMIN_NOSTR_NPUB")

		if adminPubkey == "" {
			log.Printf("ERROR: NO ADMIN PUBKEY PRESENT %+v", err)
			c.JSON(500, "Something happend!")
			return

		}

		_, value, err := nip19.Decode(adminPubkey)
		if err != nil {
			log.Printf("bech32.Decode(adminPubkey): %+v", err)
			c.JSON(500, "Something happend!")
			return
		}

		decodedKey, err := hex.DecodeString(value.(string))
		if err != nil {
			log.Printf("hex.DecodeString(value.(string)): %+v", err)
			c.JSON(500, "Something happend!")
			return
		}

		pubkey, err := schnorr.ParsePubKey(decodedKey)

		if err != nil {
			log.Printf("btcec.ParsePubKey: %+v", err)
			c.JSON(500, "Something happend!")
			return
		}

		eventHash := sha256.Sum256(nostrEvent.Serialize())

		verified := sig.Verify(eventHash[:], pubkey)

		if !verified {
			log.Printf("Private key used is not correct")
			c.Header("HX-RETARGET", "error-message")
			c.HTML(400, "incorrect-key-error", nil)
			return
		}

		nostrLogin.Activated = verified

		err = database.UpdateNostrLoginActivation(pool, nostrLogin)

		if err != nil {
			log.Printf("database.UpdateNostrLoginActivation(pool, nostrLogin): %+v", err)
			c.JSON(500, "Opps!, something wrong happened")
			return
		}

		jwtsecret := []byte(os.Getenv(JWTSECRET))

		token, err := makeJWTToken(jwtsecret)

		if err != nil {
			log.Printf("Could not makeJWTToken %+v", err)
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
