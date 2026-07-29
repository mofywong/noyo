package core

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"noyo/core/workorder"
)

func (s *Server) handleListWorkOrderIntegrationConnectors(r *ghttp.Request) {
	scope, ok := workOrderScopeForRequest(r)
	if !ok {
		return
	}
	if s.Integrations == nil {
		writeWorkOrderError(r, fmt.Errorf("integration service is unavailable"))
		return
	}
	connectors, err := s.Integrations.ListConnectors(workOrderCommandScope(scope))
	if err != nil {
		writeWorkOrderError(r, err)
		return
	}
	r.Response.WriteJson(g.Map{"code": 0, "data": connectors})
}

func (s *Server) handleSaveWorkOrderIntegrationConnector(r *ghttp.Request) {
	scope, ok := workOrderScopeForRequest(r)
	if !ok {
		return
	}
	if s.Integrations == nil {
		writeWorkOrderError(r, fmt.Errorf("integration service is unavailable"))
		return
	}
	var payload struct {
		ID      string `json:"id"`
		Name    string `json:"name"`
		KeyID   string `json:"key_id"`
		Secret  string `json:"secret"`
		Enabled *bool  `json:"enabled"`
	}
	if !decodeWorkOrderRequest(r, &payload) {
		return
	}
	secret := strings.TrimSpace(payload.Secret)
	if secret == "" {
		writeWorkOrderError(r, fmt.Errorf("connector secret is required"))
		return
	}
	enabled := true
	if payload.Enabled != nil {
		enabled = *payload.Enabled
	}
	digest := sha256.Sum256([]byte(secret))
	connector, err := s.Integrations.SaveConnector(workorder.Connector{
		ID: strings.TrimSpace(payload.ID), TenantID: scope.TenantID, ProjectID: scope.ProjectID,
		Name: strings.TrimSpace(payload.Name), KeyID: strings.TrimSpace(payload.KeyID), SecretHash: fmt.Sprintf("%x", digest), Enabled: enabled,
	})
	if err != nil {
		writeWorkOrderError(r, err)
		return
	}
	r.Response.WriteJson(g.Map{"code": 0, "data": connector})
}

func (s *Server) handleListWorkOrderIntegrationBindings(r *ghttp.Request) {
	scope, ok := workOrderScopeForRequest(r)
	if !ok {
		return
	}
	if s.Integrations == nil {
		writeWorkOrderError(r, fmt.Errorf("integration service is unavailable"))
		return
	}
	bindings, err := s.Integrations.ListBindings(workOrderCommandScope(scope))
	if err != nil {
		writeWorkOrderError(r, err)
		return
	}
	r.Response.WriteJson(g.Map{"code": 0, "data": bindings})
}

func (s *Server) handleSaveWorkOrderIntegrationBinding(r *ghttp.Request) {
	scope, ok := workOrderScopeForRequest(r)
	if !ok {
		return
	}
	if s.Integrations == nil {
		writeWorkOrderError(r, fmt.Errorf("integration service is unavailable"))
		return
	}
	var payload workorder.Binding
	if !decodeWorkOrderRequest(r, &payload) {
		return
	}
	payload.TenantID = scope.TenantID
	payload.ProjectID = scope.ProjectID
	binding, err := s.Integrations.SaveBinding(payload)
	if err != nil {
		writeWorkOrderError(r, err)
		return
	}
	r.Response.WriteJson(g.Map{"code": 0, "data": binding})
}

func (s *Server) handleListWorkOrderIntegrationMappings(r *ghttp.Request) {
	scope, ok := workOrderScopeForRequest(r)
	if !ok {
		return
	}
	if s.Integrations == nil {
		writeWorkOrderError(r, fmt.Errorf("integration service is unavailable"))
		return
	}
	mappings, err := s.Integrations.ListMappings(workOrderCommandScope(scope), strings.TrimSpace(r.Get("id").String()))
	if err != nil {
		writeWorkOrderError(r, err)
		return
	}
	r.Response.WriteJson(g.Map{"code": 0, "data": mappings})
}

func (s *Server) handleSaveWorkOrderIntegrationMapping(r *ghttp.Request) {
	scope, ok := workOrderScopeForRequest(r)
	if !ok {
		return
	}
	if s.Integrations == nil {
		writeWorkOrderError(r, fmt.Errorf("integration service is unavailable"))
		return
	}
	var payload workorder.Mapping
	if !decodeWorkOrderRequest(r, &payload) {
		return
	}
	mapping, err := s.Integrations.SaveMapping(workOrderCommandScope(scope), strings.TrimSpace(r.Get("id").String()), payload)
	if err != nil {
		writeWorkOrderError(r, err)
		return
	}
	r.Response.WriteJson(g.Map{"code": 0, "data": mapping})
}

func (s *Server) handleWorkOrderIntegrationInbox(r *ghttp.Request) {
	scope, ok := workOrderScopeForRequest(r)
	if !ok {
		return
	}
	var payload struct {
		BindingID string         `json:"binding_id"`
		MessageID string         `json:"message_id"`
		Revision  int64          `json:"revision"`
		Payload   map[string]any `json:"payload"`
	}
	if !decodeWorkOrderRequest(r, &payload) {
		return
	}
	if s.Integrations == nil {
		writeWorkOrderError(r, fmt.Errorf("integration service is unavailable"))
		return
	}
	binding := workorder.Binding{ID: strings.TrimSpace(payload.BindingID), TenantID: scope.TenantID, ProjectID: scope.ProjectID}
	messageID := strings.TrimSpace(payload.MessageID)
	record, err := s.Integrations.Ingest(binding, messageID, payload.Revision, workorder.EncodeEvent(payload.Payload), func() error {
		var inbound struct {
			TemplateID     uint           `json:"template_id"`
			Title          string         `json:"title"`
			Summary        string         `json:"summary"`
			Priority       string         `json:"priority"`
			ExternalRef    string         `json:"external_ref"`
			FormData       map[string]any `json:"form_data"`
			SourceSnapshot map[string]any `json:"source_snapshot"`
		}
		encoded, marshalErr := json.Marshal(payload.Payload)
		if marshalErr != nil {
			return fmt.Errorf("encode integration payload: %w", marshalErr)
		}
		if err := json.Unmarshal(encoded, &inbound); err != nil {
			return fmt.Errorf("decode integration payload: %w", err)
		}
		inbound.Title = strings.TrimSpace(inbound.Title)
		inbound.ExternalRef = strings.TrimSpace(inbound.ExternalRef)
		if inbound.TemplateID == 0 || inbound.Title == "" {
			return fmt.Errorf("integration payload requires template_id and title")
		}
		if inbound.ExternalRef == "" {
			inbound.ExternalRef = messageID
		}
		if inbound.SourceSnapshot == nil {
			inbound.SourceSnapshot = payload.Payload
		}
		_, err := s.workOrderCommandPort().Create(r.Context(), workorder.CreateCommand{
			Scope:        workOrderCommandScope(scope),
			Meta:         workorder.CommandMeta{IdempotencyKey: "integration-inbox:" + binding.ID + ":" + messageID, CorrelationID: r.Header.Get("X-Correlation-ID")},
			TemplateCode: strconv.FormatUint(uint64(inbound.TemplateID), 10), Title: inbound.Title, Summary: inbound.Summary,
			Priority: inbound.Priority, SourceType: WorkOrderSourceExternal, SourceRef: binding.ID + ":" + inbound.ExternalRef,
			SourceSnapshot: inbound.SourceSnapshot, FormData: inbound.FormData,
		})
		return err
	})
	if err != nil {
		writeWorkOrderError(r, err)
		return
	}
	r.Response.WriteJson(g.Map{"code": 0, "data": record})
}
