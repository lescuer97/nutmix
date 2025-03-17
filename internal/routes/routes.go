package routes

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/lescuer97/nutmix/internal/mint"
	"github.com/lescuer97/nutmix/internal/routes/middleware"
)

func V1Routes(r *gin.Engine, mint *mint.Mint, logger *slog.Logger) {
    if mint.Config.MINT_REQUIRE_AUTH {
        r.Use(middleware.ClearAuthMiddleware([]string{"/v1/auth/blind/mint", "/v1/swap"},mint, logger))
		v1AuthRoutes(r, mint, logger)
    }
	v1MintRoutes(r, mint, logger)
	v1bolt11Routes(r, mint, logger)
	v1WebSocketRoute(r, mint, logger)

}
