package llmlog

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

type transport struct {
	base     http.RoundTripper
	metadata Metadata
}

func NewTransport(base http.RoundTripper, metadata Metadata) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	if metadata.SessionID == "" {
		metadata.SessionID = NewSessionID("operation-")
	}
	return &transport{base: base, metadata: metadata}
}

func (t *transport) RoundTrip(req *http.Request) (*http.Response, error) {
	metadata := Merge(t.metadata, FromContext(req.Context()))
	endpoint := *req.URL
	endpoint.User, endpoint.RawQuery, endpoint.Fragment = nil, "", ""
	row := &Record{TenantID: metadata.TenantID, ProjectID: metadata.ProjectID, UserID: metadata.UserID,
		SessionID: metadata.SessionID, Source: metadata.Source, Provider: metadata.Provider,
		Endpoint: endpoint.String(), Method: req.Method, StartedAt: time.Now().UTC(), Status: "pending", RequestID: NewSessionID("llm-")}
	if row.Provider == "" {
		row.Provider = req.URL.Hostname()
	}
	if req.Body != nil {
		body, err := io.ReadAll(req.Body)
		_ = req.Body.Close()
		req.Body = io.NopCloser(bytes.NewReader(body))
		row.RequestBody = string(body)
		var payload struct {
			Model  string `json:"model"`
			Stream bool   `json:"stream"`
		}
		_ = json.Unmarshal(body, &payload)
		row.Model, row.Stream = payload.Model, payload.Stream
		if err != nil {
			_ = persist(row)
			finish(row, nil, err)
			return nil, err
		}
	}
	if err := persist(row); err != nil {
		logErr := fmt.Errorf("persist LLM request before dispatch: %w", err)
		finish(row, nil, logErr)
		return nil, logErr
	}
	resp, err := t.base.RoundTrip(req)
	if err != nil {
		finish(row, nil, err)
		return resp, err
	}
	row.StatusCode = resp.StatusCode
	row.Stream = row.Stream || strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream")
	// Store diagnostic headers only: credentials, cookies and arbitrary private
	// vendor headers never enter the record.
	headers := make(http.Header)
	for key, values := range resp.Header {
		lower := strings.ToLower(key)
		if lower == "content-type" || lower == "retry-after" || lower == "request-id" || lower == "x-request-id" || strings.HasPrefix(lower, "x-ratelimit-") {
			headers[key] = values
		}
	}
	headerJSON, _ := json.Marshal(headers)
	row.ResponseHeaders = string(headerJSON)
	row.ProviderRequestID = resp.Header.Get("X-Request-ID")
	if row.ProviderRequestID == "" {
		row.ProviderRequestID = resp.Header.Get("Request-ID")
	}
	if resp.Body == nil {
		finish(row, nil, nil)
		return resp, nil
	}
	body := &captureBody{ReadCloser: resp.Body, row: row, ctx: req.Context(), expected: resp.ContentLength}
	stop := context.AfterFunc(req.Context(), func() { body.complete(req.Context().Err()) })
	body.mu.Lock()
	if body.done {
		stop()
	} else {
		body.stop = stop
	}
	body.mu.Unlock()
	resp.Body = body
	return resp, nil
}

type captureBody struct {
	io.ReadCloser
	mu       sync.Mutex
	row      *Record
	ctx      context.Context
	data     bytes.Buffer
	done     bool
	stop     func() bool
	expected int64
}

func (b *captureBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	b.mu.Lock()
	if n > 0 && !b.done {
		if b.data.Len() == 0 {
			b.row.FirstByteMS = time.Since(b.row.StartedAt).Milliseconds()
		}
		_, _ = b.data.Write(p[:n])
	}
	tail := b.data.Bytes()
	if len(tail) > 64 {
		tail = tail[len(tail)-64:]
	}
	complete := !b.done && ((b.expected > 0 && int64(b.data.Len()) >= b.expected) || (b.row.Stream && bytes.Contains(tail, []byte("data: [DONE]"))))
	b.mu.Unlock()
	if complete {
		b.complete(nil)
	}
	if err != nil {
		if errors.Is(err, io.EOF) {
			b.complete(nil)
		} else {
			b.complete(err)
		}
	}
	return n, err
}

func (b *captureBody) Close() error {
	closeErr := b.ReadCloser.Close()
	err := closeErr
	if err == nil && b.ctx.Err() != nil {
		err = b.ctx.Err()
	}
	b.mu.Lock()
	unfinished := !b.done && ((b.row.Stream && !streamHasTerminal(b.data.Bytes()) && !json.Valid(b.data.Bytes())) || (!b.row.Stream && !json.Valid(b.data.Bytes())))
	b.mu.Unlock()
	if unfinished && err == nil {
		err = errors.New("response body closed before completion")
	}
	b.complete(err)
	return closeErr
}

func (b *captureBody) complete(err error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.done {
		return
	}
	b.done = true
	if b.stop != nil {
		b.stop()
	}
	finish(b.row, b.data.Bytes(), err)
}

func finish(row *Record, body []byte, err error) {
	now := time.Now().UTC()
	row.FinishedAt, row.DurationMS = &now, now.Sub(row.StartedAt).Milliseconds()
	row.ResponseBody = string(body)
	if err == nil && row.Stream && !json.Valid(body) && !streamHasTerminal(body) {
		err = errors.New("stream ended before a completion event")
	}
	row.Status = "success"
	if row.StatusCode < 200 || row.StatusCode >= 300 {
		row.Status = "error"
		if row.StatusCode != 0 {
			row.Error = fmt.Sprintf("HTTP %d", row.StatusCode)
		}
	}
	if err != nil {
		row.Status, row.Error = "error", err.Error()
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			row.Status = "cancelled"
		}
	}
	parseResponse(row, body)
	// A transient database error after a successful provider call must not leave
	// the initial row pending. The writer uses the same primary key on retry.
	for attempt := 0; attempt < 3; attempt++ {
		if persist(row) == nil {
			break
		}
	}
}

func parseResponse(row *Record, body []byte) {
	usage := map[string]any{}
	consume := func(data []byte) {
		var payload map[string]any
		if json.Unmarshal(data, &payload) != nil {
			return
		}
		if nested, ok := payload["response"].(map[string]any); ok {
			payload = nested
		}
		if id, ok := payload["id"].(string); ok {
			row.ResponseID = id
		}
		if model, ok := payload["model"].(string); ok && row.Model == "" {
			row.Model = model
		}
		if value, ok := payload["usage"].(map[string]any); ok {
			mergeUsage(usage, value)
			row.UsageAvailable = true
		}
		if message, ok := payload["message"].(map[string]any); ok {
			if value, ok := message["usage"].(map[string]any); ok {
				mergeUsage(usage, value)
				row.UsageAvailable = true
			}
		}
		if choices, ok := payload["choices"].([]any); ok {
			for _, choice := range choices {
				if object, ok := choice.(map[string]any); ok {
					if reason, ok := object["finish_reason"].(string); ok {
						row.FinishReason = reason
					}
				}
			}
		}
		if providerError, ok := payload["error"]; ok && providerError != nil {
			encoded, _ := json.Marshal(providerError)
			row.Status, row.Error = "error", string(encoded)
		}
	}
	if json.Valid(body) {
		consume(body)
	} else if row.Stream {
		for _, line := range bytes.Split(body, []byte("\n")) {
			line = bytes.TrimSpace(line)
			if bytes.HasPrefix(line, []byte("data:")) {
				consume(bytes.TrimSpace(bytes.TrimPrefix(line, []byte("data:"))))
			}
		}
	} else {
		consume(body)
	}
	if !row.UsageAvailable {
		return
	}
	encoded, _ := json.Marshal(usage)
	row.UsageJSON = string(encoded)
	row.PromptTokens = number(usage, "prompt_tokens", "input_tokens")
	row.CompletionTokens = number(usage, "completion_tokens", "output_tokens")
	row.TotalTokens = number(usage, "total_tokens")
	if row.TotalTokens == 0 {
		row.TotalTokens = row.PromptTokens + row.CompletionTokens
	}
	row.CachedTokens = number(usage, "prompt_cache_hit_tokens", "cache_read_input_tokens")
	if details, ok := usage["prompt_tokens_details"].(map[string]any); ok {
		row.CachedTokens = number(details, "cached_tokens")
	}
	if details, ok := usage["input_tokens_details"].(map[string]any); ok {
		row.CachedTokens = number(details, "cached_tokens")
	}
	for _, key := range []string{"completion_tokens_details", "output_tokens_details"} {
		if details, ok := usage[key].(map[string]any); ok {
			row.ReasoningTokens = number(details, "reasoning_tokens")
		}
	}
}

func streamHasTerminal(body []byte) bool {
	for _, line := range bytes.Split(body, []byte("\n")) {
		line = bytes.TrimSpace(line)
		if !bytes.HasPrefix(line, []byte("data:")) {
			continue
		}
		data := bytes.TrimSpace(bytes.TrimPrefix(line, []byte("data:")))
		if bytes.Equal(data, []byte("[DONE]")) {
			return true
		}
		var event struct {
			Type string `json:"type"`
		}
		if json.Unmarshal(data, &event) == nil && (event.Type == "response.completed" || event.Type == "response.failed" || event.Type == "message_stop") {
			return true
		}
	}
	return false
}

func mergeUsage(target, value map[string]any) {
	for key, v := range value {
		if nested, ok := v.(map[string]any); ok {
			existing, _ := target[key].(map[string]any)
			if existing == nil {
				existing = map[string]any{}
				target[key] = existing
			}
			mergeUsage(existing, nested)
		} else {
			target[key] = v
		}
	}
}

func number(values map[string]any, keys ...string) int64 {
	for _, key := range keys {
		if n, ok := values[key].(float64); ok {
			return int64(n)
		}
	}
	return 0
}
