package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log"
	"net"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"civika/backend/config"
)

type contextKey string

const requestIDKey contextKey = "request_id"

var requestIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{1,119}$`)

func bodyLimitMiddleware(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			next.ServeHTTP(w, r)
		})
	}
}

func requestIDMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestID := strings.TrimSpace(r.Header.Get("X-Request-Id"))
			if !isValidRequestID(requestID) {
				requestID = newRequestID()
			}
			w.Header().Set("X-Request-Id", requestID)
			ctx := context.WithValue(r.Context(), requestIDKey, requestID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func isValidRequestID(value string) bool {
	return requestIDPattern.MatchString(value)
}

func recoverMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if recovered := recover(); recovered != nil {
					log.Printf("panic recovered path=%s method=%s", r.URL.Path, r.Method)
					writeAPIError(w, r, http.StatusInternalServerError, "internal_error", "erreur interne")
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

type responseRecorder struct {
	http.ResponseWriter
	status int
}

func (r *responseRecorder) WriteHeader(statusCode int) {
	r.status = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
}

func accessLogMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			recorder := &responseRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(recorder, r)
			requestID, _ := r.Context().Value(requestIDKey).(string)
			log.Printf(
				"http_request request_id=%s method=%s path=%s status=%d duration_ms=%d",
				requestID,
				r.Method,
				r.URL.Path,
				recorder.status,
				time.Since(start).Milliseconds(),
			)
		})
	}
}

type qaVisitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

type qaRateLimiter struct {
	mu              sync.Mutex
	visitors        map[string]*qaVisitor
	qps             rate.Limit
	burst           int
	cleanupInterval time.Duration
	lastCleanup     time.Time
}

func newQARateLimiter(qps float64, burst int, cleanupInterval time.Duration) *qaRateLimiter {
	interval := cleanupInterval
	if interval <= 0 {
		interval = time.Minute
	}
	return &qaRateLimiter{
		visitors:        make(map[string]*qaVisitor),
		qps:             rate.Limit(qps),
		burst:           burst,
		cleanupInterval: interval,
		lastCleanup:     time.Now(),
	}
}

func (l *qaRateLimiter) allow(key string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	if now.Sub(l.lastCleanup) >= l.cleanupInterval {
		staleAfter := l.cleanupInterval * 2
		for visitorKey, visitor := range l.visitors {
			if now.Sub(visitor.lastSeen) > staleAfter {
				delete(l.visitors, visitorKey)
			}
		}
		l.lastCleanup = now
	}

	visitor, exists := l.visitors[key]
	if !exists {
		visitor = &qaVisitor{
			limiter: rate.NewLimiter(l.qps, l.burst),
		}
		l.visitors[key] = visitor
	}
	visitor.lastSeen = now
	return visitor.limiter.AllowN(now, 1)
}

func qaRateLimitMiddleware(cfg config.Config) func(http.Handler) http.Handler {
	if cfg.QARateLimit.QPS <= 0 || cfg.QARateLimit.Burst <= 0 {
		return func(next http.Handler) http.Handler {
			return next
		}
	}
	limiter := newQARateLimiter(cfg.QARateLimit.QPS, cfg.QARateLimit.Burst, cfg.QARateLimit.CleanupInterval)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			clientKey := qaClientKey(r)
			if !limiter.allow(clientKey, time.Now()) {
				w.Header().Set("Retry-After", "1")
				writeAPIError(w, r, http.StatusTooManyRequests, "too_many_requests", "trop de requetes, veuillez reessayer plus tard")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func qaClientKey(r *http.Request) string {
	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err == nil && host != "" {
		return host
	}
	if strings.TrimSpace(r.RemoteAddr) != "" {
		return strings.TrimSpace(r.RemoteAddr)
	}
	return "unknown"
}

func newRequestID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "req-fallback"
	}
	return hex.EncodeToString(buf)
}
