package cascade

import (
	"context"
	"errors"
	"fmt"
	"time"

	"noyo/core"
	"noyo/core/store"
	"noyo/core/types"

	"go.uber.org/zap"
)

// maintenanceHandoverInterval is how often a reconciling gateway re-checks its
// own confirmation watermark, re-drives the change drain and re-announces the
// pending state. It is deliberately short: the handover is the only moment when
// a lost publish cannot be repaired by the next local write.
const maintenanceHandoverInterval = 15 * time.Second

// maintenanceHandoverGrace is how long a reconciling gateway waits before
// reporting the identifiers that still need platform confirmation.
const maintenanceHandoverGrace = 3 * time.Minute

// unconfirmedHandoverLimit caps how many change identifiers a stalled handover
// names in its log line.
const unconfirmedHandoverLimit = 20

// Gateway maintenance is a gateway-authored state machine. An MQTT message is
// only a notification; the state survives a broker outage and is announced on
// every handshake.
func (e *gatewayEngineImpl) publishMaintenanceState() error {
	state, err := store.GetGatewayMaintenance(e.gatewayCode)
	if err != nil {
		return err
	}
	return publishMaintenanceTo(e.syncHub, gatewaySyncPeer(), state)
}

// publishMaintenanceTo sends a maintenance message through the session emitter
// rather than publishing the envelope directly.
//
// emit() fills NodeID, InstanceID, ProtocolVersion and SessionToken; a raw
// publish fills only the session token. The receiver validates every
// non-handshake message against the sender generation recorded on its cursor
// (handleInbound) and silently discards a mismatch. A hand-built envelope
// therefore never arrives: the platform never learns that a handover started,
// never acknowledges it, and the gateway waits in "reconciling" forever. Those
// drops are logged now, but the correct fix is to stop bypassing emit.
func publishMaintenanceTo(hub *syncHub, peer syncPeerRef, state *store.GatewayMaintenance) error {
	if hub == nil {
		return fmt.Errorf("sync engine is not running")
	}
	return hub.session(peer).emit(&syncMessage{
		Type:        syncMsgMaintenanceState,
		Maintenance: state,
	})
}

func (e *gatewayEngineImpl) enterMaintenance() (*store.GatewayMaintenance, error) {
	e.configMutationMu.Lock()
	defer e.configMutationMu.Unlock()
	e.runtimeApplyMu.Lock()
	defer e.runtimeApplyMu.Unlock()
	state, err := store.GetGatewayMaintenance(e.gatewayCode)
	if err != nil {
		return nil, err
	}
	if state.Mode == store.GatewayModeMaintenance {
		return state, nil
	}
	// Reopening an unconfirmed handover starts a new maintenance epoch. The
	// retained changes are included in the next exit's handover barrier.
	state, err = e.transitionAtRuntimeBoundary(state.Mode, store.GatewayModeMaintenance)
	if err != nil {
		return nil, err
	}
	e.queueMaintenanceSync()
	return state, nil
}

func (e *gatewayEngineImpl) exitMaintenance() (*store.GatewayMaintenance, error) {
	e.configMutationMu.Lock()
	defer e.configMutationMu.Unlock()
	e.runtimeApplyMu.Lock()
	defer e.runtimeApplyMu.Unlock()
	state, err := store.GetGatewayMaintenance(e.gatewayCode)
	if err != nil {
		return nil, err
	}
	if state.Mode == store.GatewayModeReconciling {
		return state, nil
	}
	if state.Mode != store.GatewayModeMaintenance {
		return nil, store.ErrGatewayModeChanged
	}
	state, err = e.transitionAtRuntimeBoundary(store.GatewayModeMaintenance, store.GatewayModeReconciling)
	if err != nil {
		return nil, err
	}
	e.queueMaintenanceSync()
	return state, nil
}

func (e *gatewayEngineImpl) transitionAtRuntimeBoundary(from, to string) (*store.GatewayMaintenance, error) {
	for {
		rows, err := store.ListPendingRuntime(100)
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			if err := e.activateGatewayRuntimeLocked(row.EntityType, row.EntityCode, row.ChangeID, false); err != nil {
				return nil, err
			}
		}
		if len(rows) > 0 {
			continue
		}
		state, err := store.TransitionGatewayMaintenanceAfterRuntime(e.gatewayCode, from, to)
		// A local write may have committed after the pending query. The mode
		// transaction leaves the gate unchanged so its task can also settle.
		if errors.Is(err, store.ErrRuntimeApplyPending) {
			continue
		}
		return state, err
	}
}

func (e *gatewayEngineImpl) handleMaintenance(peer syncPeerRef, msg *syncMessage) {
	if peer.nodeID != cloudSyncNodeID {
		return
	}
	switch msg.Type {
	case syncMsgMaintenanceRequest:
		var err error
		switch msg.MaintenanceAction {
		case "enter":
			_, err = e.enterMaintenance()
		case "exit":
			_, err = e.exitMaintenance()
		default:
			return
		}
		if err != nil {
			e.logger.Warn("Remote maintenance request rejected", zap.Error(err))
		}
	case syncMsgMaintenanceHandoverAck:
		if msg.Maintenance == nil || msg.Maintenance.GatewayCode != e.gatewayCode {
			return
		}
		e.configMutationMu.Lock()
		defer e.configMutationMu.Unlock()
		e.runtimeApplyMu.Lock()
		defer e.runtimeApplyMu.Unlock()
		current, err := store.GetGatewayMaintenance(e.gatewayCode)
		if err != nil || current.Mode != store.GatewayModeReconciling ||
			current.Epoch != msg.Maintenance.Epoch || current.StateVersion != msg.Maintenance.StateVersion ||
			current.EndSeq != msg.Maintenance.EndSeq {
			return
		}
		if _, err := store.TransitionGatewayMaintenance(e.gatewayCode, store.GatewayModeReconciling, store.GatewayModeNormal); err != nil {
			e.logger.Warn("Maintenance handover could not complete", zap.Error(err))
			return
		}
		e.queueMaintenanceSync()
	}
}

func (e *platformEngineImpl) handleMaintenance(peer syncPeerRef, msg *syncMessage) {
	if msg.Type != syncMsgMaintenanceState || msg.Maintenance == nil ||
		msg.Maintenance.GatewayCode != peer.gatewayCode {
		return
	}
	changed, err := store.ObserveGatewayMaintenance(*msg.Maintenance)
	if err != nil {
		e.logger.Warn("Could not persist gateway maintenance state", zap.String("gateway", peer.gatewayCode), zap.Error(err))
		return
	}
	if changed && e.ctx != nil {
		server, _ := e.ctx.GetCoreServer().(*core.Server)
		if server != nil && server.DeviceManager != nil && server.DeviceManager.EventBus != nil {
			server.DeviceManager.EventBus.Publish(types.Event{Type: types.EventDeviceListChanged, Timestamp: time.Now().UnixMilli()})
		}
	}
	if msg.Maintenance.Mode == store.GatewayModeReconciling {
		e.tryAcknowledgeHandover(peer)
	} else if msg.Maintenance.Mode == store.GatewayModeNormal && e.syncHub != nil {
		// A repeated authoritative normal state repairs a failed first delivery
		// after maintenance. A stale normal message must not bypass a newer
		// maintenance/recovery barrier persisted on the platform.
		state, err := store.GetGatewayMaintenance(peer.gatewayCode)
		if err != nil || state.Mode != store.GatewayModeNormal {
			return
		}
		if err := e.syncHub.session(peer).pushPending(); err != nil {
			e.logger.Debug("Pending configuration awaits gateway reconnect", zap.Error(err))
		}
	}
}

func (e *platformEngineImpl) tryAcknowledgeHandover(peer syncPeerRef) {
	state, err := store.GetGatewayMaintenance(peer.gatewayCode)
	if err != nil || state.Mode != store.GatewayModeReconciling {
		return
	}
	cursor, err := store.GetSyncCursor(peer.nodeID)
	if err != nil || cursor.LastAppliedSeq < state.EndSeq {
		return
	}
	if e.syncHub == nil {
		return
	}
	// emit() carries the sender generation; a hand-built envelope would be
	// dropped by the gateway's inbound guard just as the gateway's own state
	// message used to be dropped here.
	if err := e.syncHub.session(peer).emit(&syncMessage{
		Type:        syncMsgMaintenanceHandoverAck,
		Maintenance: state,
	}); err != nil {
		e.logger.Debug("Maintenance handover acknowledgement awaits reconnect", zap.Error(err))
	}
}

func (e *platformEngineImpl) requestMaintenance(gatewayCode string) error {
	return e.requestMaintenanceAction(gatewayCode, "enter")
}

func (e *platformEngineImpl) requestMaintenanceAction(gatewayCode, action string) error {
	if action != "enter" && action != "exit" {
		return errors.New("action must be enter or exit")
	}
	if gatewayCode == "" {
		return errors.New("gateway code is required")
	}
	if e.syncHub == nil {
		return fmt.Errorf("sync engine is not running")
	}
	peer := syncPeerRef{nodeID: gatewayCode, gatewayCode: gatewayCode, kind: "gateway"}
	return e.syncHub.session(peer).emit(&syncMessage{
		Type:              syncMsgMaintenanceRequest,
		MaintenanceAction: action,
	})
}

func (e *gatewayEngineImpl) maintenanceSyncSignal() chan struct{} {
	e.maintenanceWakeOnce.Do(func() { e.maintenanceWake = make(chan struct{}, 1) })
	return e.maintenanceWake
}

// Coalesce notifications and publish the latest durable state in the worker.
// Neither HTTP responses nor configuration/runtime locks wait for MQTT here.
func (e *gatewayEngineImpl) queueMaintenanceSync() {
	select {
	case e.maintenanceSyncSignal() <- struct{}{}:
	default:
	}
}

// The timer repairs lost messages; transitions and receipts wake it immediately.
func (e *gatewayEngineImpl) maintenanceHandoverLoop(ctx context.Context) {
	ticker := time.NewTicker(maintenanceHandoverInterval)
	defer ticker.Stop()
	wake := e.maintenanceSyncSignal()

	e.resolveMaintenanceHandover()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			e.maintenanceSyncCycle()
		case <-wake:
			state, err := store.GetGatewayMaintenance(e.gatewayCode)
			if err != nil {
				e.logger.Warn("Could not read maintenance state for delivery", zap.Error(err))
				continue
			}
			if state.Mode == store.GatewayModeReconciling {
				e.resolveMaintenanceHandover()
			} else if err := e.publishMaintenanceState(); err != nil {
				e.logger.Info("Maintenance state persisted; delivery awaits reconnect", zap.Error(err))
			}
		}
	}
}

// resolveMaintenanceHandover completes a pending handover from the local
// delivery record, or re-announces it so the platform gets another chance.
func (e *gatewayEngineImpl) maintenanceSyncCycle() {
	e.resolveMaintenanceHandover()
	state, err := store.GetGatewayMaintenance(e.gatewayCode)
	if err != nil || state.Mode == store.GatewayModeReconciling {
		return
	}
	// Entry and the final normal notification can also be lost. Keep the
	// platform's cards and pending outbox moving on a stable MQTT connection.
	if err := e.publishMaintenanceState(); err != nil {
		e.logger.Debug("Maintenance state awaits delivery", zap.Error(err))
	}
	if state.Mode == store.GatewayModeNormal && e.syncHub != nil {
		if err := e.syncHub.session(gatewaySyncPeer()).pushPending(); err != nil {
			e.logger.Debug("Gateway commits await delivery", zap.Error(err))
		}
	}
}

func (e *gatewayEngineImpl) resolveMaintenanceHandover() {
	state, err := store.GetGatewayMaintenance(e.gatewayCode)
	if err != nil || state.Mode != store.GatewayModeReconciling {
		return
	}
	cursor, err := store.GetSyncCursor(cloudSyncNodeID)
	if err != nil {
		e.logger.Warn("Could not read the platform cursor while reconciling", zap.Error(err))
		return
	}
	// LastAckedSeq is the position the platform has confirmed applying. Once it
	// covers the barrier every site-side change the handover protects is
	// durably on the platform, which is the entire purpose of the barrier.
	settled := cursor.LastAckedSeq >= state.EndSeq
	if !settled && cursor.LastSentSeq >= state.EndSeq &&
		time.Since(time.UnixMilli(state.UpdatedAt)) >= maintenanceHandoverGrace {
		// LastSentSeq is persisted before publication and cannot prove receipt.
		// Keep recovering until the platform durably confirms the barrier.
		e.logger.Warn("Maintenance handover is waiting for platform confirmation",
			zap.Int64("end_seq", state.EndSeq), zap.Int64("acked_seq", cursor.LastAckedSeq),
			zap.Int64("sent_seq", cursor.LastSentSeq),
			zap.Strings("unconfirmed", unconfirmedHandoverChanges(cursor.LastAckedSeq, state.EndSeq)))
	}
	if !settled {
		e.retryHandoverDelivery(state, cursor)
		return
	}

	e.configMutationMu.Lock()
	defer e.configMutationMu.Unlock()
	e.runtimeApplyMu.Lock()
	defer e.runtimeApplyMu.Unlock()
	// Re-read under the lock: the acknowledgement path may have completed the
	// handover while this goroutine waited for the mutexes.
	current, err := store.GetGatewayMaintenance(e.gatewayCode)
	if err != nil || current.Mode != store.GatewayModeReconciling ||
		current.Epoch != state.Epoch || current.StateVersion != state.StateVersion {
		return
	}
	if _, err := store.TransitionGatewayMaintenance(e.gatewayCode, store.GatewayModeReconciling, store.GatewayModeNormal); err != nil {
		e.logger.Warn("Maintenance handover could not complete locally", zap.Error(err))
		return
	}
	e.logger.Info("Maintenance handover completed from the local delivery record",
		zap.Int64("end_seq", state.EndSeq), zap.Int64("acked_seq", cursor.LastAckedSeq))
	e.queueMaintenanceSync()
}

// retryHandoverDelivery keeps an unmet barrier moving.
//
// Re-announcing the state is not enough. pushPending persists LastSentSeq before
// it publishes, so a publish that times out leaves the local record claiming the
// change left the gateway while the platform never saw it. Nothing else retries
// in that window: a stable connection never re-handshakes, the drain only runs
// again when a new local change is recorded, and a handover is precisely the
// moment when no further local write follows. The batch then stays unconfirmed
// until the process restarts, which is what stranded a handover whose first
// attempt was lost to a burst of publish timeouts.
func (e *gatewayEngineImpl) retryHandoverDelivery(state *store.GatewayMaintenance, cursor *store.SyncCursor) {
	if e.syncHub == nil {
		return
	}
	session := e.syncHub.session(gatewaySyncPeer())
	// pushPending returns without sending while a snapshot recovery holds the
	// session, so the drain would look healthy while nothing can leave.
	session.cursorMu.Lock()
	blocked := session.snapshotBlocked
	session.cursorMu.Unlock()
	if blocked {
		e.logger.Warn("Maintenance handover is held by snapshot recovery",
			zap.Int64("end_seq", state.EndSeq), zap.Int64("acked_seq", cursor.LastAckedSeq))
	}
	// Re-announce so a transition message the platform never persisted gets
	// another chance to start its acknowledgement path.
	if err := e.publishMaintenanceState(); err != nil {
		e.logger.Warn("Maintenance handover could not re-announce its state", zap.Error(err))
	}
	if err := session.pushPending(); err != nil {
		e.logger.Warn("Maintenance handover could not re-drive the change drain",
			zap.Int64("end_seq", state.EndSeq), zap.Int64("acked_seq", cursor.LastAckedSeq),
			zap.Error(err))
	}
}

// unconfirmedHandoverChanges names the changes the platform has not confirmed,
// so a stalled handover points at the entries and not only at the watermark.
func unconfirmedHandoverChanges(afterSeq, endSeq int64) []string {
	rows, err := store.ListSyncChangesAfter(afterSeq, unconfirmedHandoverLimit)
	if err != nil {
		return nil
	}
	ids := make([]string, 0, len(rows))
	for i := range rows {
		if rows[i].Seq > endSeq {
			break
		}
		ids = append(ids, rows[i].ChangeID())
	}
	return ids
}
