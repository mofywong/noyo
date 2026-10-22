package core

import (
	"context"
	"errors"

	"noyo/core/screen"
	"noyo/core/store"
	"noyo/core/tsdb"
)

// Both AI previews and the saved-screen runtime use the same bounded event feed.
// Callers authorize the device set; immutable owners also isolate reused codes.
func (s *Server) screenEventRows(ctx context.Context, row *store.IOCScreen, binding screen.Binding, start, end int64, limit int) ([]map[string]interface{}, error) {
	result := []map[string]interface{}{}
	if s.TSDB == nil {
		return result, nil
	}
	if limit <= 0 || limit > 200 {
		limit = 200
	}
	codes := screenBindingDevices(binding)
	var devices []store.Device
	if err := store.DB.WithContext(ctx).Where("tenant_id = ? AND project_id = ? AND code IN ?", row.TenantID, row.ProjectID, codes).Find(&devices).Error; err != nil {
		return nil, err
	}
	if len(devices) != len(codes) || len(codes) == 0 {
		return nil, errors.New("event device scope is no longer readable")
	}
	owners := []tsdb.RecordOwner{}
	deviceMap := map[string]store.Device{}
	products := map[string]map[string]screen.ProfileEvent{}
	for _, device := range devices {
		owners = append(owners, tsdb.RecordOwner{DeviceID: device.ID, TenantID: device.TenantID, ProjectID: device.ProjectID})
		deviceMap[device.Code] = device
		if _, loaded := products[device.ProductCode]; !loaded {
			products[device.ProductCode] = map[string]screen.ProfileEvent{}
			if product, err := store.GetProduct(device.ProductCode); err == nil && product != nil {
				for _, event := range screenAssistantEventMetadata(product.Config) {
					products[device.ProductCode][event.Key] = event
				}
			}
		}
	}
	response, err := s.TSDB.Query(tsdb.QueryRequest{Context: ctx, Owners: owners, SkipTotal: true, StartTime: start, EndTime: end, Type: tsdb.TypeEvent, Page: 1, PageSize: limit})
	if err != nil || response == nil {
		return result, err
	}
	for _, entry := range response.List {
		data, ok := entry.(map[string]interface{})
		if !ok {
			continue
		}
		code, _ := data["device_code"].(string)
		device, ok := deviceMap[code]
		if !ok {
			continue
		}
		key, _ := data["event_id"].(string)
		event, ok := products[device.ProductCode][key]
		if !ok {
			continue
		}
		params, _ := data["params"].(map[string]interface{})
		fields := []map[string]interface{}{}
		for _, field := range event.Fields {
			value, exists := params[field.Key]
			if !exists {
				continue
			}
			switch typed := value.(type) {
			case string:
				value = screenAssistantText(typed, 512)
			case nil, bool, float64:
			default:
				continue
			}
			fields = append(fields, map[string]interface{}{"key": field.Key, "zh": field.Zh, "en": field.En, "unit": field.Unit, "value": value})
		}
		result = append(result, map[string]interface{}{"ts": data["ts"], "deviceCode": code, "deviceName": screenAssistantText(device.Name, 128), "eventKey": key, "eventZh": event.Zh, "eventEn": event.En, "eventType": event.Type, "parameters": fields})
	}
	return result, nil
}
