package admin

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// LogoutHandler handles user logout requests
func LogoutHandler(blacklist *TokenBlacklist) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Extract token from cookie
		tokenString, err := c.Cookie(AdminAuthKey)
		if err != nil {
			// No token found, redirect to login
			if c.GetHeader("HX-Request") == "true" {
				c.Header("HX-Redirect", "/admin/login")
				c.Status(http.StatusOK)
			} else {
				c.Redirect(http.StatusFound, "/admin/login")
			}
			return
		}

		// Add token to blacklist with expiration time
		// Parse token to get expiration time
		token, _ := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			// We don't need to validate here, just parse to get claims
			return []byte(""), nil
		})

		expirationTime := time.Now().Add(24 * time.Hour) // Default expiration
		if token != nil && token.Claims != nil {
			if claims, ok := token.Claims.(jwt.MapClaims); ok {
				if exp, ok := claims["exp"].(float64); ok {
					expirationTime = time.Unix(int64(exp), 0)
				}
			}
		}

		// Add token to blacklist
		blacklist.AddToken(tokenString, expirationTime)

		// Clear the cookie
		c.SetCookie(AdminAuthKey, "", -1, "/", "", false, true)

		// Send redirect to login page
		if c.GetHeader("HX-Request") == "true" {
			c.Header("HX-Redirect", "/admin/login")
			c.Status(http.StatusOK)
		} else {
			c.Redirect(http.StatusFound, "/admin/login")
		}
	}
}
