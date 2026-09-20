package cascade

import (
	"encoding/json"
	"fmt"

	"noyo/core"
	"noyo/core/platform"
	"noyo/core/store"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"go.uber.org/zap"
)

// gatewayPeer is the only peer a gateway ever synchronizes with.
func gatewaySyncPeer() syncPeerRef {
	return syncPeerRef{nodeID: cloudSyncNodeID, kind: "cloud"}
}

// initSync wires the gateway into the synchronization machinery.
func (e *gatewayEngineImpl) initSync() error {
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
	hub.onNodeConfig = e.applyNodeConfig
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
		notify:    hub.notifyChange,
		routeDevice: func(*store.Device) string {
			return cloudSyncNodeID
		},
		routeProduct: func(*store.Product) string {
			return cloudSyncNodeID
		},
	})
	e.recorder = rec
	if err := registerChangeHooks(rec); err != nil {
		return err
	}
	store.SetSyncChangeSink(rec)
	return nil
}

func (e *gatewayEngineImpl) applyInboundChange(ch store.SyncChange) string {
	return applySyncChange(e.gatewayCode, syncApplyHooks{
		beforeApply: e.acceptInboundChange,
	}, e.logger, ch)
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

func (e *gatewayEngineImpl) syncPublish(_ syncPeerRef, msg *syncMessage) error {
	if e.client == nil || !e.client.IsConnected() {
		return fmt.Errorf("gateway mqtt client not connected")
	}
	if e.gatewayCode == "" {
		return fmt.Errorf("gateway routing code is not configured")
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
