package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/gogf/gf/v2/net/ghttp"
	"noyo/core/screen"
	"noyo/core/store"
)

// Only the proposal's public summary is streamed. Provider-internal reasoning,
// raw JSON patches and provider errors never cross this transport.
func screenAssistantSummary(content string) string {
	start := strings.Index(content, `"summary"`)
	if start < 0 {
		return ""
	}
	rest := strings.TrimSpace(content[start+9:])
	if !strings.HasPrefix(rest, ":") {
		return ""
	}
	rest = strings.TrimSpace(rest[1:])
	if !strings.HasPrefix(rest, `"`) {
		return ""
	}
	end, escaped := 1, false
	for ; end < len(rest); end++ {
		if rest[end] == '"' && !escaped {
			break
		}
		if rest[end] == '\\' {
			escaped = !escaped
		} else {
			escaped = false
		}
	}
	value := rest[:end]
	// A chunk may split a JSON escape or UTF-8 rune. Decode only a valid prefix.
	for trim := 0; trim <= 6 && len(value) > 0; trim++ {
		var summary string
		if json.Unmarshal([]byte(value+`"`), &summary) == nil {
			return summary
		}
		value = value[:len(value)-1]
	}
	return ""
}

func (s *Server) screenAssistantStream(r *ghttp.Request, ctx context.Context, model screenAssistantModel, payload map[string]interface{}, row *store.IOCScreen, doc screen.Envelope, widgets []screen.AssistantWidget, catalog screen.AssistantCatalog, allowAlarms bool) {
	r.Response.Header().Set("Content-Type", "text/event-stream")
	r.Response.Header().Set("Cache-Control", "private, no-store")
	r.Response.Header().Set("X-Accel-Buffering", "no")
	emit := func(event string, value interface{}) {
		data, _ := json.Marshal(value)
		r.Response.Write(fmt.Sprintf("event: %s\ndata: %s\n\n", event, data))
		r.Response.Flush()
	}
	fail := func(key string) {
		if ctx.Err() != nil {
			key = "aiTimeout"
		}
		emit("error", map[string]string{"errorKey": key})
	}
	plan, err := s.screenAssistantWorkflow(ctx, model, payload, row, doc, widgets, catalog, allowAlarms, true, emit)
	if err != nil {
		key := "aiFailed"
		if errors.Is(err, context.DeadlineExceeded) {
			key = "aiTimeout"
		}
		if errors.Is(err, errScreenAssistantInvalid) {
			key = "aiInvalid"
		}
		if errors.Is(err, errScreenAssistantOutOfScope) {
			key = "aiOutOfScope"
		}
		fail(key)
		return
	}
	emit("result", plan)
}
