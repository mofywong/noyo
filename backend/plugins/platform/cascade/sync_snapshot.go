package cascade

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"noyo/core/store"
)

const snapshotChunkSize = 50

type configSnapshot struct {
	GatewayCode   string                    `json:"gateway_code"`
	InstanceID    string                    `json:"instance_id"`
	CommitSeq     int64                     `json:"commit_seq"`
	TenantID      uint                      `json:"tenant_id"`
	ProjectID     uint                      `json:"project_id"`
	SchemaVersion int                       `json:"schema_version"`
	Maintenance   *store.GatewayMaintenance `json:"maintenance,omitempty"`
	Runtime       []store.RuntimeApply      `json:"runtime"`
	Resources     []store.SyncChange        `json:"resources"`
}

func snapshotCode() string {
	var data [24]byte
	if _, err := rand.Read(data[:]); err != nil {
		panic(err)
	}
	return "SNAP_" + hex.EncodeToString(data[:])
}

func snapshotHash(value interface{}) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func (s *syncSession) emit(msg *syncMessage) error {
	msg.NodeID = s.nodeID
	msg.InstanceID = s.instanceID
	msg.ProtocolVersion = syncProtocolVersion
	msg.SessionToken = s.deliveryToken
	return s.publish(s.peer, msg)
}

func appendProtectedSnapshotTx(tx *gorm.DB, snapshot *configSnapshot) error {
	if !tx.Migrator().HasTable(&store.SystemConfig{}) {
		return nil
	}
	configs, err := store.ListProtectedConfigsTx(tx)
	if err != nil {
		return err
	}
	for _, cfg := range configs {
		if cfg.GatewaySN != snapshot.GatewayCode {
			continue
		}
		payload, err := json.Marshal(cfg)
		if err != nil {
			return err
		}
		kind := store.SyncEntityNodeConfig
		if cfg.Kind == "plugin" {
			kind = store.SyncEntityPluginConfig
		}
		snapshot.Resources = append(snapshot.Resources, store.SyncChange{EntityType: kind, EntityCode: cfg.Code, Op: store.SyncOpUpsert, NewRev: cfg.Revision, LifecycleVersion: 1, Payload: string(payload), OriginNode: snapshot.GatewayCode, OriginInstance: snapshot.InstanceID, TargetNode: cloudSyncNodeID, TenantID: cfg.TenantID, ProjectID: cfg.ProjectID})
	}
	return nil
}

func resourceSnapshotTx(tx *gorm.DB, kind, code string) (string, int64, error) {
	value := configEntity(kind)
	if value == nil {
		return "", 0, fmt.Errorf("unknown snapshot resource %s", kind)
	}
	if err := tx.Session(&gorm.Session{NewDB: true}).Where("code = ?", code).Take(value).Error; err != nil {
		return "", 0, err
	}
	switch v := value.(type) {
	case *store.Device:
		p, e := buildDevicePayload(v)
		return p, v.Rev, e
	case *store.Product:
		p, e := buildProductPayload(v)
		return p, v.Rev, e
	case *store.ProtocolProfile:
		p, e := driverCanonicalPayload(v)
		return p, v.Rev, e
	case *store.Rule:
		p, e := ruleCanonicalPayload(v)
		return p, v.Version, e
	case *store.RuleGroup:
		v.ID = 0
		v.CreatedAt = 0
		v.UpdatedAt = 0
		clearConflictState(&v.SyncMeta)
		v.SyncState = store.SyncStateSynced
		p, e := json.Marshal(v)
		return string(p), v.Rev, e
	}
	return "", 0, fmt.Errorf("unknown snapshot resource")
}

// Read the manifest and its log boundary in one database snapshot. Later writes
// remain in the log beyond CommitSeq and are sent after the snapshot receipt.
func buildGatewaySnapshot(gateway, instance string, tenant, project uint) (*configSnapshot, error) {
	result := &configSnapshot{GatewayCode: gateway, InstanceID: instance, TenantID: tenant, ProjectID: project, SchemaVersion: 1, Resources: []store.SyncChange{}, Runtime: []store.RuntimeApply{}}
	err := store.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&store.SyncChange{}).Select("COALESCE(MAX(seq),0)").Scan(&result.CommitSeq).Error; err != nil {
			return err
		}
		for _, kind := range []string{store.SyncEntityProduct, store.SyncEntityDriver, store.SyncEntityRuleGroup, store.SyncEntityDevice, store.SyncEntityRule} {
			var codes []string
			query := tx.Model(configEntity(kind))
			if tenant != 0 {
				query = query.Where("tenant_id = ?", tenant)
			}
			if project != 0 {
				query = query.Where("project_id = ?", project)
			}
			if err := query.Order("code").Pluck("code", &codes).Error; err != nil {
				return err
			}
			for _, code := range codes {
				payload, rev, err := resourceSnapshotTx(tx, kind, code)
				if err != nil {
					return err
				}
				var scope struct {
					TenantID  uint `json:"tenant_id"`
					ProjectID uint `json:"project_id"`
				}
				if err := json.Unmarshal([]byte(payload), &scope); err != nil {
					return err
				}
				version := int64(1)
				var lifecycle store.ResourceLifecycle
				if err := tx.Where("entity_type = ? AND code = ?", kind, code).Take(&lifecycle).Error; err == nil {
					version = lifecycle.Version
				} else if !errors.Is(err, gorm.ErrRecordNotFound) {
					return err
				}
				result.Resources = append(result.Resources, store.SyncChange{EntityType: kind, EntityCode: code, Op: store.SyncOpUpsert, NewRev: rev, LifecycleVersion: version, Payload: payload, OriginNode: gateway, OriginInstance: instance, TargetNode: cloudSyncNodeID, TenantID: scope.TenantID, ProjectID: scope.ProjectID})
			}
		}
		var tombstones []store.ResourceLifecycle
		if err := tx.Where("deleted = ?", true).Find(&tombstones).Error; err != nil {
			return err
		}
		for _, dead := range tombstones {
			var prior store.SyncChange
			err := tx.Where("entity_type = ? AND entity_code = ? AND origin_node = ?", dead.EntityType, dead.Code, gateway).Order("seq desc").Take(&prior).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				prior.TenantID = tenant
				prior.ProjectID = project
			} else if err != nil {
				return err
			}
			result.Resources = append(result.Resources, store.SyncChange{EntityType: dead.EntityType, EntityCode: dead.Code, Op: store.SyncOpDelete, NewRev: dead.LastRev, LifecycleVersion: dead.Version, OriginNode: gateway, OriginInstance: instance, TargetNode: cloudSyncNodeID, TenantID: prior.TenantID, ProjectID: prior.ProjectID})
		}
		if tx.Migrator().HasTable(&store.RuntimeApply{}) {
			if err := tx.Find(&result.Runtime).Error; err != nil {
				return err
			}
		}
		if tx.Migrator().HasTable(&store.GatewayMaintenance{}) {
			state, err := store.GetGatewayMaintenanceTx(tx, gateway)
			if err != nil {
				return err
			}
			result.Maintenance = state
		}
		if err := appendProtectedSnapshotTx(tx, result); err != nil {
			return err
		}
		// Runtime rows also include built-in products and historical resources
		// outside this snapshot's scope. Historical deletes may also leave a
		// retry for an older upsert. Report only the current desired version;
		// ActiveRevision separately describes the version still running.
		manifest := make(map[string]store.SyncChange, len(result.Resources))
		for _, resource := range result.Resources {
			manifest[resource.EntityType+":"+resource.EntityCode] = resource
		}
		runtime := result.Runtime[:0]
		for _, row := range result.Runtime {
			resource, present := manifest[row.EntityType+":"+row.EntityCode]
			if present && row.Revision == resource.NewRev && row.LifecycleVersion == resource.LifecycleVersion {
				runtime = append(runtime, row)
			}
		}
		result.Runtime = runtime
		return nil
	})
	sort.Slice(result.Resources, func(i, j int) bool {
		a, b := result.Resources[i], result.Resources[j]
		return a.EntityType+":"+a.EntityCode < b.EntityType+":"+b.EntityCode
	})
	return result, err
}

func (s *syncSession) requestSnapshot(cursor *store.SyncCursor) error {
	if s.installSnapshot == nil {
		return fmt.Errorf("snapshot recovery unavailable for %s", s.peer.nodeID)
	}
	stage := store.SyncSnapshotStage{PeerCode: s.peer.nodeID, RequestCode: snapshotCode(), PeerInstance: cursor.PeerInstance, LocalInstance: s.instanceID, SessionToken: s.deliveryToken, Chunks: "{}"}
	if err := store.DB.Clauses(clause.OnConflict{UpdateAll: true}).Create(&stage).Error; err != nil {
		return err
	}
	s.snapshotBlocked = true
	return s.emit(&syncMessage{Type: syncMsgSnapshotRequest, SnapshotCode: stage.RequestCode})
}

func (s *syncSession) retrySnapshot(cursor *store.SyncCursor) error {
	var stage store.SyncSnapshotStage
	err := store.DB.Where("peer_code = ?", s.peer.nodeID).Take(&stage).Error
	if err == nil && !stage.Completed && stage.PeerInstance == cursor.PeerInstance && stage.LocalInstance == s.instanceID && stage.SessionToken == s.deliveryToken {
		return s.emit(&syncMessage{Type: syncMsgSnapshotRequest, SnapshotCode: stage.RequestCode})
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return s.requestSnapshot(cursor)
}

func (s *syncSession) sendSnapshot(msg *syncMessage) error {
	if s.buildSnapshot == nil || msg.SnapshotCode == "" {
		return fmt.Errorf("only a gateway may publish an authoritative snapshot")
	}
	if !s.snapshotBlocked && s.snapshotPayload != nil && s.snapshotSentCode == msg.SnapshotCode {
		hash, err := snapshotHash(s.snapshotPayload)
		if err != nil {
			return err
		}
		return s.emit(&syncMessage{Type: syncMsgSnapshotEnd, SnapshotCode: msg.SnapshotCode, SnapshotHash: hash, AppliedThrough: s.snapshotSentSeq})
	}
	snapshot := s.snapshotPayload
	if snapshot == nil || s.snapshotSentCode != msg.SnapshotCode {
		var err error
		snapshot, err = s.buildSnapshot()
		if err != nil {
			return err
		}
		s.snapshotPayload = snapshot
		s.snapshotSentCode = msg.SnapshotCode
		s.snapshotSentSeq = snapshot.CommitSeq
	}
	hash, err := snapshotHash(snapshot)
	if err != nil {
		return err
	}
	count := (len(snapshot.Resources) + snapshotChunkSize - 1) / snapshotChunkSize
	header := *snapshot
	header.Resources = nil
	s.snapshotBlocked = true
	if err := s.emit(&syncMessage{Type: syncMsgSnapshotBegin, SnapshotCode: msg.SnapshotCode, Snapshot: &header, SnapshotHash: hash, SnapshotParts: count}); err != nil {
		return err
	}
	for index := 0; index < count; index++ {
		end := min(len(snapshot.Resources), (index+1)*snapshotChunkSize)
		chunk := snapshot.Resources[index*snapshotChunkSize : end]
		chunkHash, err := snapshotHash(chunk)
		if err != nil {
			return err
		}
		if err := s.emit(&syncMessage{Type: syncMsgSnapshotChunk, SnapshotCode: msg.SnapshotCode, SnapshotIndex: index, Changes: chunk, SnapshotHash: chunkHash}); err != nil {
			return err
		}
	}
	s.snapshotSentCode = msg.SnapshotCode
	s.snapshotSentSeq = snapshot.CommitSeq
	return s.emit(&syncMessage{Type: syncMsgSnapshotEnd, SnapshotCode: msg.SnapshotCode, SnapshotHash: hash, AppliedThrough: snapshot.CommitSeq})
}

func (s *syncSession) receiveSnapshot(cursor *store.SyncCursor, msg *syncMessage) error {
	if s.installSnapshot == nil {
		return fmt.Errorf("gateway cannot install a platform snapshot")
	}
	var stage store.SyncSnapshotStage
	if err := store.DB.Where("peer_code = ?", s.peer.nodeID).Take(&stage).Error; err != nil {
		return err
	}
	if stage.RequestCode != msg.SnapshotCode || stage.PeerInstance != msg.InstanceID || stage.LocalInstance != s.instanceID {
		return fmt.Errorf("snapshot generation or request mismatch")
	}
	if stage.Completed {
		if msg.Type == syncMsgSnapshotEnd {
			var completed syncMessage
			if err := json.Unmarshal([]byte(stage.Header), &completed); err != nil || completed.Snapshot == nil {
				return fmt.Errorf("completed snapshot manifest unavailable")
			}
			return s.emit(&syncMessage{Type: syncMsgSnapshotAck, SnapshotCode: msg.SnapshotCode, AppliedThrough: completed.Snapshot.CommitSeq})
		}
		return nil
	}
	var header syncMessage
	if stage.Header != "" {
		if err := json.Unmarshal([]byte(stage.Header), &header); err != nil {
			return err
		}
	}
	chunks := map[int][]store.SyncChange{}
	if err := json.Unmarshal([]byte(stage.Chunks), &chunks); err != nil {
		return err
	}
	switch msg.Type {
	case syncMsgSnapshotBegin:
		if msg.Snapshot == nil || msg.Snapshot.InstanceID != msg.InstanceID || msg.Snapshot.GatewayCode != s.peer.nodeID || msg.Snapshot.SchemaVersion != 1 || msg.SnapshotParts < 0 || msg.SnapshotParts > 1000 {
			return fmt.Errorf("invalid snapshot manifest")
		}
		if stage.Header != "" && (header.SnapshotHash != msg.SnapshotHash || header.SnapshotParts != msg.SnapshotParts) {
			return fmt.Errorf("snapshot manifest changed")
		}
		data, err := json.Marshal(msg)
		if err != nil {
			return err
		}
		stage.Header = string(data)
	case syncMsgSnapshotChunk:
		if header.Snapshot == nil || msg.SnapshotIndex < 0 || msg.SnapshotIndex >= header.SnapshotParts || len(msg.Changes) > snapshotChunkSize {
			return fmt.Errorf("invalid snapshot chunk")
		}
		hash, err := snapshotHash(msg.Changes)
		if err != nil || hash != msg.SnapshotHash {
			return fmt.Errorf("snapshot chunk checksum mismatch")
		}
		if old, ok := chunks[msg.SnapshotIndex]; ok {
			prior, _ := snapshotHash(old)
			if prior != hash {
				return fmt.Errorf("snapshot chunk changed")
			}
		}
		chunks[msg.SnapshotIndex] = msg.Changes
		data, err := json.Marshal(chunks)
		if err != nil {
			return err
		}
		stage.Chunks = string(data)
	case syncMsgSnapshotEnd:
		if header.Snapshot == nil || len(chunks) != header.SnapshotParts {
			return fmt.Errorf("incomplete snapshot")
		}
		snapshot := *header.Snapshot
		snapshot.Resources = []store.SyncChange{}
		for i := 0; i < header.SnapshotParts; i++ {
			part, ok := chunks[i]
			if !ok {
				return fmt.Errorf("missing snapshot chunk")
			}
			snapshot.Resources = append(snapshot.Resources, part...)
		}
		hash, err := snapshotHash(&snapshot)
		if err != nil || hash != header.SnapshotHash || hash != msg.SnapshotHash || msg.AppliedThrough != snapshot.CommitSeq {
			return fmt.Errorf("snapshot manifest checksum mismatch")
		}
		err = store.SyncApplyingSession().Transaction(func(tx *gorm.DB) error {
			if err := s.installSnapshot(tx, &snapshot); err != nil {
				return err
			}
			cursor.LastAppliedSeq = snapshot.CommitSeq
			if err := tx.Clauses(clause.OnConflict{UpdateAll: true}).Create(cursor).Error; err != nil {
				return err
			}
			return tx.Model(&store.SyncSnapshotStage{}).Where("peer_code = ? AND request_code = ?", stage.PeerCode, stage.RequestCode).Update("completed", true).Error
		})
		if err != nil {
			return err
		}
		s.installedSnapshot = &snapshot
		s.snapshotBlocked = false
		if s.onProgress != nil {
			s.onProgress()
		}
		return s.emit(&syncMessage{Type: syncMsgSnapshotAck, SnapshotCode: msg.SnapshotCode, AppliedThrough: snapshot.CommitSeq})
	default:
		return fmt.Errorf("unknown snapshot message")
	}
	return store.DB.Save(&stage).Error
}

func snapshotResourceOwned(ch store.SyncChange, gateway string) bool {
	if ch.Op == store.SyncOpDelete {
		return false
	}
	if ch.EntityType == store.SyncEntityPluginConfig || ch.EntityType == store.SyncEntityNodeConfig {
		return true
	}
	var owner struct {
		OwnerNode string `json:"owner_node"`
		GatewaySN string `json:"gateway_sn"`
	}
	_ = json.Unmarshal([]byte(ch.Payload), &owner)
	return ch.EntityType == store.SyncEntityDevice || owner.OwnerNode == gateway || owner.GatewaySN == gateway
}

// A verified full snapshot may legitimately move a revision backwards after
// restoration of a gateway backup. Ordinary incremental conflict rules must
// never substitute a platform value here.
func installSnapshotResourceTx(tx *gorm.DB, ch store.SyncChange) error {
	tx = tx.Session(&gorm.Session{NewDB: true}).Set(store.SyncApplyingKey, true)
	if ch.EntityType == store.SyncEntityPluginConfig || ch.EntityType == store.SyncEntityNodeConfig {
		var cfg store.ProtectedConfig
		if err := json.Unmarshal([]byte(ch.Payload), &cfg); err != nil {
			return err
		}
		cfg.SyncState = store.SyncStateSynced
		return store.SaveProtectedConfigTx(tx, cfg)
	}
	model := configEntity(ch.EntityType)
	if model == nil {
		return fmt.Errorf("unsupported snapshot entity")
	}
	if ch.Op == store.SyncOpDelete {
		return tx.Unscoped().Where("code = ?", ch.EntityCode).Delete(model).Error
	}
	if err := json.Unmarshal([]byte(ch.Payload), model); err != nil {
		return err
	}
	value := reflect.ValueOf(model).Elem()
	if id := value.FieldByName("ID"); id.IsValid() && id.CanSet() {
		id.SetUint(0)
	}
	for _, field := range []string{"CreatedAt", "UpdatedAt"} {
		if f := value.FieldByName(field); f.IsValid() && f.CanSet() {
			f.Set(reflect.Zero(f.Type()))
		}
	}
	switch row := model.(type) {
	case *store.Device:
		clearConflictState(&row.SyncMeta)
		row.Rev = ch.NewRev
		row.SyncState = store.SyncStateSynced
	case *store.Product:
		clearConflictState(&row.SyncMeta)
		row.Rev = ch.NewRev
		row.SyncState = store.SyncStateSynced
	case *store.ProtocolProfile:
		clearConflictState(&row.SyncMeta)
		row.Rev = ch.NewRev
		row.SyncState = store.SyncStateSynced
	case *store.Rule:
		row.Version = ch.NewRev
		row.SyncState = store.SyncStateSynced
		row.ConflictPayload = ""
		row.ConflictRev = 0
		row.ConflictNode = ""
	case *store.RuleGroup:
		clearConflictState(&row.SyncMeta)
		row.Rev = ch.NewRev
		row.SyncState = store.SyncStateSynced
	}
	return tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "code"}}, UpdateAll: true}).Create(model).Error
}

func (e *platformEngineImpl) installGatewaySnapshot(tx *gorm.DB, snapshot *configSnapshot) error {
	var gateway store.Device
	if err := tx.Where("code = ? AND product_code = ?", snapshot.GatewayCode, defaultGatewayProductCode).Take(&gateway).Error; err != nil {
		return err
	}
	if gateway.TenantID != snapshot.TenantID || gateway.ProjectID != snapshot.ProjectID {
		return fmt.Errorf("snapshot scope mismatch")
	}
	seen := map[string]bool{}
	for _, ch := range snapshot.Resources {
		key := ch.EntityType + ":" + ch.EntityCode
		if seen[key] || ch.OriginNode != snapshot.GatewayCode || ch.OriginInstance != snapshot.InstanceID || ch.LifecycleVersion < 1 || ch.NewRev < 0 {
			return fmt.Errorf("invalid snapshot resource identity")
		}
		seen[key] = true
		allowed, err := e.authorizeGatewayChange(tx, ch)
		if err != nil {
			return err
		}
		if !allowed {
			return fmt.Errorf("unauthorized snapshot resource %s", key)
		}
	}
	// Only explicit gateway-owned mirror rows may be removed by an authoritative
	// absence. Shared definitions and pending platform intents remain untouched.
	for _, kind := range []string{store.SyncEntityDevice, store.SyncEntityProduct, store.SyncEntityDriver, store.SyncEntityRule} {
		ownerColumn := "owner_node"
		if kind == store.SyncEntityRule {
			ownerColumn = "gateway_sn"
		}
		var codes []string
		query := tx.Model(configEntity(kind)).Where(ownerColumn+" = ? AND tenant_id = ? AND project_id = ?", snapshot.GatewayCode, gateway.TenantID, gateway.ProjectID)
		if err := query.Pluck("code", &codes).Error; err != nil {
			return err
		}
		for _, code := range codes {
			if seen[kind+":"+code] || code == snapshot.GatewayCode {
				continue
			}
			if err := tx.Set(store.SyncApplyingKey, true).Unscoped().Where("code = ?", code).Delete(configEntity(kind)).Error; err != nil {
				return err
			}
			if err := tx.Where("entity_type = ? AND code = ?", kind, code).Delete(&store.ResourceLifecycle{}).Error; err != nil {
				return err
			}
		}
	}
	// A deleted mirror has no business row to discover above. Its old lifecycle
	// must also be replaced by authoritative absence after a gateway rebuild.
	// Keep records still represented by a shared/local row or another gateway.
	var deleted []store.ConfigProjection
	if err := tx.Where("gateway_code = ? AND tenant_id = ? AND project_id = ? AND deleted = ?", snapshot.GatewayCode, gateway.TenantID, gateway.ProjectID, true).Find(&deleted).Error; err != nil {
		return err
	}
	for _, prior := range deleted {
		if seen[prior.EntityType+":"+prior.EntityCode] || prior.EntityCode == snapshot.GatewayCode || prior.EntityType == store.SyncEntityRuleGroup {
			continue
		}
		model := configEntity(prior.EntityType)
		if model == nil {
			continue
		}
		var remaining int64
		if err := tx.Unscoped().Model(model).Where("code = ?", prior.EntityCode).Count(&remaining).Error; err != nil {
			return err
		}
		if remaining != 0 {
			continue
		}
		if err := tx.Model(&store.ConfigProjection{}).Where("entity_type = ? AND entity_code = ? AND gateway_code <> ?", prior.EntityType, prior.EntityCode, snapshot.GatewayCode).Count(&remaining).Error; err != nil {
			return err
		}
		if remaining == 0 {
			if err := tx.Where("entity_type = ? AND code = ?", prior.EntityType, prior.EntityCode).Delete(&store.ResourceLifecycle{}).Error; err != nil {
				return err
			}
		}
	}
	if err := tx.Where("gateway_code = ?", snapshot.GatewayCode).Delete(&store.ConfigProjection{}).Error; err != nil {
		return err
	}
	for _, ch := range snapshot.Resources {
		// Gateway root devices are stored flat locally. The platform uses their
		// gateway parent for command routing, just as incremental replay does.
		var err error
		ch, err = normalizeDeviceTopology(cloudSyncNodeID, ch)
		if err != nil {
			return err
		}
		owned := snapshotResourceOwned(ch, snapshot.GatewayCode)
		if ch.Op == store.SyncOpDelete && configEntity(ch.EntityType) != nil {
			ownerColumn := "owner_node"
			if ch.EntityType == store.SyncEntityRule {
				ownerColumn = "gateway_sn"
			}
			if ch.EntityType != store.SyncEntityRuleGroup {
				var count int64
				if err := tx.Model(configEntity(ch.EntityType)).Where("code = ? AND "+ownerColumn+" = ?", ch.EntityCode, snapshot.GatewayCode).Count(&count).Error; err != nil {
					return err
				}
				owned = count > 0
			}
		}
		if owned {
			if err := installSnapshotResourceTx(tx, ch); err != nil {
				return err
			}
			state := store.ResourceLifecycle{EntityType: ch.EntityType, Code: ch.EntityCode, Version: ch.LifecycleVersion, LastRev: ch.NewRev, Deleted: ch.Op == store.SyncOpDelete}
			if err := tx.Clauses(clause.OnConflict{UpdateAll: true}).Create(&state).Error; err != nil {
				return err
			}
		}
		if err := store.SaveConfigProjectionTx(tx, snapshot.GatewayCode, ch); err != nil {
			return err
		}
	}
	if err := tx.Where("gateway_code = ?", snapshot.GatewayCode).Delete(&store.GatewayRuntimeState{}).Error; err != nil {
		return err
	}
	for _, row := range snapshot.Runtime {
		if !seen[row.EntityType+":"+row.EntityCode] || row.LifecycleVersion < 1 || row.Revision < 0 || row.ActiveRevision < 0 || row.ActiveLifecycleVersion < 0 || (row.Status != store.RuntimeApplyActive && row.Status != store.RuntimeApplyPending && row.Status != store.RuntimeApplyFailed) {
			return fmt.Errorf("invalid snapshot runtime resource")
		}
		var projection store.ConfigProjection
		if err := tx.Where("gateway_code = ? AND entity_type = ? AND entity_code = ?", snapshot.GatewayCode, row.EntityType, row.EntityCode).Take(&projection).Error; err != nil {
			return err
		}
		if projection.Revision != row.Revision || projection.LifecycleVersion != row.LifecycleVersion {
			return fmt.Errorf("snapshot runtime version differs from committed resource: %s/%s runtime=%d/%d committed=%d/%d", row.EntityType, row.EntityCode, row.LifecycleVersion, row.Revision, projection.LifecycleVersion, projection.Revision)
		}
		state := store.GatewayRuntimeState{GatewayCode: snapshot.GatewayCode, EntityType: row.EntityType, EntityCode: row.EntityCode, ChangeID: row.ChangeID, Revision: row.Revision, LifecycleVersion: row.LifecycleVersion, ActiveRevision: row.ActiveRevision, ActiveLifecycleVersion: row.ActiveLifecycleVersion, Status: row.Status, LastError: row.LastError, CompensationStatus: row.CompensationStatus, CompensationError: row.CompensationError, Attempts: row.Attempts, UpdatedAt: row.UpdatedAt}
		if err := tx.Create(&state).Error; err != nil {
			return err
		}
	}
	if snapshot.Maintenance != nil {
		if snapshot.Maintenance.GatewayCode != snapshot.GatewayCode {
			return fmt.Errorf("snapshot maintenance scope mismatch")
		}
		if err := tx.Clauses(clause.OnConflict{UpdateAll: true}).Create(snapshot.Maintenance).Error; err != nil {
			return err
		}
	}
	return nil
}
