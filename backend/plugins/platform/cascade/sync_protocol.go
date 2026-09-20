package cascade

import (
	"fmt"
	"time"

	"noyo/core/store"
)

// Wire protocol message kinds exchanged on the sync/up and sync/down topics.
//
// Every message carries a "type" discriminator so a single topic pair per
// direction is enough; adding a message kind never requires a new subscription.
const (
	syncMsgHello         = "hello"
	syncMsgChanges       = "changes"
	syncMsgAck           = "ack"
	syncMsgCursorExpired = "cursor_expired"
	// syncMsgNodeConfig carries node-level settings that are not entities and
	// therefore have no place in the change log. It replaced the media network
	// block that used to ride along with the full configuration snapshot.
	syncMsgNodeConfig = "node_config"
)

// Per-change acknowledgement statuses. The sender advances its cursor on every
// one of them: a change that will never apply (rejected) must not block the
// stream, or the cursor would stall forever.
const (
	syncStatusApplied  = "applied"
	syncStatusSkipped  = "skipped"
	syncStatusConflict = "conflict"
	syncStatusRejected = "rejected"
)

const (
	// syncBatchLimit caps how many changes travel in one message.
	syncBatchLimit = 200
	// syncProtocolVersion is the protocol revision advertised during handshake.
	syncProtocolVersion = 1
	// cloudSyncNodeID is the platform's identity, used as the cursor key on the
	// gateway and as the origin node of platform-produced changes.
	cloudSyncNodeID = "cloud"
	// gatewayOrigin is the provenance stamped on entities created at the edge.
	gatewayOrigin = "gateway"
	// syncPublishTimeout bounds how long a publish waits for the broker to
	// acknowledge. Pushes run off the write path, so a slow broker delays
	// propagation rather than the user's save.
	syncPublishTimeout = 5 * time.Second
	// syncTombstoneRetention is how long a deletion record is kept after every
	// peer has acknowledged it. A peer offline longer than this cannot be caught
	// up incrementally and is asked to restart instead.
	syncTombstoneRetention = 30 * 24 * time.Hour
)

// syncMessage is the envelope for every synchronization exchange. Fields are
// shared across message kinds and left zero when unused, which keeps decoding
// trivial on both sides.
type syncMessage struct {
	Type string `json:"type"`
	// NodeID identifies the sender and keys the receiver's cursor row.
	NodeID string `json:"node_id"`
	// InstanceID identifies the sender's change log. A change here means the
	// sender rebuilt its database, so the receiver's recorded position in that
	// stream describes nothing and must be reset.
	InstanceID      string `json:"instance_id,omitempty"`
	ProtocolVersion int    `json:"protocol_version,omitempty"`

	// Handshake fields.
	//
	// PeerLastApplied states how far the sender has consumed the receiver's
	// stream. MyOldestSeq states the earliest record the sender still holds, so
	// the receiver can detect that a position has already been pruned.
	PeerLastApplied int64 `json:"peer_last_applied_seq,omitempty"`
	MyOldestSeq     int64 `json:"my_oldest_seq,omitempty"`

	// Change batch fields.
	BatchID string             `json:"batch_id,omitempty"`
	HasMore bool               `json:"has_more,omitempty"`
	Changes []store.SyncChange `json:"changes,omitempty"`

	// Acknowledgement fields.
	AppliedThrough int64           `json:"applied_through_seq,omitempty"`
	Results        []syncAckResult `json:"results,omitempty"`

	// Recovery fields.
	Reason string `json:"reason,omitempty"`
	Need   string `json:"need,omitempty"`

	// Node-level settings.
	MediaNetwork *SyncMediaNetwork `json:"media_network,omitempty"`
}

type syncAckResult struct {
	ChangeID string `json:"change_id"`
	Status   string `json:"status"`
}

// syncPeerRef identifies the remote side of a synchronization session.
type syncPeerRef struct {
	// nodeID keys the cursor row and matches the peer's advertised NodeID.
	nodeID string
	// gatewayCode is the MQTT routing code. It is empty on the gateway side,
	// where publishing always targets the platform.
	gatewayCode string
	kind        string
}

func syncUpTopic(gatewayCode string) string {
	return fmt.Sprintf("noyo/cascade/gw/%s/sync/up", gatewayCode)
}

func syncDownTopic(gatewayCode string) string {
	return fmt.Sprintf("noyo/cascade/gw/%s/sync/down", gatewayCode)
}
