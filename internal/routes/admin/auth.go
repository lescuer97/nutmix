package admin

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/lescuer97/nutmix/internal/mint"
	"github.com/lescuer97/nutmix/internal/utils"
	"github.com/nbd-wtf/go-nostr"
)

const AdminAuthKey = "admin-cookie"

func AuthMiddleware(secret []byte, blacklist *TokenBlacklist) gin.HandlerFunc {
	return func(c *gin.Context) {
		if cookie, err := c.Cookie(AdminAuthKey); err == nil {
			// Check if token is blacklisted first
			if blacklist.IsTokenBlacklisted(cookie) {
				c.SetCookie(AdminAuthKey, "", -1, "/", "", false, true)
				c.Header("HX-Redirect", "/admin/login")
				c.Abort()
				return
			}

			token, err := jwt.Parse(cookie, func(token *jwt.Token) (interface{}, error) {
				// Don't forget to validate the alg is what you expect:
				if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
					slog.Warn("Unexpected signing method", slog.Any("alg", token.Header["alg"]))
					return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
				}
				return secret, nil
			})

			if err != nil {
				slog.Debug(
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

var ErrIncorrectNpub = errors.New("Incorrect npub used in signature")

func LoginPost(mint *mint.Mint, loginKey *secp256k1.PrivateKey, adminNostrPubkey *btcec.PublicKey) gin.HandlerFunc {
	return func(c *gin.Context) {
		if adminNostrPubkey == nil {
			slog.Error("adminNostrPubkey is nil. this should have never happened")
			panic("adminNostrPubkey is nil")
		}

		// parse data for login
		slog.Debug("Attempting log in")
		var nostrEvent nostr.Event
		err := c.BindJSON(&nostrEvent)
		if err != nil {
			slog.Debug(
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
			slog.Error(
				"mint.MintDB.GetNostrAuth(tx, nostrEvent.Content)",
				slog.Any("error", err),
			)
			c.JSON(400, "Content is not available")
			return
		}

		if nostrLogin.Activated {
			c.JSON(403, "This login value was already used, please reload the page")
			return
		}

		// check valid signature
		validSig, err := nostrEvent.CheckSignature()
		if err != nil {
			slog.Info("nostrEvent.CheckSignature()", slog.Any("error", err))
			c.JSON(400, "Invalid signature")
			return
		}

		if !validSig {
			slog.Warn("Invalid Signature")
			c.JSON(403, "Invalid signature")
			return
		}

		// check signature happened with the correct private key.
		sigBytes, err := hex.DecodeString(nostrEvent.Sig)
		if err != nil {
			slog.Info("hex.DecodeString(nostrEvent.Sig)", slog.Any("error", err))
			c.JSON(500, "Something happend!")
			return
		}

		sig, err := schnorr.ParseSignature(sigBytes)
		if err != nil {
			slog.Info("schnorr.ParseSignature(sigBytes)", slog.Any("error", err))
			c.JSON(500, "Something happend!")
			return
		}

		eventHash := sha256.Sum256(nostrEvent.Serialize())
		verified := sig.Verify(eventHash[:], adminNostrPubkey)
		if !verified {
			if c.ContentType() == gin.MIMEJSON {
				c.JSON(400, "Private key used is not correct")
				return
			} else {
				c.Error(ErrIncorrectNpub)
				return
			}
		}

		nostrLogin.Activated = verified
		err = mint.MintDB.UpdateNostrAuthActivation(tx, nostrLogin.Nonce, nostrLogin.Activated)
		if err != nil {
			slog.Error("database.UpdateNostrLoginActivation(pool, nostrLogin)", slog.Any("error", err))
			c.JSON(500, "Opps!, something wrong happened")
			return
		}

		token, err := makeJWTToken(loginKey.Serialize())

		if err != nil {
			slog.Warn("Could not makeJWTToken", slog.Any("error", err))
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
