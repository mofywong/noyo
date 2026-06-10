<template>
  <div :data-bs-theme="currentTheme === 'system' ? systemTheme : currentTheme">
    <template v-if="!isStandalonePage">
      <Sidebar 
        :is-open="sidebarOpen" 
        :current-plugin="currentPluginName"
        :plugins="plugins"
        :loading="loadingPlugins"
        @navigate="handleNavigate"
      />
      
      <div class="main-content">
        <TopHeader 
          :title="pageTitle" 
          :current-theme="currentTheme"
        :mqtt-status="mqttStatus"
        @toggle-sidebar="sidebarOpen = !sidebarOpen"
        @set-theme="setTheme"
        @set-language="setLanguage"
      />
      
      <div class="content-scroll">
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
    
    <ToastContainer />
    
    <div class="modal fade" id="forceChangePasswordModal" tabindex="-1" data-bs-backdrop="static" data-bs-keyboard="false">
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
  </div>
</template>

<script setup>
import { ref, computed, onBeforeUnmount, onMounted, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import { useRouter, useRoute } from 'vue-router';
import axios from 'axios';
import Sidebar from './components/Sidebar.vue';
import TopHeader from './components/TopHeader.vue';
import ToastContainer from './components/ToastContainer.vue';
import { useToast } from './composables/useToast';
import { gatewayActionText, gatewayText } from './utils/gatewayLocale';
import { usePlugins } from './plugins/registry';
import { Modal } from 'bootstrap';
import { useAuthStore } from './stores/auth';

const { t, locale } = useI18n();
const { showToast } = useToast();
const router = useRouter();
const route = useRoute();
const authStore = useAuthStore();
const { extensions: pluginExtensions } = usePlugins();
const gt = (key, params) => gatewayText(locale.value, key, params);

// State
const sidebarOpen = ref(false); // Mobile sidebar
const plugins = ref([]);
const loadingPlugins = ref(false);
const mqttStatus = ref(null);
let mqttStatusTimer = null;

// Theme
const currentTheme = ref(localStorage.getItem('theme') || 'dark');
const systemTheme = ref(window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light');

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
const isStandalonePage = computed(() => route.name === 'Login' || route.name === 'Setup');
const shouldLoadShellData = computed(() => authStore.isLoggedIn && !isStandalonePage.value);

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
    await axios.post(`/api/plugins/${name}/config`, { enabled });
    showToast('success', gt('gateway_plugin_status_updated', { action: gatewayActionText(locale.value, enabled) }));
    await fetchPlugins(); // Refresh list
  } catch (e) {
    showToast('danger', `${gt('gateway_plugin_status_update_failed')}: ${e.message}`);
  }
};

let mqttStatusSSE = null;

const initMqttStatusSSE = () => {
  if (mqttStatusSSE) return;
  mqttStatusSSE = new EventSource('/api/extension/cascade/stream');
  mqttStatusSSE.addEventListener('status', (e) => {
    try {
      const data = JSON.parse(e.data);
      mqttStatus.value = {
        connected: data.connected === true,
        status: data.status || 'disconnected',
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
    if (mqttStatus.value) {
      mqttStatus.value = { ...mqttStatus.value, connected: false, status: 'disconnected' };
    }
  };
};

const closeMqttStatusSSE = () => {
  if (mqttStatusSSE) {
    mqttStatusSSE.close();
    mqttStatusSSE = null;
  }
};

const loadShellData = () => {
  checkLicense();
  fetchPlugins();
  initMqttStatusSSE();

  if (authStore.user && authStore.user.must_change_password) {
    forcePasswordModal = new Modal(document.getElementById('forceChangePasswordModal'));
    forcePasswordModal.show();
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

// Listen for system theme changes
window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', e => {
  systemTheme.value = e.matches ? 'dark' : 'light';
});

onMounted(() => {
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
  closeMqttStatusSSE();
});

watch(shouldLoadShellData, (enabled) => {
  if (enabled) {
    loadShellData();
  } else {
    closeMqttStatusSSE();
    plugins.value = [];
    mqttStatus.value = null;
  }
});

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
