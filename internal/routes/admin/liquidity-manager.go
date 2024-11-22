package admin

import (
	"context"
	"errors"
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/lescuer97/nutmix/internal/routes/admin/templates"
)

func LiquidityButton(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
        ctx := context.Background()
        component := templates.LiquidityButton()

        err := component.Render(ctx, c.Writer)
        if err != nil {
			 c.Error(errors.New("component.Render(ctx, c.Writer)"))
             return
        }

        return
	}
}

func LiquidityForm(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
        ctx := context.Background()
        component := templates.BoltzPostForm()

        err := component.Render(ctx, c.Writer)
        if err != nil {
			 c.Error(errors.New("component.Render(ctx, c.Writer)"))
             return
        }

        return
	}
}
