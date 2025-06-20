package cache

import (
	"bytes"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gin-contrib/cache/persistence"
)

// Cache-Control header
const ( 
	HeaderCacheControl = "Cache-Control"
	HeaderExpires = "Expires"
	HeaderLastModified = "Last-Modified"
	HeaderETag = "ETag"
)



func Cache200(store persistence.CacheStore, timeout time.Duration, handle gin.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		var cache responseCache
		key := c.Request.RequestURI
		err := store.Get(key, &cache)
		if err == nil {
			c.Writer.WriteHeader(cache.Status)
			for k, v := range cache.Header {
				c.Writer.Header().Set(k, v[0])
			}
			c.Writer.Write(cache.Data)
			return
		}

		writer := &responseCacheWriter{
			ResponseWriter: c.Writer,
			body:           &bytes.Buffer{},
		}
		c.Writer = writer

		handle(c)

		// Drop caches that are not 200
		if writer.Status() != 200 {
			log.Printf("Response status is %d, not caching", writer.Status())
			return
		}

		val := responseCache{
			Status: writer.Status(),
			Header: writer.Header(),
			Data:   writer.body.Bytes(),
		}
		err = store.Set(key, val, timeout)
		if err != nil {
			log.Printf("Failed to cache response: %v", err)
		}
	}
}



type responseCache struct {
	Status int
	Header http.Header
	Data   []byte
}

type responseCacheWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (w *responseCacheWriter) Write(data []byte) (int, error) {
	ret, err := w.body.Write(data)
	if err != nil {
		return 0, err
	}

	return ret, nil
}

func (w *responseCacheWriter) WriteString(s string) (int, error) {
	ret, err := w.body.WriteString(s)
	if err != nil {
		return 0, err
	}

	return ret, nil
}
