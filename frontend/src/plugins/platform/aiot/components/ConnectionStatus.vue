<template>
  <div v-if="connectionStatus" class="mb-4">
    <div class="alert" :class="connectionStatus.status === 'connected' ? 'alert-success' : 'alert-warning'">
      <div class="d-flex align-items-center">
        <div class="flex-shrink-0">
          <i class="bi" :class="connectionStatus.status === 'connected' ? 'bi-cloud-check-fill' : 'bi-cloud-slash-fill'" style="font-size: 1.5rem;"></i>
        </div>
        <div class="ms-3 w-100">
          <div class="d-flex justify-content-between align-items-center">
            <h6 class="alert-heading mb-1 fw-bold">{{ $t('connection_status') }}</h6>
            <span class="badge" :class="connectionStatus.status === 'connected' ? 'bg-success' : 'bg-danger'">
              {{ connectionStatus.status === 'connected' ? $t('conn_connected') : $t('conn_disconnected') }}
            </span>
          </div>
          <div class="row g-2 mt-1 small">
            <div class="col-md-6">
              <span class="text-muted">{{ $t('conn_broker') }}:</span> 
              <span class="ms-1 fw-medium">{{ connectionStatus.broker || '-' }}</span>
            </div>
            <div class="col-md-6">
              <span class="text-muted">{{ $t('conn_gateway') }}:</span> 
              <span class="ms-1 fw-medium">{{ connectionStatus.gateway_code || '-' }}</span>
            </div>
            <div v-if="connectionStatus.ts" class="col-12 text-end text-muted mt-1" style="font-size: 0.75rem;">
              {{ $t('conn_last_check') }}: {{ new Date(connectionStatus.ts).toLocaleString() }}
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted } from 'vue';
import axios from 'axios';

const props = defineProps({
  pluginName: {
    type: String,
    required: true
  }
});

const connectionStatus = ref(null);
let statusInterval = null;

const fetchConnectionStatus = async () => {
  try {
    // Determine the target mostly for cases where case might differ, 
    // but here we assume pluginName matches or we map it if needed.
    // In original code: pluginName === 'Sagoo' ? 'sagoo' : 'aiot'
    const target = props.pluginName.toLowerCase();
    const res = await axios.get(`/api/extension/${target}/status`);
    if (res.data) {
      connectionStatus.value = res.data;
    }
  } catch (e) {
    console.error("Failed to fetch connection status", e);
  }
};

onMounted(() => {
  fetchConnectionStatus();
  statusInterval = setInterval(fetchConnectionStatus, 5000);
});

onUnmounted(() => {
  if (statusInterval) {
    clearInterval(statusInterval);
  }
});
</script>
