package cascade

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"gorm.io/gorm"
	"noyo/core/semanticreview"
	"noyo/core/store"
	"noyo/core/types"
)

const semanticWireLimit = 6 * 1024 * 1024

type semanticUp struct {
	Type       string                    `json:"type"`
	DeviceCode string                    `json:"device_code"`
	Candidate  *semanticreview.Candidate `json:"candidate,omitempty"`
}
type semanticDown struct {
	DeviceCode string                 `json:"device_code"`
	Status     *semanticreview.Status `json:"status,omitempty"`
	Result     *semanticreview.Result `json:"result,omitempty"`
}
type semanticOutbox struct {
	ID                   string `gorm:"primaryKey;size:64"`
	DeviceCode           string `gorm:"index;size:256"`
	CreatedAt, ExpiresAt time.Time
	Payload              []byte
	Result               []byte
	Bytes                int64
	State                string `gorm:"index;size:32"`
}

func (semanticOutbox) TableName() string { return "cascade_semantic_outbox" }

type semanticGateway struct {
	engine         *gatewayEngineImpl
	queue          chan semanticreview.Candidate
	down           chan semanticDown
	statusRequests chan string
	mu             sync.Mutex
	bytes          int64
	statuses       map[string]semanticreview.Status
	requested      map[string]time.Time
	wg             sync.WaitGroup
}

// ExecutionRole exposes the active mode; the mere presence of Cascade must not
// steal platform-local candidates from the Copilot executor.
func (p *CascadePlugin) ExecutionRole() string { return p.Config.Mode }
func (p *CascadePlugin) SubmitCandidate(c semanticreview.Candidate) semanticreview.Submission {
	if p.ExecutionRole() != "gateway" {
		return semanticreview.Submission{ID: c.ID, Status: "disabled", Reason: "gateway_mode_required"}
	}
	e, ok := p.GatewayEngine.(*gatewayEngineImpl)
	if !ok || e.semantic == nil {
		return semanticreview.Submission{ID: c.ID, Status: "failed", Reason: "gateway_semantic_router_unavailable"}
	}
	if c.Execution != "platform" {
		if local := p.localSemanticClient(); local != nil {
			status := local.LocalSemanticStatus(c.DeviceCode)
			if status.Status == "ready" {
				// Local and remote execution are strictly sequential. Only the local
				// worker may initiate a fallback after its reserved call finishes.
				return local.SubmitCandidate(c)
			}
			reason := localUnavailableReason(status)
			if reason == "" {
				return semanticreview.Submission{ID: c.ID, Status: status.Status, Reason: "local_admission_unavailable"}
			}
			c.FallbackReason = reason
		} else {
			c.FallbackReason = "local_no_provider"
		}
		c.Execution = "platform"
	}
	if c.PlatformExecutorEpoch == "" {
		e.semantic.mu.Lock()
		c.PlatformExecutorEpoch = e.semantic.statuses[c.DeviceCode].ExecutorEpoch
		e.semantic.mu.Unlock()
	}
	return e.semantic.submit(c)
}
func (p *CascadePlugin) GetResults(device string) []semanticreview.Result {
	if p.ExecutionRole() != "gateway" || store.DB == nil {
		return []semanticreview.Result{}
	}
	var rows []semanticOutbox
	store.DB.Where("device_code = ?", device).Order("created_at desc").Limit(50).Find(&rows)
	out := make([]semanticreview.Result, 0, len(rows))
	for _, row := range rows {
		var r semanticreview.Result
		if len(row.Result) > 0 {
			if json.Unmarshal(row.Result, &r) != nil {
				continue
			}
		} else {
			var c semanticreview.Candidate
			if json.Unmarshal(row.Payload, &c) != nil {
				continue
			}
			r = semanticreview.NewResult(c, row.State)
			r.CompletedAt = time.Time{}
		}
		out = append(out, r)
	}
	if local := p.localSemanticClient(); local != nil {
		out = mergedSemanticResults(local.GetResults(device), out)
	}
	return out
}
func (p *CascadePlugin) SemanticStatus(device string) semanticreview.Status {
	r := p.PlatformSemanticStatus(device)
	r.Execution = "local_first"
	r.PlatformExecutorEpoch = r.ExecutorEpoch
	if local := p.localSemanticClient(); local != nil {
		status := local.LocalSemanticStatus(device)
		r.LocalExecutorEpoch = status.ExecutorEpoch
		if status.Status == "ready" || localUnavailableReason(status) == "" {
			r.Enabled = status.Enabled
			r.Status = status.Status
			r.ExecutorEpoch = status.ExecutorEpoch
		}
	}
	r.Results = p.GetResults(device)
	r.Pending = 0
	for _, result := range r.Results {
		if result.Status == "pending" || result.Status == "queued" || result.Status == "running" || result.Status == "forwarding" || result.Status == "buffered" {
			r.Pending++
		}
	}
	return r
}
func (p *CascadePlugin) PlatformSemanticStatus(device string) semanticreview.Status {
	r := semanticreview.Status{Status: "disconnected", Execution: "platform", Results: p.GetResults(device)}
	e, ok := p.GatewayEngine.(*gatewayEngineImpl)
	if p.ExecutionRole() != "gateway" || !ok || e.semantic == nil {
		r.Status = "disabled"
		return r
	}
	s := e.semantic
	s.mu.Lock()
	if cached, exists := s.statuses[device]; exists {
		r.Enabled = cached.Enabled
		r.Status = cached.Status
		r.ExecutorEpoch = cached.ExecutorEpoch
	}
	_, known := s.requested[device]
	if time.Since(s.requested[device]) > 5*time.Second && (known || len(s.requested) < 256) {
		select {
		case s.statusRequests <- device:
			s.requested[device] = time.Now()
		default:
		}
	}
	s.mu.Unlock()
	if e.client == nil || !e.client.IsConnected() || !e.platformOnline.Load() {
		r.Status = "disconnected"
		r.Enabled = false
	}
	for _, result := range r.Results {
		if result.Status == "pending" || result.Status == "queued" || result.Status == "buffered" {
			r.Pending++
		}
	}
	return r
}
func (e *gatewayEngineImpl) initSemanticGateway() error {
	if store.DB == nil {
		return errors.New("semantic outbox database unavailable")
	}
	if err := store.DB.AutoMigrate(&semanticOutbox{}); err != nil {
		return err
	}
	e.semantic = &semanticGateway{engine: e, queue: make(chan semanticreview.Candidate, 100), down: make(chan semanticDown, 100), statusRequests: make(chan string, 100), statuses: map[string]semanticreview.Status{}, requested: map[string]time.Time{}}
	return nil
}
func (s *semanticGateway) submit(c semanticreview.Candidate) semanticreview.Submission {
	if err := semanticreview.ValidateCandidate(c, semanticreview.Limits{}, time.Now()); err != nil {
		return semanticreview.Submission{ID: c.ID, Status: "invalid_evidence", Reason: err.Error()}
	}
	c.GatewayCode = s.engine.gatewayCode
	c.InferenceSite = "gateway"
	c.Execution = "platform"
	var size int64
	for _, f := range c.Evidence {
		size += int64(len(f.Data))
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.queue) == cap(s.queue) || s.bytes+size > 64*1024*1024 {
		return semanticreview.Submission{ID: c.ID, Status: "queue_full", Reason: "gateway_memory_limit"}
	}
	c.Scores = append([]semanticreview.Score(nil), c.Scores...)
	c.TargetBoxes = append([][4]float32(nil), c.TargetBoxes...)
	c.Evidence = append([]semanticreview.Frame(nil), c.Evidence...)
	for i := range c.Evidence {
		c.Evidence[i].Data = append([]byte(nil), c.Evidence[i].Data...)
		c.Evidence[i].ROI = append([][2]float32(nil), c.Evidence[i].ROI...)
	}
	s.queue <- c
	s.bytes += size
	return semanticreview.Submission{ID: c.ID, Status: "pending", Reason: "gateway_buffering"}
}
func semanticKey(id string) string { h := sha256.Sum256([]byte(id)); return hex.EncodeToString(h[:]) }
func (s *semanticGateway) loop(ctx context.Context) {
	defer s.wg.Done()
	tick := time.NewTicker(5 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case c := <-s.queue:
			var size int64
			for _, f := range c.Evidence {
				size += int64(len(f.Data))
			}
			s.mu.Lock()
			s.bytes -= size
			s.mu.Unlock()
			if err := s.persist(c); err != nil {
				r := semanticreview.NewResult(c, "queue_full")
				r.ReasonCodes = []string{"gateway_persistence_or_queue_limit"}
				s.publishResult(r)
			}
		case down := <-s.down:
			s.receive(down)
		case device := <-s.statusRequests:
			s.publish(semanticUp{Type: "status", DeviceCode: device})
		case <-tick.C:
			s.replay()
		}
	}
}
func (s *semanticGateway) persist(c semanticreview.Candidate) error {
	b, err := json.Marshal(c)
	if err != nil {
		return err
	}
	row := semanticOutbox{ID: semanticKey(c.ID), DeviceCode: c.DeviceCode, CreatedAt: time.Now().UTC(), ExpiresAt: c.ExpiresAt, Payload: b, Bytes: int64(len(b)), State: "buffered"}
	err = store.DB.Transaction(func(tx *gorm.DB) error {
		var old semanticOutbox
		if tx.First(&old, "id = ?", row.ID).Error == nil {
			return nil
		}
		var count, total int64
		if err := tx.Model(&semanticOutbox{}).Where("bytes > 0").Count(&count).Error; err != nil {
			return err
		}
		if err := tx.Model(&semanticOutbox{}).Select("COALESCE(SUM(bytes),0)").Scan(&total).Error; err != nil {
			return err
		}
		if count >= 100 || total+row.Bytes > 64*1024*1024 {
			return errors.New("gateway_durable_queue_limit")
		}
		return tx.Create(&row).Error
	})
	if err == nil {
		s.publish(semanticUp{Type: "candidate", DeviceCode: c.DeviceCode, Candidate: &c})
	}
	return err
}
func (s *semanticGateway) publish(up semanticUp) {
	e := s.engine
	if e.client == nil || !e.client.IsConnected() || !e.platformOnline.Load() {
		return
	}
	b, err := json.Marshal(up)
	if err != nil || len(b) > semanticWireLimit {
		return
	}
	e.client.Publish(fmt.Sprintf("noyo/cascade/gw/%s/semantic/up", e.gatewayCode), 1, false, b)
}
func (s *semanticGateway) replay() {
	var rows []semanticOutbox
	store.DB.Where("bytes > 0").Order("created_at").Limit(100).Find(&rows)
	for _, row := range rows {
		var c semanticreview.Candidate
		if json.Unmarshal(row.Payload, &c) != nil {
			continue
		}
		if time.Now().After(row.ExpiresAt) {
			r := semanticreview.NewResult(c, "expired")
			r.AttemptsUsed = min(2-(c.Round-1), c.AttemptsUsed+1) // unknown remote billing retains its possible slot
			r.ReasonCodes = []string{"gateway_candidate_expired"}
			s.receive(semanticDown{DeviceCode: c.DeviceCode, Result: &r})
			continue
		}
		s.publish(semanticUp{Type: "candidate", DeviceCode: c.DeviceCode, Candidate: &c})
	}
	// Completed evidence bytes are cleared immediately; history itself is bounded.
	store.DB.Where("created_at < ? AND bytes = 0", time.Now().Add(-7*24*time.Hour)).Delete(&semanticOutbox{})
	var count int64
	store.DB.Model(&semanticOutbox{}).Count(&count)
	if count > 1000 {
		var stale []semanticOutbox
		store.DB.Where("bytes = 0").Order("created_at").Limit(int(count - 1000)).Find(&stale)
		for _, row := range stale {
			store.DB.Delete(&semanticOutbox{}, "id = ?", row.ID)
		}
	}
}
func (s *semanticGateway) receive(down semanticDown) {
	if down.Status != nil {
		s.mu.Lock()
		if len(s.statuses) < 256 || s.statuses[down.DeviceCode].Status != "" {
			status := *down.Status
			status.Results = nil
			s.statuses[down.DeviceCode] = status
		}
		s.mu.Unlock()
		return
	}
	if down.Result == nil {
		return
	}
	r := *down.Result
	var row semanticOutbox
	if store.DB.First(&row, "id = ?", semanticKey(r.ID)).Error != nil {
		return
	}
	var c semanticreview.Candidate
	if json.Unmarshal(row.Payload, &c) != nil || r.DeviceCode != c.DeviceCode || down.DeviceCode != c.DeviceCode || r.IncidentID != c.IncidentID || r.RuleID != c.RuleID || r.RuleRevision != c.RuleRevision || r.StreamSession != c.StreamSession || r.TargetGeneration != c.TargetGeneration || r.TargetID != c.TargetID || r.ExecutorEpoch != c.ExecutorEpoch || r.GatewayCode != s.engine.gatewayCode || r.Execution != "platform" {
		return
	}
	if r.Scene != c.Scene || r.AttemptsUsed < c.AttemptsUsed || r.AttemptsUsed+c.Round-1 > 2 || (c.PlatformExecutorEpoch != "" && r.ActualExecutorEpoch != "" && r.ActualExecutorEpoch != c.PlatformExecutorEpoch) {
		return
	}
	if (r.Verdict == "confirmed" || r.Verdict == "rejected" || r.Verdict == "insufficient_evidence") && (r.ActualExecutorEpoch == "" || r.AttemptsUsed <= c.AttemptsUsed) {
		return
	}
	if time.Now().After(c.ExpiresAt) {
		r.Status = "expired"
		r.Verdict = ""
		r.DecisionSource = "local_model"
	}
	b, _ := json.Marshal(r)
	metadata := c
	metadata.Evidence = append([]semanticreview.Frame(nil), c.Evidence...)
	for i := range metadata.Evidence {
		metadata.Evidence[i].Data = nil
	}
	payload, _ := json.Marshal(metadata)
	cas := store.DB.Model(&semanticOutbox{}).Where("id = ? AND bytes > 0", row.ID).Updates(map[string]any{"result": b, "payload": payload, "bytes": 0, "state": r.Status})
	if cas.Error == nil && cas.RowsAffected == 1 {
		s.publishResult(r)
	}
}
func (s *semanticGateway) publishResult(r semanticreview.Result) {
	s.engine.ctx.PublishEvent(types.Event{Type: types.EventType(semanticreview.ResultEvent), Topic: r.DeviceCode, Payload: r, Timestamp: time.Now().UnixMilli()})
}
func (e *gatewayEngineImpl) handleSemanticDown(_ mqtt.Client, msg mqtt.Message) {
	if e.semantic == nil || msg.Retained() || len(msg.Payload()) > 65536 {
		return
	}
	var down semanticDown
	if json.Unmarshal(msg.Payload(), &down) != nil || down.DeviceCode == "" {
		return
	}
	select {
	case e.semantic.down <- down:
	default:
	}
}

// The broker's existing authenticated gateway topic ACL is the transport
// boundary. Independently verify a live, enabled device and its scoped ancestry
// before accepting a candidate or returning any provider configuration status.
func semanticGatewayOwns(db *gorm.DB, gateway, device string) bool {
	if db == nil || gateway == "" || device == "" {
		return false
	}
	var gw store.Device
	if db.Where("code = ? AND enabled = ?", gateway, true).First(&gw).Error != nil || gw.ProductCode != "noyo-gw" || gw.TenantID == 0 || gw.ProjectID == 0 {
		return false
	}
	seen := map[string]bool{}
	code := device
	for depth := 0; code != "" && depth < 16; depth++ {
		if seen[code] {
			return false
		}
		seen[code] = true
		var d store.Device
		if db.Where("code = ? AND enabled = ?", code, true).First(&d).Error != nil || d.TenantID != gw.TenantID || d.ProjectID != gw.ProjectID {
			return false
		}
		if d.Code == gateway {
			return true
		}
		code = d.ParentCode
	}
	return false
}
func (e *platformEngineImpl) handleSemanticUp(_ mqtt.Client, msg mqtt.Message) {
	if msg.Retained() || len(msg.Payload()) > semanticWireLimit {
		return
	}
	matched, gateway := parseTopicGwSn(msg.Topic(), "")
	if !matched || msg.Topic() != fmt.Sprintf("noyo/cascade/gw/%s/semantic/up", gateway) || strings.ContainsAny(gateway, "+#") {
		return
	}
	var up semanticUp
	if json.Unmarshal(msg.Payload(), &up) != nil || !semanticGatewayOwns(store.DB, gateway, up.DeviceCode) {
		return
	}
	client, ok := e.ctx.GetPlugin("ai_copilot").(semanticreview.Client)
	if up.Type == "status" {
		status := semanticreview.Status{Status: "no_provider", Execution: "platform", Results: []semanticreview.Result{}}
		if ok {
			status = client.SemanticStatus(up.DeviceCode)
		}
		status.Execution = "platform"
		status.PlatformExecutorEpoch = status.ExecutorEpoch
		status.LocalExecutorEpoch = ""
		status.Results = nil
		e.sendSemanticDown(gateway, semanticDown{DeviceCode: up.DeviceCode, Status: &status})
		return
	}
	if up.Type != "candidate" || up.Candidate == nil {
		return
	}
	c := *up.Candidate
	if c.DeviceCode != up.DeviceCode || c.GatewayCode != gateway || c.Execution != "platform" {
		return
	}
	c.InferenceSite = "gateway"
	sub := semanticreview.Submission{ID: c.ID, Status: "no_provider", Reason: "platform_visual_executor_unavailable"}
	if ok {
		sub = client.SubmitCandidate(c)
	}
	// Pending is only an in-memory receipt. The gateway retains evidence until
	// the durable, final result arrives, then QoS replay remains idempotent.
	if sub.Status != "pending" {
		r := semanticreview.NewResult(c, sub.Status)
		r.ReasonCodes = []string{sub.Reason}
		e.sendSemanticDown(gateway, semanticDown{DeviceCode: c.DeviceCode, Result: &r})
	}
}
func (e *platformEngineImpl) sendSemanticDown(gateway string, down semanticDown) {
	if e.client == nil || !e.client.IsConnected() {
		return
	}
	b, err := json.Marshal(down)
	if err != nil || len(b) > 65536 {
		return
	}
	e.client.Publish(fmt.Sprintf("noyo/cascade/gw/%s/semantic/down", gateway), 1, false, b)
}
func (e *platformEngineImpl) startSemanticResults() {
	e.semanticSubscription = e.ctx.SubscribeEvent(types.EventType(semanticreview.ResultEvent), func(event types.Event) {
		r, ok := event.Payload.(semanticreview.Result)
		if !ok || r.Execution != "platform" || r.GatewayCode == "" || !semanticGatewayOwns(store.DB, r.GatewayCode, r.DeviceCode) {
			return
		}
		e.sendSemanticDown(r.GatewayCode, semanticDown{DeviceCode: r.DeviceCode, Result: &r})
	})
}
