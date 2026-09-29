package cascade

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"noyo/core/store"
)

// Pending plugin edits are durable. A reconnect retries them without asking a
// platform user to visit the gateway page; site edits to the same field win.
func (e *platformEngineImpl) retryPendingPluginConfigLoop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		e.reconcilePendingPluginConfig("")
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (e *platformEngineImpl) reconcilePendingPluginConfig(gatewayCode string) {
	// The ordinary change log owns delivery and retry. This loop only updates
	// the plugin page from the durable mirror after the gateway result arrives.
	e.pluginReconcileMu.Lock()
	defer e.pluginReconcileMu.Unlock()
	for _, pending := range gatewayPluginStateCache.Pending(gatewayCode) {
		if e.recorder != nil && e.recorder.separateIntents {
			intent, err := latestPendingProtectedIntentTx(store.DB, store.ProtectedConfigCode(pending.GatewaySN, "plugin", pending.PluginName), pending.GatewaySN)
			if err != nil || intent != nil {
				continue
			}
		}
		value, err := store.GetProtectedConfigTx(store.DB, store.ProtectedConfigCode(pending.GatewaySN, "plugin", pending.PluginName))
		if err != nil || value.SyncState == store.SyncStatePending {
			continue
		}
		if value.Revision <= pending.BaseVersion {
			continue
		}
		if value.SyncState == store.SyncStateConflict {
			gatewayPluginStateCache.MarkConflictIfGatewayChanged(pending.GatewaySN, pending.PluginName, value.Revision)
			continue
		}
		summary := summaryFromCacheItem(pending)
		applyConfigToSchema(&summary, value.Config)
		if enabled, ok := value.Config["enabled"].(bool); ok {
			if enabled {
				summary.Status = "running"
			} else {
				summary.Status = "stopped"
			}
		}
		summary.ConfigVersion = value.Revision
		gatewayPluginStateCache.MarkSynced(pending.GatewaySN, summary, time.Now())
	}
}

// A later gateway version is safe to merge when every desired field still has
// the value observed before the platform edit, or already has the desired one.
// Missing or redacted fields are conservative conflicts.
func pluginPendingFieldsCanMerge(pending *remotePluginCacheItem, live *remotePluginSummary) (canApply, alreadyApplied bool) {
	if pending == nil || live == nil || len(pending.DesiredConfig) == 0 {
		return false, false
	}
	base := pending.SummarySnapshot
	alreadyApplied = true
	for key, desired := range pending.DesiredConfig {
		current, known := pluginSummaryField(live, key)
		if !known {
			return false, false
		}
		if samePluginValue(current, desired) {
			continue
		}
		alreadyApplied = false
		if base == nil || sensitivePluginField(key) {
			return false, false
		}
		original, known := pluginSummaryField(base, key)
		if !known || !samePluginValue(current, original) {
			return false, false
		}
	}
	return true, alreadyApplied
}

func pluginSummaryField(summary *remotePluginSummary, key string) (interface{}, bool) {
	if summary == nil {
		return nil, false
	}
	if key == "enabled" {
		return summary.Status == "running", true
	}
	if summary.Schema == nil {
		return nil, false
	}
	for _, field := range summary.Schema.Fields {
		if field.Name == key {
			return field.Value, true
		}
	}
	return nil, false
}

func sensitivePluginField(key string) bool {
	key = strings.ToLower(key)
	return strings.Contains(key, "password") || strings.Contains(key, "secret") || strings.Contains(key, "token") || strings.Contains(key, "credential")
}

func samePluginValue(a, b interface{}) bool {
	left, leftErr := json.Marshal(a)
	right, rightErr := json.Marshal(b)
	return leftErr == nil && rightErr == nil && string(left) == string(right)
}
