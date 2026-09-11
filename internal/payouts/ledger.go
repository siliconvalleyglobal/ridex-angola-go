package payouts

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/ridex/ridex-angola/internal/db"
)

// Withdrawal bounds and allowed methods follow the payout_requests CHECK
// constraint in migration 000023; 'multicada' uses the constraint's spelling.
const (
	payoutTxType    = "payout"
	adjustTxType    = "adjustment"
	referenceType   = "payout"
	txStatusPending = "pending"
)

var (
	ErrInvalidInput         = errors.New("invalid payout input")
	ErrInsufficientBalance  = errors.New("insufficient wallet balance")
	ErrPayoutAlreadyPending = errors.New("a payout request is already pending or processing")
	ErrPayoutNotFound       = errors.New("payout request not found")
	ErrPayoutTransition     = errors.New("payout status transition is not allowed")
	ErrWalletNotFound       = errors.New("driver wallet not found")
)

// Store is the database surface used by the payout ledger. Keeping it narrow
// makes the money-movement flows testable without a live database.
type Store interface {
	GetDriverWallet(context.Context, uuid.UUID) (db.DriverWallet, error)
	CreateDriverWallet(context.Context, uuid.UUID) error
	DeductDriverBalance(context.Context, db.DeductDriverBalanceParams) (db.DriverWallet, error)
	AddDriverBalance(context.Context, db.AddDriverBalanceParams) (db.DriverWallet, error)
	CreateWalletTransaction(context.Context, db.CreateWalletTransactionParams) error
	SetWalletTransactionStatus(context.Context, db.SetWalletTransactionStatusParams) error
	CreatePayoutRequest(context.Context, db.CreatePayoutRequestParams) (db.PayoutRequest, error)
	GetPayoutRequestByID(context.Context, uuid.UUID) (db.PayoutRequest, error)
	GetActivePayoutRequestsByDriver(context.Context, uuid.UUID) ([]db.PayoutRequest, error)
	MarkPayoutRequestProcessing(context.Context, uuid.UUID) (db.PayoutRequest, error)
	CompletePayoutRequest(context.Context, uuid.UUID) (db.PayoutRequest, error)
	FailPayoutRequest(context.Context, db.FailPayoutRequestParams) (db.PayoutRequest, error)
	UpdateLastPayout(context.Context, uuid.UUID) error
	ListPayoutRequestsByDriver(context.Context, db.ListPayoutRequestsByDriverParams) ([]db.PayoutRequest, error)
	ListPayoutRequestsByStatus(context.Context, db.ListPayoutRequestsByStatusParams) ([]db.PayoutRequest, error)
	ListWalletTransactions(context.Context, db.ListWalletTransactionsParams) ([]db.WalletTransaction, error)
	GetWalletEarningsSummary(context.Context, uuid.UUID) (db.GetWalletEarningsSummaryRow, error)
}

// Ledger executes wallet-backed payout flows. Every money movement is wrapped
// in a database transaction when a begin function is wired: the balance
// deduction, the payout request row, and the wallet ledger entry either all
// commit or all roll back, so a crash can never leave a debited balance
// without a matching withdrawal record.
type Ledger struct {
	store Store
	begin func(context.Context) (pgx.Tx, error)
	// methods is the enabled-method registry consulted for every withdrawal.
	// It is never nil: constructors fall back to DefaultPayoutMethods.
	methods *PayoutMethods
	// audit receives lifecycle events. It is never nil: constructors fall back
	// to the in-process no-op logger.
	audit AuditLogger
}

// NewLedger creates a payout ledger without transaction support (read paths
// and unit tests).
func NewLedger(store Store) *Ledger {
	return &Ledger{store: store, methods: DefaultPayoutMethods(), audit: NewNoopAuditLogger()}
}

// NewLedgerWithTx enables atomic wallet mutations. The db.Queries instance is
// rebound onto each transaction for the duration of the flow.
func NewLedgerWithTx(store Store, begin func(context.Context) (pgx.Tx, error)) *Ledger {
	return &Ledger{store: store, begin: begin, methods: DefaultPayoutMethods(), audit: NewNoopAuditLogger()}
}

// WithMethods overrides the enabled-method registry on an existing ledger.
func (l *Ledger) WithMethods(methods *PayoutMethods) *Ledger {
	if methods != nil {
		l.methods = methods
	}
	return l
}

// WithAudit overrides the audit logger on an existing ledger.
func (l *Ledger) WithAudit(audit AuditLogger) *Ledger {
	if audit != nil {
		l.audit = audit
	}
	return l
}

func (l *Ledger) withTx(ctx context.Context, fn func(Store) error) error {
	if l.begin == nil {
		return fn(l.store)
	}
	tx, err := l.begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	// The concrete store is *db.Queries; WithTx clones it onto the tx.
	if q, ok := l.store.(*db.Queries); ok {
		if err := fn(q.WithTx(tx)); err != nil {
			return err
		}
	} else {
		if err := fn(l.store); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// WithdrawalInput describes a driver-initiated withdrawal.
type WithdrawalInput struct {
	DriverID    uuid.UUID
	AmountCents int64
	Method      string
	Description string
}

// RequestWithdrawal atomically reserves a withdrawal: it debits the wallet,
// creates a pending payout request, and records a pending wallet ledger entry
// whose reference points at the request. A driver can only have one active
// (pending/processing) request at a time.
func (l *Ledger) RequestWithdrawal(ctx context.Context, in WithdrawalInput) (db.PayoutRequest, db.DriverWallet, error) {
	if in.DriverID == uuid.Nil {
		return db.PayoutRequest{}, db.DriverWallet{}, ErrInvalidInput
	}
	method, methodErr := l.methods.NormalizePayoutMethod(in.Method)
	if methodErr != nil {
		RecordPayoutEvent(l.audit, ctx, PayoutEvent{DriverID: in.DriverID.String(), AmountCents: in.AmountCents, Status: "rejected", Method: in.Method, Reason: "unsupported payout method"})
		return db.PayoutRequest{}, db.DriverWallet{}, fmt.Errorf("%w: unsupported payout method", ErrInvalidInput)
	}
	if info, ok := l.methods.Info(method); ok {
		if in.AmountCents < info.MinCents || in.AmountCents > info.MaxCents {
			RecordPayoutEvent(l.audit, ctx, PayoutEvent{DriverID: in.DriverID.String(), AmountCents: in.AmountCents, Status: "rejected", Method: string(method), Reason: "amount outside method bounds"})
			return db.PayoutRequest{}, db.DriverWallet{}, fmt.Errorf(
				"%w: payout amount must be between %d and %d cents", ErrInvalidInput, info.MinCents, info.MaxCents)
		}
	}

	active, err := l.store.GetActivePayoutRequestsByDriver(ctx, in.DriverID)
	if err != nil {
		return db.PayoutRequest{}, db.DriverWallet{}, fmt.Errorf("load active payouts: %w", err)
	}
	if len(active) > 0 {
		RecordPayoutEvent(l.audit, ctx, PayoutEvent{DriverID: in.DriverID.String(), AmountCents: in.AmountCents, Status: "already-pending", Method: string(method)})
		return db.PayoutRequest{}, db.DriverWallet{}, ErrPayoutAlreadyPending
	}

	var (
		request db.PayoutRequest
		wallet  db.DriverWallet
	)
	err = l.withTx(ctx, func(s Store) error {
		if err := s.CreateDriverWallet(ctx, in.DriverID); err != nil {
			return fmt.Errorf("ensure wallet: %w", err)
		}
		// DeductDriverBalance is guarded by balance_cents >= amount in SQL, so
		// no rows means the balance is insufficient.
		wallet, err = s.DeductDriverBalance(ctx, db.DeductDriverBalanceParams{
			DriverID: in.DriverID, BalanceCents: in.AmountCents,
		})
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrInsufficientBalance
		}
		if err != nil {
			return fmt.Errorf("deduct wallet balance: %w", err)
		}
		request, err = s.CreatePayoutRequest(ctx, db.CreatePayoutRequestParams{
			DriverID: in.DriverID, AmountCents: in.AmountCents, Method: string(method),
		})
		if err != nil {
			return fmt.Errorf("create payout request: %w", err)
		}
		if err := s.CreateWalletTransaction(ctx, db.CreateWalletTransactionParams{
			DriverID:      in.DriverID,
			Type:          payoutTxType,
			AmountCents:   -in.AmountCents,
			BalanceAfter:  wallet.BalanceCents,
			ReferenceID:   pgtype.UUID{Bytes: request.ID, Valid: true},
			ReferenceType: pgtype.Text{String: referenceType, Valid: true},
			Description:   pgtype.Text{String: in.Description, Valid: in.Description != ""},
			Status:        txStatusPending,
		}); err != nil {
			return fmt.Errorf("record wallet transaction: %w", err)
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, ErrInsufficientBalance) {
			RecordPayoutEvent(l.audit, ctx, PayoutEvent{DriverID: in.DriverID.String(), AmountCents: in.AmountCents, Status: "insufficient-balance", Method: string(method)})
		}
		return db.PayoutRequest{}, db.DriverWallet{}, err
	}
	RecordPayoutEvent(l.audit, ctx, PayoutEvent{PayoutID: request.ID.String(), DriverID: in.DriverID.String(), AmountCents: in.AmountCents, Status: "requested", Method: request.Method})
	return request, wallet, nil
}

// ApprovePayout moves a pending request to processing. This is the explicit
// admin acceptance step; only processing requests can be completed.
func (l *Ledger) ApprovePayout(ctx context.Context, payoutID uuid.UUID) (db.PayoutRequest, error) {
	if payoutID == uuid.Nil {
		return db.PayoutRequest{}, ErrInvalidInput
	}
	if _, err := l.store.GetPayoutRequestByID(ctx, payoutID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.PayoutRequest{}, ErrPayoutNotFound
		}
		return db.PayoutRequest{}, fmt.Errorf("load payout request: %w", err)
	}
	request, err := l.store.MarkPayoutRequestProcessing(ctx, payoutID)
	if errors.Is(err, pgx.ErrNoRows) {
		return db.PayoutRequest{}, ErrPayoutTransition
	}
	if err != nil {
		return db.PayoutRequest{}, fmt.Errorf("approve payout request: %w", err)
	}
	RecordPayoutEvent(l.audit, ctx, PayoutEvent{PayoutID: request.ID.String(), DriverID: request.DriverID.String(), AmountCents: request.AmountCents, Status: "approved", Method: request.Method})
	return request, nil
}

// CompletePayout finalizes an approved (processing) withdrawal: the request
// is marked completed, its pending wallet entry becomes completed, and the
// driver's last-payout timestamp is refreshed. Everything commits together so
// a completed request always has a settled ledger entry.
func (l *Ledger) CompletePayout(ctx context.Context, payoutID uuid.UUID) (db.PayoutRequest, error) {
	if payoutID == uuid.Nil {
		return db.PayoutRequest{}, ErrInvalidInput
	}
	var request db.PayoutRequest
	err := l.withTx(ctx, func(s Store) error {
		req, err := s.GetPayoutRequestByID(ctx, payoutID)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrPayoutNotFound
		}
		if err != nil {
			return fmt.Errorf("load payout request: %w", err)
		}
		request, err = s.CompletePayoutRequest(ctx, payoutID)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrPayoutTransition
		}
		if err != nil {
			return fmt.Errorf("complete payout request: %w", err)
		}
		if err := s.SetWalletTransactionStatus(ctx, db.SetWalletTransactionStatusParams{
			ReferenceID: pgtype.UUID{Bytes: payoutID, Valid: true},
			Status:      "completed",
			Status_2:    txStatusPending,
		}); err != nil {
			return fmt.Errorf("settle wallet transaction: %w", err)
		}
		if err := s.UpdateLastPayout(ctx, req.DriverID); err != nil {
			return fmt.Errorf("update last payout: %w", err)
		}
		return nil
	})
	if err != nil {
		return db.PayoutRequest{}, err
	}
	RecordPayoutEvent(l.audit, ctx, PayoutEvent{PayoutID: request.ID.String(), DriverID: request.DriverID.String(), AmountCents: request.AmountCents, Status: "completed", Method: request.Method})
	return request, nil
}

// FailPayout rejects a pending or processing withdrawal and returns the money
// to the driver's wallet. The failed request, the reversed ledger entry, and
// the balance refund commit together; the refund is itself recorded as a
// completed adjustment entry so the ledger always reconciles with the balance.
func (l *Ledger) FailPayout(ctx context.Context, payoutID uuid.UUID, reason string) (db.PayoutRequest, error) {
	if payoutID == uuid.Nil {
		return db.PayoutRequest{}, ErrInvalidInput
	}
	var request db.PayoutRequest
	err := l.withTx(ctx, func(s Store) error {
		req, err := s.GetPayoutRequestByID(ctx, payoutID)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrPayoutNotFound
		}
		if err != nil {
			return fmt.Errorf("load payout request: %w", err)
		}
		if req.Status == "completed" {
			return ErrPayoutTransition
		}
		request, err = s.FailPayoutRequest(ctx, db.FailPayoutRequestParams{
			ID:            payoutID,
			FailureReason: pgtype.Text{String: reason, Valid: reason != ""},
		})
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrPayoutTransition
		}
		if err != nil {
			return fmt.Errorf("fail payout request: %w", err)
		}
		if err := s.SetWalletTransactionStatus(ctx, db.SetWalletTransactionStatusParams{
			ReferenceID: pgtype.UUID{Bytes: payoutID, Valid: true},
			Status:      "reversed",
			Status_2:    txStatusPending,
		}); err != nil {
			return fmt.Errorf("reverse wallet transaction: %w", err)
		}
		wallet, err := s.AddDriverBalance(ctx, db.AddDriverBalanceParams{
			DriverID: req.DriverID, BalanceCents: req.AmountCents,
		})
		if err != nil {
			return fmt.Errorf("refund wallet balance: %w", err)
		}
		description := "payout refund"
		if reason != "" {
			description = "payout refund: " + reason
		}
		return s.CreateWalletTransaction(ctx, db.CreateWalletTransactionParams{
			DriverID:      req.DriverID,
			Type:          adjustTxType,
			AmountCents:   req.AmountCents,
			BalanceAfter:  wallet.BalanceCents,
			ReferenceID:   pgtype.UUID{Bytes: payoutID, Valid: true},
			ReferenceType: pgtype.Text{String: referenceType, Valid: true},
			Description:   pgtype.Text{String: description, Valid: true},
			Status:        "completed",
		})
	})
	if err != nil {
		return db.PayoutRequest{}, err
	}
	RecordPayoutEvent(l.audit, ctx, PayoutEvent{PayoutID: request.ID.String(), DriverID: request.DriverID.String(), AmountCents: request.AmountCents, Status: "failed", Method: request.Method, Reason: reason})
	return request, nil
}

// WalletOverview is the driver-facing wallet summary.
type WalletOverview struct {
	Wallet          db.DriverWallet           `json:"wallet"`
	EarningsSummary WalletEarningsSummaryJSON `json:"earningsSummary"`
}

// WalletEarningsSummaryJSON converts the generated numeric columns to JSON
// friendly int64s.
type WalletEarningsSummaryJSON struct {
	TodayCents int64 `json:"todayCents"`
	WeekCents  int64 `json:"weekCents"`
	MonthCents int64 `json:"monthCents"`
	TotalCents int64 `json:"totalCents"`
}

// WalletOverview ensures the wallet exists and returns it with the driver's
// rolling earnings summary.
func (l *Ledger) WalletOverview(ctx context.Context, driverID uuid.UUID) (WalletOverview, error) {
	if driverID == uuid.Nil {
		return WalletOverview{}, ErrInvalidInput
	}
	if err := l.store.CreateDriverWallet(ctx, driverID); err != nil {
		return WalletOverview{}, fmt.Errorf("ensure wallet: %w", err)
	}
	wallet, err := l.store.GetDriverWallet(ctx, driverID)
	if errors.Is(err, pgx.ErrNoRows) {
		return WalletOverview{}, ErrWalletNotFound
	}
	if err != nil {
		return WalletOverview{}, fmt.Errorf("load wallet: %w", err)
	}
	summary, err := l.store.GetWalletEarningsSummary(ctx, driverID)
	if err != nil {
		return WalletOverview{}, fmt.Errorf("load earnings summary: %w", err)
	}
	return WalletOverview{
		Wallet: wallet,
		EarningsSummary: WalletEarningsSummaryJSON{
			TodayCents: numericToInt64(summary.TodayCents),
			WeekCents:  numericToInt64(summary.WeekCents),
			MonthCents: numericToInt64(summary.MonthCents),
			TotalCents: numericToInt64(summary.TotalCents),
		},
	}, nil
}

// DriverPayouts lists a driver's payout requests with pagination.
func (l *Ledger) DriverPayouts(ctx context.Context, driverID uuid.UUID, limit, offset int32) ([]db.PayoutRequest, error) {
	if driverID == uuid.Nil {
		return nil, ErrInvalidInput
	}
	limit, offset = paginate(limit, offset)
	requests, err := l.store.ListPayoutRequestsByDriver(ctx, db.ListPayoutRequestsByDriverParams{
		DriverID: driverID, Limit: limit, Offset: offset,
	})
	if err != nil {
		return nil, fmt.Errorf("list payout requests: %w", err)
	}
	return requests, nil
}

// AdminPayoutQueue lists payout requests for the admin console, optionally
// filtered by status.
func (l *Ledger) AdminPayoutQueue(ctx context.Context, status string, limit, offset int32) ([]db.PayoutRequest, error) {
	limit, offset = paginate(limit, offset)
	requests, err := l.store.ListPayoutRequestsByStatus(ctx, db.ListPayoutRequestsByStatusParams{
		Column1: status, Limit: limit, Offset: offset,
	})
	if err != nil {
		return nil, fmt.Errorf("list payout queue: %w", err)
	}
	return requests, nil
}

// WalletTransactions lists a driver's wallet ledger entries with pagination.
func (l *Ledger) WalletTransactions(ctx context.Context, driverID uuid.UUID, limit, offset int32) ([]db.WalletTransaction, error) {
	if driverID == uuid.Nil {
		return nil, ErrInvalidInput
	}
	limit, offset = paginate(limit, offset)
	transactions, err := l.store.ListWalletTransactions(ctx, db.ListWalletTransactionsParams{
		DriverID: driverID, Limit: limit, Offset: offset,
	})
	if err != nil {
		return nil, fmt.Errorf("list wallet transactions: %w", err)
	}
	return transactions, nil
}

func paginate(limit, offset int32) (int32, int32) {
	if limit < 1 || limit > 100 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

// numericToInt64 converts a scanned numeric (string/int64/float64) into an
// int64, treating nil as zero.
func numericToInt64(value interface{}) int64 {
	switch v := value.(type) {
	case nil:
		return 0
	case int64:
		return v
	case string:
		if parsed, err := strconv.ParseInt(v, 10, 64); err == nil {
			return parsed
		}
		if parsed, err := strconv.ParseFloat(v, 64); err == nil {
			return int64(math.Round(parsed))
		}
	case float64:
		return int64(math.Round(v))
	}
	return 0
}

// WalletJSON serializes a wallet for API responses.
func WalletJSON(wallet db.DriverWallet) map[string]interface{} {
	return map[string]interface{}{
		"driverId":         wallet.DriverID,
		"balanceCents":     wallet.BalanceCents,
		"pendingCents":     wallet.PendingCents,
		"totalEarnedCents": wallet.TotalEarnedCents,
		"totalPaidCents":   wallet.TotalPaidCents,
		"currency":         wallet.Currency,
		"lastPayoutAt":     wallet.LastPayoutAt,
		"updatedAt":        wallet.UpdatedAt,
	}
}

// WalletTransactionJSON serializes a wallet ledger entry for API responses.
func WalletTransactionJSON(tx db.WalletTransaction) map[string]interface{} {
	response := map[string]interface{}{
		"id":           tx.ID,
		"driverId":     tx.DriverID,
		"type":         tx.Type,
		"amountCents":  tx.AmountCents,
		"balanceAfter": tx.BalanceAfter,
		"status":       tx.Status,
		"createdAt":    tx.CreatedAt,
	}
	if tx.ReferenceID.Valid {
		response["referenceId"] = uuid.UUID(tx.ReferenceID.Bytes).String()
	}
	if tx.ReferenceType.Valid {
		response["referenceType"] = tx.ReferenceType.String
	}
	if tx.Description.Valid {
		response["description"] = tx.Description.String
	}
	return response
}
