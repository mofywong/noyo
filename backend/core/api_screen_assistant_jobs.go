package core

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/gogf/gf/v2/net/ghttp"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"noyo/core/assistant"
	"noyo/core/screen"
	"noyo/core/store"
)

const screenAssistantJobBudget = 5 * time.Minute
const screenAssistantJobLease = 45 * time.Second

var errScreenAssistantLeaseLost = errors.New("screen assistant task lease lost")

type screenAssistantTaskInput struct {
	Payload     map[string]interface{}   `json:"payload"`
	Document    screen.Envelope          `json:"document"`
	Widgets     []screen.AssistantWidget `json:"widgets"`
	Catalog     screen.AssistantCatalog  `json:"catalog"`
	AllowAlarms bool                     `json:"allowAlarms"`
}

type screenAssistantTaskStatus struct {
	TaskID       string                `json:"taskId"`
	State        string                `json:"state"`
	Stage        string                `json:"stage,omitempty"`
	Summary      string                `json:"summary,omitempty"`
	ErrorKey     string                `json:"errorKey,omitempty"`
	Version      uint64                `json:"version"`
	Attempt      uint                  `json:"attempt"`
	Result       *screen.AssistantPlan `json:"result,omitempty"`
	BaseDocument *screen.Envelope      `json:"baseDocument,omitempty"`
	UpdatedAt    time.Time             `json:"updatedAt"`
}

func screenAssistantTaskView(job store.IOCScreenAssistantJob) screenAssistantTaskStatus {
	view := screenAssistantTaskStatus{TaskID: job.TaskID, State: job.State, Stage: job.Stage, Summary: job.Summary, ErrorKey: job.ErrorKey, Version: job.Version, Attempt: job.Attempt, UpdatedAt: job.UpdatedAt}
	if job.State == "completed" {
		var plan screen.AssistantPlan
		if json.Unmarshal([]byte(job.Result), &plan) == nil {
			view.Result = &plan
		}
	}
	return view
}

func screenAssistantTaskToken() (string, error) {
	var value [20]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(value[:]), nil
}

func screenAssistantTaskScope(r *ghttp.Request, row *store.IOCScreen) *gorm.DB {
	return store.DB.Model(&store.IOCScreenAssistantJob{}).Where("tenant_id = ? AND project_id = ? AND screen_id = ? AND user_id = ?", row.TenantID, row.ProjectID, row.ID, requestAuthContext(r).UserID)
}

// A detached worker resolves current permissions rather than retaining the
// request's auth object. This check also runs before artifacts can be returned.
func screenAssistantTaskAuthorize(job store.IOCScreenAssistantJob, input screenAssistantTaskInput) error {
	auth, err := ResolveUserAuthContext(job.UserID, job.TenantID, job.ProjectID)
	if err != nil || !screenHasPermission(auth, job.ProjectID, "screen:edit") {
		return screenFail(403, "Task access changed / 任务访问权限已改变")
	}
	if _, err = store.ScreenFind(store.DB, job.TenantID, job.ProjectID, job.ScreenID); err != nil {
		return err
	}
	if input.AllowAlarms && !auth.HasProjectPermission("alarm:list", job.ProjectID) {
		return screenFail(403, "Task data access changed / 任务数据访问权限已改变")
	}
	var profile screen.SceneProfile
	encoded, _ := json.Marshal(input.Payload["profile"])
	_ = json.Unmarshal(encoded, &profile)
	if len(input.Catalog) == 0 && len(profile.Products) == 0 {
		return nil
	}
	if !auth.HasProjectPermission("device:list", job.ProjectID) {
		return screenFail(403, "Task data access changed / 任务数据访问权限已改变")
	}
	for _, reference := range profile.Products {
		var product store.Product
		if store.DB.Where("code = ? AND (tenant_id = 0 OR (tenant_id = ? AND (project_id = ? OR project_id = 0)))", reference.Code, job.TenantID, job.ProjectID).First(&product).Error != nil {
			return screenFail(403, "Task product access changed / 任务产品访问权限已改变")
		}
		if !auth.IsSystemAdmin && product.TenantID != 0 && !auth.IsTenantAdmin && (product.ProjectID == 0 || !auth.CanAccessProject(product.ProjectID)) {
			return screenFail(403, "Task product access changed / 任务产品访问权限已改变")
		}
	}
	scope := store.GlobalAccessScope()
	if auth.TenantID != 0 {
		scope = store.AccessScope{Type: "tenant", ID: strconv.FormatUint(uint64(auth.TenantID), 10)}
	}
	for code := range input.Catalog {
		device, err := store.GetDevice(code)
		if err != nil || device.TenantID != job.TenantID || device.ProjectID != job.ProjectID {
			return screenFail(403, "Task data access changed / 任务数据访问权限已改变")
		}
		allowed, err := canReadDeviceByTagPermissionFor(auth, scope, code, "device:list")
		if err != nil || !allowed {
			return screenFail(403, "Task data access changed / 任务数据访问权限已改变")
		}
	}
	return nil
}

func (s *Server) screenAssistantStartTask(r *ghttp.Request, request screenAssistantRequest, payload map[string]interface{}, row *store.IOCScreen, doc screen.Envelope, widgets []screen.AssistantWidget, catalog screen.AssistantCatalog, allowAlarms bool) error {
	if requestAuthContext(r).UserID == 0 || requestAuthContext(r).SubjectType == "app" || !screenAuditRequestID.MatchString(request.RequestID) {
		return screenFail(400, "A valid task request ID is required / 需要有效的任务请求标识")
	}
	input := screenAssistantTaskInput{payload, doc, widgets, catalog, allowAlarms}
	encoded, err := json.Marshal(input)
	if err != nil || len(encoded) > 4<<20 {
		return screenFail(400, "Task context exceeds capacity / 任务上下文超过处理容量")
	}
	stableRequest := request
	stableRequest.History, stableRequest.RequestID, stableRequest.Stream, stableRequest.Async = nil, "", false, false
	identity, _ := json.Marshal(stableRequest)
	hash := sha256.Sum256(identity)
	token, err := screenAssistantTaskToken()
	if err != nil {
		return err
	}
	job := store.IOCScreenAssistantJob{TaskID: token, TenantID: row.TenantID, ProjectID: row.ProjectID, ScreenID: row.ID, UserID: requestAuthContext(r).UserID, RequestID: request.RequestID, ConversationID: request.ConversationID, InputHash: hex.EncodeToString(hash[:]), Input: string(encoded), Checkpoints: "{}", State: "queued", Version: 1}
	if err = store.DB.Clauses(clause.OnConflict{DoNothing: true}).Create(&job).Error; err != nil {
		return err
	}
	if err = screenAssistantTaskScope(r, row).Where("request_id = ?", request.RequestID).First(&job).Error; err != nil {
		return err
	}
	if job.InputHash != hex.EncodeToString(hash[:]) {
		return screenFail(409, "Task request ID was already used / 任务请求标识已被使用")
	}
	var retained screenAssistantTaskInput
	if json.Unmarshal([]byte(job.Input), &retained) != nil {
		return screenFail(409, "Task context is invalid / 任务上下文无效")
	}
	if err = screenAssistantTaskAuthorize(job, retained); err != nil {
		return err
	}
	if job.State == "queued" || (job.State == "running" && time.Now().After(job.LeaseUntil)) {
		go s.screenAssistantRunTask(job.TaskID)
	}
	screenWriteData(r, screenAssistantTaskView(job))
	return nil
}

func screenAssistantTaskRead(r *ghttp.Request) (*store.IOCScreen, store.IOCScreenAssistantJob, error) {
	row, err := screenLoad(r, "screen:edit")
	var job store.IOCScreenAssistantJob
	if err != nil {
		return row, job, err
	}
	query := screenAssistantTaskScope(r, row)
	if taskID := r.GetRouter("taskId").String(); taskID != "" {
		query = query.Where("task_id = ?", taskID)
	} else {
		query = query.Where("conversation_id = ?", r.GetQuery("conversationId").String()).Order("id DESC")
	}
	err = query.First(&job).Error
	if err != nil {
		return row, job, err
	}
	var input screenAssistantTaskInput
	if json.Unmarshal([]byte(job.Input), &input) != nil {
		return row, job, screenFail(409, "Task context is invalid / 任务上下文无效")
	}
	// Request permission checking covers private authorship. Revalidation against
	// fresh roles prevents a retained browser session returning revoked data.
	if err = screenAssistantTaskAuthorize(job, input); err != nil {
		return row, job, err
	}
	return row, job, nil
}

func (s *Server) handleScreenAssistantTaskCurrent(r *ghttp.Request) {
	_, job, err := screenAssistantTaskRead(r)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		screenWriteData(r, nil)
		return
	}
	if err != nil {
		screenWriteError(r, err)
		return
	}
	if job.State == "queued" || (job.State == "running" && time.Now().After(job.LeaseUntil)) {
		go s.screenAssistantRunTask(job.TaskID)
	}
	view := screenAssistantTaskView(job)
	if r.GetRouter("taskId").String() == "" {
		var input screenAssistantTaskInput
		if json.Unmarshal([]byte(job.Input), &input) == nil {
			view.BaseDocument = &input.Document
		}
	}
	screenWriteData(r, view)
}

func (s *Server) handleScreenAssistantTaskGet(r *ghttp.Request) {
	s.handleScreenAssistantTaskCurrent(r)
}

func (s *Server) handleScreenAssistantTaskResume(r *ghttp.Request) {
	_, job, err := screenAssistantTaskRead(r)
	if err != nil {
		screenWriteError(r, err)
		return
	}
	if job.State == "canceled" || job.Attempt >= 3 || job.SpentMs >= screenAssistantJobBudget.Milliseconds() {
		screenWriteError(r, screenFail(409, "Task cannot resume; create a new request / 任务无法继续，请发起新请求"))
		return
	}
	if job.State == "failed" {
		result := store.DB.Model(&store.IOCScreenAssistantJob{}).Where("id = ? AND version = ? AND state = ?", job.ID, job.Version, "failed").Updates(map[string]interface{}{"state": "queued", "error_key": "", "version": job.Version + 1, "lease_token": ""})
		if result.Error != nil {
			screenWriteError(r, result.Error)
			return
		}
		store.DB.First(&job, job.ID)
	}
	if job.State != "completed" {
		go s.screenAssistantRunTask(job.TaskID)
	}
	screenWriteData(r, screenAssistantTaskView(job))
}

func (s *Server) handleScreenAssistantTaskCancel(r *ghttp.Request) {
	_, job, err := screenAssistantTaskRead(r)
	if err != nil {
		screenWriteError(r, err)
		return
	}
	result := store.DB.Model(&store.IOCScreenAssistantJob{}).Where("id = ? AND version = ? AND state IN ?", job.ID, job.Version, []string{"queued", "running", "failed"}).Updates(map[string]interface{}{"state": "canceled", "version": job.Version + 1, "lease_token": "", "result": "", "error_key": ""})
	if result.Error != nil {
		screenWriteError(r, result.Error)
		return
	}
	store.DB.First(&job, job.ID)
	screenWriteData(r, screenAssistantTaskView(job))
}

type screenAssistantCheckpoint struct {
	InputHash string `json:"inputHash"`
	Content   string `json:"content"`
}
type screenAssistantJobContext struct {
	db          *gorm.DB
	job         store.IOCScreenAssistantJob
	checkpoints map[string]screenAssistantCheckpoint
}
type screenAssistantJobContextKey struct{}

func (j *screenAssistantJobContext) fence() *gorm.DB {
	return j.db.Model(&store.IOCScreenAssistantJob{}).Where("id = ? AND version = ? AND attempt = ? AND lease_token = ? AND state = ? AND lease_until > ?", j.job.ID, j.job.Version, j.job.Attempt, j.job.LeaseToken, "running", time.Now())
}

func screenAssistantLoadCheckpoint(ctx context.Context, stage, inputHash string) (string, bool) {
	j, ok := ctx.Value(screenAssistantJobContextKey{}).(*screenAssistantJobContext)
	if !ok || ctx.Err() != nil {
		return "", false
	}
	checkpoint, exists := j.checkpoints[stage]
	return checkpoint.Content, exists && checkpoint.InputHash == inputHash
}

func screenAssistantSaveCheckpoint(ctx context.Context, stage, inputHash, content string) error {
	j, ok := ctx.Value(screenAssistantJobContextKey{}).(*screenAssistantJobContext)
	if !ok {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(content) > 256<<10 || len(stage) > 40 || len(j.checkpoints) > 12 {
		return errScreenAssistantInvalid
	}
	j.checkpoints[stage] = screenAssistantCheckpoint{inputHash, content}
	encoded, err := json.Marshal(j.checkpoints)
	if err != nil {
		return err
	}
	result := j.fence().Updates(map[string]interface{}{"checkpoints": string(encoded)})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return errScreenAssistantLeaseLost
	}
	return nil
}

func screenAssistantTaskError(err error) string {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return "aiTimeout"
	case errors.Is(err, errScreenAssistantInvalid):
		return "aiInvalid"
	case errors.Is(err, errScreenAssistantOutOfScope):
		return "aiOutOfScope"
	default:
		return "aiFailed"
	}
}

func (s *Server) screenAssistantRunTask(taskID string) {
	db := store.DB
	var job store.IOCScreenAssistantJob
	if db.Where("task_id = ?", taskID).First(&job).Error != nil {
		return
	}
	if job.State == "running" && time.Now().Before(job.LeaseUntil) {
		return
	}
	// Charge an interrupted attempt conservatively too. A process crash must not
	// reset the total generation budget or leave a final attempt running forever.
	if job.State == "running" && !job.AttemptStartedAt.IsZero() {
		job.SpentMs += time.Since(job.AttemptStartedAt).Milliseconds()
	}
	if job.Attempt >= 3 || job.SpentMs >= screenAssistantJobBudget.Milliseconds() {
		db.Model(&store.IOCScreenAssistantJob{}).Where("id = ? AND version = ? AND (state = ? OR (state = ? AND lease_until < ?))", job.ID, job.Version, "queued", "running", time.Now()).Updates(map[string]interface{}{"state": "failed", "error_key": "aiTimeout", "spent_ms": job.SpentMs, "lease_token": "", "version": job.Version + 1})
		return
	}
	lease, err := screenAssistantTaskToken()
	if err != nil {
		return
	}
	now := time.Now()
	claimed := db.Model(&store.IOCScreenAssistantJob{}).Where("id = ? AND version = ? AND (state = ? OR (state = ? AND lease_until < ?))", job.ID, job.Version, "queued", "running", now).Updates(map[string]interface{}{"state": "running", "version": job.Version + 1, "attempt": job.Attempt + 1, "lease_token": lease, "lease_until": now.Add(screenAssistantJobLease), "attempt_started_at": now, "spent_ms": job.SpentMs, "error_key": ""})
	if claimed.Error != nil || claimed.RowsAffected != 1 {
		return
	}
	job.Version, job.Attempt, job.LeaseToken, job.State = job.Version+1, job.Attempt+1, lease, "running"
	j := &screenAssistantJobContext{db: db, job: job, checkpoints: map[string]screenAssistantCheckpoint{}}
	_ = json.Unmarshal([]byte(job.Checkpoints), &j.checkpoints)
	ctx, cancel := context.WithTimeout(context.WithValue(context.Background(), screenAssistantJobContextKey{}, j), screenAssistantJobBudget-time.Duration(job.SpentMs)*time.Millisecond)
	defer cancel()
	done := make(chan struct{})
	defer close(done)
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				result := j.fence().Updates(map[string]interface{}{"lease_until": time.Now().Add(screenAssistantJobLease)})
				if result.Error != nil || result.RowsAffected != 1 {
					cancel()
					return
				}
			}
		}
	}()
	finish := func(plan *screen.AssistantPlan, failure error) {
		updates := map[string]interface{}{"state": "failed", "error_key": screenAssistantTaskError(failure), "spent_ms": job.SpentMs + time.Since(now).Milliseconds(), "lease_token": ""}
		if plan != nil {
			encoded, err := json.Marshal(plan)
			if err == nil {
				updates["state"], updates["result"], updates["error_key"] = "completed", string(encoded), ""
			}
		}
		j.fence().Updates(updates)
	}
	defer func() {
		if recover() != nil {
			finish(nil, errors.New("task interrupted"))
		}
	}()
	var input screenAssistantTaskInput
	if json.Unmarshal([]byte(job.Input), &input) != nil {
		finish(nil, errScreenAssistantInvalid)
		return
	}
	if err = screenAssistantTaskAuthorize(job, input); err != nil {
		finish(nil, err)
		return
	}
	provider, err := s.screenAssistantProvider()
	if err != nil {
		finish(nil, err)
		return
	}
	model, err := assistant.NewRuntime(ctx, assistant.RuntimeConfig{Provider: provider})
	if err != nil {
		finish(nil, err)
		return
	}
	row, err := store.ScreenFind(db, job.TenantID, job.ProjectID, job.ScreenID)
	if err != nil {
		finish(nil, err)
		return
	}
	emit := func(event string, value interface{}) {
		fields, ok := value.(map[string]string)
		if !ok {
			return
		}
		updates := map[string]interface{}{}
		if event == "stage" {
			updates["stage"] = screenAssistantText(fields["stage"], 32)
		}
		if event == "summary" {
			updates["summary"] = screenAssistantText(strings.TrimSpace(fields["content"]), 1000)
		}
		if len(updates) != 0 {
			result := j.fence().Updates(updates)
			if result.Error != nil || result.RowsAffected != 1 {
				cancel()
			}
		}
	}
	plan, err := s.screenAssistantWorkflow(ctx, model, input.Payload, row, input.Document, input.Widgets, input.Catalog, input.AllowAlarms, true, emit)
	if err == nil {
		err = screenAssistantTaskAuthorize(job, input)
	}
	if err != nil {
		finish(nil, err)
		return
	}
	finish(&plan, nil)
}
