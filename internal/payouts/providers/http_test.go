package providers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/ridex/ridex-angola/internal/payouts"
)

func httpExecutorFor(t *testing.T, server *httptest.Server) *HTTPExecutor {
	t.Helper()
	// The real endpoints (/v1/payouts) resolve against the httptest root.
	return NewHTTPExecutor(server.URL, "test-api-key-32-bytes-long!!", time.Second)
}

func TestHTTPExecutorSubmitPostsContract(t *testing.T) {
	var gotAuth string
	var gotPath string
	var gotPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
			t.Errorf("decode payload: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{
			"reference": "bank-abc123",
			"status":    "processing",
			"message":   "accepted",
		})
	}))
	defer server.Close()

	driverID := uuid.New()
	submission, err := httpExecutorFor(t, server).Submit("bank", 60000, driverID, "req-1")
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if gotAuth != "Bearer test-api-key-32-bytes-long!!" {
		t.Fatalf("authorization = %q", gotAuth)
	}
	if gotPath != "/v1/payouts" {
		t.Fatalf("path = %q, want /v1/payouts", gotPath)
	}
	if gotPayload["method"] != "bank" || gotPayload["amountCents"].(float64) != 60000 ||
		gotPayload["driverId"] != driverID.String() || gotPayload["reference"] != "req-1" {
		t.Fatalf("payload = %#v", gotPayload)
	}
	if submission.Reference != "bank-abc123" || submission.Status != payouts.StatusProcessing || submission.Message != "accepted" {
		t.Fatalf("submission = %#v", submission)
	}
}

func TestHTTPExecutorSubmitRejectsUnknownStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"reference": "bank-x", "status": "mystery"})
	}))
	defer server.Close()

	if _, err := httpExecutorFor(t, server).Submit("bank", 100, uuid.New(), "req-1"); err == nil ||
		!strings.Contains(err.Error(), "unknown provider payout status") {
		t.Fatalf("err = %v, want unknown provider payout status", err)
	}
}

func TestHTTPExecutorSubmitRequiresReference(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"status": "processing"})
	}))
	defer server.Close()

	if _, err := httpExecutorFor(t, server).Submit("bank", 100, uuid.New(), "req-1"); err == nil ||
		!strings.Contains(err.Error(), "no reference") {
		t.Fatalf("err = %v, want missing reference", err)
	}
}

func TestHTTPExecutorStatusPollsContract(t *testing.T) {
	var gotAuth string
	var gotPath string
	var gotMethod string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		gotMethod = r.Method
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"reference": "bank-abc123", "status": "completed"})
	}))
	defer server.Close()

	info, err := httpExecutorFor(t, server).Status("bank-abc123")
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if gotMethod != http.MethodGet || gotPath != "/v1/payouts/bank-abc123" {
		t.Fatalf("request = %s %s", gotMethod, gotPath)
	}
	if gotAuth != "Bearer test-api-key-32-bytes-long!!" {
		t.Fatalf("authorization = %q", gotAuth)
	}
	if info.Status != payouts.StatusCompleted {
		t.Fatalf("status = %q, want completed", info.Status)
	}
}

func TestHTTPExecutorStatusSurfacesNon2xx(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("provider down"))
	}))
	defer server.Close()

	_, err := httpExecutorFor(t, server).Status("bank-abc123")
	if err == nil || !strings.Contains(err.Error(), "provider payout API error 503") {
		t.Fatalf("err = %v, want 503 surfaced", err)
	}
}

func TestHTTPExecutorNotConfiguredWithoutCredentials(t *testing.T) {
	exec := NewHTTPExecutor("", "", 0)
	if exec.IsEnabled() {
		t.Fatal("executor should be disabled without credentials")
	}
	if _, err := exec.Submit("bank", 100, uuid.New(), "req-1"); !errors.Is(err, ErrPayoutExecutorNotConfigured) {
		t.Fatalf("submit err = %v, want ErrPayoutExecutorNotConfigured", err)
	}
	if _, err := exec.Status("ref"); !errors.Is(err, ErrPayoutExecutorNotConfigured) {
		t.Fatalf("status err = %v, want ErrPayoutExecutorNotConfigured", err)
	}
}

func TestCanonicalStatusIsCaseInsensitiveAndTrimmed(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{" processing ", payouts.StatusProcessing},
		{"Completed", payouts.StatusCompleted},
		{"FAILED", payouts.StatusFailed},
	} {
		got, err := canonicalStatus(tc.in)
		if err != nil || got != tc.want {
			t.Fatalf("canonicalStatus(%q) = %q, %v; want %q", tc.in, got, err, tc.want)
		}
	}
}
