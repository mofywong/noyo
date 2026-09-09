package workorder

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	AlarmStatusActive       = "active"
	AlarmStatusAcknowledged = "acknowledged"
	AlarmStatusCleared      = "cleared"

	AlarmEventOpened            = "opened"
	AlarmEventRepeated          = "repeated"
	AlarmEventAcknowledged      = "acknowledged"
	AlarmEventCleared           = "cleared"
	AlarmEventRecovered         = "condition_recovered"
	AlarmEventBound             = "work_order_bound"
	AlarmEventClosed            = "closed"
	AlarmEventShelved           = "shelved"
	AlarmEventUnshelved         = "unshelved"
	AlarmEventEscalated         = "escalated"
	AlarmEventAckSLAOverdue     = "acknowledgement_sla_overdue"
	AlarmEventResolveSLAOverdue = "resolution_sla_overdue"
)

// AlarmInstance is the durable domain representation of one occurrence
// generation. EvidenceSnapshot is immutable after Open and is intentionally
// kept when the alarm is acknowledged, cleared, or linked to a work order.
type AlarmInstance struct {
	ID                string         `json:"id"`
	TenantID          uint           `json:"tenant_id"`
	ProjectID         uint           `json:"project_id"`
	AlarmFingerprint  string         `json:"alarm_fingerprint"`
	Generation        int            `json:"generation"`
	Status            string         `json:"status"`
	ConditionStatus   string         `json:"condition_status"`
	HandlingStatus    string         `json:"handling_status"`
	Severity          string         `json:"severity"`
	Title             string         `json:"title"`
	SourceType        string         `json:"source_type"`
	SourceRef         string         `json:"source_ref"`
	CorrelationKey    string         `json:"correlation_key"`
	OwnerUserID       uint           `json:"owner_user_id"`
	OccurrenceCount   int            `json:"occurrence_count"`
	FirstOccurredAt   *time.Time     `json:"first_occurred_at,omitempty"`
	LastOccurredAt    *time.Time     `json:"last_occurred_at,omitempty"`
	RecoveredAt       *time.Time     `json:"recovered_at,omitempty"`
	VerificationAfter *time.Time     `json:"verification_after,omitempty"`
	ClosedAt          *time.Time     `json:"closed_at,omitempty"`
	ClosedBy          uint           `json:"closed_by"`
	CloseDisposition  string         `json:"close_disposition,omitempty"`
	NotificationMuted bool           `json:"notification_muted"`
	InhibitedBy       string         `json:"inhibited_by,omitempty"`
	EvidenceSnapshot        map[string]any `json:"evidence_snapshot"`
	ClearedEvidenceSnapshot map[string]any `json:"cleared_evidence_snapshot,omitempty"`
	WorkOrderPublicID       string         `json:"work_order_public_id,omitempty"`
	Version           int            `json:"version"`
	AcknowledgedAt    *time.Time     `json:"acknowledged_at,omitempty"`
	ClearedAt         *time.Time     `json:"cleared_at,omitempty"`
	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at"`
}

// AlarmEvent is append-only history. It never deletes or mutates the source
// evidence, which makes the clear command auditable.
type AlarmEvent struct {
	ID               string         `json:"id"`
	AlarmInstanceID  string         `json:"alarm_instance_id"`
	TenantID         uint           `json:"tenant_id"`
	ProjectID        uint           `json:"project_id"`
	Type             string         `json:"type"`
	Payload          map[string]any `json:"payload,omitempty"`
	EvidenceSnapshot map[string]any `json:"evidence_snapshot"`
	CreatedAt        time.Time      `json:"created_at"`
}

type AlarmWorkOrderBinding struct {
	WorkOrderPublicID string         `json:"work_order_public_id"`
	SourceSnapshot    map[string]any `json:"source_snapshot,omitempty"`
	CorrelationID     string         `json:"correlation_id,omitempty"`
	CausationID       string         `json:"causation_id,omitempty"`
}

// AlarmInstanceStore is a concurrency-safe domain store. The core service can
// replace it with a GORM-backed implementation without changing command
// semantics; tests and non-persistent adapters use this implementation.
type AlarmInstanceStore struct {
	mu         sync.RWMutex
	instances  map[string]*AlarmInstance
	byScopeKey map[string][]string
	events     map[string][]AlarmEvent
}

// AlarmService is retained as the domain-facing name used by callers.
type AlarmService = AlarmInstanceStore

func NewAlarmInstanceStore() *AlarmInstanceStore {
	return &AlarmInstanceStore{
		instances:  make(map[string]*AlarmInstance),
		byScopeKey: make(map[string][]string),
		events:     make(map[string][]AlarmEvent),
	}
}

func NewAlarmService() *AlarmInstanceStore { return NewAlarmInstanceStore() }

func (s *AlarmInstanceStore) Open(ctx context.Context, scope Scope, fingerprint string, snapshot map[string]any) (*AlarmInstance, error) {
	if err := validateAlarmContext(ctx, scope, fingerprint); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	key := alarmScopeKey(scope, fingerprint)
	for _, id := range s.byScopeKey[key] {
		instance := s.instances[id]
		if instance != nil && instance.Status != AlarmStatusCleared {
			return cloneAlarmInstance(instance), nil
		}
	}
	generation := 1
	for _, id := range s.byScopeKey[key] {
		if instance := s.instances[id]; instance != nil && instance.Generation >= generation {
			generation = instance.Generation + 1
		}
	}
	now := time.Now().UTC()
	instance := &AlarmInstance{
		ID: uuid.NewString(), TenantID: scope.TenantID, ProjectID: scope.ProjectID,
		AlarmFingerprint: strings.TrimSpace(fingerprint), Generation: generation,
		Status: AlarmStatusActive, EvidenceSnapshot: cloneAlarmMap(snapshot), Version: 1,
		CreatedAt: now, UpdatedAt: now,
	}
	s.instances[instance.ID] = instance
	s.byScopeKey[key] = append(s.byScopeKey[key], instance.ID)
	s.appendEventLocked(instance, AlarmEventOpened, nil)
	return cloneAlarmInstance(instance), nil
}

// Create is an alias for Open for command-oriented callers.
func (s *AlarmInstanceStore) Create(ctx context.Context, scope Scope, fingerprint string, snapshot map[string]any) (*AlarmInstance, error) {
	return s.Open(ctx, scope, fingerprint, snapshot)
}

func (s *AlarmInstanceStore) Get(scope Scope, id string) (*AlarmInstance, error) {
	if err := validateAlarmScope(scope); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	instance := s.instances[strings.TrimSpace(id)]
	if instance == nil || instance.TenantID != scope.TenantID || instance.ProjectID != scope.ProjectID {
		return nil, NewError(CodeWorkOrderNotFound, "alarm instance not found")
	}
	return cloneAlarmInstance(instance), nil
}

func (s *AlarmInstanceStore) GetActive(ctx context.Context, scope Scope, fingerprint string) (*AlarmInstance, error) {
	if err := validateAlarmContext(ctx, scope, fingerprint); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	ids := s.byScopeKey[alarmScopeKey(scope, fingerprint)]
	for i := len(ids) - 1; i >= 0; i-- {
		instance := s.instances[ids[i]]
		if instance != nil && instance.Status != AlarmStatusCleared {
			return cloneAlarmInstance(instance), nil
		}
	}
	return nil, NewError(CodeWorkOrderNotFound, "active alarm instance not found")
}

func (s *AlarmInstanceStore) Acknowledge(ctx context.Context, scope Scope, id string) error {
	return s.transition(ctx, scope, id, AlarmStatusAcknowledged, AlarmEventAcknowledged, nil)
}

func (s *AlarmInstanceStore) Clear(ctx context.Context, scope Scope, id string, payload map[string]any) error {
	return s.transition(ctx, scope, id, AlarmStatusCleared, AlarmEventCleared, payload)
}

func (s *AlarmInstanceStore) transition(ctx context.Context, scope Scope, id, target, eventType string, payload map[string]any) error {
	if err := validateAlarmContext(ctx, scope, id); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	instance := s.instances[strings.TrimSpace(id)]
	if instance == nil || instance.TenantID != scope.TenantID || instance.ProjectID != scope.ProjectID {
		return NewError(CodeWorkOrderNotFound, "alarm instance not found")
	}
	if instance.Status == target {
		return nil
	}
	if target == AlarmStatusAcknowledged && instance.Status != AlarmStatusActive {
		return NewError(CodeInvalidTransition, "only active alarms can be acknowledged")
	}
	if target == AlarmStatusCleared && instance.Status != AlarmStatusActive && instance.Status != AlarmStatusAcknowledged {
		return NewError(CodeInvalidTransition, "only active or acknowledged alarms can be cleared")
	}
	now := time.Now().UTC()
	instance.Status, instance.Version, instance.UpdatedAt = target, instance.Version+1, now
	if target == AlarmStatusAcknowledged {
		instance.AcknowledgedAt = &now
	} else {
		instance.ClearedAt = &now
		if len(payload) > 0 {
			instance.ClearedEvidenceSnapshot = cloneAlarmMap(payload)
		}
	}
	s.appendEventLocked(instance, eventType, payload)
	return nil
}

func (s *AlarmInstanceStore) BindWorkOrder(ctx context.Context, scope Scope, id string, binding AlarmWorkOrderBinding) (*AlarmInstance, error) {
	if err := validateAlarmContext(ctx, scope, id); err != nil {
		return nil, err
	}
	binding.WorkOrderPublicID = strings.TrimSpace(binding.WorkOrderPublicID)
	if binding.WorkOrderPublicID == "" {
		return nil, NewError(CodeValidationFailed, "work order public id is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	instance := s.instances[strings.TrimSpace(id)]
	if instance == nil || instance.TenantID != scope.TenantID || instance.ProjectID != scope.ProjectID {
		return nil, NewError(CodeWorkOrderNotFound, "alarm instance not found")
	}
	if instance.Status == AlarmStatusCleared {
		return nil, NewError(CodeInvalidTransition, "cleared alarm cannot be bound")
	}
	if instance.WorkOrderPublicID != "" && instance.WorkOrderPublicID != binding.WorkOrderPublicID {
		return nil, NewError(CodeIdempotencyConflict, "active alarm is already bound to another work order")
	}
	if instance.WorkOrderPublicID == "" {
		instance.WorkOrderPublicID = binding.WorkOrderPublicID
		instance.Version++
		instance.UpdatedAt = time.Now().UTC()
		s.appendEventLocked(instance, AlarmEventBound, map[string]any{
			"work_order_public_id": binding.WorkOrderPublicID,
			"source_snapshot":      cloneAlarmMap(binding.SourceSnapshot),
			"correlation_id":       binding.CorrelationID,
			"causation_id":         binding.CausationID,
		})
	}
	return cloneAlarmInstance(instance), nil
}

// ResolveForWorkOrder enforces that from-alarm accepts the current active
// generation unless the caller explicitly names a historical generation.
func (s *AlarmInstanceStore) ResolveForWorkOrder(ctx context.Context, scope Scope, fingerprint string, generation *int) (*AlarmInstance, error) {
	if err := validateAlarmContext(ctx, scope, fingerprint); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	ids := s.byScopeKey[alarmScopeKey(scope, fingerprint)]
	for i := len(ids) - 1; i >= 0; i-- {
		instance := s.instances[ids[i]]
		if instance == nil {
			continue
		}
		if generation == nil && instance.Status == AlarmStatusCleared {
			continue
		}
		if generation != nil && instance.Generation != *generation {
			continue
		}
		return cloneAlarmInstance(instance), nil
	}
	return nil, NewError(CodeWorkOrderNotFound, "requested alarm generation not found")
}

func (s *AlarmInstanceStore) History(scope Scope, id string) []AlarmEvent {
	if err := validateAlarmScope(scope); err != nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	events := s.events[strings.TrimSpace(id)]
	result := make([]AlarmEvent, 0, len(events))
	for _, event := range events {
		event.Payload = cloneAlarmMap(event.Payload)
		event.EvidenceSnapshot = cloneAlarmMap(event.EvidenceSnapshot)
		result = append(result, event)
	}
	return result
}

func (s *AlarmInstanceStore) appendEventLocked(instance *AlarmInstance, eventType string, payload map[string]any) {
	evidence := instance.EvidenceSnapshot
	if (eventType == AlarmEventCleared || eventType == AlarmEventRecovered) && len(instance.ClearedEvidenceSnapshot) > 0 {
		evidence = instance.ClearedEvidenceSnapshot
	}
	s.events[instance.ID] = append(s.events[instance.ID], AlarmEvent{
		ID: uuid.NewString(), AlarmInstanceID: instance.ID, TenantID: instance.TenantID,
		ProjectID: instance.ProjectID, Type: eventType, Payload: cloneAlarmMap(payload),
		EvidenceSnapshot: cloneAlarmMap(evidence), CreatedAt: time.Now().UTC(),
	})
}

func validateAlarmContext(ctx context.Context, scope Scope, value string) error {
	if ctx == nil {
		return NewError(CodeValidationFailed, "context is required")
	}
	if err := validateAlarmScope(scope); err != nil {
		return err
	}
	if strings.TrimSpace(value) == "" {
		return NewError(CodeValidationFailed, "alarm fingerprint or id is required")
	}
	return nil
}

func validateAlarmScope(scope Scope) error {
	if scope.TenantID == 0 || scope.ProjectID == 0 {
		return NewError(CodeValidationFailed, "tenant and project scope are required")
	}
	return nil
}

func alarmScopeKey(scope Scope, fingerprint string) string {
	return fmt.Sprintf("%d:%d:%s", scope.TenantID, scope.ProjectID, strings.TrimSpace(fingerprint))
}

func cloneAlarmInstance(instance *AlarmInstance) *AlarmInstance {
	if instance == nil {
		return nil
	}
	copy := *instance
	copy.EvidenceSnapshot = cloneAlarmMap(instance.EvidenceSnapshot)
	copy.ClearedEvidenceSnapshot = cloneAlarmMap(instance.ClearedEvidenceSnapshot)
	return &copy
}

func cloneAlarmMap(value map[string]any) map[string]any {
	if value == nil {
		return nil
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return map[string]any{}
	}
	var copy map[string]any
	if json.Unmarshal(encoded, &copy) != nil {
		return map[string]any{}
	}
	return copy
}
