<template>
  <header class="top-header">
    <div class="d-flex align-items-center gap-3 flex-wrap">
      <button class="btn btn-link text-body d-md-none me-2" @click="$emit('toggleSidebar')">
        <i class="bi bi-list fs-4"></i>
      </button>
    </div>
    <div class="d-flex align-items-center gap-3">
      <!-- 告警下拉 -->
      <div class="dropdown me-1" :class="{ show: activeDropdown === 'alarm' }">
        <button class="btn btn-sm btn-outline-secondary position-relative border-0" type="button" aria-expanded="false" @click="toggleDropdown('alarm'); clearUnread()">
          <i class="bi bi-bell fs-5"></i>
          <span v-if="unreadCount > 0" class="position-absolute top-0 start-100 translate-middle badge rounded-pill bg-danger" style="font-size: 0.6rem; padding: 0.25rem 0.4rem;">
            {{ unreadCount > 99 ? '99+' : unreadCount }}
          </span>
        </button>
        <ul class="dropdown-menu dropdown-menu-end shadow-sm" :class="{ show: activeDropdown === 'alarm' }" style="width: 320px; max-height: 400px; overflow-y: auto;">
          <li><h6 class="dropdown-header">最新告警</h6></li>
          <li v-if="recentAlarms.length === 0"><span class="dropdown-item text-muted small">暂无新告警</span></li>
          <li v-for="evt in recentAlarms" :key="evt.ts">
            <a class="dropdown-item py-2 border-bottom" href="#" @click.prevent="goToAlarmDetail(evt)">
              <div class="d-flex w-100 justify-content-between">
                <strong class="mb-1 text-truncate" style="max-width: 150px;">{{ getDeviceName(evt.device_code) }}</strong>
                <small class="text-muted">{{ formatTimeAgo(evt.ts) }}</small>
              </div>
              <p class="mb-1 small text-truncate">
                <span class="badge text-bg-danger me-1">告警</span>
                {{ getEventName(evt) }}
              </p>
            </a>
          </li>
          <li v-if="recentAlarms.length > 0">
            <a class="dropdown-item text-center small text-primary py-2" href="#" @click.prevent="goToAlarms">查看全部告警</a>
          </li>
        </ul>
        
        <!-- Global Toast Container placed right under the message box -->
        <div class="position-absolute top-100 mt-2 p-0 d-flex flex-column gap-2" style="z-index: 1080; width: 340px; right: -10px;">
          <div v-for="toast in activeToasts" :key="toast.id" class="toast show align-items-start border-0 shadow-lg alarm-toast-item" role="alert" aria-live="assertive" aria-atomic="true">
            <div class="d-flex w-100">
              <div class="toast-body flex-grow-1 text-start py-3">
                <div class="fw-bold text-danger d-flex align-items-center mb-1" style="font-size: 0.95rem;">
                  <i class="bi bi-exclamation-circle-fill me-2 fs-5"></i>
                  <span>{{ toast.title }}</span>
                </div>
                <div class="small text-body-secondary lh-base" style="word-break: break-word;">{{ toast.message }}</div>
              </div>
              <button type="button" class="btn-close me-2 mt-3" @click="closeToast(toast.id)"></button>
            </div>
          </div>
        </div>
      </div>

      <div
        v-if="mqttStatus"
        class="mqtt-status-pill"
        :class="mqttStatus.connected ? 'is-connected' : 'is-disconnected'"
        :title="mqttStatus.broker || ''"
      >
        <span class="mqtt-status-dot"></span>
        <span class="mqtt-status-label">MQTT</span>
        <span class="mqtt-status-value">{{ mqttStatus.connected ? 'Connected' : 'Disconnected' }}</span>
      </div>

      <!-- 主题下拉 -->
      <div class="dropdown" :class="{ show: activeDropdown === 'theme' }">
        <button class="btn btn-sm btn-outline-secondary d-flex align-items-center gap-2" type="button" aria-expanded="false" @click="toggleDropdown('theme')">
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

      <!-- 语言下拉 -->
      <div class="dropdown" :class="{ show: activeDropdown === 'lang' }">
        <button class="btn btn-sm btn-outline-secondary d-flex align-items-center gap-2" type="button" aria-expanded="false" @click="toggleDropdown('lang')">
          <i class="bi bi-translate"></i> <span>{{ currentLangName }}</span>
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
        <a href="#" class="d-flex align-items-center text-decoration-none dropdown-toggle text-body" @click.prevent="toggleDropdown('user')">
          <div class="bg-body rounded-circle d-flex align-items-center justify-content-center border me-2" style="width: 32px; height: 32px;">
            <i class="bi bi-person-fill text-secondary"></i>
          </div>
          <span class="d-none d-md-block">{{ authStore.user?.display_name || authStore.user?.username }}</span>
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
        <span><i class="bi bi-exclamation-triangle-fill me-1"></i> 告警联动视频</span>
        <button type="button" class="btn-close btn-close-white" style="font-size: 0.6rem;" @click="floatingVideoDevice = null"></button>
      </div>
      <div style="height: calc(100% - 28px);">
        <GB28181PlayerWidget 
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
            <div class="mb-2"><strong>{{ $t('user_role', '角色') }}:</strong> {{ authStore.user?.role || '-' }}</div>
            <div class="mb-2"><strong>{{ $t('user_email', '邮箱') }}:</strong> {{ authStore.user?.email || '-' }}</div>
            <div class="mb-0"><strong>{{ $t('user_phone', '电话') }}:</strong> {{ authStore.user?.phone || '-' }}</div>
          </div>
          <div class="modal-footer">
            <button type="button" class="btn btn-secondary" data-bs-dismiss="modal">{{ $t('close', '关闭') }}</button>
          </div>
        </div>
      </div>
    </div>
  </header>
</template>

<script setup>
import { computed, ref, onMounted, onUnmounted } from 'vue';
import { useI18n } from 'vue-i18n';
import { useRouter } from 'vue-router';
import { Modal } from 'bootstrap';
import axios from 'axios';
import { gatewayText } from '../utils/gatewayLocale';
import GB28181PlayerWidget from '@/plugins/pro/protocol/gb28181/GB28181PlayerWidget.vue';
import { useAuthStore } from '../stores/auth.js';

defineProps({
  title: String,
  currentTheme: String,
  mqttStatus: Object
});

defineEmits(['toggleSidebar', 'setTheme', 'setLanguage']);

const { t, locale } = useI18n();
const authStore = useAuthStore();
const router = useRouter();

const languageEnglish = computed(() => gatewayText(locale.value, 'language_english'));
const languageChinese = computed(() => gatewayText(locale.value, 'language_chinese'));

const currentLangName = computed(() => {
  return locale.value === 'zh' ? languageChinese.value : languageEnglish.value;
});

const recentEvents = ref([]);
const unreadCount = ref(0);
const devices = ref({});
const products = ref({});
const floatingVideoDevice = ref(null);
let lastSeenTs = parseInt(localStorage.getItem('noyo_alarms_last_seen') || '0');
let pollTimer = null;

const activeDropdown = ref('');
const projectsList = ref([]);
const currentProjectId = ref(
  localStorage.getItem('current_project_id') ? parseInt(localStorage.getItem('current_project_id')) : ''
);

const toggleDropdown = (name) => {
  activeDropdown.value = activeDropdown.value === name ? '' : name;
};

const closeAllDropdowns = (event) => {
  if (event && event.target && event.target.closest('.dropdown')) {
    return;
  }
  activeDropdown.value = '';
};

const loadProjects = async () => {
  try {
    const res = await axios.get('/api/projects');
    if (res.data.code === 0) {
      projectsList.value = res.data.data || [];
    }
  } catch (e) {
    console.error('Failed to load projects:', e);
  }
};

const handleProjectChange = () => {
  if (currentProjectId.value !== '') {
    localStorage.setItem('current_project_id', currentProjectId.value.toString());
  } else {
    localStorage.removeItem('current_project_id');
  }
  window.location.reload();
};

// 场景告警事件ID列表（只有这些才在消息盒子中展示）
const ALARM_EVENT_IDS = [
  'illegal_parking_alarm', 'fire_lane_occupied_alarm',
  'indoor_fire_passage_occupied_alarm', 'object_missing_alarm'
];

// 从 TSDB list 中过滤出真正的场景告警
const recentAlarms = computed(() => {
  return recentEvents.value.filter(evt => {
    // TSDB 返回格式: { event_id, params, ts, device_code, _type }
    return evt.params?.scene_type || ALARM_EVENT_IDS.includes(evt.event_id);
  });
});

const sceneTranslations = {
  illegal_parking: '机动车违法停车',
  indoor_fire_passage_occupied: '室内消防通道占用',
  object_missing: '物品丢失'
};

const getDeviceName = (code) => {
  if (!code) return '-';
  if (devices.value[code] && devices.value[code].name) {
    return devices.value[code].name;
  }
  return code;
};

const getEventDef = (evt) => {
  if (!evt.device_code || !evt.event_id) return null;
  const dev = devices.value[evt.device_code];
  if (!dev) return null;
  const prod = products.value[dev.product_code];
  if (!prod || !prod.model || !prod.model.events) return null;
  
  return prod.model.events.find(e => e.key === evt.event_id);
};

const getEventName = (evt) => {
  const def = getEventDef(evt);
  if (def && def.name) {
    return def.name;
  }
  if (evt.params?.rule_name) {
    return evt.params.rule_name;
  }
  if (evt.params?.scene_type && sceneTranslations[evt.params.scene_type]) {
    return sceneTranslations[evt.params.scene_type];
  }
  return evt.event_id || '-';
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

const formatTimeAgo = (ts) => {
  const diff = Math.floor((Date.now() - ts) / 1000);
  if (diff < 0) return '刚刚';
  if (diff < 60) return `${diff}秒前`;
  if (diff < 3600) return `${Math.floor(diff / 60)}分钟前`;
  if (diff < 86400) return `${Math.floor(diff / 3600)}小时前`;
  return `${Math.floor(diff / 86400)}天前`;
};

const fetchRecentEvents = async () => {
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
      const list = res.data.data.list || [];
      recentEvents.value = list;
      
      // 只统计告警事件的未读数
      let newCount = 0;
      for (const evt of list) {
        const isAlarm = evt.params?.scene_type || ALARM_EVENT_IDS.includes(evt.event_id);
        if (isAlarm && evt.ts > lastSeenTs) {
          newCount++;
          
          if (!toastShownForTs.has(evt.ts)) {
            toastShownForTs.add(evt.ts);
            // 只有最近30秒内发生的新告警才弹窗，避免初次加载时弹出一堆历史告警
            if (Date.now() - evt.ts < 30000) {
              const alarmName = getEventName(evt);
              const deviceName = getDeviceName(evt.device_code);
              showToast(alarmName, `设备: ${deviceName} 发生了告警事件`);
              
              const dev = devices.value[evt.device_code];
              if (dev) {
                const prod = products.value[dev.product_code];
                if (prod && prod.protocol_name === 'gb28181') {
                  floatingVideoDevice.value = dev;
                }
              }
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
  }
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

onMounted(async () => {
  if (profileModalRef.value) {
    profileModal = new Modal(profileModalRef.value);
  }
  await fetchDataMetadata();
  fetchRecentEvents();
  pollTimer = setInterval(fetchRecentEvents, 5000);
  document.addEventListener('click', closeAllDropdowns);
  window.addEventListener('project-updated', loadProjects);
  if (authStore.user && authStore.user.tenant_id > 0) {
    await loadProjects();
  }
});

const getDisplayNameLabel = (user) => {
  if (!user) return t('user_display_name', '姓名');
  const r = user.role;
  if (r === 'admin' || r === 'super_admin' || r === 'tenant_admin' || r === 'project_admin') {
    return t('user_admin_name', '管理员姓名');
  }
  return t('user_display_name', '姓名');
};

onUnmounted(() => {
  if (pollTimer) clearInterval(pollTimer);
  document.removeEventListener('click', closeAllDropdowns);
  window.removeEventListener('project-updated', loadProjects);
});
</script>

<style scoped>
.mqtt-status-pill {
  align-items: center;
  border: 1px solid transparent;
  border-radius: 999px;
  display: inline-flex;
  font-size: 0.75rem;
  font-weight: 700;
  gap: 0.4rem;
  min-height: 2rem;
  padding: 0 0.7rem;
  white-space: nowrap;
}

.mqtt-status-pill.is-connected {
  background: rgba(16, 185, 129, 0.12);
  border-color: rgba(16, 185, 129, 0.24);
  color: #047857;
}

.mqtt-status-pill.is-disconnected {
  background: rgba(239, 68, 68, 0.1);
  border-color: rgba(239, 68, 68, 0.22);
  color: #b91c1c;
}

.mqtt-status-dot {
  border-radius: 50%;
  display: inline-block;
  height: 0.48rem;
  width: 0.48rem;
}

.is-connected .mqtt-status-dot {
  background: #10b981;
  box-shadow: 0 0 0 0.22rem rgba(16, 185, 129, 0.16);
}

.is-disconnected .mqtt-status-dot {
  background: #ef4444;
  box-shadow: 0 0 0 0.22rem rgba(239, 68, 68, 0.14);
}

.mqtt-status-label {
  color: inherit;
}

.mqtt-status-value {
  color: color-mix(in srgb, currentColor 82%, var(--text-secondary));
}

@media (max-width: 768px) {
  .mqtt-status-value {
    display: none;
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

@keyframes toast-slide-in {
  from { opacity: 0; transform: translateX(20px); }
  to { opacity: 0.98; transform: translateX(0); }
}

[data-bs-theme="dark"] .alarm-toast-item {
  background: linear-gradient(135deg, #3a1a1a 0%, #2d1010 100%) !important;
  border-color: rgba(220, 53, 69, 0.5) !important;
  box-shadow: 0 8px 32px rgba(220, 53, 69, 0.25), 0 2px 8px rgba(0, 0, 0, 0.4) !important;
}
</style>

<style scoped>
:deep(.svg-container svg) {
  max-width: 100%;
  max-height: 100%;
}
</style>
