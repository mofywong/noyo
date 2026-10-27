package core

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/gogf/gf/v2/net/ghttp"
	"gorm.io/gorm"
	"noyo/core/screen"
	"noyo/core/store"
)

// Validation/authorization happen before the store transaction. The transaction
// then compares the exact author base and shares the autosave CAS, committing
// draft, resource references and durable receipt together.
func (s *Server) handleScreenApply(r *ghttp.Request) {
	row, err := screenLoad(r, "screen:edit")
	if err != nil {
		screenWriteError(r, err)
		return
	}
	version, err := screenBaseVersion(r)
	if err != nil {
		screenWriteError(r, err)
		return
	}
	var body struct {
		Document         json.RawMessage `json:"document"`
		BaseDocument     json.RawMessage `json:"baseDocument"`
		BaseDraftVersion uint64          `json:"baseDraftVersion"`
		IdempotencyKey   string          `json:"idempotencyKey"`
	}
	// Each document still has its own 5 MiB contract limit.
	if err = screenReadJSON(r, &body, 2*screen.MaxDocumentBytes+65536); err != nil {
		screenWriteError(r, err)
		return
	}
	if body.BaseDraftVersion != version || !screenAuditRequestID.MatchString(body.IdempotencyKey) {
		screenWriteError(r, screenFail(400, "Matching baseDraftVersion and a valid idempotencyKey are required / 需要一致的草稿版本和有效的幂等标识"))
		return
	}
	baseDoc, _, err := screen.Validate(body.BaseDocument)
	if err != nil {
		screenWriteError(r, err)
		return
	}
	doc, refs, err := screen.Validate(body.Document)
	if err == nil {
		err = screenAuthorizeRefs(r, row, refs, doc)
	}
	if err != nil {
		screenWriteError(r, err)
		return
	}
	accepted, replayed, err := store.ScreenApplyDraft(store.DB, row.TenantID, row.ProjectID, row.ID, requestAuthContext(r).UserID, version, body.IdempotencyKey, baseDoc, doc, refs)
	if err != nil {
		screenWriteError(r, err)
		return
	}
	doc, _, err = screen.Validate([]byte(accepted.DraftJSON))
	if err != nil {
		screenWriteError(r, err)
		return
	}
	latest, err := store.ScreenFind(store.DB, row.TenantID, row.ProjectID, row.ID)
	if err != nil {
		screenWriteError(r, err)
		return
	}
	r.Response.Header().Set("ETag", fmt.Sprintf("\"draft-%d\"", accepted.DraftVersion))
	r.SetCtxVar("screen_audit_revision", fmt.Sprintf("draft-%d", accepted.DraftVersion))
	r.SetCtxVar("screen_audit_request_id", body.IdempotencyKey)
	screenWriteData(r, map[string]interface{}{"screen": accepted, "document": doc, "draftVersion": accepted.DraftVersion, "currentDraftVersion": latest.DraftVersion, "idempotencyKey": body.IdempotencyKey, "replayed": replayed})
}

// Receipt lookup resolves an interrupted apply after a reload, when the editor
// has already loaded the advanced draft version and cannot repeat its old CAS.
func (s *Server) handleScreenApplyStatus(r *ghttp.Request) {
	row, err := screenLoad(r, "screen:edit")
	if err != nil {
		screenWriteError(r, err)
		return
	}
	key := r.GetRouter("requestId").String()
	if !screenAuditRequestID.MatchString(key) {
		screenWriteError(r, screenFail(400, "Invalid apply request id / 应用标识无效"))
		return
	}
	receipt, err := store.ScreenFindApplyReceipt(store.DB, row.ID, requestAuthContext(r).UserID, key)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		screenWriteData(r, map[string]interface{}{"committed": false, "idempotencyKey": key})
		return
	}
	if err != nil {
		screenWriteError(r, err)
		return
	}
	doc, _, err := screen.Validate([]byte(receipt.DocumentJSON))
	if err != nil {
		screenWriteError(r, err)
		return
	}
	screenWriteData(r, map[string]interface{}{"committed": true, "screen": row, "document": doc, "draftVersion": receipt.DraftVersion, "currentDraftVersion": row.DraftVersion, "idempotencyKey": key})
}
