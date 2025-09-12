package ratelimiter_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jofosuware/go/shopit/pkg/ratelimiter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRateLimiter(t *testing.T) {
	rl := ratelimiter.NewRateLimiter(1, 5)
	require.NotNil(t, rl)
}

func TestAddVisitor(t *testing.T) {
	rl := ratelimiter.NewRateLimiter(1, 5)
	limiter := rl.AddVisitor("192.168.1.1")
	require.NotNil(t, limiter)

	// Test getting same visitor
	limiter2 := rl.GetLimiter("192.168.1.1")
	assert.Equal(t, limiter, limiter2)
}

func TestGetLimiter(t *testing.T) {
	rl := ratelimiter.NewRateLimiter(1, 5)

	t.Run("new visitor", func(t *testing.T) {
		limiter := rl.GetLimiter("192.168.1.2")
		require.NotNil(t, limiter)
	})

	t.Run("existing visitor", func(t *testing.T) {
		ip := "192.168.1.3"
		limiter1 := rl.GetLimiter(ip)
		limiter2 := rl.GetLimiter(ip)
		assert.Equal(t, limiter1, limiter2)
	})
}

func TestRateLimiterMiddleware(t *testing.T) {
	t.Run("allows request within limit", func(t *testing.T) {
		rl := ratelimiter.NewRateLimiter(1, 1)
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
		middleware := rl.Middleware(handler)

		req := httptest.NewRequest("GET", "/test", nil)
		rr := httptest.NewRecorder()
		middleware.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("blocks request exceeding limit", func(t *testing.T) {
		rl := ratelimiter.NewRateLimiter(1, 1)
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
		middleware := rl.Middleware(handler)

		req := httptest.NewRequest("GET", "/test", nil)
		rr := httptest.NewRecorder()

		// First request should pass
		middleware.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)

		// Second immediate request should be blocked
		rr = httptest.NewRecorder()
		middleware.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusTooManyRequests, rr.Code)
	})

	t.Run("allows request after rate limit reset", func(t *testing.T) {
		rl := ratelimiter.NewRateLimiter(1, 1)
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
		middleware := rl.Middleware(handler)

		req := httptest.NewRequest("GET", "/test", nil)
		rr := httptest.NewRecorder()

		middleware.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)

		// Wait for rate limit to reset
		time.Sleep(time.Second)

		rr = httptest.NewRecorder()
		middleware.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)
	})
}
