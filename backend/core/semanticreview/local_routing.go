package semanticreview

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"time"
)

// UnavailableError identifies transport/service unavailability, never a model
// verdict, malformed JSON, rate limit, or exhausted monetary budget.
type UnavailableError struct {
	Reason string
	Err    error
}

func (e *UnavailableError) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	return e.Reason
}
func (e *UnavailableError) Unwrap() error { return e.Err }
func FallbackReason(err error) string {
	var unavailable *UnavailableError
	if errors.As(err, &unavailable) {
		switch unavailable.Reason {
		case "local_disabled", "local_no_provider", "local_connection_failed", "local_timeout", "local_service_unavailable":
			return unavailable.Reason
		}
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "local_timeout"
	}
	var network net.Error
	if errors.As(err, &network) {
		if network.Timeout() {
			return "local_timeout"
		}
		return "local_connection_failed"
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return "local_connection_failed"
	}
	return ""
}

// SetFallback is configured before Start. The callback admits only an explicit
// platform hop, which cannot re-enter local execution.
func (s *Service) SetFallback(callback func(Candidate) Submission) { s.fallback = callback }

func requestAttempts(j job, c Candidate) int { return max(c.AttemptsUsed, j.TotalAttempts-(c.Round-1)) }
func candidateExecutionEpoch(c Candidate) string {
	if c.Execution == "local_first" {
		if c.LocalExecutorEpoch != "" {
			return c.LocalExecutorEpoch
		}
		return c.ExecutorEpoch
	}
	if c.PlatformExecutorEpoch != "" {
		return c.PlatformExecutorEpoch
	}
	if c.FallbackReason == "" {
		return c.ExecutorEpoch
	} // legacy platform-only request
	return "" // bind an unavailable/offline platform at admission, then check again after IO
}
func (s *Service) forward(j job, c Candidate, reason string) bool {
	if c.Execution != "local_first" || s.fallback == nil || reason == "" || j.TotalAttempts >= 2 || !time.Now().Before(c.ExpiresAt) || s.ctx.Err() != nil {
		return false
	}
	c.Execution = "platform"
	c.AttemptsUsed = requestAttempts(j, c)
	c.FallbackReason = reason
	b, _ := json.Marshal(c)
	cas := s.db.Model(&job{}).Where("id = ? AND (status = ? OR (status = ? AND owner = ?))", j.ID, "queued", "running", s.owner).Updates(map[string]any{"status": "forwarding", "payload": b, "bytes": len(b), "owner": s.owner, "lease_until": time.Now().Add(5 * time.Second)})
	if cas.Error != nil || cas.RowsAffected != 1 {
		return false
	}
	s.dispatchForward(j, c)
	return true
}
func (s *Service) dispatchForward(j job, c Candidate) {
	if s.fallback == nil {
		return
	} // Retain durable evidence until a router starts.
	sub := s.fallback(c)
	if sub.Status == "pending" {
		return
	}
	r := NewResult(c, sub.Status)
	r.ReasonCodes = []string{sub.Reason}
	r.AttemptsUsed = requestAttempts(j, c)
	s.finish(j, c, r)
}
func (s *Service) replayForwarding() {
	var rows []job
	s.db.Where("status = ? AND lease_until < ?", "forwarding", time.Now()).Limit(100).Find(&rows)
	for _, j := range rows {
		var c Candidate
		if json.Unmarshal(j.Payload, &c) != nil {
			continue
		}
		if !time.Now().Before(c.ExpiresAt) {
			r := NewResult(c, "result_unknown")
			r.ReasonCodes = []string{"platform_result_not_received"}
			// The forwarded HTTP request may already have been billed. Lost results
			// never release the possible remote slot.
			r.AttemptsUsed = min(2-(c.Round-1), requestAttempts(j, c)+1)
			s.finish(j, c, r)
			continue
		}
		cas := s.db.Model(&job{}).Where("id = ? AND status = ? AND lease_until < ?", j.ID, "forwarding", time.Now()).Update("lease_until", time.Now().Add(5*time.Second))
		if cas.Error == nil && cas.RowsAffected == 1 {
			s.dispatchForward(j, c)
		}
	}
}

// CompleteFallback consumes an authenticated Cascade result. It does not call
// the model, initiate another hop, or replace the original correlation epoch.
func (s *Service) CompleteFallback(r Result) {
	if r.Execution != "platform" {
		return
	}
	var rows []job
	s.db.Where("device_code = ? AND status = ?", r.DeviceCode, "forwarding").Limit(100).Find(&rows)
	for _, j := range rows {
		var c Candidate
		if json.Unmarshal(j.Payload, &c) != nil || c.ID != r.ID || c.ExecutorEpoch != r.ExecutorEpoch || c.IncidentID != r.IncidentID || c.RuleID != r.RuleID || c.RuleRevision != r.RuleRevision || c.Scene != r.Scene || c.TargetID != r.TargetID || c.TargetGeneration != r.TargetGeneration || c.StreamSession != r.StreamSession || c.GatewayCode != r.GatewayCode || r.AttemptsUsed < requestAttempts(j, c) || r.AttemptsUsed+c.Round-1 > 2 {
			continue
		}
		if c.PlatformExecutorEpoch != "" && r.ActualExecutorEpoch != "" && c.PlatformExecutorEpoch != r.ActualExecutorEpoch {
			continue
		}
		if (r.Verdict == "confirmed" || r.Verdict == "rejected" || r.Verdict == "insufficient_evidence") && (r.ActualExecutorEpoch == "" || r.AttemptsUsed <= c.AttemptsUsed) {
			continue
		}
		s.finishResult(j, c, r, false) // Cascade already publishes the authenticated final event.
		return
	}
}
