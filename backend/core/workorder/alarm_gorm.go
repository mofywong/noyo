package workorder

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"noyo/core/store"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// AlarmInstanceService is the persistence boundary used by core command
// handlers. The in-memory implementation remains useful for isolated tests,
// while production uses GormAlarmInstanceStore.
type AlarmInstanceService interface {
	Open(context.Context, Scope, string, map[string]any) (*AlarmInstance, error)
	Create(context.Context, Scope, string, map[string]any) (*AlarmInstance, error)
	Get(Scope, string) (*AlarmInstance, error)
	GetActive(context.Context, Scope, string) (*AlarmInstance, error)
	Acknowledge(context.Context, Scope, string) error
	Clear(context.Context, Scope, string, map[string]any) error
	BindWorkOrder(context.Context, Scope, string, AlarmWorkOrderBinding) (*AlarmInstance, error)
	ResolveForWorkOrder(context.Context, Scope, string, *int) (*AlarmInstance, error)
	History(Scope, string) []AlarmEvent
}

// GormAlarmInstanceStore persists alarm generations, immutable evidence, and
// work-order bindings in the same database as the work-order service.
type GormAlarmInstanceStore struct {
	db *gorm.DB
}

func NewGormAlarmInstanceStore(db *gorm.DB) *GormAlarmInstanceStore {
	return &GormAlarmInstanceStore{db: db}
}

func (s *GormAlarmInstanceStore) Open(ctx context.Context, scope Scope, fingerprint string, snapshot map[string]any) (*AlarmInstance, error) {
	if err := validateAlarmContext(ctx, scope, fingerprint); err != nil {
		return nil, err
	}
	fingerprint = strings.TrimSpace(fingerprint)
	evidence, err := encodeAlarmMap(snapshot)
	if err != nil {
		return nil, err
	}
	var result *AlarmInstance
	err = s.db.Transaction(func(tx *gorm.DB) error {
		var active store.AlarmInstance
		err := tx.Where("tenant_id = ? AND project_id = ? AND alarm_fingerprint = ? AND status <> ? AND handling_status <> ?", scope.TenantID, scope.ProjectID, fingerprint, AlarmStatusCleared, "closed").
			Order("generation DESC").First(&active).Error
		if err == nil {
			now := time.Now().UTC()
			updates := map[string]any{
				"condition_status": AlarmStatusActive,
				"last_occurred_at": &now,
				"occurrence_count": gorm.Expr("CASE WHEN occurrence_count < 1 THEN 2 ELSE occurrence_count + 1 END"),
			}
			if active.HandlingStatus == "" {
				updates["handling_status"] = "new"
			}
			if err := tx.Model(&store.AlarmInstance{}).Where("id = ?", active.ID).Updates(updates).Error; err != nil {
				return fmt.Errorf("record repeated alarm occurrence: %w", err)
			}
			active.ConditionStatus = AlarmStatusActive
			active.LastOccurredAt = &now
			if active.OccurrenceCount < 1 {
				active.OccurrenceCount = 2
			} else {
				active.OccurrenceCount++
			}
			if err := appendStoredAlarmEvent(tx, active, AlarmEventRepeated, map[string]any{"occurred_at": now.Format(time.RFC3339Nano), "payload": snapshot}); err != nil {
				return err
			}
			decoded, err := decodeStoredAlarmInstance(active)
			if err != nil {
				return err
			}
			result = decoded
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("load active alarm instance: %w", err)
		}
		generation := 1
		var latest store.AlarmInstance
		if err := tx.Where("tenant_id = ? AND project_id = ? AND alarm_fingerprint = ?", scope.TenantID, scope.ProjectID, fingerprint).
			Order("generation DESC").First(&latest).Error; err == nil {
			generation = latest.Generation + 1
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("load latest alarm generation: %w", err)
		}
		now := time.Now().UTC()
		instance := store.AlarmInstance{
			PublicID: uuid.NewString(), TenantID: scope.TenantID, ProjectID: scope.ProjectID,
			AlarmFingerprint: fingerprint, Generation: generation, Status: AlarmStatusActive,
			ConditionStatus: AlarmStatusActive, HandlingStatus: "new", Severity: "warning",
			OccurrenceCount: 1, FirstOccurredAt: &now, LastOccurredAt: &now,
			EvidenceSnapshot: evidence, Version: 1,
		}
		if err := tx.Create(&instance).Error; err != nil {
			return fmt.Errorf("create alarm instance: %w", err)
		}
		if err := appendStoredAlarmEvent(tx, instance, AlarmEventOpened, nil); err != nil {
			return err
		}
		decoded, err := decodeStoredAlarmInstance(instance)
		if err != nil {
			return err
		}
		result = decoded
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *GormAlarmInstanceStore) Create(ctx context.Context, scope Scope, fingerprint string, snapshot map[string]any) (*AlarmInstance, error) {
	return s.Open(ctx, scope, fingerprint, snapshot)
}

func (s *GormAlarmInstanceStore) Get(scope Scope, publicID string) (*AlarmInstance, error) {
	if err := validateAlarmContext(context.Background(), scope, publicID); err != nil {
		return nil, err
	}
	instance, err := s.find(scope, publicID)
	if err != nil {
		return nil, err
	}
	return decodeStoredAlarmInstance(*instance)
}

func (s *GormAlarmInstanceStore) GetActive(ctx context.Context, scope Scope, fingerprint string) (*AlarmInstance, error) {
	if err := validateAlarmContext(ctx, scope, fingerprint); err != nil {
		return nil, err
	}
	var instance store.AlarmInstance
	err := s.db.Where("tenant_id = ? AND project_id = ? AND alarm_fingerprint = ? AND status <> ? AND handling_status <> ?", scope.TenantID, scope.ProjectID, strings.TrimSpace(fingerprint), AlarmStatusCleared, "closed").
		Order("generation DESC").First(&instance).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, NewError(CodeWorkOrderNotFound, "active alarm instance not found")
	}
	if err != nil {
		return nil, fmt.Errorf("load active alarm instance: %w", err)
	}
	return decodeStoredAlarmInstance(instance)
}

func (s *GormAlarmInstanceStore) Acknowledge(ctx context.Context, scope Scope, publicID string) error {
	return s.transition(ctx, scope, publicID, AlarmStatusAcknowledged, AlarmEventAcknowledged, nil)
}

func (s *GormAlarmInstanceStore) Clear(ctx context.Context, scope Scope, publicID string, payload map[string]any) error {
	return s.transition(ctx, scope, publicID, AlarmStatusCleared, AlarmEventCleared, payload)
}

func (s *GormAlarmInstanceStore) transition(ctx context.Context, scope Scope, publicID, target, eventType string, payload map[string]any) error {
	if err := validateAlarmContext(ctx, scope, publicID); err != nil {
		return err
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		instance, err := findStoredAlarmInstance(tx, scope, publicID)
		if err != nil {
			return err
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
		updates := map[string]any{"status": target, "version": instance.Version + 1}
		if target == AlarmStatusAcknowledged {
			updates["acknowledged_at"] = &now
			updates["handling_status"] = "acknowledged"
		} else {
			updates["cleared_at"] = &now
			updates["condition_status"] = "recovered"
			updates["recovered_at"] = &now
			updates["handling_status"] = gorm.Expr("CASE WHEN work_order_public_id <> '' THEN 'pending_verification' WHEN handling_status = 'closed' THEN 'closed' ELSE 'acknowledged' END")
			if len(payload) > 0 {
				clearedEvidence, err := encodeAlarmMap(payload)
				if err == nil {
					updates["cleared_evidence_snapshot"] = clearedEvidence
					instance.ClearedEvidenceSnapshot = clearedEvidence
				}
			}
		}
		query := tx.Model(&store.AlarmInstance{}).Where("id = ? AND version = ?", instance.ID, instance.Version).Updates(updates)
		if query.Error != nil {
			return fmt.Errorf("update alarm instance: %w", query.Error)
		}
		if query.RowsAffected != 1 {
			return NewError(CodeIdempotencyConflict, "alarm instance changed concurrently")
		}
		instance.Status = target
		instance.Version++
		if target == AlarmStatusAcknowledged {
			instance.AcknowledgedAt = &now
			instance.HandlingStatus = "acknowledged"
		} else {
			instance.ClearedAt = &now
			instance.ConditionStatus = "recovered"
			instance.RecoveredAt = &now
		}
		return appendStoredAlarmEvent(tx, *instance, eventType, payload)
	})
}

func (s *GormAlarmInstanceStore) BindWorkOrder(ctx context.Context, scope Scope, publicID string, binding AlarmWorkOrderBinding) (*AlarmInstance, error) {
	if err := validateAlarmContext(ctx, scope, publicID); err != nil {
		return nil, err
	}
	binding.WorkOrderPublicID = strings.TrimSpace(binding.WorkOrderPublicID)
	if binding.WorkOrderPublicID == "" {
		return nil, NewError(CodeValidationFailed, "work order public id is required")
	}
	snapshot, err := encodeAlarmMap(binding.SourceSnapshot)
	if err != nil {
		return nil, err
	}
	var result *AlarmInstance
	err = s.db.Transaction(func(tx *gorm.DB) error {
		instance, err := findStoredAlarmInstance(tx, scope, publicID)
		if err != nil {
			return err
		}
		if instance.Status == AlarmStatusCleared {
			return NewError(CodeInvalidTransition, "cleared alarm cannot be bound")
		}
		if instance.WorkOrderPublicID != "" && instance.WorkOrderPublicID != binding.WorkOrderPublicID {
			return NewError(CodeIdempotencyConflict, "active alarm is already bound to another work order")
		}
		if instance.WorkOrderPublicID == "" {
			query := tx.Model(&store.AlarmInstance{}).Where("id = ? AND version = ?", instance.ID, instance.Version).
				Updates(map[string]any{"work_order_public_id": binding.WorkOrderPublicID, "handling_status": "in_progress", "version": instance.Version + 1})
			if query.Error != nil {
				return fmt.Errorf("bind work order to alarm: %w", query.Error)
			}
			if query.RowsAffected != 1 {
				return NewError(CodeIdempotencyConflict, "alarm instance changed concurrently")
			}
			storedBinding := store.AlarmInstanceWorkOrderBinding{
				TenantID: scope.TenantID, ProjectID: scope.ProjectID, AlarmInstanceID: instance.ID,
				WorkOrderPublicID: binding.WorkOrderPublicID, SourceSnapshot: snapshot,
				CorrelationID: strings.TrimSpace(binding.CorrelationID), CausationID: strings.TrimSpace(binding.CausationID),
			}
			if err := tx.Create(&storedBinding).Error; err != nil {
				return fmt.Errorf("create alarm work order binding: %w", err)
			}
			instance.WorkOrderPublicID = binding.WorkOrderPublicID
			instance.HandlingStatus = "in_progress"
			instance.Version++
			if err := appendStoredAlarmEvent(tx, *instance, AlarmEventBound, map[string]any{
				"work_order_public_id": binding.WorkOrderPublicID,
				"source_snapshot":      binding.SourceSnapshot,
				"correlation_id":       binding.CorrelationID,
				"causation_id":         binding.CausationID,
			}); err != nil {
				return err
			}
		}
		decoded, err := decodeStoredAlarmInstance(*instance)
		if err != nil {
			return err
		}
		result = decoded
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *GormAlarmInstanceStore) ResolveForWorkOrder(ctx context.Context, scope Scope, fingerprint string, generation *int) (*AlarmInstance, error) {
	if err := validateAlarmContext(ctx, scope, fingerprint); err != nil {
		return nil, err
	}
	query := s.db.Where("tenant_id = ? AND project_id = ? AND alarm_fingerprint = ?", scope.TenantID, scope.ProjectID, strings.TrimSpace(fingerprint))
	if generation == nil {
		query = query.Where("status <> ?", AlarmStatusCleared)
	} else {
		query = query.Where("generation = ?", *generation)
	}
	var instance store.AlarmInstance
	err := query.Order("generation DESC").First(&instance).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, NewError(CodeWorkOrderNotFound, "requested alarm generation not found")
	}
	if err != nil {
		return nil, fmt.Errorf("resolve alarm instance for work order: %w", err)
	}
	return decodeStoredAlarmInstance(instance)
}

func (s *GormAlarmInstanceStore) History(scope Scope, publicID string) []AlarmEvent {
	if err := validateAlarmScope(scope); err != nil {
		return nil
	}
	instance, err := s.find(scope, publicID)
	if err != nil {
		return nil
	}
	var records []store.AlarmInstanceEvent
	if err := s.db.Where("tenant_id = ? AND project_id = ? AND alarm_instance_id = ?", scope.TenantID, scope.ProjectID, instance.ID).
		Order("id ASC").Find(&records).Error; err != nil {
		return nil
	}
	events := make([]AlarmEvent, 0, len(records))
	for _, record := range records {
		payload, payloadErr := decodeAlarmMap(record.Payload)
		evidence, evidenceErr := decodeAlarmMap(record.EvidenceSnapshot)
		if payloadErr != nil || evidenceErr != nil {
			return nil
		}
		events = append(events, AlarmEvent{
			ID: fmt.Sprintf("%d", record.ID), AlarmInstanceID: publicID, TenantID: record.TenantID, ProjectID: record.ProjectID,
			Type: record.Type, Payload: payload, EvidenceSnapshot: evidence, CreatedAt: record.CreatedAt,
		})
	}
	return events
}

func (s *GormAlarmInstanceStore) find(scope Scope, publicID string) (*store.AlarmInstance, error) {
	instance, err := findStoredAlarmInstance(s.db, scope, publicID)
	if err != nil {
		return nil, err
	}
	return instance, nil
}

func findStoredAlarmInstance(db *gorm.DB, scope Scope, publicID string) (*store.AlarmInstance, error) {
	var instance store.AlarmInstance
	err := db.Where("tenant_id = ? AND project_id = ? AND public_id = ?", scope.TenantID, scope.ProjectID, strings.TrimSpace(publicID)).First(&instance).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, NewError(CodeWorkOrderNotFound, "alarm instance not found")
	}
	if err != nil {
		return nil, fmt.Errorf("load alarm instance: %w", err)
	}
	return &instance, nil
}

func appendStoredAlarmEvent(tx *gorm.DB, instance store.AlarmInstance, eventType string, payload map[string]any) error {
	encodedPayload, err := encodeAlarmMap(payload)
	if err != nil {
		return err
	}
	evidenceSnapshot := instance.EvidenceSnapshot
	if (eventType == AlarmEventCleared || eventType == AlarmEventRecovered) && strings.TrimSpace(instance.ClearedEvidenceSnapshot) != "" {
		evidenceSnapshot = instance.ClearedEvidenceSnapshot
	}
	record := store.AlarmInstanceEvent{
		TenantID: instance.TenantID, ProjectID: instance.ProjectID, AlarmInstanceID: instance.ID,
		Type: eventType, Payload: encodedPayload, EvidenceSnapshot: evidenceSnapshot,
	}
	if err := tx.Create(&record).Error; err != nil {
		return fmt.Errorf("append alarm event: %w", err)
	}
	return nil
}

func decodeStoredAlarmInstance(instance store.AlarmInstance) (*AlarmInstance, error) {
	evidence, err := decodeAlarmMap(instance.EvidenceSnapshot)
	if err != nil {
		return nil, err
	}
	clearedEvidence, err := decodeAlarmMap(instance.ClearedEvidenceSnapshot)
	if err != nil {
		return nil, err
	}
	return &AlarmInstance{
		ID: instance.PublicID, TenantID: instance.TenantID, ProjectID: instance.ProjectID,
		AlarmFingerprint: instance.AlarmFingerprint, Generation: instance.Generation, Status: instance.Status,
		ConditionStatus: instance.ConditionStatus, HandlingStatus: instance.HandlingStatus, Severity: instance.Severity,
		Title: instance.Title, SourceType: instance.SourceType, SourceRef: instance.SourceRef,
		CorrelationKey: instance.CorrelationKey, OwnerUserID: instance.OwnerUserID, OccurrenceCount: instance.OccurrenceCount,
		FirstOccurredAt: instance.FirstOccurredAt, LastOccurredAt: instance.LastOccurredAt, RecoveredAt: instance.RecoveredAt,
		VerificationAfter: instance.VerificationAfter, ClosedAt: instance.ClosedAt, ClosedBy: instance.ClosedBy,
		CloseDisposition: instance.CloseDisposition, NotificationMuted: instance.NotificationMuted, InhibitedBy: instance.InhibitedBy,
		EvidenceSnapshot: evidence, ClearedEvidenceSnapshot: clearedEvidence, WorkOrderPublicID: instance.WorkOrderPublicID, Version: instance.Version,
		AcknowledgedAt: instance.AcknowledgedAt, ClearedAt: instance.ClearedAt,
		CreatedAt: instance.CreatedAt, UpdatedAt: instance.UpdatedAt,
	}, nil
}

func encodeAlarmMap(value map[string]any) (string, error) {
	if value == nil {
		value = map[string]any{}
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", NewError(CodeValidationFailed, "alarm payload must be JSON serializable")
	}
	return string(encoded), nil
}

func decodeAlarmMap(value string) (map[string]any, error) {
	result := make(map[string]any)
	if strings.TrimSpace(value) == "" {
		return result, nil
	}
	if err := json.Unmarshal([]byte(value), &result); err != nil {
		return nil, fmt.Errorf("decode stored alarm JSON: %w", err)
	}
	return result, nil
}
