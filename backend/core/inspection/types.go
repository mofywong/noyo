// Package inspection contains the persistent, edition-independent inspection
// state machine and provider contract. It never changes real-time AI settings.
package inspection

import (
	"context"
	"encoding/json"
	"time"
)

type Scope struct {
	TenantID  uint `json:"tenant_id"`
	ProjectID uint `json:"project_id"`
}
type Actor struct {
	UserID   uint   `json:"user_id"`
	Name     string `json:"name"`
	Username string `json:"username,omitempty"`
	Kind     string `json:"kind"`
}
type Issue struct {
	Code string `json:"code"`
	Name string `json:"name"`
}
type Region struct {
	Left   float32 `json:"left"`
	Top    float32 `json:"top"`
	Right  float32 `json:"right"`
	Bottom float32 `json:"bottom"`
}
type Scene struct {
	WindowSeconds  int     `json:"window_seconds,omitempty"`
	ConfirmSeconds int     `json:"confirm_seconds,omitempty"`
	MinFrames      int     `json:"min_frames,omitempty"`
	MinPeople      int     `json:"min_people,omitempty"`
	ROI            *Region `json:"roi,omitempty"`
	Code           string  `json:"code"`
	Target         string  `json:"target"`
	Confidence     float32 `json:"confidence"`
}
type Camera struct {
	Code   string  `json:"code"`
	Name   string  `json:"name"`
	Domain string  `json:"domain"`
	Scenes []Scene `json:"scenes"`
}
type Spec struct {
	OrderBinding        OrderBinding            `json:"order_binding"`
	IssueOrderBindings  map[string]OrderBinding `json:"issue_order_bindings,omitempty"`
	Name                string                  `json:"name"`
	Mode                string                  `json:"mode"`
	Cameras             []Camera                `json:"cameras"`
	Issues              []Issue                 `json:"issues"`
	MinimumWatchSeconds int                     `json:"minimum_watch_seconds"`
	Schedules           []string                `json:"schedules"`
	ScheduleRules       []ScheduleRule          `json:"schedule_rules,omitempty"`
	Timezone            string                  `json:"timezone"`
	Enabled             bool                    `json:"enabled"`
	AutoOrder           bool                    `json:"auto_order"`
	TemplateID          uint                    `json:"template_id"`
	OrderFormData       map[string]any          `json:"order_form_data"`
	ReviewPolicy        string                  `json:"review_policy"`
}
type Plan struct {
	ExecutionGrantID string `json:"execution_grant_id,omitempty"`
	ID               string `gorm:"primaryKey;size:36" json:"id"`
	Scope            `gorm:"embedded"`
	Revision         uint       `json:"revision"`
	Name             string     `json:"name"`
	Mode             string     `json:"mode"`
	Enabled          bool       `json:"enabled"`
	SpecJSON         string     `gorm:"type:text" json:"-"`
	NextDueAt        *time.Time `json:"next_due_at"`
	CreatedBy        uint       `json:"created_by"`
	UpdatedAt        time.Time  `json:"updated_at"`
	CreatedAt        time.Time  `json:"created_at"`
	ExecutionCount   uint64     `gorm:"not null;default:0" json:"execution_count"`
}

func (Plan) TableName() string { return "inspection_plans" }
func (p Plan) Spec() Spec      { var v Spec; _ = json.Unmarshal([]byte(p.SpecJSON), &v); return v }

type Run struct {
	ExecutionGrantID string `json:"execution_grant_id,omitempty"`
	ID               string `gorm:"primaryKey;size:36" json:"id"`
	Scope            `gorm:"embedded"`
	PlanID           string     `gorm:"index" json:"plan_id"`
	Revision         uint       `json:"revision"`
	Name             string     `json:"name"`
	Mode             string     `json:"mode"`
	Status           string     `gorm:"index" json:"status"`
	Snapshot         string     `gorm:"type:text" json:"-"`
	Actor            Actor      `gorm:"embedded;embeddedPrefix:actor_" json:"actor"`
	Slot             string     `gorm:"uniqueIndex;size:150" json:"-"`
	Version          uint       `json:"version"`
	Current          int        `json:"current"`
	StartedAt        time.Time  `json:"started_at"`
	FinishedAt       *time.Time `json:"finished_at"`
	CreatedAt        time.Time  `json:"created_at"`
	SourceRunID      string     `json:"source_run_id,omitempty"`
	DeadlineAt       *time.Time `gorm:"index" json:"deadline_at,omitempty"`
	QueueDeadlineAt  *time.Time `gorm:"index" json:"queue_deadline_at,omitempty"`
	Detail           string     `json:"detail,omitempty"`
	Result           string     `gorm:"-" json:"result"`
	Summary          RunSummary `gorm:"-" json:"summary"`
}

func (Run) TableName() string { return "inspection_runs" }
func (r Run) Spec() Spec      { var v Spec; _ = json.Unmarshal([]byte(r.Snapshot), &v); return v }

type Item struct {
	ID                 string     `gorm:"primaryKey;size:36" json:"id"`
	RunID              string     `gorm:"uniqueIndex:inspection_run_position" json:"run_id"`
	Position           int        `gorm:"uniqueIndex:inspection_run_position" json:"position"`
	CameraCode         string     `json:"camera_code"`
	CameraName         string     `json:"camera_name"`
	Domain             string     `json:"domain"`
	CanViewVideo       bool       `gorm:"-" json:"can_view_video"`
	Status             string     `json:"status"`
	Result             string     `json:"result"`
	Stage              string     `json:"stage"`
	StageStartedAt     *time.Time `json:"stage_started_at,omitempty"`
	StageUpdatedAt     *time.Time `json:"stage_updated_at,omitempty"`
	ProgressJSON       string     `gorm:"type:text" json:"progress_json,omitempty"`
	IssuesJSON         string     `gorm:"type:text" json:"issues_json"`
	Detail             string     `json:"detail"`
	ObservedAt         *time.Time `json:"observed_at"`
	CheckedAt          *time.Time `json:"checked_at"`
	CapturedAt         *time.Time `json:"captured_at"`
	ModelVersion       string     `json:"model_version"`
	EvidenceJSON       string     `gorm:"type:text" json:"evidence_json,omitempty"`
	EpisodeID          string     `json:"episode_id,omitempty"`
	Findings           []Finding  `gorm:"-" json:"findings"`
	Coverage           string     `json:"coverage"`
	SceneResultsJSON   string     `gorm:"type:text" json:"scene_results_json"`
	ReviewStatus       string     `json:"review_status"`
	ReviewVersion      uint       `json:"review_version"`
	ReviewedIssuesJSON string     `gorm:"type:text" json:"reviewed_issues_json"`
	ReviewedBy         Actor      `gorm:"embedded;embeddedPrefix:reviewed_by_" json:"reviewed_by"`
	ReviewedAt         *time.Time `json:"reviewed_at"`
	ReviewNote         string     `json:"review_note"`
}

func (Item) TableName() string { return "inspection_items" }

type Episode struct {
	ID          string `gorm:"primaryKey" json:"id"`
	Scope       `gorm:"embedded"`
	ActiveKey   *string   `gorm:"uniqueIndex" json:"-"`
	Fingerprint string    `gorm:"index" json:"-"`
	Passes      int       `json:"passes"`
	CreatedAt   time.Time `json:"created_at"`
	CameraCode  string    `gorm:"index" json:"camera_code"`
	IssueCode   string    `json:"issue_code"`
	LegacyID    string    `gorm:"index" json:"legacy_id,omitempty"`
}

func (Episode) TableName() string { return "inspection_episodes" }

type OrderIntent struct {
	CommandJSON        string     `gorm:"type:text" json:"-"`
	RecoveredBy        uint       `json:"recovered_by,omitempty"`
	RecoveryReason     string     `json:"recovery_reason,omitempty"`
	PolicyVersionID    uint       `json:"policy_version_id"`
	ExecutionGrantID   uint       `json:"execution_grant_id"`
	FormVersion        int        `json:"form_version"`
	WorkflowVersion    int        `json:"workflow_version"`
	LeaseToken         string     `gorm:"size:36" json:"-"`
	LeaseUntil         *time.Time `json:"-"`
	ID                 string     `gorm:"primaryKey" json:"id"`
	Scope              `gorm:"embedded"`
	RunID              string     `json:"run_id"`
	ItemID             string     `json:"item_id"`
	TemplateID         uint       `json:"template_id"`
	ActorID            uint       `json:"actor_id"`
	Title              string     `json:"title"`
	FormJSON           string     `gorm:"type:text" json:"-"`
	Status             string     `gorm:"index" json:"status"`
	OrderID            uint       `json:"order_id"`
	OrderCode          string     `json:"order_code"`
	OrderTitle         string     `json:"order_title"`
	Error              string     `json:"error"`
	Attempts           int        `json:"attempts"`
	NextAttemptAt      time.Time  `json:"next_attempt_at"`
	CreatedAt          time.Time  `json:"created_at"`
	EpisodeID          string     `gorm:"index" json:"episode_id"`
	IssueCode          string     `json:"issue_code"`
	Reason             string     `json:"reason,omitempty"`
	WorkOrderStatus    string     `json:"work_order_status"`
	OrderStatusAt      *time.Time `json:"order_status_at"`
	ResolutionRequired bool       `json:"resolution_required"`
}

// Bindings in run snapshots and intents are immutable execution inputs. Plan
// changes affect future runs; retries retain the originally selected versions.
type OrderBinding struct {
	FormData         map[string]any `json:"form_data,omitempty"`
	PolicyVersionID  uint           `json:"policy_version_id"`
	ExecutionGrantID uint           `json:"execution_grant_id"`
	TemplateID       uint           `json:"template_id"`
	FormVersion      int            `json:"form_version"`
	WorkflowVersion  int            `json:"workflow_version"`
	AuthorizedBy     uint           `json:"authorized_by"`
}

func (OrderIntent) TableName() string { return "inspection_order_intents" }

type Report struct {
	Run     Run            `json:"run"`
	Spec    Spec           `json:"spec"`
	Items   []Item         `json:"items"`
	Orders  []OrderIntent  `json:"orders"`
	Reviews []ReviewEvent  `json:"reviews"`
	Archive *ReportVersion `json:"archive,omitempty"`
}
type Submission struct {
	Version      uint     `json:"version"`
	Result       string   `json:"result"`
	Issues       []string `json:"issues"`
	Detail       string   `json:"detail"`
	EvidenceJPEG string   `json:"evidence_jpeg"`
}
type Analysis struct {
	Result       string
	Issues       []string
	Detail       string
	CapturedAt   time.Time
	ModelVersion string
	Evidence     any
	Coverage     string
	SceneResults []SceneResult
}

// SceneResult separates the business judgement from observation coverage.
type SceneResult struct {
	Code        string    `json:"code"`
	Result      string    `json:"result"`
	Reason      string    `json:"reason,omitempty"`
	StartedAt   time.Time `json:"started_at"`
	FinishedAt  time.Time `json:"finished_at"`
	SampleCount int       `json:"sample_count"`
}

// A batch owns model references until Close. Analyze must finish native calls
// before returning, including on context cancellation. Real-time owns other refs.
type Batch interface {
	Analyze(context.Context, Camera) (Analysis, error)
	Close()
}
type Provider interface {
	Open(context.Context, string, []Camera) (Batch, error)
}
