<template>
  <div class="gateway-plugins-panel">
    <div class="gateway-plugins-header d-flex align-items-center justify-content-between mb-4">
      <div>
        <button v-if="!embedded" class="btn btn-link px-0 mb-2" @click="$router.push({ name: 'GatewayManagement' })">
          <i class="bi bi-arrow-left me-1"></i>{{ gt('gateway_management') }}
        </button>
        <div v-else class="gateway-plugins-kicker">{{ gt('remote_gateway_config') }}</div>
        <h5 class="mb-1">{{ gateway?.name || currentGwSn }} {{ gt('gateway_plugin_marketplace_title') }}</h5>
        <div class="text-muted small">
          {{ gt('gateway_plugins_hint') }}
          <span v-if="gateway" class="ms-2">SN {{ currentGwSn }}</span>
        </div>
      </div>
      <div class="d-flex gap-2">
        <button class="btn btn-outline-primary btn-sm" @click="fetchPlugins" :disabled="loading">
          <i class="bi bi-arrow-clockwise me-1"></i>{{ gt('sync') }}
        </button>
      </div>
    </div>

    <Marketplace
      :plugins="marketplacePlugins"
      :loading="loading"
      @configure="openPluginConfig"
      @update-status="updatePluginStatus"
    />
  </div>
</template>

<script setup>
import { computed, onMounted, ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import { useRoute, useRouter } from 'vue-router';
import axios from 'axios';
import Marketplace from './Marketplace.vue';
import { useToast } from '../composables/useToast';
import { gatewayActionText, gatewayText } from '../utils/gatewayLocale';

const props = defineProps({
  embedded: Boolean,
  gwSn: String,
  gateway: Object
});

const emit = defineEmits(['close', 'configure', 'refresh-gateway', 'plugins-loaded']);

const route = useRoute();
const router = useRouter();
const { locale } = useI18n();
const { showToast } = useToast();
const loading = ref(false);
const plugins = ref([]);
const gt = (key, params) => gatewayText(locale.value, key, params);

const currentGwSn = computed(() => props.gwSn || route.params.gwSn);
const marketplacePlugins = computed(() => plugins.value.filter((plugin) => plugin.name !== 'license_auth'));

const fetchPlugins = async () => {
  if (!currentGwSn.value) return;
  loading.value = true;
  try {
    const res = await axios.get(`/api/extension/cascade/gateways/${currentGwSn.value}/plugins`);
    if (res.data.code === 0) {
      plugins.value = res.data.data || [];
      emit('plugins-loaded', marketplacePlugins.value);
    } else {
      showToast('danger', gt('gateway_plugins_load_failed'));
    }
  } catch (e) {
    showToast('danger', gt('gateway_plugins_load_failed'));
  } finally {
    loading.value = false;
  }
};

const openPluginConfig = (name) => {
  if (props.embedded) {
    emit('configure', name);
    return;
  }
  router.push({ name: 'GatewayPluginConfig', params: { gwSn: currentGwSn.value, name } });
};

const updatePluginStatus = async (name, enabled) => {
  try {
    const res = await axios.post(`/api/extension/cascade/gateways/${currentGwSn.value}/plugins/${name}/status`, { enabled });
    if (res.data.code === 0) {
      showToast('success', gt('gateway_plugin_status_updated', { action: gatewayActionText(locale.value, enabled) }));
      await fetchPlugins();
      emit('refresh-gateway');
    } else {
      showToast('danger', gt('gateway_plugin_status_update_failed'));
    }
  } catch (e) {
    showToast('danger', gt('gateway_plugin_status_update_failed'));
  }
};

watch(currentGwSn, fetchPlugins);
onMounted(fetchPlugins);
</script>

<style scoped>
.gateway-plugins-panel {
  min-height: 100%;
}

.gateway-plugins-kicker {
  color: var(--accent-color);
  font-size: 0.72rem;
  font-weight: 700;
  letter-spacing: 0.08em;
  margin-bottom: 0.35rem;
  text-transform: uppercase;
}

.gateway-plugins-header h5 {
  color: var(--text-main);
  font-weight: 750;
}
</style>
