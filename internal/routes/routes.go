package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lescuer97/nutmix/internal/mint"
)

func V1Routes(r *gin.Engine, pool *pgxpool.Pool, mint mint.Mint) {
	v1MintRoutes(r, pool, mint)
	v1bolt11Routes(r, pool, mint)

}
