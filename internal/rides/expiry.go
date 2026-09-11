// Ride expiry: rides left in 'requested' with no driver after the TTL are
// closed by the background reaper so riders are never stuck waiting on a
// dead request.
package rides

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/ridex/ridex-angola/internal/db"
)

// DefaultRideExpiryTTL is how long an unaccepted ride request stays open
// before the reaper cancels it with reason 'expired'.
const DefaultRideExpiryTTL = 30 * time.Minute

// ExpireStaleRides cancels every 'requested' ride created before the cutoff
// and appends an 'expired' ride event for each, all inside one transaction
// per batch. Returns the number of rides expired.
func ExpireStaleRides(ctx context.Context, q *db.Queries, begin func(context.Context) (pgx.Tx, error), cutoff time.Time) (int, error) {
	return expireWithCutoff(ctx, q, begin, cutoff)
}

// ExpireStaleRidesWithTTL is the same reaper with an explicit TTL measured
// back from now.
func ExpireStaleRidesWithTTL(ctx context.Context, q *db.Queries, begin func(context.Context) (pgx.Tx, error), ttl time.Duration) (int, error) {
	return expireWithCutoff(ctx, q, begin, time.Now().Add(-ttl))
}

func expireWithCutoff(ctx context.Context, q *db.Queries, begin func(context.Context) (pgx.Tx, error), cutoff time.Time) (int, error) {
	if begin == nil {
		return expireBatch(ctx, q, cutoff)
	}
	var expired int
	tx, err := begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	expired, err = expireBatch(ctx, q.WithTx(tx), cutoff)
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return expired, nil
}

func expireBatch(ctx context.Context, q *db.Queries, cutoff time.Time) (int, error) {
	rows, err := q.ExpireStaleRequestedRides(ctx, pgtype.Timestamptz{Time: cutoff, Valid: true})
	if err != nil {
		return 0, err
	}
	for _, row := range rows {
		if err := recordRideEvent(q, ctx, row.ID, "expired", uuid.Nil, gin.H{
			"reason": "no driver accepted within the expiry window",
		}); err != nil {
			return 0, err
		}
	}
	return len(rows), nil
}
