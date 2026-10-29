package core

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
)

func (s *Server) handleListWorkOrderAssignmentRoles(r *ghttp.Request) {
	scope, ok := workOrderScopeForRequest(r)
	if !ok {
		return
	}
	roles, err := s.workOrderService().ListWorkOrderAssignmentRoles(scope)
	if err != nil {
		writeWorkOrderError(r, err)
		return
	}
	r.Response.WriteJson(g.Map{"code": 0, "data": roles})
}

func (s *Server) handleListWorkOrderAssignmentBlocks(r *ghttp.Request) {
	scope, ok := workOrderScopeForRequest(r)
	if !ok {
		return
	}
	blocks, err := s.workOrderService().ListWorkOrderAssignmentBlocks(scope, r.Get("id").Uint())
	if err != nil {
		writeWorkOrderError(r, err)
		return
	}
	r.Response.WriteJson(g.Map{"code": 0, "data": blocks})
}

func (s *Server) handleDispatchWorkOrderNode(r *ghttp.Request) {
	scope, ok := workOrderScopeForRequest(r)
	if !ok {
		return
	}
	var input WorkOrderDispatchInput
	if !decodeWorkOrderRequest(r, &input) {
		return
	}
	order, err := s.workOrderService().DispatchWorkOrderNode(scope, r.Get("id").Uint(), r.Get("instance_id").Uint(), input)
	if err != nil {
		writeWorkOrderError(r, err)
		return
	}
	writeWorkOrderResponse(r, order)
}
