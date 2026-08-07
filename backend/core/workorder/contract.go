package workorder

import "context"

// Principal identifies the caller of a command. It deliberately contains no
// database model fields so the port can be reused by HTTP, rules and AI.
type Principal struct {
	Type        string          `json:"type"`
	Ref         string          `json:"ref"`
	Permissions map[string]bool `json:"permissions,omitempty"`
}

type Scope struct {
	TenantID          uint      `json:"tenant_id"`
	ProjectID         uint      `json:"project_id"`
	AllowedProjectIDs []uint    `json:"allowed_project_ids,omitempty"`
	Principal         Principal `json:"principal"`
}

type CommandMeta struct {
	IdempotencyKey  string `json:"idempotency_key"`
	ExpectedVersion *int   `json:"expected_version,omitempty"`
	CorrelationID   string `json:"correlation_id,omitempty"`
	CausationID     string `json:"causation_id,omitempty"`
}

type CreateCommand struct {
	Scope          Scope          `json:"scope"`
	Meta           CommandMeta    `json:"meta"`
	TypeCode       string         `json:"type_code,omitempty"`
	TemplateCode   string         `json:"template_code"`
	Title          string         `json:"title"`
	Summary        string         `json:"summary,omitempty"`
	Priority       string         `json:"priority,omitempty"`
	SourceType     string         `json:"source_type,omitempty"`
	SourceRef      string         `json:"source_ref,omitempty"`
	SourceSnapshot map[string]any `json:"source_snapshot,omitempty"`
	FormData       map[string]any `json:"form_data,omitempty"`
}

type AttachmentItem struct {
	Name string `json:"name"`
	URL  string `json:"url"`
	Size int64  `json:"size,omitempty"`
	Type string `json:"type,omitempty"`
}

type ExecuteCommand struct {
	Scope             Scope            `json:"scope"`
	Meta              CommandMeta      `json:"meta"`
	WorkOrderPublicID string           `json:"work_order_public_id"`
	TaskPublicID      string           `json:"task_public_id,omitempty"`
	Action            string           `json:"action"`
	Comment           string           `json:"comment,omitempty"`
	Resolution        Resolution       `json:"resolution,omitempty"`
	Attachments       []AttachmentItem `json:"attachments,omitempty"`
	SourceType        string           `json:"source_type,omitempty"`
	SourceRef         string           `json:"source_ref,omitempty"`
	SourceSnapshot    map[string]any   `json:"source_snapshot,omitempty"`
	FormData          map[string]any   `json:"form_data,omitempty"`
}

type Resolution struct {
	ActualProblem   string   `json:"actual_problem"`
	RootCause       string   `json:"root_cause"`
	HandlingProcess string   `json:"handling_process"`
	HandlingResult  string   `json:"handling_result"`
	AttachmentIDs   []string `json:"attachment_ids,omitempty"`
}

type ApprovalCommand struct {
	Scope             Scope          `json:"scope"`
	Meta              CommandMeta    `json:"meta"`
	WorkOrderPublicID string         `json:"work_order_public_id"`
	TaskPublicID      string         `json:"task_public_id,omitempty"`
	Decision          string         `json:"decision,omitempty"`
	Comment           string         `json:"comment,omitempty"`
	SourceType        string         `json:"source_type,omitempty"`
	SourceRef         string         `json:"source_ref,omitempty"`
	SourceSnapshot    map[string]any `json:"source_snapshot,omitempty"`
}

type CommandResult struct {
	PublicID         string   `json:"public_id"`
	Code             string   `json:"code"`
	Version          int      `json:"version"`
	StatusKey        string   `json:"status_key,omitempty"`
	StatusCategory   string   `json:"status_category,omitempty"`
	AvailableActions []string `json:"available_actions,omitempty"`
	Created          bool     `json:"created,omitempty"`
}

type CommandPort interface {
	Create(context.Context, CreateCommand) (CommandResult, error)
	Execute(context.Context, ExecuteCommand) (CommandResult, error)
	Approve(context.Context, ApprovalCommand) (CommandResult, error)
	Reject(context.Context, ApprovalCommand) (CommandResult, error)
}
