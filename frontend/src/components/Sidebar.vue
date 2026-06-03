<template>
  <div class="sidebar" :class="{ show: isOpen }">
    <div class="sidebar-brand">
      <template v-if="authStore.user && authStore.user.tenant_name">
        <div v-if="authStore.user.tenant_logo" class="me-2 d-flex align-items-center justify-content-center" style="height: 32px; width: 32px; overflow: hidden;">
          <div v-if="authStore.user.tenant_logo.trim().startsWith('<svg') || authStore.user.tenant_logo.trim().startsWith('<?xml')" v-html="DOMPurify.sanitize(authStore.user.tenant_logo, { USE_PROFILES: { svg: true } })" class="svg-container" style="height: 100%; display: flex; align-items: center; justify-content: center;"></div>
          <img v-else :src="authStore.user.tenant_logo" style="max-height: 100%; max-width: 100%; object-fit: contain;">
        </div>
        <span class="text-truncate">{{ authStore.user.tenant_name }}</span>
      </template>
      <template v-else>
        <img src="/Noyo.svg" alt="Noyo Logo" class="brand-logo" />
        <span>{{ $t('brand_name') }}</span>
      </template>
    </div>

    <!-- Project Selector (Moved to Top of Sidebar) -->
    <div class="px-3 mb-3" v-if="authStore.isLoggedIn && userProjects.length > 0">
      <select class="form-select form-select-sm" v-model="currentProjectId" @change="switchProject">
        <option v-if="canUseAllProjects" :value="0">{{ $t('project_all') }}</option>
        <option v-for="p in userProjects" :key="p.ID" :value="p.ID">{{ p.name }}</option>
      </select>
    </div>
    
    <div class="sidebar-menu">
      <div class="nav-category">{{ $t('sidebar_main') }}</div>
      <a v-if="authStore.hasPermission('dashboard:view')" href="#" class="nav-link" :class="{ active: currentRouteName === 'Dashboard' }" @click.prevent="navigate('/')">
        <i class="bi bi-speedometer2"></i> <span>{{ $t('sidebar_dashboard') }}</span>
      </a>
      <a v-if="authStore.hasPermission('plugin:list')" href="#" class="nav-link" :class="{ active: currentRouteName === 'Marketplace' }" @click.prevent="navigate('/marketplace')">
        <i class="bi bi-shop"></i> <span>{{ $t('sidebar_marketplace') }}</span>
      </a>
      <a v-if="authStore.hasPermission('product:list')" href="#" class="nav-link" :class="{ active: currentRouteName === 'Products' }" @click.prevent="navigate('/products')">
        <i class="bi bi-box-seam"></i> <span>{{ $t('sidebar_products') }}</span>
      </a>
      <a v-if="!isGatewayRuntime && authStore.hasPermission('gateway:list')" href="#" class="nav-link" :class="{ active: currentRouteName === 'GatewayManagement' || currentRouteName === 'GatewayPlugins' || currentRouteName === 'GatewayPluginConfig' }" @click.prevent="navigate('/gateways')">
        <i class="bi bi-hdd-network"></i> <span>{{ gt('gateway_management') }}</span>
      </a>
      <a v-if="authStore.hasPermission('device:list')" href="#" class="nav-link" :class="{ active: currentRouteName === 'Devices' }" @click.prevent="navigate('/devices')">
        <i class="bi bi-cpu"></i> <span>{{ $t('sidebar_devices') }}</span>
      </a>
      <a v-if="authStore.hasPermission('device_tag:list')" href="#" class="nav-link" :class="{ active: currentRouteName === 'DeviceTags' }" @click.prevent="navigate('/device-tags')">
        <i class="bi bi-tags"></i> <span>{{ $t('sidebar_device_tags') }}</span>
      </a>
      <a v-if="authStore.hasPermission('device:topology')" href="#" class="nav-link" :class="{ active: currentRouteName === 'DeviceTopology' }" @click.prevent="navigate('/topology')">
        <i class="bi bi-diagram-2"></i> <span>{{ $t('sidebar_topology') }}</span>
      </a>
      <a v-if="authStore.hasPermission('alarm:list')" href="#" class="nav-link" :class="{ active: currentRouteName === 'AlarmCenter' }" @click.prevent="navigate('/alarms')">
        <i class="bi bi-bell-fill"></i> <span>{{ $t('sidebar_alarms', '告警中心') }}</span>
      </a>
      
      <!-- Dynamic Extension Menus -->
      <template v-for="menu in extensionMenus" :key="menu.name">
        <a v-if="isMenuVisible(menu)" href="#" class="nav-link" :class="{ active: currentRouteName === menu.name }" @click.prevent="navigate(menu.path)">
          <i :class="menu.icon"></i> <span>{{ menu.labelKey ? $t(menu.labelKey, menu.defaultLabel) : menu.defaultLabel }}</span>
        </a>
      </template>

      <template v-if="authStore.hasPermission('plugin:list')">
        <div class="nav-category mt-2">{{ $t('sidebar_active_plugins') }}</div>
        <div id="plugin-nav-list">
          <div v-if="loading" class="px-4 py-2 text-muted small">{{ $t('loading') }}</div>
          <div v-else-if="activePlugins.length === 0" class="px-4 py-2 text-muted small">
            {{ $t('no_active_plugins') }}
          </div>
          <div v-else>
            <div v-for="group in groupedPlugins" :key="group.category">
               <div class="nav-category mt-2">{{ group.title }}</div>
               <a v-for="plugin in group.items" :key="plugin.name" 
                 href="#"
                 class="nav-link" 
                 :class="{ active: currentRouteParams.name === plugin.name }"
                 @click.prevent="navigatePlugin(plugin.name)">
                 <img v-if="plugin.icon" :src="plugin.icon" class="me-2" style="width: 16px; height: 16px;">
                 <i v-else class="bi bi-plugin"></i>
                 <span class="flex-grow-1 text-truncate">{{ plugin.title ? (plugin.title[locale] || plugin.title['en'] || plugin.name) : plugin.name }}</span>
                 <i class="bi bi-circle-fill text-success" style="font-size: 8px; width: auto; margin: 0;"></i>
              </a>
            </div>
          </div>
        </div>
      </template>
      
      <div class="nav-category mt-2">{{ $t('sidebar_system') }}</div>
      <a v-if="authStore.hasPermission('user:list')" href="#" class="nav-link" :class="{ active: currentRouteName === 'UserManagement' }" @click.prevent="navigate('/settings/users')">
        <i class="bi bi-people"></i> <span>{{ $t('user_management', 'User Management') }}</span>
      </a>
      <a v-if="authStore.hasPermission('tenant:list')" href="#" class="nav-link" :class="{ active: currentRouteName === 'TenantManagement' }" @click.prevent="navigate('/settings/tenants')">
        <i class="bi bi-building"></i> <span>{{ $t('tenant_management') }}</span>
      </a>
      <a v-if="authStore.hasPermission('project:list')" href="#" class="nav-link" :class="{ active: currentRouteName === 'ProjectManagement' }" @click.prevent="navigate('/settings/projects')">
        <i class="bi bi-folder"></i> <span>{{ $t('project_management') }}</span>
      </a>
      <a v-if="authStore.hasPermission('role:list')" href="#" class="nav-link" :class="{ active: currentRouteName === 'RoleManagement' }" @click.prevent="navigate('/settings/roles')">
        <i class="bi bi-shield-lock"></i> <span>{{ $t('role_management') }}</span>
      </a>
      <a v-if="authStore.hasPermission('position:list')" href="#" class="nav-link" :class="{ active: currentRouteName === 'PositionManagement' }" @click.prevent="navigate('/settings/positions')">
        <i class="bi bi-person-badge"></i> <span>{{ $t('position_management', 'Position Management') }}</span>
      </a>
      <a v-if="authStore.hasPermission('app:list')" href="#" class="nav-link" :class="{ active: currentRouteName === 'AppManagement' }" @click.prevent="navigate('/settings/apps')">
        <i class="bi bi-window-sidebar"></i> <span>{{ $t('app_management', 'App Integration') }}</span>
      </a>
      <a v-if="authStore.hasPermission('audit:list')" href="#" class="nav-link" :class="{ active: currentRouteName === 'AuditLogs' }" @click.prevent="navigate('/settings/audit-logs')">
        <i class="bi bi-journal-text"></i> <span>{{ $t('audit_logs', 'Audit Logs') }}</span>
      </a>
      <a v-if="authStore.hasPermission('system:license')" href="#" class="nav-link" :class="{ active: currentRouteName === 'License' }" @click.prevent="navigate('/license')">
        <i class="bi bi-shield-check"></i> <span>{{ $t('license_info', '授权信息') }}</span>
      </a>
      <a v-if="authStore.hasPermission('system:logs')" href="#" class="nav-link" :class="{ active: currentRouteName === 'Logs' }" @click.prevent="navigate('/logs')">
        <i class="bi bi-journal-code"></i> <span>{{ $t('system_logs', '系统日志') }}</span>
      </a>
    </div>
    <!-- Powered By Footer -->
    <div class="sidebar-footer mt-auto p-3 text-center border-top border-secondary border-opacity-25">
      <div class="text-muted" style="font-size: 0.75rem; opacity: 0.7;">
        <i class="bi bi-lightning-charge-fill text-warning me-1"></i>
        <span>Powered by <strong>Noyo</strong></span>
      </div>
    </div>
  </div>
</template>

<style scoped>
:deep(.svg-container svg) {
  max-width: 100%;
  max-height: 100%;
}

.sidebar {
  display: flex;
  flex-direction: column;
}
.sidebar-menu {
  flex-grow: 1;
  overflow-y: auto;
}
.sidebar-footer {
  flex-shrink: 0;
}
</style>

<script setup>
import { computed, ref, onMounted } from 'vue';
import { useI18n } from 'vue-i18n';
import DOMPurify from 'dompurify';
import { useRouter, useRoute } from 'vue-router';
import axios from 'axios';
import { usePlugins } from '../plugins/registry.js';
import { gatewayText } from '../utils/gatewayLocale';
import { useAuthStore } from '../stores/auth.js';

const { extensions } = usePlugins();
const authStore = useAuthStore();

const userProjects = ref([]);
const currentProjectId = ref(Number(localStorage.getItem('current_project_id') || 0));

const requiresProjectContext = computed(() => {
  const allowedProjectIds = authStore.user?.allowed_project_ids || [];
  return allowedProjectIds.length > 0 && !authStore.isTenantAdmin && !authStore.isSystemAdmin;
});

const canUseAllProjects = computed(() => !requiresProjectContext.value);

const persistProjectContext = (projectId) => {
  currentProjectId.value = Number(projectId || 0);
  if (currentProjectId.value > 0) {
    localStorage.setItem('current_project_id', currentProjectId.value.toString());
  } else {
    localStorage.removeItem('current_project_id');
  }
};

const tenantName = computed(() => {
  if (authStore.user && authStore.user.tenant_id > 0) {
    return authStore.user.tenant_name || localStorage.getItem('tenant_name') || ''
  }
  return ''
});

const loadUserProjects = async () => {
  if (!authStore.isLoggedIn) return;
  try {
    const res = await axios.get('/api/auth/projects');
    if (res.data.code === 0) {
      userProjects.value = res.data.data || [];
      await normalizeCurrentProject();
    }
  } catch (e) {
    console.error("Failed to load projects", e);
  }
};

const normalizeCurrentProject = async () => {
  const projectIds = userProjects.value.map(p => Number(p.ID));
  const selectedProjectId = Number(currentProjectId.value || 0);

  if (requiresProjectContext.value) {
    if (projectIds.length === 0) {
      persistProjectContext(0);
      return;
    }
    if (!projectIds.includes(selectedProjectId)) {
      persistProjectContext(projectIds[0]);
      await authStore.refreshProfile();
      window.location.reload();
    }
    return;
  }

  if (selectedProjectId > 0 && !projectIds.includes(selectedProjectId)) {
    persistProjectContext(0);
    await authStore.refreshProfile();
    window.location.reload();
  }
};

const switchProject = async () => {
  if (requiresProjectContext.value && Number(currentProjectId.value) <= 0 && userProjects.value.length > 0) {
    currentProjectId.value = Number(userProjects.value[0].ID);
  }
  persistProjectContext(currentProjectId.value);
  await authStore.refreshProfile();
  window.location.reload(); // Reload to apply new project scope globally
};

const props = defineProps({
  isOpen: Boolean,
  plugins: Array,
  loading: Boolean
});

const emit = defineEmits(['navigate']);

const { t, locale } = useI18n();
const router = useRouter();
const route = useRoute();
const gt = (key, params) => gatewayText(locale.value, key, params);

onMounted(async () => {
  await loadUserProjects();
});

const currentRouteName = computed(() => route.name);
const currentRouteParams = computed(() => route.params);

const navigate = (path) => {
  router.push(path);
};

const navigatePlugin = (name) => {
  router.push({ name: 'PluginConfig', params: { name } });
};

const activePlugins = computed(() => {
  return props.plugins.filter(p => p.status === 'running');
});

const cascadePlugin = computed(() => {
  return props.plugins.find((plugin) => plugin.name === 'cascade');
});

const cascadeMode = computed(() => {
  const modeField = cascadePlugin.value?.schema?.fields?.find((field) => field.name === 'mode');
  return modeField?.value || cascadePlugin.value?.config?.mode || '';
});

const isGatewayRuntime = computed(() => {
  return cascadePlugin.value?.status === 'running' && cascadeMode.value === 'gateway';
});

const extensionMenus = computed(() => {
  return extensions.value.menus || [];
});

const isMenuVisible = (menu) => {
  if (menu.requiredPlugin) {
    const pluginActive = activePlugins.value.some(p => p.name === menu.requiredPlugin);
    if (!pluginActive) return false;
  }
  const requiredPermission = menu.permission || menu.requiredPermission || menu.meta?.permission || 'plugin:config';
  return authStore.hasPermission(requiredPermission);
};

const groupedPlugins = computed(() => {
  const active = activePlugins.value;
  const platforms = active.filter(p => p.category === 'platform');
  const protocols = active.filter(p => p.category === 'protocol');
  const others = active.filter(p => p.category !== 'platform' && p.category !== 'protocol');

  const groups = [];
  if (platforms.length > 0) groups.push({ title: t('cat_platform'), items: platforms });
  if (protocols.length > 0) groups.push({ title: t('cat_protocol'), items: protocols });
  if (others.length > 0) groups.push({ title: t('cat_other'), items: others });
  
  return groups;
});

const notImplemented = () => {
  alert(t('not_implemented'));
};
</script>
