package workorder

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type ManagedAttachmentStore struct {
	mu     sync.RWMutex
	policy AttachmentPolicy
	items  map[string]struct {
		meta  AttachmentMeta
		data  []byte
		scope Scope
	}
}

func NewAttachmentStore(policy AttachmentPolicy) *ManagedAttachmentStore {
	return &ManagedAttachmentStore{policy: policy, items: map[string]struct {
		meta  AttachmentMeta
		data  []byte
		scope Scope
	}{}}
}
func (s *ManagedAttachmentStore) Save(_ context.Context, scope Scope, meta AttachmentMeta, reader io.Reader) (AttachmentMeta, error) {
	if reader == nil {
		return AttachmentMeta{}, NewError(CodeValidationFailed, "attachment content is required")
	}
	data, err := ioutil.ReadAll(reader)
	if err != nil {
		return AttachmentMeta{}, err
	}
	meta.Size = int64(len(data))
	meta.TenantID, meta.ProjectID = scope.TenantID, scope.ProjectID
	if meta.StorageKey == "" {
		meta.StorageKey = opaqueAttachmentKey()
	}
	digest := sha256.Sum256(data)
	meta.SHA256 = hex.EncodeToString(digest[:])
	meta.CreatedAt = time.Now().UTC()
	if err := ValidateAttachment(meta, scope, s.policy); err != nil {
		return AttachmentMeta{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.items[meta.PublicID]; exists {
		return AttachmentMeta{}, NewError(CodeIdempotencyConflict, "attachment already exists")
	}
	s.items[meta.PublicID] = struct {
		meta  AttachmentMeta
		data  []byte
		scope Scope
	}{meta: meta, data: append([]byte(nil), data...), scope: scope}
	return meta, nil
}
func (s *ManagedAttachmentStore) Open(_ context.Context, scope Scope, publicID string) (io.ReadCloser, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	item, ok := s.items[publicID]
	if !ok || item.scope.TenantID != scope.TenantID || item.scope.ProjectID != scope.ProjectID {
		return nil, NewError(CodePermissionDenied, "attachment scope denied")
	}
	return io.NopCloser(bytes.NewReader(item.data)), nil
}

// Delete performs a business deletion while leaving physical cleanup to the
// storage lifecycle worker.  Scope is checked again instead of trusting the
// attachment ID supplied by a caller.
func (s *ManagedAttachmentStore) Delete(_ context.Context, scope Scope, publicID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.items[publicID]
	if !ok {
		return NewError(CodeWorkOrderNotFound, "attachment not found")
	}
	if item.scope.TenantID != scope.TenantID || item.scope.ProjectID != scope.ProjectID {
		return NewError(CodePermissionDenied, "attachment scope denied")
	}
	delete(s.items, publicID)
	return nil
}

type AttachmentStore interface {
	Save(context.Context, Scope, AttachmentMeta, []byte) (AttachmentMeta, error)
	Open(context.Context, Scope, string) ([]byte, error)
}
type MemoryAttachmentStore struct{ items map[string][]byte }

func NewMemoryAttachmentStore() *MemoryAttachmentStore {
	return &MemoryAttachmentStore{items: map[string][]byte{}}
}
func (s *MemoryAttachmentStore) Save(_ context.Context, scope Scope, meta AttachmentMeta, data []byte) (AttachmentMeta, error) {
	if err := ValidateAttachment(meta, scope, AttachmentPolicy{MaxBytes: 25 << 20}); err != nil {
		return AttachmentMeta{}, err
	}
	s.items[meta.PublicID] = append([]byte(nil), data...)
	return meta, nil
}
func (s *MemoryAttachmentStore) Open(_ context.Context, scope Scope, publicID string) ([]byte, error) {
	if scope.TenantID == 0 || scope.ProjectID == 0 {
		return nil, NewError(CodeValidationFailed, "tenant and project scope are required")
	}
	data, ok := s.items[publicID]
	if !ok {
		return nil, NewError(CodeWorkOrderNotFound, "attachment not found")
	}
	return append([]byte(nil), data...), nil
}

type AttachmentMeta struct {
	PublicID, WorkOrderPublicID, Name, MIME, StorageKey string
	TenantID, ProjectID                                 uint
	Size                                                int64
	SHA256                                              string
	CreatedAt                                           time.Time
}
type AttachmentPolicy struct {
	MaxBytes    int64
	AllowedMIME map[string]bool
}

func ValidateAttachment(meta AttachmentMeta, scope Scope, policy AttachmentPolicy) error {
	if scope.TenantID == 0 || scope.ProjectID == 0 {
		return NewError(CodeValidationFailed, "tenant and project scope are required")
	}
	if meta.PublicID == "" || meta.WorkOrderPublicID == "" || strings.TrimSpace(meta.Name) == "" {
		return NewError(CodeValidationFailed, "attachment identity is required")
	}
	if meta.Size < 0 {
		return NewError(CodeValidationFailed, "attachment size cannot be negative")
	}
	if policy.MaxBytes > 0 && meta.Size > policy.MaxBytes {
		return NewError(CodeValidationFailed, "attachment exceeds size limit")
	}
	if len(policy.AllowedMIME) > 0 && !policy.AllowedMIME[strings.ToLower(meta.MIME)] {
		return NewError(CodeValidationFailed, "attachment MIME type is not allowed")
	}
	if filepath.Base(meta.StorageKey) != meta.StorageKey || strings.Contains(meta.StorageKey, "..") {
		return NewError(CodeValidationFailed, "attachment storage key is invalid")
	}
	if strings.ContainsAny(meta.Name, `/\\`) || strings.TrimSpace(meta.Name) != meta.Name {
		return NewError(CodeValidationFailed, "attachment file name is invalid")
	}
	return nil
}
func SignedAttachmentPath(publicID string) string { return fmt.Sprintf("work-orders/%s", publicID) }

func opaqueAttachmentKey() string {
	var value [16]byte
	if _, err := rand.Read(value[:]); err == nil {
		return hex.EncodeToString(value[:])
	}
	return fmt.Sprintf("att-%d", time.Now().UnixNano())
}
