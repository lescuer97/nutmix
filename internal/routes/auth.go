package routes

import (
	"context"
	"fmt"
	"log"
	"log/slog"

	// "github.com/coreos/go-oidc/v3/oidc"
	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-gonic/gin"
	"github.com/lescuer97/nutmix/api/cashu"
	m "github.com/lescuer97/nutmix/internal/mint"
	"github.com/lescuer97/nutmix/internal/utils"
)

func v1AuthRoutes(r *gin.Engine, mint *m.Mint, logger *slog.Logger) {
	v1 := r.Group("/v1")
	auth := v1.Group("/auth")

	auth.GET("/blind/keys", func(c *gin.Context) {
		keys, err := mint.Signer.GetAuthActiveKeys()
		if err != nil {
			logger.Error(fmt.Sprintf("mint.Signer.GetAuthActiveKeys() %+v ", err))
			c.JSON(400, cashu.ErrorCodeToResponse(cashu.KEYSET_NOT_KNOW, nil))
			return
		}

		c.JSON(200, keys)
	})

	auth.GET("/blind/keys/:id", func(c *gin.Context) {
		id := c.Param("id")

		keysets, err := mint.Signer.GetAuthKeysById(id)

		if err != nil {
			logger.Error(fmt.Sprintf("mint.Signer.GetAuthKeysById(id) %+v", err))
			c.JSON(400, cashu.ErrorCodeToResponse(cashu.KEYSET_NOT_KNOW, nil))
			return
		}

		c.JSON(200, keysets)
	})
	auth.GET("/blind/keys/keysets", func(c *gin.Context) {
		keys, err := mint.Signer.GetAuthKeys()
		if err != nil {
			logger.Error(fmt.Errorf("mint.Signer.GetAuthKeys() %w", err).Error())
			c.JSON(500, "Server side error")
			return
		}

		c.JSON(200, keys)
	})

	auth.GET("/blind/mint", func(c *gin.Context) {

        ctx := context.Background()
        verifier := mint.OICDClient.Verifier(&oidc.Config{ClientID: mint.Config.MINT_AUTH_OICD_CLIENT_ID})
        // check if it's valid token
        token := c.GetHeader("Clear-auth")

        idToken, err := verifier.Verify(ctx,token )
        if err != nil {
            logger.Error(fmt.Errorf("verifier.Verify(ctx,token ). %w", err).Error())
			errorCode, details := utils.ParseErrorToCashuErrorCode(err)
			c.JSON(400, cashu.ErrorCodeToResponse(errorCode, details))
			return
        }
        log.Printf("\n idToken hash: %v\n", idToken.AccessTokenHash)
        // oidc.Config
        // mint.OICDClient.Endpoint
        // oidc.Provider
        c.JSON(200, nil)
	})
}
