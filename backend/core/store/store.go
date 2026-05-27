package store

import (
	"fmt"
	"os"
	"path/filepath"
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

	// AutoMigrate models
	err = DB.AutoMigrate(
		&User{},
		&Tenant{},
		&Project{},
		&Role{},
		&Permission{},
		&UserRoleBinding{},
		&RolePermission{},
		&RoleDeviceTagPermission{},
		&PluginModel{},
		&Product{},
		&Device{},
		&DeviceTag{},
		&DeviceTagBinding{},
		&SystemConfig{},
		&GatewayPluginStateModel{},
		&Position{},
		&UserPosition{},
		&PositionRole{},
		&App{},
		&AppRole{},
		&AuditLog{},
	)
	if err != nil {
		return fmt.Errorf("failed to migrate database: %w", err)
	}

	// Initialize default super admin, tenant, project, and permissions
	InitDefaultData()

	return nil
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
			Status:             1,
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
		// Positions
		{Code: "position:list", Name: "岗位列表", Module: "position", Type: "menu"},
		{Code: "position:create", Name: "创建岗位", Module: "position", Type: "button"},
		{Code: "position:edit", Name: "编辑岗位", Module: "position", Type: "button"},
		{Code: "position:delete", Name: "删除岗位", Module: "position", Type: "button"},
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
		// Plugins
		{Code: "plugin:list", Name: "插件列表", Module: "plugin", Type: "menu"},
		{Code: "plugin:config", Name: "配置插件", Module: "plugin", Type: "button"},
		// Alarm
		{Code: "alarm:list", Name: "告警列表", Module: "alarm", Type: "menu"},
		{Code: "alarm:handle", Name: "处理告警", Module: "alarm", Type: "button"},
		// Audit
		{Code: "audit:list", Name: "审计日志", Module: "audit", Type: "menu"},
		// System Config & Log
		{Code: "system:config", Name: "系统配置", Module: "system", Type: "menu"},
		{Code: "system:logs", Name: "系统日志", Module: "system", Type: "menu"},
	}

	for _, p := range defaultPermissions {
		var permission Permission
		DB.Where("code = ?", p.Code).Assign(Permission{
			Name:       p.Name,
			Module:     p.Module,
			Type:       p.Type,
			ParentCode: p.ParentCode,
			SortOrder:  p.SortOrder,
		}).FirstOrCreate(&permission, Permission{Code: p.Code})
	}

	superAdminRole := Role{
		TenantID:    0,
		ProjectID:   0,
		Code:        "super_admin",
		Name:        "超级管理员",
		Description: "系统默认全局角色：拥有系统内的所有最高权限",
		DataScope:   1,
		IsBuiltin:   true,
		Status:      1,
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
		Status:      1,
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
		Status:      1,
		IsInherited: false,
	}
	DB.Where("tenant_id = ? AND project_id = ? AND code = ?", 0, 0, "project_admin").
		Assign(projectAdminRole).
		FirstOrCreate(&projectAdminRole)

	var perms []Permission
	if err := DB.Find(&perms).Error; err == nil {
		for _, perm := range perms {
			DB.FirstOrCreate(&RolePermission{}, RolePermission{
				RoleID:       superAdminRole.ID,
				PermissionID: perm.ID,
			})

			DB.FirstOrCreate(&RolePermission{}, RolePermission{
				RoleID:       tenantAdminRole.ID,
				PermissionID: perm.ID,
			})

			if perm.Module != "project" && perm.Module != "tenant" {
				DB.FirstOrCreate(&RolePermission{}, RolePermission{
					RoleID:       projectAdminRole.ID,
					PermissionID: perm.ID,
				})
			}
		}
	}
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
