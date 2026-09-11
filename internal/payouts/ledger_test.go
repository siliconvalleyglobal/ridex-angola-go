package payouts

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/ridex/ridex-angola/internal/db"
)

type fakeWalletStore struct {
	wallet          db.DriverWallet
	payouts         []db.PayoutRequest
	transactions    []db.WalletTransaction
	earnings        db.GetWalletEarningsSummaryRow
	deductCalls     int
	balanceAddCalls int
	lastPayoutCalls int
	createdTx       []db.CreateWalletTransactionParams
}

func (f *fakeWalletStore) GetDriverWallet(context.Context, uuid.UUID) (db.DriverWallet, error) {
	if f.wallet.DriverID == uuid.Nil {
		return db.DriverWallet{}, pgx.ErrNoRows
	}
	return f.wallet, nil
}

func (f *fakeWalletStore) CreateDriverWallet(context.Context, uuid.UUID) error { return nil }

func (f *fakeWalletStore) DeductDriverBalance(_ context.Context, arg db.DeductDriverBalanceParams) (db.DriverWallet, error) {
	f.deductCalls++
	if f.wallet.BalanceCents < arg.BalanceCents {
		return db.DriverWallet{}, pgx.ErrNoRows
	}
	f.wallet.BalanceCents -= arg.BalanceCents
	return f.wallet, nil
}

func (f *fakeWalletStore) AddDriverBalance(_ context.Context, arg db.AddDriverBalanceParams) (db.DriverWallet, error) {
	f.balanceAddCalls++
	f.wallet.BalanceCents += arg.BalanceCents
	return f.wallet, nil
}

func (f *fakeWalletStore) CreateWalletTransaction(_ context.Context, arg db.CreateWalletTransactionParams) error {
	f.createdTx = append(f.createdTx, arg)
	f.transactions = append(f.transactions, db.WalletTransaction{
		ID:            uuid.New(),
		DriverID:      arg.DriverID,
		Type:          arg.Type,
		AmountCents:   arg.AmountCents,
		BalanceAfter:  arg.BalanceAfter,
		ReferenceID:   arg.ReferenceID,
		ReferenceType: arg.ReferenceType,
		Description:   arg.Description,
		Status:        arg.Status,
	})
	return nil
}

func (f *fakeWalletStore) SetWalletTransactionStatus(_ context.Context, arg db.SetWalletTransactionStatusParams) error {
	for i := range f.transactions {
		tx := &f.transactions[i]
		if tx.ReferenceID.Valid && tx.ReferenceID.Bytes == arg.ReferenceID.Bytes && tx.Status == arg.Status_2 {
			tx.Status = arg.Status
			return nil
		}
	}
	return pgx.ErrNoRows
}

func (f *fakeWalletStore) CreatePayoutRequest(_ context.Context, arg db.CreatePayoutRequestParams) (db.PayoutRequest, error) {
	request := db.PayoutRequest{
		ID: uuid.New(), DriverID: arg.DriverID, AmountCents: arg.AmountCents,
		Method: arg.Method, Status: "pending",
	}
	f.payouts = append(f.payouts, request)
	return request, nil
}

func (f *fakeWalletStore) GetPayoutRequestByID(_ context.Context, id uuid.UUID) (db.PayoutRequest, error) {
	for _, r := range f.payouts {
		if r.ID == id {
			return r, nil
		}
	}
	return db.PayoutRequest{}, pgx.ErrNoRows
}

func (f *fakeWalletStore) GetActivePayoutRequestsByDriver(_ context.Context, driverID uuid.UUID) ([]db.PayoutRequest, error) {
	var active []db.PayoutRequest
	for _, r := range f.payouts {
		if r.DriverID == driverID && (r.Status == "pending" || r.Status == "processing") {
			active = append(active, r)
		}
	}
	return active, nil
}

func (f *fakeWalletStore) MarkPayoutRequestProcessing(_ context.Context, id uuid.UUID) (db.PayoutRequest, error) {
	for i := range f.payouts {
		if f.payouts[i].ID == id {
			if f.payouts[i].Status != "pending" {
				return db.PayoutRequest{}, pgx.ErrNoRows
			}
			f.payouts[i].Status = "processing"
			return f.payouts[i], nil
		}
	}
	return db.PayoutRequest{}, pgx.ErrNoRows
}

func (f *fakeWalletStore) CompletePayoutRequest(_ context.Context, id uuid.UUID) (db.PayoutRequest, error) {
	for i := range f.payouts {
		if f.payouts[i].ID == id {
			if f.payouts[i].Status != "processing" {
				return db.PayoutRequest{}, pgx.ErrNoRows
			}
			f.payouts[i].Status = "completed"
			return f.payouts[i], nil
		}
	}
	return db.PayoutRequest{}, pgx.ErrNoRows
}

func (f *fakeWalletStore) FailPayoutRequest(_ context.Context, arg db.FailPayoutRequestParams) (db.PayoutRequest, error) {
	for i := range f.payouts {
		if f.payouts[i].ID == arg.ID {
			if f.payouts[i].Status != "pending" && f.payouts[i].Status != "processing" {
				return db.PayoutRequest{}, pgx.ErrNoRows
			}
			f.payouts[i].Status = "failed"
			return f.payouts[i], nil
		}
	}
	return db.PayoutRequest{}, pgx.ErrNoRows
}

func (f *fakeWalletStore) UpdateLastPayout(_ context.Context, _ uuid.UUID) error {
	f.lastPayoutCalls++
	return nil
}

func (f *fakeWalletStore) ListPayoutRequestsByDriver(_ context.Context, arg db.ListPayoutRequestsByDriverParams) ([]db.PayoutRequest, error) {
	var out []db.PayoutRequest
	for _, r := range f.payouts {
		if r.DriverID == arg.DriverID {
			out = append(out, r)
		}
	}
	if int32(len(out)) > arg.Limit {
		out = out[:arg.Limit]
	}
	return out, nil
}

func (f *fakeWalletStore) ListPayoutRequestsByStatus(_ context.Context, arg db.ListPayoutRequestsByStatusParams) ([]db.PayoutRequest, error) {
	var out []db.PayoutRequest
	for _, r := range f.payouts {
		if arg.Column1 == "" || r.Status == arg.Column1 {
			out = append(out, r)
		}
	}
	return out, nil
}

func (f *fakeWalletStore) ListWalletTransactions(_ context.Context, arg db.ListWalletTransactionsParams) ([]db.WalletTransaction, error) {
	var out []db.WalletTransaction
	for _, t := range f.transactions {
		if t.DriverID == arg.DriverID {
			out = append(out, t)
		}
	}
	return out, nil
}

func (f *fakeWalletStore) GetWalletEarningsSummary(context.Context, uuid.UUID) (db.GetWalletEarningsSummaryRow, error) {
	return f.earnings, nil
}

func walletStoreWith(driverID uuid.UUID, balance int64) *fakeWalletStore {
	return &fakeWalletStore{wallet: db.DriverWallet{DriverID: driverID, BalanceCents: balance, Currency: "AOA"}}
}

func TestRequestWithdrawalDebitsWalletAndRecordsLedger(t *testing.T) {
	driverID := uuid.New()
	store := walletStoreWith(driverID, 10000)
	ledger := NewLedger(store)

	request, wallet, err := ledger.RequestWithdrawal(context.Background(), WithdrawalInput{
		DriverID: driverID, AmountCents: 6000, Method: "multicada", Description: "safety net",
	})
	if err != nil {
		t.Fatalf("request withdrawal: %v", err)
	}
	if request.Status != "pending" || request.AmountCents != 6000 || request.Method != "multicada" {
		t.Fatalf("payout request = %#v", request)
	}
	if wallet.BalanceCents != 4000 {
		t.Fatalf("wallet balance = %d, want 4000", wallet.BalanceCents)
	}
	if len(store.createdTx) != 1 {
		t.Fatalf("wallet transactions = %d, want one", len(store.createdTx))
	}
	tx := store.createdTx[0]
	if tx.AmountCents != -6000 || tx.Status != txStatusPending || tx.BalanceAfter != 4000 {
		t.Fatalf("wallet transaction = %#v", tx)
	}
	if !tx.ReferenceID.Valid || tx.ReferenceID.Bytes != request.ID || tx.ReferenceType.String != referenceType {
		t.Fatalf("wallet transaction reference mismatch = %#v", tx.ReferenceID)
	}
}

func TestRequestWithdrawalInsufficientBalance(t *testing.T) {
	driverID := uuid.New()
	store := walletStoreWith(driverID, 1000)
	ledger := NewLedger(store)
	_, _, err := ledger.RequestWithdrawal(context.Background(), WithdrawalInput{
		DriverID: driverID, AmountCents: 15000, Method: "multicada",
	})
	if !errors.Is(err, ErrInsufficientBalance) {
		t.Fatalf("withdrawal: err = %v, want ErrInsufficientBalance", err)
	}
	if store.deductCalls != 1 || len(store.createdTx) != 0 {
		t.Fatalf("ledger mutated on failure: deduct=%d tx=%d", store.deductCalls, len(store.createdTx))
	}
}

func TestRequestWithdrawalRejectsWhenAlreadyActive(t *testing.T) {
	driverID := uuid.New()
	store := walletStoreWith(driverID, 30000)
	store.payouts = append(store.payouts, db.PayoutRequest{
		ID: uuid.New(), DriverID: driverID, AmountCents: 5000, Method: "bank", Status: "processing",
	})
	ledger := NewLedger(store)

	if _, _, err := ledger.RequestWithdrawal(context.Background(), WithdrawalInput{
		DriverID: driverID, AmountCents: 15000, Method: "bank",
	}); !errors.Is(err, ErrPayoutAlreadyPending) {
		t.Fatalf("concurrent withdrawal: err = %v, want ErrPayoutAlreadyPending", err)
	}
}

func TestRequestWithdrawalValidatesInput(t *testing.T) {
	ledger := NewLedger(&fakeWalletStore{})
	bankInfo, ok := ledger.methods.Info(PayoutMethodBank)
	if !ok {
		t.Fatalf("bank method info missing")
	}
	driverID := uuid.New()
	cases := []struct {
		name string
		in   WithdrawalInput
	}{
		{"below minimum", WithdrawalInput{DriverID: driverID, AmountCents: bankInfo.MinCents - 1, Method: "bank"}},
		{"above maximum", WithdrawalInput{DriverID: driverID, AmountCents: bankInfo.MaxCents + 1, Method: "bank"}},
		{"bad method", WithdrawalInput{DriverID: driverID, AmountCents: 6000, Method: "paypal"}},
		{"no driver", WithdrawalInput{AmountCents: 6000, Method: "bank"}},
	}
	for _, tc := range cases {
		if _, _, err := ledger.RequestWithdrawal(context.Background(), tc.in); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("%s: err = %v, want ErrInvalidInput", tc.name, err)
		}
	}
}

func TestApproveAndCompletePayout(t *testing.T) {
	driverID := uuid.New()
	store := walletStoreWith(driverID, 10000)
	ledger := NewLedger(store)
	request, _, err := ledger.RequestWithdrawal(context.Background(), WithdrawalInput{
		DriverID: driverID, AmountCents: 6000, Method: "multicada",
	})
	if err != nil {
		t.Fatalf("request: %v", err)
	}

	approved, err := ledger.ApprovePayout(context.Background(), request.ID)
	if err != nil || approved.Status != "processing" {
		t.Fatalf("approve: err = %v status = %s", err, approved.Status)
	}
	if _, err := ledger.ApprovePayout(context.Background(), request.ID); !errors.Is(err, ErrPayoutTransition) {
		t.Fatalf("double approve: err = %v, want ErrPayoutTransition", err)
	}

	completed, err := ledger.CompletePayout(context.Background(), request.ID)
	if err != nil || completed.Status != "completed" {
		t.Fatalf("complete: err = %v status = %s", err, completed.Status)
	}
	if store.lastPayoutCalls != 1 {
		t.Fatalf("last payout calls = %d, want 1", store.lastPayoutCalls)
	}
}

func TestCompletePayoutRequiresProcessing(t *testing.T) {
	store := walletStoreWith(uuid.New(), 10000)
	ledger := NewLedger(store)
	request := db.PayoutRequest{ID: uuid.New(), DriverID: uuid.New(), Status: "pending"}
	store.payouts = append(store.payouts, request)

	_, err := ledger.CompletePayout(context.Background(), request.ID)
	if !errors.Is(err, ErrPayoutTransition) {
		t.Fatalf("complete pending: err = %v, want ErrPayoutTransition", err)
	}
}

func TestFailPayoutReversesBalanceAndRecordsAdjustment(t *testing.T) {
	driverID := uuid.New()
	store := walletStoreWith(driverID, 4000)
	request := db.PayoutRequest{ID: uuid.New(), DriverID: driverID, AmountCents: 6000, Method: "bank", Status: "processing"}
	store.payouts = append(store.payouts, request)
	store.transactions = append(store.transactions, db.WalletTransaction{
		ID: uuid.New(), DriverID: driverID, Type: payoutTxType, AmountCents: -6000,
		BalanceAfter: 4000, Status: txStatusPending,
		ReferenceID: pgtype.UUID{Bytes: request.ID, Valid: true},
	})
	ledger := NewLedger(store)

	failed, err := ledger.FailPayout(context.Background(), request.ID, "provider rejected")
	if err != nil {
		t.Fatalf("fail payout: %v", err)
	}
	if failed.Status != "failed" {
		t.Fatalf("status = %s, want failed", failed.Status)
	}
	if store.wallet.BalanceCents != 10000 {
		t.Fatalf("balance after refund = %d, want 10000", store.wallet.BalanceCents)
	}
	if store.balanceAddCalls != 1 {
		t.Fatalf("balance add calls = %d, want 1", store.balanceAddCalls)
	}
	var adjustment *db.CreateWalletTransactionParams
	for i := range store.createdTx {
		if store.createdTx[i].Type == adjustTxType {
			adjustment = &store.createdTx[i]
		}
	}
	if adjustment == nil {
		t.Fatal("expected a completed adjustment transaction")
	}
	if adjustment.AmountCents != 6000 || adjustment.Status != "completed" {
		t.Fatalf("adjustment = %#v", adjustment)
	}
}

func TestFailPayoutCannotFailCompleted(t *testing.T) {
	store := walletStoreWith(uuid.New(), 10000)
	ledger := NewLedger(store)
	request := db.PayoutRequest{ID: uuid.New(), DriverID: uuid.New(), AmountCents: 6000, Method: "bank", Status: "completed"}
	store.payouts = append(store.payouts, request)

	_, err := ledger.FailPayout(context.Background(), request.ID, "too late")
	if !errors.Is(err, ErrPayoutTransition) {
		t.Fatalf("fail completed: err = %v, want ErrPayoutTransition", err)
	}
}

func TestWalletOverviewAggregatesEarnings(t *testing.T) {
	driverID := uuid.New()
	store := walletStoreWith(driverID, 3250)
	store.earnings = db.GetWalletEarningsSummaryRow{
		TodayCents: "1500", WeekCents: "7250", MonthCents: "32500", TotalCents: "32500",
	}
	overview, err := NewLedger(store).WalletOverview(context.Background(), driverID)
	if err != nil {
		t.Fatalf("wallet overview: %v", err)
	}
	if overview.EarningsSummary.TodayCents != 1500 || overview.EarningsSummary.WeekCents != 7250 ||
		overview.EarningsSummary.MonthCents != 32500 || overview.EarningsSummary.TotalCents != 32500 {
		t.Fatalf("earnings summary = %#v", overview.EarningsSummary)
	}
	if overview.Wallet.BalanceCents != 3250 {
		t.Fatalf("balance = %d, want 3250", overview.Wallet.BalanceCents)
	}
}

func TestNumericToInt64HandlesScannedTypes(t *testing.T) {
	cases := map[interface{}]int64{nil: 0, "1234": 1234, int64(7): 7, float64(9.6): 10, "9.6": 10, "garbage": 0}
	for value, want := range cases {
		if got := numericToInt64(value); got != want {
			t.Fatalf("numericToInt64(%v) = %d, want %d", value, got, want)
		}
	}
}
