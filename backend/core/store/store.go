package store

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"noyo/core/utils"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// DB Global database instance
var DB *gorm.DB

func newGormLogger(level string) gormlogger.Interface {
	logLevel := gormlogger.Warn
	switch strings.ToLower(level) {
	case "debug":
		logLevel = gormlogger.Info
	case "error":
		logLevel = gormlogger.Error
	case "silent":
		logLevel = gormlogger.Silent
	default:
		logLevel = gormlogger.Warn
	}

	return gormlogger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags),
		gormlogger.Config{
			SlowThreshold:             time.Second,
			LogLevel:                  logLevel,
			IgnoreRecordNotFoundError: true,
			Colorful:                  false,
		},
	)
}

func SetDBLogLevel(level string) {
	if DB == nil || DB.Config == nil {
		return
	}
	DB.Config.Logger = newGormLogger(level)
}

// InitDB initializes the database
func InitDB(dsn string) error {
	var err error

	// Ensure the directory for the database file exists
	dbDir := filepath.Dir(dsn)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return fmt.Errorf("failed to create database directory: %w", err)
	}

	// Append concurrency parameters if not present
	if !strings.Contains(dsn, "?") {
		dsn += "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_txlock=immediate"
	} else if !strings.Contains(dsn, "_txlock") {
		if !strings.Contains(dsn, "_pragma=busy_timeout") {
			dsn += "&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)"
		}
		dsn += "&_txlock=immediate"
	}

	// Use pure go sqlite
	DB, err = gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: newGormLogger("warn"),
	})
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

	migrateAppliedChangesSchema()

	// AutoMigrate models
	err = DB.AutoMigrate(
		&IOCScreen{},
		&IOCScreenApplyReceipt{},
		&IOCScreenMessage{},
		&IOCScreenConversation{},
		&IOCScreenAssistantJob{},
		&IOCScreenRelease{},
		&IOCScreenResourceRef{},
		&IOCScreenAsset{},
		&IOCDataSet{},
		&IOCDataSetVersion{},
		&SyncChange{},
		&ConfigProjection{},
		&ConfigIntent{},
		&SyncSnapshotStage{},
		&ResourceLifecycle{},
		&AppliedChange{},
		&SyncDeliveryResult{},
		&RuntimeApply{},
		&GatewayRuntimeState{},
		&SyncBootstrap{},
		&SyncCursor{},
		&SyncNodeIdentity{},
		&GatewayMaintenance{},
		&User{},
		&Tenant{},
		&Project{},
		&Role{},
		&Permission{},
		&UserRoleBinding{},
		&RolePermission{},
		&ScopePermissionLimit{},
		&ScopePermissionPolicy{},
		&RoleDeviceTagPermission{},
		&PluginModel{},
		&Product{},
		&ProtocolProfile{},
		&Device{},
		&DeviceTag{},
		&DeviceTagBinding{},
		&Rule{},
		&RuleGroup{},
		&RuleExecLog{},
		&SystemConfig{},
		&GatewayPluginStateModel{},
		&App{},
		&AppRole{},
		&AppProjectAccess{},
		&AppPermission{},
		&AppDeviceTagPermission{},
		&AuditLog{},
		&LLMRequest{},
		&ModelHistory{},
		&TokenBlacklist{},
		&AuthSession{},
		&StoredAsset{},
		&DeviceLocation{},
		&Geofence{},
		&GeofenceDevice{},
		&DeviceFenceState{},
	)
	if err != nil {
		return fmt.Errorf("failed to migrate database: %w", err)
	}
	if err := MigrateWorkOrderModels(DB); err != nil {
		return fmt.Errorf("failed to migrate work order database: %w", err)
	}
	if err := recoverInterruptedLLMRequests(DB); err != nil {
		return fmt.Errorf("failed to recover interrupted LLM request records: %w", err)
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
	if err := migrateLegacyScopePermissionPolicies(DB); err != nil {
		return fmt.Errorf("failed to migrate legacy scope permission policies: %w", err)
	}

	// Migrate plaintext AppKeys to bcrypt hashes
	migrateAppKeys()

	// Clean expired token blacklist entries
	CleanExpiredBlacklist()

	return nil
}

func migrateAppliedChangesSchema() {
	if DB == nil {
		return
	}
	_ = DB.Exec("DROP TABLE IF EXISTS `applied_changes__temp`").Error
	if DB.Migrator().HasTable("applied_changes") {
		if DB.Migrator().HasColumn("applied_changes", "status") {
			_ = DB.Exec("UPDATE `applied_changes` SET `status` = 'applied' WHERE `status` IS NULL OR `status` = ''").Error
		}
	}
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
	"tenant:list":            true,
	"tenant:create":          true,
	"tenant:edit":            true,
	"tenant:delete":          true,
	"dashboard:view":         true,
	"screen:list":            true,
	"screen:view":            true,
	"screen:create":          true,
	"screen:edit":            true,
	"screen:publish":         true,
	"screen:delete":          true,
	"audit:list":             true,
	"llm_request:list":       true,
	"system:logs":            true,
	"system:license":         true,
	"rule:list":              true,
	"rule:detail":            true,
	"rule:create":            true,
	"rule:edit":              true,
	"rule:delete":            true,
	"rule:enable":            true,
	"rule:log":               true,
	"rule_group:manage":      true,
	"ai_brain:view":          true,
	"ai_brain:review":        true,
	"ai_brain:edit":          true,
	"ai_brain:delete":        true,
	"ai_brain:suggestion":    true,
	"ai_brain:manage":        true,
	"work_order:list":        true,
	"work_order:create":      true,
	"work_order:process":     true,
	"work_order:approve":     true,
	"work_order:manage":      true,
	"work_order:integration": true,
	"alarm:list":             true,
	"alarm:handle":           true,
	"alarm:manage":           true,
	"alarm:export":           true,
	"ai_scene:list":          true,
	"inspection:list":        true,
	"inspection:manage":      true,
	"inspection:execute":     true,
	"inspection:order":       true,
	"inspection:export":      true,
	"inspection:review":      true,
	"inspection:archive":     true,
	"ai_scene:edit":          true,
	"semantic_review:list":   true,
	"ai_model:list":          true,
	"ai_model:export":        true,
	"ai_model:manage":        true,
	"ai_assistant:use":       true,
	"device_map:view":        true,
	"device_location:edit":   true,
	"geofence:list":          true,
	"geofence:create":        true,
	"geofence:edit":          true,
	"geofence:delete":        true,
}

var exclusiveTenantManagementPermissionCodes = map[string]bool{
	"tenant:list":   true,
	"tenant:create": true,
	"tenant:edit":   true,
	"tenant:delete": true,
}

const (
	VoiceAssistantUserName    = "voice_assistant"
	VoiceAssistantRoleCode    = "voice_assistant"
	VoiceAssistantDisplayName = "语音助手"
	VoiceAssistantRoleName    = "语音助手角色"
)

var VoiceAssistantDefaultReadPermissionCodes = []string{
	"dashboard:view",
	"user:list",
	"role:list",
	"tenant:list",
	"project:list",
	"app:list",
	"product:list",
	"device:list",
	"device:topology",
	"device_tag:list",
	"audit:list",
	"gateway:list",
	"alarm:list",
	"plugin:list",
	"system:config",
	"system:logs",
	"system:license",
	"ai_brain:view",
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

	if state, err := LoadSetupState(); err == nil && state.Initialized && state.TenantID > 0 {
		_ = EnsureVoiceAssistantScope(DB, state.TenantID, state.ProjectID)
	}
}

func InitPermissions() {
	defaultPermissions := []Permission{
		{Code: "screen:list", Name: "大屏列表 / Screens", Module: "screen", Type: "menu"},
		{Code: "screen:view", Name: "查看大屏 / View screen", Module: "screen", Type: "button"},
		{Code: "screen:create", Name: "创建大屏 / Create screen", Module: "screen", Type: "button"},
		{Code: "screen:edit", Name: "编辑大屏 / Edit screen", Module: "screen", Type: "button"},
		{Code: "screen:publish", Name: "发布大屏 / Publish screen", Module: "screen", Type: "button"},
		{Code: "screen:delete", Name: "删除大屏 / Delete screen", Module: "screen", Type: "button"},
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
		// Independent AI capabilities; custom roles and custom scope limits opt in.
		{Code: "ai_scene:list", Name: "AI场景 / AI scenes", Module: "ai_scene", Type: "menu"},
		{Code: "inspection:list", Name: "视频巡检 / Video inspection", Module: "inspection", Type: "menu"},
		{Code: "inspection:manage", Name: "管理巡检计划 / Manage inspection plans", Module: "inspection", Type: "button"},
		{Code: "inspection:execute", Name: "执行巡检 / Execute inspections", Module: "inspection", Type: "button"},
		{Code: "inspection:order", Name: "巡检建单 / Create inspection work orders", Module: "inspection", Type: "button"},
		{Code: "inspection:export", Name: "导出巡检报告 / Export inspection reports", Module: "inspection", Type: "button"},
		{Code: "inspection:review", Name: "复核巡检发现 / Review inspection findings", Module: "inspection", Type: "button"},
		{Code: "inspection:archive", Name: "归档巡检报告 / Archive inspection reports", Module: "inspection", Type: "button"},
		{Code: "ai_scene:edit", Name: "配置AI场景 / Configure AI scenes", Module: "ai_scene", Type: "button"},
		{Code: "semantic_review:list", Name: "语义复核记录 / Semantic reviews", Module: "semantic_review", Type: "menu"},
		{Code: "ai_model:list", Name: "模型中心 / Model center", Module: "ai_model", Type: "menu"},
		{Code: "ai_model:export", Name: "导出模型资产 / Export model assets", Module: "ai_model", Type: "button"},
		// Models are machine-wide assets, so mutation remains a system capability.
		{Code: "ai_model:manage", Name: "管理模型资产 / Manage model assets", Module: "system", Type: "button"},
		{Code: "ai_assistant:use", Name: "使用AI助手 / Use AI assistant", Module: "ai_assistant", Type: "menu"},
		// AI Brain
		{Code: "ai_brain:view", Name: "AI大脑查看", Module: "ai_brain", Type: "menu"},
		{Code: "ai_brain:review", Name: "AI大脑审核", Module: "ai_brain", Type: "button"},
		{Code: "ai_brain:edit", Name: "AI大脑编辑", Module: "ai_brain", Type: "button"},
		{Code: "ai_brain:delete", Name: "AI大脑删除", Module: "ai_brain", Type: "button"},
		{Code: "ai_brain:suggestion", Name: "AI大脑建议处理", Module: "ai_brain", Type: "button"},
		{Code: "ai_brain:manage", Name: "AI大脑管理", Module: "ai_brain", Type: "button"},
		// Gateway
		{Code: "gateway:list", Name: "网关管理", Module: "gateway", Type: "menu"},
		{Code: "gateway:config", Name: "网关配置", Module: "gateway", Type: "button"},
		// Alarm
		{Code: "alarm:list", Name: "告警列表", Module: "alarm", Type: "menu"},
		{Code: "alarm:handle", Name: "处理告警", Module: "alarm", Type: "button"},
		{Code: "alarm:manage", Name: "告警策略管理", Module: "alarm", Type: "button"},
		{Code: "alarm:export", Name: "导出告警报告 / Export alarm reports", Module: "alarm", Type: "button"},
		// Rules
		{Code: "rule:list", Name: "规则列表", Module: "rule", Type: "menu"},
		{Code: "rule:detail", Name: "规则详情", Module: "rule", Type: "button"},
		{Code: "rule:create", Name: "创建规则", Module: "rule", Type: "button"},
		{Code: "rule:edit", Name: "编辑规则", Module: "rule", Type: "button"},
		{Code: "rule:delete", Name: "删除规则", Module: "rule", Type: "button"},
		{Code: "rule:enable", Name: "启用规则", Module: "rule", Type: "button"},
		{Code: "rule:log", Name: "规则日志", Module: "rule", Type: "button"},
		{Code: "rule_group:manage", Name: "规则分组管理", Module: "rule", Type: "button"},
		// Work Orders
		{Code: "work_order:list", Name: "工单列表", Module: "work_order", Type: "menu"},
		{Code: "work_order:create", Name: "创建工单", Module: "work_order", Type: "button"},
		{Code: "work_order:process", Name: "处理工单", Module: "work_order", Type: "button"},
		{Code: "work_order:approve", Name: "审批工单", Module: "work_order", Type: "button"},
		{Code: "work_order:manage", Name: "管理工单模板", Module: "work_order", Type: "button"},
		{Code: "work_order:integration", Name: "管理工单集成", Module: "work_order", Type: "button"},
		// History
		{Code: "history:delete", Name: "删除历史记录", Module: "history", Type: "button"},
		// Audit
		{Code: "audit:list", Name: "审计日志", Module: "audit", Type: "menu"},
		{Code: "llm_request:list", Name: "LLM请求记录 / LLM requests", Module: "llm_request", Type: "menu"},
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
		// Device Map & Geofence
		{Code: "device_map:view", Name: "设备地图 / Device map", Module: "device_map", Type: "menu"},
		{Code: "device_location:edit", Name: "标注设备位置 / Set device location", Module: "device_map", Type: "button"},
		{Code: "geofence:list", Name: "地理围栏 / Geofences", Module: "geofence", Type: "menu"},
		{Code: "geofence:create", Name: "创建地理围栏 / Create geofences", Module: "geofence", Type: "button"},
		{Code: "geofence:edit", Name: "编辑地理围栏 / Edit geofences", Module: "geofence", Type: "button"},
		{Code: "geofence:delete", Name: "删除地理围栏 / Delete geofences", Module: "geofence", Type: "button"},
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
		Code:        "admin",
		Name:        "超级管理员",
		Description: "系统默认全局角色：管理租户和系统级信息，不进入租户业务数据",
		DataScope:   1, // All data
		IsBuiltin:   true,
	}
	DB.Where("tenant_id = ? AND project_id = ? AND code = ?", 0, 0, "admin").
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

	voiceAssistantRole := Role{
		TenantID:    0,
		ProjectID:   0,
		Code:        VoiceAssistantRoleCode,
		Name:        VoiceAssistantRoleName,
		Description: "系统内置虚拟角色，用于限制语音助手操作权限。",
		DataScope:   5,
		IsBuiltin:   true,
		IsInherited: false,
	}
	DB.Where("tenant_id = ? AND project_id = ? AND code = ?", 0, 0, VoiceAssistantRoleCode).
		Assign(voiceAssistantRole).
		FirstOrCreate(&voiceAssistantRole)
	_ = ensureVoiceAssistantDefaultReadPermissions(DB, voiceAssistantRole.ID)

	var voiceAssistantUser User
	if err := DB.Where("tenant_id = ? AND username = ?", 0, VoiceAssistantUserName).First(&voiceAssistantUser).Error; err != nil {
		hashedPassword, _ := utils.HashPassword(fmt.Sprintf("voice-assistant-%d", time.Now().UnixNano()))
		voiceAssistantUser = User{
			TenantID:           0,
			Username:           VoiceAssistantUserName,
			Password:           hashedPassword,
			DisplayName:        VoiceAssistantDisplayName,
			Role:               VoiceAssistantRoleCode,
			Status:             1,
			MustChangePassword: false,
		}
		DB.Create(&voiceAssistantUser)
	} else {
		DB.Model(&voiceAssistantUser).Updates(map[string]interface{}{
			"display_name":         VoiceAssistantDisplayName,
			"role":                 VoiceAssistantRoleCode,
			"status":               1,
			"must_change_password": false,
		})
	}
	DB.Where("user_id = ? AND role_id = ? AND tenant_id = ? AND project_id = ?", voiceAssistantUser.ID, voiceAssistantRole.ID, 0, 0).
		FirstOrCreate(&UserRoleBinding{}, UserRoleBinding{
			UserID:    voiceAssistantUser.ID,
			RoleID:    voiceAssistantRole.ID,
			TenantID:  0,
			ProjectID: 0,
		})

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

			if shouldGrantProjectAdminPermission(perm) {
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

func shouldGrantProjectAdminPermission(perm Permission) bool {
	if perm.Code == "work_order:integration" {
		return false
	}
	switch perm.Module {
	case "user", "role", "product", "device", "device_tag", "gateway", "alarm", "rule", "work_order", "plugin", "ai_brain", "ai_scene", "inspection", "semantic_review", "ai_model", "ai_assistant", "device_map", "geofence", "screen":
		return true
	default:
		return false
	}
}

func EnsureVoiceAssistantScope(tx *gorm.DB, tenantID, projectID uint) error {
	if tenantID == 0 {
		return nil
	}
	if tx == nil {
		tx = DB
	}

	role := Role{
		TenantID:    tenantID,
		ProjectID:   projectID,
		Code:        VoiceAssistantRoleCode,
		Name:        VoiceAssistantRoleName,
		Description: "可配置虚拟角色，用于限制当前范围内的语音助手操作权限。",
		DataScope:   5,
		IsBuiltin:   false,
		IsInherited: false,
	}
	if err := tx.Where("tenant_id = ? AND project_id = ? AND code = ?", tenantID, projectID, VoiceAssistantRoleCode).
		Assign(role).
		FirstOrCreate(&role).Error; err != nil {
		return err
	}
	if err := ensureVoiceAssistantDefaultReadPermissions(tx, role.ID); err != nil {
		return err
	}

	var user User
	if err := tx.Where("tenant_id = ? AND username = ?", tenantID, VoiceAssistantUserName).First(&user).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		hashed, hashErr := utils.HashPassword(fmt.Sprintf("voice-assistant-%d", time.Now().UnixNano()))
		if hashErr != nil {
			return hashErr
		}
		user = User{
			TenantID:           tenantID,
			Username:           VoiceAssistantUserName,
			Password:           hashed,
			DisplayName:        VoiceAssistantDisplayName,
			Role:               VoiceAssistantRoleCode,
			Status:             1,
			MustChangePassword: false,
		}
		if err := tx.Create(&user).Error; err != nil {
			return err
		}
	} else {
		user.DisplayName = VoiceAssistantDisplayName
		user.Role = VoiceAssistantRoleCode
		user.Status = 1
		user.MustChangePassword = false
		if err := tx.Save(&user).Error; err != nil {
			return err
		}
	}

	return tx.Where("user_id = ? AND role_id = ? AND tenant_id = ? AND project_id = ?", user.ID, role.ID, tenantID, projectID).
		FirstOrCreate(&UserRoleBinding{}, UserRoleBinding{
			UserID:    user.ID,
			RoleID:    role.ID,
			TenantID:  tenantID,
			ProjectID: projectID,
		}).Error
}

func ensureVoiceAssistantDefaultReadPermissions(tx *gorm.DB, roleID uint) error {
	if tx == nil {
		tx = DB
	}
	if roleID == 0 {
		return nil
	}

	seedKey := fmt.Sprintf("voice_assistant_default_read_permissions_seeded_v2:%d", roleID)
	var marker SystemConfig
	if err := tx.Where("key = ?", seedKey).First(&marker).Error; err == nil {
		return nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	var permissions []Permission
	if err := tx.Where("code IN ?", VoiceAssistantDefaultReadPermissionCodes).Find(&permissions).Error; err != nil {
		return err
	}
	for _, permission := range permissions {
		if err := tx.Where("role_id = ? AND permission_id = ?", roleID, permission.ID).
			FirstOrCreate(&RolePermission{}, RolePermission{
				RoleID:       roleID,
				PermissionID: permission.ID,
			}).Error; err != nil {
			return err
		}
	}

	return tx.Where("key = ?", seedKey).FirstOrCreate(&SystemConfig{
		Key:   seedKey,
		Value: "true",
	}).Error
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
	p.SyncMeta = existing.SyncMeta
	p.SourceTemplateCode = existing.SourceTemplateCode
	p.SourceTemplateRev = existing.SourceTemplateRev
	return DB.Save(p).Error
}

func DeleteProduct(code string) error {
	var change *SyncChange
	err := DB.Transaction(func(tx *gorm.DB) error {
		var product Product
		if err := tx.Where("code = ?", code).First(&product).Error; err != nil {
			return err
		}
		var bindings int64
		if err := tx.Model(&Device{}).Where("product_code = ?", code).Count(&bindings).Error; err != nil {
			return err
		}
		if bindings > 0 {
			return ErrConfigResourceInUse
		}

		deleted := product
		var recordErr error
		change, recordErr = recordProductDeletedTx(tx, &deleted)
		if recordErr != nil {
			return recordErr
		}
		if deferred, err := DeferredConfigDelete(tx, change); err != nil || deferred {
			return err
		}
		newCode := fmt.Sprintf("%s_del_%d", product.Code, time.Now().UnixNano())

		// Rename code to release unique constraint
		if err := tx.Model(&product).Update("code", newCode).Error; err != nil {
			return err
		}

		// Soft delete
		if err := tx.Delete(&product).Error; err != nil {
			return err
		}
		return nil
	})
	if err == nil {
		notifyCommittedChange(change)
	}
	return err
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
	d.SyncMeta = existing.SyncMeta
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
