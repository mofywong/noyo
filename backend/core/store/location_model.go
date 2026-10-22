package store

import (
	"errors"
	"time"

	"gorm.io/gorm"
)

// 定位来源常量。同一台设备同一时刻只有一个来源，写入路径互斥。
const (
	LocationSourceManual    = "manual"    // 人工标注（输入经纬度 / 地图选点）
	LocationSourceTelemetry = "telemetry" // 设备通过物模型经纬度语义属性上报
)

// 围栏形状与规则类型。
const (
	FenceKindCircle  = "circle"
	FenceKindPolygon = "polygon"

	FenceRuleForbid = "forbid"  // 禁入：进入即告警
	FenceRuleKeepIn = "keep_in" // 必须在内：离开即告警
	FenceRuleNotify = "notify"  // 通知：进出都提示（不产生告警实例）

	// 围栏来源：手绘 / 按行政区名称取边界。只影响编辑方式，不影响判定。
	FenceSourceManual = "manual"
	FenceSourceRegion = "region"
)

// 围栏判定状态。unknown 表示定位缺失或从未判定过。
const (
	FenceStateInside  = "inside"
	FenceStateOutside = "outside"
	FenceStateUnknown = "unknown"
)

// DeviceLocation 是设备位置的持久化快照，与设备 1:1。
//
// 为什么必须落库：平台的运行时最新值缓存（DeviceRuntime.Properties）是纯内存态，
// 平台重启后地图上的动态设备会全部消失；静态设备则永远不会进入该缓存。
//
// 主键用 device_code 而非自增 id —— 与 DeviceTagBinding / StoredAsset 的既有约定一致，
// 且 API 层直接以设备编码寻址，不需要二次解析。
//
// 本表不进配置同步：位置是运行时数据，走属性上行链路，各端各自维护快照。
type DeviceLocation struct {
	DeviceCode string `gorm:"primaryKey;size:64" json:"device_code"`

	Longitude float64  `gorm:"not null" json:"longitude"` // WGS-84 真实度数
	Latitude  float64  `gorm:"not null" json:"latitude"`  // WGS-84 真实度数
	Altitude  *float64 `json:"altitude,omitempty"`

	// AccuracyM 是判定引擎的必需输入，不是装饰：它决定"可信圆"大小。
	// manual 来源没有该值，判定时按 ManualAccuracyDefaultM 保守膨胀。
	AccuracyM  *float64 `json:"accuracy_m,omitempty"`
	SpeedMPS   *float64 `json:"speed_mps,omitempty"`
	HeadingDeg *float64 `json:"heading_deg,omitempty"`

	FixAt      int64 `gorm:"index;not null" json:"fix_at"`      // 位置生效时刻（遥测=设备侧；手动=录入时刻）
	ReceivedAt int64 `gorm:"index;not null" json:"received_at"` // 平台写入时刻

	Source    string `gorm:"size:16;not null;index" json:"source"` // manual | telemetry
	UpdatedBy string `gorm:"size:128" json:"updated_by,omitempty"` // source=manual 时记录操作者（审计）

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ManualAccuracyDefaultM 是手动标注位置的保守默认精度半径（米）。
//
// 人工在地图上点选的落点没有 GPS 精度可言，但围栏判定不能把它当作零误差，
// 否则人眼偏差几米就会造成误报。取 30 m 作为保守值。
const ManualAccuracyDefaultM = 30.0

// GetDeviceLocation 读取某设备的位置快照。不存在时返回 (nil, nil)。
func GetDeviceLocation(deviceCode string) (*DeviceLocation, error) {
	if DB == nil || deviceCode == "" {
		return nil, nil
	}
	var loc DeviceLocation
	err := DB.Where("device_code = ?", deviceCode).First(&loc).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &loc, nil
}

// UpsertDeviceLocation 写入或覆盖位置快照。
func UpsertDeviceLocation(loc *DeviceLocation) error {
	if DB == nil || loc == nil || loc.DeviceCode == "" {
		return errors.New("device location requires a device code")
	}
	now := time.Now()
	if loc.ReceivedAt == 0 {
		loc.ReceivedAt = now.UnixMilli()
	}
	if loc.FixAt == 0 {
		loc.FixAt = loc.ReceivedAt
	}
	return DB.Save(loc).Error
}

// DeleteDeviceLocation 删除位置快照（设备删除时级联清理）。
func DeleteDeviceLocation(deviceCode string) error {
	if DB == nil || deviceCode == "" {
		return nil
	}
	return DB.Where("device_code = ?", deviceCode).Delete(&DeviceLocation{}).Error
}

// DeleteDeviceLocationTx 在事务内删除位置快照。
func DeleteDeviceLocationTx(tx *gorm.DB, deviceCode string) error {
	if tx == nil || deviceCode == "" {
		return nil
	}
	return tx.Where("device_code = ?", deviceCode).Delete(&DeviceLocation{}).Error
}

// ListDeviceLocationsByCodes 批量读取位置快照，返回 device_code -> location。
// 地图页一次拉全量时用它做一次性装配，避免 N+1。
func ListDeviceLocationsByCodes(codes []string) (map[string]*DeviceLocation, error) {
	out := make(map[string]*DeviceLocation)
	if DB == nil || len(codes) == 0 {
		return out, nil
	}
	var rows []DeviceLocation
	if err := DB.Where("device_code IN ?", codes).Find(&rows).Error; err != nil {
		return nil, err
	}
	for i := range rows {
		loc := rows[i]
		out[loc.DeviceCode] = &loc
	}
	return out, nil
}

// LoadDeviceNames 批量读取设备名称，返回 device_code -> name。
//
// 大屏的地图与告警面板都需要「名称」而不是编码：编码是运维标识
// （DEV_06_FAS_BACNET_GW），直接贴到墙上没人认得出是哪个点位。
// 名称取不到时由调用方回退到编码，不要在这里编造占位符。
func LoadDeviceNames(codes []string) (map[string]string, error) {
	out := make(map[string]string)
	if DB == nil || len(codes) == 0 {
		return out, nil
	}
	var rows []struct {
		Code string
		Name string
	}
	if err := DB.Model(&Device{}).Select("code, name").Where("code IN ?", codes).Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		if row.Name != "" {
			out[row.Code] = row.Name
		}
	}
	return out, nil
}

// Geofence 是地理围栏定义。坐标一律 WGS-84。
//
// 注意：围栏当前只在本端持久化，尚未接入 cascade 配置同步链路。
// 接入同步意味着新增一个同步实体类型（Change 记录器 / Intent / Projection /
// RuntimeApply / 网关状态机 / 删除保护全都要接），属于独立工作量，见方案文档 §8.1。
type Geofence struct {
	gorm.Model
	TenantID  uint   `gorm:"index;not null;default:0" json:"tenant_id"`
	ProjectID uint   `gorm:"index;not null;default:0" json:"project_id"`
	Name      string `gorm:"size:128;not null" json:"name"`
	Kind      string `gorm:"size:16;not null" json:"kind"` // circle | polygon

	// circle
	CenterLng *float64 `json:"center_lng,omitempty"`
	CenterLat *float64 `json:"center_lat,omitempty"`
	RadiusM   *float64 `json:"radius_m,omitempty"`

	// polygon：GeoJSON geometry（WGS-84）
	Geometry string `json:"geometry,omitempty"`

	// 包围盒冗余，供 R*Tree 预筛与快速视口过滤
	BboxMinLng *float64 `json:"bbox_min_lng,omitempty"`
	BboxMinLat *float64 `json:"bbox_min_lat,omitempty"`
	BboxMaxLng *float64 `json:"bbox_max_lng,omitempty"`
	BboxMaxLat *float64 `json:"bbox_max_lat,omitempty"`

	RuleType    string `gorm:"size:16;not null;default:notify" json:"rule_type"` // forbid | keep_in | notify
	EnterDwellS int    `gorm:"not null;default:0" json:"enter_dwell_s"`          // 进入迟滞
	ExitDwellS  int    `gorm:"not null;default:0" json:"exit_dwell_s"`           // 离开迟滞
	Enabled     bool   `gorm:"not null;default:true" json:"enabled"`
	Color       string `gorm:"size:16" json:"color,omitempty"`
	CreatedBy   string `gorm:"size:128" json:"created_by,omitempty"`

	// Source 说明这条围栏是「手绘」还是「按行政区名称取边界」得来的。
	// 只影响 UI 的编辑方式（地区围栏不能拖顶点，只能重新选地区），不影响判定。
	Source string `gorm:"size:16;not null;default:manual" json:"source"` // manual | region
	// RegionName / RegionLevel 保留地区围栏的来源可追溯性：
	// 名称是用户输入时命中的地区名，级别是 province/city/district/town 等。
	RegionName  string `gorm:"size:191" json:"region_name,omitempty"`
	RegionLevel string `gorm:"size:32" json:"region_level,omitempty"`

	// AssignedDevices 是绑定到本围栏的设备编码，**不落库**（来自 GeofenceDevice 表）。
	// 空表示不限制 —— 沿用旧行为，对全部已定位设备生效。
	AssignedDevices []string `gorm:"-" json:"assigned_devices,omitempty"`
}

// GeofenceDevice 把设备绑定到一个围栏。
//
// 绑定是围栏的**适用范围**，不是判定结果的缓存：
//   - 围栏存在绑定 → 只对这批设备判定；
//   - 围栏没有任何绑定 → 对全部已定位设备判定（兼容既有数据，避免升级后静默失效）。
//
// 与 Geofence 一样只在本端持久化，尚未接入 cascade 配置同步。
type GeofenceDevice struct {
	FenceID    uint      `gorm:"primaryKey;autoIncrement:false" json:"fence_id"`
	DeviceCode string    `gorm:"primaryKey;size:64" json:"device_code"`
	TenantID   uint      `gorm:"index;not null;default:0" json:"tenant_id"`
	ProjectID  uint      `gorm:"index;not null;default:0" json:"project_id"`
	CreatedAt  time.Time `json:"created_at"`
}

// TableName 固定表名，避免 gorm 复数化规则随版本变化。
func (GeofenceDevice) TableName() string { return "geofence_devices" }

// ListGeofenceDeviceMap 返回 fence_id -> device_code 集合，供判定引擎一次性装配。
func ListGeofenceDeviceMap() (map[uint]map[string]bool, error) {
	out := make(map[uint]map[string]bool)
	if DB == nil {
		return out, nil
	}
	var rows []GeofenceDevice
	if err := DB.Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		set, ok := out[row.FenceID]
		if !ok {
			set = make(map[string]bool)
			out[row.FenceID] = set
		}
		set[row.DeviceCode] = true
	}
	return out, nil
}

// ListGeofenceDevices 返回某围栏绑定的设备编码（按绑定时间升序）。
func ListGeofenceDevices(fenceID uint) ([]string, error) {
	if DB == nil || fenceID == 0 {
		return nil, nil
	}
	var rows []GeofenceDevice
	if err := DB.Where("fence_id = ?", fenceID).Order("created_at ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.DeviceCode)
	}
	return out, nil
}

// ReplaceGeofenceDevices 用给定集合整体替换某围栏的绑定关系，并清掉被解绑设备的判定状态。
//
// 为什么是整体替换而不是增量：前端提交的是"最终勾选结果"，增量语义会在并发
// 编辑下产生双方各自以为对方删除的项。清理判定状态是必要的 —— 留着旧状态会让
// 已解绑的设备在违规列表里阴魂不散。
func ReplaceGeofenceDevices(fenceID uint, tenantID, projectID uint, codes []string) error {
	if DB == nil || fenceID == 0 {
		return errors.New("fence id is required")
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		var existing []GeofenceDevice
		if err := tx.Where("fence_id = ?", fenceID).Find(&existing).Error; err != nil {
			return err
		}
		keep := make(map[string]bool, len(codes))
		for _, code := range codes {
			if code != "" {
				keep[code] = true
			}
		}
		removed := make([]string, 0, len(existing))
		for _, row := range existing {
			if keep[row.DeviceCode] {
				continue
			}
			removed = append(removed, row.DeviceCode)
			if err := tx.Where("fence_id = ? AND device_code = ?", fenceID, row.DeviceCode).Delete(&GeofenceDevice{}).Error; err != nil {
				return err
			}
		}
		if len(removed) > 0 {
			if err := tx.Where("fence_id = ? AND device_code IN ?", fenceID, removed).Delete(&DeviceFenceState{}).Error; err != nil {
				return err
			}
		}
		have := make(map[string]bool, len(existing))
		for _, row := range existing {
			have[row.DeviceCode] = true
		}
		for code := range keep {
			if have[code] {
				continue
			}
			row := GeofenceDevice{FenceID: fenceID, DeviceCode: code, TenantID: tenantID, ProjectID: projectID, CreatedAt: time.Now()}
			if err := tx.Create(&row).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// DeleteGeofenceDevicesForDevice 清理某设备的全部围栏绑定（设备删除时级联）。
func DeleteGeofenceDevicesForDevice(tx *gorm.DB, deviceCode string) error {
	if tx == nil || deviceCode == "" {
		return nil
	}
	return tx.Where("device_code = ?", deviceCode).Delete(&GeofenceDevice{}).Error
}

// DeleteGeofenceDevices 清理某围栏的全部绑定（围栏删除时级联）。
func DeleteGeofenceDevices(tx *gorm.DB, fenceID uint) error {
	if tx == nil || fenceID == 0 {
		return nil
	}
	return tx.Where("fence_id = ?", fenceID).Delete(&GeofenceDevice{}).Error
}

// DeviceFenceState 是围栏判定状态。**必须有状态**——否则 GPS 边界抖动会让告警疯狂翻转。
type DeviceFenceState struct {
	DeviceCode string `gorm:"primaryKey;size:64" json:"device_code"`
	FenceID    uint   `gorm:"primaryKey" json:"fence_id"`

	State string `gorm:"size:16;not null;default:unknown" json:"state"` // inside | outside | unknown
	Since int64  `gorm:"not null;default:0" json:"since"`               // 当前状态起始时刻

	PendingState string `gorm:"size:16" json:"pending_state,omitempty"` // 待确认目标态（迟滞）
	PendingSince int64  `gorm:"not null;default:0" json:"pending_since,omitempty"`

	LastEvalAt int64 `gorm:"not null;default:0" json:"last_eval_at"`
	LastFixAt  int64 `gorm:"not null;default:0" json:"last_fix_at"`

	UpdatedAt time.Time `json:"updated_at"`
}

// ListGeofences 列出某租户/项目下的围栏。传 0 表示不过滤该维度。
func ListGeofences(tenantID, projectID uint) ([]Geofence, error) {
	if DB == nil {
		return nil, nil
	}
	q := DB.Model(&Geofence{})
	if tenantID > 0 {
		q = q.Where("tenant_id = ?", tenantID)
	}
	if projectID > 0 {
		q = q.Where("project_id = ?", projectID)
	}
	var rows []Geofence
	if err := q.Order("id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// ListEnabledGeofences 列出全部启用的围栏（判定引擎全量评估用）。
func ListEnabledGeofences() ([]Geofence, error) {
	if DB == nil {
		return nil, nil
	}
	var rows []Geofence
	if err := DB.Where("enabled = ?", true).Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// GetGeofence 按 ID 读取围栏。
func GetGeofence(id uint) (*Geofence, error) {
	if DB == nil {
		return nil, nil
	}
	var row Geofence
	err := DB.First(&row, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// CreateGeofence 新建围栏。
func CreateGeofence(f *Geofence) error {
	if DB == nil || f == nil {
		return errors.New("geofence is nil")
	}
	return DB.Create(f).Error
}

// UpdateGeofence 全量更新围栏。
func UpdateGeofence(f *Geofence) error {
	if DB == nil || f == nil {
		return errors.New("geofence is nil")
	}
	return DB.Save(f).Error
}

// DeleteGeofence 删除围栏、它的设备绑定与全部判定状态（避免孤儿状态导致误判）。
func DeleteGeofence(id uint) error {
	if DB == nil {
		return nil
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := DeleteGeofenceDevices(tx, id); err != nil {
			return err
		}
		if err := tx.Where("fence_id = ?", id).Delete(&DeviceFenceState{}).Error; err != nil {
			return err
		}
		return tx.Delete(&Geofence{}, id).Error
	})
}

// GetDeviceFenceState 读取单条判定状态。不存在返回 (nil, nil)。
func GetDeviceFenceState(deviceCode string, fenceID uint) (*DeviceFenceState, error) {
	if DB == nil {
		return nil, nil
	}
	var row DeviceFenceState
	err := DB.Where("device_code = ? AND fence_id = ?", deviceCode, fenceID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// UpsertDeviceFenceState 写入判定状态。
func UpsertDeviceFenceState(st *DeviceFenceState) error {
	if DB == nil || st == nil {
		return errors.New("fence state is nil")
	}
	return DB.Save(st).Error
}

// ListDeviceFenceStates 列出某设备（或全部设备）的判定状态。
func ListDeviceFenceStates(deviceCode string) ([]DeviceFenceState, error) {
	if DB == nil {
		return nil, nil
	}
	q := DB.Model(&DeviceFenceState{})
	if deviceCode != "" {
		q = q.Where("device_code = ?", deviceCode)
	}
	var rows []DeviceFenceState
	if err := q.Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// ListPendingFenceStates 列出所有"待确认"的判定状态（迟滞未到期或未推进）。
//
// 为什么必须能查全表：时间迟滞是**时间驱动**的，但位置上报是**事件驱动**的。
// 一次人工标注只产生一次新位置，之后没有任何请求会再来推动状态机，
// 于是 pending 会永远悬着 —— 表现为"位置改到围栏外好几分钟了也不告警"。
func ListPendingFenceStates() ([]DeviceFenceState, error) {
	if DB == nil {
		return nil, nil
	}
	var rows []DeviceFenceState
	if err := DB.Where("pending_state <> '' AND pending_state IS NOT NULL").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// DeleteFenceStatesForDevice 清理某设备的全部围栏判定状态（设备删除时级联）。
func DeleteFenceStatesForDevice(tx *gorm.DB, deviceCode string) error {
	if tx == nil || deviceCode == "" {
		return nil
	}
	return tx.Where("device_code = ?", deviceCode).Delete(&DeviceFenceState{}).Error
}
