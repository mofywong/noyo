package store

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// WorkOrderTemplate stores the project-scoped definition container. Published
// form and workflow versions are immutable and an order always stores the
// versions it was created from.
type WorkOrderTemplate struct {
	gorm.Model
	TenantID                uint   `gorm:"not null;uniqueIndex:idx_work_order_template_scope_code" json:"tenant_id"`
	ProjectID               uint   `gorm:"not null;uniqueIndex:idx_work_order_template_scope_code;index" json:"project_id"`
	Code                    string `gorm:"size:64;not null;uniqueIndex:idx_work_order_template_scope_code" json:"code"`
	Name                    string `gorm:"size:128;not null" json:"name"`
	Description             string `gorm:"type:text" json:"description"`
	Enabled                 bool   `gorm:"not null;default:true" json:"enabled"`
	SystemManaged           bool   `gorm:"not null;default:false" json:"system_managed"`
	CurrentFormVersion      int    `gorm:"not null;default:0" json:"current_form_version"`
	CurrentWorkflowVersion  int    `gorm:"not null;default:0" json:"current_workflow_version"`
	FormDraftDefinition     string `gorm:"type:text" json:"-"`
	WorkflowDraftDefinition string `gorm:"type:text" json:"-"`
}

func (WorkOrderTemplate) TableName() string {
	return "work_order_templates"
}

type WorkOrderFormVersion struct {
	gorm.Model
	PublicID           string `gorm:"size:36;not null;uniqueIndex" json:"public_id"`
	TenantID           uint   `gorm:"not null;index" json:"tenant_id"`
	ProjectID          uint   `gorm:"not null;index" json:"project_id"`
	TemplateID         uint   `gorm:"not null;uniqueIndex:idx_work_order_form_template_version" json:"template_id"`
	Version            int    `gorm:"not null;uniqueIndex:idx_work_order_form_template_version" json:"version"`
	Definition         string `gorm:"type:text;not null" json:"definition"`
	SchemaJSON         string `gorm:"type:text" json:"schema_json"`
	UISchemaJSON       string `gorm:"type:text" json:"ui_schema_json"`
	DefaultsJSON       string `gorm:"type:text" json:"defaults_json"`
	RequiredJSON       string `gorm:"type:text" json:"required_json"`
	FieldOrderJSON     string `gorm:"type:text" json:"field_order_json"`
	ResourceFieldsJSON string `gorm:"type:text" json:"resource_fields_json"`
	Checksum           string `gorm:"size:64;index" json:"checksum"`
	PublishedBy        uint   `gorm:"not null" json:"published_by"`
}

func (WorkOrderFormVersion) TableName() string {
	return "work_order_form_versions"
}

func (v *WorkOrderFormVersion) BeforeCreate(tx *gorm.DB) error {
	if v.PublicID == "" {
		v.PublicID = uuid.NewString()
	}
	return nil
}

type WorkOrderWorkflowVersion struct {
	gorm.Model
	TenantID    uint   `gorm:"not null;index" json:"tenant_id"`
	ProjectID   uint   `gorm:"not null;index" json:"project_id"`
	TemplateID  uint   `gorm:"not null;uniqueIndex:idx_work_order_workflow_template_version" json:"template_id"`
	Version     int    `gorm:"not null;uniqueIndex:idx_work_order_workflow_template_version" json:"version"`
	Definition  string `gorm:"type:text;not null" json:"definition"`
	PublishedBy uint   `gorm:"not null" json:"published_by"`
}

func (WorkOrderWorkflowVersion) TableName() string {
	return "work_order_workflow_versions"
}

type WorkOrder struct {
	gorm.Model
	PublicID               string     `gorm:"size:36;uniqueIndex;index" json:"public_id"`
	Version                int        `gorm:"not null;default:1;index" json:"version"`
	TenantID               uint       `gorm:"not null;index:idx_work_order_scope_created,priority:1;uniqueIndex:idx_work_order_source_idempotency,priority:1" json:"tenant_id"`
	ProjectID              uint       `gorm:"not null;index:idx_work_order_scope_created,priority:2;index;uniqueIndex:idx_work_order_source_idempotency,priority:2" json:"project_id"`
	Code                   string     `gorm:"size:64;not null;uniqueIndex" json:"code"`
	TemplateID             uint       `gorm:"not null;index" json:"template_id"`
	FormVersion            int        `gorm:"not null" json:"form_version"`
	FormSchemaVersionID    string     `gorm:"size:36;index" json:"form_schema_version_id"`
	FormSchemaChecksum     string     `gorm:"size:64;index" json:"form_schema_checksum"`
	FormSnapshotJSON       string     `gorm:"type:text" json:"form_snapshot_json"`
	WorkflowVersion        int        `gorm:"not null" json:"workflow_version"`
	Title                  string     `gorm:"size:256;not null" json:"title"`
	Summary                string     `gorm:"type:text" json:"summary"`
	Priority               string     `gorm:"size:24;not null;default:normal;index" json:"priority"`
	Status                 string     `gorm:"size:64;not null;index" json:"status"`
	ResolutionJSON         string     `gorm:"type:text" json:"resolution_json"`
	ResolvedAt             *time.Time `json:"resolved_at"`
	WorkflowCycle          int        `gorm:"not null;default:0" json:"workflow_cycle"`
	AssigneeUserID         uint       `gorm:"not null;default:0;index" json:"assignee_user_id"`
	CreatedBy              uint       `gorm:"not null" json:"created_by"`
	SourceType             string     `gorm:"size:32;not null;uniqueIndex:idx_work_order_source_idempotency,priority:3" json:"source_type"`
	SourceID               string     `gorm:"size:191;not null;uniqueIndex:idx_work_order_source_idempotency,priority:4" json:"source_id"`
	IdempotencyKey         string     `gorm:"size:191;not null;uniqueIndex:idx_work_order_source_idempotency,priority:5" json:"idempotency_key"`
	SourceSnapshot         string     `gorm:"type:text;not null" json:"source_snapshot"`
	FormData               string     `gorm:"type:text;not null" json:"form_data"`
	FormDefinitionSnapshot string     `gorm:"type:text;not null" json:"form_definition_snapshot"`
	WorkflowSnapshot       string     `gorm:"type:text;not null" json:"workflow_snapshot"`
	ClosedAt               *time.Time `json:"closed_at"`
}

// WorkOrderResourceReference stores the resolved resource scope at creation
// time. It is intentionally separate from the form JSON so later resource
// edits cannot change the meaning of an existing work order.
type WorkOrderResourceReference struct {
	gorm.Model
	TenantID         uint   `gorm:"not null;index:idx_work_order_resource_scope" json:"tenant_id"`
	ProjectID        uint   `gorm:"not null;index:idx_work_order_resource_scope" json:"project_id"`
	WorkOrderID      uint   `gorm:"not null;index" json:"work_order_id"`
	FieldKey         string `gorm:"size:128;not null;index" json:"field_key"`
	ResourceType     string `gorm:"size:32;not null;index" json:"resource_type"`
	ResourceRef      string `gorm:"size:191;not null" json:"resource_ref"`
	ResourceSnapshot string `gorm:"type:text;not null" json:"resource_snapshot"`
}

func (WorkOrderResourceReference) TableName() string { return "work_order_resource_references" }

func (w *WorkOrder) BeforeCreate(tx *gorm.DB) error {
	if w.PublicID == "" {
		w.PublicID = uuid.NewString()
	}
	if w.Version <= 0 {
		w.Version = 1
	}
	return nil
}

func (WorkOrder) TableName() string {
	return "work_orders"
}

type WorkOrderApprovalTask struct {
	gorm.Model
	TenantID       uint       `gorm:"not null;index" json:"tenant_id"`
	ProjectID      uint       `gorm:"not null;index" json:"project_id"`
	WorkOrderID    uint       `gorm:"not null;uniqueIndex:idx_work_order_approval_task" json:"work_order_id"`
	TransitionKey  string     `gorm:"size:64;not null;uniqueIndex:idx_work_order_approval_task" json:"transition_key"`
	WorkflowCycle  int        `gorm:"not null;uniqueIndex:idx_work_order_approval_task" json:"workflow_cycle"`
	ApproverUserID uint       `gorm:"not null;uniqueIndex:idx_work_order_approval_task;index" json:"approver_user_id"`
	Status         string     `gorm:"size:24;not null;index" json:"status"`
	Comment        string     `gorm:"type:text" json:"comment"`
	DecidedAt      *time.Time `json:"decided_at"`
}

func (WorkOrderApprovalTask) TableName() string {
	return "work_order_approval_tasks"
}

// WorkOrderNodeInstance is the immutable execution record for one graph node
// in a workflow cycle. Parallel branches are represented by distinct branch
// identifiers while the aggregate parallel-join record uses an empty branch.
type WorkOrderNodeInstance struct {
	gorm.Model
	PublicID      string     `gorm:"size:36;not null;uniqueIndex" json:"public_id"`
	TenantID      uint       `gorm:"not null;index" json:"tenant_id"`
	ProjectID     uint       `gorm:"not null;index" json:"project_id"`
	WorkOrderID   uint       `gorm:"not null;uniqueIndex:idx_work_order_node_instance,priority:1;index" json:"work_order_id"`
	WorkflowCycle int        `gorm:"not null;uniqueIndex:idx_work_order_node_instance,priority:2" json:"workflow_cycle"`
	NodeID        string     `gorm:"size:128;not null;uniqueIndex:idx_work_order_node_instance,priority:3;index" json:"node_id"`
	NodeType      string     `gorm:"size:32;not null" json:"node_type"`
	BranchID      string     `gorm:"size:128;not null;default:'';uniqueIndex:idx_work_order_node_instance,priority:4" json:"branch_id"`
	Status        string     `gorm:"size:24;not null;index" json:"status"`
	ActivatedAt   time.Time  `gorm:"not null" json:"activated_at"`
	CompletedAt   *time.Time `json:"completed_at"`
}

func (WorkOrderNodeInstance) TableName() string { return "work_order_node_instances" }

func (i *WorkOrderNodeInstance) BeforeCreate(tx *gorm.DB) error {
	if i.PublicID == "" {
		i.PublicID = uuid.NewString()
	}
	return nil
}

// WorkOrderTask is a user action generated by a graph user-task node. It
// snapshots the participant identity so the audit record remains meaningful
// after a user profile is renamed or later disabled.
type WorkOrderTask struct {
	gorm.Model
	PublicID                string     `gorm:"size:36;not null;uniqueIndex" json:"public_id"`
	TenantID                uint       `gorm:"not null;index" json:"tenant_id"`
	ProjectID               uint       `gorm:"not null;index" json:"project_id"`
	WorkOrderID             uint       `gorm:"not null;index" json:"work_order_id"`
	WorkOrderNodeInstanceID uint       `gorm:"not null;uniqueIndex:idx_work_order_task_assignee,priority:1;index" json:"work_order_node_instance_id"`
	WorkflowCycle           int        `gorm:"not null;index" json:"workflow_cycle"`
	NodeID                  string     `gorm:"size:128;not null;index" json:"node_id"`
	TaskKind                string     `gorm:"size:24;not null" json:"task_kind"`
	CompletionMode          string     `gorm:"size:16;not null" json:"completion_mode"`
	AssigneeUserID          uint       `gorm:"not null;uniqueIndex:idx_work_order_task_assignee,priority:2;index" json:"assignee_user_id"`
	AssigneeDisplayName     string     `gorm:"size:128;not null" json:"assignee_display_name"`
	Status                  string     `gorm:"size:24;not null;index" json:"status"`
	Comment                 string     `gorm:"type:text" json:"comment"`
	CompletedAt             *time.Time `json:"completed_at"`
}

func (WorkOrderTask) TableName() string { return "work_order_tasks" }

func (t *WorkOrderTask) BeforeCreate(tx *gorm.DB) error {
	if t.PublicID == "" {
		t.PublicID = uuid.NewString()
	}
	return nil
}

// WorkOrderNotification is an inbox record generated together with a task.
// It makes assignment delivery durable, while Web SSE only accelerates the
// presentation of the same record to an active browser session.
type WorkOrderNotification struct {
	gorm.Model
	PublicID          string     `gorm:"size:36;not null;uniqueIndex" json:"public_id"`
	TenantID          uint       `gorm:"not null;index" json:"tenant_id"`
	ProjectID         uint       `gorm:"not null;index" json:"project_id"`
	RecipientUserID   uint       `gorm:"not null;uniqueIndex:idx_work_order_notification_task,priority:2;index" json:"recipient_user_id"`
	WorkOrderID       uint       `gorm:"not null;index" json:"work_order_id"`
	WorkOrderPublicID string     `gorm:"size:36;not null;index" json:"work_order_public_id"`
	WorkOrderCode     string     `gorm:"size:64;not null" json:"work_order_code"`
	WorkOrderTitle    string     `gorm:"size:256;not null" json:"work_order_title"`
	WorkOrderTaskID   uint       `gorm:"not null;uniqueIndex:idx_work_order_notification_task,priority:1;index" json:"work_order_task_id"`
	TaskPublicID      string     `gorm:"size:36;not null;index" json:"task_public_id"`
	NodeID            string     `gorm:"size:128;not null;index" json:"node_id"`
	TaskKind          string     `gorm:"size:24;not null" json:"task_kind"`
	Type              string     `gorm:"size:64;not null;uniqueIndex:idx_work_order_notification_task,priority:3;index" json:"type"`
	Title             string     `gorm:"size:256;not null" json:"title"`
	Content           string     `gorm:"type:text;not null" json:"content"`
	Status            string     `gorm:"size:24;not null;index" json:"status"`
	ReadAt            *time.Time `json:"read_at"`
}

func (WorkOrderNotification) TableName() string { return "work_order_notifications" }

func (n *WorkOrderNotification) BeforeCreate(tx *gorm.DB) error {
	if n.PublicID == "" {
		n.PublicID = uuid.NewString()
	}
	return nil
}

type WorkOrderEvent struct {
	gorm.Model
	TenantID    uint   `gorm:"not null;index" json:"tenant_id"`
	ProjectID   uint   `gorm:"not null;index" json:"project_id"`
	WorkOrderID uint   `gorm:"not null;uniqueIndex:idx_work_order_event_sequence" json:"work_order_id"`
	Sequence    int    `gorm:"not null;uniqueIndex:idx_work_order_event_sequence" json:"sequence"`
	Type        string `gorm:"size:64;not null;index" json:"type"`
	ActorUserID uint   `gorm:"not null" json:"actor_user_id"`
	Payload     string `gorm:"type:text;not null" json:"payload"`
}

func (WorkOrderEvent) TableName() string {
	return "work_order_events"
}

// WorkOrderOutboxEvent is a transactionally persisted integration event. A
// connector claims records with a lease and only marks them delivered after a
// successful handoff, so transport failures never roll back business history.
type WorkOrderOutboxEvent struct {
	gorm.Model
	TenantID     uint       `gorm:"not null;index" json:"tenant_id"`
	ProjectID    uint       `gorm:"not null;index" json:"project_id"`
	WorkOrderID  uint       `gorm:"not null;uniqueIndex:idx_work_order_outbox_event" json:"work_order_id"`
	EventID      string     `gorm:"size:191;not null;uniqueIndex:idx_work_order_outbox_event" json:"event_id"`
	EventType    string     `gorm:"size:128;not null;index" json:"event_type"`
	Payload      string     `gorm:"type:text;not null" json:"payload"`
	Status       string     `gorm:"size:24;not null;index" json:"status"`
	AttemptCount int        `gorm:"not null;default:0" json:"attempt_count"`
	AvailableAt  time.Time  `gorm:"not null;index" json:"available_at"`
	LockedAt     *time.Time `json:"locked_at"`
	LockedUntil  *time.Time `gorm:"index" json:"locked_until"`
	LockToken    string     `gorm:"size:128;index" json:"lock_token"`
	LastError    string     `gorm:"type:text" json:"last_error"`
	DeliveredAt  *time.Time `gorm:"index" json:"delivered_at"`
}

func (WorkOrderOutboxEvent) TableName() string {
	return "work_order_outbox_events"
}

// WorkOrderLink keeps integrations independent of the internal primary key.
// A relation is unique inside an order and can be used by alarm, AI and
// external-business adapters without extending the state machine tables.
type WorkOrderLink struct {
	gorm.Model
	TenantID     uint   `gorm:"not null;index" json:"tenant_id"`
	ProjectID    uint   `gorm:"not null;index" json:"project_id"`
	WorkOrderID  uint   `gorm:"not null;uniqueIndex:idx_work_order_link" json:"work_order_id"`
	RelationType string `gorm:"size:32;not null;uniqueIndex:idx_work_order_link" json:"relation_type"`
	ExternalRef  string `gorm:"size:191;not null;uniqueIndex:idx_work_order_link" json:"external_ref"`
	Metadata     string `gorm:"type:text;not null" json:"metadata"`
}

func (WorkOrderLink) TableName() string {
	return "work_order_links"
}

// WorkOrderCommandReceipt records the stable result of a command so retries
// from HTTP, rules and connectors are safe. The uniqueness boundary includes
// tenant/project and principal identity, never the database primary key.
type WorkOrderCommandReceipt struct {
	gorm.Model
	TenantID       uint       `gorm:"not null;uniqueIndex:idx_work_order_command_receipt_scope_key,priority:1;index" json:"tenant_id"`
	ProjectID      uint       `gorm:"not null;uniqueIndex:idx_work_order_command_receipt_scope_key,priority:2;index" json:"project_id"`
	PrincipalType  string     `gorm:"size:64;not null;uniqueIndex:idx_work_order_command_receipt_scope_key,priority:3" json:"principal_type"`
	PrincipalRef   string     `gorm:"size:191;not null;uniqueIndex:idx_work_order_command_receipt_scope_key,priority:4" json:"principal_ref"`
	IdempotencyKey string     `gorm:"size:191;not null;uniqueIndex:idx_work_order_command_receipt_scope_key,priority:5" json:"idempotency_key"`
	RequestDigest  string     `gorm:"size:64;not null" json:"request_digest"`
	CommandType    string     `gorm:"size:64;not null" json:"command_type"`
	Status         string     `gorm:"size:24;not null" json:"status"`
	ResponseJSON   string     `gorm:"type:text" json:"response_json"`
	ErrorCode      string     `gorm:"size:64" json:"error_code"`
	ExpiresAt      *time.Time `gorm:"index" json:"expires_at"`
}

func (WorkOrderCommandReceipt) TableName() string {
	return "work_order_command_receipts"
}

// AlarmInstance stores one generation of an alarm. Clearing changes status
// and appends an event; it never deletes the immutable evidence snapshot.
type AlarmInstance struct {
	gorm.Model
	PublicID         string `gorm:"size:36;not null;uniqueIndex" json:"public_id"`
	TenantID         uint   `gorm:"not null;uniqueIndex:idx_alarm_instance_generation,priority:1;index" json:"tenant_id"`
	ProjectID        uint   `gorm:"not null;uniqueIndex:idx_alarm_instance_generation,priority:2;index" json:"project_id"`
	AlarmFingerprint string `gorm:"size:191;not null;uniqueIndex:idx_alarm_instance_generation,priority:3;index" json:"alarm_fingerprint"`
	Generation       int    `gorm:"not null;uniqueIndex:idx_alarm_instance_generation,priority:4" json:"generation"`
	Status           string `gorm:"size:24;not null;index" json:"status"`
	// Status is retained for legacy callers. ConditionStatus describes what the
	// monitored system reports, while HandlingStatus describes the human/work-
	// order lifecycle. They intentionally do not collapse into one value.
	ConditionStatus   string     `gorm:"size:24;not null;default:firing;index" json:"condition_status"`
	HandlingStatus    string     `gorm:"size:32;not null;default:new;index" json:"handling_status"`
	Severity          string     `gorm:"size:24;not null;default:warning;index" json:"severity"`
	Title             string     `gorm:"size:255" json:"title"`
	SourceType        string     `gorm:"size:64;index" json:"source_type"`
	SourceRef         string     `gorm:"size:191;index" json:"source_ref"`
	CorrelationKey    string     `gorm:"size:191;index" json:"correlation_key"`
	OwnerUserID       uint       `gorm:"index" json:"owner_user_id"`
	OccurrenceCount   int        `gorm:"not null;default:1" json:"occurrence_count"`
	FirstOccurredAt   *time.Time `gorm:"index" json:"first_occurred_at"`
	LastOccurredAt    *time.Time `gorm:"index" json:"last_occurred_at"`
	RecoveredAt       *time.Time `gorm:"index" json:"recovered_at"`
	VerificationAfter *time.Time `gorm:"index" json:"verification_after"`
	ClosedAt          *time.Time `gorm:"index" json:"closed_at"`
	ClosedBy          uint       `json:"closed_by"`
	CloseDisposition  string     `gorm:"size:64;index" json:"close_disposition"`
	PolicySnapshot    string     `gorm:"type:text" json:"policy_snapshot"`
	NotificationMuted bool       `gorm:"not null;default:false;index" json:"notification_muted"`
	InhibitedBy       string     `gorm:"size:36;index" json:"inhibited_by"`
	EvidenceSnapshot  string     `gorm:"type:text;not null" json:"evidence_snapshot"`
	WorkOrderPublicID string     `gorm:"size:36;index" json:"work_order_public_id"`
	Version           int        `gorm:"not null;default:1" json:"version"`
	AcknowledgedAt    *time.Time `json:"acknowledged_at"`
	ClearedAt         *time.Time `json:"cleared_at"`
}

func (AlarmInstance) TableName() string { return "alarm_instances" }

type AlarmInstanceEvent struct {
	gorm.Model
	TenantID         uint   `gorm:"not null;index" json:"tenant_id"`
	ProjectID        uint   `gorm:"not null;index" json:"project_id"`
	AlarmInstanceID  uint   `gorm:"not null;index" json:"alarm_instance_id"`
	Type             string `gorm:"size:32;not null;index" json:"type"`
	Payload          string `gorm:"type:text;not null" json:"payload"`
	EvidenceSnapshot string `gorm:"type:text;not null" json:"evidence_snapshot"`
}

func (AlarmInstanceEvent) TableName() string { return "alarm_instance_events" }

type AlarmInstanceWorkOrderBinding struct {
	gorm.Model
	TenantID          uint   `gorm:"not null;index;uniqueIndex:idx_alarm_work_order_binding,priority:1" json:"tenant_id"`
	ProjectID         uint   `gorm:"not null;index;uniqueIndex:idx_alarm_work_order_binding,priority:2" json:"project_id"`
	AlarmInstanceID   uint   `gorm:"not null;uniqueIndex:idx_alarm_work_order_binding,priority:3" json:"alarm_instance_id"`
	WorkOrderPublicID string `gorm:"size:36;not null;index" json:"work_order_public_id"`
	SourceSnapshot    string `gorm:"type:text;not null" json:"source_snapshot"`
	CorrelationID     string `gorm:"size:191" json:"correlation_id"`
	CausationID       string `gorm:"size:191" json:"causation_id"`
}

func (AlarmInstanceWorkOrderBinding) TableName() string { return "alarm_instance_work_order_bindings" }

// AlarmPolicy controls acknowledgement, recovery verification, work-order and
// notification behavior without forcing every alarm source to duplicate the
// same lifecycle logic.
type AlarmPolicy struct {
	gorm.Model
	TenantID   uint   `gorm:"not null;uniqueIndex:idx_alarm_policy_scope_code,priority:1;index" json:"tenant_id"`
	ProjectID  uint   `gorm:"not null;uniqueIndex:idx_alarm_policy_scope_code,priority:2;index" json:"project_id"`
	Code       string `gorm:"size:128;not null;uniqueIndex:idx_alarm_policy_scope_code,priority:3" json:"code"`
	Name       string `gorm:"size:255;not null" json:"name"`
	NameEN     string `gorm:"size:255" json:"name_en"`
	SourceType string `gorm:"size:64;index" json:"source_type"`
	SourceRef  string `gorm:"size:191;index" json:"source_ref"`
	// MatchMode selects whether the policy applies to specific product events,
	// product-model event levels, or every device event. Empty values keep the
	// legacy SourceRef/SourceRefs behavior during migration.
	MatchMode string `gorm:"size:24;index" json:"match_mode"`
	// SourceRefs extends SourceRef to multiple event identifiers. SourceRef keeps
	// the first value so existing integrations and records remain compatible.
	SourceRefs []string `gorm:"serializer:json;type:text" json:"source_refs"`
	// EventLevels contains product-model event types (info, alert, error) when
	// MatchMode is "levels".
	EventLevels []string `gorm:"serializer:json;type:text" json:"event_levels"`
	// Priority resolves overlapping policies deterministically. Higher values win;
	// the primary key remains the stable final tie breaker for legacy policies.
	Priority              int    `gorm:"not null;default:0;index" json:"priority"`
	Severity              string `gorm:"size:24;not null;default:warning" json:"severity"`
	RequireWorkOrder      bool   `gorm:"not null;default:false" json:"require_work_order"`
	AutoCreateWorkOrder   bool   `gorm:"not null;default:false" json:"auto_create_work_order"`
	WorkOrderTemplateID   uint   `json:"work_order_template_id"`
	AutoClose             bool   `gorm:"not null;default:true" json:"auto_close"`
	RecoveryHoldSeconds   int    `gorm:"not null;default:0" json:"recovery_hold_seconds"`
	AcknowledgeSLASeconds int    `gorm:"not null;default:0" json:"acknowledge_sla_seconds"`
	ResolutionSLASeconds  int    `gorm:"not null;default:0" json:"resolution_sla_seconds"`
	EscalateAfterSeconds  int    `gorm:"not null;default:0" json:"escalate_after_seconds"`
	AllowExceptionClose   bool   `gorm:"not null;default:false" json:"allow_exception_close"`
	Enabled               bool   `gorm:"not null;default:true;index" json:"enabled"`
}

func (AlarmPolicy) TableName() string { return "alarm_policies" }

// AlarmSuppression records temporary shelving, maintenance windows and root
// cause inhibition. It suppresses notification only; the alarm evidence and
// monitoring state continue to be retained.
type AlarmSuppression struct {
	gorm.Model
	PublicID              string `gorm:"size:36;not null;uniqueIndex" json:"public_id"`
	TenantID              uint   `gorm:"not null;index" json:"tenant_id"`
	ProjectID             uint   `gorm:"not null;index" json:"project_id"`
	Type                  string `gorm:"size:32;not null;index" json:"type"`
	TargetFingerprint     string `gorm:"size:191;index" json:"target_fingerprint"`
	TargetAlarmInstanceID string `gorm:"size:36;index" json:"target_alarm_instance_id"`
	SourceAlarmInstanceID string `gorm:"size:36;index" json:"source_alarm_instance_id"`
	Reason                string `gorm:"type:text;not null" json:"reason"`
	CreatedBy             uint   `json:"created_by"`
	// PreviousHandlingStatus lets a timed or manual unshelve restore the
	// operator's real workflow state instead of guessing from the alarm source.
	PreviousHandlingStatus string     `gorm:"size:32" json:"previous_handling_status"`
	StartsAt               time.Time  `gorm:"not null;index" json:"starts_at"`
	EndsAt                 *time.Time `gorm:"index" json:"ends_at"`
	ReleasedAt             *time.Time `gorm:"index" json:"released_at"`
}

func (AlarmSuppression) TableName() string { return "alarm_suppressions" }

type WorkOrderIntegrationConnector struct {
	gorm.Model
	PublicID   string `gorm:"size:36;not null;uniqueIndex" json:"public_id"`
	TenantID   uint   `gorm:"not null;index" json:"tenant_id"`
	ProjectID  uint   `gorm:"not null;index" json:"project_id"`
	Name       string `gorm:"size:128;not null" json:"name"`
	KeyID      string `gorm:"size:128;not null" json:"key_id"`
	SecretHash string `gorm:"size:128;not null" json:"-"`
	Enabled    bool   `gorm:"not null;default:true" json:"enabled"`
}

func (WorkOrderIntegrationConnector) TableName() string { return "work_order_integration_connectors" }

type WorkOrderIntegrationBinding struct {
	gorm.Model
	PublicID          string `gorm:"size:64;not null;uniqueIndex" json:"public_id"`
	TenantID          uint   `gorm:"not null;index" json:"tenant_id"`
	ProjectID         uint   `gorm:"not null;index" json:"project_id"`
	ConnectorPublicID string `gorm:"size:36;not null;index" json:"connector_public_id"`
	RemoteProject     string `gorm:"size:191" json:"remote_project"`
	Origin            string `gorm:"size:64" json:"origin"`
	Direction         string `gorm:"size:32" json:"direction"`
	LastRevision      int64  `gorm:"not null;default:0" json:"last_revision"`
}

func (WorkOrderIntegrationBinding) TableName() string { return "work_order_integration_bindings" }

type WorkOrderIntegrationMapping struct {
	gorm.Model
	PublicID        string `gorm:"size:64;not null;uniqueIndex" json:"public_id"`
	TenantID        uint   `gorm:"not null;index" json:"tenant_id"`
	ProjectID       uint   `gorm:"not null;index" json:"project_id"`
	BindingPublicID string `gorm:"size:64;not null;index" json:"binding_public_id"`
	Name            string `gorm:"size:128;not null" json:"name"`
	Revision        int64  `gorm:"not null" json:"revision"`
	FieldsJSON      string `gorm:"type:text;not null" json:"fields_json"`
	StatusJSON      string `gorm:"type:text;not null" json:"status_json"`
	PriorityJSON    string `gorm:"type:text;not null" json:"priority_json"`
}

func (WorkOrderIntegrationMapping) TableName() string { return "work_order_integration_mappings" }

type WorkOrderIntegrationInboxMessage struct {
	gorm.Model
	TenantID        uint      `gorm:"not null;index;uniqueIndex:idx_work_order_inbox_message,priority:1" json:"tenant_id"`
	ProjectID       uint      `gorm:"not null;index;uniqueIndex:idx_work_order_inbox_message,priority:2" json:"project_id"`
	BindingPublicID string    `gorm:"size:64;not null;index;uniqueIndex:idx_work_order_inbox_message,priority:3" json:"binding_public_id"`
	MessageID       string    `gorm:"size:191;not null;uniqueIndex:idx_work_order_inbox_message,priority:4" json:"message_id"`
	Digest          string    `gorm:"size:64;not null" json:"digest"`
	Revision        int64     `gorm:"not null" json:"revision"`
	Status          string    `gorm:"size:24;not null;index" json:"status"`
	Error           string    `gorm:"type:text" json:"error"`
	ReceivedAt      time.Time `gorm:"not null;index" json:"received_at"`
}

func (WorkOrderIntegrationInboxMessage) TableName() string {
	return "work_order_integration_inbox_messages"
}

func MigrateWorkOrderModels(db *gorm.DB) error {
	if err := db.AutoMigrate(
		&WorkOrderTemplate{},
		&WorkOrderFormVersion{},
		&WorkOrderWorkflowVersion{},
		&WorkOrder{},
		&WorkOrderApprovalTask{},
		&WorkOrderNodeInstance{},
		&WorkOrderTask{},
		&WorkOrderNotification{},
		&WorkOrderEvent{},
		&WorkOrderOutboxEvent{},
		&WorkOrderLink{},
		&WorkOrderCommandReceipt{},
		&WorkOrderResourceReference{},
		&AlarmInstance{},
		&AlarmInstanceEvent{},
		&AlarmInstanceWorkOrderBinding{},
		&AlarmPolicy{},
		&AlarmSuppression{},
		&WorkOrderIntegrationConnector{},
		&WorkOrderIntegrationBinding{},
		&WorkOrderIntegrationMapping{},
		&WorkOrderIntegrationInboxMessage{},
	); err != nil {
		return err
	}
	// Older Task 1 rows do not have a public identifier/version. Fill those
	// values during migration without changing their database primary keys.
	var legacy []WorkOrder
	if err := db.Where("public_id = '' OR public_id IS NULL OR version <= 0").Find(&legacy).Error; err != nil {
		return err
	}
	for i := range legacy {
		if err := db.Model(&WorkOrder{}).Where("id = ?", legacy[i].ID).Updates(map[string]any{
			"public_id": uuid.NewString(),
			"version":   1,
		}).Error; err != nil {
			return err
		}
	}
	var formVersions []WorkOrderFormVersion
	if err := db.Where("public_id = '' OR public_id IS NULL").Find(&formVersions).Error; err != nil {
		return err
	}
	for i := range formVersions {
		if err := db.Model(&WorkOrderFormVersion{}).Where("id = ?", formVersions[i].ID).Update("public_id", uuid.NewString()).Error; err != nil {
			return err
		}
	}
	// Existing alarm instances predate the dual condition/handling state. Keep
	// them visible and semantically stable after migration instead of treating
	// historical cleared records as newly firing alarms.
	if err := db.Exec(`UPDATE alarm_instances SET
		condition_status = CASE WHEN status = ? THEN ? ELSE ? END,
		handling_status = CASE WHEN status = ? THEN ? WHEN status = ? THEN ? ELSE ? END,
		severity = CASE WHEN severity = '' OR severity IS NULL THEN ? ELSE severity END,
		occurrence_count = CASE WHEN occurrence_count < 1 THEN 1 ELSE occurrence_count END,
		closed_at = CASE WHEN status = ? AND closed_at IS NULL THEN cleared_at ELSE closed_at END,
		close_disposition = CASE WHEN status = ? AND (close_disposition = '' OR close_disposition IS NULL) THEN ? ELSE close_disposition END,
		version = CASE WHEN version IS NULL OR version < 1 THEN 1 ELSE version END
		WHERE condition_status = '' OR condition_status IS NULL OR condition_status = 'active' OR (status = 'cleared' AND condition_status <> 'recovered') OR handling_status = '' OR handling_status IS NULL OR severity = '' OR severity IS NULL OR occurrence_count < 1 OR version IS NULL OR version < 1`,
		"cleared", "recovered", "firing",
		"cleared", "closed", "acknowledged", "acknowledged", "new",
		"warning", "cleared", "cleared", "legacy_cleared",
	).Error; err != nil {
		return err
	}
	return nil
}
