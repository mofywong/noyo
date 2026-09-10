package core

import (
	"encoding/json"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"net/http"
	"noyo/core/alarmmedia"
	"os"
	"sort"
)

// Access is derived from persisted, project-scoped alarm evidence, never from
// a client-supplied device or an unscoped recording lookup.
func (s *Server) alarmMediaIDs(r *ghttp.Request) (map[string]bool, bool) {
	scope, _, ok := alarmScopeForRequest(r)
	if !ok {
		return nil, false
	}
	center, err := s.alarmCenter()
	if err != nil {
		writeAlarmCenterError(r, err)
		return nil, false
	}
	instance, err := center.Get(scope, r.Get("id").String())
	if err != nil {
		writeAlarmCenterError(r, err)
		return nil, false
	}
	events, err := center.Events(scope, instance.PublicID)
	if err != nil {
		writeAlarmCenterError(r, err)
		return nil, false
	}
	ids := map[string]bool{}
	var walk func(any)
	walk = func(v any) {
		switch x := v.(type) {
		case map[string]any:
			for k, v := range x {
				if k == "recording_id" {
					if id, ok := v.(string); ok && alarmmedia.ValidID(id) {
						ids[id] = true
					}
				} else {
					walk(v)
				}
			}
		case []any:
			for _, v := range x {
				walk(v)
			}
		}
	}
	scan := func(raw string) {
		var v any
		if json.Unmarshal([]byte(raw), &v) == nil {
			walk(v)
		}
	}
	scan(instance.EvidenceSnapshot)
	scan(instance.ClearedEvidenceSnapshot)
	for _, e := range events {
		scan(e.EvidenceSnapshot)
		scan(e.Payload)
	}
	for id := range ids {
		if rec, e := alarmmedia.Load(id); e == nil && (rec.TenantID != scope.TenantID || rec.ProjectID != scope.ProjectID) {
			delete(ids, id)
		}
	}
	return ids, true
}
func (s *Server) handleAlarmMedia(r *ghttp.Request) {
	ids, ok := s.alarmMediaIDs(r)
	if !ok {
		return
	}
	records := []alarmmedia.Record{}
	for id := range ids {
		rec, err := alarmmedia.Load(id)
		if err != nil {
			rec = alarmmedia.Record{ID: id, Status: "failed", Reason: "media_missing"}
		}
		records = append(records, rec)
	}
	sort.Slice(records, func(i, j int) bool { return records[i].OccurredAt > records[j].OccurredAt })
	r.Response.Header().Set("Cache-Control", "no-store")
	r.Response.WriteJson(g.Map{"code": 0, "data": records})
}
func (s *Server) handleAlarmMediaFile(r *ghttp.Request) {
	ids, ok := s.alarmMediaIDs(r)
	if !ok {
		return
	}
	id := r.Get("recording").String()
	if !ids[id] {
		r.Response.WriteStatus(404)
		return
	}
	kind := r.Get("kind").String()
	if kind != "preview.mp4" {
		r.Response.WriteStatus(404)
		return
	}
	rec, err := alarmmedia.Load(id)
	if err != nil || (rec.Status != "ready" && rec.Status != "partial") {
		r.Response.WriteStatus(409)
		return
	}
	path, err := alarmmedia.Path(id, kind)
	if err != nil {
		r.Response.WriteStatus(404)
		return
	}
	f, err := os.Open(path)
	if err != nil {
		r.Response.WriteStatus(404)
		return
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		r.Response.WriteStatus(404)
		return
	}
	r.Response.Header().Set("Cache-Control", "private, no-store")
	r.Response.Header().Set("Referrer-Policy", "no-referrer")
	r.Response.Header().Set("X-Content-Type-Options", "nosniff")
	r.Response.Header().Set("Content-Type", "video/mp4")
	http.ServeContent(r.Response.Writer, r.Request, kind, st.ModTime(), f)
}
