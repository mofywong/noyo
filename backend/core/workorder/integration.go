package workorder

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Connector struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	KeyID      string `json:"key_id"`
	SecretHash string `json:"-"`
	TenantID   uint   `json:"tenant_id"`
	ProjectID  uint   `json:"project_id"`
	Enabled    bool   `json:"enabled"`
}
type Binding struct {
	ID            string `json:"id"`
	ConnectorID   string `json:"connector_id"`
	RemoteProject string `json:"remote_project"`
	TenantID      uint   `json:"tenant_id"`
	ProjectID     uint   `json:"project_id"`
	Origin        string `json:"origin"`
	Direction     string `json:"direction"`
	LastRevision  int64  `json:"last_revision"`
}
type Mapping struct {
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	Revision int64             `json:"revision"`
	Fields   map[string]string `json:"fields"`
	Status   map[string]string `json:"status"`
	Priority map[string]string `json:"priority"`
}
type InboxMessage struct {
	BindingID  string    `json:"binding_id"`
	MessageID  string    `json:"message_id"`
	Digest     string    `json:"digest"`
	Revision   int64     `json:"revision"`
	Status     string    `json:"status"`
	Error      string    `json:"error,omitempty"`
	ReceivedAt time.Time `json:"received_at"`
}

const (
	InboxProcessing = "processing"
	InboxApplied    = "applied"
	InboxDuplicate  = "duplicate"
	InboxRejected   = "rejected"
)

type IntegrationStore struct {
	mu       sync.Mutex
	inbox    map[string]InboxMessage
	bindings map[string]Binding
	outbox   *OutboxStore
}

func NewIntegrationStore() *IntegrationStore {
	return &IntegrationStore{inbox: map[string]InboxMessage{}, bindings: map[string]Binding{}, outbox: NewOutboxStore()}
}
func (s *IntegrationStore) PutBinding(binding Binding) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.bindings[binding.ID] = binding
}
func (s *IntegrationStore) Ingest(binding Binding, messageID string, revision int64, body []byte, apply func() error) (InboxMessage, error) {
	if binding.ID == "" || messageID == "" {
		return InboxMessage{}, NewError(CodeValidationFailed, "binding and message id are required")
	}
	digest := sha256.Sum256(body)
	key := binding.ID + ":" + messageID
	s.mu.Lock()
	previous, exists := s.inbox[key]
	if exists {
		s.mu.Unlock()
		if previous.Digest != hex.EncodeToString(digest[:]) {
			return InboxMessage{}, NewError(CodeIdempotencyConflict, "message digest changed")
		}
		return previous, nil
	}
	s.mu.Unlock()
	if revision < binding.LastRevision {
		record := InboxMessage{BindingID: binding.ID, MessageID: messageID, Digest: hex.EncodeToString(digest[:]), Revision: revision, Status: InboxDuplicate, ReceivedAt: time.Now().UTC()}
		s.mu.Lock()
		s.inbox[key] = record
		s.mu.Unlock()
		return record, nil
	}
	if err := apply(); err != nil {
		record := InboxMessage{BindingID: binding.ID, MessageID: messageID, Digest: hex.EncodeToString(digest[:]), Revision: revision, Status: InboxRejected, Error: err.Error(), ReceivedAt: time.Now().UTC()}
		s.mu.Lock()
		s.inbox[key] = record
		s.mu.Unlock()
		return record, err
	}
	record := InboxMessage{BindingID: binding.ID, MessageID: messageID, Digest: hex.EncodeToString(digest[:]), Revision: revision, Status: InboxApplied, ReceivedAt: time.Now().UTC()}
	s.mu.Lock()
	s.inbox[key] = record
	binding.LastRevision = revision
	s.bindings[binding.ID] = binding
	s.mu.Unlock()
	return record, nil
}

func SignRequest(secret string, method, path, timestamp, messageID string, body []byte) string {
	sum := sha256.Sum256(body)
	payload := strings.Join([]string{method, path, timestamp, messageID, hex.EncodeToString(sum[:])}, "\n")
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}
func VerifyRequest(secret string, request *http.Request, body []byte, now time.Time, maxSkew time.Duration) error {
	ts, id, sig := request.Header.Get("X-Noyo-Timestamp"), request.Header.Get("X-Noyo-Message-Id"), request.Header.Get("X-Noyo-Signature")
	if ts == "" || id == "" || sig == "" {
		return NewError(CodeValidationFailed, "signature headers are required")
	}
	parsed, err := strconv.ParseInt(ts, 10, 64)
	if err != nil || now.Sub(time.Unix(parsed, 0)).Abs() > maxSkew {
		return NewError(CodeValidationFailed, "signature timestamp expired")
	}
	expected := SignRequest(secret, request.Method, request.URL.Path, ts, id, body)
	if !hmac.Equal([]byte(strings.ToLower(sig)), []byte(expected)) {
		return NewError(CodePermissionDenied, "signature mismatch")
	}
	return nil
}
func ValidateMapping(mapping Mapping) error {
	if mapping.ID == "" || mapping.Revision <= 0 {
		return fmt.Errorf("mapping id and positive revision are required")
	}
	if len(mapping.Fields) == 0 {
		return fmt.Errorf("mapping fields are required")
	}
	return nil
}
func EncodeEvent(value any) []byte { data, _ := json.Marshal(value); return data }
