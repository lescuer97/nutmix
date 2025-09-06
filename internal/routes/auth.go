package routes

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/lescuer97/nutmix/api/cashu"
	m "github.com/lescuer97/nutmix/internal/mint"
	"github.com/lescuer97/nutmix/internal/utils"
)

func AuthActivatedMiddleware(mint *m.Mint) gin.HandlerFunc {
	return func(c *gin.Context) {

		if !mint.Config.MINT_REQUIRE_AUTH {
			slog.Warn(fmt.Errorf("Tried using route that does not exists because auth not being active").Error())
			c.JSON(404, "route does not exists")
			c.Abort()
			return
		}
		c.Next()
	}
}

func v1AuthRoutes(r *gin.Engine, mint *m.Mint) {
	v1 := r.Group("/v1")
	auth := v1.Group("/auth")
	auth.Use(AuthActivatedMiddleware(mint))

	auth.GET("/blind/keys", func(c *gin.Context) {
		keys, err := mint.Signer.GetAuthActiveKeys()
		if err != nil {
			slog.Error("mint.Signer.GetAuthActiveKeys()", slog.Any("error", err))
			c.JSON(400, cashu.ErrorCodeToResponse(cashu.KEYSET_NOT_KNOW, nil))
			return
		}

		c.JSON(200, keys)
	})

	auth.GET("/blind/keys/:id", func(c *gin.Context) {
		id := c.Param("id")

		keysets, err := mint.Signer.GetAuthKeysById(id)

		if err != nil {
			slog.Error("mint.Signer.GetAuthKeysById(id)", slog.Any("error", err))
			c.JSON(400, cashu.ErrorCodeToResponse(cashu.KEYSET_NOT_KNOW, nil))
			return
		}

		c.JSON(200, keysets)
	})

	auth.GET("/blind/keysets", func(c *gin.Context) {
		keys, err := mint.Signer.GetAuthKeys()
		if err != nil {
			slog.Error("mint.Signer.GetAuthKeys()", slog.Any("error", err))
			c.JSON(500, "Server side error")
			return
		}

		c.JSON(200, keys)
	})

	auth.POST("/blind/mint", func(c *gin.Context) {
		var mintRequest cashu.PostMintBolt11Request
		err := c.BindJSON(&mintRequest)
		if err != nil {
			slog.Info("Incorrect body", slog.Any("error", err))
			c.JSON(400, "Malformed body request")
			return
		}

		ctx := context.Background()
		tx, err := mint.MintDB.GetTx(ctx)
		if err != nil {
			c.Error(fmt.Errorf("m.MintDB.GetTx(ctx). %w", err))
			return
		}
		defer mint.MintDB.Rollback(ctx, tx)

		keysets, err := mint.Signer.GetAuthKeys()
		if err != nil {
			slog.Error("mint.Signer.GetKeys()", slog.Any("error", err))
			errorCode, details := utils.ParseErrorToCashuErrorCode(err)
			c.JSON(400, cashu.ErrorCodeToResponse(errorCode, details))
			return
		}
		unit, err := mint.VerifyOutputs(tx, mintRequest.Outputs, keysets.Keysets)
		if err != nil {
			slog.Error("mint.VerifyOutputs(mintRequest.Outputs)", slog.Any("error", err))
			errorCode, details := utils.ParseErrorToCashuErrorCode(err)
			c.JSON(400, cashu.ErrorCodeToResponse(errorCode, details))
			return
		}

		if unit != cashu.AUTH {
			details := `You can only use "auth" tokens in this endpoint`
			c.JSON(400, cashu.ErrorCodeToResponse(cashu.UNIT_NOT_SUPPORTED, &details))
			return
		}

		amountBlindMessages := uint64(0)

		for _, blindMessage := range mintRequest.Outputs {
			amountBlindMessages += blindMessage.Amount
			// check all blind messages have the same unit
		}

		if amountBlindMessages > uint64(mint.Config.MINT_AUTH_MAX_BLIND_TOKENS) {
			slog.Warn("Trying to mint auth tokens over the limit")
			c.JSON(400, cashu.ErrorCodeToResponse(cashu.MAXIMUM_BAT_MINT_LIMIT_EXCEEDED, nil))
			return
		}

		blindedSignatures, recoverySigsDb, err := mint.Signer.SignBlindMessages(mintRequest.Outputs)
		if err != nil {
			slog.Error("mint.Signer.SignBlindMessages(mintRequest.Outputs)", slog.Any("error", err))
			errorCode, details := utils.ParseErrorToCashuErrorCode(err)
			c.JSON(400, cashu.ErrorCodeToResponse(errorCode, details))
			return
		}

		err = mint.MintDB.SaveRestoreSigs(tx, recoverySigsDb)
		if err != nil {
			slog.Error("SetRecoverySigs on minting", slog.Any("error", err))
			slog.Error("recoverySigsDb", slog.Any("recovery_sigs", recoverySigsDb))
			return
		}

		err = mint.MintDB.Commit(ctx, tx)
		if err != nil {
			c.Error(fmt.Errorf("mint.MintDB.Commit(ctx tx). %w", err))
			return
		}

		c.JSON(200, cashu.PostMintBolt11Response{
			Signatures: blindedSignatures,
		})
	})
}
