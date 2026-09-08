<template>
  <div v-if="!hasPermission" class="container py-5 text-center">
    <i class="bi bi-shield-exclamation display-1 text-muted"></i>
    <h4 class="mt-3 text-muted">{{ $t('access_denied', 'Access Denied') }}</h4>
    <p class="text-muted">{{ $t('no_permission_dashboard', 'You do not have permission to access the dashboard.') }}</p>
  </div>
  <div
    v-else
    class="dashboard-container"
  >
    <!-- Page Header（UX 规范 §7.2 页面三段式） -->
    <div class="page-header d-flex flex-wrap align-items-center justify-content-between gap-3 mb-4">
      <div class="dashboard-page-heading">
        <h1 class="page-title mb-1">{{ $t('page_dashboard', 'Dashboard') }}</h1>
        <p class="page-subtitle mb-0">{{ $t('gateway_subtitle') }}</p>
      </div>
      <!-- 浮动玻璃工具栏（规格 §11：滚动自适应透明度/blur） -->
      <LiquidGlassNavbar scroll-target=".content-scroll">
        <span class="freshness small text-secondary">
          <i class="bi bi-arrow-repeat me-1"></i>{{ $t('updated_at', 'Updated') }}
          <span class="font-monospace">{{ timeStr }}</span>
        </span>
        <LiquidGlassButton
          variant="secondary"
          size="sm"
          :icon="isRefreshing ? 'bi bi-arrow-repeat spin' : 'bi bi-arrow-clockwise'"
          :disabled="isRefreshing"
          @click="handleRefresh"
        >
          {{ $t('refresh', 'Refresh') }}
        </LiquidGlassButton>
        <LiquidGlassPopover
          placement="bottom-end"
          :label="$t('data_freshness', 'Data Freshness')"
        >
          <template #trigger>
            <span class="text-secondary px-2"><i class="bi bi-info-circle"></i></span>
          </template>
          <div class="small">
            <div class="fw-semibold mb-1">{{ $t('data_freshness', 'Data Freshness') }}</div>
            <div class="text-secondary">{{ $t('auto_refresh_hint', 'Dashboard data refreshes automatically. Use Refresh for an immediate update.') }}</div>
          </div>
        </LiquidGlassPopover>
      </LiquidGlassNavbar>
    </div>

    <!-- 首屏骨架屏（UX 规范 §8.7，轮询期间不闪烁） -->
    <template v-if="initialLoading">
      <div class="dashboard-kpi-grid mb-3">
        <div
          v-for="i in 4"
          :key="'skeleton-kpi-' + i"
          class="card dashboard-card dashboard-kpi-card"
        >
          <div class="p-4">
            <div class="skeleton mb-3" style="width: 44px; height: 44px; border-radius: var(--radius-control);"></div>
            <div class="skeleton mb-2" style="width: 55%; height: 12px;"></div>
            <div class="skeleton" style="width: 35%; height: 26px;"></div>
          </div>
        </div>
      </div>
      <div class="dashboard-content-grid">
        <div
          class="card dashboard-card dashboard-content-card dashboard-content-card--guardian"
        >
          <div class="p-4">
            <div class="skeleton mb-3" style="width: 40%; height: 18px;"></div>
            <div class="skeleton mb-2" style="height: 52px;"></div>
            <div class="skeleton" style="height: 52px;"></div>
          </div>
        </div>
        <div
          class="card dashboard-card dashboard-content-card dashboard-content-card--resources"
        >
          <div class="p-4">
            <div class="skeleton mb-4" style="width: 30%; height: 18px;"></div>
            <div class="d-flex justify-content-around">
              <div class="skeleton skeleton-round" style="width: 92px; height: 92px;" v-for="j in 3" :key="'ring-' + j"></div>
            </div>
          </div>
        </div>
        <div
          class="card dashboard-card dashboard-content-card dashboard-content-card--service"
        >
          <div class="p-4">
            <div class="skeleton mb-3" style="width: 40%; height: 18px;"></div>
            <div class="skeleton mb-2" style="height: 24px;"></div>
            <div class="skeleton mb-2" style="height: 24px;"></div>
            <div class="skeleton" style="height: 24px;"></div>
          </div>
        </div>
      </div>
    </template>

    <template v-else>
      <div class="dashboard-kpi-grid mb-3">
        <div
          class="card dashboard-card dashboard-kpi-card"
          data-dashboard-card
        >
          <div class="p-4 d-flex align-items-center h-100">
            <div class="icon-box-sm icon-chart-5 me-3">
              <i class="bi bi-plugin"></i>
            </div>
            <div class="flex-grow-1">
              <h6 class="kpi-label mb-2">{{ $t('card_total_plugins', 'Total Plugins') }}</h6>
              <div class="d-flex align-items-baseline gap-2">
                <span class="kpi-value font-monospace">{{ stats.plugins.total }}</span>
                <span class="dash-pill dash-pill--success">
                  <span class="dash-pill-dot"></span>
                  <span>{{ stats.plugins.active }} {{ $t('card_active_plugins', 'Active') }}</span>
                </span>
              </div>
            </div>
          </div>
        </div>

        <div
          class="card dashboard-card dashboard-kpi-card"
          data-dashboard-card
        >
          <div class="p-4 d-flex align-items-center h-100">
            <div class="icon-box-sm icon-chart-1 me-3">
              <i class="bi bi-grid-3x3-gap-fill"></i>
            </div>
            <div class="flex-grow-1">
              <h6 class="kpi-label mb-2">{{ $t('card_total_products', 'Total Products') }}</h6>
              <div class="d-flex align-items-baseline gap-2">
                <span class="kpi-value font-monospace">{{ stats.products.total }}</span>
                <span class="dash-pill dash-pill--primary">
                  <i class="bi bi-layers-fill me-1" style="font-size: 10px;"></i>
                  <span>{{ $t('status_ready', 'Standard') }}</span>
                </span>
              </div>
            </div>
          </div>
        </div>

        <div
          class="card dashboard-card dashboard-kpi-card"
          data-dashboard-card
        >
          <div class="p-4 d-flex align-items-center h-100">
            <div class="icon-box-sm icon-chart-2 me-3">
              <i class="bi bi-router"></i>
            </div>
            <div class="flex-grow-1">
              <h6 class="kpi-label mb-2">{{ $t('card_total_devices', 'Total Devices') }}</h6>
              <div class="d-flex align-items-center gap-2 mb-2">
                <span class="kpi-value font-monospace">{{ stats.devices.total }}</span>
                <span class="small font-monospace">
                  <span class="text-success fw-semibold">{{ stats.devices.online }} {{ $t('dev_online', 'Online') }}</span>
                  <span class="text-secondary opacity-50"> / </span>
                  <span class="text-secondary">{{ stats.devices.offline }} {{ $t('dev_offline', 'Offline') }}</span>
                </span>
              </div>
              <div class="dash-progress-track">
                <div class="dash-progress-bar dash-progress-bar--devices" :style="{ width: onlineRate + '%' }"></div>
              </div>
            </div>
          </div>
        </div>

        <div
          class="card dashboard-card dashboard-kpi-card dashboard-ai-action"
          data-dashboard-card
          data-dashboard-ai-trigger
          role="button"
          tabindex="0"
          @click="openAICopilot"
          @keydown.enter="openAICopilot"
        >
          <div class="p-4 d-flex align-items-center h-100 w-100 position-relative">
            <div class="icon-box-sm icon-brand me-3 position-relative">
              <i class="bi bi-robot"></i>
              <span class="ai-spark-beacon"></span>
            </div>
            <div class="flex-grow-1">
              <h6 class="kpi-label mb-2">{{ $t('ai_copilot', 'AI Copilot') }}</h6>
              <span class="dash-pill dash-pill--brand">
                <i class="bi bi-stars me-1" style="font-size: 10px;"></i>
                <span>{{ $t('status_running', 'Running') }}</span>
              </span>
            </div>
            <div class="ai-chat-icon-wrap">
              <i class="bi bi-chat-dots-fill text-primary fs-4"></i>
            </div>
          </div>
        </div>
      </div>

      <div class="dashboard-content-grid">
        <!-- AI 守护 -->
        <div
          class="card dashboard-card dashboard-content-card dashboard-content-card--guardian"
          data-dashboard-card
        >
          <div class="card-header d-flex align-items-center">
            <div class="icon-box-xs icon-warning me-2">
              <i class="bi bi-shield-check"></i>
            </div>
            <h6 class="mb-0 fw-bold">{{ $t('ai_guardian', 'AI Guardian') }}</h6>
            <span class="dash-pill ms-auto" :class="aiStats.anomaly_count > 0 ? 'dash-pill--danger' : 'dash-pill--success'">
              <span class="dash-pill-dot" v-if="aiStats.anomaly_count > 0"></span>
              <i class="bi bi-check-circle-fill me-1" v-else style="font-size: 10px;"></i>
              <span>{{ aiStats.anomaly_count }} {{ $t('ai_anomalies', 'Anomalies') }}</span>
            </span>
          </div>
          <div class="card-body pt-0 px-4 pb-4">
            <div class="row g-2 mb-3">
              <div class="col-4">
                <div class="dash-metric-well p-3 text-center">
                  <div class="metric-value font-monospace text-primary">{{ aiStats.active_tasks }}</div>
                  <div class="metric-label">{{ $t('ai_active_tasks', 'Active Tasks') }}</div>
                </div>
              </div>
              <div class="col-4">
                <div class="dash-metric-well p-3 text-center">
                  <div class="metric-value font-monospace" :class="getHealthColorClass(aiStats.avg_health)">
                    {{ aiStats.avg_health > 0 ? aiStats.avg_health.toFixed(1) : '-' }}
                  </div>
                  <div class="metric-label">{{ $t('ai_health_avg', 'Avg Health') }}</div>
                </div>
              </div>
              <div class="col-4">
                <div class="dash-metric-well p-3 text-center">
                  <div class="metric-value font-monospace text-danger">{{ aiStats.anomaly_count }}</div>
                  <div class="metric-label">{{ $t('ai_anomalies', 'Anomalies') }}</div>
                </div>
              </div>
            </div>

            <h6 class="text-danger small fw-bold text-uppercase mb-2" v-if="aiStats.anomalies && aiStats.anomalies.length > 0">
              <i class="bi bi-exclamation-triangle-fill me-1"></i> {{ $t('ai_risk_devices', 'Risk Devices') }}
            </h6>
            <div v-if="aiStats.anomalies && aiStats.anomalies.length > 0" class="dash-risk-list small">
              <div v-for="(item, idx) in aiStats.anomalies.slice(0, 3)" :key="idx" class="dash-risk-item py-2 px-2 d-flex justify-content-between align-items-center">
                <span class="text-truncate pe-2 d-flex align-items-center gap-1">
                  <i class="bi bi-exclamation-triangle-fill text-danger me-1"></i>
                  <span>{{ deviceDisplay(item.device_code) }}</span>
                  <span class="text-secondary opacity-75">({{ item.property }})</span>
                </span>
                <span class="dash-pill dash-pill--danger font-monospace px-2 py-0">{{ item.health_score.toFixed(1) }}</span>
              </div>
            </div>
            <div v-else class="empty-state text-center py-4">
              <div class="empty-state-icon mb-2">
                <i class="bi bi-shield-check text-success fs-3"></i>
              </div>
              <div class="text-secondary small">{{ $t('ai_no_anomalies', 'No anomalies detected') }}</div>
            </div>
          </div>
        </div>

        <!-- 系统资源 -->
        <div
          class="card dashboard-card dashboard-content-card dashboard-content-card--resources"
          data-dashboard-card
        >
          <div class="card-header d-flex align-items-center">
            <i class="bi bi-server me-2 text-primary"></i>
            <h6 class="mb-0 fw-bold">{{ $t('sys_resources', 'System Resources') }}</h6>
          </div>
          <div class="card-body pt-0 px-4 pb-4">
            <div class="row g-3">
              <div class="col-4 text-center">
                <div class="noyo-glass-chart d-inline-block position-relative rounded-circle" style="width: 96px; height: 96px;">
                  <svg viewBox="0 0 100 100" class="w-100 h-100">
                    <defs>
                      <linearGradient id="dash-cpu-grad" x1="0%" y1="0%" x2="100%" y2="100%">
                        <stop offset="0%" stop-color="var(--chart-1)" />
                        <stop offset="100%" stop-color="var(--chart-6)" />
                      </linearGradient>
                    </defs>
                    <circle cx="50" cy="50" r="40" fill="none" stroke="var(--chart-1)" stroke-width="7" opacity="0.10"/>
                    <circle cx="50" cy="50" r="40" fill="none" stroke="var(--chart-1)" stroke-width="7" stroke-dasharray="1 4" opacity="0.16"/>
                    <circle cx="50" cy="50" r="40" fill="none" stroke="url(#dash-cpu-grad)" stroke-width="7"
                      :stroke-dasharray="`${sysStats.cpu * 2.513} 251.3`" stroke-linecap="round"
                      transform="rotate(-90 50 50)" class="resource-ring resource-ring--cpu"/>
                    <circle :cx="cpuCap.x" :cy="cpuCap.y" r="4" fill="var(--chart-6)" class="resource-cap resource-cap--cpu" v-if="sysStats.cpu > 1"/>
                  </svg>
                  <div class="position-absolute top-50 start-50 translate-middle">
                    <span class="fw-bold fs-5 font-monospace">{{ sysStats.cpu.toFixed(0) }}%</span>
                  </div>
                </div>
                <div class="small fw-bold text-uppercase text-secondary mt-2">{{ $t('sys_cpu', 'CPU') }}</div>
                <div class="small text-secondary font-monospace"><span class="opacity-75">{{ $t('routines_short', 'Routines') }}:</span> {{ sysStats.numGoroutine }}</div>
              </div>

              <div class="col-4 text-center">
                <div class="noyo-glass-chart d-inline-block position-relative rounded-circle" style="width: 96px; height: 96px;">
                  <svg viewBox="0 0 100 100" class="w-100 h-100">
                    <defs>
                      <linearGradient id="dash-mem-grad" x1="0%" y1="0%" x2="100%" y2="100%">
                        <stop offset="0%" stop-color="var(--chart-5)" />
                        <stop offset="100%" stop-color="var(--accent-color)" />
                      </linearGradient>
                    </defs>
                    <circle cx="50" cy="50" r="40" fill="none" stroke="var(--chart-5)" stroke-width="7" opacity="0.10"/>
                    <circle cx="50" cy="50" r="40" fill="none" stroke="var(--chart-5)" stroke-width="7" stroke-dasharray="1 4" opacity="0.16"/>
                    <circle cx="50" cy="50" r="40" fill="none" stroke="url(#dash-mem-grad)" stroke-width="7"
                      :stroke-dasharray="`${sysStats.memoryPercent * 2.513} 251.3`" stroke-linecap="round"
                      transform="rotate(-90 50 50)" class="resource-ring resource-ring--memory"/>
                    <circle :cx="memCap.x" :cy="memCap.y" r="4" fill="var(--accent-color)" class="resource-cap resource-cap--memory" v-if="sysStats.memoryPercent > 1"/>
                  </svg>
                  <div class="position-absolute top-50 start-50 translate-middle text-center">
                    <span class="fw-bold fs-5 font-monospace">{{ sysStats.memoryPercent.toFixed(0) }}%</span>
                  </div>
                </div>
                <div class="small fw-bold text-uppercase text-secondary mt-2">{{ $t('sys_memory', 'Memory') }}</div>
                <div class="small text-secondary font-monospace">{{ formatBytes(sysStats.memoryUsed) }} / {{ formatBytes(sysStats.memoryTotal) }}</div>
              </div>

              <div class="col-4 text-center">
                <div class="noyo-glass-chart d-inline-block position-relative rounded-circle" style="width: 96px; height: 96px;">
                  <svg viewBox="0 0 100 100" class="w-100 h-100">
                    <defs>
                      <linearGradient id="dash-disk-grad" x1="0%" y1="0%" x2="100%" y2="100%">
                        <stop offset="0%" stop-color="var(--chart-6)" />
                        <stop offset="100%" stop-color="var(--chart-2)" />
                      </linearGradient>
                    </defs>
                    <circle cx="50" cy="50" r="40" fill="none" stroke="var(--chart-6)" stroke-width="7" opacity="0.10"/>
                    <circle cx="50" cy="50" r="40" fill="none" stroke="var(--chart-6)" stroke-width="7" stroke-dasharray="1 4" opacity="0.16"/>
                    <circle cx="50" cy="50" r="40" fill="none" stroke="url(#dash-disk-grad)" stroke-width="7"
                      :stroke-dasharray="`${sysStats.diskPercent * 2.513} 251.3`" stroke-linecap="round"
                      transform="rotate(-90 50 50)" class="resource-ring resource-ring--disk"/>
                    <circle :cx="diskCap.x" :cy="diskCap.y" r="4" fill="var(--chart-2)" class="resource-cap resource-cap--disk" v-if="sysStats.diskPercent > 1"/>
                  </svg>
                  <div class="position-absolute top-50 start-50 translate-middle">
                    <span class="fw-bold fs-5 font-monospace">{{ sysStats.diskPercent.toFixed(0) }}%</span>
                  </div>
                </div>
                <div class="small fw-bold text-uppercase text-secondary mt-2">{{ $t('sys_disk', 'Disk') }}</div>
                <div class="small text-secondary font-monospace">{{ formatBytes(sysStats.diskUsed) }} / {{ formatBytes(sysStats.diskTotal) }}</div>
              </div>
            </div>

            <!-- 实时网关脉搏波形图 (CPU Activity Pulse Sparkline) -->
            <div class="dash-divider my-3" aria-hidden="true"></div>
            <div class="sparkline-section px-1">
              <div class="d-flex justify-content-between align-items-center mb-2">
                <span class="small fw-semibold d-flex align-items-center gap-1 text-secondary">
                  <i class="bi bi-activity text-primary"></i>
                  <span>{{ $t('cpu_load_trend', 'CPU Activity Pulse') }}</span>
                </span>
                <span class="small font-monospace text-secondary opacity-75">{{ $t('sparkline_range_hint', '~60s (3s/pt)') }}</span>
              </div>
              <div class="sparkline-container position-relative" style="height: 48px;">
                <svg viewBox="0 0 300 48" preserveAspectRatio="none" class="w-100 h-100 sparkline-svg">
                  <defs>
                    <linearGradient id="dash-sparkline-grad" x1="0" y1="0" x2="0" y2="1">
                      <stop offset="0%" stop-color="var(--chart-1)" stop-opacity="0.32" />
                      <stop offset="100%" stop-color="var(--chart-1)" stop-opacity="0.0" />
                    </linearGradient>
                  </defs>
                  <path :d="sparklineAreaPath" fill="url(#dash-sparkline-grad)" />
                  <path :d="sparklineLinePath" fill="none" stroke="var(--chart-1)" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="sparkline-path" />
                  <circle v-if="sparklinePoints.length > 0" :cx="sparklinePoints[sparklinePoints.length - 1].x" :cy="sparklinePoints[sparklinePoints.length - 1].y" r="3.5" fill="var(--chart-1)" class="sparkline-tip-dot" />
                </svg>
              </div>
            </div>
          </div>
        </div>

        <!-- 服务资源 -->
        <div
          class="card dashboard-card dashboard-content-card dashboard-content-card--service"
          data-dashboard-card
        >
          <div class="card-header d-flex align-items-center">
            <i class="bi bi-gear-wide-connected me-2" style="color: var(--chart-5);"></i>
            <h6 class="mb-0 fw-bold">{{ $t('svc_resources', 'Service Resources') }}</h6>
          </div>
          <div class="card-body mx-3 mb-3 p-3">
            <div class="mb-3">
              <div class="d-flex justify-content-between mb-1">
                <span class="small text-secondary text-uppercase fw-bold">{{ $t('svc_cpu', 'Service CPU') }}</span>
                <span class="fw-bold font-monospace">{{ sysStats.serviceCPU.toFixed(2) }}%</span>
              </div>
              <div class="dash-progress-track">
                <div class="dash-progress-bar dash-progress-bar--service" :style="{ width: Math.min(sysStats.serviceCPU * 10, 100) + '%' }"></div>
              </div>
            </div>

            <div class="mb-3">
              <div class="d-flex justify-content-between mb-1">
                <span class="small text-secondary text-uppercase fw-bold">{{ $t('svc_mem', 'Service Memory') }}</span>
                <span class="fw-bold font-monospace">{{ formatBytes(sysStats.serviceMemory) }}</span>
              </div>
              <div class="dash-progress-track">
                <div class="dash-progress-bar dash-progress-bar--service" :style="{ width: Math.min(sysStats.serviceMemory / sysStats.memoryTotal * 100 * 5, 100) + '%' }"></div>
              </div>
            </div>

            <div class="dash-divider my-3" aria-hidden="true"></div>

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

            <div class="dash-divider my-3" aria-hidden="true"></div>

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
    </template>

  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted, computed } from 'vue';
import axios from 'axios';
import { useI18n } from 'vue-i18n';
import { useAuthStore } from '../stores/auth.js';
import { formatNamedReference } from '../utils/entityDisplay.js';
import LiquidGlassButton from '../components/liquid-glass/LiquidGlassButton.vue';
import LiquidGlassNavbar from '../components/liquid-glass/LiquidGlassNavbar.vue';
import LiquidGlassPopover from '../components/liquid-glass/LiquidGlassPopover.vue';

const { t } = useI18n();
const authStore = useAuthStore();
const hasPermission = computed(() => authStore.hasPermission('dashboard:view'));

const stats = ref({
  plugins: { total: 0, active: 0 },
  products: { total: 0 },
  devices: { total: 0, online: 0, offline: 0 }
});

const isRefreshing = ref(false);

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

// 计算精密环形图指针端点位置
const getCapCoords = (percent, radius = 40, cx = 50, cy = 50) => {
  const p = Math.max(0, Math.min(100, percent || 0));
  const angleDeg = -90 + (p * 3.6);
  const rad = (angleDeg * Math.PI) / 180;
  return {
    x: +(cx + radius * Math.cos(rad)).toFixed(2),
    y: +(cy + radius * Math.sin(rad)).toFixed(2)
  };
};

const cpuCap = computed(() => getCapCoords(sysStats.value.cpu));
const memCap = computed(() => getCapCoords(sysStats.value.memoryPercent));
const diskCap = computed(() => getCapCoords(sysStats.value.diskPercent));

// 实时网关脉搏波形图 (CPU Activity Sparkline 历史队列与贝塞尔曲线)
const cpuHistory = ref([15, 22, 18, 25, 28, 24, 20, 29, 26, 32, 28, 25, 21, 26, 27, 30, 26, 24, 21, 20]);
const sparklineWidth = 300;
const sparklineHeight = 48;

const sparklinePoints = computed(() => {
  const data = cpuHistory.value;
  if (!data || data.length === 0) return [];
  const step = sparklineWidth / Math.max(data.length - 1, 1);
  return data.map((val, idx) => {
    const x = +(idx * step).toFixed(1);
    const clamped = Math.max(2, Math.min(96, val));
    const y = +(sparklineHeight - 4 - ((clamped / 100) * (sparklineHeight - 10))).toFixed(1);
    return { x, y };
  });
});

const sparklineLinePath = computed(() => {
  const pts = sparklinePoints.value;
  if (pts.length < 2) return '';
  let d = `M ${pts[0].x},${pts[0].y}`;
  for (let i = 0; i < pts.length - 1; i++) {
    const p0 = i > 0 ? pts[i - 1] : pts[i];
    const p1 = pts[i];
    const p2 = pts[i + 1];
    const p3 = i < pts.length - 2 ? pts[i + 2] : p2;
    const cp1x = +(p1.x + (p2.x - p0.x) / 6).toFixed(1);
    const cp1y = +(p1.y + (p2.y - p0.y) / 6).toFixed(1);
    const cp2x = +(p2.x - (p3.x - p1.x) / 6).toFixed(1);
    const cp2y = +(p2.y - (p3.y - p1.y) / 6).toFixed(1);
    d += ` C ${cp1x},${cp1y} ${cp2x},${cp2y} ${p2.x},${p2.y}`;
  }
  return d;
});

const sparklineAreaPath = computed(() => {
  const line = sparklineLinePath.value;
  if (!line) return '';
  const pts = sparklinePoints.value;
  const lastX = pts[pts.length - 1].x;
  return `${line} L ${lastX},${sparklineHeight} L 0,${sparklineHeight} Z`;
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
            if (data.cpu !== undefined) {
              cpuHistory.value.push(Math.max(2, Math.min(98, Number(data.cpu) || 0)));
              if (cpuHistory.value.length > 20) {
                cpuHistory.value.shift();
              }
            }
            lastUpdated.value = new Date();
            initialLoading.value = false;
        }
    } catch (e) {
        console.error("System stats error", e);
    }
}

const handleRefresh = async () => {
    if (isRefreshing.value) return;
    isRefreshing.value = true;
    try {
      await Promise.all([
        fetchDashboardData(),
        fetchSystemStats(),
        fetchAIStats()
      ]);
    } finally {
      setTimeout(() => {
        isRefreshing.value = false;
      }, 600);
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
  padding-bottom: 2rem;
  position: relative;
}

/* ===== 1. Page Header ===== */
.page-title {
  font-size: 1.6rem;
  font-weight: 700;
  line-height: 1.3;
  color: var(--text-primary);
  letter-spacing: -0.02em;
}

.page-subtitle {
  font-size: 0.75rem;
  color: var(--text-secondary);
}

.freshness {
  display: inline-flex;
  align-items: center;
  gap: 0.25rem;
  padding: 0.4rem 0.85rem;
  border-radius: 9999px;
  background: var(--bg-surface);
  backdrop-filter: blur(20px) saturate(180%);
  -webkit-backdrop-filter: blur(20px) saturate(180%);
  border: 1px solid var(--border-color);
  box-shadow: var(--shadow-sm);
}

/* ===== 2. Grid Layouts & Apple Liquid Glass Cards ===== */
.dashboard-kpi-grid {
  display: grid;
  gap: 16px;
  grid-template-columns: repeat(4, minmax(0, 1fr));
}

.dashboard-content-grid {
  display: grid;
  gap: 16px;
  grid-template-columns: minmax(0, 4fr) minmax(0, 5fr) minmax(0, 3fr);
}

.dashboard-card {
  position: relative;
  overflow: hidden;
  isolation: isolate;
  border-radius: var(--radius-card, 18px);
  background: var(--noyo-dashboard-liquid-tint);
  backdrop-filter: blur(var(--noyo-dashboard-liquid-blur, 10px)) saturate(175%) brightness(var(--noyo-dashboard-liquid-backdrop-brightness, 1.05));
  -webkit-backdrop-filter: blur(var(--noyo-dashboard-liquid-blur, 10px)) saturate(175%) brightness(var(--noyo-dashboard-liquid-backdrop-brightness, 1.05));
  border: 1px solid var(--noyo-dashboard-liquid-edge);
  box-shadow:
    var(--noyo-dashboard-liquid-shadow),
    inset 0 1.5px 0.5px var(--noyo-dashboard-liquid-specular),
    inset 0 -1.5px 1px rgba(0, 0, 0, 0.05),
    inset 0 0 16px rgba(255, 255, 255, 0.25);
  transition:
    transform 0.45s cubic-bezier(0.34, 1.56, 0.64, 1),
    box-shadow 0.35s ease,
    background 0.25s ease,
    border-color 0.25s ease;
}

[data-bs-theme="dark"] .dashboard-card {
  box-shadow:
    var(--noyo-dashboard-liquid-shadow),
    inset 0 1.5px 0.5px var(--noyo-dashboard-liquid-specular),
    inset 0 -1.5px 1px rgba(0, 0, 0, 0.45),
    inset 0 0 16px rgba(255, 255, 255, 0.03);
}

/* 玻璃顶部弧形镜面反光 (Top Curved Specular Arc) */
.dashboard-card::before {
  content: '';
  position: absolute;
  inset: 0;
  border-radius: inherit;
  pointer-events: none;
  background: linear-gradient(
    180deg,
    rgba(255, 255, 255, 0.65) 0%,
    rgba(255, 255, 255, 0.15) 28%,
    transparent 55%
  );
  mix-blend-mode: overlay;
  z-index: 1;
}

[data-bs-theme="dark"] .dashboard-card::before {
  background: linear-gradient(
    180deg,
    rgba(255, 255, 255, 0.26) 0%,
    rgba(255, 255, 255, 0.05) 28%,
    transparent 55%
  );
  mix-blend-mode: screen;
}

/* 卡片内部所有内容浮于玻璃反射层之上 */
.dashboard-card > * {
  position: relative;
  z-index: 3;
}

@media (hover: hover) and (pointer: fine) {
  .dashboard-card:hover {
    transform: translateY(-4px) scale(1.008);
    box-shadow:
      0 20px 42px -6px rgba(15, 23, 42, 0.12),
      0 4px 14px rgba(0, 0, 0, 0.04),
      inset 0 1.5px 0.5px rgba(255, 255, 255, 0.95),
      inset 0 -1.5px 1px rgba(0, 0, 0, 0.06),
      inset 0 0 20px rgba(255, 255, 255, 0.4);
    border-color: rgba(255, 255, 255, 0.9);
  }

  [data-bs-theme="dark"] .dashboard-card:hover {
    box-shadow:
      0 24px 50px -8px rgba(0, 0, 0, 0.75),
      0 6px 18px rgba(0, 0, 0, 0.4),
      inset 0 1.5px 0.5px rgba(255, 255, 255, 0.35),
      inset 0 -1.5px 1px rgba(0, 0, 0, 0.5),
      inset 0 0 20px rgba(255, 255, 255, 0.08);
    border-color: rgba(255, 255, 255, 0.3);
  }
}

.dashboard-kpi-card {
  min-height: 140px;
}

.dashboard-content-card {
  min-height: 456px;
}

.dashboard-content-card .card-header {
  min-height: 56px;
  padding: 16px 24px;
  border-bottom: 1px solid var(--border-color);
  background: transparent;
  color: var(--text-primary);
}

/* ===== 3. KPI 指标 ===== */
.kpi-label {
  font-size: 0.75rem;
  font-weight: 600;
  letter-spacing: 0.05em;
  text-transform: uppercase;
  color: var(--text-secondary);
}

.kpi-value {
  font-size: 1.6rem;
  font-weight: 800;
  line-height: 1.25;
  color: var(--text-primary);
  letter-spacing: -0.02em;
}

/* 3D 浮雕透光图标胶囊 (Liquid 3D Icon Capsule) */
.icon-box-sm {
  width: 46px;
  height: 46px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  border-radius: 16px;
  font-size: 1.35rem;
  border: 1px solid var(--border-color);
  box-shadow: inset 0 1.5px 2px rgba(255, 255, 255, 0.4), var(--shadow-sm);
  transition: transform 0.25s ease;
}

[data-bs-theme="dark"] .icon-box-sm {
  box-shadow: inset 0 1.5px 2px rgba(255, 255, 255, 0.15), var(--shadow-sm);
}

.icon-box-xs {
  width: 30px;
  height: 30px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  border-radius: 10px;
  border: 1px solid var(--border-color);
  box-shadow: inset 0 1px 1px rgba(255, 255, 255, 0.3);
}

.icon-box-xs i {
  font-size: 0.875rem;
}

.icon-chart-1 { color: var(--chart-1); background: color-mix(in srgb, var(--chart-1) 16%, var(--bg-surface)); }
.icon-chart-2 { color: var(--chart-2); background: color-mix(in srgb, var(--chart-2) 16%, var(--bg-surface)); }
.icon-chart-5 { color: var(--chart-5); background: color-mix(in srgb, var(--chart-5) 16%, var(--bg-surface)); }
.icon-brand { color: var(--color-brand); background: color-mix(in srgb, var(--color-brand) 16%, var(--bg-surface)); }
.icon-warning { color: var(--color-warning); background: color-mix(in srgb, var(--color-warning) 16%, var(--bg-surface)); }

/* ===== 4. AI Copilot 触发卡 ===== */
.dashboard-ai-action {
  cursor: pointer;
  position: relative;
  transition: transform var(--noyo-duration-standard) var(--noyo-ease-standard), box-shadow var(--noyo-duration-standard) var(--noyo-ease-standard);
}

.dashboard-ai-action:hover {
  transform: translateY(-2px);
  box-shadow: var(--shadow-card-hover), 0 0 20px color-mix(in srgb, var(--color-brand) 20%, transparent);
}

.ai-spark-beacon {
  position: absolute;
  top: -2px;
  right: -2px;
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--color-brand);
  box-shadow: 0 0 8px var(--color-brand);
  animation: beacon-pulse 2.4s ease-in-out infinite;
}

@keyframes beacon-pulse {
  0%, 100% { transform: scale(1); opacity: 0.8; }
  50% { transform: scale(1.35); opacity: 1; }
}

.ai-chat-icon-wrap {
  transition: transform var(--noyo-duration-fast) var(--noyo-ease-standard);
}

.dashboard-ai-action:hover .ai-chat-icon-wrap {
  transform: scale(1.15) rotate(4deg);
}

/* ===== 5. 晶体微发光胶囊 (Glow Pill Badges) ===== */
.dash-pill {
  display: inline-flex;
  align-items: center;
  gap: 0.25rem;
  padding: 0.2rem 0.6rem;
  border-radius: 9999px;
  font-size: 0.72rem;
  font-weight: 600;
  letter-spacing: 0.02em;
  line-height: 1.2;
  backdrop-filter: blur(8px);
  -webkit-backdrop-filter: blur(8px);
}

.dash-pill-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background-color: currentColor;
  box-shadow: 0 0 6px currentColor;
  flex-shrink: 0;
  display: inline-block;
}

.dash-pill--success {
  color: var(--color-success) !important;
  background: color-mix(in srgb, var(--color-success) 12%, transparent);
  border: 1px solid color-mix(in srgb, var(--color-success) 30%, transparent);
}

.dash-pill--danger {
  color: var(--color-danger) !important;
  background: color-mix(in srgb, var(--color-danger) 12%, transparent);
  border: 1px solid color-mix(in srgb, var(--color-danger) 30%, transparent);
}

.dash-pill--primary {
  color: var(--color-brand) !important;
  background: color-mix(in srgb, var(--color-brand) 12%, transparent);
  border: 1px solid color-mix(in srgb, var(--color-brand) 30%, transparent);
}

.dash-pill--brand {
  color: var(--color-brand) !important;
  background: color-mix(in srgb, var(--color-brand) 12%, transparent);
  border: 1px solid color-mix(in srgb, var(--color-brand) 30%, transparent);
}

/* ===== 6. 次级精密微凹槽 (Sub-surface Sunk Wells) ===== */
.dash-metric-well {
  background: var(--bg-surface-secondary, rgba(15, 23, 42, 0.035));
  border: 1px solid var(--border-color);
  border-radius: 14px;
  box-shadow: inset 0 1px 2px rgba(0, 0, 0, 0.03);
  transition: all var(--noyo-duration-fast) var(--noyo-ease-standard);
}

.dash-metric-well:hover {
  background: var(--bg-surface-hover, rgba(15, 23, 42, 0.055));
  transform: translateY(-1px);
}

[data-bs-theme="dark"] .dash-metric-well {
  background: rgba(0, 0, 0, 0.25);
  border: 1px solid var(--border-color);
  box-shadow: inset 0 1.5px 3px rgba(0, 0, 0, 0.45);
}

.metric-value {
  font-size: 1.4rem;
  font-weight: 800;
  line-height: 1.25;
}

.metric-label {
  font-size: 0.75rem;
  font-weight: 600;
  color: var(--text-secondary);
  margin-top: 0.25rem;
}

.empty-state {
  color: var(--text-secondary);
  padding: 1.25rem 0;
}

/* ===== 7. 风险列表 ===== */
.dash-risk-list {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.dash-risk-item {
  border-radius: 8px;
  transition: background var(--noyo-duration-fast) var(--noyo-ease-standard);
}

.dash-risk-item:hover {
  background: color-mix(in srgb, var(--text-primary) 5%, transparent);
}

/* ===== 8. 流体渐变进度条 ===== */
.dash-progress-track {
  width: 100%;
  height: 6px;
  border-radius: 999px;
  background: color-mix(in srgb, var(--text-secondary) 15%, transparent);
  overflow: hidden;
  position: relative;
}

.dash-progress-bar {
  height: 100%;
  border-radius: 999px;
  transition: width var(--noyo-duration-standard) var(--noyo-ease-standard);
  position: relative;
}

.dash-progress-bar--devices {
  background: linear-gradient(90deg, var(--chart-2) 0%, var(--chart-6) 100%);
  box-shadow: 0 0 8px color-mix(in srgb, var(--chart-2) 40%, transparent);
}

.dash-progress-bar--service {
  background: linear-gradient(90deg, var(--chart-5) 0%, var(--chart-1) 100%);
  box-shadow: 0 0 8px color-mix(in srgb, var(--chart-5) 40%, transparent);
}

/* ===== 9. 微光精密刻线 ===== */
.dash-divider {
  width: 100%;
  height: 1px;
  background: linear-gradient(90deg, transparent 0%, var(--border-color) 15%, var(--border-color) 85%, transparent 100%);
}

/* ===== 10. 环形图指针端点与 Sparkline ===== */
.resource-ring {
  transition: stroke-dasharray var(--noyo-duration-standard) var(--noyo-ease-standard);
}

.resource-ring--cpu {
  filter: drop-shadow(0 2px 6px color-mix(in srgb, var(--chart-1) 35%, transparent));
}

.resource-ring--memory {
  filter: drop-shadow(0 2px 6px color-mix(in srgb, var(--chart-5) 35%, transparent));
}

.resource-ring--disk {
  filter: drop-shadow(0 2px 6px color-mix(in srgb, var(--chart-6) 35%, transparent));
}

.resource-cap {
  filter: drop-shadow(0 0 4px currentColor);
  transition: cx var(--noyo-duration-standard) var(--noyo-ease-standard), cy var(--noyo-duration-standard) var(--noyo-ease-standard);
}

.sparkline-section {
  position: relative;
  z-index: 1;
}

.sparkline-svg {
  overflow: visible;
}

.sparkline-path {
  transition: d var(--noyo-duration-standard) var(--noyo-ease-standard);
  filter: drop-shadow(0 2px 6px color-mix(in srgb, var(--chart-1) 40%, transparent));
}

.sparkline-tip-dot {
  filter: drop-shadow(0 0 5px var(--chart-1));
  animation: beacon-pulse 2s ease-in-out infinite;
}

.refresh-spin {
  animation: spin 0.6s linear infinite;
}

@keyframes spin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}

/* ===== 10.1 液态玻璃折射与无障碍 ===== */
.dashboard-refraction-ready .dashboard-kpi-card :deep(.noyo-glass-group__backdrop) {
  filter: url(#noyo-dashboard-kpi-refraction);
  backdrop-filter: blur(0.5px);
}

.dashboard-refraction-ready .dashboard-content-card :deep(.noyo-glass-group__backdrop) {
  filter: url(#noyo-dashboard-panel-refraction);
  backdrop-filter: blur(0.75px);
}

.dashboard-card :deep(.noyo-glass-group__content) {
  filter: none;
  transform: none;
}

.dashboard-card :deep(.noyo-glass-group__backdrop),
.dashboard-card :deep(.noyo-glass-group__tint),
.dashboard-card :deep(.noyo-glass-group__rim) {
  transform: scaleX(calc(1 + var(--noyo-glass-energy) * 0.014)) scaleY(calc(1 - var(--noyo-glass-energy) * 0.008));
}

@media (prefers-reduced-motion: reduce) {
  .resource-ring {
    transition: none;
  }
}

@media (prefers-reduced-transparency: reduce) {
  .dashboard-refraction-ready .dashboard-card :deep(.noyo-glass-group__backdrop) {
    backdrop-filter: none;
  }
}

/* ===== 11. 骨架屏 ===== */
.skeleton {
  background: color-mix(in srgb, var(--text-secondary) 10%, var(--bg-surface));
  border-radius: 8px;
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
  background: linear-gradient(90deg, transparent, color-mix(in srgb, var(--text-secondary) 15%, transparent), transparent);
  animation: skeleton-shimmer 1.2s infinite;
}

@keyframes skeleton-shimmer {
  0% { transform: translateX(-100%); }
  100% { transform: translateX(100%); }
}

/* 响应式断点（§14） */
@media (max-width: 767.98px) {
  .dashboard-container .page-header {
    align-items: stretch !important;
  }

  .dashboard-container :deep(.lg-navbar) {
    justify-content: space-between;
    flex-wrap: wrap;
  }

  .dashboard-kpi-grid,
  .dashboard-content-grid {
    grid-template-columns: 1fr;
  }
}

@media (min-width: 768px) and (max-width: 1199.98px) {
  .dashboard-kpi-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .dashboard-content-grid {
    grid-template-columns: 1fr 1fr;
  }

  .dashboard-content-card--service {
    grid-column: 1 / -1;
  }
}
</style>
