package routes

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/lescuer97/nutmix/api/cashu"
	m "github.com/lescuer97/nutmix/internal/mint"
	"github.com/lescuer97/nutmix/internal/utils"
)

func v1bolt11Routes(r *gin.Engine, mint *m.Mint) {
	v1 := r.Group("/v1")

	v1.POST("/mint/quote/bolt11", func(c *gin.Context) {
		var mintRequest cashu.PostMintQuoteBolt11Request
		err := c.BindJSON(&mintRequest)

		if err != nil {
			slog.Info("Incorrect body", slog.Any("error", err))
			c.JSON(400, "Malformed body request")
			return
		}

		response, err := mint.CreateMintQuote(c.Request.Context(), mintRequest, m.Bolt11)
		if err != nil {
			slog.Info("mint.Swap(c.Request.Context(), swapRequest)", slog.Any("error", err))
			errorCode, details := utils.ParseErrorToCashuErrorCode(err)
			c.JSON(400, cashu.ErrorCodeToResponse(errorCode, details))
			return
		}

		c.JSON(200, response)
	})

	v1.GET("/mint/quote/bolt11/:quote", func(c *gin.Context) {
		quoteId := c.Param("quote")
		response, err := mint.MintQuoteStatus(c.Request.Context(), quoteId, m.Bolt11)
		if err != nil {
			slog.Info("mint.MintQuoteStatus(c.Request.Context(), quoteId, m.Bolt11)", slog.Any("error", err))
			errorCode, details := utils.ParseErrorToCashuErrorCode(err)
			c.JSON(400, cashu.ErrorCodeToResponse(errorCode, details))
			return
		}
		c.JSON(200, response)
	})

	v1.POST("/mint/bolt11", func(c *gin.Context) {
		var mintRequest cashu.PostMintBolt11Request

		err := c.BindJSON(&mintRequest)
		if err != nil {
			slog.Info("Incorrect body", slog.Any("error", err))
			errorCode, details := utils.ParseErrorToCashuErrorCode(err)
			c.JSON(400, cashu.ErrorCodeToResponse(errorCode, details))
			return
		}
		response, err := mint.Mint(c.Request.Context(), mintRequest, m.Bolt11)
		if err != nil {
			slog.Info("Incorrect body", slog.Any("error", err))
			errorCode, details := utils.ParseErrorToCashuErrorCode(err)
			c.JSON(400, cashu.ErrorCodeToResponse(errorCode, details))
			return
		}
		c.JSON(200, response)
	})

	v1.POST("/melt/quote/bolt11", func(c *gin.Context) {
		var meltRequest cashu.PostMeltQuoteBolt11Request
		err := c.BindJSON(&meltRequest)

		if err != nil {
			slog.Info("Incorrect body", slog.Any("error", err))
			c.JSON(400, "Malformed body request")
			return
		}

		dbRequest, err := mint.MeltQuote(c.Request.Context(), meltRequest, m.Bolt11)
		if err != nil {
			slog.Warn("mint.MeltQuote", slog.Any("error", err))
			errorCode, details := utils.ParseErrorToCashuErrorCode(err)
			c.JSON(400, cashu.ErrorCodeToResponse(errorCode, details))
			return
		}
		c.JSON(200, dbRequest.GetPostMeltQuoteResponse())
	})

	v1.GET("/melt/quote/bolt11/:quote", func(c *gin.Context) {
		quoteId := c.Param("quote")

		quote, err := mint.CheckMeltQuoteState(c.Request.Context(), quoteId)
		if err != nil {
			slog.Warn("mint.CheckMeltQuoteState(quoteId)", slog.Any("error", err))
			errorCode, details := utils.ParseErrorToCashuErrorCode(err)
			c.JSON(400, cashu.ErrorCodeToResponse(errorCode, details))
			return
		}

		c.JSON(200, quote.GetPostMeltQuoteResponse())
	})

	v1.POST("/melt/bolt11", func(c *gin.Context) {
		var meltRequest cashu.PostMeltBolt11Request
		err := c.BindJSON(&meltRequest)
		if err != nil {
			slog.Info("Incorrect body", slog.Any("error", err))
			errorCode, details := utils.ParseErrorToCashuErrorCode(err)
			c.JSON(400, cashu.ErrorCodeToResponse(errorCode, details))
			return
		}

		quote, err := mint.Melt(c.Request.Context(), meltRequest, m.Bolt11)
		if err != nil {
			slog.Warn("mint.Melt(ctx, meltRequest)", slog.Any("error", err))
			errorCode, details := utils.ParseErrorToCashuErrorCode(err)
			c.JSON(400, cashu.ErrorCodeToResponse(errorCode, details))
			return
		}

		c.JSON(200, quote)
	})
}
