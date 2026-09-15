package core

import (
	"encoding/json"
	"noyo/core/store"
	"strings"
)

// Match only complete JSON string values, never substring or SQL LIKE matches.
func workOrderJSONReferencesAsset(raw, path string) bool {
	var value any
	if json.Unmarshal([]byte(raw), &value) != nil {
		return false
	}
	var contains func(any) bool
	contains = func(value any) bool {
		switch v := value.(type) {
		case string:
			return v == path
		case []any:
			for _, child := range v {
				if contains(child) {
					return true
				}
			}
		case map[string]any:
			for _, child := range v {
				if contains(child) {
					return true
				}
			}
		}
		return false
	}
	return contains(value)
}

// Work-order reads use work_order:list plus the tenant/project scope. The
// relationship filters on the list page are optional views, not object ACLs.
// Require an actual live order reference in that same readable scope.
func canReadWorkOrderAsset(auth *AuthContext, path string) bool {
	if auth == nil || store.DB == nil || (!strings.HasPrefix(path, "/data/images/") && !strings.HasPrefix(path, "/data/attachments/")) {
		return false
	}
	if auth.IsSystemAdmin {
		return true
	}
	if auth.TenantID == 0 {
		return false
	}
	query := store.DB.Model(&store.WorkOrder{}).Where("tenant_id = ?", auth.TenantID)
	if auth.ProjectID > 0 {
		query = query.Where("project_id = ?", auth.ProjectID)
	}
	if !auth.IsTenantAdmin {
		query = query.Where("project_id IN ?", auth.AllowedProjectIDs)
	}
	// Stream candidate rows rather than loading every order/form into memory.
	rows, err := query.Select("id", "tenant_id", "project_id", "form_data").Rows()
	if err != nil {
		return false
	}
	defer rows.Close()
	var candidates []store.WorkOrder
	for rows.Next() {
		var order store.WorkOrder
		if store.DB.ScanRows(rows, &order) != nil {
			return false
		}
		if !auth.HasProjectPermission("work_order:list", order.ProjectID) {
			continue
		}
		if workOrderJSONReferencesAsset(order.FormData, path) {
			return true
		}
		// Finish reading before child queries, including single-connection SQLite.
		order.FormData = ""
		candidates = append(candidates, order)
	}
	if rows.Err() != nil {
		return false
	}
	if rows.Close() != nil {
		return false
	}
	for _, order := range candidates {
		var events []store.WorkOrderEvent
		if store.DB.Select("payload").Where("tenant_id = ? AND project_id = ? AND work_order_id = ?", order.TenantID, order.ProjectID, order.ID).Find(&events).Error != nil {
			return false
		}
		// Comments, transitions and task completion attachments are persisted in
		// event payloads; task rows contain only the human comment and task state.
		for _, event := range events {
			if workOrderJSONReferencesAsset(event.Payload, path) {
				return true
			}
		}
	}
	return false
}
