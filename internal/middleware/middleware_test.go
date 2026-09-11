package middleware

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func TestRequestIDPreservesValidIDAndGeneratesInvalidID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequestID())
	r.GET("/", func(c *gin.Context) {
		if c.GetString("requestID") == "" {
			t.Errorf("request ID was not stored in context")
		}
		c.Status(204)
	})

	valid := uuid.New().String()
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set(RequestIDHeader, valid)
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)
	if got := res.Header().Get(RequestIDHeader); got != valid {
		t.Fatalf("request ID = %q, want %q", got, valid)
	}

	req = httptest.NewRequest("GET", "/", nil)
	req.Header.Set(RequestIDHeader, "not-a-uuid")
	res = httptest.NewRecorder()
	r.ServeHTTP(res, req)
	if got := res.Header().Get(RequestIDHeader); got == "" {
		t.Fatal("generated request ID is empty")
	}
}

func TestRateLimiterRejectsAfterLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(NewRateLimiter(1, time.Minute))
	r.GET("/", func(c *gin.Context) { c.Status(204) })

	res := httptest.NewRecorder()
	r.ServeHTTP(res, httptest.NewRequest("GET", "/", nil))
	if res.Code != 204 {
		t.Fatalf("first request status = %d, want 204", res.Code)
	}

	res = httptest.NewRecorder()
	r.ServeHTTP(res, httptest.NewRequest("GET", "/", nil))
	if res.Code != 429 {
		t.Fatalf("second request status = %d, want 429", res.Code)
	}
	if res.Header().Get("Retry-After") == "" {
		t.Fatal("missing Retry-After header")
	}
}

func TestCORSAllowsConfiguredOriginAndHandlesPreflight(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(CORS([]string{"https://app.example"}, true))
	r.POST("/", func(c *gin.Context) { c.Status(204) })

	req := httptest.NewRequest("OPTIONS", "/", nil)
	req.Header.Set("Origin", "https://app.example")
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)
	if res.Code != 204 || res.Header().Get("Access-Control-Allow-Origin") != "https://app.example" {
		t.Fatalf("preflight response = %d, allow-origin %q", res.Code, res.Header().Get("Access-Control-Allow-Origin"))
	}

	req = httptest.NewRequest("OPTIONS", "/", nil)
	req.Header.Set("Origin", "https://other.example")
	res = httptest.NewRecorder()
	r.ServeHTTP(res, req)
	if res.Code != 403 {
		t.Fatalf("unconfigured preflight status = %d, want 403", res.Code)
	}
}

func TestRecoveryJSONDoesNotExposePanic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequestID(), RecoveryJSON())
	r.GET("/", func(c *gin.Context) { panic("secret implementation detail") })
	res := httptest.NewRecorder()
	r.ServeHTTP(res, httptest.NewRequest("GET", "/", nil))
	if res.Code != 500 {
		t.Fatalf("status = %d, want 500", res.Code)
	}
	var body map[string]string
	if err := json.NewDecoder(bytes.NewReader(res.Body.Bytes())).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["error"] != "internal server error" || body["code"] != "internal_error" {
		t.Fatalf("unexpected error body: %#v", body)
	}
}

func TestRequestSizeLimitRejectsKnownOversizeBodies(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequestID(), RequestSizeLimit(4))
	r.POST("/", func(c *gin.Context) { c.Status(204) })
	req := httptest.NewRequest("POST", "/", strings.NewReader("12345"))
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)
	if res.Code != 413 {
		t.Fatalf("status = %d, want 413", res.Code)
	}
}
