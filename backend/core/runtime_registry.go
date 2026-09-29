package core

import (
	"encoding/json"
	"fmt"
	"noyo/core/store"

	"gorm.io/gorm"
)

func (dr *DeviceRegistry) runtimeProfile(code string) (*store.ProtocolProfile, error) {
	dr.mu.RLock()
	profiles := dr.profiles
	p := profiles[code]
	dr.mu.RUnlock()
	if profiles == nil {
		return store.GetProtocolProfile(code)
	}
	if p == nil {
		return nil, gorm.ErrRecordNotFound
	}
	return p, nil
}

func (dr *DeviceRegistry) restoreActiveRuntime() error {
	profiles := make(map[string]*store.ProtocolProfile)
	if store.DB.Migrator().HasTable(&store.ProtocolProfile{}) {
		var rows []store.ProtocolProfile
		if err := store.DB.Find(&rows).Error; err != nil {
			return err
		}
		for i := range rows {
			p := rows[i]
			profiles[p.Code] = &p
		}
	}
	dr.profiles = profiles
	rows, err := store.LastActiveRuntimeConfigs()
	if err != nil {
		return err
	}
	for _, row := range rows {
		if err := dr.stageRuntimeConfig(row.EntityType, row.EntityCode, row.ActivePayload); err != nil {
			return fmt.Errorf("restore active %s %s: %w", row.EntityType, row.EntityCode, err)
		}
	}
	return nil
}

func (dr *DeviceRegistry) cloneRuntime() *DeviceRegistry {
	dr.mu.RLock()
	defer dr.mu.RUnlock()
	next := NewDeviceRegistry(dr.Logger)
	next.profiles = make(map[string]*store.ProtocolProfile, len(dr.profiles))
	for code, d := range dr.devices {
		val := *d
		next.devices[code] = &val
	}
	for code, p := range dr.products {
		val := *p
		next.products[code] = &val
	}
	for code, p := range dr.profiles {
		val := *p
		next.profiles[code] = &val
	}
	for code, children := range dr.childrenIndex {
		next.childrenIndex[code] = append([]string(nil), children...)
	}
	return next
}

// Ordinary local restarts consume freshly saved profiles, while a synchronized
// failed/pending profile continues using its retained active snapshot.
func (dr *DeviceRegistry) refreshConfiguredProfiles(deviceCode string) error {
	if store.DB == nil || !store.DB.Migrator().HasTable(&store.ProtocolProfile{}) {
		return nil
	}
	codes := make(map[string]bool)
	if device := dr.devices[deviceCode]; device != nil {
		codes[device.ProtocolProfileCode] = true
		if parent := dr.devices[device.ParentCode]; parent != nil {
			codes[parent.ProtocolProfileCode] = true
		}
		for _, childCode := range dr.childrenIndex[deviceCode] {
			if child := dr.devices[childCode]; child != nil {
				codes[child.ProtocolProfileCode] = true
			}
		}
	}
	hasRuntime := store.DB.Migrator().HasTable(&store.RuntimeApply{})
	for code := range codes {
		if code == "" {
			continue
		}
		if hasRuntime {
			row, err := store.GetRuntimeApply(store.SyncEntityDriver, code)
			if err != nil && err != gorm.ErrRecordNotFound {
				return err
			}
			if row != nil && row.Status != store.RuntimeApplyActive {
				continue
			}
		}
		profile, err := store.GetProtocolProfile(code)
		if err != nil {
			return err
		}
		dr.profiles[code] = profile
	}
	return nil
}

// stageRuntimeConfig operates only on an isolated candidate registry; it never
// writes business rows or rolls back the desired configuration revision.
func (dr *DeviceRegistry) stageRuntimeConfig(entityType, code, payload string) error {
	switch entityType {
	case store.SyncEntityDevice:
		if payload == "" {
			dr.removeRuntimeDevice(code)
			return nil
		}
		var d store.Device
		if err := json.Unmarshal([]byte(payload), &d); err != nil {
			return err
		}
		d.Code = code
		if err := validateRuntimeJSON(d.Config); err != nil {
			return err
		}
		// Use the internal update to avoid platform projection filtering.
		dr.updateRuntimeDevice(&d)
	case store.SyncEntityProduct:
		if payload == "" {
			dr.removeRuntimeProduct(code)
			return nil
		}
		var p store.Product
		if err := json.Unmarshal([]byte(payload), &p); err != nil {
			return err
		}
		p.Code = code
		if err := validateRuntimeJSON(p.Config); err != nil {
			return err
		}
		dr.products[code] = &p
	case store.SyncEntityDriver:
		if payload == "" {
			delete(dr.profiles, code)
			return nil
		}
		var p store.ProtocolProfile
		if err := json.Unmarshal([]byte(payload), &p); err != nil {
			return err
		}
		p.Code = code
		if err := validateRuntimeJSON(p.Config); err != nil {
			return err
		}
		dr.profiles[code] = &p
	}
	return nil
}

func (dr *DeviceRegistry) installRuntime(next *DeviceRegistry) {
	dr.mu.Lock()
	defer dr.mu.Unlock()
	dr.devices, dr.products, dr.profiles, dr.childrenIndex = next.devices, next.products, next.profiles, next.childrenIndex
}
