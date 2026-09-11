package health

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/ridex/ridex-angola/config"
)

func TestHandler_Liveness(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{}
	handler := NewHandler(cfg, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/health", nil)
	handler.Liveness(c)

	if w.Code != http.StatusOK {
		t.Errorf("Liveness() status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestHandler_Readiness_NoDB(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{}
	handler := NewHandler(cfg, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/health", nil)
	handler.Readiness(c)

	// Without a pool, should return service unavailable
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("Readiness() status = %d, want %d", w.Code, http.StatusServiceUnavailable)
	}
}

func TestHandler_Health_NoDB(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{}
	handler := NewHandler(cfg, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/health", nil)
	handler.Health(c)

	// Without a pool, should return service unavailable
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("Health() status = %d, want %d", w.Code, http.StatusServiceUnavailable)
	}
}

func TestHandler_ContextTimeout(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{}
	handler := NewHandler(cfg, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	// Create a request with an expired context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately
	c.Request = httptest.NewRequest(http.MethodGet, "/health", nil).WithContext(ctx)
	handler.Readiness(c)

	// Should handle expired context gracefully
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("Readiness() with expired context status = %d, want %d", w.Code, http.StatusServiceUnavailable)
	}
}
