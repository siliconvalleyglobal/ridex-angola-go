package db

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

const confirmCashPayment = `INSERT INTO cash_payments
(ride_id, driver_id, amount_due, amount_paid, change_cents)
SELECT $1, $2, r.suggested_fare_cents, $3, $3 - r.suggested_fare_cents
FROM rides r
WHERE r.id = $1 AND r.driver_id = $2 AND r.status = 'completed' AND r.payment_method = 'cash'
ON CONFLICT (ride_id) DO NOTHING
RETURNING ride_id, driver_id, amount_due, amount_paid, change_cents, confirmed_at`

func (q *Queries) ConfirmCashPayment(ctx context.Context, rideID, driverID uuid.UUID, amountPaid pgtype.Numeric) (CashPayment, error) {
	row := q.db.QueryRow(ctx, confirmCashPayment, rideID, driverID, amountPaid)
	var item CashPayment
	err := row.Scan(&item.RideID, &item.DriverID, &item.AmountDue, &item.AmountPaid, &item.ChangeCents, &item.ConfirmedAt)
	return item, err
}

const getCashPayment = `SELECT ride_id, driver_id, amount_due, amount_paid, change_cents, confirmed_at
FROM cash_payments WHERE ride_id = $1`

func (q *Queries) GetCashPayment(ctx context.Context, rideID uuid.UUID) (CashPayment, error) {
	row := q.db.QueryRow(ctx, getCashPayment, rideID)
	var item CashPayment
	err := row.Scan(&item.RideID, &item.DriverID, &item.AmountDue, &item.AmountPaid, &item.ChangeCents, &item.ConfirmedAt)
	return item, err
}
