package middleware

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-gonic/gin"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lescuer97/nutmix/internal/mint"
)

// ClearAuthMiddleware creates a middleware that checks for the "clear auth" header
// but only for paths that match patterns in the specified allowedPathPatterns list
func ClearAuthMiddleware(allowedPathPatterns []string, mint *mint.Mint, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		requestPath := c.Request.URL.Path

		// Check if current path matches any of the patterns
		for _, pattern := range allowedPathPatterns {
			if matchesPattern(requestPath, pattern) {
                logger.Info("Trying to access restricted route")
				// For paths matching the pattern, check for the "clear auth" header
				clearAuth := c.GetHeader("Clear-auth")
				if clearAuth == "" {
		            logger.Warn(fmt.Errorf("Tried to do a clear auth without token.").Error())
		            c.JSON(400, cashu.ErrorCodeToResponse(cashu.ENDPOINT_REQUIRES_CLEAR_AUTH, nil))
					c.Abort()
					return
				}
                    verifier := mint.OICDClient.Verifier(&oidc.Config{ClientID: mint.Config.MINT_AUTH_OICD_CLIENT_ID})
		            // check if it's valid token
		            token := c.GetHeader("Clear-auth")

                    ctx := context.Background()
		            _, err := verifier.Verify(ctx, token)
		            if err != nil {
		            	logger.Error(fmt.Errorf("verifier.Verify(ctx,token ). %w", err).Error())
		            	c.JSON(400, cashu.ErrorCodeToResponse(cashu.CLEAR_AUTH_FAILED, nil))
		            	return
		            }
				// Header exists, continue processing
				break
			}
		}

		// Continue to the next middleware/handler
		c.Next()
	}
}

// matchesPattern checks if a path matches a pattern
// Simple implementation that handles wildcards at the end of paths (e.g., /v1/mint/*)
func matchesPattern(path, pattern string) bool {
	// If pattern ends with *, it's a wildcard match
	if strings.HasSuffix(pattern, "/*") {
		prefix := pattern[:len(pattern)-2] // Remove the "/*"
		return strings.HasPrefix(path, prefix)
	}
	
	// Otherwise, exact match
	return path == pattern
}
