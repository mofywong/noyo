package workorder

import (
	"context"
	"sync"
	"time"
)

type DeviceMaintenanceRecord struct {
	DeviceID, WorkOrderPublicID, WorkOrderCode, Fault, Assignee, Resolution string
	CompletedAt                                                             time.Time
	Attachments                                                             []AttachmentMeta
}
type DeviceHistoryRecord struct {
	DeviceID, WorkOrderPublicID, WorkOrderCode, Fault, Assignee, Resolution string
	CompletedAt                                                             time.Time
	Attachments                                                             []AttachmentMeta
}
type memoryDeviceHistory struct {
	mu      sync.Mutex
	records map[string][]DeviceHistoryRecord
}

func NewDeviceHistoryWriter() *memoryDeviceHistory {
	return &memoryDeviceHistory{records: map[string][]DeviceHistoryRecord{}}
}
func (w *memoryDeviceHistory) WriteClosed(_ context.Context, scope Scope, record DeviceMaintenanceRecord) error {
	if scope.TenantID == 0 || scope.ProjectID == 0 || record.DeviceID == "" {
		return NewError(CodeValidationFailed, "device history scope is required")
	}
	attachments := make([]AttachmentMeta, len(record.Attachments))
	for i, attachment := range record.Attachments {
		attachment.StorageKey = ""
		attachments[i] = attachment
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	w.records[scopeKey(scope, "history:"+record.DeviceID)] = append(w.records[scopeKey(scope, "history:"+record.DeviceID)], DeviceHistoryRecord{DeviceID: record.DeviceID, WorkOrderPublicID: record.WorkOrderPublicID, WorkOrderCode: record.WorkOrderCode, Fault: record.Fault, Assignee: record.Assignee, Resolution: record.Resolution, CompletedAt: record.CompletedAt, Attachments: attachments})
	return nil
}

// Write implements the narrow subscriber port used by event consumers.  The
// payload is intentionally limited to public identifiers and resolution
// metadata; storage paths are ignored even if a caller includes one.
func (w *memoryDeviceHistory) Write(ctx context.Context, scope Scope, deviceID string, payload map[string]any) error {
	record := DeviceMaintenanceRecord{DeviceID: deviceID}
	if payload != nil {
		record.WorkOrderPublicID, _ = payload["work_order_public_id"].(string)
		record.WorkOrderCode, _ = payload["work_order_code"].(string)
		record.Fault, _ = payload["fault"].(string)
		if record.Fault == "" {
			record.Fault, _ = payload["description"].(string)
		}
		record.Assignee, _ = payload["assignee"].(string)
		record.Resolution, _ = payload["resolution"].(string)
		if at, ok := payload["completed_at"].(time.Time); ok {
			record.CompletedAt = at
		}
	}
	return w.WriteClosed(ctx, scope, record)
}

var _ DeviceHistoryWriter = (*memoryDeviceHistory)(nil)

func (w *memoryDeviceHistory) History(_ context.Context, scope Scope, deviceID string) ([]DeviceHistoryRecord, error) {
	if scope.TenantID == 0 || scope.ProjectID == 0 || deviceID == "" {
		return nil, NewError(CodeValidationFailed, "device history scope is required")
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	records := w.records[scopeKey(scope, "history:"+deviceID)]
	result := make([]DeviceHistoryRecord, len(records))
	copy(result, records)
	return result, nil
}
