package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"gorm.io/gorm"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/google/uuid"

	"noyo/core/config"
	"noyo/core/store"
	"noyo/core/utils"
)

const (
	SetupModeMultiTenantPlatform  = "multi_tenant_platform"
	SetupModeMultiProjectPlatform = "multi_project_platform"
	SetupModePlatformGateway      = "platform_gateway"
	SetupModeLocalProject         = "local_project"

	legacySetupModePlatform          = "platform"
	legacySetupModeGatewayStandalone = "gateway_standalone"
	legacySetupModeGatewayManaged    = "gateway_managed"

	gatewayAdminRoleCode = "gateway_admin"
)

type setupAdminRequest struct {
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Password    string `json:"password"`
}

type setupServerRequest struct {
	Port int `json:"port"`
}

type setupLocalProjectRequest struct {
	TenantName  string `json:"tenant_name"`
	ProjectName string `json:"project_name"`
}

type setupGatewayRequest struct {
	SN       string `json:"gateway_sn"`
	Name     string `json:"gateway_name"`
	MQTTURL  string `json:"mqtt_url"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type setupMQTTAPIRequest struct {
	Enabled            bool   `json:"enabled"`
	Broker             string `json:"broker"`
	Username           string `json:"username"`
	Password           string `json:"password"`
	ClientID           string `json:"client_id"`
	GatewayCode        string `json:"gateway_code"`
	EnableTLS          bool   `json:"enable_tls"`
	InsecureSkipVerify bool   `json:"insecure_skip_verify"`
}

type setupApplyRequest struct {
	Mode         string                   `json:"mode"`
	Admin        setupAdminRequest        `json:"admin"`
	Server       setupServerRequest       `json:"server"`
	LocalProject setupLocalProjectRequest `json:"local_project"`
	Gateway      setupGatewayRequest      `json:"gateway"`
	MQTTAPI      setupMQTTAPIRequest      `json:"mqtt_api"`
	Plugins      map[string]g.Map         `json:"plugins"`
}

type setupRuntimeMode struct {
	Value       string `json:"value"`
	Label       string `json:"label"`
	Description string `json:"description"`
}

type setupStatusResponse struct {
	Initialized  bool                `json:"initialized"`
	Mode         string              `json:"mode"`
	TenantID     uint                `json:"tenant_id"`
	ProjectID    uint                `json:"project_id"`
	CompletedAt  int64               `json:"completed_at"`
	ServerPort   int                 `json:"server_port"`
	RuntimeModes []setupRuntimeMode  `json:"runtime_modes"`
	PluginSteps  []PluginSetupSchema `json:"plugin_steps"`
}

func (s *Server) handleGetSetupStatus(r *ghttp.Request) {
	mode := r.GetQuery("mode").String()
	status := s.buildSetupStatus(mode)
	r.Response.WriteJson(g.Map{"code": 0, "data": status})
}

func (s *Server) handleApplySetup(r *ghttp.Request) {
	var req setupApplyRequest
	if err := json.Unmarshal(r.GetBody(), &req); err != nil {
		r.Response.WriteJson(g.Map{"code": 400, "message": "Invalid JSON"})
		return
	}

	portChanged := false
	if req.Server.Port > 0 && s.Config != nil && s.Config.Server.Port != req.Server.Port {
		portChanged = true
	}

	if err := s.applyInitialSetup(req); err != nil {
		if errors.Is(err, errSetupAlreadyInitialized) {
			r.Response.WriteJson(g.Map{"code": 409, "message": err.Error()})
			return
		}
		r.Response.WriteJson(g.Map{"code": 400, "message": err.Error()})
		return
	}

	r.Response.WriteJson(g.Map{
		"code":    0,
		"message": "Initial setup completed.",
		"data":    s.buildSetupStatus(req.Mode),
	})

	if portChanged {
		go func() {
			s.Logger.Info("HTTP port changed in setup, exiting process to allow daemon restart...")
			time.Sleep(2 * time.Second)
			os.Exit(0)
		}()
	}
}

var errSetupAlreadyInitialized = errors.New("setup has already been completed")

func (s *Server) buildSetupStatus(mode string) setupStatusResponse {
	state, err := store.LoadSetupState()
	if err != nil {
		state = &store.SetupState{}
	}
	stateMode := normalizeSetupMode(state.Mode)
	if mode == "" {
		mode = stateMode
	}
	mode = normalizeSetupMode(mode)

	status := setupStatusResponse{
		Initialized: state.Initialized,
		Mode:        stateMode,
		TenantID:    state.TenantID,
		ProjectID:   state.ProjectID,
		CompletedAt: state.CompletedAt,
		RuntimeModes: []setupRuntimeMode{
			{Value: SetupModeMultiTenantPlatform, Label: "多租户运营平台", Description: "面向平台运营或集团化场景，保留完整租户隔离与多项目 RBAC。"},
			{Value: SetupModeMultiProjectPlatform, Label: "多项目管理平台", Description: "面向单一组织的多项目管理，底层使用隐藏默认租户承载权限边界。"},
			{Value: SetupModePlatformGateway, Label: "平台接入网关", Description: "边缘侧单项目网关，通过级联 MQTT 接入上级平台。"},
			{Value: SetupModeLocalProject, Label: "本地单项目管理", Description: "小项目本地独立运行，单项目完成接入与设备管理。"},
		},
		PluginSteps: collectSetupPluginSchemas(s.Manager, mode),
	}
	if status.Mode == "" {
		status.Mode = mode
	}
	if s != nil && s.Config != nil {
		status.ServerPort = s.Config.Server.Port
	}
	return status
}

func collectSetupPluginSchemas(manager *PluginManager, mode string) []PluginSetupSchema {
	if manager == nil {
		return []PluginSetupSchema{}
	}

	steps := make([]PluginSetupSchema, 0)
	for _, plugin := range manager.GetPlugins() {
		if !isFirstRunSetupPlugin(plugin) {
			continue
		}
		provider, ok := plugin.(ISetupSchemaProvider)
		if !ok {
			continue
		}
		schema := provider.GetSetupSchema(mode)
		if schema == nil {
			continue
		}
		steps = append(steps, *schema)
	}
	return steps
}

func isFirstRunSetupPlugin(plugin IManagedPlugin) bool {
	if plugin == nil || plugin.GetMeta() == nil {
		return false
	}
	return strings.EqualFold(plugin.GetMeta().Name, "cascade")
}

func (s *Server) applyInitialSetup(req setupApplyRequest) error {
	mode := normalizeSetupMode(req.Mode)
	if !isValidSetupMode(mode) {
		return fmt.Errorf("unsupported setup mode: %s", mode)
	}
	req.Mode = mode
	mergeSetupPluginPayloads(&req)

	state, err := store.LoadSetupState()
	if err != nil {
		return err
	}
	if state.Initialized {
		return errSetupAlreadyInitialized
	}

	if err := validateSetupRequest(req, mode); err != nil {
		return err
	}

	if mode == SetupModePlatformGateway {
		if err := verifyGatewayRegistration(req.Gateway, req.LocalProject); err != nil {
			return err
		}
	}

	var pluginsToReload []string
	err = store.DB.Transaction(func(tx *gorm.DB) error {
		cfg := s.effectiveConfig()
		if req.Server.Port > 0 {
			cfg.Server.Port = req.Server.Port
		}
		if err := saveGlobalConfigInTx(tx, cfg); err != nil {
			return err
		}

		var tenantID, projectID uint
		if isSingleProjectSetupMode(mode) {
			tenant, project, err := createGatewayLocalScope(tx, req.LocalProject)
			if err != nil {
				return err
			}
			tenantID = tenant.ID
			projectID = project.ID

			admin, err := configureInitialAdminInTx(tx, req.Admin, tenantID, RoleCodeSuperAdmin)
			if err != nil {
				return err
			}
			if err := bootstrapGatewayAdminRBAC(tx, admin.ID, tenantID, projectID); err != nil {
				return err
			}
		} else if mode == SetupModeMultiProjectPlatform {
			tenant, err := createHiddenDefaultTenantScope(tx, req.LocalProject)
			if err != nil {
				return err
			}
			tenantID = tenant.ID

			admin, err := configureInitialAdminInTx(tx, req.Admin, tenantID, RoleCodeSuperAdmin)
			if err != nil {
				return err
			}
			if err := bootstrapMultiProjectPlatformAdminRBAC(tx, admin.ID, tenantID); err != nil {
				return err
			}
		} else if err := configurePlatformAdminInTx(tx, req.Admin); err != nil {
			return err
		}

		if mode == SetupModePlatformGateway {
			cascadeConfig := cascadeGatewayConfig(req.Gateway, req.LocalProject)
			if err := savePluginConfigInTx(tx, "cascade", 0, 0, true, cascadeConfig); err != nil {
				return err
			}
			if err := savePluginConfigInTx(tx, "cascade", tenantID, projectID, true, cascadeConfig); err != nil {
				return err
			}
			pluginsToReload = append(pluginsToReload, "cascade")
		} else if isPlatformSetupMode(mode) && strings.TrimSpace(req.Gateway.MQTTURL) != "" {
			cascadeConfig := cascadePlatformConfig(req.Gateway)
			if err := savePluginConfigInTx(tx, "cascade", 0, 0, true, cascadeConfig); err != nil {
				return err
			}
			pluginsToReload = append(pluginsToReload, "cascade")
		}

		if shouldConfigureMQTTAPI(req.MQTTAPI) {
			mqttAPIConfig := mqttAPIConfig(req.MQTTAPI, req.Gateway.SN)
			if err := savePluginConfigInTx(tx, "MQTT_API", 0, 0, req.MQTTAPI.Enabled, mqttAPIConfig); err != nil {
				return err
			}
			if tenantID > 0 && projectID > 0 {
				if err := savePluginConfigInTx(tx, "MQTT_API", tenantID, projectID, req.MQTTAPI.Enabled, mqttAPIConfig); err != nil {
					return err
				}
			}
			pluginsToReload = append(pluginsToReload, "MQTT_API")
		}
		if err := saveGenericSetupPluginConfigs(tx, req.Plugins, tenantID, projectID, &pluginsToReload); err != nil {
			return err
		}

		setupState := &store.SetupState{
			Initialized: true,
			Mode:        mode,
			TenantID:    tenantID,
			ProjectID:   projectID,
			CompletedAt: time.Now().UnixMilli(),
		}
		return saveSetupStateInTx(tx, setupState)
	})
	if err != nil {
		return err
	}

	if s != nil {
		s.Config = s.effectiveConfig()
		if req.Server.Port > 0 {
			s.Config.Server.Port = req.Server.Port
		}
		s.reloadConfiguredPlugins(pluginsToReload)
	}
	return nil
}

func mergeSetupPluginPayloads(req *setupApplyRequest) {
	if req == nil || len(req.Plugins) == 0 {
		return
	}

	if cfg := setupPluginConfig(req.Plugins, "cascade"); cfg != nil {
		req.Gateway.SN = stringFromSetupConfig(cfg, "gateway_sn", req.Gateway.SN)
		req.Gateway.Name = stringFromSetupConfig(cfg, "gateway_name", req.Gateway.Name)
		req.Gateway.MQTTURL = stringFromSetupConfig(cfg, "mqtt_url", req.Gateway.MQTTURL)
		req.Gateway.Username = stringFromSetupConfig(cfg, "username", req.Gateway.Username)
		req.Gateway.Password = stringFromSetupConfig(cfg, "password", req.Gateway.Password)
	}

	if cfg := setupPluginConfig(req.Plugins, "MQTT_API"); cfg != nil {
		req.MQTTAPI.Enabled = boolFromSetupConfig(cfg, "enabled", req.MQTTAPI.Enabled)
		req.MQTTAPI.Broker = stringFromSetupConfig(cfg, "broker", req.MQTTAPI.Broker)
		req.MQTTAPI.Username = stringFromSetupConfig(cfg, "username", req.MQTTAPI.Username)
		req.MQTTAPI.Password = stringFromSetupConfig(cfg, "password", req.MQTTAPI.Password)
		req.MQTTAPI.ClientID = stringFromSetupConfig(cfg, "client_id", req.MQTTAPI.ClientID)
		req.MQTTAPI.GatewayCode = stringFromSetupConfig(cfg, "gateway_code", req.MQTTAPI.GatewayCode)
		req.MQTTAPI.EnableTLS = boolFromSetupConfig(cfg, "enable_tls", req.MQTTAPI.EnableTLS)
		req.MQTTAPI.InsecureSkipVerify = boolFromSetupConfig(cfg, "insecure_skip_verify", req.MQTTAPI.InsecureSkipVerify)
	}
}

func isValidSetupMode(mode string) bool {
	switch mode {
	case SetupModeMultiTenantPlatform, SetupModeMultiProjectPlatform, SetupModePlatformGateway, SetupModeLocalProject:
		return true
	default:
		return false
	}
}

func normalizeSetupMode(mode string) string {
	switch strings.TrimSpace(mode) {
	case "", legacySetupModePlatform, SetupModeMultiTenantPlatform:
		return SetupModeMultiTenantPlatform
	case legacySetupModeGatewayManaged, SetupModePlatformGateway:
		return SetupModePlatformGateway
	case legacySetupModeGatewayStandalone, SetupModeLocalProject:
		return SetupModeLocalProject
	case SetupModeMultiProjectPlatform:
		return SetupModeMultiProjectPlatform
	default:
		return strings.TrimSpace(mode)
	}
}

func NormalizeSetupMode(mode string) string {
	return normalizeSetupMode(mode)
}

func isPlatformSetupMode(mode string) bool {
	mode = normalizeSetupMode(mode)
	return mode == SetupModeMultiTenantPlatform || mode == SetupModeMultiProjectPlatform
}

func isSingleProjectSetupMode(mode string) bool {
	mode = normalizeSetupMode(mode)
	return mode == SetupModePlatformGateway || mode == SetupModeLocalProject
}

func IsSingleProjectSetupMode(mode string) bool {
	return isSingleProjectSetupMode(mode)
}

func validateSetupRequest(req setupApplyRequest, mode string) error {
	if req.Server.Port > 0 && (req.Server.Port < 1 || req.Server.Port > 65535) {
		return fmt.Errorf("server port must be between 1 and 65535")
	}
	if strings.TrimSpace(req.Admin.Password) == "" {
		return fmt.Errorf("admin password is required")
	}
	if err := validatePasswordStrength(req.Admin.Password); err != nil {
		return err
	}
	if mode == SetupModePlatformGateway {
		if strings.TrimSpace(req.Gateway.SN) == "" {
			return fmt.Errorf("gateway SN is required for platform gateway mode")
		}
		if strings.TrimSpace(req.Gateway.MQTTURL) == "" {
			return fmt.Errorf("MQTT URL is required for platform gateway mode")
		}
	}
	return nil
}

func (s *Server) effectiveConfig() *config.GlobalConfig {
	if s != nil && s.Config != nil {
		cfg := *s.Config
		return &cfg
	}
	return config.DefaultConfig()
}

func createGatewayLocalScope(tx *gorm.DB, req setupLocalProjectRequest) (*store.Tenant, *store.Project, error) {
	tenant := &store.Tenant{
		Code:        "local_gateway",
		Name:        valueOrDefaultString(req.TenantName, "Local Gateway"),
		LoginSuffix: "local",
	}
	if err := tx.Create(tenant).Error; err != nil {
		return nil, nil, err
	}

	project := &store.Project{
		TenantID: tenant.ID,
		Code:     "default",
		Name:     valueOrDefaultString(req.ProjectName, "Default Project"),
	}
	if err := tx.Create(project).Error; err != nil {
		return nil, nil, err
	}
	return tenant, project, nil
}

func createHiddenDefaultTenantScope(tx *gorm.DB, req setupLocalProjectRequest) (*store.Tenant, error) {
	tenant := &store.Tenant{
		Code:        "default",
		Name:        valueOrDefaultString(req.TenantName, "Default Organization"),
		LoginSuffix: "default",
	}
	if err := tx.Create(tenant).Error; err != nil {
		return nil, err
	}
	return tenant, nil
}

func configureInitialAdminInTx(tx *gorm.DB, req setupAdminRequest, tenantID uint, role string) (*store.User, error) {
	var admin store.User
	if err := tx.Order("id asc").First(&admin).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		admin = store.User{}
	}

	hashed, err := utils.HashPassword(req.Password)
	if err != nil {
		return nil, err
	}
	admin.TenantID = tenantID
	admin.Username = valueOrDefaultString(req.Username, "admin")
	admin.DisplayName = valueOrDefaultString(req.DisplayName, "Administrator")
	admin.Password = hashed
	admin.Role = role
	admin.Status = 1
	admin.MustChangePassword = false

	if admin.ID == 0 {
		if err := tx.Create(&admin).Error; err != nil {
			return nil, err
		}
	} else if err := tx.Save(&admin).Error; err != nil {
		return nil, err
	}
	return &admin, nil
}

func configurePlatformAdminInTx(tx *gorm.DB, req setupAdminRequest) error {
	admin, err := configureInitialAdminInTx(tx, req, 0, RoleCodeSuperAdmin)
	if err != nil {
		return err
	}

	var superAdminRole store.Role
	if err := tx.Where("code = ? AND tenant_id = ? AND project_id = ?", RoleCodeSuperAdmin, 0, 0).First(&superAdminRole).Error; err != nil {
		return err
	}
	return tx.FirstOrCreate(&store.UserRoleBinding{}, store.UserRoleBinding{
		UserID:    admin.ID,
		RoleID:    superAdminRole.ID,
		TenantID:  0,
		ProjectID: 0,
	}).Error
}

func bootstrapGatewayAdminRBAC(tx *gorm.DB, userID, tenantID, projectID uint) error {
	if err := tx.Unscoped().Where("user_id = ?", userID).Delete(&store.UserRoleBinding{}).Error; err != nil {
		return err
	}

	var superAdminRole store.Role
	if err := tx.Where("code = ? AND tenant_id = ? AND project_id = ?", RoleCodeSuperAdmin, 0, 0).First(&superAdminRole).Error; err == nil {
		if err := tx.Create(&store.UserRoleBinding{
			UserID:    userID,
			RoleID:    superAdminRole.ID,
			TenantID:  0,
			ProjectID: 0,
		}).Error; err != nil {
			return err
		}
	}

	var tenantAdminRole store.Role
	if err := tx.Where("code = ? AND tenant_id = ? AND project_id = ?", RoleCodeTenantAdmin, 0, 0).First(&tenantAdminRole).Error; err != nil {
		return err
	}
	if err := tx.Create(&store.UserRoleBinding{
		UserID:    userID,
		RoleID:    tenantAdminRole.ID,
		TenantID:  tenantID,
		ProjectID: 0,
	}).Error; err != nil {
		return err
	}

	var projectAdminRole store.Role
	if err := tx.Where("code = ? AND tenant_id = ? AND project_id = ?", RoleCodeProjectAdmin, 0, 0).First(&projectAdminRole).Error; err != nil {
		return err
	}
	if err := tx.Create(&store.UserRoleBinding{
		UserID:    userID,
		RoleID:    projectAdminRole.ID,
		TenantID:  tenantID,
		ProjectID: projectID,
	}).Error; err != nil {
		return err
	}

	var gatewayRole store.Role
	if err := tx.Where("code = ? AND tenant_id = ? AND project_id = ?", gatewayAdminRoleCode, tenantID, projectID).First(&gatewayRole).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		gatewayRole = store.Role{
			TenantID:    tenantID,
			ProjectID:   projectID,
			Code:        gatewayAdminRoleCode,
			Name:        "Local Gateway Admin",
			Description: "Full local permissions for gateway standalone or managed mode.",
			DataScope:   1,
			IsBuiltin:   false,
		}
		if err := tx.Create(&gatewayRole).Error; err != nil {
			return err
		}
	}

	permissionIDs, err := allPermissionIDs(tx)
	if err != nil {
		return err
	}
	for _, permissionID := range permissionIDs {
		if err := tx.FirstOrCreate(&store.RolePermission{}, store.RolePermission{
			RoleID:       gatewayRole.ID,
			PermissionID: permissionID,
		}).Error; err != nil {
			return err
		}
	}
	if err := tx.Create(&store.UserRoleBinding{
		UserID:    userID,
		RoleID:    gatewayRole.ID,
		TenantID:  tenantID,
		ProjectID: projectID,
	}).Error; err != nil {
		return err
	}
	if err := replaceScopePermissionLimit(tx, permissionLimitScopeTenant, tenantID, 0, permissionIDs); err != nil {
		return err
	}
	return replaceScopePermissionLimit(tx, permissionLimitScopeProject, tenantID, projectID, permissionIDs)
}

func bootstrapMultiProjectPlatformAdminRBAC(tx *gorm.DB, userID, tenantID uint) error {
	if err := tx.Unscoped().Where("user_id = ?", userID).Delete(&store.UserRoleBinding{}).Error; err != nil {
		return err
	}

	var superAdminRole store.Role
	if err := tx.Where("code = ? AND tenant_id = ? AND project_id = ?", RoleCodeSuperAdmin, 0, 0).First(&superAdminRole).Error; err != nil {
		return err
	}
	if err := tx.Create(&store.UserRoleBinding{
		UserID:    userID,
		RoleID:    superAdminRole.ID,
		TenantID:  0,
		ProjectID: 0,
	}).Error; err != nil {
		return err
	}

	var tenantAdminRole store.Role
	if err := tx.Where("code = ? AND tenant_id = ? AND project_id = ?", RoleCodeTenantAdmin, 0, 0).First(&tenantAdminRole).Error; err != nil {
		return err
	}
	if err := tx.Create(&store.UserRoleBinding{
		UserID:    userID,
		RoleID:    tenantAdminRole.ID,
		TenantID:  tenantID,
		ProjectID: 0,
	}).Error; err != nil {
		return err
	}

	permissionIDs, err := allPermissionIDs(tx)
	if err != nil {
		return err
	}
	return replaceScopePermissionLimit(tx, permissionLimitScopeTenant, tenantID, 0, permissionIDs)
}

func allPermissionIDs(tx *gorm.DB) ([]uint, error) {
	permissionIDs := make([]uint, 0)
	if err := tx.Model(&store.Permission{}).Order("id asc").Pluck("id", &permissionIDs).Error; err != nil {
		return nil, err
	}
	return permissionIDs, nil
}

func cascadeGatewayConfig(req setupGatewayRequest, localProject setupLocalProjectRequest) map[string]interface{} {
	return map[string]interface{}{
		"enabled":      true,
		"mode":         "gateway",
		"mqtt_url":     strings.TrimSpace(req.MQTTURL),
		"username":     strings.TrimSpace(req.Username),
		"password":     req.Password,
		"gateway_sn":   strings.TrimSpace(req.SN),
		"gateway_name": valueOrDefaultString(req.Name, strings.TrimSpace(req.SN)),
		"tenant_name":  localProject.TenantName,
		"project_name": localProject.ProjectName,
	}
}

func cascadePlatformConfig(req setupGatewayRequest) map[string]interface{} {
	return map[string]interface{}{
		"enabled":      true,
		"mode":         "platform",
		"mqtt_url":     strings.TrimSpace(req.MQTTURL),
		"username":     strings.TrimSpace(req.Username),
		"password":     req.Password,
		"gateway_sn":   "",
		"gateway_name": "",
	}
}

func shouldConfigureMQTTAPI(req setupMQTTAPIRequest) bool {
	return req.Enabled ||
		strings.TrimSpace(req.Broker) != "" ||
		strings.TrimSpace(req.Username) != "" ||
		strings.TrimSpace(req.Password) != "" ||
		strings.TrimSpace(req.ClientID) != "" ||
		strings.TrimSpace(req.GatewayCode) != ""
}

func mqttAPIConfig(req setupMQTTAPIRequest, fallbackGatewayCode string) map[string]interface{} {
	return map[string]interface{}{
		"enabled":              req.Enabled,
		"enable_tls":           req.EnableTLS,
		"insecure_skip_verify": req.InsecureSkipVerify,
		"broker":               strings.TrimSpace(req.Broker),
		"username":             strings.TrimSpace(req.Username),
		"password":             req.Password,
		"client_id":            strings.TrimSpace(req.ClientID),
		"gateway_code":         valueOrDefaultString(req.GatewayCode, fallbackGatewayCode),
	}
}

func saveGenericSetupPluginConfigs(tx *gorm.DB, pluginConfigs map[string]g.Map, tenantID, projectID uint, pluginsToReload *[]string) error {
	for name, cfg := range pluginConfigs {
		name = strings.TrimSpace(name)
		if name == "" || len(cfg) == 0 || name == "cascade" || name == "MQTT_API" {
			continue
		}
		enabled := boolFromSetupConfig(cfg, "enabled", false)
		if err := savePluginConfigInTx(tx, name, 0, 0, enabled, cfg); err != nil {
			return err
		}
		if tenantID > 0 && projectID > 0 {
			if err := savePluginConfigInTx(tx, name, tenantID, projectID, enabled, cfg); err != nil {
				return err
			}
		}
		if pluginsToReload != nil {
			*pluginsToReload = append(*pluginsToReload, name)
		}
	}
	return nil
}

func setupPluginConfig(configs map[string]g.Map, name string) g.Map {
	if configs == nil {
		return nil
	}
	if cfg, ok := configs[name]; ok {
		return cfg
	}
	for key, cfg := range configs {
		if strings.EqualFold(key, name) {
			return cfg
		}
	}
	return nil
}

func stringFromSetupConfig(cfg g.Map, key, fallback string) string {
	value, ok := cfg[key]
	if !ok || value == nil {
		return fallback
	}
	switch v := value.(type) {
	case string:
		if strings.TrimSpace(v) == "" {
			return fallback
		}
		return v
	default:
		text := fmt.Sprint(v)
		if strings.TrimSpace(text) == "" {
			return fallback
		}
		return text
	}
}

func boolFromSetupConfig(cfg g.Map, key string, fallback bool) bool {
	value, ok := cfg[key]
	if !ok || value == nil {
		return fallback
	}
	switch v := value.(type) {
	case bool:
		return v
	case string:
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "1", "true", "yes", "on":
			return true
		case "0", "false", "no", "off":
			return false
		default:
			return fallback
		}
	case float64:
		return v != 0
	case int:
		return v != 0
	default:
		return fallback
	}
}

func saveGlobalConfigInTx(tx *gorm.DB, cfg *config.GlobalConfig) error {
	return saveSystemConfigValueInTx(tx, "global_config", cfg)
}

func saveSetupStateInTx(tx *gorm.DB, state *store.SetupState) error {
	return saveSystemConfigValueInTx(tx, "setup_state", state)
}

func saveSystemConfigValueInTx(tx *gorm.DB, key string, value interface{}) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}

	var sysConfig store.SystemConfig
	err = tx.Where("key = ?", key).First(&sysConfig).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return tx.Create(&store.SystemConfig{Key: key, Value: string(data)}).Error
		}
		return err
	}
	return tx.Model(&sysConfig).Update("value", string(data)).Error
}

func savePluginConfigInTx(tx *gorm.DB, name string, tenantID, projectID uint, enabled bool, cfg interface{}) error {
	data, err := json.Marshal(cfg)
	if err != nil {
		return err
	}

	var plugin store.PluginModel
	err = tx.Where("name = ? AND tenant_id = ? AND project_id = ?", name, tenantID, projectID).First(&plugin).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			plugin = store.PluginModel{Name: name, TenantID: tenantID, ProjectID: projectID}
		} else {
			return err
		}
	}
	plugin.Enabled = enabled
	plugin.Config = string(data)
	return tx.Save(&plugin).Error
}

func (s *Server) reloadConfiguredPlugins(pluginNames []string) {
	if s == nil || s.Manager == nil {
		return
	}
	seen := map[string]bool{}
	for _, name := range pluginNames {
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		if s.Manager.GetPlugin(name) != nil {
			_ = s.Manager.ReloadPlugin(name)
			continue
		}
		_ = s.Manager.LoadPlugin(name)
	}
}

func valueOrDefaultString(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value != "" {
		return value
	}
	return fallback
}

func verifyGatewayRegistration(gateway setupGatewayRequest, localProject setupLocalProjectRequest) error {
	opts := mqtt.NewClientOptions().AddBroker(gateway.MQTTURL)
	clientID := fmt.Sprintf("noyo-setup-verify-%s", uuid.New().String()[:8])
	opts.SetClientID(clientID)
	opts.SetUsername(gateway.Username)
	opts.SetPassword(gateway.Password)
	opts.SetConnectTimeout(5 * time.Second)

	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		return fmt.Errorf("failed to connect to platform MQTT broker: %v", token.Error())
	}
	defer client.Disconnect(250)

	respChan := make(chan string, 1)

	respTopic := fmt.Sprintf("noyo/cascade/gw/%s/register/response", gateway.SN)
	if token := client.Subscribe(respTopic, 1, func(c mqtt.Client, msg mqtt.Message) {
		var resp struct {
			Status  string `json:"status"`
			Message string `json:"message"`
		}
		if err := json.Unmarshal(msg.Payload(), &resp); err == nil {
			if resp.Status == "error" {
				respChan <- fmt.Sprintf("Registration failed: %s", resp.Message)
			} else {
				respChan <- ""
			}
		}
	}); token.Wait() && token.Error() != nil {
		return fmt.Errorf("failed to subscribe to register response: %v", token.Error())
	}

	reqPayload := map[string]string{
		"gateway_name": gateway.Name,
		"tenant_name":  localProject.TenantName,
		"project_name": localProject.ProjectName,
	}
	payloadBytes, _ := json.Marshal(reqPayload)
	reqTopic := fmt.Sprintf("noyo/cascade/gw/%s/register/request", gateway.SN)

	if token := client.Publish(reqTopic, 1, false, payloadBytes); token.Wait() && token.Error() != nil {
		return fmt.Errorf("failed to publish register request: %v", token.Error())
	}

	select {
	case errStr := <-respChan:
		if errStr != "" {
			return errors.New(errStr)
		}
		return nil
	case <-time.After(10 * time.Second):
		return errors.New("timeout waiting for platform registration verification")
	}
}
