<template>
  <div :data-bs-theme="currentTheme === 'system' ? systemTheme : currentTheme">
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
          <!-- Global AI Copilot Floating Widget -->
          <GlobalAiCopilot />
        </div>
      </div>
    </div>
    
    <ToastContainer />
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue';
import { useI18n } from 'vue-i18n';
import { useRouter, useRoute } from 'vue-router';
import axios from 'axios';
import Sidebar from './components/Sidebar.vue';
import TopHeader from './components/TopHeader.vue';
import ToastContainer from './components/ToastContainer.vue';
import GlobalAiCopilot from './components/common/GlobalAiCopilot.vue'; // Added GlobalAiCopilot import
import { useToast } from './composables/useToast';

const { t, locale } = useI18n();
const { showToast } = useToast();
const router = useRouter();
const route = useRoute();

// State
const sidebarOpen = ref(false); // Mobile sidebar
const plugins = ref([]);
const loadingPlugins = ref(false);

// Theme
const currentTheme = ref(localStorage.getItem('theme') || 'dark');
const systemTheme = ref(window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light');

// Computed
const currentPluginName = computed(() => route.params.name);

const pageTitle = computed(() => {
  const name = route.name;
  if (name === 'Dashboard') return t('page_dashboard');
  if (name === 'Marketplace') return t('page_marketplace');
  if (name === 'Products') return t('sidebar_products');
  if (name === 'Devices') return t('sidebar_devices');
  if (name === 'PluginConfig') return `${currentPluginName.value} ${t('page_configure')}`;
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
    showToast('danger', 'Failed to fetch plugins: ' + e.message);
  } finally {
    loadingPlugins.value = false;
  }
};

const updatePluginStatus = async (name, enabled) => {
  try {
    await axios.post(`/api/plugins/${name}/config`, { enabled });
    showToast('success', `Plugin ${enabled ? 'enabled' : 'disabled'} successfully.`);
    await fetchPlugins(); // Refresh list
  } catch (e) {
    showToast('danger', 'Failed to update plugin status: ' + e.message);
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
  fetchPlugins();
  // Restore language
  const savedLang = localStorage.getItem('lang');
  if (savedLang) {
    locale.value = savedLang;
  }
});
</script>
