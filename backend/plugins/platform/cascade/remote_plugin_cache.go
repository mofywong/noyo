package cascade

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"noyo/core/store"

	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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
	db    *gorm.DB
}

var gatewayPluginStateCache = newRemotePluginMemoryCache()

func newRemotePluginMemoryCache() *remotePluginMemoryCache {
	return &remotePluginMemoryCache{items: make(map[string]*remotePluginCacheItem)}
}

func remotePluginCacheKey(gwSn, pluginName string) string {
	return gwSn + "\x00" + pluginName
}

// RestoreFromStore binds the cache to the platform database and recovers
// offline edits before the first gateway handshake can retry them.
func (c *remotePluginMemoryCache) RestoreFromStore(db *gorm.DB) error {
	var rows []store.GatewayPluginStateModel
	if err := db.Find(&rows).Error; err != nil {
		return err
	}
	items := make(map[string]*remotePluginCacheItem, len(rows))
	for _, row := range rows {
		item := &remotePluginCacheItem{
			GatewaySN: row.GatewaySN, PluginName: row.PluginName,
			DesiredEnabled: row.DesiredEnabled, BaseVersion: row.BaseVersion,
			GatewayVersion: row.GatewayVersion, SyncState: row.SyncState,
			EnabledAt: row.EnabledAt, LastSyncedAt: row.LastSyncedAt,
			UpdatedAt: row.UpdatedAt.UnixMilli(),
		}
		if row.DesiredConfig != "" {
			if err := json.Unmarshal([]byte(row.DesiredConfig), &item.DesiredConfig); err != nil {
				return fmt.Errorf("decode desired plugin config %s/%s: %w", row.GatewaySN, row.PluginName, err)
			}
		}
		if row.SummarySnapshot != "" {
			if err := json.Unmarshal([]byte(row.SummarySnapshot), &item.SummarySnapshot); err != nil {
				return fmt.Errorf("decode plugin snapshot %s/%s: %w", row.GatewaySN, row.PluginName, err)
			}
		}
		items[remotePluginCacheKey(row.GatewaySN, row.PluginName)] = item
	}
	c.mu.Lock()
	c.items, c.db = items, db
	c.mu.Unlock()
	return nil
}

func (c *remotePluginMemoryCache) persistLocked(item *remotePluginCacheItem) error {
	if c.db == nil {
		return nil
	}
	return c.persistTx(c.db, item)
}

func (c *remotePluginMemoryCache) persistTx(db *gorm.DB, item *remotePluginCacheItem) error {
	config, err := json.Marshal(item.DesiredConfig)
	if err != nil {
		return err
	}
	summary, err := json.Marshal(item.SummarySnapshot)
	if err != nil {
		return err
	}
	row := store.GatewayPluginStateModel{
		GatewaySN: item.GatewaySN, PluginName: item.PluginName,
		DesiredConfig: string(config), DesiredEnabled: item.DesiredEnabled,
		SummarySnapshot: string(summary), BaseVersion: item.BaseVersion,
		GatewayVersion: item.GatewayVersion, SyncState: item.SyncState,
		EnabledAt: item.EnabledAt, LastSyncedAt: item.LastSyncedAt,
	}
	return db.Set(store.SyncApplyingKey, true).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "gateway_sn"}, {Name: "plugin_name"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"desired_config", "desired_enabled", "summary_snapshot", "base_version",
			"gateway_version", "sync_state", "enabled_at", "last_synced_at", "updated_at",
		}),
	}).Create(&row).Error
}

func (c *remotePluginMemoryCache) persistObservedStateLocked(item *remotePluginCacheItem) bool {
	if err := c.persistLocked(item); err != nil {
		// Pending intent stays durable and is retried after restart. Surface a
		// storage failure instead of silently claiming the observation persisted.
		zap.L().Error("Gateway plugin sync state could not be persisted", zap.String("gateway", item.GatewaySN), zap.String("plugin", item.PluginName), zap.Error(err))
		return false
	}
	return true
}

func (c *remotePluginMemoryCache) Pending(gwSn string) []*remotePluginCacheItem {
	c.mu.RLock()
	defer c.mu.RUnlock()
	var items []*remotePluginCacheItem
	for _, item := range c.items {
		if (gwSn == "" || item.GatewaySN == gwSn) && item.SyncState == remotePluginSyncPending {
			items = append(items, cloneCacheItem(item))
		}
	}
	return items
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
	item := cloneCacheItem(c.items[key])
	if item == nil {
		item = &remotePluginCacheItem{GatewaySN: gwSn, PluginName: pluginName}
	}

	item.DesiredConfig = cloneConfigMap(config)
	item.DesiredEnabled = enabled
	item.BaseVersion = baseVersion
	item.SyncState = remotePluginSyncPending
	item.UpdatedAt = now.UnixMilli()
	if prior := c.items[key]; prior != nil && item.UpdatedAt <= prior.UpdatedAt {
		item.UpdatedAt = prior.UpdatedAt + 1
	}
	if enabled {
		item.EnabledAt = now.UnixMilli()
	}
	if item.SummarySnapshot != nil {
		item.SummarySnapshot.SyncState = item.SyncState
		item.SummarySnapshot.BaseVersion = item.BaseVersion
		item.SummarySnapshot.EnabledAt = item.EnabledAt
	}

	if err := c.persistDesiredLocked(item); err != nil {
		return nil, err
	}
	c.items[key] = item
	return cloneCacheItem(item), nil
}

func (c *remotePluginMemoryCache) persistDesiredLocked(item *remotePluginCacheItem) error {
	if c.db == nil {
		return nil
	}
	return c.db.Transaction(func(tx *gorm.DB) error {
		if err := c.persistTx(tx, item); err != nil {
			return err
		}
		// A standalone cache has no synchronization coordinator. Production
		// binds a ProtectedConfigSink before any HTTP write becomes available.
		return recordPluginCacheIntentTx(tx, item)
	})
}

func (c *remotePluginMemoryCache) SaveSnapshot(gwSn string, summary remotePluginSummary, now time.Time) *remotePluginCacheItem {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := remotePluginCacheKey(gwSn, summary.Name)
	previous := c.items[key]
	item := cloneCacheItem(previous)
	if item == nil {
		item = &remotePluginCacheItem{GatewaySN: gwSn, PluginName: summary.Name}
	}

	if summary.Status == "running" && item.EnabledAt == 0 {
		item.EnabledAt = firstNonZero(summary.EnabledAt, summary.UpdatedAt, now.UnixMilli())
	}
	item.GatewayVersion = summary.ConfigVersion
	if item.SyncState != remotePluginSyncPending && item.SyncState != remotePluginSyncConflict {
		item.BaseVersion = summary.ConfigVersion
		item.LastSyncedAt = now.UnixMilli()
	}
	if item.SyncState != remotePluginSyncPending && item.SyncState != remotePluginSyncConflict {
		item.UpdatedAt = now.UnixMilli()
	}
	if item.SyncState == "" || item.SyncState == remotePluginSyncSyncing || item.SyncState == remotePluginSyncFailed {
		item.SyncState = remotePluginSyncSynced
	}

	summary.SyncState = valueOrDefault(item.SyncState, remotePluginSyncSynced)
	summary.BaseVersion = item.BaseVersion
	summary.GatewayVersion = item.GatewayVersion
	summary.EnabledAt = item.EnabledAt
	summary.LastSyncedAt = item.LastSyncedAt
	summary.IsOfflineEditable = true
	if item.SyncState != remotePluginSyncPending && item.SyncState != remotePluginSyncConflict {
		item.SummarySnapshot = cloneRemotePluginSummary(&summary)
	}
	if !c.persistObservedStateLocked(item) {
		return cloneCacheItem(previous)
	}
	c.items[key] = item

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

	key := remotePluginCacheKey(gwSn, pluginName)
	previous := c.items[key]
	item := cloneCacheItem(previous)
	if item == nil {
		item = &remotePluginCacheItem{GatewaySN: gwSn, PluginName: pluginName}
	}

	item.GatewayVersion = gatewayVersion
	if item.SyncState == remotePluginSyncPending && gatewayVersion != item.BaseVersion {
		item.SyncState = remotePluginSyncConflict
		if item.SummarySnapshot != nil {
			item.SummarySnapshot.SyncState = item.SyncState
			item.SummarySnapshot.GatewayVersion = gatewayVersion
		}
		if !c.persistObservedStateLocked(item) {
			return cloneCacheItem(previous), false
		}
		c.items[key] = item
		return cloneCacheItem(item), true
	}

	// Matching the base version only permits a retry; the gateway has not yet
	// acknowledged the desired configuration.
	if item.SummarySnapshot != nil {
		item.SummarySnapshot.SyncState = item.SyncState
		item.SummarySnapshot.GatewayVersion = gatewayVersion
	}
	if !c.persistObservedStateLocked(item) {
		return cloneCacheItem(previous), false
	}
	c.items[key] = item
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
	c.mu.Lock()
	defer c.mu.Unlock()

	key := remotePluginCacheKey(gwSn, summary.Name)
	previous := c.items[key]
	item := cloneCacheItem(previous)
	if item == nil {
		item = &remotePluginCacheItem{GatewaySN: gwSn, PluginName: summary.Name}
	}
	item.BaseVersion = summary.ConfigVersion
	item.GatewayVersion = summary.ConfigVersion
	item.SyncState = remotePluginSyncSynced
	item.LastSyncedAt = now.UnixMilli()
	item.UpdatedAt = now.UnixMilli()
	item.DesiredConfig = nil
	if summary.Status == "running" && item.EnabledAt == 0 {
		item.EnabledAt = firstNonZero(summary.EnabledAt, summary.UpdatedAt, now.UnixMilli())
	}
	summary.SyncState = item.SyncState
	summary.BaseVersion = item.BaseVersion
	summary.GatewayVersion = item.GatewayVersion
	summary.EnabledAt = item.EnabledAt
	summary.LastSyncedAt = item.LastSyncedAt
	summary.IsOfflineEditable = true
	item.SummarySnapshot = cloneRemotePluginSummary(&summary)
	if !c.persistObservedStateLocked(item) {
		if previous != nil {
			return summaryFromCacheItem(previous)
		}
		summary.SyncState = remotePluginSyncPending
		return summary
	}
	c.items[key] = item
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
