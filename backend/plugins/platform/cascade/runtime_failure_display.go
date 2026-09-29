package cascade

import (
	"noyo/core/store"

	"gorm.io/gorm"
)

type gatewayRuntimeFailure struct {
	store.GatewayRuntimeState
	Deleted                   bool   `json:"deleted"`
	HasProjection             bool   `json:"has_projection"`
	ConfirmedRevision         *int64 `json:"confirmed_revision"`
	ConfirmedLifecycleVersion *int64 `json:"confirmed_lifecycle_version"`
}

// currentGatewayRuntimeFailures keeps failed deletions and historical results,
// while excluding failures superseded by a newer confirmed resource version.
// Gateway cards and their failure details must use this same scope.
func currentGatewayRuntimeFailures(db *gorm.DB) *gorm.DB {
	query := db.Model(&store.GatewayRuntimeState{}).Where("gateway_runtime_states.status = ?", store.RuntimeApplyFailed)
	if !db.Migrator().HasTable(&store.ConfigProjection{}) {
		return query
	}
	return query.Where(`NOT EXISTS (SELECT 1 FROM config_projections confirmed
		WHERE confirmed.gateway_code = gateway_runtime_states.gateway_code
		AND confirmed.entity_type = gateway_runtime_states.entity_type
		AND confirmed.entity_code = gateway_runtime_states.entity_code
		AND (confirmed.lifecycle_version > gateway_runtime_states.lifecycle_version
		OR (confirmed.lifecycle_version = gateway_runtime_states.lifecycle_version
		AND confirmed.revision > gateway_runtime_states.revision)))`)
}

func listGatewayRuntimeFailures(db *gorm.DB, gatewayCode string, page, pageSize int) ([]gatewayRuntimeFailure, int64, error) {
	query := currentGatewayRuntimeFailures(db).Where("gateway_runtime_states.gateway_code = ?", gatewayCode)
	var total int64
	if err := query.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	rows := []gatewayRuntimeFailure{}
	query = query.Select("gateway_runtime_states.*")
	if db.Migrator().HasTable(&store.ConfigProjection{}) {
		query = query.Joins(`LEFT JOIN config_projections confirmed
			ON confirmed.gateway_code = gateway_runtime_states.gateway_code
			AND confirmed.entity_type = gateway_runtime_states.entity_type
			AND confirmed.entity_code = gateway_runtime_states.entity_code`).
			Select(`gateway_runtime_states.*, confirmed.deleted AS deleted,
				CASE WHEN confirmed.gateway_code IS NULL THEN 0 ELSE 1 END AS has_projection,
				confirmed.revision AS confirmed_revision,
				confirmed.lifecycle_version AS confirmed_lifecycle_version`)
	}
	err := query.Order("gateway_runtime_states.updated_at desc, gateway_runtime_states.entity_type asc, gateway_runtime_states.entity_code asc").
		Offset((page - 1) * pageSize).Limit(pageSize).Scan(&rows).Error
	return rows, total, err
}
