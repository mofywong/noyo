package cascade

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"noyo/core/store"

	"github.com/google/uuid"
	"go.uber.org/zap"
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

	mu          sync.Mutex
	pushing     bool
	pendingMore bool
	helloSent   bool
}

// resetForReconnect clears per-connection state. Both sides re-handshake after
// every reconnect so a gap that opened while the link was down is always
// discovered by comparing cursors, never by guessing.
func (s *syncSession) resetForReconnect() {
	s.mu.Lock()
	s.helloSent = false
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
	s.helloSent = true
	s.mu.Unlock()
	return s.publish(s.peer, &syncMessage{
		Type:            syncMsgHello,
		NodeID:          s.nodeID,
		InstanceID:      s.instanceID,
		ProtocolVersion: syncProtocolVersion,
		PeerLastApplied: cursor.LastAppliedSeq,
		MyOldestSeq:     oldest,
	})
}

// announceHello sends a handshake only when this connection has not sent one.
func (s *syncSession) announceHello() error {
	if !s.shouldAnnounce() {
		return nil
	}
	return s.sendHello()
}

func (s *syncSession) handle(msg *syncMessage) error {
	if msg == nil || msg.Type == "" {
		return nil
	}
	if msg.NodeID != "" && msg.NodeID != s.peer.nodeID {
		s.logger.Warn("Sync message peer mismatch; using the routing identity",
			zap.String("expected", s.peer.nodeID),
			zap.String("received", msg.NodeID),
			zap.String("type", msg.Type))
	}

	cursor, err := store.GetSyncCursor(s.peer.nodeID)
	if err != nil {
		return err
	}
	if cursor.PeerKind == "" {
		cursor.PeerKind = s.peer.kind
	}
	cursor.LastOnlineAt = time.Now().UnixMilli()

	// A peer that rebuilt its database restarts its sequence numbers. The
	// position recorded here then refers to a log that no longer exists, so the
	// only safe reading is that the peer's stream has not been seen at all.
	if msg.InstanceID != "" && cursor.PeerInstance != msg.InstanceID {
		if cursor.PeerInstance != "" {
			s.logger.Warn("Peer change log was rebuilt; restarting its stream from the beginning",
				zap.String("peer", s.peer.nodeID),
				zap.String("previous_instance", cursor.PeerInstance),
				zap.String("instance", msg.InstanceID),
				zap.Int64("discarded_position", cursor.LastAppliedSeq))
		}
		cursor.PeerInstance = msg.InstanceID
		cursor.LastAppliedSeq = 0
	}

	switch msg.Type {
	case syncMsgHello:
		return s.handleHello(cursor, msg)
	case syncMsgChanges:
		return s.handleChanges(cursor, msg)
	case syncMsgAck:
		return s.handleAck(cursor, msg)
	case syncMsgCursorExpired:
		return s.handleCursorExpired(cursor, msg)
	default:
		return nil
	}
}

func (s *syncSession) handleHello(cursor *store.SyncCursor, msg *syncMessage) error {
	// The peer tells us how far it has consumed our stream, which is exactly our
	// acknowledgement position.
	if msg.PeerLastApplied > cursor.LastAckedSeq {
		cursor.LastAckedSeq = msg.PeerLastApplied
	}
	if err := store.SaveSyncCursor(cursor); err != nil {
		return err
	}
	// Replying is safe: announceHello only fires once per connection, so a
	// hello exchange terminates.
	if err := s.announceHello(); err != nil {
		return err
	}
	if s.onHello != nil {
		s.onHello()
	}
	return s.pushPending()
}

func (s *syncSession) handleChanges(cursor *store.SyncCursor, msg *syncMessage) error {
	results := make([]syncAckResult, 0, len(msg.Changes))
	highest := cursor.LastAppliedSeq
	for i := range msg.Changes {
		ch := msg.Changes[i]
		status := s.apply(ch)
		results = append(results, syncAckResult{ChangeID: ch.ChangeID(), Status: status})
		if ch.Seq > highest {
			highest = ch.Seq
		}
	}
	cursor.LastAppliedSeq = highest
	if err := store.SaveSyncCursor(cursor); err != nil {
		return err
	}
	return s.publish(s.peer, &syncMessage{
		Type:           syncMsgAck,
		NodeID:         s.nodeID,
		BatchID:        msg.BatchID,
		AppliedThrough: highest,
		Results:        results,
	})
}

func (s *syncSession) handleAck(cursor *store.SyncCursor, msg *syncMessage) error {
	if msg.AppliedThrough <= cursor.LastAckedSeq {
		return nil
	}
	previous := cursor.LastAckedSeq
	cursor.LastAckedSeq = msg.AppliedThrough
	if err := store.SaveSyncCursor(cursor); err != nil {
		return err
	}
	if err := store.MarkChangesSettled(previous, msg.AppliedThrough); err != nil {
		s.logger.Warn("Failed to clear pending sync state for acknowledged changes", zap.Error(err))
	}
	// Keep draining: the peer may have acknowledged only a partial batch.
	return s.pushPending()
}

func (s *syncSession) handleCursorExpired(cursor *store.SyncCursor, msg *syncMessage) error {
	// Our position in the peer's stream refers to history the peer no longer
	// holds, so incremental catch-up is impossible; the only correct recovery is
	// to re-consume its stream from the beginning.
	s.logger.Warn("Peer pruned changes past our position; restarting from the start of its stream",
		zap.String("peer", s.peer.nodeID),
		zap.Int64("discarded_position", cursor.LastAppliedSeq),
		zap.String("reason", msg.Reason))
	cursor.LastAppliedSeq = 0
	if err := store.SaveSyncCursor(cursor); err != nil {
		return err
	}
	return s.sendHello()
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
	cursor, err := store.GetSyncCursor(s.peer.nodeID)
	if err != nil {
		return err
	}

	oldest, err := store.MinSyncChangeSeq()
	if err != nil {
		return err
	}
	if cursor.LastAckedSeq > 0 && oldest > 0 && cursor.LastAckedSeq+1 < oldest {
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

		err = s.publish(s.peer, &syncMessage{
			Type:            syncMsgChanges,
			NodeID:          s.nodeID,
			InstanceID:      s.instanceID,
			ProtocolVersion: syncProtocolVersion,
			BatchID:         uuid.NewString(),
			HasMore:         !lastPage,
			Changes:         batch,
		})
		if err != nil {
			// Leave the cursor untouched so the batch is retried after the next
			// handshake instead of being skipped.
			return err
		}
		cursor.LastSentSeq = after
		if err := store.SaveSyncCursor(cursor); err != nil {
			return err
		}
		if lastPage {
			return nil
		}
	}
}

func (s *syncSession) reportCursorExpired(cursor *store.SyncCursor) error {
	s.logger.Warn("Cannot satisfy peer position incrementally; requesting a restart",
		zap.String("peer", s.peer.nodeID),
		zap.Int64("peer_position", cursor.LastAckedSeq))
	cursor.LastSentSeq = 0
	cursor.LastAckedSeq = 0
	if err := store.SaveSyncCursor(cursor); err != nil {
		return err
	}
	return s.publish(s.peer, &syncMessage{
		Type:   syncMsgCursorExpired,
		NodeID: s.nodeID,
		Reason: "oldest_available",
		Need:   "restart",
	})
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
	onNodeConfig func(msg *syncMessage)

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
		nodeID:     h.nodeID,
		instanceID: h.instanceID,
		peer:       peer,
		logger:     h.logger,
		publish:    h.publish,
		apply:      h.apply,
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
	if msg.NodeID == "" {
		msg.NodeID = peer.nodeID
	}
	// Node-level settings carry no cursor semantics and are consumed directly.
	if msg.Type == syncMsgNodeConfig {
		if h.onNodeConfig != nil {
			h.onNodeConfig(msg)
		}
		return
	}
	// A message always names its sender, so adopt the advertised identity: on
	// the platform it must agree with the MQTT routing code, on the gateway it is
	// always the cloud.
	if peer.nodeID == "" {
		peer.nodeID = msg.NodeID
		if peer.kind == "" {
			peer.kind = "peer"
		}
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
	}
	if h.fallback.nodeID == "" {
		return
	}
	if err := h.session(h.fallback).sendHello(); err != nil {
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
