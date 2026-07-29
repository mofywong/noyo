package workorder

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	AIHandoffStatusPending         = "pending"
	AIHandoffStatusAccepted        = "accepted"
	AIHandoffStatusRejected        = "rejected"
	AIHandoffStatusWorkOrderLinked = "work_order_linked"
	AIHandoffStatusFailed          = "failed"
)

var ErrAIHandoffBusy = errors.New("AI handoff operation is already in progress")

// AIHandoff records the recoverable saga between a suggestion and its one
// work order. Version is incremented for every state change and source data is
// immutable, so retrying a failed writeback cannot create a second order.
type AIHandoff struct {
	ID                string         `json:"id"`
	TenantID          uint           `json:"tenant_id"`
	ProjectID         uint           `json:"project_id"`
	SuggestionRef     string         `json:"suggestion_ref"`
	Status            string         `json:"status"`
	Version           int            `json:"version"`
	WorkOrderPublicID string         `json:"work_order_public_id,omitempty"`
	SourceSnapshot    map[string]any `json:"source_snapshot,omitempty"`
	LastError         string         `json:"last_error,omitempty"`
	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at"`
	inFlight          bool
}

type AIHandoffWorkOrderRequest struct {
	Scope          Scope
	SuggestionRef  string
	IdempotencyKey string
	SourceSnapshot map[string]any
}

type AIHandoffLinkRequest struct {
	Scope             Scope
	SuggestionRef     string
	WorkOrderPublicID string
}

// AIHandoffExecutor provides the two external side effects needed by the
// saga. Implementations typically call workorder.CommandPort.Create and then
// persist the suggestion-to-order link. Link is deliberately separate so a
// failed writeback is retryable without repeating Create.
type AIHandoffExecutor interface {
	Create(context.Context, AIHandoffWorkOrderRequest) (string, error)
	Link(context.Context, AIHandoffLinkRequest) error
}

type AIHandoffStore struct {
	mu         sync.RWMutex
	handoffs   map[string]*AIHandoff
	byScopeRef map[string]string
}

func NewAIHandoffStore() *AIHandoffStore {
	return &AIHandoffStore{handoffs: make(map[string]*AIHandoff), byScopeRef: make(map[string]string)}
}

func (s *AIHandoffStore) Begin(ctx context.Context, scope Scope, suggestionRef string, snapshot map[string]any) (*AIHandoff, error) {
	if err := validateHandoffContext(ctx, scope, suggestionRef); err != nil {
		return nil, err
	}
	suggestionRef = strings.TrimSpace(suggestionRef)
	key := handoffScopeKey(scope, suggestionRef)
	s.mu.Lock()
	defer s.mu.Unlock()
	if existingID := s.byScopeRef[key]; existingID != "" {
		return cloneAIHandoff(s.handoffs[existingID]), nil
	}
	now := time.Now().UTC()
	handoff := &AIHandoff{ID: uuid.NewString(), TenantID: scope.TenantID, ProjectID: scope.ProjectID,
		SuggestionRef: suggestionRef, Status: AIHandoffStatusPending, Version: 1,
		SourceSnapshot: cloneHandoffMap(snapshot), CreatedAt: now, UpdatedAt: now}
	s.handoffs[handoff.ID] = handoff
	s.byScopeRef[key] = handoff.ID
	return cloneAIHandoff(handoff), nil
}

func (s *AIHandoffStore) Get(scope Scope, id string) (*AIHandoff, error) {
	if err := validateHandoffScope(scope); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	handoff := s.handoffs[strings.TrimSpace(id)]
	if handoff == nil || handoff.TenantID != scope.TenantID || handoff.ProjectID != scope.ProjectID {
		return nil, NewError(CodeWorkOrderNotFound, "AI handoff not found")
	}
	return cloneAIHandoff(handoff), nil
}

func (s *AIHandoffStore) Reject(ctx context.Context, id string) (*AIHandoff, error) {
	if ctx == nil {
		return nil, NewError(CodeValidationFailed, "context is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	handoff, err := s.lookupLocked(id)
	if err != nil {
		return nil, err
	}
	if handoff.inFlight {
		return nil, ErrAIHandoffBusy
	}
	if handoff.Status != AIHandoffStatusPending {
		return nil, NewError(CodeInvalidTransition, "AI handoff is no longer pending")
	}
	handoff.Status = AIHandoffStatusRejected
	handoff.Version++
	handoff.UpdatedAt = time.Now().UTC()
	handoff.LastError = ""
	return cloneAIHandoff(handoff), nil
}

func (s *AIHandoffStore) Accept(ctx context.Context, id string, executor AIHandoffExecutor) (*AIHandoff, error) {
	if ctx == nil {
		return nil, NewError(CodeValidationFailed, "context is required")
	}
	if executor == nil {
		return nil, NewError(CodeValidationFailed, "AI handoff executor is required")
	}
	s.mu.Lock()
	handoff, err := s.lookupLocked(id)
	if err != nil {
		s.mu.Unlock()
		return nil, err
	}
	if handoff.inFlight {
		s.mu.Unlock()
		return nil, ErrAIHandoffBusy
	}
	if handoff.Status != AIHandoffStatusPending {
		s.mu.Unlock()
		return nil, NewError(CodeInvalidTransition, "AI handoff is no longer pending")
	}
	handoff.Status = AIHandoffStatusAccepted
	handoff.Version++
	handoff.UpdatedAt = time.Now().UTC()
	handoff.inFlight = true
	request := AIHandoffWorkOrderRequest{Scope: Scope{TenantID: handoff.TenantID, ProjectID: handoff.ProjectID}, SuggestionRef: handoff.SuggestionRef,
		IdempotencyKey: "ai-handoff:" + handoff.SuggestionRef, SourceSnapshot: cloneHandoffMap(handoff.SourceSnapshot)}
	snapshotID := handoff.ID
	s.mu.Unlock()

	workOrderID, createErr := executor.Create(ctx, request)
	if createErr != nil {
		return s.fail(snapshotID, createErr)
	}
	workOrderID = strings.TrimSpace(workOrderID)
	if workOrderID == "" {
		return s.fail(snapshotID, errors.New("AI handoff executor returned an empty work order id"))
	}
	if linkErr := executor.Link(ctx, AIHandoffLinkRequest{Scope: request.Scope, SuggestionRef: request.SuggestionRef, WorkOrderPublicID: workOrderID}); linkErr != nil {
		return s.failWithWorkOrder(snapshotID, workOrderID, linkErr)
	}
	return s.complete(snapshotID, workOrderID)
}

func (s *AIHandoffStore) Retry(ctx context.Context, id string, executor AIHandoffExecutor) (*AIHandoff, error) {
	if ctx == nil {
		return nil, NewError(CodeValidationFailed, "context is required")
	}
	if executor == nil {
		return nil, NewError(CodeValidationFailed, "AI handoff executor is required")
	}
	s.mu.Lock()
	handoff, err := s.lookupLocked(id)
	if err != nil {
		s.mu.Unlock()
		return nil, err
	}
	if handoff.inFlight {
		s.mu.Unlock()
		return nil, ErrAIHandoffBusy
	}
	if handoff.Status != AIHandoffStatusFailed {
		s.mu.Unlock()
		return nil, NewError(CodeInvalidTransition, "only failed AI handoffs can be retried")
	}
	handoff.Status = AIHandoffStatusAccepted
	handoff.Version++
	handoff.UpdatedAt = time.Now().UTC()
	handoff.LastError = ""
	handoff.inFlight = true
	request := AIHandoffWorkOrderRequest{Scope: Scope{TenantID: handoff.TenantID, ProjectID: handoff.ProjectID}, SuggestionRef: handoff.SuggestionRef,
		IdempotencyKey: "ai-handoff:" + handoff.SuggestionRef, SourceSnapshot: cloneHandoffMap(handoff.SourceSnapshot)}
	workOrderID := handoff.WorkOrderPublicID
	snapshotID := handoff.ID
	s.mu.Unlock()

	if workOrderID == "" {
		var createErr error
		workOrderID, createErr = executor.Create(ctx, request)
		if createErr != nil {
			return s.fail(snapshotID, createErr)
		}
		workOrderID = strings.TrimSpace(workOrderID)
		if workOrderID == "" {
			return s.fail(snapshotID, errors.New("AI handoff executor returned an empty work order id"))
		}
	}
	if linkErr := executor.Link(ctx, AIHandoffLinkRequest{Scope: request.Scope, SuggestionRef: request.SuggestionRef, WorkOrderPublicID: workOrderID}); linkErr != nil {
		return s.failWithWorkOrder(snapshotID, workOrderID, linkErr)
	}
	return s.complete(snapshotID, workOrderID)
}

func (s *AIHandoffStore) lookupLocked(id string) (*AIHandoff, error) {
	handoff := s.handoffs[strings.TrimSpace(id)]
	if handoff == nil {
		return nil, NewError(CodeWorkOrderNotFound, "AI handoff not found")
	}
	return handoff, nil
}

func (s *AIHandoffStore) fail(id string, cause error) (*AIHandoff, error) {
	return s.failWithWorkOrder(id, "", cause)
}

func (s *AIHandoffStore) failWithWorkOrder(id, workOrderID string, cause error) (*AIHandoff, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	handoff, err := s.lookupLocked(id)
	if err != nil {
		return nil, err
	}
	handoff.Status = AIHandoffStatusFailed
	handoff.WorkOrderPublicID = strings.TrimSpace(workOrderID)
	handoff.LastError = cause.Error()
	handoff.Version++
	handoff.UpdatedAt = time.Now().UTC()
	handoff.inFlight = false
	return cloneAIHandoff(handoff), cause
}

func (s *AIHandoffStore) complete(id, workOrderID string) (*AIHandoff, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	handoff, err := s.lookupLocked(id)
	if err != nil {
		return nil, err
	}
	handoff.Status = AIHandoffStatusWorkOrderLinked
	handoff.WorkOrderPublicID = strings.TrimSpace(workOrderID)
	handoff.LastError = ""
	handoff.Version++
	handoff.UpdatedAt = time.Now().UTC()
	handoff.inFlight = false
	return cloneAIHandoff(handoff), nil
}

func validateHandoffContext(ctx context.Context, scope Scope, suggestionRef string) error {
	if ctx == nil {
		return NewError(CodeValidationFailed, "context is required")
	}
	if err := validateHandoffScope(scope); err != nil {
		return err
	}
	if strings.TrimSpace(suggestionRef) == "" {
		return NewError(CodeValidationFailed, "suggestion reference is required")
	}
	return nil
}

func validateHandoffScope(scope Scope) error {
	if scope.TenantID == 0 || scope.ProjectID == 0 {
		return NewError(CodeValidationFailed, "tenant and project scope are required")
	}
	return nil
}

func handoffScopeKey(scope Scope, suggestionRef string) string {
	return fmt.Sprintf("%d:%d:%s", scope.TenantID, scope.ProjectID, strings.TrimSpace(suggestionRef))
}

func cloneAIHandoff(handoff *AIHandoff) *AIHandoff {
	if handoff == nil {
		return nil
	}
	copy := *handoff
	copy.SourceSnapshot = cloneHandoffMap(handoff.SourceSnapshot)
	copy.inFlight = false
	return &copy
}

func cloneHandoffMap(value map[string]any) map[string]any {
	if value == nil {
		return nil
	}
	result := make(map[string]any, len(value))
	for key, item := range value {
		switch nested := item.(type) {
		case map[string]any:
			result[key] = cloneHandoffMap(nested)
		case []any:
			items := make([]any, len(nested))
			for i, child := range nested {
				if childMap, ok := child.(map[string]any); ok {
					items[i] = cloneHandoffMap(childMap)
				} else {
					items[i] = child
				}
			}
			result[key] = items
		default:
			result[key] = item
		}
	}
	return result
}
