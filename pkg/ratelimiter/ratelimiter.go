package ratelimiter

import (
	"fmt"
	"github.com/jofosuware/go/shopit/pkg/utils"
	"net/http"
	"sync"

	"golang.org/x/time/rate"
)

type RateLimiter struct {
	visitors map[string]*rate.Limiter
	mu       sync.Mutex
	rate     rate.Limit
	burst    int
}

// NewRateLimiter creates a new RateLimiter instance
func NewRateLimiter(r rate.Limit, b int) *RateLimiter {
	return &RateLimiter{
		visitors: make(map[string]*rate.Limiter),
		rate:     r,
		burst:    b,
	}
}

// AddVisitor adds a new visitor with a rate limiter
func (rl *RateLimiter) AddVisitor(ip string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	limiter := rate.NewLimiter(rl.rate, rl.burst)
	rl.visitors[ip] = limiter
	return limiter
}

// GetLimiter retrieves the limiter for an IP, creating one if it doesn't exist
func (rl *RateLimiter) GetLimiter(ip string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	limiter, exists := rl.visitors[ip]
	if !exists {
		limiter = rate.NewLimiter(rl.rate, rl.burst)
		rl.visitors[ip] = limiter
	}
	return limiter
}

// Middleware for rate limiting
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := r.RemoteAddr
		limiter := rl.GetLimiter(ip)

		if !limiter.Allow() {
			_ = utils.TooManyRequests(w)
			fmt.Println("Too many requests")
			return
		}

		next.ServeHTTP(w, r)
	})
}
