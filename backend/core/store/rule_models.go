package store

import "gorm.io/gorm"

type Rule struct {
	ID          uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	TenantID    uint   `gorm:"index:idx_rule_tenant_project" json:"tenant_id"`
	ProjectID   uint   `gorm:"index:idx_rule_tenant_project" json:"project_id"`
	Code        string `gorm:"uniqueIndex;size:64;not null" json:"code"`
	Name        string `gorm:"size:128;not null" json:"name"`
	Description string `gorm:"size:512" json:"description"`
	GroupID     *uint  `gorm:"index" json:"group_id"`
	Enabled     bool   `gorm:"default:false" json:"enabled"`
	Priority    int    `gorm:"default:50" json:"priority"`
	Status      string `gorm:"size:20;default:'draft'" json:"status"`

	Triggers      string `gorm:"type:text;not null" json:"triggers"`
	Conditions    string `gorm:"type:text" json:"conditions"`
	Actions       string `gorm:"type:text;not null" json:"actions"`
	EffectiveTime string `gorm:"type:text" json:"effective_time"`

	ThrottleSec int    `gorm:"default:60" json:"throttle_sec"`
	MaxPerHour  int    `gorm:"default:60" json:"max_per_hour"`
	SilentStart string `gorm:"size:5" json:"silent_start"`
	SilentEnd   string `gorm:"size:5" json:"silent_end"`
	RetryCount  int    `gorm:"default:0" json:"retry_count"`

	Scope     string `gorm:"size:20;default:'platform'" json:"scope"`
	GatewaySN string `gorm:"size:64;index" json:"gateway_sn"`
	Version   int64  `gorm:"default:1" json:"version"`
	SyncState string `gorm:"size:20" json:"sync_state"`
	SyncError string `gorm:"size:512" json:"sync_error"`

	LastTriggeredAt *int64 `json:"last_triggered_at"`
	TriggerCount    int64  `gorm:"default:0" json:"trigger_count"`
	ErrorMessage    string `gorm:"size:512" json:"error_message"`
	EnabledBy       uint   `gorm:"index" json:"enabled_by"`

	CreatedAt int64 `gorm:"autoCreateTime:milli" json:"created_at"`
	UpdatedAt int64 `gorm:"autoUpdateTime:milli" json:"updated_at"`
}

type RuleGroup struct {
	ID          uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	TenantID    uint   `gorm:"index:idx_rule_group_scope" json:"tenant_id"`
	ProjectID   uint   `gorm:"index:idx_rule_group_scope" json:"project_id"`
	Name        string `gorm:"size:128;not null" json:"name"`
	Description string `gorm:"size:512" json:"description"`
	SortOrder   int    `gorm:"default:0" json:"sort_order"`
	CreatedAt   int64  `gorm:"autoCreateTime:milli" json:"created_at"`
	UpdatedAt   int64  `gorm:"autoUpdateTime:milli" json:"updated_at"`
}

type RuleExecLog struct {
	ID          uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	TenantID    uint   `gorm:"index:idx_rel_tenant" json:"tenant_id"`
	ProjectID   uint   `gorm:"index:idx_rel_project" json:"project_id"`
	RuleID      uint   `gorm:"index:idx_rel_rule" json:"rule_id"`
	RuleCode    string `gorm:"size:64;index" json:"rule_code"`
	RuleName    string `gorm:"size:128" json:"rule_name"`
	RuleVersion int64  `gorm:"index" json:"rule_version"`
	Scope       string `gorm:"size:20;index" json:"scope"`
	GatewaySN   string `gorm:"size:64;index" json:"gateway_sn"`

	TriggerID     string `gorm:"size:64;index" json:"trigger_id"`
	TriggerType   string `gorm:"size:32" json:"trigger_type"`
	TriggerDetail string `gorm:"type:text" json:"trigger_detail"`
	TraceID       string `gorm:"size:64;index" json:"trace_id"`
	ChainDepth    int    `json:"chain_depth"`

	ConditionResult bool   `json:"condition_result"`
	ConditionDetail string `gorm:"type:text" json:"condition_detail"`
	ActionResults   string `gorm:"type:text" json:"action_results"`
	Success         bool   `json:"success"`
	ErrorMessage    string `gorm:"size:512" json:"error_message"`

	DurationMs int64 `json:"duration_ms"`
	ExecutedAt int64 `gorm:"index" json:"executed_at"`
	ExecutedAs uint  `gorm:"index" json:"executed_as"`
}

func SaveRule(rule *Rule) error {
	if rule.Code != "" {
		var existing Rule
		if err := DB.Where("code = ?", rule.Code).First(&existing).Error; err == nil {
			rule.ID = existing.ID
			rule.CreatedAt = existing.CreatedAt
			if rule.Version <= existing.Version {
				rule.Version = existing.Version + 1
			}
			return DB.Save(rule).Error
		} else if err != gorm.ErrRecordNotFound {
			return err
		}
	}
	return DB.Create(rule).Error
}

func ListRules(page, pageSize int, tenantID, projectID uint) ([]Rule, int64, error) {
	var rules []Rule
	var total int64
	db := DB.Model(&Rule{})
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
		db = db.Offset((page - 1) * pageSize).Limit(pageSize)
	}
	err := db.Order("updated_at desc").Find(&rules).Error
	return rules, total, err
}

func GetRule(code string) (*Rule, error) {
	var rule Rule
	if err := DB.Where("code = ?", code).First(&rule).Error; err != nil {
		return nil, err
	}
	return &rule, nil
}

func DeleteRule(code string) error {
	return DB.Where("code = ?", code).Delete(&Rule{}).Error
}

func SaveRuleGroup(group *RuleGroup) error {
	if group.ID > 0 {
		return DB.Save(group).Error
	}
	return DB.Create(group).Error
}

func ListRuleGroups(tenantID, projectID uint) ([]RuleGroup, error) {
	var groups []RuleGroup
	db := DB.Model(&RuleGroup{})
	if tenantID > 0 {
		db = db.Where("tenant_id = ?", tenantID)
	}
	if projectID > 0 {
		db = db.Where("project_id = ?", projectID)
	}
	err := db.Order("sort_order asc, created_at desc").Find(&groups).Error
	return groups, err
}

func DeleteRuleGroup(id uint) error {
	return DB.Delete(&RuleGroup{}, id).Error
}

func CreateRuleExecLog(log *RuleExecLog) error {
	return DB.Create(log).Error
}

func ListRuleExecLogs(ruleCode string, page, pageSize int) ([]RuleExecLog, int64, error) {
	var logs []RuleExecLog
	var total int64
	db := DB.Model(&RuleExecLog{}).Where("rule_code = ?", ruleCode)
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if page > 0 && pageSize > 0 {
		db = db.Offset((page - 1) * pageSize).Limit(pageSize)
	}
	err := db.Order("executed_at desc").Find(&logs).Error
	return logs, total, err
}
