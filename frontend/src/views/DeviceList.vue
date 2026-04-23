<template>
  <div class="card border-0 shadow-sm h-100">
    <div class="card-header bg-transparent border-0 py-3">
      <div v-if="selectedDevices.length > 0" class="d-flex align-items-center p-2 rounded bg-secondary bg-opacity-10">
        <span class="me-3 fw-bold">{{ selectedDevices.length }} {{ $t('selected') }}</span>
        <div class="btn-group btn-group-sm">
          <button class="btn btn-outline-success" @click="batchEnable">
            <i class="bi bi-check-circle"></i> {{ $t('dev_enable') }}
          </button>
          <button class="btn btn-outline-secondary" @click="batchDisable">
            <i class="bi bi-x-circle"></i> {{ $t('dev_disable') }}
          </button>
          <button class="btn btn-outline-danger" @click="batchDelete">
            <i class="bi bi-trash"></i> {{ $t('dev_delete') }}
          </button>
        </div>
        <button class="btn btn-link text-muted ms-auto" @click="selectedDevices = []">
          <i class="bi bi-x-lg"></i>
        </button>
      </div>
      <div v-else class="d-flex justify-content-between align-items-center">
        <h5 class="mb-0">{{ $t('sidebar_devices') }}</h5>
        <div class="d-flex gap-2">
          <button class="btn btn-outline-primary btn-sm" @click="downloadTemplate">
            <i class="bi bi-download me-1"></i> {{ $t('download_template') }}
          </button>
          <button class="btn btn-outline-primary btn-sm" @click="triggerImport">
            <i class="bi bi-upload me-1"></i> {{ $t('import_devices') }}
          </button>
          <input type="file" ref="fileInput" class="d-none" accept=".xlsx" @change="handleFileUpload">
          <button class="btn btn-primary btn-sm" @click="openCreateModal">
            <i class="bi bi-plus-lg me-1"></i> {{ $t('dev_create') }}
          </button>
          <button class="btn btn-outline-warning btn-sm fw-bold" @click="openAIBatchConfigModal">
            <i class="bi bi-shield-check me-1"></i> AI 批量配置
          </button>
          <button class="btn btn-outline-info btn-sm" @click="showDiscoveryModal = true" :title="$t('discover_devices')">
            <i class="bi bi-search"></i>
          </button>
        </div>
      </div>

      <!-- Filters -->
      <div class="row g-2 mt-3">
        <div class="col-md-3">
          <select class="form-select form-select-sm" v-model="filterProduct">
            <option value="">{{ $t('dev_product') }}: {{ $t('all') }}</option>
            <option v-for="p in products" :key="p.code" :value="p.code">{{ p.name }}</option>
          </select>
        </div>
        <div class="col-md-3">
          <select class="form-select form-select-sm" v-model="filterParent">
            <option value="">{{ $t('dev_parent') }}: {{ $t('all') }}</option>
            <option v-for="p in uniqueParents" :key="p.code" :value="p.code">{{ p.name }}</option>
          </select>
        </div>
        <div class="col-md-3">
          <select class="form-select form-select-sm" v-model="filterEnabled">
             <option value="">{{ $t('dev_status') }}: {{ $t('all') }}</option>
             <option value="true">{{ $t('dev_enabled') }}</option>
             <option value="false">{{ $t('dev_disabled') }}</option>
          </select>
        </div>
        <div class="col-md-3">
          <select class="form-select form-select-sm" v-model="filterOnline">
             <option value="">{{ $t('dev_online_status') }}: {{ $t('all') }}</option>
             <option value="true">{{ $t('dev_online') }}</option>
             <option value="false">{{ $t('dev_offline') }}</option>
          </select>
        </div>
      </div>
    </div>
    <div class="card-body p-0">
      <div class="table-responsive" style="min-height: 400px">
        <table class="table table-hover align-middle mb-0">
          <thead class="bg-light">
            <tr>
              <th class="ps-4" style="width: 40px">
                <input class="form-check-input" type="checkbox" :checked="allSelected" @change="toggleAll">
              </th>
              <th>{{ $t('dev_code') }}</th>
              <th>{{ $t('dev_name') }}</th>
              <th>{{ $t('dev_online_status') }}</th>
              <th>{{ $t('dev_product') }}</th>
              <th class="d-none d-lg-table-cell">{{ $t('dev_parent') }}</th>
              <th class="d-none d-xl-table-cell" style="font-size: 0.8rem; color: #6c757d;">
                <div style="line-height: 1.2;">{{ $t('dev_created') }}</div>
                <div style="line-height: 1.2;">{{ $t('dev_updated') }}</div>
              </th>
              <th>AI 健康</th>
              <th>{{ $t('dev_status') }}</th>
              <th class="text-end pe-4">{{ $t('tsl_actions') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-if="loading" class="text-center">
              <td colspan="11" class="py-4 text-muted">{{ $t('loading') }}</td>
            </tr>
            <tr v-else-if="filteredDevices.length === 0" class="text-center">
              <td colspan="11" class="py-4 text-muted">{{ $t('dev_no_devices') }}</td>
            </tr>
            <tr 
              v-for="device in paginatedDevices" 
              :key="device.code"
              style="cursor: pointer"
            >
              <td class="ps-4">
                <input class="form-check-input" type="checkbox" :checked="selectedDevices.includes(device.code)" @change="toggleSelection(device.code)">
              </td>
              <td class="font-monospace fw-bold text-primary" @mouseenter="showHoverData(device, $event)" @mouseleave="hideHoverData">{{ device.code }}</td>
              <td @mouseenter="showHoverData(device, $event)" @mouseleave="hideHoverData">{{ device.name || '-' }}</td>
              <td @mouseenter="showHoverData(device, $event)" @mouseleave="hideHoverData">
                <span class="badge rounded-pill" :class="device.online ? 'bg-success' : 'bg-secondary'">
                  <i class="bi me-1" :class="device.online ? 'bi-circle-fill' : 'bi-circle-fill text-white-50'"></i>
                  {{ device.online ? $t('dev_online') : $t('dev_offline') }}
                </span>
                <div v-if="device.last_active && new Date(device.last_active).getFullYear() > 1" class="small text-muted mt-1" style="font-size: 0.7rem">
                   {{ new Date(device.last_active).toLocaleString() }}
                </div>
              </td>
              <td @mouseenter="showHoverData(device, $event)" @mouseleave="hideHoverData"><span class="badge bg-light text-body border">{{ getProductName(device.product_code) }}</span></td>
              <td class="d-none d-lg-table-cell" @mouseenter="showHoverData(device, $event)" @mouseleave="hideHoverData">
                <span v-if="device.parent_code" class="text-muted small">
                  <i class="bi bi-arrow-return-right"></i> {{ getDeviceName(device.parent_code) }}
                </span>
                <span v-else class="text-muted">-</span>
              </td>
              <td class="d-none d-xl-table-cell small text-muted" style="font-size: 0.75rem;" @mouseenter="showHoverData(device, $event)" @mouseleave="hideHoverData">
                <div>{{ device.CreatedAt ? new Date(device.CreatedAt).toLocaleString() : '-' }}</div>
                <div>{{ device.UpdatedAt ? new Date(device.UpdatedAt).toLocaleString() : '-' }}</div>
              </td>
              <td @click.stop="openSingleAIModal(device)" style="position: relative;">
                <!-- 在线 + 有得分 -->
                <div v-if="device.online && device.ai_health_score !== undefined && device.ai_health_score !== null"
                     class="d-flex align-items-center ai-health-cell"
                     :class="device.ai_latched ? 'text-danger' : (device.ai_health_score > 80 ? 'text-success' : (device.ai_health_score > 60 ? 'text-warning' : 'text-danger'))"
                     @mouseenter="showHealthTooltip(device, $event)"
                     @mouseleave="hideHealthTooltip">
                  <i :class="device.ai_latched ? 'bi bi-shield-fill-x' : 'bi bi-shield-fill-check'" class="fs-5 me-1"></i>
                  <span class="fw-bold">{{ device.ai_latched ? (device.ai_health_trigger != null ? device.ai_health_trigger.toFixed(1) : '异常') : device.ai_health_score.toFixed(1) }}</span>
                  <i v-if="device.ai_latched" class="bi bi-lock-fill ms-1" title="异常锁定"></i>
                </div>
                <!-- 离线 + 已配置AI设备守护 -->
                <div v-else-if="!device.online && (device.ai_health_details || configuredDeviceCodes.has(device.code))"
                     class="d-flex align-items-center text-secondary ai-health-cell"
                     style="opacity: 0.5"
                     @mouseenter="showHealthTooltip(device, $event)"
                     @mouseleave="hideHealthTooltip">
                  <i class="bi bi-shield-slash fs-5 me-1"></i>
                  <span class="small">离线</span>
                </div>
                <!-- 已配置但还在生成中 (无得分) -->
                <div v-else-if="configuredDeviceCodes.has(device.code)" 
                     class="d-flex align-items-center text-primary ai-health-cell"
                     style="opacity: 0.8"
                     title="AI守护已配置，正在生成初始数据...">
                  <i class="bi bi-shield-check fs-5 me-1 animation-blink"></i>
                  <span class="small">生成中...</span>
                </div>
                <!-- 未配置 -->
                <div v-else class="text-muted small d-flex align-items-center" title="点击配置 AI 设备守护" style="opacity: 0.6">
                  <i class="bi bi-shield me-1"></i>
                  未配置
                </div>
              </td>
              <td>
                <div class="form-check form-switch">
                  <input class="form-check-input" type="checkbox" role="switch" :checked="device.enabled" @click.prevent="toggleDevice(device)">
                  <label class="form-check-label small text-muted ms-1">{{ device.enabled ? $t('dev_enabled') : $t('dev_disabled') }}</label>
                </div>
              </td>
              <td class="text-end pe-4">
                <div class="dropdown">
                  <button class="btn btn-sm btn-light border-0" type="button" data-bs-toggle="dropdown" aria-expanded="false" data-bs-boundary="viewport" data-bs-popper-config='{"strategy":"fixed"}'>
                    <i class="bi bi-three-dots-vertical"></i>
                  </button>
                  <ul class="dropdown-menu dropdown-menu-end shadow-sm border-0">
                    <li><hr class="dropdown-divider"></li>
                    <li>
                      <a class="dropdown-item" href="#" @click="openDataModal(device, 'realtime')">
                        <i class="bi bi-activity me-2 text-success"></i> {{ $t('dev_data') }}
                      </a>
                    </li>
                    <li>
                      <a class="dropdown-item" href="#" @click="openSingleAIModal(device)">
                        <i class="bi bi-shield-check me-2 text-warning"></i> AI 设备守护 (Device Guardian)
                      </a>
                    </li>
                    <li>
                      <a class="dropdown-item" href="#" @click="toggleDevice(device)">
                        <i class="bi me-2" :class="device.enabled ? 'bi-stop-fill text-warning' : 'bi-play-fill text-success'"></i>
                        {{ device.enabled ? $t('stop') : $t('start') }}
                      </a>
                    </li>
                    <li v-if="!device.parent_code || isChildOfCascade(device)">
                      <a class="dropdown-item" href="#" @click="openCreateSubDeviceModal(device)">
                        <i class="bi bi-plus-square me-2 text-primary"></i> {{ $t('dev_create_sub') }}
                      </a>
                    </li>
                    <li>
                      <a class="dropdown-item" href="#" @click="openEditModal(device)">
                        <i class="bi bi-pencil me-2 text-info"></i> {{ $t('dev_edit') }}
                      </a>
                    </li>
                    <li v-if="needsProtocolMapping(device)">
                      <a class="dropdown-item" href="#" @click="openMappingModal(device)">
                        <i class="bi bi-diagram-3 me-2 text-body"></i> {{ $t('tsl_prop_proto_map') }}
                      </a>
                    </li>
                    <li><hr class="dropdown-divider"></li>
                    <li>
                      <a class="dropdown-item text-danger" href="#" @click="deleteDevice(device)">
                        <i class="bi bi-trash me-2"></i> {{ $t('dev_delete') }}
                      </a>
                    </li>
                  </ul>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
    <div class="card-footer bg-transparent border-0 d-flex justify-content-end align-items-center py-3" v-if="total > 0">
      <div class="d-flex align-items-center gap-2">
        <select class="form-select form-select-sm" style="width: auto" v-model="pageSize" @change="changePageSize">
          <option :value="10">10 / {{ $t('page') }}</option>
          <option :value="20">20 / {{ $t('page') }}</option>
          <option :value="50">50 / {{ $t('page') }}</option>
        </select>
        <nav>
          <ul class="pagination pagination-sm mb-0">
            <li class="page-item disabled me-2 d-flex align-items-center">
              <span class="text-muted small border-0 bg-transparent">共 {{ total }} 条</span>
            </li>
            <li class="page-item" :class="{ disabled: page === 1 }">
              <button class="page-link" @click="changePage(page - 1)">
                <i class="bi bi-chevron-left"></i>
              </button>
            </li>
            <li class="page-item disabled">
              <span class="page-link">{{ page }} / {{ Math.ceil(total / pageSize) }}</span>
            </li>
            <li class="page-item" :class="{ disabled: page * pageSize >= total }">
              <button class="page-link" @click="changePage(page + 1)">
                <i class="bi bi-chevron-right"></i>
              </button>
            </li>
          </ul>
        </nav>
      </div>
    </div>

    <!-- Hover Tooltip -->
    <div 
      v-if="hoveredDevice" 
      class="card shadow-lg position-fixed border-0 bg-body-tertiary" 
      style="z-index: 1050; width: 380px; font-size: 0.9rem; pointer-events: none; --bs-bg-opacity: 0.9; backdrop-filter: blur(10px);"
      :style="{ top: tooltipPos.top + 'px', left: tooltipPos.left + 'px' }"
    >
      <div class="card-header py-2 border-bottom bg-transparent fw-bold d-flex justify-content-between align-items-center">
        <span class="text-body-emphasis">{{ hoveredDevice.name }}</span>
        <span class="badge rounded-pill" :class="hoveredDevice.online ? 'bg-success-subtle text-success border border-success-subtle' : 'bg-secondary-subtle text-secondary border border-secondary-subtle'">
           {{ hoveredDevice.online ? $t('dev_online') : $t('dev_offline') }}
        </span>
      </div>
      <div class="card-body p-2">
        <div v-if="hoverDisplayList.length === 0" class="text-muted text-center py-2">{{ $t('tsl_no_data') }}</div>
        <table v-else class="table table-sm table-borderless mb-0 align-middle">
           <tbody>
             <tr v-for="item in hoverDisplayList" :key="item.key">
               <td class="text-muted" :title="item.key" style="width: 30%">{{ item.name }}</td>
               <td style="width: 30%">
                  <Sparkline :data="item.trend" :width="80" :height="20" color="#6c757d" />
               </td>
               <td class="text-end fw-bold" style="width: 40%" :class="hoveredDevice.online ? '' : 'text-warning'">
                 {{ item.value }} <span v-if="item.unit" class="text-muted fw-normal small">{{ item.unit }}</span>
               </td>
             </tr>
           </tbody>
        </table>
      </div>
    </div>

    <!-- AI Health Tooltip -->
    <div 
      v-if="healthTooltipDevice" 
      class="card shadow-lg position-fixed border-0" 
      style="z-index: 1060; width: 240px; font-size: 0.85rem; pointer-events: none; backdrop-filter: blur(12px); background: rgba(var(--bs-body-bg-rgb), 0.92);"
      :style="{ top: healthTooltipPos.top + 'px', left: healthTooltipPos.left + 'px' }"
    >
      <div class="card-body py-2 px-3">
        <!-- Online with scores -->
        <template v-if="healthTooltipDevice.online && healthTooltipDevice.ai_health_details">
          <div class="fw-bold mb-2 d-flex justify-content-between align-items-center">
            <span>AI 健康：{{ healthTooltipDevice.ai_health_score?.toFixed(1) }}</span>
            <i class="bi bi-shield-fill-check" :class="healthTooltipDevice.ai_health_score > 80 ? 'text-success' : (healthTooltipDevice.ai_health_score > 60 ? 'text-warning' : 'text-danger')"></i>
          </div>
          <hr class="my-1 opacity-25">
          <div v-for="(score, prop) in healthTooltipDevice.ai_health_details" :key="prop" class="d-flex justify-content-between align-items-center py-1">
            <span class="text-muted">
              <i class="bi bi-circle-fill me-1" style="font-size: 0.5rem;" :class="score > 80 ? 'text-success' : (score > 60 ? 'text-warning' : 'text-danger')"></i>
              {{ prop }}
            </span>
            <span class="fw-bold" :class="score > 80 ? 'text-success' : (score > 60 ? 'text-warning' : 'text-danger')">{{ score.toFixed ? score.toFixed(1) : score }}</span>
          </div>
          <hr class="my-1 opacity-25">
          <div class="text-muted text-center" style="font-size: 0.75rem;">综合取最低值</div>
        </template>
        <!-- Offline -->
        <template v-else>
          <div class="text-secondary text-center py-1">
            <i class="bi bi-shield-slash fs-5 d-block mb-1"></i>
            <div class="fw-bold">设备离线</div>
            <div v-if="healthTooltipDevice.last_active && new Date(healthTooltipDevice.last_active).getFullYear() > 1" class="small mt-1" style="font-size: 0.75rem;">
              最后在线: {{ new Date(healthTooltipDevice.last_active).toLocaleString() }}
            </div>
          </div>
        </template>
      </div>
    </div>

    <!-- Create Modal -->
    <div v-if="showCreateModal" class="modal fade show d-block" style="background: rgba(0,0,0,0.5)">
      <div class="modal-dialog modal-lg">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title">{{ isEditing ? $t('dev_edit') : $t('dev_create') }}</h5>
            <button type="button" class="btn-close" @click="showCreateModal = false"></button>
          </div>
          <div class="modal-body">
            <div class="row">
              <div class="col-md-6">
                <div class="mb-3">
                  <label class="form-label">{{ $t('dev_product') }}</label>
                  <select v-model="newDevice.product_code" class="form-select" :disabled="isEditing" @change="handleProductChange">
                    <option value="" disabled>{{ $t('dev_select_prod_hint') }}</option>
                    <option v-for="p in products" :key="p.code" :value="p.code">{{ p.name }} ({{ p.code }})</option>
                  </select>
                  <div v-if="selectedProductNoProtocol && (!newDevice.parent_code || isChildOfCascade(newDevice))" class="form-text text-warning">
                    <i class="bi bi-exclamation-triangle me-1"></i>
                    {{ $t('prod_no_protocol_hint') }}
                  </div>
                </div>
                <div class="mb-3">
                  <label class="form-label">{{ $t('dev_code') }}</label>
                  <input v-model="newDevice.code" type="text" class="form-control" :disabled="isEditing">
                </div>
                <div class="mb-3">
                  <label class="form-label">{{ $t('dev_name') }}</label>
                  <input v-model="newDevice.name" type="text" class="form-control">
                </div>
                <div class="mb-3">
                  <label class="form-label">{{ $t('dev_parent') }} ({{ $t('optional') }})</label>
                  <select v-model="newDevice.parent_code" class="form-select">
                    <option value="">{{ $t('none') }}</option>
                    <option v-for="d in devices.filter(d => (!d.parent_code || isChildOfCascade(d)) && d.code !== newDevice.code)" :key="d.code" :value="d.code">{{ d.name || d.code }}</option>
                  </select>
                  <div class="form-text small text-muted">{{ $t('dev_parent_hint') }}</div>
                </div>
                <div class="form-check mb-3">
                  <input class="form-check-input" type="checkbox" v-model="newDevice.enabled" id="enableCheck">
                  <label class="form-check-label" for="enableCheck">
                    {{ $t('dev_enable_now') }}
                  </label>
                </div>
              </div>
              <div class="col-md-6 border-start">
                <h6 class="mb-3">{{ $t('dev_config') }}</h6>
                <div v-if="currentSchema">
                   <SchemaForm :schema="currentSchema" v-model="newDevice.config" />
                </div>
                <div v-else class="text-muted small">
                  {{ $t('dev_select_prod_hint') }}
                </div>
              </div>
            </div>
          </div>
          <div class="modal-footer">
            <button type="button" class="btn btn-secondary" @click="showCreateModal = false">{{ $t('tsl_cancel') }}</button>
            <button type="button" class="btn btn-primary" @click="saveDevice">{{ $t('tsl_confirm') }}</button>
          </div>
        </div>
      </div>
    </div>

    <!-- Mapping Modal -->
    <div v-if="showMappingModal" class="modal fade show d-block" style="background: rgba(0,0,0,0.5)">
      <div class="modal-dialog modal-xl">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title">{{ $t('tsl_prop_proto_map') }} - {{ currentMappingDevice?.code }}</h5>
            <button type="button" class="btn-close" @click="closeMappingModal"></button>
          </div>
          <div class="modal-body">
             <DeviceMappingEditor 
               :deviceConfig="currentMappingDeviceConfig"
               :tslProperties="currentMappingProperties"
               :tslEvents="currentMappingEvents"
               :pollingGroups="currentMappingPollingGroups"
               :protocolName="currentMappingProtocol"
               :isSubDevice="!!currentMappingDevice.parent_code && !isParentCascade"
               :productCode="currentMappingDevice?.product_code"
               :parentCode="currentMappingDevice?.parent_code"
               @update:deviceConfig="updateDeviceMappingConfig"
             />
          </div>
          <div class="modal-footer">
             <button type="button" class="btn btn-secondary" style="min-width: 80px" @click="closeMappingModal">{{ $t('tsl_cancel') }}</button>
             <button type="button" class="btn btn-primary" style="min-width: 80px" @click="saveDeviceMapping">{{ $t('common_save') }}</button>
          </div>
        </div>
      </div>
    </div>

    <!-- Single AI Config Modal -->
    <div v-if="singleAIModalVisible" class="modal fade show d-block" style="background: rgba(0,0,0,0.5); z-index: 1060;">
      <div class="modal-dialog modal-xl">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title">
              <i class="bi bi-shield-check text-warning me-2"></i>
              AI 设备守护 - {{ currentSingleAIDevice?.code }}
            </h5>
            <button type="button" class="btn-close" @click="closeSingleAIModal"></button>
          </div>
          <div class="modal-body">
            <div class="row h-100">
              <!-- Left side: Form -->
              <div class="col-md-4 border-end">
                <h6 class="mb-3 fw-bold">守护参数配置</h6>
                <div class="mb-3">
                  <label class="form-label small fw-bold">监控属性 (数值型)</label>
                  <select class="form-select form-select-sm" v-model="aiConfig.property">
                     <option value="" disabled>-- 请选择 --</option>
                     <option v-for="prop in getSingleAINumericProperties()" :key="prop.key" :value="prop.key">
                       {{ prop.name }} ({{ prop.key }})
                     </option>
                  </select>
                </div>
                <div class="mb-3">
                  <label class="form-label small fw-bold">
                    输入序列窗长
                    <i class="bi bi-info-circle text-muted ms-1" title="AI 分析趋势时参考的最近历史数据点数量。设置越大，参考的背景趋势越久。"></i>
                  </label>
                  <div class="d-flex justify-content-between mb-1">
                     <span class="text-muted small">回顾时长</span>
                     <span class="text-muted small fw-bold">{{ aiConfig.window_size }} pt</span>
                  </div>
                  <input type="range" class="form-range" min="10" max="200" step="10" v-model.number="aiConfig.window_size">
                </div>
                <div class="mb-3">
                  <label class="form-label small fw-bold">
                    异常判定阈值 ($\sigma$)
                    <i class="bi bi-info-circle text-muted ms-1" title="衡量波动是否异常的标准差倍数。设置越小越灵敏（易报），设置越大越宽松（稳健）。"></i>
                  </label>
                  <div class="d-flex justify-content-between mb-1">
                     <span class="text-muted small">灵敏度因子</span>
                     <span class="text-muted small fw-bold">{{ aiConfig.threshold_sigma }}</span>
                  </div>
                  <input type="range" class="form-range" min="1.0" max="10.0" step="0.5" v-model.number="aiConfig.threshold_sigma">
                </div>
                <div class="form-check form-switch mt-4">
                  <input class="form-check-input" type="checkbox" role="switch" id="singleEnableSwitch" v-model="aiConfig.enabled">
                  <label class="form-check-label text-warning fw-bold" for="singleEnableSwitch">启用 AI 设备守护</label>
                </div>
              </div>
              
              <!-- Right side: Chart -->
              <div class="col-md-8">
                <div class="d-flex justify-content-between align-items-center mb-2">
                  <h6 class="mb-0 fw-bold">实时运行状态</h6>
                  <div class="small d-flex align-items-center">
                    健康得分:
                    <span v-if="aiLatestLatched"
                          :class="(aiLatchTriggerScore ?? 0) > 80 ? 'text-success' : ((aiLatchTriggerScore ?? 0) > 60 ? 'text-warning' : 'text-danger')"
                          class="fw-bold fs-5 mx-2">
                      {{ aiLatchTriggerScore != null ? aiLatchTriggerScore.toFixed(1) : '异常' }}
                    </span>
                    <span v-else-if="aiLatestHealth !== null" :class="aiLatestHealth > 80 ? 'text-success' : (aiLatestHealth > 60 ? 'text-warning' : 'text-danger')" class="fw-bold fs-5 mx-2">
                      {{ aiLatestHealth.toFixed(1) }}
                    </span>
                    <span v-else class="fw-bold fs-5 mx-2 text-muted">-</span>
                    <span v-if="aiLatestLatched" class="badge bg-warning text-dark me-2">
                       <i class="bi bi-lock-fill"></i> 异常锁定
                    </span>
                    <span v-else-if="aiLatestAnomaly" class="badge bg-danger animation-blink me-2">异常警告</span>
                    
                    <button v-if="aiLatestLatched" class="btn btn-sm btn-outline-danger py-0 px-2" style="font-size: 0.75rem" @click="clearLatchedState" title="解除异常锁定状态">
                      <i class="bi bi-unlock"></i> 解除
                    </button>
                  </div>
                </div>
                <div v-if="aiLatestLatched" class="alert alert-warning py-2 px-3 mb-2 d-flex align-items-center" style="font-size: 0.85rem">
                  <i class="bi bi-exclamation-triangle-fill me-2"></i>
                  设备处于异常锁定状态，健康检测已暂停。请检查设备后手动解除锁定以恢复监控。
                </div>
                <div class="border rounded bg-light position-relative" style="height: 350px;">
                  <div v-if="aiChartLoading && (!aiChartOption || !aiChartOption.series)" class="position-absolute top-0 start-0 w-100 h-100 d-flex align-items-center justify-content-center bg-white bg-opacity-75" style="z-index: 10">
                     <div class="spinner-border text-primary" role="status"></div>
                  </div>
                  <VChart v-if="aiChartOption && aiChartOption.series && aiChartOption.series.length > 0" :option="aiChartOption" autoresize style="width: 100%; height: 100%;" />
                  <div v-else class="h-100 d-flex align-items-center justify-content-center text-muted">
                    暂无 AI 数据，请配置并开启设备守护
                  </div>
                </div>
              </div>
            </div>
          </div>
          <div class="modal-footer">
            <button type="button" class="btn btn-outline-info me-auto" @click="openAIHistoryModal">
               <i class="bi bi-clock-history me-1"></i> 故障历史
            </button>
            <button type="button" class="btn btn-secondary" @click="closeSingleAIModal">取消</button>
            <button type="button" class="btn btn-primary" @click="saveSingleAIConfig">
              <i class="bi bi-save me-1"></i> 保存并应用配置
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- AI History Modal -->
    <div v-if="aiHistoryModalVisible" class="modal fade show d-block" style="background: rgba(0,0,0,0.5); z-index: 1070;">
      <div class="modal-dialog modal-lg">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title"><i class="bi bi-clock-history text-danger me-2"></i>AI 故障历史记录</h5>
            <button type="button" class="btn-close" @click="aiHistoryModalVisible = false"></button>
          </div>
          <div class="modal-body">
             <div v-if="aiHistoryLoading" class="text-center py-5">
               <div class="spinner-border text-primary" role="status"></div>
               <div class="mt-2 text-muted">正在加载历史记录...</div>
             </div>
             <div v-else-if="aiHistoryEvents.length === 0" class="text-center py-5 text-muted">
               <i class="bi bi-check-circle fs-1 text-success mb-2"></i>
               <div>近7天内未检测到设备异常</div>
             </div>
             <div v-else>
               <div class="table-responsive">
                 <table class="table table-striped table-hover small">
                   <thead>
                     <tr>
                       <th>发生时间</th>
                       <th>监控属性</th>
                       <th>健康评分</th>
                       <th>原始值</th>
                       <th>残差值</th>
                       <th>阈值 ($\sigma$)</th>
                     </tr>
                   </thead>
                   <tbody>
                     <tr v-for="(evt, idx) in aiHistoryEvents" :key="idx">
                       <td>{{ new Date(evt.ts).toLocaleString() }}</td>
                       <td>{{ evt.property }}</td>
                        <td>
                          <span :class="evt.health_score > 60 ? 'text-warning' : 'text-danger'" class="fw-bold">
                            {{ evt.health_score !== undefined ? evt.health_score.toFixed(1) : '-' }}
                          </span>
                        </td>
                        <td>{{ evt.current_value !== undefined ? evt.current_value.toFixed(2) : '-' }}</td>
                        <td class="text-danger">{{ evt.residual !== undefined ? evt.residual.toFixed(4) : '-' }}</td>
                        <td>{{ evt.threshold_std }}</td>
                      </tr>
                    </tbody>
                  </table>
               </div>
             </div>
          </div>
          <div class="modal-footer">
            <button type="button" class="btn btn-secondary" @click="aiHistoryModalVisible = false">关闭</button>
          </div>
        </div>
      </div>
    </div>

    <!-- AI Batch Config Modal -->
    <div v-if="showBatchAIModal" class="modal fade show d-block" style="background: rgba(0,0,0,0.5); z-index: 1060;">
      <div class="modal-dialog modal-lg">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title"><i class="bi bi-shield-check text-warning me-2"></i>AI 设备守护批量配置</h5>
            <button type="button" class="btn-close" @click="showBatchAIModal = false"></button>
          </div>
          <div class="modal-body">
            <div class="alert alert-info py-2 small">
               <i class="bi bi-info-circle me-1"></i> 批量为统一产品下的多个设备下发相同的 AI 设备守护参数。
            </div>
            
            <div class="row g-3">
               <div class="col-md-6">
                  <label class="form-label fw-bold small">所属产品</label>
                  <select class="form-select" v-model="batchAiConfig.product_code" @change="onBatchProductChange">
                     <option value="" disabled>-- 请选择产品 --</option>
                     <option v-for="p in products" :key="p.code" :value="p.code">{{ p.name }} ({{ p.code }})</option>
                  </select>
               </div>
               <div class="col-md-6">
                  <label class="form-label fw-bold small">监控属性 (数值型)</label>
                  <select class="form-select" v-model="batchAiConfig.property" :disabled="!batchAiConfig.product_code">
                     <option value="" disabled>-- 请选择 --</option>
                     <option v-for="prop in batchProductProperties" :key="prop.key" :value="prop.key">
                       {{ prop.name }} ({{ prop.key }})
                     </option>
                  </select>
               </div>
            </div>

            <div class="mt-4" v-if="batchAiConfig.product_code">
               <label class="form-label fw-bold small">选择目标应用设备 (可多选)</label>
               <div class="border rounded p-2 bg-light" style="max-height: 200px; overflow-y: auto;">
                  <div v-if="batchDeviceList.length === 0" class="text-muted text-center py-3 small">该产品下暂无设备</div>
                  <div class="form-check" v-for="dev in batchDeviceList" :key="dev.code">
                     <input class="form-check-input" type="checkbox" :value="dev.code" v-model="batchAiConfig.devices" :id="'batch_dev_' + dev.code">
                     <label class="form-check-label" :for="'batch_dev_' + dev.code">
                       {{ dev.name || dev.code }} <span class="text-muted small">({{ dev.code }})</span>
                     </label>
                  </div>
               </div>
               <div class="mt-2 text-primary small d-flex justify-content-between">
                  <span>
                    <a href="#" class="text-decoration-none me-3" @click.prevent="batchAiConfig.devices = batchDeviceList.map(d=>d.code)">全选</a>
                    <a href="#" class="text-decoration-none" @click.prevent="batchAiConfig.devices = []">反选</a>
                  </span>
                  <span>已选择 {{ batchAiConfig.devices.length }} 个设备</span>
               </div>
            </div>

            <div class="row g-3 mt-3">
               <div class="col-md-6">
                  <label class="form-label small fw-bold d-flex justify-content-between">
                     <span>输入序列窗长 (Window Size)</span>
                     <span class="text-muted">{{ batchAiConfig.window_size }} pt</span>
                  </label>
                  <input type="range" class="form-range" min="10" max="200" step="10" v-model.number="batchAiConfig.window_size">
               </div>
               <div class="col-md-6">
                  <label class="form-label small fw-bold d-flex justify-content-between">
                     <span>异常判定阈值 ($\sigma$)</span>
                     <span class="text-muted">{{ batchAiConfig.threshold_sigma }}</span>
                  </label>
                  <input type="range" class="form-range" min="1.0" max="10.0" step="0.5" v-model.number="batchAiConfig.threshold_sigma">
               </div>
            </div>
          </div>
          <div class="modal-footer d-flex justify-content-between">
            <div class="form-check form-switch mt-1">
               <input class="form-check-input" type="checkbox" role="switch" id="batchEnableSwitch" v-model="batchAiConfig.enabled">
               <label class="form-check-label text-warning fw-bold" for="batchEnableSwitch">立即启用监控</label>
            </div>
            <div>
               <button type="button" class="btn btn-secondary me-2" @click="showBatchAIModal = false">取消</button>
               <button type="button" class="btn btn-primary" :disabled="!batchAiConfig.product_code || !batchAiConfig.property || batchAiConfig.devices.length === 0" @click="saveBatchAITasks">
                 <i class="bi bi-check2-all me-1"></i> 批量下发配置
               </button>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Data Modal -->
    <div v-if="showDataModal" class="modal fade show d-block" style="background: rgba(0,0,0,0.5)">
      <div class="modal-dialog modal-xl">
        <div class="modal-content" style="height: 90vh; display: flex; flex-direction: column;">
          <div class="modal-header">
            <h5 class="modal-title">{{ $t('dev_data_title') }} - {{ currentDataDevice?.code }}</h5>
            <button type="button" class="btn-close" @click="closeDataModal"></button>
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
            <button type="button" class="btn btn-secondary" style="min-width: 80px" @click="closeDataModal">{{ $t('tsl_cancel') }}</button>
            <button v-if="activeTab === 'realtime'" type="button" class="btn btn-primary" style="min-width: 80px" @click="submitBatchWrite">
              <i class="bi bi-save me-1"></i> {{ $t('dev_data_save_all') }}
            </button>
          </div>
        </div>
      </div>
    </div>
    <!-- Import Device Modal -->
    <div v-if="showImportModal" class="modal fade show d-block" style="background: rgba(0,0,0,0.5)">
      <div class="modal-dialog">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title">{{ $t('import_device') }}</h5>
            <button type="button" class="btn-close" @click="closeImportModal"></button>
          </div>
          <div class="modal-body">
            <div class="mb-3">
              <label class="form-label">{{ $t('select_protocol') }} <span class="text-danger">*</span></label>
              <select class="form-select" v-model="importProtocol">
                <option value="" disabled>{{ $t('select_protocol_hint') }}</option>
                <option v-for="p in availableProtocols" :key="p" :value="p">{{ p }}</option>
              </select>
            </div>
            <div class="mb-3">
              <label class="form-label">{{ $t('select_file') }} <span class="text-danger">*</span></label>
              <input type="file" class="form-control" @change="handleImportFileChange" accept=".xlsx">
              <div class="form-text text-muted">{{ $t('import_file_hint') }}</div>
            </div>
          </div>
          <div class="modal-footer">
            <button type="button" class="btn btn-secondary" @click="closeImportModal">{{ $t('tsl_cancel') }}</button>
            <button type="button" class="btn btn-primary" @click="confirmImport" :disabled="!importProtocol || !importFile">
              <i class="bi bi-upload me-1"></i> {{ $t('import') }}
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- Download Template Modal -->
    <div v-if="showDownloadModal" class="modal fade show d-block" style="background: rgba(0,0,0,0.5)">
      <div class="modal-dialog">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title">{{ $t('download_template') }}</h5>
            <button type="button" class="btn-close" @click="showDownloadModal = false"></button>
          </div>
          <div class="modal-body">
            <div class="mb-3">
              <label class="form-label fw-bold">{{ $t('select_products_prefill') }}</label>
              <div class="form-text text-muted mb-2">{{ $t('select_products_hint') }}</div>
              
              <!-- Protocol Indicator -->
              <div v-if="targetProtocol" class="alert alert-info py-2 mb-2">
                <i class="bi bi-info-circle me-1"></i> {{ $t('dev_protocol') }}: <strong>{{ targetProtocol }}</strong>
              </div>

              <div v-if="products.length === 0" class="text-muted text-center py-3">
                {{ $t('no_products_found') }}
              </div>
              <div v-else>
                <div class="d-flex justify-content-between mb-2">
                  <button class="btn btn-sm btn-outline-secondary" @click="selectAllProducts">
                    {{ $t('select_all') }}
                  </button>
                  <button class="btn btn-sm btn-outline-secondary" @click="deselectAllProducts">
                    {{ $t('deselect_all') }}
                  </button>
                </div>
                <div class="border rounded p-2" style="max-height: 300px; overflow-y: auto;">
                  <div v-for="p in products" :key="p.code" class="form-check">
                    <input class="form-check-input" type="checkbox" :value="p.code" v-model="selectedProductsForTemplate" :id="'prod_check_' + p.code" :disabled="isProductDisabled(p)">
                    <label class="form-check-label" :for="'prod_check_' + p.code" :class="{'text-muted': isProductDisabled(p)}">
                      {{ p.name }} <span class="text-muted small">({{ p.code }})</span>
                      <span class="badge bg-primary-subtle text-primary border border-primary-subtle ms-1" style="font-size: 0.7rem">{{ p.protocol_name }}</span>
                    </label>
                  </div>
                </div>
              </div>
            </div>
          </div>
          <div class="modal-footer">
            <button type="button" class="btn btn-secondary" @click="showDownloadModal = false">{{ $t('tsl_cancel') }}</button>
            <button type="button" class="btn btn-primary" @click="confirmDownloadTemplate" :disabled="!targetProtocol">
              <i class="bi bi-download me-1"></i> {{ $t('download') }}
            </button>
          </div>
        </div>
      </div>
    </div>
    
    <DeviceDiscoveryModal v-if="showDiscoveryModal" @close="showDiscoveryModal = false" @device-added="fetchDevices(false)" />
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted, watch } from 'vue';
import axios from 'axios';
import { useI18n } from 'vue-i18n';
import SchemaForm from '../components/SchemaForm.vue';
import DeviceMappingEditor from '../components/device/DeviceMappingEditor.vue';
import DeviceDiscoveryModal from '../components/device/DeviceDiscoveryModal.vue';
import Sparkline from '../components/Sparkline.vue';
import { use } from 'echarts/core';
import { CanvasRenderer } from 'echarts/renderers';
import { LineChart, BarChart, ScatterChart } from 'echarts/charts';
import {
  TitleComponent,
  TooltipComponent,
  LegendComponent,
  GridComponent,
  DataZoomComponent
} from 'echarts/components';
import VChart from 'vue-echarts';

use([
  CanvasRenderer,
  LineChart,
  BarChart,
  ScatterChart,
  TitleComponent,
  TooltipComponent,
  LegendComponent,
  GridComponent,
  DataZoomComponent
]);

const { t, locale } = useI18n();

const devices = ref([]);
const configuredDeviceCodes = ref(new Set());
const products = ref([]);
const loading = ref(false);
const showCreateModal = ref(false);
const showDiscoveryModal = ref(false);
const newDevice = ref({ code: '', name: '', product_code: '', parent_code: '', enabled: true, config: {} });
const currentSchema = ref(null);
const isEditing = ref(false);
// 检查当前选择的产品是否没有协议
const selectedProductNoProtocol = computed(() => {
  if (!newDevice.value.product_code) return false;
  const p = products.value.find(prod => prod.code === newDevice.value.product_code);
  return p && !p.protocol_name;
});
const selectedDevices = ref([]);
const hoveredDevice = ref(null);
const hoveredData = ref({});
const hoveredTrendData = ref({}); // Store trend data for hover
const hoveredTSLMap = ref({}); // Store TSL info for hover
const tooltipPos = ref({ top: 0, left: 0 });
let hoverTimer = null;
let hoverTrendTimer = null; // Timer for trend data
let hideDebounceTimer = null;

// AI Health Tooltip state
const healthTooltipDevice = ref(null);
const healthTooltipPos = ref({ top: 0, left: 0 });
let healthTooltipTimer = null;

const showHealthTooltip = (device, event) => {
  if (healthTooltipTimer) {
    clearTimeout(healthTooltipTimer);
    healthTooltipTimer = null;
  }
  healthTooltipDevice.value = device;
  const x = event.clientX + 12;
  const y = event.clientY - 10;
  const winWidth = window.innerWidth;
  healthTooltipPos.value = {
    top: y,
    left: x + 250 > winWidth ? x - 260 : x,
  };
};

const hideHealthTooltip = () => {
  healthTooltipTimer = setTimeout(() => {
    healthTooltipDevice.value = null;
  }, 100);
};

// Computed property for rich tooltip data
const hoverDisplayList = computed(() => {
  if (!hoveredData.value) return [];
  const keys = Object.keys(hoveredData.value);
  const list = keys.map(key => {
    const tsl = hoveredTSLMap.value[key];
    return {
      key: key,
      name: tsl ? tsl.name : key,
      unit: tsl && tsl.dataType && tsl.dataType.specs ? tsl.dataType.specs.unit : '',
      value: hoveredData.value[key],
      trend: hoveredTrendData.value[key] || []
    };
  });
  return list.sort((a, b) => a.key.localeCompare(b.key));
});

// Filter State
const filterProduct = ref('');
const filterParent = ref('');
const filterEnabled = ref('');
const filterOnline = ref('');

let eventSource = null;
let sseHeartbeatTimer = null;
let sseFetchDebounceTimer = null;
let sseReconnectTimer = null;

// SSE 防抖拉取：多个事件短时间内触发时只执行一次 fetchDevices
const debouncedSSEFetch = () => {
  if (sseFetchDebounceTimer) clearTimeout(sseFetchDebounceTimer);
  sseFetchDebounceTimer = setTimeout(() => {
    fetchDevices(true);
  }, 300);
};

// SSE 心跳重置：收到任何SSE消息时调用，45秒无消息则重连
const resetSSEHeartbeat = () => {
  if (sseHeartbeatTimer) clearTimeout(sseHeartbeatTimer);
  sseHeartbeatTimer = setTimeout(() => {
    console.warn('[SSE] Heartbeat timeout, reconnecting...');
    reconnectSSE();
  }, 45000); // 后端心跳间隔15秒，给3倍容忍
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

  // 后端发送的初始连接确认事件
  eventSource.addEventListener('connected', () => {
    console.log('[SSE] Connected to device stream');
    resetSSEHeartbeat();
  });

  const handleSSE = () => {
    resetSSEHeartbeat();
    debouncedSSEFetch();
  };

  eventSource.addEventListener('device.list.changed', handleSSE);
  eventSource.addEventListener('device.status.changed', handleSSE);

  // 后端每15秒发送一次心跳事件，用于前端检测连接是否存活
  eventSource.addEventListener('heartbeat', () => {
    resetSSEHeartbeat();
  });

  // 捕获所有消息（包括心跳 comment 会触发 onmessage 但不会触发 addEventListener）
  // 注意：SSE comment (:keepalive) 不会触发任何JS事件，
  // 但如果连接断开，EventSource 会触发 onerror

  eventSource.onerror = (err) => {
    console.warn('[SSE] Connection error, will auto-reconnect', err);
    // EventSource 自动重连，但我们也重置心跳
    // 如果 readyState 是 CLOSED (2)，需要手动重建
    if (eventSource && eventSource.readyState === EventSource.CLOSED) {
      console.warn('[SSE] Connection closed, scheduling manual reconnect');
      sseReconnectTimer = setTimeout(() => {
        setupSSE();
      }, 3000);
    } else {
      // CONNECTING 状态，EventSource 正在自动重连
      resetSSEHeartbeat();
    }
  };

  eventSource.onopen = () => {
    console.log('[SSE] Connection opened');
    resetSSEHeartbeat();
    // 连接恢复后立即拉取最新状态
    fetchDevices(true);
  };
};

const reconnectSSE = () => {
  if (eventSource) {
    eventSource.close();
    eventSource = null;
  }
  // 重连前先拉取一次最新数据
  fetchDevices(true);
  sseReconnectTimer = setTimeout(() => {
    setupSSE();
  }, 1000);
};

// 分页状态
const page = ref(1);
const pageSize = ref(10);
const total = computed(() => filteredDevices.value.length);

const paginatedDevices = computed(() => {
  const start = (page.value - 1) * pageSize.value;
  const end = start + pageSize.value;
  return filteredDevices.value.slice(start, end);
});

const changePage = (p) => {
    if (p < 1 || p > Math.ceil(total.value / pageSize.value)) return;
    page.value = p;
};

const changePageSize = () => {
  page.value = 1;
};

// 过滤器变更时重置页码
watch([filterProduct, filterParent, filterEnabled, filterOnline], () => {
    page.value = 1;
});

const uniqueParents = computed(() => {
  const parents = new Set();
  devices.value.forEach(d => {
    if (d.parent_code) parents.add(d.parent_code);
  });
  return Array.from(parents).map(code => {
      const p = devices.value.find(d => d.code === code);
      return { code, name: p ? (p.name || code) : code };
  });
});

const filteredDevices = computed(() => {
  return devices.value.filter(d => {
    if (filterProduct.value && d.product_code !== filterProduct.value) return false;
    if (filterParent.value) {
        // Show parent itself OR its children
        if (d.code !== filterParent.value && d.parent_code !== filterParent.value) return false;
    }
    if (filterEnabled.value !== '') {
        const want = filterEnabled.value === 'true';
        if (d.enabled !== want) return false;
    }
    if (filterOnline.value !== '') {
        const want = filterOnline.value === 'true';
        if (d.online !== want) return false;
    }
    return true;
  });
});

const allSelected = computed(() => {
  return filteredDevices.value.length > 0 && selectedDevices.value.length === filteredDevices.value.length;
});

const toggleSelection = (code) => {
  const idx = selectedDevices.value.indexOf(code);
  if (idx === -1) {
    selectedDevices.value.push(code);
  } else {
    selectedDevices.value.splice(idx, 1);
  }
};

const toggleAll = () => {
  if (allSelected.value) {
    selectedDevices.value = [];
  } else {
    selectedDevices.value = filteredDevices.value.map(d => d.code);
  }
};

const getProductName = (code) => {
  const p = products.value.find(prod => prod.code === code);
  return p ? `${p.name} (${p.code})` : code;
};

const batchDelete = async () => {
  if (!confirm(t('common_delete_confirm'))) return;
  loading.value = true;
  for (const code of selectedDevices.value) {
    try {
      await axios.delete(`/api/devices/${code}`);
    } catch (e) {
      console.error(e);
    }
  }
  selectedDevices.value = [];
  fetchDevices();
};

const batchEnable = async () => {
  loading.value = true;
  for (const code of selectedDevices.value) {
    try {
      await axios.post(`/api/devices/${code}/start`);
    } catch (e) { console.error(e); }
  }
  selectedDevices.value = [];
  fetchDevices();
};

const batchDisable = async () => {
  loading.value = true;
  for (const code of selectedDevices.value) {
    try {
      await axios.post(`/api/devices/${code}/stop`);
    } catch (e) { console.error(e); }
  }
  selectedDevices.value = [];
  fetchDevices();
};

// Data Modal State
const showDataModal = ref(false);
const currentDataDevice = ref(null);
const deviceData = ref({});
const realtimeTrendData = ref({}); // key -> array of values
const realtimeTrendLoading = ref(false);
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

// AI Predictive Maintenance State
const aiConfig = ref({
  enabled: false,
  property: '',
  window_size: 50,
  prediction_length: 1,
  threshold_sigma: 3.5,
  is_calibrated: false,
});
const aiLatestHealth = ref(null);
const aiLatestAnomaly = ref(false);
const aiLatestLatched = ref(false);
const aiLatchTriggerScore = ref(null);
const aiChartOption = ref({});
const aiChartLoading = ref(false);
let aiChartTimer = null;
let aiConfigSaveTimer = null;
const singleAIModalVisible = ref(false);
const currentSingleAIDevice = ref(null);
const singleAITSLMap = ref({});

// AI History Modal State
const aiHistoryModalVisible = ref(false);
const aiHistoryEvents = ref([]);
const aiHistoryLoading = ref(false);

const openAIHistoryModal = async () => {
    if (!currentSingleAIDevice.value) return;
    aiHistoryModalVisible.value = true;
    fetchAIHistoryEvents();
};

const fetchAIHistoryEvents = async () => {
    if (!currentSingleAIDevice.value) return;
    aiHistoryLoading.value = true;
    try {
        const res = await axios.get(`/api/devices/${currentSingleAIDevice.value.code}/events`, {
            params: {
                type: 2, // Event
                start: Date.now() - 7 * 24 * 3600 * 1000, // Last 7 days
                end: Date.now(),
                page: 1,
                pageSize: 50
            }
        });
        if (res.data.code === 0 && res.data.data) {
            aiHistoryEvents.value = res.data.data.map(item => {
                return {
                    ts: item.ts,
                    eventId: item.event_id,
                    ...item.params
                };
            }).filter(e => e.eventId === 'ai_fault');
        } else {
            aiHistoryEvents.value = [];
        }
    } catch (e) {
        console.error("Failed to fetch history", e);
        aiHistoryEvents.value = [];
    } finally {
        aiHistoryLoading.value = false;
    }
};

// Batch AI Configuration State
const showBatchAIModal = ref(false);
const batchAiConfig = ref({
  product_code: '',
  devices: [],
  enabled: true,
  property: '',
  window_size: 50,
  prediction_length: 1,
  threshold_sigma: 3.5
});
const batchProductProperties = ref([]);
const batchDeviceList = ref([]);

const showHoverData = async (device, event) => {
  // Cancel pending hide if exists
  if (hideDebounceTimer) {
    clearTimeout(hideDebounceTimer);
    hideDebounceTimer = null;
  }

  // If already hovering same device, just update position
  if (hoveredDevice.value && hoveredDevice.value.code === device.code) {
      updateTooltipPosition(event);
      return;
  }

  if (hoverTimer) clearInterval(hoverTimer);
  if (hoverTrendTimer) {
    clearInterval(hoverTrendTimer);
    hoverTrendTimer = null;
  }
  
  hoveredDevice.value = { ...device }; // Copy device info
  hoveredData.value = {};
  hoveredTrendData.value = {};
  hoveredTSLMap.value = {}; // Clear previous TSL map

  // Load TSL for the hovered device
  const product = products.value.find(p => p.code === device.product_code);
  if (product && product.config) {
      try {
          const prodConfig = JSON.parse(product.config);
          const tslProps = prodConfig.tsl?.properties || [];
          tslProps.forEach(p => {
              hoveredTSLMap.value[p.identifier] = p;
          });
      } catch (e) {
          // silent fail
      }
  }
  
  updateTooltipPosition(event);
  
  // Fetch initial
  fetchHoverData(device.code);
  fetchHoverTrend(device.code);
  
  // Poll
  hoverTimer = setInterval(() => fetchHoverData(device.code), 2000);
  hoverTrendTimer = setInterval(() => fetchHoverTrend(device.code), 30000); // 30s for trend
};

const updateTooltipPosition = (event) => {
  const x = event.clientX + 15;
  const y = event.clientY + 15;
  const winWidth = window.innerWidth;
  // If too close to right edge, show on left
  const finalX = x + 300 > winWidth ? x - 320 : x;
  tooltipPos.value = { top: y, left: finalX };
};

const hideHoverData = () => {
  // Delay hiding to prevent flicker when moving between cells
  hideDebounceTimer = setTimeout(() => {
    if (hoverTimer) {
      clearInterval(hoverTimer);
      hoverTimer = null;
    }
    if (hoverTrendTimer) {
      clearInterval(hoverTrendTimer);
      hoverTrendTimer = null;
    }
    hoveredDevice.value = null;
  }, 100);
};

const fetchHoverTrend = async (code) => {
  if (!hoveredDevice.value || hoveredDevice.value.code !== code) return;
  
  const endTs = Date.now();
  const startTs = endTs - 30 * 60 * 1000; // 30 minutes ago

  try {
    const res = await axios.post('/api/history/query', {
      device_code: code,
      start_time: startTs,
      end_time: endTs,
      type: 1, // property
      aggregate: true, 
      max_points: 30, // Small sparkline, fewer points
      agg_method: 'avg'
    });

    if (res.data.code === 0) {
      const list = res.data.data.list || [];
      const trendMap = {};
      
      // Initialize lists
      Object.keys(hoveredTSLMap.value).forEach(key => {
        trendMap[key] = [];
      });

      list.forEach(item => {
        Object.keys(item).forEach(key => {
          if (key === 'ts' || key === '_type' || key === 'raw' || key === 'error') return;
          if (!trendMap[key]) trendMap[key] = [];
          trendMap[key].push(item[key]);
        });
      });
      
      // Only update if still hovering same device
      if (hoveredDevice.value && hoveredDevice.value.code === code) {
          hoveredTrendData.value = trendMap;
      }
    }
  } catch (e) {
    // silent fail
  }
};

const fetchHoverData = async (code) => {
  if (!hoveredDevice.value || hoveredDevice.value.code !== code) return;
  try {
    const res = await axios.get(`/api/devices/${code}/data`);
    if (res.data.code === 0) {
      hoveredData.value = res.data.data || {};
    }
  } catch (e) {
    // silent fail
  }
};

const getDeviceName = (code) => {
  const d = devices.value.find(dev => dev.code === code);
  return d ? d.name : code;
};

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

// History Data State
const activeTab = ref('realtime');
const historyQuery = ref({
  startTime: '',
  endTime: '',
  type: 'property' // property | event
});
const historyTableData = ref([]);
const historyChartData = ref([]);
const historyTableLoading = ref(false);
const historyChartLoading = ref(false);
const historyChartInterval = ref(0);
const historyPage = ref(1);
const historyMaxPoints = ref(2000); // Max points to display (N)
const historyAggMethod = ref('avg'); // avg, min, max, median
const historyPageSize = ref(10);
const historyTotal = ref(0);
const historyJumpPage = ref(1);
const historyRange = ref('1d');

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

const chartOption = ref(null);
const availableProperties = ref([]);
const selectedProperties = ref([]);

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

watch(() => historyQuery.value.type, () => {
  historyTableData.value = [];
  historyChartData.value = [];
  chartOption.value = null;
  historyPage.value = 1;
  // Automatically fetch new data type
  fetchHistory();
});

watch(activeTab, (val) => {
  if (val === 'history') {
    fetchHistory();
  }
});

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

const getNumericProperties = () => {
   return availableProperties.value.filter(p => {
       const tsl = currentDataTSLMap.value[p.key];
       if (!tsl) return true; // fallback
       const t = (tsl.dataType?.type || tsl.data_type || '').toLowerCase();
       return ['int', 'float', 'double', 'long', 'int32', 'int64', 'number'].includes(t);
   });
};

const getSingleAINumericProperties = () => {
    const props = [];
    for (const key in singleAITSLMap.value) {
        const tsl = singleAITSLMap.value[key];
        const t = (tsl.dataType?.type || tsl.data_type || '').toLowerCase();
        if (['int', 'float', 'double', 'long', 'int32', 'int64', 'number'].includes(t)) {
             props.push({ key, name: tsl.name || key });
        }
    }
    return props;
};

const fetchDeviceAIConfig = async () => {
  if (!currentSingleAIDevice.value) return;
  try {
     const res = await axios.get(`/api/plugins/ai_predict/config/tasks/${currentSingleAIDevice.value.code}`);
     if (res.data.code === 0 && res.data.data) {
        const task = res.data.data;
        aiConfig.value = {
            ...aiConfig.value,
            ...task,
            // Map nested baseline parameter to flat form field
            threshold_sigma: task.baseline?.threshold_sigma || 3.5
        };
     } else {
        // default
        aiConfig.value = { enabled: false, property: '', window_size: 50, prediction_length: 1, threshold_sigma: 3.5, is_calibrated: false };
     }
  } catch (e) {
     aiConfig.value.enabled = false;
  }
};

const saveSingleAIConfig = async () => {
    if (!currentSingleAIDevice.value) return;
    try {
        // Wrap flat UI fields into the structured WatchTask for backend
        const payload = {
            ...aiConfig.value,
            device_code: currentSingleAIDevice.value.code,
            baseline: {
                threshold_sigma: aiConfig.value.threshold_sigma
            }
        };
        const res = await axios.post(`/api/plugins/ai_predict/config/tasks`, payload);
        if (res.data.code === 0) {
            alert('AI 设备守护配置已成功保存并应用');
            fetchConfiguredTasks();
            fetchAITrend(); // Refresh chart to show latest status
        } else {
            alert('保存失败: ' + res.data.message);
        }
    } catch (e) {
        console.error("Failed to update AI config", e);
        alert('保存配置时发生异常');
    }
};

const debouncedSaveAIConfig = () => {
   // Kept for backward compatibility if needed, but we now use explicit save
   if (aiConfigSaveTimer) clearTimeout(aiConfigSaveTimer);
   aiConfigSaveTimer = setTimeout(saveSingleAIConfig, 500);
};

const fetchAITrend = async () => {
   if (!currentSingleAIDevice.value || !aiConfig.value.property) {
       aiChartOption.value = {};
       return;
   }

   const endTs = Date.now();
   const startTs = endTs - 5 * 60 * 1000;

   aiChartLoading.value = true;
   try {
      // Use non-aggregated query with a large page_size so that we get every raw
      // record in the 5-minute window.  Records are returned DESC (newest first).
      //
      // Data shape: each TSDB record is a **separate** row.  A device property report
      // produces a row like {"temp": 25.5}, and each AI evaluation produces a separate
      // row like {"temp_ai_health": 100, "temp_ai_anomaly": false, ...}.
      //
      // Dynamically compute page_size to avoid left-side blank when high-frequency
      // devices push out early records from a fixed 1000 limit.
      const tslPropCount = Object.keys(singleAITSLMap.value).length || 1;
      const estimatedPerMin = 60 * (tslPropCount + 1); // +1 for AI virtual props
      const dynamicPageSize = Math.min(Math.max(Math.ceil(estimatedPerMin * 5 * 1.5), 1000), 5000);
      
      const historyReq = axios.post('/api/history/query', {
         device_code: currentSingleAIDevice.value.code,
         start_time: startTs,
         end_time: endTs,
         type: 1,
         aggregate: false,
         page_size: dynamicPageSize
      });
      
      // Also fetch latest device state to keep ai_health_score up to date in real-time
      const deviceReq = axios.get(`/api/devices/${currentSingleAIDevice.value.code}`);

      const [resRaw, resDevice] = await Promise.all([
           historyReq,
           deviceReq.catch(() => ({ data: { code: -1 } })) // Ignore device fetch error
       ]);

       if (!currentSingleAIDevice.value) return;
 
       if (resDevice.data?.code === 0 && resDevice.data?.data) {
          const freshData = resDevice.data.data;
          currentSingleAIDevice.value = { ...currentSingleAIDevice.value, ...freshData };
          
          // Also update the device in the main list so the background view is updated
          const listIdx = devices.value.findIndex(d => d.code === freshData.code);
          if (listIdx !== -1) {
             devices.value[listIdx] = { ...devices.value[listIdx], ...freshData };
          }
      }

      // Backend returns DESC order (newest first) for non-aggregated queries.
      const rawList = resRaw.data?.data?.list || [];

      const rawData = [];
      // combinedPoints will store { ts, rawVal, healthScore, isLocked, ... } for ASC processing
      const combinedPoints = [];

      const prop = aiConfig.value.property;
      const healthKey = `${prop}_ai_health`;
      const triggerKey = `${prop}_ai_health_trigger`;
      const anomalyKey = `${prop}_ai_anomaly`;
      const latchedKey = `${prop}_ai_latched`;

      let newestHealth = null;
      let foundNewestAnomaly = false;
      let foundNewestLatched = false;
      let triggerScore = null;

      // DESC pass: collect raw data, health points, and status flags
      for (const item of rawList) {
         // Raw property value
         if (item[prop] !== undefined && item[prop] !== null) {
            rawData.push([item.ts, item[prop]]);
         }
         
         // Build combined item
         const hasHealth = (item[healthKey] !== undefined && item[healthKey] !== null);
         const hasLatched = (item[latchedKey] !== undefined);
         const isLocked = item[latchedKey] === true;
         
         if (hasHealth || hasLatched) {
            combinedPoints.push({ 
                ts: item.ts, 
                hasHealth: hasHealth, 
                score: hasHealth ? item[healthKey] : null, 
                hasLatched: hasLatched,
                isLocked: isLocked 
            });

            if (isLocked) {
               // Prefer _ai_health_trigger field; fall back to non-zero _ai_health
               if (triggerScore === null && item[triggerKey] != null && item[triggerKey] > 0) {
                  triggerScore = item[triggerKey];
               } else if (triggerScore === null && hasHealth && item[healthKey] > 0) {
                  triggerScore = item[healthKey];
               }
            } else if (hasHealth) {
               // First non-locked occurrence in DESC = newest real score
               if (newestHealth === null) {
                  newestHealth = item[healthKey];
               }
            }
         } else if (item[prop] !== undefined && item[prop] !== null) {
            // No health score and no latched flag. If we have raw data, this is a "Calculating" point.
            combinedPoints.push({
                ts: item.ts,
                hasHealth: false,
                score: null,
                hasLatched: false,
                isLocked: false
            });
         }

         // Anomaly / latched flags — first occurrence = newest (DESC order)
         if (!foundNewestAnomaly && item[anomalyKey] !== undefined) {
             aiLatestAnomaly.value = item[anomalyKey] === true;
             foundNewestAnomaly = true;
         }
         if (!foundNewestLatched && item[latchedKey] !== undefined) {
             aiLatestLatched.value = item[latchedKey] === true;
             foundNewestLatched = true;
         }
      }

      // If no _ai_latched record exists in the 5-minute TSDB window (e.g. device was locked
      // >5 min ago, or backend just restarted and no injection has run yet), fall back to
      // the device's authoritative in-memory state so the state machine uses the right value.
      if (!foundNewestLatched && currentSingleAIDevice.value?.ai_latched !== undefined) {
         aiLatestLatched.value = currentSingleAIDevice.value.ai_latched === true;
      }

      aiLatchTriggerScore.value = triggerScore;

      // Reverse to chronological order (ASC) for ECharts time axis
      rawData.reverse();
      combinedPoints.reverse();

      // --- Gap Detection & Insertion ---
      // Determine gap threshold based on median interval
      const intervals = [];
      for (let i = 1; i < combinedPoints.length; i++) {
          intervals.push(combinedPoints[i].ts - combinedPoints[i-1].ts);
      }
      intervals.sort((a, b) => a - b);
      let medianInterval = 1000; // Default 1s
      if (intervals.length > 0) {
          medianInterval = intervals[Math.floor(intervals.length / 2)];
      }
      // Threshold: at least 2.5x median, but min 5s (to avoid breaking on minor jitter)
      const gapThreshold = Math.max(medianInterval * 2.5, 5000);

      // Process Raw Data: Insert nulls for gaps
      const processedRawData = [];
      if (rawData.length > 0) {
          processedRawData.push(rawData[0]);
          for (let i = 1; i < rawData.length; i++) {
              if (rawData[i][0] - rawData[i-1][0] > gapThreshold) {
                  processedRawData.push([rawData[i][0] - 1, null]);
              }
              processedRawData.push(rawData[i]);
          }
      }

      // Process Combined Points: Insert gap markers
      const processedCombinedPoints = [];
      if (combinedPoints.length > 0) {
          processedCombinedPoints.push(combinedPoints[0]);
          for (let i = 1; i < combinedPoints.length; i++) {
              if (combinedPoints[i].ts - combinedPoints[i-1].ts > gapThreshold) {
                  processedCombinedPoints.push({ isGap: true, ts: combinedPoints[i].ts - 1 });
              }
              processedCombinedPoints.push(combinedPoints[i]);
          }
      }

      // State-machine pass (ASC): build healthData, lockedData, and calculatingData mutually exclusively.
      const healthData = [];
      const lockedData = [];
      const calculatingData = [];
      
      let inLockedState = false;
      
      // State for value propagation (filling gaps between AI evaluations)
      let lastKnownHealthScore = null;
      let lastKnownIsLocked = false;
      let lastKnownHealthTs = 0;
      const PROPAGATION_TIMEOUT = 60000; // Stop propagating if no AI result for 60s

      // Pre-scan: find the earliest valid health score to pre-populate propagation state.
      // This prevents false "Calculating" grey lines at the chart start when raw data points
      // arrive before the first _ai_health point (AI evaluates every 30s, raw data is more frequent).
      for (const item of processedCombinedPoints) {
        if (item.hasHealth && item.score !== null) {
          lastKnownHealthScore = item.score;
          lastKnownHealthTs = item.ts;
          lastKnownIsLocked = item.isLocked;
          break;
        }
        if (item.hasLatched && item.isLocked !== null) {
          lastKnownIsLocked = item.isLocked;
          // Don't break — keep looking for an actual health score
        }
      }

      for (const item of processedCombinedPoints) {
         if (item.isGap) {
             healthData.push([item.ts, null]);
             lockedData.push([item.ts, null]);
             calculatingData.push([item.ts, null]);
             
             // Reset propagation state on gap
             lastKnownHealthScore = null;
             inLockedState = false; // Reset chart state machine too
             continue;
         }
         
         const { ts, hasHealth, score, isLocked, hasLatched } = item;
         
         let effectiveHasHealth = hasHealth;
         let effectiveScore = score;
         let effectiveIsLocked = isLocked;

         if (hasHealth || hasLatched) {
             // Update known state
             if (hasHealth) {
                 lastKnownHealthScore = score;
                 lastKnownHealthTs = ts;
             }
             if (hasLatched) {
                 lastKnownIsLocked = isLocked;
                 if (!isLocked && !hasHealth) {
                     // Manual unlock event (no health score attached)
                     // Invalidate the old health score so it goes into "Calculating" state
                     lastKnownHealthScore = null;
                     effectiveHasHealth = false;
                 } else if (hasHealth) {
                     effectiveHasHealth = true;
                     effectiveScore = lastKnownHealthScore;
                 }
             } else if (hasHealth) {
                 lastKnownIsLocked = isLocked;
                 effectiveHasHealth = true;
                 effectiveScore = lastKnownHealthScore;
             }
             effectiveIsLocked = lastKnownIsLocked;
         } else {
             // Try to propagate from last known state
             if (lastKnownHealthScore !== null && (ts - lastKnownHealthTs < PROPAGATION_TIMEOUT)) {
                 effectiveHasHealth = true;
                 effectiveScore = lastKnownHealthScore;
                 effectiveIsLocked = lastKnownIsLocked;
             }
         }

         if (effectiveHasHealth) {
             // We have a health score (Real or Propagated)
             
             // Break the calculating series line here
             calculatingData.push([ts, null]);
             
             // For locked points, display at the trigger score level
             const displayScore = effectiveIsLocked ? (triggerScore ?? effectiveScore) : effectiveScore;

             if (!inLockedState) {
                if (effectiveIsLocked) {
                   // Transition: normal → locked
                   inLockedState = true;
                   healthData.push([ts, displayScore]);     // yellow extends TO lock boundary
                   healthData.push([ts + 1, null]);         // null marker: breaks yellow line here
                   lockedData.push([ts, displayScore]);     // red starts at lock boundary
                } else {
                   healthData.push([ts, effectiveScore]);
                   
                   // Ensure lockedData is broken
                   lockedData.push([ts, null]);
                }
             } else {
                if (effectiveIsLocked) {
                   lockedData.push([ts, displayScore]);
                   
                   // Ensure healthData is broken
                   healthData.push([ts, null]);
                } else if (!aiLatestLatched.value) {
                   // Transition: locked → normal (device is currently unlocked — genuine recovery)
                   inLockedState = false;
                   lockedData.push([ts, effectiveScore]);            // red extends TO recovery boundary
                   lockedData.push([ts + 1, null]);         // null marker: breaks red line here
                   healthData.push([ts, effectiveScore]);            // yellow resumes from recovery boundary
                } else {
                   // Device is still locked — ignore stale non-locked records
                }
             }
         } else {
             // No health score -> Calculating
             calculatingData.push([ts, 0]);
             
             // Break other lines
             healthData.push([ts, null]);
             lockedData.push([ts, null]);
         }
      }

      // 将健康状态系列向左延伸到窗口起点，消除左侧空白
      // 仅当设备在窗口起点时已处于锁定状态（即窗口内第一个健康点就是锁定的），
      // 才将 lockedData 向左延伸；若设备是在窗口中途才进入锁定状态，
      // 则不能往前延伸，否则会与 healthData 在锁定前的正常段产生视觉重叠。
      const firstPointWasLocked = combinedPoints.length > 0 && combinedPoints[0].hasHealth && combinedPoints[0].isLocked;
      if (lockedData.length > 0 && lockedData[0][0] > startTs && firstPointWasLocked) {
         lockedData.unshift([startTs, lockedData[0][1]]);
      }

      // Update the displayed health score.
      // Priority: use device list's in-memory state to determine "generating" status.
      // If the device list has no score (null/undefined), it means Noyo hasn't produced
      // a score in this run cycle yet — show "generating" even if old DB records exist.
      const deviceInMemoryScore = currentSingleAIDevice.value?.ai_health_score;

      // Check if the chart ends in "Calculating" state
      let isEndingInCalculating = false;
      if (calculatingData.length > 0) {
          const lastPoint = calculatingData[calculatingData.length - 1];
          // Check the value (index 1). If not null, it means we are in calculating state (value 0).
          isEndingInCalculating = (lastPoint[1] !== null);
      }

      if (deviceInMemoryScore === null || deviceInMemoryScore === undefined) {
        // Device list says "generating" — respect that, don't override with stale DB data
        aiLatestHealth.value = null;
      } else if (isEndingInCalculating) {
        // If the chart shows "Calculating" (dashed line), the text should also show "-" (null)
        aiLatestHealth.value = null;
      } else {
        // Device list has a live score — use the freshest from DB or in-memory
        aiLatestHealth.value = newestHealth !== null ? newestHealth : deviceInMemoryScore;
      }

      aiChartOption.value = {
         tooltip: { 
            trigger: 'axis',
            formatter: (params) => {
               if (!params || params.length === 0) return '';
               let timeStr = '';
               const item0 = params[0];
               if (Array.isArray(item0.value)) {
                  timeStr = new Date(Number(item0.value[0])).toLocaleString();
               } else if (item0.axisValue) {
                  timeStr = new Date(Number(item0.axisValue)).toLocaleString();
               }
               
               let html = `<div style="margin-bottom: 3px; font-weight: bold;">${timeStr}</div>`;
               
               params.forEach(item => {
                  let val = Array.isArray(item.value) ? item.value[1] : item.value;
                  // Skip displaying null, undefined or NaN values
                  if (val !== null && val !== undefined && val !== '-' && !Number.isNaN(val)) {
                     // Optionally format numbers to fixed decimals if they are floats
                     let displayVal = val;
                     if (item.seriesName === '健康得分计算中') {
                        displayVal = ''; // Or '计算中...' - empty is cleaner if name is already '健康得分计算中'
                     } else if (typeof val === 'number' && !Number.isInteger(val)) {
                        displayVal = val.toFixed(2);
                     }
                     html += `<div style="display: flex; justify-content: space-between; align-items: center;">
                               <span style="margin-right: 15px;">${item.marker}${item.seriesName}</span>
                               <span style="font-weight: bold;">${displayVal}</span>
                             </div>`;
                  }
               });
               return html;
            }
         },
         legend: { data: ['原始数值', '健康得分 (AI)', '异常锁定', '健康得分计算中'], bottom: 0 },
         grid: { left: '3%', right: '4%', bottom: '10%', containLabel: true },
         xAxis: {
            type: 'time',
            min: startTs,
            max: endTs
         },
         yAxis: [
            { type: 'value', name: 'Raw', position: 'left' },
            { type: 'value', name: 'Health', position: 'right', min: 0, max: 105 }
         ],
         series: [
            {
               name: '原始数值',
               type: 'line',
               data: processedRawData,
               smooth: true,
               showSymbol: false,
               connectNulls: false,
               yAxisIndex: 0
            },
            {
               name: '健康得分 (AI)',
               type: 'line',
               data: healthData,
               smooth: true,
               showSymbol: false,
               connectNulls: false,
               yAxisIndex: 1,
               itemStyle: { color: '#ffc107' },
               areaStyle: {
                  color: 'rgba(255, 193, 7, 0.2)'
               }
            },
            {
               name: '异常锁定',
               type: 'line',
               data: lockedData,
               showSymbol: lockedData.length <= 5,
               connectNulls: false,
               yAxisIndex: 1,
               itemStyle: { color: '#dc3545' },
               lineStyle: { color: '#dc3545', width: 3, type: 'dashed' }
            },
            {
               name: '健康得分计算中',
               type: 'line',
               data: calculatingData,
               showSymbol: false,
               connectNulls: false,
               yAxisIndex: 1,
               itemStyle: { color: '#6c757d' },
               lineStyle: { color: '#6c757d', width: 3, type: 'dotted' }
            }
         ],
         dataZoom: [{ type: 'inside' }]
      };
   } catch (e) {
      console.error(e);
   } finally {
      aiChartLoading.value = false;
   }
};

const openSingleAIModal = async (device) => {
  currentSingleAIDevice.value = device;
  singleAITSLMap.value = {};
  aiChartOption.value = {};
  aiLatestHealth.value = null;
  aiLatestAnomaly.value = false;
  aiLatestLatched.value = false;
  aiLatchTriggerScore.value = null;
  
  // Always fetch latest product details to ensure TSL is up-to-date
  let product = null;
  try {
      const res = await axios.get(`/api/products/${device.product_code}`);
      if (res.data.code === 0) {
          product = res.data.data;
      }
  } catch (e) {
      console.warn("Failed to fetch product for TSL", e);
      product = products.value.find(p => p.code === device.product_code);
  }

  if (product && product.config) {
      try {
          const prodConfig = JSON.parse(product.config);
          const tslProps = prodConfig.tsl?.properties || [];
          tslProps.forEach(p => {
              singleAITSLMap.value[p.identifier] = p;
          });
      } catch (e) {
          console.warn("Invalid Product Config JSON");
      }
  }

  singleAIModalVisible.value = true;
  
  await fetchDeviceAIConfig();
  fetchAITrend();
  aiChartTimer = setInterval(fetchAITrend, 5000);
};

const closeSingleAIModal = () => {
    singleAIModalVisible.value = false;
    currentSingleAIDevice.value = null;
    if (aiChartTimer) {
        clearInterval(aiChartTimer);
        aiChartTimer = null;
    }
};

const clearLatchedState = async () => {
    if (!currentSingleAIDevice.value || !aiConfig.value.property) return;
    if (!confirm("确定要解除异常锁定状态吗？\n解除后将立即恢复AI健康监测，系统将重新评估设备健康状态。")) return;
    
    const taskId = `${currentSingleAIDevice.value.code}_${aiConfig.value.property}`;
    
    try {
        const res = await axios.post(`/api/plugins/ai_predict/latch/${taskId}/clear`);
        if (res.data.code === 0) {
            alert("解除锁定成功");
            fetchAITrend(); // Refresh status immediately
        } else {
            alert("解除失败: " + res.data.message);
        }
    } catch (e) {
        console.error(e);
        alert("解除失败");
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

const openDataModal = async (device, initialTab = 'realtime') => {
  currentDataDevice.value = device;
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
    const config = device.config ? JSON.parse(device.config) : {};
    if (config.points) {
      if (Array.isArray(config.points)) {
        config.points.forEach(p => {
          if (p.name) {
            pointConfigs.value[p.name] = p;
          }
        });
      } else {
        pointConfigs.value = config.points;
      }
    }
  } catch (e) {
    console.error("Failed to parse device config", e);
  }

  // Always fetch latest product details to ensure TSL is up-to-date
  let product = null;
  try {
      const res = await axios.get(`/api/products/${device.product_code}`);
      if (res.data.code === 0) {
          product = res.data.data;
      }
  } catch (e) {
      console.warn("Failed to fetch product for TSL", e);
      // Fallback to local list if API fails
      product = products.value.find(p => p.code === device.product_code);
  }

  if (product && product.config) {
      try {
          const prodConfig = JSON.parse(product.config);
          const tslProps = prodConfig.tsl?.properties || [];
          tslProps.forEach(p => {
              currentDataTSLMap.value[p.identifier] = p;
          });
          const tslEvents = prodConfig.tsl?.events || [];
          tslEvents.forEach(e => {
              currentDataTSLEventMap.value[e.identifier] = e;
          });
          const tslServices = prodConfig.tsl?.services || [];
          tslServices.forEach(s => {
              currentDataTSLServiceMap.value[s.identifier] = s;
          });

          // Init service params
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

  showDataModal.value = true;
  activeTab.value = initialTab || 'realtime';
  initHistoryDates();
  fetchDeviceData();
  fetchRealtimeTrend();
  
  dataTimer = setInterval(fetchDeviceData, 2000);
  trendTimer = setInterval(fetchRealtimeTrend, 30000);
};

const closeDataModal = () => {
  showDataModal.value = false;
  currentDataDevice.value = null;
  writeValues.value = {};
  pointConfigs.value = {};
  currentDataTSLMap.value = {};
  currentDataTSLEventMap.value = {};
  currentDataTSLServiceMap.value = {};
  if (dataTimer) {
    clearInterval(dataTimer);
    dataTimer = null;
  }
  if (trendTimer) {
    clearInterval(trendTimer);
    trendTimer = null;
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

onUnmounted(() => {
  if (dataTimer) {
    clearInterval(dataTimer);
  }
  if (trendTimer) {
    clearInterval(trendTimer);
  }
  if (hoverTimer) clearInterval(hoverTimer);
  if (hoverTrendTimer) clearInterval(hoverTrendTimer);
  if (hideDebounceTimer) clearTimeout(hideDebounceTimer);
  if (healthTooltipTimer) clearTimeout(healthTooltipTimer);
  window.removeEventListener('noyo-data-updated', fetchDevices);
  if (sseHeartbeatTimer) clearTimeout(sseHeartbeatTimer);
  if (sseFetchDebounceTimer) clearTimeout(sseFetchDebounceTimer);
  if (sseReconnectTimer) clearTimeout(sseReconnectTimer);
  if (eventSource) {
    eventSource.close();
    eventSource = null;
  }
});

// Mapping State
const showMappingModal = ref(false);
const currentMappingDevice = ref(null);
const currentMappingDeviceConfig = ref({});
const currentMappingProperties = ref([]);
const currentMappingEvents = ref([]);
const currentMappingPollingGroups = ref([]);
const currentMappingProtocol = ref('');
const isParentCascade = ref(false);

const fetchDevices = async (silent = false) => {
  if (!silent) loading.value = true;
  try {
    const res = await axios.get('/api/devices', { params: { page: 0, _t: Date.now() } });
    if (res.data.code === 0) {
      devices.value = res.data.data || [];
    }
  } catch (e) {
    console.error(e);
  } finally {
    if (!silent) loading.value = false;
  }
};

const fetchProducts = async () => {
  try {
    const res = await axios.get('/api/products', { params: { page: 0 } });
    if (res.data.code === 0) {
      products.value = res.data.data || [];
    }
  } catch (e) {
    console.error(e);
  }
};

// 协议映射标志缓存: protocolName -> bool
const protocolMappingMap = ref({});

const fetchPluginsInfo = async () => {
  try {
    const res = await axios.get('/api/plugins');
    if (res.data.code === 0) {
      const map = {};
      for (const p of (res.data.data || [])) {
        if (p.protocolMappingRequired !== undefined) {
          map[p.name] = p.protocolMappingRequired;
        }
      }
      protocolMappingMap.value = map;
    }
  } catch (e) {
    console.error(e);
  }
};

// 判断设备是否需要协议映射
const needsProtocolMapping = (device) => {
  const product = products.value.find(p => p.code === device.product_code);
  if (!product || !product.protocol_name) return false;
  // 子设备取父设备的协议
  let protocolName = product.protocol_name;
  if (device.parent_code && !protocolName) {
    const parentDev = devices.value.find(d => d.code === device.parent_code);
    if (parentDev) {
      const parentProd = products.value.find(p => p.code === parentDev.product_code);
      protocolName = parentProd?.protocol_name || '';
    }
  }
  if (!protocolName) return false;
  const required = protocolMappingMap.value[protocolName];
  return required !== false; // 默认 true
};

const getProtocol = (productCode) => {
    const p = products.value.find(prod => prod.code === productCode);
    return p ? p.protocol_name : '';
};

const isChildOfCascade = (device) => {
  if (!device.parent_code) return false;
  const parent = devices.value.find(d => d.code === device.parent_code);
  if (!parent) return false;
  const parentProduct = products.value.find(p => p.code === parent.product_code);
  return parentProduct && parentProduct.protocol_name === 'cascade';
};

const fetchProtocolSchema = async (productCode, parentCode) => {
  if (!productCode) {
    currentSchema.value = null;
    return;
  }
  try {
    // 使用新的 config-schema API，子设备从父设备获取协议的 SubDeviceConfigSchema
    const params = new URLSearchParams();
    params.append('productCode', productCode);
    if (parentCode) params.append('parentCode', parentCode);
    
    const res = await axios.get(`/api/devices/config-schema?${params.toString()}`);
    if (res.data.code === 0) {
      currentSchema.value = res.data.data.schema;
      
      // Apply defaults from schema to config if missing
      if (currentSchema.value && currentSchema.value.properties) {
          const config = newDevice.value.config || {};
          let changed = false;
          for (const key in currentSchema.value.properties) {
              const prop = currentSchema.value.properties[key];
              if (config[key] === undefined && prop.default !== undefined) {
                  config[key] = prop.default;
                  changed = true;
              }
          }
          if (changed) {
              newDevice.value.config = { ...config };
          }
      }
    } else {
      currentSchema.value = null;
    }
  } catch (e) {
    console.error(e);
    currentSchema.value = null;
  }
};

const handleProductChange = () => {
  newDevice.value.config = {}; // Reset config
  if (newDevice.value.product_code) {
    fetchProtocolSchema(newDevice.value.product_code, newDevice.value.parent_code);
  } else {
    currentSchema.value = null;
  }
};

// 监听父设备变化，重新加载 Schema（子设备 Schema 可能不同）
watch(() => newDevice.value.parent_code, (newParentCode) => {
  if (newDevice.value.product_code) {
    fetchProtocolSchema(newDevice.value.product_code, newParentCode);
  }
});

const fetchConfiguredTasks = async () => {
  try {
    const res = await axios.get('/api/plugins/ai_predict/config/tasks');
    if (res.data.code === 0 && res.data.data) {
      const codes = new Set();
      Object.values(res.data.data).forEach(task => {
        if (task.enabled) codes.add(task.device_code);
      });
      configuredDeviceCodes.value = codes;
    } else {
      configuredDeviceCodes.value = new Set();
    }
  } catch (e) {
    console.error("Failed to fetch AI tasks", e);
  }
};

onMounted(() => {
  fetchDevices();
  fetchProducts();
  fetchPluginsInfo();
  fetchConfiguredTasks();
  window.addEventListener('noyo-data-updated', fetchDevices);

  // 建立SSE连接，支持心跳检测和自动重连
  setupSSE();
});

const openCreateModal = () => {
  isEditing.value = false;
  newDevice.value = { code: '', name: '', product_code: '', parent_code: '', enabled: true, config: {} };
  currentSchema.value = null;
  showCreateModal.value = true;
};

// Import/Export Logic
const fileInput = ref(null);
const showDownloadModal = ref(false);
const selectedProductsForTemplate = ref([]);

const targetProtocol = computed(() => {
  if (selectedProductsForTemplate.value.length === 0) return null;
  const firstCode = selectedProductsForTemplate.value[0];
  const p = products.value.find(prod => prod.code === firstCode);
  return p ? p.protocol_name : null;
});

const isProductDisabled = (p) => {
  if (!targetProtocol.value) return false;
  return p.protocol_name !== targetProtocol.value;
};

const downloadTemplate = () => {
  selectedProductsForTemplate.value = [];
  showDownloadModal.value = true;
};

const selectAllProducts = () => {
  let proto = targetProtocol.value;
  if (!proto && products.value.length > 0) {
    // If no protocol selected yet, default to the first product's protocol
    proto = products.value[0].protocol_name;
  }
  
  if (proto) {
    // Only select products matching the protocol
    selectedProductsForTemplate.value = products.value
      .filter(p => p.protocol_name === proto)
      .map(p => p.code);
  }
};

const deselectAllProducts = () => {
  selectedProductsForTemplate.value = [];
};

const confirmDownloadTemplate = () => {
  if (!targetProtocol.value) {
      // Should not happen as button is disabled
      return;
  }
  const lang = locale.value || 'zh';
  let url = `/api/devices/import/template?lang=${lang}&protocol=${targetProtocol.value}`;
  if (selectedProductsForTemplate.value.length > 0) {
    url += `&product_codes=${selectedProductsForTemplate.value.join(',')}`;
  }
  window.location.href = url;
  showDownloadModal.value = false;
};

const triggerImport = () => {
  importProtocol.value = '';
  importFile.value = null;
  // Auto-select if filtered or locked for template
  if (targetProtocol.value) {
      importProtocol.value = targetProtocol.value;
  } else if (filterProduct.value) {
      const p = products.value.find(prod => prod.code === filterProduct.value);
      if (p) importProtocol.value = p.protocol_name;
  }
  showImportModal.value = true;
};

const showImportModal = ref(false);
const importProtocol = ref('');
const importFile = ref(null);

const availableProtocols = computed(() => {
    const protos = new Set(products.value.map(p => p.protocol_name).filter(Boolean));
    return Array.from(protos);
});

const closeImportModal = () => {
    showImportModal.value = false;
    importFile.value = null;
    importProtocol.value = '';
};

const handleImportFileChange = (event) => {
    importFile.value = event.target.files[0];
};

const confirmImport = async () => {
  if (!importFile.value || !importProtocol.value) return;

  const formData = new FormData();
  formData.append('file', importFile.value);

  try {
    const res = await axios.post(`/api/devices/import?protocol=${importProtocol.value}`, formData, {
      headers: {
        'Content-Type': 'multipart/form-data'
      }
    });
    if (res.data.code === 0) {
      alert(t('import_success') + ': ' + res.data.message); // message contains count
      fetchDevices();
      closeImportModal();
    } else {
      alert(t('import_fail') + ': ' + res.data.message);
    }
  } catch (e) {
    alert(t('import_fail'));
    console.error(e);
  }
};

const openEditModal = async (device) => {
  isEditing.value = true;
  currentSchema.value = null;
  // Deep copy
  newDevice.value = { ...device };
  // Config is string, need to parse
  try {
    newDevice.value.config = device.config ? JSON.parse(device.config) : {};
    // Backward compatibility: map host to ip for old configs
    if (newDevice.value.config.host && !newDevice.value.config.ip) {
        newDevice.value.config.ip = newDevice.value.config.host;
    }
    // Backward compatibility: map protocol_type for Modbus
    if (newDevice.value.config.protocol_type === 'TCP') {
        newDevice.value.config.protocol_type = 'Modbus-TCP';
    } else if (newDevice.value.config.protocol_type === 'RTU_OVER_TCP') {
        newDevice.value.config.protocol_type = 'Modbus-RTU over TCP';
    }
  } catch(e) {
    newDevice.value.config = {};
  }
  
  // Fetch Schema (使用产品编码而不是协议名称)
  if (device.product_code) {
    fetchProtocolSchema(device.product_code, device.parent_code);
  }
  
  showCreateModal.value = true;
};

const saveDevice = async () => {
  try {
    // Prepare payload: stringify config
    const payload = {
      ...newDevice.value,
      config: JSON.stringify(newDevice.value.config)
    };

    let res;
    if (isEditing.value) {
       res = await axios.put(`/api/devices/${newDevice.value.code}`, payload);
    } else {
       res = await axios.post('/api/devices', payload);
    }

    if (res.data.code === 0) {
      showCreateModal.value = false;
      fetchDevices();
      newDevice.value = { code: '', name: '', product_code: '', parent_code: '', enabled: true, config: {} };
      currentSchema.value = null;
    } else {
      alert(res.data.message);
    }
  } catch (e) {
    console.error(e);
    alert(t('common_save_fail'));
  }
};

const deleteDevice = async (device) => {
  if (!confirm(t('common_delete_confirm'))) return;
  try {
    const res = await axios.delete(`/api/devices/${device.code}`);
    if (res.data.code === 0) {
      fetchDevices();
    } else {
      alert(res.data.message);
    }
  } catch (e) {
    console.error(e);
    alert(t('common_delete_fail'));
  }
};

const openCreateSubDeviceModal = (parentDevice) => {
  isEditing.value = false;
  newDevice.value = { 
    code: '', 
    name: '',
    product_code: '', 
    parent_code: parentDevice.code, 
    enabled: true, 
    config: {} 
  };
  currentSchema.value = null;
  showCreateModal.value = true;
};

const toggleDevice = async (device) => {
  const action = device.enabled ? 'stop' : 'start';
  try {
    const res = await axios.post(`/api/devices/${device.code}/${action}`);
    if (res.data.code === 0) {
      fetchDevices(); // Refresh list
    } else {
      alert(res.data.message);
    }
  } catch (e) {
    console.error(e);
    alert(t('dev_action_fail'));
  }
};

const openMappingModal = async (device) => {
  currentMappingDevice.value = device;
  
  // Parse Device Config
  try {
    currentMappingDeviceConfig.value = device.config ? JSON.parse(device.config) : {};
  } catch (e) {
    currentMappingDeviceConfig.value = {};
  }

  // Find Product and TSL
  const product = products.value.find(p => p.code === device.product_code);
  if (product) {
      // 子设备从父设备获取协议名称
      if (device.parent_code && !product.protocol_name) {
          // 尝试从父设备的产品获取协议
          try {
              const res = await axios.get(`/api/devices/${device.parent_code}`);
              if (res.data.code === 0 && res.data.data) {
                  const parentDev = res.data.data;
                  const parentProduct = products.value.find(p => p.code === parentDev.product_code);
                  currentMappingProtocol.value = parentProduct?.protocol_name || '';
              }
          } catch (e) {
              console.error("Failed to fetch parent device for protocol", e);
              currentMappingProtocol.value = '';
          }
      } else {
          currentMappingProtocol.value = product.protocol_name || '';
      }
      
      if (product.config) {
        try {
            const prodConfig = JSON.parse(product.config);
            currentMappingProperties.value = prodConfig.tsl?.properties || [];
            currentMappingEvents.value = prodConfig.tsl?.events || [];
        } catch (e) {
            console.warn("Invalid Product Config JSON");
            currentMappingProperties.value = [];
            currentMappingEvents.value = [];
        }
      } else {
          currentMappingProperties.value = [];
          currentMappingEvents.value = [];
      }
  } else {
    currentMappingProperties.value = [];
    currentMappingEvents.value = [];
    currentMappingProtocol.value = '';
  }

  isParentCascade.value = false;
  // Fetch Parent Config for Polling Groups if parent_code exists
  currentMappingPollingGroups.value = [];
  if (device.parent_code) {
      try {
          const res = await axios.get(`/api/devices/${device.parent_code}`);
          if (res.data.code === 0 && res.data.data) {
              const parentDev = res.data.data;
              const parentProduct = products.value.find(p => p.code === parentDev.product_code);
              if (parentProduct && parentProduct.protocol_name === 'cascade') {
                  isParentCascade.value = true;
              }
              if (parentDev.config) {
                  let parentConfig = {};
                  try {
                      parentConfig = typeof parentDev.config === 'string' 
                        ? JSON.parse(parentDev.config) 
                        : parentDev.config;
                  } catch (e) {
                      console.error("Failed to parse parent device config:", e);
                  }
                  currentMappingPollingGroups.value = parentConfig.polling_groups || [];
              }
          }
      } catch (e) {
          console.error("Failed to fetch parent device", e);
      }
  }

  showMappingModal.value = true;
};

const closeMappingModal = () => {
  showMappingModal.value = false;
  currentMappingDevice.value = null;
  currentMappingDeviceConfig.value = {};
  currentMappingProperties.value = [];
  currentMappingPollingGroups.value = [];
  currentMappingProtocol.value = '';
};

const updateDeviceMappingConfig = (newConfig) => {
  currentMappingDeviceConfig.value = newConfig;
};

const saveDeviceMapping = async () => {
  if (!currentMappingDevice.value) return;
  
  try {
    const payload = {
      ...currentMappingDevice.value,
      config: JSON.stringify(currentMappingDeviceConfig.value)
    };

    const res = await axios.put(`/api/devices/${currentMappingDevice.value.code}`, payload);
    if (res.data.code === 0) {
        // Update local list
        const index = devices.value.findIndex(d => d.code === currentMappingDevice.value.code);
        if (index !== -1) {
            devices.value[index] = { ...devices.value[index], config: payload.config };
        }
        closeMappingModal();
    } else {
        alert(res.data.message);
    }
  } catch (e) {
    console.error(e);
    alert(t('common_save_fail'));
  }
};

const openAIBatchConfigModal = () => {
    batchAiConfig.value = {
        product_code: '',
        devices: [],
        enabled: true,
        property: '',
        window_size: 50,
        prediction_length: 1,
        threshold_sigma: 3.5
    };
    batchProductProperties.value = [];
    batchDeviceList.value = [];
    showBatchAIModal.value = true;
};

const onBatchProductChange = () => {
    batchAiConfig.value.devices = [];
    batchAiConfig.value.property = '';
    
    // Parse Product TSL
    const product = products.value.find(p => p.code === batchAiConfig.value.product_code);
    if (product && product.config) {
        try {
            const prodConfig = JSON.parse(product.config);
            const props = prodConfig.tsl?.properties || [];
            batchProductProperties.value = props.filter(p => {
               const type = p.dataType?.type?.toLowerCase() || p.data_type?.toLowerCase();
               return type === 'int' || type === 'float' || type === 'double' || type === 'int32' || type === 'int64' || type === 'number';
            }).map(p => ({
               key: p.identifier,
               name: p.name
            }));
        } catch (e) {
            batchProductProperties.value = [];
        }
    } else {
        batchProductProperties.value = [];
    }
    
    // Filter devices
    batchDeviceList.value = devices.value.filter(d => d.product_code === batchAiConfig.value.product_code);
    // select all by default
    batchAiConfig.value.devices = batchDeviceList.value.map(d => d.code);
};

const saveBatchAITasks = async () => {
    try {
        // Prepare the payload 
        const res = await axios.post('/api/plugins/ai_predict/config/tasks/batch', batchAiConfig.value);
        if (res.data.code === 0) {
            showBatchAIModal.value = false;
            fetchConfiguredTasks();
        } else {
            alert(res.data.message || '批量配置失败');
        }
    } catch (e) {
        console.error("Batch config fail", e);
        alert('提交配置异常');
    }
};

</script>

<style scoped>
.animation-blink {
  animation: blinker 1.5s linear infinite;
}
@keyframes blinker {
  50% { opacity: 0; }
}
</style>
