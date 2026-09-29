package cascade

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"noyo/core/store"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// syncSession drives synchronization with exactly one peer.
//
// The platform holds one session per gateway; the gateway holds a single
// session for the cloud. Both roles share this implementation because the
// protocol is symmetric: the only differences are which MQTT topic carries the
// traffic and which changes are addressed to the peer.
type syncSession struct {
	nodeID  string
	peer    syncPeerRef
	logger  *zap.Logger
	publish func(peer syncPeerRef, msg *syncMessage) error
	// instanceID identifies this node's change log and is advertised to the peer
	// so it can tell a rebuilt log from a continuing one.
	instanceID string
	// addressed reports whether a change must be delivered to this peer.
	addressed func(ch *store.SyncChange) bool
	apply     func(ch store.SyncChange) string
	// onHello runs after a peer handshake completes. The platform uses it to
	// push node-level settings that the peer needs on every fresh connection.
	onHello func()
	// onProgress runs after a peer batch or receipt and its cursor are durably saved.
	onProgress func()

	mu sync.Mutex
	// cursorMu serializes peer cursor changes with batch publication. MQTT can
	// invoke callbacks concurrently even when changes belong to one stream.
	cursorMu          sync.Mutex
	pushing           bool
	pendingMore       bool
	helloSent         bool
	helloCode         string
	peerHelloCode     string
	deliveryToken     string
	snapshotBlocked   bool
	snapshotSentCode  string
	snapshotSentSeq   int64
	snapshotPayload   *configSnapshot
	buildSnapshot     func() (*configSnapshot, error)
	installSnapshot   func(*gorm.DB, *configSnapshot) error
	afterSnapshot     func(*configSnapshot)
	installedSnapshot *configSnapshot
}

// resetForReconnect clears per-connection state. Both sides re-handshake after
// every reconnect so a gap that opened while the link was down is always
// discovered by comparing cursors, never by guessing.
func (s *syncSession) resetForReconnect() {
	s.mu.Lock()
	s.helloSent = false
	s.helloCode = ""
	s.deliveryToken = ""
	s.snapshotPayload = nil
	s.snapshotSentCode = ""
	s.snapshotBlocked = s.buildSnapshot != nil || s.installSnapshot != nil
	s.mu.Unlock()
}

func (s *syncSession) shouldAnnounce() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.helloSent {
		return false
	}
	s.helloSent = true
	return true
}

// sendHello announces this node's position in the peer's stream.
func (s *syncSession) sendHello() error {
	oldest, err := store.MinSyncChangeSeq()
	if err != nil {
		return err
	}
	cursor, err := store.GetSyncCursor(s.peer.nodeID)
	if err != nil {
		return err
	}
	s.mu.Lock()
	if s.installSnapshot != nil && s.helloCode == "" {
		s.helloCode = snapshotCode()
	}
	if s.buildSnapshot != nil && s.deliveryToken == "" {
		s.deliveryToken = snapshotCode()
		s.snapshotBlocked = true
	}
	s.helloSent = true
	s.mu.Unlock()
	err = s.emit(&syncMessage{
		Type:            syncMsgHello,
		NodeID:          s.nodeID,
		InstanceID:      s.instanceID,
		ProtocolVersion: syncProtocolVersion,
		PeerLastApplied: cursor.LastAppliedSeq,
		MyOldestSeq:     oldest,
		HelloCode:       s.helloCode,
	})
	if err != nil {
		s.mu.Lock()
		s.helloSent = false
		s.mu.Unlock()
	}
	return err
}

// announceHello sends a handshake only when this connection has not sent one.
func (s *syncSession) announceHello() error {
	if !s.shouldAnnounce() {
		return nil
	}
	if err := s.sendHello(); err != nil {
		s.mu.Lock()
		s.helloSent = false
		s.mu.Unlock()
		return err
	}
	return nil
}

func (s *syncSession) handle(msg *syncMessage) error {
	if msg == nil || msg.Type == "" {
		return nil
	}
	if msg.ProtocolVersion != syncProtocolVersion {
		return fmt.Errorf("unsupported cascade sync protocol version %d", msg.ProtocolVersion)
	}
	if (msg.Type == syncMsgHello || msg.Type == syncMsgChanges) && msg.InstanceID == "" {
		return fmt.Errorf("sync %s is missing sender instance", msg.Type)
	}
	if msg.NodeID != "" && msg.NodeID != s.peer.nodeID {
		return fmt.Errorf("sync message sender %q does not match routed peer %q", msg.NodeID, s.peer.nodeID)
	}

	if msg.Type != syncMsgHello && (s.buildSnapshot != nil || s.installSnapshot != nil) && (s.deliveryToken == "" || msg.SessionToken != s.deliveryToken) {
		return fmt.Errorf("stale sync delivery session")
	}
	s.cursorMu.Lock()
	cursor, err := store.GetSyncCursor(s.peer.nodeID)
	if err != nil {
		s.cursorMu.Unlock()
		return err
	}
	if cursor.PeerKind == "" {
		cursor.PeerKind = s.peer.kind
	}
	cursor.LastOnlineAt = time.Now().UnixMilli()

	// A peer that rebuilt its database restarts its sequence numbers. The
	// position recorded here then refers to a log that no longer exists, so the
	// only safe reading is that the peer's stream has not been seen at all.
	peerRebuilt := msg.Type == syncMsgHello && msg.InstanceID != "" && cursor.PeerInstance != msg.InstanceID
	if msg.Type != syncMsgHello && msg.InstanceID != "" && cursor.PeerInstance != msg.InstanceID {
		s.cursorMu.Unlock()
		return fmt.Errorf("unannounced sync generation")
	}
	if peerRebuilt {
		if cursor.PeerInstance != "" {
			s.logger.Warn("Peer change log was rebuilt; restarting its stream from the beginning",
				zap.String("peer", s.peer.nodeID),
				zap.String("previous_instance", cursor.PeerInstance),
				zap.String("instance", msg.InstanceID),
				zap.Int64("discarded_position", cursor.LastAppliedSeq))
		}
		cursor.PeerInstance = msg.InstanceID
		cursor.LastAppliedSeq = 0
		// The peer also lost its acknowledgement of our log. Retain and replay
		// the complete local stream before treating any resource as synchronized.
		cursor.LastAckedSeq = 0
		cursor.LastSentSeq = 0
		if err := store.ResetSyncPeerInstance(cursor); err != nil {
			s.cursorMu.Unlock()
			return err
		}
		// The rebuilt peer has no record of our generation even when our MQTT
		// connection stayed online. Re-arm one reply before any recovery traffic.
		s.mu.Lock()
		s.helloSent = false
		s.mu.Unlock()
	}

	var handleErr error
	pushAfter := msg.Type == syncMsgHello
	ackAdvanced := false
	switch msg.Type {
	case syncMsgHello:
		recoverProjection := s.installSnapshot != nil && (peerRebuilt || s.deliveryToken != msg.SessionToken)
		if s.installSnapshot != nil {
			s.deliveryToken = msg.SessionToken
			if recoverProjection {
				s.snapshotBlocked = true
			}
		}
		if s.buildSnapshot != nil && msg.SessionToken == "" {
			// A platform-only reconnect must invalidate the old delivery token
			// before any queued write can run. A duplicate challenge reuses its
			// token, including after recovery, rather than starting another cycle.
			s.mu.Lock()
			fresh := msg.HelloCode != "" && msg.HelloCode != s.peerHelloCode
			if fresh || (msg.HelloCode == "" && !s.snapshotBlocked) {
				s.deliveryToken = snapshotCode()
				s.snapshotBlocked = true
				s.snapshotPayload = nil
				s.snapshotSentCode = ""
				s.snapshotSentSeq = 0
			}
			s.peerHelloCode = msg.HelloCode
			s.helloSent = false
			s.mu.Unlock()
		} else if s.buildSnapshot != nil && msg.HelloCode != "" {
			s.peerHelloCode = msg.HelloCode
		}
		handleErr = s.handleHello(cursor, msg)
		// A new connection always validates the authoritative projection. This
		// also detects backup rollback with an unchanged persisted instance ID.
		if handleErr == nil && recoverProjection {
			handleErr = s.requestSnapshot(cursor)
		}
	case syncMsgChanges:
		handleErr = s.handleChanges(cursor, msg)
	case syncMsgAck:
		previousAck := cursor.LastAckedSeq
		handleErr = s.handleAck(cursor, msg)
		ackAdvanced = handleErr == nil && cursor.LastAckedSeq > previousAck
		pushAfter = ackAdvanced
		for _, result := range msg.Results {
			if result.Status == syncStatusRejected {
				pushAfter = false
				break
			}
		}
	case syncMsgSnapshotRequest:
		handleErr = s.sendSnapshot(msg)
	case syncMsgSnapshotBegin, syncMsgSnapshotChunk, syncMsgSnapshotEnd:
		handleErr = s.receiveSnapshot(cursor, msg)
		pushAfter = handleErr == nil && msg.Type == syncMsgSnapshotEnd
	case syncMsgSnapshotAck:
		if s.buildSnapshot == nil || msg.SnapshotCode != s.snapshotSentCode || msg.AppliedThrough != s.snapshotSentSeq {
			handleErr = fmt.Errorf("invalid snapshot receipt")
		} else if !s.snapshotBlocked {
			// A delayed receipt cannot move an already advancing cursor back.
			pushAfter = false
		} else {
			previousAck := cursor.LastAckedSeq
			cursor.LastAckedSeq = msg.AppliedThrough
			cursor.LastSentSeq = max(cursor.LastSentSeq, msg.AppliedThrough)
			handleErr = store.SaveSyncCursor(cursor)
			if handleErr == nil {
				s.snapshotBlocked = false
				ackAdvanced = cursor.LastAckedSeq > previousAck
			}
			pushAfter = handleErr == nil
		}
	case syncMsgCursorExpired:
		handleErr = s.handleCursorExpired(cursor, msg)
	}
	s.cursorMu.Unlock()
	if handleErr != nil {
		return handleErr
	}
	// Run receipt notifications after releasing cursorMu: completing a
	// maintenance handover can read or re-drive this same session.
	if ackAdvanced && s.onProgress != nil {
		s.onProgress()
	}
	if msg.Type == syncMsgSnapshotEnd && s.installedSnapshot != nil {
		snapshot := s.installedSnapshot
		s.installedSnapshot = nil
		if s.afterSnapshot != nil {
			s.afterSnapshot(snapshot)
		}
	}
	if msg.Type == syncMsgHello && s.onHello != nil {
		s.onHello()
	}
	if pushAfter && !s.snapshotBlocked {
		return s.pushPending()
	}
	return nil
}

func (s *syncSession) handleHello(cursor *store.SyncCursor, msg *syncMessage) error {
	// A hello carries only a position, not the individual apply outcomes. Replay
	// from our last acknowledged position so a lost ACK cannot turn a conflict
	// into a false success; the peer's idempotency record returns its outcome.
	if err := store.SaveSyncCursor(cursor); err != nil {
		return err
	}
	// Replying is safe: announceHello only fires once per connection, so a
	// hello exchange terminates.
	if err := s.announceHello(); err != nil {
		return err
	}
	return nil
}

func (s *syncSession) handleChanges(cursor *store.SyncCursor, msg *syncMessage) error {
	if s.snapshotBlocked {
		return fmt.Errorf("snapshot recovery barrier is active")
	}
	if msg.InstanceID == "" {
		return fmt.Errorf("sync changes are missing sender instance")
	}
	results := make([]syncAckResult, 0, len(msg.Changes))
	previousApplied := cursor.LastAppliedSeq
	highest := previousApplied
	for i := range msg.Changes {
		ch := msg.Changes[i]
		if ch.LifecycleVersion < 1 {
			return fmt.Errorf("sync change %s is missing its code lifecycle version", ch.ChangeID())
		}
		if ch.OriginNode != s.peer.nodeID || ch.OriginInstance != msg.InstanceID {
			return fmt.Errorf("sync change %s has a sender identity different from %s", ch.ChangeID(), s.peer.nodeID)
		}
		if ch.Seq <= highest {
			status, err := store.GetAppliedChangeStatus(ch.ChangeID())
			if err != nil {
				return err
			}
			if status == "" {
				status = syncStatusSkipped
			}
			result, err := syncResultForChange(ch.ChangeID(), status)
			if err != nil {
				return err
			}
			results = append(results, result)
			continue
		}
		status := s.apply(ch)
		result, err := syncResultForChange(ch.ChangeID(), status)
		if err != nil {
			return err
		}
		results = append(results, result)
		// A rejected change was not durably accepted. Advancing past it would
		// permanently discard a maintenance-blocked write (or a failed DB write)
		// on the next reconnect. Keep the cursor at the last committed change.
		if status == syncStatusRejected {
			break
		}
		if ch.Seq > highest {
			highest = ch.Seq
		}
	}
	cursor.LastAppliedSeq = highest
	if err := store.SaveSyncCursor(cursor); err != nil {
		return err
	}
	// A rejection or replay has made no delivery progress. Waking the
	// maintenance coordinator would announce NORMAL and make the platform
	// immediately resend the same pending change, creating a retry loop.
	if highest > previousApplied && s.onProgress != nil {
		s.onProgress()
	}
	return s.emit(&syncMessage{
		Type:            syncMsgAck,
		NodeID:          s.nodeID,
		ProtocolVersion: syncProtocolVersion,
		BatchID:         msg.BatchID,
		AppliedThrough:  highest,
		Results:         results,
	})
}

func (s *syncSession) handleAck(cursor *store.SyncCursor, msg *syncMessage) error {
	for _, result := range msg.Results {
		if result.Status == syncStatusRejected {
			s.logger.Warn("Peer rejected sync change; delivery remains pending",
				zap.String("peer", s.peer.nodeID), zap.String("change_id", result.ChangeID),
				zap.Int64("applied_through", msg.AppliedThrough))
		}
	}
	if msg.AppliedThrough <= cursor.LastAckedSeq {
		return nil
	}
	if msg.AppliedThrough > cursor.LastSentSeq {
		return nil
	}
	previous := cursor.LastAckedSeq
	results := make(map[string]string, len(msg.Results))
	failures := make(map[string]store.SyncFailure)
	for _, result := range msg.Results {
		results[result.ChangeID] = result.Status
		if result.Status == syncStatusFailed {
			failures[result.ChangeID] = store.SyncFailure{ErrorCode: result.ErrorCode, LastError: result.LastError}
		}
	}
	if err := store.ApplySyncAck(cursor, previous, msg.AppliedThrough, results, s.addressed, failures); err != nil {
		return err
	}
	return nil
}

func syncResultForChange(changeID, status string) (syncAckResult, error) {
	result := syncAckResult{ChangeID: changeID, Status: status}
	if status != syncStatusFailed {
		return result, nil
	}
	var applied store.AppliedChange
	if err := store.DB.Session(&gorm.Session{NewDB: true}).Where("change_id = ?", changeID).Take(&applied).Error; err != nil {
		return result, err
	}
	result.ErrorCode, result.LastError = applied.ErrorCode, applied.LastError
	return result, nil
}

func (s *syncSession) handleCursorExpired(cursor *store.SyncCursor, msg *syncMessage) error {
	if s.installSnapshot != nil {
		return s.requestSnapshot(cursor)
	}
	// Resetting to zero would let a later batch silently skip the unavailable
	// prefix and claim convergence. Retain the last verified position until a
	// complete snapshot recovery can replace that prefix atomically.
	s.logger.Error("Peer pruned changes past our position; snapshot recovery is required",
		zap.String("peer", s.peer.nodeID),
		zap.Int64("last_verified_position", cursor.LastAppliedSeq),
		zap.String("reason", msg.Reason))
	return fmt.Errorf("sync gap with %s after sequence %d requires snapshot recovery: %s", s.peer.nodeID, cursor.LastAppliedSeq, msg.Reason)
}

// pushPending delivers every change addressed to this peer that it has not yet
// acknowledged. Changes already sent but unacknowledged are re-sent: the peer's
// idempotency table makes that harmless, and it means a dropped message needs no
// dedicated recovery path.
//
// Concurrent callers coalesce instead of queueing. A caller that arrives while a
// drain is running only raises pendingMore, so a change recorded mid-drain is
// still delivered by the running drain rather than being left until the next
// unrelated trigger.
func (s *syncSession) pushPending() error {
	s.mu.Lock()
	if s.pushing {
		s.pendingMore = true
		s.mu.Unlock()
		return nil
	}
	s.pushing = true
	s.mu.Unlock()

	var err error
	for {
		s.mu.Lock()
		s.pendingMore = false
		s.mu.Unlock()

		err = s.drain()
		if err != nil {
			break
		}

		s.mu.Lock()
		again := s.pendingMore
		s.mu.Unlock()
		if !again {
			break
		}
	}

	s.mu.Lock()
	s.pushing = false
	s.mu.Unlock()
	return err
}

func (s *syncSession) drain() error {
	s.cursorMu.Lock()
	defer s.cursorMu.Unlock()
	cursor, err := store.GetSyncCursor(s.peer.nodeID)
	if err != nil {
		return err
	}
	if s.snapshotBlocked {
		if s.installSnapshot != nil && s.deliveryToken != "" {
			return s.retrySnapshot(cursor)
		}
		if s.buildSnapshot != nil && s.snapshotPayload != nil {
			return s.sendSnapshot(&syncMessage{SnapshotCode: s.snapshotSentCode})
		}
		if s.buildSnapshot != nil {
			// The platform's snapshot request can be lost after the hello. A new
			// delivery token re-arms its recovery request while fencing delayed
			// messages from the unfinished exchange. Keep the barrier in place.
			s.mu.Lock()
			s.deliveryToken = snapshotCode()
			s.helloSent = false
			s.mu.Unlock()
			return s.announceHello()
		}
		return nil
	}

	oldest, err := store.MinSyncChangeSeq()
	if err != nil {
		return err
	}
	if oldest > 1 && cursor.LastAckedSeq+1 < oldest {
		return s.reportCursorExpired(cursor)
	}

	after := cursor.LastAckedSeq
	for {
		page, err := store.ListSyncChangesAfter(after, syncBatchLimit)
		if err != nil {
			return err
		}
		if len(page) == 0 {
			return nil
		}
		// Advance over the whole page, including entries not addressed to this
		// peer, so a page of foreign changes cannot stall the scan position.
		after = page[len(page)-1].Seq
		lastPage := len(page) < syncBatchLimit

		batch := make([]store.SyncChange, 0, len(page))
		for i := range page {
			if s.addressed(&page[i]) {
				batch = append(batch, page[i])
			}
		}
		if len(batch) == 0 {
			cursor.LastSentSeq = after
			if err := store.SaveSyncCursor(cursor); err != nil {
				return err
			}
			if lastPage {
				return nil
			}
			continue
		}

		// Persist the upper bound before publication: a fast peer can reply while
		// Publish is still returning, and that ACK must pass the sent-position check.
		cursor.LastSentSeq = after
		if err := store.SaveSyncCursor(cursor); err != nil {
			return err
		}
		err = s.emit(&syncMessage{
			Type:            syncMsgChanges,
			NodeID:          s.nodeID,
			InstanceID:      s.instanceID,
			ProtocolVersion: syncProtocolVersion,
			BatchID:         uuid.NewString(),
			HasMore:         !lastPage,
			Changes:         batch,
		})
		// Send one addressed batch at a time. A later page must not overtake an
		// earlier one while MQTT callbacks run concurrently; its ACK drives the
		// next drain. A failed send is retried from LastAckedSeq on reconnect.
		return err
	}
}

func (s *syncSession) reportCursorExpired(cursor *store.SyncCursor) error {
	s.logger.Error("Cannot satisfy peer position incrementally; snapshot recovery is required",
		zap.String("peer", s.peer.nodeID),
		zap.Int64("peer_position", cursor.LastAckedSeq))
	if err := s.emit(&syncMessage{
		Type:            syncMsgCursorExpired,
		NodeID:          s.nodeID,
		ProtocolVersion: syncProtocolVersion,
		Reason:          "oldest_available",
		Need:            "snapshot",
	}); err != nil {
		return err
	}
	return fmt.Errorf("sync gap with %s after sequence %d requires snapshot recovery", s.peer.nodeID, cursor.LastAckedSeq)
}

// syncHub owns the sessions belonging to one engine.
type syncHub struct {
	nodeID     string
	instanceID string
	logger     *zap.Logger
	publish    func(peer syncPeerRef, msg *syncMessage) error
	peers      func(ch *store.SyncChange) []syncPeerRef
	apply      func(ch store.SyncChange) string
	fallback   syncPeerRef
	// onHello runs after a peer handshake; the platform pushes node settings.
	onHello func(peer syncPeerRef)
	// onNodeConfig consumes incoming node-level settings. They are transport
	// concerns rather than entities, so they never enter the change log.
	onNodeConfig    func(msg *syncMessage)
	onMaintenance   func(peer syncPeerRef, msg *syncMessage)
	onRuntime       func(peer syncPeerRef, msg *syncMessage)
	onProgress      func(peer syncPeerRef)
	buildSnapshot   func() (*configSnapshot, error)
	installSnapshot func(*gorm.DB, *configSnapshot) error
	afterSnapshot   func(*configSnapshot)

	mu       sync.Mutex
	sessions map[string]*syncSession
}

func newSyncHub(
	nodeID, instanceID string,
	logger *zap.Logger,
	publish func(peer syncPeerRef, msg *syncMessage) error,
	peers func(ch *store.SyncChange) []syncPeerRef,
	apply func(ch store.SyncChange) string,
) *syncHub {
	return &syncHub{
		nodeID:     nodeID,
		instanceID: instanceID,
		logger:     logger,
		publish:    publish,
		peers:      peers,
		apply:      apply,
		sessions:   make(map[string]*syncSession),
	}
}

func (h *syncHub) session(peer syncPeerRef) *syncSession {
	h.mu.Lock()
	defer h.mu.Unlock()
	if s, ok := h.sessions[peer.nodeID]; ok {
		return s
	}
	s := &syncSession{
		nodeID:          h.nodeID,
		instanceID:      h.instanceID,
		peer:            peer,
		logger:          h.logger,
		publish:         h.publish,
		apply:           h.apply,
		buildSnapshot:   h.buildSnapshot,
		installSnapshot: h.installSnapshot,
		afterSnapshot:   h.afterSnapshot,
	}
	s.addressed = func(ch *store.SyncChange) bool {
		if h.peers == nil {
			return true
		}
		for _, p := range h.peers(ch) {
			if p.nodeID == peer.nodeID {
				return true
			}
		}
		return false
	}
	s.onHello = func() {
		if h.onHello != nil {
			h.onHello(peer)
		}
	}
	s.onProgress = func() {
		if h.onProgress != nil {
			h.onProgress(peer)
		}
	}
	h.sessions[peer.nodeID] = s
	return s
}

// handleInbound routes a message received from a peer.
func (h *syncHub) handleInbound(peer syncPeerRef, payload []byte) {
	msg, err := decodeSyncMessage(payload)
	if err != nil {
		h.logger.Warn("Discarding malformed sync message",
			zap.String("peer", peer.nodeID), zap.Error(err))
		return
	}
	if peer.nodeID == "" || msg.NodeID != peer.nodeID || msg.ProtocolVersion != syncProtocolVersion {
		h.logger.Warn("Discarding sync message with invalid sender or protocol",
			zap.String("peer", peer.nodeID), zap.String("sender", msg.NodeID),
			zap.Int("protocol_version", msg.ProtocolVersion))
		return
	}
	if msg.Type != syncMsgHello && (h.buildSnapshot != nil || h.installSnapshot != nil) {
		session := h.session(peer)
		cursor, err := store.GetSyncCursor(peer.nodeID)
		// These drops used to be silent, which is what made a maintenance
		// handover look like it had stalled with no evidence in any log. Every
		// reason is reported, and the reason names the field that was wrong.
		if err != nil {
			h.logger.Warn("Discarding sync message: cursor unavailable",
				zap.String("peer", peer.nodeID), zap.String("type", msg.Type), zap.Error(err))
			return
		}
		if session.deliveryToken == "" {
			h.logger.Warn("Discarding sync message: handshake has not completed on this connection",
				zap.String("peer", peer.nodeID), zap.String("type", msg.Type))
			return
		}
		if session.deliveryToken != msg.SessionToken {
			h.logger.Warn("Discarding sync message: stale session token",
				zap.String("peer", peer.nodeID), zap.String("type", msg.Type))
			return
		}
		if cursor.PeerInstance != msg.InstanceID {
			h.logger.Warn("Discarding sync message: sender generation is not the announced one",
				zap.String("peer", peer.nodeID), zap.String("type", msg.Type),
				zap.String("instance_id", msg.InstanceID), zap.String("announced", cursor.PeerInstance))
			return
		}
	}
	// Node-level settings carry no cursor semantics and are consumed directly.
	if msg.Type == syncMsgNodeConfig {
		if h.onNodeConfig != nil {
			h.onNodeConfig(msg)
		}
		return
	}
	if msg.Type == syncMsgMaintenanceState || msg.Type == syncMsgMaintenanceRequest || msg.Type == syncMsgMaintenanceHandoverAck {
		if h.onMaintenance != nil {
			h.onMaintenance(peer, msg)
		}
		return
	}
	if msg.Type == syncMsgRuntimeResult || msg.Type == syncMsgRuntimeResultAck {
		if h.onRuntime != nil {
			h.onRuntime(peer, msg)
		}
		return
	}
	if err := h.session(peer).handle(msg); err != nil {
		h.logger.Warn("Sync session handling failed",
			zap.String("peer", peer.nodeID), zap.String("type", msg.Type), zap.Error(err))
	}
}

// onConnected re-arms the handshake for every known session and announces this
// node so peers can run their own catch-up against our cursor.
func (h *syncHub) onConnected() {
	h.mu.Lock()
	sessions := make([]*syncSession, 0, len(h.sessions))
	for _, s := range h.sessions {
		sessions = append(sessions, s)
	}
	h.mu.Unlock()

	for _, s := range sessions {
		s.resetForReconnect()
		if err := s.announceHello(); err != nil {
			h.logger.Warn("Failed to re-announce sync session", zap.String("peer", s.peer.nodeID), zap.Error(err))
		}
	}
	if h.fallback.nodeID == "" {
		return
	}
	if err := h.session(h.fallback).announceHello(); err != nil {
		h.logger.Warn("Failed to announce sync position after reconnect",
			zap.String("peer", h.fallback.nodeID), zap.Error(err))
	}
}

// notifyChange pushes a freshly recorded local change to every peer it is
// addressed to. Delivery failure is intentionally not retried here: the peer's
// cursor still points at the last acknowledged position, so the next handshake
// re-sends this change.
func (h *syncHub) notifyChange(ch store.SyncChange) {
	var targets []syncPeerRef
	if h.peers != nil {
		targets = h.peers(&ch)
	}
	if len(targets) == 0 {
		if h.fallback.nodeID == "" {
			return
		}
		targets = []syncPeerRef{h.fallback}
	}
	for _, peer := range targets {
		if err := h.session(peer).pushPending(); err != nil {
			h.logger.Warn("Failed to push pending changes",
				zap.String("peer", peer.nodeID), zap.Error(err))
		}
	}
}

// announceAll re-announces this node to every peer it has ever talked to. It is
// used when the peer's position can no longer be satisfied and both sides must
// re-handshake.
func (h *syncHub) announceAll() {
	h.mu.Lock()
	sessions := make([]*syncSession, 0, len(h.sessions))
	for _, s := range h.sessions {
		sessions = append(sessions, s)
	}
	h.mu.Unlock()
	for _, s := range sessions {
		if err := s.sendHello(); err != nil {
			h.logger.Warn("Failed to re-announce sync position",
				zap.String("peer", s.peer.nodeID), zap.Error(err))
		}
	}
}

func decodeSyncMessage(payload []byte) (*syncMessage, error) {
	var msg syncMessage
	if err := json.Unmarshal(payload, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

// pruneSettledChanges reclaims change records that every peer has acknowledged
// and that are older than the retention window.
//
// Retention is deliberately generous. A peer that has been offline longer than
// the window can no longer be caught up incrementally, and the next handshake
// asks it to restart from the beginning instead of silently skipping the pruned
// records. That restart is the correctness guarantee behind pruning at all.
func pruneSettledChanges(logger *zap.Logger, selfNodeID string) {
	cursors, err := store.ListSyncCursors()
	if err != nil || len(cursors) == 0 {
		return
	}
	var safeThrough int64 = -1
	for i := range cursors {
		if cursors[i].PeerID == selfNodeID {
			continue
		}
		if safeThrough < 0 || cursors[i].LastAckedSeq < safeThrough {
			safeThrough = cursors[i].LastAckedSeq
		}
	}
	if safeThrough <= 0 {
		return
	}
	cutoff := time.Now().Add(-syncTombstoneRetention).UnixMilli()
	res := store.DB.Where("seq <= ? AND created_at < ?", safeThrough, cutoff).Delete(&store.SyncChange{})
	if res.Error != nil {
		logger.Warn("Failed to prune settled changes", zap.Error(res.Error))
		return
	}
	if res.RowsAffected > 0 {
		logger.Info("Pruned settled change records",
			zap.Int64("through_seq", safeThrough),
			zap.Int64("removed", res.RowsAffected))
	}
	if _, err := store.PruneAppliedChanges(syncTombstoneRetention); err != nil {
		logger.Warn("Failed to prune idempotency records", zap.Error(err))
	}
}

// startSyncMaintenance runs the periodic reclamation of settled change records.
func startSyncMaintenance(ctx context.Context, logger *zap.Logger, selfNodeID string) {
	const interval = 6 * time.Hour
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pruneSettledChanges(logger, selfNodeID)
		}
	}
}
