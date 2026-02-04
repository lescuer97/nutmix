package middleware

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
)

// TimeoutMiddleware creates a middleware that adds a per-request timeout to the context
// This ensures c.Request.Context() has a deadline set
func TimeoutMiddleware(timeout time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Wrap the existing request context with a timeout
		ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
		defer cancel()

		// Update the request inside Gin to point to the new context
		c.Request = c.Request.WithContext(ctx)

		c.Next()
	}
}
