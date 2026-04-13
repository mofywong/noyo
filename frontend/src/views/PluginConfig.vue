<template>
  <div v-if="loading" class="d-flex justify-content-center py-5">
    <div class="spinner-border text-primary" role="status">
      <span class="visually-hidden">Loading...</span>
    </div>
  </div>
  
  <div v-else-if="plugin && schema">
    <div class="row justify-content-center">
      <div class="col-lg-8">
        <div class="card mb-4">
          <div class="card-header py-3">
            <div class="d-flex align-items-center justify-content-between w-100">
              <div class="d-flex align-items-center">
                <div class="bg-primary bg-opacity-10 p-2 rounded me-3 text-primary">
                  <i class="bi bi-sliders fs-5"></i>
                </div>
                <div>
                  <h5 class="mb-0">{{ $t('configure') }}</h5>
                  <small class="text-muted">{{ $t('plugin_update_hint', { name: getLocalized(plugin.title) || plugin.name }) }}</small>
                </div>
              </div>
              
              <div class="d-flex align-items-center gap-3">
                <div class="form-check form-switch mb-0 d-flex align-items-center">
                  <input class="form-check-input me-2" type="checkbox" role="switch" 
                         style="cursor: pointer;"
                         :id="'config-switch-'+plugin.name" 
                         :checked="plugin.status === 'running'"
                         @change="toggleStatus($event.target.checked)">
                  <label class="form-check-label small fw-medium text-muted" :for="'config-switch-'+plugin.name">
                    {{ plugin.status === 'running' ? $t('plugin_enabled') : $t('plugin_disabled') }}
                  </label>
                </div>
                <div class="vr mx-1"></div>
                <span v-if="plugin.status === 'running'" class="badge bg-success">{{ $t('status_running') }}</span>
                <span v-else class="badge bg-secondary">{{ $t('status_stopped') }}</span>
              </div>
            </div>
          </div>
          
          <div class="card-body">
            <!-- Platform Connection Status Display (Dynamic Plugin Component) -->
            <component :is="activePluginComponent" :pluginName="pluginName" :key="pluginName" v-if="activePluginComponent" />

            <!-- Custom Config Component (e.g. AI Copilot) -->
            <component :is="activeConfigComponent" :pluginName="pluginName" :key="'cfg-'+pluginName" v-if="activeConfigComponent" />

            <template v-else>
            <div v-if="!schema.fields || schema.fields.length === 0" class="alert alert-info">
              {{ $t('plugin_no_config') }}
            </div>
            
            <form v-else @submit.prevent="saveConfig">
              <div v-for="field in schema.fields" :key="field.name">
                <template v-if="field.name !== 'enabled'">
                  <!-- Switch/Bool -->
                  <div v-if="field.type === 'switch' || field.type === 'bool'" class="mb-3">
                    <label class="fw-bold d-block mb-1">{{ getLocalized(field.title) || field.name }}</label>
                    <div v-if="getLocalized(field.description)" class="form-text text-muted mb-2 mt-0">
                      {{ getLocalized(field.description) }}
                    </div>
                    <div class="form-check form-switch ps-0">
                      <input class="form-check-input ms-0" type="checkbox" role="switch" 
                             :id="'field-'+field.name" 
                             v-model="formData[field.name]">
                    </div>
                  </div>
                  
                  <!-- Number -->
                  <div v-else-if="field.type === 'int' || field.type === 'float' || field.type === 'number'" class="mb-3">
                    <label :for="'field-'+field.name" class="form-label fw-bold d-block mb-1">
                      {{ getLocalized(field.title) || field.name }}
                    </label>
                    <div v-if="getLocalized(field.description)" class="form-text text-muted mb-2 mt-0">
                      {{ getLocalized(field.description) }}
                    </div>
                    <input type="number" class="form-control" :id="'field-'+field.name" 
                           v-model.number="formData[field.name]">
                  </div>

                  <!-- Select -->
                  <div v-else-if="field.type === 'select'" class="mb-3">
                    <label :for="'field-'+field.name" class="form-label fw-bold d-block mb-1">
                      {{ getLocalized(field.title) || field.name }}
                    </label>
                    <div v-if="getLocalized(field.description)" class="form-text text-muted mb-2 mt-0">
                      {{ getLocalized(field.description) }}
                    </div>
                    <select class="form-select" :id="'field-'+field.name" v-model="formData[field.name]">
                      <option v-for="opt in field.options" :key="opt.value" :value="opt.value">
                        {{ opt.label || opt.value }}
                      </option>
                    </select>
                  </div>
                  
                  <!-- Text (Default) -->
                  <div v-else class="mb-3">
                    <label :for="'field-'+field.name" class="form-label fw-bold d-block mb-1">
                      {{ getLocalized(field.title) || field.name }}
                    </label>
                    <div v-if="getLocalized(field.description)" class="form-text text-muted mb-2 mt-0">
                      {{ getLocalized(field.description) }}
                    </div>
                    <input type="text" class="form-control" :id="'field-'+field.name" 
                           v-model="formData[field.name]">
                  </div>
                </template>
              </div>
              
              <div class="d-flex justify-content-end mt-4 pt-3 border-top">
                <button type="button" class="btn btn-light me-2" @click="goBack">{{ $t('tsl_cancel') }}</button>
                <button type="submit" class="btn btn-primary px-4" :disabled="saving">
                  <span v-if="saving" class="spinner-border spinner-border-sm me-2" role="status" aria-hidden="true"></span>
                  <i v-else class="bi bi-save me-2"></i>
                  {{ saving ? $t('saving') : $t('save_restart') }}
                </button>
              </div>
            </form>
            </template>
          </div>
        </div>
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
