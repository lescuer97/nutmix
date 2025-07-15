package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-contrib/cache/persistence"
	"github.com/gin-gonic/gin"
	"github.com/lescuer97/nutmix/internal/routes/middleware"
)

func TestCacheMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	store := persistence.NewInMemoryStore(1 * time.Minute)

	handerCallCount := 0
	mutex := &sync.Mutex{}

	r := gin.New()

	r.Use(middleware.CacheMiddleware(store))

	r.POST("/v1/swap", func(c *gin.Context) {
		mutex.Lock()
		handerCallCount++
		mutex.Unlock()

		if c.Query("fail") == "true" {
			c.JSON(http.StatusBadRequest, gin.H{"status": "failed"})
		} else {
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		}
	})

	// Test case 1: Successful request should be cached
	t.Run("caches successful responses", func(t *testing.T) {
		handerCallCount = 0 // Reset counter

		body := `{"key":"value"}`

		// First request
		req1, _ := http.NewRequest("POST", "/v1/swap", bytes.NewBufferString(body))
		w1 := httptest.NewRecorder()
		r.ServeHTTP(w1, req1)

		if w1.Code != http.StatusOK {
			t.Errorf("Expected status OK, got %d", w1.Code)
		}
		if handerCallCount != 1 {
			t.Errorf("Expected handler to be called once, got %d", handerCallCount)
		}

		// Second request - should be served from cache
		req2, _ := http.NewRequest("POST", "/v1/swap", bytes.NewBufferString(body))
		w2 := httptest.NewRecorder()
		r.ServeHTTP(w2, req2)

		if w2.Code != http.StatusOK {
			t.Errorf("Expected status OK, got %d", w2.Code)
		}
		if handerCallCount != 1 {
			t.Errorf("Expected handler to be called once, got %d", handerCallCount)
		}
	})

	// Test case 2: Failed request should not be cached
	t.Run("does not cache failed responses", func(t *testing.T) {
		handerCallCount = 0 // Reset counter

		body := `{"key":"failure"}`

		// First request
		req1, _ := http.NewRequest("POST", "/v1/swap?fail=true", bytes.NewBufferString(body))
		w1 := httptest.NewRecorder()
		r.ServeHTTP(w1, req1)

		if w1.Code != http.StatusBadRequest {
			t.Errorf("Expected status Bad Request, got %d", w1.Code)
		}
		if handerCallCount != 1 {
			t.Errorf("Expected handler to be called once, got %d", handerCallCount)
		}

		// Second request - should not be cached
		req2, _ := http.NewRequest("POST", "/v1/swap?fail=true", bytes.NewBufferString(body))
		w2 := httptest.NewRecorder()
		r.ServeHTTP(w2, req2)

		if w2.Code != http.StatusBadRequest {
			t.Errorf("Expected status Bad Request, got %d", w2.Code)
		}
		if handerCallCount != 2 {
			t.Errorf("Expected handler to be called twice, got %d", handerCallCount)
		}
	})
}
