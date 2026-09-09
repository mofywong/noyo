<template>
  <header class="top-header">
    <div class="d-flex align-items-center gap-3 flex-wrap">
      <button class="header-icon-btn d-md-none me-2" @click="$emit('toggleSidebar')">
        <i class="bi bi-list fs-4"></i>
      </button>
    </div>
    <div class="d-flex align-items-center gap-2">
      <!-- MQTT 状态微交互（位于消息中心左侧） -->
      <div class="dropdown me-1" :class="{ show: activeDropdown === 'mqtt' }">
        <button
          class="header-action-btn position-relative d-flex align-items-center justify-content-center"
          :class="mqttActionBtnClass"
          type="button"
          :aria-expanded="activeDropdown === 'mqtt'"
          :title="mqttTooltipText"
          @click="toggleDropdown('mqtt')"
        >
          <i class="bi bi-broadcast-pin mqtt-antenna-icon"></i>
        </button>
        <div
          class="dropdown-menu dropdown-menu-end mqtt-details-dropdown noyo-glass-popover"
          :class="{ show: activeDropdown === 'mqtt' }"
        >
          <div class="mqtt-details-header">
            <div class="mqtt-details-title">
              <span class="mqtt-details-icon" :class="mqttConnected ? 'mqtt-details-icon--online' : 'mqtt-details-icon--offline'">
                <i class="bi bi-broadcast-pin"></i>
              </span>
              <span class="fw-semibold text-body">{{ $t('header_mqtt_status', 'MQTT 消息总线') }}</span>
            </div>
            <span class="dash-pill" :class="mqttConnected ? 'dash-pill--success' : 'dash-pill--neutral'">
              <span class="dash-pill-dot"></span>
              {{ mqttConnected ? $t('status_online', '已连接') : $t('dev_offline', '未连接') }}
            </span>
          </div>
          <div class="mqtt-details-content">
            <!-- 工作模式 -->
            <div v-if="mqttMode" class="mqtt-details-row">
              <span class="mqtt-details-label">{{ $t('header_mqtt_mode', '工作模式') }}</span>
              <span class="mqtt-mode-badge">
                <i class="bi me-1" :class="mqttMode === 'gateway' ? 'bi-hdd-network' : 'bi-cloud'"></i>
                {{ mqttModeLabel }}
              </span>
            </div>
            <!-- 网关标识（如有） -->
            <div v-if="mqttGatewayCode" class="mqtt-details-row">
              <span class="mqtt-details-label">{{ $t('header_mqtt_gateway', '网关标识') }}</span>
              <span class="mqtt-gateway-code">{{ mqttGatewayCode }}</span>
            </div>
            <!-- Broker 地址卡片（支持换行完整展示与一键复制） -->
            <div class="mqtt-broker-card">
              <div class="mqtt-broker-card__header">
                <span class="mqtt-details-label">{{ $t('header_mqtt_broker', 'Broker 地址') }}</span>
                <button
                  v-if="mqttBroker"
                  type="button"
                  class="copy-broker-btn"
                  :title="$t('header_mqtt_copy_broker', '复制地址')"
                  @click="copyBrokerAddress"
                >
                  <i :class="copiedBroker ? 'bi bi-check2 text-success' : 'bi bi-clipboard'"></i>
                  <span>{{ copiedBroker ? $t('header_mqtt_copied', '已复制') : $t('header_mqtt_copy_broker', '复制地址') }}</span>
                </button>
              </div>
              <div class="broker-address-text">
                {{ mqttBroker || '-' }}
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- 告警下拉 -->
      <div class="dropdown me-1" :class="{ show: activeDropdown === 'alarm' }">
        <button
          class="header-action-btn position-relative"
          :class="{ 'is-active': activeDropdown === 'alarm' }"
          type="button"
          aria-expanded="false"
          @click="toggleDropdown('alarm'); clearUnread()"
        >
          <i class="bi bi-bell"></i>
          <span v-if="unreadCount > 0" class="position-absolute top-0 start-100 translate-middle badge rounded-pill bg-danger" style="font-size: 0.6rem; padding: 0.25rem 0.4rem;">
            {{ unreadCount > 99 ? '99+' : unreadCount }}
          </span>
        </button>
        <ul class="dropdown-menu dropdown-menu-end shadow-sm" :class="{ show: activeDropdown === 'alarm' }" style="width: 320px; max-height: 400px; overflow-y: auto;">
          <li><h6 class="dropdown-header">{{ $t('header_latest_alarms', '最新告警') }}</h6></li>
          <li v-if="recentAlarms.length === 0"><span class="dropdown-item text-muted small">{{ $t('header_no_alarms', '暂无新告警') }}</span></li>
          <li v-for="evt in recentAlarms" :key="evt.ts">
            <a class="dropdown-item py-2 border-bottom" href="#" @click.prevent="goToAlarmDetail(evt)">
              <div class="d-flex w-100 justify-content-between">
                <strong class="mb-1 text-truncate" style="max-width: 150px;">{{ getDeviceName(evt.device_code) }}</strong>
                <small class="text-muted">{{ formatTimeAgo(evt.ts) }}</small>
              </div>
              <p class="mb-1 small text-truncate">
                <span class="spec-badge spec-badge--danger me-1">{{ getEventTypeLabel(evt) }}</span>
                {{ getEventName(evt) }}
              </p>
            </a>
          </li>
          <li v-if="recentAlarms.length > 0">
            <a class="dropdown-item text-center small text-primary py-2" href="#" @click.prevent="goToAlarms">{{ $t('header_view_all_alarms', '查看全部告警') }}</a>
          </li>
        </ul>
        
        <!-- Global Toast Container placed right under the message box -->
        <div class="position-absolute top-100 mt-2 p-0 d-flex flex-column gap-2" style="z-index: 1080; width: 340px; right: -10px;">
          <div v-for="toast in activeToasts" :key="toast.id" class="toast show align-items-start border-0 shadow-lg alarm-toast-item" role="alert" aria-live="assertive" aria-atomic="true">
            <div class="d-flex w-100">
              <div class="toast-body flex-grow-1 text-start py-3">
                <div class="fw-bold d-flex align-items-center mb-1 alarm-toast-title" style="font-size: 0.95rem;">
                  <i class="bi bi-exclamation-circle-fill me-2 fs-5"></i>
                  <span>{{ toast.title }}</span>
                </div>
                <div class="small lh-base alarm-toast-message">{{ toast.message }}</div>
              </div>
              <button type="button" class="btn-close me-2 mt-3" @click="closeToast(toast.id)"></button>
            </div>
          </div>
        </div>
      </div>

      <!-- 主题下拉 -->
      <div class="dropdown" :class="{ show: activeDropdown === 'theme' }">
        <button
          class="header-action-btn d-flex align-items-center justify-content-center"
          :class="{ 'is-active': activeDropdown === 'theme' }"
          type="button"
          aria-expanded="false"
          :title="$t('theme_toggle', '主题切换')"
          @click="toggleDropdown('theme')"
        >
          <i class="bi bi-circle-half"></i>
        </button>
        <ul class="dropdown-menu dropdown-menu-end shadow-sm" :class="{ show: activeDropdown === 'theme' }">
          <li><button class="dropdown-item d-flex align-items-center gap-2" @click="$emit('setTheme', 'light'); activeDropdown = ''">
            <i class="bi bi-sun"></i> <span>{{ $t('theme_light') }}</span>
          </button></li>
          <li><button class="dropdown-item d-flex align-items-center gap-2" @click="$emit('setTheme', 'dark'); activeDropdown = ''">
            <i class="bi bi-moon"></i> <span>{{ $t('theme_dark') }}</span>
          </button></li>
          <li><button class="dropdown-item d-flex align-items-center gap-2" @click="$emit('setTheme', 'system'); activeDropdown = ''">
            <i class="bi bi-circle-half"></i> <span>{{ $t('theme_system') }}</span>
          </button></li>
        </ul>
      </div>

      <!-- 玻璃质感调节下拉（水滴图标 + 融入式状态栏按钮） -->
      <div class="dropdown" :class="{ show: activeDropdown === 'liquid-glass' }">
        <button
          id="liquid-glass-density-trigger"
          ref="liquidGlassDensityTriggerRef"
          class="header-action-btn d-flex align-items-center justify-content-center"
          :class="{ 'is-active': activeDropdown === 'liquid-glass' }"
          type="button"
          data-liquid-glass-density-trigger
          :title="liquidGlassCopy.title"
          :aria-label="liquidGlassCopy.title"
          aria-controls="liquid-glass-density-menu"
          :aria-expanded="activeDropdown === 'liquid-glass'"
          @click="toggleDropdown('liquid-glass')"
        >
          <i class="bi bi-droplet-half"></i>
        </button>
      </div>
      <Teleport to="body">
        <div
          v-show="activeDropdown === 'liquid-glass'"
          id="liquid-glass-density-menu"
          ref="liquidGlassDensityMenuRef"
          class="dropdown-menu dropdown-menu-end shadow-sm liquid-glass-density-menu"
          :class="{ show: activeDropdown === 'liquid-glass' }"
          :style="liquidGlassDensityMenuStyle"
          role="group"
          :aria-label="liquidGlassCopy.title"
          @click.stop
        >
          <div class="d-flex align-items-center justify-content-between gap-3 mb-2">
            <div class="d-flex align-items-center gap-2">
              <i class="bi bi-droplet-half text-primary fs-6"></i>
              <span class="fw-semibold">{{ liquidGlassCopy.title }}</span>
            </div>
            <span class="liquid-glass-density-value font-monospace fw-bold">{{ liquidGlassDensity }}%</span>
          </div>
          <input
            class="form-range liquid-glass-density-range mb-2"
            type="range"
            min="0"
            max="100"
            step="1"
            data-liquid-glass-density-range
            :value="liquidGlassDensity"
            :aria-label="liquidGlassCopy.sliderLabel"
            aria-valuemin="0"
            aria-valuemax="100"
            :aria-valuenow="liquidGlassDensity"
            @input="$emit('setLiquidGlassDensity', Number($event.target.value))"
          >
          <div class="d-flex align-items-center justify-content-between liquid-glass-density-ends mb-3">
            <span>{{ liquidGlassCopy.clear }}</span>
            <span>{{ liquidGlassCopy.contrast }}</span>
          </div>
          <div class="pt-2 border-top d-flex justify-content-between align-items-center">
            <span class="text-secondary small">{{ liquidGlassCopy.defaultHint }}</span>
            <button
              type="button"
              class="btn btn-sm btn-outline-secondary d-inline-flex align-items-center gap-1 density-reset-btn"
              :disabled="liquidGlassDensity === 50"
              @click="$emit('setLiquidGlassDensity', 50)"
            >
              <i class="bi bi-arrow-counterclockwise"></i>
              <span>{{ liquidGlassCopy.reset }}</span>
            </button>
          </div>
        </div>
      </Teleport>

      <!-- 自定义背景下拉 -->
      <div class="dropdown" :class="{ show: activeDropdown === 'bg' }">
        <button
          class="header-action-btn d-flex align-items-center justify-content-center"
          :class="{ 'is-active': activeDropdown === 'bg' }"
          type="button"
          :title="$t('bg_settings')"
          aria-expanded="false"
          @click="toggleDropdown('bg')"
        >
          <i class="bi bi-image"></i>
        </button>
        <ul class="dropdown-menu dropdown-menu-end shadow-sm p-3" :class="{ show: activeDropdown === 'bg' }" style="min-width: 220px;">
          <li><h6 class="dropdown-header px-0 mb-2">{{ $t('bg_settings') }}</h6></li>
          <li class="mb-2">
            <label class="btn btn-sm btn-primary w-100 d-flex align-items-center justify-content-center gap-2 mb-0">
              <i class="bi bi-upload"></i> <span>{{ $t('upload_bg_image') }}</span>
              <input type="file" accept="image/*" class="d-none" @change="handleBgUpload" />
            </label>
          </li>
          <li v-if="customBg">
            <button class="btn btn-sm btn-outline-danger w-100 d-flex align-items-center justify-content-center gap-2" @click="removeCustomBg">
              <i class="bi bi-arrow-counterclockwise"></i> <span>{{ $t('restore_default_bg') }}</span>
            </button>
          </li>
        </ul>
      </div>

      <!-- 语言下拉 -->
      <div class="dropdown" :class="{ show: activeDropdown === 'lang' }">
        <button
          class="header-action-btn header-lang-btn d-flex align-items-center gap-1.5"
          :class="{ 'is-active': activeDropdown === 'lang' }"
          type="button"
          aria-expanded="false"
          @click="toggleDropdown('lang')"
        >
          <i class="bi bi-translate"></i>
          <span class="lang-text">{{ currentLangName }}</span>
        </button>
        <ul class="dropdown-menu dropdown-menu-end shadow-sm" :class="{ show: activeDropdown === 'lang' }">
          <li><button class="dropdown-item d-flex align-items-center gap-2" @click="$emit('setLanguage', 'en'); activeDropdown = ''">
            <span>{{ languageEnglish }}</span>
          </button></li>
          <li><button class="dropdown-item d-flex align-items-center gap-2" @click="$emit('setLanguage', 'zh'); activeDropdown = ''">
            <span>{{ languageChinese }}</span>
          </button></li>
        </ul>
      </div>

      <!-- 用户下拉 -->
      <div class="dropdown" :class="{ show: activeDropdown === 'user' }">
        <a
          href="#"
          class="header-user-btn d-flex align-items-center text-decoration-none dropdown-toggle text-body"
          :class="{ 'is-active': activeDropdown === 'user' }"
          @click.prevent="toggleDropdown('user')"
        >
          <div class="user-avatar-circle">
            <i class="bi bi-person-fill text-secondary"></i>
          </div>
          <span class="d-none d-md-block user-display-name ms-1.5 me-1">{{ authStore.user?.display_name || authStore.user?.username }}</span>
        </a>
        <ul class="dropdown-menu dropdown-menu-end shadow-sm border-0" :class="{ show: activeDropdown === 'user' }">
          <li><a class="dropdown-item" href="#" @click.prevent="openProfileModal">
            <i class="bi bi-person me-2"></i>{{ $t('header_profile', '个人资料') }}
          </a></li>
          <li><hr class="dropdown-divider"></li>
          <li><a class="dropdown-item text-danger" href="#" @click.prevent="handleLogout">
            <i class="bi bi-box-arrow-right me-2"></i>{{ $t('header_logout', '退出登录') }}
          </a></li>
        </ul>
      </div>
    </div>

    <!-- 摄像机实时视频播放悬浮框 -->
    <div v-if="floatingVideoDevice" 
         class="position-fixed shadow-lg border rounded overflow-hidden" 
         style="bottom: 20px; left: 20px; width: 480px; height: 320px; z-index: 1080; border-color: rgba(220,53,69,0.5) !important;">
      <div class="bg-danger text-white px-2 py-1 small d-flex justify-content-between align-items-center">
        <span><i class="bi bi-exclamation-triangle-fill me-1"></i> {{ $t('header_alarm_video', '告警联动视频') }}</span>
        <button type="button" class="btn-close btn-close-white" style="font-size: 0.6rem;" @click="floatingVideoDevice = null"></button>
      </div>
      <div style="height: calc(100% - 28px);">
        <component
          v-if="alarmVideoWidget"
          :is="alarmVideoWidget.component"
          :device="floatingVideoDevice" 
          :embedded="true"
          @close="floatingVideoDevice = null" 
        />
      </div>
    </div>

    <!-- 个人资料弹框 -->
    <div class="modal fade" id="profileModal" tabindex="-1" ref="profileModalRef">
      <div class="modal-dialog">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title">{{ $t('header_profile', '个人资料') }}</h5>
            <button type="button" class="btn-close" data-bs-dismiss="modal" aria-label="Close"></button>
          </div>
          <div class="modal-body">
            <div class="mb-2"><strong>{{ $t('user_username', '用户名') }}:</strong> {{ authStore.user?.username }}</div>
            <div class="mb-2"><strong>{{ getDisplayNameLabel(authStore.user) }}:</strong> {{ authStore.user?.display_name || '-' }}</div>
            <div class="mb-2"><strong>{{ $t('user_role', '角色') }}:</strong> {{ userRoleDisplay || '-' }}</div>
            <div class="mb-2"><strong>{{ $t('user_email', '邮箱') }}:</strong> {{ authStore.user?.email || '-' }}</div>
            <div class="mb-0"><strong>{{ $t('user_phone', '电话') }}:</strong> {{ authStore.user?.phone || '-' }}</div>
          </div>
          <div class="modal-footer">
            <button type="button" class="btn btn-outline-danger" data-bs-dismiss="modal">{{ $t('close', '关闭') }}</button>
          </div>
        </div>
      </div>
    </div>
  </header>
</template>

<script setup>
import { computed, nextTick, ref, onMounted, onUnmounted } from 'vue';
import { useI18n } from 'vue-i18n';
import { useRouter } from 'vue-router';
import { Modal } from 'bootstrap';
import axios from 'axios';
import { gatewayText } from '../utils/gatewayLocale';
import { useAuthStore } from '../stores/auth.js';
import { usePlugins } from '../plugins/registry.js';
import { isSingleProjectMode, SYSTEM_MODES, systemModeLabel } from '../utils/systemMode.js';
import {
  findAlarmVideoDevice,
  getAlarmEventName,
  getAlarmEventTypeLabel,
  getAlarmToastMessage,
  getAlarmEventKey,
  mergeRecentAlarmEvents,
  isAlarmEvent
} from '../utils/alarmEvents.js';
import { formatNamedReference } from '../utils/entityDisplay.js';
import { formatDateTime } from '../utils/dateTime.js';

const props = defineProps({
  title: String,
  currentTheme: String,
  mqttStatus: Object,
  liquidGlassDensity: {
    type: Number,
    default: 58
  }
});

defineEmits(['toggleSidebar', 'setTheme', 'setLanguage', 'setLiquidGlassDensity']);

const { t, locale } = useI18n();
const authStore = useAuthStore();
const router = useRouter();
const { extensions } = usePlugins();
const alarmVideoWidget = computed(() => (extensions.value.alarmVideoWidgets || [])[0] || null);

const mqttConnected = computed(() => Boolean(props.mqttStatus && props.mqttStatus.connected));
const mqttBroker = computed(() => (props.mqttStatus && props.mqttStatus.broker) || '');
const mqttMode = computed(() => (props.mqttStatus && props.mqttStatus.mode) || '');
const mqttGatewayCode = computed(() => (props.mqttStatus && props.mqttStatus.gatewayCode) || '');

const mqttActionBtnClass = computed(() => ({
  'is-active': activeDropdown.value === 'mqtt',
  'header-action-btn--mqtt-online': mqttConnected.value,
  'header-action-btn--mqtt-connecting': props.mqttStatus && (props.mqttStatus.status === 'connecting' || props.mqttStatus.status === 'initializing'),
  'header-action-btn--mqtt-offline': !mqttConnected.value
}));

const mqttModeLabel = computed(() => {
  const mode = String(mqttMode.value || '').trim().toLowerCase();
  if (mode === 'gateway') {
    return t('header_mqtt_mode_gateway', '网关模式');
  }
  if (mode === 'platform') {
    return t('header_mqtt_mode_platform', '平台模式');
  }
  return mode || '-';
});

const copiedBroker = ref(false);
let copyBrokerTimer = null;
const copyBrokerAddress = async () => {
  if (!mqttBroker.value) return;
  try {
    await navigator.clipboard.writeText(mqttBroker.value);
    copiedBroker.value = true;
    if (copyBrokerTimer) clearTimeout(copyBrokerTimer);
    copyBrokerTimer = setTimeout(() => {
      copiedBroker.value = false;
    }, 2000);
  } catch (err) {
    const textarea = document.createElement('textarea');
    textarea.value = mqttBroker.value;
    document.body.appendChild(textarea);
    textarea.select();
    document.execCommand('copy');
    document.body.removeChild(textarea);
    copiedBroker.value = true;
    if (copyBrokerTimer) clearTimeout(copyBrokerTimer);
    copyBrokerTimer = setTimeout(() => {
      copiedBroker.value = false;
    }, 2000);
  }
};

const mqttTooltipText = computed(() => {
  const title = t('header_mqtt_status', 'MQTT 消息总线');
  const status = mqttConnected.value ? t('status_online', '已连接') : t('dev_offline', '未连接');
  const mode = mqttModeLabel.value ? ` [${mqttModeLabel.value}]` : '';
  const broker = mqttBroker.value ? ` (${mqttBroker.value})` : '';
  return `${title}: ${status}${mode}${broker}`;
});

const systemModeName = computed(() => systemModeLabel(authStore.systemMode));
const systemModeIcon = computed(() => isSingleProjectMode(authStore.systemMode) ? 'bi-hdd-network' : 'bi-cloud');
const systemModeBadgeClass = computed(() => ({
  'bg-primary text-white': authStore.systemMode === SYSTEM_MODES.MULTI_TENANT_PLATFORM,
  'bg-secondary text-white': authStore.systemMode === SYSTEM_MODES.MULTI_PROJECT_PLATFORM,
  'bg-success text-white': authStore.systemMode === SYSTEM_MODES.PLATFORM_GATEWAY,
  'bg-info text-dark': authStore.systemMode === SYSTEM_MODES.LOCAL_PROJECT,
}));

const languageEnglish = computed(() => gatewayText(locale.value, 'language_english'));
const languageChinese = computed(() => gatewayText(locale.value, 'language_chinese'));

const currentLangName = computed(() => {
  return locale.value === 'zh' ? languageChinese.value : languageEnglish.value;
});

const liquidGlassCopy = computed(() => locale.value === 'zh'
  ? {
      title: '玻璃质感',
      sliderLabel: '玻璃质感与通透度调节',
      clear: '通透流光',
      contrast: '凝霜对比',
      reset: '恢复默认',
      defaultHint: '标准默认值 50%'
    }
  : {
      title: 'Glass Clarity',
      sliderLabel: 'Adjust glass clarity and texture',
      clear: 'Ultra Clear',
      contrast: 'Frosted Glass',
      reset: 'Reset Default',
      defaultHint: 'Default 50%'
    });

const recentEvents = ref([]);
const unreadCount = ref(0);
const devices = ref({});
const products = ref({});
const floatingVideoDevice = ref(null);
const pendingVideoAlarmEvents = [];
let lastSeenTs = parseInt(localStorage.getItem('noyo_alarms_last_seen') || '0');
let eventSource = null;

const activeDropdown = ref('');
const liquidGlassDensityTriggerRef = ref(null);
const liquidGlassDensityMenuRef = ref(null);
const liquidGlassDensityMenuStyle = ref({
  position: 'fixed',
  top: '0px',
  left: '0px',
  zIndex: 1080
});
const projectsList = ref([]);
const currentProjectId = ref(
  localStorage.getItem('current_project_id') ? parseInt(localStorage.getItem('current_project_id')) : ''
);

const customBg = ref(localStorage.getItem('noyo_custom_bg') || '');

const handleBgUpload = (event) => {
  const file = event.target.files && event.target.files[0];
  if (!file) return;
  if (file.size > 12 * 1024 * 1024) {
    alert(locale.value === 'zh' ? '图片文件过大，请选择 12MB 以内的图片' : 'Image file is too large, please select an image under 12MB');
    return;
  }
  const reader = new FileReader();
  reader.onload = (e) => {
    const dataUrl = e.target.result;
    customBg.value = dataUrl;
    try {
      localStorage.setItem('noyo_custom_bg', dataUrl);
    } catch (err) {
      console.warn('localStorage size limit exceeded for background image', err);
    }
    window.dispatchEvent(new CustomEvent('noyo-bg-changed', { detail: dataUrl }));
    activeDropdown.value = '';
  };
  reader.readAsDataURL(file);
};

const removeCustomBg = () => {
  customBg.value = '';
  localStorage.removeItem('noyo_custom_bg');
  window.dispatchEvent(new CustomEvent('noyo-bg-changed', { detail: '' }));
  activeDropdown.value = '';
};

const positionLiquidGlassDensityMenu = () => {
  if (activeDropdown.value !== 'liquid-glass') return;
  const trigger = liquidGlassDensityTriggerRef.value;
  const menu = liquidGlassDensityMenuRef.value;
  if (!trigger || !menu) return;
  const triggerRect = trigger.getBoundingClientRect();
  const menuWidth = menu.offsetWidth || 280;
  const menuHeight = menu.offsetHeight || 128;
  const viewportGap = 8;
  const left = Math.min(
    Math.max(viewportGap, triggerRect.right - menuWidth),
    Math.max(viewportGap, window.innerWidth - menuWidth - viewportGap)
  );
  const fitsBelow = triggerRect.bottom + viewportGap + menuHeight <= window.innerHeight;
  const top = fitsBelow
    ? triggerRect.bottom + viewportGap
    : Math.max(viewportGap, triggerRect.top - menuHeight - viewportGap);
  liquidGlassDensityMenuStyle.value = {
    position: 'fixed',
    top: `${Math.round(top)}px`,
    left: `${Math.round(left)}px`,
    zIndex: 1080
  };
};

const toggleDropdown = async (name) => {
  const opening = activeDropdown.value !== name;
  activeDropdown.value = opening ? name : '';
  if (opening && name === 'liquid-glass') {
    await nextTick();
    positionLiquidGlassDensityMenu();
  }
};

const closeLiquidGlassDensityMenu = async (restoreFocus = false) => {
  if (activeDropdown.value !== 'liquid-glass') return;
  activeDropdown.value = '';
  if (restoreFocus) {
    await nextTick();
    liquidGlassDensityTriggerRef.value?.focus();
  }
};

const handleDropdownKeydown = (event) => {
  if (event.key !== 'Escape' || activeDropdown.value !== 'liquid-glass') return;
  event.preventDefault();
  closeLiquidGlassDensityMenu(true);
};

const closeAllDropdowns = (event) => {
  if (event?.target && liquidGlassDensityMenuRef.value?.contains(event.target)) {
    return;
  }
  if (event && event.target && event.target.closest('.dropdown')) {
    return;
  }
  activeDropdown.value = '';
};

const loadProjects = async () => {
  try {
    const res = await axios.get('/api/auth/projects');
    if (res.data.code === 0) {
      projectsList.value = res.data.data || [];
    }
  } catch (e) {
    console.error('Failed to load projects:', e);
  }
};

const handleProjectChange = async () => {
  if (currentProjectId.value !== '') {
    localStorage.setItem('current_project_id', currentProjectId.value.toString());
  } else {
    localStorage.removeItem('current_project_id');
  }
  await authStore.refreshProfile();
  window.location.reload();
};

// 从 TSDB list 中过滤出真正的场景告警
const recentAlarms = computed(() => {
  return recentEvents.value.filter(isAlarmEvent);
});

const getDeviceName = (code) => {
  if (!code) return '-';
  return formatNamedReference(devices.value[code]?.name, code);
};

const getEventDef = (evt) => {
  if (!evt.device_code || !evt.event_id) return null;
  const dev = devices.value[evt.device_code];
  if (!dev) return null;
  const prod = products.value[dev.product_code];
  if (!prod || !prod.model || !prod.model.events) return null;
  
  return prod.model.events.find(e => e.key === evt.event_id);
};

const isAlarmVideoDevice = (device) => Boolean(alarmVideoWidget.value?.condition?.(device));

const openAlarmVideoIfReady = (evt) => {
  const device = findAlarmVideoDevice([evt], devices.value, isAlarmVideoDevice);
  if (!device) return false;
  floatingVideoDevice.value = device;
  return true;
};

const queueAlarmVideoOpen = (evt) => {
  if (openAlarmVideoIfReady(evt)) return;
  pendingVideoAlarmEvents.push(evt);
  if (pendingVideoAlarmEvents.length > 20) {
    pendingVideoAlarmEvents.shift();
  }
};

const flushPendingVideoAlarmOpen = () => {
  const device = findAlarmVideoDevice(pendingVideoAlarmEvents, devices.value, isAlarmVideoDevice);
  if (!device) return;
  floatingVideoDevice.value = device;
  pendingVideoAlarmEvents.length = 0;
};

const getEventName = (evt) => {
  return getAlarmEventName(evt, getEventDef(evt), locale.value);
};

const getEventTypeLabel = (evt) => {
  return getAlarmEventTypeLabel(evt, getEventDef(evt), locale.value);
};

const activeToasts = ref([]);
let toastIdCounter = 0;
const toastShownForTs = new Set();

const showToast = (title, message) => {
  const id = toastIdCounter++;
  const toast = { id, title, message };
  activeToasts.value.push(toast);
  setTimeout(() => {
    closeToast(id);
  }, 5000);
};

const closeToast = (id) => {
  activeToasts.value = activeToasts.value.filter(t => t.id !== id);
};

const formatTimeAgo = (ts) => formatDateTime(ts);

let eventsFetchPending = false;
let eventsRefreshTimer = null;
let eventStreamStopped = false;

const fetchRecentEvents = async () => {
  if (eventsFetchPending || eventStreamStopped || !localStorage.getItem('access_token') || !authStore.hasPermission('device:list')) return;
  eventsFetchPending = true;
  try {
    const res = await axios.post('/api/history/query', {
      device_code: "",
      type: 2, // Event
      start_time: 0,
      end_time: 0,
      page: 1,
      page_size: 50  // 多拉一些，过滤后取告警
    });
    if (res.data.code === 0 && res.data.data) {
      if (eventStreamStopped) return;
      const list = mergeRecentAlarmEvents(recentEvents.value, res.data.data.list || []);
      recentEvents.value = list;
      
      // 只统计告警事件的未读数
      let newCount = 0;
      for (const evt of list) {
        const isAlarm = isAlarmEvent(evt);
        if (isAlarm && evt.ts > lastSeenTs) {
          newCount++;
          
          const eventKey = getAlarmEventKey(evt);
          if (!toastShownForTs.has(eventKey)) {
            toastShownForTs.add(eventKey);
            // 只有最近30秒内发生的新告警才弹窗，避免初次加载时弹出一堆历史告警
            if (Date.now() - evt.ts < 30000) {
              const alarmName = getEventName(evt);
              const deviceName = getDeviceName(evt.device_code);
              showToast(alarmName, getAlarmToastMessage(evt, deviceName, locale.value));
              
              queueAlarmVideoOpen(evt);
            }
          }
        }
      }
      unreadCount.value = newCount;
      
      // 防止内存泄漏，保留最近100个记录
      if (toastShownForTs.size > 100) {
        const toDelete = Array.from(toastShownForTs).slice(0, 50);
        toDelete.forEach(ts => toastShownForTs.delete(ts));
      }
    }
  } catch (e) {
    // ignore
  } finally {
    eventsFetchPending = false;
  }
};

const setupEventStream = () => {
  const token = localStorage.getItem('access_token');
  if (eventSource || eventStreamStopped || !token || !authStore.hasPermission('device:list')) return;
  eventSource = new EventSource('/api/devices/stream?token=' + encodeURIComponent(token));
  eventSource.addEventListener('open', fetchRecentEvents);
  
  eventSource.addEventListener('event.reported', (e) => {
    try {
      const data = JSON.parse(e.data);
      const evt = {
        device_code: data.Topic,
        event_id: data.Payload.eventId,
        params: data.Payload.params,
        ts: data.Timestamp
      };
      
      const isAlarm = isAlarmEvent(evt);
      if (isAlarm) {
        recentEvents.value = mergeRecentAlarmEvents(recentEvents.value, [evt]);
        unreadCount.value = recentEvents.value.filter(item => item.ts > lastSeenTs).length;
        
        const eventKey = getAlarmEventKey(evt);
        if (!toastShownForTs.has(eventKey)) {
          toastShownForTs.add(eventKey);
          const alarmName = getEventName(evt);
          const deviceName = getDeviceName(evt.device_code);
          showToast(alarmName, getAlarmToastMessage(evt, deviceName, locale.value));
          
          queueAlarmVideoOpen(evt);
        }
        
        if (toastShownForTs.size > 100) {
          const toDelete = Array.from(toastShownForTs).slice(0, 50);
          toDelete.forEach(ts => toastShownForTs.delete(ts));
        }
      }
    } catch (err) {
      console.error('Failed to parse SSE event:', err);
    }
  });

  eventSource.onerror = () => {
    if (!eventStreamStopped && eventSource?.readyState === EventSource.CLOSED) {
      setTimeout(() => {
        eventSource = null;
        setupEventStream();
      }, 3000);
    }
  };
};

const fetchDataMetadata = async () => {
  try {
    const [devRes, prodRes] = await Promise.all([
      axios.get('/api/devices'),
      axios.get('/api/products')
    ]);
    
    if (devRes.data.code === 0 && devRes.data.data) {
      const devMap = {};
      devRes.data.data.forEach(d => {
        devMap[d.code] = d;
      });
      devices.value = devMap;
      flushPendingVideoAlarmOpen();
    }
    
    if (prodRes.data.code === 0 && prodRes.data.data) {
      const prodMap = {};
      prodRes.data.data.forEach(p => {
        if (typeof p.model === 'string') {
          try { p.model = JSON.parse(p.model); } catch (e) { p.model = {}; }
        }
        prodMap[p.code] = p;
      });
      products.value = prodMap;
    }
  } catch (e) {
    console.error('Failed to load metadata in TopHeader', e);
  }
};

const clearUnread = () => {
  unreadCount.value = 0;
  if (recentEvents.value.length > 0) {
    lastSeenTs = recentEvents.value[0].ts;
    localStorage.setItem('noyo_alarms_last_seen', lastSeenTs.toString());
  }
};

const goToAlarms = () => {
  clearUnread();
  activeDropdown.value = '';
  router.push('/alarms');
};

const goToAlarmDetail = (evt) => {
  clearUnread();
  activeDropdown.value = '';
  // 携带告警时间戳参数，告警中心页面会据此自动打开详情弹框
  // 加入 _t 随机参数确保即使已在告警页面也能触发 watch 变化
  router.push({ path: '/alarms', query: { highlight: evt.ts, _t: Date.now() } });
};

const handleLogout = async () => {
  try {
    await axios.post('/api/auth/logout');
  } catch (e) {
    // Ignore error
  }
  authStore.logout();
  router.push('/login');
};

const profileModalRef = ref(null);
let profileModal = null;

const openProfileModal = () => {
  activeDropdown.value = '';
  if (profileModal) {
    profileModal.show();
  }
};

onMounted(() => {
  if (profileModalRef.value) {
    profileModal = new Modal(profileModalRef.value);
  }
  setupEventStream();
  eventsRefreshTimer = setInterval(fetchRecentEvents, 5000);
  fetchDataMetadata().then(() => fetchRecentEvents());
  document.addEventListener('click', closeAllDropdowns);
  document.addEventListener('keydown', handleDropdownKeydown);
  window.addEventListener('resize', positionLiquidGlassDensityMenu);
  window.addEventListener('scroll', positionLiquidGlassDensityMenu, true);
  window.addEventListener('project-updated', loadProjects);
  if (authStore.user && authStore.user.tenant_id > 0) {
    loadProjects();
  }
});

const userRoleDisplay = computed(() => {
  if (!authStore.user) return '';
  if (authStore.user.tenant_roles && authStore.user.tenant_roles.length > 0) {
     const isGateway = isSingleProjectMode(authStore.systemMode);
     const roleNames = authStore.user.tenant_roles.map(r => isGateway && (r.role_code === 'tenant_admin' || r.role_code === 'super_admin') ? t('role_super_admin', '超级管理员') : r.role_name);
     return Array.from(new Set(roleNames)).join(', ');
  }
  return authStore.user.role;
});

const getDisplayNameLabel = (user) => {
  if (!user) return t('user_display_name', '姓名');
  if (user.is_system_admin || user.is_tenant_admin || user.is_project_admin) {
    return t('user_admin_name', '管理员姓名');
  }
  return t('user_display_name', '姓名');
};

onUnmounted(() => {
  eventStreamStopped = true;
  clearInterval(eventsRefreshTimer);
  if (eventSource) {
    eventSource.close();
    eventSource = null;
  }
  document.removeEventListener('click', closeAllDropdowns);
  document.removeEventListener('keydown', handleDropdownKeydown);
  window.removeEventListener('resize', positionLiquidGlassDensityMenu);
  window.removeEventListener('scroll', positionLiquidGlassDensityMenu, true);
  window.removeEventListener('project-updated', loadProjects);
});
</script>

<style scoped>
/* 状态栏一体化融合式液态玻璃按钮 */
.header-action-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  height: 32px;
  min-width: 32px;
  padding: 0 9px;
  border-radius: 9999px;
  border: 1px solid var(--border-color, rgba(255, 255, 255, 0.2));
  background: var(--noyo-glass-island-action, rgba(255, 255, 255, 0.08));
  color: var(--text-main);
  font-size: 0.9rem;
  line-height: 1;
  cursor: pointer;
  backdrop-filter: blur(12px);
  -webkit-backdrop-filter: blur(12px);
  transition: all var(--noyo-duration-fast, 0.15s) var(--noyo-ease-standard, ease);
  outline: none;
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.03);
}

.header-action-btn:hover {
  background: var(--noyo-glass-island-action-hover, rgba(255, 255, 255, 0.18));
  border-color: rgba(147, 197, 253, 0.5);
  color: var(--text-main);
  transform: translateY(-1px);
  box-shadow: 0 3px 10px rgba(0, 0, 0, 0.08);
}

.header-action-btn:active,
.header-action-btn.is-active {
  background: rgba(59, 130, 246, 0.15);
  border-color: rgba(59, 130, 246, 0.45);
  color: var(--color-brand);
}

.header-icon-btn {
  background: transparent;
  border: none;
  color: var(--text-main);
  padding: 4px;
  border-radius: 8px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  transition: background 0.15s ease;
}

.header-icon-btn:hover {
  background: rgba(255, 255, 255, 0.1);
}

.header-lang-btn {
  padding: 0 10px;
  gap: 5px;
}

.header-lang-btn .lang-text {
  font-size: 0.78rem;
  font-weight: 500;
}

.header-user-btn {
  display: inline-flex;
  align-items: center;
  height: 34px;
  padding: 2px 10px 2px 3px;
  border-radius: 9999px;
  border: 1px solid var(--border-color, rgba(255, 255, 255, 0.2));
  background: var(--noyo-glass-island-action, rgba(255, 255, 255, 0.08));
  color: var(--text-main);
  font-size: 0.85rem;
  font-weight: 500;
  backdrop-filter: blur(12px);
  -webkit-backdrop-filter: blur(12px);
  transition: all var(--noyo-duration-fast, 0.15s) ease;
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.03);
}

.header-user-btn:hover {
  background: var(--noyo-glass-island-action-hover, rgba(255, 255, 255, 0.18));
  border-color: rgba(147, 197, 253, 0.5);
  transform: translateY(-1px);
  box-shadow: 0 3px 10px rgba(0, 0, 0, 0.08);
}

.user-avatar-circle {
  width: 26px;
  height: 26px;
  border-radius: 50%;
  background: var(--bg-surface);
  border: 1px solid var(--border-color);
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 0.85rem;
  flex-shrink: 0;
}

.user-display-name {
  max-width: 140px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.liquid-glass-density-menu {
  min-width: 300px;
  padding: 1.1rem;
  border-radius: var(--radius-card, 16px);
  background: var(--noyo-dashboard-liquid-tint, var(--bg-surface));
  backdrop-filter: blur(var(--noyo-dashboard-liquid-blur, 10px)) saturate(175%) brightness(var(--noyo-dashboard-liquid-backdrop-brightness, 1.05));
  -webkit-backdrop-filter: blur(var(--noyo-dashboard-liquid-blur, 10px)) saturate(175%) brightness(var(--noyo-dashboard-liquid-backdrop-brightness, 1.05));
  border: 1px solid var(--noyo-dashboard-liquid-edge, var(--border-color));
}

.liquid-glass-density-value {
  color: var(--color-brand);
  font-family: var(--bs-font-monospace);
  font-size: 0.85rem;
  font-variant-numeric: tabular-nums;
}

.liquid-glass-density-range {
  accent-color: var(--color-brand);
  margin-bottom: 0.5rem;
}

.liquid-glass-density-ends {
  color: var(--text-secondary);
  font-size: 0.72rem;
  font-weight: 500;
}

.density-reset-btn {
  font-size: 0.76rem;
  border-radius: 999px;
  padding: 3px 10px;
  background: var(--noyo-glass-island-action, rgba(255, 255, 255, 0.08));
  border: 1px solid var(--border-color);
  color: var(--text-secondary);
  transition: all 0.15s ease;
}

.density-reset-btn:hover:not(:disabled) {
  background: rgba(59, 130, 246, 0.12);
  color: var(--color-brand);
  border-color: rgba(59, 130, 246, 0.3);
}

/* MQTT 按钮高雅翡翠绿在线态（去点化，整体背景与天线发光表示在线） */
.header-action-btn--mqtt-online {
  background: rgba(16, 185, 129, 0.12) !important;
  border-color: rgba(16, 185, 129, 0.35) !important;
  color: #059669 !important;
  transition: all var(--noyo-duration-fast, 0.15s) ease;
}

.header-action-btn--mqtt-online .mqtt-antenna-icon {
  color: #10b981 !important;
  filter: drop-shadow(0 0 4px rgba(16, 185, 129, 0.45));
}

.header-action-btn--mqtt-online:hover,
.header-action-btn--mqtt-online.is-active {
  background: rgba(16, 185, 129, 0.22) !important;
  border-color: rgba(16, 185, 129, 0.55) !important;
  box-shadow: 0 0 12px rgba(16, 185, 129, 0.25) !important;
}

[data-bs-theme="dark"] .header-action-btn--mqtt-online {
  background: rgba(16, 185, 129, 0.18) !important;
  border-color: rgba(52, 211, 153, 0.45) !important;
  color: #34d399 !important;
}

[data-bs-theme="dark"] .header-action-btn--mqtt-online .mqtt-antenna-icon {
  color: #34d399 !important;
  filter: drop-shadow(0 0 6px rgba(52, 211, 153, 0.6));
}

[data-bs-theme="dark"] .header-action-btn--mqtt-online:hover,
[data-bs-theme="dark"] .header-action-btn--mqtt-online.is-active {
  background: rgba(16, 185, 129, 0.28) !important;
  border-color: rgba(52, 211, 153, 0.65) !important;
  box-shadow: 0 0 14px rgba(52, 211, 153, 0.35) !important;
}

/* 连接中呼吸态 */
.header-action-btn--mqtt-connecting .mqtt-antenna-icon {
  color: var(--color-warning, #f59e0b) !important;
  animation: mqttAntennaPulse 1.8s ease-in-out infinite;
}

/* 离线态 */
.header-action-btn--mqtt-offline .mqtt-antenna-icon {
  color: var(--text-muted, #94a3b8) !important;
}

@keyframes mqttAntennaPulse {
  0%, 100% { opacity: 1; transform: scale(1); }
  50% { opacity: 0.55; transform: scale(1.08); }
}

.copy-broker-btn {
	appearance: none;
	display: inline-flex;
	align-items: center;
	gap: var(--space-1, 4px);
	padding: var(--space-1, 4px) var(--space-2, 8px);
	border: 1px solid transparent;
	border-radius: var(--radius-pill, 999px);
	background: transparent;
	font-size: 0.72rem;
	font-weight: 600;
	color: var(--color-brand) !important;
	transition: background-color var(--noyo-duration-fast, 0.15s) ease,
		border-color var(--noyo-duration-fast, 0.15s) ease,
		color var(--noyo-duration-fast, 0.15s) ease;
}

.copy-broker-btn:hover {
	background: var(--bg-surface-secondary);
	border-color: var(--border-color);
}

.broker-address-text {
	padding: var(--space-2, 8px) var(--space-3, 12px);
	border: 1px solid var(--border-color);
	border-radius: var(--radius-control, 8px);
	word-break: break-all;
	font-size: 0.78rem;
	line-height: 1.45;
	font-family: var(--bs-font-monospace);
	color: var(--text-primary);
	background: var(--bg-surface-secondary);
}

.mqtt-details-dropdown {
	width: min(calc(100vw - var(--space-8, 32px)), 360px);
	min-width: 320px;
	max-width: 360px;
	padding: var(--space-4, 16px);
	border: 1px solid var(--noyo-solid-border) !important;
	border-radius: var(--radius-card, 16px);
	background: var(--noyo-solid-surface) !important;
	background-image: none !important;
	opacity: 1;
	box-shadow: var(--noyo-solid-shadow-raised) !important;
	backdrop-filter: none;
	-webkit-backdrop-filter: none;
}

.mqtt-details-header,
.mqtt-details-title,
.mqtt-details-row,
.mqtt-broker-card__header {
	display: flex;
	align-items: center;
}

.mqtt-details-header,
.mqtt-details-row,
.mqtt-broker-card__header {
	justify-content: space-between;
}

.mqtt-details-header {
	gap: var(--space-3, 12px);
	padding-bottom: var(--space-3, 12px);
	border-bottom: 1px solid var(--border-color);
}

.mqtt-details-title {
	min-width: 0;
	gap: var(--space-2, 8px);
}

.mqtt-details-icon {
	display: inline-flex;
	align-items: center;
	justify-content: center;
	flex: 0 0 auto;
	width: 32px;
	height: 32px;
	border-radius: var(--radius-control, 8px);
	background: var(--bg-surface-secondary);
	color: var(--text-secondary);
}

.mqtt-details-icon--online {
	color: var(--color-success);
}

.mqtt-details-content {
	display: flex;
	flex-direction: column;
	gap: var(--space-3, 12px);
	padding-top: var(--space-3, 12px);
}

.mqtt-details-row {
	gap: var(--space-3, 12px);
	min-height: 32px;
}

.mqtt-details-label {
	color: var(--text-secondary);
	font-size: 0.78rem;
	font-weight: 500;
}

.mqtt-mode-badge,
.mqtt-gateway-code {
	display: inline-flex;
	align-items: center;
	max-width: 60%;
	padding: var(--space-1, 4px) var(--space-2, 8px);
	border: 1px solid var(--border-color);
	border-radius: var(--radius-pill, 999px);
	background: var(--bg-surface-secondary);
	color: var(--text-primary);
	font-size: 0.76rem;
	font-weight: 600;
}

.mqtt-gateway-code {
	min-width: 0;
	overflow: hidden;
	text-overflow: ellipsis;
	white-space: nowrap;
	font-family: var(--bs-font-monospace);
}

.mqtt-broker-card {
	display: flex;
	flex-direction: column;
	gap: var(--space-2, 8px);
	padding: var(--space-3, 12px);
	border: 1px solid var(--border-color);
	border-radius: var(--radius-control, 8px);
	background: var(--bg-surface-secondary);
}

@media (max-width: 399px) {
	.mqtt-details-dropdown {
		min-width: 0;
	}
}
</style>

<style>
/* Toast 告警弹窗高对比度样式（非 scoped，确保跨主题生效） */
.alarm-toast-item {
  opacity: 0.98;
  transition: all 0.3s ease;
  background: linear-gradient(135deg, #fff5f5 0%, #ffe8e8 100%) !important;
  border: 1px solid rgba(220, 53, 69, 0.35) !important;
  border-left: 5px solid #dc3545 !important;
  border-radius: 8px;
  box-shadow: 0 8px 32px rgba(220, 53, 69, 0.18), 0 2px 8px rgba(0, 0, 0, 0.12) !important;
  animation: toast-slide-in 0.35s ease-out;
}

.alarm-toast-title {
  color: #991b1b;
}

.alarm-toast-message {
  color: #3f1f1f;
  word-break: break-word;
}

@keyframes toast-slide-in {
  from { opacity: 0; transform: translateX(20px); }
  to { opacity: 0.98; transform: translateX(0); }
}

[data-bs-theme="dark"] .alarm-toast-item {
  background: linear-gradient(135deg, #3a1a1a 0%, #2d1010 100%) !important;
  border-color: rgba(220, 53, 69, 0.5) !important;
  box-shadow: 0 8px 32px rgba(220, 53, 69, 0.25), 0 2px 8px rgba(0, 0, 0, 0.4) !important;
}

[data-bs-theme="dark"] .alarm-toast-title {
  color: #fecaca;
}

[data-bs-theme="dark"] .alarm-toast-message {
  color: #fee2e2;
}
</style>

<style scoped>
:deep(.svg-container svg) {
  max-width: 100%;
  max-height: 100%;
}
</style>
