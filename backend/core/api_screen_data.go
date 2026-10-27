package core

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gogf/gf/v2/net/ghttp"
	"gorm.io/gorm"
	"noyo/core/dataquery"
	"noyo/core/screen"
	"noyo/core/store"
	"noyo/core/tsdb"
)

// All callers authorize before consulting the cache. Scope, current identity,
// resolved devices and product revisions are part of the key.
var screenDataCache = dataquery.NewCache(16, 4)

type ScreenDataScope struct {
	TenantID, ProjectID, UserID uint
	AppID                       string
	Devices                     []store.Device
}
type ScreenDataProvider func(context.Context, ScreenDataScope, dataquery.Source, dataquery.Input) ([]dataquery.Row, error)
type screenProvider struct {
	permission string
	read       ScreenDataProvider
}

var screenProviders = struct {
	sync.RWMutex
	items map[string]screenProvider
}{items: map[string]screenProvider{}}

// RegisterScreenDataProvider is the server-side extension point for trusted
// integrations. Credentials and network/database access stay on the server.
// Providers must use the supplied tenant/project and authorized device scope.
func RegisterScreenDataProvider(kind, permission string, read ScreenDataProvider) error {
	if !strings.HasPrefix(kind, "ext.") || len(kind) > 128 || permission == "" || read == nil {
		return errors.New("Invalid data provider / 数据提供器无效")
	}
	screenProviders.Lock()
	defer screenProviders.Unlock()
	if _, ok := screenProviders.items[kind]; ok {
		return errors.New("Provider already registered / 数据提供器已注册")
	}
	screenProviders.items[kind] = screenProvider{permission, read}
	return nil
}

func screenDatasetPlan(row *store.IOCScreen, b screen.Binding) (dataquery.Plan, error) {
	if b.Query != nil {
		return *b.Query, nil
	}
	var plan dataquery.Plan
	if b.Dataset == nil {
		return plan, screenFail(400, "Missing dataset / 缺少数据集")
	}
	var version store.IOCDataSetVersion
	err := store.DB.Table("ioc_data_set_versions AS v").Select("v.*").Joins("JOIN ioc_data_sets AS d ON d.id = v.dataset_id").Where("d.tenant_id = ? AND d.project_id = ? AND d.id = ? AND v.version = ?", row.TenantID, row.ProjectID, b.Dataset.ID, b.Dataset.Version).Take(&version).Error
	if err != nil {
		return plan, screenFail(404, "Dataset version unavailable in this project / 当前项目中不存在此数据集版本")
	}
	if err = json.Unmarshal([]byte(version.PlanJSON), &plan); err != nil {
		return plan, err
	}
	return plan, dataquery.Validate(plan)
}

type screenPreparedSource struct {
	source   dataquery.Source
	devices  []store.Device
	provider *screenProvider
}

// No request object escapes into shared work: ACL checks run synchronously.
func screenPrepareData(r *ghttp.Request, row *store.IOCScreen, plan dataquery.Plan, input dataquery.Input) (map[string]screenPreparedSource, string, error) {
	if err := dataquery.Validate(plan); err != nil {
		return nil, "", err
	}
	prepared := map[string]screenPreparedSource{}
	auth := requestAuthContext(r)
	fingerprint := []interface{}{row.TenantID, row.ProjectID, auth.SubjectType, auth.UserID, auth.AppID, fmt.Sprintf("%p", store.DB)}
	productFields := map[string]map[string]bool{}
	for _, source := range plan.Sources {
		item := screenPreparedSource{source: source}
		if source.Kind == "static" {
			prepared[source.ID] = item
			continue
		}
		permission := "device:list"
		switch source.Kind {
		case "telemetry", "latest", "devices", "events":
		case "alarms":
			permission = "alarm:list"
		case "workorders":
			permission = "work_order:list"
		default:
			screenProviders.RLock()
			provider, ok := screenProviders.items[source.Kind]
			screenProviders.RUnlock()
			if !ok {
				return nil, "", screenFail(400, "Unsupported data source / 不支持的数据源: "+source.Kind)
			}
			item.provider = &provider
			permission = provider.permission
		}
		if !auth.HasProjectPermission(permission, row.ProjectID) {
			return nil, "", screenFail(403, "Data read permission denied / 无数据读取权限")
		}
		selector := source.Selector
		if source.Kind == "workorders" {
			if len(selector.DeviceCodes) > 0 || selector.ProductCode != "" || selector.ProductParameter != "" || len(selector.TagIDs) > 0 {
				return nil, "", screenFail(400, "Filter work orders by result fields / 请通过工单结果字段筛选")
			}
			prepared[source.ID] = item
			continue
		}
		if item.provider != nil && len(selector.DeviceCodes) == 0 && selector.ProductCode == "" && selector.ProductParameter == "" && len(selector.TagIDs) == 0 {
			prepared[source.ID] = item
			continue
		}
		if !auth.HasProjectPermission("device:list", row.ProjectID) {
			return nil, "", screenFail(403, "Device read permission denied / 无设备读取权限")
		}
		if selector.ProductParameter != "" {
			value, ok := input.Parameters[selector.ProductParameter].(string)
			if !ok || value == "" {
				return nil, "", screenFail(400, "Product parameter is required / 需要产品参数")
			}
			selector.ProductCode = value
		}
		query := store.DB.WithContext(r.Context()).Where("tenant_id = ? AND project_id = ?", row.TenantID, row.ProjectID)
		if len(selector.DeviceCodes) > 0 {
			query = query.Where("code IN ?", selector.DeviceCodes)
		}
		if selector.ProductCode != "" {
			query = query.Where("product_code = ?", selector.ProductCode)
		}
		if len(selector.TagIDs) > 0 {
			query = query.Where("code IN (?)", store.DB.Model(&store.DeviceTagBinding{}).Select("device_code").Where("scope_type = ? AND scope_id = ? AND tag_id IN ?", "tenant", fmt.Sprint(row.TenantID), selector.TagIDs))
		}
		var devices []store.Device
		deviceLimit := dataquery.MaxDevices
		if selector.ProductCode != "" && (source.Kind == "devices" || source.Kind == "latest" || source.Kind == "alarms") {
			deviceLimit = screenAssistantContextDeviceBudget
		}
		if err := query.Order("code").Limit(deviceLimit + 1).Find(&devices).Error; err != nil {
			return nil, "", screenFail(503, "Device catalog unavailable / 设备目录暂不可用")
		}
		if len(devices) > deviceLimit {
			return nil, "", dataquery.ErrBudget
		}
		readable := map[string]bool{}
		for _, device := range devices {
			if !canAccessDevice(r, &device) {
				continue
			}
			if source.Kind == "telemetry" || source.Kind == "latest" {
				fields, known := productFields[device.ProductCode]
				if !known {
					fields = map[string]bool{}
					var product store.Product
					if err := store.DB.Where("code = ?", device.ProductCode).First(&product).Error; err != nil || !canAccessProduct(r, &product) {
						if len(selector.DeviceCodes) > 0 {
							return nil, "", screenFail(403, "Product is outside readable scope / 产品不在可读范围")
						}
						// A broad selector spans a whole product or the whole project.
						// A device whose product definition is missing or unreadable
						// cannot contribute a declared field, so it is excluded instead
						// of failing every other device in the same query.
						continue
					}
					for _, property := range loadProductTSLProperties(device.ProductCode) {
						fields[propertyStringField(property, "identifier")] = true
					}
					productFields[device.ProductCode] = fields
					fingerprint = append(fingerprint, product.Code, product.UpdatedAt, product.Config)
				}
				if len(source.Fields) == 0 {
					return nil, "", screenFail(400, "Select telemetry fields / 请选择遥测字段")
				}
				for _, field := range source.Fields {
					if !fields[field] || field == "ts" || field == "device_code" || field == "product_code" || field == "device_name" {
						return nil, "", screenFail(403, "Undeclared or reserved telemetry field / 遥测字段未声明或与保留字段冲突")
					}
				}
			}
			readable[device.Code] = true
			item.devices = append(item.devices, device)
			fingerprint = append(fingerprint, device.Code, device.UpdatedAt)
		}
		for _, code := range selector.DeviceCodes {
			if !readable[code] {
				return nil, "", screenFail(403, "Device outside readable scope / 设备不在可读范围")
			}
		}
		prepared[source.ID] = item
	}
	raw, _ := json.Marshal(fingerprint)
	sum := sha256.Sum256(raw)
	return prepared, hex.EncodeToString(sum[:]), nil
}

func (s *Server) screenDataReader(prepared map[string]screenPreparedSource, scope ScreenDataScope) dataquery.Reader {
	db := store.DB
	return func(ctx context.Context, source dataquery.Source, input dataquery.Input) ([]dataquery.Row, error) {
		item := prepared[source.ID]
		scope.Devices = item.devices
		rows := []dataquery.Row{}
		codes := []string{}
		owners := []tsdb.RecordOwner{}
		devices := map[string]store.Device{}
		for _, device := range item.devices {
			owners = append(owners, tsdb.RecordOwner{DeviceID: device.ID, TenantID: device.TenantID, ProjectID: device.ProjectID})
			codes = append(codes, device.Code)
			devices[device.Code] = device
		}
		if item.provider != nil {
			result, err := item.provider.read(ctx, scope, source, input)
			if err != nil {
				return nil, screenFail(503, "Data provider unavailable / 数据提供器暂不可用")
			}
			return result, nil
		}
		if len(codes) == 0 && source.Kind != "workorders" {
			return rows, nil
		}
		switch source.Kind {
		case "telemetry":
			if s.TSDB == nil {
				return nil, screenFail(503, "History service unavailable / 历史服务暂不可用")
			}
			err := s.TSDB.Scan(ctx, tsdb.ScanRequest{Owners: owners, DeviceCodes: codes, Keys: source.Fields, StartTime: input.From, EndTime: input.To, Limit: dataquery.MaxScanRows}, func(record tsdb.Record) error {
				var payload map[string]interface{}
				if json.Unmarshal(record.Payload, &payload) != nil {
					return errors.New("Invalid stored telemetry / 历史遥测格式无效")
				}
				device := devices[record.DeviceCode]
				entry := dataquery.Row{"ts": record.Ts, "device_code": device.Code, "product_code": device.ProductCode, "device_name": device.Name}
				for _, field := range source.Fields {
					entry[field] = payload[field]
				}
				rows = append(rows, entry)
				return nil
			})
			if err != nil {
				if errors.Is(err, tsdb.ErrScanLimit) {
					return nil, dataquery.ErrBudget
				}
				return nil, screenFail(503, "History scan failed / 历史读取失败")
			}
			return rows, nil
		case "latest", "devices":
			if source.Kind == "latest" && s.DeviceManager == nil {
				return nil, screenFail(503, "Live service unavailable / 实时服务暂不可用")
			}
			for _, device := range item.devices {
				if err := ctx.Err(); err != nil {
					return nil, err
				}
				entry := dataquery.Row{"device_code": device.Code, "product_code": device.ProductCode, "device_name": device.Name, "enabled": device.Enabled}
				if s.DeviceManager != nil {
					snapshot := s.DeviceManager.GetLatestSnapshot(device.Code)
					entry["online"] = snapshot.Status.Online
					if snapshot.StatusReceivedAt == 0 {
						entry["online"] = nil
					}
					entry["ts"] = snapshot.StatusReceivedAt
					if source.Kind == "latest" {
						entry = dataquery.Row{"device_code": device.Code, "product_code": device.ProductCode, "device_name": device.Name}
						var observed int64
						for _, field := range source.Fields {
							sample := snapshot.Samples[field]
							entry[field] = sample.Value
							if sample.ReceivedAt > observed {
								observed = sample.ReceivedAt
							}
						}
						entry["ts"] = observed
					}
				} else {
					entry["online"] = nil
					entry["ts"] = nil
				}
				rows = append(rows, entry)
			}
			return rows, nil
		case "events":
			codes := make([]string, 0, len(item.devices))
			for _, device := range item.devices {
				codes = append(codes, device.Code)
			}
			if len(codes) == 0 {
				return rows, nil
			}
			events, err := s.screenEventRows(ctx, &store.IOCScreen{TenantID: scope.TenantID, ProjectID: scope.ProjectID}, screen.Binding{Kind: "eventList", DeviceCodes: codes}, input.From, input.To, 200)
			if err != nil {
				return nil, err
			}
			for _, event := range events {
				parameters, _ := json.Marshal(event["parameters"])
				rows = append(rows, dataquery.Row{"ts": event["ts"], "device_code": event["deviceCode"], "device_name": event["deviceName"], "event_zh": event["eventZh"], "event_en": event["eventEn"], "event_type": event["eventType"], "parameters": string(parameters)})
			}
			return rows, nil
		case "alarms":
			query := db.WithContext(ctx).Model(&store.AlarmInstance{}).Select("id", "public_id", "evidence_snapshot", "title", "severity", "condition_status", "handling_status", "last_occurred_at", "work_order_public_id").Where("tenant_id = ? AND project_id = ? AND last_occurred_at >= ? AND last_occurred_at < ?", scope.TenantID, scope.ProjectID, time.UnixMilli(input.From), time.UnixMilli(input.To)).Order("id").Limit(dataquery.MaxRows + 1)
			cursor, err := query.Rows()
			if err != nil {
				return nil, screenFail(503, "Alarm service unavailable / 告警服务暂不可用")
			}
			defer cursor.Close()
			count, bytesRead := 0, 0
			for cursor.Next() {
				var alarm store.AlarmInstance
				if err := db.ScanRows(cursor, &alarm); err != nil {
					return nil, screenFail(503, "Alarm service unavailable / 告警服务暂不可用")
				}
				count++
				bytesRead += len(alarm.EvidenceSnapshot) + len(alarm.Title)
				if count > dataquery.MaxRows || bytesRead > dataquery.MaxBytes {
					return nil, dataquery.ErrBudget
				}
				var evidence struct {
					DeviceCode string `json:"device_code"`
				}
				if json.Unmarshal([]byte(alarm.EvidenceSnapshot), &evidence) != nil {
					continue
				}
				if _, ok := devices[evidence.DeviceCode]; !ok {
					continue
				}
				if alarm.LastOccurredAt == nil {
					continue
				}
				rows = append(rows, dataquery.Row{"id": alarm.PublicID, "device_code": evidence.DeviceCode, "title": alarm.Title, "severity": alarm.Severity, "condition_status": alarm.ConditionStatus, "handling_status": alarm.HandlingStatus, "work_order_id": alarm.WorkOrderPublicID, "ts": alarm.LastOccurredAt.UnixMilli()})
			}
			if cursor.Err() != nil {
				return nil, screenFail(503, "Alarm service unavailable / 告警服务暂不可用")
			}
			return rows, nil
		case "workorders":
			var orders []store.WorkOrder
			if err := db.WithContext(ctx).Select("id", "public_id", "code", "title", "priority", "status", "source_type", "source_id", "created_at", "closed_at").Where("tenant_id = ? AND project_id = ? AND created_at >= ? AND created_at < ?", scope.TenantID, scope.ProjectID, time.UnixMilli(input.From), time.UnixMilli(input.To)).Order("id").Limit(dataquery.MaxRows + 1).Find(&orders).Error; err != nil {
				return nil, screenFail(503, "Work order service unavailable / 工单服务暂不可用")
			}
			if len(orders) > dataquery.MaxRows {
				return nil, dataquery.ErrBudget
			}
			for _, order := range orders {
				var closed interface{}
				if order.ClosedAt != nil {
					closed = order.ClosedAt.UnixMilli()
				}
				rows = append(rows, dataquery.Row{"id": order.PublicID, "code": order.Code, "title": order.Title, "priority": order.Priority, "status": order.Status, "source_type": order.SourceType, "source_id": order.SourceID, "ts": order.CreatedAt.UnixMilli(), "closed_at": closed})
			}
			return rows, nil
		}
		return nil, screenFail(400, "Unsupported source / 不支持的数据源")
	}
}

func (s *Server) screenExecuteData(r *ghttp.Request, row *store.IOCScreen, plan dataquery.Plan, parameters map[string]interface{}, from, to int64, contexts ...context.Context) (dataquery.Result, error) {
	ctx := r.Context()
	if len(contexts) > 0 {
		ctx = contexts[0]
	}
	if err := ctx.Err(); err != nil {
		return dataquery.Result{}, err
	}
	now := time.Now().UnixMilli()
	if to == 0 {
		to = now / 5000 * 5000
	} // deterministic refresh boundary, also returned in meta
	input, err := dataquery.Resolve(plan, parameters, from, to, now)
	if err != nil {
		return dataquery.Result{}, err
	}
	prepared, identity, err := screenPrepareData(r, row, plan, input)
	if err != nil {
		return dataquery.Result{}, err
	}
	keyJSON, _ := json.Marshal([]interface{}{fmt.Sprintf("%p", s), identity, plan, input.From, input.To, input.Parameters})
	hash := sha256.Sum256(keyJSON)
	scope := ScreenDataScope{TenantID: row.TenantID, ProjectID: row.ProjectID, UserID: requestAuthContext(r).UserID, AppID: requestAuthContext(r).AppID}
	reader := s.screenDataReader(prepared, scope)
	return screenDataCache.Do(ctx, hex.EncodeToString(hash[:]), 5*time.Second, func(ctx context.Context) (dataquery.Result, error) {
		return dataquery.Execute(ctx, plan, input, reader)
	})
}

func (s *Server) handleScreenDataPreview(r *ghttp.Request) {
	row, err := screenLoad(r, "screen:edit")
	if err != nil {
		screenWriteError(r, err)
		return
	}
	var body struct {
		Plan       dataquery.Plan         `json:"plan"`
		Parameters map[string]interface{} `json:"parameters"`
		From       int64                  `json:"from"`
		To         int64                  `json:"to"`
	}
	if err = screenReadJSON(r, &body); err != nil {
		screenWriteError(r, err)
		return
	}
	result, err := s.screenExecuteData(r, row, body.Plan, body.Parameters, body.From, body.To)
	if err != nil {
		screenWriteError(r, err)
		return
	}
	screenWriteData(r, result)
}

func (s *Server) handleScreenRuntimeData(r *ghttp.Request) {
	row, doc, revision, err := screenAuthorizedDocument(r)
	if err != nil {
		screenWriteError(r, err)
		return
	}
	var body struct {
		BindingIDs []string                          `json:"bindingIds"`
		Parameters map[string]map[string]interface{} `json:"parameters"`
		From       int64                             `json:"from"`
		To         int64                             `json:"to"`
	}
	if err = screenReadJSON(r, &body); err != nil {
		screenWriteError(r, err)
		return
	}
	if len(body.BindingIDs) == 0 || len(body.BindingIDs) > 24 || len(body.Parameters) > 24 {
		screenWriteError(r, dataquery.ErrBudget)
		return
	}
	results := map[string]interface{}{}
	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()
	now := time.Now().UnixMilli()
	for _, id := range body.BindingIDs {
		if _, done := results[id]; done {
			continue
		}
		binding, ok := doc.Bindings[id]
		if !ok || binding.Kind != "dataset" {
			screenWriteError(r, screenFail(400, "Unknown dataset binding / 未知数据集绑定"))
			return
		}
		plan, queryErr := screenDatasetPlan(row, binding)
		params := map[string]interface{}{}
		for k, v := range binding.Parameters {
			params[k] = v
		}
		for k, v := range body.Parameters[id] {
			params[k] = v
		}
		var result dataquery.Result
		if queryErr == nil {
			result, queryErr = s.screenExecuteData(r, row, plan, params, body.From, body.To, ctx)
		}
		if queryErr != nil {
			quality := "error"
			var apiErr screenAPIError
			if errors.As(queryErr, &apiErr) && apiErr.status == 403 {
				quality = "access_denied"
			}
			results[id] = map[string]interface{}{"value": nil, "quality": quality, "message": queryErr.Error(), "receivedAt": now, "parameters": plan.Parameters}
			continue
		}
		results[id] = map[string]interface{}{"value": result.Rows, "quality": result.Meta.Quality, "receivedAt": result.Meta.GeneratedAt, "meta": result.Meta, "fields": result.Fields, "parameters": plan.Parameters}
	}
	screenWriteData(r, map[string]interface{}{"revision": revision, "bindings": results})
}

func (s *Server) handleScreenDatasets(r *ghttp.Request) {
	row, err := screenLoad(r, "screen:edit")
	if err != nil {
		screenWriteError(r, err)
		return
	}
	var datasets []store.IOCDataSet
	if err = store.DB.Where("tenant_id = ? AND project_id = ?", row.TenantID, row.ProjectID).Order("updated_at DESC, id DESC").Limit(101).Find(&datasets).Error; err != nil {
		screenWriteError(r, err)
		return
	}
	more := len(datasets) > 100
	if more {
		datasets = datasets[:100]
	}
	screenWriteData(r, map[string]interface{}{"datasets": datasets, "hasMore": more})
}
func (s *Server) handleScreenDatasetVersion(r *ghttp.Request) {
	row, err := screenLoad(r, "screen:edit")
	if err != nil {
		screenWriteError(r, err)
		return
	}
	ref := screen.DatasetRef{ID: r.GetRouter("datasetId").Uint(), Version: r.GetRouter("version").Uint64()}
	plan, err := screenDatasetPlan(row, screen.Binding{Dataset: &ref})
	if err != nil {
		screenWriteError(r, err)
		return
	}
	screenWriteData(r, map[string]interface{}{"plan": plan, "dataset": ref})
}
func (s *Server) handleScreenDatasetSave(r *ghttp.Request) {
	row, err := screenLoad(r, "screen:edit")
	if err != nil {
		screenWriteError(r, err)
		return
	}
	var body struct {
		ID          uint                   `json:"id"`
		BaseVersion uint64                 `json:"baseVersion"`
		Name        string                 `json:"name"`
		Plan        dataquery.Plan         `json:"plan"`
		Parameters  map[string]interface{} `json:"parameters"`
	}
	if err = screenReadJSON(r, &body); err != nil {
		screenWriteError(r, err)
		return
	}
	body.Name = strings.TrimSpace(body.Name)
	if body.Name == "" || len([]rune(body.Name)) > 128 {
		screenWriteError(r, screenFail(400, "Dataset name is required (1–128 characters) / 数据集名称需为1至128个字符"))
		return
	}
	input, err := dataquery.Resolve(body.Plan, body.Parameters, 0, 0, time.Now().UnixMilli())
	if err == nil {
		_, _, err = screenPrepareData(r, row, body.Plan, input)
	}
	if err != nil {
		screenWriteError(r, err)
		return
	}
	raw, _ := json.Marshal(body.Plan)
	var dataset store.IOCDataSet
	err = store.DB.Transaction(func(tx *gorm.DB) error {
		if body.ID == 0 {
			dataset = store.IOCDataSet{TenantID: row.TenantID, ProjectID: row.ProjectID, Name: body.Name, Version: 1, CreatedBy: requestAuthContext(r).UserID}
			if err := tx.Create(&dataset).Error; err != nil {
				return err
			}
		} else {
			update := tx.Model(&store.IOCDataSet{}).Where("id = ? AND tenant_id = ? AND project_id = ? AND version = ?", body.ID, row.TenantID, row.ProjectID, body.BaseVersion).Updates(map[string]interface{}{"name": body.Name, "version": body.BaseVersion + 1})
			if update.Error != nil {
				return update.Error
			}
			if update.RowsAffected != 1 {
				return store.ErrScreenConflict
			}
			if err := tx.First(&dataset, body.ID).Error; err != nil {
				return err
			}
		}
		return tx.Create(&store.IOCDataSetVersion{DatasetID: dataset.ID, Version: dataset.Version, PlanJSON: string(raw)}).Error
	})
	if err != nil {
		screenWriteError(r, err)
		return
	}
	screenWriteData(r, dataset)
}
