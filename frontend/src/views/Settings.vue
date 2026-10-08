<template>
  <div class="settings-page page-fixed-height">
    <div class="page-header mb-4">
      <div>
        <h1>{{ $t('sidebar_settings') }}</h1>
        <p class="page-subtitle">{{ $t('settings_subtitle', '配置网关系统服务端口、时序数据库与核心运行参数') }}</p>
      </div>
      <div>
        <LiquidGlassButton
          variant="primary"
          :icon="saving ? 'bi bi-arrow-repeat spin' : 'bi bi-check2'"
          :disabled="saving || loading"
          @click="saveConfig"
        >
          {{ saving ? $t('saving') : $t('save') }}
        </LiquidGlassButton>
      </div>
    </div>

    <div class="settings-content">
    <div v-if="loading" class="text-center py-4">
      <div class="spinner-border text-primary" role="status">
        <span class="visually-hidden">{{ $t('loading') }}</span>
      </div>
    </div>
    <form v-else @submit.prevent="saveConfig" style="max-width: 960px;">
      <!-- Server Config Card -->
      <div class="card border-0 shadow-sm table-glass-card mb-4">
        <div class="card-header bg-transparent border-bottom-0 pt-4 pb-0">
          <h5 class="mb-0"><i class="bi bi-hdd-network me-2"></i>{{ $t('server_config') || 'Server Configuration' }}</h5>
        </div>
        <div class="card-body">
          <div class="row g-3">
            <div class="col-md-6">
              <label class="form-label">{{ $t('server_port') || 'Port' }}</label>
              <input type="number" class="form-control" v-model.number="form.server.port" min="1" max="65535">
              <div class="form-text">{{ $t('server_port_hint') || 'Requires restart to take effect.' }}</div>
            </div>
          </div>
        </div>
      </div>

      <!-- TSDB Config Card -->
      <div class="card border-0 shadow-sm table-glass-card mb-4">
        <div class="card-header bg-transparent border-bottom-0 pt-4 pb-0">
          <h5 class="mb-0 fw-bold"><i class="bi bi-database me-2"></i>{{ $t('tsdb_config') || 'TSDB Configuration' }}</h5>
        </div>
        <div class="card-body">
          <div class="row g-3">
            <div class="col-md-12 d-flex align-items-center mb-2">
              <div class="form-check form-switch">
                <input class="form-check-input" type="checkbox" id="tsdbEnabledSwitch" v-model="form.tsdb.enabled">
                <label class="form-check-label" for="tsdbEnabledSwitch">{{ $t('tsdb_enabled') || 'Enable TSDB' }}</label>
              </div>
            </div>
            <div class="col-md-6">
              <label class="form-label">{{ $t('tsdb_dir') || 'Directory' }}</label>
              <input type="text" class="form-control" v-model="form.tsdb.dir" placeholder="./data/db/history">
            </div>
            <div class="col-md-6">
              <label class="form-label">{{ $t('tsdb_retention_days') || 'Retention Days' }}</label>
              <div class="input-group">
                <input type="number" class="form-control" v-model.number="form.tsdb.retention_days" min="1">
                <span class="input-group-text">{{ $t('days') }}</span>
              </div>
            </div>
            <div class="col-md-6">
              <label class="form-label">{{ $t('tsdb_batch_size') || 'Batch Size' }}</label>
              <input type="number" class="form-control" v-model.number="form.tsdb.batch_size" min="1">
            </div>
            <div class="col-md-6">
              <label class="form-label">{{ $t('tsdb_flush_interval') || 'Flush Interval (ms)' }}</label>
              <div class="input-group">
                <input type="number" class="form-control" v-model.number="form.tsdb.flush_interval_ms" min="100">
                <span class="input-group-text">ms</span>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- Alarm Recording Config Card -->
      <div class="card border-0 shadow-sm table-glass-card mb-4">
        <div class="card-header bg-transparent border-bottom-0 pt-4 pb-0">
          <h5 class="mb-0 fw-bold"><i class="bi bi-camera-video me-2" aria-hidden="true"></i>{{ $t('alarm_media_config') }}</h5>
        </div>
        <div class="card-body">
          <div class="row g-3">
            <div class="col-md-6">
              <label class="form-label" for="alarmMediaMaxStorage">{{ $t('alarm_media_max_storage') }}</label>
              <div class="input-group">
                <input id="alarmMediaMaxStorage" type="number" class="form-control" v-model.number="form.alarm_media.max_storage_gib" min="1" max="1048576" step="1" required :disabled="saving" aria-describedby="alarmMediaMaxStorageHint">
                <span class="input-group-text">GiB</span>
              </div>
              <div id="alarmMediaMaxStorageHint" class="form-text">{{ $t('alarm_media_max_storage_hint') }}</div>
            </div>
            <div class="col-md-6">
              <label class="form-label" for="alarmMediaRetention">{{ $t('alarm_media_retention_days') }}</label>
              <div class="input-group">
                <input id="alarmMediaRetention" type="number" class="form-control" v-model.number="form.alarm_media.retention_days" min="1" max="36500" step="1" required :disabled="saving" aria-describedby="alarmMediaRetentionHint">
                <span class="input-group-text">{{ $t('days') }}</span>
              </div>
              <div id="alarmMediaRetentionHint" class="form-text">{{ $t('alarm_media_retention_hint') }}</div>
            </div>
          </div>
          <p class="form-text mt-4 mb-0">{{ $t('alarm_media_overwrite_hint') }}</p>
        </div>
      </div>

      <!-- Log Config Card -->
      <div class="card border-0 shadow-sm table-glass-card mb-4">
        <div class="card-header bg-transparent border-bottom-0 pt-4 pb-0">
          <h5 class="mb-0 fw-bold"><i class="bi bi-journal-text me-2"></i>{{ $t('log_config') }}</h5>
        </div>
        <div class="card-body">
          <div class="row g-3">
            <div class="col-md-6">
              <label class="form-label">{{ $t('log_level') }}</label>
              <select class="form-select" v-model="form.log.level">
                <option value="debug">Debug</option>
                <option value="info">Info</option>
                <option value="warn">Warn</option>
                <option value="error">Error</option>
              </select>
            </div>
            <div class="col-md-6">
              <label class="form-label">{{ $t('log_dir') }}</label>
              <input type="text" class="form-control" v-model="form.log.dir" placeholder="./data/logs">
            </div>
            <div class="col-md-6">
              <label class="form-label">{{ $t('log_max_days') }}</label>
              <div class="input-group">
                <input type="number" class="form-control" v-model.number="form.log.max_days" min="1">
                <span class="input-group-text">{{ $t('days') }}</span>
              </div>
            </div>
            <div class="col-md-6">
              <label class="form-label">{{ $t('log_max_size') }}</label>
              <div class="input-group">
                <input type="number" class="form-control" v-model.number="form.log.max_size" min="1">
                <span class="input-group-text">{{ $t('mb') }}</span>
              </div>
            </div>
            <div class="col-md-6">
              <label class="form-label">{{ $t('log_max_backups') }}</label>
              <input type="number" class="form-control" v-model.number="form.log.max_backups" min="1">
            </div>
            <div class="col-md-6 d-flex align-items-end">
              <div class="form-check form-switch mb-2">
                <input class="form-check-input" type="checkbox" id="compressSwitch" v-model="form.log.compress">
                <label class="form-check-label" for="compressSwitch">{{ $t('log_compress') }}</label>
              </div>
            </div>
          </div>
          
          <div class="mt-4 text-end">
            <button type="submit" class="btn btn-primary" :disabled="saving">
              <span v-if="saving" class="spinner-border spinner-border-sm me-2" role="status" aria-hidden="true"></span>
              <i v-else class="bi bi-save me-2"></i>{{ $t('save_config') || 'Save Config' }}
            </button>
          </div>
        </div>
      </div>
    </form>
    <component
      v-for="(panel, index) in settingsPanels"
      :key="`settings-panel-${index}`"
      :is="panel.component"
      :remote-api-base="props.remoteApiBase"
    />
    </div>
  </div>
</template>

<script setup>
import { computed, ref, onMounted } from 'vue';
import axios from 'axios';
import { inject } from 'vue';
import { useI18n } from 'vue-i18n';
import { useAuthStore } from '../stores/auth';
import { usePlugins } from '../plugins/registry.js';
import { useConfirm } from '../composables/useConfirm.js';
import { resolveApiErrorMessage } from '../utils/apiMessages.js';
const auth = useAuthStore();
const { extensions } = usePlugins();

const props = defineProps({
  remoteApiBase: {
    type: String,
    default: '/api/system'
  }
});

const { t, locale } = useI18n();
const { confirmDialog } = useConfirm();
const showToast = inject('showToast');
const settingsPanels = computed(() => (extensions.value.settingsPanels || []).filter((panel) => {
  return !panel.condition || panel.condition({ remoteApiBase: props.remoteApiBase, isSystemAdmin: auth.isSystemAdmin });
}));

const loading = ref(true);
const saving = ref(false);
const form = ref({
  server: { port: 8989 },
  alarm_media: {
    max_storage_gib: 10,
    retention_days: 30
  },
  tsdb: {
    enabled: true,
    dir: './data/db/history',
    retention_days: 90,
    batch_size: 100,
    flush_interval_ms: 1000
  },
  log: {
    level: 'info',
    dir: './data/logs',
    max_days: 7,
    max_size: 50,
    max_backups: 10,
    compress: true
  }
});
const savedAlarmMedia = ref({ ...form.value.alarm_media });

const fetchConfig = async () => {
  try {
    const res = await axios.get(`${props.remoteApiBase}/config`);
    if (res.data.code === 0 && res.data.data) {
      form.value = {
        ...form.value,
        ...res.data.data,
        alarm_media: { ...form.value.alarm_media, ...res.data.data.alarm_media }
      };
      savedAlarmMedia.value = { ...form.value.alarm_media };
    } else {
      showToast('danger', resolveApiErrorMessage(res.data, locale.value, t('log_config_load_fail')));
    }
  } catch (e) {
    showToast('danger', resolveApiErrorMessage(e, locale.value, t('log_config_load_fail')));
  } finally {
    loading.value = false;
  }
};

const saveConfig = async () => {
  if (saving.value || loading.value) return;
  const { max_storage_gib, retention_days } = form.value.alarm_media;
  if (!Number.isInteger(max_storage_gib) || max_storage_gib < 1 || max_storage_gib > 1048576) {
    showToast('danger', t('alarm_media_capacity_invalid'));
    return;
  }
  if (!Number.isInteger(retention_days) || retention_days < 1 || retention_days > 36500) {
    showToast('danger', t('alarm_media_retention_invalid'));
    return;
  }
  saving.value = true;
  try {
    const config = JSON.parse(JSON.stringify(form.value));
    if (max_storage_gib < savedAlarmMedia.value.max_storage_gib || retention_days < savedAlarmMedia.value.retention_days) {
      const confirmed = await confirmDialog({
        title: t('alarm_media_reduce_title'),
        message: t('alarm_media_reduce_confirm', {
          oldCapacity: savedAlarmMedia.value.max_storage_gib,
          oldDays: savedAlarmMedia.value.retention_days,
          capacity: max_storage_gib,
          days: retention_days
        }),
        variant: 'danger',
        confirmText: t('save'),
        cancelText: t('common_cancel')
      });
      if (!confirmed) return;
    }
    const oldPort = window.location.port || (window.location.protocol === 'https:' ? '443' : '80');
    const newPort = String(config.server.port);
    const portChanged = oldPort !== newPort;

    const res = await axios.post(`${props.remoteApiBase}/config`, config);
    if (res.data.code === 0) {
      savedAlarmMedia.value = { ...config.alarm_media };
      if (portChanged) {
        showToast('success', `${res.data.message || t('log_config_success')} 端口已变更，后台服务重启中，将在3秒后自动跳转...`);
        setTimeout(() => {
          window.location.href = `${window.location.protocol}//${window.location.hostname}:${newPort}/login`;
        }, 3000);
      } else {
        showToast('success', res.data.message || t('log_config_success'));
      }
    } else {
      showToast('danger', resolveApiErrorMessage(res.data, locale.value, t('log_config_fail')));
    }
  } catch (e) {
    showToast('danger', resolveApiErrorMessage(e, locale.value, t('log_config_fail')));
  } finally {
    saving.value = false;
  }
};

onMounted(() => {
  fetchConfig();
});
</script>

<style scoped>
.settings-content {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding-bottom: 24px;
}
.settings-container {
  max-width: 800px;
  margin: 0 auto;
}
</style>
