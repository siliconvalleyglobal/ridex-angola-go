package identity

import (
	"time"

	"github.com/google/uuid"
)

// VerificationStatus represents the status of identity verification.
type VerificationStatus string

const (
	VerificationPending  VerificationStatus = "pending"
	VerificationVerified VerificationStatus = "verified"
	VerificationRejected VerificationStatus = "rejected"
	VerificationExpired  VerificationStatus = "expired"
)

// IdentityVerification represents a user's identity verification record.
type IdentityVerification struct {
	ID              uuid.UUID
	UserID          uuid.UUID
	Status          VerificationStatus
	IDDocumentType  string // bi, passport, drivers_license
	IDDocumentURL   string
	SelfieURL       string
	VerifiedAt      *time.Time
	VerifiedBy      *uuid.UUID
	ConfidenceScore float64
	RejectionReason string
	ExpiresAt       *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// DriverBadge represents a verification or achievement badge.
type DriverBadge struct {
	ID          uuid.UUID
	DriverID    uuid.UUID
	BadgeType   string // verified, top_rated, safety_champion, million_miles
	DisplayName string
	Description string
	EarnedAt    time.Time
	ValidUntil  *time.Time
	Active      bool
}

// VerificationRequest represents a new verification submission.
type VerificationRequest struct {
	UserID         uuid.UUID
	IDDocumentType string
	IDDocumentURL  string
	SelfieURL      string
}

// VerificationService handles identity verification operations.
type VerificationService struct{}

// NewVerificationService creates a new verification service.
func NewVerificationService() *VerificationService {
	return &VerificationService{}
}

// SubmitVerification submits documents for verification.
func (s *VerificationService) SubmitVerification(req VerificationRequest) (*IdentityVerification, error) {
	return &IdentityVerification{
		ID:             uuid.New(),
		UserID:         req.UserID,
		Status:         VerificationPending,
		IDDocumentType: req.IDDocumentType,
		IDDocumentURL:  req.IDDocumentURL,
		SelfieURL:      req.SelfieURL,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}, nil
}

// ApproveVerification approves a verification request.
func (s *VerificationService) ApproveVerification(verificationID, verifierID uuid.UUID, confidence float64) (*IdentityVerification, error) {
	now := time.Now()
	return &IdentityVerification{
		ID:              verificationID,
		Status:          VerificationVerified,
		VerifiedBy:      &verifierID,
		VerifiedAt:      &now,
		ConfidenceScore: confidence,
		ExpiresAt:       ptr(now.AddDate(1, 0, 0)), // 1 year validity
		UpdatedAt:       now,
	}, nil
}

// RejectVerification rejects a verification request.
func (s *VerificationService) RejectVerification(verificationID uuid.UUID, reason string) (*IdentityVerification, error) {
	now := time.Now()
	return &IdentityVerification{
		ID:              verificationID,
		Status:          VerificationRejected,
		RejectionReason: reason,
		UpdatedAt:       now,
	}, nil
}

// IsVerified checks if a user has valid verification.
func IsVerified(verification *IdentityVerification) bool {
	if verification == nil {
		return false
	}
	if verification.Status != VerificationVerified {
		return false
	}
	if verification.ExpiresAt != nil && time.Now().After(*verification.ExpiresAt) {
		return false
	}
	return true
}

// GetBadgeTypes returns available badge types.
func GetBadgeTypes() []string {
	return []string{
		"verified",
		"top_rated",
		"safety_champion",
		"million_miles",
		"early_adopter",
		"community_helper",
	}
}

func ptr[T any](v T) *T {
	return &v
}
