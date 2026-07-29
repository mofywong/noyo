package workorder

import (
	"context"
	"strings"
	"sync"
	"time"
)

type NotificationSelector struct {
	Roles     []string `json:"roles"`
	Assignee  bool     `json:"assignee"`
	Approvers bool     `json:"approvers"`
	Creator   bool     `json:"creator"`
}

func (s NotificationSelector) Resolve(roles map[string][]string, assignee string, approvers []string, creator string) []string {
	seen := map[string]bool{}
	result := []string{}
	add := func(value string) {
		if strings.TrimSpace(value) != "" && !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	for _, role := range s.Roles {
		for _, user := range roles[role] {
			add(user)
		}
	}
	if s.Assignee {
		add(assignee)
	}
	if s.Approvers {
		for _, user := range approvers {
			add(user)
		}
	}
	if s.Creator {
		add(creator)
	}
	return result
}

type NotificationEvent struct {
	WorkOrderPublicID, Type string
	Recipients              []string
}

type NotificationPolicyResolver struct {
	audit NotificationAuditWriter
	mu    sync.Mutex
	last  map[notificationKey]time.Time
}

// ResolveSelector retains the selector-only helper for callers that do not
// need frequency control or audit emission.
func (*NotificationPolicyResolver) ResolveSelector(policy NotificationSelector, roles map[string][]string, assignee string, approvers []string, creator string) []string {
	return policy.Resolve(roles, assignee, approvers, creator)
}

type DeviceHistoryWriter interface {
	Write(context.Context, Scope, string, map[string]any) error
}

type NotificationPolicy struct {
	ID, EventType string
	Cooldown      time.Duration
	Selectors     []NotificationSelector
}
type NotificationContext struct {
	Roles             map[string][]string
	Assignee, Creator string
	Approvers         []string
}
type NotificationAuditEvent struct {
	WorkOrderPublicID, PolicyID string
	Suppressed                  bool
	Recipients                  []string
	At                          time.Time
}
type NotificationAuditWriter interface {
	WriteNotificationAudit(context.Context, NotificationAuditEvent) error
}
type notificationKey struct {
	tenant, project   uint
	workOrder, policy string
}

func NewNotificationPolicyResolver(audit NotificationAuditWriter) *NotificationPolicyResolver {
	return &NotificationPolicyResolver{audit: audit, last: map[notificationKey]time.Time{}}
}
func (r *NotificationPolicyResolver) Resolve(ctx context.Context, scope Scope, workOrderID string, policy NotificationPolicy, values NotificationContext, now time.Time) ([]string, error) {
	if scope.TenantID == 0 || scope.ProjectID == 0 {
		return nil, NewError(CodeValidationFailed, "tenant and project scope are required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	key := notificationKey{scope.TenantID, scope.ProjectID, workOrderID, policy.ID}
	if previous, ok := r.last[key]; ok && policy.Cooldown > 0 && now.Sub(previous) < policy.Cooldown {
		if r.audit != nil {
			_ = r.audit.WriteNotificationAudit(ctx, NotificationAuditEvent{WorkOrderPublicID: workOrderID, PolicyID: policy.ID, Suppressed: true, At: now})
		}
		return nil, nil
	}
	recipients := []string{}
	for _, selector := range policy.Selectors {
		recipients = append(recipients, selector.Resolve(values.Roles, values.Assignee, values.Approvers, values.Creator)...)
	}
	unique := map[string]bool{}
	dedup := recipients[:0]
	for _, recipient := range recipients {
		if !unique[recipient] {
			unique[recipient] = true
			dedup = append(dedup, recipient)
		}
	}
	r.last[key] = now
	if r.audit != nil {
		_ = r.audit.WriteNotificationAudit(ctx, NotificationAuditEvent{WorkOrderPublicID: workOrderID, PolicyID: policy.ID, Recipients: dedup, At: now})
	}
	return dedup, nil
}
