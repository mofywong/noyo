package core

import (
	"noyo/core/tsdb"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
)

// handleListDeviceEvents retrieves events for a device
func (s *Server) handleListDeviceEvents(r *ghttp.Request) {
	code := r.Get("code").String()
	startTime := r.Get("start").Int64()
	endTime := r.Get("end").Int64()
	page := r.Get("page", 1).Int()
	pageSize := r.Get("pageSize", 20).Int()
	eventType := r.Get("type", tsdb.TypeEvent).Int()

	if startTime == 0 {
		// Default to last 24 hours
		startTime = time.Now().Add(-24 * time.Hour).UnixMilli()
	}
	if endTime == 0 {
		endTime = time.Now().UnixMilli()
	}

	req := tsdb.QueryRequest{
		DeviceCode: code,
		StartTime:  startTime,
		EndTime:    endTime,
		Type:       eventType,
		Page:       page,
		PageSize:   pageSize,
	}

	res, err := s.TSDB.Query(req)
	if err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": err.Error()})
		return
	}

	r.Response.WriteJson(g.Map{
		"code":  0,
		"data":  res.List,
		"total": res.Total,
	})
}
