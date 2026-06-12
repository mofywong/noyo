<template>
  <div class="alarm-center container-fluid py-4">
    <div class="d-flex justify-content-between align-items-center mb-4">
      <h2 class="h4 mb-0 fw-bold text-primary border-start border-primary border-4 ps-2">{{ $t('sidebar_alarms', '告警中心') }}</h2>
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
                <th class="text-center">操作</th>
              </tr>
            </thead>
            <tbody>
              <tr v-if="loading && events.length === 0">
                <td colspan="7" class="text-center py-5 text-muted">
                  <div class="spinner-border spinner-border-sm me-2" role="status"></div>加载中...
                </td>
              </tr>
              <tr v-else-if="events.length === 0">
                <td colspan="7" class="text-center py-5 text-muted">暂无告警事件</td>
              </tr>
              <tr v-for="evt in events" :key="(evt._record_id || evt.ts) + evt.event_id" v-else
                  class="alarm-row" @click="openDetail(evt)" style="cursor: pointer;">
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
                       @click.stop="previewImage(evt.params.snapshot_url)">
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
                <td class="text-center">
                  <button class="btn btn-outline-success btn-sm" @click.stop="dismissAlarm(evt)" :disabled="evt._dismissing"
                          title="消警" v-permission="'alarm:handle'">
                    <span v-if="evt._dismissing" class="spinner-border spinner-border-sm" role="status"></span>
                    <i v-else class="bi bi-check-circle"></i>
                    <span class="ms-1">消警</span>
                  </button>
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

    <!-- Alarm Detail Modal -->
    <div v-if="detailEvent" class="modal fade show d-block" style="background: rgba(0,0,0,0.6)" @click.self="detailEvent = null" tabindex="-1">
      <div class="modal-dialog modal-lg modal-dialog-centered">
        <div class="modal-content border-0 shadow-lg">
          <div class="modal-header border-bottom">
            <h5 class="modal-title d-flex align-items-center">
              <i class="bi bi-exclamation-triangle-fill text-danger me-2"></i>
              告警详情
            </h5>
            <button type="button" class="btn-close" @click="detailEvent = null"></button>
          </div>
          <div class="modal-body">
            <div class="row g-3">
              <div class="col-md-6">
                <div class="detail-field">
                  <label class="detail-label">事件名称</label>
                  <div class="detail-value">
                    <span class="badge rounded-pill fw-normal px-2 py-1 alarm-badge" :class="getEventTypeColor(detailEvent)">
                      <i class="bi bi-record-circle-fill me-1 small-icon"></i>
                      {{ getEventName(detailEvent) }}
                    </span>
                  </div>
                </div>
              </div>
              <div class="col-md-6">
                <div class="detail-field">
                  <label class="detail-label">事件类型</label>
                  <div class="detail-value">
                    <span class="badge rounded-pill fw-normal px-2 py-1 alarm-badge" :class="getEventTypeColor(detailEvent)">
                      {{ getEventTypeLabel(detailEvent) }}
                    </span>
                  </div>
                </div>
              </div>
              <div class="col-md-6">
                <div class="detail-field">
                  <label class="detail-label">设备</label>
                  <div class="detail-value">
                    {{ getDeviceName(detailEvent.device_code) }}
                    <span class="text-muted small ms-1">({{ detailEvent.device_code }})</span>
                  </div>
                </div>
              </div>
              <div class="col-md-6">
                <div class="detail-field">
                  <label class="detail-label">发生时间</label>
                  <div class="detail-value">{{ formatTime(detailEvent.ts) }}</div>
                </div>
              </div>
              <div class="col-12" v-if="detailEvent.params?.rule_name">
                <div class="detail-field">
                  <label class="detail-label">规则名称</label>
                  <div class="detail-value">{{ detailEvent.params.rule_name }}</div>
                </div>
              </div>
              <div class="col-12" v-if="detailEvent.params?.scene_type">
                <div class="detail-field">
                  <label class="detail-label">场景类型</label>
                  <div class="detail-value">{{ sceneTranslations[detailEvent.params.scene_type] || detailEvent.params.scene_type }}</div>
                </div>
              </div>
              <div class="col-md-6" v-if="detailEvent.params?.target_class">
                <div class="detail-field">
                  <label class="detail-label">目标类型</label>
                  <div class="detail-value">{{ detailEvent.params.target_name || detailEvent.params.target_class }}</div>
                </div>
              </div>
              <div class="col-md-6" v-if="detailEvent.params?.confidence">
                <div class="detail-field">
                  <label class="detail-label">置信度</label>
                  <div class="detail-value">{{ (detailEvent.params.confidence * 100).toFixed(1) }}%</div>
                </div>
              </div>
              <div class="col-md-6" v-if="detailEvent.params?.duration_seconds">
                <div class="detail-field">
                  <label class="detail-label">持续时间</label>
                  <div class="detail-value">{{ detailEvent.params.duration_seconds }} 秒</div>
                </div>
              </div>
              <div class="col-12" v-if="detailEvent.params?.snapshot_url">
                <div class="detail-field">
                  <label class="detail-label">抓拍图片</label>
                  <div class="detail-value">
                    <img :src="detailEvent.params.snapshot_url" 
                         class="img-fluid rounded shadow-sm" 
                         style="max-height: 400px; cursor: pointer;" 
                         alt="snapshot"
                         @click="previewImage(detailEvent.params.snapshot_url)" />
                  </div>
                </div>
              </div>
              <div class="col-12">
                <div class="detail-field">
                  <label class="detail-label">完整参数</label>
                  <div class="detail-value">
                    <pre class="mb-0 p-2 rounded small" style="background: var(--bg-body); max-height: 200px; overflow: auto;">{{ formatDetailParams(detailEvent.params) }}</pre>
                  </div>
                </div>
              </div>
            </div>
          </div>
          <div class="modal-footer border-top">
            <button class="btn btn-outline-success" @click="dismissAlarm(detailEvent)" :disabled="detailEvent._dismissing" v-permission="'alarm:handle'">
              <span v-if="detailEvent._dismissing" class="spinner-border spinner-border-sm me-1" role="status"></span>
              <i v-else class="bi bi-check-circle me-1"></i>消警
            </button>
            <button type="button" class="btn btn-secondary" @click="detailEvent = null">关闭</button>
          </div>
        </div>
      </div>
    </div>

    <!-- Image Preview Modal -->
    <div v-if="previewUrl" class="modal fade show d-block" style="background: rgba(0,0,0,0.8); z-index: 1060;" @click.self="previewUrl = null" tabindex="-1">
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
import { ref, onMounted, onUnmounted, watch } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import axios from 'axios';

const route = useRoute();
const router = useRouter();
const events = ref([]);
const total = ref(0);
const page = ref(1);
const pageSize = ref(20);
const loading = ref(false);
const previewUrl = ref(null);
const detailEvent = ref(null);
const devices = ref({});
const products = ref({});

let eventSource = null;

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

const formatDetailParams = (params) => {
  if (!params) return '-';
  const copy = { ...params };
  delete copy.snapshot_url;
  delete copy.snapshot_base64;
  return JSON.stringify(copy, null, 2);
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

// 内部方法：纯粹获取事件数据，不调用 checkHighlight，供 checkHighlight 内部使用
const fetchEventsCore = async () => {
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
  }
};

const fetchEvents = async () => {
  if (loading.value) return;
  loading.value = true;
  try {
    await fetchEventsCore();
    // 如果 URL 参数中带有 highlight，自动打开对应的告警详情
    checkHighlight();
  } finally {
    loading.value = false;
  }
};

const checkHighlight = async () => {
  const highlightTs = route.query.highlight;
  if (highlightTs) {
    const ts = parseInt(highlightTs);
    let found = events.value.find(evt => evt.ts === ts);
    // 如果事件列表中未找到（可能数据还未加载或不在当前页），刷新后重试
    if (!found) {
      page.value = 1; // 回到第一页
      await fetchEventsCore();
      found = events.value.find(evt => evt.ts === ts);
    }
    if (found) {
      detailEvent.value = found;
    }
    // 消费掉 query 参数，避免重复触发
    router.replace({ path: '/alarms', query: {} });
  }
};

const openDetail = (evt) => {
  detailEvent.value = evt;
};

const dismissAlarm = async (evt) => {
  if (!evt._record_id || !evt.ts) {
    console.error('Cannot dismiss: missing _record_id or ts');
    return;
  }
  evt._dismissing = true;
  try {
    const res = await axios.delete('/api/history/record', {
      data: { id: evt._record_id, ts: evt.ts }
    });
    if (res.data.code === 0) {
      // 从列表中移除
      events.value = events.value.filter(e => e !== evt);
      total.value = Math.max(0, total.value - 1);
      // 如果正在查看详情，关闭弹框
      if (detailEvent.value === evt) {
        detailEvent.value = null;
      }
    } else {
      console.error('Dismiss failed:', res.data.message);
    }
  } catch (e) {
    console.error('Dismiss error:', e);
  } finally {
    evt._dismissing = false;
  }
};

const previewImage = (url) => {
  previewUrl.value = url;
};

// 监听路由变化，如果从消息盒子跳转过来则自动打开详情
// 监听整个 query 对象，确保 _t 随机参数变化也能触发
watch(() => route.query, (newQuery) => {
  if (newQuery?.highlight) {
    checkHighlight();
  }
}, { deep: true });

const setupEventStream = () => {
  if (eventSource) return;
  eventSource = new EventSource('/api/devices/stream?token=' + localStorage.getItem('access_token'));
  
  eventSource.addEventListener('event.reported', (e) => {
    try {
      if (page.value !== 1) return; // Only real-time update on first page
      
      const data = JSON.parse(e.data);
      const evt = {
        device_code: data.Topic,
        event_id: data.Payload.eventId,
        params: data.Payload.params,
        ts: data.Timestamp
      };
      
      events.value.unshift(evt);
      if (events.value.length > pageSize.value) {
        events.value.pop();
      }
      total.value++;
    } catch (err) {
      console.error('Failed to parse SSE event:', err);
    }
  });

  eventSource.onerror = () => {
    if (eventSource.readyState === EventSource.CLOSED) {
      setTimeout(() => {
        eventSource = null;
        setupEventStream();
      }, 3000);
    }
  };
};

onMounted(async () => {
  await fetchDataMetadata();
  fetchEvents();
  setupEventStream();
});

onUnmounted(() => {
  if (eventSource) {
    eventSource.close();
    eventSource = null;
  }
});
</script>

<style scoped>
.table th {
  font-weight: 600;
  font-size: 0.9rem;
  color: #495057;
}

.alarm-row:hover {
  background-color: rgba(59, 130, 246, 0.04) !important;
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

.detail-field {
  margin-bottom: 0.5rem;
}

.detail-label {
  display: block;
  font-size: 0.75rem;
  font-weight: 600;
  text-transform: uppercase;
  color: var(--text-secondary, #64748b);
  letter-spacing: 0.05em;
  margin-bottom: 0.25rem;
}

.detail-value {
  font-size: 0.95rem;
  color: var(--text-main, #0f172a);
}

.modal-content {
  border-radius: 0.75rem;
}
</style>
