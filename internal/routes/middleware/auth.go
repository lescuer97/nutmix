package middleware

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"regexp"
	// "strings"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-gonic/gin"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lescuer97/nutmix/internal/mint"
)

// ClearAuthMiddleware creates a middleware that checks for the "clear auth" header
// but only for paths that match patterns in the specified allowedPathPatterns list
func ClearAuthMiddleware(mint *mint.Mint, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		requestPath := c.Request.URL.Path

        log.Printf("requestPath: %v", requestPath)
		// Check if current path matches any of the patterns
		for _, pattern := range mint.Config.MINT_AUTH_CLEAR_AUTH_URLS {
            matches, err :=matchesPattern(requestPath, pattern)
            if err != nil {
                log.Panicf("This should not happen and something went wrong %+v. Patten: %s",err, pattern )
            }
			if matches {
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

// ClearAuthMiddleware creates a middleware that checks for the "clear auth" header
// but only for paths that match patterns in the specified allowedPathPatterns list
func BlindAuthMiddleware(mint *mint.Mint, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		requestPath := c.Request.URL.Path

        log.Printf("requestPath: %v", requestPath)
		// Check if current path matches any of the patterns
		for _, pattern := range mint.Config.MINT_AUTH_BLIND_AUTH_URLS {
            matches, err :=matchesPattern(requestPath, pattern)
            if err != nil {
                log.Panicf("This should not happen and something went wrong %+v. Patten: %s",err, pattern )
            }
			if matches {
				logger.Info("Trying to access restricted route")
				// For paths matching the pattern, check for the "clear auth" header
				blindAuth := c.GetHeader("Blind-auth")
				if blindAuth == "" {
					logger.Warn(fmt.Errorf("Tried to do a blind auth without token.").Error())
					c.JSON(400, cashu.ErrorCodeToResponse(cashu.ENDPOINT_REQUIRES_CLEAR_AUTH, nil))
					c.Abort()
					return
				}
                authProof, err := cashu.DecodeAuthToken(blindAuth)
                if err != nil {
					logger.Warn(fmt.Errorf("cashu.DecodeAuthToken(blindAuth). ").Error())
					c.JSON(400, cashu.ErrorCodeToResponse(cashu.BLIND_AUTH_FAILED, nil))
					c.Abort()
					return

                }

				err = mint.Signer.VerifyAuthProof(authProof)
				if err != nil {
					logger.Error(fmt.Errorf("mint.Signer.VerifyAuthProof(authProof). %w", err).Error())
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
func matchesPattern(path, pattern string) (bool, error) {
    regex, err := regexp.Compile(pattern)
    if err != nil {
        return false,  fmt.Errorf("regexp.Compile(pattern). %w", err)
    }
    return regex.MatchString(path), nil
}
