package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"noyo/core/workorder"
)

type HTTPClient struct {
	BaseURL string
	Client  *http.Client
	Headers http.Header
}

func NewHTTPClient(baseURL string, client *http.Client) *HTTPClient {
	if client == nil {
		client = http.DefaultClient
	}
	return &HTTPClient{BaseURL: strings.TrimRight(baseURL, "/"), Client: client, Headers: make(http.Header)}
}
func (c *HTTPClient) Create(ctx context.Context, command workorder.CreateCommand) (workorder.CommandResult, error) {
	var result workorder.CommandResult
	err := c.do(ctx, http.MethodPost, "/api/work-orders", command.Scope, command.Meta, command, &result)
	return result, err
}
func (c *HTTPClient) Execute(ctx context.Context, command workorder.ExecuteCommand) (workorder.CommandResult, error) {
	var result workorder.CommandResult
	err := c.do(ctx, http.MethodPost, "/api/work-orders/"+command.WorkOrderPublicID+"/transitions", command.Scope, command.Meta, command, &result)
	return result, err
}
func (c *HTTPClient) Approve(ctx context.Context, command workorder.ApprovalCommand) (workorder.CommandResult, error) {
	var result workorder.CommandResult
	err := c.do(ctx, http.MethodPost, "/api/work-orders/"+command.WorkOrderPublicID+"/approve", command.Scope, command.Meta, command, &result)
	return result, err
}
func (c *HTTPClient) Reject(ctx context.Context, command workorder.ApprovalCommand) (workorder.CommandResult, error) {
	var result workorder.CommandResult
	err := c.do(ctx, http.MethodPost, "/api/work-orders/"+command.WorkOrderPublicID+"/reject", command.Scope, command.Meta, command, &result)
	return result, err
}
func (c *HTTPClient) do(ctx context.Context, method, path string, scope workorder.Scope, meta workorder.CommandMeta, payload any, result *workorder.CommandResult) error {
	if scope.TenantID == 0 || scope.ProjectID == 0 {
		return workorder.NewError(workorder.CodeValidationFailed, "tenant and project scope are required")
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", meta.IdempotencyKey)
	request.Header.Set("X-Noyo-Tenant", fmt.Sprint(scope.TenantID))
	request.Header.Set("X-Noyo-Project", fmt.Sprint(scope.ProjectID))
	if meta.ExpectedVersion != nil {
		request.Header.Set("If-Match", fmt.Sprint(*meta.ExpectedVersion))
	}
	if meta.CorrelationID != "" {
		request.Header.Set("X-Correlation-ID", meta.CorrelationID)
	}
	for key, values := range c.Headers {
		for _, value := range values {
			request.Header.Add(key, value)
		}
	}
	response, err := c.Client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	var envelope struct {
		Code    any             `json:"code"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	}
	if err := json.NewDecoder(response.Body).Decode(&envelope); err != nil {
		return err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		code := workorder.CodeValidationFailed
		if value, ok := envelope.Code.(string); ok && value != "" {
			code = value
		}
		return workorder.NewError(code, envelope.Message)
	}
	if len(envelope.Data) > 0 {
		return json.Unmarshal(envelope.Data, result)
	}
	return nil
}
