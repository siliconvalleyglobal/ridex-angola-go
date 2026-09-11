package health

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ridex/ridex-angola/config"
)

// Handler implements health check endpoints for observability.
type Handler struct {
	cfg  *config.Config
	pool *pgxpool.Pool
}

// NewHandler creates a new health check handler.
func NewHandler(cfg *config.Config, pool *pgxpool.Pool) *Handler {
	return &Handler{cfg: cfg, pool: pool}
}

// Liveness returns 200 OK if the service is running.
// This endpoint is suitable for Kubernetes liveness probes.
func (h *Handler) Liveness(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "ok",
		"service":   "ridex-angola",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

// Readiness returns 200 OK if the service is ready to accept traffic.
// This endpoint checks database connectivity and is suitable for Kubernetes readiness probes.
func (h *Handler) Readiness(c *gin.Context) {
	// If no pool is configured, return not ready
	if h.pool == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status":  "not_ready",
			"service": "ridex-angola",
			"checks": gin.H{
				"postgres": "not_configured",
			},
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	// Check PostgreSQL connectivity
	if err := h.pool.Ping(ctx); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status":  "not_ready",
			"service": "ridex-angola",
			"checks": gin.H{
				"postgres": "unavailable",
			},
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "ready",
		"service": "ridex-angola",
		"checks": gin.H{
			"postgres": "ok",
		},
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

// Health returns detailed health status including all dependencies.
// This endpoint is suitable for load balancer health checks.
func (h *Handler) Health(c *gin.Context) {
	checks := make(map[string]string)
	healthy := true

	// If no pool, report as unhealthy
	if h.pool == nil {
		checks["postgres"] = "not_configured"
		healthy = false
	} else {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
		defer cancel()

		// Check PostgreSQL
		if err := h.pool.Ping(ctx); err != nil {
			checks["postgres"] = "unavailable: " + err.Error()
			healthy = false
		} else {
			checks["postgres"] = "ok"
		}
	}

	status := http.StatusOK
	healthStatus := "healthy"
	if !healthy {
		status = http.StatusServiceUnavailable
		healthStatus = "unhealthy"
	}

	c.JSON(status, gin.H{
		"status":    healthStatus,
		"service":   "ridex-angola",
		"version":   "1.0.0",
		"checks":    checks,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}
