package payouts

import (
	"context"
	"errors"
	"fmt"
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

func (f *fakeWalletStore) ListPayoutRequestsNeedingSubmission(_ context.Context, limit int32) ([]db.PayoutRequest, error) {
	var out []db.PayoutRequest
	for _, r := range f.payouts {
		if r.Status == "processing" && !r.ReferenceID.Valid {
			out = append(out, r)
		}
	}
	if int32(len(out)) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (f *fakeWalletStore) ListPayoutRequestsInFlight(_ context.Context, limit int32) ([]db.PayoutRequest, error) {
	var out []db.PayoutRequest
	for _, r := range f.payouts {
		if r.Status == "processing" && r.ReferenceID.Valid {
			out = append(out, r)
		}
	}
	if int32(len(out)) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (f *fakeWalletStore) SetPayoutRequestReference(_ context.Context, arg db.SetPayoutRequestReferenceParams) (db.PayoutRequest, error) {
	for i := range f.payouts {
		if f.payouts[i].ID == arg.ID {
			if f.payouts[i].Status != "processing" || f.payouts[i].ReferenceID.Valid {
				return db.PayoutRequest{}, pgx.ErrNoRows
			}
			f.payouts[i].ReferenceID = arg.ReferenceID
			return f.payouts[i], nil
		}
	}
	return db.PayoutRequest{}, pgx.ErrNoRows
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

type fakeExecutor struct {
	refs      map[string]string
	lastRef   string
	submitErr error
	statusErr error
}

func (e *fakeExecutor) Submit(_ string, _ int64, _ uuid.UUID, _ string) (Submission, error) {
	if e.submitErr != nil {
		return Submission{}, e.submitErr
	}
	ref := "bank-" + uuid.New().String()
	e.lastRef = ref
	e.refs[ref] = StatusProcessing
	return Submission{Reference: ref, Status: StatusProcessing, Message: "accepted"}, nil
}

func (e *fakeExecutor) Status(reference string) (StatusInfo, error) {
	state, ok := e.refs[reference]
	if !ok {
		return StatusInfo{}, errors.New("unknown reference: " + reference)
	}
	if e.statusErr != nil {
		return StatusInfo{}, e.statusErr
	}
	return StatusInfo{Reference: reference, Status: state, Message: "fake status"}, nil
}

// approvedStoreWith returns a store with a withdrawn, admin-approved payout in
// 'processing' (no provider reference) and the wallet pending ledger entry.
func approvedStoreWith(driverID uuid.UUID) (*fakeWalletStore, db.PayoutRequest, *Ledger) {
	store := walletStoreWith(driverID, 10000)
	ledger := NewLedger(store)

	request, _, err := ledger.RequestWithdrawal(context.Background(), WithdrawalInput{
		DriverID: driverID, AmountCents: 6000, Method: "multicada",
	})
	if err != nil {
		return nil, db.PayoutRequest{}, nil
	}
	approved, err := ledger.ApprovePayout(context.Background(), request.ID)
	if err != nil {
		return nil, db.PayoutRequest{}, nil
	}
	return store, approved, ledger
}

func TestProcessPayoutSubmissionsSubmitsAndPersistsReference(t *testing.T) {
	driverID := uuid.New()
	store, request, ledger := approvedStoreWith(driverID)
	if request.ID == uuid.Nil {
		t.Fatal("store setup failed")
	}
	exec := &fakeExecutor{refs: make(map[string]string)}
	result, err := ledger.WithExecutor(exec).ProcessPayoutSubmissions(context.Background(), 25)
	if err != nil {
		t.Fatalf("process submissions: %v", err)
	}
	if result.Checked != 1 || result.Submitted != 1 {
		t.Fatalf("process result = %#v, want checked 1 submitted 1", result)
	}
	if !store.payouts[0].ReferenceID.Valid || store.payouts[0].ReferenceID.String != exec.lastRef {
		t.Fatalf("stored reference = %#v, want %q", store.payouts[0].ReferenceID, exec.lastRef)
	}
}

func TestProcessPayoutSubmissionsReversesOnSubmitError(t *testing.T) {
	driverID := uuid.New()
	store, _, _ := approvedStoreWith(driverID)
	exec := &fakeExecutor{refs: make(map[string]string), submitErr: errors.New("provider down")}
	ledger := NewLedger(store).WithExecutor(exec)
	result, err := ledger.ProcessPayoutSubmissions(context.Background(), 25)
	if err != nil {
		t.Fatalf("process submissions: %v", err)
	}
	if result.Checked != 1 || result.Failed != 1 {
		t.Fatalf("process result = %#v, want checked 1 failed 1", result)
	}
	if store.payouts[0].Status != "failed" {
		t.Fatalf("payout status = %q, want failed", store.payouts[0].Status)
	}
	if store.wallet.BalanceCents != 10000 {
		t.Fatalf("refunded balance = %d, want 10000", store.wallet.BalanceCents)
	}
}

func TestProcessPayoutSubmissionsHoldsOnUncertain(t *testing.T) {
	driverID := uuid.New()
	store, request, ledger := approvedStoreWith(driverID)
	if request.ID == uuid.Nil {
		t.Fatal("store setup failed")
	}
	exec := &fakeExecutor{refs: make(map[string]string), submitErr: fmt.Errorf("%w: response lost", ErrSubmitUncertain)}
	result, err := ledger.WithExecutor(exec).ProcessPayoutSubmissions(context.Background(), 25)
	if err != nil {
		t.Fatalf("process submissions: %v", err)
	}
	if result.Checked != 1 || result.Uncertain != 1 || result.Failed != 0 || result.Submitted != 0 {
		t.Fatalf("process result = %#v, want checked 1 uncertain 1 failed 0", result)
	}
	if store.payouts[0].Status != "processing" {
		t.Fatalf("payout status = %q, want processing (may be in flight at the provider)", store.payouts[0].Status)
	}
	if store.wallet.BalanceCents != 4000 {
		t.Fatalf("balance = %d, want 4000 (debit held while the outcome is unknown)", store.wallet.BalanceCents)
	}
}

func TestProcessPayoutSubmissionsRequiresExecutor(t *testing.T) {
	ledger := NewLedger(walletStoreWith(uuid.New(), 10000))
	if _, err := ledger.ProcessPayoutSubmissions(context.Background(), 25); !errors.Is(err, ErrExecutorUnavailable) {
		t.Fatalf("process without executor: err = %v, want ErrExecutorUnavailable", err)
	}
	if _, err := ledger.PollPayoutExecutions(context.Background(), 25); !errors.Is(err, ErrExecutorUnavailable) {
		t.Fatalf("poll without executor: err = %v, want ErrExecutorUnavailable", err)
	}
}

func TestPollPayoutExecutionsSettlesCompleted(t *testing.T) {
	driverID := uuid.New()
	store, request, ledger := approvedStoreWith(driverID)
	if request.ID == uuid.Nil {
		t.Fatal("store setup failed")
	}
	const reference = "bank-ref-completed"
	store.payouts[0].ReferenceID = pgtype.Text{String: reference, Valid: true}
	exec := &fakeExecutor{refs: map[string]string{reference: StatusCompleted}}

	result, err := ledger.WithExecutor(exec).PollPayoutExecutions(context.Background(), 25)
	if err != nil {
		t.Fatalf("poll: %v", err)
	}
	if result.Checked != 1 || result.Updated != 1 {
		t.Fatalf("poll result = %#v, want checked 1 updated 1", result)
	}
	if store.payouts[0].Status != "completed" {
		t.Fatalf("payout status = %q, want completed", store.payouts[0].Status)
	}
	if store.wallet.BalanceCents != 4000 {
		t.Fatalf("balance = %d, want 4000 (settlement keeps the debit)", store.wallet.BalanceCents)
	}
	if store.lastPayoutCalls != 1 {
		t.Fatalf("last payout updates = %d, want 1", store.lastPayoutCalls)
	}
}

func TestPollPayoutExecutionsReversesFailed(t *testing.T) {
	driverID := uuid.New()
	store, request, ledger := approvedStoreWith(driverID)
	if request.ID == uuid.Nil {
		t.Fatal("store setup failed")
	}
	const reference = "bank-ref-failed"
	store.payouts[0].ReferenceID = pgtype.Text{String: reference, Valid: true}
	exec := &fakeExecutor{refs: map[string]string{reference: StatusFailed}}

	result, err := ledger.WithExecutor(exec).PollPayoutExecutions(context.Background(), 25)
	if err != nil {
		t.Fatalf("poll: %v", err)
	}
	if result.Checked != 1 || result.Updated != 1 {
		t.Fatalf("poll result = %#v, want checked 1 updated 1", result)
	}
	if store.payouts[0].Status != "failed" {
		t.Fatalf("payout status = %q, want failed", store.payouts[0].Status)
	}
	if store.wallet.BalanceCents != 10000 {
		t.Fatalf("balance = %d, want 10000 (withdrawal refunded)", store.wallet.BalanceCents)
	}
}

func TestPollPayoutExecutionsSkipsUnknownStates(t *testing.T) {
	driverID := uuid.New()
	store, request, ledger := approvedStoreWith(driverID)
	if request.ID == uuid.Nil {
		t.Fatal("store setup failed")
	}
	const reference = "bank-ref-mystery"
	store.payouts[0].ReferenceID = pgtype.Text{String: reference, Valid: true}
	exec := &fakeExecutor{refs: map[string]string{reference: "mystery_state"}}

	result, err := ledger.WithExecutor(exec).PollPayoutExecutions(context.Background(), 25)
	if err != nil {
		t.Fatalf("poll: %v", err)
	}
	if result.Checked != 1 || result.Skipped != 1 || result.Updated != 0 {
		t.Fatalf("poll result = %#v, want checked 1 skipped 1", result)
	}
	if store.payouts[0].Status != "processing" {
		t.Fatalf("payout status = %q, want processing (unknown state never guessed)", store.payouts[0].Status)
	}
}

func TestPollPayoutExecutionsSkipsPollErrors(t *testing.T) {
	driverID := uuid.New()
	store, request, ledger := approvedStoreWith(driverID)
	if request.ID == uuid.Nil {
		t.Fatal("store setup failed")
	}
	const reference = "bank-ref-timeout"
	store.payouts[0].ReferenceID = pgtype.Text{String: reference, Valid: true}
	exec := &fakeExecutor{refs: map[string]string{reference: StatusCompleted}, statusErr: errors.New("provider timeout")}

	result, err := ledger.WithExecutor(exec).PollPayoutExecutions(context.Background(), 25)
	if err != nil {
		t.Fatalf("poll: %v", err)
	}
	if result.Checked != 1 || result.Skipped != 1 {
		t.Fatalf("poll result = %#v, want checked 1 skipped 1", result)
	}
	if store.payouts[0].Status != "processing" {
		t.Fatalf("payout status = %q, want processing (transient error must not settle)", store.payouts[0].Status)
	}
}
