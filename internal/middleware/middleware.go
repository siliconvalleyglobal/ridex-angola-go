package middleware

import (
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const RequestIDHeader = "X-Request-ID"

// RequestID adds a traceable ID to every request and response.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID, err := uuid.Parse(c.GetHeader(RequestIDHeader))
		if err != nil {
			requestID = uuid.New()
		}
		id := requestID.String()
		c.Set("requestID", id)
		c.Header(RequestIDHeader, id)
		c.Next()
	}
}

// SecurityHeaders applies headers appropriate for an API that does not serve HTML.
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("Referrer-Policy", "no-referrer")
		c.Header("Content-Security-Policy", "default-src 'none'")
		c.Header("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		c.Next()
	}
}

// CORS restricts browser callers to the explicitly configured origins. An
// empty allow-list intentionally denies cross-origin requests.
func CORS(allowedOrigins []string, allowCredentials bool) gin.HandlerFunc {
	origins := make(map[string]struct{}, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		origin = strings.TrimSpace(origin)
		if origin != "" {
			origins[origin] = struct{}{}
		}
	}
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" {
			if _, ok := origins[origin]; ok {
				c.Header("Access-Control-Allow-Origin", origin)
				c.Header("Vary", "Origin")
				c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
				c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type, Idempotency-Key, X-Request-ID")
				if allowCredentials {
					c.Header("Access-Control-Allow-Credentials", "true")
				}
			} else if c.Request.Method == http.MethodOptions {
				c.AbortWithStatus(http.StatusForbidden)
				return
			}
		}
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// RequestSizeLimit prevents handlers from reading unbounded request bodies.
func RequestSizeLimit(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if maxBytes > 0 {
			if c.Request.ContentLength > maxBytes {
				c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, gin.H{
					"error":     "request body too large",
					"code":      "request_too_large",
					"requestId": c.GetString("requestID"),
				})
				return
			}
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		}
		c.Next()
	}
}

// RecoveryJSON keeps panic responses machine-readable and avoids exposing
// implementation details in production responses.
func RecoveryJSON() gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, _ any) {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error":     "internal server error",
			"code":      "internal_error",
			"requestId": c.GetString("requestID"),
		})
	})
}

// ErrorHandler normalizes errors raised by middleware or handlers that use
// c.Error without already writing a response.
func ErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		if len(c.Errors) == 0 || c.Writer.Written() {
			return
		}
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error":     "internal server error",
			"code":      "internal_error",
			"requestId": c.GetString("requestID"),
		})
	}
}

type rateWindow struct {
	start time.Time
	count int
}

// RateLimiter is a small fixed-window limiter for a single API process.
// It is intentionally in-memory; deployments with multiple replicas should
// replace it with a shared store when that operational dependency is selected.
type RateLimiter struct {
	limit  int
	window time.Duration
	mu     sync.Mutex
	hits   map[string]rateWindow
	now    func() time.Time
}

func NewRateLimiter(limit int, window time.Duration) gin.HandlerFunc {
	limiter := &RateLimiter{
		limit:  limit,
		window: window,
		hits:   make(map[string]rateWindow),
		now:    time.Now,
	}
	return limiter.Handler()
}

func (l *RateLimiter) Handler() gin.HandlerFunc {
	return func(c *gin.Context) {
		if l.limit <= 0 || l.window <= 0 {
			c.Next()
			return
		}

		now := l.now()
		key := c.ClientIP()
		l.mu.Lock()
		if len(l.hits) > 10000 {
			for client, hit := range l.hits {
				if now.Sub(hit.start) >= l.window {
					delete(l.hits, client)
				}
			}
		}
		entry := l.hits[key]
		if entry.start.IsZero() || now.Sub(entry.start) >= l.window {
			entry = rateWindow{start: now, count: 0}
		}
		entry.count++
		l.hits[key] = entry
		allowed := entry.count <= l.limit
		remaining := l.limit - entry.count
		if remaining < 0 {
			remaining = 0
		}
		retryAfter := int((l.window - now.Sub(entry.start) + time.Second - 1) / time.Second)
		l.mu.Unlock()

		c.Header("X-RateLimit-Limit", strconv.Itoa(l.limit))
		c.Header("X-RateLimit-Remaining", strconv.Itoa(remaining))
		if !allowed {
			if retryAfter < 1 {
				retryAfter = 1
			}
			c.Header("Retry-After", strconv.Itoa(retryAfter))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded"})
			return
		}
		c.Next()
	}
}
