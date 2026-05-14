<template>
  <div v-if="loading" class="d-flex justify-content-center py-5">
    <div class="spinner-border text-primary" role="status">
      <span class="visually-hidden">Loading...</span>
    </div>
  </div>
  
  <div v-else-if="plugin && schema">
    <div class="row justify-content-center">
      <div class="col-lg-8">
        <PluginConfigForm
          :plugin="plugin"
          :schema="schema"
          v-model:form-data="formData"
          :locale="locale"
          :saving="saving"
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
            <component :is="activePluginComponent" :pluginName="pluginName" :key="pluginName" v-if="activePluginComponent" />
          </template>
          <template v-if="activeConfigComponent" #custom>
            <component :is="activeConfigComponent" :pluginName="pluginName" :key="'cfg-'+pluginName" v-if="activeConfigComponent" />
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
import { ref, watch, computed, onMounted, onUnmounted } from 'vue';
import { useI18n } from 'vue-i18n';
import { useRoute, useRouter } from 'vue-router';
import axios from 'axios';
import { useToast } from '../composables/useToast';
import { usePlugins } from '../plugins/registry';
import PluginConfigForm from '../components/plugins/PluginConfigForm.vue';

const props = defineProps({
  plugins: {
    type: Array,
    default: () => []
  }
});

const emit = defineEmits(['update-status']);
const { locale, t } = useI18n();
const route = useRoute();
const router = useRouter();
const { showToast } = useToast();
const { getPluginManifest } = usePlugins();

const loading = ref(false);
const saving = ref(false);
const schema = ref(null);
const formData = ref({});

const pluginName = computed(() => route.params.name);
const plugin = computed(() => props.plugins.find(p => p.name === pluginName.value));

// Connection Status Component Logic
const activePluginComponent = computed(() => {
  const manifest = getPluginManifest(pluginName.value);
  return manifest?.components?.status || null;
});

// Custom Config Component (replaces generic form when present)
const activeConfigComponent = computed(() => {
  const manifest = getPluginManifest(pluginName.value);
  return manifest?.components?.config || null;
});

const getLocalized = (obj) => {
  if (!obj) return '';
  return obj[locale.value] || obj['en'] || '';
};

const toggleStatus = (checked) => {
  emit('update-status', plugin.value.name, checked);
};

const fetchSchema = async () => {
  if (!pluginName.value) return;
  loading.value = true;
  schema.value = null;
  try {
    const res = await axios.get(`/api/plugins/${pluginName.value}`);
    if (res.data.code === 0) {
      schema.value = res.data.data;
    }
  } catch (e) {
    console.error("Failed to fetch schema", e);
    showToast('danger', 'Failed to fetch schema: ' + e.message);
  } finally {
    loading.value = false;
  }
};

// Initialize form data when schema loads
watch(schema, (newSchema) => {
  if (newSchema && newSchema.fields) {
    const data = {};
    newSchema.fields.forEach(field => {
      data[field.name] = field.value;
    });
    formData.value = data;
  }
}, { immediate: true });

watch(pluginName, (newName) => {
  fetchSchema();
}, { immediate: true });


const saveConfig = async () => {
  saving.value = true;
  try {
    const res = await axios.post(`/api/plugins/${pluginName.value}/config`, formData.value);
    if (res.data.code === 0) {
      showToast('success', 'Saved & Restarted Successfully');
      // Ideally trigger a plugin list refresh if status changed, but usually config doesn't change status directly
    } else {
      showToast('danger', res.data.message);
    }
  } catch (e) {
    showToast('danger', 'Failed to save config: ' + e.message);
  } finally {
    saving.value = false;
  }
};

const goBack = () => {
  router.push({ name: 'Marketplace' });
};
</script>
