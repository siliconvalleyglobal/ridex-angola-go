// Package notifications provides provider-neutral, in-app notification
// persistence. It deliberately has no push, SMS, or other external delivery.
package notifications

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/ridex/ridex-angola/internal/db"
)

// Store is the database surface needed by the in-process notification service.
type Store interface {
	CreateNotification(context.Context, db.CreateNotificationParams) (db.Notification, error)
	ListUnreadNotifications(context.Context, db.ListUnreadNotificationsParams) ([]db.Notification, error)
	ListReadNotifications(context.Context, db.ListReadNotificationsParams) ([]db.Notification, error)
	MarkNotificationRead(context.Context, db.MarkNotificationReadParams) (db.Notification, error)
	GetNotificationPreferences(context.Context, uuid.UUID) (db.NotificationPreference, error)
	UpsertNotificationPreferences(context.Context, db.UpsertNotificationPreferencesParams) (db.NotificationPreference, error)
	DeleteNotificationPreferences(context.Context, uuid.UUID) error
}

// Service is the provider-neutral notification boundary used by handlers and
// domain code. Implementations may later enqueue external delivery, but this
// implementation only persists locally in PostgreSQL.
type Service interface {
	Notify(context.Context, Input) (*db.Notification, error)
	List(context.Context, uuid.UUID, bool, int32, int32) ([]db.Notification, error)
	MarkRead(context.Context, uuid.UUID, uuid.UUID) (db.Notification, error)
	Preferences(context.Context, uuid.UUID) (db.NotificationPreference, error)
	SavePreferences(context.Context, PreferencesInput) (db.NotificationPreference, error)
	DeletePreferences(context.Context, uuid.UUID) error
}

// Input describes an in-app notification before persistence.
type Input struct {
	UserID uuid.UUID
	Type   string
	Title  string
	Body   string
	Data   json.RawMessage
}

// PreferencesInput contains a user's in-app notification settings.
type PreferencesInput struct {
	UserID         uuid.UUID
	RideUpdates    bool
	PaymentUpdates bool
	KycUpdates     bool
	Marketing      bool
}

// Pusher is the optional external push-delivery boundary. When wired, the
// service fans each persisted notification out to every registered device.
// Delivery failures are logged and never fail the persistence path.
type Pusher interface {
	SendPush(ctx context.Context, deviceToken string, notification PushNotification) error
}

// TokenStore lists the registered push devices for a user.
type TokenStore interface {
	ListDeviceTokensByUser(context.Context, uuid.UUID) ([]db.DeviceToken, error)
}

// InProcessService persists notifications without making external delivery
// calls. It is safe to share between HTTP handlers and domain services.
type InProcessService struct {
	store   Store
	pusher  Pusher
	tokens  TokenStore
}

func NewService(store Store) *InProcessService {
	return &InProcessService{store: store}
}

// WithPush wires external push delivery: every successfully persisted
// notification is also fanned out to the user's registered devices.
func (s *InProcessService) WithPush(push Pusher, tokens TokenStore) *InProcessService {
	s.pusher = push
	s.tokens = tokens
	return s
}

func (s *InProcessService) Notify(ctx context.Context, input Input) (*db.Notification, error) {
	if input.UserID == uuid.Nil || strings.TrimSpace(input.Type) == "" ||
		strings.TrimSpace(input.Title) == "" || strings.TrimSpace(input.Body) == "" {
		return nil, errors.New("notification user, type, title, and body are required")
	}
	prefs, err := s.Preferences(ctx, input.UserID)
	if err != nil {
		return nil, err
	}
	if !preferenceEnabled(prefs, input.Type) {
		return nil, nil
	}
	data := []byte(input.Data)
	if len(data) == 0 {
		data = []byte(`{}`)
	}
	notification, err := s.store.CreateNotification(ctx, db.CreateNotificationParams{
		UserID: input.UserID,
		Type:   strings.TrimSpace(input.Type),
		Title:  strings.TrimSpace(input.Title),
		Body:   input.Body,
		Data:   data,
	})
	if err != nil {
		return nil, err
	}
	s.fanOutPush(ctx, input, notification)
	return &notification, nil
}

// fanOutPush best-effort delivers a persisted notification to every
// registered device. It is deliberately resilient: push outages never block
// or fail the in-app notification write.
func (s *InProcessService) fanOutPush(ctx context.Context, input Input, notification db.Notification) {
	if s.pusher == nil || s.tokens == nil {
		return
	}
	tokens, err := s.tokens.ListDeviceTokensByUser(ctx, input.UserID)
	if err != nil || len(tokens) == 0 {
		return
	}
	data := map[string]string{"type": notification.Type, "notificationId": notification.ID.String()}
	payload := PushNotification{
		Title: notification.Title, Body: notification.Body,
		Data: data, Priority: "normal",
	}
	for _, token := range tokens {
		// Best effort per device; a single bad token does not block others.
		_ = s.pusher.SendPush(ctx, token.Token, payload)
	}
}

func (s *InProcessService) List(ctx context.Context, userID uuid.UUID, unread bool, limit, offset int32) ([]db.Notification, error) {
	if unread {
		return s.store.ListUnreadNotifications(ctx, db.ListUnreadNotificationsParams{UserID: userID, Limit: limit, Offset: offset})
	}
	return s.store.ListReadNotifications(ctx, db.ListReadNotificationsParams{UserID: userID, Limit: limit, Offset: offset})
}

func (s *InProcessService) MarkRead(ctx context.Context, userID, notificationID uuid.UUID) (db.Notification, error) {
	return s.store.MarkNotificationRead(ctx, db.MarkNotificationReadParams{ID: notificationID, UserID: userID})
}

func (s *InProcessService) Preferences(ctx context.Context, userID uuid.UUID) (db.NotificationPreference, error) {
	prefs, err := s.store.GetNotificationPreferences(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return defaultPreferences(userID), nil
	}
	return prefs, err
}

func (s *InProcessService) SavePreferences(ctx context.Context, input PreferencesInput) (db.NotificationPreference, error) {
	return s.store.UpsertNotificationPreferences(ctx, db.UpsertNotificationPreferencesParams{
		UserID: input.UserID, RideUpdates: input.RideUpdates,
		PaymentUpdates: input.PaymentUpdates, KycUpdates: input.KycUpdates,
		Marketing: input.Marketing,
	})
}

func (s *InProcessService) DeletePreferences(ctx context.Context, userID uuid.UUID) error {
	return s.store.DeleteNotificationPreferences(ctx, userID)
}

func defaultPreferences(userID uuid.UUID) db.NotificationPreference {
	return db.NotificationPreference{
		UserID: userID, RideUpdates: true, PaymentUpdates: true,
		KycUpdates: true, Marketing: false,
	}
}

func preferenceEnabled(prefs db.NotificationPreference, notificationType string) bool {
	switch strings.ToLower(strings.TrimSpace(notificationType)) {
	case "ride", "ride_update", "ride_offer", "ride_status":
		return prefs.RideUpdates
	case "payment", "payment_update", "payment_status", "invoice":
		return prefs.PaymentUpdates
	case "kyc", "kyc_update", "kyc_status":
		return prefs.KycUpdates
	case "marketing", "promotion":
		return prefs.Marketing
	default:
		return true
	}
}

// NotificationJSON is the stable API representation of a persisted
// notification, avoiding pgtype-specific JSON output.
type NotificationJSON struct {
	ID        uuid.UUID       `json:"id"`
	Type      string          `json:"type"`
	Title     string          `json:"title"`
	Body      string          `json:"body"`
	Data      json.RawMessage `json:"data"`
	ReadAt    *time.Time      `json:"readAt,omitempty"`
	CreatedAt time.Time       `json:"createdAt"`
}

func ToJSON(notification db.Notification) NotificationJSON {
	data := json.RawMessage(notification.Data)
	if len(data) == 0 {
		data = json.RawMessage(`{}`)
	}
	var readAt *time.Time
	if notification.ReadAt.Valid {
		t := notification.ReadAt.Time
		readAt = &t
	}
	return NotificationJSON{
		ID: notification.ID, Type: notification.Type, Title: notification.Title,
		Body: notification.Body, Data: data, ReadAt: readAt,
		CreatedAt: notification.CreatedAt.Time,
	}
}
