<template>
  <div class="settings-container">
    <h4 class="mb-4">{{ $t('sidebar_settings') }}</h4>

    <div class="card border-0 shadow-sm mb-4">
      <div class="card-header bg-transparent border-bottom-0 pt-4 pb-0">
        <h5 class="mb-0"><i class="bi bi-journal-text me-2"></i>{{ $t('log_config') }}</h5>
      </div>
      <div class="card-body">
        <div v-if="loading" class="text-center py-4">
          <div class="spinner-border text-primary" role="status">
            <span class="visually-hidden">{{ $t('loading') }}</span>
          </div>
        </div>
        <form v-else @submit.prevent="saveConfig">
          <div class="row g-3">
            <div class="col-md-6">
              <label class="form-label">{{ $t('log_level') }}</label>
              <select class="form-select" v-model="form.level">
                <option value="debug">Debug</option>
                <option value="info">Info</option>
                <option value="warn">Warn</option>
                <option value="error">Error</option>
              </select>
            </div>
            <div class="col-md-6">
              <label class="form-label">{{ $t('log_dir') }}</label>
              <input type="text" class="form-control" v-model="form.dir" placeholder="./data/logs">
            </div>
            <div class="col-md-6">
              <label class="form-label">{{ $t('log_max_days') }}</label>
              <div class="input-group">
                <input type="number" class="form-control" v-model="form.max_days" min="1">
                <span class="input-group-text">{{ $t('days') }}</span>
              </div>
            </div>
            <div class="col-md-6">
              <label class="form-label">{{ $t('log_max_size') }}</label>
              <div class="input-group">
                <input type="number" class="form-control" v-model="form.max_size" min="1">
                <span class="input-group-text">{{ $t('mb') }}</span>
              </div>
            </div>
            <div class="col-md-6">
              <label class="form-label">{{ $t('log_max_backups') }}</label>
              <input type="number" class="form-control" v-model="form.max_backups" min="1">
            </div>
            <div class="col-md-6 d-flex align-items-end">
              <div class="form-check form-switch mb-2">
                <input class="form-check-input" type="checkbox" id="compressSwitch" v-model="form.compress">
                <label class="form-check-label" for="compressSwitch">{{ $t('log_compress') }}</label>
              </div>
            </div>
          </div>
          
          <div class="mt-4 text-end">
            <button type="submit" class="btn btn-primary" :disabled="saving">
              <span v-if="saving" class="spinner-border spinner-border-sm me-2" role="status" aria-hidden="true"></span>
              <i v-else class="bi bi-save me-2"></i>{{ $t('save_config') }}
            </button>
          </div>
        </form>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue';
import axios from 'axios';
import { inject } from 'vue';
import { useI18n } from 'vue-i18n';

const { t } = useI18n();
const showToast = inject('showToast');

const loading = ref(true);
const saving = ref(false);
const form = ref({
  level: 'info',
  dir: './data/logs',
  max_days: 7,
  max_size: 50,
  max_backups: 10,
  compress: true
});

const fetchConfig = async () => {
  try {
    const res = await axios.get('/api/system/log/config');
    if (res.data.code === 0 && res.data.data) {
      form.value = { ...form.value, ...res.data.data };
    }
  } catch (e) {
    showToast('danger', t('log_config_fail') + e.message);
  } finally {
    loading.value = false;
  }
};

const saveConfig = async () => {
  saving.value = true;
  try {
    const res = await axios.post('/api/system/log/config', form.value);
    if (res.data.code === 0) {
      showToast('success', res.data.message || t('log_config_success'));
    } else {
      showToast('danger', res.data.message);
    }
  } catch (e) {
    showToast('danger', t('log_config_fail') + e.message);
  } finally {
    saving.value = false;
  }
};

onMounted(() => {
  fetchConfig();
});
</script>

<style scoped>
.settings-container {
  max-width: 800px;
  margin: 0 auto;
}
</style>
