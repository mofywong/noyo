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
	syncMsgSnapshotRequest = "snapshot_request"
	syncMsgSnapshotBegin   = "snapshot_begin"
	syncMsgSnapshotChunk   = "snapshot_chunk"
	syncMsgSnapshotEnd     = "snapshot_end"
	syncMsgSnapshotAck     = "snapshot_ack"
	syncMsgHello           = "hello"
	syncMsgChanges         = "changes"
	syncMsgAck             = "ack"
	syncMsgCursorExpired   = "cursor_expired"
	// syncMsgNodeConfig carries node-level settings that are not entities and
	// therefore have no place in the change log. It replaced the media network
	// block that used to ride along with the full configuration snapshot.
	syncMsgNodeConfig             = "node_config"
	syncMsgMaintenanceState       = "maintenance_state"
	syncMsgMaintenanceRequest     = "maintenance_request"
	syncMsgMaintenanceHandoverAck = "maintenance_handover_ack"
	syncMsgRuntimeResult          = "runtime_result"
	syncMsgRuntimeResultAck       = "runtime_result_ack"
)

// Per-change acknowledgement statuses. Rejected is retried after maintenance
// ends or a transient error is repaired; it never advances the receive cursor.
const (
	syncStatusApplied  = "applied"
	syncStatusSkipped  = "skipped"
	syncStatusConflict = "conflict"
	syncStatusRejected = "rejected"
	// Failed is a durable business outcome. It consumes the transport record
	// without applying configuration or blocking unrelated resources.
	syncStatusFailed = "failed"
)

const (
	// syncBatchLimit caps how many changes travel in one message.
	syncBatchLimit = 200
	// syncProtocolVersion is the protocol revision advertised during handshake.
	// Dependency envelopes and terminal failure ACKs require both peers to
	// understand v3; a v2 peer must not silently accept an incomplete request.
	syncProtocolVersion = store.SyncCursorProtocolVersion
	// cloudSyncNodeID is the platform's identity, used as the cursor key on the
	// gateway and as the origin node of platform-produced changes.
	cloudSyncNodeID = "cloud"
	// gatewayOrigin is the provenance stamped on entities created at the edge.
	gatewayOrigin = "gateway"
	// syncPublishTimeout bounds how long a publish waits for the broker to
	// acknowledge. Pushes run off the write path, so a slow broker delays
	// propagation rather than the user's save.
	syncPublishTimeout = 5 * time.Second
	// Kept for the dormant pruning helper. The scheduler does not invoke it
	// until a full snapshot recovery protocol is available.
	syncTombstoneRetention = 30 * 24 * time.Hour
)

// syncMessage is the envelope for every synchronization exchange. Fields are
// shared across message kinds and left zero when unused, which keeps decoding
// trivial on both sides.
type syncMessage struct {
	Type          string          `json:"type"`
	SessionToken  string          `json:"session_token,omitempty"`
	HelloCode     string          `json:"hello_code,omitempty"`
	SnapshotCode  string          `json:"snapshot_code,omitempty"`
	Snapshot      *configSnapshot `json:"snapshot,omitempty"`
	SnapshotHash  string          `json:"snapshot_hash,omitempty"`
	SnapshotParts int             `json:"snapshot_parts,omitempty"`
	SnapshotIndex int             `json:"snapshot_index,omitempty"`
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
	MediaNetwork      *SyncMediaNetwork         `json:"media_network,omitempty"`
	Maintenance       *store.GatewayMaintenance `json:"maintenance,omitempty"`
	MaintenanceAction string                    `json:"maintenance_action,omitempty"`
	RuntimeResult     *store.RuntimeApply       `json:"runtime_result,omitempty"`
	RuntimeChangeID   string                    `json:"runtime_change_id,omitempty"`
	RuntimeAttempts   int                       `json:"runtime_attempts,omitempty"`
}

type syncAckResult struct {
	ChangeID  string `json:"change_id"`
	Status    string `json:"status"`
	ErrorCode string `json:"error_code,omitempty"`
	LastError string `json:"last_error,omitempty"`
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
