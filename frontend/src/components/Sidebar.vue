<template>
  <div class="sidebar" :class="{ show: isOpen }">
    <div class="sidebar-brand">
      <img src="/Noyo.svg" alt="Noyo Logo" class="brand-logo" />
      <span>{{ $t('brand_name') }}</span>
    </div>
    
    <div class="sidebar-menu">
      <div class="nav-category">{{ $t('sidebar_main') }}</div>
      <a href="#" class="nav-link" :class="{ active: currentRouteName === 'Dashboard' }" @click.prevent="navigate('/')">
        <i class="bi bi-speedometer2"></i> <span>{{ $t('sidebar_dashboard') }}</span>
      </a>
      <a href="#" class="nav-link" :class="{ active: currentRouteName === 'Marketplace' }" @click.prevent="navigate('/marketplace')">
        <i class="bi bi-shop"></i> <span>{{ $t('sidebar_marketplace') }}</span>
      </a>
      <a href="#" class="nav-link" :class="{ active: currentRouteName === 'Products' }" @click.prevent="navigate('/products')">
        <i class="bi bi-box-seam"></i> <span>{{ $t('sidebar_products') }}</span>
      </a>
      <a href="#" class="nav-link" :class="{ active: currentRouteName === 'Devices' }" @click.prevent="navigate('/devices')">
        <i class="bi bi-cpu"></i> <span>{{ $t('sidebar_devices') }}</span>
      </a>
      <a href="#" class="nav-link" :class="{ active: currentRouteName === 'DeviceTopology' }" @click.prevent="navigate('/topology')">
        <i class="bi bi-diagram-2"></i> <span>{{ $t('sidebar_topology') }}</span>
      </a>
      <a v-if="hasGb28181Plugin" href="#" class="nav-link" :class="{ active: currentRouteName === 'VideoSquare' }" @click.prevent="navigate('/video-square')">
        <i class="bi bi-grid-3x3-gap"></i> <span>{{ $t('sidebar_video_square', '视频广场') }}</span>
      </a>
      
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
      
      <div class="nav-category mt-2">{{ $t('sidebar_system') }}</div>
      <a href="#" class="nav-link" :class="{ active: currentRouteName === 'Settings' }" @click.prevent="navigate('/settings')">
        <i class="bi bi-gear"></i> <span>{{ $t('sidebar_settings') }}</span>
      </a>
      <a v-if="isPro" href="#" class="nav-link" :class="{ active: currentRouteName === 'License' }" @click.prevent="navigate('/license')">
        <i class="bi bi-shield-check"></i> <span>{{ $t('license_info', '授权信息') }}</span>
      </a>
      <a href="#" class="nav-link" :class="{ active: currentRouteName === 'Logs' }" @click.prevent="navigate('/logs')">
        <i class="bi bi-journal-text"></i> <span>{{ $t('sidebar_logs') }}</span>
      </a>
    </div>
  </div>
</template>

<script setup>
import { computed, ref, onMounted } from 'vue';
import { useI18n } from 'vue-i18n';
import { useRouter, useRoute } from 'vue-router';
import axios from 'axios';

const props = defineProps({
  isOpen: Boolean,
  plugins: Array,
  loading: Boolean
});

const emit = defineEmits(['navigate']);

const { t, locale } = useI18n();
const router = useRouter();
const route = useRoute();

const isPro = ref(false);

onMounted(async () => {
  try {
    const res = await axios.get('/api/extension/license/status');
    if (res.data && res.data.code === 200) {
      isPro.value = true;
    }
  } catch (e) {
    isPro.value = false;
  }
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

const hasGb28181Plugin = computed(() => {
  return activePlugins.value.some(p => p.name === 'gb28181');
});

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
