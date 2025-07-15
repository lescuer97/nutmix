package middleware

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gin-contrib/cache/persistence"
	"github.com/gin-gonic/gin"
)

type responseWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (w responseWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

var cachedPaths = map[string]bool{
	"/v1/mint/bolt11": true,
	"/v1/melt/bolt11": true,
	"/v1/swap":        true,
}

func CacheMiddleware(store *persistence.InMemoryStore) gin.HandlerFunc {
	return func(c *gin.Context) {

		if !cachedPaths[c.Request.URL.Path] {
			c.Next()
			return
		}

		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.Next()
			return
		}
		c.Request.Body = io.NopCloser(bytes.NewBuffer(body))

		hash := sha256.Sum256(body)
		cacheKey := c.Request.URL.Path + "-" + fmt.Sprintf("%x", hash)

		var cachedResponse []byte
		if err := store.Get(cacheKey, &cachedResponse); err == nil {
			c.Data(http.StatusOK, "application/json; charset=utf-8", cachedResponse)
			c.Abort()
			return
		}

		w := &responseWriter{body: &bytes.Buffer{}, ResponseWriter: c.Writer}
		c.Writer = w
		c.Next()
		if c.Writer.Status() == http.StatusOK {
			store.Set(cacheKey, w.body.Bytes(), 45*time.Minute)
		}
	}
}
