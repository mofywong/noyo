<template>
  <div class="gateway-management-page">
    <div class="gateway-page-header">
      <div>
        <div class="gateway-page-kicker">{{ gt('remote_gateway_config') }}</div>
        <h5 class="gateway-page-title">{{ gt('gateway_management') }}</h5>
        <div class="gateway-page-subtitle">{{ gt('gateway_management_hint') }}</div>
      </div>
      <button class="btn btn-outline-primary btn-sm gateway-refresh-btn" @click="fetchGateways" :disabled="loading">
        <i class="bi bi-arrow-clockwise me-1"></i>{{ $t('refresh') }}
      </button>
    </div>

    <div v-if="loading" class="d-flex justify-content-center py-5">
      <div class="spinner-border text-primary" role="status"></div>
    </div>

    <div v-else-if="gateways.length === 0" class="alert alert-info">
      {{ gt('no_gateways') }}
    </div>

    <div v-else>
      <div class="gateway-summary-grid">
        <div class="gateway-summary-card">
          <div class="summary-icon summary-icon-total">
            <i class="bi bi-hdd-network"></i>
          </div>
          <div>
            <div class="summary-value">{{ gateways.length }}</div>
            <div class="summary-label">{{ gt('gateway') }}</div>
          </div>
        </div>
        <div class="gateway-summary-card">
          <div class="summary-icon summary-icon-online">
            <i class="bi bi-activity"></i>
          </div>
          <div>
            <div class="summary-value">{{ onlineCount }}</div>
            <div class="summary-label">{{ $t('status_online') }}</div>
          </div>
        </div>
        <div class="gateway-summary-card">
          <div class="summary-icon summary-icon-enabled">
            <i class="bi bi-check2-circle"></i>
          </div>
          <div>
            <div class="summary-value">{{ enabledCount }}</div>
            <div class="summary-label">{{ gt('enabled') }}</div>
          </div>
        </div>
      </div>

      <div class="gateway-card-grid">
        <article v-for="gw in sortedGateways" :key="gw.sn" class="gateway-card" :class="{ 'is-online': gw.online, 'is-unavailable': !gw.online }">
          <div class="gateway-card-topline">
            <span class="gateway-status-pill" :class="gw.online ? 'is-online' : 'is-offline'">
              <span class="gateway-status-dot"></span>
              {{ gw.online ? $t('status_online') : $t('dev_offline') }}
            </span>
            <span class="gateway-enabled-pill" :class="gw.enabled ? 'is-enabled' : 'is-disabled'">
              {{ gw.enabled ? gt('enabled') : gt('disabled') }}
            </span>
          </div>

          <div class="gateway-card-main">
            <div class="gateway-avatar">
              <i class="bi bi-router"></i>
            </div>
            <div class="gateway-title-group">
              <h6 class="gateway-name">{{ gw.name || gw.sn }}</h6>
              <div class="gateway-product">{{ gw.productCode || '-' }}</div>
            </div>
          </div>

          <div class="gateway-meta-panel">
            <div class="gateway-meta-row">
              <span class="gateway-meta-label">SN</span>
              <code class="gateway-sn">{{ gw.sn }}</code>
            </div>
            <div class="gateway-meta-row">
              <span class="gateway-meta-label">{{ gt('updated_at') }}</span>
              <span class="gateway-meta-value">{{ formatTime(gw.updatedAt) }}</span>
            </div>
          </div>

          <button class="btn btn-primary gateway-enter-btn" @click="openGateway(gw)" :disabled="!gw.online">
            <span>{{ gt('enter') }}</span>
            <i class="bi bi-arrow-right-short"></i>
          </button>
        </article>
      </div>
    </div>

    <Transition name="gateway-workspace">
      <section v-if="selectedGateway" class="gateway-workspace-overlay">
        <div class="gateway-workspace-shell gateway-blue-breath">
          <header class="gateway-workspace-header">
            <div class="gateway-workspace-title">
              <button class="btn btn-light btn-sm gateway-workspace-close" @click="closeGateway" :title="gt('close')">
                <i class="bi bi-x-lg"></i>
              </button>
              <div class="gateway-workspace-icon">
                <i class="bi bi-hdd-network"></i>
              </div>
              <div class="min-w-0">
                <div class="gateway-workspace-kicker">{{ gt('gateway_management') }}</div>
                <h5>{{ selectedGateway.name || selectedGateway.sn }}</h5>
                <div class="gateway-workspace-sn">SN {{ selectedGateway.sn }}</div>
              </div>
            </div>
            <div class="gateway-workspace-meta">
              <span class="gateway-status-pill" :class="selectedGateway.online ? 'is-online' : 'is-offline'">
                <span class="gateway-status-dot"></span>
                {{ selectedGateway.online ? $t('status_online') : $t('dev_offline') }}
              </span>
              <span class="gateway-enabled-pill" :class="selectedGateway.enabled ? 'is-enabled' : 'is-disabled'">
                {{ selectedGateway.enabled ? gt('enabled') : gt('disabled') }}
              </span>
              <span class="gateway-sync-pill">{{ gt('gateway_sync_synced') }}</span>
            </div>
          </header>

          <main class="gateway-workspace-layout">
            <aside class="gateway-workspace-menu">
              <button
                class="nav-link gateway-menu-item gateway-menu-marketplace"
                :class="{ active: selectedWorkspaceView === 'marketplace' }"
                @click="openMarketplace"
              >
                <i class="bi bi-shop"></i>
                <span>{{ gt('gateway_plugin_marketplace_title') }}</span>
              </button>

              <div class="gateway-menu-section gateway-menu-system-section">
                <div class="nav-category gateway-menu-label">{{ $t('sidebar_system') }}</div>
                <button
                  class="nav-link gateway-menu-item"
                  :class="{ active: selectedWorkspaceView === 'system' }"
                  @click="openSystem"
                >
                  <i class="bi bi-gear"></i>
                  <span>{{ $t('sidebar_settings') }}</span>
                </button>
                <button
                  class="nav-link gateway-menu-item"
                  :class="{ active: selectedWorkspaceView === 'license' }"
                  @click="openLicense"
                >
                  <i class="bi bi-shield-check"></i>
                  <span>{{ $t('license_info', '授权信息') }}</span>
                </button>
                <button
                  class="nav-link gateway-menu-item"
                  :class="{ active: selectedWorkspaceView === 'logs' }"
                  @click="openLogs"
                >
                  <i class="bi bi-journal-text"></i>
                  <span>{{ $t('sidebar_logs') }}</span>
                </button>
              </div>

              <div class="gateway-menu-section">
                <div class="nav-category gateway-menu-label">{{ gt('enabled_plugins') }}</div>
                <template v-for="group in groupedEnabledGatewayPlugins" :key="group.category">
                  <div class="nav-category gateway-menu-group-label">{{ group.title }}</div>
                  <button
                    v-for="plugin in group.items"
                    :key="plugin.name"
                    class="nav-link gateway-menu-item"
                    :class="{ active: selectedWorkspaceView === 'plugin' && selectedPluginName === plugin.name }"
                    @click="openPlugin(plugin.name)"
                  >
                    <img v-if="plugin.icon" :src="plugin.icon" alt="" class="gateway-menu-plugin-icon">
                    <i v-else class="bi bi-plugin"></i>
                    <span>{{ getPluginTitle(plugin) }}</span>
                  </button>
                </template>
                <div v-if="marketplaceGatewayPlugins.length > 0 && enabledGatewayPlugins.length === 0" class="gateway-menu-empty">
                  {{ $t('no_active_plugins') }}
                </div>
                <div v-else-if="marketplaceGatewayPlugins.length === 0" class="gateway-menu-empty">
                  {{ $t('loading') }}
                </div>
              </div>
            </aside>

            <section class="gateway-workspace-content">
              <GatewayPluginConfig
                v-if="selectedWorkspaceView === 'plugin'"
                embedded
                :gateway="selectedGateway"
                :gw-sn="selectedGateway.sn"
                :plugin-name="selectedPluginName"
                @back="openMarketplace"
                @refresh-gateway="refreshGatewayWorkspace"
              />
              <Settings
                v-else-if="selectedWorkspaceView === 'system'"
                :remote-api-base="gatewaySystemApiBase"
              />
              <License
                v-else-if="selectedWorkspaceView === 'license'"
                :remote-api-base="gatewayLicenseApiBase"
              />
              <Logs
                v-else-if="selectedWorkspaceView === 'logs'"
                :remote-log-base="gatewayLogApiBase"
              />
              <GatewayPlugins
                v-else
                embedded
                :gateway="selectedGateway"
                :gw-sn="selectedGateway.sn"
                @close="closeGateway"
                @configure="openPlugin"
                @plugins-loaded="gatewayPlugins = $event"
                @refresh-gateway="refreshGatewayWorkspace"
              />
            </section>
          </main>
        </div>
      </section>
    </Transition>
  </div>
</template>

<script setup>
import { computed, onBeforeUnmount, onMounted, ref } from 'vue';
import { useI18n } from 'vue-i18n';
import axios from 'axios';
import GatewayPluginConfig from './GatewayPluginConfig.vue';
import GatewayPlugins from './GatewayPlugins.vue';
import License from './License.vue';
import Logs from './Logs.vue';
import Settings from './Settings.vue';
import { useToast } from '../composables/useToast';
import { gatewayDateTime, gatewayText } from '../utils/gatewayLocale';

const { t, locale } = useI18n();
const { showToast } = useToast();
const loading = ref(false);
const gateways = ref([]);
const selectedGateway = ref(null);
const selectedPluginName = ref('');
const selectedWorkspaceView = ref('marketplace');
const gatewayPlugins = ref([]);
const gt = (key, params) => gatewayText(locale.value, key, params);
const onlineCount = computed(() => gateways.value.filter((gw) => gw.online).length);
const enabledCount = computed(() => gateways.value.filter((gw) => gw.enabled).length);
const marketplaceGatewayPlugins = computed(() => gatewayPlugins.value.filter((plugin) => plugin.name !== 'license_auth'));
const enabledGatewayPlugins = computed(() => marketplaceGatewayPlugins.value.filter((plugin) => plugin.status === 'running'));
const gatewayRemoteBase = computed(() => selectedGateway.value ? `/api/extension/cascade/gateways/${selectedGateway.value.sn}` : '');
const gatewaySystemApiBase = computed(() => `${gatewayRemoteBase.value}/system`);
const gatewayLicenseApiBase = computed(() => `${gatewayRemoteBase.value}/license`);
const gatewayLogApiBase = computed(() => `${gatewayRemoteBase.value}/system/log`);
let gatewayEventSource = null;
let gatewaySSEFetchDebounceTimer = null;
let gatewaySSEHeartbeatTimer = null;
let gatewaySSEReconnectTimer = null;

const gatewayOnlineTime = (gateway) => Number(gateway.onlineAt || gateway.lastOnlineAt || gateway.updatedAt || 0);

const gatewaySortRank = (gateway) => {
  if (gateway.enabled === false) return 2;
  if (!gateway.online) return 1;
  return 0;
};

const sortedGateways = computed(() => [...gateways.value].sort((a, b) => {
  const rankDelta = gatewaySortRank(a) - gatewaySortRank(b);
  if (rankDelta !== 0) return rankDelta;

  const timeDelta = gatewayOnlineTime(b) - gatewayOnlineTime(a);
  if (timeDelta !== 0) return timeDelta;

  return String(a.name || a.sn).localeCompare(String(b.name || b.sn));
}));

const groupedEnabledGatewayPlugins = computed(() => {
  const platforms = enabledGatewayPlugins.value.filter((plugin) => plugin.category === 'platform');
  const protocols = enabledGatewayPlugins.value.filter((plugin) => plugin.category === 'protocol');
  const others = enabledGatewayPlugins.value.filter((plugin) => plugin.category !== 'platform' && plugin.category !== 'protocol');
  const groups = [];
  if (platforms.length > 0) groups.push({ category: 'platform', title: t('cat_platform'), items: platforms });
  if (protocols.length > 0) groups.push({ category: 'protocol', title: t('cat_protocol'), items: protocols });
  if (others.length > 0) groups.push({ category: 'other', title: t('cat_other'), items: others });
  return groups;
});

const fetchGateways = async (options = {}) => {
  const silent = options.silent === true;
  if (!silent) loading.value = true;
  try {
    const res = await axios.get('/api/extension/cascade/gateways');
    if (res.data.code === 0) {
      const gatewayList = res.data.data || [];
      gateways.value = gatewayList;
      if (selectedGateway.value) {
        const updatedGateway = gatewayList.find((gw) => gw.sn === selectedGateway.value.sn);
        if (updatedGateway) {
          selectedGateway.value = updatedGateway;
        } else {
          closeGateway();
        }
      }
    } else {
      showToast('danger', gt('gateway_load_failed'));
    }
  } catch (e) {
    showToast('danger', gt('gateway_load_failed'));
  } finally {
    if (!silent) loading.value = false;
  }
};

const openGateway = (gateway) => {
  if (!gateway?.online) return;
  selectedGateway.value = gateway;
  openMarketplace();
  gatewayPlugins.value = [];
};

const closeGateway = () => {
  selectedGateway.value = null;
  openMarketplace();
  gatewayPlugins.value = [];
};

const openMarketplace = () => {
  selectedWorkspaceView.value = 'marketplace';
  selectedPluginName.value = '';
};

const openPlugin = (pluginName) => {
  selectedWorkspaceView.value = 'plugin';
  selectedPluginName.value = pluginName;
};

const openSystem = () => {
  selectedWorkspaceView.value = 'system';
  selectedPluginName.value = '';
};

const openLicense = () => {
  selectedWorkspaceView.value = 'license';
  selectedPluginName.value = '';
};

const openLogs = () => {
  selectedWorkspaceView.value = 'logs';
  selectedPluginName.value = '';
};

const refreshGatewayWorkspace = async () => {
  await fetchGateways({ silent: true });
};

const clearGatewayStatusTimers = () => {
  if (gatewaySSEFetchDebounceTimer) clearTimeout(gatewaySSEFetchDebounceTimer);
  if (gatewaySSEHeartbeatTimer) clearTimeout(gatewaySSEHeartbeatTimer);
  if (gatewaySSEReconnectTimer) clearTimeout(gatewaySSEReconnectTimer);
  gatewaySSEFetchDebounceTimer = null;
  gatewaySSEHeartbeatTimer = null;
  gatewaySSEReconnectTimer = null;
};

const debouncedGatewayStatusFetch = () => {
  if (gatewaySSEFetchDebounceTimer) clearTimeout(gatewaySSEFetchDebounceTimer);
  gatewaySSEFetchDebounceTimer = setTimeout(() => {
    fetchGateways({ silent: true });
  }, 300);
};

const resetGatewayStatusHeartbeat = () => {
  if (gatewaySSEHeartbeatTimer) clearTimeout(gatewaySSEHeartbeatTimer);
  gatewaySSEHeartbeatTimer = setTimeout(() => {
    reconnectGatewayStatusStream();
  }, 45000);
};

const teardownGatewayStatusStream = () => {
  clearGatewayStatusTimers();
  if (gatewayEventSource) {
    gatewayEventSource.close();
    gatewayEventSource = null;
  }
};

const setupGatewayStatusStream = () => {
  if (typeof EventSource === 'undefined') return;
  teardownGatewayStatusStream();
  gatewayEventSource = new EventSource('/api/devices/stream');

  const handleGatewayStatusEvent = () => {
    resetGatewayStatusHeartbeat();
    debouncedGatewayStatusFetch();
  };

  gatewayEventSource.addEventListener('connected', () => {
    resetGatewayStatusHeartbeat();
  });
  gatewayEventSource.addEventListener('device.list.changed', handleGatewayStatusEvent);
  gatewayEventSource.addEventListener('device.status.changed', handleGatewayStatusEvent);
  gatewayEventSource.addEventListener('heartbeat', resetGatewayStatusHeartbeat);
  gatewayEventSource.onopen = () => {
    resetGatewayStatusHeartbeat();
    fetchGateways({ silent: true });
  };
  gatewayEventSource.onerror = () => {
    resetGatewayStatusHeartbeat();
    if (gatewayEventSource?.readyState === EventSource.CLOSED) {
      gatewaySSEReconnectTimer = setTimeout(setupGatewayStatusStream, 3000);
    }
  };
};

const reconnectGatewayStatusStream = () => {
  if (gatewayEventSource) {
    gatewayEventSource.close();
    gatewayEventSource = null;
  }
  fetchGateways({ silent: true });
  gatewaySSEReconnectTimer = setTimeout(setupGatewayStatusStream, 1000);
};

const getPluginTitle = (plugin) => {
  const title = plugin?.title;
  if (!title) return plugin?.name || '';
  if (typeof title === 'string') return title;
  return title[locale.value] || title.en || plugin.name;
};

const formatTime = (value) => {
  return gatewayDateTime(locale.value, value);
};

onMounted(() => {
  fetchGateways();
  setupGatewayStatusStream();
});
onBeforeUnmount(teardownGatewayStatusStream);
</script>

<style scoped>
.gateway-management-page {
  min-height: 100%;
}

.gateway-page-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
  margin-bottom: 1.5rem;
}

.gateway-page-kicker {
  color: var(--accent-color);
  font-size: 0.72rem;
  font-weight: 700;
  letter-spacing: 0.08em;
  margin-bottom: 0.35rem;
  text-transform: uppercase;
}

.gateway-page-title {
  color: var(--text-main);
  font-size: 1.35rem;
  font-weight: 750;
  margin: 0 0 0.35rem;
}

.gateway-page-subtitle {
  color: var(--text-secondary);
  font-size: 0.9rem;
}

.gateway-refresh-btn {
  border-radius: 0.45rem;
  flex: 0 0 auto;
}

.gateway-summary-grid {
  display: grid;
  gap: 1rem;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  margin-bottom: 1.25rem;
}

.gateway-summary-card {
  align-items: center;
  background: linear-gradient(135deg, var(--bg-surface), color-mix(in srgb, var(--bg-body) 64%, var(--bg-surface)));
  border: 1px solid color-mix(in srgb, var(--border-color) 78%, transparent);
  border-radius: 0.5rem;
  box-shadow: var(--card-shadow);
  display: flex;
  gap: 0.9rem;
  min-height: 5.25rem;
  padding: 1rem 1.1rem;
}

.summary-icon {
  align-items: center;
  border-radius: 0.45rem;
  display: inline-flex;
  flex: 0 0 auto;
  height: 2.5rem;
  justify-content: center;
  width: 2.5rem;
}

.summary-icon-total {
  background: rgba(59, 130, 246, 0.12);
  color: #2563eb;
}

.summary-icon-online {
  background: rgba(16, 185, 129, 0.12);
  color: #059669;
}

.summary-icon-enabled {
  background: rgba(14, 165, 233, 0.12);
  color: #0284c7;
}

.summary-value {
  color: var(--text-main);
  font-size: 1.45rem;
  font-weight: 800;
  line-height: 1.1;
}

.summary-label {
  color: var(--text-secondary);
  font-size: 0.78rem;
  margin-top: 0.25rem;
}

.gateway-card-grid {
  display: grid;
  gap: 1rem;
  grid-template-columns: repeat(auto-fill, minmax(18rem, 1fr));
}

.gateway-card {
  background:
    linear-gradient(180deg, color-mix(in srgb, var(--bg-surface) 94%, var(--accent-color)), var(--bg-surface));
  border: 1px solid color-mix(in srgb, var(--border-color) 84%, transparent);
  border-radius: 0.5rem;
  box-shadow: var(--card-shadow);
  display: flex;
  flex-direction: column;
  min-height: 18rem;
  padding: 1.1rem;
  transition: border-color 0.18s ease, box-shadow 0.18s ease, transform 0.18s ease;
}

.gateway-card:hover {
  border-color: color-mix(in srgb, var(--accent-color) 34%, var(--border-color));
  box-shadow: 0 16px 38px rgb(15 23 42 / 0.12);
  transform: translateY(-2px);
}

.gateway-card.is-online {
  animation: gatewayOnlineBreath 2.8s ease-in-out infinite;
  border-color: rgba(34, 197, 94, 0.28);
  box-shadow:
    0 0 0 1px rgba(34, 197, 94, 0.16),
    0 0 18px rgba(34, 197, 94, 0.18),
    var(--card-shadow);
}

@keyframes gatewayOnlineBreath {
  0%,
  100% {
    box-shadow:
      0 0 0 1px rgba(34, 197, 94, 0.12),
      0 0 14px rgba(34, 197, 94, 0.14),
      var(--card-shadow);
  }

  50% {
    box-shadow:
      0 0 0 1px rgba(34, 197, 94, 0.26),
      0 0 28px rgba(34, 197, 94, 0.28),
      0 0 46px rgba(34, 197, 94, 0.14),
      var(--card-shadow);
  }
}

.gateway-card.is-unavailable {
  opacity: 0.72;
}

.gateway-card.is-unavailable:hover {
  border-color: color-mix(in srgb, var(--border-color) 84%, transparent);
  box-shadow: var(--card-shadow);
  transform: none;
}

.gateway-card-topline {
  align-items: center;
  display: flex;
  gap: 0.5rem;
  justify-content: space-between;
  margin-bottom: 1.1rem;
}

.gateway-status-pill,
.gateway-enabled-pill {
  align-items: center;
  border-radius: 999px;
  display: inline-flex;
  font-size: 0.74rem;
  font-weight: 700;
  gap: 0.4rem;
  line-height: 1;
  min-height: 1.65rem;
  padding: 0 0.65rem;
  white-space: nowrap;
}

.gateway-status-pill.is-online {
  background: rgba(16, 185, 129, 0.12);
  color: #047857;
}

.gateway-status-pill.is-offline {
  background: rgba(100, 116, 139, 0.12);
  color: var(--text-secondary);
}

.gateway-enabled-pill.is-enabled {
  background: rgba(59, 130, 246, 0.12);
  color: #2563eb;
}

.gateway-enabled-pill.is-disabled {
  background: color-mix(in srgb, var(--bg-body) 82%, var(--bg-surface));
  color: var(--text-secondary);
}

.gateway-status-dot {
  border-radius: 50%;
  display: inline-block;
  height: 0.45rem;
  width: 0.45rem;
}

.is-online .gateway-status-dot {
  background: #10b981;
  box-shadow: 0 0 0 0.22rem rgba(16, 185, 129, 0.16);
}

.is-offline .gateway-status-dot {
  background: #94a3b8;
  box-shadow: 0 0 0 0.22rem rgba(148, 163, 184, 0.14);
}

.gateway-card-main {
  align-items: center;
  display: flex;
  gap: 0.85rem;
  margin-bottom: 1rem;
}

.gateway-avatar {
  align-items: center;
  background: color-mix(in srgb, var(--accent-color) 12%, var(--bg-body));
  border: 1px solid color-mix(in srgb, var(--accent-color) 20%, var(--border-color));
  border-radius: 0.5rem;
  color: var(--accent-color);
  display: inline-flex;
  flex: 0 0 auto;
  font-size: 1.25rem;
  height: 3rem;
  justify-content: center;
  width: 3rem;
}

.gateway-title-group {
  min-width: 0;
}

.gateway-name {
  color: var(--text-main);
  font-size: 1rem;
  font-weight: 750;
  line-height: 1.25;
  margin: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.gateway-product {
  color: var(--text-secondary);
  font-size: 0.78rem;
  margin-top: 0.3rem;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.gateway-meta-panel {
  background: color-mix(in srgb, var(--bg-body) 74%, var(--bg-surface));
  border: 1px solid color-mix(in srgb, var(--border-color) 78%, transparent);
  border-radius: 0.5rem;
  display: grid;
  gap: 0.75rem;
  margin-bottom: 1rem;
  padding: 0.85rem;
}

.gateway-meta-row {
  align-items: center;
  display: flex;
  gap: 0.75rem;
  justify-content: space-between;
  min-width: 0;
}

.gateway-meta-label {
  color: var(--text-secondary);
  flex: 0 0 auto;
  font-size: 0.72rem;
  font-weight: 700;
}

.gateway-meta-value,
.gateway-sn {
  color: var(--text-main);
  font-size: 0.78rem;
  min-width: 0;
  overflow: hidden;
  text-align: right;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.gateway-sn {
  background: transparent;
  color: var(--accent-color);
}

.gateway-enter-btn {
  align-items: center;
  border-radius: 0.45rem;
  display: inline-flex;
  font-weight: 700;
  gap: 0.35rem;
  justify-content: center;
  margin-top: auto;
  min-height: 2.4rem;
  width: 100%;
}

.gateway-enter-btn i {
  font-size: 1.15rem;
}

.gateway-workspace-overlay {
  background: color-mix(in srgb, var(--bg-body) 92%, transparent);
  backdrop-filter: blur(12px);
  inset: 0;
  padding: 1rem;
  position: fixed;
  z-index: 1060;
}

.gateway-workspace-shell {
  background: var(--bg-body);
  border: 1px solid color-mix(in srgb, var(--border-color) 82%, transparent);
  border-radius: 0.5rem;
  box-shadow: 0 24px 70px rgb(15 23 42 / 0.22);
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
  overflow: hidden;
  position: relative;
}

.gateway-workspace-shell.gateway-blue-breath {
  animation: gatewayBlueBreath 2.8s ease-in-out infinite;
  box-shadow:
    0 0 0 1px color-mix(in srgb, var(--accent-color) 38%, transparent),
    0 0 22px color-mix(in srgb, var(--accent-color) 36%, transparent),
    0 0 46px color-mix(in srgb, var(--accent-color) 22%, transparent),
    0 24px 70px rgb(15 23 42 / 0.22);
}

@keyframes gatewayBlueBreath {
  0%,
  100% {
    box-shadow:
      0 0 0 1px color-mix(in srgb, var(--accent-color) 30%, transparent),
      0 0 16px color-mix(in srgb, var(--accent-color) 24%, transparent),
      0 0 34px color-mix(in srgb, var(--accent-color) 14%, transparent),
      0 24px 70px rgb(15 23 42 / 0.22);
  }

  50% {
    box-shadow:
      0 0 0 1px color-mix(in srgb, var(--accent-color) 58%, transparent),
      0 0 30px color-mix(in srgb, var(--accent-color) 46%, transparent),
      0 0 64px color-mix(in srgb, var(--accent-color) 28%, transparent),
      0 24px 70px rgb(15 23 42 / 0.22);
  }
}

.gateway-workspace-header {
  align-items: center;
  background: linear-gradient(180deg, var(--bg-surface), color-mix(in srgb, var(--bg-surface) 72%, var(--bg-body)));
  border-bottom: 1px solid var(--border-color);
  display: flex;
  gap: 1rem;
  justify-content: space-between;
  min-height: 5.25rem;
  padding: 1rem 1.25rem;
}

.gateway-workspace-title {
  align-items: center;
  display: flex;
  gap: 0.8rem;
  min-width: 0;
}

.gateway-workspace-close {
  align-items: center;
  border-radius: 0.45rem;
  display: inline-flex;
  flex: 0 0 auto;
  height: 2.25rem;
  justify-content: center;
  padding: 0;
  width: 2.25rem;
}

.gateway-workspace-icon {
  align-items: center;
  background: color-mix(in srgb, var(--accent-color) 14%, var(--bg-body));
  border: 1px solid color-mix(in srgb, var(--accent-color) 22%, var(--border-color));
  border-radius: 0.5rem;
  color: var(--accent-color);
  display: inline-flex;
  flex: 0 0 auto;
  font-size: 1.25rem;
  height: 2.75rem;
  justify-content: center;
  width: 2.75rem;
}

.gateway-workspace-kicker,
.gateway-workspace-sn {
  color: var(--text-secondary);
  font-size: 0.76rem;
  line-height: 1.25;
}

.gateway-workspace-title h5 {
  color: var(--text-main);
  font-size: 1.08rem;
  font-weight: 760;
  margin: 0.1rem 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.gateway-workspace-meta {
  align-items: center;
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
  justify-content: flex-end;
}

.gateway-sync-pill {
  background: color-mix(in srgb, var(--accent-color) 10%, var(--bg-surface));
  border-radius: 999px;
  color: var(--accent-color);
  display: inline-flex;
  font-size: 0.74rem;
  font-weight: 700;
  line-height: 1;
  min-height: 1.65rem;
  padding: 0.45rem 0.65rem;
  white-space: nowrap;
}

.gateway-workspace-layout {
  display: grid;
  flex: 1;
  grid-template-columns: minmax(13rem, 17rem) minmax(0, 1fr);
  min-height: 0;
}

.gateway-workspace-menu {
  background: color-mix(in srgb, var(--bg-surface) 72%, var(--bg-body));
  border-right: 1px solid var(--border-color);
  min-height: 0;
  overflow: auto;
  padding: 1rem 0.85rem;
}

.gateway-menu-section {
  margin-top: 1rem;
}

.gateway-menu-system-section {
  border-bottom: 1px solid color-mix(in srgb, var(--border-color) 70%, transparent);
  margin-bottom: 1rem;
  padding-bottom: 0.75rem;
}

.gateway-menu-label,
.gateway-menu-group-label {
  color: var(--text-secondary);
  font-size: 0.7rem;
  font-weight: 800;
  letter-spacing: 0.06em;
  margin: 0.35rem 0 0.25rem;
  padding: 0.4rem 1.5rem 0.25rem;
  text-transform: uppercase;
}

.gateway-menu-label {
  margin-top: 0;
}

.gateway-menu-group-label {
  font-size: 0.68rem;
}

.gateway-menu-item.nav-link {
  background: transparent;
  border: 0;
  border-left: 3px solid transparent;
  border-radius: 0;
  color: #94a3b8;
  font-size: inherit;
  font-weight: 500;
  gap: 0.5rem;
  justify-content: flex-start;
  margin: 0;
  min-height: auto;
  padding: 0.75rem 1.5rem;
  text-align: left;
  width: 100%;
}

.gateway-menu-item.nav-link:hover,
.gateway-menu-item.nav-link.active {
  background-color: var(--sidebar-hover-bg);
  color: #fff;
}

.gateway-menu-item.nav-link.active {
  background-color: var(--sidebar-active-bg);
  border-left-color: var(--accent-color);
}

.gateway-menu-item span {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.gateway-menu-plugin-icon {
  flex: 0 0 auto;
  height: 1rem;
  width: 1rem;
}

.gateway-menu-empty {
  color: var(--text-secondary);
  font-size: 0.78rem;
  padding: 0.25rem 0.65rem;
}

.gateway-workspace-content {
  flex: 1;
  min-height: 0;
  overflow: auto;
  padding: 1.25rem;
}

.gateway-workspace-enter-active,
.gateway-workspace-leave-active {
  transition: opacity 0.18s ease;
}

.gateway-workspace-enter-active .gateway-workspace-shell,
.gateway-workspace-leave-active .gateway-workspace-shell {
  transition: transform 0.22s ease, opacity 0.22s ease;
}

.gateway-workspace-enter-from,
.gateway-workspace-leave-to {
  opacity: 0;
}

.gateway-workspace-enter-from .gateway-workspace-shell,
.gateway-workspace-leave-to .gateway-workspace-shell {
  opacity: 0;
  transform: translateY(18px) scale(0.985);
}

[data-bs-theme="dark"] .gateway-card:hover {
  box-shadow: 0 18px 42px rgb(0 0 0 / 0.34);
}

[data-bs-theme="dark"] .summary-icon-total,
[data-bs-theme="dark"] .gateway-enabled-pill.is-enabled {
  color: #60a5fa;
}

[data-bs-theme="dark"] .summary-icon-online,
[data-bs-theme="dark"] .gateway-status-pill.is-online {
  color: #34d399;
}

@media (max-width: 992px) {
  .gateway-summary-grid {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 576px) {
  .gateway-page-header {
    align-items: stretch;
    flex-direction: column;
  }

  .gateway-refresh-btn {
    width: 100%;
  }

  .gateway-card-grid {
    grid-template-columns: 1fr;
  }

  .gateway-workspace-overlay {
    padding: 0;
  }

  .gateway-workspace-shell {
    border-radius: 0;
  }

  .gateway-workspace-header {
    align-items: flex-start;
    flex-direction: column;
  }

  .gateway-workspace-layout {
    grid-template-columns: 1fr;
  }

  .gateway-workspace-menu {
    border-bottom: 1px solid var(--border-color);
    border-right: 0;
    display: flex;
    gap: 0.5rem;
    overflow-x: auto;
    padding: 0.75rem;
  }

  .gateway-menu-section {
    align-items: center;
    display: flex;
    gap: 0.5rem;
    margin-top: 0;
  }

  .gateway-menu-label,
  .gateway-menu-empty {
    display: none;
  }

  .gateway-menu-item {
    flex: 0 0 auto;
    width: auto;
  }

  .gateway-workspace-content {
    padding: 1rem;
  }
}
</style>
