package middleware

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lescuer97/nutmix/internal/mint"
)

// ClearAuthMiddleware creates a middleware that checks for the "clear auth" header
// but only for paths that match patterns in the specified allowedPathPatterns list
func ClearAuthMiddleware(mint *mint.Mint) gin.HandlerFunc {
	return func(c *gin.Context) {
		requestPath := c.Request.URL.Path

		if mint.Config.MINT_REQUIRE_AUTH {
			// Check if current path matches any of the patterns
			for _, pattern := range mint.Config.MINT_AUTH_CLEAR_AUTH_URLS {
				if err := cashu.ValidateAuthEndpointPattern(pattern); err != nil {
					slog.Error("Invalid clear auth endpoint configuration", slog.Any("error", err))
					c.JSON(500, cashu.ErrorCodeToResponse(cashu.CLEAR_AUTH_FAILED, nil))
					c.Abort()
					return
				}
				matches, err := matchesPattern(requestPath, pattern)
				if err != nil {
					slog.Error("Invalid clear auth endpoint configuration", slog.String("pattern", pattern), slog.Any("error", err))
					c.JSON(500, cashu.ErrorCodeToResponse(cashu.CLEAR_AUTH_FAILED, nil))
					c.Abort()
					return
				}
				if matches {
					clearAuth := c.GetHeader("Clear-auth")
					if clearAuth == "" {
						slog.Warn("Tried to do a clear auth without token.")
						c.JSON(401, cashu.ErrorCodeToResponse(cashu.ENDPOINT_REQUIRES_CLEAR_AUTH, nil))
						c.Abort()
						return
					}
					if mint.OICDClient == nil {
						ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
						defer cancel()
						err := mint.SetupOidcService(ctx, mint.Config.MINT_AUTH_OICD_URL)
						if err != nil {
							slog.Error("Could not setup oidc service during middleware.", slog.Any("error", err))
							errMsg := "This is a mint connectin error with the oidc service"
							c.JSON(400, cashu.ErrorCodeToResponse(cashu.CLEAR_AUTH_FAILED, &errMsg))
							c.Abort()
							return
						}
					}
					slog.Info("Trying to access restricted route")
					// For paths matching the pattern, check for the "clear auth" header
					// check if it's valid token
					token := c.GetHeader("Clear-auth")
					err := mint.VerifyAuthClearToken(token)
					if err != nil {
						slog.Warn("mint.VerifyAuthClearToken(token)", slog.Any("error", err))
						c.JSON(400, cashu.ErrorCodeToResponse(cashu.CLEAR_AUTH_FAILED, nil))
						c.Abort()
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
			return
		}
		if mint.Config.MINT_REQUIRE_AUTH {
			// Check if current path matches any of the patterns
			for _, pattern := range mint.Config.MINT_AUTH_BLIND_AUTH_URLS {
				if err := cashu.ValidateAuthEndpointPattern(pattern); err != nil {
					slog.Error("Invalid blind auth endpoint configuration", slog.Any("error", err))
					c.JSON(500, cashu.ErrorCodeToResponse(cashu.BLIND_AUTH_FAILED, nil))
					c.Abort()
					return
				}
				matches, err := matchesPattern(requestPath, pattern)
				if err != nil {
					slog.Error("Invalid blind auth endpoint configuration", slog.String("pattern", pattern), slog.Any("error", err))
					c.JSON(500, cashu.ErrorCodeToResponse(cashu.BLIND_AUTH_FAILED, nil))
					c.Abort()
					return
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
						slog.Warn("mint.VerifyAuthBlindToken(authProof)", slog.Any("error", err))
						c.JSON(400, cashu.ErrorCodeToResponse(cashu.BLIND_AUTH_FAILED, nil))
						c.Abort()
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
	if strings.Contains(pattern, "*") && (!strings.HasSuffix(pattern, "*") || strings.Count(pattern, "*") != 1) {
		return false, fmt.Errorf("wildcard must appear only once at the end")
	}
	if strings.HasSuffix(pattern, "*") {
		return strings.HasPrefix(path, strings.TrimSuffix(pattern, "*")), nil
	}
	return path == pattern, nil
}
