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
	"github.com/jackc/pgx/v5"
	"github.com/lescuer97/nutmix/internal/mint"
	"github.com/lescuer97/nutmix/internal/utils"
	"github.com/nbd-wtf/go-nostr"
)

const AdminAuthKey = "admin-cookie"

func handleUnauthorized(c *gin.Context) {
	slog.Debug("Handling unauthorized request", slog.String("path", c.Request.URL.Path), slog.String("method", c.Request.Method))
	c.SetCookie(AdminAuthKey, "", -1, "/", "", false, true)
	if c.GetHeader("HX-Request") == "true" {
		c.Header("HX-Redirect", "/admin/login")
		c.Status(http.StatusOK) // HTMX expects 200 for redirects
	} else {
		c.Redirect(http.StatusFound, "/admin/login")
	}
	c.Abort()
}

func AuthMiddleware(secret []byte, blacklist *TokenBlacklist) gin.HandlerFunc {
	return func(c *gin.Context) {
		cookie, err := c.Cookie(AdminAuthKey)
		if err != nil {
			slog.Debug("No admin cookie found", slog.String("error", err.Error()))
			if c.Request.URL.Path == "/admin/login" {
				return
			}
			handleUnauthorized(c)
			return
		}

		// Check if token is blacklisted first
		if blacklist.IsTokenBlacklisted(cookie) {
			slog.Debug("Token is blacklisted")
			handleUnauthorized(c)
			return
		}

		token, err := jwt.Parse(cookie, func(token *jwt.Token) (interface{}, error) {
			// Don't forget to validate the alg is what you expect:
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				slog.Warn("Unexpected signing method", slog.Any("alg", token.Header["alg"]))
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return secret, nil
		})

		if err != nil {
			slog.Debug(
				"jwt.Parse(cookie)",
				slog.String(utils.LogExtraInfo, err.Error()),
			)
			handleUnauthorized(c)
			return
		}

		if !token.Valid {
			slog.Debug("token is not valid", slog.Any("token", token))
			handleUnauthorized(c)
			return
		}

		// Success path
		if c.Request.URL.Path == "/admin/login" {
			slog.Debug("Redirecting to /admin from login page")
			c.Header("HX-Redirect", "/admin")
			c.Redirect(http.StatusTemporaryRedirect, "/admin")
			c.Abort()
			return
		}

		c.Next()
	}
}

var ErrIncorrectNpub = errors.New("incorrect npub used in signature")

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
			_ = c.Error(fmt.Errorf("mint.MintDB.GetTx(). %w", err))
			return
		}

		defer func() {
			if p := recover(); p != nil {
				_ = c.Error(fmt.Errorf("rolling back because of failure %+v", err))
				if rollbackErr := mint.MintDB.Rollback(ctx, tx); rollbackErr != nil {
					if !errors.Is(rollbackErr, pgx.ErrTxClosed) {
						slog.Error("Failed to rollback transaction", slog.Any("error", rollbackErr))
					}
				}

			} else if err != nil {
				_ = c.Error(fmt.Errorf("rolling back because of failure %+v", err))
				if rollbackErr := mint.MintDB.Rollback(ctx, tx); rollbackErr != nil {
					if !errors.Is(rollbackErr, pgx.ErrTxClosed) {
						slog.Error("Failed to rollback transaction", slog.Any("error", rollbackErr))
					}
				}
			} else {
				err = mint.MintDB.Commit(context.Background(), tx)
				if err != nil {
					_ = c.Error(fmt.Errorf("failed to commit transaction: %+v", err))
				}
			}
		}()

		nostrLogin, err := mint.MintDB.GetNostrAuth(tx, nostrEvent.Content)
		if err != nil {
			_ = c.Error(errors.Join(ErrCouldNotParseLogin, err))
			return
		}

		if nostrLogin.Activated {
			c.JSON(403, "This login value was already used, please reload the page")
			return
		}

		// check valid signature
		validSig, err := nostrEvent.CheckSignature()
		if err != nil {
			_ = c.Error(errors.Join(ErrInvalidNostrSignature, err))
			return
		}

		if !validSig {
			_ = c.Error(errors.Join(ErrInvalidNostrSignature, err))
			return
		}

		// check signature happened with the correct private key.
		sigBytes, err := hex.DecodeString(nostrEvent.Sig)
		if err != nil {
			_ = c.Error(errors.Join(ErrInvalidNostrSignature, err))
			return
		}

		sig, err := schnorr.ParseSignature(sigBytes)
		if err != nil {
			_ = c.Error(errors.Join(ErrInvalidNostrSignature, err))
			return
		}

		eventHash := sha256.Sum256(nostrEvent.Serialize())
		verified := sig.Verify(eventHash[:], adminNostrPubkey)
		if !verified {
			_ = c.Error(ErrIncorrectNpub)
			return
		}

		nostrLogin.Activated = verified
		err = mint.MintDB.UpdateNostrAuthActivation(tx, nostrLogin.Nonce, nostrLogin.Activated)
		if err != nil {
			_ = c.Error(errors.Join(ErrCouldNotParseLogin, fmt.Errorf("mint.MintDB.UpdateNostrAuthActivation(tx, nostrLogin.Nonce, nostrLogin.Activated). %w", err)))
			return
		}

		token, err := makeJWTToken(loginKey.Serialize())
		if err != nil {
			_ = c.Error(fmt.Errorf("makeJWTToken(loginKey.Serialize()). %w", err))
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
