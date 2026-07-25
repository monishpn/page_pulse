package middleware

import (
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"gofr.dev/pkg/gofr"
	gofrHTTP "gofr.dev/pkg/gofr/http"

	"github.com/monishpn/page_pulse/models"
)

// Although we have defaults set in main, this is to make sure it strconv doesn't break.
const (
	defaultMaxRequests   = 10
	defaultWindowSeconds = 60
)

type RateLimiter struct {
	ctx               *gofr.Context
	maxRequests       int
	window            time.Duration
	trustProxyHeaders bool
	paths             map[string]bool
}

// NewRateLimit builds a RateLimiter scoped to the given paths.
func NewRateLimit(maxRequests, windowSeconds string, trustProxyHeaders bool, paths ...string) *RateLimiter {
	maxReq, err := strconv.Atoi(maxRequests)
	if err != nil || maxReq <= 0 {
		maxReq = defaultMaxRequests
	}

	windowSecondsInt, err := strconv.Atoi(windowSeconds)
	if err != nil || windowSecondsInt <= 0 {
		windowSecondsInt = defaultWindowSeconds
	}

	pathSet := make(map[string]bool, len(paths))
	for _, p := range paths {
		pathSet[p] = true
	}

	return &RateLimiter{
		maxRequests:       maxReq,
		window:            time.Duration(windowSecondsInt) * time.Second,
		trustProxyHeaders: trustProxyHeaders,
		paths:             pathSet,
	}
}

// SetContext injects the DI-managed context once app.OnStart has run.
func (rl *RateLimiter) SetContext(ctx *gofr.Context) {
	rl.ctx = ctx
}

func (rl *RateLimiter) Middleware(inner http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !rl.paths[r.URL.Path] {
			inner.ServeHTTP(w, r)
			return
		}

		if rl.ctx == nil || rl.ctx.Redis == nil {
			// OnStart hasn't populated the context yet; fail open.
			inner.ServeHTTP(w, r)
			return
		}

		windowSeconds := int64(rl.window.Seconds())
		now := time.Now().Unix()
		windowStart := now - now%windowSeconds

		key := fmt.Sprintf("ratelimit:%s:%d", clientIP(r, rl.trustProxyHeaders), windowStart)

		count, err := rl.ctx.Redis.Incr(r.Context(), key).Result()
		if err != nil {
			// Redis is unreachable; fail open rather than blocking audits.
			inner.ServeHTTP(w, r)
			return
		}

		if count == 1 {
			rl.ctx.Redis.Expire(r.Context(), key, rl.window)
		}

		if count > int64(rl.maxRequests) {
			retryAfter := windowStart + windowSeconds - now
			w.Header().Set("Retry-After", strconv.FormatInt(retryAfter, 10))

			responder := gofrHTTP.NewResponder(w, r.Method)
			responder.Respond(nil, &models.CustomError{
				Message: "rate limit exceeded, please retry after 1 minute",
				Code:    http.StatusTooManyRequests,
			})

			return
		}

		inner.ServeHTTP(w, r)
	})
}

// clientIP returns the request's client IP. X-Forwarded-For/X-Real-IP are
// only trusted when explicitly enabled, since otherwise a client can spoof
// them to dodge its own limit.
func clientIP(r *http.Request, trustProxyHeaders bool) string {
	if trustProxyHeaders {
		if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
			if ip := strings.TrimSpace(strings.Split(fwd, ",")[0]); ip != "" {
				return ip
			}
		}

		if ip := strings.TrimSpace(r.Header.Get("X-Real-IP")); ip != "" {
			return ip
		}
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}

	return host
}
