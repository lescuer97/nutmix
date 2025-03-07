package routes

import (
	"github.com/gin-gonic/gin"
	m "github.com/lescuer97/nutmix/internal/mint"
	"log/slog"
)

func v1AuthRoutes(r *gin.Engine, mint *m.Mint, logger *slog.Logger) {
	v1 := r.Group("/v1")
	auth := v1.Group("/auth")

	auth.GET("/blind/keys", func(c *gin.Context) {
	})

	auth.GET("/blind/keys/:id", func(c *gin.Context) {
	})
	auth.GET("/blind/keys/keysets", func(c *gin.Context) {
	})

	auth.GET("/blind/mint", func(c *gin.Context) {
	})
}
