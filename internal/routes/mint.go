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
			utils.JSON(c, 400, cashu.ErrorCodeToResponse(cashu.KEYSET_NOT_KNOW, nil))
			return
		}

		utils.JSON(c, 200, keys)
	})

	v1.GET("/keys/:id", func(c *gin.Context) {
		id := c.Param("id")

		keysets, err := mint.Signer.GetKeysById(id)

		if err != nil {
			slog.Warn("mint.Signer.GetKeysById(id)", slog.Any("error", err))
			utils.JSON(c, 400, cashu.ErrorCodeToResponse(cashu.KEYSET_NOT_KNOW, nil))
			return
		}

		utils.JSON(c, 200, keysets)
	})

	v1.GET("/keysets", func(c *gin.Context) {
		keys, err := mint.Signer.GetKeysets()
		if err != nil {
			slog.Error("mint.Signer.GetKeys()", slog.Any("error", err))
			utils.JSON(c, 500, "Server side error")
			return
		}

		utils.JSON(c, 200, keys)
	})

	v1.GET("/info", func(c *gin.Context) {
		info := mint.Info()
		utils.JSON(c, 200, info)
	})

	v1.POST("/swap", func(c *gin.Context) {
		var swapRequest cashu.PostSwapRequest

		err := utils.DecodeJSONV2(c, &swapRequest)
		if err != nil {
			slog.Info("Incorrect body", slog.Any("error", err))
			errorCode, details := utils.ParseErrorToCashuErrorCode(err)
			utils.JSON(c, 400, cashu.ErrorCodeToResponse(errorCode, details))
			return
		}

		response, err := mint.ExecuteSwap(c.Request.Context(), swapRequest)
		if err != nil {
			slog.Info("mint.ExecuteSwap(c.Request.Context(), swapRequest)", slog.Any("error", err))
			errorCode, details := utils.ParseErrorToCashuErrorCode(err)
			utils.JSON(c, 400, cashu.ErrorCodeToResponse(errorCode, details))
			return
		}
		utils.JSON(c, 200, response)
	})

	v1.POST("/checkstate", func(c *gin.Context) {
		var checkStateRequest cashu.PostCheckStateRequest
		err := utils.DecodeJSONV2(c, &checkStateRequest)
		if err != nil {
			slog.Info("utils.DecodeJSONV2(c, &checkStateRequest)", slog.Any("error", err))
			utils.JSON(c, 400, "Malformed Body")
			return
		}

		checkStateResponse := cashu.PostCheckStateResponse{
			States: make([]cashu.CheckState, 0),
		}

		states, err := m.CheckProofState(c.Request.Context(), mint, checkStateRequest.Ys)
		if err != nil {
			slog.Info("could not check proofs state", slog.Any("error", err))
			utils.JSON(c, 400, "could not validate proofs state")
			return
		}
		checkStateResponse.States = states

		utils.JSON(c, 200, checkStateResponse)
	})

	v1.POST("/restore", func(c *gin.Context) {
		var restoreRequest cashu.PostRestoreRequest
		err := utils.DecodeJSONV2(c, &restoreRequest)

		if err != nil {
			slog.Info("utils.DecodeJSONV2(c, &restoreRequest)", slog.Any("error", err))
			utils.JSON(c, 400, "Malformed body request")
			return
		}

		response, err := mint.Restore(c.Request.Context(), restoreRequest)
		if err != nil {
			slog.Info("mint.Restore(c.Request.Context(), restoreRequest)", slog.Any("error", err))
			errorCode, details := utils.ParseErrorToCashuErrorCode(err)
			utils.JSON(c, 400, cashu.ErrorCodeToResponse(errorCode, details))
			return
		}

		utils.JSON(c, 200, response)
	})
}
