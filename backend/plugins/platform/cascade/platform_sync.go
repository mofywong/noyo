package cascade

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"noyo/core"
	"noyo/core/store"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// initSync wires the platform into the synchronization machinery: the change
// recorder that turns local writes into a propagation log, the mutation sink
// that reports deletions, and the hub that owns one session per gateway.
func (e *platformEngineImpl) initSync() error {
	instanceID, err := store.NodeInstanceID(cloudSyncNodeID)
	if err != nil {
		return fmt.Errorf("resolve platform change-log instance: %w", err)
	}
	e.instanceID = instanceID
	hub := newSyncHub(
		cloudSyncNodeID,
		instanceID,
		e.logger,
		e.syncPublish,
		e.syncPeersForChange,
		e.applyInboundChange,
	)
	e.syncHub = hub
	hub.onHello = e.publishNodeConfig

	rec := newChangeRecorder(changeRecorder{
		nodeID:       cloudSyncNodeID,
		instanceID:   instanceID,
		origin:       cloudSyncNodeID,
		logger:       e.logger,
		notify:       hub.notifyChange,
		routeDevice:  e.routeDevice,
		routeProduct: e.routeProduct,
	})
	e.recorder = rec
	if err := registerChangeHooks(rec); err != nil {
		return err
	}
	store.SetSyncChangeSink(rec)
	return nil
}

func (e *platformEngineImpl) applyInboundChange(ch store.SyncChange) string {
	return applySyncChange(cloudSyncNodeID, syncApplyHooks{
		afterApply: func(applied store.SyncChange, status string) {
			if status == syncStatusApplied || status == syncStatusConflict {
				e.refreshSyncedDevice(applied)
				e.relayAppliedChange(applied)
			}
		},
	}, e.logger, ch)
}

// Refresh from committed state, rather than the incoming payload: conflicts
// retain the local record and topology normalization may change its parent.
func (e *platformEngineImpl) refreshSyncedDevice(ch store.SyncChange) {
	if ch.EntityType != store.SyncEntityDevice || e.ctx == nil {
		return
	}
	server, ok := e.ctx.GetCoreServer().(*core.Server)
	if !ok || server == nil || server.DeviceManager == nil || server.DeviceManager.Registry == nil {
		return
	}
	device, err := store.GetDevice(ch.EntityCode)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		server.DeviceManager.Registry.RemoveDevice(ch.EntityCode)
		return
	}
	if err != nil {
		e.logger.Error("Failed to refresh synced device registry", zap.String("device", ch.EntityCode), zap.Error(err))
		return
	}
	server.DeviceManager.Registry.UpdateDevice(device)
}

func (e *platformEngineImpl) syncPublish(peer syncPeerRef, msg *syncMessage) error {
	if e.client == nil || !e.client.IsConnected() {
		return fmt.Errorf("platform mqtt client not connected")
	}
	if peer.gatewayCode == "" {
		return fmt.Errorf("sync peer %q has no routing code", peer.nodeID)
	}
	payload, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	token := e.client.Publish(syncDownTopic(peer.gatewayCode), 1, false, payload)
	if !token.WaitTimeout(syncPublishTimeout) {
		return fmt.Errorf("publishing sync message to %s timed out", peer.gatewayCode)
	}
	return token.Error()
}

// publishNodeConfig pushes node-level settings to a gateway.
//
// It replaced the media network block that used to ride along with the full
// configuration snapshot. Sending it on every handshake keeps gateways correct
// after a restart without giving the snapshot path a reason to exist.
func (e *platformEngineImpl) publishNodeConfig(peer syncPeerRef) {
	media := e.gatewayMediaNetwork()
	if media == nil {
		return
	}
	if err := e.syncPublish(peer, &syncMessage{
		Type:         syncMsgNodeConfig,
		NodeID:       cloudSyncNodeID,
		MediaNetwork: media,
	}); err != nil {
		e.logger.Warn("Failed to publish node configuration to gateway",
			zap.String("gateway", peer.nodeID), zap.Error(err))
	}
}

func (e *platformEngineImpl) gatewayMediaNetwork() *SyncMediaNetwork {
	coreServer, ok := e.ctx.GetCoreServer().(*core.Server)
	if !ok || coreServer == nil || coreServer.Manager == nil {
		return nil
	}
	provider := coreServer.Manager.MediaNetworkProvider()
	if provider == nil {
		return nil
	}
	cfg, found := provider.GetGatewayMediaNetwork()
	if !found {
		return nil
	}
	return &SyncMediaNetwork{
		StunURLs:     cfg.StunURLs,
		TurnURLs:     cfg.TurnURLs,
		TurnUsername: cfg.TurnUsername,
		TurnPassword: cfg.TurnPassword,
	}
}

// syncPeersForChange resolves the gateways a change must reach.
func (e *platformEngineImpl) syncPeersForChange(ch *store.SyncChange) []syncPeerRef {
	if ch == nil {
		return nil
	}
	targets := ch.TargetNodes()
	// Edge-originated changes are addressed to the cloud sentinel. Sending one
	// back to a gateway would restart the loop the replay guard exists to break.
	if len(targets) == 1 && targets[0] == cloudSyncNodeID {
		return nil
	}
	if len(targets) > 0 {
		return syncPeerRefs(targets)
	}
	// Without an explicit address, fan out to the whole scope rather than drop
	// the change. A wrongly addressed change is visible and correctable; a
	// dropped one is silent.
	return syncPeerRefs(e.gatewaysInScope(ch.TenantID, ch.ProjectID))
}

func syncPeerRefs(codes []string) []syncPeerRef {
	out := make([]syncPeerRef, 0, len(codes))
	seen := make(map[string]bool, len(codes))
	for _, code := range codes {
		code = strings.TrimSpace(code)
		if code == "" || seen[code] {
			continue
		}
		seen[code] = true
		out = append(out, syncPeerRef{nodeID: code, gatewayCode: code, kind: "gateway"})
	}
	return out
}

// gatewaysInScope lists the gateway routing codes belonging to a scope.
func (e *platformEngineImpl) gatewaysInScope(tenantID, projectID uint) []string {
	query := store.DB.Model(&store.Device{}).Where("product_code = ?", defaultGatewayProductCode)
	if tenantID > 0 {
		query = query.Where("tenant_id = ?", tenantID)
	}
	if projectID > 0 {
		query = query.Where("project_id = ?", projectID)
	}
	var devices []store.Device
	if err := query.Find(&devices).Error; err != nil {
		e.logger.Warn("Failed to list gateways in scope", zap.Error(err))
		return nil
	}
	codes := make([]string, 0, len(devices))
	for i := range devices {
		if devices[i].Code != "" {
			codes = append(codes, devices[i].Code)
		}
	}
	return codes
}

// routeDevice resolves the gateway that must receive a device's changes.
//
// Devices are stored flat: their parent code is empty relative to the gateway,
// so the ownership stamp carried on the entity is the only reliable address.
// The hierarchy walk remains as a fallback for deployments that do nest them.
func (e *platformEngineImpl) routeDevice(d *store.Device) string {
	if d == nil {
		return ""
	}
	if d.OwnerNode != "" {
		return d.OwnerNode
	}
	if gw := e.findTopLevelGateway(store.DB, d.Code); gw != "" {
		return gw
	}
	// No owner recorded and no hierarchy to follow. On a single-gateway project
	// this is unambiguous; with several gateways the caller fans out to all of
	// them rather than guessing and losing the change.
	scope := e.gatewaysInScope(d.TenantID, d.ProjectID)
	if len(scope) == 1 {
		return scope[0]
	}
	return ""
}

// routeProduct resolves the gateways that must receive a product's changes.
// Products are shared across a scope rather than owned by one gateway.
func (e *platformEngineImpl) routeProduct(p *store.Product) string {
	if p == nil {
		return ""
	}
	return strings.Join(e.gatewaysInScope(p.TenantID, p.ProjectID), ",")
}

// handleSyncUp receives handshakes, change batches and acknowledgements from a
// gateway. The MQTT topic segment is the routing identity and must agree with
// the gateway device code registered on the platform.
func (e *platformEngineImpl) handleSyncUp(_ mqtt.Client, msg mqtt.Message) {
	if msg.Retained() {
		return
	}
	if e.syncHub == nil {
		return
	}
	matched, gatewayCode := parseTopicGwSn(msg.Topic(), "noyo/cascade/gw/%s/sync/up")
	if !matched || gatewayCode == "" {
		return
	}
	e.syncHub.handleInbound(
		syncPeerRef{nodeID: gatewayCode, gatewayCode: gatewayCode, kind: "gateway"},
		msg.Payload(),
	)
}

// relayAppliedChange re-addresses a change that arrived from one gateway to the
// other gateways in scope.
//
// It exists for entities that are shared rather than owned. A device resolves to
// exactly the gateway that owns it, so excluding the origin leaves nothing to
// relay and the call becomes a no-op. A product resolves to every gateway in
// scope, which is how a product created on one gateway becomes visible on the
// others without the platform having to invent a new change format.
func (e *platformEngineImpl) relayAppliedChange(ch store.SyncChange) {
	if e.syncHub == nil {
		return
	}
	targets := e.relayTargets(ch)
	if len(targets) == 0 {
		return
	}
	relay := store.SyncChange{
		EntityType: ch.EntityType,
		EntityCode: ch.EntityCode,
		Op:         ch.Op,
		// The entity was just written at ch.NewRev, so the relayed change offers
		// exactly that revision. Bumping it would make every receiving gateway
		// see a conflict against content it already has.
		BaseRev:        ch.NewRev,
		NewRev:         ch.NewRev,
		Payload:        ch.Payload,
		OriginNode:     cloudSyncNodeID,
		OriginInstance: e.instanceID,
		TargetNode:     strings.Join(targets, ","),
		TenantID:       ch.TenantID,
		ProjectID:      ch.ProjectID,
		CreatedAt:      time.Now().UnixMilli(),
	}
	if _, err := store.AppendSyncChange(&relay); err != nil {
		e.logger.Warn("Failed to record relay change",
			zap.String("entity_type", ch.EntityType),
			zap.String("entity_code", ch.EntityCode),
			zap.Error(err))
		return
	}
	e.syncHub.notifyChange(relay)
}

func (e *platformEngineImpl) relayTargets(ch store.SyncChange) []string {
	var candidates []string
	switch ch.EntityType {
	case store.SyncEntityDevice:
		d, err := store.GetDevice(ch.EntityCode)
		if err != nil || d == nil {
			return nil
		}
		candidates = strings.Split(e.routeDevice(d), ",")
	case store.SyncEntityProduct:
		candidates = e.gatewaysInScope(ch.TenantID, ch.ProjectID)
	default:
		return nil
	}
	out := make([]string, 0, len(candidates))
	for _, code := range candidates {
		code = strings.TrimSpace(code)
		if code == "" || code == ch.OriginNode {
			continue
		}
		out = append(out, code)
	}
	return out
}
