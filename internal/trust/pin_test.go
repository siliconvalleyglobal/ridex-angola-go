package trust

import (
	"strconv"
	"testing"
)

// TestPin_UniformAndBounded verifies the PIN generator produces valid
// 4-digit PINs across many draws. The historical bug was `1000 + b[0]%9000`
// with a single random byte, which could only ever produce 1000..1255.
func TestPin_UniformAndBounded(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		pin, err := pin()
		if err != nil {
			t.Fatalf("pin() error = %v", err)
		}
		n, err := strconv.Atoi(pin)
		if err != nil {
			t.Fatalf("pin() = %q is not numeric", pin)
		}
		if len(pin) != 4 || n < 1000 || n > 9999 {
			t.Fatalf("pin() = %q outside [1000,9999]", pin)
		}
		seen[pin] = true
	}
	// With 9000 possible PINs and 1000 draws, the probability of zero
	// collisions is astronomically small; require broad spread instead
	// of strict uniformity.
	if len(seen) < 500 {
		t.Errorf("pin() produced only %d distinct values in 1000 draws; distribution looks biased", len(seen))
	}
}
