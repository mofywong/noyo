package cascade

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"time"

	"noyo/core/config"
	"noyo/core/store"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func protectedConfigEntity(value store.ProtectedConfig) string {
	if value.Kind == "plugin" {
		return store.SyncEntityPluginConfig
	}
	return store.SyncEntityNodeConfig
}

func (r *changeRecorder) ProtectedConfigCommitted(code string) {
	if r.notify == nil {
		return
	}
	var change store.SyncChange
	if err := store.DB.Where("origin_instance = ? AND entity_code = ? AND entity_type IN ?", r.instanceID, code, []string{store.SyncEntityPluginConfig, store.SyncEntityNodeConfig}).Order("seq desc").Take(&change).Error; err == nil {
		r.notify(change)
	}
}

func protectedScopeTx(tx *gorm.DB, gateway string) (uint, uint, error) {
	var device store.Device
	err := tx.Where("code = ? AND product_code = ?", gateway, defaultGatewayProductCode).Take(&device).Error
	if err == nil {
		return device.TenantID, device.ProjectID, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, 0, err
	}
	// The edge can edit while disconnected before its gateway root is mirrored.
	var setup store.SystemConfig
	if err := tx.Where("key = ?", "setup_state").Take(&setup).Error; err != nil {
		return 0, 0, err
	}
	var scope store.SetupState
	if err := json.Unmarshal([]byte(setup.Value), &scope); err != nil {
		return 0, 0, err
	}
	if !scope.Initialized || scope.TenantID == 0 || scope.ProjectID == 0 {
		return 0, 0, fmt.Errorf("gateway scope is not initialized")
	}
	return scope.TenantID, scope.ProjectID, nil
}

func (r *changeRecorder) RecordProtectedIntentTx(tx *gorm.DB, desired store.ProtectedConfig, base *store.ProtectedConfig) error {
	if err := desired.Validate(); err != nil {
		return err
	}
	local := r.nodeID != cloudSyncNodeID
	if local && desired.GatewaySN != r.nodeID {
		return fmt.Errorf("protected configuration belongs to another gateway")
	}
	previous, err := store.GetProtectedConfigTx(tx, desired.Code)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	latestRev := int64(0)
	if previous != nil {
		latestRev = previous.Revision
	}
	if base == nil {
		base = previous
	}
	baseRev := int64(0)
	if base != nil {
		baseRev = base.Revision
	}
	if baseRev > latestRev {
		latestRev = baseRev
	}
	if !local && r.separateIntents {
		pending, err := latestPendingProtectedIntentTx(tx, desired.Code, desired.GatewaySN)
		if err != nil {
			return err
		}
		if pending != nil && pending.Revision > latestRev {
			latestRev = pending.Revision
		}
	}
	if desired.Revision <= latestRev {
		desired.Revision = latestRev + 1
	}
	if desired.Revision == 0 {
		desired.Revision = 1
	}
	desired.SyncState = store.SyncStatePending
	data, err := json.Marshal(desired)
	if err != nil {
		return err
	}
	baseData := ""
	if base != nil {
		encoded, err := json.Marshal(base)
		if err != nil {
			return err
		}
		baseData = string(encoded)
	}
	target := desired.GatewaySN
	if local {
		target = cloudSyncNodeID
	}
	change := store.SyncChange{EntityType: protectedConfigEntity(desired), EntityCode: desired.Code,
		Op: store.SyncOpUpsert, BaseRev: baseRev, NewRev: desired.Revision, LifecycleVersion: 1,
		Payload: string(data), BasePayload: baseData, OriginNode: r.nodeID, OriginInstance: r.instanceID,
		TargetNode: target, TenantID: desired.TenantID, ProjectID: desired.ProjectID, CreatedAt: time.Now().UnixMilli()}
	if local || !r.separateIntents {
		if err := store.SaveProtectedConfigTx(tx, desired); err != nil {
			return err
		}
	}
	if _, err := store.AppendSyncChangeTx(tx, &change); err != nil {
		return err
	}
	if local {
		return store.QueueRuntimeApplyTx(tx, change)
	}
	if r.separateIntents {
		return store.SaveConfigIntentTx(tx, change)
	}
	return nil
}

func latestPendingProtectedIntentTx(tx *gorm.DB, code, gateway string) (*store.ProtectedConfig, error) {
	var rows []store.ConfigIntent
	if err := tx.Session(&gorm.Session{NewDB: true}).Where("entity_code = ? AND gateway_code = ? AND status = ?", code, gateway, "pending").Find(&rows).Error; err != nil {
		return nil, err
	}
	var latest *store.ProtectedConfig
	for _, row := range rows {
		var value store.ProtectedConfig
		if err := json.Unmarshal([]byte(row.Payload), &value); err != nil {
			return nil, err
		}
		if latest == nil || value.Revision > latest.Revision {
			copy := value
			latest = &copy
		}
	}
	return latest, nil
}

func (r *changeRecorder) RecordLocalPluginTx(tx *gorm.DB, model store.PluginModel) error {
	if r.nodeID == cloudSyncNodeID {
		return nil
	} // platform-owned plugin settings are local, not gateway intent
	tenant, project := model.TenantID, model.ProjectID
	if tenant == 0 || project == 0 {
		var err error
		tenant, project, err = protectedScopeTx(tx, r.nodeID)
		if err != nil {
			return err
		}
	}
	var fields map[string]interface{}
	if err := json.Unmarshal([]byte(model.Config), &fields); err != nil {
		return err
	}
	if fields == nil {
		fields = make(map[string]interface{})
	}
	fields["enabled"] = model.Enabled
	return r.RecordProtectedIntentTx(tx, store.ProtectedConfig{Code: store.ProtectedConfigCode(r.nodeID, "plugin", model.Name),
		GatewaySN: r.nodeID, PluginName: model.Name, Kind: "plugin", TenantID: tenant, ProjectID: project, Revision: model.Revision, Config: fields}, nil)
}

func (r *changeRecorder) RecordLocalGlobalConfigTx(tx *gorm.DB, cfg *config.GlobalConfig) error {
	if r.nodeID == cloudSyncNodeID {
		return nil
	}
	tenant, project, err := protectedScopeTx(tx, r.nodeID)
	if err != nil {
		return err
	}
	fields, err := systemConfigFields(cfg)
	if err != nil {
		return err
	}
	return r.RecordProtectedIntentTx(tx, store.ProtectedConfig{Code: store.ProtectedConfigCode(r.nodeID, "system", ""), GatewaySN: r.nodeID,
		Kind: "system", TenantID: tenant, ProjectID: project, Config: fields}, nil)
}

func systemConfigFields(cfg *config.GlobalConfig) (map[string]interface{}, error) {
	mediaPolicy := cfg.AlarmMedia
	mediaPolicy.Normalize()
	data, err := json.Marshal(struct {
		Server     config.ServerConfig     `json:"server"`
		TSDB       config.TSDBConfig       `json:"tsdb"`
		Log        config.LogConfig        `json:"log"`
		AlarmMedia config.AlarmMediaConfig `json:"alarm_media"`
	}{cfg.Server, cfg.TSDB, cfg.Log, mediaPolicy})
	if err != nil {
		return nil, err
	}
	var result map[string]interface{}
	err = json.Unmarshal(data, &result)
	return result, err
}

// Apply intent and bookkeeping atomically; execution is queued by the gateway
// replay hook. On a divergent version a three-way field comparison keeps site
// values while accepting independent platform fields.
func applyProtectedConfigChangeTx(tx *gorm.DB, ch store.SyncChange) (string, error) {
	var incoming store.ProtectedConfig
	if err := json.Unmarshal([]byte(ch.Payload), &incoming); err != nil {
		return "", err
	}
	if err := incoming.Validate(); err != nil {
		return "", err
	}
	if incoming.Code != ch.EntityCode || incoming.Revision != ch.NewRev || incoming.TenantID != ch.TenantID || incoming.ProjectID != ch.ProjectID || protectedConfigEntity(incoming) != ch.EntityType || ch.Op != store.SyncOpUpsert {
		return "", fmt.Errorf("protected configuration metadata mismatch")
	}
	local, err := store.GetProtectedConfigTx(tx, incoming.Code)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return "", err
	}
	if incoming.Kind == "plugin" && ch.OriginNode == cloudSyncNodeID {
		var model store.PluginModel
		err := tx.Where("name = ? AND tenant_id = ? AND project_id = ?", incoming.PluginName, incoming.TenantID, incoming.ProjectID).Take(&model).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return "", err
		}
		// Local plugin edits do not update the protected mirror. Use the newer
		// gateway model so an old snapshot cannot hide a local parameter change.
		if err == nil && (local == nil || model.Revision >= local.Revision) {
			baseline := incoming
			if local != nil {
				baseline = *local
			}
			baseline.Revision = model.Revision
			baseline.Config = nil
			if err = json.Unmarshal([]byte(model.Config), &baseline.Config); err != nil {
				return "", err
			}
			if baseline.Config == nil {
				baseline.Config = make(map[string]interface{})
			}
			baseline.Config["enabled"] = model.Enabled
			local = &baseline
		}
	}
	if local != nil && (local.TenantID != incoming.TenantID || local.ProjectID != incoming.ProjectID) {
		return "", fmt.Errorf("protected configuration scope changed")
	}
	conflict := false
	canonicalNeeded := false
	if local != nil && incoming.Kind == "plugin" && ch.OriginNode == cloudSyncNodeID && local.Revision == ch.BaseRev {
		// Remote schemas omit credentials. An omitted field keeps its committed
		// value; an explicitly supplied empty value still clears the field.
		merged := cloneConfigMap(local.Config)
		for key, desired := range incoming.Config {
			merged[key] = desired
		}
		incoming.Config = merged
	}
	if local != nil && ch.OriginNode == cloudSyncNodeID && local.Revision != ch.BaseRev {
		var base store.ProtectedConfig
		if ch.BasePayload != "" {
			if err := json.Unmarshal([]byte(ch.BasePayload), &base); err != nil {
				return "", err
			}
		}
		merged := cloneConfigMap(local.Config)
		for key, desired := range incoming.Config {
			prior, known := base.Config[key]
			current, present := local.Config[key]
			if reflect.DeepEqual(current, desired) && present {
				continue
			}
			if known && reflect.DeepEqual(desired, prior) {
				continue
			}
			if known && present && reflect.DeepEqual(current, prior) {
				merged[key] = desired
			} else {
				conflict = true
			}
		}
		incoming.Config = merged
		if incoming.Revision <= local.Revision {
			incoming.Revision = local.Revision + 1
		}
		canonicalNeeded = true
	}
	if local != nil && ch.OriginNode != cloudSyncNodeID && incoming.Revision < local.Revision {
		return syncStatusSkipped, nil
	}
	incoming.SyncState = store.SyncStateSynced
	if conflict {
		incoming.SyncState = store.SyncStateConflict
	}
	if err := store.SaveProtectedConfigTx(tx, incoming); err != nil {
		return "", err
	}
	// The platform keeps an isolated mirror; plugin/global models there are its
	// own runtime. Only the gateway persists settings used by its next restart.
	if ch.OriginNode == cloudSyncNodeID {
		if err := persistProtectedGatewayModelsTx(tx, incoming); err != nil {
			return "", err
		}
		if canonicalNeeded {
			data, err := json.Marshal(incoming)
			if err != nil {
				return "", err
			}
			if err = appendGatewayCanonicalTx(tx, ch, string(data), ch.NewRev, incoming.Revision); err != nil {
				return "", err
			}
		}
	}
	if conflict {
		return syncStatusConflict, nil
	}
	return syncStatusApplied, nil
}

func effectiveProtectedRuntimeChangeTx(tx *gorm.DB, ch store.SyncChange) (store.SyncChange, error) {
	if ch.EntityType != store.SyncEntityPluginConfig && ch.EntityType != store.SyncEntityNodeConfig {
		return ch, nil
	}
	value, err := store.GetProtectedConfigTx(tx, ch.EntityCode)
	if err != nil {
		return ch, err
	}
	data, err := json.Marshal(value)
	if err != nil {
		return ch, err
	}
	ch.NewRev, ch.Payload = value.Revision, string(data)
	return ch, nil
}

func persistProtectedGatewayModelsTx(tx *gorm.DB, value store.ProtectedConfig) error {
	tx = tx.Session(&gorm.Session{NewDB: true})
	if value.Kind == "plugin" {
		fields := cloneConfigMap(value.Config)
		enabled, _ := fields["enabled"].(bool)
		delete(fields, "enabled")
		data, err := json.Marshal(fields)
		if err != nil {
			return err
		}
		return tx.Set(store.SyncApplyingKey, true).Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "name"}, {Name: "tenant_id"}, {Name: "project_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"config", "enabled", "revision", "updated_at"}),
		}).Create(&store.PluginModel{Name: value.PluginName, TenantID: value.TenantID, ProjectID: value.ProjectID, Enabled: enabled, Config: string(data), Revision: value.Revision}).Error
	}
	if value.Kind == "system" {
		cfg := *config.DefaultConfig()
		var previous store.SystemConfig
		err := tx.Where("key = ?", "global_config").Take(&previous).Error
		if err == nil {
			if err := json.Unmarshal([]byte(previous.Value), &cfg); err != nil {
				return err
			}
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		data, err := json.Marshal(value.Config)
		if err != nil {
			return err
		}
		if err = json.Unmarshal(data, &cfg); err != nil {
			return err
		}
		if err := cfg.AlarmMedia.Validate(); err != nil {
			return err
		}
		return store.SaveGlobalConfigTx(tx.Set(store.SyncApplyingKey, true), &cfg)
	}
	return nil
}

func recordPluginCacheIntentTx(tx *gorm.DB, item *remotePluginCacheItem) error {
	if !store.HasProtectedConfigSink() {
		return nil
	}
	tenant, project, err := protectedScopeTx(tx, item.GatewaySN)
	if err != nil {
		return err
	}
	baseFields := make(map[string]interface{})
	if item.SummarySnapshot != nil {
		baseFields["enabled"] = item.SummarySnapshot.Status == "running"
		if item.SummarySnapshot.Schema != nil {
			for _, field := range item.SummarySnapshot.Schema.Fields {
				if !sensitivePluginField(field.Name) {
					baseFields[field.Name] = field.Value
				}
			}
		}
	}
	base := store.ProtectedConfig{Code: store.ProtectedConfigCode(item.GatewaySN, "plugin", item.PluginName), GatewaySN: item.GatewaySN,
		PluginName: item.PluginName, Kind: "plugin", TenantID: tenant, ProjectID: project, Revision: item.BaseVersion, Config: baseFields}
	fields := cloneConfigMap(baseFields)
	for key, value := range item.DesiredConfig {
		fields[key] = value
	}
	fields["enabled"] = item.DesiredEnabled
	desired := base
	desired.Config = fields
	desired.Revision = base.Revision + 1
	return store.RecordProtectedIntentTx(tx, desired, &base)
}

func (e *platformEngineImpl) recordMediaNetworkConfig(peer syncPeerRef, settings *SyncMediaNetwork) error {
	if settings == nil {
		return nil
	}
	data, err := json.Marshal(settings)
	if err != nil {
		return err
	}
	var fields map[string]interface{}
	if err = json.Unmarshal(data, &fields); err != nil {
		return err
	}
	var recorded store.ProtectedConfig
	err = store.DB.Transaction(func(tx *gorm.DB) error {
		tenant, project, err := protectedScopeTx(tx, peer.gatewayCode)
		if err != nil {
			return err
		}
		recorded = store.ProtectedConfig{Code: store.ProtectedConfigCode(peer.gatewayCode, "media_network", ""), GatewaySN: peer.gatewayCode, Kind: "media_network", TenantID: tenant, ProjectID: project, Config: fields}
		previous, err := store.GetProtectedConfigTx(tx, recorded.Code)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		var pending *store.ProtectedConfig
		if e.recorder != nil && e.recorder.separateIntents {
			pending, err = latestPendingProtectedIntentTx(tx, recorded.Code, recorded.GatewaySN)
			if err != nil {
				return err
			}
			if pending != nil && reflect.DeepEqual(pending.Config, fields) {
				return nil
			}
		}
		if pending == nil && previous != nil && reflect.DeepEqual(previous.Config, fields) {
			return nil
		}
		return store.RecordProtectedIntentTx(tx, recorded, previous)
	})
	return err
}

func (e *platformEngineImpl) queueRemoteSystemConfig(gateway string, params map[string]interface{}) (*store.ProtectedConfig, error) {
	if !store.HasProtectedConfigSink() {
		return nil, fmt.Errorf("configuration synchronization is unavailable")
	}
	var request remoteSystemConfigSetRequest
	request.Config = *config.DefaultConfig()
	if err := scanRemotePluginParams(params, &request); err != nil {
		return nil, err
	}
	var optional struct {
		Config struct {
			AlarmMedia *config.AlarmMediaConfig `json:"alarm_media"`
		} `json:"config"`
	}
	if err := scanRemotePluginParams(params, &optional); err != nil {
		return nil, err
	}
	if request.Config.Server.Port <= 0 || request.Config.Server.Port > 65535 {
		return nil, fmt.Errorf("invalid HTTP port")
	}
	if err := request.Config.AlarmMedia.Validate(); err != nil {
		return nil, err
	}
	fields, err := systemConfigFields(&request.Config)
	if err != nil {
		return nil, err
	}
	if optional.Config.AlarmMedia == nil {
		delete(fields, "alarm_media")
	}
	code := store.ProtectedConfigCode(gateway, "system", "")
	var recorded *store.ProtectedConfig
	err = store.DB.Transaction(func(tx *gorm.DB) error {
		tenant, project, err := protectedScopeTx(tx, gateway)
		if err != nil {
			return err
		}
		previous, err := store.GetProtectedConfigTx(tx, code)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		desired := store.ProtectedConfig{Code: code, GatewaySN: gateway, Kind: "system", TenantID: tenant, ProjectID: project, Config: fields}
		if err := store.RecordProtectedIntentTx(tx, desired, previous); err != nil {
			return err
		}
		if e.recorder != nil && e.recorder.separateIntents {
			recorded, err = latestPendingProtectedIntentTx(tx, code, gateway)
		} else {
			recorded, err = store.GetProtectedConfigTx(tx, code)
		}
		return err
	})
	if err != nil {
		return nil, err
	}
	store.NotifyProtectedConfigCommitted(code)
	return recorded, nil
}
