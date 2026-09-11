package pricing

import (
	"math/big"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
)

func TestNumericCents(t *testing.T) {
	zoneRate := pgtype.Numeric{Int: big.NewInt(1250), Exp: 0, Valid: true}
	if got := numericCents(zoneRate, 500); got != 1250 {
		t.Fatalf("numericCents() = %d, want 1250", got)
	}

	if got := numericCents(pgtype.Numeric{}, 500); got != 500 {
		t.Fatalf("numericCents() fallback = %d, want 500", got)
	}
}
