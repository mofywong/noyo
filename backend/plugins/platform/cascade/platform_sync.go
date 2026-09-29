package cascade

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"noyo/core"
	"noyo/core/store"
	"noyo/core/types"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// initSync wires the platform into the synchronization machinery: the change
// recorder that turns local writes into a propagation log, the mutation sink
// that reports deletions, and the hub that owns one session per gateway.
func (e *platformEngineImpl) initSync() error {
	store.EnablePlatformConfigProjection(true)
	if err := store.RegisterConfigWriteGuard("platform", ""); err != nil {
		return fmt.Errorf("register platform configuration guard: %w", err)
	}
	if err := gatewayPluginStateCache.RestoreFromStore(store.DB); err != nil {
		return fmt.Errorf("restore pending gateway plugin configurations: %w", err)
	}
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
	hub.installSnapshot = e.installGatewaySnapshot
	hub.afterSnapshot = func(snapshot *configSnapshot) {
		if e.ctx == nil {
			return
		}
		server, ok := e.ctx.GetCoreServer().(*core.Server)
		if !ok || server == nil {
			return
		}
		if server.DeviceManager != nil && server.DeviceManager.Registry != nil {
			if err := server.DeviceManager.Registry.Reload(); err != nil {
				e.logger.Error("Failed to refresh authoritative snapshot registry", zap.Error(err))
			}
		}
		if server.RuleEngine != nil {
			if err := server.RuleEngine.LoadRules(); err != nil {
				e.logger.Error("Failed to refresh authoritative snapshot rules", zap.Error(err))
			}
		}
	}
	hub.onHello = func(peer syncPeerRef) {
		e.publishNodeConfig(peer)
		go e.reconcilePendingPluginConfig(peer.gatewayCode)
	}
	hub.onMaintenance = e.handleMaintenance
	hub.onRuntime = e.handleGatewayRuntimeResult
	hub.onProgress = e.tryAcknowledgeHandover

	rec := newChangeRecorder(changeRecorder{
		nodeID:          cloudSyncNodeID,
		separateIntents: true,
		instanceID:      instanceID,
		origin:          cloudSyncNodeID,
		logger:          e.logger,
		notify:          hub.notifyChange,
		routeDevice:     e.routeDevice,
		routeProduct:    e.routeProduct,
		routeScope: func(tenantID, projectID uint) string {
			return strings.Join(e.gatewaysInScope(tenantID, projectID), ",")
		},
	})
	e.recorder = rec
	if err := registerChangeHooks(rec); err != nil {
		return err
	}
	store.SetSyncChangeSink(rec)
	store.SetProtectedConfigSink(rec)
	if err := bootstrapExistingResources(cloudSyncNodeID, instanceID); err != nil {
		return fmt.Errorf("bootstrap platform configuration log: %w", err)
	}
	return nil
}

func (e *platformEngineImpl) handleGatewayRuntimeResult(peer syncPeerRef, msg *syncMessage) {
	if msg.Type != syncMsgRuntimeResult || peer.gatewayCode == "" || msg.RuntimeResult == nil || msg.InstanceID == "" {
		return
	}
	result := msg.RuntimeResult
	if (result.Status != store.RuntimeApplyActive && result.Status != store.RuntimeApplyFailed) || result.Attempts < 1 {
		return
	}
	cursor, err := store.GetSyncCursor(peer.nodeID)
	if err != nil || cursor.PeerInstance != msg.InstanceID {
		return
	}
	separator := strings.LastIndexByte(result.ChangeID, ':')
	if separator < 1 {
		return
	}
	seq, err := strconv.ParseInt(result.ChangeID[separator+1:], 10, 64)
	if err != nil || seq < 1 {
		return
	}
	producer := result.ChangeID[:separator]
	authorized := false
	if producer == e.instanceID {
		var change store.SyncChange
		if err := store.DB.Where("seq = ? AND origin_instance = ?", seq, e.instanceID).Take(&change).Error; err != nil {
			return
		}
		if change.OriginNode != cloudSyncNodeID || change.ChangeID() != result.ChangeID || change.EntityType != result.EntityType || change.EntityCode != result.EntityCode || change.LifecycleVersion != result.LifecycleVersion {
			return
		}
		addressed := false
		for _, target := range e.syncDestinationsForChange(&change) {
			if target.nodeID == peer.nodeID {
				addressed = true
				break
			}
		}
		if !addressed {
			return
		}
		authorized = change.NewRev == result.Revision
	} else if producer != msg.InstanceID {
		return
	}
	if !authorized && store.DB.Migrator().HasTable(&store.ConfigProjection{}) {
		var confirmed store.ConfigProjection
		if err := store.DB.Where("gateway_code = ? AND entity_type = ? AND entity_code = ?", peer.gatewayCode, result.EntityType, result.EntityCode).Take(&confirmed).Error; err != nil {
			return
		}
		authorized = confirmed.Revision == result.Revision && confirmed.LifecycleVersion == result.LifecycleVersion
	}
	if !authorized {
		return
	}
	detail := []rune(result.LastError)
	if len(detail) > 1024 {
		detail = detail[:1024]
	}
	state := store.GatewayRuntimeState{
		GatewayCode: peer.gatewayCode, EntityType: result.EntityType, EntityCode: result.EntityCode,
		ChangeID: result.ChangeID, ChangeSeq: seq, Revision: result.Revision, LifecycleVersion: result.LifecycleVersion,
		ActiveRevision: result.ActiveRevision, ActiveLifecycleVersion: result.ActiveLifecycleVersion, CompensationStatus: result.CompensationStatus, CompensationError: result.CompensationError,
		Status: result.Status, LastError: string(detail), Attempts: result.Attempts, UpdatedAt: time.Now().UnixMilli(),
	}
	if err := store.RecordGatewayRuntimeState(state); err != nil {
		e.logger.Warn("Could not save gateway runtime result", zap.String("gateway", peer.gatewayCode), zap.Error(err))
		return
	}
	if err := e.syncPublish(peer, &syncMessage{
		Type: syncMsgRuntimeResultAck, NodeID: cloudSyncNodeID,
		InstanceID: e.instanceID, ProtocolVersion: syncProtocolVersion,
		RuntimeChangeID: result.ChangeID, RuntimeAttempts: result.Attempts,
	}); err != nil {
		e.logger.Debug("Gateway runtime result acknowledgement awaits retry", zap.String("gateway", peer.gatewayCode), zap.Error(err))
	}
}

func (e *platformEngineImpl) applyInboundChange(ch store.SyncChange) string {
	var relay *store.SyncChange
	return applySyncChange(cloudSyncNodeID, syncApplyHooks{
		preferRemote:   ch.OriginNode != cloudSyncNodeID,
		authorizeApply: e.authorizeGatewayChange,
		afterApplyTx: func(tx *gorm.DB, applied store.SyncChange, status string) error {
			if status != syncStatusApplied {
				return nil
			}
			var err error
			if err = recordConfirmedProjectionTx(tx, applied); err != nil {
				return err
			}
			relay, err = e.recordGatewayRelayTx(tx, applied)
			return err
		},
		afterApply: func(applied store.SyncChange, status string) {
			if status == syncStatusApplied || status == syncStatusConflict {
				e.refreshSyncedDevice(applied)
				e.refreshSyncedProduct(applied)
				if status == syncStatusApplied && applied.EntityType == store.SyncEntityRule {
					if server, ok := e.ctx.GetCoreServer().(*core.Server); ok && server.RuleEngine != nil {
						if err := server.RuleEngine.LoadRules(); err != nil {
							e.logger.Error("Failed to load synchronized rules", zap.Error(err))
						}
					}
				}
				if relay != nil && e.syncHub != nil {
					e.syncHub.notifyChange(*relay)
				}
			}
		},
	}, e.logger, ch)
}

func (e *platformEngineImpl) refreshSyncedProduct(ch store.SyncChange) {
	if ch.EntityType != store.SyncEntityProduct || e.ctx == nil {
		return
	}
	server, ok := e.ctx.GetCoreServer().(*core.Server)
	if !ok || server == nil || server.DeviceManager == nil || server.DeviceManager.Registry == nil {
		return
	}
	product, err := store.GetProduct(ch.EntityCode)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		server.DeviceManager.Registry.RemoveProduct(ch.EntityCode)
		return
	}
	if err != nil {
		e.logger.Error("Failed to refresh synced product registry", zap.String("product", ch.EntityCode), zap.Error(err))
		return
	}
	server.DeviceManager.Registry.UpdateProduct(product)
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
	} else if err != nil {
		e.logger.Error("Failed to refresh synced device registry", zap.String("device", ch.EntityCode), zap.Error(err))
		return
	} else {
		server.DeviceManager.Registry.UpdateDevice(device)
	}
	// The journal is committed before this hook. Refresh visible rows only
	// after their confirmed state (including deletion) is available to HTTP.
	if server.DeviceManager.EventBus != nil {
		server.DeviceManager.EventBus.Publish(types.Event{Type: types.EventDeviceListChanged, Timestamp: time.Now().UnixMilli()})
	}
}

func (e *platformEngineImpl) syncPublish(peer syncPeerRef, msg *syncMessage) error {
	if e.client == nil || !e.client.IsConnected() {
		return fmt.Errorf("platform mqtt client not connected")
	}
	if peer.gatewayCode == "" {
		return fmt.Errorf("sync peer %q has no routing code", peer.nodeID)
	}
	if msg.SessionToken == "" && e.syncHub != nil && msg.Type != syncMsgHello {
		msg.SessionToken = e.syncHub.session(peer).deliveryToken
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
	if err := e.recordMediaNetworkConfig(peer, media); err != nil {
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
	// Database recovery invalidates transport receipts, not completed user
	// operations. Replaying historical accepted edits would overwrite the
	// gateway's restored authority. Only unresolved intents are dispatched anew.
	if ch.OriginNode == cloudSyncNodeID && store.DB.Migrator().HasTable(&store.ConfigIntent{}) {
		var intent store.ConfigIntent
		err := store.DB.Where("operation_code = ?", ch.ChangeID()).Take(&intent).Error
		if err == nil && intent.Status != "pending" {
			return nil
		}
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
	}
	return e.syncDestinationsForChange(ch)
}

func (e *platformEngineImpl) syncDestinationsForChange(ch *store.SyncChange) []syncPeerRef {
	targets := ch.TargetNodes()
	// Edge-originated changes are addressed to the cloud sentinel. Sending one
	// back to a gateway would restart the loop the replay guard exists to break.
	if len(targets) == 1 && targets[0] == cloudSyncNodeID {
		return nil
	}
	if len(targets) > 0 {
		return withoutSyncPeer(syncPeerRefs(targets), ch.ExcludedNode)
	}
	// Without an explicit address, fan out to the whole scope rather than drop
	// the change. A wrongly addressed change is visible and correctable; a
	// dropped one is silent.
	return withoutSyncPeer(syncPeerRefs(e.gatewaysInScope(ch.TenantID, ch.ProjectID)), ch.ExcludedNode)
}

func withoutSyncPeer(peers []syncPeerRef, excluded string) []syncPeerRef {
	if excluded == "" {
		return peers
	}
	out := peers[:0]
	for _, peer := range peers {
		if peer.nodeID != excluded {
			out = append(out, peer)
		}
	}
	return out
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
	if d.ProductCode == defaultGatewayProductCode {
		return d.Code
	}
	if d.ParentCode != "" {
		if gw := e.findTopLevelGateway(store.DB, d.ParentCode); gw != "" {
			return gw
		}
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
	if p.OwnerNode != "" {
		return p.OwnerNode
	}
	return ""
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

// recordGatewayRelayTx makes a shared gateway edit and its forwarding outbox
// atomic. If the relay cannot be saved, the inbound change remains unconsumed
// and the gateway retries it rather than silently losing propagation.
func (e *platformEngineImpl) recordGatewayRelayTx(tx *gorm.DB, ch store.SyncChange) (*store.SyncChange, error) {
	if ch.OriginNode == cloudSyncNodeID {
		return nil, nil
	}
	if ch.EntityType != store.SyncEntityProduct && ch.EntityType != store.SyncEntityRuleGroup {
		return nil, nil
	}
	if ch.EntityType == store.SyncEntityProduct {
		var product struct {
			OwnerNode string `json:"owner_node"`
		}
		if ch.Op == store.SyncOpDelete {
			// Gateway-local product instances are owned by the originating site;
			// deleting one cannot affect another gateway.
			return nil, nil
		}
		if err := json.Unmarshal([]byte(ch.Payload), &product); err != nil {
			return nil, err
		}
		if product.OwnerNode != "" {
			return nil, nil
		}
	}
	var gateways []string
	if err := tx.Session(&gorm.Session{NewDB: true}).Model(&store.Device{}).
		Where("tenant_id = ? AND project_id = ? AND product_code = ?", ch.TenantID, ch.ProjectID, defaultGatewayProductCode).
		Pluck("code", &gateways).Error; err != nil {
		return nil, err
	}
	targets := make([]string, 0, len(gateways))
	for _, code := range gateways {
		if code != "" && code != ch.OriginNode {
			targets = append(targets, code)
		}
	}
	relay := store.SyncChange{
		EntityType: ch.EntityType,
		EntityCode: ch.EntityCode,
		Op:         ch.Op,
		// The entity was just written at ch.NewRev, so the relayed change offers
		// exactly that revision. Bumping it would make every receiving gateway
		// see a conflict against content it already has.
		BaseRev:          ch.NewRev,
		NewRev:           ch.NewRev,
		LifecycleVersion: ch.LifecycleVersion,
		Payload:          ch.Payload,
		OriginNode:       cloudSyncNodeID,
		OriginInstance:   e.instanceID,
		AckTargets:       strings.Join(targets, ","),
		ExcludedNode:     ch.OriginNode,
		TenantID:         ch.TenantID,
		ProjectID:        ch.ProjectID,
		CreatedAt:        time.Now().UnixMilli(),
	}
	if _, err := store.AppendSyncChangeTx(tx.Session(&gorm.Session{NewDB: true}), &relay); err != nil {
		return nil, err
	}
	return &relay, nil
}
