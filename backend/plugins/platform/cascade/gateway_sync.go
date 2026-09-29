package cascade

import (
	"encoding/json"
	"errors"
	"fmt"

	"noyo/core"
	"noyo/core/config"
	"noyo/core/platform"
	"noyo/core/store"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// gatewayPeer is the only peer a gateway ever synchronizes with.
func gatewaySyncPeer() syncPeerRef {
	return syncPeerRef{nodeID: cloudSyncNodeID, kind: "cloud"}
}

// initSync wires the gateway into the synchronization machinery.
func (e *gatewayEngineImpl) initSync() error {
	store.EnablePlatformConfigProjection(false)
	if err := store.RegisterConfigWriteGuard("gateway", e.gatewayCode); err != nil {
		return fmt.Errorf("register gateway configuration guard: %w", err)
	}
	instanceID, err := store.NodeInstanceID(e.gatewayCode)
	if err != nil {
		return fmt.Errorf("resolve gateway change-log instance: %w", err)
	}
	hub := newSyncHub(
		e.gatewayCode,
		instanceID,
		e.logger,
		e.syncPublish,
		func(*store.SyncChange) []syncPeerRef { return []syncPeerRef{gatewaySyncPeer()} },
		e.applyInboundChange,
	)
	hub.fallback = gatewaySyncPeer()
	hub.buildSnapshot = func() (*configSnapshot, error) {
		return buildGatewaySnapshot(e.gatewayCode, instanceID, e.config.TenantID, e.config.ProjectID)
	}
	hub.onMaintenance = e.handleMaintenance
	hub.onRuntime = e.handleRuntimeResultAck
	hub.onProgress = func(syncPeerRef) { e.queueMaintenanceSync() }
	hub.onHello = func(syncPeerRef) {
		if err := e.restoreProtectedMediaNetwork(); err != nil {
			e.logger.Warn("Persisted media network configuration awaits runtime provider", zap.Error(err))
		}
		if err := e.publishMaintenanceState(); err != nil {
			e.logger.Debug("Maintenance state awaits reconnect", zap.Error(err))
		}
		e.flushUnreportedRuntime()
	}
	e.syncHub = hub

	rec := newChangeRecorder(changeRecorder{
		nodeID:     e.gatewayCode,
		instanceID: instanceID,
		// Every device on a gateway belongs to that gateway. Stamping the owner
		// here is what lets the platform route the device's later changes back:
		// devices are stored flat, so the hierarchy carries no ownership.
		origin:    gatewayOrigin,
		ownerNode: e.gatewayCode,
		logger:    e.logger,
		notify: func(ch store.SyncChange) {
			hub.notifyChange(ch)
			go e.activateGatewayRuntime(ch.EntityType, ch.EntityCode, ch.ChangeID())
		},
		routeDevice: func(*store.Device) string {
			return cloudSyncNodeID
		},
		routeProduct: func(*store.Product) string {
			return cloudSyncNodeID
		},
		routeScope: func(uint, uint) string { return cloudSyncNodeID },
	})
	e.recorder = rec
	if err := registerChangeHooks(rec); err != nil {
		return err
	}
	store.SetSyncChangeSink(rec)
	store.SetProtectedConfigSink(rec)
	if err := store.DB.Transaction(func(tx *gorm.DB) error {
		var running *config.GlobalConfig
		if e.server != nil {
			running = e.server.Config
		}
		return bootstrapProtectedGatewayConfigsTx(tx, e.gatewayCode, instanceID, running)
	}); err != nil {
		return fmt.Errorf("bootstrap gateway protected configuration log: %w", err)
	}
	if err := bootstrapExistingResources(e.gatewayCode, instanceID); err != nil {
		return fmt.Errorf("bootstrap gateway configuration log: %w", err)
	}
	if err := e.restoreProtectedMediaNetwork(); err != nil {
		e.logger.Warn("Persisted media network configuration awaits runtime provider", zap.Error(err))
	}
	return nil
}

func (e *gatewayEngineImpl) applyInboundChange(ch store.SyncChange) string {
	e.configMutationMu.Lock()
	defer e.configMutationMu.Unlock()
	var runtimeBasePayload string
	var runtimeBaseRevision int64
	return applySyncChange(e.gatewayCode, syncApplyHooks{
		beforeApply: e.acceptInboundChange,
		localNode:   e.gatewayCode,
		localInstance: func() string {
			if e.recorder != nil {
				return e.recorder.instanceID
			}
			return ""
		}(),
		deferApply: func(tx *gorm.DB, ch store.SyncChange) (bool, error) {
			if !inventoryConfigEntity(ch.EntityType) {
				return false, nil
			}
			state, err := store.GetGatewayMaintenanceTx(tx, e.gatewayCode)
			return state != nil && state.Mode != store.GatewayModeNormal, err
		},
		beforeMutationTx: func(tx *gorm.DB, change *store.SyncChange) error {
			var err error
			if change.EntityType == store.SyncEntityPluginConfig || change.EntityType == store.SyncEntityNodeConfig {
				previous, loadErr := store.GetProtectedConfigTx(tx, change.EntityCode)
				if errors.Is(loadErr, gorm.ErrRecordNotFound) {
					return nil
				}
				if loadErr != nil {
					return loadErr
				}
				payload, encodeErr := json.Marshal(previous)
				if encodeErr != nil {
					return encodeErr
				}
				runtimeBasePayload, runtimeBaseRevision = string(payload), previous.Revision
			} else {
				runtimeBasePayload, runtimeBaseRevision, err = resourceSnapshotTx(tx, change.EntityType, change.EntityCode)
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return nil
				}
			}
			return err
		},
		afterApplyTx: func(tx *gorm.DB, ch store.SyncChange, status string) error {
			if status == syncStatusApplied || ((ch.EntityType == store.SyncEntityPluginConfig || ch.EntityType == store.SyncEntityNodeConfig) && status == syncStatusConflict) {
				actual, err := gatewayCommittedChangeTx(tx, ch)
				if err != nil {
					return err
				}
				actual.BasePayload, actual.BaseRev = runtimeBasePayload, runtimeBaseRevision
				return store.QueueRuntimeApplyTx(tx, actual)
			}
			return nil
		},
		afterDependencies: func(committed []store.SyncChange) {
			for _, dependency := range committed {
				e.activateGatewayRuntime(dependency.EntityType, dependency.EntityCode, dependency.ChangeID())
			}
		},
		afterApply: func(applied store.SyncChange, status string) {
			if e.syncHub != nil {
				go func() { _ = e.syncHub.session(gatewaySyncPeer()).pushPending() }()
			}
			if status == syncStatusApplied || ((applied.EntityType == store.SyncEntityPluginConfig || applied.EntityType == store.SyncEntityNodeConfig) && status == syncStatusConflict) {
				e.activateGatewayRuntime(applied.EntityType, applied.EntityCode, applied.ChangeID())
			}
		},
	}, e.logger, ch)
}

func (e *gatewayEngineImpl) activateGatewayRuntime(entityType, entityCode, changeID string) {
	e.runtimeApplyMu.Lock()
	defer e.runtimeApplyMu.Unlock()
	_ = e.activateGatewayRuntimeLocked(entityType, entityCode, changeID, false)
}

// The maintenance coordinator owns runtimeApplyMu while draining committed
// tasks, including work whose asynchronous notification has not run yet.
func (e *gatewayEngineImpl) activateGatewayRuntimeLocked(entityType, entityCode, changeID string, retryFailed bool) error {
	current, err := store.GetRuntimeApply(entityType, entityCode)
	if err != nil {
		e.logger.Warn("Could not load gateway runtime apply state", zap.String("change_id", changeID), zap.Error(err))
		return err
	}
	if current.ChangeID != changeID || current.Status == store.RuntimeApplyActive {
		return nil
	}
	if current.Status == store.RuntimeApplyFailed {
		// Delayed commit notifications are not a second activation attempt.
		// Failed work is paused through maintenance/handover and retried later.
		if !retryFailed {
			return nil
		}
		if e.gatewayCode != "" && inventoryConfigEntity(entityType) {
			state, err := store.GetGatewayMaintenance(e.gatewayCode)
			if err != nil {
				return err
			}
			if state.Mode != store.GatewayModeNormal {
				return nil
			}
		}
	}
	change := store.SyncChange{EntityType: entityType, EntityCode: entityCode, NewRev: current.Revision, Payload: current.Payload, Op: current.Operation}
	applyErr := e.applyGatewayRuntimeChange(change)
	compensationStatus := ""
	var compensationErr error
	var activationErr *core.RuntimeActivationError
	if errors.As(applyErr, &activationErr) {
		compensationStatus, compensationErr = activationErr.CompensationStatus, activationErr.CompensationError
	}
	if applyErr != nil && entityType == store.SyncEntityRule && e.server != nil && e.server.RuleEngine != nil {
		compensationStatus = store.RuntimeCompensated
	}
	if err := store.CompleteRuntimeApplyAttempt(*current, applyErr, compensationStatus, compensationErr); err != nil {
		e.logger.Error("Could not save gateway runtime apply result", zap.String("change_id", changeID), zap.Error(err))
		return err
	} else {
		e.flushUnreportedRuntime()
	}
	if applyErr != nil {
		e.logger.Warn("Synchronized configuration awaits runtime activation", zap.String("change_id", changeID), zap.Error(applyErr))
	}
	return nil
}

func inventoryConfigEntity(entityType string) bool {
	return entityType == store.SyncEntityDevice || entityType == store.SyncEntityProduct || entityType == store.SyncEntityDriver
}

func (e *gatewayEngineImpl) flushUnreportedRuntime() {
	if e.syncHub == nil {
		return
	}
	rows, err := store.ListUnreportedRuntime(100)
	if err != nil {
		e.logger.Warn("Could not list unreported gateway runtime results", zap.Error(err))
		return
	}
	for i := range rows {
		row := rows[i]
		if err := e.syncHub.publish(gatewaySyncPeer(), &syncMessage{
			Type: syncMsgRuntimeResult, NodeID: e.gatewayCode,
			InstanceID: e.syncHub.instanceID, ProtocolVersion: syncProtocolVersion,
			RuntimeResult: &row,
		}); err != nil {
			e.logger.Debug("Gateway runtime result awaits reconnect", zap.String("change_id", row.ChangeID), zap.Error(err))
			return
		}
	}
}

func (e *gatewayEngineImpl) handleRuntimeResultAck(peer syncPeerRef, msg *syncMessage) {
	if peer.nodeID != cloudSyncNodeID || msg.Type != syncMsgRuntimeResultAck || msg.RuntimeChangeID == "" || msg.RuntimeAttempts < 1 {
		return
	}
	cursor, err := store.GetSyncCursor(cloudSyncNodeID)
	if err != nil || cursor.PeerInstance == "" || cursor.PeerInstance != msg.InstanceID {
		return
	}
	if err := store.AckRuntimeApply(msg.RuntimeChangeID, msg.RuntimeAttempts); err != nil {
		e.logger.Warn("Could not acknowledge gateway runtime result", zap.String("change_id", msg.RuntimeChangeID), zap.Error(err))
	}
}

func (e *gatewayEngineImpl) applyGatewayRuntimeChange(ch store.SyncChange) error {
	if ch.EntityType == store.SyncEntityPluginConfig || ch.EntityType == store.SyncEntityNodeConfig {
		return e.applyProtectedGatewayRuntimeChange(ch)
	}
	if ch.EntityType == store.SyncEntityRule {
		if e.server == nil || e.server.RuleEngine == nil {
			return fmt.Errorf("rule engine is unavailable")
		}
		return e.server.RuleEngine.LoadRules()
	}
	if ch.EntityType == store.SyncEntityRuleGroup {
		return nil
	}
	return e.refreshSyncedGatewayRuntime(ch)
}

// Refresh the in-memory registry and running device tasks after a committed
// configuration replay. Failures are persisted and retried independently from
// the durable synchronization ACK.
func (e *gatewayEngineImpl) refreshSyncedGatewayRuntime(ch store.SyncChange) error {
	if e.server == nil || e.server.DeviceManager == nil || e.server.DeviceManager.Registry == nil {
		return fmt.Errorf("device registry is unavailable")
	}
	return e.server.DeviceManager.ApplyRuntimeConfig(ch.EntityType, ch.EntityCode, ch.Payload)
}

// acceptInboundChange rejects a device change that is explicitly owned by a
// different gateway.
//
// The platform falls back to a scope-wide fan-out when a device declares no
// owner. Without this guard such a change would be applied by every gateway in
// the project, silently replicating one site's device onto another. Rejecting is
// safe: the change is acknowledged either way, so a wrong decision here costs
// correctness of a single record rather than stalling the peer's cursor.
func (e *gatewayEngineImpl) acceptInboundChange(ch store.SyncChange) bool {
	if ch.EntityType == store.SyncEntityPluginConfig || ch.EntityType == store.SyncEntityNodeConfig {
		var cfg store.ProtectedConfig
		return json.Unmarshal([]byte(ch.Payload), &cfg) == nil && cfg.Validate() == nil && cfg.GatewaySN == e.gatewayCode && cfg.Code == ch.EntityCode && cfg.TenantID == ch.TenantID && cfg.ProjectID == ch.ProjectID && protectedConfigEntity(cfg) == ch.EntityType
	}
	if ch.EntityType == store.SyncEntityProduct || ch.EntityType == store.SyncEntityDriver {
		if ch.Payload == "" {
			if ch.EntityType == store.SyncEntityProduct {
				local, err := store.GetProduct(ch.EntityCode)
				return err != nil || local.OwnerNode == "" || local.OwnerNode == e.gatewayCode
			}
			local, err := store.GetProtocolProfile(ch.EntityCode)
			return err != nil || local.OwnerNode == "" || local.OwnerNode == e.gatewayCode
		}
		var owner struct {
			OwnerNode string `json:"owner_node"`
		}
		if err := json.Unmarshal([]byte(ch.Payload), &owner); err == nil && owner.OwnerNode != "" && owner.OwnerNode != e.gatewayCode {
			return false
		}
	}
	if ch.EntityType == store.SyncEntityRule {
		if ch.Payload == "" {
			local, err := store.GetRule(ch.EntityCode)
			return err != nil || local.GatewaySN == "" || local.GatewaySN == e.gatewayCode
		}
		var rule store.Rule
		if err := json.Unmarshal([]byte(ch.Payload), &rule); err != nil {
			return true
		}
		return rule.GatewaySN == "" || rule.GatewaySN == e.gatewayCode
	}
	if ch.EntityType != store.SyncEntityDevice {
		return true
	}
	if ch.Payload == "" {
		// A deletion carries no snapshot. Fall back to the local record so a
		// device owned by another gateway is not deleted here by mistake.
		local, err := store.GetDevice(ch.EntityCode)
		if err != nil || local == nil {
			return true
		}
		return ownsDevice(local.OwnerNode, e.gatewayCode)
	}
	var incoming store.Device
	if err := json.Unmarshal([]byte(ch.Payload), &incoming); err != nil {
		// Let the applier report the malformed payload.
		return true
	}
	return ownsDevice(incoming.OwnerNode, e.gatewayCode)
}

func ownsDevice(owner, gatewayCode string) bool {
	return owner == "" || owner == gatewayCode
}

// applyNodeConfig consumes node-level settings pushed by the platform.
//
// The media network block (STUN/TURN) used to arrive embedded in the full
// configuration snapshot. It is a transport setting rather than an entity, so it
// travels as its own message and no longer justifies keeping the snapshot path
// alive.
func (e *gatewayEngineImpl) applyNodeConfig(msg *syncMessage) {
	if msg.MediaNetwork == nil {
		return
	}
	coreServer, ok := e.ctx.GetCoreServer().(*core.Server)
	if !ok || coreServer == nil || coreServer.Manager == nil {
		return
	}
	provider := coreServer.Manager.MediaNetworkProvider()
	if provider == nil {
		return
	}
	settings := msg.MediaNetwork
	provider.SetGatewayMediaNetwork(platform.GatewayMediaNetwork{
		StunURLs:     settings.StunURLs,
		TurnURLs:     settings.TurnURLs,
		TurnUsername: settings.TurnUsername,
		TurnPassword: settings.TurnPassword,
	})
	e.logger.Info("Applied media network configuration from platform",
		zap.String("stun_urls", settings.StunURLs),
		zap.String("turn_urls", settings.TurnURLs))
}

func (e *gatewayEngineImpl) syncPublish(peer syncPeerRef, msg *syncMessage) error {
	if e.client == nil || !e.client.IsConnected() {
		return fmt.Errorf("gateway mqtt client not connected")
	}
	if e.gatewayCode == "" {
		return fmt.Errorf("gateway routing code is not configured")
	}
	if msg.SessionToken == "" && e.syncHub != nil && msg.Type != syncMsgHello {
		msg.SessionToken = e.syncHub.session(peer).deliveryToken
	}
	payload, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	token := e.client.Publish(syncUpTopic(e.gatewayCode), 1, false, payload)
	if !token.WaitTimeout(syncPublishTimeout) {
		return fmt.Errorf("publishing sync message to the platform timed out")
	}
	return token.Error()
}

// handleSyncDown receives handshakes, change batches and acknowledgements from
// the platform.
func (e *gatewayEngineImpl) handleSyncDown(_ mqtt.Client, msg mqtt.Message) {
	if msg.Retained() || e.syncHub == nil {
		return
	}
	e.syncHub.handleInbound(gatewaySyncPeer(), msg.Payload())
}

// announceSyncPosition re-handshakes with the platform.
//
// It replaces the previous "pull the whole configuration and reconcile by
// difference" behaviour. A handshake is strictly safer: the peer replies with
// exactly the changes this gateway has not acknowledged, so nothing is inferred
// from a snapshot's contents and no local device can be removed by omission.
func (e *gatewayEngineImpl) announceSyncPosition() {
	if e.syncHub == nil {
		return
	}
	if e.client == nil || !e.client.IsConnected() {
		return
	}
	if err := e.syncHub.session(gatewaySyncPeer()).sendHello(); err != nil {
		e.logger.Warn("Failed to announce sync position to the platform", zap.Error(err))
	}
}

// syncDiagnostics reports the local propagation state. It is used by the status
// endpoint so an operator can see whether the gateway is caught up.
type syncDiagnostics struct {
	NodeID          string `json:"node_id"`
	LastAckedSeq    int64  `json:"last_acked_seq"`
	LastAppliedSeq  int64  `json:"last_applied_seq"`
	PendingChanges  int64  `json:"pending_changes"`
	ConflictedCount int64  `json:"conflicted_devices"`
}

func (e *gatewayEngineImpl) syncDiagnostics() syncDiagnostics {
	diag := syncDiagnostics{NodeID: e.gatewayCode}
	cursor, err := store.GetSyncCursor(cloudSyncNodeID)
	if err == nil && cursor != nil {
		diag.LastAckedSeq = cursor.LastAckedSeq
		diag.LastAppliedSeq = cursor.LastAppliedSeq
	}
	var pending int64
	if err := store.DB.Model(&store.SyncChange{}).
		Where("seq > ?", diag.LastAckedSeq).Count(&pending).Error; err == nil {
		diag.PendingChanges = pending
	}
	var conflicted int64
	if err := store.DB.Model(&store.Device{}).
		Where("sync_state = ?", store.SyncStateConflict).Count(&conflicted).Error; err == nil {
		diag.ConflictedCount = conflicted
	}
	return diag
}
