<template>
<!-- Data Modal -->
    <div v-if="visible" class="modal fade show d-block" style="background: rgba(0,0,0,0.5)">
      <div class="modal-dialog modal-xl">
        <div class="modal-content" style="height: 90vh; display: flex; flex-direction: column;">
          <div class="modal-header">
            <h5 class="modal-title">{{ $t('dev_data_title') }} - {{ currentDataDevice?.code }}</h5>
            <button type="button" class="btn-close" @click="closeModal"></button>
          </div>
          <div class="modal-body d-flex flex-column">
            <!-- Tabs -->
            <ul class="nav nav-tabs mb-3">
              <li class="nav-item">
                <a class="nav-link" :class="{ active: activeTab === 'realtime' }" href="#" @click.prevent="activeTab = 'realtime'">{{ $t('dev_data_realtime') }}</a>
              </li>
              <li class="nav-item">
                <a class="nav-link" :class="{ active: activeTab === 'history' }" href="#" @click.prevent="activeTab = 'history'">{{ $t('dev_data_history') }}</a>
              </li>
              <li class="nav-item">
                <a class="nav-link" :class="{ active: activeTab === 'services' }" href="#" @click.prevent="activeTab = 'services'">服务调用</a>
              </li>
            </ul>

            <!-- Realtime Tab -->
            <div v-if="activeTab === 'realtime'" class="flex-grow-1">
              <div v-if="displayDataList.length === 0" class="text-center text-muted py-3">
                {{ $t('tsl_no_data') }}
              </div>
              <table v-else class="table table-hover align-middle">
                <thead>
                  <tr>
                    <th style="width: 20%">{{ $t('tsl_name') }}</th>
                    <th style="width: 15%">{{ $t('dev_data_point') }}</th>
                    <th style="width: 10%">{{ $t('tsl_prop_unit') }}</th>
                    <th style="width: 15%">{{ $t('dev_data_value') }}</th>
                    <th style="width: 15%">{{ $t('trend_30m') }}</th>
                    <th style="width: 25%">{{ $t('dev_data_write_val') }}</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="item in displayDataList" :key="item.key">
                    <td>{{ item.name || '-' }}</td>
                    <td>{{ item.key }}</td>
                    <td>{{ item.unit || '-' }}</td>
                    <td :class="currentDataDevice?.online ? '' : 'text-warning'">{{ item.value }}</td>
                    <td>
                      <Sparkline :data="item.trend" :width="100" :height="30" />
                    </td>
                    <td>
                      <input 
                        type="text" 
                        class="form-control form-control-sm" 
                        v-model="writeValues[item.key]" 
                        :placeholder="item.writable ? $t('dev_data_val_placeholder') : $t('dev_data_val_disabled')"
                        :disabled="!item.writable"
                      >
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>

            <!-- History Tab -->
            <div v-if="activeTab === 'history'" class="flex-grow-1 d-flex flex-column">
              <!-- Filters -->
              <div class="row g-2 mb-3 align-items-center">
                <div class="col-12 mb-2">
                  <div class="btn-group btn-group-sm">
                    <button type="button" class="btn" :class="historyRange === '1min' ? 'btn-secondary' : 'btn-outline-secondary'" @click="setHistoryRange('1min')">{{ $t('time_1min') }}</button>
                    <button type="button" class="btn" :class="historyRange === '10min' ? 'btn-secondary' : 'btn-outline-secondary'" @click="setHistoryRange('10min')">{{ $t('time_10min') }}</button>
                    <button type="button" class="btn" :class="historyRange === '30min' ? 'btn-secondary' : 'btn-outline-secondary'" @click="setHistoryRange('30min')">{{ $t('time_30min') }}</button>
                    <button type="button" class="btn" :class="historyRange === '1h' ? 'btn-secondary' : 'btn-outline-secondary'" @click="setHistoryRange('1h')">{{ $t('time_1h') }}</button>
                    <button type="button" class="btn" :class="historyRange === '1d' ? 'btn-secondary' : 'btn-outline-secondary'" @click="setHistoryRange('1d')">{{ $t('time_1d') }}</button>
                    <button type="button" class="btn" :class="historyRange === '1w' ? 'btn-secondary' : 'btn-outline-secondary'" @click="setHistoryRange('1w')">{{ $t('time_1w') }}</button>
                    <button type="button" class="btn" :class="historyRange === '1m' ? 'btn-secondary' : 'btn-outline-secondary'" @click="setHistoryRange('1m')">{{ $t('time_1m') }}</button>
                    <button type="button" class="btn" :class="historyRange === '1y' ? 'btn-secondary' : 'btn-outline-secondary'" @click="setHistoryRange('1y')">{{ $t('time_1y') }}</button>
                  </div>
                </div>
                <div class="col-auto">
                  <input type="datetime-local" class="form-control form-control-sm" v-model="historyQuery.startTime" @input="historyRange = null">
                </div>
                <div class="col-auto text-muted">-</div>
                <div class="col-auto">
                  <input type="datetime-local" class="form-control form-control-sm" v-model="historyQuery.endTime" @input="historyRange = null">
                </div>
                <div class="col-auto">
                  <select class="form-select form-select-sm" v-model="historyQuery.type">
                    <option value="property">{{ $t('tsl_type_prop') }}</option>
                    <option value="event">{{ $t('tsl_type_event') }}</option>
                  </select>
                </div>
                <div class="col-auto" v-if="historyQuery.type === 'property' && availableProperties.length > 0">
                   <div class="dropdown">
                      <button class="btn btn-sm btn-outline-secondary dropdown-toggle" type="button" data-bs-toggle="dropdown" aria-expanded="false">
                        {{ $t('select_properties') }} ({{ selectedProperties.length }})
                      </button>
                      <ul class="dropdown-menu p-2 shadow" style="max-height: 300px; overflow-y: auto;">
                        <li>
                          <div class="form-check">
                            <input class="form-check-input" type="checkbox" :checked="isAllPropertiesSelected" @change="toggleAllProperties">
                            <label class="form-check-label user-select-none" @click.prevent="toggleAllProperties">{{ $t('select_all') }}</label>
                          </div>
                        </li>
                        <li><hr class="dropdown-divider"></li>
                        <li v-for="prop in availableProperties" :key="prop.key">
                          <div class="form-check">
                            <input class="form-check-input" type="checkbox" :value="prop.key" v-model="selectedProperties" @change="renderChart">
                            <label class="form-check-label user-select-none" @click.prevent="toggleProperty(prop.key)">{{ prop.name }}</label>
                          </div>
                        </li>
                      </ul>
                   </div>
                </div>
                <div class="col-auto">
                  <button class="btn btn-primary btn-sm" @click="fetchHistory" :disabled="historyTableLoading || historyChartLoading">
                    <span v-if="historyTableLoading || historyChartLoading" class="spinner-border spinner-border-sm me-1"></span>
                    {{ $t('query') }}
                  </button>
                </div>
                <div class="col-auto d-flex align-items-center" v-if="historyQuery.type === 'property'">
                  <label class="form-label mb-0 small me-2" style="white-space: nowrap;">{{ $t('hist_max_points') }}:</label>
                  <input type="range" class="form-range me-2" min="100" max="5000" step="100" v-model.number="historyMaxPoints" style="width: 80px;" @change="fetchHistoryChart">
                  <input type="number" class="form-control form-control-sm me-3" min="100" max="5000" step="100" v-model.number="historyMaxPoints" style="width: 70px;" @change="fetchHistoryChart">
                  
                  <label class="form-label mb-0 small me-2" style="white-space: nowrap;">{{ $t('agg_method') }}:</label>
                  <select class="form-select form-select-sm" v-model="historyAggMethod" @change="fetchHistoryChart" style="width: 100px;">
                    <option value="avg">{{ $t('agg_avg') }}</option>
                    <option value="min">{{ $t('agg_min') }}</option>
                    <option value="max">{{ $t('agg_max') }}</option>
                    <option value="median">{{ $t('agg_median') }}</option>
                  </select>
                </div>
              </div>

              <!-- Chart -->
              <div class="border rounded mb-3 p-2 bg-light position-relative" style="height: 220px;">
                <div v-if="historyChartLoading" class="position-absolute top-0 start-0 w-100 h-100 d-flex align-items-center justify-content-center bg-white bg-opacity-75" style="z-index: 10">
                   <div class="spinner-border text-primary" role="status"></div>
                </div>
                <VChart v-if="chartOption" :option="chartOption" autoresize style="width: 100%; height: 100%;" />
                <div v-else class="h-100 d-flex align-items-center justify-content-center text-muted">
                  {{ $t('no_data_chart') }}
                </div>
              </div>

              <!-- Data List (Simple Log) -->
              <div class="flex-grow-1 overflow-auto border-top pt-2">
                <table class="table table-sm table-striped small">
                  <thead>
                    <tr>
                      <th>{{ $t('time') }}</th>
                      <th v-if="historyQuery.type === 'event'">{{ $t('type') }}</th>
                      <th>{{ $t('data') }}</th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr v-if="historyTableLoading">
                      <td :colspan="historyQuery.type === 'event' ? 3 : 2" class="text-center py-3">{{ $t('loading') }}</td>
                    </tr>
                    <tr v-else-if="historyTableData.length === 0">
                      <td :colspan="historyQuery.type === 'event' ? 3 : 2" class="text-center py-3 text-muted">{{ $t('tsl_no_data') }}</td>
                    </tr>
                    <tr v-else v-for="(item, index) in historyTableData" :key="index">
                      <td style="white-space: nowrap;">{{ new Date(item.ts).toLocaleString() }}</td>
                      <td v-if="historyQuery.type === 'event'">{{ getEventTypeLabel(item) }}</td>
                      <td class="text-break">{{ formatHistoryData(item) }}</td>
                    </tr>
                  </tbody>
                </table>
              </div>

              <!-- History Pagination -->
              <div class="d-flex justify-content-between align-items-center mt-2 border-top pt-2" v-if="historyTotal > 0">
                <div class="text-muted small ms-1">{{ $t('total_records', { count: historyTotal }) }}</div>
                <div class="d-flex align-items-center gap-2">
                  <select class="form-select form-select-sm" style="width: auto" v-model="historyPageSize" @change="changeHistoryPageSize">
                    <option :value="10">10 / {{ $t('page') }}</option>
                    <option :value="20">20 / {{ $t('page') }}</option>
                    <option :value="50">50 / {{ $t('page') }}</option>
                  </select>
                  <nav>
                    <ul class="pagination pagination-sm mb-0">
                      <li class="page-item" :class="{ disabled: historyPage === 1 }">
                        <button class="page-link" @click="changeHistoryPage(historyPage - 1)">
                          <i class="bi bi-chevron-left"></i>
                        </button>
                      </li>
                      <li class="page-item disabled">
                        <span class="page-link">{{ historyPage }} / {{ Math.ceil(historyTotal / historyPageSize) }}</span>
                      </li>
                      <li class="page-item" :class="{ disabled: historyPage * historyPageSize >= historyTotal }">
                        <button class="page-link" @click="changeHistoryPage(historyPage + 1)">
                          <i class="bi bi-chevron-right"></i>
                        </button>
                      </li>
                    </ul>
                  </nav>
                  <div class="input-group input-group-sm" style="width: 120px">
                    <input type="number" class="form-control" v-model.number="historyJumpPage" @keyup.enter="handleHistoryJump" placeholder="Go">
                    <button class="btn btn-outline-secondary" type="button" @click="handleHistoryJump">Go</button>
                  </div>
                </div>
              </div>
            </div>

            <!-- Services Tab -->
            <div v-if="activeTab === 'services'" class="flex-grow-1 overflow-auto">
              <div v-if="Object.keys(currentDataTSLServiceMap || {}).length === 0" class="text-center text-muted py-3">
                此产品未定义物模型服务
              </div>
              <div v-else class="accordion" id="servicesAccordion">
                <div class="accordion-item mb-2 border rounded" v-for="(srv, key) in currentDataTSLServiceMap" :key="key">
                  <h2 class="accordion-header" :id="'heading' + key">
                    <button class="accordion-button collapsed py-2" type="button" data-bs-toggle="collapse" :data-bs-target="'#collapse' + key" aria-expanded="false" :aria-controls="'collapse' + key">
                      <i class="bi bi-play-circle me-2 text-primary"></i> <strong>{{ srv.name }}</strong> <span class="text-muted ms-2 small">({{ key }})</span>
                    </button>
                  </h2>
                  <div :id="'collapse' + key" class="accordion-collapse collapse" :aria-labelledby="'heading' + key" data-bs-parent="#servicesAccordion">
                    <div class="accordion-body bg-light">
                      <div class="mb-3 text-muted small" v-if="srv.desc">{{ srv.desc }}</div>
                      
                      <!-- Input Form -->
                      <h6 class="small fw-bold">输入参数 (Input):</h6>
                      <div v-if="!srv.inputData || srv.inputData.length === 0" class="text-muted small mb-3">无输入参数</div>
                      <div class="row g-2 mb-3" v-else>
                        <div class="col-md-6 col-lg-4" v-for="param in srv.inputData" :key="param.identifier">
                          <label class="form-label small mb-1">{{ param.name }}<span v-if="param.required" class="text-danger ms-1">*</span> <span class="badge border text-secondary ms-1 p-1">{{ param.identifier }}</span> <span class="text-muted ms-1">({{ param.dataType?.type }})</span></label>
                          <select v-if="param.dataType?.type === 'bool'" class="form-select form-select-sm" v-model="serviceParams[key][param.identifier]">
                            <option value="">-- 请选择 --</option>
                            <option value="true">True</option>
                            <option value="false">False</option>
                          </select>
                          <select v-else-if="param.dataType?.type === 'enum'" class="form-select form-select-sm" v-model="serviceParams[key][param.identifier]">
                            <option value="">-- 请选择 --</option>
                            <option v-for="(val, k) in param.dataType.specs" :key="k" :value="k">{{ val }}</option>
                          </select>
                          <input v-else-if="param.dataType?.type === 'date'" type="datetime-local" class="form-control form-control-sm" v-model="serviceParams[key][param.identifier]">
                          <input v-else type="text" class="form-control form-control-sm" v-model="serviceParams[key][param.identifier]">
                        </div>
                      </div>

                      <button class="btn btn-primary btn-sm mb-3" @click="invokeDeviceService(key)" :disabled="invokeServiceLoading[key]">
                        <span v-if="invokeServiceLoading[key]" class="spinner-border spinner-border-sm me-1"></span>
                        <i v-else class="bi bi-send me-1"></i> 发送指令
                      </button>

                      <!-- Output Result -->
                      <div v-if="invokeServiceResult[key]" class="mt-3">
                        <div class="d-flex justify-content-between align-items-center mb-1">
                          <h6 class="small fw-bold mb-0">调用结果 (Output):</h6>
                          <div class="btn-group btn-group-sm" v-if="invokeServiceResult[key].success && srv.outputData && srv.outputData.length > 0">
                            <button class="btn" :class="invokeServiceResultMode[key] === 'json' ? 'btn-secondary' : 'btn-outline-secondary'" @click="invokeServiceResultMode[key] = 'json'">JSON</button>
                            <button class="btn" :class="invokeServiceResultMode[key] === 'ui' ? 'btn-secondary' : 'btn-outline-secondary'" @click="invokeServiceResultMode[key] = 'ui'">UI 视图</button>
                          </div>
                        </div>
                        <div v-if="invokeServiceResult[key].success">
                           <!-- UI Mode -->
                           <div v-if="invokeServiceResultMode[key] === 'ui' && srv.outputData && srv.outputData.length > 0" class="row g-2 border rounded p-2 bg-white">
                             <div class="col-md-6 col-lg-4" v-for="outParam in srv.outputData" :key="outParam.identifier">
                               <label class="form-label small mb-1 text-muted">{{ outParam.name }}<span v-if="outParam.required" class="text-danger ms-1">*</span> <span class="badge border text-secondary ms-1 p-1">{{ outParam.identifier }}</span></label>
                               <div class="form-control form-control-sm bg-light text-break overflow-auto" style="min-height:30px;">{{ getOutputValue(invokeServiceResult[key].data, outParam) }}</div>
                             </div>
                           </div>
                           <!-- JSON Mode -->
                           <div v-else class="position-relative">
                             <div class="alert alert-success p-2 small mb-0 font-monospace" style="white-space: pre-wrap; overflow-x: auto; padding-right: 30px !important;">{{ JSON.stringify(invokeServiceResult[key].data, null, 2) || '调用成功 (无返回数据)' }}</div>
                             <button class="btn btn-sm btn-link text-secondary position-absolute top-0 end-0 m-1 p-0" style="width: 24px; height: 24px; display: flex; align-items: center; justify-content: center; background: rgba(255,255,255,0.8); border-radius: 4px;" @click="copyToClipboard(JSON.stringify(invokeServiceResult[key].data, null, 2) || '调用成功 (无返回数据)')" title="复制">
                               <i class="bi bi-clipboard"></i>
                             </button>
                           </div>
                        </div>
                        <div v-else class="alert alert-danger p-2 small mb-0">
                          <i class="bi bi-exclamation-triangle me-1"></i> {{ invokeServiceResult[key].error }}
                        </div>
                      </div>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>
          <div class="modal-footer">
            <button type="button" class="btn btn-secondary" style="min-width: 80px" @click="closeModal">{{ $t('tsl_cancel') }}</button>
            <button v-if="activeTab === 'realtime'" type="button" class="btn btn-primary" style="min-width: 80px" @click="submitBatchWrite">
              <i class="bi bi-save me-1"></i> {{ $t('dev_data_save_all') }}
            </button>
          </div>
        </div>
      </div>
    </div>
</template>

<script setup>
import { ref, computed, watch, onUnmounted } from 'vue';
import axios from 'axios';
import { useI18n } from 'vue-i18n';
import Sparkline from '../Sparkline.vue';
import VChart from 'vue-echarts';

const props = defineProps({
  visible: Boolean,
  device: Object,
  products: Array
});

const emit = defineEmits(['close']);
const { t } = useI18n();

// Component State
const activeTab = ref('realtime');
const currentDataDevice = computed(() => props.device);
const products = computed(() => props.products || []);
const deviceData = ref({});
const realtimeTrendData = ref({});
const writeValues = ref({});
const pointConfigs = ref({});
const currentDataTSLMap = ref({});
const currentDataTSLEventMap = ref({});
const currentDataTSLServiceMap = ref({});
const serviceParams = ref({});
const invokeServiceLoading = ref({});
const invokeServiceResult = ref({});
const invokeServiceResultMode = ref({});

let dataTimer = null;
let trendTimer = null;

const historyQuery = ref({
  startTime: '',
  endTime: '',
  type: 'property'
});
const historyTableData = ref([]);
const historyChartData = ref([]);
const historyTableLoading = ref(false);
const historyChartLoading = ref(false);
const historyChartInterval = ref(0);
const historyPage = ref(1);
const historyMaxPoints = ref(2000); 
const historyAggMethod = ref('avg'); 
const historyPageSize = ref(10);
const historyTotal = ref(0);
const historyJumpPage = ref(1);
const historyRange = ref('1d');

const chartOption = ref(null);
const availableProperties = ref([]);
const selectedProperties = ref([]);

const closeModal = () => {
  emit('close');
};

// Initialize when modal becomes visible
watch(() => props.visible, async (val) => {
  if (val && props.device) {
    deviceData.value = {};
    realtimeTrendData.value = {};
    writeValues.value = {};
    pointConfigs.value = {};
    currentDataTSLMap.value = {};
    currentDataTSLEventMap.value = {};
    currentDataTSLServiceMap.value = {};
    serviceParams.value = {};
    invokeServiceLoading.value = {};
    invokeServiceResult.value = {};
    invokeServiceResultMode.value = {};
    
    // Parse config to get point configs
    try {
      const config = props.device.config ? JSON.parse(props.device.config) : {};
      if (config.points) {
        if (Array.isArray(config.points)) {
          config.points.forEach(p => {
            if (p.name) pointConfigs.value[p.name] = p;
          });
        } else {
          pointConfigs.value = config.points;
        }
      }
    } catch (e) {
      console.error("Failed to parse device config", e);
    }

    let product = null;
    try {
        const res = await axios.get(`/api/products/${props.device.product_code}`);
        if (res.data.code === 0) {
            product = res.data.data;
        }
    } catch (e) {
        console.warn("Failed to fetch product for TSL", e);
        product = products.value.find(p => p.code === props.device.product_code);
    }

    if (product && product.config) {
        try {
            const prodConfig = JSON.parse(product.config);
            const tslProps = prodConfig.tsl?.properties || [];
            tslProps.forEach(p => currentDataTSLMap.value[p.identifier] = p);
            const tslEvents = prodConfig.tsl?.events || [];
            tslEvents.forEach(e => currentDataTSLEventMap.value[e.identifier] = e);
            const tslServices = prodConfig.tsl?.services || [];
            tslServices.forEach(s => currentDataTSLServiceMap.value[s.identifier] = s);

            for (const key of Object.keys(currentDataTSLServiceMap.value)) {
                serviceParams.value[key] = {};
                const srv = currentDataTSLServiceMap.value[key];
                if (srv.inputData) {
                    srv.inputData.forEach(param => {
                        serviceParams.value[key][param.identifier] = '';
                    });
                }
            }
        } catch (e) {
            console.warn("Invalid Product Config JSON for Data Modal");
        }
    }

    activeTab.value = 'realtime';
    initHistoryDates();
    fetchDeviceData();
    fetchRealtimeTrend();
    
    dataTimer = setInterval(fetchDeviceData, 2000);
    trendTimer = setInterval(fetchRealtimeTrend, 30000);
  } else {
    // Cleanup when hiding
    if (dataTimer) { clearInterval(dataTimer); dataTimer = null; }
    if (trendTimer) { clearInterval(trendTimer); trendTimer = null; }
  }
});

onUnmounted(() => {
  if (dataTimer) clearInterval(dataTimer);
  if (trendTimer) clearInterval(trendTimer);
});

const getEventTypeLabel = (item) => {
  if (historyQuery.value.type === 'property') {
      return t('tsl_type_prop');
  }
  if (historyQuery.value.type === 'event') {
      const evtId = item.event_id;
      if (!evtId) return t('tsl_type_event');
      
      const tslEvent = currentDataTSLEventMap.value[evtId];
      if (!tslEvent) return evtId;
      
      const type = (tslEvent.type || '').toLowerCase();
      switch (type) {
        case 'info': return t('tsl_evt_type_info');
        case 'alert': 
        case 'warn': 
        case 'warning':
          return t('tsl_evt_type_alert');
        case 'error': 
        case 'fault':
          return t('tsl_evt_type_error');
        default: return tslEvent.type;
      }
  }
  return historyQuery.value.type;
};


const formatHistoryData = (item) => {
  const clone = { ...item };
  delete clone.ts;
  delete clone._type;
  
  if (historyQuery.value.type === 'event') {
    delete clone.event_id;
  }
  
  return JSON.stringify(clone);
};


const changeHistoryPage = (p) => {
  if (p < 1 || p > Math.ceil(historyTotal.value / historyPageSize.value)) return;
  historyPage.value = p;
  fetchHistoryTable();
};


const changeHistoryPageSize = () => {
  historyPage.value = 1;
  fetchHistoryTable();
};


const handleHistoryJump = () => {
  const p = parseInt(historyJumpPage.value);
  if (!p || isNaN(p)) return;
  changeHistoryPage(p);
};


const isAllPropertiesSelected = computed(() => {
    return availableProperties.value.length > 0 && selectedProperties.value.length === availableProperties.value.length;
});


const toggleAllProperties = () => {
    if (isAllPropertiesSelected.value) {
        selectedProperties.value = [];
    } else {
        selectedProperties.value = availableProperties.value.map(p => p.key);
    }
    renderChart();
};


const toggleProperty = (key) => {
    const idx = selectedProperties.value.indexOf(key);
    if (idx > -1) {
        selectedProperties.value.splice(idx, 1);
    } else {
        selectedProperties.value.push(key);
    }
    renderChart();
};


const initHistoryDates = () => {
  const end = new Date();
  const start = new Date();
  start.setTime(end.getTime() - 10 * 60 * 1000); // 10 minutes ago
  
  // Format to YYYY-MM-DDTHH:mm for datetime-local input
  const format = (d) => {
    const pad = (n) => n < 10 ? '0' + n : n;
    return `${d.getFullYear()}-${pad(d.getMonth()+1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
  };
  
  historyRange.value = '10min';
  historyQuery.value.startTime = format(start);
  historyQuery.value.endTime = format(end);
  historyQuery.value.type = 'property';
  historyTableData.value = [];
  historyChartData.value = [];
  chartOption.value = null;
};


const setHistoryRange = (range) => {
  historyRange.value = range;
  const end = new Date();
  const start = new Date();
  
  switch(range) {
    case '1min':
      start.setTime(end.getTime() - 60 * 1000);
      break;
    case '10min':
      start.setTime(end.getTime() - 10 * 60 * 1000);
      break;
    case '30min':
      start.setTime(end.getTime() - 30 * 60 * 1000);
      break;
    case '1h':
      start.setTime(end.getTime() - 3600 * 1000);
      break;
    case '1d':
      start.setTime(end.getTime() - 24 * 3600 * 1000);
      break;
    case '1w':
      start.setTime(end.getTime() - 7 * 24 * 3600 * 1000);
      break;
    case '1m':
      start.setMonth(end.getMonth() - 1);
      break;
    case '1y':
      start.setFullYear(end.getFullYear() - 1);
      break;
  }
  
  const format = (d) => {
    const pad = (n) => n < 10 ? '0' + n : n;
    return `${d.getFullYear()}-${pad(d.getMonth()+1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
  };
  
  historyQuery.value.startTime = format(start);
  historyQuery.value.endTime = format(end);
  fetchHistory();
};


const fetchHistory = () => {
  historyPage.value = 1;
  fetchHistoryTable();
  fetchHistoryChart();
};


const fetchHistoryTable = async () => {
  if (!historyQuery.value.startTime || !historyQuery.value.endTime) return;
  historyTableLoading.value = true;
  try {
    const startTs = new Date(historyQuery.value.startTime).getTime();
    const endTs = new Date(historyQuery.value.endTime).getTime();

    const res = await axios.post('/api/history/query', {
      device_code: currentDataDevice.value.code,
      start_time: startTs,
      end_time: endTs,
      type: historyQuery.value.type === 'event' ? 2 : 1,
      page: historyPage.value,
      page_size: historyPageSize.value,
      aggregate: false
    });

    if (res.data.code === 0) {
      historyTableData.value = res.data.data.list || [];
      historyTotal.value = res.data.data.total || 0;
    }
  } catch (e) {
    console.error(e);
  } finally {
    historyTableLoading.value = false;
  }
};


const fetchHistoryChart = async () => {
  if (!historyQuery.value.startTime || !historyQuery.value.endTime) return;
  historyChartLoading.value = true;
  try {
    const startTs = new Date(historyQuery.value.startTime).getTime();
    const endTs = new Date(historyQuery.value.endTime).getTime();

    const res = await axios.post('/api/history/query', {
      device_code: currentDataDevice.value.code,
      start_time: startTs,
      end_time: endTs,
      type: historyQuery.value.type === 'event' ? 2 : 1,
      aggregate: true,
      max_points: historyMaxPoints.value,
      agg_method: historyAggMethod.value
    });

    if (res.data.code === 0) {
      historyChartData.value = res.data.data.list || [];
      historyChartInterval.value = res.data.data.interval || 0;

      if (historyQuery.value.type === 'property') {
        const propKeys = new Set();
        historyChartData.value.forEach(item => {
          Object.keys(item).forEach(k => {
            if (k !== 'ts' && k !== '_type' && k !== 'raw' && k !== 'error') propKeys.add(k);
          });
        });

        availableProperties.value = Array.from(propKeys).map(key => {
          const tsl = currentDataTSLMap.value[key];
          return { key, name: tsl ? tsl.name : key };
        }).sort((a, b) => a.name.localeCompare(b.name));

        if (selectedProperties.value.length === 0) {
          selectedProperties.value = availableProperties.value.map(p => p.key);
        }
      } else {
        availableProperties.value = [];
        selectedProperties.value = [];
      }

      renderChart();
    }
  } catch (e) {
    console.error(e);
  } finally {
    historyChartLoading.value = false;
  }
};


const renderChart = () => {
  if (historyChartData.value.length === 0) {
    chartOption.value = null;
    return;
  }

  if (historyQuery.value.type === 'event') {
    const counts = {};
    historyChartData.value.forEach(item => {
      const evtId = item.event_id || 'Unknown';
      const tslEvent = currentDataTSLEventMap.value[evtId];
      const displayName = tslEvent ? `${tslEvent.name} (${evtId})` : evtId;
      const cnt = item.count !== undefined ? item.count : 1;
      counts[displayName] = (counts[displayName] || 0) + cnt;
    });

    const events = Object.keys(counts);
    const data = events.map(e => counts[e]);

    chartOption.value = {
      tooltip: {
        trigger: 'axis',
        axisPointer: { type: 'shadow' },
        formatter: (params) => {
          const p = params[0];
          return `${p.name}<br/>${t('count')}: ${p.value}`;
        }
      },
      grid: { left: '3%', right: '4%', bottom: '3%', containLabel: true },
      xAxis: {
        type: 'category',
        data: events,
        axisLabel: { interval: 0, rotate: 30 }
      },
      yAxis: {
        type: 'value',
        minInterval: 1
      },
      series: [{
        name: t('count'),
        data: data,
        type: 'bar',
        barMaxWidth: 50,
        label: { show: true, position: 'top' }
      }]
    };
  } else {
    // const timestamps = historyChartData.value.map(item => new Date(item.ts).toLocaleString());
    const series = [];
    const legendData = [];

    availableProperties.value.forEach(prop => {
      if (!selectedProperties.value.includes(prop.key)) return;

      const displayName = `${prop.name} (${prop.key})`;
      legendData.push(displayName);

      const data = historyChartData.value.map(item => {
        return [item.ts, item[prop.key] !== undefined ? item[prop.key] : null];
      });

      series.push({
        name: displayName,
        type: 'line',
        data: data,
        smooth: true,
        connectNulls: true,
        showSymbol: data.length < 100 // Only show points if few data
      });
    });

    chartOption.value = {
      tooltip: { 
        trigger: 'axis',
        formatter: (params) => {
          try {
            if (!params || params.length === 0) return '';
            
            const item0 = params[0];
            // Ensure value is an array [ts, value]
            if (!Array.isArray(item0.value)) return '';

            const ts = Number(item0.value[0]);
            if (isNaN(ts)) return '';

            const date = new Date(ts);
            let timeStr = date.toLocaleString();
            
            const interval = Number(historyChartInterval.value || 0);

            if (interval > 0) {
               const endDate = new Date(ts + interval);
               // Use time string for end time to keep it short
               timeStr += ` ~ ${endDate.toLocaleTimeString()}`;
            }
            
            let html = `<div style="margin-bottom: 3px; font-weight: bold;">${timeStr}</div>`;
            
            params.forEach(item => {
              if (!Array.isArray(item.value) || item.value.length < 2) return;
              
              let val = item.value[1];
              if (val !== null && val !== undefined) {
                // If aggregated, format to 2 decimals if it's a float
                if (typeof val === 'number') {
                    if (interval > 0 && !Number.isInteger(val)) {
                      val = val.toFixed(2);
                    }
                }
                html += `<div style="display: flex; justify-content: space-between; align-items: center;">
                          <span style="margin-right: 10px;">${item.marker}${item.seriesName}</span>
                          <span style="font-weight: bold;">${val}</span>
                        </div>`;
              }
            });
            return html;
          } catch (e) {
            console.error('Tooltip error:', e);
            return '';
          }
        }
      },
      legend: { data: legendData, bottom: 0 },
      grid: { left: '3%', right: '4%', bottom: '10%', containLabel: true },
      xAxis: { type: 'time' },
      yAxis: { type: 'value', scale: true },
      dataZoom: [{ type: 'inside' }, { type: 'slider' }],
      series: series
    };
  }
};


const fetchRealtimeTrend = async () => {
  if (!currentDataDevice.value) return;
  
  const endTs = Date.now();
  const startTs = endTs - 30 * 60 * 1000; // 30 minutes ago

  try {
    const res = await axios.post('/api/history/query', {
      device_code: currentDataDevice.value.code,
      start_time: startTs,
      end_time: endTs,
      type: 1, // property
      aggregate: true, 
      max_points: 60, // 30 mins, maybe 60 points (1 per 30s) is enough for sparkline
      agg_method: 'avg'
    });

    if (res.data.code === 0) {
      const list = res.data.data.list || [];
      const trendMap = {};
      
      // Initialize lists for all known properties
      Object.keys(currentDataTSLMap.value).forEach(key => {
        trendMap[key] = [];
      });

      list.forEach(item => {
        Object.keys(item).forEach(key => {
          if (key === 'ts' || key === '_type' || key === 'raw' || key === 'error') return;
          if (!trendMap[key]) trendMap[key] = [];
          trendMap[key].push(item[key]);
        });
      });
      
      realtimeTrendData.value = trendMap;
    }
  } catch (e) {
    console.error("Failed to fetch trend", e);
  }
};


const isPointWritable = (key) => {
  const cfg = pointConfigs.value[key];
  return cfg && cfg.enable_write === true;
};


const displayDataList = computed(() => {
  const keys = new Set([
    ...Object.keys(pointConfigs.value || {}),
    ...Object.keys(deviceData.value || {})
  ]);
  
  const list = [];
  keys.forEach(key => {
    const tslProp = currentDataTSLMap.value[key];
    list.push({
      key: key,
      name: tslProp ? tslProp.name : key,
      unit: tslProp && tslProp.dataType && tslProp.dataType.specs ? tslProp.dataType.specs.unit : '',
      value: deviceData.value[key] !== undefined ? deviceData.value[key] : '-',
      writable: isPointWritable(key),
      trend: realtimeTrendData.value[key] || []
    });
  });
  return list.sort((a, b) => a.key.localeCompare(b.key));
});


const fetchDeviceData = async () => {
  if (!currentDataDevice.value) return;
  try {
    const res = await axios.get(`/api/devices/${currentDataDevice.value.code}/data`);
    if (res.data.code === 0) {
      deviceData.value = res.data.data || {};
    }
  } catch (e) {
    console.error(e);
  }
};


const submitBatchWrite = async () => {
  if (!currentDataDevice.value) return;
  
  const updates = [];
  for (const [pointId, rawVal] of Object.entries(writeValues.value)) {
      if (rawVal === '' || rawVal === null || rawVal === undefined) continue;
      
      let val = rawVal;
      // Attempt to parse value as number if it looks like one
      if (!isNaN(val) && val !== '' && val !== null) {
          if (String(val).includes('.')) {
              val = parseFloat(val);
          } else {
              val = parseInt(val, 10);
          }
      }
      
      updates.push(
          axios.post(`/api/devices/${currentDataDevice.value.code}/write`, {
              point_id: pointId,
              value: val
          }).then(res => ({ pointId, success: res.data.code === 0, msg: res.data.message }))
            .catch(err => ({ pointId, success: false, msg: err.message }))
      );
  }
  
  if (updates.length === 0) {
      alert(t('dev_data_no_input'));
      return;
  }
  
  const results = await Promise.all(updates);
  const failures = results.filter(r => !r.success);
  
  if (failures.length === 0) {
      alert(t('write_success'));
      writeValues.value = {}; // Clear inputs on full success
      fetchDeviceData();
  } else {
      const msg = failures.map(f => `${f.pointId}: ${f.msg}`).join('\n');
      alert(t('write_fail') + '\n' + msg);
  }
};


const getOutputValue = (data, outParam) => {
  const identifier = outParam.identifier;
  if (data === null || data === undefined) return '-';
  let val = '-';
  if (typeof data !== 'object') {
    val = data;
  } else {
    val = data[identifier] !== undefined ? data[identifier] : '-';
  }
  
  if (outParam.dataType?.type === 'enum' && outParam.dataType?.specs && val !== '-') {
    const enumName = outParam.dataType.specs[val];
    if (enumName !== undefined) {
      return `${enumName} (${val})`;
    }
  }
  return val;
};


const copyToClipboard = async (text) => {
  try {
    await navigator.clipboard.writeText(text);
    alert('已复制到剪贴板');
  } catch (err) {
    console.error('Failed to copy: ', err);
    alert('复制失败');
  }
};


const invokeDeviceService = async (serviceId) => {
  if (!currentDataDevice.value) return;
  invokeServiceLoading.value[serviceId] = true;
  invokeServiceResult.value[serviceId] = null;
  const params = serviceParams.value[serviceId] || {};
  
  const parsedParams = {};
  const srv = currentDataTSLServiceMap.value[serviceId];
  for (const param of (srv.inputData || [])) {
    let val = params[param.identifier];
    if (param.required && (val === '' || val === null || val === undefined)) {
      alert(`必填参数 [${param.name}] 不能为空！`);
      invokeServiceLoading.value[serviceId] = false;
      return;
    }
    if (val !== '' && val !== null && val !== undefined) {
      if (param.dataType?.type === 'int' || param.dataType?.type === 'float' || param.dataType?.type === 'double') {
        val = Number(val);
      } else if (param.dataType?.type === 'bool') {
        val = val === 'true' || val === true;
      }
    }
    parsedParams[param.identifier] = val;
  }

  try {
    const res = await axios.post(`/api/devices/${currentDataDevice.value.code}/invoke`, {
      service_id: serviceId,
      params: parsedParams
    });
    if (res.data.code === 0) {
      invokeServiceResult.value[serviceId] = { success: true, data: res.data.data };
      if (srv.outputData && srv.outputData.length > 0) {
          invokeServiceResultMode.value[serviceId] = 'ui';
      } else {
          invokeServiceResultMode.value[serviceId] = 'json';
      }
    } else {
      invokeServiceResult.value[serviceId] = { success: false, error: res.data.message };
    }
  } catch (err) {
    invokeServiceResult.value[serviceId] = { success: false, error: err.message || err };
  } finally {
    invokeServiceLoading.value[serviceId] = false;
  }
};


watch(activeTab, (val) => {
  if (val === 'history') {
    fetchHistory();
  }
});

watch(() => historyQuery.value.type, () => {
  historyTableData.value = [];
  historyChartData.value = [];
  chartOption.value = null;
  historyPage.value = 1;
  // Automatically fetch new data type
  fetchHistory();
});




</script>

<style scoped>
</style>
