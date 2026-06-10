package store

import (
	"encoding/json"
	"errors"

	"gorm.io/gorm"
)

const setupStateKey = "setup_state"

type SetupState struct {
	Initialized bool   `json:"initialized"`
	Mode        string `json:"mode"`
	TenantID    uint   `json:"tenant_id"`
	ProjectID   uint   `json:"project_id"`
	CompletedAt int64  `json:"completed_at"`
}

func LoadSetupState() (*SetupState, error) {
	var sysConfig SystemConfig
	err := DB.Where("key = ?", setupStateKey).First(&sysConfig).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &SetupState{}, nil
		}
		return nil, err
	}

	var state SetupState
	if err := json.Unmarshal([]byte(sysConfig.Value), &state); err != nil {
		return nil, err
	}
	return &state, nil
}

func SaveSetupState(state *SetupState) error {
	if state == nil {
		state = &SetupState{}
	}
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}

	var sysConfig SystemConfig
	err = DB.Where("key = ?", setupStateKey).First(&sysConfig).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return DB.Create(&SystemConfig{
				Key:   setupStateKey,
				Value: string(data),
			}).Error
		}
		return err
	}

	return DB.Model(&sysConfig).Update("value", string(data)).Error
}
