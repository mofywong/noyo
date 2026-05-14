package cascade

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

const (
	remotePluginSyncSynced   = "synced"
	remotePluginSyncPending  = "pending"
	remotePluginSyncSyncing  = "syncing"
	remotePluginSyncConflict = "conflict"
	remotePluginSyncFailed   = "failed"
)

type remotePluginCacheItem struct {
	GatewaySN       string
	PluginName      string
	DesiredConfig   map[string]interface{}
	DesiredEnabled  bool
	SummarySnapshot *remotePluginSummary
	BaseVersion     int64
	GatewayVersion  int64
	SyncState       string
	EnabledAt       int64
	LastSyncedAt    int64
	UpdatedAt       int64
}

type remotePluginMemoryCache struct {
	mu    sync.RWMutex
	items map[string]*remotePluginCacheItem
}

var gatewayPluginStateCache = newRemotePluginMemoryCache()

func newRemotePluginMemoryCache() *remotePluginMemoryCache {
	return &remotePluginMemoryCache{items: make(map[string]*remotePluginCacheItem)}
}

func remotePluginCacheKey(gwSn, pluginName string) string {
	return gwSn + "\x00" + pluginName
}

func (c *remotePluginMemoryCache) SaveDesired(gwSn, pluginName string, config map[string]interface{}, enabled bool, baseVersion int64, now time.Time) (*remotePluginCacheItem, error) {
	if gwSn == "" {
		return nil, fmt.Errorf("gateway sn is required")
	}
	if pluginName == "" {
		return nil, fmt.Errorf("plugin is required")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	key := remotePluginCacheKey(gwSn, pluginName)
	item := c.items[key]
	if item == nil {
		item = &remotePluginCacheItem{GatewaySN: gwSn, PluginName: pluginName}
		c.items[key] = item
	}

	item.DesiredConfig = cloneConfigMap(config)
	item.DesiredEnabled = enabled
	item.BaseVersion = baseVersion
	item.SyncState = remotePluginSyncPending
	item.UpdatedAt = now.UnixMilli()
	if enabled {
		item.EnabledAt = now.UnixMilli()
	}
	if item.SummarySnapshot != nil {
		item.SummarySnapshot.SyncState = item.SyncState
		item.SummarySnapshot.BaseVersion = item.BaseVersion
		item.SummarySnapshot.EnabledAt = item.EnabledAt
		item.SummarySnapshot.UpdatedAt = item.UpdatedAt
		applyConfigToSchema(item.SummarySnapshot, item.DesiredConfig)
	}

	return cloneCacheItem(item), nil
}

func (c *remotePluginMemoryCache) SaveSnapshot(gwSn string, summary remotePluginSummary, now time.Time) *remotePluginCacheItem {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := remotePluginCacheKey(gwSn, summary.Name)
	item := c.items[key]
	if item == nil {
		item = &remotePluginCacheItem{GatewaySN: gwSn, PluginName: summary.Name}
		c.items[key] = item
	}

	if summary.Status == "running" && item.EnabledAt == 0 {
		item.EnabledAt = firstNonZero(summary.EnabledAt, summary.UpdatedAt, now.UnixMilli())
	}
	item.GatewayVersion = summary.ConfigVersion
	item.LastSyncedAt = now.UnixMilli()
	item.UpdatedAt = now.UnixMilli()
	if item.SyncState == "" || item.SyncState == remotePluginSyncSyncing || item.SyncState == remotePluginSyncFailed {
		item.SyncState = remotePluginSyncSynced
	}

	summary.SyncState = valueOrDefault(item.SyncState, remotePluginSyncSynced)
	summary.BaseVersion = item.BaseVersion
	summary.GatewayVersion = item.GatewayVersion
	summary.EnabledAt = item.EnabledAt
	summary.LastSyncedAt = item.LastSyncedAt
	summary.IsOfflineEditable = true
	item.SummarySnapshot = cloneRemotePluginSummary(&summary)

	return cloneCacheItem(item)
}

func (c *remotePluginMemoryCache) Get(gwSn, pluginName string) (*remotePluginCacheItem, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, ok := c.items[remotePluginCacheKey(gwSn, pluginName)]
	if !ok {
		return nil, false
	}
	return cloneCacheItem(item), true
}

func (c *remotePluginMemoryCache) List(gwSn string) []remotePluginSummary {
	c.mu.RLock()
	defer c.mu.RUnlock()

	summaries := make([]remotePluginSummary, 0)
	for _, item := range c.items {
		if item.GatewaySN != gwSn || item.SummarySnapshot == nil {
			continue
		}
		summary := cloneRemotePluginSummary(item.SummarySnapshot)
		summary.SyncState = valueOrDefault(item.SyncState, remotePluginSyncSynced)
		summary.BaseVersion = item.BaseVersion
		summary.GatewayVersion = item.GatewayVersion
		summary.EnabledAt = item.EnabledAt
		summary.LastSyncedAt = item.LastSyncedAt
		summary.IsOfflineEditable = true
		applyConfigToSchema(summary, item.DesiredConfig)
		summaries = append(summaries, *summary)
	}
	return summaries
}

func (c *remotePluginMemoryCache) MarkConflictIfGatewayChanged(gwSn, pluginName string, gatewayVersion int64) (*remotePluginCacheItem, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	item := c.items[remotePluginCacheKey(gwSn, pluginName)]
	if item == nil {
		item = &remotePluginCacheItem{GatewaySN: gwSn, PluginName: pluginName}
		c.items[remotePluginCacheKey(gwSn, pluginName)] = item
	}

	item.GatewayVersion = gatewayVersion
	if item.BaseVersion > 0 && gatewayVersion > item.BaseVersion {
		item.SyncState = remotePluginSyncConflict
		if item.SummarySnapshot != nil {
			item.SummarySnapshot.SyncState = item.SyncState
			item.SummarySnapshot.GatewayVersion = gatewayVersion
		}
		return cloneCacheItem(item), true
	}

	item.SyncState = remotePluginSyncSynced
	if item.SummarySnapshot != nil {
		item.SummarySnapshot.SyncState = item.SyncState
		item.SummarySnapshot.GatewayVersion = gatewayVersion
	}
	return cloneCacheItem(item), false
}

func (c *remotePluginMemoryCache) MergeSummary(gwSn string, summary remotePluginSummary) remotePluginSummary {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item := c.items[remotePluginCacheKey(gwSn, summary.Name)]
	if item == nil {
		summary.SyncState = remotePluginSyncSynced
		summary.GatewayVersion = summary.ConfigVersion
		summary.EnabledAt = firstNonZero(summary.EnabledAt, summary.UpdatedAt)
		summary.IsOfflineEditable = true
		return summary
	}

	summary.SyncState = valueOrDefault(item.SyncState, remotePluginSyncSynced)
	summary.BaseVersion = item.BaseVersion
	summary.GatewayVersion = firstNonZero(item.GatewayVersion, summary.ConfigVersion)
	summary.EnabledAt = firstNonZero(item.EnabledAt, summary.EnabledAt, summary.UpdatedAt)
	summary.LastSyncedAt = item.LastSyncedAt
	summary.IsOfflineEditable = true
	if item.SyncState == remotePluginSyncPending || item.SyncState == remotePluginSyncConflict {
		applyConfigToSchema(&summary, item.DesiredConfig)
	}
	return summary
}

func (c *remotePluginMemoryCache) MarkSynced(gwSn string, summary remotePluginSummary, now time.Time) remotePluginSummary {
	c.SaveSnapshot(gwSn, summary, now)
	c.mu.Lock()
	defer c.mu.Unlock()

	item := c.items[remotePluginCacheKey(gwSn, summary.Name)]
	if item != nil {
		item.BaseVersion = summary.ConfigVersion
		item.GatewayVersion = summary.ConfigVersion
		item.SyncState = remotePluginSyncSynced
		item.LastSyncedAt = now.UnixMilli()
		item.DesiredConfig = nil
		if item.SummarySnapshot != nil {
			item.SummarySnapshot.SyncState = item.SyncState
			item.SummarySnapshot.BaseVersion = item.BaseVersion
			item.SummarySnapshot.GatewayVersion = item.GatewayVersion
			item.SummarySnapshot.LastSyncedAt = item.LastSyncedAt
		}
		summary = *item.SummarySnapshot
	}
	return summary
}

func cloneConfigMap(src map[string]interface{}) map[string]interface{} {
	if src == nil {
		return nil
	}
	data, _ := json.Marshal(src)
	var dst map[string]interface{}
	_ = json.Unmarshal(data, &dst)
	return dst
}

func cloneCacheItem(src *remotePluginCacheItem) *remotePluginCacheItem {
	if src == nil {
		return nil
	}
	dst := *src
	dst.DesiredConfig = cloneConfigMap(src.DesiredConfig)
	dst.SummarySnapshot = cloneRemotePluginSummary(src.SummarySnapshot)
	return &dst
}

func cloneRemotePluginSummary(src *remotePluginSummary) *remotePluginSummary {
	if src == nil {
		return nil
	}
	data, _ := json.Marshal(src)
	var dst remotePluginSummary
	_ = json.Unmarshal(data, &dst)
	return &dst
}

func applyConfigToSchema(summary *remotePluginSummary, config map[string]interface{}) {
	if summary == nil || summary.Schema == nil || config == nil {
		return
	}
	for i := range summary.Schema.Fields {
		if value, ok := config[summary.Schema.Fields[i].Name]; ok {
			summary.Schema.Fields[i].Value = value
		}
	}
}

func firstNonZero(values ...int64) int64 {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}

func valueOrDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
