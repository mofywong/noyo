<template>
  <div class="alarm-center container-fluid py-4">
    <div class="d-flex justify-content-between align-items-center mb-4">
      <h2>{{ $t('sidebar_alarms', '告警中心') }}</h2>
      <button class="btn btn-outline-primary btn-sm" @click="fetchEvents" :disabled="loading">
        <span v-if="loading" class="spinner-border spinner-border-sm me-1" role="status" aria-hidden="true"></span>
        <i v-else class="bi bi-arrow-clockwise me-1"></i>刷新
      </button>
    </div>

    <div class="card mb-4 border-0 shadow-sm">
      <div class="card-body p-0">
        <div class="table-responsive">
          <table class="table table-hover mb-0 align-middle">
            <thead class="table-light">
              <tr>
                <th style="min-width: 150px">事件时间</th>
                <th>设备</th>
                <th>事件名称</th>
                <th>类型</th>
                <th style="min-width: 250px">参数</th>
                <th class="text-center">抓拍图片</th>
              </tr>
            </thead>
            <tbody>
              <tr v-if="loading && events.length === 0">
                <td colspan="6" class="text-center py-5 text-muted">
                  <div class="spinner-border spinner-border-sm me-2" role="status"></div>加载中...
                </td>
              </tr>
              <tr v-else-if="events.length === 0">
                <td colspan="6" class="text-center py-5 text-muted">暂无告警事件</td>
              </tr>
              <tr v-for="evt in events" :key="evt.ts + evt.event_id" v-else>
                <td>{{ formatTime(evt.ts) }}</td>
                <td>
                  <div class="fw-medium">{{ getDeviceName(evt.device_code) }}</div>
                  <div class="small text-muted">{{ evt.device_code }}</div>
                </td>
                <td>
                  <span class="badge rounded-pill fw-normal px-2 py-1 alarm-badge" :class="getEventTypeColor(evt)">
                    <i class="bi bi-record-circle-fill me-1 small-icon"></i>
                    {{ getEventName(evt) }}
                  </span>
                </td>
                <td>
                  <span class="badge rounded-pill fw-normal px-2 py-1 alarm-badge" :class="getEventTypeColor(evt)">
                    {{ getEventTypeLabel(evt) }}
                  </span>
                </td>
                <td style="max-width: 300px">
                  <div class="text-truncate text-muted small" :title="JSON.stringify(evt.params)">
                    {{ formatParams(evt.params) }}
                  </div>
                </td>
                <td class="text-center">
                  <div v-if="evt.params?.snapshot_url" 
                       class="position-relative d-inline-block" 
                       style="cursor: pointer;"
                       @click="previewImage(evt.params.snapshot_url)">
                    <img :src="evt.params.snapshot_url" 
                         class="img-thumbnail rounded shadow-sm" 
                         style="max-height: 80px;" 
                         alt="snapshot" />
                    <div class="position-absolute bottom-0 end-0 bg-dark text-white rounded-circle d-flex align-items-center justify-content-center m-1" style="width:20px;height:20px;opacity:0.8;">
                      <i class="bi bi-zoom-in" style="font-size:10px;"></i>
                    </div>
                  </div>
                  <span v-else class="text-muted small">-</span>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </div>
    
    <!-- Paginator -->
    <div class="d-flex justify-content-between align-items-center" v-if="total > 0">
      <div class="text-muted small">共 {{ total }} 条记录</div>
      <div class="btn-group">
        <button class="btn btn-outline-secondary btn-sm" :disabled="page <= 1" @click="page--; fetchEvents()">上一页</button>
        <button class="btn btn-outline-secondary btn-sm" :disabled="page * pageSize >= total" @click="page++; fetchEvents()">下一页</button>
      </div>
    </div>

    <!-- Image Preview Modal -->
    <div v-if="previewUrl" class="modal fade show d-block" style="background: rgba(0,0,0,0.8)" @click.self="previewUrl = null" tabindex="-1">
      <div class="modal-dialog modal-xl modal-dialog-centered">
        <div class="modal-content bg-transparent border-0">
          <div class="modal-header border-0 pb-0 justify-content-end">
            <button type="button" class="btn-close btn-close-white" @click="previewUrl = null"></button>
          </div>
          <div class="modal-body text-center pt-0">
            <img :src="previewUrl" class="img-fluid rounded shadow-lg" style="max-height: 85vh;" />
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted } from 'vue';
import axios from 'axios';

const events = ref([]);
const total = ref(0);
const page = ref(1);
const pageSize = ref(20);
const loading = ref(false);
const previewUrl = ref(null);
const devices = ref({});
const products = ref({});

let timer = null;

const formatTime = (ts) => {
  if (!ts) return '-';
  const d = new Date(ts);
  return d.toLocaleString();
};

const formatParams = (params) => {
  if (!params) return '-';
  const copy = { ...params };
  delete copy.snapshot_url;
  delete copy.snapshot_base64;
  return JSON.stringify(copy);
};

const getDeviceName = (code) => {
  if (!code) return '-';
  if (devices.value[code] && devices.value[code].name) {
    return devices.value[code].name;
  }
  return code;
};

const getEventDef = (evt) => {
  if (!evt.device_code || !evt.event_id) return null;
  const dev = devices.value[evt.device_code];
  if (!dev) return null;
  const prod = products.value[dev.product_code];
  if (!prod || !prod.model || !prod.model.events) return null;
  
  return prod.model.events.find(e => e.key === evt.event_id);
};

const sceneTranslations = {
  illegal_parking: '机动车违法停车',
  indoor_fire_passage_occupied: '室内消防通道占用',
  object_missing: '物品丢失'
};

const getEventName = (evt) => {
  const def = getEventDef(evt);
  if (def && def.name) {
    return def.name;
  }
  if (evt.params?.rule_name) {
    return evt.params.rule_name;
  }
  if (evt.params?.scene_type && sceneTranslations[evt.params.scene_type]) {
    return sceneTranslations[evt.params.scene_type];
  }
  return evt.event_id || '-';
};

const getEventTypeLabel = (evt) => {
  const def = getEventDef(evt);
  if (def && def.type) {
    if (def.type === 'alarm') return '告警';
    if (def.type === 'fault') return '故障';
    if (def.type === 'info') return '消息';
    return def.type;
  }
  if (evt.params?.scene_type) return '告警';
  return evt._type === 2 ? '告警' : '消息';
};

const getEventTypeColor = (evt) => {
  const def = getEventDef(evt);
  let type = '';
  if (def && def.type) {
    type = def.type;
  } else if (evt.params?.scene_type) {
    type = 'alarm';
  } else {
    type = evt._type === 2 ? 'alarm' : 'info';
  }
  
  if (type === 'alarm') return 'badge-soft-danger';
  if (type === 'fault') return 'badge-soft-warning';
  return 'badge-soft-info';
};

const fetchDataMetadata = async () => {
  try {
    const [devRes, prodRes] = await Promise.all([
      axios.get('/api/devices'),
      axios.get('/api/products')
    ]);
    
    if (devRes.data.code === 0 && devRes.data.data) {
      const devMap = {};
      devRes.data.data.forEach(d => {
        devMap[d.code] = d;
      });
      devices.value = devMap;
    }
    
    if (prodRes.data.code === 0 && prodRes.data.data) {
      const prodMap = {};
      prodRes.data.data.forEach(p => {
        if (typeof p.model === 'string') {
          try { p.model = JSON.parse(p.model); } catch (e) { p.model = {}; }
        }
        prodMap[p.code] = p;
      });
      products.value = prodMap;
    }
  } catch (e) {
    console.error('Failed to load metadata', e);
  }
};

const fetchEvents = async () => {
  if (loading.value) return;
  loading.value = true;
  try {
    const res = await axios.post('/api/history/query', {
      device_code: "",
      type: 2, // Event
      start_time: 0,
      end_time: 0,
      page: page.value,
      page_size: pageSize.value
    });
    if (res.data.code === 0 && res.data.data) {
      events.value = res.data.data.list || [];
      total.value = res.data.data.total || 0;
    }
  } catch (e) {
    console.error(e);
  } finally {
    loading.value = false;
  }
};

const previewImage = (url) => {
  previewUrl.value = url;
};

onMounted(async () => {
  await fetchDataMetadata();
  fetchEvents();
  // refresh every 10 seconds if on first page
  timer = setInterval(() => {
    if (page.value === 1 && !previewUrl.value) {
      fetchEvents();
    }
  }, 10000);
});

onUnmounted(() => {
  if (timer) clearInterval(timer);
});
</script>

<style scoped>
.table th {
  font-weight: 600;
  font-size: 0.9rem;
  color: #495057;
}

.alarm-badge {
  font-size: 0.85rem;
  letter-spacing: 0.5px;
  border: 1px solid transparent;
  display: inline-flex;
  align-items: center;
}

.small-icon {
  font-size: 0.65rem;
}

.badge-soft-danger {
  background-color: rgba(220, 53, 69, 0.1) !important;
  color: #dc3545 !important;
  border-color: rgba(220, 53, 69, 0.2);
}

.badge-soft-warning {
  background-color: rgba(255, 193, 7, 0.1) !important;
  color: #d39e00 !important;
  border-color: rgba(255, 193, 7, 0.2);
}

.badge-soft-info {
  background-color: rgba(13, 202, 240, 0.1) !important;
  color: #0dcaf0 !important;
  border-color: rgba(13, 202, 240, 0.2);
}
</style>
