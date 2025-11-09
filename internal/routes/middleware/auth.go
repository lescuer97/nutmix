package middleware

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lescuer97/nutmix/internal/mint"
	"log"
	"log/slog"
	"regexp"
)

// ClearAuthMiddleware creates a middleware that checks for the "clear auth" header
// but only for paths that match patterns in the specified allowedPathPatterns list
func ClearAuthMiddleware(mint *mint.Mint) gin.HandlerFunc {
	return func(c *gin.Context) {
		requestPath := c.Request.URL.Path

		if !mint.Config.MINT_REQUIRE_AUTH {
			c.Next()
		}
		if mint.Config.MINT_REQUIRE_AUTH {
			// Check if current path matches any of the patterns
			for _, pattern := range mint.Config.MINT_AUTH_CLEAR_AUTH_URLS {
				if !mint.Config.MINT_REQUIRE_AUTH {
					log.Panicf("mint require auth should always be on when using the middleware")
				}

				matches, err := matchesPattern(requestPath, pattern)
				if err != nil {
					log.Panicf("This should not happen and something went wrong %+v. Patten: %s", err, pattern)
				}
				if matches {
					slog.Info("Trying to access restricted route")
					// For paths matching the pattern, check for the "clear auth" header
					clearAuth := c.GetHeader("Clear-auth")
					if clearAuth == "" {
						slog.Warn("Tried to do a clear auth without token.")
						c.JSON(401, cashu.ErrorCodeToResponse(cashu.ENDPOINT_REQUIRES_CLEAR_AUTH, nil))
						c.Abort()
						return
					}
					// check if it's valid token
					token := c.GetHeader("Clear-auth")
					err := mint.VerifyAuthClearToken(token)
					if err != nil {
						slog.Error("mint.VerifyAuthClearToken(token)", slog.Any("error", err))
						c.JSON(400, cashu.ErrorCodeToResponse(cashu.CLEAR_AUTH_FAILED, nil))
						return
					}
					// Header exists, continue processing
					break
				}
			}
		}

		// Continue to the next middleware/handler
		c.Next()
	}
}

// ClearAuthMiddleware creates a middleware that checks for the "clear auth" header
// but only for paths that match patterns in the specified allowedPathPatterns list
func BlindAuthMiddleware(mint *mint.Mint) gin.HandlerFunc {
	return func(c *gin.Context) {
		requestPath := c.Request.URL.Path

		if !mint.Config.MINT_REQUIRE_AUTH {
			c.Next()
		}
		if mint.Config.MINT_REQUIRE_AUTH {
			// Check if current path matches any of the patterns
			for _, pattern := range mint.Config.MINT_AUTH_BLIND_AUTH_URLS {
				if !mint.Config.MINT_REQUIRE_AUTH {
					log.Panicf("mint require auth should always be on when using the middleware")
				}
				matches, err := matchesPattern(requestPath, pattern)
				if err != nil {
					log.Panicf("This should not happen and something went wrong %+v. Patten: %s", err, pattern)
				}
				if matches {
					slog.Info("Trying to access restricted route")
					// For paths matching the pattern, check for the "clear auth" header
					blindAuth := c.GetHeader("Blind-auth")
					if blindAuth == "" {
						slog.Warn("Tried to do a blind auth without token.")
						c.JSON(401, cashu.ErrorCodeToResponse(cashu.ENDPOINT_REQUIRES_BLIND_AUTH, nil))
						c.Abort()
						return
					}
					authProof, err := cashu.DecodeAuthToken(blindAuth)
					if err != nil {
						slog.Warn("cashu.DecodeAuthToken(blindAuth)")
						c.JSON(400, cashu.ErrorCodeToResponse(cashu.BLIND_AUTH_FAILED, nil))
						c.Abort()
						return
					}

					authProof.Amount = 1
					err = mint.VerifyAuthBlindToken(authProof)
					if err != nil {
						slog.Error("mint.VerifyAuthBlindToken(authProof)", slog.Any("error", err))
						c.JSON(400, cashu.ErrorCodeToResponse(cashu.BLIND_AUTH_FAILED, nil))
						return
					}
					// Header exists, continue processing
					break
				}
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
		return false, fmt.Errorf("regexp.Compile(pattern). %w", err)
	}
	return regex.MatchString(path), nil
}
