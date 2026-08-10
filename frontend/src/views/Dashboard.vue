<template>
  <div v-if="!hasPermission" class="container py-5 text-center">
    <i class="bi bi-shield-exclamation display-1 text-muted"></i>
    <h4 class="mt-3 text-muted">{{ $t('access_denied', 'Access Denied') }}</h4>
    <p class="text-muted">{{ $t('no_permission_dashboard', 'You do not have permission to access the dashboard.') }}</p>
  </div>
  <div v-else class="dashboard-container">
    <!-- Page Header（UX 规范 §7.2 页面三段式） -->
    <div class="page-header d-flex flex-wrap align-items-center justify-content-between gap-3 mb-4">
      <div>
        <h1 class="page-title mb-1">{{ $t('page_dashboard', 'Dashboard') }}</h1>
        <p class="page-subtitle mb-0">{{ $t('gateway_subtitle') }}</p>
      </div>
      <div class="d-flex align-items-center gap-3">
        <span class="freshness small text-secondary">
          <i class="bi bi-arrow-repeat me-1"></i>{{ $t('updated_at', 'Updated') }}
          <span class="font-monospace">{{ timeStr }}</span>
        </span>
        <button class="btn btn-sm btn-outline-secondary" @click="handleRefresh">
          <i class="bi bi-arrow-clockwise me-1"></i>{{ $t('refresh', 'Refresh') }}
        </button>
      </div>
    </div>

    <!-- 首屏骨架屏（UX 规范 §8.7，轮询期间不闪烁） -->
    <template v-if="initialLoading">
      <div class="row g-3 mb-3">
        <div class="col-6 col-lg-3" v-for="i in 4" :key="'skeleton-kpi-' + i">
          <div class="card dash-card h-100">
            <div class="card-body p-4">
              <div class="skeleton mb-3" style="width: 44px; height: 44px; border-radius: var(--radius-control);"></div>
              <div class="skeleton mb-2" style="width: 55%; height: 12px;"></div>
              <div class="skeleton" style="width: 35%; height: 26px;"></div>
            </div>
          </div>
        </div>
      </div>
      <div class="row g-3">
        <div class="col-lg-4">
          <div class="card dash-card h-100">
            <div class="card-body p-4">
              <div class="skeleton mb-3" style="width: 40%; height: 18px;"></div>
              <div class="skeleton mb-2" style="height: 52px;"></div>
              <div class="skeleton" style="height: 52px;"></div>
            </div>
          </div>
        </div>
        <div class="col-lg-5">
          <div class="card dash-card h-100">
            <div class="card-body p-4">
              <div class="skeleton mb-4" style="width: 30%; height: 18px;"></div>
              <div class="d-flex justify-content-around">
                <div class="skeleton skeleton-round" style="width: 92px; height: 92px;" v-for="j in 3" :key="'ring-' + j"></div>
              </div>
            </div>
          </div>
        </div>
        <div class="col-lg-3">
          <div class="card dash-card h-100">
            <div class="card-body p-4">
              <div class="skeleton mb-3" style="width: 40%; height: 18px;"></div>
              <div class="skeleton mb-2" style="height: 24px;"></div>
              <div class="skeleton mb-2" style="height: 24px;"></div>
              <div class="skeleton" style="height: 24px;"></div>
            </div>
          </div>
        </div>
      </div>
    </template>

    <template v-else>
      <!-- KPI 行（UX 规范 §7.3 卡片网格） -->
      <div class="row g-3 mb-3">
        <div class="col-6 col-lg-3">
          <div class="card dash-card h-100">
            <div class="card-body p-4 d-flex align-items-center">
              <div class="icon-box-sm icon-chart-5 me-3">
                <i class="bi bi-plugin"></i>
              </div>
              <div class="flex-grow-1">
                <h6 class="kpi-label mb-2">{{ $t('card_total_plugins', 'Total Plugins') }}</h6>
                <div class="d-flex align-items-baseline gap-2">
                  <span class="kpi-value font-monospace">{{ stats.plugins.total }}</span>
                  <span class="badge bg-success-subtle text-success border border-success-subtle small">
                    <i class="bi bi-circle-fill me-1" style="font-size: 6px;"></i>{{ stats.plugins.active }} {{ $t('card_active_plugins', 'Active') }}
                  </span>
                </div>
              </div>
            </div>
          </div>
        </div>

        <div class="col-6 col-lg-3">
          <div class="card dash-card h-100">
            <div class="card-body p-4 d-flex align-items-center">
              <div class="icon-box-sm icon-chart-1 me-3">
                <i class="bi bi-grid-3x3-gap-fill"></i>
              </div>
              <div class="flex-grow-1">
                <h6 class="kpi-label mb-2">{{ $t('card_total_products', 'Total Products') }}</h6>
                <span class="kpi-value font-monospace">{{ stats.products.total }}</span>
              </div>
            </div>
          </div>
        </div>

        <div class="col-6 col-lg-3">
          <div class="card dash-card h-100">
            <div class="card-body p-4 d-flex align-items-center">
              <div class="icon-box-sm icon-chart-2 me-3">
                <i class="bi bi-router"></i>
              </div>
              <div class="flex-grow-1">
                <h6 class="kpi-label mb-2">{{ $t('card_total_devices', 'Total Devices') }}</h6>
                <div class="d-flex align-items-center gap-2 mb-2">
                  <span class="kpi-value font-monospace">{{ stats.devices.total }}</span>
                  <span class="small">
                    <span class="text-success">{{ stats.devices.online }} {{ $t('dev_online', 'Online') }}</span>
                    <span class="text-secondary"> / {{ stats.devices.offline }} {{ $t('dev_offline', 'Offline') }}</span>
                  </span>
                </div>
                <div class="progress" style="height: 6px;">
                  <div class="progress-bar bg-success" :style="{ width: onlineRate + '%' }"></div>
                </div>
              </div>
            </div>
          </div>
        </div>

        <div class="col-6 col-lg-3">
          <div class="card dash-card h-100 ai-copilot-card cursor-pointer" @click="openAICopilot">
            <div class="card-body p-4 d-flex align-items-center">
              <div class="icon-box-sm icon-brand me-3">
                <i class="bi bi-robot"></i>
              </div>
              <div class="flex-grow-1">
                <h6 class="kpi-label mb-2">{{ $t('ai_copilot', 'AI Copilot') }}</h6>
                <span class="badge bg-success-subtle text-success border border-success-subtle">
                  <i class="bi bi-check-lg me-1"></i> {{ $t('status_running', 'Running') }}
                </span>
              </div>
              <i class="bi bi-chat-dots-fill text-primary fs-4"></i>
            </div>
          </div>
        </div>
      </div>

      <!-- 主内容行 -->
      <div class="row g-3">
        <!-- AI 守护 -->
        <div class="col-lg-4">
          <div class="card dash-card h-100">
            <div class="card-header dash-card-header d-flex align-items-center">
              <div class="icon-box-xs icon-warning me-2">
                <i class="bi bi-shield-check"></i>
              </div>
              <h6 class="mb-0 fw-bold">{{ $t('ai_guardian', 'AI Guardian') }}</h6>
              <span class="badge bg-danger-subtle text-danger border border-danger-subtle ms-auto">{{ aiStats.anomaly_count }} {{ $t('ai_anomalies', 'Anomalies') }}</span>
            </div>
            <div class="card-body pt-0 px-4 pb-4">
              <div class="row g-2 mb-3">
                <div class="col-4">
                  <div class="metric-block p-3 text-center">
                    <div class="metric-value font-monospace text-primary">{{ aiStats.active_tasks }}</div>
                    <div class="metric-label">{{ $t('ai_active_tasks', 'Active Tasks') }}</div>
                  </div>
                </div>
                <div class="col-4">
                  <div class="metric-block p-3 text-center">
                    <div class="metric-value font-monospace" :class="getHealthColorClass(aiStats.avg_health)">
                      {{ aiStats.avg_health > 0 ? aiStats.avg_health.toFixed(1) : '-' }}
                    </div>
                    <div class="metric-label">{{ $t('ai_health_avg', 'Avg Health') }}</div>
                  </div>
                </div>
                <div class="col-4">
                  <div class="metric-block p-3 text-center">
                    <div class="metric-value font-monospace text-danger">{{ aiStats.anomaly_count }}</div>
                    <div class="metric-label">{{ $t('ai_anomalies', 'Anomalies') }}</div>
                  </div>
                </div>
              </div>

              <h6 class="text-danger small fw-bold text-uppercase mb-2" v-if="aiStats.anomalies && aiStats.anomalies.length > 0">
                <i class="bi bi-exclamation-triangle-fill me-1"></i> {{ $t('ai_risk_devices', 'Risk Devices') }}
              </h6>
              <div v-if="aiStats.anomalies && aiStats.anomalies.length > 0" class="list-group list-group-flush small">
                <div v-for="(item, idx) in aiStats.anomalies.slice(0, 3)" :key="idx" class="list-group-item px-0 py-2 d-flex justify-content-between align-items-center bg-transparent">
                  <span class="text-truncate pe-2">
                    {{ deviceDisplay(item.device_code) }} <span class="text-secondary">({{ item.property }})</span>
                  </span>
                  <span class="badge bg-danger-subtle text-danger border border-danger-subtle flex-shrink-0 font-monospace">{{ item.health_score.toFixed(1) }}</span>
                </div>
              </div>
              <div v-else class="empty-state text-center">
                <i class="bi bi-check-circle-fill text-success me-1"></i> {{ $t('ai_no_anomalies', 'No anomalies detected') }}
              </div>
            </div>
          </div>
        </div>

        <!-- 系统资源 -->
        <div class="col-lg-5">
          <div class="card dash-card h-100">
            <div class="card-header dash-card-header d-flex align-items-center">
              <i class="bi bi-server me-2 text-primary"></i>
              <h6 class="mb-0 fw-bold">{{ $t('sys_resources', 'System Resources') }}</h6>
            </div>
            <div class="card-body pt-0 px-4 pb-4">
              <div class="row g-3">
                <div class="col-4 text-center">
                  <div class="d-inline-block position-relative" style="width: 100px; height: 100px;">
                    <svg viewBox="0 0 100 100" class="w-100 h-100">
                      <circle cx="50" cy="50" r="42" fill="none" stroke="var(--chart-1)" stroke-width="8" opacity="0.1"/>
                      <circle cx="50" cy="50" r="42" fill="none" stroke="var(--chart-1)" stroke-width="8"
                        :stroke-dasharray="`${sysStats.cpu * 2.64} 264`" stroke-linecap="round"
                        transform="rotate(-90 50 50)" style="transition: stroke-dasharray 0.5s ease;"/>
                    </svg>
                    <div class="position-absolute top-50 start-50 translate-middle">
                      <span class="fw-bold fs-5 font-monospace">{{ sysStats.cpu.toFixed(0) }}%</span>
                    </div>
                  </div>
                  <div class="small fw-bold text-uppercase text-secondary mt-2">{{ $t('sys_cpu', 'CPU') }}</div>
                </div>

                <div class="col-4 text-center">
                  <div class="d-inline-block position-relative" style="width: 100px; height: 100px;">
                    <svg viewBox="0 0 100 100" class="w-100 h-100">
                      <circle cx="50" cy="50" r="42" fill="none" stroke="var(--chart-5)" stroke-width="8" opacity="0.1"/>
                      <circle cx="50" cy="50" r="42" fill="none" stroke="var(--chart-5)" stroke-width="8"
                        :stroke-dasharray="`${sysStats.memoryPercent * 2.64} 264`" stroke-linecap="round"
                        transform="rotate(-90 50 50)" style="transition: stroke-dasharray 0.5s ease;"/>
                    </svg>
                    <div class="position-absolute top-50 start-50 translate-middle text-center">
                      <span class="fw-bold fs-5 font-monospace">{{ sysStats.memoryPercent.toFixed(0) }}%</span>
                    </div>
                  </div>
                  <div class="small fw-bold text-uppercase text-secondary mt-2">{{ $t('sys_memory', 'Memory') }}</div>
                  <div class="small text-secondary font-monospace">{{ formatBytes(sysStats.memoryUsed) }} / {{ formatBytes(sysStats.memoryTotal) }}</div>
                </div>

                <div class="col-4 text-center">
                  <div class="d-inline-block position-relative" style="width: 100px; height: 100px;">
                    <svg viewBox="0 0 100 100" class="w-100 h-100">
                      <circle cx="50" cy="50" r="42" fill="none" stroke="var(--chart-6)" stroke-width="8" opacity="0.1"/>
                      <circle cx="50" cy="50" r="42" fill="none" stroke="var(--chart-6)" stroke-width="8"
                        :stroke-dasharray="`${sysStats.diskPercent * 2.64} 264`" stroke-linecap="round"
                        transform="rotate(-90 50 50)" style="transition: stroke-dasharray 0.5s ease;"/>
                    </svg>
                    <div class="position-absolute top-50 start-50 translate-middle">
                      <span class="fw-bold fs-5 font-monospace">{{ sysStats.diskPercent.toFixed(0) }}%</span>
                    </div>
                  </div>
                  <div class="small fw-bold text-uppercase text-secondary mt-2">{{ $t('sys_disk', 'Disk') }}</div>
                  <div class="small text-secondary font-monospace">{{ formatBytes(sysStats.diskUsed) }} / {{ formatBytes(sysStats.diskTotal) }}</div>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- 服务资源 -->
        <div class="col-lg-3">
          <div class="card dash-card h-100">
            <div class="card-header dash-card-header d-flex align-items-center">
              <i class="bi bi-gear-wide-connected me-2" style="color: var(--chart-5);"></i>
              <h6 class="mb-0 fw-bold">{{ $t('svc_resources', 'Service Resources') }}</h6>
            </div>
            <div class="card-body pt-0 px-4 pb-4">
              <div class="mb-3">
                <div class="d-flex justify-content-between mb-1">
                  <span class="small text-secondary text-uppercase fw-bold">{{ $t('svc_cpu', 'Service CPU') }}</span>
                  <span class="fw-bold font-monospace" style="color: var(--chart-5);">{{ sysStats.serviceCPU.toFixed(2) }}%</span>
                </div>
                <div class="progress rounded-pill" style="height: 6px;">
                  <div class="progress-bar" style="background-color: var(--chart-5);" :style="{ width: Math.min(sysStats.serviceCPU * 10, 100) + '%' }"></div>
                </div>
              </div>

              <div class="mb-3">
                <div class="d-flex justify-content-between mb-1">
                  <span class="small text-secondary text-uppercase fw-bold">{{ $t('svc_mem', 'Service Memory') }}</span>
                  <span class="fw-bold font-monospace" style="color: var(--chart-5);">{{ formatBytes(sysStats.serviceMemory) }}</span>
                </div>
                <div class="progress rounded-pill" style="height: 6px;">
                  <div class="progress-bar" style="background-color: var(--chart-5);" :style="{ width: Math.min(sysStats.serviceMemory / sysStats.memoryTotal * 100 * 5, 100) + '%' }"></div>
                </div>
              </div>

              <hr class="my-3">

              <div class="small">
                <div class="d-flex justify-content-between py-1">
                  <span class="text-secondary">{{ $t('sys_pid', 'PID') }}</span>
                  <span class="fw-bold font-monospace">{{ sysStats.pid }}</span>
                </div>
                <div class="d-flex justify-content-between py-1">
                  <span class="text-secondary">{{ $t('sys_go_routines', 'Go Routines') }}</span>
                  <span class="fw-bold font-monospace">{{ sysStats.numGoroutine }}</span>
                </div>
                <div class="d-flex justify-content-between py-1">
                  <span class="text-secondary">{{ $t('sys_gc_cycles', 'GC Cycles') }}</span>
                  <span class="fw-bold font-monospace">{{ sysStats.numGC }}</span>
                </div>
                <div class="d-flex justify-content-between py-1">
                  <span class="text-secondary">{{ $t('sys_build_info', 'Build Info') }}</span>
                  <span class="fw-bold font-monospace text-truncate" style="max-width: 80px;">{{ sysStats.goVersion }}</span>
                </div>
              </div>

              <hr class="my-3">

              <!-- 运行信息（原欢迎卡信息，UX 规范 §7.2 信息重组） -->
              <div class="small">
                <div class="d-flex justify-content-between py-1">
                  <span class="text-secondary">{{ $t('sys_version', 'Version') }}</span>
                  <span class="fw-bold font-monospace">{{ sysStats.version }}</span>
                </div>
                <div class="d-flex justify-content-between py-1">
                  <span class="text-secondary">{{ $t('sys_ip', 'IP Address') }}</span>
                  <span class="fw-bold font-monospace">{{ sysStats.ip || '-' }}</span>
                </div>
                <div class="d-flex justify-content-between py-1">
                  <span class="text-secondary">{{ $t('sys_uptime', 'Uptime') }}</span>
                  <span class="fw-bold font-monospace">{{ formatUptime(sysStats.uptime) }}</span>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </template>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted, computed } from 'vue';
import axios from 'axios';
import { useI18n } from 'vue-i18n';
import { useAuthStore } from '../stores/auth.js';
import { formatNamedReference } from '../utils/entityDisplay.js';

const { t } = useI18n();
const authStore = useAuthStore();
const hasPermission = computed(() => authStore.hasPermission('dashboard:view'));

const stats = ref({
  plugins: { total: 0, active: 0 },
  products: { total: 0 },
  devices: { total: 0, online: 0, offline: 0 }
});

const sysStats = ref({
  cpu: 0,
  memoryTotal: 1024 * 1024 * 1024 * 4, // Default 4GB
  memoryUsed: 0,
  memoryPercent: 0,
  diskTotal: 1024 * 1024 * 1024 * 100, // Default 100GB
  diskUsed: 0,
  diskPercent: 0,
  serviceCPU: 0,
  serviceMemory: 0,
  uptime: 0,
  ip: '',
  os: '',
  arch: '',
  version: 'v1.0.0',
  pid: 0,
  numGoroutine: 0,
  numGC: 0,
  goVersion: ''
});

const aiStats = ref({
    total_tasks: 0,
    active_tasks: 0,
    avg_health: 0,
    anomaly_count: 0,
    anomalies: []
});
const deviceNames = ref({});
const deviceDisplay = (code) => formatNamedReference(deviceNames.value[code], code);

// 数据新鲜度（UX 规范 §10.1）：最近一次成功拉取时间
const lastUpdated = ref(null);
const timeStr = computed(() => {
  if (!lastUpdated.value) return '--:--:--';
  const d = lastUpdated.value;
  const pad = (n) => String(n).padStart(2, '0');
  return `${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`;
});

// 首屏骨架屏（UX 规范 §8.7）：首次数据返回后关闭，轮询期间不闪烁
const initialLoading = ref(true);

const fetchAIStats = async () => {
    if (!authStore.hasPermission('plugin:list')) return;
    try {
        const res = await axios.get('/api/plugins/ai_predict/stats');
        if (res.data.code === 0 && res.data.data) {
            aiStats.value = res.data.data;
        }
    } catch (e) {
        // AI plugin might be disabled
    }
};

const getHealthColorClass = (score) => {
    if (!score) return 'text-muted';
    if (score >= 80) return 'text-success';
    if (score >= 60) return 'text-warning';
    return 'text-danger';
};

const openAICopilot = () => {
    window.dispatchEvent(new CustomEvent('noyo-open-copilot'));
};

let pollTimer = null;

const onlineRate = computed(() => {
  if (stats.value.devices.total === 0) return 0;
  return Math.round((stats.value.devices.online / stats.value.devices.total) * 100);
});

const fetchDashboardData = async () => {
    try {
        if (authStore.hasPermission('plugin:list')) {
            const resPlugins = await axios.get('/api/plugins');
            if (resPlugins.data.code === 0) {
                const list = resPlugins.data.data || [];
                stats.value.plugins.total = list.length;
                stats.value.plugins.active = list.filter(p => p.status === 'running').length;
            }
        }

        if (authStore.hasPermission('device:list')) {
            const resDevices = await axios.get('/api/devices');
            if (resDevices.data.code === 0) {
                const data = resDevices.data.data;
                const list = Array.isArray(data?.list) ? data.list : (Array.isArray(data) ? data : []);
                stats.value.devices.total = data?.total || list.length;
                stats.value.devices.online = list.filter(d => d.online).length;
                stats.value.devices.offline = stats.value.devices.total - stats.value.devices.online;
                deviceNames.value = Object.fromEntries(list.map(device => [device.code, device.name]));
            }
        }

        if (authStore.hasPermission('product:list')) {
            try {
                const resProd = await axios.get('/api/products', {
                    params: { page: 1, pageSize: 1 }
                });
                if (resProd.data.code === 0) {
                    stats.value.products.total = resProd.data.total || (resProd.data.data ? resProd.data.data.length : 0);
                }
            } catch(e) {
                console.error("Product fetch error", e);
            }
        }

    } catch (e) {
        console.error("Dashboard fetch error", e);
    } finally {
        initialLoading.value = false;
    }
};

const fetchSystemStats = async () => {
    try {
        const res = await axios.get('/api/system/stats');
        if (res.data.code === 0) {
            // Map snake_case from backend to camelCase for frontend
            const data = res.data.data;
            sysStats.value = {
                cpu: data.cpu,
                memoryTotal: data.memory_total,
                memoryUsed: data.memory_used,
                memoryPercent: data.memory_percent,
                diskTotal: data.disk_total,
                diskUsed: data.disk_used,
                diskPercent: data.disk_percent,
                serviceCPU: data.service_cpu,
                serviceMemory: data.service_memory,
                uptime: data.uptime,
                ip: data.ip,
                os: data.os,
                arch: data.arch,
                version: data.version,
                pid: data.pid,
                numGoroutine: data.num_goroutine,
                numGC: data.num_gc,
                goVersion: data.go_version
            };
            lastUpdated.value = new Date();
            initialLoading.value = false;
        }
    } catch (e) {
        console.error("System stats error", e);
    }
}

const handleRefresh = () => {
    fetchDashboardData();
    fetchSystemStats();
    fetchAIStats();
}

const formatBytes = (bytes, decimals = 2) => {
    if (!+bytes) return '0 B';
    const k = 1024;
    const dm = decimals < 0 ? 0 : decimals;
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB', 'PB', 'EB', 'ZB', 'YB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return `${parseFloat((bytes / Math.pow(k, i)).toFixed(dm))} ${sizes[i]}`;
}

const formatUptime = (seconds) => {
    // UX 规范 §10.3：无数据时返回占位符，不使用模拟数据
    if (!seconds) return '-';
    const d = Math.floor(seconds / (3600*24));
    const h = Math.floor(seconds % (3600*24) / 3600);
    const m = Math.floor(seconds % 3600 / 60);
    return `${d}d ${h}h ${m}m`;
}

onMounted(() => {
  fetchDashboardData();
  fetchSystemStats();
  fetchAIStats();
  // Poll system stats every 3 seconds
  pollTimer = setInterval(() => {
    fetchSystemStats();
    fetchAIStats();
  }, 3000);
});

onUnmounted(() => {
  if (pollTimer) clearInterval(pollTimer);
});
</script>

<style scoped>
.dashboard-container {
  padding-bottom: 1rem;
}

/* ===== Page Header（UX 规范 §7.2） ===== */
.page-title {
  font-size: 1.5rem;
  font-weight: 600;
  line-height: 1.3;
  color: var(--text-main);
}

.page-subtitle {
  font-size: 0.75rem;
  color: var(--text-secondary);
}

.freshness {
  display: inline-flex;
  align-items: center;
  gap: 0.25rem;
  padding: 0.375rem 0.75rem;
  border-radius: var(--radius-control);
  background: var(--bg-hover);
}

/* ===== 卡片（UX 规范 §3.5 / §7.3） ===== */
.dash-card {
  background-color: var(--bg-surface);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-card);
  box-shadow: var(--card-shadow);
  transition: transform 0.15s cubic-bezier(0.2, 0, 0, 1), box-shadow 0.15s cubic-bezier(0.2, 0, 0, 1);
}

.dash-card:hover {
  transform: translateY(-1px);
  box-shadow: 0 6px 16px rgba(0, 0, 0, 0.08);
}

[data-bs-theme="dark"] .dash-card {
  box-shadow: var(--card-shadow);
}

[data-bs-theme="dark"] .dash-card:hover {
  box-shadow: 0 6px 16px rgba(0, 0, 0, 0.4);
  border-color: rgba(96, 165, 250, 0.3);
}

.dash-card-header {
  background-color: transparent;
  border-bottom: 1px solid var(--border-color);
  padding: 0.875rem 1.5rem;
}

.dash-card .card-body {
  padding: 1.5rem;
}

/* ===== KPI（UX 规范 §7.3 / §5.1） ===== */
.kpi-label {
  font-size: 0.75rem;
  font-weight: 600;
  letter-spacing: 0.05em;
  text-transform: uppercase;
  color: var(--text-secondary);
}

.kpi-value {
  font-size: 1.5rem;
  font-weight: 700;
  line-height: 1.3;
  color: var(--text-main);
}

.icon-box-sm {
  width: 44px;
  height: 44px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  border-radius: var(--radius-control);
  font-size: 1.25rem;
}

.icon-box-xs {
  width: 28px;
  height: 28px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  border-radius: var(--radius-control);
}

.icon-box-xs i {
  font-size: 0.875rem;
}

/* 图表色图标（UX 规范 §9.2 调色板，双主题 10-15% 底色） */
.icon-chart-1 { color: var(--chart-1); background-color: rgba(59, 130, 246, 0.10); }
.icon-chart-2 { color: var(--chart-2); background-color: rgba(16, 185, 129, 0.10); }
.icon-chart-5 { color: var(--chart-5); background-color: rgba(139, 92, 246, 0.10); }
[data-bs-theme="dark"] .icon-chart-1 { background-color: rgba(96, 165, 250, 0.15); }
[data-bs-theme="dark"] .icon-chart-2 { background-color: rgba(52, 211, 153, 0.15); }
[data-bs-theme="dark"] .icon-chart-5 { background-color: rgba(167, 139, 250, 0.15); }

.icon-brand {
  color: var(--accent-color);
  background-color: var(--sidebar-active-bg);
}

.icon-warning {
  color: var(--color-warning, #d97706);
  background-color: rgba(217, 119, 6, 0.10);
}

[data-bs-theme="dark"] .icon-warning {
  color: #fbbf24;
  background-color: rgba(251, 191, 36, 0.15);
}

/* ===== 指标块（AI 守护） ===== */
.metric-block {
  background: var(--bg-hover);
  border-radius: var(--radius-control);
}

.metric-value {
  font-size: 1.35rem;
  font-weight: 700;
  line-height: 1.3;
}

.metric-label {
  font-size: 0.75rem;
  color: var(--text-secondary);
  margin-top: 0.125rem;
}

.empty-state {
  color: var(--text-secondary);
  padding: 1rem 0;
}

/* ===== 骨架屏（UX 规范 §8.7） ===== */
.skeleton {
  background: var(--bg-hover);
  border-radius: 6px;
  position: relative;
  overflow: hidden;
}

.skeleton-round {
  border-radius: 50%;
}

.skeleton::after {
  content: '';
  position: absolute;
  inset: 0;
  background: linear-gradient(90deg, transparent, rgba(255, 255, 255, 0.08), transparent);
  animation: skeleton-shimmer 1.2s infinite;
}

[data-bs-theme="dark"] .skeleton::after {
  background: linear-gradient(90deg, transparent, rgba(255, 255, 255, 0.08), transparent);
}

@keyframes skeleton-shimmer {
  0% { transform: translateX(-100%); }
  100% { transform: translateX(100%); }
}

.cursor-pointer {
  cursor: pointer;
}
</style>
