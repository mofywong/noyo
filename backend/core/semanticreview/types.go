// Package semanticreview defines the tool-free visual review boundary shared by
// platform, gateway and professional inference plugins.
package semanticreview

import "time"

const CandidateEvent = "ai.semantic.candidate"
const ResultEvent = "ai.semantic.result"
const SchemaVersion = "1"

type Score struct {
	Type  string  `json:"type"`
	Model string  `json:"model"`
	Value float32 `json:"value"`
}
type Frame struct {
	ID            string       `json:"id"`
	CapturedAt    time.Time    `json:"captured_at"`
	StreamSession string       `json:"stream_session"`
	Width         int          `json:"width"`
	Height        int          `json:"height"`
	Crop          [4]int       `json:"crop"`
	ROI           [][2]float32 `json:"roi,omitempty"`
	SHA256        string       `json:"sha256"`
	MIME          string       `json:"mime"`
	Data          []byte       `json:"data,omitempty"`
	Available     *bool        `json:"available,omitempty"` // record detail only; image bytes use the authorized evidence endpoint
}
type Candidate struct {
	ID                    string       `json:"id"`
	IncidentID            string       `json:"incident_id"`
	DeviceCode            string       `json:"device_code"`
	GatewayCode           string       `json:"gateway_code,omitempty"`
	RuleID                string       `json:"rule_id"`
	RuleRevision          string       `json:"rule_revision"`
	Scene                 string       `json:"scene"`
	EnterConfirmSeconds   int          `json:"enter_confirm_seconds,omitempty"`
	CooldownSeconds       *int         `json:"cooldown_seconds,omitempty"` // nil retains legacy cooldown; zero is unlimited
	TargetID              string       `json:"target_id"`
	TargetGeneration      string       `json:"target_generation"`
	StreamSession         string       `json:"stream_session"`
	ObservedAt            time.Time    `json:"observed_at"`
	ExpiresAt             time.Time    `json:"expires_at"`
	Round                 int          `json:"round"`
	Execution             string       `json:"execution"`
	ExecutorEpoch         string       `json:"executor_epoch"`
	LocalExecutorEpoch    string       `json:"local_executor_epoch,omitempty"`
	PlatformExecutorEpoch string       `json:"platform_executor_epoch,omitempty"`
	AttemptsUsed          int          `json:"attempts_used"` // this request's attempts before the next hop
	FallbackReason        string       `json:"fallback_reason,omitempty"`
	LocalResult           string       `json:"local_result"`
	InferenceSite         string       `json:"inference_site"`
	Scores                []Score      `json:"scores"`
	Evidence              []Frame      `json:"evidence"`
	TargetBoxes           [][4]float32 `json:"target_boxes"`
	TargetCoordScale      int          `json:"target_coord_scale"`
}
type Scope struct {
	TenantID, ProjectID uint
	GatewayCode         string
}

// Older candidates omitted the rule window and used the sleep default.
func (c Candidate) SleepConfirmationDuration() time.Duration {
	seconds := c.EnterConfirmSeconds
	if seconds <= 0 {
		seconds = 30
	}
	return time.Duration(seconds) * time.Second
}

func (c Candidate) ReviewCooldownDuration() time.Duration {
	seconds := 30
	if c.CooldownSeconds != nil {
		seconds = *c.CooldownSeconds
	}
	return time.Duration(seconds) * time.Second
}

type Submission struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
}
type Result struct {
	ID                    string    `json:"id"`
	IncidentID            string    `json:"incident_id"`
	DeviceCode            string    `json:"device_code"`
	GatewayCode           string    `json:"gateway_code,omitempty"`
	RuleID                string    `json:"rule_id"`
	RuleRevision          string    `json:"rule_revision"`
	Scene                 string    `json:"scene"`
	TargetID              string    `json:"target_id"`
	TargetGeneration      string    `json:"target_generation"`
	StreamSession         string    `json:"stream_session"`
	ExecutorEpoch         string    `json:"executor_epoch"`
	ActualExecutorEpoch   string    `json:"actual_executor_epoch,omitempty"`
	LocalExecutorEpoch    string    `json:"local_executor_epoch,omitempty"`
	PlatformExecutorEpoch string    `json:"platform_executor_epoch,omitempty"`
	AttemptsUsed          int       `json:"attempts_used"` // this request's cumulative calls across both sites
	FallbackReason        string    `json:"fallback_reason,omitempty"`
	ObservedAt            time.Time `json:"observed_at"`
	ExpiresAt             time.Time `json:"expires_at"`
	CompletedAt           time.Time `json:"completed_at"`
	LocalResult           string    `json:"local_result"`
	Scores                []Score   `json:"scores"`
	InferenceSite         string    `json:"inference_site"`
	Execution             string    `json:"execution"`
	Status                string    `json:"status"`
	Verdict               string    `json:"verdict,omitempty"`
	DecisionSource        string    `json:"decision_source"`
	SummaryZH             string    `json:"summary_zh"`
	SummaryEN             string    `json:"summary_en"`
	EvidenceFrameIDs      []string  `json:"evidence_frame_ids,omitempty"`
	ReasonCodes           []string  `json:"reason_codes,omitempty"`
	PostureType           string    `json:"posture_type,omitempty"`
	Visibility            string    `json:"visibility,omitempty"`
	Provider              string    `json:"provider,omitempty"`
	Model                 string    `json:"model,omitempty"`
	RequestID             string    `json:"request_id,omitempty"`
	InputTokens           int       `json:"input_tokens,omitempty"`
	OutputTokens          int       `json:"output_tokens,omitempty"`
	UsageReported         bool      `json:"usage_reported,omitempty"`
	DurationMs            int64     `json:"duration_ms,omitempty"`
}
type Status struct {
	Enabled               bool     `json:"enabled"`
	Status                string   `json:"status"`
	Pending               int      `json:"pending"`
	Execution             string   `json:"execution"`
	ExecutorEpoch         string   `json:"executor_epoch"`
	LocalExecutorEpoch    string   `json:"local_executor_epoch,omitempty"`
	PlatformExecutorEpoch string   `json:"platform_executor_epoch,omitempty"`
	Results               []Result `json:"results"`
}
type Client interface {
	SubmitCandidate(Candidate) Submission
	SemanticStatus(string) Status
	GetResults(string) []Result
}

func NewResult(c Candidate, status string) Result {
	site := "platform"
	if c.Execution == "local_first" {
		site = "local"
	}
	return Result{ID: c.ID, IncidentID: c.IncidentID, DeviceCode: c.DeviceCode, GatewayCode: c.GatewayCode, RuleID: c.RuleID, RuleRevision: c.RuleRevision, Scene: c.Scene, TargetID: c.TargetID, TargetGeneration: c.TargetGeneration, StreamSession: c.StreamSession, ExecutorEpoch: c.ExecutorEpoch, LocalExecutorEpoch: c.LocalExecutorEpoch, PlatformExecutorEpoch: c.PlatformExecutorEpoch, AttemptsUsed: c.AttemptsUsed, FallbackReason: c.FallbackReason, ObservedAt: c.ObservedAt, ExpiresAt: c.ExpiresAt, CompletedAt: time.Now().UTC(), LocalResult: c.LocalResult, Scores: c.Scores, InferenceSite: c.InferenceSite, Execution: site, Status: status, DecisionSource: "local_model"}
}
