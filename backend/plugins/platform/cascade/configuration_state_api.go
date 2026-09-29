package cascade

import (
	"encoding/json"
	"regexp"
	"strings"
	"time"

	"github.com/gogf/gf/v2/net/ghttp"
	"gorm.io/gorm"
	"noyo/core"
	"noyo/core/store"
)

func configurationPermission(kind string) string {
	switch kind {
	case store.SyncEntityDevice:
		return "device:list"
	case store.SyncEntityProduct, store.SyncEntityDriver:
		return "product:list"
	case store.SyncEntityRule:
		return "rule:list"
	case store.SyncEntityPluginConfig, store.SyncEntityNodeConfig:
		return "gateway:config"
	}
	return ""
}

// Configuration status uses the same tenant/project restrictions as the resource
// APIs. Runtime rows are joined to an authorized projection, never listed globally.
func configurationScope(tx *gorm.DB, principal *core.AuthContext, tenant, project uint) *gorm.DB {
	if !principal.IsSystemAdmin {
		tx = tx.Where("tenant_id = ?", principal.TenantID)
		if ids, restricted := principal.ProjectIDsForTenantQuery(); restricted {
			tx = tx.Where("project_id IN ?", ids)
		}
	}
	if tenant != 0 {
		tx = tx.Where("tenant_id = ?", tenant)
	}
	if project != 0 {
		tx = tx.Where("project_id = ?", project)
	}
	return tx
}

func redactConfigurationPayload(payload string) string {
	var value interface{}
	if json.Unmarshal([]byte(payload), &value) != nil {
		return ""
	}
	var scrub func(interface{}) interface{}
	scrub = func(v interface{}) interface{} {
		switch item := v.(type) {
		case map[string]interface{}:
			for key, field := range item {
				name := strings.NewReplacer("_", "", "-", "").Replace(strings.ToLower(key))
				if strings.Contains(name, "password") || strings.Contains(name, "secret") || strings.Contains(name, "token") || strings.Contains(name, "privatekey") || strings.Contains(name, "credential") || name == "apikey" {
					item[key] = "[redacted]"
					continue
				}
				item[key] = scrub(field)
			}
		case []interface{}:
			for i := range item {
				item[i] = scrub(item[i])
			}
		case string:
			// Some plugin configurations store nested JSON as text.
			var nested interface{}
			if (strings.HasPrefix(strings.TrimSpace(item), "{") || strings.HasPrefix(strings.TrimSpace(item), "[")) && json.Unmarshal([]byte(item), &nested) == nil {
				data, _ := json.Marshal(scrub(nested))
				return string(data)
			}
		}
		return v
	}
	data, _ := json.Marshal(scrub(value))
	return string(data)
}

var configurationErrorCredential = regexp.MustCompile(`(?i)("?(?:password|secret|token|api[_-]?key|private[_-]?key|credential)"?\s*[:=]\s*)("[^"\n]*"|'[^'\n]*'|[^\s,;]+)`)
var configurationErrorURLCredential = regexp.MustCompile(`(://)[^\s/@]+:[^\s/@]+@`)

func redactConfigurationError(detail string) string {
	detail = configurationErrorCredential.ReplaceAllString(detail, "${1}[redacted]")
	return configurationErrorURLCredential.ReplaceAllString(detail, "${1}[redacted]@")
}

func (p *CascadePlugin) handleConfigurationStates(r *ghttp.Request) {
	if r.Method != "GET" {
		r.Response.WriteStatus(405)
		return
	}
	kind := r.GetQuery("entity_type").String()
	permission := configurationPermission(kind)
	principal := core.RequestAuthContext(r)
	if permission == "" {
		r.Response.WriteStatus(400, map[string]interface{}{"code": 400, "message": "Unsupported resource type"})
		return
	}
	if principal == nil || !principal.HasPermission(permission) {
		r.Response.WriteStatus(403)
		return
	}
	tenant, project := r.GetCtxVar("tenant_id").Uint(), r.GetCtxVar("project_id").Uint()
	code, gateway := r.GetQuery("entity_code").String(), r.GetQuery("gateway_code").String()
	summary := r.GetQuery("summary").Bool()
	var codes []string
	bounded := !r.GetQuery("entity_codes").IsNil()
	if bounded {
		if err := json.Unmarshal([]byte(r.GetQuery("entity_codes").String()), &codes); err != nil || len(codes) > 500 {
			r.Response.WriteStatus(400)
			return
		}
	}
	active := p.getActivePlugin()
	projections := []store.ConfigProjection{}
	intents := []store.ConfigIntent{}
	runtime := []store.GatewayRuntimeState{}
	local := []store.RuntimeApply{}
	query := configurationScope(store.DB.Model(&store.ConfigProjection{}), principal, tenant, project).Where("entity_type = ?", kind)
	if code != "" {
		query = query.Where("entity_code = ?", code)
	}
	if gateway != "" {
		query = query.Where("gateway_code = ?", gateway)
	}
	if bounded {
		query = query.Where("entity_code IN ?", codes)
	}
	if summary {
		query = query.Omit("payload")
	}
	if err := query.Order("entity_code asc").Find(&projections).Error; err != nil {
		r.Response.WriteStatus(500)
		return
	}
	if engine, ok := active.GatewayEngine.(*gatewayEngineImpl); ok && active.Config.Mode == "gateway" && (gateway == "" || gateway == engine.gatewayCode) {
		if summary && configEntity(kind) != nil {
			rows, err := localConfigurationSummary(engine, kind, principal, tenant, project, code, codes, bounded)
			if err != nil {
				r.Response.WriteStatus(500)
				return
			}
			projections = append(projections, rows...)
		} else {
			snapshot, err := buildGatewaySnapshot(engine.gatewayCode, engine.recorder.instanceID, engine.config.TenantID, engine.config.ProjectID)
			if err != nil {
				r.Response.WriteStatus(500)
				return
			}
			for _, ch := range snapshot.Resources {
				if ch.EntityType != kind || (code != "" && ch.EntityCode != code) {
					continue
				}
				if bounded && !configurationCodeIncluded(codes, ch.EntityCode) {
					continue
				}
				if !principal.IsSystemAdmin && (ch.TenantID != principal.TenantID || (!principal.IsTenantAdmin && !principal.CanAccessProject(ch.ProjectID))) {
					continue
				}
				if tenant != 0 && tenant != ch.TenantID || project != 0 && project != ch.ProjectID {
					continue
				}
				projections = append(projections, store.ConfigProjection{GatewayCode: engine.gatewayCode, EntityType: kind, EntityCode: ch.EntityCode, TenantID: ch.TenantID, ProjectID: ch.ProjectID, Payload: ch.Payload, Revision: ch.NewRev, LifecycleVersion: ch.LifecycleVersion, Deleted: ch.Op == store.SyncOpDelete, ConfirmedAt: time.Now().UnixMilli()})
			}
		}
	}
	query = configurationScope(store.DB.Model(&store.ConfigIntent{}), principal, tenant, project).Where("entity_type = ?", kind)
	if code != "" {
		query = query.Where("entity_code = ?", code)
	}
	if gateway != "" {
		query = query.Where("gateway_code = ?", gateway)
	}
	if summary {
		// Keep only the latest intent per persisted target group. The frontend
		// combines overlapping multi-gateway targets without loading their history.
		query = query.Where(`NOT EXISTS (SELECT 1 FROM config_intents newer
			WHERE newer.entity_type = config_intents.entity_type AND newer.entity_code = config_intents.entity_code
			AND newer.tenant_id = config_intents.tenant_id AND newer.project_id = config_intents.project_id
			AND newer.gateway_code = config_intents.gateway_code AND newer.target_node = config_intents.target_node
			AND (newer.created_at > config_intents.created_at OR (newer.created_at = config_intents.created_at AND newer.sequence > config_intents.sequence)
			OR (newer.created_at = config_intents.created_at AND newer.sequence = config_intents.sequence AND newer.operation_code > config_intents.operation_code)))`)
	}
	if bounded {
		if summary && r.GetQuery("include_pending_creates").Bool() {
			// A proposed creation has no business row and cannot be in page codes.
			// Keep it discoverable until a gateway confirms the resource.
			query = query.Where(`entity_code IN ? OR (kind <> ? AND status <> ? AND base_payload IN ?
				AND NOT EXISTS (SELECT 1 FROM config_projections confirmed WHERE confirmed.entity_type = config_intents.entity_type
				AND confirmed.entity_code = config_intents.entity_code AND confirmed.tenant_id = config_intents.tenant_id
				AND confirmed.project_id = config_intents.project_id AND confirmed.deleted = ?))`, codes, store.SyncOpDelete, "committed", []string{"", "{}", "null"}, false)
		} else {
			query = query.Where("entity_code IN ?", codes)
		}
	}
	if err := query.Order("created_at desc").Find(&intents).Error; err != nil {
		r.Response.WriteStatus(500)
		return
	}
	filteredProjections := projections[:0]
	authorized := make(map[string]map[string]bool)
	for _, row := range projections {
		if !principal.HasProjectPermission(permission, row.ProjectID) {
			continue
		}
		row.Payload = redactConfigurationPayload(row.Payload)
		filteredProjections = append(filteredProjections, row)
		if authorized[row.EntityCode] == nil {
			authorized[row.EntityCode] = make(map[string]bool)
		}
		authorized[row.EntityCode][row.GatewayCode] = true
	}
	authorizedCodes := make([]string, 0, len(authorized))
	for value := range authorized {
		authorizedCodes = append(authorizedCodes, value)
	}
	if len(authorizedCodes) > 0 {
		if err := store.DB.Where("entity_type = ? AND entity_code IN ?", kind, authorizedCodes).Find(&runtime).Error; err != nil {
			r.Response.WriteStatus(500)
			return
		}
		filtered := runtime[:0]
		for _, row := range runtime {
			if authorized[row.EntityCode][row.GatewayCode] {
				filtered = append(filtered, row)
			}
		}
		runtime = filtered
		if active.Config.Mode == "gateway" {
			query := store.DB.Where("entity_type = ? AND entity_code IN ?", kind, authorizedCodes)
			if summary {
				query = query.Omit("payload", "active_payload")
			}
			if err := query.Find(&local).Error; err != nil {
				r.Response.WriteStatus(500)
				return
			}
		}
	}
	for i := range runtime {
		runtime[i].LastError = redactConfigurationError(runtime[i].LastError)
		runtime[i].CompensationError = redactConfigurationError(runtime[i].CompensationError)
	}
	for i := range local {
		local[i].LastError = redactConfigurationError(local[i].LastError)
		local[i].CompensationError = redactConfigurationError(local[i].CompensationError)
		local[i].Payload = redactConfigurationPayload(local[i].Payload)
		local[i].ActivePayload = redactConfigurationPayload(local[i].ActivePayload)
	}
	filteredIntents := intents[:0]
	for _, row := range intents {
		if principal.HasProjectPermission(permission, row.ProjectID) {
			row.Payload = redactConfigurationPayload(row.Payload)
			row.BasePayload = redactConfigurationPayload(row.BasePayload)
			row.LastError = redactConfigurationError(row.LastError)
			filteredIntents = append(filteredIntents, row)
		}
	}
	r.Response.WriteJson(map[string]interface{}{"code": 0, "data": map[string]interface{}{"projections": filteredProjections, "intents": filteredIntents, "runtime": runtime, "runtime_apply": local}})
}

func configurationCodeIncluded(codes []string, code string) bool {
	for _, value := range codes {
		if value == code {
			return true
		}
	}
	return false
}

// Lists need revision metadata, not a complete recovery snapshot and its N+1
// canonical payload queries. Load only the requested kind and page in batches.
func localConfigurationSummary(engine *gatewayEngineImpl, kind string, principal *core.AuthContext, tenant, project uint, code string, codes []string, bounded bool) ([]store.ConfigProjection, error) {
	var rows []struct {
		Code                string
		TenantID, ProjectID uint
		Revision            int64
	}
	query := configurationScope(store.DB.Model(configEntity(kind)), principal, tenant, project)
	if engine.config.TenantID != 0 {
		query = query.Where("tenant_id = ?", engine.config.TenantID)
	}
	if engine.config.ProjectID != 0 {
		query = query.Where("project_id = ?", engine.config.ProjectID)
	}
	if code != "" {
		query = query.Where("code = ?", code)
	}
	if bounded {
		query = query.Where("code IN ?", codes)
	}
	revision := "rev"
	if kind == store.SyncEntityRule {
		revision = "version"
	}
	if err := query.Select("code, tenant_id, project_id, " + revision + " AS revision").Scan(&rows).Error; err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(rows))
	for _, row := range rows {
		keys = append(keys, row.Code)
	}
	var lifecycles []store.ResourceLifecycle
	if len(keys) > 0 {
		if err := store.DB.Where("entity_type = ? AND code IN ?", kind, keys).Find(&lifecycles).Error; err != nil {
			return nil, err
		}
	}
	versions := make(map[string]int64, len(lifecycles))
	for _, row := range lifecycles {
		versions[row.Code] = row.Version
	}
	result := make([]store.ConfigProjection, 0, len(rows))
	for _, row := range rows {
		version := versions[row.Code]
		if version == 0 {
			version = 1
		}
		result = append(result, store.ConfigProjection{GatewayCode: engine.gatewayCode, EntityType: kind, EntityCode: row.Code, TenantID: row.TenantID, ProjectID: row.ProjectID, Revision: row.Revision, LifecycleVersion: version})
	}
	return result, nil
}
