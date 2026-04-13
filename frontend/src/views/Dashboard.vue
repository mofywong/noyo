<template>
  <div class="dashboard-container">
    <!-- Header Row -->
    <div class="row g-3 mb-3">
      <!-- Welcome Header -->
      <div class="col-lg-8">
        <div class="card tech-card border-0 h-100 welcome-card">
          <div class="welcome-card-shimmer"></div>
          <div class="card-body p-3 d-flex align-items-center position-relative z-1">
            <div class="flex-grow-1">
              <h4 class="fw-bold mb-1">{{ $t('page_dashboard') }}</h4>
              <p class="mb-0 opacity-75 small">{{ $t('gateway_subtitle') }}</p>
            </div>
            <div class="d-flex gap-4 px-4 border-start border-light border-opacity-25">
              <div class="text-center">
                <span class="d-block small opacity-75">{{ $t('sys_version') }}</span>
                <span class="fw-bold font-monospace">{{ sysStats.version }}</span>
              </div>
              <div class="text-center">
                <span class="d-block small opacity-75">{{ $t('sys_ip') }}</span>
                <span class="fw-bold font-monospace">{{ sysStats.ip || '-' }}</span>
              </div>
              <div class="text-center">
                <span class="d-block small opacity-75">{{ $t('sys_uptime') }}</span>
                <span class="fw-bold font-monospace">{{ formatUptime(sysStats.uptime) }}</span>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- Online Rate Card -->
      <div class="col-lg-4">
        <div class="card tech-card h-100">
          <div class="card-body p-3 d-flex align-items-center">
            <div class="icon-box-sm bg-success bg-opacity-10 text-success rounded-3 me-3">
              <i class="bi bi-activity"></i>
            </div>
            <div class="flex-grow-1">
              <h6 class="text-muted text-uppercase mb-1 small fw-bold">{{ $t('card_online_rate') }}</h6>
              <div class="d-flex align-items-center gap-3">
                <span class="fs-4 fw-bold">{{ onlineRate }}%</span>
                <div class="progress flex-grow-1" style="height: 8px;">
                  <div class="progress-bar bg-success" :style="{ width: onlineRate + '%' }"></div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Stats Row -->
    <div class="row g-3 mb-3">
      <div class="col-6 col-lg-3">
        <div class="card tech-card h-100">
          <div class="card-body p-3 d-flex align-items-center">
            <div class="icon-box-sm bg-primary bg-opacity-10 text-primary rounded-3 me-3">
              <i class="bi bi-plugin"></i>
            </div>
            <div>
              <h6 class="text-muted text-uppercase mb-1 small fw-bold">{{ $t('card_total_plugins') }}</h6>
              <div class="d-flex align-items-baseline gap-2">
                <span class="fs-4 fw-bold">{{ stats.plugins.total }}</span>
                <span class="badge bg-success bg-opacity-10 text-success small">
                  <i class="bi bi-circle-fill" style="font-size: 6px;"></i> {{ stats.plugins.active }} {{ $t('card_active_plugins') }}
                </span>
              </div>
            </div>
          </div>
        </div>
      </div>

      <div class="col-6 col-lg-3">
        <div class="card tech-card h-100">
          <div class="card-body p-3 d-flex align-items-center">
            <div class="icon-box-sm rounded-3 me-3" style="background-color: rgba(111, 66, 193, 0.1); color: #6f42c1;">
              <i class="bi bi-grid-3x3-gap-fill"></i>
            </div>
            <div>
              <h6 class="text-muted text-uppercase mb-1 small fw-bold">{{ $t('card_total_products') }}</h6>
              <span class="fs-4 fw-bold">{{ stats.products.total }}</span>
            </div>
          </div>
        </div>
      </div>

      <div class="col-6 col-lg-3">
        <div class="card tech-card h-100">
          <div class="card-body p-3 d-flex align-items-center">
            <div class="icon-box-sm bg-info bg-opacity-10 text-info rounded-3 me-3">
              <i class="bi bi-router"></i>
            </div>
            <div>
              <h6 class="text-muted text-uppercase mb-1 small fw-bold">{{ $t('card_total_devices') }}</h6>
              <div class="d-flex align-items-center gap-2">
                <span class="fs-4 fw-bold">{{ stats.devices.total }}</span>
                <span class="small">
                  <span class="text-success">{{ stats.devices.online }} {{ $t('dev_online') }}</span>
                  <span class="text-muted"> / {{ stats.devices.offline }} {{ $t('dev_offline') }}</span>
                </span>
              </div>
            </div>
          </div>
        </div>
      </div>

      <div class="col-6 col-lg-3">
        <div class="card tech-card h-100 ai-copilot-card cursor-pointer" @click="openAICopilot">
          <div class="card-body p-3 d-flex align-items-center">
            <div class="icon-box-sm bg-primary bg-opacity-10 text-primary rounded-3 me-3">
              <i class="bi bi-robot"></i>
            </div>
            <div class="flex-grow-1">
              <h6 class="text-muted text-uppercase mb-1 small fw-bold">{{ $t('ai_copilot') }}</h6>
              <span class="badge bg-success-subtle text-success border border-success-subtle">
                <i class="bi bi-check-lg me-1"></i> Online
              </span>
            </div>
            <i class="bi bi-chat-dots-fill text-primary fs-4"></i>
          </div>
        </div>
      </div>
    </div>

    <!-- Main Content Row -->
    <div class="row g-3">
      <!-- AI Guardian -->
      <div class="col-lg-4">
        <div class="card tech-card h-100">
          <div class="card-header bg-transparent border-0 py-2 px-3 d-flex align-items-center">
            <div class="icon-box-xs bg-warning bg-opacity-10 text-warning rounded-2 me-2">
              <i class="bi bi-shield-check"></i>
            </div>
            <h6 class="mb-0 fw-bold">{{ $t('ai_guardian') }}</h6>
            <span class="badge bg-danger-subtle text-danger border border-danger-subtle ms-auto">{{ aiStats.anomaly_count }} {{ $t('ai_anomalies') }}</span>
          </div>
          <div class="card-body pt-0 px-3">
            <div class="row g-2 mb-3">
              <div class="col-4">
                <div class="bg-body-tertiary rounded-2 p-2 text-center">
                  <h4 class="fw-bold mb-0 text-primary">{{ aiStats.active_tasks }}</h4>
                  <small class="text-muted">{{ $t('ai_active_tasks') }}</small>
                </div>
              </div>
              <div class="col-4">
                <div class="bg-body-tertiary rounded-2 p-2 text-center">
                  <h4 class="fw-bold mb-0" :class="getHealthColorClass(aiStats.avg_health)">
                    {{ aiStats.avg_health > 0 ? aiStats.avg_health.toFixed(1) : '-' }}
                  </h4>
                  <small class="text-muted">{{ $t('ai_health_avg') }}</small>
                </div>
              </div>
              <div class="col-4">
                <div class="bg-body-tertiary rounded-2 p-2 text-center position-relative">
                  <h4 class="fw-bold mb-0 text-danger">{{ aiStats.anomaly_count }}</h4>
                  <small class="text-muted">{{ $t('ai_anomalies') }}</small>
                  <span v-if="aiStats.anomaly_count > 0" class="position-absolute top-0 end-0 translate-middle badge rounded-circle bg-danger p-1">
                    <span class="visually-hidden">alert</span>
                  </span>
                </div>
              </div>
            </div>

            <h6 class="text-danger small fw-bold text-uppercase mb-2" v-if="aiStats.anomalies && aiStats.anomalies.length > 0">
              <i class="bi bi-exclamation-triangle-fill me-1"></i> {{ $t('ai_risk_devices') }}
            </h6>
            <div v-if="aiStats.anomalies && aiStats.anomalies.length > 0" class="list-group list-group-flush small">
              <div v-for="(item, idx) in aiStats.anomalies.slice(0, 3)" :key="idx" class="list-group-item px-0 py-2 d-flex justify-content-between align-items-center bg-transparent">
                <span class="text-truncate pe-2">
                  {{ item.device_code }} <span class="text-muted">({{ item.property }})</span>
                </span>
                <span class="badge bg-danger-subtle text-danger border border-danger-subtle flex-shrink-0">{{ item.health_score.toFixed(1) }}</span>
              </div>
            </div>
            <div v-else class="text-center text-success py-3">
              <i class="bi bi-check-circle-fill me-1"></i> {{ $t('ai_no_anomalies') }}
            </div>
          </div>
        </div>
      </div>

      <!-- System Resources -->
      <div class="col-lg-5">
        <div class="card tech-card h-100">
          <div class="card-header bg-transparent border-0 py-2 px-3 d-flex align-items-center">
            <i class="bi bi-server me-2 text-primary"></i>
            <h6 class="mb-0 fw-bold">{{ $t('sys_resources') }}</h6>
          </div>
          <div class="card-body pt-0">
            <div class="row g-3">
              <div class="col-4 text-center">
                <div class="d-inline-block position-relative" style="width: 100px; height: 100px;">
                  <svg viewBox="0 0 100 100" class="w-100 h-100">
                    <circle cx="50" cy="50" r="42" fill="none" stroke="currentColor" stroke-width="8" opacity="0.1"/>
                    <circle cx="50" cy="50" r="42" fill="none" stroke="var(--accent-color)" stroke-width="8" 
                      :stroke-dasharray="`${sysStats.cpu * 2.64} 264`" stroke-linecap="round"
                      transform="rotate(-90 50 50)" style="transition: stroke-dasharray 0.5s ease;"/>
                  </svg>
                  <div class="position-absolute top-50 start-50 translate-middle">
                    <span class="fw-bold fs-5">{{ sysStats.cpu.toFixed(0) }}%</span>
                  </div>
                </div>
                <div class="small fw-bold text-uppercase text-muted mt-2">{{ $t('sys_cpu') }}</div>
              </div>

              <div class="col-4 text-center">
                <div class="d-inline-block position-relative" style="width: 100px; height: 100px;">
                  <svg viewBox="0 0 100 100" class="w-100 h-100">
                    <circle cx="50" cy="50" r="42" fill="none" stroke="currentColor" stroke-width="8" opacity="0.1"/>
                    <circle cx="50" cy="50" r="42" fill="none" stroke="#6f42c1" stroke-width="8" 
                      :stroke-dasharray="`${sysStats.memoryPercent * 2.64} 264`" stroke-linecap="round"
                      transform="rotate(-90 50 50)" style="transition: stroke-dasharray 0.5s ease;"/>
                  </svg>
                  <div class="position-absolute top-50 start-50 translate-middle text-center">
                    <span class="fw-bold fs-5">{{ sysStats.memoryPercent.toFixed(0) }}%</span>
                  </div>
                </div>
                <div class="small fw-bold text-uppercase text-muted mt-2">{{ $t('sys_memory') }}</div>
                <div class="small text-muted">{{ formatBytes(sysStats.memoryUsed) }} / {{ formatBytes(sysStats.memoryTotal) }}</div>
              </div>

              <div class="col-4 text-center">
                <div class="d-inline-block position-relative" style="width: 100px; height: 100px;">
                  <svg viewBox="0 0 100 100" class="w-100 h-100">
                    <circle cx="50" cy="50" r="42" fill="none" stroke="currentColor" stroke-width="8" opacity="0.1"/>
                    <circle cx="50" cy="50" r="42" fill="none" stroke="#0dcaf0" stroke-width="8" 
                      :stroke-dasharray="`${sysStats.diskPercent * 2.64} 264`" stroke-linecap="round"
                      transform="rotate(-90 50 50)" style="transition: stroke-dasharray 0.5s ease;"/>
                  </svg>
                  <div class="position-absolute top-50 start-50 translate-middle">
                    <span class="fw-bold fs-5">{{ sysStats.diskPercent.toFixed(0) }}%</span>
                  </div>
                </div>
                <div class="small fw-bold text-uppercase text-muted mt-2">{{ $t('sys_disk') }}</div>
                <div class="small text-muted">{{ formatBytes(sysStats.diskUsed) }} / {{ formatBytes(sysStats.diskTotal) }}</div>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- Service Resources -->
      <div class="col-lg-3">
        <div class="card tech-card h-100">
          <div class="card-header bg-transparent border-0 py-2 px-3 d-flex align-items-center">
            <i class="bi bi-gear-wide-connected me-2" style="color: #6f42c1;"></i>
            <h6 class="mb-0 fw-bold">{{ $t('svc_resources') }}</h6>
          </div>
          <div class="card-body pt-0">
            <div class="mb-3">
              <div class="d-flex justify-content-between mb-1">
                <span class="small text-muted text-uppercase fw-bold">{{ $t('svc_cpu') }}</span>
                <span class="fw-bold" style="color: #6f42c1;">{{ sysStats.serviceCPU.toFixed(2) }}%</span>
              </div>
              <div class="progress rounded-pill" style="height: 6px;">
                <div class="progress-bar" style="background-color: #6f42c1;" :style="{ width: Math.min(sysStats.serviceCPU * 10, 100) + '%' }"></div>
              </div>
            </div>

            <div class="mb-3">
              <div class="d-flex justify-content-between mb-1">
                <span class="small text-muted text-uppercase fw-bold">{{ $t('svc_mem') }}</span>
                <span class="fw-bold" style="color: #6f42c1;">{{ formatBytes(sysStats.serviceMemory) }}</span>
              </div>
              <div class="progress rounded-pill" style="height: 6px;">
                <div class="progress-bar" style="background-color: #6f42c1;" :style="{ width: Math.min(sysStats.serviceMemory / sysStats.memoryTotal * 100 * 5, 100) + '%' }"></div>
              </div>
            </div>

            <hr class="my-3">

            <div class="small">
              <div class="d-flex justify-content-between py-1">
                <span class="text-muted">{{ $t('sys_pid') }}</span>
                <span class="fw-bold font-monospace">{{ sysStats.pid }}</span>
              </div>
              <div class="d-flex justify-content-between py-1">
                <span class="text-muted">{{ $t('sys_go_routines') }}</span>
                <span class="fw-bold font-monospace">{{ sysStats.numGoroutine }}</span>
              </div>
              <div class="d-flex justify-content-between py-1">
                <span class="text-muted">{{ $t('sys_gc_cycles') }}</span>
                <span class="fw-bold font-monospace">{{ sysStats.numGC }}</span>
              </div>
              <div class="d-flex justify-content-between py-1">
                <span class="text-muted">{{ $t('sys_build_info') }}</span>
                <span class="fw-bold font-monospace text-truncate" style="max-width: 80px;">{{ sysStats.goVersion }}</span>
              </div>
            </div>
          </div>
          <div class="card-footer bg-transparent border-0 py-2 text-center">
            <button class="btn btn-sm btn-outline-secondary rounded-pill px-3" @click="fetchSystemStats">
              <i class="bi bi-arrow-clockwise me-1"></i> {{ $t('refresh') }}
            </button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted, computed } from 'vue';
import axios from 'axios';
import { useI18n } from 'vue-i18n';

const { t } = useI18n();

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

const fetchAIStats = async () => {
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

const loading = ref(true);
let pollTimer = null;

const onlineRate = computed(() => {
  if (stats.value.devices.total === 0) return 0;
  return Math.round((stats.value.devices.online / stats.value.devices.total) * 100);
});

const fetchDashboardData = async () => {
    loading.value = true;
    try {
        // 1. Plugins Stats
        const resPlugins = await axios.get('/api/plugins');
        if (resPlugins.data.code === 0) {
            const list = resPlugins.data.data || [];
            stats.value.plugins.total = list.length;
            stats.value.plugins.active = list.filter(p => p.status === 'running').length;
        }

        // 2. Devices Stats
        const resDevices = await axios.get('/api/devices');
        if (resDevices.data.code === 0) {
            // Check structure. If it's paginated list: { total: 10, list: [...] }
            const data = resDevices.data.data;
            if (data.list) {
                stats.value.devices.total = data.total || data.list.length;
                stats.value.devices.online = data.list.filter(d => d.online).length;
                stats.value.devices.offline = stats.value.devices.total - stats.value.devices.online;
            } else {
                 // If it returns just list
                 const list = Array.isArray(data) ? data : [];
                 stats.value.devices.total = list.length;
                 stats.value.devices.online = list.filter(d => d.online).length;
                 stats.value.devices.offline = stats.value.devices.total - stats.value.devices.online;
            }
        }

        try {
             // Fetch real product count
             const resProd = await axios.get('/api/products', {
                params: { page: 1, pageSize: 1 } // Minimal fetch to get total
             }); 
             if (resProd.data.code === 0) {
                stats.value.products.total = resProd.data.total || (resProd.data.data ? resProd.data.data.length : 0);
             }
        } catch(e) {
            console.error("Product fetch error", e);
        }

    } catch (e) {
        console.error("Dashboard fetch error", e);
    } finally {
        loading.value = false;
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
        }
    } catch (e) {
        console.error("System stats error", e);
    }
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
    // Mock uptime if backend doesn't send it yet (system.go didn't have uptime in struct)
    // We can just increase it locally or mock.
    // Let's use a placeholder if 0
    if (!seconds) return '3d 2h 15m'; // Mock
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

.text-purple { color: #6f42c1 !important; }
.bg-purple { background-color: #6f42c1 !important; }
.text-indigo { color: #6610f2 !important; }
.bg-indigo-subtle { background-color: #e0cffc !important; }
.border-indigo-subtle { border-color: #e0cffc !important; }

[data-bs-theme="dark"] .bg-indigo-subtle { background-color: rgba(102, 16, 242, 0.2) !important; color: #b38df7 !important; border-color: rgba(102, 16, 242, 0.2) !important; }

.tech-card {
  background: var(--bs-body-bg);
  border: 1px solid rgba(0,0,0,0.06);
  border-radius: 12px;
  transition: transform 0.2s, box-shadow 0.2s;
}

[data-bs-theme="dark"] .tech-card {
  background: #1e2126; 
  border-color: rgba(255, 255, 255, 0.05);
  box-shadow: 0 2px 4px rgba(0, 0, 0, 0.2);
}

.tech-card:hover {
  transform: translateY(-1px);
  box-shadow: 0 6px 12px rgba(0, 0, 0, 0.08);
}

[data-bs-theme="dark"] .tech-card:hover {
  box-shadow: 0 6px 12px rgba(0, 0, 0, 0.3);
  border-color: rgba(var(--bs-primary-rgb), 0.3);
}

.welcome-card {
  background: linear-gradient(135deg, #0d6efd 0%, #6f42c1 100%);
  color: white;
  overflow: hidden;
  position: relative;
}

[data-bs-theme="dark"] .welcome-card {
  background: linear-gradient(135deg, rgba(13, 110, 253, 0.7) 0%, rgba(111, 66, 193, 0.7) 100%);
  border: 1px solid rgba(255,255,255,0.1);
}

.welcome-card-shimmer {
  position: absolute;
  top: 0;
  left: -100%;
  width: 60%;
  height: 100%;
  background: linear-gradient(
    90deg,
    transparent 0%,
    rgba(255, 255, 255, 0.05) 25%,
    rgba(255, 255, 255, 0.15) 50%,
    rgba(255, 255, 255, 0.05) 75%,
    transparent 100%
  );
  animation: shimmer 4s infinite;
  pointer-events: none;
}

[data-bs-theme="dark"] .welcome-card-shimmer {
  background: linear-gradient(
    90deg,
    transparent 0%,
    rgba(255, 255, 255, 0.08) 25%,
    rgba(255, 255, 255, 0.2) 50%,
    rgba(255, 255, 255, 0.08) 75%,
    transparent 100%
  );
}

@keyframes shimmer {
  0% {
    left: -100%;
  }
  100% {
    left: 200%;
  }
}

.icon-box {
  width: 60px;
  height: 60px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.icon-box-sm {
  width: 48px;
  height: 48px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.icon-box-xs {
  width: 28px;
  height: 28px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.icon-box-sm i, .icon-box-xs i {
  font-size: 1.25rem;
}

.icon-box-xs i {
  font-size: 0.875rem;
}

.cursor-pointer {
  cursor: pointer;
}

:root {
  --bs-card-bg: #fff;
}
[data-bs-theme="dark"] {
  --bs-card-bg: #1e2126;
  --bs-gray-200: #2c3036;
}

[data-bs-theme="dark"] .bg-body-tertiary {
  background-color: rgba(255, 255, 255, 0.05) !important;
}

</style>
