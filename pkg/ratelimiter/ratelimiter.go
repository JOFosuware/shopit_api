package ratelimiter

import (
	"errors"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jofosuware/go/shopit/pkg/utils"
	"golang.org/x/time/rate"
)

const (
	defaultMaxVisitors = 10_000
	defaultVisitorTTL  = 10 * time.Minute
)

var (
	ErrInvalidRate             = errors.New("rate must be positive and finite")
	ErrInvalidBurst            = errors.New("burst must be positive")
	ErrInvalidMaxVisitors      = errors.New("maximum visitors must be positive")
	ErrInvalidVisitorTTL       = errors.New("visitor TTL must be positive")
	ErrInvalidTrustedProxyCIDR = errors.New("trusted proxy CIDR is invalid")
)

// Options configures an in-memory, per-client token-bucket rate limiter.
type Options struct {
	Rate              rate.Limit
	Burst             int
	MaxVisitors       int
	VisitorTTL        time.Duration
	TrustedProxyCIDRs []string
}

type visitor struct {
	limiter  *rate.Limiter
	lastSeen atomic.Int64
}

// RateLimiter applies independent token buckets keyed by the request's peer IP.
// Its state is local to this process and should not be used as a cluster-wide
// limit when the service runs multiple instances.
type RateLimiter struct {
	mu              sync.RWMutex
	visitors        map[string]*visitor
	rate            rate.Limit
	burst           int
	max             int
	ttl             time.Duration
	cleanupInterval time.Duration
	lastCleanup     time.Time
	trustedProxies  []*net.IPNet
}

// NewRateLimiter creates a limiter with bounded visitor state and default
// cleanup settings. It panics if the rate or burst is invalid; use
// NewRateLimiterWithOptions when configuration errors need to be handled.
func NewRateLimiter(r rate.Limit, burst int) *RateLimiter {
	limiter, err := NewRateLimiterWithOptions(Options{
		Rate:        r,
		Burst:       burst,
		MaxVisitors: defaultMaxVisitors,
		VisitorTTL:  defaultVisitorTTL,
	})
	if err != nil {
		panic(err)
	}
	return limiter
}

// NewRateLimiterWithOptions creates a limiter with explicit state limits.
func NewRateLimiterWithOptions(options Options) (*RateLimiter, error) {
	if options.Rate <= 0 || math.IsNaN(float64(options.Rate)) || math.IsInf(float64(options.Rate), 0) {
		return nil, ErrInvalidRate
	}
	if options.Burst < 1 {
		return nil, ErrInvalidBurst
	}
	if options.MaxVisitors < 1 {
		return nil, ErrInvalidMaxVisitors
	}
	if options.VisitorTTL <= 0 {
		return nil, ErrInvalidVisitorTTL
	}

	trustedProxies := make([]*net.IPNet, 0, len(options.TrustedProxyCIDRs))
	for _, cidr := range options.TrustedProxyCIDRs {
		_, network, err := net.ParseCIDR(strings.TrimSpace(cidr))
		if err != nil {
			return nil, ErrInvalidTrustedProxyCIDR
		}
		trustedProxies = append(trustedProxies, network)
	}

	cleanupInterval := options.VisitorTTL / 2
	if cleanupInterval <= 0 {
		cleanupInterval = options.VisitorTTL
	}
	cleanupInterval = min(cleanupInterval, time.Minute)

	return &RateLimiter{
		visitors:        make(map[string]*visitor),
		rate:            options.Rate,
		burst:           options.Burst,
		max:             options.MaxVisitors,
		ttl:             options.VisitorTTL,
		cleanupInterval: cleanupInterval,
		trustedProxies:  trustedProxies,
	}, nil
}

// AddVisitor returns the visitor's limiter, creating it only if one does not
// already exist. Existing token buckets are never reset by this method.
func (rl *RateLimiter) AddVisitor(ip string) *rate.Limiter {
	return rl.GetLimiter(ip)
}

// GetLimiter returns the visitor's limiter. When capacity is reached, expired
// entries are reclaimed first, then one existing entry is evicted if needed.
func (rl *RateLimiter) GetLimiter(ip string) *rate.Limiter {
	entry := rl.getVisitor(ip, true)
	if entry == nil {
		return nil
	}
	return entry.limiter
}

func (rl *RateLimiter) getVisitor(ip string, touch bool) *visitor {
	if rl == nil {
		return nil
	}
	key := normalizeIP(ip)
	if key == "" {
		return nil
	}

	now := time.Now()
	rl.mu.RLock()
	existing := rl.visitors[key]
	if existing != nil && now.Sub(time.Unix(0, existing.lastSeen.Load())) < rl.ttl {
		if touch {
			existing.lastSeen.Store(now.UnixNano())
		}
		rl.mu.RUnlock()
		return existing
	}
	rl.mu.RUnlock()

	rl.mu.Lock()
	defer rl.mu.Unlock()

	if existing := rl.visitors[key]; existing != nil {
		if now.Sub(time.Unix(0, existing.lastSeen.Load())) < rl.ttl {
			if touch {
				existing.lastSeen.Store(now.UnixNano())
			}
			return existing
		}
		delete(rl.visitors, key)
	}

	if len(rl.visitors) >= rl.max {
		if now.Sub(rl.lastCleanup) >= rl.cleanupInterval {
			rl.removeExpiredVisitors(now)
			rl.lastCleanup = now
		}
		if len(rl.visitors) >= rl.max {
			for key := range rl.visitors {
				delete(rl.visitors, key)
				break
			}
		}
	}

	entry := &visitor{limiter: rate.NewLimiter(rl.rate, rl.burst)}
	entry.lastSeen.Store(now.UnixNano())
	rl.visitors[key] = entry
	return entry
}

// Middleware limits requests by their network peer IP. X-Forwarded-For is
// considered only when the immediate peer belongs to a configured trusted
// proxy CIDR.
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if next == nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		entry := rl.getVisitor(rl.clientIP(r), false)
		if entry == nil || !entry.limiter.Allow() {
			var limiter *rate.Limiter
			if entry != nil {
				limiter = entry.limiter
			}
			w.Header().Set("Retry-After", retryAfter(limiter))
			_ = utils.TooManyRequests(w)
			return
		}

		entry.lastSeen.Store(time.Now().UnixNano())
		next.ServeHTTP(w, r)
	})
}

func (rl *RateLimiter) removeExpiredVisitors(now time.Time) {
	for key, existing := range rl.visitors {
		if now.Sub(time.Unix(0, existing.lastSeen.Load())) >= rl.ttl {
			delete(rl.visitors, key)
		}
	}
}

func retryAfter(limiter *rate.Limiter) string {
	if limiter == nil || limiter.Limit() == rate.Inf {
		return "1"
	}
	seconds := math.Ceil((1 - limiter.Tokens()) / float64(limiter.Limit()))
	if seconds < 1 {
		seconds = 1
	}
	return strconv.Itoa(int(seconds))
}

func normalizeIP(address string) string {
	address = strings.TrimSpace(address)
	if address == "" {
		return ""
	}

	host, _, err := net.SplitHostPort(address)
	if err != nil {
		host = strings.Trim(address, "[]")
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.String()
	}
	return ""
}

func (rl *RateLimiter) clientIP(r *http.Request) string {
	peerIP := normalizeIP(r.RemoteAddr)
	peer := net.ParseIP(peerIP)
	if peer == nil || !rl.isTrustedProxy(peer) {
		return peerIP
	}

	header := r.Header.Get("X-Forwarded-For")
	if header == "" {
		return peerIP
	}

	forwarded := strings.Split(header, ",")
	chain := make([]net.IP, 0, len(forwarded))
	for _, address := range forwarded {
		ip := net.ParseIP(strings.TrimSpace(address))
		if ip == nil {
			return peerIP
		}
		chain = append(chain, ip)
	}

	client := peer
	for i := len(chain) - 1; i >= 0; i-- {
		if !rl.isTrustedProxy(client) {
			return client.String()
		}
		client = chain[i]
	}
	return client.String()
}

func (rl *RateLimiter) isTrustedProxy(ip net.IP) bool {
	for _, proxy := range rl.trustedProxies {
		if proxy.Contains(ip) {
			return true
		}
	}
	return false
}
