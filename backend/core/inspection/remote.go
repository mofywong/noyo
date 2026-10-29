package inspection

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// RemoteCommand travels on the authenticated cascade gateway command topic.
// Commands only admit/query work; MQTT callbacks never run native inference.
type RemoteCommand struct {
	Action     string    `json:"action"`
	Gateway    string    `json:"gateway"`
	BatchID    string    `json:"batch_id"`
	RequestID  string    `json:"request_id"`
	Generation string    `json:"generation"`
	Epoch      int64     `json:"epoch"`
	Deadline   time.Time `json:"deadline"`
	Scope      Scope     `json:"scope"`
	Cameras    []Camera  `json:"cameras,omitempty"`
	CameraCode string    `json:"camera_code,omitempty"`
}
type RemoteReply struct {
	Status       string         `json:"status"`
	Stage        string         `json:"stage,omitempty"`
	ProgressJSON string         `json:"progress_json,omitempty"`
	Generation   string         `json:"generation"`
	Analysis     *Analysis      `json:"analysis,omitempty"`
	Error        string         `json:"error,omitempty"`
	Capabilities map[string]any `json:"capabilities,omitempty"`
}
type RemoteHandler interface {
	HandleInspectionCommand(RemoteCommand) (RemoteReply, error)
}

type RemoteBatchRecord struct {
	ID          string `gorm:"primaryKey;size:64"`
	Gateway     string
	Epoch       int64
	Generation  string
	Hash        string
	Scope       `gorm:"embedded"`
	CamerasJSON string `gorm:"type:text"`
	Status      string `gorm:"index"`
	Error       string
	Deadline    time.Time
	CreatedAt   time.Time
}

func (RemoteBatchRecord) TableName() string { return "inspection_remote_batches" }

type RemoteCheck struct {
	ID           string `gorm:"primaryKey;size:64"`
	BatchID      string `gorm:"uniqueIndex:inspection_remote_camera"`
	CameraCode   string `gorm:"uniqueIndex:inspection_remote_camera"`
	Status       string `gorm:"index"`
	Stage        string
	ProgressJSON string `gorm:"type:text"`
	ResultJSON   string `gorm:"type:text"`
	Error        string
	CreatedAt    time.Time
}

func (RemoteCheck) TableName() string { return "inspection_remote_checks" }

type RemoteEpoch struct {
	Gateway string `gorm:"primaryKey"`
	Epoch   int64
}

func (RemoteEpoch) TableName() string { return "inspection_remote_epochs" }

// RemoteExecutor persists idempotency before admission and fences old owners.
// Provider must serialize physical-host batches, including local inspections.
type RemoteExecutor struct {
	db         *gorm.DB
	provider   Provider
	validate   func(RemoteCommand) error
	mu         sync.Mutex
	wg         sync.WaitGroup
	ctx        context.Context
	cancel     context.CancelFunc
	active     bool
	generation string
	cancels    map[string]context.CancelFunc
}

func NewRemoteExecutor(db *gorm.DB, provider Provider, validate func(RemoteCommand) error) (*RemoteExecutor, error) {
	if err := db.AutoMigrate(&RemoteBatchRecord{}, &RemoteCheck{}, &RemoteEpoch{}); err != nil {
		return nil, err
	}
	return &RemoteExecutor{db: db, provider: provider, validate: validate, cancels: map[string]context.CancelFunc{}}, nil
}
func (e *RemoteExecutor) Start() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.active {
		return nil
	}
	if err := e.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&RemoteCheck{}).Where("status IN ?", []string{"pending", "running"}).Updates(map[string]any{"status": "failed", "error": "execution_interrupted"}).Error; err != nil {
			return err
		}
		return tx.Model(&RemoteBatchRecord{}).Where("status IN ?", []string{"queued", "active", "closing"}).Updates(map[string]any{"status": "interrupted", "error": "execution_interrupted"}).Error
	}); err != nil {
		return err
	}
	e.ctx, e.cancel = context.WithCancel(context.Background())
	e.generation = uuid.NewString()
	e.active = true
	return nil
}
func (e *RemoteExecutor) Stop() {
	e.mu.Lock()
	e.active = false
	if e.cancel != nil {
		e.cancel()
	}
	e.mu.Unlock()
	e.wg.Wait()
}
func remoteHash(v any) string {
	sum := sha256.Sum256([]byte(encoded(v)))
	return hex.EncodeToString(sum[:])
}
func (e *RemoteExecutor) Handle(c RemoteCommand) (RemoteReply, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	reply := RemoteReply{Generation: e.generation}
	if !e.active {
		return reply, errors.New("inspection_unavailable")
	}
	if c.BatchID == "" || len(c.BatchID) > 64 || c.Epoch <= 0 || c.Gateway == "" {
		return reply, ErrInvalid
	}
	if c.Generation != "" && c.Generation != e.generation {
		return reply, errors.New("inspection_remote_restarted")
	}
	var batch RemoteBatchRecord
	err := e.db.First(&batch, "id = ?", c.BatchID).Error
	if c.Action == "open" {
		hash := remoteHash(struct {
			Scope    Scope
			Cameras  []Camera
			Gateway  string
			Epoch    int64
			Deadline time.Time
		}{c.Scope, c.Cameras, c.Gateway, c.Epoch, c.Deadline})
		if err == nil {
			if batch.Hash != hash {
				return reply, ErrConflict
			}
			return RemoteReply{Status: batch.Status, Generation: batch.Generation, Error: batch.Error}, nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return reply, err
		}
		if c.Scope.TenantID == 0 || c.Scope.ProjectID == 0 || !c.Deadline.After(time.Now()) || c.Deadline.After(time.Now().Add(12*time.Hour)) {
			return reply, ErrInvalid
		}
		spec := Spec{Name: "Remote inspection", Mode: "ai", Cameras: c.Cameras, Timezone: "UTC"}
		if err := Validate(&spec); err != nil {
			return reply, err
		}
		if e.validate == nil {
			return reply, errors.New("execution_grant_revoked")
		}
		if err := e.validate(c); err != nil {
			return reply, err
		}
		var count int64
		if err := e.db.Model(&RemoteBatchRecord{}).Where("status IN ?", []string{"queued", "active", "closing"}).Count(&count).Error; err != nil {
			return reply, err
		}
		if count >= 16 {
			return reply, errors.New("inspection_gateway_busy")
		}
		batch = RemoteBatchRecord{ID: c.BatchID, Gateway: c.Gateway, Epoch: c.Epoch, Generation: e.generation, Hash: hash, Scope: c.Scope, CamerasJSON: encoded(c.Cameras), Status: "queued", Deadline: c.Deadline}
		if err := e.db.Transaction(func(tx *gorm.DB) error {
			var fence RemoteEpoch
			if err := tx.First(&fence, "gateway = ?", c.Gateway).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			if fence.Epoch >= c.Epoch {
				return errors.New("inspection_remote_fenced")
			}
			if err := tx.Save(&RemoteEpoch{Gateway: c.Gateway, Epoch: c.Epoch}).Error; err != nil {
				return err
			}
			if err := tx.Model(&RemoteBatchRecord{}).Where("gateway = ? AND epoch < ? AND status IN ?", c.Gateway, c.Epoch, []string{"queued", "active"}).Update("status", "closing").Error; err != nil {
				return err
			}
			return tx.Create(&batch).Error
		}); err != nil {
			return reply, err
		}
		for _, cancel := range e.cancels {
			cancel()
		}
		ctx, cancel := context.WithDeadline(e.ctx, c.Deadline)
		e.cancels[c.BatchID] = cancel
		e.wg.Add(1)
		go e.execute(ctx, cancel, batch)
		reply.Status = "queued"
		return reply, nil
	}
	if err != nil {
		return reply, err
	}
	if batch.Gateway != c.Gateway || batch.Epoch != c.Epoch || batch.Scope != c.Scope {
		return reply, ErrConflict
	}
	if batch.Generation != e.generation {
		return RemoteReply{Status: "interrupted", Generation: e.generation, Error: "execution_interrupted"}, nil
	}
	reply.Status = batch.Status
	reply.Error = batch.Error
	if c.Action == "close" || c.Action == "cancel" {
		if batch.Status == "active" || batch.Status == "queued" {
			if err := e.db.Model(&batch).Update("status", "closing").Error; err != nil {
				return reply, err
			}
			if cancel := e.cancels[batch.ID]; cancel != nil {
				cancel()
			}
			reply.Status = "closing"
		}
		return reply, nil
	}
	if c.Action != "status" && c.Action != "analyze" {
		return reply, ErrInvalid
	}
	if c.RequestID == "" {
		if c.Action == "status" {
			return reply, nil
		}
		return reply, ErrInvalid
	}
	if len(c.RequestID) > 64 {
		return reply, ErrInvalid
	}
	var check RemoteCheck
	err = e.db.First(&check, "id = ?", c.RequestID).Error
	if c.Action == "analyze" && errors.Is(err, gorm.ErrRecordNotFound) {
		if batch.Status != "active" || !batch.Deadline.After(time.Now()) {
			return reply, errors.New("inspection_remote_fenced")
		}
		var cameras []Camera
		_ = json.Unmarshal([]byte(batch.CamerasJSON), &cameras)
		found := false
		for _, camera := range cameras {
			if camera.Code == c.CameraCode {
				found = true
			}
		}
		if !found {
			return reply, ErrInvalid
		}
		check = RemoteCheck{ID: c.RequestID, BatchID: batch.ID, CameraCode: c.CameraCode, Status: "pending"}
		err = e.db.Create(&check).Error
	}
	if err != nil {
		return reply, err
	}
	if check.BatchID != batch.ID || (c.CameraCode != "" && check.CameraCode != c.CameraCode) {
		return reply, ErrConflict
	}
	reply.Status = check.Status
	reply.Stage = check.Stage
	reply.ProgressJSON = check.ProgressJSON
	reply.Error = check.Error
	if check.Status == "completed" {
		var result Analysis
		if err := json.Unmarshal([]byte(check.ResultJSON), &result); err != nil {
			return reply, err
		}
		reply.Analysis = &result
	}
	return reply, nil
}
func (e *RemoteExecutor) execute(ctx context.Context, cancel context.CancelFunc, row RemoteBatchRecord) {
	defer e.wg.Done()
	defer cancel()
	status, message := "closed", ""
	defer func() {
		e.mu.Lock()
		defer e.mu.Unlock()
		delete(e.cancels, row.ID)
		_ = e.db.Model(&RemoteCheck{}).Where("batch_id = ? AND status IN ?", row.ID, []string{"pending", "running"}).Updates(map[string]any{"status": "failed", "error": "execution_interrupted"}).Error
		_ = e.db.Model(&RemoteBatchRecord{}).Where("id = ?", row.ID).Updates(map[string]any{"status": status, "error": message}).Error
	}()
	var cameras []Camera
	_ = json.Unmarshal([]byte(row.CamerasJSON), &cameras)
	if e.provider == nil {
		status = "failed"
		message = "provider_unavailable"
		return
	}
	batch, err := e.provider.Open(ctx, "local:gb28181", cameras)
	if batch != nil {
		defer batch.Close()
	}
	if err != nil || batch == nil {
		status = "failed"
		message = "inspection_model_unavailable"
		return
	}
	if ctx.Err() != nil {
		return
	}
	if result := e.db.Model(&RemoteBatchRecord{}).Where("id = ? AND status = ?", row.ID, "queued").Update("status", "active"); result.Error != nil || result.RowsAffected != 1 {
		return
	}
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		var request RemoteCheck
		if err := e.db.Where("batch_id = ? AND status = ?", row.ID, "pending").Order("created_at").First(&request).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				continue
			}
			status = "failed"
			message = "inspection_request_failed"
			return
		}
		var camera Camera
		for _, c := range cameras {
			if c.Code == request.CameraCode {
				camera = c
				break
			}
		}
		if result := e.db.Model(&RemoteCheck{}).Where("id = ? AND status = ?", request.ID, "pending").Update("status", "running"); result.Error != nil || result.RowsAffected != 1 {
			continue
		}
		itemCtx, itemCancel := context.WithTimeout(ctx, CameraTimeout(camera))
		itemCtx = WithObservationProgress(itemCtx, func(progress ObservationProgress) error {
			return e.db.Model(&RemoteCheck{}).Where("id = ? AND status = ?", request.ID, "running").UpdateColumn("progress_json", encoded(progress)).Error
		})
		lastStage := ""
		itemCtx = WithProgress(itemCtx, func(stage string) error {
			if lastStage == stage {
				return nil
			}
			if err := e.db.Model(&RemoteCheck{}).Where("id = ? AND status = ?", request.ID, "running").UpdateColumn("stage", stage).Error; err != nil {
				return err
			}
			lastStage = stage
			return nil
		})
		var analysis Analysis
		var err error
		if e.validate != nil {
			err = e.validate(RemoteCommand{Gateway: row.Gateway, Scope: row.Scope, Cameras: []Camera{camera}})
		}
		if err == nil {
			analysis, err = batch.Analyze(itemCtx, camera)
		}
		itemCancel()
		values := map[string]any{"status": "completed", "result_json": encoded(analysis)}
		if err != nil {
			values = map[string]any{"status": "failed", "error": inspectionErrorCode(err)}
		}
		if err := e.db.Model(&RemoteCheck{}).Where("id = ?", request.ID).Updates(values).Error; err != nil {
			status = "failed"
			message = "inspection_request_failed"
			return
		}
	}
}
func inspectionErrorCode(err error) string {
	if errors.Is(err, context.Canceled) {
		return "execution_interrupted"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "inspection_frame_timeout"
	}
	return err.Error()
}
