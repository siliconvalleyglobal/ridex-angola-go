package dashboard

import (
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/ridex/ridex-angola/internal/db"
)

// serveJSON marshals any value through a real gin router so tests can assert on
// the exact JSON bytes the handler helpers produce.
func serveJSON(value any) string {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/test", func(c *gin.Context) { c.JSON(http.StatusOK, value) })
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		return "server error body=" + res.Body.String()
	}
	return res.Body.String()
}

func TestRidesJSONShapesRideRows(t *testing.T) {
	ride := db.Ride{
		ID:                 uuid.New(),
		RiderID:            uuid.New(),
		DriverID:           pgtype.UUID{Bytes: uuid.New(), Valid: true},
		Status:             "completed",
		PickupAddress:      "Aeroporto Internacional de Luanda",
		DestinationAddress: "Hotel Vitoria, Ex-Cinema Vitori",
		SuggestedFareCents: pgtype.Numeric{Int: big.NewInt(25000), Exp: 0, Valid: true},
		AcceptedFareCents:  pgtype.Numeric{Int: big.NewInt(24000), Exp: 0, Valid: true},
		Currency:           "AOA",
		PaymentMethod:      "card",
	}
	out := ridesJSON([]db.Ride{ride})
	if len(out) != 1 {
		t.Fatalf("ridesJSON() length = %d, want 1", len(out))
	}
	text := serveJSON(out[0])
	checks := []string{
		`"id"`, `"status"`, `"pickup"`, `"destination"`,
		`"suggestedFareCents"`, `"acceptedFareCents"`, `"paymentMethod"`,
		`"cancelledAt"`, `"cancellationReason"`,
		`"Aeroporto Internacional de Luanda"`, `"card"`,
	}
	for _, want := range checks {
		if !strings.Contains(text, want) {
			t.Fatalf("ridesJSON() output = %s, want it to contain %q", text, want)
		}
	}
}

func TestEarningsJSONShapesSummary(t *testing.T) {
	out := earningsJSON(db.GetDriverEarningsSummaryRow{
		CompletedRides:  12,
		GrossFares:      pgtype.Numeric{Int: big.NewInt(120000), Exp: 0, Valid: true},
		CashEarnings:    pgtype.Numeric{Int: big.NewInt(30000), Exp: 0, Valid: true},
		DigitalEarnings: pgtype.Numeric{Int: big.NewInt(90000), Exp: 0, Valid: true},
		TotalEarnings:   pgtype.Numeric{Int: big.NewInt(120000), Exp: 0, Valid: true},
		UnpaidFares:     pgtype.Numeric{Int: big.NewInt(5000), Exp: 0, Valid: true},
	})
	text := serveJSON(out)
	if !strings.Contains(text, `"completedRides":12`) {
		t.Fatalf("earningsJSON() output = %s, want completedRides 12", text)
	}
	if !strings.Contains(text, `"currency":"AOA"`) {
		t.Fatalf("earningsJSON() output = %s, want AOA currency", text)
	}
}

func TestPaymentJSONListShapesCharges(t *testing.T) {
	charge := db.PaymentCharge{
		ID: uuid.New(), RideID: uuid.New(), Provider: "vpos",
		ProviderChargeID: "VPOS-1", Status: "completed", Currency: "AOA",
	}
	out := paymentJSONList([]db.PaymentCharge{charge})
	if len(out) != 1 {
		t.Fatalf("paymentJSONList() length = %d, want 1", len(out))
	}
	text := serveJSON(out[0])
	checks := []string{`"providerChargeId"`, `"provider":"vpos"`, `"status":"completed"`, `"currency":"AOA"`}
	for _, want := range checks {
		if !strings.Contains(text, want) {
			t.Fatalf("paymentJSONList() output = %s, want it to contain %q", text, want)
		}
	}
}

func TestSaftInvoicesJSONShapesRows(t *testing.T) {
	invoice := db.SaftInvoice{
		ID: uuid.New(), RideID: uuid.New(), InvoiceNumber: "RIDEX-2026-0001",
		Status: "issued",
	}
	out := saftInvoicesJSON([]db.SaftInvoice{invoice})
	if len(out) != 1 {
		t.Fatalf("saftInvoicesJSON() length = %d, want 1", len(out))
	}
	text := serveJSON(out[0])
	if !strings.Contains(text, `"invoiceNumber":"RIDEX-2026-0001"`) {
		t.Fatalf("saftInvoicesJSON() output = %s, want invoice number", text)
	}
	if !strings.Contains(text, `"status":"issued"`) {
		t.Fatalf("saftInvoicesJSON() output = %s, want issued status", text)
	}
}
