package store

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"noyo/core/utils"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// DB Global database instance
var DB *gorm.DB

// InitDB initializes the database
func InitDB(dsn string) error {
	var err error

	// Ensure the directory for the database file exists
	dbDir := filepath.Dir(dsn)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return fmt.Errorf("failed to create database directory: %w", err)
	}

	// Use pure go sqlite
	DB, err = gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("failed to connect database: %w", err)
	}

	// Configure connection pool
	sqlDB, err := DB.DB()
	if err != nil {
		return fmt.Errorf("failed to get sql.DB: %w", err)
	}
	sqlDB.SetMaxIdleConns(10)           // Max idle connections
	sqlDB.SetMaxOpenConns(100)          // Max open connections
	sqlDB.SetConnMaxLifetime(time.Hour) // Connection max lifetime

	if err := migratePluginScopeSchema(); err != nil {
		return fmt.Errorf("failed to migrate plugin scope schema: %w", err)
	}

	// AutoMigrate models
	err = DB.AutoMigrate(
		&User{},
		&Tenant{},
		&Project{},
		&Role{},
		&Permission{},
		&UserRoleBinding{},
		&RolePermission{},
		&ScopePermissionLimit{},
		&RoleDeviceTagPermission{},
		&PluginModel{},
		&Product{},
		&Device{},
		&DeviceTag{},
		&DeviceTagBinding{},
		&SystemConfig{},
		&GatewayPluginStateModel{},
		&App{},
		&AppRole{},
		&AppProjectAccess{},
		&AppPermission{},
		&AppDeviceTagPermission{},
		&AuditLog{},
		&TokenBlacklist{},
	)
	if err != nil {
		return fmt.Errorf("failed to migrate database: %w", err)
	}
	_ = DB.Migrator().DropIndex(&AppRole{}, "idx_app_role")
	_ = DB.Migrator().DropIndex(&RoleDeviceTagPermission{}, "idx_role_tag")
	DB.Exec(`
		UPDATE role_device_tag_permissions
		SET project_id = (
			SELECT roles.project_id
			FROM roles
			WHERE roles.id = role_device_tag_permissions.role_id
		)
		WHERE project_id = 0
		  AND role_id IN (
			SELECT id
			FROM roles
			WHERE project_id > 0
		  )
	`)
	DB.Exec("UPDATE apps SET status = 1 WHERE status IS NULL")
	if err := purgeLegacyPositionPermissions(); err != nil {
		return fmt.Errorf("failed to purge legacy position permissions: %w", err)
	}

	// Initialize default super admin, tenant, project, and permissions
	InitDefaultData()

	// Migrate plaintext AppKeys to bcrypt hashes
	migrateAppKeys()

	// Clean expired token blacklist entries
	CleanExpiredBlacklist()

	return nil
}

func purgeLegacyPositionPermissions() error {
	var permissionIDs []uint
	if err := DB.Model(&Permission{}).Where("code LIKE ?", "position:%").Pluck("id", &permissionIDs).Error; err != nil {
		return err
	}
	if len(permissionIDs) == 0 {
		return nil
	}
	if err := DB.Unscoped().Where("permission_id IN ?", permissionIDs).Delete(&RolePermission{}).Error; err != nil {
		return err
	}
	if err := DB.Unscoped().Where("permission_id IN ?", permissionIDs).Delete(&ScopePermissionLimit{}).Error; err != nil {
		return err
	}
	return DB.Unscoped().Where("id IN ?", permissionIDs).Delete(&Permission{}).Error
}

var superAdminDefaultPermissionCodes = map[string]bool{
	"tenant:list":    true,
	"tenant:create":  true,
	"tenant:edit":    true,
	"tenant:delete":  true,
	"dashboard:view": true,
	"audit:list":     true,
	"system:logs":    true,
	"system:license": true,
}

var exclusiveTenantManagementPermissionCodes = map[string]bool{
	"tenant:list":   true,
	"tenant:create": true,
	"tenant:edit":   true,
	"tenant:delete": true,
}

func InitDefaultData() {
	// Initialize default super admin
	var userCount int64
	if err := DB.Model(&User{}).Count(&userCount).Error; err == nil && userCount == 0 {
		hashedPassword, _ := utils.HashPassword("admin123")
		defaultAdmin := User{
			TenantID:           0,
			Username:           "admin",
			Password:           hashedPassword,
			DisplayName:        "超级管理员",
			Role:               "admin",
			MustChangePassword: true,
		}
		DB.Create(&defaultAdmin)
	}

	// Initialize default system permissions
	InitPermissions()
}

func InitPermissions() {
	defaultPermissions := []Permission{
		// Tenants
		{Code: "tenant:list", Name: "租户列表", Module: "tenant", Type: "menu"},
		{Code: "tenant:create", Name: "创建租户", Module: "tenant", Type: "button"},
		{Code: "tenant:edit", Name: "编辑租户", Module: "tenant", Type: "button"},
		{Code: "tenant:delete", Name: "删除租户", Module: "tenant", Type: "button"},
		// Projects
		{Code: "project:list", Name: "项目列表", Module: "project", Type: "menu"},
		{Code: "project:create", Name: "创建项目", Module: "project", Type: "button"},
		{Code: "project:edit", Name: "编辑项目", Module: "project", Type: "button"},
		{Code: "project:delete", Name: "删除项目", Module: "project", Type: "button"},
		// Users
		{Code: "user:list", Name: "用户列表", Module: "user", Type: "menu"},
		{Code: "user:create", Name: "创建用户", Module: "user", Type: "button"},
		{Code: "user:edit", Name: "编辑用户", Module: "user", Type: "button"},
		{Code: "user:delete", Name: "删除用户", Module: "user", Type: "button"},
		// Roles
		{Code: "role:list", Name: "角色列表", Module: "role", Type: "menu"},
		{Code: "role:create", Name: "创建角色", Module: "role", Type: "button"},
		{Code: "role:edit", Name: "编辑角色", Module: "role", Type: "button"},
		{Code: "role:delete", Name: "删除角色", Module: "role", Type: "button"},
		// Apps
		{Code: "app:list", Name: "应用列表", Module: "app", Type: "menu"},
		{Code: "app:create", Name: "创建应用", Module: "app", Type: "button"},
		{Code: "app:edit", Name: "编辑应用", Module: "app", Type: "button"},
		{Code: "app:delete", Name: "删除应用", Module: "app", Type: "button"},
		{Code: "app:reset-key", Name: "重置应用密钥", Module: "app", Type: "button"},
		// Products
		{Code: "product:list", Name: "产品列表", Module: "product", Type: "menu"},
		{Code: "product:create", Name: "创建产品", Module: "product", Type: "button"},
		{Code: "product:edit", Name: "编辑产品", Module: "product", Type: "button"},
		{Code: "product:delete", Name: "删除产品", Module: "product", Type: "button"},
		// Devices
		{Code: "device:list", Name: "设备列表", Module: "device", Type: "menu"},
		{Code: "device:create", Name: "创建设备", Module: "device", Type: "button"},
		{Code: "device:edit", Name: "编辑设备", Module: "device", Type: "button"},
		{Code: "device:delete", Name: "删除设备", Module: "device", Type: "button"},
		{Code: "device:control", Name: "设备控制", Module: "device", Type: "button"},
		{Code: "device:topology", Name: "设备拓扑", Module: "device", Type: "menu"},
		// Device Tags
		{Code: "device_tag:list", Name: "设备标签列表", Module: "device_tag", Type: "menu"},
		{Code: "device_tag:create", Name: "创建设备标签", Module: "device_tag", Type: "button"},
		{Code: "device_tag:edit", Name: "编辑设备标签", Module: "device_tag", Type: "button"},
		{Code: "device_tag:delete", Name: "删除设备标签", Module: "device_tag", Type: "button"},
		// Plugins
		{Code: "plugin:list", Name: "插件列表", Module: "plugin", Type: "menu"},
		{Code: "plugin:config", Name: "配置插件", Module: "plugin", Type: "button"},
		// Gateway
		{Code: "gateway:list", Name: "网关管理", Module: "gateway", Type: "menu"},
		{Code: "gateway:config", Name: "网关配置", Module: "gateway", Type: "button"},
		// Alarm
		{Code: "alarm:list", Name: "告警列表", Module: "alarm", Type: "menu"},
		{Code: "alarm:handle", Name: "处理告警", Module: "alarm", Type: "button"},
		// History
		{Code: "history:delete", Name: "删除历史记录", Module: "history", Type: "button"},
		// Audit
		{Code: "audit:list", Name: "审计日志", Module: "audit", Type: "menu"},
		// System Config & Log
		{Code: "system:config", Name: "系统配置", Module: "system", Type: "menu"},
		{Code: "system:logs", Name: "系统日志", Module: "system", Type: "menu"},
		{Code: "system:license", Name: "授权信息", Module: "system", Type: "menu"},
		// Dashboard
		{Code: "dashboard:view", Name: "仪表盘", Module: "system", Type: "menu"},
		// Tenant Transfer
		{Code: "tenant:transfer", Name: "转让租户管理员", Module: "tenant", Type: "button"},
		// Device Upload
		{Code: "device:upload", Name: "设备图片上传", Module: "device", Type: "button"},
	}

	for _, p := range defaultPermissions {
		var permission Permission
		DB.Where("code = ?", p.Code).
			Assign(Permission{
				Code:       p.Code,
				Name:       p.Name,
				Module:     p.Module,
				Type:       p.Type,
				ParentCode: p.ParentCode,
				SortOrder:  p.SortOrder,
			}).
			FirstOrCreate(&permission)
	}

	superAdminRole := Role{
		TenantID:    0,
		ProjectID:   0,
		Code:        "super_admin",
		Name:        "超级管理员",
		Description: "系统默认全局角色：管理租户和系统级信息，不进入租户业务数据",
		DataScope:   1,
		IsBuiltin:   true,
		IsInherited: false,
	}
	DB.Where("tenant_id = ? AND project_id = ? AND code = ?", 0, 0, "super_admin").
		Assign(superAdminRole).
		FirstOrCreate(&superAdminRole)

	// Bind default admin user to super_admin role
	var defaultAdmin User
	if err := DB.Where("username = ?", "admin").First(&defaultAdmin).Error; err == nil {
		DB.FirstOrCreate(&UserRoleBinding{}, UserRoleBinding{
			UserID:    defaultAdmin.ID,
			RoleID:    superAdminRole.ID,
			TenantID:  0, // Global binding
			ProjectID: 0,
		})
	}

	tenantAdminRole := Role{
		TenantID:    0,
		ProjectID:   0,
		Code:        "tenant_admin",
		Name:        "租户管理员",
		Description: "系统默认全局角色：拥有租户下的所有权限",
		DataScope:   1,
		IsBuiltin:   true,
		IsInherited: false,
	}
	DB.Where("tenant_id = ? AND project_id = ? AND code = ?", 0, 0, "tenant_admin").
		Assign(tenantAdminRole).
		FirstOrCreate(&tenantAdminRole)

	projectAdminRole := Role{
		TenantID:    0,
		ProjectID:   0,
		Code:        "project_admin",
		Name:        "项目管理员",
		Description: "系统默认全局角色：拥有所在项目的所有权限",
		DataScope:   2,
		IsBuiltin:   true,
		IsInherited: false,
	}
	DB.Where("tenant_id = ? AND project_id = ? AND code = ?", 0, 0, "project_admin").
		Assign(projectAdminRole).
		FirstOrCreate(&projectAdminRole)

	migrateProjectMembershipViewerBindings()

	var perms []Permission
	if err := DB.Find(&perms).Error; err == nil {
		for _, perm := range perms {
			if superAdminDefaultPermissionCodes[perm.Code] {
				DB.Where("role_id = ? AND permission_id = ?", superAdminRole.ID, perm.ID).
					FirstOrCreate(&RolePermission{
						RoleID:       superAdminRole.ID,
						PermissionID: perm.ID,
					})
			}

			// tenant:transfer is a tenant-module permission that tenant_admin also needs
			if perm.Code == "tenant:transfer" {
				DB.Where("role_id = ? AND permission_id = ?", tenantAdminRole.ID, perm.ID).
					FirstOrCreate(&RolePermission{
						RoleID:       tenantAdminRole.ID,
						PermissionID: perm.ID,
					})
			}

			// dashboard:view is needed by all roles to access the root path
			if perm.Code == "dashboard:view" {
				DB.Where("role_id = ? AND permission_id = ?", tenantAdminRole.ID, perm.ID).
					FirstOrCreate(&RolePermission{
						RoleID:       tenantAdminRole.ID,
						PermissionID: perm.ID,
					})
				DB.Where("role_id = ? AND permission_id = ?", projectAdminRole.ID, perm.ID).
					FirstOrCreate(&RolePermission{
						RoleID:       projectAdminRole.ID,
						PermissionID: perm.ID,
					})
			}

			if perm.Module != "tenant" && perm.Module != "system" {
				DB.Where("role_id = ? AND permission_id = ?", tenantAdminRole.ID, perm.ID).
					FirstOrCreate(&RolePermission{
						RoleID:       tenantAdminRole.ID,
						PermissionID: perm.ID,
					})
			}

			if perm.Module == "user" || perm.Module == "role" || perm.Module == "product" || perm.Module == "device" || perm.Module == "device_tag" || perm.Module == "gateway" || perm.Module == "alarm" || perm.Module == "plugin" {
				DB.Where("role_id = ? AND permission_id = ?", projectAdminRole.ID, perm.ID).
					FirstOrCreate(&RolePermission{
						RoleID:       projectAdminRole.ID,
						PermissionID: perm.ID,
					})
			}
		}
	}
	syncRolePermissionAllowlist(superAdminRole.ID, superAdminDefaultPermissionCodes)
	removePermissionCodesFromOtherRoles(superAdminRole.ID, exclusiveTenantManagementPermissionCodes)
}

func migrateProjectMembershipViewerBindings() {
	var viewerRole Role
	if err := DB.Where("tenant_id = ? AND project_id = ? AND code = ?", 0, 0, "viewer").First(&viewerRole).Error; err != nil {
		return
	}
	DB.Model(&UserRoleBinding{}).
		Where("role_id = ? AND project_id > ?", viewerRole.ID, 0).
		Update("role_id", 0)
	DB.Unscoped().Where("role_id = ?", viewerRole.ID).Delete(&RolePermission{})
	DB.Unscoped().Delete(&viewerRole)
}

func syncRolePermissionAllowlist(roleID uint, allowedCodes map[string]bool) {
	permissionIDs := permissionIDsForCodeSet(allowedCodes)
	if len(permissionIDs) == 0 {
		DB.Where("role_id = ?", roleID).Delete(&RolePermission{})
		return
	}
	DB.Where("role_id = ? AND permission_id NOT IN ?", roleID, permissionIDs).Delete(&RolePermission{})
}

func removePermissionCodesFromOtherRoles(exemptRoleID uint, codes map[string]bool) {
	permissionIDs := permissionIDsForCodeSet(codes)
	if len(permissionIDs) == 0 {
		return
	}
	DB.Where("role_id <> ? AND permission_id IN ?", exemptRoleID, permissionIDs).Delete(&RolePermission{})
}

func permissionIDsForCodeSet(codes map[string]bool) []uint {
	codeList := make([]string, 0, len(codes))
	for code, enabled := range codes {
		if enabled {
			codeList = append(codeList, code)
		}
	}
	if len(codeList) == 0 {
		return nil
	}

	var permissionIDs []uint
	DB.Model(&Permission{}).Where("code IN ?", codeList).Pluck("id", &permissionIDs)
	return permissionIDs
}

// CloseDB closes the database connection (if needed, though GORM manages pool)
func CloseDB() {
	sqlDB, err := DB.DB()
	if err != nil {
		return
	}
	sqlDB.Close()
}

// --- Product CRUD Helper ---

func UpdateProduct(p *Product) error {
	var existing Product
	if err := DB.Where("code = ?", p.Code).First(&existing).Error; err != nil {
		return err
	}
	p.ID = existing.ID
	p.CreatedAt = existing.CreatedAt
	return DB.Save(p).Error
}

func DeleteProduct(code string) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		var product Product
		if err := tx.Where("code = ?", code).First(&product).Error; err != nil {
			return err
		}

		newCode := fmt.Sprintf("%s_del_%d", product.Code, time.Now().Unix())

		// Rename code to release unique constraint
		if err := tx.Model(&product).Update("code", newCode).Error; err != nil {
			return err
		}

		// Soft delete
		return tx.Delete(&product).Error
	})
}

// --- Device CRUD Helper ---

// UpdateDevice updates non-zero fields of a device
func UpdateDevice(d *Device) error {
	var existing Device
	if err := DB.Where("code = ?", d.Code).First(&existing).Error; err != nil {
		return err
	}
	d.ID = existing.ID
	d.CreatedAt = existing.CreatedAt
	return DB.Save(d).Error
}

// migrateAppKeys hashes plaintext AppKeys in the database.
// Bcrypt hashes start with "$2", so any AppKey not starting with "$2" is plaintext.
func migrateAppKeys() {
	var apps []App
	DB.Find(&apps)
	for _, app := range apps {
		if app.AppKey != "" && !strings.HasPrefix(app.AppKey, "$2") {
			hashed, err := utils.HashPassword(app.AppKey)
			if err != nil {
				continue
			}
			DB.Model(&app).Update("app_key", hashed)
		}
	}
}

// CleanExpiredBlacklist removes expired token blacklist entries from the database.
func CleanExpiredBlacklist() {
	DB.Where("expires_at < ?", time.Now()).Delete(&TokenBlacklist{})
}
