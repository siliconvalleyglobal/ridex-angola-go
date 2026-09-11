package payouts

import (
	"fmt"
	"strings"
)

// PayoutMethodInfo carries the per-method metadata surfaced to callers (and
// eventually the frontend).
// PayoutMethod used to be defined here; it now lives in payout.go so the
// simulated PayoutService and the wallet-backed ledger share one registry.
type PayoutMethodInfo struct {
	Label         string
	Currency      string
	MinCents      int64
	MaxCents      int64
	ReferenceHint string
}

// PayoutMethods is the configured payout-method registry. Construction is
// console-like: parse the comma-separated enable-list, optionally override
// per-method bounds, and hard-fail when configuration names a provider that
// has no metadata at all.
type PayoutMethods struct {
	enabled    map[PayoutMethod]bool
	meta       map[PayoutMethod]PayoutMethodInfo
	defaultMin int64
	defaultMax int64
}

// NewPayoutMethods builds a payout-method registry from a comma-separated
// enable-list. Unknown tokens are ignored; calling WithMinCents / WithMaxCents
// / WithInfo after construction overrides the built-in defaults.
func NewPayoutMethods(enabled string) *PayoutMethods {
	m := &PayoutMethods{
		enabled:    make(map[PayoutMethod]bool),
		meta:       map[PayoutMethod]PayoutMethodInfo{},
		defaultMin: 5000,
		defaultMax: 500000,
	}
	for _, name := range splitAllowList(enabled) {
		name = strings.TrimSpace(name)
		low := strings.ToLower(name)
		var key PayoutMethod
		switch low {
		case string(PayoutMethodBank), string(PayoutMethodBankTransfer):
			key = PayoutMethodBank
		case string(PayoutMethodMobileMoney):
			key = PayoutMethodMobileMoney
		case string(PayoutMethodMulticada), string(PayoutMethodMulticaixa):
			key = PayoutMethodMulticada
		default:
			continue // ignore unknown tokens rather than hard-fail here
		}
		m.enabled[key] = true
	}
	return m
}

func splitAllowList(list string) []string {
	fields := strings.FieldsFunc(list, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t' || r == '\n'
	})
	var out []string
	for _, field := range fields {
		if trimmed := strings.TrimSpace(field); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

// DefaultPayoutMethods returns the registry used when PAYOUT_METHODS is unset:
// all built-in methods enabled with their default bounds.
func DefaultPayoutMethods() *PayoutMethods {
	m := NewPayoutMethods("bank,mobile_money,multicada")
	for key, info := range defaultMethodMeta() {
		m.meta[key] = info
	}
	return m
}

// WithMinCents sets the lower bound applied to every enabled method unless an
// explicit per-method value is configured through WithInfo.
func (m *PayoutMethods) WithMinCents(min int64) *PayoutMethods { m.defaultMin = min; return m }

// WithMaxCents sets the upper bound applied to every enabled method unless an
// explicit per-method value is configured through WithInfo.
func (m *PayoutMethods) WithMaxCents(max int64) *PayoutMethods { m.defaultMax = max; return m }

// WithInfo overrides the metadata for a single method. Passing the empty string
// key does nothing.
func (m *PayoutMethods) WithInfo(key PayoutMethod, info PayoutMethodInfo) *PayoutMethods {
	if key != "" {
		m.meta[key] = info
	}
	return m
}

// Enabled reports whether the (already-normalized) method is permitted.
func (m *PayoutMethods) Enabled(key PayoutMethod) bool {
	if m == nil {
		return false
	}
	mKey := key
	if mKey == PayoutMethodBankTransfer {
		mKey = PayoutMethodBank
	}
	if mKey == PayoutMethodMulticaixa {
		mKey = PayoutMethodMulticada
	}
	return m.enabled[mKey]
}

// Info returns metadata for the method. If no custom metadata was supplied the
// built-in defaults are returned (label + AOA currency for local payouts).
// PayoutMethodInfo is only defined for enabled methods; a disabled method has
// no metadata entry, which keeps zero-value bounds from silently bypassing
// the minimum/maximum checks on an allow-listed method.
func (m *PayoutMethods) Info(key PayoutMethod) (PayoutMethodInfo, bool) {
	if m == nil {
		return PayoutMethodInfo{}, false
	}
	lookup := key
	if lookup == PayoutMethodBankTransfer {
		lookup = PayoutMethodBank
	}
	if lookup == PayoutMethodMulticaixa {
		lookup = PayoutMethodMulticada
	}
	info, ok := m.meta[lookup]
	return info, ok
}

// NormalizePayoutMethod trims and lowercases the raw method and returns the
// canonical PayoutMethod only when it is enabled in m. Callers should use this
// at the ledger boundary so invalid values never reach money-movement code.
// The legacy 'bank_transfer' spelling normalizes to 'bank' so older clients
// keep working.
func (m *PayoutMethods) NormalizePayoutMethod(raw string) (PayoutMethod, error) {
	if strings.TrimSpace(raw) == "" {
		return "", ErrInvalidInput
	}
	key := PayoutMethod(strings.ToLower(strings.TrimSpace(raw)))
	if key == PayoutMethodBankTransfer {
		key = PayoutMethodBank
	}
	if key == PayoutMethodMulticaixa {
		key = PayoutMethodMulticada
	}
	if !m.Enabled(key) {
		return "", fmt.Errorf("%w: unsupported payout method", ErrInvalidInput)
	}
	return key, nil
}

func defaultMethodMeta() map[PayoutMethod]PayoutMethodInfo {
	return map[PayoutMethod]PayoutMethodInfo{
		PayoutMethodBank: {
			Label: "Bank transfer", Currency: "AOA",
			MinCents: 15000, MaxCents: 500000,
			ReferenceHint: "IBAN or BI tax number",
		},
		PayoutMethodMobileMoney: {
			Label: "Mobile money", Currency: "AOA",
			MinCents: 5000, MaxCents: 500000,
			ReferenceHint: "Registered mobile money number",
		},
		PayoutMethodMulticada: {
			Label: "Multicaixa Express", Currency: "AOA",
			MinCents: 5000, MaxCents: 500000,
			ReferenceHint: "Multicaixa customer reference",
		},
	}
}
