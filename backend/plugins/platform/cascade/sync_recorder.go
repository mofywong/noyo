package cascade

import (
	"fmt"
	"strings"
	"time"

	"noyo/core/store"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// deviceContentColumns are the device columns whose modification changes what
// the entity *is*. Bookkeeping columns (rev, sync_state, conflict_*,
// sync_writer, origin, timestamps) are deliberately absent: writing them must
// never produce a change, or the synchronization machinery would record its own
// bookkeeping as user activity and the two nodes would echo each other.
var deviceContentColumns = map[string]bool{
	"tenant_id": true, "project_id": true, "code": true, "name": true,
	"product_code": true, "parent_code": true, "protocol_name": true,
	"protocol_profile_code": true, "enabled": true, "config": true,
}

// productContentColumns is the product equivalent of deviceContentColumns.
var productContentColumns = map[string]bool{
	"tenant_id": true, "project_id": true, "code": true, "name": true, "config": true,
}

// changeRecorder turns local writes into change-log entries. It is driven by
// GORM callbacks so every write path (UI, AI assistant, HTTP API, internal
// timers) is covered without modifying each call site.
type changeRecorder struct {
	// nodeID is this node's identity and becomes OriginNode on every change.
	nodeID string
	// instanceID identifies this node's change log and becomes OriginInstance.
	instanceID string
	// origin is the provenance stamped on entities that do not have one yet.
	origin string
	// ownerNode is stamped on devices that do not declare an owner. The gateway
	// sets it to its own code so the platform can route the device's later
	// changes back without relying on a hierarchy that does not exist.
	ownerNode string
	logger    *zap.Logger
	notify    func(ch store.SyncChange)
	// routeDevice and routeProduct decide where a change must be delivered.
	// The platform addresses changes to the gateways that own the entity; the
	// gateway always addresses the cloud.
	routeDevice  func(d *store.Device) string
	routeProduct func(p *store.Product) string
}

func newChangeRecorder(in changeRecorder) *changeRecorder {
	rec := in
	if rec.routeDevice == nil {
		rec.routeDevice = func(*store.Device) string { return "" }
	}
	if rec.routeProduct == nil {
		rec.routeProduct = func(*store.Product) string { return "" }
	}
	return &rec
}

// suppressed reports whether a statement must not produce a change.
//
// The primary guard is the session marker set by the replay path. The second is
// data-driven: the applier stamps every entity it writes with the peer's node
// id, so a write carrying a foreign writer can never be a local mutation. The
// second guard matters because failing to suppress a replayed write would make
// the two nodes echo the same change indefinitely.
func (r *changeRecorder) suppressed(db *gorm.DB) bool {
	if db == nil || db.Statement == nil {
		return true
	}
	if _, ok := db.Get(store.SyncApplyingKey); ok {
		return true
	}
	switch dest := db.Statement.Dest.(type) {
	case *store.Device:
		return foreignWriter(dest.SyncWriter, r.nodeID)
	case store.Device:
		return foreignWriter(dest.SyncWriter, r.nodeID)
	case *store.Product:
		return foreignWriter(dest.SyncWriter, r.nodeID)
	case store.Product:
		return foreignWriter(dest.SyncWriter, r.nodeID)
	}
	return false
}

func foreignWriter(writer, nodeID string) bool {
	return writer != "" && writer != nodeID
}

// --- Device recording ---

func (r *changeRecorder) recordDeviceCreated(db *gorm.DB) {
	if r.suppressed(db) {
		return
	}
	r.recordDeviceUpsert(deviceCodeFrom(db))
}

func (r *changeRecorder) recordDeviceUpdated(db *gorm.DB) {
	if r.suppressed(db) {
		return
	}
	if !contentChanged(db.Statement.Dest, deviceContentColumns) {
		return
	}
	if isCodeOnlyUpdate(db.Statement.Dest) {
		// DeleteDevice renames the code before soft-deleting the row. Recording
		// that as an update would resurrect the device on the peer; the deletion
		// is reported separately through the store mutation sink.
		return
	}
	r.recordDeviceUpsert(deviceCodeFrom(db))
}

func (r *changeRecorder) recordDeviceUpsert(code string) {
	if code == "" {
		return
	}
	var recorded *store.SyncChange
	err := store.SyncApplyingSession().Transaction(func(tx *gorm.DB) error {
		var stored store.Device
		if err := tx.Where("code = ?", code).First(&stored).Error; err != nil {
			// The row was removed in the same breath as the write; there is
			// nothing to propagate as an upsert.
			return nil
		}
		newRev := stored.Rev + 1
		snapshot := stored
		snapshot.Rev = newRev
		payload, err := buildDevicePayload(&snapshot)
		if err != nil {
			return err
		}
		updates := map[string]interface{}{
			"rev":        newRev,
			"sync_state": store.SyncStatePending,
		}
		if stored.Origin == "" {
			updates["origin"] = r.origin
		}
		if stored.OwnerNode == "" && r.ownerNode != "" {
			updates["owner_node"] = r.ownerNode
			stored.OwnerNode = r.ownerNode
		}
		// Guard on the observed revision so two writers racing on the same
		// entity cannot both record a change claiming the same new revision.
		res := tx.Model(&store.Device{}).Where("code = ? AND rev = ?", code, stored.Rev).Updates(updates)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return nil
		}
		ch := &store.SyncChange{
			EntityType:     store.SyncEntityDevice,
			EntityCode:     code,
			Op:             store.SyncOpUpsert,
			BaseRev:        stored.Rev,
			NewRev:         newRev,
			Payload:        payload,
			OriginNode:     r.nodeID,
			OriginInstance: r.instanceID,
			TargetNode:     r.routeDevice(&stored),
			TenantID:       stored.TenantID,
			ProjectID:      stored.ProjectID,
			CreatedAt:      time.Now().UnixMilli(),
		}
		if _, err := store.AppendSyncChangeTx(tx, ch); err != nil {
			return err
		}
		recorded = ch
		return nil
	})
	r.finish(recorded, "device", code, err)
}

// DeviceDeleted implements store.SyncChangeSink. The row has already been
// renamed and soft-deleted, so the revision is taken from the caller's snapshot
// and used as the conflict precondition on the receiving side: a peer that
// edited the device concurrently keeps its version and reports a conflict
// instead of losing the edit.
func (r *changeRecorder) DeviceDeleted(d *store.Device) {
	if d == nil || d.Code == "" {
		return
	}
	ch := &store.SyncChange{
		EntityType:     store.SyncEntityDevice,
		EntityCode:     d.Code,
		Op:             store.SyncOpDelete,
		BaseRev:        d.Rev,
		NewRev:         d.Rev + 1,
		OriginNode:     r.nodeID,
		OriginInstance: r.instanceID,
		TargetNode:     r.routeDevice(d),
		TenantID:       d.TenantID,
		ProjectID:      d.ProjectID,
		CreatedAt:      time.Now().UnixMilli(),
	}
	if _, err := store.AppendSyncChange(ch); err != nil {
		r.logger.Error("Failed to record device deletion",
			zap.String("code", d.Code), zap.Error(err))
		return
	}
	r.dispatch(*ch)
}

// --- Product recording ---

func (r *changeRecorder) recordProductCreated(db *gorm.DB) {
	if r.suppressed(db) {
		return
	}
	r.recordProductUpsert(productCodeFrom(db))
}

func (r *changeRecorder) recordProductUpdated(db *gorm.DB) {
	if r.suppressed(db) {
		return
	}
	if !contentChanged(db.Statement.Dest, productContentColumns) {
		return
	}
	r.recordProductUpsert(productCodeFrom(db))
}

func (r *changeRecorder) recordProductUpsert(code string) {
	if code == "" {
		return
	}
	var recorded *store.SyncChange
	err := store.SyncApplyingSession().Transaction(func(tx *gorm.DB) error {
		var stored store.Product
		if err := tx.Where("code = ?", code).First(&stored).Error; err != nil {
			return nil
		}
		newRev := stored.Rev + 1
		snapshot := stored
		snapshot.Rev = newRev
		payload, err := buildProductPayload(&snapshot)
		if err != nil {
			return err
		}
		updates := map[string]interface{}{
			"rev":        newRev,
			"sync_state": store.SyncStatePending,
		}
		if stored.Origin == "" {
			updates["origin"] = r.origin
		}
		res := tx.Model(&store.Product{}).Where("code = ? AND rev = ?", code, stored.Rev).Updates(updates)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return nil
		}
		ch := &store.SyncChange{
			EntityType:     store.SyncEntityProduct,
			EntityCode:     code,
			Op:             store.SyncOpUpsert,
			BaseRev:        stored.Rev,
			NewRev:         newRev,
			Payload:        payload,
			OriginNode:     r.nodeID,
			OriginInstance: r.instanceID,
			TargetNode:     r.routeProduct(&stored),
			TenantID:       stored.TenantID,
			ProjectID:      stored.ProjectID,
			CreatedAt:      time.Now().UnixMilli(),
		}
		if _, err := store.AppendSyncChangeTx(tx, ch); err != nil {
			return err
		}
		recorded = ch
		return nil
	})
	r.finish(recorded, "product", code, err)
}

// --- Shared helpers ---

func (r *changeRecorder) finish(ch *store.SyncChange, kind, code string, err error) {
	if err != nil {
		r.logger.Error("Failed to record local change",
			zap.String("entity_type", kind), zap.String("code", code), zap.Error(err))
		return
	}
	if ch == nil {
		return
	}
	r.logger.Debug("Recorded local change",
		zap.String("entity_type", kind),
		zap.String("code", code),
		zap.String("change_id", ch.ChangeID()))
	r.dispatch(*ch)
}

func (r *changeRecorder) dispatch(ch store.SyncChange) {
	if r.notify != nil {
		r.notify(ch)
	}
}

// contentChanged reports whether an update statement touched entity content.
// A struct destination (Save) always carries content; a column map does only
// when at least one of its keys is a content column.
func contentChanged(dest interface{}, columns map[string]bool) bool {
	switch v := dest.(type) {
	case map[string]interface{}:
		if len(v) == 0 {
			return true
		}
		for key := range v {
			if columns[strings.ToLower(key)] {
				return true
			}
		}
		return false
	case map[string]string:
		if len(v) == 0 {
			return true
		}
		for key := range v {
			if columns[strings.ToLower(key)] {
				return true
			}
		}
		return false
	}
	return true
}

// isCodeOnlyUpdate detects the single-column code rename that DeleteDevice
// performs. Renaming an entity code is not a supported user operation, so the
// only producer of this shape is the deletion path.
func isCodeOnlyUpdate(dest interface{}) bool {
	switch v := dest.(type) {
	case map[string]interface{}:
		if len(v) != 1 {
			return false
		}
		for k := range v {
			return strings.EqualFold(k, "code")
		}
	case map[string]string:
		if len(v) != 1 {
			return false
		}
		for k := range v {
			return strings.EqualFold(k, "code")
		}
	}
	return false
}

func deviceCodeFrom(db *gorm.DB) string {
	if db == nil || db.Statement == nil {
		return ""
	}
	if d, ok := db.Statement.Dest.(*store.Device); ok && d != nil {
		return d.Code
	}
	if d, ok := db.Statement.Dest.(store.Device); ok {
		return d.Code
	}
	if m, ok := db.Statement.Model.(*store.Device); ok && m != nil {
		return m.Code
	}
	return ""
}

func productCodeFrom(db *gorm.DB) string {
	if db == nil || db.Statement == nil {
		return ""
	}
	if p, ok := db.Statement.Dest.(*store.Product); ok && p != nil {
		return p.Code
	}
	if p, ok := db.Statement.Dest.(store.Product); ok {
		return p.Code
	}
	if m, ok := db.Statement.Model.(*store.Product); ok && m != nil {
		return m.Code
	}
	return ""
}

// registerChangeHooks installs the create/update callbacks that feed the change
// log. Deletions are deliberately not registered here; they are reported through
// store.SyncChangeSink because the deletion path destroys the original code
// before any callback can observe it.
func registerChangeHooks(rec *changeRecorder) error {
	creates := map[string]func(*gorm.DB){
		"cascade:sync_device_create":  rec.recordDeviceCreated,
		"cascade:sync_product_create": rec.recordProductCreated,
	}
	updates := map[string]func(*gorm.DB){
		"cascade:sync_device_update":  rec.recordDeviceUpdated,
		"cascade:sync_product_update": rec.recordProductUpdated,
	}
	for name, handler := range creates {
		if err := store.DB.Callback().Create().After("gorm:commit_or_rollback_transaction").Register(name, handler); err != nil {
			return fmt.Errorf("register %s hook: %w", name, err)
		}
	}
	for name, handler := range updates {
		if err := store.DB.Callback().Update().After("gorm:commit_or_rollback_transaction").Register(name, handler); err != nil {
			return fmt.Errorf("register %s hook: %w", name, err)
		}
	}
	return nil
}
