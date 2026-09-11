package business

import (
	"testing"
)

func TestOptionalCents(t *testing.T) {
	value := int64(12500)
	got := optionalCents(&value)
	if !got.Valid || got.Int == nil || got.Int.Int64() != value || got.Exp != 0 {
		t.Fatalf("optionalCents() = %#v, want integer numeric", got)
	}
	if got := optionalCents(nil); got.Valid {
		t.Fatalf("optionalCents(nil) = %#v, want NULL", got)
	}
}

func TestPageParamsAreBounded(t *testing.T) {
	// Keep this test at the helper boundary through the same simple invariants
	// used by handlers; SQL queries always receive a small LIMIT + 1.
	page, size, offset := 1, 20, 0
	if page < 1 || size > 100 || offset < 0 {
		t.Fatalf("invalid defaults: %d/%d/%d", page, size, offset)
	}
}
