package ratelimiter_test

import (
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jofosuware/go/shopit/pkg/ratelimiter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"
)

func TestNewRateLimiter(t *testing.T) {
	rl := ratelimiter.NewRateLimiter(1, 5)
	require.NotNil(t, rl)
}

func TestAddVisitor(t *testing.T) {
	rl := ratelimiter.NewRateLimiter(1, 1)
	limiter := rl.AddVisitor("192.168.1.1")
	require.NotNil(t, limiter)

	require.True(t, limiter.Allow())
	limiter2 := rl.GetLimiter("192.168.1.1")
	assert.Equal(t, limiter, limiter2)
	assert.False(t, limiter2.Allow())
}

func TestNewRateLimiterWithOptionsRejectsInvalidConfiguration(t *testing.T) {
	tests := []struct {
		name    string
		options ratelimiter.Options
		wantErr error
	}{
		{
			name:    "non-positive rate",
			options: ratelimiter.Options{Rate: 0, Burst: 1, MaxVisitors: 1, VisitorTTL: time.Minute},
			wantErr: ratelimiter.ErrInvalidRate,
		},
		{
			name:    "not-a-number rate",
			options: ratelimiter.Options{Rate: rate.Limit(math.NaN()), Burst: 1, MaxVisitors: 1, VisitorTTL: time.Minute},
			wantErr: ratelimiter.ErrInvalidRate,
		},
		{
			name:    "infinite rate",
			options: ratelimiter.Options{Rate: rate.Limit(math.Inf(1)), Burst: 1, MaxVisitors: 1, VisitorTTL: time.Minute},
			wantErr: ratelimiter.ErrInvalidRate,
		},
		{
			name:    "non-positive burst",
			options: ratelimiter.Options{Rate: 1, Burst: 0, MaxVisitors: 1, VisitorTTL: time.Minute},
			wantErr: ratelimiter.ErrInvalidBurst,
		},
		{
			name:    "non-positive visitor capacity",
			options: ratelimiter.Options{Rate: 1, Burst: 1, MaxVisitors: 0, VisitorTTL: time.Minute},
			wantErr: ratelimiter.ErrInvalidMaxVisitors,
		},
		{
			name:    "non-positive visitor TTL",
			options: ratelimiter.Options{Rate: 1, Burst: 1, MaxVisitors: 1, VisitorTTL: 0},
			wantErr: ratelimiter.ErrInvalidVisitorTTL,
		},
		{
			name: "invalid trusted proxy CIDR",
			options: ratelimiter.Options{
				Rate: 1, Burst: 1, MaxVisitors: 1, VisitorTTL: time.Minute,
				TrustedProxyCIDRs: []string{"not-a-cidr"},
			},
			wantErr: ratelimiter.ErrInvalidTrustedProxyCIDR,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limiter, err := ratelimiter.NewRateLimiterWithOptions(tt.options)
			require.Nil(t, limiter)
			require.ErrorIs(t, err, tt.wantErr)
		})
	}
}

func TestGetLimiterNormalizesIPAndBoundsVisitorState(t *testing.T) {
	rl, err := ratelimiter.NewRateLimiterWithOptions(ratelimiter.Options{
		Rate:        1,
		Burst:       1,
		MaxVisitors: 1,
		VisitorTTL:  time.Minute,
	})
	require.NoError(t, err)

	first := rl.GetLimiter("192.0.2.1:1234")
	require.NotNil(t, first)
	require.Same(t, first, rl.GetLimiter("192.0.2.1"))
	require.NotNil(t, rl.GetLimiter("192.0.2.2"))
}

func TestGetLimiterReclaimsExpiredVisitor(t *testing.T) {
	rl, err := ratelimiter.NewRateLimiterWithOptions(ratelimiter.Options{
		Rate:        1,
		Burst:       1,
		MaxVisitors: 1,
		VisitorTTL:  10 * time.Millisecond,
	})
	require.NoError(t, err)

	first := rl.GetLimiter("192.0.2.1")
	require.NotNil(t, first)
	require.True(t, first.Allow())
	require.False(t, first.Allow())

	time.Sleep(20 * time.Millisecond)

	second := rl.GetLimiter("192.0.2.2")
	require.NotNil(t, second)
	require.NotSame(t, first, second)
	require.True(t, second.Allow())
}

func TestBlockedRequestsDoNotKeepVisitorStateAlive(t *testing.T) {
	rl, err := ratelimiter.NewRateLimiterWithOptions(ratelimiter.Options{
		Rate:        1,
		Burst:       1,
		MaxVisitors: 1,
		VisitorTTL:  20 * time.Millisecond,
	})
	require.NoError(t, err)
	handler := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	send := func(remoteAddr string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(http.MethodGet, "/test", nil)
		request.RemoteAddr = remoteAddr
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		return response
	}

	require.Equal(t, http.StatusOK, send("192.0.2.1:1234").Code)
	require.Equal(t, http.StatusTooManyRequests, send("192.0.2.1:1234").Code)
	time.Sleep(15 * time.Millisecond)
	require.Equal(t, http.StatusTooManyRequests, send("192.0.2.1:1234").Code)
	time.Sleep(15 * time.Millisecond)
	require.Equal(t, http.StatusOK, send("192.0.2.2:1234").Code)
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

	t.Run("ignores untrusted forwarded headers", func(t *testing.T) {
		rl := ratelimiter.NewRateLimiter(1, 1)
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
		middleware := rl.Middleware(handler)

		first := httptest.NewRequest(http.MethodGet, "/test", nil)
		first.RemoteAddr = "192.0.2.1:1234"
		first.Header.Set("X-Forwarded-For", "198.51.100.1")
		firstResponse := httptest.NewRecorder()
		middleware.ServeHTTP(firstResponse, first)
		require.Equal(t, http.StatusOK, firstResponse.Code)

		second := httptest.NewRequest(http.MethodGet, "/test", nil)
		second.RemoteAddr = "192.0.2.1:5678"
		second.Header.Set("X-Forwarded-For", "198.51.100.2")
		secondResponse := httptest.NewRecorder()
		middleware.ServeHTTP(secondResponse, second)
		require.Equal(t, http.StatusTooManyRequests, secondResponse.Code)
		assert.Equal(t, "1", secondResponse.Header().Get("Retry-After"))
	})

	t.Run("uses forwarded address only through explicitly trusted proxies", func(t *testing.T) {
		rl, err := ratelimiter.NewRateLimiterWithOptions(ratelimiter.Options{
			Rate: 1, Burst: 1, MaxVisitors: 10, VisitorTTL: time.Minute,
			TrustedProxyCIDRs: []string{"10.0.0.0/8"},
		})
		require.NoError(t, err)
		middleware := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		send := func(forwarded string) *httptest.ResponseRecorder {
			request := httptest.NewRequest(http.MethodGet, "/test", nil)
			request.RemoteAddr = "10.0.0.2:443"
			request.Header.Set("X-Forwarded-For", forwarded)
			response := httptest.NewRecorder()
			middleware.ServeHTTP(response, request)
			return response
		}

		require.Equal(t, http.StatusOK, send("198.51.100.1, 10.0.0.1").Code)
		blocked := send("203.0.113.1, 10.0.0.1")
		require.Equal(t, http.StatusOK, blocked.Code)

		sameClient := send("198.51.100.1, 10.0.0.3")
		require.Equal(t, http.StatusTooManyRequests, sameClient.Code)
	})

	t.Run("sets retry-after from the configured refill rate", func(t *testing.T) {
		rl := ratelimiter.NewRateLimiter(0.1, 1)
		middleware := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		request := httptest.NewRequest(http.MethodGet, "/test", nil)
		request.RemoteAddr = "192.0.2.1:1234"

		first := httptest.NewRecorder()
		middleware.ServeHTTP(first, request)
		require.Equal(t, http.StatusOK, first.Code)

		second := httptest.NewRecorder()
		middleware.ServeHTTP(second, request)
		require.Equal(t, http.StatusTooManyRequests, second.Code)
		assert.Equal(t, "10", second.Header().Get("Retry-After"))
	})

	t.Run("evicts visitor state when capacity is reached", func(t *testing.T) {
		rl, err := ratelimiter.NewRateLimiterWithOptions(ratelimiter.Options{
			Rate:        1,
			Burst:       1,
			MaxVisitors: 1,
			VisitorTTL:  time.Minute,
		})
		require.NoError(t, err)
		handler := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		rl.GetLimiter("192.0.2.1")

		request := httptest.NewRequest(http.MethodGet, "/test", nil)
		request.RemoteAddr = "192.0.2.2:1234"
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)

		require.Equal(t, http.StatusOK, response.Code)
	})
}
