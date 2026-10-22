package core

import (
	"fmt"
	"noyo/core/store"
	"noyo/core/tsdb"

	"github.com/gogf/gf/v2/net/ghttp"
)

// Resolve the current inventory first, then pass an internal ownership snapshot
// to history storage. Client-supplied codes are never authorization boundaries.
func historyOwners(r *ghttp.Request, code string) ([]tsdb.RecordOwner, error) {
	auth := requestAuthContext(r)
	if auth == nil || auth.TenantID == 0 {
		return nil, fmt.Errorf("History access denied / 无权访问历史数据")
	}
	query := store.DB.WithContext(r.Context()).Where("tenant_id = ?", auth.TenantID)
	if ids, restricted := auth.ProjectIDsForTenantQuery(); restricted {
		if len(ids) == 0 {
			return []tsdb.RecordOwner{}, nil
		}
		query = query.Where("project_id IN ?", ids)
	}
	if selected := r.GetCtxVar("project_id").Uint(); selected > 0 {
		query = query.Where("project_id = ?", selected)
	}
	if code != "" {
		query = query.Where("code = ?", code)
	}
	var devices []store.Device
	if err := query.Select("id,code,tenant_id,project_id").Limit(5001).Find(&devices).Error; err != nil {
		return nil, err
	}
	if len(devices) > 5000 {
		return nil, fmt.Errorf("Select a project or device to narrow history scope / 请选择项目或设备以缩小历史范围")
	}
	owners := make([]tsdb.RecordOwner, 0, len(devices))
	for _, device := range devices {
		allowed, err := canReadDeviceByTagPermissionFor(auth, currentDeviceTagScope(r), device.Code, "device:list")
		if err != nil {
			return nil, err
		}
		if allowed && canAccessDevice(r, &device) {
			owners = append(owners, tsdb.RecordOwner{DeviceID: device.ID, TenantID: device.TenantID, ProjectID: device.ProjectID})
		}
	}
	if code != "" && len(owners) == 0 {
		return nil, fmt.Errorf("History access denied / 无权访问历史数据")
	}
	return owners, nil
}

func canDeleteHistoryRecord(r *ghttp.Request, record *tsdb.Record) bool {
	if record == nil || record.DeviceID == 0 || record.TenantID == 0 {
		return false
	}
	owners, err := historyOwners(r, record.DeviceCode)
	if err != nil {
		return false
	}
	matches := false
	for _, owner := range owners {
		if owner.DeviceID == record.DeviceID && owner.TenantID == record.TenantID && owner.ProjectID == record.ProjectID {
			matches = true
			break
		}
	}
	if !matches {
		return false
	}
	auth := requestAuthContext(r)
	if auth.SubjectType == "app" && !auth.HasProjectPermission("history:delete", record.ProjectID) {
		return false
	}
	allowed, err := canWriteDeviceByTagPermission(auth, currentDeviceTagScope(r), record.DeviceCode)
	return err == nil && allowed
}

func (dm *DeviceManager) historyOwner(deviceCode string) (tsdb.RecordOwner, error) {
	device, err := store.GetDevice(deviceCode)
	if err != nil {
		return tsdb.RecordOwner{}, err
	}
	return tsdb.RecordOwner{DeviceID: device.ID, TenantID: device.TenantID, ProjectID: device.ProjectID}, nil
}
