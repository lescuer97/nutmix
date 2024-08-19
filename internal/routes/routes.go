package routes

import (
	"context"
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lescuer97/nutmix/internal/mint"
)

func V1Routes(ctx context.Context, r *gin.Engine, pool *pgxpool.Pool, mint *mint.Mint, logger *slog.Logger) {
	v1MintRoutes(r, pool, mint, logger)
	v1bolt11Routes(ctx, r, pool, mint, logger)

}
