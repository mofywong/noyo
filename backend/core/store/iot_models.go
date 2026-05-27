package store

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"
)

// Product represents a type of device (e.g., "Smart Meter Model X")
type Product struct {
	gorm.Model
	TenantID     uint   `gorm:"index;not null;default:0" json:"tenant_id"`
	ProjectID    uint   `gorm:"index;not null;default:0" json:"project_id"`
	Code         string `gorm:"uniqueIndex;size:64;not null" json:"code"`
	Name         string `gorm:"size:128;not null" json:"name"`
	ProtocolName string `json:"protocol_name"` // e.g. "Modbus", "OPC-UA"
	// Config stores protocol-specific product configuration (e.g. Polling Groups, TSL)
	// Stored as JSON string
	Config string `json:"config"`
}

// Device represents a physical instance
type Device struct {
	gorm.Model
	TenantID    uint   `gorm:"index;not null;default:0" json:"tenant_id"`
	ProjectID   uint   `gorm:"index;not null;default:0" json:"project_id"`
	Code        string `gorm:"uniqueIndex;size:64;not null" json:"code"`
	Name        string `gorm:"size:128;not null" json:"name"`
	ProductCode string `gorm:"index;not null" json:"product_code"`
	ParentCode  string `gorm:"index" json:"parent_code"`
	Enabled     bool   `json:"enabled"`
	// Config stores device-specific connection parameters (e.g. IP, Port, SlaveID)
	// Stored as JSON string
	Config string `json:"config"`
}

// AccessScope identifies the data boundary for tag visibility.
// Current community runtime uses global/global; future RBAC can replace this
// with organization, project, role, or user scopes without changing tag APIs.
type AccessScope struct {
	Type string
	ID   string
}

func GlobalAccessScope() AccessScope {
	return AccessScope{Type: "global", ID: "global"}
}

func normalizeAccessScope(scope AccessScope) AccessScope {
	scope.Type = strings.TrimSpace(scope.Type)
	scope.ID = strings.TrimSpace(scope.ID)
	if scope.Type == "" {
		scope.Type = "global"
	}
	if scope.ID == "" {
		scope.ID = "global"
	}
	return scope
}

// DeviceTag is a user-managed label that can be attached to many devices.
type DeviceTag struct {
	gorm.Model
	ScopeType   string `gorm:"size:32;not null;default:global;uniqueIndex:idx_device_tags_scope_name" json:"scope_type"`
	ScopeID     string `gorm:"size:128;not null;default:global;uniqueIndex:idx_device_tags_scope_name" json:"scope_id"`
	Name        string `gorm:"size:64;not null;uniqueIndex:idx_device_tags_scope_name" json:"name"`
	Color       string `gorm:"size:16;not null;default:#0d6efd" json:"color"`
	Icon        string `gorm:"size:64;not null;default:bi-tag" json:"icon"`
	Description string `json:"description"`
}

// DeviceTagBinding stores the many-to-many relation between devices and tags.
type DeviceTagBinding struct {
	gorm.Model
	ScopeType  string `gorm:"size:32;not null;default:global;uniqueIndex:idx_device_tag_bindings_scope_device_tag;index:idx_device_tag_bindings_tag"`
	ScopeID    string `gorm:"size:128;not null;default:global;uniqueIndex:idx_device_tag_bindings_scope_device_tag;index:idx_device_tag_bindings_tag"`
	DeviceCode string `gorm:"size:128;not null;uniqueIndex:idx_device_tag_bindings_scope_device_tag;index" json:"device_code"`
	TagID      uint   `gorm:"not null;uniqueIndex:idx_device_tag_bindings_scope_device_tag;index:idx_device_tag_bindings_tag" json:"tag_id"`
}

type DeviceTagWithCount struct {
	DeviceTag
	DeviceCount int64 `json:"device_count"`
}

// --- Data Access Methods ---

func GetProduct(code string) (*Product, error) {
	var p Product
	result := DB.Where("code = ?", code).First(&p)
	if result.Error != nil {
		return nil, result.Error
	}
	return &p, nil
}

// ListProducts retrieves products with pagination
func ListProducts(page, pageSize int, tenantID, projectID uint) ([]Product, int64, error) {
	var products []Product
	var total int64

	db := DB.Model(&Product{})
	if tenantID > 0 {
		db = db.Where("tenant_id = ?", tenantID)
	}
	if projectID > 0 {
		db = db.Where("project_id = ?", projectID)
	}

	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if page > 0 && pageSize > 0 {
		offset := (page - 1) * pageSize
		db = db.Offset(offset).Limit(pageSize)
	}

	result := db.Order("created_at desc").Find(&products)
	return products, total, result.Error
}

func SaveProduct(p *Product) error {
	var existing Product
	err := DB.Unscoped().Where("code = ?", p.Code).First(&existing).Error
	if err == nil {
		p.ID = existing.ID
		p.CreatedAt = existing.CreatedAt // Preserve original CreatedAt
		p.DeletedAt = gorm.DeletedAt{}   // Restore if it was soft-deleted
		return DB.Unscoped().Save(p).Error
	}
	return DB.Create(p).Error
}

func GetDevice(code string) (*Device, error) {
	var d Device
	result := DB.Where("code = ?", code).First(&d)
	if result.Error != nil {
		return nil, result.Error
	}
	return &d, nil
}

// ListDevices retrieves devices with pagination
func ListDevices(page, pageSize int, tenantID, projectID uint, allowedProjectIDs ...[]uint) ([]Device, int64, error) {
	var devices []Device
	var total int64

	db := DB.Model(&Device{})
	if tenantID > 0 {
		db = db.Where("tenant_id = ?", tenantID)
	}
	if projectID > 0 {
		db = db.Where("project_id = ?", projectID)
	} else if len(allowedProjectIDs) > 0 {
		ids := allowedProjectIDs[0]
		if len(ids) == 0 {
			return []Device{}, 0, nil
		}
		db = db.Where("project_id IN ?", ids)
	}

	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if page > 0 && pageSize > 0 {
		offset := (page - 1) * pageSize
		db = db.Offset(offset).Limit(pageSize)
	}

	result := db.Order("created_at desc").Find(&devices)
	return devices, total, result.Error
}

func ListDevicesByParent(parentCode string) ([]Device, error) {
	var devices []Device
	result := DB.Where("parent_code = ?", parentCode).Find(&devices)
	return devices, result.Error
}

func ListDevicesByProduct(productCode string) ([]Device, error) {
	var devices []Device
	result := DB.Where("product_code = ?", productCode).Find(&devices)
	return devices, result.Error
}

func SaveDevice(d *Device) error {
	var existing Device
	err := DB.Unscoped().Where("code = ?", d.Code).First(&existing).Error
	if err == nil {
		d.ID = existing.ID
		d.CreatedAt = existing.CreatedAt // Preserve original CreatedAt
		d.DeletedAt = gorm.DeletedAt{}   // Restore if it was soft-deleted
		return DB.Unscoped().Save(d).Error
	}
	return DB.Create(d).Error
}

func DeleteDevice(code string) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		var device Device
		if err := tx.Where("code = ?", code).First(&device).Error; err != nil {
			return err
		}

		if err := tx.Unscoped().Where("device_code = ?", code).Delete(&DeviceTagBinding{}).Error; err != nil {
			return err
		}

		newCode := fmt.Sprintf("%s_del_%d", device.Code, time.Now().Unix())

		// Rename code to release unique constraint
		if err := tx.Model(&device).Update("code", newCode).Error; err != nil {
			return err
		}

		// Soft delete
		return tx.Delete(&device).Error
	})
}

func ListDeviceTags(scope AccessScope) ([]DeviceTagWithCount, error) {
	scope = normalizeAccessScope(scope)

	var tags []DeviceTag
	if err := DB.Where("scope_type = ? AND scope_id = ?", scope.Type, scope.ID).
		Order("updated_at desc").
		Find(&tags).Error; err != nil {
		return nil, err
	}

	type countRow struct {
		TagID uint
		Count int64
	}
	var counts []countRow
	if err := DB.Model(&DeviceTagBinding{}).
		Select("tag_id, COUNT(*) as count").
		Where("scope_type = ? AND scope_id = ?", scope.Type, scope.ID).
		Group("tag_id").
		Scan(&counts).Error; err != nil {
		return nil, err
	}

	countByTag := make(map[uint]int64, len(counts))
	for _, row := range counts {
		countByTag[row.TagID] = row.Count
	}

	result := make([]DeviceTagWithCount, 0, len(tags))
	for _, tag := range tags {
		result = append(result, DeviceTagWithCount{
			DeviceTag:   tag,
			DeviceCount: countByTag[tag.ID],
		})
	}
	return result, nil
}

func CreateDeviceTag(scope AccessScope, tag *DeviceTag) (*DeviceTag, error) {
	scope = normalizeAccessScope(scope)
	if tag == nil {
		return nil, fmt.Errorf("tag is required")
	}
	name := strings.TrimSpace(tag.Name)
	if name == "" {
		return nil, fmt.Errorf("tag name is required")
	}
	color := strings.TrimSpace(tag.Color)
	if color == "" {
		color = "#0d6efd"
	}

	newTag := &DeviceTag{
		ScopeType:   scope.Type,
		ScopeID:     scope.ID,
		Name:        name,
		Color:       color,
		Icon:        tag.Icon,
		Description: strings.TrimSpace(tag.Description),
	}
	if err := DB.Create(newTag).Error; err != nil {
		return nil, err
	}
	return newTag, nil
}

func UpdateDeviceTag(scope AccessScope, tag *DeviceTag) (*DeviceTag, error) {
	scope = normalizeAccessScope(scope)
	if tag == nil || tag.ID == 0 {
		return nil, fmt.Errorf("tag id is required")
	}
	name := strings.TrimSpace(tag.Name)
	if name == "" {
		return nil, fmt.Errorf("tag name is required")
	}
	color := strings.TrimSpace(tag.Color)
	if color == "" {
		color = "#0d6efd"
	}

	var existing DeviceTag
	if err := DB.Where("id = ? AND scope_type = ? AND scope_id = ?", tag.ID, scope.Type, scope.ID).First(&existing).Error; err != nil {
		return nil, err
	}
	existing.Name = name
	existing.Color = color
	existing.Icon = tag.Icon
	existing.Description = strings.TrimSpace(tag.Description)
	if err := DB.Save(&existing).Error; err != nil {
		return nil, err
	}
	return &existing, nil
}

func DeleteDeviceTag(scope AccessScope, tagID uint) error {
	scope = normalizeAccessScope(scope)
	if tagID == 0 {
		return fmt.Errorf("tag id is required")
	}

	return DB.Transaction(func(tx *gorm.DB) error {
		var tag DeviceTag
		if err := tx.Where("id = ? AND scope_type = ? AND scope_id = ?", tagID, scope.Type, scope.ID).First(&tag).Error; err != nil {
			return err
		}

		if err := tx.Unscoped().Where("scope_type = ? AND scope_id = ? AND tag_id = ?", scope.Type, scope.ID, tagID).Delete(&DeviceTagBinding{}).Error; err != nil {
			return err
		}

		newName := fmt.Sprintf("%s_del_%d", tag.Name, time.Now().UnixNano())
		if err := tx.Model(&tag).Update("name", newName).Error; err != nil {
			return err
		}
		return tx.Delete(&tag).Error
	})
}

func ReplaceDeviceTags(scope AccessScope, deviceCode string, tagIDs []uint) error {
	scope = normalizeAccessScope(scope)
	deviceCode = strings.TrimSpace(deviceCode)
	if deviceCode == "" {
		return fmt.Errorf("device code is required")
	}
	tagIDs = uniqueUintIDs(tagIDs)

	return DB.Transaction(func(tx *gorm.DB) error {
		var device Device
		if err := tx.Where("code = ?", deviceCode).First(&device).Error; err != nil {
			return err
		}
		if err := validateTagIDsInScope(tx, scope, tagIDs); err != nil {
			return err
		}
		if err := tx.Unscoped().Where("scope_type = ? AND scope_id = ? AND device_code = ?", scope.Type, scope.ID, deviceCode).Delete(&DeviceTagBinding{}).Error; err != nil {
			return err
		}
		for _, tagID := range tagIDs {
			binding := DeviceTagBinding{
				ScopeType:  scope.Type,
				ScopeID:    scope.ID,
				DeviceCode: deviceCode,
				TagID:      tagID,
			}
			if err := tx.Create(&binding).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func ReplaceDevicesForTag(scope AccessScope, tagID uint, deviceCodes []string) error {
	scope = normalizeAccessScope(scope)
	if tagID == 0 {
		return fmt.Errorf("tag id is required")
	}
	deviceCodes = uniqueDeviceCodes(deviceCodes)

	return DB.Transaction(func(tx *gorm.DB) error {
		if err := validateTagIDsInScope(tx, scope, []uint{tagID}); err != nil {
			return err
		}
		if err := validateDeviceCodes(tx, deviceCodes); err != nil {
			return err
		}
		if err := tx.Unscoped().Where("scope_type = ? AND scope_id = ? AND tag_id = ?", scope.Type, scope.ID, tagID).Delete(&DeviceTagBinding{}).Error; err != nil {
			return err
		}
		for _, deviceCode := range deviceCodes {
			binding := DeviceTagBinding{
				ScopeType:  scope.Type,
				ScopeID:    scope.ID,
				DeviceCode: deviceCode,
				TagID:      tagID,
			}
			if err := tx.Create(&binding).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func ListTagsForDevices(scope AccessScope, deviceCodes []string) (map[string][]DeviceTag, error) {
	scope = normalizeAccessScope(scope)
	deviceCodes = uniqueDeviceCodes(deviceCodes)
	result := make(map[string][]DeviceTag, len(deviceCodes))
	if len(deviceCodes) == 0 {
		return result, nil
	}

	var bindings []DeviceTagBinding
	if err := DB.Where("scope_type = ? AND scope_id = ? AND device_code IN ?", scope.Type, scope.ID, deviceCodes).
		Find(&bindings).Error; err != nil {
		return nil, err
	}
	if len(bindings) == 0 {
		return result, nil
	}

	tagIDs := make([]uint, 0, len(bindings))
	for _, binding := range bindings {
		tagIDs = append(tagIDs, binding.TagID)
	}
	tagIDs = uniqueUintIDs(tagIDs)

	var tags []DeviceTag
	if err := DB.Where("scope_type = ? AND scope_id = ? AND id IN ?", scope.Type, scope.ID, tagIDs).
		Order("name asc").
		Find(&tags).Error; err != nil {
		return nil, err
	}

	tagByID := make(map[uint]DeviceTag, len(tags))
	for _, tag := range tags {
		tagByID[tag.ID] = tag
	}
	for _, binding := range bindings {
		if tag, ok := tagByID[binding.TagID]; ok {
			result[binding.DeviceCode] = append(result[binding.DeviceCode], tag)
		}
	}
	for deviceCode := range result {
		sort.Slice(result[deviceCode], func(i, j int) bool {
			return result[deviceCode][i].Name < result[deviceCode][j].Name
		})
	}
	return result, nil
}

func ListDeviceCodesForTag(scope AccessScope, tagID uint) ([]string, error) {
	scope = normalizeAccessScope(scope)
	if tagID == 0 {
		return []string{}, nil
	}

	var bindings []DeviceTagBinding
	if err := DB.Where("scope_type = ? AND scope_id = ? AND tag_id = ?", scope.Type, scope.ID, tagID).
		Order("device_code asc").
		Find(&bindings).Error; err != nil {
		return nil, err
	}
	codes := make([]string, 0, len(bindings))
	for _, binding := range bindings {
		codes = append(codes, binding.DeviceCode)
	}
	return codes, nil
}

func validateTagIDsInScope(tx *gorm.DB, scope AccessScope, tagIDs []uint) error {
	if len(tagIDs) == 0 {
		return nil
	}
	var count int64
	if err := tx.Model(&DeviceTag{}).
		Where("scope_type = ? AND scope_id = ? AND id IN ?", scope.Type, scope.ID, tagIDs).
		Count(&count).Error; err != nil {
		return err
	}
	if count != int64(len(tagIDs)) {
		return fmt.Errorf("one or more tags do not exist in current scope")
	}
	return nil
}

func validateDeviceCodes(tx *gorm.DB, deviceCodes []string) error {
	if len(deviceCodes) == 0 {
		return nil
	}
	var count int64
	if err := tx.Model(&Device{}).Where("code IN ?", deviceCodes).Count(&count).Error; err != nil {
		return err
	}
	if count != int64(len(deviceCodes)) {
		return fmt.Errorf("one or more devices do not exist")
	}
	return nil
}

func uniqueUintIDs(ids []uint) []uint {
	seen := make(map[uint]bool, len(ids))
	result := make([]uint, 0, len(ids))
	for _, id := range ids {
		if id == 0 || seen[id] {
			continue
		}
		seen[id] = true
		result = append(result, id)
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}

func uniqueDeviceCodes(codes []string) []string {
	seen := make(map[string]bool, len(codes))
	result := make([]string, 0, len(codes))
	for _, code := range codes {
		code = strings.TrimSpace(code)
		if code == "" || seen[code] {
			continue
		}
		seen[code] = true
		result = append(result, code)
	}
	sort.Strings(result)
	return result
}
