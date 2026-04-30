<template>
  <div class="video-square-page d-flex flex-column">
    <div class="video-square-hero card border-0 mb-2 overflow-hidden">
      <div class="card-body p-2 p-lg-3">
        <div class="d-flex flex-column flex-xl-row justify-content-between gap-3">
          <div>
            <div class="d-flex align-items-center mb-2">
              <div class="hero-icon me-3">
                <i class="bi bi-grid-3x3-gap-fill"></i>
              </div>
              <div>
                <h5 class="mb-0 fw-bold">
                  {{ $t('page_video_square', '视频广场') }}
                </h5>
              </div>
            </div>
            <div class="d-flex flex-wrap gap-2">
              <span class="info-pill">
                <i class="bi bi-camera-video me-1"></i> 设备 {{ filteredDevices.length }}
              </span>
              <span class="info-pill">
                <i class="bi bi-broadcast-pin me-1 text-success"></i> 在线 {{ onlineDeviceCount }}
              </span>
              <span class="info-pill">
                <i class="bi bi-grid me-1 text-primary"></i> 在播 {{ occupiedCount }}/{{ gridSize }}
              </span>
            </div>
          </div>

          <div class="d-flex flex-column align-items-stretch align-items-xl-end gap-2">
            <div class="toolbar-panel">
              <div class="input-group input-group-sm toolbar-search">
                <span class="input-group-text border-0 bg-transparent">
                  <i class="bi bi-search text-secondary"></i>
                </span>
                <input
                  v-model.trim="searchKeyword"
                  type="text"
                  class="form-control border-0 shadow-none"
                  placeholder="搜索设备名称或编码"
                >
              </div>
              <button
                type="button"
                class="btn btn-sm"
                :class="onlineOnly ? 'btn-success' : 'btn-outline-secondary'"
                @click="onlineOnly = !onlineOnly"
              >
                <i class="bi bi-wifi me-1"></i>{{ onlineOnly ? '仅在线' : '全部设备' }}
              </button>
            </div>

            <div class="d-flex flex-wrap gap-2 justify-content-xl-end">
              <div class="btn-group btn-group-sm grid-switcher" role="group">
                <input type="radio" class="btn-check" name="grid-layout" id="grid-1" :value="1" v-model="gridSize">
                <label class="btn btn-outline-secondary" for="grid-1"><i class="bi bi-square"></i> 1宫格</label>

                <input type="radio" class="btn-check" name="grid-layout" id="grid-4" :value="4" v-model="gridSize">
                <label class="btn btn-outline-secondary" for="grid-4"><i class="bi bi-grid"></i> 4宫格</label>

                <input type="radio" class="btn-check" name="grid-layout" id="grid-9" :value="9" v-model="gridSize">
                <label class="btn btn-outline-secondary" for="grid-9"><i class="bi bi-grid-3x3"></i> 9宫格</label>
              </div>

              <button type="button" class="btn btn-sm btn-outline-primary" @click="fillWithOnlineDevices">
                <i class="bi bi-magic me-1"></i>一键上屏
              </button>
              <button type="button" class="btn btn-sm btn-outline-secondary" @click="clearGrid">
                <i class="bi bi-x-circle me-1"></i>清空宫格
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>

    <div class="row flex-grow-1 g-3 video-square-content">
      <div class="col-xl-3 col-xxl-2 d-flex flex-column">
        <div class="card device-panel border-0 shadow-sm flex-grow-1 overflow-hidden">
          <div class="card-header border-0 pb-0">
            <div class="d-flex justify-content-between align-items-center">
              <div>
                <h6 class="mb-1 fw-bold">视频设备列表</h6>
                <div class="text-secondary small">点击设备即可加入当前宫格</div>
              </div>
              <span class="badge text-bg-light">{{ filteredDevices.length }}</span>
            </div>
          </div>
          <div class="card-body pt-3 px-2 px-lg-3 overflow-auto">
            <div v-if="loading" class="text-center py-5 text-muted small">
              <div class="spinner-border spinner-border-sm me-1"></div> 加载中...
            </div>
            <div v-else-if="devices.length === 0" class="empty-state text-center py-5 text-muted small">
              <i class="bi bi-camera-video-off d-block fs-2 mb-2"></i>
              暂无视频设备
            </div>
            <div v-else-if="filteredDevices.length === 0" class="empty-state text-center py-5 text-muted small">
              <i class="bi bi-search d-block fs-2 mb-2"></i>
              未找到匹配设备
            </div>
            <div v-else class="d-flex flex-column gap-2">
              <button
                v-for="device in filteredDevices"
                :key="device.code"
                type="button"
                class="device-item text-start"
                :class="{ active: isAssigned(device.code) }"
                @click="assignDeviceToGrid(device)"
              >
                <div class="d-flex align-items-center gap-3">
                  <div class="device-icon" :class="device.online ? 'online' : 'offline'">
                    <i class="bi" :class="device.online ? 'bi-camera-video-fill' : 'bi-camera-video'"></i>
                  </div>
                  <div class="flex-grow-1" style="min-width: 0;">
                    <div class="small fw-semibold text-truncate" :title="device.name || device.code">{{ device.name || device.code }}</div>
                    <div class="text-secondary device-code text-truncate" :title="device.code">{{ device.code }}</div>
                  </div>
                  <div class="text-end flex-shrink-0" style="min-width: 40px;">
                    <div class="small fw-semibold" :class="device.online ? 'text-success' : 'text-secondary'">
                      {{ device.online ? '在线' : '离线' }}
                    </div>
                    <div v-if="isAssigned(device.code)" class="device-tag">已上屏</div>
                  </div>
                </div>
              </button>
            </div>
          </div>
        </div>
      </div>

      <div class="col-xl-9 col-xxl-10 d-flex flex-column">
        <div class="card video-panel border-0 shadow-sm flex-grow-1">
          <div class="card-header border-0">
            <div class="d-flex flex-column flex-lg-row justify-content-between gap-2 align-items-lg-center">
              <div>
                <h6 class="mb-1 fw-bold">实时监看区域</h6>
                <div class="text-secondary small">当前布局 {{ gridSize }} 宫格，支持快速替换与关闭单路画面</div>
              </div>
              <div class="d-flex flex-wrap gap-2">
                <span class="grid-hint"><i class="bi bi-arrows-fullscreen me-1"></i>建议优先播放在线设备</span>
                <span class="grid-hint"><i class="bi bi-info-circle me-1"></i>如无画面将显示明确状态</span>
              </div>
            </div>
          </div>
          <div class="card-body p-2 p-lg-3 d-flex flex-column">
            <div class="video-grid-panel flex-grow-1">
              <div class="video-grid-container h-100 w-100" :class="`grid-${gridSize}`">
                <div v-for="index in gridSize" :key="index" class="video-grid-cell">
                  <div class="slot-label">窗口 {{ index }}</div>
                  <GB28181PlayerWidget 
                    :device="gridDevices[index - 1]" 
                    @close="removeDeviceFromGrid(index - 1)" 
                  />
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted, watch } from 'vue';
import axios from 'axios';
import { useI18n } from 'vue-i18n';
import GB28181PlayerWidget from '@/plugins/pro/protocol/gb28181/GB28181PlayerWidget.vue';

const { t } = useI18n();

const loading = ref(false);
const devices = ref([]);
const gridSize = ref(4); // 1, 4, 9
const gridDevices = ref(Array(9).fill(null));
const searchKeyword = ref('');
const onlineOnly = ref(false);

const filteredDevices = computed(() => {
  const keyword = searchKeyword.value.trim().toLowerCase();
  return devices.value.filter(device => {
    if (onlineOnly.value && !device.online) return false;
    if (!keyword) return true;
    const name = (device.name || '').toLowerCase();
    const code = (device.code || '').toLowerCase();
    return name.includes(keyword) || code.includes(keyword);
  });
});

const onlineDeviceCount = computed(() => devices.value.filter(device => device.online).length);
const occupiedCount = computed(() => gridDevices.value.slice(0, gridSize.value).filter(Boolean).length);

const fetchVideoDevices = async (isUpdate = false) => {
  if (!isUpdate) {
    loading.value = true;
  }
  try {
    const productsRes = await axios.get('/api/products');
    const allProducts = productsRes.data?.data || [];
    
    // Find all products with protocol gb28181
    const videoProducts = allProducts.filter(p => p.protocol_name === 'gb28181');
    const videoProductCodes = new Set(videoProducts.map(p => p.code));

    const devicesRes = await axios.get('/api/devices', { params: { page: 0, _t: Date.now() } });
    const allDevices = devicesRes.data?.data || [];
    
    devices.value = allDevices.filter(d => videoProductCodes.has(d.product_code));
  } catch (e) {
    console.error("Failed to fetch video devices", e);
  } finally {
    if (!isUpdate) {
      loading.value = false;
    }
  }
};

let eventSource = null;
let sseHeartbeatTimer = null;
let sseReconnectTimer = null;

const resetSSEHeartbeat = () => {
  if (sseHeartbeatTimer) clearTimeout(sseHeartbeatTimer);
  sseHeartbeatTimer = setTimeout(() => {
    console.warn('[SSE] Heartbeat timeout, reconnecting...');
    reconnectSSE();
  }, 45000);
};

const setupSSE = () => {
  if (eventSource) {
    eventSource.close();
    eventSource = null;
  }
  if (sseReconnectTimer) {
    clearTimeout(sseReconnectTimer);
    sseReconnectTimer = null;
  }

  eventSource = new EventSource('/api/devices/stream');

  eventSource.addEventListener('connected', () => {
    console.log('[SSE] Connected to device stream');
    resetSSEHeartbeat();
  });

  eventSource.addEventListener('heartbeat', () => {
    resetSSEHeartbeat();
  });

  eventSource.addEventListener('device.status.changed', (event) => {
    resetSSEHeartbeat();
    try {
      const data = JSON.parse(event.data);
      const deviceCode = data.Topic || data.topic;
      const payload = data.Payload || data.payload;
      
      if (deviceCode && payload) {
        const isOnline = payload === 'online';
        const device = devices.value.find(d => d.code === deviceCode);
        if (device) {
          device.online = isOnline;
          // Synchronize grid devices if assigned
          const gridDev = gridDevices.value.find(d => d && d.code === deviceCode);
          if (gridDev) {
            gridDev.online = isOnline;
          }
        }
      } else {
        fetchVideoDevices(true);
      }
    } catch (e) {
      fetchVideoDevices(true);
    }
  });

  eventSource.addEventListener('device.list.changed', () => {
    resetSSEHeartbeat();
    fetchVideoDevices(true);
  });

  eventSource.onerror = (err) => {
    console.warn('[SSE] Connection error', err);
    if (eventSource && eventSource.readyState === EventSource.CLOSED) {
      sseReconnectTimer = setTimeout(() => {
        setupSSE();
      }, 3000);
    } else {
      resetSSEHeartbeat();
    }
  };

  eventSource.onopen = () => {
    console.log('[SSE] Connection opened');
    resetSSEHeartbeat();
    fetchVideoDevices(true);
  };
};

const reconnectSSE = () => {
  if (eventSource) {
    eventSource.close();
    eventSource = null;
  }
  fetchVideoDevices(true);
  sseReconnectTimer = setTimeout(() => {
    setupSSE();
  }, 1000);
};

onMounted(() => {
  fetchVideoDevices();
  setupSSE();
});

onUnmounted(() => {
  if (sseHeartbeatTimer) clearTimeout(sseHeartbeatTimer);
  if (sseReconnectTimer) clearTimeout(sseReconnectTimer);
  if (eventSource) {
    eventSource.close();
    eventSource = null;
  }
});

const assignDeviceToGrid = (device) => {
  // Check if device is already in grid
  const existingIndex = gridDevices.value.findIndex(d => d && d.code === device.code);
  if (existingIndex !== -1) {
    if (existingIndex < gridSize.value) {
      return;
    }
    gridDevices.value[existingIndex] = null;
  }
  
  const visibleDevices = gridDevices.value.slice(0, gridSize.value).filter(Boolean);
  if (visibleDevices.length >= gridSize.value) {
    gridDevices.value[0] = device;
    return;
  }
  
  // Find first empty slot
  const emptyIndex = gridDevices.value.findIndex((d, idx) => !d && idx < gridSize.value);
  if (emptyIndex !== -1) {
    gridDevices.value[emptyIndex] = device;
  } else {
    // If full, replace the first one
    gridDevices.value[0] = device;
  }
};

const removeDeviceFromGrid = (index) => {
  gridDevices.value[index] = null;
};

const clearGrid = () => {
  for (let i = 0; i < 9; i++) {
    gridDevices.value[i] = null;
  }
};

const fillWithOnlineDevices = () => {
  const source = filteredDevices.value.filter(device => device.online);
  if (source.length === 0) return;

  const nextGrid = Array(9).fill(null);
  source.slice(0, gridSize.value).forEach((device, index) => {
    nextGrid[index] = device;
  });
  gridDevices.value = nextGrid;
};

const isAssigned = (deviceCode) => {
  return gridDevices.value.slice(0, gridSize.value).some(device => device && device.code === deviceCode);
};

// Clear out-of-bounds devices when grid size shrinks
watch(gridSize, (newSize) => {
  for (let i = newSize; i < 9; i++) {
    gridDevices.value[i] = null;
  }
});
</script>

<style scoped>
.video-square-page {
  min-height: calc(100vh - 8.5rem);
}

.video-square-content {
  min-height: calc(100vh - 16rem);
}

.video-square-hero {
  background:
    radial-gradient(circle at top right, rgba(13, 110, 253, 0.18), transparent 32%),
    linear-gradient(135deg, rgba(13, 17, 23, 0.98), rgba(21, 29, 40, 0.94));
  color: #fff;
}

.hero-icon {
  width: 40px;
  height: 40px;
  border-radius: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 1.15rem;
  color: #fff;
  background: linear-gradient(135deg, rgba(13, 110, 253, 0.9), rgba(55, 125, 255, 0.55));
  box-shadow: 0 4px 12px rgba(13, 110, 253, 0.24);
}

.info-pill {
  display: inline-flex;
  align-items: center;
  padding: 0.45rem 0.8rem;
  border-radius: 999px;
  font-size: 0.82rem;
  color: rgba(255, 255, 255, 0.9);
  background: rgba(255, 255, 255, 0.08);
  border: 1px solid rgba(255, 255, 255, 0.08);
}

.toolbar-panel {
  display: flex;
  gap: 0.75rem;
  align-items: center;
  flex-wrap: wrap;
}

.toolbar-search {
  min-width: min(100%, 300px);
  background: rgba(255, 255, 255, 0.08);
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 999px;
  overflow: hidden;
}

.toolbar-search .form-control,
.toolbar-search .input-group-text {
  color: #fff;
}

.toolbar-search .form-control::placeholder {
  color: rgba(255, 255, 255, 0.55);
}

.device-panel,
.video-panel {
  background: var(--bg-surface);
}

.video-panel {
  min-height: calc(100vh - 16rem);
}

.video-panel .card-body {
  min-height: 0;
}

.device-item {
  width: 100%;
  border: 1px solid var(--border-color);
  border-radius: 16px;
  padding: 0.9rem 1rem;
  background: linear-gradient(180deg, rgba(255, 255, 255, 0.02), transparent);
  transition: all 0.2s ease;
}

.device-item:hover {
  transform: translateY(-1px);
  border-color: rgba(13, 110, 253, 0.3);
  box-shadow: 0 8px 24px rgba(15, 23, 42, 0.08);
}

.device-item.active {
  border-color: rgba(13, 110, 253, 0.45);
  background: rgba(13, 110, 253, 0.08);
}

.device-icon {
  width: 44px;
  height: 44px;
  border-radius: 14px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 1.05rem;
  flex-shrink: 0;
}

.device-icon.online {
  color: #20c997;
  background: rgba(32, 201, 151, 0.12);
}

.device-icon.offline {
  color: #6c757d;
  background: rgba(108, 117, 125, 0.12);
}

.device-code {
  font-size: 0.72rem;
}

.device-tag {
  margin-top: 0.2rem;
  font-size: 0.68rem;
  color: #0d6efd;
}

.empty-state {
  border: 1px dashed var(--border-color);
  border-radius: 16px;
  background: rgba(148, 163, 184, 0.05);
}

.grid-hint {
  display: inline-flex;
  align-items: center;
  padding: 0.35rem 0.7rem;
  border-radius: 999px;
  font-size: 0.78rem;
  color: var(--text-secondary);
  background: rgba(148, 163, 184, 0.08);
}

.video-grid-panel {
  min-height: calc(100vh - 19rem);
  height: 100%;
  display: flex;
  background: linear-gradient(180deg, #0b1017, #06080d);
  border-radius: 18px;
  padding: 0.4rem;
}

.video-grid-container {
  display: grid;
  flex: 1 1 auto;
  min-height: 0;
  gap: 0.75rem;
  background-color: transparent;
}

.grid-1 {
  grid-template-columns: 1fr;
  grid-template-rows: 1fr;
}

.grid-4 {
  grid-template-columns: 1fr 1fr;
  grid-template-rows: 1fr 1fr;
}

.grid-9 {
  grid-template-columns: 1fr 1fr 1fr;
  grid-template-rows: 1fr 1fr 1fr;
}

.video-grid-cell {
  height: 100%;
  min-height: 0;
  min-width: 0;
  position: relative;
  border-radius: 16px;
  overflow: hidden;
  background: rgba(255, 255, 255, 0.02);
  border: 1px solid rgba(255, 255, 255, 0.06);
}

.slot-label {
  position: absolute;
  top: 0.65rem;
  left: 0.75rem;
  z-index: 8;
  padding: 0.18rem 0.55rem;
  border-radius: 999px;
  font-size: 0.7rem;
  color: rgba(255, 255, 255, 0.7);
  background: rgba(15, 23, 42, 0.45);
  backdrop-filter: blur(6px);
}

@media (max-width: 1199.98px) {
  .video-square-page {
    min-height: auto;
  }

  .video-square-content {
    min-height: auto;
  }

  .video-panel {
    min-height: auto;
  }

  .video-grid-panel {
    min-height: 65vh;
  }
}

@media (max-width: 767.98px) {
  .toolbar-panel {
    width: 100%;
  }

  .toolbar-search {
    min-width: 100%;
  }

  .grid-switcher {
    width: 100%;
  }

  .grid-switcher .btn {
    flex: 1 1 auto;
  }

  .video-grid-panel {
    min-height: 58vh;
  }
}
</style>
