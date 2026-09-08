<template>
  <div
    data-liquid-glass-density-root
    :data-bs-theme="currentTheme === 'system' ? systemTheme : currentTheme"
    :style="liquidGlassRootStyle"
  >
    <template v-if="!isStandalonePage">
      <Sidebar 
        :is-open="sidebarOpen" 
        :current-plugin="currentPluginName"
        :plugins="plugins"
        :loading="loadingPlugins"
        @navigate="handleNavigate"
      />
      
      <div 
        class="main-content" 
        :style="customBgImage ? { backgroundImage: `url(${customBgImage})` } : {}"
      >
        <TopHeader 
          :title="pageTitle" 
          :current-theme="currentTheme"
          :mqtt-status="mqttStatus"
          :liquid-glass-density="liquidGlassDensity"
          @toggle-sidebar="sidebarOpen = !sidebarOpen"
          @set-theme="setTheme"
          @set-language="setLanguage"
          @set-liquid-glass-density="setLiquidGlassDensity"
        />
      
      <div
        class="content-scroll"
      >
        <div class="container-fluid">
          <router-view 
            :plugins="plugins"
            @navigate="handleNavigate"
            @configure="openPluginConfig"
            @update-status="updatePluginStatus"
          />
          <!-- Global Widgets from Plugins -->
          <component 
            v-for="(Widget, index) in pluginExtensions.globalWidgets" 
            :key="'widget-'+index" 
            :is="Widget" 
          />
        </div>
      </div>
      </div>
    </template>
    
    <template v-else>
      <router-view />
    </template>
    
    <Teleport to="body">
      <div v-if="!isStandalonePage && activeHabitRuleSuggestion" class="modal fade show d-block habit-rule-suggestion-modal" tabindex="-1" role="dialog" aria-modal="true">
        <div class="modal-dialog modal-dialog-centered">
          <div class="modal-content shadow">
            <div class="modal-header">
              <h5 class="modal-title">{{ habitRuleText('title') }}</h5>
            </div>
            <div class="modal-body">
              <template v-if="!createdHabitRule">
                <p class="mb-3">{{ activeHabitRuleSuggestion.summary }}</p>
                <div class="border rounded p-3 bg-body-tertiary">
                  <div><strong>{{ habitRuleText('ruleName') }}：</strong>{{ activeHabitRuleDescription.name }}</div>
                  <div class="mt-2"><strong>{{ habitRuleText('trigger') }}：</strong>{{ activeHabitRuleDescription.trigger }}</div>
                  <div class="mt-2"><strong>{{ habitRuleText('action') }}：</strong>{{ activeHabitRuleDescription.action }}</div>
                </div>
                <p class="text-muted small mt-3 mb-0">{{ habitRuleText('confirmationHint') }}</p>
              </template>
              <template v-else>
                <div class="alert alert-success py-2">{{ habitRuleText('created') }}</div>
                <div class="border rounded p-3 bg-body-tertiary">
                  <div><strong>{{ habitRuleText('ruleName') }}：</strong>{{ activeHabitRuleDescription.name }}</div>
                  <div class="mt-2"><strong>{{ habitRuleText('trigger') }}：</strong>{{ activeHabitRuleDescription.trigger }}</div>
                  <div class="mt-2"><strong>{{ habitRuleText('action') }}：</strong>{{ activeHabitRuleDescription.action }}</div>
                  <div class="mt-2"><strong>{{ habitRuleText('status') }}：</strong>{{ createdHabitRule.enabled ? habitRuleText('enabled') : habitRuleText('disabled') }}</div>
                </div>
              </template>
            </div>
            <div class="modal-footer">
              <template v-if="!createdHabitRule">
                <button type="button" class="btn btn-outline-secondary" :disabled="habitRuleActionLoading" @click="dismissHabitRuleSuggestion">{{ habitRuleText('no') }}</button>
                <button type="button" class="btn btn-primary" :disabled="habitRuleActionLoading" @click="confirmHabitRuleSuggestion">
                  <span v-if="habitRuleActionLoading" class="spinner-border spinner-border-sm me-1"></span>{{ habitRuleText('yes') }}
                </button>
              </template>
              <button v-else type="button" class="btn btn-primary" @click="acknowledgeCreatedHabitRule">{{ habitRuleText('acknowledge') }}</button>
            </div>
          </div>
        </div>
      </div>

      <div v-if="!isStandalonePage" class="modal fade" id="forceChangePasswordModal" tabindex="-1" data-bs-backdrop="static" data-bs-keyboard="false">
        <div class="modal-dialog">
          <div class="modal-content">
            <div class="modal-header">
              <h5 class="modal-title">{{ $t('auth_force_change_password', '安全要求：请修改初始密码') }}</h5>
            </div>
            <div class="modal-body">
              <p class="text-danger small">{{ $t('auth_force_change_password_desc', '出于安全考虑，您必须修改初始密码后才能继续使用系统。') }}</p>
              <form @submit.prevent="submitForceChangePassword">
                <div class="mb-3">
                  <label class="form-label">{{ $t('auth_old_password', '旧密码') }}</label>
                  <input v-model="forcePasswordForm.oldPassword" type="password" class="form-control" required>
                </div>
                <div class="mb-3">
                  <label class="form-label">{{ $t('auth_new_password', '新密码') }}</label>
                  <input v-model="forcePasswordForm.newPassword" type="password" class="form-control" required>
                </div>
                <div class="mb-3">
                  <label class="form-label">{{ $t('auth_confirm_new_password', '确认新密码') }}</label>
                  <input v-model="forcePasswordForm.confirmPassword" type="password" class="form-control" required>
                </div>
                <button type="submit" class="btn btn-primary w-100" :disabled="forcePasswordForm.loading">
                  {{ $t('auth_submit_password', '提交修改') }}
                </button>
              </form>
            </div>
          </div>
        </div>
      </div>
    </Teleport>

    <ToastContainer v-if="!isStandalonePage" />
    <ConfirmDialog v-if="!isStandalonePage" />
  </div>
</template>

<script setup>
import { ref, computed, nextTick, onBeforeUnmount, onMounted, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import { useRouter, useRoute } from 'vue-router';
import axios from 'axios';
import Sidebar from './components/Sidebar.vue';
import TopHeader from './components/TopHeader.vue';
import ToastContainer from './components/ToastContainer.vue';
import ConfirmDialog from './components/ConfirmDialog.vue';
import { useToast } from './composables/useToast';
import { gatewayActionText, gatewayText } from './utils/gatewayLocale';
import { isSuccessfulDeviceWriteResponse } from './utils/aiBrainSuggestionReminder';
import { buildHabitRuleCreatePayload, describeHabitRule, isHabitRuleSuggestion } from './utils/aiBrainHabitRule';
import { usePlugins } from './plugins/registry';
import { Modal } from 'bootstrap';
import { useAuthStore } from './stores/auth';
import {
  readLiquidGlassDensity,
  writeLiquidGlassDensity
} from './utils/liquidGlassPreference';

const { t, locale } = useI18n();
const { showToast } = useToast();
const router = useRouter();
const route = useRoute();
const authStore = useAuthStore();
const { extensions: pluginExtensions } = usePlugins();
const isDevelopmentDemoPath = computed(() => (
  import.meta.env.DEV && (
    route.name === 'LiquidGlassDemo' ||
    (!route.name && window.location.pathname === '/design/liquid-glass')
  )
));
const gt = (key, params) => gatewayText(locale.value, key, params);

// State
const sidebarOpen = ref(false); // Mobile sidebar
const plugins = ref([]);
const loadingPlugins = ref(false);
const defaultMqttStatus = () => ({ connected: false, status: 'disconnected', mode: '', broker: '', gatewayCode: '' });
const mqttStatus = ref(defaultMqttStatus());
let mqttStatusTimer = null;

// Theme
const currentTheme = ref(localStorage.getItem('theme') || 'dark');
const systemTheme = ref(window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light');
const liquidGlassDensity = ref(readLiquidGlassDensity(window.localStorage));
const liquidGlassRootStyle = computed(() => ({
  '--noyo-liquid-glass-density': String(liquidGlassDensity.value / 100),
  '--noyo-liquid-glass-density-percent': `${liquidGlassDensity.value}%`,
}));

const resolvedTheme = computed(() => (currentTheme.value === 'system' ? systemTheme.value : currentTheme.value));

watch(resolvedTheme, (theme) => {
  if (typeof document !== 'undefined' && document.documentElement) {
    document.documentElement.setAttribute('data-bs-theme', theme);
    if (document.body) {
      document.body.setAttribute('data-bs-theme', theme);
    }
  }
}, { immediate: true });

watch(liquidGlassDensity, (val) => {
  if (typeof document !== 'undefined' && document.documentElement) {
    document.documentElement.style.setProperty('--noyo-liquid-glass-density', String(val / 100));
    document.documentElement.style.setProperty('--noyo-liquid-glass-density-percent', `${val}%`);
  }
}, { immediate: true });

const licenseData = ref(null);
const forcePasswordForm = ref({ oldPassword: '', newPassword: '', confirmPassword: '', loading: false });
let forcePasswordModal = null;

const copyToClipboard = async (text) => {
  try {
    await navigator.clipboard.writeText(text);
    showToast('success', t('copied_to_clipboard', '已复制到剪贴板'));
  } catch (err) {
    showToast('danger', t('copy_failed', '复制失败'));
  }
};

// Computed
const currentPluginName = computed(() => route.params.name);
const isStandalonePage = computed(() => (
  isDevelopmentDemoPath.value ||
  route.name === 'Login' ||
  route.name === 'Setup' ||
  route.meta?.standalone === true
));
const shouldLoadShellData = computed(() => authStore.isLoggedIn && !isStandalonePage.value);
const activeMqttPlugin = computed(() => {
  const mqttPluginNames = ['cascade', 'platform-cascade', 'mqtt_api', 'aiot', 'sagoo'];
  return plugins.value.find((plugin) => {
    const name = String(plugin?.name || plugin?.Name || '').trim().toLowerCase();
    const status = String(plugin?.status || plugin?.Status || '').trim().toLowerCase();
    const isRunning = plugin?.enabled === true || plugin?.Enabled === true || status === 'running' || status === 'enabled' || status === '1';
    return isRunning && mqttPluginNames.some((target) => name === target || name.includes(target));
  }) || null;
});
const isMqttPluginEnabled = computed(() => Boolean(activeMqttPlugin.value));
const cascadePluginEnabled = computed(() => {
  const name = String(activeMqttPlugin.value?.name || activeMqttPlugin.value?.Name || '').trim().toLowerCase();
  return name.includes('cascade');
});

const pageTitle = computed(() => {
  const name = route.name;
  if (name === 'Dashboard') return t('page_dashboard');
  if (name === 'Marketplace') return t('page_marketplace');
  if (name === 'Products') return t('sidebar_products');
  if (name === 'Devices') return t('sidebar_devices');
  if (name === 'DeviceTags') return t('sidebar_device_tags');
  if (name === 'GatewayManagement') return gt('gateway_management');
  if (name === 'GatewayPlugins') return `${route.params.gwSn} / ${gt('gateway_plugin_marketplace_title')}`;
  if (name === 'GatewayPluginConfig') return `${route.params.gwSn} / ${gt('gateway_plugin_config_title')}`;
  if (name === 'VideoSquare') return t('sidebar_video_square', '视频广场');
  if (name === 'PluginConfig') return `${currentPluginName.value} ${t('page_configure')}`;
  if (name === 'Settings') return t('sidebar_settings');
  if (name === 'License') return t('license_info', '授权信息');
  if (name === 'Logs') return t('sidebar_logs');
  return '';
});

// API
const fetchPlugins = async () => {
  loadingPlugins.value = true;
  try {
    const res = await axios.get('/api/plugins');
    if (res.data.code === 0) {
      plugins.value = res.data.data;
      syncMqttStatusSSE();
    }
  } catch (e) {
    console.error("Failed to fetch plugins", e);
    showToast('danger', 'Failed to fetch plugins: ' + (e.message || 'Unknown error'));
  } finally {
    loadingPlugins.value = false;
  }
};

const updatePluginStatus = async (name, enabled) => {
  try {
    const res = await axios.post(`/api/plugins/${name}/config`, { enabled });
    if (res.data && res.data.code !== 0) {
      throw new Error(res.data.message || 'API Error');
    }
    // 可逆操作：即时执行 + Toast 撤销（§10.3）
    showToast('success', gt('gateway_plugin_status_updated', { action: gatewayActionText(locale.value, enabled) }), {
      actionLabel: t('common_undo', '撤销 Undo'),
      onAction: () => updatePluginStatus(name, !enabled)
    });
    await fetchPlugins(); // Refresh list
  } catch (e) {
    showToast('danger', `${gt('gateway_plugin_status_update_failed')}: ${e.message}`);
  }
};

let mqttStatusSSE = null;
let workOrderNotificationSSE = null;
const shownWorkOrderNotificationIDs = new Set();
let aiBrainSuggestionTimer = null;
let aiBrainSuggestionInterceptor = null;
const activeHabitRuleSuggestion = ref(null);
const createdHabitRule = ref(null);
const habitRuleActionLoading = ref(false);

const habitRuleMessages = {
  en: {
    title: 'Xiaoyou discovered a usage habit', ruleName: 'Rule', trigger: 'Trigger', action: 'Action', status: 'Status',
    confirmationHint: 'Choose Yes to create and enable this rule now. You can manage it later in Rule Scenarios.',
    created: 'The automation rule was created and enabled.', enabled: 'Enabled', disabled: 'Disabled',
    yes: 'Yes, create rule', no: 'No, dismiss', acknowledge: 'Got it', failed: 'Failed to process the AI Brain habit suggestion',
  },
  zh: {
    title: '小优发现了您的使用习惯', ruleName: '具体规则', trigger: '触发条件', action: '执行动作', status: '当前状态',
    confirmationHint: '选择“是”后将立即创建并启用该规则，之后可在规则场景中管理。',
    created: '自动化规则已创建并启用。', enabled: '已启用', disabled: '未启用',
    yes: '是，创建规则', no: '否，忽略建议', acknowledge: '我知道了', failed: 'AI 大脑习惯建议处理失败',
  },
};

const habitRuleLanguage = computed(() => String(locale.value || '').toLowerCase().startsWith('en') ? 'en' : 'zh');
const habitRuleText = (key) => habitRuleMessages[habitRuleLanguage.value][key] || habitRuleMessages.en[key] || key;
const activeHabitRuleDescription = computed(() => {
  try {
    return describeHabitRule(activeHabitRuleSuggestion.value);
  } catch (error) {
    return { name: '-', trigger: '-', action: '-' };
  }
});

const fetchMqttStatus = async () => {
  if (!activeMqttPlugin.value) return;
  const rawName = String(activeMqttPlugin.value?.name || activeMqttPlugin.value?.Name || '').trim().toLowerCase();
  const pluginEndpoint = rawName.includes('cascade') ? 'cascade' : (rawName.includes('mqtt') ? 'mqtt_api' : rawName);

  try {
    const res = await axios.get(`/api/extension/${pluginEndpoint}/status`);
    if (res.data) {
      const data = res.data;
      const isConnected = data.connected === true || String(data.status || '').toLowerCase() === 'connected';
      mqttStatus.value = {
        connected: isConnected,
        status: isConnected ? 'connected' : (data.status || 'disconnected'),
        mode: data.mode || '',
        broker: data.broker || '',
        gatewayCode: data.gateway_code || data.gatewayCode || '',
        ts: data.ts || null
      };
    }
  } catch (err) {
    // Keep previous state or fallback
  }
};

const initMqttStatusSSE = () => {
  if (mqttStatusSSE || !cascadePluginEnabled.value) return;
  const token = encodeURIComponent(authStore.token || localStorage.getItem('access_token') || '');
  if (!token) return;
  mqttStatusSSE = new EventSource(`/api/extension/cascade/stream?token=${token}`);
  mqttStatusSSE.addEventListener('status', (e) => {
    try {
      const data = JSON.parse(e.data);
      const isConnected = data.connected === true || String(data.status || '').toLowerCase() === 'connected';
      mqttStatus.value = {
        connected: isConnected,
        status: isConnected ? 'connected' : (data.status || 'disconnected'),
        mode: data.mode || '',
        broker: data.broker || '',
        gatewayCode: data.gateway_code || '',
        ts: data.ts || null
      };
    } catch (err) {
      console.error("Failed to parse MQTT SSE data", err);
    }
  });
  mqttStatusSSE.onerror = () => {
    // Temporary reconnect blip: fallback to REST check instead of immediately showing disconnected
    fetchMqttStatus();
  };
};

const closeMqttStatusSSE = () => {
  if (mqttStatusSSE) {
    mqttStatusSSE.close();
    mqttStatusSSE = null;
  }
  if (mqttStatusTimer) {
    clearInterval(mqttStatusTimer);
    mqttStatusTimer = null;
  }
};

const syncMqttStatusSSE = () => {
  if (isMqttPluginEnabled.value) {
    fetchMqttStatus();
    if (!mqttStatusTimer) {
      mqttStatusTimer = setInterval(fetchMqttStatus, 10000);
    }
    if (cascadePluginEnabled.value) {
      initMqttStatusSSE();
    }
    return;
  }
  closeMqttStatusSSE();
  mqttStatus.value = defaultMqttStatus();
};

const handlePluginConfigUpdated = async () => {
  if (mqttStatusSSE) {
    mqttStatusSSE.close();
    mqttStatusSSE = null;
  }
  await fetchPlugins();
  await fetchMqttStatus();
  if (cascadePluginEnabled.value) {
    initMqttStatusSSE();
  }
};

const initWorkOrderNotificationSSE = () => {
  if (workOrderNotificationSSE || !authStore.hasPermission('work_order:list')) return;
  const token = encodeURIComponent(authStore.token || localStorage.getItem('access_token') || '');
  workOrderNotificationSSE = new EventSource(`/api/work-order-notifications/stream?token=${token}`);
  workOrderNotificationSSE.addEventListener('snapshot', () => {
    // Historical unread records are available from the durable inbox. They do
    // not create a burst of popups when a browser reconnects.
  });
  workOrderNotificationSSE.addEventListener('notification', (event) => {
    try {
      const notification = JSON.parse(event.data);
      const notificationID = notification.public_id || notification.PublicID;
      if (!notificationID || shownWorkOrderNotificationIDs.has(notificationID)) return;
      shownWorkOrderNotificationIDs.add(notificationID);
      if (shownWorkOrderNotificationIDs.size > 200) {
        const [first] = shownWorkOrderNotificationIDs;
        if (first) shownWorkOrderNotificationIDs.delete(first);
      }
      const title = notification.title || '工单任务提醒';
      const content = notification.content || notification.work_order_title || '有新的工单任务需要处理';
      showToast('warning', `${title}：${content}`);
    } catch (error) {
      console.error('Failed to parse work-order notification event', error);
    }
  });
  workOrderNotificationSSE.onerror = () => {
    if (workOrderNotificationSSE?.readyState === EventSource.CLOSED) {
      workOrderNotificationSSE.close();
      workOrderNotificationSSE = null;
      window.setTimeout(initWorkOrderNotificationSSE, 3000);
    }
  };
};

const closeWorkOrderNotificationSSE = () => {
  if (workOrderNotificationSSE) {
    workOrderNotificationSSE.close();
    workOrderNotificationSSE = null;
  }
  shownWorkOrderNotificationIDs.clear();
};

const checkAIBrainSuggestions = async () => {
  if (!authStore.hasPermission('ai_brain:suggestion')) return;
  try {
    const res = await axios.get('/api/plugins/ai_brain/suggestions', {
      params: { status: 'pending', page_size: 50 },
    });
    if (!res.data || res.data.code !== 0) return;
    const page = res.data.data;
    const items = Array.isArray(page) ? page : (Array.isArray(page?.items) ? page.items : []);
    if (!activeHabitRuleSuggestion.value && !createdHabitRule.value) {
      const habitSuggestion = items.find(isHabitRuleSuggestion);
      if (habitSuggestion) activeHabitRuleSuggestion.value = habitSuggestion;
    }
  } catch {}
};

const transitionHabitRuleSuggestion = async (suggestion, action) => {
  const response = await axios.post(`/api/plugins/ai_brain/suggestions/${suggestion.id}/${action}`, {});
  if (!response.data || response.data.code !== 0) {
    throw new Error(response.data?.message || habitRuleText('failed'));
  }
  return response.data.data;
};

const confirmHabitRuleSuggestion = async () => {
  if (!activeHabitRuleSuggestion.value || habitRuleActionLoading.value) return;
  habitRuleActionLoading.value = true;
  try {
    const payload = buildHabitRuleCreatePayload(activeHabitRuleSuggestion.value);
    const response = await axios.post('/api/rules', payload);
    if (!response.data || response.data.code !== 0) {
      throw new Error(response.data?.message || habitRuleText('failed'));
    }
    await transitionHabitRuleSuggestion(activeHabitRuleSuggestion.value, 'accept');
    createdHabitRule.value = response.data.data;
  } catch (error) {
    showToast('danger', `${habitRuleText('failed')}: ${error.message || error}`);
  } finally {
    habitRuleActionLoading.value = false;
  }
};

const dismissHabitRuleSuggestion = async () => {
  if (!activeHabitRuleSuggestion.value || habitRuleActionLoading.value) return;
  habitRuleActionLoading.value = true;
  try {
    await transitionHabitRuleSuggestion(activeHabitRuleSuggestion.value, 'dismiss');
    activeHabitRuleSuggestion.value = null;
  } catch (error) {
    showToast('danger', `${habitRuleText('failed')}: ${error.message || error}`);
  } finally {
    habitRuleActionLoading.value = false;
  }
};

const acknowledgeCreatedHabitRule = () => {
  activeHabitRuleSuggestion.value = null;
  createdHabitRule.value = null;
};

const startAIBrainSuggestionReminder = () => {
  if (aiBrainSuggestionTimer) return;
  checkAIBrainSuggestions();
  aiBrainSuggestionTimer = window.setInterval(checkAIBrainSuggestions, 120000);
  aiBrainSuggestionInterceptor = axios.interceptors.response.use((response) => {
    if (isSuccessfulDeviceWriteResponse(response)) {
      window.setTimeout(checkAIBrainSuggestions, 0);
    }
    return response;
  }, (error) => Promise.reject(error));
};

const stopAIBrainSuggestionReminder = () => {
  if (aiBrainSuggestionTimer) {
    window.clearInterval(aiBrainSuggestionTimer);
    aiBrainSuggestionTimer = null;
  }
  if (aiBrainSuggestionInterceptor !== null) {
    axios.interceptors.response.eject(aiBrainSuggestionInterceptor);
    aiBrainSuggestionInterceptor = null;
  }
  activeHabitRuleSuggestion.value = null;
  createdHabitRule.value = null;
};

const showForcePasswordModal = async () => {
  await nextTick();
  const modalElement = document.getElementById('forceChangePasswordModal');
  if (!modalElement) return;
  forcePasswordModal = Modal.getOrCreateInstance(modalElement);
  forcePasswordModal.show();
};

const loadShellData = () => {
  checkLicense();
  fetchPlugins();
  fetchMqttStatus();
  authStore.refreshProfile().catch(() => {});
  initWorkOrderNotificationSSE();
  startAIBrainSuggestionReminder();

  axios.get('/api/setup/status').then(res => {
    if (res.data && res.data.code === 0) {
      authStore.setSystemMode(res.data.data.mode);
    }
  }).catch(e => {});

  if (authStore.user && authStore.user.must_change_password) {
    void showForcePasswordModal();
  }
};

// Navigation
const handleNavigate = (target) => {
  // Map old view names to route names if necessary, or assume target.view matches route names (lowercase/uppercase?)
  // My route names are PascalCase: Dashboard, Marketplace, Products, Devices
  // Sidebar emits: dashboard, marketplace, products, devices
  
  let routeName = '';
  if (target.view === 'dashboard') routeName = 'Dashboard';
  else if (target.view === 'marketplace') routeName = 'Marketplace';
  else if (target.view === 'products') routeName = 'Products';
  else if (target.view === 'devices') routeName = 'Devices';
  
  if (target.pluginName) {
    openPluginConfig(target.pluginName);
  } else if (routeName) {
    router.push({ name: routeName });
  }
  
  // Close sidebar on mobile
  if (window.innerWidth < 768) {
    sidebarOpen.value = false;
  }
};

const openPluginConfig = (name) => {
  router.push({ name: 'PluginConfig', params: { name } });
};

// Theme & Language
const setTheme = (theme) => {
  currentTheme.value = theme;
  localStorage.setItem('theme', theme);
  window.dispatchEvent(new CustomEvent('noyo-theme-changed', { detail: { theme } }));
};

const setLanguage = (lang) => {
  locale.value = lang;
  localStorage.setItem('lang', lang);
};

const setLiquidGlassDensity = (value) => {
  liquidGlassDensity.value = writeLiquidGlassDensity(window.localStorage, value);
};

const customBgImage = ref(localStorage.getItem('noyo_custom_bg') || '');

const handleBgChange = (e) => {
  if (e && e.detail !== undefined) {
    customBgImage.value = e.detail;
  } else {
    customBgImage.value = localStorage.getItem('noyo_custom_bg') || '';
  }
};

onMounted(() => {
  window.addEventListener('noyo-bg-changed', handleBgChange);
  window.addEventListener('cascade-config-updated', handlePluginConfigUpdated);
  window.addEventListener('plugin-config-updated', handlePluginConfigUpdated);

  // Restore language
  const savedLang = localStorage.getItem('lang');
  if (savedLang) {
    locale.value = savedLang;
  }

  if (shouldLoadShellData.value) {
    loadShellData();
  }
});

onBeforeUnmount(() => {
  window.removeEventListener('noyo-bg-changed', handleBgChange);
  window.removeEventListener('cascade-config-updated', handlePluginConfigUpdated);
  window.removeEventListener('plugin-config-updated', handlePluginConfigUpdated);
  closeMqttStatusSSE();
  closeWorkOrderNotificationSSE();
  stopAIBrainSuggestionReminder();
});

watch(shouldLoadShellData, (enabled) => {
  if (enabled) {
    loadShellData();
  } else {
    closeMqttStatusSSE();
    closeWorkOrderNotificationSSE();
    stopAIBrainSuggestionReminder();
    plugins.value = [];
    mqttStatus.value = defaultMqttStatus();
  }
}, { flush: 'post' });

const checkLicense = async () => {
  try {
    const res = await axios.get('/api/extension/license/status');
    if (res.data && res.data.code === 200) {
      licenseData.value = res.data.data;
      if (licenseData.value.status === 'authorized' && licenseData.value.expire_time && licenseData.value.type !== 'permanent') {
        const expireDate = new Date(licenseData.value.expire_time);
        const now = new Date();
        const diffDays = Math.ceil((expireDate - now) / (1000 * 60 * 60 * 24));
        if (diffDays <= 7 && diffDays >= 0) {
          showToast('warning', t('license_expiring_soon', `您的许可证将在 ${diffDays} 天后过期，请尽快更新！`));
        }
      }
    }
  } catch (e) {
    // API not found (e.g. open source version)
  }
};

const submitForceChangePassword = async () => {
  if (forcePasswordForm.value.newPassword !== forcePasswordForm.value.confirmPassword) {
    showToast('danger', t('auth_password_mismatch', '两次输入的密码不一致！'));
    return;
  }
  forcePasswordForm.value.loading = true;
  try {
    const res = await authStore.changePassword(forcePasswordForm.value.oldPassword, forcePasswordForm.value.newPassword);
    if (res.code === 0) {
      showToast('success', t('auth_password_changed', '密码修改成功，请重新登录'));
      if (forcePasswordModal) forcePasswordModal.hide();
      authStore.logout();
      router.push('/login');
    } else {
      showToast('danger', res.message || t('auth_password_change_failed', '密码修改失败'));
    }
  } catch (err) {
    showToast('danger', err.response?.data?.message || t('auth_network_error', '网络错误'));
  } finally {
    forcePasswordForm.value.loading = false;
  }
};

</script>
