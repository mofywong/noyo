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
	"source_template_code": true, "source_template_rev": true, "owner_node": true,
}

// changeRecorder turns local writes into change-log entries. It is driven by
// GORM callbacks so every write path (UI, AI assistant, HTTP API, internal
// timers) is covered without modifying each call site.
type changeRecorder struct {
	// nodeID is this node's identity and becomes OriginNode on every change.
	nodeID          string
	separateIntents bool
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
	routeScope   func(tenantID, projectID uint) string
}

func newChangeRecorder(in changeRecorder) *changeRecorder {
	rec := in
	if rec.routeDevice == nil {
		rec.routeDevice = func(*store.Device) string { return "" }
	}
	if rec.routeProduct == nil {
		rec.routeProduct = func(*store.Product) string { return "" }
	}
	if rec.routeScope == nil {
		rec.routeScope = func(uint, uint) string { return "" }
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
	case *store.ProtocolProfile:
		return foreignWriter(dest.SyncWriter, r.nodeID)
	case *store.Rule:
		return foreignWriter(dest.SyncWriter, r.nodeID)
	case *store.RuleGroup:
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
	r.recordDeviceUpsert(db, deviceCodeFrom(db), true)
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
	r.recordDeviceUpsert(db, deviceCodeFrom(db), false)
}

func (r *changeRecorder) recordDeviceUpsert(db *gorm.DB, code string, created bool) {
	if code == "" {
		return
	}
	var recorded *store.SyncChange
	tx := db.Session(&gorm.Session{NewDB: true})
	err := func() error {
		var stored store.Device
		if err := tx.Where("code = ?", code).First(&stored).Error; err != nil {
			// The row was removed in the same breath as the write; there is
			// nothing to propagate as an upsert.
			return nil
		}
		newRev, baseRev := stored.Rev+1, stored.Rev
		if created {
			newRev, baseRev = 1, 0
		}
		lifecycle, advancedRev, err := r.nextResourceVersionTx(tx, store.SyncEntityDevice, code, created, false, newRev)
		if err != nil {
			return err
		}
		newRev = advancedRev
		updates := map[string]interface{}{
			"rev":        newRev,
			"sync_state": store.SyncStatePending,
		}
		if stored.Origin == "" {
			updates["origin"] = r.origin
			stored.Origin = r.origin
		}
		if stored.OwnerNode == "" && r.ownerNode != "" {
			updates["owner_node"] = r.ownerNode
			stored.OwnerNode = r.ownerNode
		}
		target := r.routeDevice(&stored)
		if r.nodeID == cloudSyncNodeID && target == "" {
			return fmt.Errorf("device %s has no unambiguous gateway code", code)
		}
		snapshot := stored
		snapshot.Rev = newRev
		payload, err := buildDevicePayload(&snapshot)
		if err != nil {
			return err
		}
		// Guard on the observed revision so two writers racing on the same
		// entity cannot both record a change claiming the same new revision.
		res := tx.Model(&store.Device{}).Set(store.SyncApplyingKey, true).
			Where("code = ? AND rev = ?", code, stored.Rev).Updates(updates)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return nil
		}
		ch := &store.SyncChange{
			EntityType:       store.SyncEntityDevice,
			EntityCode:       code,
			Op:               store.SyncOpUpsert,
			BaseRev:          baseRev,
			NewRev:           newRev,
			LifecycleVersion: lifecycle,
			Payload:          payload,
			BasePayload:      capturedSyncBase(db),
			OriginNode:       r.nodeID,
			OriginInstance:   r.instanceID,
			TargetNode:       target,
			TenantID:         stored.TenantID,
			ProjectID:        stored.ProjectID,
			CreatedAt:        time.Now().UnixMilli(),
		}
		if err := collectConfigDependenciesTx(tx, ch); err != nil {
			return err
		}
		if _, err := store.AppendSyncChangeTx(tx, ch); err != nil {
			return err
		}
		recorded = ch
		return nil
	}()
	r.finishTx(db, recorded, "device", code, err)
}

// DeviceDeletedTx appends the tombstone in the same transaction as the delete.
func (r *changeRecorder) DeviceDeletedTx(tx *gorm.DB, d *store.Device) (*store.SyncChange, error) {
	if d == nil || d.Code == "" {
		return nil, nil
	}
	target := r.routeDevice(d)
	if r.nodeID == cloudSyncNodeID && target == "" {
		return nil, fmt.Errorf("device %s has no unambiguous gateway code", d.Code)
	}
	lifecycle, newRev, err := r.nextResourceVersionTx(tx, store.SyncEntityDevice, d.Code, false, true, d.Rev+1)
	if err != nil {
		return nil, err
	}
	ch := &store.SyncChange{
		EntityType:       store.SyncEntityDevice,
		EntityCode:       d.Code,
		Op:               store.SyncOpDelete,
		BaseRev:          d.Rev,
		NewRev:           newRev,
		LifecycleVersion: lifecycle,
		OriginNode:       r.nodeID,
		OriginInstance:   r.instanceID,
		TargetNode:       target,
		TenantID:         d.TenantID,
		ProjectID:        d.ProjectID,
		CreatedAt:        time.Now().UnixMilli(),
	}
	_, err = store.AppendSyncChangeTx(tx, ch)
	if err == nil {
		err = r.stagePlatformDeleteTx(tx, ch, d)
	}
	if err == nil {
		err = r.queueLocalRuntimeTx(tx, ch)
	}
	return ch, err
}

func (r *changeRecorder) ProductDeletedTx(tx *gorm.DB, p *store.Product) (*store.SyncChange, error) {
	if p == nil || p.Code == "" {
		return nil, nil
	}
	lifecycle, newRev, err := r.nextResourceVersionTx(tx, store.SyncEntityProduct, p.Code, false, true, p.Rev+1)
	if err != nil {
		return nil, err
	}
	ch := &store.SyncChange{
		EntityType: store.SyncEntityProduct, EntityCode: p.Code,
		Op: store.SyncOpDelete, BaseRev: p.Rev, NewRev: newRev, LifecycleVersion: lifecycle,
		OriginNode: r.nodeID, OriginInstance: r.instanceID,
		TargetNode: r.routeProduct(p), TenantID: p.TenantID, ProjectID: p.ProjectID,
		CreatedAt: time.Now().UnixMilli(),
	}
	if r.nodeID == cloudSyncNodeID && p.OwnerNode == "" {
		ch.AckTargets = r.routeScope(p.TenantID, p.ProjectID)
	}
	_, err = store.AppendSyncChangeTx(tx, ch)
	if err == nil {
		err = r.stagePlatformDeleteTx(tx, ch, p)
	}
	if err == nil {
		err = r.queueLocalRuntimeTx(tx, ch)
	}
	return ch, err
}

func (r *changeRecorder) ChangeCommitted(ch store.SyncChange) {
	r.dispatch(ch)
}

// --- Product recording ---

func (r *changeRecorder) recordProductCreated(db *gorm.DB) {
	if r.suppressed(db) {
		return
	}
	r.recordProductUpsert(db, productCodeFrom(db), true)
}

func (r *changeRecorder) recordProductUpdated(db *gorm.DB) {
	if r.suppressed(db) {
		return
	}
	if isCodeOnlyUpdate(db.Statement.Dest) {
		return
	}
	if !contentChanged(db.Statement.Dest, productContentColumns) {
		return
	}
	r.recordProductUpsert(db, productCodeFrom(db), false)
}

func (r *changeRecorder) recordProductUpsert(db *gorm.DB, code string, created bool) {
	if code == "" {
		return
	}
	var recorded *store.SyncChange
	tx := db.Session(&gorm.Session{NewDB: true})
	err := func() error {
		var stored store.Product
		if err := tx.Where("code = ?", code).First(&stored).Error; err != nil {
			return nil
		}
		if created && r.ownerNode != "" && stored.OwnerNode == "" {
			stored.OwnerNode = r.ownerNode
			if err := tx.Model(&store.Product{}).Set(store.SyncApplyingKey, true).Where("code = ?", code).Update("owner_node", r.ownerNode).Error; err != nil {
				return err
			}
		}
		newRev, baseRev := stored.Rev+1, stored.Rev
		if created {
			newRev, baseRev = 1, 0
		}
		lifecycle, advancedRev, err := r.nextResourceVersionTx(tx, store.SyncEntityProduct, code, created, false, newRev)
		if err != nil {
			return err
		}
		newRev = advancedRev
		snapshot := stored
		snapshot.Rev = newRev
		payload, err := buildProductPayload(&snapshot)
		if err != nil {
			return err
		}
		ackTargets := ""
		syncState := store.SyncStatePending
		if r.nodeID == cloudSyncNodeID && stored.OwnerNode == "" {
			ackTargets = r.routeScope(stored.TenantID, stored.ProjectID)
			if ackTargets == "" {
				syncState = store.SyncStateSynced
			}
		}
		updates := map[string]interface{}{
			"rev":        newRev,
			"sync_state": syncState,
		}
		if stored.Origin == "" {
			updates["origin"] = r.origin
		}
		res := tx.Model(&store.Product{}).Set(store.SyncApplyingKey, true).
			Where("code = ? AND rev = ?", code, stored.Rev).Updates(updates)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return nil
		}
		ch := &store.SyncChange{
			EntityType:       store.SyncEntityProduct,
			EntityCode:       code,
			Op:               store.SyncOpUpsert,
			BaseRev:          baseRev,
			NewRev:           newRev,
			LifecycleVersion: lifecycle,
			Payload:          payload,
			BasePayload:      capturedSyncBase(db),
			OriginNode:       r.nodeID,
			OriginInstance:   r.instanceID,
			TargetNode:       r.routeProduct(&stored),
			TenantID:         stored.TenantID,
			ProjectID:        stored.ProjectID,
			CreatedAt:        time.Now().UnixMilli(),
		}
		ch.AckTargets = ackTargets
		if _, err := store.AppendSyncChangeTx(tx, ch); err != nil {
			return err
		}
		recorded = ch
		return nil
	}()
	r.finishTx(db, recorded, "product", code, err)
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

func (r *changeRecorder) finishTx(db *gorm.DB, ch *store.SyncChange, kind, code string, err error) {
	if err == nil && ch != nil {
		err = r.stagePlatformIntentTx(db, ch)
	}
	if err == nil {
		err = r.queueLocalRuntimeTx(db, ch)
	}
	if err != nil {
		// The outer business transaction must roll back with the failed outbox
		// append. Logging alone would make a successful API write disappear from
		// the sync stream after a crash.
		db.AddError(err)
		r.logger.Error("Failed to record local change", zap.String("entity_type", kind), zap.String("code", code), zap.Error(err))
		return
	}
	if ch != nil && r.notify != nil {
		// An explicit outer transaction may still roll back. Dispatch only after
		// the change is visible on the committed database connection.
		go r.dispatchWhenCommitted(store.DB, *ch)
	}
}

// A local tombstone must supersede any retry of the previous upsert in the
// same transaction. Otherwise snapshots report an obsolete desired revision.
func (r *changeRecorder) queueLocalRuntimeTx(tx *gorm.DB, ch *store.SyncChange) error {
	if ch == nil || r.nodeID == cloudSyncNodeID || !tx.Migrator().HasTable(&store.RuntimeApply{}) {
		return nil
	}
	return store.QueueRuntimeApplyTx(tx, *ch)
}

func (r *changeRecorder) dispatchWhenCommitted(db *gorm.DB, ch store.SyncChange) {
	for attempt := 0; attempt < 30; attempt++ {
		var count int64
		if err := db.Model(&store.SyncChange{}).Where("seq = ? AND origin_node = ?", ch.Seq, ch.OriginNode).Count(&count).Error; err == nil && count == 1 {
			r.dispatch(ch)
			return
		}
		time.Sleep(time.Second)
	}
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
	store.SetProtectedConfigSink(rec)
	bases := map[string]string{
		"cascade:sync_device_base":     store.SyncEntityDevice,
		"cascade:sync_product_base":    store.SyncEntityProduct,
		"cascade:sync_driver_base":     store.SyncEntityDriver,
		"cascade:sync_rule_base":       store.SyncEntityRule,
		"cascade:sync_rule_group_base": store.SyncEntityRuleGroup,
	}
	for name, kind := range bases {
		if err := store.DB.Callback().Update().Before("gorm:update").Register(name, rec.captureSyncBase(kind)); err != nil {
			return fmt.Errorf("register %s hook: %w", name, err)
		}
	}
	creates := map[string]func(*gorm.DB){
		"cascade:sync_device_create":     rec.recordDeviceCreated,
		"cascade:sync_product_create":    rec.recordProductCreated,
		"cascade:sync_driver_create":     rec.recordDriverCreated,
		"cascade:sync_rule_create":       rec.recordRuleCreated,
		"cascade:sync_rule_group_create": rec.recordRuleGroupCreated,
	}
	updates := map[string]func(*gorm.DB){
		"cascade:sync_device_update":     rec.recordDeviceUpdated,
		"cascade:sync_product_update":    rec.recordProductUpdated,
		"cascade:sync_driver_update":     rec.recordDriverUpdated,
		"cascade:sync_rule_update":       rec.recordRuleUpdated,
		"cascade:sync_rule_group_update": rec.recordRuleGroupUpdated,
	}
	for name, handler := range creates {
		if err := store.DB.Callback().Create().After("gorm:create").Before("gorm:commit_or_rollback_transaction").Register(name, handler); err != nil {
			return fmt.Errorf("register %s hook: %w", name, err)
		}
	}
	for name, handler := range updates {
		if err := store.DB.Callback().Update().After("gorm:update").Before("gorm:commit_or_rollback_transaction").Register(name, handler); err != nil {
			return fmt.Errorf("register %s hook: %w", name, err)
		}
	}
	return nil
}
