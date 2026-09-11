package alerts

import (
	"time"

	"github.com/google/uuid"
)

// AlertSeverity represents the severity of an alert.
type AlertSeverity string

const (
	AlertSeverityInfo     AlertSeverity = "info"
	AlertSeverityWarning  AlertSeverity = "warning"
	AlertSeverityCritical AlertSeverity = "critical"
)

// AlertRule defines conditions that trigger alerts.
type AlertRule struct {
	ID            uuid.UUID
	Name          string
	Description   string
	Condition     string
	Threshold     float64
	Severity      AlertSeverity
	Action        string
	Recipients    []string
	Active        bool
	CreatedAt     time.Time
	LastTriggered *time.Time
	CooldownMin   int
}

// AlertEvent represents a triggered alert.
type AlertEvent struct {
	ID             uuid.UUID
	RuleID         uuid.UUID
	Severity       AlertSeverity
	Title          string
	Message        string
	Data           map[string]string
	Acknowledged   bool
	AcknowledgedBy *uuid.UUID
	AcknowledgedAt *time.Time
	CreatedAt      time.Time
}

// AlertService manages alert rules and events.
type AlertService struct{}

// NewAlertService creates a new alert service.
func NewAlertService() *AlertService {
	return &AlertService{}
}

// CreateRule creates a new alert rule.
func (s *AlertService) CreateRule(name, description, condition string, threshold float64, severity AlertSeverity, action string, recipients []string) *AlertRule {
	return &AlertRule{
		ID:          uuid.New(),
		Name:        name,
		Description: description,
		Condition:   condition,
		Threshold:   threshold,
		Severity:    severity,
		Action:      action,
		Recipients:  recipients,
		Active:      true,
		CreatedAt:   time.Now(),
		CooldownMin: 15,
	}
}

// EvaluateRule checks if an alert rule should trigger.
func (s *AlertService) EvaluateRule(rule *AlertRule, currentValue float64) *AlertEvent {
	if !rule.Active {
		return nil
	}

	// Check cooldown
	if rule.LastTriggered != nil {
		elapsed := time.Since(*rule.LastTriggered)
		if elapsed.Minutes() < float64(rule.CooldownMin) {
			return nil
		}
	}

	var shouldTrigger bool
	switch rule.Condition {
	case "greater_than":
		shouldTrigger = currentValue > rule.Threshold
	case "less_than":
		shouldTrigger = currentValue < rule.Threshold
	case "equals":
		shouldTrigger = currentValue == rule.Threshold
	default:
		shouldTrigger = false
	}

	if !shouldTrigger {
		return nil
	}

	now := time.Now()
	rule.LastTriggered = &now

	return &AlertEvent{
		ID:        uuid.New(),
		RuleID:    rule.ID,
		Severity:  rule.Severity,
		Title:     rule.Name,
		Message:   rule.Description,
		Data:      map[string]string{"value": formatFloat(currentValue), "threshold": formatFloat(rule.Threshold)},
		CreatedAt: now,
	}
}

// AcknowledgeAlert marks an alert as acknowledged.
func (s *AlertService) AcknowledgeAlert(alertID, userID uuid.UUID) *AlertEvent {
	now := time.Now()
	return &AlertEvent{
		ID:             alertID,
		Acknowledged:   true,
		AcknowledgedBy: &userID,
		AcknowledgedAt: &now,
	}
}

// Common alert rule conditions
const (
	ConditionWaitTimeHigh     = "wait_time_high"
	ConditionDemandSurge      = "demand_surge"
	ConditionDriverShortage   = "driver_shortage"
	ConditionSystemDown       = "system_down"
	ConditionHighCancellation = "high_cancellation"
)

// DefaultAlertRules returns standard alert rules.
func DefaultAlertRules() []*AlertRule {
	service := NewAlertService()
	return []*AlertRule{
		service.CreateRule(
			"High Wait Time",
			"Average wait time exceeds threshold",
			"greater_than",
			300, // 5 minutes
			AlertSeverityWarning,
			"notify_admin",
			[]string{"admin@ridex.ao"},
		),
		service.CreateRule(
			"Driver Shortage",
			"Not enough drivers available",
			"less_than",
			5,
			AlertSeverityCritical,
			"notify_admin_sms",
			[]string{"admin@ridex.ao", "+244900000000"},
		),
		service.CreateRule(
			"High Cancellation Rate",
			"Cancellation rate exceeds threshold",
			"greater_than",
			20, // 20%
			AlertSeverityWarning,
			"notify_admin",
			[]string{"admin@ridex.ao"},
		),
		service.CreateRule(
			"Demand Surge",
			"Demand surge active in zone",
			"greater_than",
			2.0,
			AlertSeverityInfo,
			"notify_drivers",
			[]string{"drivers"},
		),
	}
}

func formatFloat(f float64) string {
	return "0" // placeholder
}
