<template>
  <header class="top-header">
    <div class="d-flex align-items-center">
      <button class="btn btn-link text-body d-md-none me-3" @click="$emit('toggleSidebar')">
        <i class="bi bi-list fs-4"></i>
      </button>
      <div class="header-title">{{ title }}</div>
    </div>
    <div class="d-flex align-items-center gap-3">
      <div class="dropdown me-1">
        <button class="btn btn-sm btn-outline-secondary position-relative border-0" type="button" data-bs-toggle="dropdown" aria-expanded="false" @click="clearUnread">
          <i class="bi bi-bell fs-5"></i>
          <span v-if="unreadCount > 0" class="position-absolute top-0 start-100 translate-middle badge rounded-pill bg-danger" style="font-size: 0.6rem; padding: 0.25rem 0.4rem;">
            {{ unreadCount > 99 ? '99+' : unreadCount }}
          </span>
        </button>
        <ul class="dropdown-menu dropdown-menu-end shadow-sm" style="width: 320px; max-height: 400px; overflow-y: auto;">
          <li><h6 class="dropdown-header">最新告警</h6></li>
          <li v-if="recentAlarms.length === 0"><span class="dropdown-item text-muted small">暂无新告警</span></li>
          <li v-for="evt in recentAlarms" :key="evt.ts">
            <a class="dropdown-item py-2 border-bottom" href="#" @click.prevent="goToAlarms">
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
          <div v-for="toast in activeToasts" :key="toast.id" class="toast show align-items-start border-0 shadow-lg" role="alert" aria-live="assertive" aria-atomic="true" style="opacity: 0.98; transition: all 0.3s ease; background-color: var(--bs-body-bg); border-left: 4px solid var(--bs-danger) !important; border-radius: 6px;">
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

      <div class="dropdown">
        <button class="btn btn-sm btn-outline-secondary d-flex align-items-center gap-2" type="button" data-bs-toggle="dropdown" aria-expanded="false">
          <i class="bi bi-circle-half"></i>
        </button>
        <ul class="dropdown-menu dropdown-menu-end shadow-sm">
          <li><button class="dropdown-item d-flex align-items-center gap-2" @click="$emit('setTheme', 'light')">
            <i class="bi bi-sun"></i> <span>{{ $t('theme_light') }}</span>
          </button></li>
          <li><button class="dropdown-item d-flex align-items-center gap-2" @click="$emit('setTheme', 'dark')">
            <i class="bi bi-moon"></i> <span>{{ $t('theme_dark') }}</span>
          </button></li>
          <li><button class="dropdown-item d-flex align-items-center gap-2" @click="$emit('setTheme', 'system')">
            <i class="bi bi-circle-half"></i> <span>{{ $t('theme_system') }}</span>
          </button></li>
        </ul>
      </div>

      <div class="dropdown">
        <button class="btn btn-sm btn-outline-secondary d-flex align-items-center gap-2" type="button" data-bs-toggle="dropdown" aria-expanded="false">
          <i class="bi bi-translate"></i> <span>{{ currentLangName }}</span>
        </button>
        <ul class="dropdown-menu dropdown-menu-end shadow-sm">
          <li><button class="dropdown-item d-flex align-items-center gap-2" @click="$emit('setLanguage', 'en')">
            <span>{{ languageEnglish }}</span>
          </button></li>
          <li><button class="dropdown-item d-flex align-items-center gap-2" @click="$emit('setLanguage', 'zh')">
            <span>{{ languageChinese }}</span>
          </button></li>
        </ul>
      </div>

      <div class="dropdown">
        <a href="#" class="d-flex align-items-center text-decoration-none dropdown-toggle text-body" data-bs-toggle="dropdown">
          <div class="bg-body rounded-circle d-flex align-items-center justify-content-center border" style="width: 32px; height: 32px;">
            <i class="bi bi-person-fill text-secondary"></i>
          </div>
        </a>
        <ul class="dropdown-menu dropdown-menu-end shadow-sm border-0">
          <li><a class="dropdown-item" href="#">{{ $t('header_profile') }}</a></li>
          <li><hr class="dropdown-divider"></li>
          <li><a class="dropdown-item text-danger" href="#">{{ $t('header_logout') }}</a></li>
        </ul>
      </div>
    </div>
  </header>
</template>

<script setup>
import { computed, ref, onMounted, onUnmounted } from 'vue';
import { useI18n } from 'vue-i18n';
import { useRouter } from 'vue-router';
import axios from 'axios';
import { gatewayText } from '../utils/gatewayLocale';

defineProps({
  title: String,
  currentTheme: String,
  mqttStatus: Object
});

defineEmits(['toggleSidebar', 'setTheme', 'setLanguage']);

const { locale } = useI18n();

const languageEnglish = computed(() => gatewayText(locale.value, 'language_english'));
const languageChinese = computed(() => gatewayText(locale.value, 'language_chinese'));

const currentLangName = computed(() => {
  return locale.value === 'zh' ? languageChinese.value : languageEnglish.value;
});

const router = useRouter();
const recentEvents = ref([]);
const unreadCount = ref(0);
const devices = ref({});
const products = ref({});
let lastSeenTs = parseInt(localStorage.getItem('noyo_alarms_last_seen') || '0');
let pollTimer = null;

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
  router.push('/alarms');
};

onMounted(async () => {
  await fetchDataMetadata();
  fetchRecentEvents();
  pollTimer = setInterval(fetchRecentEvents, 5000);
});

onUnmounted(() => {
  if (pollTimer) clearInterval(pollTimer);
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
