package cascade

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"noyo/core"
	"noyo/core/protocol"
	"noyo/core/store"
)

const (
	remotePluginMethodList      = "plugin_list"
	remotePluginMethodConfigGet = "plugin_config_get"
	remotePluginMethodConfigSet = "plugin_config_set"
	remotePluginMethodStatusSet = "plugin_status_set"

	remoteSystemMethodConfigGet     = "system_config_get"
	remoteSystemMethodConfigSet     = "system_config_set"
	remoteSystemMethodLicenseStatus = "license_status"
	remoteSystemMethodLicenseUpload = "license_upload"
	remoteSystemMethodLogFiles      = "log_files"
	remoteSystemMethodLogFile       = "log_file"
	remoteSystemMethodLogTail       = "log_tail"
)

func protectedRemoteConfigMutation(method string) bool {
	return method == remotePluginMethodConfigSet || method == remotePluginMethodStatusSet || method == remoteSystemMethodConfigSet
}

type remotePluginConfigSetRequest struct {
	Plugin      string                 `json:"plugin"`
	BaseVersion int64                  `json:"base_version"`
	Config      map[string]interface{} `json:"config"`
}

type remotePluginConfigGetRequest struct {
	Plugin string `json:"plugin"`
}

type remotePluginStatusSetRequest struct {
	Plugin      string `json:"plugin"`
	Enabled     bool   `json:"enabled"`
	BaseVersion int64  `json:"base_version"`
}

type SyncMediaNetwork struct {
	StunURLs     string `json:"stun_urls"`
	TurnURLs     string `json:"turn_urls"`
	TurnUsername string `json:"turn_username"`
	TurnPassword string `json:"turn_password"`
}

type remotePluginSummary struct {
	Name                    string                   `json:"name"`
	Title                   map[string]string        `json:"title"`
	Description             map[string]string        `json:"description"`
	Status                  string                   `json:"status"`
	Category                string                   `json:"category"`
	Icon                    string                   `json:"icon"`
	Schema                  *core.PluginConfigSchema `json:"schema"`
	Config                  map[string]interface{}   `json:"config,omitempty"`
	EditableBaseVersion     int64                    `json:"base_version"`
	Version                 string                   `json:"version"`
	ConfigVersion           int64                    `json:"configVersion"`
	UpdatedAt               int64                    `json:"updatedAt"`
	UpdatedBy               string                   `json:"updatedBy"`
	SyncState               string                   `json:"syncState"`
	BaseVersion             int64                    `json:"baseVersion"`
	GatewayVersion          int64                    `json:"gatewayVersion"`
	EnabledAt               int64                    `json:"enabledAt"`
	LastSyncedAt            int64                    `json:"lastSyncedAt"`
	IsOfflineEditable       bool                     `json:"isOfflineEditable"`
	ProtocolMappingRequired *bool                    `json:"protocolMappingRequired,omitempty"`
}

func parseRemotePluginConfigSetParams(params interface{}) (*remotePluginConfigSetRequest, error) {
	var req remotePluginConfigSetRequest
	if err := scanRemotePluginParams(params, &req); err != nil {
		return nil, err
	}
	if req.Plugin == "" {
		return nil, fmt.Errorf("plugin is required")
	}
	if req.Config == nil {
		req.Config = make(map[string]interface{})
	}
	return &req, nil
}

func parseRemotePluginConfigGetParams(params interface{}) (*remotePluginConfigGetRequest, error) {
	var req remotePluginConfigGetRequest
	if err := scanRemotePluginParams(params, &req); err != nil {
		return nil, err
	}
	if req.Plugin == "" {
		return nil, fmt.Errorf("plugin is required")
	}
	return &req, nil
}

func parseRemotePluginStatusSetParams(params interface{}) (*remotePluginStatusSetRequest, error) {
	var req remotePluginStatusSetRequest
	if err := scanRemotePluginParams(params, &req); err != nil {
		return nil, err
	}
	if req.Plugin == "" {
		return nil, fmt.Errorf("plugin is required")
	}
	return &req, nil
}

func scanRemotePluginParams(params interface{}, target interface{}) error {
	if params == nil {
		return fmt.Errorf("params are required")
	}
	data, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("marshal params: %w", err)
	}
	if err := json.Unmarshal(data, target); err != nil {
		return fmt.Errorf("parse params: %w", err)
	}
	return nil
}

func listGatewayPlugins(coreServer *core.Server) ([]remotePluginSummary, error) {
	if coreServer == nil || coreServer.Manager == nil {
		return nil, fmt.Errorf("core server not available")
	}
	plugins := coreServer.Manager.GetPlugins()
	summaries := make([]remotePluginSummary, 0, len(plugins))
	for _, plugin := range plugins {
		if plugin.GetMeta().Name == "license_auth" {
			continue
		}
		summaries = append(summaries, buildRemotePluginSummary(plugin))
	}
	return summaries, nil
}

func getGatewayPluginConfig(coreServer *core.Server, pluginName string) (*remotePluginSummary, error) {
	if coreServer == nil || coreServer.Manager == nil {
		return nil, fmt.Errorf("core server not available")
	}
	plugin := coreServer.Manager.GetPlugin(pluginName)
	if plugin == nil {
		return nil, fmt.Errorf("plugin %s not found", pluginName)
	}
	summary := buildRemotePluginSummary(plugin)
	return &summary, nil
}

func setGatewayPluginConfig(coreServer *core.Server, req *remotePluginConfigSetRequest) (*remotePluginSummary, error) {
	if coreServer == nil || coreServer.Manager == nil {
		return nil, fmt.Errorf("core server not available")
	}
	plugin := coreServer.Manager.GetPlugin(req.Plugin)
	if plugin == nil {
		return nil, fmt.Errorf("plugin %s not found", req.Plugin)
	}
	if err := core.UpdatePluginConfigAtVersion(plugin, req.Config, req.BaseVersion); err != nil {
		return nil, err
	}
	if err := coreServer.Manager.ReloadPlugin(req.Plugin); err != nil {
		return nil, err
	}
	return getGatewayPluginConfig(coreServer, req.Plugin)
}

func setGatewayPluginStatus(coreServer *core.Server, req *remotePluginStatusSetRequest) (*remotePluginSummary, error) {
	cfg := map[string]interface{}{"enabled": req.Enabled}
	return setGatewayPluginConfig(coreServer, &remotePluginConfigSetRequest{
		Plugin:      req.Plugin,
		Config:      cfg,
		BaseVersion: req.BaseVersion,
	})
}

func buildRemotePluginSummary(plugin core.IManagedPlugin) remotePluginSummary {
	meta := plugin.GetMeta()
	status := "stopped"
	if plugin.IsEnabled() {
		status = "running"
	}

	iconStr := ""
	if len(meta.Icon) > 0 {
		iconStr = "data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString(meta.Icon)
	}

	var protocolMappingRequired *bool
	if pp, ok := plugin.(protocol.IProtocolPlugin); ok {
		v := pp.ProtocolMappingRequired()
		protocolMappingRequired = &v
	}

	configVersion, updatedAt := getPluginConfigVersion(meta.Name)
	schema := core.GetPluginConfigSchema(plugin)

	return remotePluginSummary{
		Name:                    meta.Name,
		Title:                   meta.Title,
		Description:             meta.Description,
		Status:                  status,
		Category:                meta.Category,
		Icon:                    iconStr,
		Schema:                  schema,
		Config:                  schema.Config,
		EditableBaseVersion:     schema.BaseVersion,
		Version:                 meta.Version,
		ConfigVersion:           configVersion,
		UpdatedAt:               updatedAt,
		UpdatedBy:               "gateway",
		SyncState:               remotePluginSyncSynced,
		BaseVersion:             configVersion,
		GatewayVersion:          configVersion,
		EnabledAt:               map[bool]int64{true: updatedAt, false: 0}[plugin.IsEnabled()],
		LastSyncedAt:            time.Now().UnixMilli(),
		IsOfflineEditable:       true,
		ProtocolMappingRequired: protocolMappingRequired,
	}
}

func getPluginConfigVersion(pluginName string) (int64, int64) {
	version, updatedAt, _ := getPluginConfigVersionChecked(pluginName)
	return version, updatedAt
}

func getPluginConfigVersionChecked(pluginName string) (int64, int64, error) {
	var model *store.PluginModel
	var err error
	if tenantID, projectID, scoped := core.SingleProjectRuntimePluginScope(); scoped {
		model, err = store.GetPluginForScope(pluginName, tenantID, projectID)
	} else {
		model, err = store.GetPlugin(pluginName)
	}
	if err != nil {
		return 0, 0, err
	}
	if model == nil {
		return 0, 0, nil
	}
	updatedAt := model.UpdatedAt.UnixMilli()
	return model.Revision, updatedAt, nil
}

func buildRemotePluginCommand(method, pluginName string, params map[string]interface{}) ([]byte, string, error) {
	if params == nil {
		params = make(map[string]interface{})
	}
	if pluginName != "" {
		params["plugin"] = pluginName
	}
	cmdID := fmt.Sprintf("plugin_%s_%d", strings.ReplaceAll(method, "_", "-"), time.Now().UnixNano())
	payload := map[string]interface{}{
		"id":        cmdID,
		"version":   "1.0",
		"method":    method,
		"params":    params,
		"timestamp": time.Now().UnixMilli(),
	}
	payloadBytes, err := json.Marshal(payload)
	return payloadBytes, cmdID, err
}
