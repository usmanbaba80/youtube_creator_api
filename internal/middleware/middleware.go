package middleware

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"youtube-creator-api/internal/response"
	"youtube-creator-api/internal/security"
)

func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
		next.ServeHTTP(w, r)
	})
}

func CORS(allowedOrigins []string) func(http.Handler) http.Handler {
	allowed := map[string]struct{}{}
	for _, o := range allowedOrigins {
		allowed[strings.TrimSpace(o)] = struct{}{}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" {
				if _, ok := allowed[origin]; ok {
					w.Header().Set("Access-Control-Allow-Origin", origin)
					w.Header().Set("Vary", "Origin")
					w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-API-Key, X-Request-ID")
					w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
					w.Header().Set("Access-Control-Max-Age", "600")
				}
			}
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func APIKey(keys []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := r.Header.Get("X-API-Key")
			if key == "" {
				auth := r.Header.Get("Authorization")
				if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
					key = strings.TrimSpace(auth[7:])
				}
			}
			if !security.ValidAPIKey(key, keys) {
				response.Fail(w, http.StatusUnauthorized, "unauthorized", "invalid or missing API key")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

type rateBucket struct {
	count   int
	resetAt time.Time
}

type RateLimiter struct {
	mu       sync.Mutex
	rpm      int
	buckets  map[string]*rateBucket
	lastSweep time.Time
}

func NewRateLimiter(rpm int) *RateLimiter {
	if rpm < 1 {
		rpm = 60
	}
	return &RateLimiter{
		rpm:       rpm,
		buckets:   make(map[string]*rateBucket),
		lastSweep: time.Now(),
	}
}

func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := clientKey(r)
		now := time.Now()

		rl.mu.Lock()
		if now.Sub(rl.lastSweep) > time.Minute {
			for k, b := range rl.buckets {
				if now.After(b.resetAt) {
					delete(rl.buckets, k)
				}
			}
			rl.lastSweep = now
		}
		b, ok := rl.buckets[key]
		if !ok || now.After(b.resetAt) {
			b = &rateBucket{count: 0, resetAt: now.Add(time.Minute)}
			rl.buckets[key] = b
		}
		b.count++
		over := b.count > rl.rpm
		rl.mu.Unlock()

		if over {
			w.Header().Set("Retry-After", "60")
			response.Fail(w, http.StatusTooManyRequests, "rate_limited", "too many requests")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func clientKey(r *http.Request) string {
	apiKey := r.Header.Get("X-API-Key")
	ip := r.Header.Get("X-Forwarded-For")
	if ip != "" {
		ip = strings.TrimSpace(strings.Split(ip, ",")[0])
	} else {
		ip = r.RemoteAddr
	}
	return ip + "|" + apiKey
}
