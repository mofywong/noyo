package assistant

import (
	"context"

	"github.com/cloudwego/eino/schema"
)

type scopeContextKey struct{}
type toolResultObserverContextKey struct{}

// Scope carries the authenticated Noyo request boundary into assistant tools.
type Scope struct {
	TenantID         uint
	ProjectID        uint
	UserID           uint
	AuthHeader       string
	APIBaseURL       string
	Channel          string
	AssistantSubject string
}

func WithScope(ctx context.Context, scope Scope) context.Context {
	return context.WithValue(ctx, scopeContextKey{}, scope)
}

func ScopeFromContext(ctx context.Context) (Scope, bool) {
	scope, ok := ctx.Value(scopeContextKey{}).(Scope)
	return scope, ok
}

type ToolResultObserver func(*schema.Message)

func WithToolResultObserver(ctx context.Context, observer ToolResultObserver) context.Context {
	return context.WithValue(ctx, toolResultObserverContextKey{}, observer)
}

func NotifyToolResult(ctx context.Context, msg *schema.Message) {
	if ctx == nil || msg == nil {
		return
	}
	observer, ok := ctx.Value(toolResultObserverContextKey{}).(ToolResultObserver)
	if !ok || observer == nil {
		return
	}
	observer(msg)
}
