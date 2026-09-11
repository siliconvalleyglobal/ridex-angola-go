// Package identity implements DB-backed identity verification: a user
// submits an ID document and a selfie, admins review and approve/reject.
// It replaces the earlier in-memory stub; every mutation persists to
// identity_verifications through the sqlc queries.
package identity

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/ridex/ridex-angola/internal/db"
)

var (
	ErrInvalidDocument = errors.New("id_document_type must be bi, passport, or drivers_license")
	ErrNotFound        = errors.New("identity verification not found")
	ErrNotPending      = errors.New("verification is not pending")
)

// allowedDocumentTypes matches the identity_verifications CHECK constraint.
var allowedDocumentTypes = map[string]bool{
	"bi": true, "passport": true, "drivers_license": true,
}

// Service persists identity verification workflows.
type Service struct {
	q *db.Queries
}

func NewService(q *db.Queries) *Service { return &Service{q: q} }

// Submit records a new pending verification (ID document + selfie).
func (s *Service) Submit(ctx context.Context, userID uuid.UUID, documentType, documentURL, selfieURL string) (db.IdentityVerification, error) {
	documentType = strings.ToLower(strings.TrimSpace(documentType))
	if !allowedDocumentTypes[documentType] {
		return db.IdentityVerification{}, ErrInvalidDocument
	}
	if strings.TrimSpace(documentURL) == "" || strings.TrimSpace(selfieURL) == "" {
		return db.IdentityVerification{}, ErrInvalidDocument
	}
	return s.q.CreateIdentityVerification(ctx, db.CreateIdentityVerificationParams{
		UserID:         userID,
		Status:         "pending",
		IDDocumentType: pgtype.Text{String: documentType, Valid: true},
		IDDocumentUrl:  pgtype.Text{String: strings.TrimSpace(documentURL), Valid: true},
		SelfieUrl:      pgtype.Text{String: strings.TrimSpace(selfieURL), Valid: true},
	})
}

// Mine returns the caller's latest verification, or ErrNotFound.
func (s *Service) Mine(ctx context.Context, userID uuid.UUID) (db.IdentityVerification, error) {
	verification, err := s.q.GetIdentityVerificationByUser(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return db.IdentityVerification{}, ErrNotFound
	}
	return verification, err
}

// Pending lists verifications awaiting review.
func (s *Service) Pending(ctx context.Context, limit, offset int32) ([]db.IdentityVerification, error) {
	return s.q.ListPendingVerifications(ctx, limit)
}

// Review approves or rejects a pending verification. Approved verifications
// are valid for one year.
func (s *Service) Review(ctx context.Context, verificationID, reviewerID uuid.UUID, approve bool, reason string, confidence float64) (db.IdentityVerification, error) {
	current, err := s.q.GetIdentityVerificationByID(ctx, verificationID)
	if errors.Is(err, pgx.ErrNoRows) {
		return db.IdentityVerification{}, ErrNotFound
	}
	if err != nil {
		return db.IdentityVerification{}, err
	}
	if current.Status != "pending" {
		return db.IdentityVerification{}, ErrNotPending
	}
	if approve {
		score := pgtype.Numeric{}
		if err := score.Scan(confidence); err != nil {
			return db.IdentityVerification{}, err
		}
		return s.q.ApproveIdentityVerification(ctx, db.ApproveIdentityVerificationParams{
			ID: verificationID, VerifiedBy: pgtype.UUID{Bytes: reviewerID, Valid: true},
			ConfidenceScore: score,
		})
	}
	if strings.TrimSpace(reason) == "" {
		return db.IdentityVerification{}, ErrInvalidDocument
	}
	return s.q.RejectIdentityVerification(ctx, db.RejectIdentityVerificationParams{
		ID: verificationID, RejectionReason: pgtype.Text{String: reason, Valid: true},
	})
}
