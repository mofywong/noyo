<template>
  <div v-if="loading" class="d-flex justify-content-center py-5">
    <div class="spinner-border text-primary" role="status">
      <span class="visually-hidden">Loading...</span>
    </div>
  </div>

  <div v-else-if="plugin && schema">
    <div class="row justify-content-center">
      <div class="col-lg-8">
        <div v-if="!embedded" class="mb-3">
          <button class="btn btn-link px-0" @click="goBack">
            <i class="bi bi-arrow-left me-1"></i>{{ gwSn }} / {{ gt('gateway_plugin_marketplace_title') }}
          </button>
        </div>
        <div v-else class="mb-3">
          <button class="btn btn-light btn-sm" @click="goBack">
            <i class="bi bi-arrow-left me-1"></i>{{ gt('gateway_plugin_marketplace_title') }}
          </button>
        </div>

        <div class="alert alert-info d-flex align-items-center justify-content-between">
          <div>
            <strong>{{ gt('remote_gateway_config') }}</strong>
            <div class="small mb-0">
              {{ gt('gateway') }}: <code>{{ gwSn }}</code>
              <span class="mx-2">|</span>
              {{ gt('version') }}: {{ plugin.configVersion || 0 }}
              <span class="mx-2">|</span>
              {{ gt('updated_at') }}: {{ formatTime(plugin.updatedAt) }}
              <span class="mx-2">|</span>
              {{ syncStateLabel }}
            </div>
          </div>
          <button class="btn btn-sm btn-outline-primary" @click="pullGatewayConfig">
            <i class="bi bi-cloud-download me-1"></i>{{ gt('pull_gateway_config') }}
          </button>
        </div>

        <div v-if="gateway && !gateway.online" class="alert alert-warning">
          <i class="bi bi-cloud-slash me-2"></i>{{ gt('gateway_offline_editable') }}
        </div>

        <div v-if="syncState === 'conflict'" class="alert alert-danger gateway-conflict-banner">
          <div>
            <strong>{{ gt('gateway_sync_conflict') }}</strong>
            <div class="small">{{ gt('gateway_conflict_hint') }}</div>
          </div>
          <div class="d-flex gap-2 mt-3 mt-md-0">
            <button class="btn btn-sm btn-danger" @click="overrideGateway" :disabled="saving">
              <i class="bi bi-upload me-1"></i>{{ gt('override_gateway_config') }}
            </button>
            <button class="btn btn-sm btn-outline-danger" @click="pullGatewayConfig" :disabled="loading">
              <i class="bi bi-download me-1"></i>{{ gt('pull_gateway_config_full') }}
            </button>
          </div>
        </div>

        <PluginConfigForm
          :plugin="plugin"
          :schema="schema"
          v-model:form-data="formData"
          :locale="locale"
          :saving="saving"
          switch-id-prefix="remote-config-switch"
          save-icon-class="bi bi-cloud-upload me-2"
          :title="$t('configure')"
          :hint="$t('plugin_update_hint', { name: getLocalized(plugin.title) || plugin.name })"
          :enabled-text="$t('plugin_enabled')"
          :disabled-text="$t('plugin_disabled')"
          :running-text="$t('status_running')"
          :stopped-text="$t('status_stopped')"
          :no-config-text="$t('plugin_no_config')"
          :cancel-text="$t('tsl_cancel')"
          :save-text="$t('save_restart')"
          :saving-text="$t('saving')"
          @toggle-status="toggleStatus"
          @cancel="goBack"
          @save="saveConfig"
        >
          <template v-if="activePluginComponent" #before>
            <component
              :is="activePluginComponent"
              :key="`remote-status-${pluginName}`"
              :pluginName="pluginName"
              :remote-context="remotePluginContext"
              :api-base="remotePluginContext.apiBase"
            />
          </template>
          <template v-if="activeConfigComponent" #custom>
            <component
              :is="activeConfigComponent"
              :key="`remote-config-${pluginName}`"
              :pluginName="pluginName"
              :remote-context="remotePluginContext"
              :api-base="remotePluginContext.apiBase"
              :schema="schema"
              :gateway="gateway"
              @saved="handleCustomConfigSaved"
              @cancel="goBack"
            />
          </template>
        </PluginConfigForm>
      </div>
    </div>
  </div>

  <div v-else class="alert alert-danger">
    {{ $t('plugin_not_found') }}
  </div>
</template>

<script setup>
import { computed, onMounted, ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import { useRoute, useRouter } from 'vue-router';
import axios from 'axios';
import { useToast } from '../composables/useToast';
import { gatewayActionText, gatewayDateTime, gatewayText } from '../utils/gatewayLocale';
import { usePlugins } from '../plugins/registry';
import PluginConfigForm from '../components/plugins/PluginConfigForm.vue';

const props = defineProps({
  embedded: Boolean,
  gwSn: String,
  pluginName: String,
  gateway: Object
});

const emit = defineEmits(['back', 'refresh-gateway']);

const { locale } = useI18n();
const route = useRoute();
const router = useRouter();
const { showToast } = useToast();
const { getPluginManifest } = usePlugins();

const loading = ref(false);
const saving = ref(false);
const plugin = ref(null);
const schema = ref(null);
const formData = ref({});
const gt = (key, params) => gatewayText(locale.value, key, params);

const gwSn = computed(() => props.gwSn || route.params.gwSn);
const pluginName = computed(() => props.pluginName || route.params.name);
const syncState = computed(() => plugin.value?.syncState || 'synced');
const activePluginComponent = computed(() => {
  const manifest = getPluginManifest(pluginName.value);
  return manifest?.components?.status || null;
});
const activeConfigComponent = computed(() => {
  const manifest = getPluginManifest(pluginName.value);
  return manifest?.components?.config || null;
});
const remotePluginContext = computed(() => ({
  remote: true,
  gateway: props.gateway || null,
  gwSn: gwSn.value,
  pluginName: pluginName.value,
  plugin: plugin.value,
  schema: schema.value,
  syncState: syncState.value,
  baseVersion: plugin.value?.baseVersion || plugin.value?.configVersion || 0,
  apiBase: `/api/extension/cascade/gateways/${gwSn.value}/plugins/${pluginName.value}`
}));
const syncStateLabel = computed(() => {
  const key = `gateway_sync_${syncState.value}`;
  return gatewayText(locale.value, key);
});

const getLocalized = (obj) => {
  if (!obj) return '';
  if (typeof obj === 'string') return obj;
  return obj[locale.value] || obj.en || '';
};

const fetchPlugin = async () => {
  if (!gwSn.value || !pluginName.value) return;
  loading.value = true;
  try {
    const res = await axios.get(`/api/extension/cascade/gateways/${gwSn.value}/plugins/${pluginName.value}`);
    if (res.data.code === 0) {
      plugin.value = res.data.data;
      schema.value = res.data.data?.schema || null;
      resetFormData();
    } else {
      showToast('danger', gt('gateway_plugin_config_load_failed'));
    }
  } catch (e) {
    showToast('danger', gt('gateway_plugin_config_load_failed'));
  } finally {
    loading.value = false;
  }
};

const resetFormData = () => {
  const data = {};
  if (schema.value && schema.value.fields) {
    schema.value.fields.forEach(field => {
      data[field.name] = field.value;
    });
  }
  formData.value = data;
};

const saveConfig = async () => {
  saving.value = true;
  try {
    const payload = { ...formData.value, base_version: plugin.value?.baseVersion || plugin.value?.configVersion || 0 };
    const res = await axios.post(`/api/extension/cascade/gateways/${gwSn.value}/plugins/${pluginName.value}/config`, payload);
    if (res.data.code === 0) {
      showToast('success', gt('gateway_plugin_config_saved'));
      plugin.value = res.data.data;
      schema.value = res.data.data?.schema || schema.value;
      resetFormData();
      emit('refresh-gateway');
    } else {
      showToast('danger', gt('gateway_plugin_config_save_failed'));
    }
  } catch (e) {
    showToast('danger', gt('gateway_plugin_config_save_failed'));
  } finally {
    saving.value = false;
  }
};

const toggleStatus = async (enabled) => {
  try {
    const res = await axios.post(`/api/extension/cascade/gateways/${gwSn.value}/plugins/${pluginName.value}/status`, {
      enabled,
      base_version: plugin.value?.baseVersion || plugin.value?.configVersion || 0
    });
    if (res.data.code === 0) {
      plugin.value = res.data.data;
      schema.value = res.data.data?.schema || schema.value;
      resetFormData();
      showToast('success', gt('gateway_plugin_status_updated', { action: gatewayActionText(locale.value, enabled) }));
      emit('refresh-gateway');
    } else {
      showToast('danger', gt('gateway_plugin_status_update_failed'));
    }
  } catch (e) {
    showToast('danger', gt('gateway_plugin_status_update_failed'));
  }
};

const overrideGateway = async () => {
  saving.value = true;
  try {
    const res = await axios.post(`/api/extension/cascade/gateways/${gwSn.value}/plugins/${pluginName.value}/config?resolve=override`, formData.value);
    if (res.data.code === 0) {
      plugin.value = res.data.data;
      schema.value = res.data.data?.schema || schema.value;
      resetFormData();
      showToast('success', gt('gateway_plugin_config_saved'));
      emit('refresh-gateway');
    } else {
      showToast('danger', gt('gateway_plugin_config_save_failed'));
    }
  } catch (e) {
    showToast('danger', gt('gateway_plugin_config_save_failed'));
  } finally {
    saving.value = false;
  }
};

const handleCustomConfigSaved = (updatedPlugin) => {
  if (updatedPlugin) {
    plugin.value = updatedPlugin;
    schema.value = updatedPlugin?.schema || schema.value;
    resetFormData();
  } else {
    fetchPlugin();
  }
  emit('refresh-gateway');
};

const pullGatewayConfig = async () => {
  await fetchPlugin();
  emit('refresh-gateway');
};

const goBack = () => {
  if (props.embedded) {
    emit('back');
    return;
  }
  router.push({ name: 'GatewayPlugins', params: { gwSn: gwSn.value } });
};

const formatTime = (value) => {
  return gatewayDateTime(locale.value, value);
};

watch([gwSn, pluginName], fetchPlugin);
onMounted(fetchPlugin);
</script>
