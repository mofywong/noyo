package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"noyo/core/store"
	"noyo/core/workorder"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	AlarmConditionFiring    = "firing"
	AlarmConditionRecovered = "recovered"
	AlarmConditionUnknown   = "unknown"

	AlarmHandlingNew                 = "new"
	AlarmHandlingAcknowledged        = "acknowledged"
	AlarmHandlingInProgress          = "in_progress"
	AlarmHandlingPendingVerification = "pending_verification"
	AlarmHandlingShelved             = "shelved"
	AlarmHandlingClosed              = "closed"

	AlarmCloseAutoRecovered = "auto_recovered"
	AlarmCloseWorkOrder     = "work_order_closed"

	AlarmPolicyMatchEvents = "events"
	AlarmPolicyMatchLevels = "levels"
	AlarmPolicyMatchAll    = "all"
)

var (
	ErrAlarmNotFound = errors.New("alarm instance not found")
	ErrAlarmConflict = errors.New("alarm instance was modified, refresh and retry")
	ErrAlarmInvalid  = errors.New("invalid alarm operation")
)

// AlarmCenterService owns the operational state around the immutable alarm
// occurrence stored by workorder.AlarmInstanceService. Device/rule sources keep
// using their existing entry points; this service enriches the resulting
// instance with policy, handling, suppression and SLA information.
type AlarmCenterService struct {
	db             *gorm.DB
	instances      workorder.AlarmInstanceService
	workOrders     *WorkOrderService
	workOrderPorts workorder.CommandPort
}

func NewAlarmCenterService(db *gorm.DB, instances workorder.AlarmInstanceService, workOrders *WorkOrderService, workOrderPorts workorder.CommandPort) *AlarmCenterService {
	return &AlarmCenterService{db: db, instances: instances, workOrders: workOrders, workOrderPorts: workOrderPorts}
}

type AlarmSignalMetadata struct {
	SourceType     string
	SourceRef      string
	ProductCode    string
	EventLevel     string
	CorrelationKey string
	Severity       string
	Title          string
	OccurredAt     time.Time
	Payload        map[string]any
}

type AlarmListOptions struct {
	Page          int
	PageSize      int
	Condition     string
	Handling      string
	Severity      string
	SourceType    string
	WorkOrderID   string
	OwnerUserID   uint
	Search        string
	IncludeClosed bool
	ClosedToday   bool
}

type AlarmStats struct {
	Total                 int64 `json:"total"`
	Firing                int64 `json:"firing"`
	Unacknowledged        int64 `json:"unacknowledged"`
	InProgress            int64 `json:"in_progress"`
	PendingVerification   int64 `json:"pending_verification"`
	Shelved               int64 `json:"shelved"`
	ClosedToday           int64 `json:"closed_today"`
	OverdueAcknowledgment int64 `json:"overdue_acknowledgment"`
}

func (s *AlarmCenterService) List(scope workorder.Scope, options AlarmListOptions) ([]store.AlarmInstance, int64, error) {
	if err := validateAlarmCenterScope(scope); err != nil {
		return nil, 0, err
	}
	if options.Page < 1 {
		options.Page = 1
	}
	if options.PageSize < 1 {
		options.PageSize = 10
	}
	if options.PageSize > 200 {
		options.PageSize = 200
	}
	query := s.db.Where("tenant_id = ? AND project_id = ?", scope.TenantID, scope.ProjectID)
	if !options.IncludeClosed && !options.ClosedToday {
		query = query.Where("handling_status <> ?", AlarmHandlingClosed)
	}
	if options.ClosedToday {
		localNow := time.Now().In(time.Local)
		startOfDay := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, localNow.Location()).UTC()
		query = query.Where("handling_status = ? AND closed_at >= ?", AlarmHandlingClosed, startOfDay)
	}
	if value := strings.TrimSpace(options.Condition); value != "" {
		query = query.Where("condition_status = ?", value)
	}
	if value := strings.TrimSpace(options.Handling); value != "" {
		query = query.Where("handling_status = ?", value)
	}
	if value := strings.TrimSpace(options.Severity); value != "" {
		query = query.Where("severity = ?", value)
	}
	if value := strings.TrimSpace(options.SourceType); value != "" {
		query = query.Where("source_type = ?", value)
	}
	if value := strings.TrimSpace(options.WorkOrderID); value != "" {
		query = query.Where("work_order_public_id = ?", value)
	}
	if options.OwnerUserID > 0 {
		query = query.Where("owner_user_id = ?", options.OwnerUserID)
	}
	if value := strings.TrimSpace(options.Search); value != "" {
		pattern := "%" + strings.ToLower(value) + "%"
		query = query.Where("LOWER(title) LIKE ? OR LOWER(alarm_fingerprint) LIKE ? OR LOWER(source_ref) LIKE ?", pattern, pattern, pattern)
	}
	var total int64
	if err := query.Model(&store.AlarmInstance{}).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count alarm instances: %w", err)
	}
	var records []store.AlarmInstance
	if err := query.Order("CASE severity WHEN 'critical' THEN 0 WHEN 'major' THEN 1 WHEN 'warning' THEN 2 ELSE 3 END").Order("last_occurred_at DESC, id DESC").
		Offset((options.Page - 1) * options.PageSize).Limit(options.PageSize).Find(&records).Error; err != nil {
		return nil, 0, fmt.Errorf("list alarm instances: %w", err)
	}
	return records, total, nil
}

func (s *AlarmCenterService) Get(scope workorder.Scope, publicID string) (*store.AlarmInstance, error) {
	if err := validateAlarmCenterScope(scope); err != nil {
		return nil, err
	}
	var instance store.AlarmInstance
	if err := s.db.Where("tenant_id = ? AND project_id = ? AND public_id = ?", scope.TenantID, scope.ProjectID, strings.TrimSpace(publicID)).First(&instance).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrAlarmNotFound
		}
		return nil, fmt.Errorf("load alarm instance: %w", err)
	}
	return &instance, nil
}

func (s *AlarmCenterService) Events(scope workorder.Scope, publicID string) ([]store.AlarmInstanceEvent, error) {
	instance, err := s.Get(scope, publicID)
	if err != nil {
		return nil, err
	}
	var events []store.AlarmInstanceEvent
	if err := s.db.Where("tenant_id = ? AND project_id = ? AND alarm_instance_id = ?", scope.TenantID, scope.ProjectID, instance.ID).Order("id ASC").Find(&events).Error; err != nil {
		return nil, fmt.Errorf("list alarm events: %w", err)
	}
	return events, nil
}

func (s *AlarmCenterService) Stats(scope workorder.Scope) (AlarmStats, error) {
	if err := validateAlarmCenterScope(scope); err != nil {
		return AlarmStats{}, err
	}
	base := s.db.Model(&store.AlarmInstance{}).Where("tenant_id = ? AND project_id = ?", scope.TenantID, scope.ProjectID)
	stats := AlarmStats{}
	if err := base.Session(&gorm.Session{}).Select(`
		COUNT(*) AS total,
		COALESCE(SUM(CASE WHEN condition_status = ? THEN 1 ELSE 0 END), 0) AS firing,
		COALESCE(SUM(CASE WHEN condition_status = ? AND handling_status = ? THEN 1 ELSE 0 END), 0) AS unacknowledged,
		COALESCE(SUM(CASE WHEN handling_status = ? THEN 1 ELSE 0 END), 0) AS in_progress,
		COALESCE(SUM(CASE WHEN handling_status = ? THEN 1 ELSE 0 END), 0) AS pending_verification,
		COALESCE(SUM(CASE WHEN handling_status = ? THEN 1 ELSE 0 END), 0) AS shelved`,
		AlarmConditionFiring,
		AlarmConditionFiring, AlarmHandlingNew,
		AlarmHandlingInProgress,
		AlarmHandlingPendingVerification,
		AlarmHandlingShelved,
	).Scan(&stats).Error; err != nil {
		return AlarmStats{}, fmt.Errorf("count alarm statistics: %w", err)
	}
	localNow := time.Now().In(time.Local)
	startOfDay := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, localNow.Location()).UTC()
	if err := base.Session(&gorm.Session{}).Where("handling_status = ? AND closed_at >= ?", AlarmHandlingClosed, startOfDay).Count(&stats.ClosedToday).Error; err != nil {
		return AlarmStats{}, fmt.Errorf("count today closed alarms: %w", err)
	}
	var unacknowledged []store.AlarmInstance
	if err := base.Session(&gorm.Session{}).Where("condition_status = ? AND handling_status = ? AND first_occurred_at IS NOT NULL", AlarmConditionFiring, AlarmHandlingNew).Find(&unacknowledged).Error; err != nil {
		return AlarmStats{}, fmt.Errorf("load acknowledgement SLA candidates: %w", err)
	}
	now := time.Now().UTC()
	for _, instance := range unacknowledged {
		policy := policyFromSnapshot(instance.PolicySnapshot)
		if instance.FirstOccurredAt != nil && policy.AcknowledgeSLASeconds > 0 && !now.Before(instance.FirstOccurredAt.Add(time.Duration(policy.AcknowledgeSLASeconds)*time.Second)) {
			stats.OverdueAcknowledgment++
		}
	}
	return stats, nil
}

func (s *AlarmCenterService) Observe(ctx context.Context, scope workorder.Scope, publicID string, metadata AlarmSignalMetadata) error {
	if err := validateAlarmCenterScope(scope); err != nil {
		return err
	}
	if metadata.OccurredAt.IsZero() {
		metadata.OccurredAt = time.Now().UTC()
	}
	policy, err := s.matchPolicyForSignal(scope, metadata)
	if err != nil {
		return err
	}
	severity := metadata.Severity
	if policy.ID > 0 && strings.TrimSpace(policy.Severity) != "" {
		severity = policy.Severity
	}
	err = s.db.Transaction(func(tx *gorm.DB) error {
		instance, err := findAlarmInstance(tx, scope, publicID)
		if err != nil {
			return err
		}
		updates := map[string]any{
			"condition_status":   AlarmConditionFiring,
			"last_occurred_at":   &metadata.OccurredAt,
			"severity":           normalizedAlarmSeverity(severity),
			"source_type":        strings.TrimSpace(metadata.SourceType),
			"source_ref":         strings.TrimSpace(metadata.SourceRef),
			"correlation_key":    strings.TrimSpace(metadata.CorrelationKey),
			"title":              strings.TrimSpace(metadata.Title),
			"recovered_at":       nil,
			"verification_after": nil,
		}
		if instance.FirstOccurredAt == nil {
			updates["first_occurred_at"] = &metadata.OccurredAt
		}
		if instance.HandlingStatus == "" || instance.HandlingStatus == AlarmHandlingClosed {
			updates["handling_status"] = AlarmHandlingNew
		}
		encoded, marshalErr := json.Marshal(policySnapshot(policy))
		if marshalErr != nil {
			return marshalErr
		}
		updates["policy_snapshot"] = string(encoded)
		muted, inhibitedBy, suppressionErr := s.suppressionFor(tx, scope, instance.PublicID, instance.AlarmFingerprint, metadata.OccurredAt)
		if suppressionErr != nil {
			return suppressionErr
		}
		updates["notification_muted"] = muted
		updates["inhibited_by"] = inhibitedBy
		if muted && instance.HandlingStatus != AlarmHandlingClosed {
			updates["handling_status"] = AlarmHandlingShelved
		} else if instance.HandlingStatus == AlarmHandlingShelved {
			updates["handling_status"] = restoreAlarmHandling(instance, policy)
		}
		if err := updateAlarmInstance(tx, instance, updates); err != nil {
			return fmt.Errorf("enrich alarm instance: %w", err)
		}
		return appendAlarmCenterEvent(tx, instance, "signal_observed", map[string]any{"source_type": metadata.SourceType, "source_ref": metadata.SourceRef, "occurred_at": metadata.OccurredAt, "muted": muted, "payload": metadata.Payload})
	})
	if err != nil {
		return err
	}
	if policy.RequireWorkOrder && policy.AutoCreateWorkOrder {
		if err := s.autoCreateWorkOrder(ctx, scope, publicID, policy); err != nil {
			_ = s.recordServiceEvent(scope, publicID, "work_order_auto_create_failed", map[string]any{"error": err.Error()})
		}
	}
	return nil
}

func (s *AlarmCenterService) Recover(scope workorder.Scope, publicID string, payload map[string]any) error {
	if err := validateAlarmCenterScope(scope); err != nil {
		return err
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		instance, err := findAlarmInstance(tx, scope, publicID)
		if err != nil {
			return err
		}
		if normalizedCondition(instance.ConditionStatus) == AlarmConditionRecovered {
			return nil
		}
		now := time.Now().UTC()
		policy := policyFromSnapshot(instance.PolicySnapshot)
		var verificationAfter *time.Time
		if policy.RecoveryHoldSeconds > 0 {
			verificationAt := now.Add(time.Duration(policy.RecoveryHoldSeconds) * time.Second)
			verificationAfter = &verificationAt
		}
		handling := restoreRecoveredHandling(instance, policy, verificationAfter)
		updates := map[string]any{"condition_status": AlarmConditionRecovered, "recovered_at": &now, "verification_after": verificationAfter, "handling_status": handling}
		if err := updateAlarmInstance(tx, instance, updates); err != nil {
			return fmt.Errorf("mark alarm recovered: %w", err)
		}
		return appendAlarmCenterEvent(tx, instance, workorder.AlarmEventRecovered, map[string]any{"recovered_at": now, "payload": payload})
	})
}

func (s *AlarmCenterService) Acknowledge(scope workorder.Scope, publicID string, actorUserID uint, comment string) error {
	return s.updateHandling(scope, publicID, actorUserID, AlarmHandlingAcknowledged, workorder.AlarmEventAcknowledged, comment, nil)
}

func (s *AlarmCenterService) Assign(scope workorder.Scope, publicID string, actorUserID, ownerUserID uint, comment string) error {
	if ownerUserID == 0 {
		return fmt.Errorf("assignee is required")
	}
	return s.updateHandling(scope, publicID, actorUserID, AlarmHandlingInProgress, "assigned", comment, map[string]any{"owner_user_id": ownerUserID})
}

func (s *AlarmCenterService) Shelve(scope workorder.Scope, publicID string, actorUserID uint, until *time.Time, reason string, sourceAlarmID string) (*store.AlarmSuppression, error) {
	if strings.TrimSpace(reason) == "" {
		return nil, fmt.Errorf("shelve reason is required")
	}
	var created store.AlarmSuppression
	err := s.db.Transaction(func(tx *gorm.DB) error {
		instance, err := findAlarmInstance(tx, scope, publicID)
		if err != nil {
			return err
		}
		now := time.Now().UTC()
		if instance.HandlingStatus == AlarmHandlingClosed {
			return fmt.Errorf("%w: closed alarm cannot be shelved", ErrAlarmInvalid)
		}
		if until != nil && !until.After(now) {
			return fmt.Errorf("%w: shelve expiry must be in the future", ErrAlarmInvalid)
		}
		created = store.AlarmSuppression{PublicID: uuid.NewString(), TenantID: scope.TenantID, ProjectID: scope.ProjectID, Type: suppressionType(sourceAlarmID), TargetAlarmInstanceID: instance.PublicID, TargetFingerprint: instance.AlarmFingerprint, SourceAlarmInstanceID: strings.TrimSpace(sourceAlarmID), Reason: strings.TrimSpace(reason), CreatedBy: actorUserID, PreviousHandlingStatus: instance.HandlingStatus, StartsAt: now, EndsAt: until}
		if err := tx.Create(&created).Error; err != nil {
			return fmt.Errorf("create alarm suppression: %w", err)
		}
		if err := updateAlarmInstance(tx, instance, map[string]any{"notification_muted": true, "inhibited_by": strings.TrimSpace(sourceAlarmID), "handling_status": AlarmHandlingShelved}); err != nil {
			return fmt.Errorf("shelve alarm instance: %w", err)
		}
		return appendAlarmCenterEvent(tx, instance, workorder.AlarmEventShelved, map[string]any{"reason": reason, "until": until, "source_alarm_instance_id": sourceAlarmID, "actor_user_id": actorUserID})
	})
	if err != nil {
		return nil, err
	}
	return &created, nil
}

func (s *AlarmCenterService) Unshelve(scope workorder.Scope, publicID string, actorUserID uint, comment string) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		instance, err := findAlarmInstance(tx, scope, publicID)
		if err != nil {
			return err
		}
		now := time.Now().UTC()
		if instance.HandlingStatus == AlarmHandlingClosed {
			return fmt.Errorf("%w: closed alarm cannot be unshelved", ErrAlarmInvalid)
		}
		var suppressions []store.AlarmSuppression
		if err := tx.Where("tenant_id = ? AND project_id = ? AND target_alarm_instance_id = ? AND released_at IS NULL", scope.TenantID, scope.ProjectID, instance.PublicID).Order("id DESC").Find(&suppressions).Error; err != nil {
			return fmt.Errorf("load active alarm suppressions: %w", err)
		}
		if err := tx.Model(&store.AlarmSuppression{}).Where("tenant_id = ? AND project_id = ? AND target_alarm_instance_id = ? AND released_at IS NULL", scope.TenantID, scope.ProjectID, instance.PublicID).Update("released_at", &now).Error; err != nil {
			return fmt.Errorf("release alarm suppression: %w", err)
		}
		previous := ""
		if len(suppressions) > 0 {
			previous = suppressions[0].PreviousHandlingStatus
		}
		nextHandling := restoreAlarmHandlingWithPrevious(instance, policyFromSnapshot(instance.PolicySnapshot), previous)
		if err := updateAlarmInstance(tx, instance, map[string]any{"notification_muted": false, "inhibited_by": "", "handling_status": nextHandling}); err != nil {
			return fmt.Errorf("unshelve alarm instance: %w", err)
		}
		return appendAlarmCenterEvent(tx, instance, workorder.AlarmEventUnshelved, map[string]any{"comment": comment, "actor_user_id": actorUserID})
	})
}

func (s *AlarmCenterService) Close(scope workorder.Scope, publicID string, actorUserID uint, disposition, comment string) error {
	if strings.TrimSpace(disposition) == "" {
		return fmt.Errorf("close disposition is required")
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		instance, err := findAlarmInstance(tx, scope, publicID)
		if err != nil {
			return err
		}
		if instance.HandlingStatus == AlarmHandlingClosed {
			return nil
		}
		policy := policyFromSnapshot(instance.PolicySnapshot)
		exception := isAlarmExceptionDisposition(disposition)
		if normalizedCondition(instance.ConditionStatus) != AlarmConditionRecovered && !exception {
			return fmt.Errorf("%w: active alarm condition cannot be closed", ErrAlarmInvalid)
		}
		if exception && !policy.AllowExceptionClose {
			return fmt.Errorf("%w: the matched alarm policy does not allow exception closing", ErrAlarmInvalid)
		}
		if policy.RequireWorkOrder && !exception && strings.TrimSpace(instance.WorkOrderPublicID) == "" {
			return fmt.Errorf("this alarm requires a work order before closing")
		}
		if policy.RequireWorkOrder && !exception {
			var order store.WorkOrder
			if err := tx.Where("tenant_id = ? AND project_id = ? AND public_id = ?", scope.TenantID, scope.ProjectID, instance.WorkOrderPublicID).First(&order).Error; err != nil {
				return fmt.Errorf("load linked work order before closing alarm: %w", err)
			}
			if order.ClosedAt == nil {
				return fmt.Errorf("%w: the linked work order must be closed before closing this alarm", ErrAlarmInvalid)
			}
			workflow, err := decodeWorkOrderWorkflowDefinition(order.WorkflowSnapshot)
			if err != nil {
				return fmt.Errorf("decode linked work order workflow: %w", err)
			}
			status, ok := workflow.statusByKey(order.Status)
			if !ok || status.Category != WorkOrderStatusClosed {
				return fmt.Errorf("%w: the linked work order must be completed rather than cancelled before closing this alarm", ErrAlarmInvalid)
			}
		}
		if instance.VerificationAfter != nil && time.Now().UTC().Before(*instance.VerificationAfter) && !exception {
			return fmt.Errorf("%w: alarm recovery is still in the verification window", ErrAlarmInvalid)
		}
		now := time.Now().UTC()
		updates := map[string]any{
			"handling_status":   AlarmHandlingClosed,
			"closed_at":         &now,
			"closed_by":         actorUserID,
			"close_disposition": strings.TrimSpace(disposition),
			// A manually exception-closed firing instance must be terminal for the
			// legacy store too, otherwise the next signal reuses stale close fields.
			"status": workorder.AlarmStatusCleared,
		}
		if err := updateAlarmInstance(tx, instance, updates); err != nil {
			return fmt.Errorf("close alarm instance: %w", err)
		}
		return appendAlarmCenterEvent(tx, instance, workorder.AlarmEventClosed, map[string]any{"disposition": disposition, "comment": comment, "actor_user_id": actorUserID})
	})
}

func (s *AlarmCenterService) ReconcileDue(now time.Time) error {
	if err := s.releaseExpiredSuppressions(now); err != nil {
		return err
	}
	for afterID := uint(0); ; {
		var candidates []store.AlarmInstance
		if err := s.db.Where("handling_status <> ? AND id > ?", AlarmHandlingClosed, afterID).
			Order("id ASC").Limit(200).Find(&candidates).Error; err != nil {
			return fmt.Errorf("find alarm reconciliation candidates: %w", err)
		}
		if len(candidates) == 0 {
			break
		}
		for _, instance := range candidates {
			if strings.TrimSpace(instance.WorkOrderPublicID) != "" {
				var order store.WorkOrder
				err := s.db.Where("tenant_id = ? AND project_id = ? AND public_id = ?", instance.TenantID, instance.ProjectID, instance.WorkOrderPublicID).First(&order).Error
				if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
					return fmt.Errorf("load linked work order during alarm reconciliation: %w", err)
				}
				if err == nil && (order.Status == WorkOrderStatusResolved || order.Status == WorkOrderStatusClosed) {
					if err := s.db.Transaction(func(tx *gorm.DB) error {
						return closeLinkedAlarmsForWorkOrder(tx, &order, 0, "")
					}); err != nil {
						return fmt.Errorf("reconcile alarm from completed work order: %w", err)
					}
					continue
				}
			}
			policy := policyFromSnapshot(instance.PolicySnapshot)
			if normalizedCondition(instance.ConditionStatus) == AlarmConditionFiring {
				if instance.HandlingStatus == AlarmHandlingNew && instance.FirstOccurredAt != nil && policy.AcknowledgeSLASeconds > 0 && !now.Before(instance.FirstOccurredAt.Add(time.Duration(policy.AcknowledgeSLASeconds)*time.Second)) {
					if err := s.recordEscalation(instance, workorder.AlarmEventAckSLAOverdue, now); err != nil {
						return err
					}
				}
				if instance.FirstOccurredAt != nil && policy.ResolutionSLASeconds > 0 && !now.Before(instance.FirstOccurredAt.Add(time.Duration(policy.ResolutionSLASeconds)*time.Second)) {
					if err := s.recordEscalation(instance, workorder.AlarmEventResolveSLAOverdue, now); err != nil {
						return err
					}
				}
				if instance.FirstOccurredAt != nil && policy.EscalateAfterSeconds > 0 && !now.Before(instance.FirstOccurredAt.Add(time.Duration(policy.EscalateAfterSeconds)*time.Second)) {
					if err := s.recordEscalation(instance, workorder.AlarmEventEscalated, now); err != nil {
						return err
					}
				}
				continue
			}
			if normalizedCondition(instance.ConditionStatus) != AlarmConditionRecovered || !policy.AutoClose || policy.RequireWorkOrder || instance.WorkOrderPublicID != "" || (instance.VerificationAfter != nil && now.Before(*instance.VerificationAfter)) {
				continue
			}
			scope := workorder.Scope{TenantID: instance.TenantID, ProjectID: instance.ProjectID, Principal: workorder.Principal{Type: "system", Ref: "alarm-auto-close"}}
			if err := s.Close(scope, instance.PublicID, 0, AlarmCloseAutoRecovered, "recovery verification passed"); err != nil {
				return err
			}
		}
		afterID = candidates[len(candidates)-1].ID
	}
	return nil
}

func (s *AlarmCenterService) recordEscalation(instance store.AlarmInstance, eventType string, now time.Time) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		var locked store.AlarmInstance
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND handling_status <> ?", instance.ID, AlarmHandlingClosed).First(&locked).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return fmt.Errorf("lock alarm escalation: %w", err)
		}
		var existing int64
		if err := tx.Model(&store.AlarmInstanceEvent{}).Where("alarm_instance_id = ? AND type = ?", locked.ID, eventType).Count(&existing).Error; err != nil {
			return fmt.Errorf("check existing alarm escalation: %w", err)
		}
		if existing > 0 {
			return nil
		}
		if err := updateAlarmInstance(tx, &locked, map[string]any{"updated_at": now}); err != nil {
			return fmt.Errorf("mark alarm escalation: %w", err)
		}
		return appendAlarmCenterEvent(tx, &locked, eventType, map[string]any{"occurred_at": now, "policy": policyFromSnapshot(locked.PolicySnapshot)})
	})
}

func (s *AlarmCenterService) RunAutoCloseLoop() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		_ = s.ReconcileDue(time.Now().UTC())
	}
}

func (s *AlarmCenterService) ListPolicies(scope workorder.Scope) ([]store.AlarmPolicy, error) {
	if err := validateAlarmCenterScope(scope); err != nil {
		return nil, err
	}
	var policies []store.AlarmPolicy
	if err := s.db.Where("tenant_id = ? AND project_id = ?", scope.TenantID, scope.ProjectID).Order("priority DESC, id ASC").Find(&policies).Error; err != nil {
		return nil, fmt.Errorf("list alarm policies: %w", err)
	}
	return policies, nil
}

func (s *AlarmCenterService) SavePolicy(scope workorder.Scope, input store.AlarmPolicy) (*store.AlarmPolicy, error) {
	if err := validateAlarmCenterScope(scope); err != nil {
		return nil, err
	}
	input.Code = strings.TrimSpace(input.Code)
	input.Name = strings.TrimSpace(input.Name)
	if input.Code == "" || input.Name == "" {
		return nil, fmt.Errorf("policy code and name are required")
	}
	input.SourceType = strings.TrimSpace(input.SourceType)
	input.SourceRef = strings.TrimSpace(input.SourceRef)
	input.SourceRefs = normalizeAlarmPolicySourceRefs(input.SourceRefs)
	if len(input.SourceRefs) == 0 && input.SourceRef != "" && input.SourceRef != "*" {
		input.SourceRefs = []string{input.SourceRef}
	}
	input.EventLevels = normalizeAlarmPolicyEventLevels(input.EventLevels)
	input.MatchMode = normalizedAlarmPolicyMatchMode(input.MatchMode, input.SourceRefs, input.EventLevels)
	if input.MatchMode == "" {
		return nil, fmt.Errorf("%w: alarm policy match mode must be events, levels, or all", ErrAlarmInvalid)
	}
	switch input.MatchMode {
	case AlarmPolicyMatchAll:
		input.SourceRef = ""
		input.SourceRefs = nil
		input.EventLevels = nil
	case AlarmPolicyMatchLevels:
		if len(input.EventLevels) == 0 {
			return nil, fmt.Errorf("%w: at least one product-model event level is required", ErrAlarmInvalid)
		}
		input.SourceRef = ""
		input.SourceRefs = nil
	case AlarmPolicyMatchEvents:
		if len(input.SourceRefs) == 0 {
			return nil, fmt.Errorf("%w: at least one alarm policy event is required", ErrAlarmInvalid)
		}
		input.EventLevels = nil
	}
	for _, sourceRef := range input.SourceRefs {
		if len(sourceRef) > 191 {
			return nil, fmt.Errorf("%w: alarm policy event identifiers cannot exceed 191 characters", ErrAlarmInvalid)
		}
	}
	if len(input.SourceRefs) > 0 {
		input.SourceRef = input.SourceRefs[0]
	}
	input.TenantID, input.ProjectID = scope.TenantID, scope.ProjectID
	input.Severity = normalizedAlarmSeverity(input.Severity)
	if input.Priority < 0 || input.RecoveryHoldSeconds < 0 || input.AcknowledgeSLASeconds < 0 || input.ResolutionSLASeconds < 0 || input.EscalateAfterSeconds < 0 {
		return nil, fmt.Errorf("%w: alarm policy priority and durations cannot be negative", ErrAlarmInvalid)
	}
	if input.AutoCreateWorkOrder && (!input.RequireWorkOrder || input.WorkOrderTemplateID == 0) {
		return nil, fmt.Errorf("%w: automatic work-order creation requires a work-order policy and template", ErrAlarmInvalid)
	}
	encodedSourceRefs, err := json.Marshal(input.SourceRefs)
	if err != nil {
		return nil, fmt.Errorf("encode alarm policy event identifiers: %w", err)
	}
	encodedEventLevels, err := json.Marshal(input.EventLevels)
	if err != nil {
		return nil, fmt.Errorf("encode alarm policy event levels: %w", err)
	}
	var saved store.AlarmPolicy
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if input.ID == 0 {
			if err := tx.Create(&input).Error; err != nil {
				return fmt.Errorf("create alarm policy: %w", err)
			}
			saved = input
			return nil
		}
		var existing store.AlarmPolicy
		if err := tx.Where("id = ? AND tenant_id = ? AND project_id = ?", input.ID, scope.TenantID, scope.ProjectID).First(&existing).Error; err != nil {
			return fmt.Errorf("alarm policy not found")
		}
		updates := map[string]any{
			"code":                    input.Code,
			"name":                    input.Name,
			"name_en":                 input.NameEN,
			"source_type":             input.SourceType,
			"source_ref":              input.SourceRef,
			"match_mode":              input.MatchMode,
			"source_refs":             string(encodedSourceRefs),
			"event_levels":            string(encodedEventLevels),
			"priority":                input.Priority,
			"severity":                input.Severity,
			"require_work_order":      input.RequireWorkOrder,
			"auto_create_work_order":  input.AutoCreateWorkOrder,
			"work_order_template_id":  input.WorkOrderTemplateID,
			"auto_close":              input.AutoClose,
			"recovery_hold_seconds":   input.RecoveryHoldSeconds,
			"acknowledge_sla_seconds": input.AcknowledgeSLASeconds,
			"resolution_sla_seconds":  input.ResolutionSLASeconds,
			"escalate_after_seconds":  input.EscalateAfterSeconds,
			"allow_exception_close":   input.AllowExceptionClose,
			"enabled":                 input.Enabled,
		}
		if err := tx.Model(&existing).Updates(updates).Error; err != nil {
			return fmt.Errorf("update alarm policy: %w", err)
		}
		if err := tx.Where("id = ?", existing.ID).First(&saved).Error; err != nil {
			return fmt.Errorf("reload alarm policy: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &saved, nil
}

func (s *AlarmCenterService) DeletePolicy(scope workorder.Scope, policyID uint) error {
	if err := validateAlarmCenterScope(scope); err != nil {
		return err
	}
	if policyID == 0 {
		return fmt.Errorf("%w: alarm policy id is required", ErrAlarmInvalid)
	}
	result := s.db.Where("id = ? AND tenant_id = ? AND project_id = ?", policyID, scope.TenantID, scope.ProjectID).Delete(&store.AlarmPolicy{})
	if result.Error != nil {
		return fmt.Errorf("delete alarm policy: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("%w: alarm policy not found", ErrAlarmNotFound)
	}
	return nil
}

func (s *AlarmCenterService) updateHandling(scope workorder.Scope, publicID string, actorUserID uint, target, eventType, comment string, extras map[string]any) error {
	if err := validateAlarmCenterScope(scope); err != nil {
		return err
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		instance, err := findAlarmInstance(tx, scope, publicID)
		if err != nil {
			return err
		}
		if instance.HandlingStatus == AlarmHandlingClosed {
			return fmt.Errorf("%w: closed alarm cannot be changed", ErrAlarmInvalid)
		}
		updates := map[string]any{"handling_status": target}
		if target == AlarmHandlingAcknowledged {
			now := time.Now().UTC()
			updates["acknowledged_at"] = &now
			if instance.Status == workorder.AlarmStatusActive {
				updates["status"] = workorder.AlarmStatusAcknowledged
			}
		}
		for key, value := range extras {
			updates[key] = value
		}
		if err := updateAlarmInstance(tx, instance, updates); err != nil {
			return fmt.Errorf("update alarm handling status: %w", err)
		}
		payload := map[string]any{"comment": comment, "actor_user_id": actorUserID}
		for key, value := range extras {
			payload[key] = value
		}
		return appendAlarmCenterEvent(tx, instance, eventType, payload)
	})
}

func (s *AlarmCenterService) autoCreateWorkOrder(ctx context.Context, scope workorder.Scope, alarmPublicID string, policy store.AlarmPolicy) error {
	if s.instances == nil || s.workOrders == nil || s.workOrderPorts == nil || policy.WorkOrderTemplateID == 0 {
		return nil
	}
	instance, err := s.Get(scope, alarmPublicID)
	if err != nil {
		return err
	}
	if strings.TrimSpace(instance.WorkOrderPublicID) != "" {
		return nil
	}
	templateDetail, err := s.workOrders.GetTemplateDetail(WorkOrderScope{TenantID: scope.TenantID, ProjectID: scope.ProjectID}, policy.WorkOrderTemplateID)
	if err != nil {
		return err
	}
	evidence := map[string]any{}
	_ = json.Unmarshal([]byte(instance.EvidenceSnapshot), &evidence)
	result, err := s.workOrderPorts.Create(ctx, workorder.CreateCommand{
		Scope:        workorder.Scope{TenantID: scope.TenantID, ProjectID: scope.ProjectID, Principal: workorder.Principal{Type: "alarm", Ref: instance.PublicID, Permissions: map[string]bool{"work_order:create": true}}},
		Meta:         workorder.CommandMeta{IdempotencyKey: "alarm:" + instance.PublicID, CorrelationID: "alarm:" + instance.PublicID, CausationID: instance.PublicID},
		TemplateCode: strconv.FormatUint(uint64(policy.WorkOrderTemplateID), 10),
		Title:        firstNonBlank(instance.Title, instance.SourceRef, instance.AlarmFingerprint),
		Summary:      "Automatically created from alarm " + instance.AlarmFingerprint,
		Priority:     alarmWorkOrderPriority(instance.Severity), SourceType: WorkOrderSourceAlarm, SourceRef: instance.PublicID,
		SourceSnapshot: evidence,
		FormData:       alarmWorkOrderFormData(templateDetail.FormDefinition, evidence),
	})
	if err != nil {
		return err
	}
	_, err = s.instances.BindWorkOrder(ctx, workorder.Scope{TenantID: scope.TenantID, ProjectID: scope.ProjectID}, instance.PublicID, workorder.AlarmWorkOrderBinding{WorkOrderPublicID: result.PublicID, SourceSnapshot: evidence, CorrelationID: "alarm:" + instance.PublicID, CausationID: instance.PublicID})
	return err
}

func alarmWorkOrderFormData(definition WorkOrderFormDefinition, evidence map[string]any) map[string]any {
	values := make(map[string]any, len(evidence))
	for key, value := range evidence {
		values[key] = value
	}
	if params, ok := evidence["params"].(map[string]any); ok {
		for key, value := range params {
			values[key] = value
		}
	}
	formData := make(map[string]any)
	for _, field := range definition.Fields {
		value, exists := values[field.Key]
		if !exists {
			continue
		}
		if normalized, ok := normalizeAlarmWorkOrderFieldValue(field, value); ok {
			formData[field.Key] = normalized
		}
	}
	return formData
}

func normalizeAlarmWorkOrderFieldValue(field WorkOrderFormField, value any) (any, bool) {
	switch field.Type {
	case WorkOrderFormFieldText, WorkOrderFormFieldTextarea, WorkOrderFormFieldDevice, WorkOrderFormFieldUser:
		switch typed := value.(type) {
		case string:
			return typed, true
		case fmt.Stringer:
			return typed.String(), true
		case float64, float32, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, bool:
			return fmt.Sprint(typed), true
		}
	case WorkOrderFormFieldNumber:
		switch value.(type) {
		case float64, float32, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, json.Number:
			return value, true
		}
	case WorkOrderFormFieldInteger:
		if isWorkOrderIntegerValue(value) {
			return value, true
		}
	case WorkOrderFormFieldBoolean:
		if _, ok := value.(bool); ok {
			return value, true
		}
	case WorkOrderFormFieldSelect:
		text, ok := value.(string)
		if !ok {
			return nil, false
		}
		for _, option := range field.Options {
			if option == text {
				return text, true
			}
		}
	case WorkOrderFormFieldMultiSelect:
		if values, ok := value.([]string); ok {
			return values, true
		}
	}
	return nil, false
}

func (s *AlarmCenterService) matchPolicy(scope workorder.Scope, sourceType, sourceRef string) (store.AlarmPolicy, error) {
	return s.matchPolicyForSignal(scope, AlarmSignalMetadata{SourceType: sourceType, SourceRef: sourceRef})
}

func (s *AlarmCenterService) matchPolicyForSignal(scope workorder.Scope, metadata AlarmSignalMetadata) (store.AlarmPolicy, error) {
	var policies []store.AlarmPolicy
	if err := s.db.Where("tenant_id = ? AND project_id = ? AND enabled = ?", scope.TenantID, scope.ProjectID, true).Order("priority DESC, id ASC").Find(&policies).Error; err != nil {
		return store.AlarmPolicy{}, fmt.Errorf("load alarm policy: %w", err)
	}
	var best store.AlarmPolicy
	bestScore := -1
	for _, policy := range policies {
		score := policyMatchScoreForSignal(policy, metadata)
		if score > bestScore {
			best, bestScore = policy, score
		}
	}
	if bestScore < 0 {
		return defaultAlarmPolicy(scope), nil
	}
	return best, nil
}

// HasMatchingPolicy reports whether an event is intentionally configured as an
// alarm. It keeps arbitrary device events out of the alarm center while still
// allowing a policy to opt any event in without editing a hard-coded allowlist.
func (s *AlarmCenterService) HasMatchingPolicy(scope workorder.Scope, sourceType, sourceRef string) (bool, error) {
	policy, err := s.matchPolicy(scope, sourceType, sourceRef)
	if err != nil {
		return false, err
	}
	return policy.ID != 0, nil
}

func (s *AlarmCenterService) HasMatchingPolicyForSignal(scope workorder.Scope, metadata AlarmSignalMetadata) (bool, error) {
	policy, err := s.matchPolicyForSignal(scope, metadata)
	if err != nil {
		return false, err
	}
	return policy.ID != 0, nil
}

func (s *AlarmCenterService) suppressionFor(tx *gorm.DB, scope workorder.Scope, publicID, fingerprint string, now time.Time) (bool, string, error) {
	var suppression store.AlarmSuppression
	err := tx.Where("tenant_id = ? AND project_id = ? AND released_at IS NULL AND starts_at <= ? AND (ends_at IS NULL OR ends_at > ?) AND (target_alarm_instance_id = ? OR target_fingerprint = ?)", scope.TenantID, scope.ProjectID, now, now, publicID, fingerprint).Order("id DESC").First(&suppression).Error
	if err == gorm.ErrRecordNotFound {
		return false, "", nil
	}
	if err != nil {
		return false, "", fmt.Errorf("load alarm suppression: %w", err)
	}
	return true, suppression.SourceAlarmInstanceID, nil
}

func (s *AlarmCenterService) releaseExpiredSuppressions(now time.Time) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		var expired []store.AlarmSuppression
		if err := tx.Where("released_at IS NULL AND ends_at IS NOT NULL AND ends_at <= ?", now).Order("id ASC").Find(&expired).Error; err != nil {
			return fmt.Errorf("find expired alarm suppressions: %w", err)
		}
		for _, suppression := range expired {
			if err := tx.Model(&store.AlarmSuppression{}).Where("id = ? AND released_at IS NULL", suppression.ID).Update("released_at", &now).Error; err != nil {
				return fmt.Errorf("release expired alarm suppression: %w", err)
			}
			instance, err := findAlarmInstance(tx, workorder.Scope{TenantID: suppression.TenantID, ProjectID: suppression.ProjectID}, suppression.TargetAlarmInstanceID)
			if errors.Is(err, ErrAlarmNotFound) {
				continue
			}
			if err != nil {
				return err
			}
			muted, _, err := s.suppressionFor(tx, workorder.Scope{TenantID: suppression.TenantID, ProjectID: suppression.ProjectID}, instance.PublicID, instance.AlarmFingerprint, now)
			if err != nil {
				return err
			}
			if muted || instance.HandlingStatus == AlarmHandlingClosed || instance.HandlingStatus != AlarmHandlingShelved {
				continue
			}
			policy := policyFromSnapshot(instance.PolicySnapshot)
			updates := map[string]any{
				"notification_muted": false,
				"inhibited_by":       "",
				"handling_status":    restoreAlarmHandlingWithPrevious(instance, policy, suppression.PreviousHandlingStatus),
			}
			if err := updateAlarmInstance(tx, instance, updates); err != nil {
				return fmt.Errorf("restore expired alarm suppression: %w", err)
			}
			if err := appendAlarmCenterEvent(tx, instance, workorder.AlarmEventUnshelved, map[string]any{"reason": "suppression expired", "suppression_id": suppression.PublicID}); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *AlarmCenterService) recordServiceEvent(scope workorder.Scope, publicID, eventType string, payload map[string]any) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		instance, err := findAlarmInstance(tx, scope, publicID)
		if err != nil {
			return err
		}
		return appendAlarmCenterEvent(tx, instance, eventType, payload)
	})
}

func findAlarmInstance(tx *gorm.DB, scope workorder.Scope, publicID string) (*store.AlarmInstance, error) {
	var instance store.AlarmInstance
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND project_id = ? AND public_id = ?", scope.TenantID, scope.ProjectID, strings.TrimSpace(publicID)).First(&instance).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrAlarmNotFound
		}
		return nil, fmt.Errorf("load alarm instance: %w", err)
	}
	return &instance, nil
}

func updateAlarmInstance(tx *gorm.DB, instance *store.AlarmInstance, updates map[string]any) error {
	updates["version"] = gorm.Expr("version + 1")
	result := tx.Model(&store.AlarmInstance{}).Where("id = ? AND version = ?", instance.ID, instance.Version).Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrAlarmConflict
	}
	return nil
}

func appendAlarmCenterEvent(tx *gorm.DB, instance *store.AlarmInstance, eventType string, payload map[string]any) error {
	if payload == nil {
		payload = map[string]any{}
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode alarm event payload: %w", err)
	}
	return tx.Create(&store.AlarmInstanceEvent{TenantID: instance.TenantID, ProjectID: instance.ProjectID, AlarmInstanceID: instance.ID, Type: strings.TrimSpace(eventType), Payload: string(encoded), EvidenceSnapshot: instance.EvidenceSnapshot}).Error
}

func validateAlarmCenterScope(scope workorder.Scope) error {
	if scope.TenantID == 0 || scope.ProjectID == 0 {
		return fmt.Errorf("tenant and project scope are required")
	}
	return nil
}

func normalizedAlarmSeverity(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "critical", "urgent", "danger", "emergency":
		return "critical"
	case "major", "high":
		return "major"
	case "warning", "normal", "medium":
		return "warning"
	default:
		return "info"
	}
}

func normalizedCondition(value string) string {
	switch strings.TrimSpace(value) {
	case AlarmConditionRecovered, workorder.AlarmStatusCleared:
		return AlarmConditionRecovered
	case AlarmConditionUnknown:
		return AlarmConditionUnknown
	default:
		return AlarmConditionFiring
	}
}

func policySnapshot(policy store.AlarmPolicy) map[string]any {
	return map[string]any{
		"code":                    policy.Code,
		"priority":                policy.Priority,
		"severity":                policy.Severity,
		"require_work_order":      policy.RequireWorkOrder,
		"auto_create_work_order":  policy.AutoCreateWorkOrder,
		"work_order_template_id":  policy.WorkOrderTemplateID,
		"auto_close":              policy.AutoClose,
		"recovery_hold_seconds":   policy.RecoveryHoldSeconds,
		"acknowledge_sla_seconds": policy.AcknowledgeSLASeconds,
		"resolution_sla_seconds":  policy.ResolutionSLASeconds,
		"escalate_after_seconds":  policy.EscalateAfterSeconds,
		"allow_exception_close":   policy.AllowExceptionClose,
	}
}

type alarmPolicySnapshot struct {
	Code                  string `json:"code"`
	Priority              int    `json:"priority"`
	Severity              string `json:"severity"`
	RequireWorkOrder      bool   `json:"require_work_order"`
	AutoCreateWorkOrder   bool   `json:"auto_create_work_order"`
	WorkOrderTemplateID   uint   `json:"work_order_template_id"`
	AutoClose             bool   `json:"auto_close"`
	RecoveryHoldSeconds   int    `json:"recovery_hold_seconds"`
	AcknowledgeSLASeconds int    `json:"acknowledge_sla_seconds"`
	ResolutionSLASeconds  int    `json:"resolution_sla_seconds"`
	EscalateAfterSeconds  int    `json:"escalate_after_seconds"`
	AllowExceptionClose   bool   `json:"allow_exception_close"`
}

func policyFromSnapshot(value string) alarmPolicySnapshot {
	policy := alarmPolicySnapshot{AutoClose: true}
	if strings.TrimSpace(value) != "" {
		_ = json.Unmarshal([]byte(value), &policy)
	}
	return policy
}

func defaultAlarmPolicy(scope workorder.Scope) store.AlarmPolicy {
	return store.AlarmPolicy{TenantID: scope.TenantID, ProjectID: scope.ProjectID, Code: "default", Name: "Default alarm policy", NameEN: "Default alarm policy", Severity: "warning", AutoClose: true, Enabled: true}
}

func restoreAlarmHandling(instance *store.AlarmInstance, policy store.AlarmPolicy) string {
	return restoreAlarmHandlingWithPrevious(instance, alarmPolicySnapshot{
		RequireWorkOrder:    policy.RequireWorkOrder,
		RecoveryHoldSeconds: policy.RecoveryHoldSeconds,
	}, "")
}

func restoreAlarmHandlingWithPrevious(instance *store.AlarmInstance, policy alarmPolicySnapshot, previous string) string {
	if normalizedCondition(instance.ConditionStatus) == AlarmConditionRecovered {
		return restoreRecoveredHandling(instance, policy, instance.VerificationAfter)
	}
	switch strings.TrimSpace(previous) {
	case AlarmHandlingNew, AlarmHandlingAcknowledged, AlarmHandlingInProgress:
		return previous
	}
	if strings.TrimSpace(instance.WorkOrderPublicID) != "" || instance.OwnerUserID != 0 {
		return AlarmHandlingInProgress
	}
	return AlarmHandlingNew
}

func restoreRecoveredHandling(instance *store.AlarmInstance, policy alarmPolicySnapshot, verificationAfter *time.Time) string {
	if instance.HandlingStatus == AlarmHandlingShelved {
		return AlarmHandlingShelved
	}
	if strings.TrimSpace(instance.WorkOrderPublicID) != "" || policy.RequireWorkOrder || verificationAfter != nil || policy.RecoveryHoldSeconds > 0 {
		return AlarmHandlingPendingVerification
	}
	return AlarmHandlingAcknowledged
}

func policyMatchScore(policy store.AlarmPolicy, sourceType, sourceRef string) int {
	return policyMatchScoreForSignal(policy, AlarmSignalMetadata{SourceType: sourceType, SourceRef: sourceRef})
}

func policyMatchScoreForSignal(policy store.AlarmPolicy, metadata AlarmSignalMetadata) int {
	match := func(pattern, value string) (bool, int) {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" || pattern == "*" {
			return true, 0
		}
		if strings.EqualFold(pattern, strings.TrimSpace(value)) {
			return true, 2
		}
		return false, 0
	}
	typeOK, typeScore := match(policy.SourceType, metadata.SourceType)
	if !typeOK {
		return -1
	}
	mode := normalizedAlarmPolicyMatchMode(policy.MatchMode, normalizeAlarmPolicySourceRefs(policy.SourceRefs), normalizeAlarmPolicyEventLevels(policy.EventLevels))
	switch mode {
	case AlarmPolicyMatchAll:
		return typeScore
	case AlarmPolicyMatchLevels:
		eventLevel := strings.ToLower(strings.TrimSpace(metadata.EventLevel))
		for _, level := range normalizeAlarmPolicyEventLevels(policy.EventLevels) {
			if level == eventLevel {
				return typeScore + 2
			}
		}
		return -1
	case AlarmPolicyMatchEvents:
		// Continue with the compatible SourceRef/SourceRefs matching below.
	default:
		return -1
	}

	refOK, refScore := false, 0
	patterns := normalizeAlarmPolicySourceRefs(policy.SourceRefs)
	if len(patterns) == 0 {
		patterns = []string{policy.SourceRef}
	}
	for _, pattern := range patterns {
		matched, score := match(pattern, metadata.SourceRef)
		if productCode := strings.TrimSpace(metadata.ProductCode); productCode != "" {
			if scoped, scopedScore := match(pattern, productCode+":"+strings.TrimSpace(metadata.SourceRef)); scoped {
				matched, score = true, scopedScore+2
			}
		}
		if matched && (!refOK || score > refScore) {
			refOK, refScore = true, score
		}
	}
	if !refOK {
		return -1
	}
	return typeScore + refScore
}

func normalizedAlarmPolicyMatchMode(value string, sourceRefs, eventLevels []string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case AlarmPolicyMatchEvents:
		return AlarmPolicyMatchEvents
	case AlarmPolicyMatchLevels:
		return AlarmPolicyMatchLevels
	case AlarmPolicyMatchAll:
		return AlarmPolicyMatchAll
	case "":
		if len(eventLevels) > 0 {
			return AlarmPolicyMatchLevels
		}
		if len(sourceRefs) > 0 {
			return AlarmPolicyMatchEvents
		}
		return AlarmPolicyMatchAll
	default:
		return ""
	}
}

func normalizeAlarmPolicyEventLevels(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		switch value {
		case "info", "alert", "error":
		default:
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func normalizeAlarmPolicySourceRefs(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || value == "*" {
			continue
		}
		key := strings.ToLower(value)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, value)
	}
	return result
}

func firstNonBlank(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return "Alarm"
}

func alarmWorkOrderPriority(severity string) string {
	switch normalizedAlarmSeverity(severity) {
	case "critical":
		return "urgent"
	case "major":
		return "high"
	case "warning":
		return "normal"
	default:
		return "low"
	}
}

func isAlarmExceptionDisposition(value string) bool {
	switch strings.TrimSpace(value) {
	case "false_positive", "duplicate", "maintenance", "no_action_required", "forced_close":
		return true
	default:
		return false
	}
}

func suppressionType(sourceAlarmID string) string {
	if strings.TrimSpace(sourceAlarmID) != "" {
		return "inhibition"
	}
	return "shelve"
}
