package rides

import (
	"testing"
)

func TestExceedsLimit(t *testing.T) {
	used := numeric(900)
	limit := numeric(1000)
	if exceedsLimit(used, 100, limit) {
		t.Fatal("request at the limit should be allowed")
	}
	if exceedsLimit(used, 99, limit) {
		t.Fatal("request below the limit should be allowed")
	}
	if !exceedsLimit(used, 100, numeric(0)) {
		t.Fatal("zero spending limit must be enforced")
	}
}
