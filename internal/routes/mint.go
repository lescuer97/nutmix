package routes

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/lescuer97/nutmix/api/cashu"
	m "github.com/lescuer97/nutmix/internal/mint"
	"github.com/lescuer97/nutmix/internal/utils"
)

func registerV1MintRoutes(r *gin.Engine, mint *m.Mint) {
	v1 := r.Group("/v1")

	v1.GET("/keys", func(c *gin.Context) {
		keys, err := mint.Signer.GetActiveKeys()
		if err != nil {
			slog.Error("mint.Signer.GetActiveKeys()", slog.Any("error", err))
			c.JSON(400, cashu.ErrorCodeToResponse(cashu.KEYSET_NOT_KNOW, nil))
			return
		}

		c.JSON(200, keys)
	})

	v1.GET("/keys/:id", func(c *gin.Context) {
		id := c.Param("id")

		keysets, err := mint.Signer.GetKeysById(id)

		if err != nil {
			slog.Warn("mint.Signer.GetKeysById(id)", slog.Any("error", err))
			c.JSON(400, cashu.ErrorCodeToResponse(cashu.KEYSET_NOT_KNOW, nil))
			return
		}

		c.JSON(200, keysets)
	})
	v1.GET("/keysets", func(c *gin.Context) {
		keys, err := mint.Signer.GetKeysets()
		if err != nil {
			slog.Error("mint.Signer.GetKeys()", slog.Any("error", err))
			c.JSON(500, "Server side error")
			return
		}

		c.JSON(200, keys)
	})

	v1.GET("/info", func(c *gin.Context) {
		info := mint.Info()
		c.JSON(200, info)
	})

	v1.POST("/swap", func(c *gin.Context) {
		var swapRequest cashu.PostSwapRequest

		err := c.BindJSON(&swapRequest)
		if err != nil {
			slog.Info("Incorrect body", slog.Any("error", err))
			errorCode, details := utils.ParseErrorToCashuErrorCode(err)
			c.JSON(400, cashu.ErrorCodeToResponse(errorCode, details))
			return
		}

		response, err := mint.ExecuteSwap(c.Request.Context(), swapRequest)
		if err != nil {
			slog.Info("mint.ExecuteSwap(c.Request.Context(), swapRequest)", slog.Any("error", err))
			errorCode, details := utils.ParseErrorToCashuErrorCode(err)
			c.JSON(400, cashu.ErrorCodeToResponse(errorCode, details))
			return
		}
		go mint.Observer.SendProofsEvent(swapRequest.Inputs)
		c.JSON(200, response)
	})

	v1.POST("/checkstate", func(c *gin.Context) {
		var checkStateRequest cashu.PostCheckStateRequest
		err := c.BindJSON(&checkStateRequest)
		if err != nil {
			slog.Info("c.BindJSON(&checkStateRequest)", slog.Any("error", err))
			c.JSON(400, "Malformed Body")
			return
		}

		checkStateResponse := cashu.PostCheckStateResponse{
			States: make([]cashu.CheckState, 0),
		}

		states, err := m.CheckProofState(c.Request.Context(), mint, checkStateRequest.Ys)
		if err != nil {
			slog.Info("could not check proofs state", slog.Any("error", err))
			c.JSON(400, "could not validate proofs state")
			return
		}
		checkStateResponse.States = states

		c.JSON(200, checkStateResponse)
	})

	v1.POST("/restore", func(c *gin.Context) {
		var restoreRequest cashu.PostRestoreRequest
		err := c.BindJSON(&restoreRequest)

		if err != nil {
			slog.Info("c.BindJSON(&restoreRequest)", slog.Any("error", err))
			c.JSON(400, "Malformed body request")
			return
		}

		response, err := mint.Restore(c.Request.Context(), restoreRequest)
		if err != nil {
			slog.Info("mint.Restore(c.Request.Context(), restoreRequest)", slog.Any("error", err))
			errorCode, details := utils.ParseErrorToCashuErrorCode(err)
			c.JSON(400, cashu.ErrorCodeToResponse(errorCode, details))
			return
		}

		c.JSON(200, response)
	})
}
