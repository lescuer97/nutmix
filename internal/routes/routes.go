package routes

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/lescuer97/nutmix/internal/mint"
)

func V1Routes(r *gin.Engine, mint *mint.Mint, logger *slog.Logger) {
	v1MintRoutes(r, mint, logger)
	v1bolt11Routes(r, mint, logger)
	v1WebSocketRoute(r, mint, logger)
	if mint.Config.MINT_REQUIRE_AUTH {
		v1AuthRoutes(r, mint, logger)

	}

}
