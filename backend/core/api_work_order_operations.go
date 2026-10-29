package core

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
)

func workOrderOperationResponse(r *ghttp.Request, data any, err error) {
	if err != nil {
		writeWorkOrderError(r, err)
		return
	}
	r.Response.WriteJson(g.Map{"code": 0, "data": data})
}
func (s *Server) handleListWorkOrderPolicies(r *ghttp.Request) {
	scope, ok := workOrderScopeForRequest(r)
	if !ok {
		return
	}
	data, err := s.workOrderService().ListWorkOrderPolicies(scope)
	workOrderOperationResponse(r, data, err)
}
func (s *Server) handlePublishWorkOrderPolicy(r *ghttp.Request) {
	scope, ok := workOrderScopeForRequest(r)
	if !ok {
		return
	}
	var input WorkOrderPolicyPublishInput
	if !decodeWorkOrderRequest(r, &input) {
		return
	}
	data, err := s.workOrderService().PublishWorkOrderPolicy(scope, r.Get("id").Uint(), input)
	workOrderOperationResponse(r, data, err)
}
func (s *Server) handleEnableWorkOrderPolicy(r *ghttp.Request) {
	scope, ok := workOrderScopeForRequest(r)
	if !ok {
		return
	}
	var input struct {
		ExpectedVersion int  `json:"expected_version"`
		Enabled         bool `json:"enabled"`
	}
	if !decodeWorkOrderRequest(r, &input) {
		return
	}
	err := s.workOrderService().SetWorkOrderPolicyEnabled(scope, r.Get("id").Uint(), input.ExpectedVersion, input.Enabled)
	workOrderOperationResponse(r, nil, err)
}
func (s *Server) handlePreviewWorkOrderPolicy(r *ghttp.Request) {
	scope, ok := workOrderScopeForRequest(r)
	if !ok {
		return
	}
	var input WorkOrderRoutingContext
	if !decodeWorkOrderRequest(r, &input) {
		return
	}
	data, err := s.workOrderService().PreviewWorkOrderPolicy(scope, r.Get("version_id").Uint(), input)
	workOrderOperationResponse(r, data, err)
}
func (s *Server) handleWorkOrderPolicyVersion(r *ghttp.Request) {
	scope, ok := workOrderScopeForRequest(r)
	if !ok {
		return
	}
	data, err := s.workOrderService().GetWorkOrderPolicyVersionDetail(scope, r.Get("version_id").Uint())
	workOrderOperationResponse(r, data, err)
}
func (s *Server) handleListWorkOrderExecutionGrants(r *ghttp.Request) {
	scope, ok := workOrderScopeForRequest(r)
	if !ok {
		return
	}
	data, err := s.workOrderService().ListWorkOrderExecutionGrants(scope)
	workOrderOperationResponse(r, data, err)
}
func (s *Server) handleCreateWorkOrderExecutionGrant(r *ghttp.Request) {
	scope, ok := workOrderScopeForRequest(r)
	if !ok {
		return
	}
	var input WorkOrderExecutionGrantInput
	if !decodeWorkOrderRequest(r, &input) {
		return
	}
	data, err := s.workOrderService().CreateWorkOrderExecutionGrant(scope, input)
	workOrderOperationResponse(r, data, err)
}
func (s *Server) handleRevokeWorkOrderExecutionGrant(r *ghttp.Request) {
	scope, ok := workOrderScopeForRequest(r)
	if !ok {
		return
	}
	var input struct {
		Reason string `json:"reason"`
	}
	if !decodeWorkOrderRequest(r, &input) {
		return
	}
	err := s.workOrderService().RevokeWorkOrderExecutionGrant(scope, r.Get("id").Uint(), input.Reason)
	workOrderOperationResponse(r, nil, err)
}
func (s *Server) handleClaimWorkOrderCandidateTask(r *ghttp.Request) {
	scope, ok := workOrderScopeForRequest(r)
	if !ok {
		return
	}
	var input struct {
		ExpectedVersion int `json:"expected_version"`
	}
	if !decodeWorkOrderRequest(r, &input) {
		return
	}
	order, err := s.workOrderService().ClaimWorkOrderTask(scope, r.Get("id").Uint(), r.Get("task_id").Uint(), input.ExpectedVersion)
	if err != nil {
		writeWorkOrderError(r, err)
		return
	}
	writeWorkOrderResponse(r, order)
}
func (s *Server) handleGetWorkOrderSLA(r *ghttp.Request) {
	scope, ok := workOrderScopeForRequest(r)
	if !ok {
		return
	}
	data, err := s.workOrderService().GetWorkOrderSLA(scope, r.Get("id").Uint())
	workOrderOperationResponse(r, data, err)
}
func (s *Server) handlePauseWorkOrderSLA(r *ghttp.Request) {
	scope, ok := workOrderScopeForRequest(r)
	if !ok {
		return
	}
	var input struct {
		ExpectedVersion int    `json:"expected_version"`
		Pause           bool   `json:"pause"`
		Reason          string `json:"reason"`
	}
	if !decodeWorkOrderRequest(r, &input) {
		return
	}
	order, err := s.workOrderService().PauseWorkOrderSLA(scope, r.Get("id").Uint(), input.ExpectedVersion, input.Pause, input.Reason)
	if err != nil {
		writeWorkOrderError(r, err)
		return
	}
	writeWorkOrderResponse(r, order)
}
func (s *Server) handleWorkOrderOperationsSummary(r *ghttp.Request) {
	scope, ok := workOrderScopeForRequest(r)
	if !ok {
		return
	}
	data, err := s.workOrderService().GetWorkOrderOperationsSummary(scope)
	workOrderOperationResponse(r, data, err)
}
func (s *Server) handleFailedWorkOrderDeliveries(r *ghttp.Request) {
	scope, ok := workOrderScopeForRequest(r)
	if !ok {
		return
	}
	data, err := s.workOrderService().ListFailedWorkOrderDeliveries(scope)
	workOrderOperationResponse(r, data, err)
}
func (s *Server) handleRecoverWorkOrderDelivery(r *ghttp.Request) {
	scope, ok := workOrderScopeForRequest(r)
	if !ok {
		return
	}
	var input struct {
		ExpectedAttempts int    `json:"expected_attempts"`
		Stop             bool   `json:"stop"`
		Reason           string `json:"reason"`
	}
	if !decodeWorkOrderRequest(r, &input) {
		return
	}
	err := s.workOrderService().RecoverWorkOrderDelivery(scope, r.Get("id").Uint(), input.ExpectedAttempts, input.Stop, input.Reason)
	workOrderOperationResponse(r, nil, err)
}
