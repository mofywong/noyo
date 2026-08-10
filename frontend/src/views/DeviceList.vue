<template>
  <div>
    <!-- Page Header（§7.2） -->
    <div class="page-header">
      <div>
        <h1>{{ $t('sidebar_devices') }}</h1>
        <p class="page-subtitle">{{ $t('dev_subtitle') }}</p>
      </div>
      <div class="d-flex gap-2 align-items-center">
        <button class="btn btn-outline-primary" @click="openAIBatchConfigModal" v-permission="'device:edit'">
          <i class="bi bi-shield-check me-1"></i> {{ $t('ai_batch_config') }}
        </button>
        <button class="btn btn-primary" @click="openCreateModal" v-permission="'device:create'">
          <i class="bi bi-plus-lg me-1"></i> {{ $t('dev_create') }}
        </button>
      </div>
    </div>

    <!-- KPI 统计行（§7.3 / §3.3 语义色） -->
    <div class="row g-3 mb-3">
      <div class="col-6 col-md-3 col-xl-2">
        <div class="kpi-card">
          <div class="kpi-label">{{ $t('dev_stat_total') }}</div>
          <div class="kpi-value">{{ stats.total }}</div>
        </div>
      </div>
      <div class="col-6 col-md-3 col-xl-2">
        <div class="kpi-card">
          <div class="kpi-label"><span class="status-dot status-dot--online me-1 align-middle"></span>{{ $t('dev_stat_online') }}</div>
          <div class="kpi-value">{{ stats.online }}</div>
        </div>
      </div>
      <div class="col-6 col-md-3 col-xl-2">
        <div class="kpi-card">
          <div class="kpi-label"><span class="status-dot status-dot--offline me-1 align-middle"></span>{{ $t('dev_stat_offline') }}</div>
          <div class="kpi-value">{{ stats.offline }}</div>
        </div>
      </div>
      <div class="col-6 col-md-3 col-xl-2">
        <div class="kpi-card">
          <div class="kpi-label"><i class="bi bi-shield-fill-x me-1" style="color: var(--color-danger);"></i>{{ $t('dev_stat_alarm') }}</div>
          <div class="kpi-value" style="color: var(--color-danger);">{{ stats.alarm }}</div>
        </div>
      </div>
    </div>

    <!-- Toolbar（§7.2：搜索 + 筛选折叠 + 批量操作条） -->
    <div class="page-toolbar">
      <div class="input-group" style="max-width: 320px;">
        <span class="input-group-text bg-transparent"><i class="bi bi-search"></i></span>
        <input v-model="search" type="search" class="form-control" :placeholder="$t('dev_search_placeholder')" :aria-label="$t('dev_search_placeholder')">
      </div>
      <div class="dropdown" v-if="devices.length > 0">
        <button class="btn btn-outline-secondary" type="button" @click="showFilters = !showFilters" :aria-expanded="showFilters">
          <i class="bi bi-funnel me-1"></i> {{ $t('dev_filter_toggle') }}
          <span v-if="hasActiveFilters" class="badge text-bg-primary ms-1">&bull;</span>
        </button>
        <div v-if="showFilters" class="noyo-filter-panel">
          <div class="row g-2">
            <div class="col-md-6 col-xl-4">
              <select class="form-select form-select-sm" v-model="filterProduct" :aria-label="$t('dev_product')">
                <option value="">{{ $t('dev_product') }}: {{ $t('all') }}</option>
                <option v-for="p in products" :key="p.code" :value="p.code">{{ p.name }}</option>
              </select>
            </div>
            <div class="col-md-6 col-xl-4">
              <select class="form-select form-select-sm" v-model="filterParent" :aria-label="$t('dev_parent')">
                <option value="">{{ $t('dev_parent') }}: {{ $t('all') }}</option>
                <option v-for="p in uniqueParents" :key="p.code" :value="p.code">{{ p.name }}</option>
              </select>
            </div>
            <div class="col-md-6 col-xl-4">
              <select class="form-select form-select-sm" v-model="filterEnabled" :aria-label="$t('dev_status')">
                <option value="">{{ $t('dev_status') }}: {{ $t('all') }}</option>
                <option value="true">{{ $t('dev_enabled') }}</option>
                <option value="false">{{ $t('dev_disabled') }}</option>
              </select>
            </div>
            <div class="col-md-6 col-xl-4">
              <select class="form-select form-select-sm" v-model="filterOnline" :aria-label="$t('dev_online_status')">
                <option value="">{{ $t('dev_online_status') }}: {{ $t('all') }}</option>
                <option value="true">{{ $t('dev_online') }}</option>
                <option value="false">{{ $t('dev_offline') }}</option>
              </select>
            </div>
            <div class="col-md-6 col-xl-4">
              <select class="form-select form-select-sm" v-model="filterTag" :aria-label="$t('dev_tags')">
                <option value="">{{ $t('dev_tags') }}: {{ $t('all') }}</option>
                <option v-for="tag in deviceTags" :key="tag.ID" :value="String(tag.ID)">{{ tag.name }}</option>
              </select>
            </div>
            <div class="col-md-6 col-xl-4 d-flex align-items-center">
              <button v-if="hasActiveFilters" class="btn btn-link btn-sm text-decoration-none p-0" @click="clearFilters">
                <i class="bi bi-x-circle me-1"></i>{{ $t('dev_clear_filters') }}
              </button>
            </div>
          </div>
        </div>
      </div>
      <div class="ms-auto d-flex gap-2 align-items-center">
        <!-- 批量操作条（§8.3：选中后出现，不改变页面布局） -->
        <div v-if="selectedDevices.length > 0" class="noyo-batch-bar">
          <span class="fw-bold me-2">{{ selectedDevices.length }} {{ $t('selected') }}</span>
          <div class="btn-group btn-group-sm">
            <button class="btn btn-outline-success" @click="batchEnable" v-permission="'device:control'">
              <i class="bi bi-check-circle me-1"></i>{{ $t('dev_enable') }}
            </button>
            <button class="btn btn-outline-secondary" @click="batchDisable" v-permission="'device:control'">
              <i class="bi bi-x-circle me-1"></i>{{ $t('dev_disable') }}
            </button>
            <button class="btn btn-outline-danger" @click="batchDelete" v-permission="'device:delete'">
              <i class="bi bi-trash me-1"></i>{{ $t('dev_delete') }}
            </button>
          </div>
          <button class="btn btn-link text-muted p-0 ms-2" @click="selectedDevices = []" :aria-label="$t('common_close')">
            <i class="bi bi-x-lg"></i>
          </button>
        </div>
        <!-- 次要操作（ghost 按钮，§8.1） -->
        <button class="btn btn-outline-primary btn-sm" @click="downloadTemplate" v-permission="'device:create'">
          <i class="bi bi-download me-1"></i> {{ $t('download_template') }}
        </button>
        <button class="btn btn-outline-primary btn-sm" @click="triggerImport" v-permission="'device:create'">
          <i class="bi bi-upload me-1"></i> {{ $t('import_devices') }}
        </button>
        <input type="file" ref="fileInput" class="d-none" accept=".xlsx" @change="handleFileUpload">
        <button class="btn btn-outline-info btn-sm" @click="showDiscoveryModal = true" :title="$t('discover_devices')" v-permission="'device:create'">
          <i class="bi bi-search"></i>
        </button>
      </div>
    </div>

    <!-- 表格容器 -->
    <div class="card border-0 shadow-sm">
      <div class="card-body p-0">
        <div class="table-responsive device-table-wrap" style="min-height: 400px;">
          <table class="table table-hover align-middle mb-0 table-compact">
            <thead>
              <tr>
                <th class="ps-4" style="width: 36px">
                  <input class="form-check-input" type="checkbox" :checked="allSelected" @change="toggleAll" :aria-label="$t('selected')">
                </th>
                <th>{{ $t('dev_code') }}</th>
                <th>{{ $t('dev_name') }}</th>
                <th v-if="showProjectColumn">{{ $t('project_name') }}</th>
                <th>{{ $t('dev_tags') }}</th>
                <th>{{ $t('dev_online_status') }}</th>
                <th>{{ $t('dev_product') }}</th>
                <th>{{ $t('ai_health') }}</th>
                <th>{{ $t('dev_status') }}</th>
                <th class="text-end pe-4">{{ $t('tsl_actions') }}</th>
              </tr>
            </thead>
          <tbody>
            <!-- 骨架屏（§8.7：镜像表格结构） -->
            <tr v-if="loading" v-for="n in 6" :key="'sk-' + n">
              <td class="ps-4"><div class="skeleton" style="width: 16px; height: 16px;"></div></td>
              <td><div class="skeleton" style="width: 110px; height: 16px;"></div></td>
              <td><div class="skeleton" style="width: 130px; height: 16px;"></div></td>
              <td v-if="showProjectColumn"><div class="skeleton" style="width: 80px; height: 16px;"></div></td>
              <td><div class="skeleton" style="width: 100px; height: 16px;"></div></td>
              <td><div class="skeleton" style="width: 90px; height: 20px;"></div></td>
              <td><div class="skeleton" style="width: 110px; height: 24px;"></div></td>
              <td><div class="skeleton" style="width: 80px; height: 20px;"></div></td>
              <td><div class="skeleton" style="width: 60px; height: 20px;"></div></td>
              <td class="text-end pe-4"><div class="skeleton ms-auto" style="width: 96px; height: 24px;"></div></td>
            </tr>
            <!-- 空状态：无设备（§8.6） -->
            <tr v-else-if="devices.length === 0">
              <td :colspan="showProjectColumn ? 11 : 10" class="p-0">
                <div class="empty-state">
                  <i class="bi bi-hdd-network"></i>
                  <p>{{ $t('dev_empty_message') }}</p>
                  <button v-permission="'device:create'" type="button" class="btn btn-primary" @click="openCreateModal">
                    {{ $t('dev_create') }}
                  </button>
                </div>
              </td>
            </tr>
            <!-- 空状态：搜索/筛选无结果（§10.3 空结果规则） -->
            <tr v-else-if="filteredDevices.length === 0">
              <td :colspan="showProjectColumn ? 11 : 10" class="p-0">
                <div class="empty-state">
                  <i class="bi bi-funnel"></i>
                  <p>{{ $t('dev_search_no_match') }}</p>
                  <button type="button" class="btn btn-primary" @click="clearFilters">
                    {{ $t('dev_clear_filters') }}
                  </button>
                </div>
              </td>
            </tr>
            <tr
              v-for="device in paginatedDevices"
              :key="device.code"
              class="device-row"
              @click="openDrawer(device)"
            >
              <td class="ps-4" @click.stop>
                <input class="form-check-input" type="checkbox" :checked="selectedDevices.includes(device.code)" @change="toggleSelection(device.code)" :aria-label="device.name || device.code">
              </td>
              <td class="fw-bold text-primary text-truncate font-mono" style="max-width: 200px;" :title="device.code">{{ device.code }}</td>
              <td class="text-truncate" style="max-width: 160px;" :title="device.name">{{ device.name || '-' }}</td>
              <td v-if="showProjectColumn">
                <span class="badge text-bg-light border">{{ device.project_name || '-' }}</span>
              </td>
              <td class="device-tags-cell">
                <div v-if="device.tags && device.tags.length" class="device-tag-chip-list">
                  <span
                    v-for="(tag, index) in device.tags.slice(0, 4)"
                    :key="tag.ID"
                    class="device-tag-chip"
                    :style="tagBadgeStyle(tag)"
                    :title="tag.description || tag.name"
                  >
                    <i :class="tag.icon || 'bi-tag'" class="device-tag-chip__icon"></i>
                    <span class="device-tag-chip__name">{{ tag.name }}</span>
                    <i class="bi bi-x device-tag-chip__close" @click.stop="unbindDeviceTag(device, tag)" :title="$t('dev_tag_unbind')" v-permission="'device:edit'"></i>
                  </span>
                  <span v-if="device.tags.length > 4" class="device-tag-chip device-tag-chip--overflow">
                    +{{ device.tags.length - 4 }}
                  </span>
                </div>
                <span v-else class="text-muted small">-</span>
              </td>
              <td>
                <!-- 状态指示器（§3.3：在线呼吸点 / 离线灰点） -->
                <span class="spec-badge" :class="device.online ? 'spec-badge--success' : 'spec-badge--neutral'">
                  <span class="status-dot" :class="device.online ? 'status-dot--online' : 'status-dot--offline'"></span>
                  {{ device.online ? $t('dev_online') : $t('dev_offline') }}
                </span>
                <div v-if="device.last_active && new Date(device.last_active).getFullYear() > 1" class="small text-muted mt-1 font-mono" style="font-size: 0.7rem">
                  {{ formatDateTime(device.last_active) }}
                </div>
              </td>
              <td class="text-truncate" style="max-width: 150px;" :title="getProductName(device.product_code)">
                <div class="d-flex flex-column align-items-start">
                  <span class="badge bg-light text-body border text-truncate w-100" style="vertical-align: middle;">{{ getProductName(device.product_code) }}</span>
                  <span v-if="device.protocol_profile_code" class="badge bg-primary-subtle text-primary border border-primary-subtle mt-1 text-truncate" style="font-size: 0.65rem; max-width: 100%;">
                    <i class="bi bi-hdd-network"></i> {{ getDriverName(device.protocol_profile_code) }}
                  </span>
                </div>
              </td>
              <td @click.stop="openSingleAIModal(device)">
                <!-- 在线 + 有得分 -->
                <div v-if="device.online && device.ai_health_score !== undefined && device.ai_health_score !== null"
                     class="d-flex align-items-center ai-health-cell"
                     :class="device.ai_latched ? 'text-danger' : (device.ai_health_score > 80 ? 'text-success' : (device.ai_health_score > 60 ? 'text-warning' : 'text-danger'))">
                  <i :class="device.ai_latched ? 'bi bi-shield-fill-x' : 'bi bi-shield-fill-check'" class="fs-5 me-1"></i>
                  <span class="fw-bold font-mono">{{ device.ai_latched ? (device.ai_health_trigger != null ? device.ai_health_trigger.toFixed(1) : aiText('异常', 'Anomaly')) : device.ai_health_score.toFixed(1) }}</span>
                  <i v-if="device.ai_latched" class="bi bi-lock-fill ms-1" :title="$t('ai_latched')"></i>
                </div>
                <!-- 离线 + 已配置 AI 设备守护 -->
                <div v-else-if="!device.online && (device.ai_health_details || configuredDeviceCodes.has(device.code))"
                     class="d-flex align-items-center text-secondary ai-health-cell"
                     style="opacity: 0.5">
                  <i class="bi bi-shield-slash fs-5 me-1"></i>
                  <span class="small">{{ $t('ai_offline') }}</span>
                </div>
                <!-- 已配置但还在生成中 -->
                <div v-else-if="configuredDeviceCodes.has(device.code)"
                     class="d-flex align-items-center text-primary ai-health-cell"
                     style="opacity: 0.8"
                     :title="aiText('AI守护已配置，正在生成初始数据...', 'AI Guardian configured; generating initial data…')">
                  <i class="bi bi-shield-check fs-5 me-1 animation-blink"></i>
                  <span class="small">{{ $t('ai_generating') }}</span>
                </div>
                <!-- 未配置 -->
                <div v-else class="text-muted small d-flex align-items-center" :title="aiText('点击配置 AI 设备守护', 'Click to configure AI Device Guardian')" style="opacity: 0.6">
                  <i class="bi bi-shield me-1"></i>
                  {{ $t('ai_not_configured') }}
                </div>
              </td>
              <td @click.stop>
                <div class="form-check form-switch" v-permission="'device:control'">
                  <input class="form-check-input" type="checkbox" role="switch" :checked="device.enabled" @click.prevent="toggleDevice(device)">
                  <label class="form-check-label small text-muted ms-1">{{ device.enabled ? $t('dev_enabled') : $t('dev_disabled') }}</label>
                </div>
              </td>
              <td class="text-end pe-4" @click.stop>
                <button v-if="isCameraDevice(device)" class="btn btn-sm btn-outline-primary rounded-circle me-1 d-inline-flex align-items-center justify-content-center" style="width: 28px; height: 28px; padding: 0;" @click="playVideo(device)" :title="aiText('播放实时视频', 'Play live video')" v-permission="'device:control'">
                  <i class="bi bi-play-fill fs-6"></i>
                </button>
                <div class="d-inline-block">
                  <button class="btn btn-sm btn-light border-0" type="button" :aria-expanded="activeDeviceActionMenu === device.code" @click="openDeviceActionMenu(device, $event)">
                    <i class="bi bi-three-dots-vertical"></i>
                  </button>
                  <ul
                    v-if="activeDeviceActionMenu === device.code"
                    class="dropdown-menu dropdown-menu-end shadow-sm border-0 show device-action-menu"
                    :style="{ top: `${deviceActionMenuPosition.top}px`, left: `${deviceActionMenuPosition.left}px` }"
                    @click.stop
                  >
                    <li><hr class="dropdown-divider"></li>
                    <li>
                      <a class="dropdown-item" href="#" @click.prevent="runDeviceMenuAction(() => openDataModal(device, 'realtime'))">
                        <i class="bi bi-activity me-2 text-success"></i> {{ $t('dev_data') }}
                      </a>
                    </li>
                    <template v-for="action in extensionDeviceActions" :key="action.name">
                      <li v-if="isActionVisible(action, device)">
                        <a class="dropdown-item" href="#" @click.prevent="runDeviceMenuAction(() => executeAction(action, device))">
                          <i :class="[action.icon, action.color, 'me-2']"></i> {{ action.labelKey ? $t(action.labelKey, action.label) : action.label }}
                        </a>
                      </li>
                    </template>
                    <li v-permission="'device:edit'">
                      <a class="dropdown-item" href="#" @click.prevent="runDeviceMenuAction(() => openDeviceTagsModal(device))">
                        <i class="bi bi-tags me-2 text-primary"></i> {{ $t('dev_tag_manage') }}
                      </a>
                    </li>
                    <li v-permission="'device:edit'">
                      <a class="dropdown-item" href="#" @click.prevent="runDeviceMenuAction(() => openSingleAIModal(device))">
                        <i class="bi bi-shield-check me-2 text-warning"></i> {{ aiText('AI 设备守护', 'AI Device Guardian') }}
                      </a>
                    </li>
                    <li v-permission="'device:control'">
                      <a class="dropdown-item" href="#" @click.prevent="runDeviceMenuAction(() => toggleDevice(device))">
                        <i class="bi me-2" :class="device.enabled ? 'bi-stop-fill text-warning' : 'bi-play-fill text-success'"></i>
                        {{ device.enabled ? $t('stop') : $t('start') }}
                      </a>
                    </li>
                    <li v-if="!device.parent_code || isChildOfCascade(device)" v-permission="'device:create'">
                      <a class="dropdown-item" href="#" @click.prevent="runDeviceMenuAction(() => openCreateSubDeviceModal(device))">
                        <i class="bi bi-plus-square me-2 text-primary"></i> {{ $t('dev_create_sub') }}
                      </a>
                    </li>
                    <li v-permission="'device:edit'">
                      <a class="dropdown-item" href="#" @click.prevent="runDeviceMenuAction(() => openEditModal(device))">
                        <i class="bi bi-pencil me-2 text-info"></i> {{ $t('dev_edit') }}
                      </a>
                    </li>
                    <li v-if="needsProtocolMapping(device)" v-permission="'device:edit'">
                      <a class="dropdown-item" href="#" @click.prevent="runDeviceMenuAction(() => openMappingModal(device))">
                        <i class="bi bi-diagram-3 me-2 text-body"></i> {{ $t('tsl_prop_proto_map') }}
                      </a>
                    </li>
                    <li><hr class="dropdown-divider"></li>
                    <li v-permission="'device:delete'">
                      <a class="dropdown-item text-danger" href="#" @click.prevent="runDeviceMenuAction(() => deleteDevice(device))">
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
    </div>
    <ListPagination :page="page" :page-size="pageSize" :total="total" id-prefix="devices" @update:page="changePage" @update:page-size="changePageSize" />

    <!-- 详情抽屉（§8.4：通用 DetailDrawer，与产品列表共用） -->
    <DetailDrawer :visible="drawerVisible" :title="drawerDevice?.name || drawerDevice?.code || ''" @close="closeDrawer">
      <template #header-badge>
        <span class="spec-badge" :class="drawerDevice?.online ? 'spec-badge--success' : 'spec-badge--neutral'">
          <span class="status-dot" :class="drawerDevice?.online ? 'status-dot--online' : 'status-dot--offline'"></span>
          {{ drawerDevice?.online ? $t('dev_online') : $t('dev_offline') }}
        </span>
      </template>
      <!-- 基本信息 -->
      <section class="noyo-drawer-section">
        <h6 class="noyo-drawer-section-title">{{ $t('drawer_basic_info') }}</h6>
        <dl class="row mb-0 fs-6">
          <dt class="col-4 text-secondary fw-normal">{{ $t('dev_code') }}</dt>
          <dd class="col-8 font-mono">{{ drawerDevice?.code || '-' }}</dd>
          <dt class="col-4 text-secondary fw-normal">{{ $t('dev_name') }}</dt>
          <dd class="col-8">{{ drawerDevice?.name || '-' }}</dd>
          <dt class="col-4 text-secondary fw-normal">{{ $t('dev_product') }}</dt>
          <dd class="col-8">{{ getProductName(drawerDevice?.product_code) }}</dd>
          <dt class="col-4 text-secondary fw-normal">{{ $t('dev_protocol') }}</dt>
          <dd class="col-8 font-mono">{{ getDeviceProtocol(drawerDevice) || '-' }}</dd>
          <dt class="col-4 text-secondary fw-normal">{{ $t('dev_parent') }}</dt>
          <dd class="col-8">{{ drawerDevice?.parent_code ? getDeviceName(drawerDevice.parent_code) : '-' }}</dd>
          <dt class="col-4 text-secondary fw-normal">{{ $t('dev_created') }}</dt>
          <dd class="col-8 font-mono">{{ formatDateTime(drawerDevice?.CreatedAt) }}</dd>
          <dt class="col-4 text-secondary fw-normal">{{ $t('dev_updated') }}</dt>
          <dd class="col-8 font-mono">{{ formatDateTime(drawerDevice?.UpdatedAt) }}</dd>
        </dl>
      </section>
      <!-- 实时数据（§9：Sparkline 双主题 + 数据新鲜度标记） -->
      <section class="noyo-drawer-section">
        <div class="d-flex justify-content-between align-items-center mb-2">
          <h6 class="noyo-drawer-section-title mb-0">{{ $t('drawer_realtime') }}</h6>
          <span v-if="drawerUpdatedAt" class="small text-muted font-mono">{{ $t('drawer_updated_at', { time: formatDateTime(drawerUpdatedAt.getTime()) }) }}</span>
        </div>
        <div v-if="drawerLoading" class="skeleton" style="height: 120px;"></div>
        <div v-else-if="drawerDisplayList.length === 0" class="text-muted text-center py-4 small">{{ $t('drawer_no_data') }}</div>
        <table v-else class="table table-sm table-borderless mb-0 align-middle">
          <tbody>
            <tr v-for="item in drawerDisplayList" :key="item.key">
              <td class="text-muted" :title="item.key" style="width: 32%">{{ item.name }}</td>
              <td style="width: 30%">
                <Sparkline :data="item.trend" :width="80" :height="20" :color="drawerSparkColor" />
              </td>
              <td class="text-end fw-bold font-mono" style="width: 38%">
                {{ item.value }} <span v-if="item.unit" class="text-muted fw-normal small">{{ item.unit }}</span>
              </td>
            </tr>
          </tbody>
        </table>
      </section>
      <!-- 标签 -->
      <section v-if="drawerDevice?.tags?.length" class="noyo-drawer-section">
        <h6 class="noyo-drawer-section-title">{{ $t('drawer_tags') }}</h6>
        <div class="device-tag-chip-list">
          <span v-for="tag in drawerDevice.tags" :key="tag.ID" class="device-tag-chip" :style="tagBadgeStyle(tag)">
            <i :class="tag.icon || 'bi-tag'" class="device-tag-chip__icon"></i>
            <span class="device-tag-chip__name">{{ tag.name }}</span>
          </span>
        </div>
      </section>
      <!-- AI 健康（原 AI Health Tooltip 信息并入抽屉） -->
      <section v-if="drawerDevice && (drawerDevice.ai_health_details || drawerDevice.ai_health_score != null || configuredDeviceCodes.has(drawerDevice.code))" class="noyo-drawer-section">
        <h6 class="noyo-drawer-section-title">{{ $t('drawer_ai_health') }}</h6>
        <template v-if="drawerDevice.online && drawerDevice.ai_health_details">
          <div class="d-flex justify-content-between align-items-center mb-2">
            <span class="fw-bold font-mono fs-5" :class="drawerDevice.ai_health_score > 80 ? 'text-success' : (drawerDevice.ai_health_score > 60 ? 'text-warning' : 'text-danger')">{{ drawerDevice.ai_health_score?.toFixed(1) }}</span>
            <i class="bi bi-shield-fill-check" :class="drawerDevice.ai_health_score > 80 ? 'text-success' : (drawerDevice.ai_health_score > 60 ? 'text-warning' : 'text-danger')"></i>
          </div>
          <div v-for="(score, prop) in drawerDevice.ai_health_details" :key="prop" class="d-flex justify-content-between align-items-center py-1">
            <span class="text-muted small">
              <i class="bi bi-circle-fill me-1" style="font-size: 0.5rem;" :class="score > 80 ? 'text-success' : (score > 60 ? 'text-warning' : 'text-danger')"></i>
              {{ prop }}
            </span>
            <span class="fw-bold font-mono" :class="score > 80 ? 'text-success' : (score > 60 ? 'text-warning' : 'text-danger')">{{ score.toFixed ? score.toFixed(1) : score }}</span>
          </div>
          <div class="text-muted small mt-1" style="font-size: 0.75rem;">{{ $t('drawer_health_min') }}</div>
        </template>
        <div v-else-if="!drawerDevice.online && configuredDeviceCodes.has(drawerDevice.code)" class="text-secondary small">
          <i class="bi bi-shield-slash me-1"></i>{{ $t('ai_offline') }}
          <div v-if="drawerDevice.last_active && new Date(drawerDevice.last_active).getFullYear() > 1" class="small text-muted mt-1 font-mono">
            {{ $t('drawer_last_active') }}: {{ formatDateTime(drawerDevice.last_active) }}
          </div>
        </div>
        <div v-else class="text-secondary small"><i class="bi bi-shield me-1"></i>{{ $t('ai_not_configured') }}</div>
      </section>
      <template #footer>
        <button class="btn btn-outline-secondary btn-sm" @click="openDataModal(drawerDevice, 'realtime')">
          <i class="bi bi-activity me-1"></i>{{ $t('drawer_view_data') }}
        </button>
        <button v-if="!drawerDevice?.parent_code || isChildOfCascade(drawerDevice)" class="btn btn-outline-secondary btn-sm" @click="openCreateSubDeviceModal(drawerDevice)">
          <i class="bi bi-plus-square me-1"></i>{{ $t('dev_create_sub') }}
        </button>
        <button v-if="needsProtocolMapping(drawerDevice)" class="btn btn-outline-secondary btn-sm" @click="openMappingModal(drawerDevice)">
          <i class="bi bi-diagram-3 me-1"></i>{{ $t('drawer_mapping') }}
        </button>
        <button class="btn btn-outline-primary btn-sm" @click="openSingleAIModal(drawerDevice)">
          <i class="bi bi-shield-check me-1"></i>{{ $t('drawer_configure_guardian') }}
        </button>
        <button class="btn btn-outline-secondary btn-sm" @click="openEditModal(drawerDevice)">
          <i class="bi bi-pencil me-1"></i>{{ $t('dev_edit') }}
        </button>
        <button class="btn btn-outline-danger btn-sm" @click="deleteDevice(drawerDevice)">
          <i class="bi bi-trash me-1"></i>{{ $t('drawer_delete') }}
        </button>
      </template>
    </DetailDrawer>

    <!-- Create Modal -->
    <div v-if="showCreateModal" class="modal fade show d-block modal-overlay-theme">
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
                </div>
                <div class="mb-3" v-if="!isSubDeviceForm">
                  <label class="form-label">{{ $t('sidebar_device_drivers', '设备驱动') }}</label>
                  <select v-model="newDevice.protocol_profile_code" class="form-select" @change="handleProtocolProfileChange">
                    <option value="">{{ $t('none', '无') }}</option>
                    <option v-for="d in drivers" :key="d.code" :value="d.code">{{ d.name }} ({{ d.code }})</option>
                  </select>
                </div>
                <div v-else class="alert alert-info py-2 small">
                  <i class="bi bi-info-circle me-1"></i>{{ $t('dev_sub_device_driver_inherited', '子设备不需要选择设备驱动，将依附父子系统的协议接入。') }}
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

    <!-- Device Tags Modal -->
    <div v-if="showDeviceTagsModal" class="modal fade show d-block modal-overlay-theme">
      <div class="modal-dialog">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title">{{ $t('dev_tag_manage') }} - {{ currentTagDevice?.code }}</h5>
            <button type="button" class="btn-close" @click="closeDeviceTagsModal"></button>
          </div>
          <div class="modal-body">
            <div v-if="deviceTags.length === 0" class="text-muted text-center py-4">
              {{ $t('dev_no_tags') }}
            </div>
            <div v-else class="device-tag-picker">
              <label
                v-for="tag in deviceTags"
                :key="tag.ID"
                class="d-flex align-items-center justify-content-between border rounded px-3 py-2 mb-2"
              >
                <span class="d-flex align-items-center gap-2">
                  <input class="form-check-input m-0" type="checkbox" :value="tag.ID" v-model="selectedTagIds">
                  <span class="device-tag-chip" :style="tagBadgeStyle(tag)">
                    <i :class="tag.icon || 'bi-tag'" class="device-tag-chip__icon"></i>
                    <span class="device-tag-chip__name">{{ tag.name }}</span>
                  </span>
                </span>
                <span class="text-muted small">{{ tag.device_count || 0 }}</span>
              </label>
            </div>
          </div>
          <div class="modal-footer">
            <button type="button" class="btn btn-secondary" @click="closeDeviceTagsModal">{{ $t('tsl_cancel') }}</button>
            <button type="button" class="btn btn-primary" @click="saveDeviceTags">{{ $t('common_save') }}</button>
          </div>
        </div>
      </div>
    </div>

    <!-- Mapping Modal -->
    <div v-if="showMappingModal" class="modal fade show d-block modal-overlay-theme">
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
    <div v-if="singleAIModalVisible" class="modal fade show d-block modal-overlay-theme" style="z-index: 1060;">
      <div class="modal-dialog modal-xl">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title">
              <i class="bi bi-shield-check text-warning me-2"></i>
              {{ aiText('AI 设备守护', 'AI Device Guardian') }} - {{ currentSingleAIDevice?.code }}
            </h5>
            <button type="button" class="btn-close" @click="closeSingleAIModal"></button>
          </div>
          <div class="modal-body">
            <div class="row h-100">
              <!-- Left side: Form -->
              <div class="col-md-4 border-end">
                <h6 class="mb-3 fw-bold">{{ aiText('守护配置', 'Guardian Settings') }}</h6>
                <div class="alert alert-info py-2 px-3 small mb-3">
                  <i class="bi bi-info-circle me-1"></i>
                  {{ aiText('首次启用或重新校准时，请确认设备处于正常工况；系统会把随后一段稳定数据学习为正常基线。', 'Before enabling or recalibrating, make sure the device is operating normally. The system will learn the next stable data segment as the normal baseline.') }}
                </div>
                <div class="mb-3">
                  <label class="form-label small fw-bold">{{ aiText('监测点', 'Monitoring Point') }}</label>
                  <select class="form-select form-select-sm" v-model="aiConfig.property">
                     <option value="" disabled>{{ aiText('-- 请选择数值型属性 --', '-- Select numeric property --') }}</option>
                     <option v-for="prop in getSingleAINumericProperties()" :key="prop.key" :value="prop.key">
                       {{ prop.name }} ({{ prop.key }})
                     </option>
                  </select>
                  <div class="form-text">{{ aiText('选择最能代表设备状态的温度、压力、电流等数值点位。', 'Choose a numeric point such as temperature, pressure, or current that best represents device condition.') }}</div>
                </div>
                <div class="mb-3">
                  <label class="form-label small fw-bold">
                    {{ aiText('回顾数据量', 'Review Points') }}
                    <i class="bi bi-info-circle text-muted ms-1" :title="aiText('每次评分前需要参考的最近数据点数量。点数越多越稳，点数越少响应越快。', 'Number of recent points used before each score. More points are steadier; fewer points respond faster.')"></i>
                  </label>
                  <div class="d-flex justify-content-between mb-1">
                     <span class="text-muted small">{{ aiText('用于判断趋势的最近数据', 'Recent data used for trend judgment') }}</span>
                     <span class="text-muted small fw-bold">{{ aiConfig.window_size }} {{ aiText('点', 'points') }}</span>
                  </div>
                  <input type="range" class="form-range" min="10" max="200" step="10" v-model.number="aiConfig.window_size">
                  <div class="form-text">{{ aiWindowSizeHelp }}</div>
                </div>
                <div class="mb-3">
                  <label class="form-label small fw-bold">
                    {{ aiText('告警灵敏度', 'Alert Sensitivity') }}
                    <i class="bi bi-info-circle text-muted ms-1" :title="aiText('越灵敏越容易发现小幅异常，也更可能误报；越宽松越少误报，但可能延迟发现异常。', 'Higher sensitivity catches smaller anomalies but may raise more false alarms. Lower sensitivity is quieter but may detect later.')"></i>
                  </label>
                  <div class="d-flex justify-content-between mb-1">
                     <span class="text-muted small">{{ aiSensitivityLabel }}</span>
                     <span class="text-muted small fw-bold">{{ aiConfig.threshold_sigma }}</span>
                  </div>
                  <div class="btn-group btn-group-sm w-100 mb-2" role="group" aria-label="AI sensitivity presets">
                    <button type="button" class="btn" :class="aiConfig.threshold_sigma === 2.5 ? 'btn-warning' : 'btn-outline-secondary'" @click="aiConfig.threshold_sigma = 2.5">{{ aiText('灵敏', 'Sensitive') }}</button>
                    <button type="button" class="btn" :class="aiConfig.threshold_sigma === 3.5 ? 'btn-warning' : 'btn-outline-secondary'" @click="aiConfig.threshold_sigma = 3.5">{{ aiText('均衡', 'Balanced') }}</button>
                    <button type="button" class="btn" :class="aiConfig.threshold_sigma === 5.0 ? 'btn-warning' : 'btn-outline-secondary'" @click="aiConfig.threshold_sigma = 5.0">{{ aiText('稳健', 'Stable') }}</button>
                  </div>
                  <input type="range" class="form-range" min="1.0" max="10.0" step="0.5" v-model.number="aiConfig.threshold_sigma">
                  <div class="form-text">{{ aiSensitivityHelp }}</div>
                </div>
                <div class="form-check form-switch mt-4">
                  <input class="form-check-input" type="checkbox" role="switch" id="singleEnableSwitch" v-model="aiConfig.enabled">
                  <label class="form-check-label text-warning fw-bold" for="singleEnableSwitch">{{ aiText('启用 AI 设备守护', 'Enable AI Device Guardian') }}</label>
                </div>
              </div>
              
              <!-- Right side: Chart -->
              <div class="col-md-8">
                <div class="d-flex justify-content-between align-items-center mb-2">
                  <h6 class="mb-0 fw-bold">{{ $t('ai_real_time_status') }}</h6>
                  <div class="small d-flex align-items-center">
                    {{ $t('ai_health_score') }}:
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
                       <i class="bi bi-lock-fill"></i> {{ $t('ai_latched') }}
                    </span>
                    <span v-else-if="aiLatestAnomaly" class="badge bg-danger animation-blink me-2">{{ $t('ai_anomaly_warning') }}</span>
                    
                    <button v-if="aiConfig.property" class="btn btn-sm btn-outline-primary py-0 px-2 me-2" style="font-size: 0.75rem" :disabled="aiRecalibrating || !aiConfig.enabled" @click="restartAIBaselineCalibration" :title="$t('ai_calibrate')">
                      <span v-if="aiRecalibrating" class="spinner-border spinner-border-sm me-1" role="status"></span>
                      <i v-else class="bi bi-arrow-clockwise"></i> {{ $t('ai_calibrate') }}
                    </button>
                    <button v-if="aiLatestLatched" class="btn btn-sm btn-outline-danger py-0 px-2" style="font-size: 0.75rem" @click="clearLatchedState" :title="$t('ai_unlock')">
                      <i class="bi bi-unlock"></i> {{ $t('ai_unlock') }}
                    </button>
                  </div>
                </div>
                <div v-if="aiConfig.enabled && aiConfig.property" class="border rounded bg-theme-surface px-3 py-2 mb-2">
                  <div class="d-flex justify-content-between align-items-center small mb-1">
                    <span class="fw-bold">
                      <i class="bi bi-activity me-1 text-primary"></i>
                      {{ aiText('计算进度', 'Calculation Progress') }}
                    </span>
                    <span class="text-muted">{{ aiProgress.current }} / {{ aiProgress.required }} · {{ aiProgress.percent }}%</span>
                  </div>
                  <div class="progress" style="height: 8px;">
                    <div class="progress-bar" :class="aiProgressBarClass" role="progressbar" :style="{ width: aiProgress.percent + '%' }" :aria-valuenow="aiProgress.percent" aria-valuemin="0" aria-valuemax="100"></div>
                  </div>
                  <div class="small text-muted mt-1">
                    {{ aiProgress.message }}
                    <span v-if="aiProgress.stage === 'calibrating'">{{ aiText('：正在采集正常样本，请保持设备在正常工况。', ': collecting normal samples. Keep the device operating normally.') }}</span>
                    <span v-else-if="aiProgress.stage === 'collecting_evaluation_window'">{{ aiText('：收到足够数据后会生成新的健康得分。', ': a new health score will be generated after enough data is received.') }}</span>
                  </div>
                </div>
                <div v-if="aiLatestLatched" class="alert alert-warning py-2 px-3 mb-2 d-flex align-items-center" style="font-size: 0.85rem">
                  <i class="bi bi-exclamation-triangle-fill me-2"></i>
                  {{ aiText('设备处于异常锁定状态，健康检测已暂停。请检查设备后手动解除锁定以恢复监控。', 'The device is in anomaly lock; health monitoring is paused. Inspect the device and clear the lock manually to resume.') }}
                </div>
                <div class="border rounded bg-theme-surface position-relative" style="height: 350px;">
                  <div v-if="aiChartLoading && (!aiChartOption || !aiChartOption.series)" class="position-absolute top-0 start-0 w-100 h-100 d-flex align-items-center justify-content-center bg-theme-surface bg-opacity-75" style="z-index: 10">
                     <div class="spinner-border text-primary" role="status"></div>
                  </div>
                  <VChart v-if="aiChartOption && aiChartOption.series && aiChartOption.series.length > 0" :option="aiChartOption" autoresize style="width: 100%; height: 100%;" />
                  <div v-else class="h-100 d-flex align-items-center justify-content-center text-muted">
                    {{ aiText('暂无 AI 数据，请配置并开启设备守护', 'No AI data yet. Configure and enable the guardian.') }}
                  </div>
                </div>
              </div>
            </div>
          </div>
          <div class="modal-footer">
            <button type="button" class="btn btn-outline-info me-auto" @click="openAIHistoryModal">
               <i class="bi bi-clock-history me-1"></i> {{ $t('ai_fault_history') }}
            </button>
            <button type="button" class="btn btn-outline-secondary" @click="closeSingleAIModal">{{ $t('tsl_cancel') }}</button>
            <button type="button" class="btn btn-primary" @click="saveSingleAIConfig">
              <i class="bi bi-save me-1"></i> {{ $t('ai_save_apply') }}
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- AI History Modal -->
    <div v-if="aiHistoryModalVisible" class="modal fade show d-block modal-overlay-theme" style="z-index: 1070;">
      <div class="modal-dialog modal-lg">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title"><i class="bi bi-clock-history text-danger me-2"></i>{{ aiText('AI 故障历史记录', 'AI Fault History') }}</h5>
            <button type="button" class="btn-close" @click="aiHistoryModalVisible = false"></button>
          </div>
          <div class="modal-body">
             <div v-if="aiHistoryLoading" class="text-center py-5">
               <div class="spinner-border text-primary" role="status"></div>
               <div class="mt-2 text-muted">{{ aiText('正在加载历史记录...', 'Loading history…') }}</div>
             </div>
             <div v-else-if="aiHistoryEvents.length === 0" class="text-center py-5 text-muted">
               <i class="bi bi-check-circle fs-1 text-success mb-2"></i>
               <div>{{ aiText('近7天内未检测到设备异常', 'No anomalies detected in the last 7 days') }}</div>
             </div>
             <div v-else>
               <div class="table-responsive">
                 <table class="table table-striped table-hover small">
                   <thead>
                     <tr>
                       <th>{{ aiText('发生时间', 'Time') }}</th>
                       <th>{{ aiText('监控属性', 'Property') }}</th>
                       <th>{{ aiText('健康评分', 'Health Score') }}</th>
                       <th>{{ aiText('原始值', 'Raw Value') }}</th>
                       <th>{{ aiText('残差值', 'Residual') }}</th>
                       <th>{{ aiText('阈值', 'Threshold') }} ($\sigma$)</th>
                     </tr>
                   </thead>
                   <tbody>
                     <tr v-for="(evt, idx) in aiHistoryEvents" :key="idx">
                       <td>{{ formatDateTime(evt.ts) }}</td>
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
            <button type="button" class="btn btn-outline-secondary" @click="aiHistoryModalVisible = false">{{ $t('common_close') }}</button>
          </div>
        </div>
      </div>
    </div>

    <!-- AI Batch Config Modal -->
    <div v-if="showBatchAIModal" class="modal fade show d-block modal-overlay-theme" style="z-index: 1060;">
      <div class="modal-dialog modal-lg">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title"><i class="bi bi-shield-check text-warning me-2"></i>{{ aiText('AI 设备守护批量配置', 'AI Device Guardian Batch Settings') }}</h5>
            <button type="button" class="btn-close" @click="showBatchAIModal = false"></button>
          </div>
          <div class="modal-body">
            <div class="alert alert-info py-2 small">
               <i class="bi bi-info-circle me-1"></i> {{ aiText('批量为同一产品下的多个设备下发相同守护配置。启用后，请确认这些设备当前都处于正常工况，系统会自动学习正常基线。', 'Apply the same guardian settings to devices of one product. Before enabling, make sure these devices are operating normally because baseline learning assumes normal operation.') }}
            </div>
            
            <div class="row g-3">
               <div class="col-md-6">
                  <label class="form-label fw-bold small">{{ aiText('所属产品', 'Product') }}</label>
                  <select class="form-select" v-model="batchAiConfig.product_code" @change="onBatchProductChange">
                     <option value="" disabled>{{ aiText('-- 请选择产品 --', '-- Select product --') }}</option>
                     <option v-for="p in products" :key="p.code" :value="p.code">{{ p.name }} ({{ p.code }})</option>
                  </select>
               </div>
               <div class="col-md-6">
                  <label class="form-label fw-bold small">{{ aiText('监测点', 'Monitoring Point') }}</label>
                  <select class="form-select" v-model="batchAiConfig.property" :disabled="!batchAiConfig.product_code">
                     <option value="" disabled>{{ aiText('-- 请选择数值型属性 --', '-- Select numeric property --') }}</option>
                     <option v-for="prop in batchProductProperties" :key="prop.key" :value="prop.key">
                       {{ prop.name }} ({{ prop.key }})
                     </option>
                  </select>
               </div>
            </div>

            <div class="mt-4" v-if="batchAiConfig.product_code">
               <label class="form-label fw-bold small">{{ aiText('选择目标应用设备 (可多选)', 'Select target devices (multi-select)') }}</label>
               <div class="border rounded p-2 bg-theme-surface" style="max-height: 200px; overflow-y: auto;">
                  <div v-if="batchDeviceList.length === 0" class="text-muted text-center py-3 small">{{ aiText('该产品下暂无设备', 'No devices under this product') }}</div>
                  <div class="form-check" v-for="dev in batchDeviceList" :key="dev.code">
                     <input class="form-check-input" type="checkbox" :value="dev.code" v-model="batchAiConfig.devices" :id="'batch_dev_' + dev.code">
                     <label class="form-check-label" :for="'batch_dev_' + dev.code">
                       {{ dev.name || dev.code }} <span class="text-muted small">({{ dev.code }})</span>
                     </label>
                  </div>
               </div>
               <div class="mt-2 text-primary small d-flex justify-content-between">
                  <span>
                    <a href="#" class="text-decoration-none me-3" @click.prevent="batchAiConfig.devices = batchDeviceList.map(d=>d.code)">{{ $t('select_all') }}</a>
                    <a href="#" class="text-decoration-none" @click.prevent="batchAiConfig.devices = []">反选</a>
                  </span>
                  <span>已选择 {{ batchAiConfig.devices.length }} {{ aiText('个设备', 'devices') }}</span>
               </div>
            </div>

            <div class="row g-3 mt-3">
               <div class="col-md-6">
                  <label class="form-label small fw-bold d-flex justify-content-between">
                     <span>{{ aiText('回顾数据量', 'Review Points') }}</span>
                     <span class="text-muted">{{ batchAiConfig.window_size }} {{ aiText('点', 'points') }}</span>
                  </label>
                  <input type="range" class="form-range" min="10" max="200" step="10" v-model.number="batchAiConfig.window_size">
                  <div class="form-text small">{{ aiText('点数越少响应越快，点数越多越稳健。', 'Fewer points respond faster; more points are steadier.') }}</div>
               </div>
               <div class="col-md-6">
                  <label class="form-label small fw-bold d-flex justify-content-between">
                     <span>{{ aiText('告警灵敏度', 'Alert Sensitivity') }}</span>
                     <span class="text-muted">{{ batchAiConfig.threshold_sigma }}</span>
                  </label>
                  <div class="btn-group btn-group-sm w-100 mb-2" role="group" aria-label="Batch AI sensitivity presets">
                    <button type="button" class="btn" :class="batchAiConfig.threshold_sigma === 2.5 ? 'btn-warning' : 'btn-outline-secondary'" @click="batchAiConfig.threshold_sigma = 2.5">{{ aiText('灵敏', 'Sensitive') }}</button>
                    <button type="button" class="btn" :class="batchAiConfig.threshold_sigma === 3.5 ? 'btn-warning' : 'btn-outline-secondary'" @click="batchAiConfig.threshold_sigma = 3.5">{{ aiText('均衡', 'Balanced') }}</button>
                    <button type="button" class="btn" :class="batchAiConfig.threshold_sigma === 5.0 ? 'btn-warning' : 'btn-outline-secondary'" @click="batchAiConfig.threshold_sigma = 5.0">{{ aiText('稳健', 'Stable') }}</button>
                  </div>
                  <input type="range" class="form-range" min="1.0" max="10.0" step="0.5" v-model.number="batchAiConfig.threshold_sigma">
                  <div class="form-text small">{{ aiText('灵敏更容易发现小变化；稳健更少误报。', 'Sensitive catches smaller changes; stable reduces false alarms.') }}</div>
               </div>
            </div>
          </div>
          <div class="modal-footer d-flex justify-content-between">
            <div class="form-check form-switch mt-1">
               <input class="form-check-input" type="checkbox" role="switch" id="batchEnableSwitch" v-model="batchAiConfig.enabled">
               <label class="form-check-label text-warning fw-bold" for="batchEnableSwitch">{{ aiText('立即启用监控', 'Enable Monitoring Now') }}</label>
            </div>
            <div>
               <button type="button" class="btn btn-outline-secondary me-2" @click="showBatchAIModal = false">{{ $t('tsl_cancel') }}</button>
               <button type="button" class="btn btn-primary" :disabled="!batchAiConfig.product_code || !batchAiConfig.property || batchAiConfig.devices.length === 0" @click="saveBatchAITasks">
                 <i class="bi bi-check2-all me-1"></i> {{ aiText('批量下发配置', 'Deploy Batch Config') }}
               </button>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Data Modal -->
    <DeviceDataModal 
      :visible="showDataModal" 
      :device="currentDataDevice" 
      :products="products" 
      @close="closeDataModal" 
    />
    
    <!-- Dynamic Extension Modals -->
    <component 
      v-for="modal in activeExtensionModals" 
      :key="modal.name"
      :is="modal.component"
      v-bind="modal.props"
      @close="closeExtensionModal(modal.name)"
    />

    <!-- Import Device Modal -->
    <div v-if="showImportModal" class="modal fade show d-block modal-overlay-theme">
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
    <div v-if="showDownloadModal" class="modal fade show d-block modal-overlay-theme">
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
                      <span v-if="getProductProtocol(p.code)" class="badge bg-primary-subtle text-primary border border-primary-subtle ms-1" style="font-size: 0.7rem">{{ getProductProtocol(p.code) }}</span>
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
import { ref, computed, onMounted, onUnmounted, watch, nextTick } from 'vue';
import axios from 'axios';
import { useI18n } from 'vue-i18n';
import SchemaForm from '../components/SchemaForm.vue';
import DeviceMappingEditor from '../components/device/DeviceMappingEditor.vue';
import DeviceDiscoveryModal from '../components/device/DeviceDiscoveryModal.vue';
import DeviceDataModal from '../components/device/DeviceDataModal.vue';
import ListPagination from '../components/ListPagination.vue';
import Sparkline from '../components/Sparkline.vue';
import DetailDrawer from '../components/DetailDrawer.vue';
import { usePlugins } from '../plugins/registry.js';
import { isSingleProjectMode } from '../utils/systemMode.js';
import { applyDriverDefaults } from '../utils/deviceDriverDefaults.js';
import { formatNamedReference } from '../utils/entityDisplay.js';
import { formatDateTime } from '../utils/dateTime.js';
import { useConfirm } from '../composables/useConfirm';
import { useToast } from '../composables/useToast';
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
const { confirmDialog } = useConfirm();
const { showToast } = useToast();

const devices = ref([]);
const configuredDeviceCodes = ref(new Set());
const products = ref([]);
const drivers = ref([]);
const loading = ref(false);
const showCreateModal = ref(false);
const showDiscoveryModal = ref(false);
const activeDeviceActionMenu = ref('');
const deviceActionMenuPosition = ref({ top: 0, left: 0 });
const showProjectColumn = computed(() => {
  const mode = localStorage.getItem('system_mode') || '';
  if (isSingleProjectMode(mode)) return false;
  return Number(localStorage.getItem('current_project_id') || 0) === 0;
});
const newDevice = ref({ code: '', name: '', product_code: '', protocol_profile_code: '', protocol_name: '', parent_code: '', enabled: true, config: {} });
const isSubDeviceForm = computed(() => !isEditing.value && !!newDevice.value.parent_code);
const currentSchema = ref(null);
const isEditing = ref(false);
const selectedDevices = ref([]);

// 详情抽屉状态（§8.4：右侧 720px 抽屉，替代原 hover 浮层）
const drawerVisible = ref(false);
const drawerDevice = ref(null);
const drawerData = ref({});
const drawerTrendData = ref({});
const drawerTSLMap = ref({});
const drawerLoading = ref(false);
const drawerUpdatedAt = ref(null);
let drawerTimer = null;
let drawerTrendTimer = null;

// Computed property for rich drawer data
const drawerDisplayList = computed(() => {
  if (!drawerData.value) return [];
  const keys = Object.keys(drawerData.value);
  const list = keys.map(key => {
    const tsl = drawerTSLMap.value[key];
    return {
      key: key,
      name: tsl ? tsl.name : key,
      unit: tsl && tsl.dataType && tsl.dataType.specs ? tsl.dataType.specs.unit : '',
      value: drawerData.value[key],
      trend: drawerTrendData.value[key] || []
    };
  });
  return list.sort((a, b) => a.key.localeCompare(b.key));
});

// 抽屉 Sparkline 颜色（§9 双主题）
const drawerSparkColor = computed(() => chartToken('--text-tertiary'));

// Filter State（§7.2：>3 个条件折叠收纳）
const search = ref('');
const showFilters = ref(false);
const filterProduct = ref('');
const filterParent = ref('');
const filterEnabled = ref('');
const filterOnline = ref('');
const filterTag = ref('');
const deviceTags = ref([]);

const hasActiveFilters = computed(() =>
  search.value.trim() !== ''
  || filterProduct.value !== ''
  || filterParent.value !== ''
  || filterEnabled.value !== ''
  || filterOnline.value !== ''
  || filterTag.value !== ''
);

const clearFilters = () => {
  search.value = '';
  filterProduct.value = '';
  filterParent.value = '';
  filterEnabled.value = '';
  filterOnline.value = '';
  filterTag.value = '';
};
const selectedTagIds = ref([]);
const showDeviceTagsModal = ref(false);
const currentTagDevice = ref(null);

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

const hasOwn = (obj, key) => Object.prototype.hasOwnProperty.call(obj, key);

const numericAIValue = (value) => {
  if (value === null || value === undefined || value === '') return null;
  const numberValue = Number(value);
  return Number.isFinite(numberValue) ? numberValue : null;
};

const isAIStatePayload = (properties) => {
  if (!properties || typeof properties !== 'object') return false;
  return Object.keys(properties).some(key =>
    key.endsWith('_ai_health') ||
    key.endsWith('_ai_anomaly') ||
    key.endsWith('_ai_latched') ||
    key.endsWith('_ai_health_trigger')
  );
};

const mergeAIStateIntoDevice = (device, properties) => {
  if (!device || !isAIStatePayload(properties)) return device;

  const patch = {};
  const healthDetails = { ...(device.ai_health_details || {}) };
  const latchedDetails = { ...(device._ai_latched_details || {}) };
  const anomalyDetails = { ...(device._ai_anomaly_details || {}) };
  let healthChanged = false;
  let latchedChanged = false;
  let anomalyChanged = false;

  Object.entries(properties).forEach(([key, value]) => {
    if (key.endsWith('_ai_health') && !key.endsWith('_ai_health_trigger')) {
      const prop = key.slice(0, -'_ai_health'.length);
      const score = numericAIValue(value);
      healthChanged = true;
      if (score === null) {
        delete healthDetails[prop];
      } else {
        healthDetails[prop] = score;
      }
      return;
    }

    if (key.endsWith('_ai_latched')) {
      const prop = key.slice(0, -'_ai_latched'.length);
      latchedDetails[prop] = value === true;
      latchedChanged = true;
      return;
    }

    if (key.endsWith('_ai_anomaly')) {
      const prop = key.slice(0, -'_ai_anomaly'.length);
      anomalyDetails[prop] = value === true;
      anomalyChanged = true;
      return;
    }

    if (key.endsWith('_ai_health_trigger')) {
      patch.ai_health_trigger = numericAIValue(value);
    }
  });

  if (healthChanged) {
    const scores = Object.values(healthDetails).filter(value => Number.isFinite(Number(value))).map(Number);
    patch.ai_health_details = scores.length > 0 ? healthDetails : null;
    patch.ai_health_score = scores.length > 0 ? Math.min(...scores) : null;
  }

  if (latchedChanged) {
    patch._ai_latched_details = latchedDetails;
    patch.ai_latched = Object.values(latchedDetails).some(Boolean);
    if (!patch.ai_latched && !hasOwn(patch, 'ai_health_trigger')) {
      patch.ai_health_trigger = null;
    }
  }

  if (anomalyChanged) {
    patch._ai_anomaly_details = anomalyDetails;
    patch.ai_anomaly = Object.values(anomalyDetails).some(Boolean);
  }

  return { ...device, ...patch };
};

const applyAIStatePayload = (deviceCode, properties) => {
  if (!deviceCode || !isAIStatePayload(properties)) return;

  const idx = devices.value.findIndex(device => device.code === deviceCode);
  if (idx !== -1) {
    devices.value[idx] = mergeAIStateIntoDevice(devices.value[idx], properties);
  }

  if (currentSingleAIDevice.value?.code === deviceCode) {
    currentSingleAIDevice.value = mergeAIStateIntoDevice(currentSingleAIDevice.value, properties);

    if (hasOwn(currentSingleAIDevice.value, 'ai_health_score')) {
      aiLatestHealth.value = currentSingleAIDevice.value.ai_health_score ?? null;
    }
    if (hasOwn(currentSingleAIDevice.value, 'ai_anomaly')) {
      aiLatestAnomaly.value = currentSingleAIDevice.value.ai_anomaly === true;
    }
    if (hasOwn(currentSingleAIDevice.value, 'ai_latched')) {
      aiLatestLatched.value = currentSingleAIDevice.value.ai_latched === true;
    }
    if (hasOwn(currentSingleAIDevice.value, 'ai_health_trigger')) {
      aiLatchTriggerScore.value = currentSingleAIDevice.value.ai_health_trigger ?? null;
    }
  }

  if (drawerDevice.value?.code === deviceCode) {
    drawerDevice.value = mergeAIStateIntoDevice(drawerDevice.value, properties);
  }
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

  eventSource = new EventSource('/api/devices/stream?token=' + encodeURIComponent(localStorage.getItem('access_token') || ''));

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
  eventSource.addEventListener('property.reported', (event) => {
    resetSSEHeartbeat();
    try {
      const data = JSON.parse(event.data);
      const properties = data.Payload || {};
      if (isAIStatePayload(properties)) {
        applyAIStatePayload(data.Topic, properties);
      }
    } catch (err) {
      console.error('[SSE] Failed to parse property.reported event', err);
    }
  });

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

const changePageSize = (size) => {
  pageSize.value = Number(size) || 10;
  page.value = 1;
};

// 过滤器变更时重置页码
watch([search, filterProduct, filterParent, filterEnabled, filterOnline, filterTag], () => {
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
  const q = search.value.trim().toLowerCase();
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
    if (filterTag.value !== '') {
        const wantTagId = Number(filterTag.value);
        if (!d.tags || !d.tags.some(t => t.ID === wantTagId)) return false;
    }
    if (q) {
      const ref = String(d.code || '').toLowerCase();
      const name = String(d.name || '').toLowerCase();
      if (!ref.includes(q) && !name.includes(q)) return false;
    }
    return true;
  });
});

// KPI 统计行（§7.3 / §3.3）
const stats = computed(() => {
  const list = devices.value;
  return {
    total: list.length,
    online: list.filter(d => d.online).length,
    offline: list.filter(d => !d.online).length,
    alarm: list.filter(d => d.ai_anomaly === true || d.ai_latched === true).length
  };
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

const tagBadgeStyle = (tag) => {
  const color = tag?.color || chartToken('--color-brand');
  const rgb = hexToRgb(color) || { r: 59, g: 130, b: 246 };
  return {
    '--tag-color': color,
    '--tag-rgb': `${rgb.r}, ${rgb.g}, ${rgb.b}`
  };
};

const hexToRgb = (color) => {
  if (!/^#([0-9a-f]{3}|[0-9a-f]{6})$/i.test(color || '')) return null;
  let hex = color.slice(1);
  if (hex.length === 3) {
    hex = hex.split('').map(char => char + char).join('');
  }
  const value = Number.parseInt(hex, 16);
  return {
    r: (value >> 16) & 0xff,
    g: (value >> 8) & 0xff,
    b: value & 0xff
  };
};

const getDriverProtocol = (profileCode) => {
  if (!profileCode) return '';
  const driver = drivers.value.find(drv => drv.code === profileCode);
  return driver ? driver.protocol_name : '';
};

const getProductProtocol = (productCode) => {
  if (!productCode) return '';
  const driver = drivers.value.find(drv => drv.product_code === productCode);
  return driver ? driver.protocol_name : '';
};

const getDeviceProtocol = (device) => {
  if (!device) return '';
  return device.protocol_name || getDriverProtocol(device.protocol_profile_code) || getProductProtocol(device.product_code);
};

const isCameraDevice = (device) => {
  const protocol = getDeviceProtocol(device);
  return protocol === 'gb28181' || device.protocol === 'gb28181' || device._protocol === 'gb28181';
};

const playVideo = (device) => {
  const action = extensionDeviceActions.value.find(a => a.name === 'gb28181-player');
  if (action && action.action) {
    executeAction(action, device);
  }
};

const getProductName = (code) => {
  const p = products.value.find(prod => prod.code === code);
  return p ? `${p.name} (${p.code})` : code;
};

const getDriverName = (code) => {
  if (!code) return '';
  const d = drivers.value.find(drv => drv.code === code);
  return d ? formatNamedReference(d.name, d.code) : code;
};

const batchDelete = async () => {
  const count = selectedDevices.value.length;
  if (count === 0) return;
  // 破坏性操作确认（§8.4 / §10.3）
  const ok = await confirmDialog({
    title: t('dev_batch_delete_title', { count }),
    message: t('dev_batch_delete_message', { count }),
    variant: 'danger',
    confirmText: t('common_delete'),
    cancelText: t('common_cancel')
  });
  if (!ok) return;
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
  showToast('success', t('dev_deleted_success'));
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
  showToast('success', t('dev_action_success'));
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
  showToast('success', t('dev_action_success'));
};

// Data Modal State
const showDataModal = ref(false);
const currentDataDevice = ref(null);

// Dynamic Extension Actions & Modals
const { extensions } = usePlugins();
const extensionDeviceActions = computed(() => extensions.value.deviceActions || []);
const activeExtensionModals = ref([]);

const closeDeviceActionMenu = () => {
  activeDeviceActionMenu.value = '';
};

const openDeviceActionMenu = (device, event) => {
  const rect = event.currentTarget.getBoundingClientRect();
  const menuWidth = 220;
  const left = Math.max(8, Math.min(rect.right - menuWidth, window.innerWidth - menuWidth - 8));
  activeDeviceActionMenu.value = activeDeviceActionMenu.value === device.code ? '' : device.code;
  deviceActionMenuPosition.value = {
    top: Math.min(rect.bottom + 4, window.innerHeight - 8),
    left
  };
};

const runDeviceMenuAction = (fn) => {
  closeDeviceActionMenu();
  fn();
};

const isActionVisible = (action, device) => {
  if (action.condition && typeof action.condition === 'function') {
    return action.condition(device);
  }
  return true;
};

const executeAction = (action, device) => {
  if (action.action && typeof action.action === 'function') {
    // Pass a local emit-like function that handles 'open-modal'
    action.action(device, (eventName, payload) => {
      if (eventName === 'open-modal') {
        openExtensionModal(payload.name, action.component, { device: payload.device });
      }
    });
  }
};

const openExtensionModal = (name, component, props) => {
  const existing = activeExtensionModals.value.find(m => m.name === name);
  if (!existing) {
    activeExtensionModals.value.push({ name, component, props });
  } else {
    existing.props = props;
  }
};

const closeExtensionModal = (name) => {
  activeExtensionModals.value = activeExtensionModals.value.filter(m => m.name !== name);
};

const openDataModal = (device, tab = 'realtime') => {
  currentDataDevice.value = device;
  showDataModal.value = true;
};

const closeDataModal = () => {
  showDataModal.value = false;
  currentDataDevice.value = null;
};

// AI Predictive Maintenance State
const aiConfig = ref({
  enabled: false,
  property: '',
  window_size: 50,
  prediction_length: 1,
  threshold_sigma: 3.5,
  is_calibrated: false,
  progress: null,
});
const aiLatestHealth = ref(null);
const aiLatestAnomaly = ref(false);
const aiLatestLatched = ref(false);
const aiLatchTriggerScore = ref(null);
const aiChartOption = ref({});
const aiChartLoading = ref(false);
const aiRecalibrating = ref(false);
let aiChartTimer = null;
let aiConfigSaveTimer = null;
const singleAIModalVisible = ref(false);
const currentSingleAIDevice = ref(null);
const singleAITSLMap = ref({});

const aiText = (zh, en) => {
  const currentLocale = String(locale.value || locale || '').toLowerCase();
  return currentLocale.startsWith('en') ? en : zh;
};

// 读取 CSS 设计令牌供 ECharts 使用（§9.1/§9.2 双主题图表配色）
const chartToken = (name) => {
  try {
    const v = getComputedStyle(document.documentElement).getPropertyValue(name).trim();
    return v || '#3b82f6';
  } catch (e) {
    return '#3b82f6';
  }
};

const chartTokenAlpha = (name, alpha) => {
  const v = chartToken(name);
  const m = v.match(/^#([0-9a-f]{6})$/i);
  if (!m) return v;
  const a = Math.round(Math.max(0, Math.min(1, alpha)) * 255).toString(16).padStart(2, '0');
  return v + a;
};

const defaultAIProgress = () => ({
  stage: aiConfig.value.enabled ? 'collecting_evaluation_window' : 'disabled',
  current: 0,
  required: aiConfig.value.window_size || 50,
  percent: 0,
  message: '',
  is_learning: false,
});

const aiProgressMessage = (stage, fallback) => {
  const messages = {
    disabled: aiText('AI 设备守护未启用', 'AI Device Guardian is disabled'),
    collecting_window: aiText('正在收集回顾数据', 'Collecting review data'),
    calibrating: aiText('正在学习正常基线', 'Learning the normal baseline'),
    collecting_evaluation_window: aiText('正在收集本次评分所需的回顾数据', 'Collecting review data for the next score'),
    ready: aiText('回顾数据已就绪，正在持续监测', 'Review data is ready; monitoring is active'),
  };
  return messages[stage] || fallback || aiText('正在等待设备数据', 'Waiting for device data');
};

const aiProgress = computed(() => {
  const progress = aiConfig.value.progress || defaultAIProgress();
  const required = Number(progress.required || aiConfig.value.window_size || 50);
  const current = Math.min(Number(progress.current || 0), required);
  const percent = Math.max(0, Math.min(100, Number(progress.percent ?? (required > 0 ? Math.round(current * 100 / required) : 0))));
  return {
    ...progress,
    current,
    required,
    percent,
    message: aiProgressMessage(progress.stage, progress.message),
  };
});

const aiProgressBarClass = computed(() => {
  if (aiProgress.value.stage === 'ready') return 'bg-success';
  if (aiProgress.value.stage === 'calibrating') return 'bg-warning';
  if (aiProgress.value.stage === 'disabled') return 'bg-secondary';
  return 'bg-info';
});

const aiSensitivityLabel = computed(() => {
  const value = Number(aiConfig.value.threshold_sigma || 3.5);
  if (value <= 2.5) return aiText('灵敏：更早发现异常', 'Sensitive: detects anomalies earlier');
  if (value >= 5) return aiText('稳健：减少误报', 'Stable: reduces false alarms');
  return aiText('均衡：推荐设置', 'Balanced: recommended');
});

const aiSensitivityHelp = computed(() => {
  const value = Number(aiConfig.value.threshold_sigma || 3.5);
  if (value <= 2.5) return aiText('适合关键设备或异常代价高的场景，可能增加误报。', 'Best for critical equipment or costly anomalies; may increase false alarms.');
  if (value >= 5) return aiText('适合波动较大的点位，告警更保守。', 'Best for noisy points; alerts are more conservative.');
  return aiText('适合大多数温度、压力、电流等连续数值点位。', 'Works well for most continuous points such as temperature, pressure, and current.');
});

const aiWindowSizeHelp = computed(() => {
  if (aiConfig.value.window_size <= 30) return aiText('响应更快，适合变化频繁的点位。', 'Responds faster; suitable for frequently changing points.');
  if (aiConfig.value.window_size >= 100) return aiText('更稳健，适合慢变化设备。', 'More stable; suitable for slow-changing equipment.');
  return aiText('均衡设置，适合大多数设备。', 'Balanced setting for most devices.');
});

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

const openDrawer = async (device) => {
  if (!device) return;
  drawerDevice.value = { ...device };
  drawerData.value = {};
  drawerTrendData.value = {};
  drawerTSLMap.value = {};
  drawerLoading.value = true;
  drawerUpdatedAt.value = null;
  drawerVisible.value = true;

  // 焦点管理（§12：打开时聚焦抽屉，Esc 关闭）
  await nextTick();
  document.querySelector('.noyo-drawer')?.focus?.();

  // Load TSL for the drawer device
  const product = products.value.find(p => p.code === device.product_code);
  if (product && product.config) {
      try {
          const prodConfig = JSON.parse(product.config);
          const tslProps = prodConfig.tsl?.properties || [];
          tslProps.forEach(p => {
              drawerTSLMap.value[p.identifier] = p;
          });
      } catch (e) {
          // silent fail
      }
  }

  // Fetch initial data
  await fetchDrawerData(device.code);
  fetchDrawerTrend(device.code);
  drawerLoading.value = false;

  // Poll（§10.1：实时数据到达即更新）
  // 竞态守卫：await 期间用户可能已切换设备，仅当仍是当前设备才接管轮询
  if (drawerDevice.value?.code !== device.code) return;
  if (drawerTimer) clearInterval(drawerTimer);
  if (drawerTrendTimer) clearInterval(drawerTrendTimer);
  drawerTimer = setInterval(() => fetchDrawerData(device.code), 2000);
  drawerTrendTimer = setInterval(() => fetchDrawerTrend(device.code), 30000); // 30s for trend
};

const closeDrawer = () => {
  if (drawerTimer) {
    clearInterval(drawerTimer);
    drawerTimer = null;
  }
  if (drawerTrendTimer) {
    clearInterval(drawerTrendTimer);
    drawerTrendTimer = null;
  }
  drawerVisible.value = false;
  drawerDevice.value = null;
  drawerData.value = {};
  drawerTrendData.value = {};
  drawerTSLMap.value = {};
};

const fetchDrawerTrend = async (code) => {
  if (!drawerDevice.value || drawerDevice.value.code !== code) return;
  
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
      Object.keys(drawerTSLMap.value).forEach(key => {
        trendMap[key] = [];
      });

      list.forEach(item => {
        Object.keys(item).forEach(key => {
          if (key === 'ts' || key === '_type' || key === 'raw' || key === 'error') return;
          if (!trendMap[key]) trendMap[key] = [];
          trendMap[key].push(item[key]);
        });
      });
      
      // Only update if still showing same device
      if (drawerDevice.value && drawerDevice.value.code === code) {
          drawerTrendData.value = trendMap;
      }
    }
  } catch (e) {
    // silent fail
  }
};

const fetchDrawerData = async (code) => {
  if (!drawerDevice.value || drawerDevice.value.code !== code) return;
  try {
    const res = await axios.get(`/api/devices/${code}/data`);
    if (res.data.code === 0) {
      drawerData.value = res.data.data || {};
      drawerUpdatedAt.value = new Date();
    }
  } catch (e) {
    // silent fail
  }
};

const getDeviceName = (code) => {
  const d = devices.value.find(dev => dev.code === code);
  return d ? d.name : code;
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
            threshold_sigma: task.baseline?.threshold_sigma || 3.5,
            progress: task.progress || null,
        };
      } else {
         // default
        aiConfig.value = { enabled: false, property: '', window_size: 50, prediction_length: 1, threshold_sigma: 3.5, is_calibrated: false, progress: null };
      }
   } catch (e) {
      aiConfig.value.enabled = false;
   }
};

const fetchAIProgress = async () => {
  if (!currentSingleAIDevice.value || !aiConfig.value.property) return;
  try {
     const res = await axios.get(`/api/plugins/ai_predict/config/tasks/${currentSingleAIDevice.value.code}`);
     if (res.data.code === 0 && res.data.data && res.data.data.property === aiConfig.value.property) {
        aiConfig.value.progress = res.data.data.progress || null;
        aiConfig.value.is_calibrated = res.data.data.is_calibrated === true;
        aiConfig.value.is_latched = res.data.data.is_latched === true;
        if (res.data.data.baseline?.threshold_sigma) {
           aiConfig.value.baseline = {
              ...(aiConfig.value.baseline || {}),
              ...res.data.data.baseline,
           };
        }
     }
  } catch (e) {
     // Keep the previous progress display if the lightweight refresh fails.
  }
};

const saveSingleAIConfig = async () => {
    if (!currentSingleAIDevice.value) return;
    try {
        const { progress, ...configForSave } = aiConfig.value;
        // Wrap flat UI fields into the structured WatchTask for backend
        const payload = {
            ...configForSave,
            device_code: currentSingleAIDevice.value.code,
            baseline: {
                ...(aiConfig.value.baseline || {}),
                threshold_sigma: aiConfig.value.threshold_sigma
            }
        };
        const res = await axios.post(`/api/plugins/ai_predict/config/tasks`, payload);
        if (res.data.code === 0) {
            showToast('success', t('ai_config_saved'));
            fetchConfiguredTasks();
            fetchDeviceAIConfig();
            fetchAITrend(); // Refresh chart to show latest status
        } else {
            showToast('danger', t('ai_config_save_fail') + ': ' + res.data.message);
        }
    } catch (e) {
        console.error("Failed to update AI config", e);
        showToast('danger', t('ai_config_save_fail'));
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

      if (isEndingInCalculating) {
        // If the chart shows "Calculating" (dashed line), the text should also show "-" (null)
        aiLatestHealth.value = null;
      } else if (newestHealth !== null) {
        // The chart is already showing a fresh health line; keep the header in sync.
        aiLatestHealth.value = newestHealth;
      } else if (deviceInMemoryScore === null || deviceInMemoryScore === undefined) {
        aiLatestHealth.value = null;
      } else {
        aiLatestHealth.value = deviceInMemoryScore;
      }

      const chartStatePayload = {};
      if (newestHealth !== null) {
        chartStatePayload[healthKey] = aiLatestHealth.value;
      }
      if (foundNewestAnomaly) {
        chartStatePayload[anomalyKey] = aiLatestAnomaly.value;
      }
      if (foundNewestLatched) {
        chartStatePayload[latchedKey] = aiLatestLatched.value;
      }
      if (aiLatchTriggerScore.value !== null) {
        chartStatePayload[triggerKey] = aiLatchTriggerScore.value;
      }
      if (Object.keys(chartStatePayload).length > 0) {
        applyAIStatePayload(currentSingleAIDevice.value.code, chartStatePayload);
      }

      const seriesRawName = aiText('原始数值', 'Raw Value');
      const seriesHealthName = aiText('健康得分 (AI)', 'Health Score (AI)');
      const seriesLockedName = aiText('异常锁定', 'Anomaly Locked');
      const seriesCalculatingName = aiText('健康得分计算中', 'Calculating…');

      aiChartOption.value = {
         tooltip: { 
            trigger: 'axis',
            formatter: (params) => {
               if (!params || params.length === 0) return '';
               let timeStr = '';
               const item0 = params[0];
               if (Array.isArray(item0.value)) {
                  timeStr = formatDateTime(Number(item0.value[0]));
               } else if (item0.axisValue) {
                  timeStr = formatDateTime(Number(item0.axisValue));
               }
               
               let html = `<div style="margin-bottom: 3px; font-weight: bold;">${timeStr}</div>`;
               
               params.forEach(item => {
                  let val = Array.isArray(item.value) ? item.value[1] : item.value;
                  // Skip displaying null, undefined or NaN values
                  if (val !== null && val !== undefined && val !== '-' && !Number.isNaN(val)) {
                     // Optionally format numbers to fixed decimals if they are floats
                     let displayVal = val;
                     if (item.seriesName === seriesCalculatingName) {
                        displayVal = ''; // empty is cleaner if name is already the calculating label
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
         legend: { data: [seriesRawName, seriesHealthName, seriesLockedName, seriesCalculatingName], bottom: 0 },
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
               name: seriesRawName,
               type: 'line',
               data: processedRawData,
               smooth: true,
               showSymbol: false,
               connectNulls: false,
               yAxisIndex: 0
            },
            {
               name: seriesHealthName,
               type: 'line',
               data: healthData,
               smooth: true,
               showSymbol: false,
               connectNulls: false,
               yAxisIndex: 1,
               itemStyle: { color: chartToken('--color-warning') },
               areaStyle: {
                  color: chartTokenAlpha('--color-warning', 0.2)
               }
            },
            {
               name: seriesLockedName,
               type: 'line',
               data: lockedData,
               showSymbol: lockedData.length <= 5,
               connectNulls: false,
               yAxisIndex: 1,
               itemStyle: { color: chartToken('--color-danger') },
               lineStyle: { color: chartToken('--color-danger'), width: 3, type: 'dashed' }
            },
            {
               name: seriesCalculatingName,
               type: 'line',
               data: calculatingData,
               showSymbol: false,
               connectNulls: false,
               yAxisIndex: 1,
               itemStyle: { color: chartToken('--text-tertiary') },
               lineStyle: { color: chartToken('--text-tertiary'), width: 3, type: 'dotted' }
            }
         ],
         dataZoom: [{ type: 'inside' }]
      };
   } catch (e) {
      console.error(e);
   } finally {
      fetchAIProgress();
      aiChartLoading.value = false;
   }
};

const openDeviceTagsModal = (device) => {
  currentTagDevice.value = device;
  selectedTagIds.value = (device.tags || []).map(t => t.ID);
  showDeviceTagsModal.value = true;
};

const closeDeviceTagsModal = () => {
  showDeviceTagsModal.value = false;
  currentTagDevice.value = null;
  selectedTagIds.value = [];
};

const saveDeviceTags = async () => {
  if (!currentTagDevice.value) return;
  try {
    const res = await axios.put(
      `/api/devices/${currentTagDevice.value.code}/tags`,
      { tag_ids: selectedTagIds.value }
    );
    if (res.data && res.data.success) {
      // Refresh tags for this device in the list
      const device = devices.value.find(d => d.code === currentTagDevice.value.code);
      if (device) {
        device.tags = deviceTags.value.filter(t => selectedTagIds.value.includes(t.ID));
      }
      closeDeviceTagsModal();
      showToast('success', t('dev_action_success'));
    } else {
      showToast('danger', res.data?.message || t('common_save_fail'));
    }
  } catch (e) {
    console.error('Save device tags failed', e);
    showToast('danger', t('common_save_fail'));
  }
};

const fetchDeviceTags = async () => {
  try {
    const res = await axios.get('/api/device-tags');
    if (res.data && res.data.data) {
      deviceTags.value = res.data.data;
    }
  } catch (e) {
    console.error('Fetch device tags failed', e);
  }
};

const unbindDeviceTag = async (device, tag) => {
  // Remove a tag from a device directly via the chip close button
  const newTagIds = (device.tags || [])
    .filter(t => t.ID !== tag.ID)
    .map(t => t.ID);
  try {
    const res = await axios.put(`/api/devices/${device.code}/tags`, { tag_ids: newTagIds });
    if (res.data && res.data.code === 0) {
      // Update local device data immediately
      const localDevice = devices.value.find(d => d.code === device.code);
      if (localDevice) {
        localDevice.tags = (localDevice.tags || []).filter(t => t.ID !== tag.ID);
      }
    } else {
      console.warn('Unbind tag failed:', res.data?.message);
    }
  } catch (e) {
    console.error('Unbind device tag failed', e);
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

  if (aiChartTimer) clearInterval(aiChartTimer);
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
    // 破坏性操作确认（§8.4 / §10.3）
    const ok = await confirmDialog({
        title: aiText('解除异常锁定', 'Clear Anomaly Lock'),
        message: aiText(
            '解除后将立即恢复AI健康监测，系统将重新评估设备健康状态。',
            'After clearing, AI health monitoring will resume and the system will recalculate the device health score.'
        ),
        variant: 'danger',
        confirmText: t('common_confirm'),
        cancelText: t('common_cancel')
    });
    if (!ok) return;
    
    const taskId = `${currentSingleAIDevice.value.code}_${aiConfig.value.property}`;
    
    try {
        const res = await axios.post(`/api/plugins/ai_predict/latch/${taskId}/clear`);
        if (res.data.code === 0) {
            showToast('success', aiText('解除锁定成功', 'Lock cleared'));
            aiConfig.value.progress = {
                stage: 'collecting_evaluation_window',
                current: 0,
                required: aiConfig.value.window_size || 50,
                percent: 0,
                message: '',
                is_learning: false,
            };
            fetchAITrend(); // Refresh status immediately
        } else {
            showToast('danger', aiText('解除失败: ', 'Clear failed: ') + res.data.message);
        }
    } catch (e) {
        console.error(e);
        showToast('danger', aiText('解除失败', 'Clear failed'));
    }
};

const restartAIBaselineCalibration = async () => {
    if (!currentSingleAIDevice.value || !aiConfig.value.property) return;
    // 破坏性操作确认（§8.4 / §10.3）
    const ok = await confirmDialog({
        title: aiText('重新校准基线', 'Restart Baseline Calibration'),
        message: aiText(
            '系统会清除当前学习到的正常基线，并把接下来一段数据重新学习为正常工况。请确认设备现在处于正常状态。',
            'The system will clear the learned baseline and learn the next data segment as normal operation. Make sure the device is currently operating normally.'
        ),
        variant: 'danger',
        confirmText: t('common_confirm'),
        cancelText: t('common_cancel')
    });
    if (!ok) return;

    const taskId = `${currentSingleAIDevice.value.code}_${aiConfig.value.property}`;
    aiRecalibrating.value = true;
    try {
        const res = await axios.post(`/api/plugins/ai_predict/calibration/${taskId}/restart`);
        if (res.data.code === 0) {
            aiLatestHealth.value = null;
            aiLatestAnomaly.value = false;
            aiLatestLatched.value = false;
            aiLatchTriggerScore.value = null;
            aiConfig.value.is_calibrated = false;
            aiConfig.value.progress = {
                stage: 'collecting_window',
                current: 0,
                required: aiConfig.value.window_size || 50,
                percent: 0,
                message: '',
                is_learning: true,
            };
            await fetchDeviceAIConfig();
            fetchConfiguredTasks();
            fetchAITrend();
            showToast('success', aiText('已开始重新校准，请保持设备处于正常工况。', 'Recalibration started. Keep the device operating normally.'));
        } else {
            showToast('danger', aiText('重新校准失败: ', 'Recalibration failed: ') + res.data.message);
        }
    } catch (e) {
        console.error(e);
        showToast('danger', aiText('重新校准失败', 'Recalibration failed'));
    } finally {
        aiRecalibrating.value = false;
    }
};


onUnmounted(() => {
  if (drawerTimer) clearInterval(drawerTimer);
  if (drawerTrendTimer) clearInterval(drawerTrendTimer);
  if (aiChartTimer) clearInterval(aiChartTimer);
  if (aiConfigSaveTimer) clearTimeout(aiConfigSaveTimer);
  window.removeEventListener('noyo-data-updated', fetchDevices);
  window.removeEventListener('click', closeDeviceActionMenu);
  window.removeEventListener('scroll', closeDeviceActionMenu, true);
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

const fetchDrivers = async () => {
  try {
    const res = await axios.get('/api/protocol-profiles', { params: { page: 0, pageSize: 1000 } });
    if (res.data.code === 0 || res.data.data) {
      drivers.value = res.data.data || [];
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
  let protocolName = device.protocol_name;
  if (!protocolName && device.parent_code) {
    const parentDev = devices.value.find(d => d.code === device.parent_code);
    if (parentDev) {
      protocolName = parentDev.protocol_name || '';
    }
  }
  if (!protocolName) return false;
  const required = protocolMappingMap.value[protocolName];
  return required !== false; // 默认 true
};

const getProtocol = (productCode) => {
    return getProductProtocol(productCode);
};

const isChildOfCascade = (device) => {
  if (!device.parent_code) return false;
  const parent = devices.value.find(d => d.code === device.parent_code);
  if (!parent) return false;
  return getDeviceProtocol(parent) === 'cascade';
};

const fetchProtocolSchema = async (protocolName, parentCode, profileCode, defaultConfig = null) => {
    if (parentCode) {
        let parentDevice = devices.value.find(d => d.code === parentCode);
        try {
            if (!parentDevice) {
                const res = await axios.get(`/api/devices/${parentCode}`);
                if (res.data.code === 0 && res.data.data) {
                    parentDevice = res.data.data;
                }
            }
            if (parentDevice) {
                protocolName = protocolName || parentDevice.protocol_name;
                profileCode = profileCode || parentDevice.protocol_profile_code;
            }
        } catch (e) {
            console.error("Failed to fetch parent device for protocol", e);
        }
    }
    
    if (!protocolName) {
      currentSchema.value = null;
      return;
    }
    try {
      const params = new URLSearchParams();
      params.append('protocolName', protocolName);
      params.append('type', 'device');
      if (newDevice.value.product_code) params.append('productCode', newDevice.value.product_code);
      if (parentCode) params.append('isSubDevice', 'true');
      if (profileCode) params.append('profileCode', profileCode);
      
      const res = await axios.get(`/api/devices/config-schema?${params.toString()}`);
      if (res.data.code === 0) {
        currentSchema.value = res.data.data;
      
      // Apply defaults from schema to config if missing
      if (currentSchema.value && currentSchema.value.properties) {
          let config = newDevice.value.config || {};
          let changed = false;
          if (defaultConfig && typeof defaultConfig === 'object') {
              const beforeDefaults = JSON.stringify(config);
              config = applyDriverDefaults(config, defaultConfig, currentSchema.value);
              changed = JSON.stringify(config) !== beforeDefaults;
          }
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
    // config schema is now fetched via protocol change, not product change.
  };

const handleProtocolProfileChange = () => {
    if (isSubDeviceForm.value) {
      clearDeviceDriverSelection();
      fetchProtocolSchema('', newDevice.value.parent_code, '');
      return;
    }
    newDevice.value.config = {}; // Reset config
    const selectedDriver = drivers.value.find(d => d.code === newDevice.value.protocol_profile_code);
    if (selectedDriver) {
      newDevice.value.protocol_name = selectedDriver.protocol_name;
      let defaultConf = {};
      if (selectedDriver.config) {
        try {
          defaultConf = JSON.parse(selectedDriver.config);
        } catch (e) {
          console.error("Failed to parse driver config", e);
        }
      }
      fetchProtocolSchema(selectedDriver.protocol_name, newDevice.value.parent_code, selectedDriver.code, defaultConf);
    } else {
      newDevice.value.protocol_name = '';
      currentSchema.value = null;
    }
  };

const clearDeviceDriverSelection = () => {
  newDevice.value.protocol_profile_code = '';
  newDevice.value.protocol_name = '';
};

// 监听父设备变化，重新加载 Schema（子设备 Schema 可能不同）
watch(() => newDevice.value.parent_code, (newParentCode) => {
    if (isSubDeviceForm.value) {
      clearDeviceDriverSelection();
      fetchProtocolSchema('', newParentCode, '');
      return;
    }
    fetchProtocolSchema(newDevice.value.protocol_name, newParentCode, newDevice.value.protocol_profile_code);
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
  fetchDrivers();
  fetchPluginsInfo();
  fetchConfiguredTasks();
  fetchDeviceTags();
  window.addEventListener('noyo-data-updated', fetchDevices);
  window.addEventListener('click', closeDeviceActionMenu);
  window.addEventListener('scroll', closeDeviceActionMenu, true);

  // 建立SSE连接，支持心跳检测和自动重连
  setupSSE();
});

const openCreateModal = () => {
  isEditing.value = false;
  newDevice.value = { code: '', name: '', product_code: '', protocol_profile_code: '', protocol_name: '', parent_code: '', enabled: true, config: {} };
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
  return getProductProtocol(firstCode) || null;
});

const isProductDisabled = (p) => {
  if (!targetProtocol.value) return false;
  return getProductProtocol(p.code) !== targetProtocol.value;
};

const downloadTemplate = () => {
  selectedProductsForTemplate.value = [];
  showDownloadModal.value = true;
};

const selectAllProducts = () => {
  let proto = targetProtocol.value;
  if (!proto && products.value.length > 0) {
    // If no protocol selected yet, default to the first product's protocol
    proto = getProductProtocol(products.value[0].code);
  }
  
  if (proto) {
    // Only select products matching the protocol
    selectedProductsForTemplate.value = products.value
      .filter(p => getProductProtocol(p.code) === proto)
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
      importProtocol.value = getProductProtocol(filterProduct.value);
  }
  showImportModal.value = true;
};

const showImportModal = ref(false);
const importProtocol = ref('');
const importFile = ref(null);

const availableProtocols = computed(() => {
    const protos = new Set(drivers.value.map(d => d.protocol_name).filter(Boolean));
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
      showToast('success', t('import_success') + ': ' + res.data.message); // message contains count
      fetchDevices();
      closeImportModal();
    } else {
      showToast('danger', t('import_fail') + ': ' + res.data.message);
    }
  } catch (e) {
    showToast('danger', t('import_fail'));
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
  
  // Fetch Schema
  fetchProtocolSchema(device.protocol_name, device.parent_code, device.protocol_profile_code);
  
  showCreateModal.value = true;
};

const saveDevice = async () => {
  try {
    // Prepare payload: stringify config
    const payload = {
      ...newDevice.value,
      protocol_profile_code: isSubDeviceForm.value ? '' : newDevice.value.protocol_profile_code,
      protocol_name: isSubDeviceForm.value ? '' : newDevice.value.protocol_name,
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
      showToast('success', t('dev_save_success'));
      newDevice.value = { code: '', name: '', product_code: '', protocol_profile_code: '', protocol_name: '', parent_code: '', enabled: true, config: {} };
      currentSchema.value = null;
    } else {
      showToast('danger', res.data.message);
    }
  } catch (e) {
    console.error(e);
    showToast('danger', t('common_save_fail'));
  }
};

const deleteDevice = async (device) => {
  // 破坏性操作确认（§8.4 / §10.3，写明设备名与后果）
  const ok = await confirmDialog({
    title: t('dev_delete_title'),
    message: t('dev_delete_message', { name: device.name || device.code }),
    variant: 'danger',
    confirmText: t('common_delete'),
    cancelText: t('common_cancel')
  });
  if (!ok) return;
  try {
    const res = await axios.delete(`/api/devices/${device.code}`);
    if (res.data.code === 0) {
      if (drawerDevice.value?.code === device.code) closeDrawer();
      showToast('success', t('dev_deleted_success'));
      fetchDevices();
    } else {
      showToast('danger', res.data.message);
    }
  } catch (e) {
    console.error(e);
    showToast('danger', t('common_delete_fail'));
  }
};

const openCreateSubDeviceModal = (parentDevice) => {
  isEditing.value = false;
  newDevice.value = { 
    code: '', 
    name: '',
    product_code: '', 
    protocol_profile_code: '',
    protocol_name: '',
    parent_code: parentDevice.code, 
    enabled: true, 
    config: {} 
  };
  clearDeviceDriverSelection();
  currentSchema.value = null;
  showCreateModal.value = true;
};

const toggleDevice = async (device) => {
  const action = device.enabled ? 'stop' : 'start';
  try {
    const res = await axios.post(`/api/devices/${device.code}/${action}`);
    if (res.data.code === 0) {
      fetchDevices(); // Refresh list
      showToast('success', t('dev_action_success'));
    } else {
      showToast('danger', res.data.message);
    }
  } catch (e) {
    console.error(e);
    showToast('danger', t('dev_action_fail'));
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
      let protocolName = device.protocol_name;
        if (!protocolName && device.parent_code) {
            try {
                const res = await axios.get(`/api/devices/${device.parent_code}`);
                if (res.data.code === 0 && res.data.data) {
                    protocolName = res.data.data.protocol_name || '';
                }
            } catch (e) {
                console.error("Failed to fetch parent device for protocol", e);
            }
        }
        currentMappingProtocol.value = protocolName || '';
      
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
                if (parentDev.protocol_name === 'cascade') {
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
        showToast('success', t('dev_action_success'));
    } else {
        showToast('danger', res.data.message);
    }
  } catch (e) {
    console.error(e);
    showToast('danger', t('common_save_fail'));
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
            showToast('success', aiText('批量配置已下发', 'Batch config applied'));
        } else {
            showToast('danger', res.data.message || aiText('批量配置失败', 'Batch config failed'));
        }
    } catch (e) {
        console.error("Batch config fail", e);
        showToast('danger', aiText('提交配置异常', 'Failed to submit batch config'));
    }
};

</script>

<style scoped>
.animation-blink {
  animation: blinker 1.5s linear infinite;
}
.device-table-wrap {
  overflow-x: visible;
  overflow-y: visible;
}
.device-action-menu {
  position: fixed;
  display: block;
  min-width: 220px;
  z-index: 1080;
}
@media (max-width: 1200px) {
  .device-table-wrap {
    overflow-x: auto;
  }
}
.device-tags-cell {
  min-width: 120px;
  max-width: 260px;
}
.device-tag-chip-list {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}
.device-tag-chip {
  --tag-color: var(--color-brand);
  --tag-rgb: 59, 130, 246;
  display: inline-flex;
  align-items: center;
  gap: 3px;
  padding: 2px 4px 2px 6px;
  border-radius: 3px 5px 5px 3px;
  border: 1px solid rgba(var(--tag-rgb), 0.15);
  border-left-width: 3px;
  border-left-color: rgb(var(--tag-rgb));
  background: linear-gradient(135deg, rgba(var(--tag-rgb), 0.10) 0%, rgba(var(--tag-rgb), 0.04) 100%);
  color: rgb(var(--tag-rgb));
  font-size: 0.72rem;
  font-weight: 600;
  line-height: 1.6;
  cursor: default;
  transition: all 0.2s ease;
  flex-shrink: 0;
}
.device-tag-chip:hover {
  background: linear-gradient(135deg, rgba(var(--tag-rgb), 0.18) 0%, rgba(var(--tag-rgb), 0.08) 100%);
  border-color: rgba(var(--tag-rgb), 0.30);
  border-left-color: rgb(var(--tag-rgb));
  transform: translateY(-1px);
  box-shadow: 0 2px 8px rgba(var(--tag-rgb), 0.15);
}
.device-tag-chip__icon {
  font-size: 0.68rem;
  flex-shrink: 0;
}
.device-tag-chip__name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 80px;
}
.device-tag-chip__close {
  font-size: 0.65rem;
  cursor: pointer;
  padding: 1px;
  border-radius: 50%;
  flex-shrink: 0;
  opacity: 0.5;
  transition: all 0.15s ease;
  margin-left: 1px;
}
.device-tag-chip__close:hover {
  opacity: 1;
  background: rgba(var(--tag-rgb), 0.20);
  color: rgb(var(--tag-rgb));
}
.device-tag-chip--overflow {
  border-left-color: var(--text-tertiary);
  background: linear-gradient(135deg, color-mix(in srgb, var(--text-tertiary) 8%, transparent) 0%, color-mix(in srgb, var(--text-tertiary) 3%, transparent) 100%);
  border-color: color-mix(in srgb, var(--text-tertiary) 12%, transparent);
  color: var(--text-secondary);
  font-weight: 500;
  cursor: pointer;
}
.device-tag-chip--overflow:hover {
  background: linear-gradient(135deg, color-mix(in srgb, var(--text-tertiary) 14%, transparent) 0%, color-mix(in srgb, var(--text-tertiary) 6%, transparent) 100%);
  box-shadow: 0 2px 8px color-mix(in srgb, var(--text-tertiary) 12%, transparent);
  border-color: color-mix(in srgb, var(--text-tertiary) 22%, transparent);
}
@keyframes blinker {
  50% { opacity: 0; }
}

/* 批量操作条（§8.3：sticky 语义，选中后出现） */
.noyo-batch-bar {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 12px;
  border-radius: 8px;
  background: color-mix(in srgb, var(--color-info) 10%, var(--bg-elevated));
  border: 1px solid color-mix(in srgb, var(--color-info) 25%, transparent);
}

/* 筛选折叠面板（§7.2） */
.noyo-filter-panel {
  position: absolute;
  top: calc(100% + 4px);
  left: 0;
  z-index: 1040;
  width: min(560px, 90vw);
  padding: 16px;
  background: var(--bg-elevated);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-control);
  box-shadow: var(--shadow-floating);
}

[data-bs-theme='dark'] .noyo-filter-panel {
  box-shadow: var(--shadow-floating), var(--surface-highlight);
}
</style>
