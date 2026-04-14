<template>
  <div class="settings-container">
    <h4 class="mb-4">{{ $t('license_info', '授权信息') }}</h4>

    <div v-if="licenseData && licenseData.status" class="card border-0 shadow-sm mb-4">
      <div class="card-header bg-transparent border-bottom-0 pt-4 pb-0 d-flex justify-content-between align-items-center">
        <h5 class="mb-0"><i class="bi bi-shield-check me-2"></i>{{ $t('license_info', '授权信息') }}</h5>
        <div>
          <span class="badge me-2" :class="licenseData.status === 'authorized' ? 'bg-success' : 'bg-danger'">
            {{ licenseData.status === 'authorized' ? $t('license_authorized', '已授权') : $t('license_unauthorized', '未授权') }}
          </span>
          <button class="btn btn-sm btn-outline-primary" @click="$refs.licenseInput.click()">
            <i class="bi bi-upload"></i> {{ $t('license_update', '更新许可证') }}
          </button>
          <input type="file" ref="licenseInput" class="d-none" accept=".lic" @change="handleLicenseUpload">
        </div>
      </div>
      <div class="card-body">
        <div class="row g-3">
          <div class="col-md-6">
            <label class="form-label text-muted small mb-1">{{ $t('machine_id', '机器码') }}</label>
            <div class="input-group input-group-sm">
              <input type="text" class="form-control" :value="licenseData.machine_id" readonly>
              <button class="btn btn-outline-secondary" type="button" @click="copyToClipboard(licenseData.machine_id)">
                <i class="bi bi-clipboard"></i>
              </button>
            </div>
          </div>
          <div class="col-md-6">
            <label class="form-label text-muted small mb-1">{{ $t('license_type', '授权类型') }}</label>
            <div class="fw-bold">{{ licenseData.type === 'trial' ? $t('license_trial', '试用版') : (licenseData.type === 'authorized' ? $t('license_pro', '正式版') : (licenseData.type === 'permanent' ? $t('license_permanent', '永久授权') : '-')) }}</div>
          </div>
          <div class="col-md-6">
            <label class="form-label text-muted small mb-1">{{ $t('license_start_time', '授权开始时间') }}</label>
            <div>{{ licenseData.start_time || '-' }}</div>
          </div>
          <div class="col-md-6">
            <label class="form-label text-muted small mb-1">{{ $t('license_expire_time', '授权到期时间') }}</label>
            <div>{{ licenseData.expire_time || '-' }}</div>
          </div>
          <div class="col-12 mt-3" v-if="licenseData.status !== 'authorized'">
            <div class="alert alert-warning py-2 mb-0">
              <i class="bi bi-exclamation-triangle me-2"></i> {{ licenseData.message }}
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue';
import { useI18n } from 'vue-i18n';
import axios from 'axios';
import { useToast } from '../composables/useToast';

const { t } = useI18n();
const { showToast } = useToast();

const licenseData = ref(null);
const licenseInput = ref(null);

const fetchLicenseData = async () => {
  try {
    const res = await axios.get('/api/extension/license/status');
    if (res.data.code === 200) {
      licenseData.value = res.data.data;
    }
  } catch (e) {
    // API not found (e.g. open source version)
  }
};

const copyToClipboard = async (text) => {
  try {
    await navigator.clipboard.writeText(text);
    showToast('success', t('copied_to_clipboard', '已复制到剪贴板'));  
  } catch (err) {
    showToast('danger', t('copy_failed', '复制失败'));
  }
};

const handleLicenseUpload = async (event) => {
  const file = event.target.files[0];
  if (!file) return;

  const formData = new FormData();
  formData.append('file', file);

  try {
    const res = await axios.post('/api/extension/license/upload', formData, {
      headers: {
        'Content-Type': 'multipart/form-data'
      }
    });

    if (res.data.code === 200) {
      showToast('success', res.data.message || t('license_upload_success', '许可证验证成功'));
      await fetchLicenseData();
      window.location.reload(); // Refresh the page as requested
    } else {
      showToast('danger', res.data.message || t('license_upload_fail', '许可证验证失败'));
    }
  } catch (e) {
    const msg = e.response?.data?.message || e.message;
    showToast('danger', t('license_upload_fail', '许可证验证失败') + ': ' + msg);
  } finally {
    if (licenseInput.value) {
      licenseInput.value.value = '';
    }
  }
};

onMounted(() => {
  fetchLicenseData();
});
</script>