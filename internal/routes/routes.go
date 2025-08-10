package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/lescuer97/nutmix/internal/mint"
	"github.com/lescuer97/nutmix/internal/routes/middleware"
)

func V1Routes(r *gin.Engine, mint *mint.Mint) {
	r.Use(middleware.ClearAuthMiddleware(mint))
	r.Use(middleware.BlindAuthMiddleware(mint))
	v1AuthRoutes(r, mint)
	v1MintRoutes(r, mint)
	v1bolt11Routes(r, mint)
	v1WebSocketRoute(r, mint)

}
