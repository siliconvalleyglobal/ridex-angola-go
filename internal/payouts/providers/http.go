// Package providers implements payout executors backed by real money-movement
// transports, mirroring the payment provider layout. The HTTPExecutor speaks
// a small documented REST contract so any banking/Multicaixa provider can be
// wired by URL and API key without touching the payout ledger.
package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/ridex/ridex-angola/internal/payouts"
)

// SubmitEndpoint and StatusEndpoint form the payout REST contract a banking
// provider must implement:
//
//	POST /v1/payouts        {method, amountCents, driverId, reference}
//	                        -> 2xx {reference, status, message}
//	GET  /v1/payouts/{ref}  -> 2xx {reference, status, message}
//
// status is one of processing | completed | failed. Unknown statuses are
// rejected, never mapped — mirroring the ledger's no-guessing rule.
const (
	SubmitEndpoint = "/v1/payouts"
	StatusEndpoint = "/v1/payouts/"

	// DefaultTimeout caps every provider call so a slow provider cannot hang
	// the worker's payout job.
	DefaultTimeout = 30 * time.Second

	// maxPayoutResponseBody caps provider response bodies so a hostile
	// provider cannot exhaust worker memory.
	maxPayoutResponseBody = 1 << 20 // 1 MiB
)

// ErrPayoutExecutorNotConfigured means the HTTP executor has no credentials.
var ErrPayoutExecutorNotConfigured = fmt.Errorf("payout executor is not configured")

// HTTPExecutor implements payouts.Executor over a banking provider's REST API.
type HTTPExecutor struct {
	baseURL string
	apiKey  string
	client  *http.Client

	// submitEndpoint and statusEndpoint are variable so tests can point the
	// executor at an httptest server.
	submitEndpoint string
	statusEndpoint string
}

// NewHTTPExecutor creates an HTTP-backed payout executor. baseURL is the
// provider's API root (e.g. https://api.bank.ao) and apiKey is transmitted
// as a bearer token on every call.
func NewHTTPExecutor(baseURL, apiKey string, timeout time.Duration) *HTTPExecutor {
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	return &HTTPExecutor{
		baseURL:        strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		apiKey:         strings.TrimSpace(apiKey),
		client:         &http.Client{Timeout: timeout},
		submitEndpoint: SubmitEndpoint,
		statusEndpoint: StatusEndpoint,
	}
}

// IsEnabled reports whether the executor has the credentials to transmit.
func (e *HTTPExecutor) IsEnabled() bool {
	return e.baseURL != "" && e.apiKey != ""
}

// payoutRequest is the Submit payload.
type payoutRequest struct {
	Method      string `json:"method"`
	AmountCents int64  `json:"amountCents"`
	DriverID    string `json:"driverId"`
	Reference   string `json:"reference"`
}

// providerPayout is the documented provider response for both Submit and
// Status. Only the three canonical statuses are accepted.
type providerPayout struct {
	Reference string `json:"reference"`
	Status    string `json:"status"`
	Message   string `json:"message"`
}

// canonicalStatus maps a provider status to the executor statuses. Unknown
// states return an error: the caller polls again instead of settling on a
// guessed status.
func canonicalStatus(status string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case payouts.StatusProcessing:
		return payouts.StatusProcessing, nil
	case payouts.StatusCompleted:
		return payouts.StatusCompleted, nil
	case payouts.StatusFailed:
		return payouts.StatusFailed, nil
	default:
		return "", fmt.Errorf("unknown provider payout status %q", status)
	}
}

// uncertain wraps err as an uncertain submission outcome: the provider may
// have received and processed the payout, so the ledger must hold it in
// flight rather than reverse.
func uncertain(err error) error {
	return fmt.Errorf("%w: %v", payouts.ErrSubmitUncertain, err)
}

// Submit initiates an external payout through the provider's REST API.
// The boundary carries no context, mirroring payouts.Executor: the client
// timeout caps the call instead.
func (e *HTTPExecutor) Submit(method string, amountCents int64, driverID uuid.UUID, reference string) (payouts.Submission, error) {
	ctx := context.Background()
	if !e.IsEnabled() {
		return payouts.Submission{}, fmt.Errorf("%w: HTTP payout executor needs a base URL and API key", ErrPayoutExecutorNotConfigured)
	}

	payload, err := json.Marshal(payoutRequest{
		Method:      method,
		AmountCents: amountCents,
		DriverID:    driverID.String(),
		Reference:   reference,
	})
	if err != nil {
		return payouts.Submission{}, fmt.Errorf("marshal payout request: %w", err)
	}

	body, err := e.call(ctx, http.MethodPost, e.baseURL+e.submitEndpoint, payload)
	if err != nil {
		return payouts.Submission{}, err
	}

	var provider providerPayout
	if err := json.Unmarshal(body, &provider); err != nil {
		return payouts.Submission{}, uncertain(fmt.Errorf("decode provider payout response: %w", err))
	}
	if strings.TrimSpace(provider.Reference) == "" {
		return payouts.Submission{}, uncertain(fmt.Errorf("provider payout response has no reference: %s", string(body)))
	}
	status, err := canonicalStatus(provider.Status)
	if err != nil {
		return payouts.Submission{}, uncertain(err)
	}
	return payouts.Submission{Reference: provider.Reference, Status: status, Message: provider.Message}, nil
}

// Status polls the provider for the current state of a submitted payout.
// The boundary carries no context, mirroring payouts.Executor: the client
// timeout caps the call instead.
func (e *HTTPExecutor) Status(reference string) (payouts.StatusInfo, error) {
	ctx := context.Background()
	if !e.IsEnabled() {
		return payouts.StatusInfo{}, fmt.Errorf("%w: HTTP payout executor needs a base URL and API key", ErrPayoutExecutorNotConfigured)
	}
	if strings.TrimSpace(reference) == "" {
		return payouts.StatusInfo{}, fmt.Errorf("payout reference is required")
	}

	body, err := e.call(ctx, http.MethodGet, e.baseURL+e.statusEndpoint+url.PathEscape(strings.TrimSpace(reference)), nil)
	if err != nil {
		return payouts.StatusInfo{}, err
	}

	var provider providerPayout
	if err := json.Unmarshal(body, &provider); err != nil {
		return payouts.StatusInfo{}, fmt.Errorf("decode provider payout status: %w", err)
	}
	status, err := canonicalStatus(provider.Status)
	if err != nil {
		return payouts.StatusInfo{}, err
	}
	return payouts.StatusInfo{Reference: reference, Status: status, Message: provider.Message}, nil
}

// call transmits one authenticated request and returns the 2xx body. Non-2xx
// responses surface as errors with the provider body for debugging.
func (e *HTTPExecutor) call(ctx context.Context, method, endpoint string, payload []byte) ([]byte, error) {
	var reader io.Reader
	if payload != nil {
		reader = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return nil, fmt.Errorf("create provider payout request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+e.apiKey)

	resp, err := e.client.Do(req)
	if err != nil {
		// Transport failures (timeout, connection reset) cannot prove the
		// provider never processed the payout.
		return nil, uncertain(fmt.Errorf("execute provider payout request: %w", err))
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxPayoutResponseBody))
	if err != nil {
		return nil, uncertain(fmt.Errorf("read provider payout response: %w", err))
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		apiErr := fmt.Errorf("provider payout API error %d: %s", resp.StatusCode, string(body))
		if resp.StatusCode >= 500 {
			// A 5xx may have followed a processed submission.
			return nil, uncertain(apiErr)
		}
		return nil, apiErr
	}
	return body, nil
}
