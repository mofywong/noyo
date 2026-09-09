<template>
  <div class="alarm-center-page page-fixed-height">
    <!-- Header -->
    <div class="page-header list-page-header">
      <div>
        <h1>{{ t('title') }}</h1>
        <p class="page-subtitle">{{ t('subtitle') }}</p>
      </div>
      <div class="d-flex align-items-center gap-2">
        <LiquidGlassButton
          variant="secondary"
          size="sm"
          :icon="loading ? 'bi bi-arrow-repeat spin' : 'bi bi-arrow-clockwise'"
          :disabled="loading"
          @click="refreshAll"
        >
          {{ t('refresh') }}
        </LiquidGlassButton>
        <LiquidGlassButton
          variant="outline-secondary"
          size="sm"
          icon="bi bi-sliders"
          :disabled="loading"
          @click="openPolicies"
        >
          {{ t('policies') }}
        </LiquidGlassButton>
      </div>
    </div>

    <!-- Error Alert -->
    <div v-if="requestError" class="alert alert-danger alert-dismissible fade show flex-shrink-0" role="alert">
      <i class="bi bi-exclamation-octagon me-1"></i>{{ requestError }}
      <button type="button" class="btn-close" :aria-label="t('close')" @click="requestError = ''"></button>
    </div>

    <!-- Nav Tabs -->
    <ul class="nav nav-tabs mb-3 flex-shrink-0" role="tablist" :aria-label="t('viewSwitch')">
      <li class="nav-item" role="presentation">
        <button
          id="alarm-queue-tab"
          class="nav-link"
          :class="{ active: activeView === 'queue' }"
          type="button"
          role="tab"
          :aria-selected="activeView === 'queue'"
          @click="activeView = 'queue'"
        >
          <i class="bi bi-list-check me-1"></i>{{ t('queue') }}
          <span class="badge rounded-pill ms-1" :class="activeView === 'queue' ? 'text-bg-primary' : 'text-bg-secondary'">{{ total }}</span>
        </button>
      </li>
      <li class="nav-item" role="presentation">
        <button
          id="alarm-history-tab"
          class="nav-link"
          :class="{ active: activeView === 'history' }"
          type="button"
          role="tab"
          :aria-selected="activeView === 'history'"
          @click="openHistory"
        >
          <i class="bi bi-clock-history me-1"></i>{{ t('history') }}
          <span class="badge rounded-pill ms-1" :class="activeView === 'history' ? 'text-bg-primary' : 'text-bg-secondary'">{{ legacyTotal }}</span>
        </button>
      </li>
    </ul>

    <!-- Tab Content: Active Queue -->
    <template v-if="activeView === 'queue'">
      <div class="card border-0 shadow-sm table-glass-card d-flex flex-column flex-grow-1 min-h-0 overflow-hidden" role="tabpanel" aria-labelledby="alarm-queue-tab">
      <div class="card-header bg-transparent p-2 border-bottom flex-shrink-0 alarm-filter-toolbar" :aria-label="t('overview')">
        <CompactListMetrics v-model="activeQuickFilters" :metrics="statCards" class="alarm-filter-metrics" :aria-label="t('overview')" @toggle="applyQuickFilter" />
        <div class="alarm-filter-query">
          <div class="input-group input-group-sm">
            <span class="input-group-text bg-transparent"><i class="bi bi-search" aria-hidden="true"></i></span>
            <input
              id="alarm-search"
              v-model.trim="filters.search"
              class="form-control"
              :placeholder="t('searchPlaceholder')"
              :aria-label="t('searchPlaceholder')"
              @keyup.enter="applyFilters"
            >
          </div>
          <LiquidGlassPopover placement="bottom-start" panel-class="noyo-device-filter-popover" :label="t('filterConditions')" :trigger-label="t('filterConditions')">
            <template #trigger>
              <span class="btn btn-outline-secondary btn-sm device-filter-trigger">
                <i class="bi bi-funnel" aria-hidden="true"></i>
                <span>{{ t('filterConditions') }}</span>
                <span v-if="activeFilterLabels.length" class="device-filter-count">{{ activeFilterLabels.length }}</span>
              </span>
            </template>
            <div class="noyo-filter-panel">
              <div class="row g-2">
                <div class="col-12 col-md-6">
                  <select id="alarm-severity" v-model="filters.severity" class="form-select form-select-sm" @change="applyFilters">
                    <option value="">{{ t('severity') }}: {{ t('all') }}</option>
                    <option value="critical">{{ t('critical') }}</option>
                    <option value="major">{{ t('major') }}</option>
                    <option value="warning">{{ t('warning') }}</option>
                    <option value="info">{{ t('info') }}</option>
                  </select>
                </div>
                <div class="col-12 col-md-6">
                  <select id="alarm-condition" v-model="filters.condition" class="form-select form-select-sm" @change="applyFilters">
                    <option value="">{{ t('condition') }}: {{ t('all') }}</option>
                    <option value="firing">{{ t('firing') }}</option>
                    <option value="recovered">{{ t('recovered') }}</option>
                  </select>
                </div>
                <div class="col-12 col-md-6">
                  <select id="alarm-handling" v-model="filters.handling" class="form-select form-select-sm" @change="applyFilters">
                    <option value="">{{ t('handling') }}: {{ t('all') }}</option>
                    <option v-for="key in handlingKeys" :key="key" :value="key">{{ handlingText(key) }}</option>
                  </select>
                </div>
                <div class="col-12 col-md-6 d-flex align-items-center">
                  <div class="form-check form-switch m-0 small">
                    <input id="include-closed" v-model="filters.includeClosed" class="form-check-input" type="checkbox" @change="toggleIncludeClosed">
                    <label class="form-check-label ms-1" for="include-closed">{{ t('includeClosed') }}</label>
                  </div>
                </div>
                <div class="col-12 d-flex gap-2 justify-content-end">
                  <button type="button" class="btn btn-primary btn-sm px-3" :disabled="loading" @click="applyFilters">
                    <i class="bi bi-search me-1"></i>{{ t('filter') }}
                  </button>
                  <button type="button" class="btn btn-outline-secondary btn-sm" :title="t('reset')" @click="resetFilters">
                    <i class="bi bi-arrow-counterclockwise me-1"></i>{{ t('reset') }}
                  </button>
                </div>
              </div>
            </div>
          </LiquidGlassPopover>
          <button type="button" class="btn btn-primary btn-sm text-nowrap" :disabled="loading" @click="applyFilters">
            <i class="bi bi-search me-1" aria-hidden="true"></i>{{ t('search') }}
          </button>
        </div>
      </div>

        <div class="card-body p-0 d-flex flex-column flex-grow-1 min-h-0 overflow-hidden">
          <div class="table-responsive flex-grow-1 overflow-auto">
            <table class="table table-hover align-middle mb-0 alarm-table">
              <thead class="table-light sticky-top">
                <tr>
                  <th class="ps-4" style="min-width: 280px;">{{ t('alarm') }}</th>
                  <th style="min-width: 140px;">{{ t('state') }}</th>
                  <th style="min-width: 110px;">{{ t('handling') }}</th>
                  <th class="text-center" style="min-width: 80px;">{{ t('occurrence') }}</th>
                  <th style="min-width: 110px;">{{ t('owner') }}</th>
                  <th style="min-width: 150px;">{{ t('time') }}</th>
                  <th style="min-width: 120px;">{{ t('workOrder') }}</th>
                  <th class="text-end pe-4" style="min-width: 160px;">{{ t('actions') }}</th>
                </tr>
              </thead>
              <tbody>
                <tr v-if="loading && !items.length">
                  <td colspan="8" class="py-5 text-center text-body-secondary">
                    <span class="spinner-border spinner-border-sm me-2"></span>{{ t('loading') }}
                  </td>
                </tr>
                <tr v-else-if="!items.length">
                  <td colspan="8" class="py-5 text-center text-body-secondary">
                    <i class="bi bi-shield-check d-block fs-2 mb-2 text-success"></i>
                    <div>{{ t('empty') }}</div>
                    <button type="button" class="btn btn-link btn-sm mt-2" @click="openHistory">{{ t('viewHistory') }}</button>
                  </td>
                </tr>
                <tr
                  v-for="item in items"
                  :key="alarmOf(item).public_id"
                  class="alarm-row cursor-pointer"
                  tabindex="0"
                  @click="openDetail(item)"
                  @keydown.enter.prevent="openDetail(item)"
                  @keydown.space.prevent="openDetail(item)"
                >
                  <!-- 告警与来源（两行排版） -->
                  <td class="ps-4">
                    <div class="d-flex align-items-start gap-2">
                      <span class="severity-dot mt-1 flex-shrink-0" :class="`severity-dot--${severityOf(item)}`"></span>
                      <div class="d-flex flex-column min-w-0">
                        <span class="fw-semibold text-body text-truncate mb-1" :title="titleOf(item)">
                          {{ titleOf(item) }}
                        </span>
                        <div class="d-flex flex-wrap align-items-center gap-1 small text-body-secondary">
                          <span class="badge text-bg-light border text-body-secondary fw-normal">
                            <i class="bi bi-hdd-network me-1"></i>{{ alarmDeviceName(item) }}
                          </span>
                          <span v-if="alarmEventName(item)" class="badge text-bg-light border text-body-secondary fw-normal">
                            <i class="bi bi-lightning-charge me-1"></i>{{ alarmEventName(item) }}
                          </span>
                        </div>
                      </div>
                    </div>
                  </td>
                  <!-- 状态与级别 -->
                  <td>
                    <div class="d-flex flex-column gap-1 align-items-start">
                      <span class="badge rounded-pill" :class="conditionClass(alarmOf(item).condition_status)">
                        {{ conditionText(alarmOf(item).condition_status) }}
                      </span>
                      <span class="badge rounded-pill" :class="`text-bg-${severityTone(severityOf(item))}`">
                        {{ severityText(severityOf(item)) }}
                      </span>
                    </div>
                  </td>
                  <!-- 处置状态 -->
                  <td>
                    <span class="badge rounded-pill badge-handling">
                      {{ handlingText(alarmOf(item).handling_status) }}
                    </span>
                  </td>
                  <!-- 触发频次 -->
                  <td class="text-center">
                    <span class="badge rounded-pill" :class="alarmOf(item).occurrence_count > 1 ? 'text-bg-warning' : 'text-bg-light border text-body'">
                      {{ alarmOf(item).occurrence_count || 1 }}
                    </span>
                    <span v-if="alarmOf(item).notification_muted" class="text-warning ms-1" :title="t('shelved')">
                      <i class="bi bi-bell-slash"></i>
                    </span>
                  </td>
                  <!-- 责任人 -->
                  <td>
                    <span v-if="alarmOf(item).owner_user_id" class="badge text-bg-info-subtle text-info-emphasis border border-info-subtle">
                      <i class="bi bi-person me-1"></i>{{ currentHandlerText(item) }}
                    </span>
                    <span v-else class="text-body-secondary small">
                      <i class="bi bi-person-x me-1"></i>{{ t('unclaimed') }}
                    </span>
                  </td>
                  <!-- 发生时间 -->
                  <td>
                    <div class="small text-body">
                      {{ formatTime(alarmOf(item).last_occurred_at || alarmOf(item).created_at) }}
                    </div>
                  </td>
                  <!-- 关联工单 -->
                  <td>
                    <button
                      v-if="alarmOf(item).work_order_public_id"
                      type="button"
                      class="btn btn-sm btn-outline-primary py-0 px-2 rounded-pill"
                      @click.stop="goWorkOrder(alarmOf(item).work_order_public_id)"
                    >
                      <i class="bi bi-ticket-detailed me-1"></i>{{ t('viewWorkOrder') }}
                    </button>
                    <span v-else class="text-body-secondary small">-</span>
                  </td>
                  <!-- 操作 -->
                  <td class="text-end pe-4" @click.stop>
                    <div class="table-actions">
                      <button
                        v-if="canAcknowledge(item)"
                        type="button"
                        class="table-action-btn table-action-btn--text table-action-btn--primary"
                        @click="acknowledge(item)"
                      >
                        <i class="bi bi-check2"></i> {{ t('acknowledge') }}
                      </button>
                      <button
                        v-if="canCreateWorkOrder(item)"
                        type="button"
                        class="table-action-btn table-action-btn--text table-action-btn--info"
                        @click="openWorkOrder(item)"
                      >
                        <i class="bi bi-ticket-detailed"></i> {{ t('createWorkOrder') }}
                      </button>
                      <button
                        type="button"
                        class="table-action-btn table-action-btn--text"
                        @click="openDetail(item)"
                      >
                        <i class="bi bi-eye"></i> {{ t('details') }}
                      </button>
                    </div>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
          <ListPagination
            :page="page"
            :page-size="pageSize"
            :total="total"
            :page-size-options="pageSizeOptions"
            :disabled="loading"
            id-prefix="alarm-queue"
            @update:page="goQueuePage"
            @update:page-size="changeQueuePageSize"
          />
        </div>
      </div>
    </template>

    <!-- Tab Content: History Events -->
    <template v-else>
      <div class="card border-0 shadow-sm table-glass-card flex-grow-1 min-h-0 overflow-hidden" role="tabpanel" aria-labelledby="alarm-history-tab">
        <div class="card-header bg-transparent d-flex flex-wrap justify-content-between align-items-center gap-2 py-2 px-3 border-bottom">
          <div class="d-flex align-items-center gap-2">
            <strong class="h6 mb-0">{{ t('history') }}</strong>
            <span class="alarm-history-help">
              <button type="button" class="alarm-history-help__trigger" :aria-label="t('historyNotice')">
                <i class="bi bi-info-circle"></i>
              </button>
              <span class="alarm-history-help__popover" role="tooltip">{{ t('historyNotice') }}</span>
            </span>
          </div>
          <div style="max-width: 280px;">
            <div class="input-group input-group-sm">
              <span class="input-group-text bg-transparent"><i class="bi bi-search"></i></span>
              <input
                id="legacy-search"
                v-model.trim="legacySearch"
                class="form-control"
                :placeholder="t('historySearchPlaceholder')"
              >
            </div>
          </div>
        </div>
        <div class="card-body p-0 d-flex flex-column h-100 overflow-hidden">
          <div class="table-responsive flex-grow-1 overflow-auto">
            <table class="table table-hover align-middle mb-0 alarm-table">
              <thead class="table-light sticky-top">
                <tr>
                  <th class="ps-4" style="min-width: 240px;">{{ t('event') }}</th>
                  <th style="min-width: 180px;">{{ t('device') }}</th>
                  <th style="min-width: 160px;">{{ t('time') }}</th>
                  <th class="text-end pe-4" style="min-width: 100px;">{{ t('actions') }}</th>
                </tr>
              </thead>
              <tbody>
                <tr v-if="historyLoading">
                  <td colspan="4" class="py-5 text-center text-body-secondary">
                    <span class="spinner-border spinner-border-sm me-2"></span>{{ t('loading') }}
                  </td>
                </tr>
                <tr v-else-if="!filteredLegacyEvents.length">
                  <td colspan="4" class="py-5 text-center text-body-secondary">
                    <i class="bi bi-inbox d-block fs-2 mb-2 text-secondary"></i>
                    <div>{{ t('historyEmpty') }}</div>
                  </td>
                </tr>
                <tr
                  v-for="(event, index) in filteredLegacyEvents"
                  :key="legacyEventKey(event, index)"
                  class="alarm-row cursor-pointer"
                  tabindex="0"
                  @click="openLegacyDetail(event)"
                  @keydown.enter.prevent="openLegacyDetail(event)"
                  @keydown.space.prevent="openLegacyDetail(event)"
                >
                  <td class="ps-4">
                    <div class="fw-semibold text-body">{{ legacyEventTitle(event) }}</div>
                  </td>
                  <td>{{ legacyDeviceName(event) }}</td>
                  <td>{{ formatTime(event.ts || event.timestamp || event.created_at) }}</td>
                  <td class="text-end pe-4" @click.stop>
                    <div class="table-actions">
                      <button type="button" class="table-action-btn table-action-btn--text" @click.stop="openLegacyDetail(event)">
                        <i class="bi bi-eye"></i> {{ t('details') }}
                      </button>
                    </div>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
          <ListPagination
            :page="legacyPage"
            :page-size="legacyPageSize"
            :total="legacyTotal"
            :page-size-options="pageSizeOptions"
            :disabled="historyLoading"
            id-prefix="alarm-history"
            @update:page="goLegacyPage"
            @update:page-size="changeLegacyPageSize"
          />
        </div>
      </div>
    </template>

    <!-- 抽屉与全局弹窗 (Teleport to body 避免被侧边栏与顶部栏遮挡) -->
    <Teleport to="body">
      <!-- 告警详情抽屉 -->
      <div v-if="selected" class="offcanvas-backdrop fade show" @click="closeDetail"></div>
      <aside
        v-if="selected"
        ref="detailDialog"
        class="alarm-drawer shadow-lg"
        aria-modal="true"
        role="dialog"
        tabindex="-1"
        aria-labelledby="alarm-detail-title"
      >
        <div class="alarm-drawer__head">
          <div class="d-flex flex-column gap-1 min-w-0 pe-2">
            <div class="d-flex align-items-center gap-2">
              <span class="badge rounded-pill" :class="conditionClass(selectedAlarm.condition_status)">
                {{ conditionText(selectedAlarm.condition_status) }}
              </span>
              <span class="badge rounded-pill" :class="`text-bg-${severityTone(severityOf(selected))}`">
                {{ severityText(severityOf(selected)) }}
              </span>
            </div>
            <h2 id="alarm-detail-title" class="h5 mb-0 text-truncate" :title="titleOf(selected)">
              {{ titleOf(selected) }}
            </h2>
            <p class="text-body-secondary small mb-0 text-truncate">
              {{ alarmContextText(selected) }}
            </p>
          </div>
          <button type="button" class="btn-close flex-shrink-0" :aria-label="t('close')" @click="closeDetail"></button>
        </div>

        <div class="alarm-drawer__body">
          <div v-if="detailLoading" class="text-center py-5">
            <span class="spinner-border spinner-border-sm me-2"></span>{{ t('loading') }}
          </div>
          <template v-else>
            <!-- 核心指标摘要卡片 -->
            <div class="card border mb-3 alarm-detail-summary">
              <div class="card-body p-3">
                <div class="row g-2 small">
                  <div class="col-6 d-flex justify-content-between">
                    <span class="text-secondary">{{ t('severity') }}:</span>
                    <strong :class="`text-${severityTone(severityOf(selected))}`">{{ severityText(severityOf(selected)) }}</strong>
                  </div>
                  <div class="col-6 d-flex justify-content-between">
                    <span class="text-secondary">{{ t('condition') }}:</span>
                    <strong>{{ conditionText(selectedAlarm.condition_status) }}</strong>
                  </div>
                  <div class="col-6 d-flex justify-content-between">
                    <span class="text-secondary">{{ t('handling') }}:</span>
                    <span class="badge rounded-pill badge-handling">{{ handlingText(selectedAlarm.handling_status) }}</span>
                  </div>
                  <div class="col-6 d-flex justify-content-between">
                    <span class="text-secondary">{{ t('occurrence') }}:</span>
                    <strong>{{ selectedAlarm.occurrence_count || 1 }}</strong>
                  </div>
                  <div class="col-12 d-flex justify-content-between border-top pt-2 mt-1">
                    <span class="text-secondary">{{ t('owner') }}:</span>
                    <strong>{{ currentHandlerText(selected) }}</strong>
                  </div>
                </div>
              </div>
            </div>

            <!-- 处置动作操作栏 -->
            <div class="d-grid gap-2 mb-4">
              <button v-if="canAcknowledge(selected)" type="button" class="btn btn-primary" @click="acknowledge(selected)">
                <i class="bi bi-check2-circle me-1"></i>{{ t('acknowledge') }}
              </button>
              <button v-if="canCreateWorkOrder(selected)" type="button" class="btn btn-outline-primary" @click="openWorkOrder(selected)">
                <i class="bi bi-ticket-detailed me-1"></i>{{ t('createWorkOrder') }}
              </button>
              <button v-else-if="selectedAlarm.work_order_public_id" type="button" class="btn btn-outline-primary" @click="goWorkOrder(selectedAlarm.work_order_public_id)">
                <i class="bi bi-box-arrow-up-right me-1"></i>{{ t('viewWorkOrder') }}
              </button>
              <div class="d-flex gap-2">
                <button v-if="canAssignAlarm(selected)" type="button" class="btn btn-outline-secondary flex-grow-1" @click="openOperation('assign')">
                  <i class="bi bi-person-plus me-1"></i>{{ t('assign') }}
                </button>
                <details v-if="canOperate(selected)" class="alarm-more-actions flex-grow-1">
                  <summary class="btn btn-outline-secondary w-100">
                    <i class="bi bi-three-dots me-1"></i>{{ t('moreActions') }}
                  </summary>
                  <div class="alarm-more-actions__menu border rounded p-2 mt-2 d-grid gap-2 shadow-sm">
                    <p class="small text-body-secondary mb-0">{{ t('pauseNotificationsHelp') }}</p>
                    <button v-if="selectedAlarm.handling_status !== 'shelved'" type="button" class="btn btn-sm btn-outline-secondary" @click="openOperation('shelve')">
                      <i class="bi bi-bell-slash me-1"></i>{{ t('shelve') }}
                    </button>
                    <button v-else type="button" class="btn btn-sm btn-outline-secondary" @click="unshelve">
                      <i class="bi bi-bell me-1"></i>{{ t('unshelve') }}
                    </button>
                    <button v-if="canClose(selected)" type="button" class="btn btn-sm btn-outline-danger" @click="openOperation('close')">
                      <i class="bi bi-check2-all me-1"></i>{{ t('closeAlarm') }}
                    </button>
                  </div>
                </details>
              </div>
            </div>

            <!-- 告警与消警现场证据卡片 -->
            <section class="card border mb-4">
              <div class="card-header bg-transparent py-2 d-flex flex-wrap justify-content-between align-items-center gap-2">
                <h3 class="section-title mb-0"><i class="bi bi-shield-check me-1"></i>{{ t('evidence') }}</h3>
                <div v-if="hasClearedEvidence(selected)" class="btn-group btn-group-sm" role="group" :aria-label="t('evidence')">
                  <button
                    type="button"
                    class="btn"
                    :class="activeEvidenceTab === 'alarm' ? 'btn-primary' : 'btn-outline-secondary'"
                    @click="activeEvidenceTab = 'alarm'"
                  >
                    <i class="bi bi-exclamation-triangle-fill text-danger me-1"></i>{{ t('alarmEvidence') }}
                  </button>
                  <button
                    type="button"
                    class="btn"
                    :class="activeEvidenceTab === 'cleared' ? 'btn-success text-white' : 'btn-outline-secondary'"
                    @click="activeEvidenceTab = 'cleared'"
                  >
                    <i class="bi bi-check-circle-fill me-1" :class="activeEvidenceTab === 'cleared' ? 'text-white' : 'text-success'"></i>{{ t('clearedEvidence') }}
                  </button>
                  <button
                    v-if="snapshotUrl(selected) && clearedSnapshotUrl(selected)"
                    type="button"
                    class="btn"
                    :class="activeEvidenceTab === 'comparison' ? 'btn-primary' : 'btn-outline-secondary'"
                    @click="activeEvidenceTab = 'comparison'"
                  >
                    <i class="bi bi-layout-split me-1"></i>{{ t('evidenceComparison') }}
                  </button>
                </div>
              </div>
              <div class="card-body p-3">
                <!-- 1. 告警证据视图 (Alarm Evidence) -->
                <div v-if="activeEvidenceTab === 'alarm' || !hasClearedEvidence(selected)">
                  <div v-if="snapshotUrl(selected)" class="mb-3 text-center">
                    <a :href="snapshotUrl(selected)" target="_blank" rel="noopener" class="d-inline-block position-relative">
                      <img :src="snapshotUrl(selected)" class="img-fluid rounded border shadow-sm border-danger" style="max-height: 180px; object-fit: contain;" :alt="t('snapshot')">
                      <span class="badge bg-danger position-absolute top-0 start-0 m-1 shadow-sm">{{ t('firingState') }}</span>
                    </a>
                  </div>
                  <div v-else class="text-body-secondary small text-center py-3 mb-3 border rounded bg-body-tertiary">
                    {{ t('noAlarmEvidence') }}
                  </div>
                  <div class="row g-2 small">
                    <div class="col-12 col-sm-6">
                      <div class="text-secondary fw-normal">{{ t('device') }}</div>
                      <div class="fw-semibold text-body">{{ alarmDeviceName(selected) }}</div>
                    </div>
                    <div class="col-12 col-sm-6">
                      <div class="text-secondary fw-normal">{{ t('product') }}</div>
                      <div class="fw-semibold text-body">{{ alarmProductName(selected) }}</div>
                    </div>
                    <div class="col-12 col-sm-6">
                      <div class="text-secondary fw-normal">{{ t('alarmTarget') }}</div>
                      <div class="fw-semibold text-danger">{{ alarmTargetName(selected) }}</div>
                    </div>
                    <div class="col-12 col-sm-6">
                      <div class="text-secondary fw-normal">{{ t('event') }}</div>
                      <div class="fw-semibold text-body">{{ alarmEventName(selected) }}</div>
                    </div>
                    <div class="col-12 col-sm-6">
                      <div class="text-secondary fw-normal">{{ t('firstSeen') }}</div>
                      <div class="text-body">{{ formatTime(selectedAlarm.first_occurred_at) }}</div>
                    </div>
                    <div class="col-12 col-sm-6">
                      <div class="text-secondary fw-normal">{{ t('source') }}</div>
                      <div class="fw-semibold text-body">{{ sourceTypeText(selectedAlarm.source_type) }}</div>
                    </div>
                  </div>
                </div>

                <!-- 2. 消警证据视图 (Clearance Evidence) -->
                <div v-else-if="activeEvidenceTab === 'cleared'">
                  <div v-if="clearedSnapshotUrl(selected)" class="mb-3 text-center">
                    <a :href="clearedSnapshotUrl(selected)" target="_blank" rel="noopener" class="d-inline-block position-relative">
                      <img :src="clearedSnapshotUrl(selected)" class="img-fluid rounded border shadow-sm border-success" style="max-height: 180px; object-fit: contain;" :alt="t('clearedSnapshot')">
                      <span class="badge bg-success position-absolute top-0 start-0 m-1 shadow-sm">{{ t('clearedState') }}</span>
                    </a>
                  </div>
                  <div v-else class="text-body-secondary small text-center py-3 mb-3 border rounded bg-body-tertiary">
                    {{ t('noClearedEvidence') }}
                  </div>
                  <div class="row g-2 small">
                    <div class="col-12 col-sm-6">
                      <div class="text-secondary fw-normal">{{ t('device') }}</div>
                      <div class="fw-semibold text-body">{{ clearedAlarmDeviceName(selected) }}</div>
                    </div>
                    <div class="col-12 col-sm-6">
                      <div class="text-secondary fw-normal">{{ t('product') }}</div>
                      <div class="fw-semibold text-body">{{ clearedAlarmProductName(selected) }}</div>
                    </div>
                    <div class="col-12 col-sm-6">
                      <div class="text-secondary fw-normal">{{ t('clearedTarget') }}</div>
                      <div class="fw-semibold text-success">{{ clearedAlarmTargetName(selected) }}</div>
                    </div>
                    <div class="col-12 col-sm-6">
                      <div class="text-secondary fw-normal">{{ t('event') }}</div>
                      <div class="fw-semibold text-body">{{ clearedAlarmEventName(selected) }}</div>
                    </div>
                    <div class="col-12 col-sm-6">
                      <div class="text-secondary fw-normal">{{ t('clearedAt') }}</div>
                      <div class="text-body fw-semibold text-success">{{ formatTime(selectedAlarm.recovered_at || clearedEvidenceOf(selected).cleared_at) }}</div>
                    </div>
                    <div class="col-12 col-sm-6">
                      <div class="text-secondary fw-normal">{{ t('firstSeen') }}</div>
                      <div class="text-body">{{ formatTime(selectedAlarm.first_occurred_at) }}</div>
                    </div>
                  </div>
                </div>

                <!-- 3. 双图对比视图 (Comparison) -->
                <div v-else-if="activeEvidenceTab === 'comparison'">
                  <div class="row g-2 mb-3">
                    <div class="col-6 text-center">
                      <div class="small fw-semibold text-danger mb-1"><i class="bi bi-exclamation-triangle-fill me-1"></i>{{ t('alarmEvidence') }}</div>
                      <a :href="snapshotUrl(selected)" target="_blank" rel="noopener" class="d-inline-block">
                        <img :src="snapshotUrl(selected)" class="img-fluid rounded border border-danger shadow-sm" style="max-height: 140px; object-fit: contain;" :alt="t('snapshot')">
                      </a>
                      <div class="small text-body-secondary mt-1 text-truncate">{{ formatTime(selectedAlarm.first_occurred_at) }}</div>
                    </div>
                    <div class="col-6 text-center">
                      <div class="small fw-semibold text-success mb-1"><i class="bi bi-check-circle-fill me-1"></i>{{ t('clearedEvidence') }}</div>
                      <a :href="clearedSnapshotUrl(selected)" target="_blank" rel="noopener" class="d-inline-block">
                        <img :src="clearedSnapshotUrl(selected)" class="img-fluid rounded border border-success shadow-sm" style="max-height: 140px; object-fit: contain;" :alt="t('clearedSnapshot')">
                      </a>
                      <div class="small text-success mt-1 text-truncate">{{ formatTime(selectedAlarm.recovered_at || clearedEvidenceOf(selected).cleared_at) }}</div>
                    </div>
                  </div>
                  <div class="row g-2 small border-top pt-2">
                    <div class="col-12 col-sm-6">
                      <div class="text-secondary fw-normal">{{ t('alarmTarget') }}</div>
                      <div class="fw-semibold text-danger">{{ alarmTargetName(selected) }}</div>
                    </div>
                    <div class="col-12 col-sm-6">
                      <div class="text-secondary fw-normal">{{ t('clearedTarget') }}</div>
                      <div class="fw-semibold text-success">{{ clearedAlarmTargetName(selected) }}</div>
                    </div>
                  </div>
                </div>
              </div>
            </section>

            <!-- 处置流转时间线 -->
            <section class="card border">
              <div class="card-header bg-transparent py-2">
                <h3 class="section-title mb-0"><i class="bi bi-clock-history me-1"></i>{{ t('timeline') }}</h3>
              </div>
              <div class="card-body p-3">
                <ol class="timeline mb-0">
                  <li v-for="event in selectedEvents" :key="event.event.id || event.event.ID">
                    <strong class="text-body">{{ eventText(event.event.type) }}</strong>
                    <time :datetime="event.created_at || event.event.created_at || event.event.CreatedAt || undefined">
                      {{ formatTime(event.created_at || event.event.created_at || event.event.CreatedAt) }}
                    </time>
                    <p v-if="timelineDetail(event)" class="mb-0 text-body-secondary small mt-1">
                      {{ timelineDetail(event) }}
                    </p>
                    <div v-if="timelineSnapshotUrl(event)" class="mt-2">
                      <a :href="timelineSnapshotUrl(event)" target="_blank" rel="noopener" class="d-inline-block">
                        <img :src="timelineSnapshotUrl(event)" class="rounded border shadow-sm" style="max-height: 60px; object-fit: contain;" :alt="t('snapshot')">
                      </a>
                    </div>
                  </li>
                  <li v-if="!selectedEvents.length" class="text-body-secondary small">
                    {{ t('noTimeline') }}
                  </li>
                </ol>
              </div>
            </section>
          </template>
        </div>
      </aside>

      <!-- 处置操作弹窗 -->
      <div v-if="operation" class="modal fade show d-block modal-backdrop-layer" @click.self="closeOperation"><div ref="operationDialog" class="modal-dialog modal-dialog-centered" role="dialog" aria-modal="true" :aria-labelledby="'operation-title'"><div class="modal-content"><div class="modal-header"><h2 id="operation-title" class="modal-title h5">{{ operationTitle }}</h2><button type="button" class="btn-close" :aria-label="t('close')" @click="closeOperation"></button></div><div class="modal-body"><div v-if="operationError" class="alert alert-danger py-2">{{ operationError }}</div><template v-if="operation === 'assign'"><label class="form-label" for="alarm-assignee">{{ t('assignee') }}</label><select id="alarm-assignee" v-model.number="operationForm.ownerUserID" class="form-select"><option :value="0">{{ t('selectAssignee') }}</option><option v-for="person in participants" :key="person.id" :value="person.id">{{ participantName(person) }}</option></select></template><template v-else-if="operation === 'shelve'"><label class="form-label" for="alarm-shelve-reason">{{ t('reason') }}</label><textarea id="alarm-shelve-reason" v-model.trim="operationForm.reason" class="form-control" rows="3" :placeholder="t('shelveReasonHint')"></textarea><label class="form-label mt-3" for="alarm-shelve-until">{{ t('until') }}</label><input id="alarm-shelve-until" v-model="operationForm.until" type="datetime-local" class="form-control"><div class="form-text">{{ t('untilHint') }}</div></template><template v-else><label class="form-label" for="alarm-disposition">{{ t('disposition') }}</label><select id="alarm-disposition" v-model="operationForm.disposition" class="form-select"><option v-if="selectedAlarm.condition_status === 'recovered'" value="resolved">{{ t('resolved') }}</option><option v-if="canExceptionClose(selected)" value="false_positive">{{ t('falsePositive') }}</option><option v-if="canExceptionClose(selected)" value="duplicate">{{ t('duplicate') }}</option><option v-if="canExceptionClose(selected)" value="maintenance">{{ t('maintenance') }}</option><option v-if="canExceptionClose(selected)" value="no_action_required">{{ t('noAction') }}</option></select><label class="form-label mt-3" for="alarm-close-comment">{{ t('comment') }}</label><textarea id="alarm-close-comment" v-model.trim="operationForm.comment" class="form-control" rows="3"></textarea><div class="form-text">{{ t('closeGuardHint') }}</div></template></div><div class="modal-footer"><button type="button" class="btn btn-outline-secondary" @click="closeOperation">{{ t('cancel') }}</button><button type="button" class="btn btn-primary" :disabled="operationSubmitting" @click="submitOperation"><span v-if="operationSubmitting" class="spinner-border spinner-border-sm me-1"></span>{{ t('confirm') }}</button></div></div></div></div>

      <!-- 创建联动工单弹窗 -->
      <div v-if="showWorkOrder" class="modal fade show d-block modal-backdrop-layer" @click.self="closeWorkOrder">
        <div ref="workOrderDialog" class="modal-dialog modal-lg modal-dialog-scrollable" role="dialog" aria-modal="true" :aria-labelledby="'work-order-title'">
          <div class="modal-content">
            <div class="modal-header">
              <div>
                <h2 id="work-order-title" class="modal-title h5 mb-1">{{ t('createWorkOrder') }}</h2>
                <p class="small text-body-secondary mb-0">{{ t('evidenceAttached') }}</p>
              </div>
              <button type="button" class="btn-close" :aria-label="t('close')" @click="closeWorkOrder"></button>
            </div>
            <div class="modal-body">
              <div v-if="workOrderError" class="alert alert-danger py-2 mb-3">{{ workOrderError }}</div>
              <div v-if="workOrderSuccess" class="alert alert-success py-2 mb-3">{{ workOrderSuccess }}</div>

              <!-- 告警关联证据摘要卡片 -->
              <div v-if="workOrder.alarm" class="card alarm-muted-surface border mb-3">
                <div class="card-body p-3">
                  <div class="d-flex flex-wrap justify-content-between align-items-start gap-2 mb-2">
                    <div class="d-flex align-items-center gap-2">
                      <span class="severity-dot" :class="`severity-dot--${severityOf(workOrder.alarm)}`"></span>
                      <strong class="h6 mb-0">{{ titleOf(workOrder.alarm) }}</strong>
                    </div>
                    <div class="d-flex gap-1">
                      <span class="badge rounded-pill" :class="`text-bg-${severityTone(severityOf(workOrder.alarm))}`">{{ severityText(severityOf(workOrder.alarm)) }}</span>
                      <span class="badge rounded-pill" :class="conditionClass(alarmOf(workOrder.alarm).condition_status)">{{ conditionText(alarmOf(workOrder.alarm).condition_status) }}</span>
                    </div>
                  </div>
                  <div class="row g-2 small text-body-secondary">
                    <div class="col-12 col-md-6 d-flex gap-2">
                      <span class="text-secondary fw-normal">{{ t('device') }}:</span>
                      <strong class="text-body">{{ alarmDeviceName(workOrder.alarm) }}</strong>
                    </div>
                    <div class="col-12 col-md-6 d-flex gap-2">
                      <span class="text-secondary fw-normal">{{ t('event') }}:</span>
                      <strong class="text-body">{{ alarmEventName(workOrder.alarm) }}</strong>
                    </div>
                    <div class="col-12 col-md-6 d-flex gap-2">
                      <span class="text-secondary fw-normal">{{ t('firstSeen') }}:</span>
                      <span>{{ formatTime(alarmOf(workOrder.alarm).first_occurred_at) }}</span>
                    </div>
                    <div class="col-12 col-md-6 d-flex gap-2">
                      <span class="text-secondary fw-normal">{{ t('occurrence') }}:</span>
                      <span>{{ alarmOf(workOrder.alarm).occurrence_count || 1 }}</span>
                    </div>
                  </div>
                  <div v-if="snapshotUrl(workOrder.alarm)" class="mt-2 pt-2 border-top">
                    <a :href="snapshotUrl(workOrder.alarm)" target="_blank" rel="noopener" class="d-inline-block">
                      <img :src="snapshotUrl(workOrder.alarm)" class="rounded border" style="max-height: 100px; object-fit: contain;" :alt="t('snapshot')">
                    </a>
                  </div>
                </div>
              </div>

              <!-- 工单基础信息表单 -->
              <div class="row g-3 mb-3">
                <div class="col-md-6">
                  <label class="form-label small" for="alarm-work-order-template">{{ t('template') }} <span class="text-danger">*</span></label>
                  <select id="alarm-work-order-template" v-model.number="workOrder.templateId" class="form-select" @change="loadWorkOrderTemplate">
                    <option :value="0">{{ t('selectTemplate') }}</option>
                    <option v-for="template in workOrderTemplates" :key="template.ID" :value="template.ID">{{ template.name }}</option>
                  </select>
                </div>
                <div class="col-md-6">
                  <label class="form-label small" for="alarm-work-order-priority">{{ t('priority') }} <span class="text-danger">*</span></label>
                  <select id="alarm-work-order-priority" v-model="workOrder.priority" class="form-select">
                    <option value="low">{{ t('low') }}</option>
                    <option value="normal">{{ t('normal') }}</option>
                    <option value="high">{{ t('high') }}</option>
                    <option value="urgent">{{ t('urgent') }}</option>
                  </select>
                </div>
                <div class="col-12">
                  <label class="form-label small" for="alarm-work-order-summary">{{ t('workOrderTitle') }} <span class="text-danger">*</span></label>
                  <input id="alarm-work-order-summary" v-model.trim="workOrder.title" class="form-control" :placeholder="t('workOrderTitle')">
                </div>
                <div class="col-12">
                  <label class="form-label small" for="alarm-work-order-description">{{ t('summary') }}</label>
                  <textarea id="alarm-work-order-description" v-model.trim="workOrder.summary" class="form-control" rows="2" :placeholder="t('summaryHint')"></textarea>
                </div>
              </div>

              <!-- 动态工单表单 -->
              <div v-if="workOrderTemplate" class="card alarm-muted-surface border pt-2 px-3 pb-3">
                <div class="d-flex justify-content-between align-items-center mb-2">
                  <h3 class="h6 mb-0 text-secondary"><i class="bi bi-ui-checks-grid me-1"></i>{{ t('form') }}</h3>
                  <span class="small text-body-secondary">{{ t('formHint') }}</span>
                </div>
                <WorkOrderFormFields v-model="workOrder.formData" :definition="workOrderTemplate.form_definition" id-prefix="alarm-work-order" :locale="locale" />
              </div>
            </div>
            <div class="modal-footer">
              <button type="button" class="btn btn-outline-secondary" @click="closeWorkOrder">{{ t('cancel') }}</button>
              <button type="button" class="btn btn-primary" :disabled="workOrderSubmitting" @click="createWorkOrder">
                <span v-if="workOrderSubmitting" class="spinner-border spinner-border-sm me-1"></span>{{ t('create') }}
              </button>
            </div>
          </div>
        </div>
      </div>

      <!-- 告警策略配置弹窗 -->
      <div v-if="showPolicies" class="modal fade show d-block modal-backdrop-layer">
        <div ref="policyDialog" class="modal-dialog modal-lg modal-dialog-scrollable" role="dialog" aria-modal="true" :aria-labelledby="'policy-title'">
          <div class="modal-content">
            <div class="modal-header"><h2 id="policy-title" class="modal-title h5">{{ t('policies') }}</h2><button type="button" class="btn-close" :aria-label="t('close')" @click="closePolicies"></button></div>
            <div class="modal-body" @mouseover="handlePolicyHelpEnter" @mouseout="handlePolicyHelpLeave" @focusin="handlePolicyHelpEnter" @focusout="handlePolicyHelpLeave" @scroll.passive="hidePolicyHelp">
              <div v-if="policyError" class="alert alert-danger py-2">{{ policyError }}</div>
              <div class="d-flex flex-wrap justify-content-between gap-2 mb-3"><p class="small text-body-secondary mb-0">{{ t('policyHint') }}</p><button type="button" class="btn btn-sm btn-outline-primary" @click="editPolicy({ enabled: true, auto_close: true, severity: 'warning' })"><i class="bi bi-plus-lg me-1"></i>{{ t('newPolicy') }}</button></div>
              <div v-if="editingPolicy" class="card alarm-muted-surface card-body mb-3">
                <h3 class="h6 mb-3">{{ t('policyBasicSettings') }}</h3>
                <div class="row g-2">
                  <div class="col-md-6"><label class="form-label small" for="policy-name">{{ t('policyName') }} <span class="text-danger">*</span><button type="button" class="policy-help" :aria-label="t('policyNameHelp')" :data-help="t('policyNameHelp')">?</button></label><input id="policy-name" v-model.trim="editingPolicy.name" class="form-control" maxlength="255" required></div>
                  <div class="col-md-6"><label class="form-label small" for="policy-severity">{{ t('handlingSeverity') }} <span class="text-danger">*</span><button type="button" class="policy-help" :aria-label="t('severityHelp')" :data-help="t('severityHelp')">?</button></label><select id="policy-severity" v-model="editingPolicy.severity" class="form-select"><option value="critical">{{ t('critical') }}</option><option value="major">{{ t('major') }}</option><option value="warning">{{ t('warning') }}</option><option value="info">{{ t('info') }}</option></select></div>
                  <div class="col-md-6"><label class="form-label small" for="policy-enabled">{{ t('policyStatus') }} <button type="button" class="policy-help" :aria-label="t('policyStatusHelp')" :data-help="t('policyStatusHelp')">?</button></label><select id="policy-enabled" v-model="editingPolicy.enabled" class="form-select"><option :value="true">{{ t('enabled') }}</option><option :value="false">{{ t('disabled') }}</option></select></div>
                  <div class="col-md-6"><label class="form-label small" for="policy-match-mode">{{ t('matchBasis') }} <span class="text-danger">*</span><button type="button" class="policy-help" :aria-label="t('matchBasisHelp')" :data-help="t('matchBasisHelp')">?</button></label><select id="policy-match-mode" v-model="editingPolicy.match_mode" class="form-select" @change="syncPolicyMatchMode"><option value="events">{{ t('matchSpecificEvents') }}</option><option value="levels">{{ t('matchEventLevels') }}</option><option value="all">{{ t('allDeviceEvents') }}</option></select></div>
                  <div class="col-12">
                    <div id="policy-event-label" class="form-label small mb-1">{{ t('applicableEvent') }} <span class="badge rounded-pill text-bg-secondary ms-2">{{ policyEventSelectionText(editingPolicy) }}</span></div>
                    <div v-if="editingPolicy.match_mode === 'events'" class="policy-event-picker" role="group" aria-labelledby="policy-event-label">
                      <div v-if="selectedPolicyEventOptions.length" class="policy-selected-events"><button v-for="option in selectedPolicyEventOptions" :key="option.value" type="button" class="badge rounded-pill text-bg-primary border-0" :aria-label="`${t('remove')}: ${option.label}`" @click="removePolicyEvent(option.value)">{{ option.label }} <i class="bi bi-x"></i></button></div>
                      <div class="policy-event-browser">
                        <label class="form-label small mb-1" for="policy-product-filter">{{ t('selectProduct') }}</label>
                        <select id="policy-product-filter" v-model="policyProductGroup" class="form-select form-select-sm"><option v-for="group in policyEventGroups" :key="group.key" :value="group.key">{{ group.label }} · {{ group.options.length }}</option></select>
                        <label class="visually-hidden" for="policy-event-search">{{ t('searchEvents') }}</label><input id="policy-event-search" v-model.trim="policyEventSearch" class="form-control form-control-sm mt-2" :placeholder="t('searchEvents')">
                      </div>
                      <div class="policy-event-options">
                        <label v-for="option in filteredPolicyEventOptions" :key="option.value" class="policy-event-option"><input v-model="editingPolicy.source_refs" class="form-check-input" type="checkbox" :value="option.value" @change="selectSpecificPolicyEvent"><span><strong>{{ option.label }}</strong><small v-if="option.meta" class="d-block text-body-secondary">{{ option.meta }}</small></span></label>
                        <div v-if="!filteredPolicyEventOptions.length" class="small text-body-secondary p-2">{{ t('noMatchingEvents') }}</div>
                      </div>
                    </div>
                    <div v-else-if="editingPolicy.match_mode === 'levels'" class="policy-level-picker border rounded p-3" role="group" aria-labelledby="policy-event-label">
                      <p class="small text-body-secondary mb-2">{{ t('eventLevelHint') }}</p>
                      <label v-for="level in policyEventLevels" :key="level.value" class="form-check form-check-inline mb-0"><input v-model="editingPolicy.event_levels" class="form-check-input" type="checkbox" :value="level.value"><span class="form-check-label">{{ level.label }}</span></label>
                    </div>
                    <div v-else class="alert alert-secondary py-2 mb-0">{{ t('allEventsPolicyHint') }}</div>
                  </div>
                </div>
                <h3 class="h6 mt-4 mb-3">{{ t('policyHandlingSettings') }}</h3>
                <div class="row g-2">
                  <div class="col-md-6"><label class="form-label small" for="policy-recovery-mode">{{ t('recoveryMode') }} <button type="button" class="policy-help" :aria-label="t('recoveryModeHelp')" :data-help="t('recoveryModeHelp')">?</button></label><select id="policy-recovery-mode" v-model="editingPolicy.auto_close" class="form-select"><option :value="true">{{ t('recoveryAutoClose') }}</option><option :value="false">{{ t('recoveryManualClose') }}</option></select></div>
                  <div class="col-md-6"><label class="form-label small" for="policy-recovery-hold">{{ t('recoveryHold') }} <button type="button" class="policy-help" :aria-label="t('recoveryHoldHelp')" :data-help="t('recoveryHoldHelp')">?</button></label><select id="policy-recovery-hold" v-model.number="editingPolicy.recovery_hold_seconds" class="form-select"><option v-for="option in recoveryHoldOptions" :key="option.value" :value="option.value">{{ option.label }}</option></select></div>
                  <div class="col-md-6"><label class="form-label small" for="policy-work-order-mode">{{ t('workOrderHandling') }} <button type="button" class="policy-help" :aria-label="t('workOrderHandlingHelp')" :data-help="t('workOrderHandlingHelp')">?</button></label><select id="policy-work-order-mode" v-model="editingPolicy.work_order_mode" class="form-select" @change="syncPolicyWorkOrderOptions"><option value="none">{{ t('workOrderNone') }}</option><option value="required">{{ t('workOrderRequired') }}</option><option value="auto">{{ t('workOrderAuto') }}</option></select></div>
                  <div v-if="editingPolicy.work_order_mode === 'auto'" class="col-md-6"><label class="form-label small" for="policy-work-order-template">{{ t('template') }} <span class="text-danger">*</span><button type="button" class="policy-help" :aria-label="t('workOrderTemplateHelp')" :data-help="t('workOrderTemplateHelp')">?</button></label><select id="policy-work-order-template" v-model.number="editingPolicy.work_order_template_id" class="form-select"><option :value="0">{{ t('selectTemplate') }}</option><option v-for="template in policyTemplates" :key="template.ID" :value="template.ID">{{ template.name }}</option></select></div>
                  <div class="col-md-6"><label class="form-label small" for="policy-exception-close">{{ t('exceptionClose') }} <button type="button" class="policy-help" :aria-label="t('exceptionCloseHelp')" :data-help="t('exceptionCloseHelp')">?</button></label><select id="policy-exception-close" v-model="editingPolicy.allow_exception_close" class="form-select"><option :value="false">{{ t('notAllowed') }}</option><option :value="true">{{ t('allowed') }}</option></select></div>
                  <div class="col-lg-6"><label class="form-label small" for="policy-ack-days">{{ t('ackSla') }} <button type="button" class="policy-help" :aria-label="t('ackSlaHelp')" :data-help="t('ackSlaHelp')">?</button></label><div class="policy-duration" role="group" :aria-label="t('ackSla')"><div class="input-group input-group-sm"><input id="policy-ack-days" v-model.number="editingPolicy.acknowledge_sla_parts.days" type="number" min="0" step="1" class="form-control" @input="markPolicyDurationChanged('acknowledge')"><span class="input-group-text">{{ t('days') }}</span></div><div class="input-group input-group-sm"><input v-model.number="editingPolicy.acknowledge_sla_parts.hours" type="number" min="0" max="23" step="1" class="form-control" :aria-label="t('hours')" @input="markPolicyDurationChanged('acknowledge')"><span class="input-group-text">{{ t('hours') }}</span></div><div class="input-group input-group-sm"><input v-model.number="editingPolicy.acknowledge_sla_parts.minutes" type="number" min="0" max="59" step="1" class="form-control" :aria-label="t('minutes')" @input="markPolicyDurationChanged('acknowledge')"><span class="input-group-text">{{ t('minutes') }}</span></div></div><div class="form-text">{{ t('zeroDurationNoLimit') }}</div><div v-if="editingPolicy.acknowledge_sla_remainder" class="form-text text-warning">{{ t('legacySecondRemainder') }}</div></div>
                  <div class="col-lg-6"><label class="form-label small" for="policy-resolution-days">{{ t('resolutionSla') }} <button type="button" class="policy-help" :aria-label="t('resolutionSlaHelp')" :data-help="t('resolutionSlaHelp')">?</button></label><div class="policy-duration" role="group" :aria-label="t('resolutionSla')"><div class="input-group input-group-sm"><input id="policy-resolution-days" v-model.number="editingPolicy.resolution_sla_parts.days" type="number" min="0" step="1" class="form-control" @input="markPolicyDurationChanged('resolution')"><span class="input-group-text">{{ t('days') }}</span></div><div class="input-group input-group-sm"><input v-model.number="editingPolicy.resolution_sla_parts.hours" type="number" min="0" max="23" step="1" class="form-control" :aria-label="t('hours')" @input="markPolicyDurationChanged('resolution')"><span class="input-group-text">{{ t('hours') }}</span></div><div class="input-group input-group-sm"><input v-model.number="editingPolicy.resolution_sla_parts.minutes" type="number" min="0" max="59" step="1" class="form-control" :aria-label="t('minutes')" @input="markPolicyDurationChanged('resolution')"><span class="input-group-text">{{ t('minutes') }}</span></div></div><div class="form-text">{{ t('zeroDurationNoLimit') }}</div><div v-if="editingPolicy.resolution_sla_remainder" class="form-text text-warning">{{ t('legacySecondRemainder') }}</div></div>
                </div>
                <div class="text-end mt-3"><button type="button" class="btn btn-sm btn-outline-secondary me-2" @click="editingPolicy = null">{{ t('cancel') }}</button><button type="button" class="btn btn-sm btn-primary" @click="savePolicy">{{ t('save') }}</button></div>
              </div>
              <div v-if="!policies.length && !editingPolicy" class="alarm-empty-state text-center border rounded p-4"><i class="bi bi-sliders fs-3 d-block mb-2"></i><h3 class="h6">{{ t('noPoliciesTitle') }}</h3><p class="small text-body-secondary mb-3">{{ t('noPoliciesHint') }}</p><button type="button" class="btn btn-outline-primary btn-sm" @click="editPolicy({ enabled: true, auto_close: true, severity: 'warning' })">{{ t('newPolicy') }}</button></div>
              <div v-if="policies.length && (!editingPolicy || editingPolicy.ID)" class="table-responsive border rounded">
                <table class="table table-hover align-middle mb-0 policy-table"><thead><tr><th>{{ t('policyName') }}</th><th>{{ t('applicableEvent') }}</th><th>{{ t('handlingSeverity') }}</th><th>{{ t('policyStatus') }}</th><th class="text-end">{{ t('actions') }}</th></tr></thead><tbody><tr v-for="policy in policies" :key="policy.ID"><td><strong>{{ policy.name }}</strong><div class="small text-body-secondary">{{ policyWorkOrderModeLabel(policy) }}</div></td><td>{{ policyEventLabel(policy) }}</td><td><span class="badge rounded-pill" :class="`text-bg-${severityTone(policy.severity)}`">{{ severityText(policy.severity) }}</span></td><td><span class="badge rounded-pill" :class="policy.enabled ? 'text-bg-success' : 'text-bg-secondary'">{{ policy.enabled ? t('enabled') : t('disabled') }}</span></td><td class="text-end"><div class="btn-group btn-group-sm"><button type="button" class="btn btn-outline-secondary" @click="editPolicy(policy)">{{ t('edit') }}</button><button type="button" class="btn btn-outline-danger" @click="deletePolicy(policy)">{{ t('delete') }}</button></div></td></tr></tbody></table>
              </div>
            </div>
          </div>
        </div>
      </div>

      <div v-if="policyHelp.visible" ref="policyHelpPopover" class="policy-help-popover" role="tooltip" :style="{ top: `${policyHelp.top}px`, left: `${policyHelp.left}px` }">{{ policyHelp.text }}</div>

      <!-- 历史事件详情抽屉 -->
      <div v-if="legacyDetail" class="offcanvas-backdrop fade show" @click="closeLegacyDetail"></div>
      <aside v-if="legacyDetail" ref="legacyDialog" class="alarm-drawer shadow-lg" aria-modal="true" role="dialog" tabindex="-1" aria-labelledby="history-detail-title">
        <div class="alarm-drawer__head"><div><h2 id="history-detail-title" class="h5 mb-1">{{ legacyEventTitle(legacyDetail) }}</h2><p class="small text-body-secondary mb-0">{{ t('historyReadOnly') }}</p></div><button type="button" class="btn-close" :aria-label="t('close')" @click="closeLegacyDetail"></button></div>
        <div class="alarm-drawer__body"><section v-if="legacySnapshotUrls(legacyDetail).length" class="legacy-snapshot-grid mb-3"><a v-for="(url, index) in legacySnapshotUrls(legacyDetail)" :key="url" :href="url" target="_blank" rel="noopener"><img :src="url" class="img-fluid rounded border" :alt="`${t('snapshot')} ${index + 1}`"></a></section><dl class="small"><div><dt>{{ t('device') }}</dt><dd>{{ legacyDeviceName(legacyDetail) }}</dd></div><div><dt>{{ t('product') }}</dt><dd>{{ legacyProductName(legacyDetail) }}</dd></div><div><dt>{{ t('time') }}</dt><dd>{{ formatTime(legacyDetail.ts || legacyDetail.timestamp || legacyDetail.created_at) }}</dd></div><div><dt>{{ t('event') }}</dt><dd>{{ legacyEventTitle(legacyDetail) }}</dd></div></dl><section v-if="legacyEventParameters(legacyDetail).length" class="mt-4"><h3 class="section-title">{{ t('eventParameters') }}</h3><dl class="small mb-0"><div v-for="parameter in legacyEventParameters(legacyDetail)" :key="parameter.id"><dt>{{ legacyParameterLabel(parameter) }}</dt><dd>{{ formatLegacyParameterValue(parameter.value) }}</dd></div></dl></section><h3 class="section-title mt-4">{{ t('originalData') }}</h3><pre class="alarm-json mb-0">{{ JSON.stringify(legacyDetail.params || legacyDetail, null, 2) }}</pre></div>
      </aside>
    </Teleport>
  </div>
</template>

<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import axios from 'axios'
import WorkOrderFormFields from '../components/work-order/WorkOrderFormFields.vue'
import ListPagination from '../components/ListPagination.vue'
import { ALARM_EVENT_IDS } from '../utils/alarmEvents.js'
import { loadAllWorkOrderParticipants } from '../utils/workOrderApi.js'
import { formatDateTime } from '../utils/dateTime.js'
import CompactListMetrics from '../components/CompactListMetrics.vue'
import LiquidGlassPopover from '../components/liquid-glass/LiquidGlassPopover.vue'

const { locale } = useI18n()
const router = useRouter()
const language = computed(() => String(locale.value || 'zh').toLowerCase().startsWith('en') ? 'en' : 'zh')
const copy = {
  zh: { eyebrow: '运行保障', title: '告警中心', subtitle: '将当前告警的状态处置与历史设备事件分开呈现，避免历史记录被误判为待处理故障。', policies: '告警策略', refresh: '刷新', close: '关闭', overview: '告警概览', viewSwitch: '告警视图', queue: '处置队列', history: '历史设备事件', search: '搜索', searchPlaceholder: '标题、来源或指纹', historySearchPlaceholder: '设备、事件类型或编号', severity: '级别', condition: '条件状态', handling: '处置状态', all: '全部', critical: '紧急', major: '严重', warning: '警告', info: '提示', firing: '告警中', recovered: '已恢复', filter: '查询', reset: '重置', appliedFilters: '当前筛选：', records: '条记录', includeClosed: '包含已关闭', alarm: '告警', state: '状态', occurrence: '次数', owner: '负责人', workOrder: '工单', actions: '操作', loading: '加载中…', empty: '当前没有符合筛选条件的可处置告警。', viewHistory: '查看已有历史事件', device: '设备', time: '时间', event: '事件', historyEmpty: '当前页没有匹配的历史事件。', historyNotice: '历史设备事件来自既有时序数据，只读展示，不表示当前仍有待处置告警；只有新上报并被告警策略物化的事件才能确认、关闭或联动工单。', historyReadOnly: '此记录为历史证据，只读展示，不可直接创建处置工单。', originalData: '原始事件数据', user: '用户', unassigned: '未分配', notLinked: '未关联', acknowledge: '确认', details: '详情', page: '第', previous: '上一页', next: '下一页', createWorkOrder: '创建联动工单', viewWorkOrder: '查看关联工单', shelve: '搁置', unshelve: '解除搁置', assign: '分派', closeAlarm: '关闭告警', alarmDetails: '告警详情', evidence: '告警证据', snapshot: '现场截图', source: '来源', firstSeen: '首次发生', recoveredAt: '恢复时间', timeline: '处置时间线', noTimeline: '暂无处置记录', assignee: '处理人', selectAssignee: '请选择处理人', reason: '搁置原因', shelveReasonHint: '例如：计划维护或已知重复告警', until: '搁置至', untilHint: '留空则需手动解除搁置。', disposition: '关闭结论', resolved: '已解决', falsePositive: '误报', duplicate: '重复告警', maintenance: '维护窗口', noAction: '无需处理', comment: '处置说明', closeGuardHint: '正常关闭需条件已恢复并通过校验；是否要求工单由服务端策略强制执行。', cancel: '取消', confirm: '确认', template: '工单模板', selectTemplate: '请选择模板', priority: '优先级', low: '低', normal: '普通', high: '高', urgent: '紧急', workOrderTitle: '工单标题', summary: '工单摘要', summaryHint: '补充现场观察、影响范围或处置目标。', form: '工单表单', formHint: '告警证据、时间线与原始上下文会自动绑定到工单，无需复制 JSON。', create: '创建工单', evidenceAttached: '告警证据将自动附着到工单；仅需填写人工处置所需字段。', policyHint: '策略按来源匹配，控制告警级别、恢复观察、工单要求和自动关闭。', newPolicy: '新建策略', noPoliciesTitle: '尚未配置告警策略', noPoliciesHint: '创建策略后，新的匹配事件将进入可确认、可关闭并可联动工单的处置队列。', code: '编码', name: '名称', sourceType: '来源类型', sourceRef: '来源标识', any: '任意', recoveryHold: '恢复观察（秒）', ackSla: '确认 SLA（秒）', enabled: '启用', autoClose: '恢复后自动关闭', requireWorkOrder: '关闭前必须有关联工单', allowException: '允许例外关闭', save: '保存', edit: '编辑', total: '全部告警', active: '告警中', unack: '未确认', progress: '处理中', verification: '待验证', shelved: '已搁置', closedToday: '今日关闭', new: '新建', acknowledged: '已确认', in_progress: '处理中', pending_verification: '待验证', closed: '已关闭', eventOpened: '首次告警', eventRepeated: '重复发生', eventRecovered: '条件恢复', eventClosed: '告警关闭', eventBound: '关联工单', eventShelved: '告警搁置', eventUnshelved: '解除搁置', eventEscalated: 'SLA 升级', ai_fault: 'AI 设备异常' },
  en: { eyebrow: 'OPERATIONS', title: 'Alarm Center', subtitle: 'Current operational alarms and historical device events are separated so archived evidence is never mistaken for an active incident.', policies: 'Alarm policies', refresh: 'Refresh', close: 'Close', overview: 'Alarm overview', viewSwitch: 'Alarm views', queue: 'Operations queue', history: 'Historical device events', search: 'Search', searchPlaceholder: 'Title, source, or fingerprint', historySearchPlaceholder: 'Device, event type, or identifier', severity: 'Severity', condition: 'Condition', handling: 'Handling', all: 'All', critical: 'Critical', major: 'Major', warning: 'Warning', info: 'Info', firing: 'Firing', recovered: 'Recovered', filter: 'Filter', reset: 'Reset', appliedFilters: 'Applied filters:', records: 'records', includeClosed: 'Include closed', alarm: 'Alarm', state: 'State', occurrence: 'Count', owner: 'Owner', workOrder: 'Work order', actions: 'Actions', loading: 'Loading…', empty: 'No operational alarms match the current filters.', viewHistory: 'View existing historical events', device: 'Device', time: 'Time', event: 'Event', historyEmpty: 'No matching historical events on this page.', historyNotice: 'Historical device events come from existing time-series data and are read-only. They do not indicate an active incident; only newly reported events materialized by an alarm policy can be acknowledged, closed, or linked to a work order.', historyReadOnly: 'This is historical evidence. It is read-only and cannot directly create an operational work order.', originalData: 'Original event data', user: 'User', unassigned: 'Unassigned', notLinked: 'Not linked', acknowledge: 'Acknowledge', details: 'Details', page: 'Page', previous: 'Previous', next: 'Next', createWorkOrder: 'Create linked work order', viewWorkOrder: 'View linked work order', shelve: 'Shelve', unshelve: 'Unshelve', assign: 'Assign', closeAlarm: 'Close alarm', alarmDetails: 'Alarm details', evidence: 'Alarm evidence', snapshot: 'Snapshot', source: 'Source', firstSeen: 'First seen', recoveredAt: 'Recovered at', timeline: 'Handling timeline', noTimeline: 'No handling events yet', assignee: 'Assignee', selectAssignee: 'Select an assignee', reason: 'Reason', shelveReasonHint: 'For example: scheduled maintenance or a known duplicate', until: 'Shelved until', untilHint: 'Leave blank to require manual unshelving.', disposition: 'Closure disposition', resolved: 'Resolved', falsePositive: 'False positive', duplicate: 'Duplicate', maintenance: 'Maintenance window', noAction: 'No action required', comment: 'Handling note', closeGuardHint: 'Normal closure requires a recovered, verified condition. Work-order requirements are enforced by the server policy.', cancel: 'Cancel', confirm: 'Confirm', template: 'Work-order template', selectTemplate: 'Select a template', priority: 'Priority', low: 'Low', normal: 'Normal', high: 'High', urgent: 'Urgent', workOrderTitle: 'Work-order title', summary: 'Work-order summary', summaryHint: 'Add field observations, impact, or the handling objective.', form: 'Work-order form', formHint: 'Alarm evidence, timeline, and original context are linked automatically; no JSON copy-paste is needed.', create: 'Create work order', evidenceAttached: 'Alarm evidence will be attached automatically. Enter only the fields needed for human handling.', policyHint: 'Policies match sources and control severity, recovery hold, work-order requirements, and automatic closure.', newPolicy: 'New policy', noPoliciesTitle: 'No alarm policies configured', noPoliciesHint: 'Create a policy so matching new events enter the operational queue with acknowledgement, closure, and work-order linkage.', code: 'Code', name: 'Name', sourceType: 'Source type', sourceRef: 'Source reference', any: 'Any', recoveryHold: 'Recovery hold (sec)', ackSla: 'Acknowledgement SLA (sec)', enabled: 'Enabled', autoClose: 'Auto-close after recovery', requireWorkOrder: 'Work order required before closure', allowException: 'Allow exception closure', save: 'Save', edit: 'Edit', total: 'Total alarms', active: 'Firing', unack: 'Unacknowledged', progress: 'In progress', verification: 'Pending verification', shelved: 'Shelved', closedToday: 'Closed today', new: 'New', acknowledged: 'Acknowledged', in_progress: 'In progress', pending_verification: 'Pending verification', closed: 'Closed', eventOpened: 'Alarm opened', eventRepeated: 'Repeated occurrence', eventRecovered: 'Condition recovered', eventClosed: 'Alarm closed', eventBound: 'Work order linked', eventShelved: 'Alarm shelved', eventUnshelved: 'Unshelved', eventEscalated: 'SLA escalated', ai_fault: 'AI device fault' }
}
const extraCopy = {
  zh: {
    eventParameters: '事件参数',
    owner: '当前处理人',
    unassigned: '待分派',
    unclaimed: '待认领',
    handlingCompleted: '处置完成',
    assign: '分派处理人',
    assignee: '告警处理人',
    selectAssignee: '请选择告警处理人',
    moreActions: '更多操作',
    shelve: '暂停告警通知',
    unshelve: '恢复告警通知',
    shelved: '通知已暂停',
    reason: '暂停通知原因',
    shelveReasonHint: '例如：计划维护期间暂停重复通知',
    until: '暂停通知至',
    untilHint: '留空则需手动恢复告警通知。',
    pauseNotificationsHelp: '仅暂停此告警的后续通知，不会暂停或关闭关联工单。',
    eventShelved: '告警通知已暂停',
    eventUnshelved: '告警通知已恢复',
    itemsPerPage: '每页显示',
    pagination: '告警列表分页',
    goToPage: '前往第 {page} 页',
    policyHint: '选择哪些事件进入告警中心，以及发生和恢复后如何处理。编码、来源类型、匹配优先级等技术参数由系统自动维护。',
    policyBasicSettings: '适用范围',
    policyHandlingSettings: '处置方式',
    policyName: '策略名称',
    policyNameHelp: '必填。用于在策略列表中识别这条规则，例如“重点区域入侵告警”。不会改变设备上报的原始事件名称。',
    policyNameRequired: '请填写策略名称。',
    applicableEvent: '适用事件',
    applicableEventHelp: '按具体事件时，先选择产品或系统事件来源，再从该范围内多选事件；按事件级别时，可选择物模型消息、告警或故障。选择“所有设备事件”会匹配当前项目后续上报的任意设备事件。',
    allDeviceEvents: '所有设备事件',
    searchEvents: '搜索事件名称或标识',
    noMatchingEvents: '没有匹配的事件。',
    allEventsSelected: '全部事件',
    eventSelectionRequired: '请至少选择一个适用事件，或选择“所有设备事件”。',
    selectedEventCount: '已选 {count} 项',
    moreEventCount: '等 {count} 项',
    handlingSeverity: '处置级别',
    severityHelp: '这是告警中心的处置严重度，不是物模型的事件类型。物模型“消息/告警/故障”描述事件语义；这里的“提示/警告/严重/紧急”决定告警颜色和自动工单默认优先级。配置策略后，此值优先于事件上报参数中的 alarm_severity 或 level。',
    policyStatus: '策略状态',
    policyStatusHelp: '停用后，新上报事件不再匹配此策略；已经生成的告警不会被删除或改变。',
    disabled: '停用',
    recoveryMode: '恢复后处理',
    recoveryModeHelp: '设备上报恢复后，可由系统在观察期结束时自动关闭，也可以保留在队列中等待人工确认关闭。',
    recoveryAutoClose: '观察期结束后自动关闭',
    recoveryManualClose: '保留并由人工关闭',
    recoveryHold: '恢复观察时间',
    recoveryHoldHelp: '用于过滤设备短暂恢复后马上再次告警的抖动。观察期内不会自动关闭；稳定超过该时间后才允许关闭。',
    workOrderHandling: '工单处理',
    workOrderHandlingHelp: '“不要求”适合简单告警；“关闭前必须有关联工单”要求人工建单并完成；“自动创建”会在首次告警时按模板建单。',
    workOrderNone: '不要求工单',
    workOrderRequired: '关闭前必须有关联工单',
    workOrderAuto: '告警时自动创建工单',
    workOrderTemplateHelp: '必选。系统用该模板创建工单，并自动附带告警来源、证据和时间线。',
    workOrderTemplateRequired: '选择自动创建工单时，必须选择一个工单模板。',
    exceptionClose: '告警未恢复时关闭',
    exceptionCloseHelp: '允许处理人将仍在告警中的事件标记为误报、重复、维护窗口或无需处理。一般建议禁止，仅在业务确实需要时开启。',
    notAllowed: '不允许（推荐）',
    allowed: '允许',
    ackSla: '确认时限',
    ackSlaHelp: '从首次告警开始计时。超过所配置的天、小时、分钟后仍未被人员确认，系统记录“确认超时”升级事件；三项均为 0 表示不限制。',
    resolutionSla: '处置时限',
    resolutionSlaHelp: '从首次告警开始计时。超过所配置的天、小时、分钟后仍未关闭，系统记录“处置超时”升级事件；三项均为 0 表示不限制。',
    days: '天',
    hours: '小时',
    minutes: '分钟',
    zeroDurationNoLimit: '三项均为 0 时不限制。',
    invalidDuration: '时限必须使用非负整数，小时为 0–23，分钟为 0–59。',
    legacySecondRemainder: '当前历史值包含秒数；不修改时保持原值，修改后按分钟保存。',
    tslInfoEvent: '物模型：消息',
    tslAlertEvent: '物模型：告警',
    tslErrorEvent: '物模型：故障',
    noLimit: '不限制',
    immediately: '立即',
    currentSetting: '当前设置',
    aiFaultEvent: 'AI 预测维护异常',
    ruleAlarmEvent: '规则引擎告警',
    illegalParkingEvent: '非法停车告警',
    fireLaneOccupiedEvent: '消防车道占用告警',
    indoorFirePassageEvent: '消防通道阻塞告警',
    objectMissingEvent: '物品缺失告警',
    areaIntrusionEvent: '区域入侵告警',
    product: '产品',
    remove: '移除',
    delete: '删除',
    deletePolicyConfirm: '确定删除告警策略“{name}”吗？删除后，新事件将不再匹配此策略。',
    matchBasis: '匹配依据',
    matchBasisHelp: '可按具体产品事件、物模型事件级别或全部设备事件匹配。具体事件最精确；事件级别适合让同类消息、告警或故障统一进入告警中心。',
    matchSpecificEvents: '按具体产品事件',
    matchEventLevels: '按事件级别',
    selectProduct: '先选择产品或事件来源',
    systemAndObservedEvents: '系统内置与已上报事件',
    eventLevelHint: '匹配产品物模型中的事件类型；可多选。此处不会改变告警中心的处置级别。',
    eventLevelSelectionRequired: '请至少选择一个事件级别。',
    allEventsPolicyHint: '当前项目后续上报的所有设备事件都会进入告警中心，请仅在确有全量监控需求时使用。',
    eventLevelInfo: '消息',
    eventLevelAlert: '告警',
    eventLevelError: '故障',
    sourceDeviceEvent: '设备事件',
    timelineObserved: '收到事件信号',
    timelineAcknowledged: '告警已确认',
    timelineAssigned: '告警已分派',
    timelineAutoCreateFailed: '自动创建工单失败',
    timelineUnknown: '系统处置记录',
    timelineAutoRecovery: '恢复观察期已通过',
    timelineSuppressionExpired: '暂停通知时间已到，已自动恢复告警通知',
    assignedTo: '分派给 {name}',
    handledBy: '操作人：{name}',
    dispositionText: '关闭结论：{value}',
    clearedEvidence: '消警证据',
    clearedSnapshot: '消警现场截图',
    alarmEvidence: '告警证据',
    evidenceComparison: '双图对比',
    clearedAt: '消除时间',
    clearedTarget: '消除状态',
    noClearedEvidence: '暂无消警现场证据',
    noAlarmEvidence: '暂无告警现场抓拍',
    alarmTarget: '告警目标',
    firingState: '告警中',
    clearedState: '已消除'
  },
  en: {
    eventParameters: 'Event parameters',
    owner: 'Current handler',
    unassigned: 'Awaiting assignment',
    unclaimed: 'Awaiting claim',
    handlingCompleted: 'Completed',
    assign: 'Assign handler',
    assignee: 'Alarm handler',
    selectAssignee: 'Select an alarm handler',
    moreActions: 'More actions',
    shelve: 'Pause alarm notifications',
    unshelve: 'Resume alarm notifications',
    shelved: 'Notifications paused',
    reason: 'Reason for pausing notifications',
    shelveReasonHint: 'For example: pause repeated notifications during scheduled maintenance',
    until: 'Pause notifications until',
    untilHint: 'Leave blank to resume alarm notifications manually.',
    pauseNotificationsHelp: 'This only pauses subsequent alarm notifications. It does not pause or close the linked work order.',
    eventShelved: 'Alarm notifications paused',
    eventUnshelved: 'Alarm notifications resumed',
    itemsPerPage: 'Rows per page',
    pagination: 'Alarm list pagination',
    goToPage: 'Go to page {page}',
    policyHint: 'Choose which events enter the Alarm Center and what happens when they fire or recover. Technical identifiers, source types, and match priority are maintained automatically.',
    policyBasicSettings: 'Applies to',
    policyHandlingSettings: 'Handling behavior',
    policyName: 'Policy name',
    policyNameHelp: 'Required. Identifies this policy in the list, for example “Critical area intrusion”. It does not change the original event name reported by a device.',
    policyNameRequired: 'Enter a policy name.',
    applicableEvent: 'Applicable event',
    applicableEventHelp: 'For specific events, select a product or built-in source first and then choose events in that scope. For event levels, select product-model Message, Alert, or Error. “All device events” matches any device event subsequently reported in the project.',
    allDeviceEvents: 'All device events',
    searchEvents: 'Search event name or identifier',
    noMatchingEvents: 'No events match the search.',
    allEventsSelected: 'All events',
    eventSelectionRequired: 'Select at least one applicable event, or choose “All device events”.',
    selectedEventCount: '{count} selected',
    moreEventCount: '{count} events',
    handlingSeverity: 'Operational severity',
    severityHelp: 'This is the Alarm Center operational severity, not the product-model event type. Product-model Message/Alert/Error describes event semantics; Info/Warning/Major/Critical controls alarm color and the default priority of automatically created work orders. A configured policy overrides alarm_severity or level reported by the event.',
    policyStatus: 'Policy status',
    policyStatusHelp: 'When disabled, new events no longer match this policy. Existing alarms are kept unchanged.',
    disabled: 'Disabled',
    recoveryMode: 'After recovery',
    recoveryModeHelp: 'After a recovery signal, the system can close the alarm when the observation period ends or keep it for a person to close.',
    recoveryAutoClose: 'Auto-close after observation',
    recoveryManualClose: 'Keep for manual closure',
    recoveryHold: 'Recovery observation',
    recoveryHoldHelp: 'Filters brief recoveries followed by another alarm. The alarm cannot auto-close during this period and becomes closable only after remaining stable.',
    workOrderHandling: 'Work-order handling',
    workOrderHandlingHelp: 'Use “Not required” for simple alarms, require a completed linked work order before closure, or create one automatically from a template on the first occurrence.',
    workOrderNone: 'Work order not required',
    workOrderRequired: 'Linked work order required before closure',
    workOrderAuto: 'Create a work order automatically',
    workOrderTemplateHelp: 'Required for automatic creation. The selected template is used and receives alarm source, evidence, and timeline data.',
    workOrderTemplateRequired: 'Select a work-order template when automatic creation is enabled.',
    exceptionClose: 'Close while still firing',
    exceptionCloseHelp: 'Lets handlers close a firing alarm as a false positive, duplicate, maintenance window, or no-action case. Keep this disabled unless the operating process requires it.',
    notAllowed: 'Not allowed (recommended)',
    allowed: 'Allowed',
    ackSla: 'Acknowledgement deadline',
    ackSlaHelp: 'Counts from the first occurrence. If nobody acknowledges the alarm within the configured days, hours, and minutes, an acknowledgement-overdue escalation is recorded. All zeros means no limit.',
    resolutionSla: 'Resolution deadline',
    resolutionSlaHelp: 'Counts from the first occurrence. If the alarm remains open beyond the configured days, hours, and minutes, a resolution-overdue escalation is recorded. All zeros means no limit.',
    days: 'days',
    hours: 'hours',
    minutes: 'minutes',
    zeroDurationNoLimit: 'All zeros means no limit.',
    invalidDuration: 'Use non-negative integers; hours must be 0–23 and minutes 0–59.',
    legacySecondRemainder: 'The existing value includes seconds. It is preserved until this duration is edited, then saved to minute precision.',
    tslInfoEvent: 'Product model: Message',
    tslAlertEvent: 'Product model: Alert',
    tslErrorEvent: 'Product model: Error',
    noLimit: 'No limit',
    immediately: 'Immediately',
    currentSetting: 'Current setting',
    aiFaultEvent: 'AI predictive maintenance fault',
    ruleAlarmEvent: 'Rule-engine alarm',
    illegalParkingEvent: 'Illegal parking alarm',
    fireLaneOccupiedEvent: 'Fire-lane occupancy alarm',
    indoorFirePassageEvent: 'Fire passage blockage alarm',
    objectMissingEvent: 'Object missing alarm',
    areaIntrusionEvent: 'Area intrusion alarm',
    product: 'Product',
    remove: 'Remove',
    delete: 'Delete',
    deletePolicyConfirm: 'Delete alarm policy “{name}”? New events will no longer match it.',
    matchBasis: 'Match by',
    matchBasisHelp: 'Match specific product events, product-model event levels, or every device event. Specific events are the most precise; levels are useful for handling all messages, alerts, or errors consistently.',
    matchSpecificEvents: 'Specific product events',
    matchEventLevels: 'Event levels',
    selectProduct: 'Select a product or event source first',
    systemAndObservedEvents: 'Built-in and observed events',
    eventLevelHint: 'Matches the event type in the product model. Multiple levels may be selected. This does not change the Alarm Center operational severity.',
    eventLevelSelectionRequired: 'Select at least one event level.',
    allEventsPolicyHint: 'Every device event subsequently reported in this project will enter the Alarm Center. Use this only when full event coverage is intended.',
    eventLevelInfo: 'Message',
    eventLevelAlert: 'Alert',
    eventLevelError: 'Error',
    sourceDeviceEvent: 'Device event',
    timelineObserved: 'Event signal received',
    timelineAcknowledged: 'Alarm acknowledged',
    timelineAssigned: 'Alarm assigned',
    timelineAutoCreateFailed: 'Automatic work-order creation failed',
    timelineUnknown: 'System handling record',
    timelineAutoRecovery: 'Recovery observation passed',
    timelineSuppressionExpired: 'Notification pause expired and notifications resumed automatically',
    assignedTo: 'Assigned to {name}',
    handledBy: 'Handled by {name}',
    dispositionText: 'Closure disposition: {value}',
    clearedEvidence: 'Clearance evidence',
    clearedSnapshot: 'Clearance snapshot',
    alarmEvidence: 'Alarm evidence',
    evidenceComparison: 'Comparison',
    clearedAt: 'Cleared at',
    clearedTarget: 'Clearance status',
    noClearedEvidence: 'No clearance evidence',
    noAlarmEvidence: 'No alarm snapshot',
    alarmTarget: 'Alarm target',
    firingState: 'Firing',
    clearedState: 'Cleared'
  }
}
const t = key => {
  if (key === 'filterConditions') return language.value === 'zh' ? '筛选条件' : 'Filters'
  if (extraCopy[language.value]?.[key]) return extraCopy[language.value][key]
  return copy[language.value]?.[key] || copy.en[key] || key
}

const builtInPolicyEventKeys = {
  ai_fault: 'aiFaultEvent',
  rule_alarm: 'ruleAlarmEvent',
  illegal_parking_alarm: 'illegalParkingEvent',
  fire_lane_occupied_alarm: 'fireLaneOccupiedEvent',
  indoor_fire_passage_occupied_alarm: 'indoorFirePassageEvent',
  object_missing_alarm: 'objectMissingEvent',
  area_intrusion_alarm: 'areaIntrusionEvent'
}
const builtInPolicyEventIDs = ['ai_fault', ...ALARM_EVENT_IDS.filter(id => id !== 'area_intrusion_leave')]
const recoveryHoldPresets = [0, 30, 60, 300, 600]
const handlingKeys = ['new', 'acknowledged', 'in_progress', 'pending_verification', 'shelved', 'closed']
const pageSizeOptions = [10, 20, 50, 100]
const page = ref(1)
const pageSize = ref(10)
const total = ref(0)
const items = ref([])
const stats = ref({})
const loading = ref(false)
const requestError = ref('')
const filters = ref({ search: '', severity: '', condition: '', handling: '', includeClosed: true, closedToday: false })
const activeQuickFilters = ref([])
const activeView = ref('queue')
const legacyPage = ref(1)
const legacyPageSize = ref(10)
const legacyEvents = ref([])
const legacyTotal = ref(0)
const legacySearch = ref('')
const historyLoading = ref(false)
const legacyDetail = ref(null)
const selected = ref(null)
const selectedEvents = ref([])
const detailLoading = ref(false)
const participants = ref([])
const operation = ref('')
const operationSubmitting = ref(false)
const operationError = ref('')
const operationForm = ref({})
const showWorkOrder = ref(false)
const workOrderTemplates = ref([])
const workOrderTemplate = ref(null)
const workOrderSubmitting = ref(false)
const workOrderError = ref('')
const workOrderSuccess = ref('')
const workOrder = ref({})
const showPolicies = ref(false)
const policies = ref([])
const policyTemplates = ref([])
const policySourceEvents = ref([])
const policyEventSearch = ref('')
const policyProductGroup = ref('')
const catalogProducts = ref([])
const catalogDevices = ref([])
const editingPolicy = ref(null)
const policyError = ref('')
const policyHelp = ref({ visible: false, text: '', top: 0, left: 0 })
const policyHelpPopover = ref(null)
const operationDialog = ref(null)
const workOrderDialog = ref(null)
const policyDialog = ref(null)
const legacyDialog = ref(null)
const detailDialog = ref(null)
const modalReturnFocus = ref(null)
const detailReturnFocus = ref(null)
let queueLoadVersion = 0
let historyLoadVersion = 0
let detailLoadVersion = 0
let policyHelpAnchor = null

const statCards = computed(() => [
  { key: 'total', tone: 'neutral', label: t('total'), value: stats.value.total || 0, hint: t('queue'), filter: {} },
  { key: 'firing', tone: 'danger', label: t('active'), value: stats.value.firing || 0, hint: t('condition'), filter: { condition: 'firing' } },
  { key: 'unack', tone: 'warning', label: t('unack'), value: stats.value.unacknowledged || 0, hint: t('handling'), filter: { condition: 'firing', handling: 'new' } },
  { key: 'progress', tone: 'primary', label: t('progress'), value: stats.value.in_progress || 0, hint: t('handling'), filter: { handling: 'in_progress' } },
  { key: 'verify', tone: 'info', label: t('verification'), value: stats.value.pending_verification || 0, hint: t('handling'), filter: { handling: 'pending_verification' } },
  { key: 'closed', tone: 'success', label: t('closedToday'), value: stats.value.closed_today || 0, hint: t('closed'), filter: { includeClosed: true, handling: 'closed', closedToday: true } }
])

function statCardIcon(key) {
  switch (key) {
    case 'total': return 'bi-bell-fill'
    case 'firing': return 'bi-exclamation-octagon-fill'
    case 'unack': return 'bi-exclamation-triangle-fill'
    case 'progress': return 'bi-gear-wide-connected'
    case 'verify': return 'bi-shield-check'
    case 'closed': return 'bi-check-circle-fill'
    default: return 'bi-bell'
  }
}

function statCardBoxClass(tone) {
  switch (tone) {
    case 'danger': return 'icon-danger'
    case 'warning': return 'icon-warning'
    case 'primary': return 'icon-brand'
    case 'info': return 'icon-chart-2'
    case 'success': return 'icon-chart-2'
    default: return 'icon-neutral'
  }
}

function statCardValueColor(tone) {
  switch (tone) {
    case 'danger': return 'var(--color-danger, #ef4444)'
    case 'warning': return 'var(--color-warning, #f59e0b)'
    case 'primary': return 'var(--color-primary, #3b82f6)'
    case 'info': return 'var(--color-info, #06b6d4)'
    case 'success': return 'var(--color-success, #10b981)'
    default: return 'inherit'
  }
}
const selectedAlarm = computed(() => alarmOf(selected.value))
const operationTitle = computed(() => operation.value === 'assign' ? t('assign') : operation.value === 'shelve' ? t('shelve') : t('closeAlarm'))
const activeFilterLabels = computed(() => {
  const labels = []
  if (filters.value.search) labels.push(filters.value.search)
  if (filters.value.severity) labels.push(severityText(filters.value.severity))
  if (filters.value.condition) labels.push(conditionText(filters.value.condition))
  if (filters.value.handling) labels.push(handlingText(filters.value.handling))
  if (filters.value.includeClosed && !filters.value.closedToday) labels.push(t('includeClosed'))
  if (filters.value.closedToday) labels.push(t('closedToday'))
  return labels
})
const filteredLegacyEvents = computed(() => {
  const keyword = legacySearch.value.toLowerCase()
  if (!keyword) return legacyEvents.value
  return legacyEvents.value.filter(event => [legacyDeviceName(event), legacyProductName(event), legacyEventTitle(event), event.device_code, event.event_id].some(value => String(value || '').toLowerCase().includes(keyword)))
})
const deviceByCode = computed(() => new Map(catalogDevices.value.map(device => [String(device.code || '').trim(), device])))
const productByCode = computed(() => new Map(catalogProducts.value.map(product => [String(product.code || '').trim(), product])))
const policyEventLevels = computed(() => [
  { value: 'info', label: t('eventLevelInfo') },
  { value: 'alert', label: t('eventLevelAlert') },
  { value: 'error', label: t('eventLevelError') }
])
const eventNameIdentifiers = computed(() => {
  const names = new Map()
  for (const event of policySourceEvents.value) {
    const key = String(event.name || '').trim().toLowerCase()
    if (!key) continue
    if (!names.has(key)) names.set(key, new Set())
    names.get(key).add(event.id)
  }
  return names
})
const policyEventOptions = computed(() => {
  const rawOptions = new Map()
  for (const id of builtInPolicyEventIDs) rawOptions.set(id, { value: id, id, name: t(builtInPolicyEventKeys[id] || id), type: '', groupKey: '__system__', groupLabel: t('systemAndObservedEvents') })
  for (const event of policySourceEvents.value) rawOptions.set(event.value, { ...event, groupKey: event.productCode, groupLabel: productDisplayName(event.productCode, event.productName) })
  for (const event of legacyEvents.value) {
    const id = String(event?.event_id || '').trim()
    if (!id) continue
    const productCode = legacyProductCode(event)
    const scopedValue = productCode ? `${productCode}:${id}` : id
    if (!rawOptions.has(scopedValue)) rawOptions.set(scopedValue, { value: scopedValue, id, name: prettifyIdentifier(id), type: '', groupKey: productCode || '__system__', groupLabel: productDisplayName(productCode) || t('systemAndObservedEvents') })
  }
  const referencedValues = [...policies.value.flatMap(policySourceRefs), ...policySourceRefs(editingPolicy.value)]
  for (const value of referencedValues) {
    if (rawOptions.has(value)) continue
    const { productCode, eventID } = splitPolicyEventRef(value)
    const matching = policySourceEvents.value.find(event => event.id === eventID && (!productCode || event.productCode === productCode))
    rawOptions.set(value, { value, id: eventID, name: matching?.name || prettifyIdentifier(eventID), type: matching?.type || '', groupKey: productCode || '__system__', groupLabel: productDisplayName(productCode) || t('systemAndObservedEvents') })
  }
  const collator = new Intl.Collator(language.value === 'en' ? 'en' : 'zh-CN')
  return Array.from(rawOptions.values(), option => {
    const duplicateName = (eventNameIdentifiers.value.get(String(option.name || '').trim().toLowerCase())?.size || 0) > 1
    return {
      ...option,
      label: duplicateName ? `${option.name} (${option.id})` : (option.name || option.id),
      meta: [option.type ? eventLevelText(option.type) : '', option.groupKey === '__system__' ? '' : option.groupLabel].filter(Boolean).join(' · ')
    }
  }).sort((left, right) => collator.compare(left.label, right.label))
})
const policyEventGroups = computed(() => {
  const groups = new Map()
  for (const option of policyEventOptions.value) {
    const key = option.groupKey || '__system__'
    if (!groups.has(key)) groups.set(key, { key, label: option.groupLabel || t('systemAndObservedEvents'), options: [] })
    groups.get(key).options.push(option)
  }
  const result = Array.from(groups.values())
  result.sort((left, right) => left.key === '__system__' ? 1 : right.key === '__system__' ? -1 : left.label.localeCompare(right.label, language.value === 'en' ? 'en' : 'zh-CN'))
  return result
})
const selectedPolicyEventOptions = computed(() => policySourceRefs(editingPolicy.value).map(value => {
  const option = policyEventOptions.value.find(item => item.value === value)
  if (!option) return { value, label: prettifyIdentifier(splitPolicyEventRef(value).eventID) }
  return { ...option, label: option.groupKey === '__system__' ? option.label : `${option.groupLabel} · ${option.label}` }
}))
const filteredPolicyEventOptions = computed(() => {
  const group = policyEventGroups.value.find(item => item.key === policyProductGroup.value)
  const options = group?.options || []
  const keyword = policyEventSearch.value.toLowerCase()
  if (!keyword) return options
  return options.filter(option => `${option.label} ${option.id} ${option.meta}`.toLowerCase().includes(keyword))
})
const recoveryHoldOptions = computed(() => buildDurationOptions(recoveryHoldPresets, editingPolicy.value?.recovery_hold_seconds, t('immediately')))

function alarmOf(item) { return item?.alarm || {} }
function evidenceOf(item) { return item?.evidence || {} }
function prettifyIdentifier(value) { return String(value || '').trim().replaceAll('_', ' ').replaceAll('-', ' ') || '-' }
function splitPolicyEventRef(value) {
  const ref = String(value || '').trim()
  const separator = ref.indexOf(':')
  if (separator > 0) {
    const productCode = ref.slice(0, separator)
    if (productByCode.value.has(productCode) || policySourceEvents.value.some(event => event.productCode === productCode)) return { productCode, eventID: ref.slice(separator + 1) }
  }
  return { productCode: '', eventID: ref }
}
function duplicateEntityName(items, name, code) {
  if (!name) return false
  return items.filter(item => String(item.name || '').trim().toLowerCase() === String(name).trim().toLowerCase() && String(item.code || '') !== String(code || '')).length > 0
}
function deviceDisplayName(code, explicitName = '') {
  const device = deviceByCode.value.get(String(code || '').trim())
  const name = String(explicitName || device?.name || '').trim()
  if (!name) return String(code || '').trim() || '-'
  return duplicateEntityName(catalogDevices.value, name, code) ? `${name} (${code})` : name
}
function productDisplayName(code, explicitName = '') {
  const product = productByCode.value.get(String(code || '').trim())
  const name = String(explicitName || product?.name || '').trim()
  if (!name) return String(code || '').trim() || '-'
  return duplicateEntityName(catalogProducts.value, name, code) ? `${name} (${code})` : name
}
function eventLevelText(level) { return t(({ info: 'eventLevelInfo', alert: 'eventLevelAlert', error: 'eventLevelError' })[String(level || '').toLowerCase()] || level) }
function productModel(product) {
  let model = product?.model ?? product?.config ?? {}
  if (typeof model === 'string') {
    try { model = JSON.parse(model || '{}') } catch (_) { model = {} }
  }
  return model?.tsl || model?.model || model || {}
}
function productEventID(event) { return String(event?.identifier || event?.id || event?.key || '').trim() }
function productEventParameters(event) {
  for (const key of ['outputData', 'output_data', 'params', 'parameters', 'properties']) {
    if (Array.isArray(event?.[key])) return event[key]
  }
  return []
}
function productEventParameterID(parameter) { return String(parameter?.identifier || parameter?.id || parameter?.key || '').trim() }
function eventDefinition(eventID, productCode = '') {
  const id = String(eventID || '').trim()
  return policySourceEvents.value.find(event => event.id === id && (!productCode || event.productCode === productCode)) || policySourceEvents.value.find(event => event.id === id)
}
function eventDisplayName(eventID, productCode = '', explicitName = '') {
  const id = String(eventID || '').trim()
  const definition = eventDefinition(id, productCode)
  const builtInName = builtInPolicyEventKeys[id] ? t(builtInPolicyEventKeys[id]) : ''
  const name = String(explicitName || definition?.name || builtInName || '').trim()
  if (!name) return prettifyIdentifier(id)
  const duplicateName = (eventNameIdentifiers.value.get(name.toLowerCase())?.size || 0) > 1
  return duplicateName ? `${name} (${id})` : name
}
function alarmDeviceCode(item) { const evidence = evidenceOf(item); return evidence.device_code || evidence.params?.device_code || '' }
function alarmProductCode(item) { const evidence = evidenceOf(item); return evidence.product_code || evidence.params?.product_code || deviceByCode.value.get(alarmDeviceCode(item))?.product_code || '' }
function alarmEventID(item) { const alarm = alarmOf(item); const evidence = evidenceOf(item); return alarm.source_ref || evidence.event_id || evidence.params?.event_id || '' }
function alarmDeviceName(item) { const evidence = evidenceOf(item); return deviceDisplayName(alarmDeviceCode(item), evidence.device_name || evidence.params?.device_name) }
function alarmProductName(item) { const evidence = evidenceOf(item); return productDisplayName(alarmProductCode(item), evidence.product_name || evidence.params?.product_name) }
function alarmEventName(item) { const evidence = evidenceOf(item); return eventDisplayName(alarmEventID(item), alarmProductCode(item), evidence.event_name || evidence.params?.event_name) }
function titleOf(item) {
  const alarm = alarmOf(item)
  const title = String(alarm.title || '').trim()
  return title && title !== alarm.source_ref ? title : alarmEventName(item)
}
function alarmContextText(item) { return [alarmDeviceName(item), alarmEventName(item)].filter(value => value && value !== '-').join(' · ') || '-' }
function severityOf(item) { return alarmOf(item).severity || 'info' }
function severityText(value) { return t(value || 'info') }
function severityTone(value) { return ({ critical: 'danger', major: 'danger', warning: 'warning', info: 'primary' })[value] || 'secondary' }
function conditionText(value) { return t(value === 'recovered' ? 'recovered' : 'firing') }
function conditionClass(value) { return value === 'recovered' ? 'text-bg-success' : 'text-bg-danger' }
function handlingText(value) { return t(value || 'new') }
function formatTime(value) { return formatDateTime(value) }
function eventText(value) { const labels = { signal_observed: 'timelineObserved', opened: 'eventOpened', repeated: 'eventRepeated', acknowledged: 'timelineAcknowledged', assigned: 'timelineAssigned', recovered: 'eventRecovered', condition_recovered: 'eventRecovered', cleared: 'eventRecovered', closed: 'eventClosed', work_order_bound: 'eventBound', shelved: 'eventShelved', unshelved: 'eventUnshelved', escalated: 'eventEscalated', acknowledgement_sla_overdue: 'eventEscalated', resolution_sla_overdue: 'eventEscalated', work_order_auto_create_failed: 'timelineAutoCreateFailed' }; return t(labels[value] || 'timelineUnknown') }
function participantID(person) { return Number(person?.id || person?.ID || person?.user_id || 0) }
function ownerName(userID) {
  if (!Number(userID)) return t('unassigned')
  const person = participants.value.find(item => participantID(item) === Number(userID))
  if (!person) return `${t('user')} #${userID}`
  const name = participantName(person)
  const duplicate = participants.value.some(item => participantID(item) !== Number(userID) && participantName(item).toLowerCase() === name.toLowerCase())
  return duplicate ? `${name} (#${userID})` : name
}
function currentHandlerText(item) {
  const alarm = alarmOf(item)
  if (alarm.handling_status === 'closed') return t('handlingCompleted')
  if (!alarm.work_order_public_id) return ownerName(alarm.owner_user_id)
  const names = [...new Set((Array.isArray(item?.current_handlers) ? item.current_handlers : []).map(handler => {
    const displayName = String(handler?.display_name || '').trim()
    return displayName || ownerName(handler?.user_id)
  }).filter(Boolean))]
  return names.length ? names.join(language.value === 'en' ? ', ' : '、') : t('unclaimed')
}
function sourceTypeText(value) { return value === 'device_event' ? t('sourceDeviceEvent') : prettifyIdentifier(value) }
function dispositionLabel(value) { const key = ({ resolved: 'resolved', false_positive: 'falsePositive', duplicate: 'duplicate', maintenance: 'maintenance', no_action_required: 'noAction', auto_recovered: 'recovered', work_order_closed: 'eventClosed' })[value]; return key ? t(key) : t('timelineUnknown') }
function localizedTimelineText(value) {
  const text = String(value || '').trim()
  if (text === 'recovery verification passed') return t('timelineAutoRecovery')
  if (text === 'suppression expired') return t('timelineSuppressionExpired')
  return text
}
function timelineDetail(item) {
  const payload = item?.payload || {}
  const details = []
  const comment = localizedTimelineText(payload.comment)
  const reason = localizedTimelineText(payload.reason)
  if (comment) details.push(comment)
  else if (reason) details.push(reason)
  if (payload.owner_user_id) details.push(formatCopy('assignedTo', { name: ownerName(payload.owner_user_id) }))
  if (payload.disposition) details.push(formatCopy('dispositionText', { value: dispositionLabel(payload.disposition) }))
  if (payload.actor_user_id) details.push(formatCopy('handledBy', { name: ownerName(payload.actor_user_id) }))
  return details.join(language.value === 'en' ? '; ' : '；')
}
function canAcknowledge(item) { const alarm = alarmOf(item); return alarm.handling_status === 'new' && alarm.condition_status !== 'recovered' }
function canOperate(item) { return alarmOf(item).handling_status !== 'closed' }
function canAssignAlarm(item) { const alarm = alarmOf(item); return canOperate(item) && !alarm.work_order_public_id }
function canCreateWorkOrder(item) { const alarm = alarmOf(item); return canOperate(item) && !alarm.work_order_public_id }
function canExceptionClose(item) { return Boolean(item?.policy?.allow_exception_close) }
function canClose(item) { const alarm = alarmOf(item); return canOperate(item) && (alarm.condition_status === 'recovered' || canExceptionClose(item)) }
const activeEvidenceTab = ref('alarm')
function snapshotUrl(item) { const evidence = evidenceOf(item); return normaliseSnapshot(evidence.params?.snapshot_url || evidence.snapshot_url || evidence.params?.snapshot_base64) }
function clearedEvidenceOf(item) { return item?.cleared_evidence || {} }
function clearedSnapshotUrl(item) { const evidence = clearedEvidenceOf(item); return normaliseSnapshot(evidence.params?.snapshot_url || evidence.snapshot_url || evidence.params?.snapshot_base64) }
function hasClearedEvidence(item) { const cleared = clearedEvidenceOf(item); return Boolean(clearedSnapshotUrl(item) || cleared.event_id || cleared.params?.event_id || item?.alarm?.recovered_at || item?.alarm?.cleared_at) }
function hasAlarmEvidence(item) { return Boolean(snapshotUrl(item)) }
function timelineSnapshotUrl(event) { const evidence = event?.evidence || {}; return normaliseSnapshot(evidence.params?.snapshot_url || evidence.snapshot_url || evidence.params?.snapshot_base64) }
function clearedAlarmDeviceCode(item) { const evidence = clearedEvidenceOf(item); return evidence.device_code || evidence.params?.device_code || alarmDeviceCode(item) }
function clearedAlarmProductCode(item) { const evidence = clearedEvidenceOf(item); return evidence.product_code || evidence.params?.product_code || alarmProductCode(item) }
function clearedAlarmDeviceName(item) { const evidence = clearedEvidenceOf(item); return deviceDisplayName(clearedAlarmDeviceCode(item), evidence.device_name || evidence.params?.device_name) }
function clearedAlarmProductName(item) { const evidence = clearedEvidenceOf(item); return productDisplayName(clearedAlarmProductCode(item), evidence.product_name || evidence.params?.product_name) }
function clearedAlarmEventName(item) { const evidence = clearedEvidenceOf(item); const eventID = evidence.event_id || evidence.params?.event_id || ''; return eventDisplayName(eventID, clearedAlarmProductCode(item), evidence.event_name || evidence.params?.event_name) }
function clearedAlarmTargetName(item) { const evidence = clearedEvidenceOf(item); return evidence.params?.target_name || evidence.params?.alarm_status || t('clearedState') }
function alarmTargetName(item) { const evidence = evidenceOf(item); return evidence.params?.target_name || evidence.params?.alarm_status || t('firingState') }
function legacySnapshotUrls(event) {
  const payload = event?.params && typeof event.params === 'object' ? event.params : {}
  const values = ['snapshot_url', 'image_url', 'picture_url', 'capture_url', 'snapshot', 'image', 'picture', 'snapshot_base64', 'image_base64']
    .flatMap(key => [payload?.[key], event?.[key]])
    .concat(payload?.images || [], payload?.snapshot_urls || [], event?.images || [])
    .flat()
    .map(normaliseSnapshot)
    .filter(Boolean)
  return [...new Set(values)]
}
function normaliseSnapshot(value) {
  if (!value || typeof value !== 'string') return ''
  const text = value.trim()
  if (!text) return ''
  if (text.startsWith('data:') || text.startsWith('http') || text.startsWith('/')) return text
  if (/^[A-Za-z0-9+/=\r\n]+$/.test(text) && text.length > 256) return `data:image/jpeg;base64,${text.replace(/\s+/g, '')}`
  return `/${text.replace(/^\.?\//, '')}`
}
function participantName(person) { return person.display_name || person.username || `${t('user')} #${participantID(person)}` }
function errorMessage(error) { return error?.response?.data?.message || error?.message || 'Request failed' }
function legacyEventKey(event, index = 0) { return `${event.device_code || ''}-${event.event_id || ''}-${event.ts || event.timestamp || event.created_at || ''}-${index}` }
function legacyProductCode(event) { return event?.product_code || deviceByCode.value.get(String(event?.device_code || '').trim())?.product_code || '' }
function legacyDeviceName(event) { return deviceDisplayName(event?.device_code, event?.device_name) }
function legacyProductName(event) { return productDisplayName(legacyProductCode(event), event?.product_name) }
function legacyEventTitle(event) { return eventDisplayName(event?.event_id, legacyProductCode(event), event?.event_name) }
function eventParameterDefinitions(eventID, productCode = '') {
  const product = productByCode.value.get(String(productCode || '').trim())
  const definition = (productModel(product).events || []).find(event => productEventID(event) === String(eventID || '').trim())
  return productEventParameters(definition)
}
function legacyEventParameters(event) {
  const params = event?.params && typeof event.params === 'object' && !Array.isArray(event.params) ? event.params : {}
  const definitions = new Map(eventParameterDefinitions(event?.event_id, legacyProductCode(event)).map(parameter => [productEventParameterID(parameter), parameter]))
  return Object.entries(params)
    .filter(([id]) => !String(id).startsWith('_'))
    .map(([id, value]) => {
      const definition = definitions.get(String(id))
      return { id, name: String(definition?.name || '').trim() || prettifyIdentifier(id), value }
    })
}
function legacyParameterLabel(parameter) { return language.value === 'en' ? `${parameter.name} (${parameter.id})` : `${parameter.name}（${parameter.id}）` }
function formatLegacyParameterValue(value) { return value && typeof value === 'object' ? JSON.stringify(value) : String(value ?? '-') }
function formatPolicyDuration(seconds) {
  const value = Number(seconds) || 0
  if (value % 3600 === 0) return `${value / 3600} ${language.value === 'en' ? 'hr' : '小时'}`
  if (value % 60 === 0) return `${value / 60} ${language.value === 'en' ? 'min' : '分钟'}`
  return `${value} ${language.value === 'en' ? 'sec' : '秒'}`
}
function buildDurationOptions(presets, currentValue, zeroLabel) {
  const values = new Set(presets)
  const current = Number(currentValue)
  if (Number.isFinite(current) && current >= 0) values.add(current)
  return Array.from(values).sort((left, right) => left - right).map(value => ({
    value,
    label: value === 0 ? zeroLabel : `${formatPolicyDuration(value)}${presets.includes(value) ? '' : ` (${t('currentSetting')})`}`
  }))
}
function formatCopy(key, values = {}) { return Object.entries(values).reduce((text, [name, value]) => text.replaceAll(`{${name}}`, value), t(key)) }
function policySourceRefs(policy) {
  let values = policy?.source_refs
  let hasExplicitValues = Array.isArray(values)
  if (typeof values === 'string') {
    try { values = JSON.parse(values || '[]'); hasExplicitValues = Array.isArray(values) } catch (_) { values = [] }
  }
  if (!Array.isArray(values)) values = []
  if (!values.length && !hasExplicitValues && policy?.source_ref) values = [policy.source_ref]
  return Array.from(new Set(values.map(value => String(value || '').trim()).filter(value => value && value !== '*')))
}
function policyLevels(policy) {
  let values = policy?.event_levels
  if (typeof values === 'string') {
    try { values = JSON.parse(values || '[]') } catch (_) { values = [] }
  }
  if (!Array.isArray(values)) values = []
  return Array.from(new Set(values.map(value => String(value || '').trim().toLowerCase()).filter(value => ['info', 'alert', 'error'].includes(value))))
}
function policyMatchMode(policy) {
  const mode = String(policy?.match_mode || '').trim().toLowerCase()
  if (['events', 'levels', 'all'].includes(mode)) return mode
  if (policyLevels(policy).length) return 'levels'
  return policySourceRefs(policy).length ? 'events' : 'all'
}
function policyEventLabel(policy) {
  const mode = policyMatchMode(policy)
  if (mode === 'all') return t('allDeviceEvents')
  if (mode === 'levels') return policyLevels(policy).map(eventLevelText).join(language.value === 'en' ? ', ' : '、')
  const sourceRefs = policySourceRefs(policy)
  const labels = sourceRefs.map(value => {
    const option = policyEventOptions.value.find(item => item.value === value)
    if (option) return option.groupKey === '__system__' ? option.label : `${option.groupLabel} · ${option.label}`
    const { productCode, eventID } = splitPolicyEventRef(value)
    const name = eventDisplayName(eventID, productCode)
    return productCode ? `${productDisplayName(productCode)} · ${name}` : name
  })
  if (labels.length <= 2) return labels.join(language.value === 'en' ? ', ' : '、')
  return `${labels.slice(0, 2).join(language.value === 'en' ? ', ' : '、')} · ${formatCopy('moreEventCount', { count: labels.length })}`
}
function policyEventSelectionText(policy) {
  const mode = policyMatchMode(policy)
  if (mode === 'all') return t('allEventsSelected')
  if (mode === 'levels') return formatCopy('selectedEventCount', { count: policyLevels(policy).length })
  return formatCopy('selectedEventCount', { count: policySourceRefs(policy).length })
}
function syncPolicyMatchMode() {
  if (!editingPolicy.value) return
  if (editingPolicy.value.match_mode !== 'events') {
    editingPolicy.value.source_refs = []
    editingPolicy.value.source_ref = ''
  }
  if (editingPolicy.value.match_mode !== 'levels') editingPolicy.value.event_levels = []
}
function selectSpecificPolicyEvent() {
  if (!editingPolicy.value) return
  editingPolicy.value.source_refs = policySourceRefs(editingPolicy.value)
}
function removePolicyEvent(value) { if (editingPolicy.value) editingPolicy.value.source_refs = policySourceRefs(editingPolicy.value).filter(item => item !== value) }
function durationPartsFromSeconds(seconds) {
  const totalSeconds = Math.max(0, Math.floor(Number(seconds) || 0))
  const totalMinutes = Math.ceil(totalSeconds / 60)
  return {
    days: Math.floor(totalMinutes / 1440),
    hours: Math.floor((totalMinutes % 1440) / 60),
    minutes: totalMinutes % 60
  }
}
function durationPartsToSeconds(parts) {
  const values = [parts?.days, parts?.hours, parts?.minutes].map(value => Number(value))
  if (values.some(value => !Number.isInteger(value) || value < 0) || values[1] > 23 || values[2] > 59) return null
  const seconds = values[0] * 86400 + values[1] * 3600 + values[2] * 60
  return Number.isSafeInteger(seconds) ? seconds : null
}
function markPolicyDurationChanged(kind) {
  if (editingPolicy.value) editingPolicy.value[`${kind}_sla_changed`] = true
}
function policyDurationSeconds(kind) {
  if (!editingPolicy.value) return null
  if (!editingPolicy.value[`${kind}_sla_changed`]) return editingPolicy.value[`${kind}_sla_original_seconds`]
  return durationPartsToSeconds(editingPolicy.value[`${kind}_sla_parts`])
}
function policyHelpButton(event) {
  return event.target instanceof Element ? event.target.closest('.policy-help') : null
}
function handlePolicyHelpEnter(event) {
  const button = policyHelpButton(event)
  if (!button || (event.relatedTarget instanceof Node && button.contains(event.relatedTarget))) return
  policyHelpAnchor = button
  const rect = button.getBoundingClientRect()
  policyHelp.value = { visible: true, text: button.dataset.help || button.getAttribute('aria-label') || '', top: rect.bottom + 8, left: rect.left }
  nextTick(positionPolicyHelp)
}
function handlePolicyHelpLeave(event) {
  const button = policyHelpButton(event)
  if (!button || (event.relatedTarget instanceof Node && button.contains(event.relatedTarget))) return
  hidePolicyHelp()
}
function hidePolicyHelp() { policyHelpAnchor = null; policyHelp.value.visible = false }
function positionPolicyHelp() {
  if (!policyHelpAnchor || !policyHelpPopover.value) return
  const anchor = policyHelpAnchor.getBoundingClientRect()
  const popover = policyHelpPopover.value.getBoundingClientRect()
  const margin = 12
  let left = anchor.left + anchor.width / 2 - popover.width / 2
  left = Math.min(Math.max(margin, left), window.innerWidth - popover.width - margin)
  let top = anchor.bottom + 8
  if (top + popover.height > window.innerHeight - margin) top = anchor.top - popover.height - 8
  policyHelp.value.left = left
  policyHelp.value.top = Math.max(margin, top)
}
function policyWorkOrderMode(policy) {
  if (policy?.auto_create_work_order) return 'auto'
  return policy?.require_work_order ? 'required' : 'none'
}
function policyWorkOrderModeLabel(policy) { return t(({ none: 'workOrderNone', required: 'workOrderRequired', auto: 'workOrderAuto' })[policyWorkOrderMode(policy)]) }
function extractPolicySourceEvents(products) {
  const result = []
  const seen = new Set()
  for (const product of products || []) {
    const model = productModel(product)
    const productCode = String(product?.code || '').trim()
    const productName = String(product?.name || productCode).trim()
    for (const event of model.events || []) {
      const id = productEventID(event)
      if (!productCode || !id) continue
      const value = `${productCode}:${id}`
      if (seen.has(value)) continue
      seen.add(value)
      result.push({ value, id, name: String(event?.name || '').trim() || prettifyIdentifier(id), type: String(event?.type || '').trim().toLowerCase(), productCode, productName })
    }
  }
  return result
}

async function loadQueue() {
  const requestVersion = ++queueLoadVersion
  loading.value = true
  try {
    const params = {
      page: page.value,
      page_size: pageSize.value,
      search: filters.value.search,
      severity: filters.value.severity,
      condition: filters.value.condition,
      handling: filters.value.handling,
      include_closed: filters.value.includeClosed,
      closed_today: filters.value.closedToday
    }
    const [listResult, statsResult] = await Promise.allSettled([axios.get('/api/alarm-instances', { params }), axios.get('/api/alarm-instances/stats')])
    if (requestVersion !== queueLoadVersion) return
    if (listResult.status !== 'fulfilled' || listResult.value.data?.code !== 0) throw new Error(listResult.status === 'fulfilled' ? listResult.value.data?.message : listResult.reason?.message)
    const listResponse = listResult.value
    items.value = listResponse.data?.data?.items || []
    total.value = listResponse.data?.data?.total || 0
    if (statsResult.status === 'fulfilled' && statsResult.value.data?.code === 0) stats.value = statsResult.value.data?.data || {}
    else if (statsResult.status === 'rejected') requestError.value = errorMessage(statsResult.reason)
  } catch (error) {
    if (requestVersion !== queueLoadVersion) return
    console.error('load alarm center', error)
    requestError.value = errorMessage(error)
  } finally {
    if (requestVersion === queueLoadVersion) loading.value = false
  }
}

async function loadLegacyHistory() {
  const requestVersion = ++historyLoadVersion
  historyLoading.value = true
  try {
    const response = await axios.post('/api/history/query', { device_code: '', type: 2, start_time: 0, end_time: Date.now(), page: legacyPage.value, page_size: legacyPageSize.value })
    if (requestVersion !== historyLoadVersion) return
    if (response.data?.code !== 0) throw new Error(response.data?.message)
    legacyEvents.value = response.data?.data?.list || []
    legacyTotal.value = response.data?.data?.total || 0
  } catch (error) {
    if (requestVersion !== historyLoadVersion) return
    console.error('load legacy event history', error)
    requestError.value = errorMessage(error)
  } finally {
    if (requestVersion === historyLoadVersion) historyLoading.value = false
  }
}

async function refreshAll() {
  await Promise.all([loadQueue(), loadLegacyHistory(), loadEntityCatalog()])
}
async function loadEntityCatalog() {
  const [productsResult, devicesResult] = await Promise.allSettled([
    axios.get('/api/products', { params: { page: 1, pageSize: 1000 } }),
    axios.get('/api/devices', { params: { page: 1, pageSize: 1000 } })
  ])
  if (productsResult.status === 'fulfilled' && productsResult.value.data?.code === 0) catalogProducts.value = productsResult.value.data?.data || []
  if (devicesResult.status === 'fulfilled' && devicesResult.value.data?.code === 0) catalogDevices.value = devicesResult.value.data?.data || []
  policySourceEvents.value = extractPolicySourceEvents(catalogProducts.value)
}
function applyFilters() { page.value = 1; activeQuickFilters.value = []; loadQueue() }
function toggleIncludeClosed() { if (!filters.value.includeClosed && filters.value.closedToday) { filters.value.closedToday = false; if (filters.value.handling === 'closed') filters.value.handling = '' } applyFilters() }
function resetFilters() { filters.value = { search: '', severity: '', condition: '', handling: '', includeClosed: true, closedToday: false }; activeQuickFilters.value = []; applyFilters() }
function applyQuickFilter() {
  const selectedCards = statCards.value.filter((item) => activeQuickFilters.value.includes(item.key))
  const selectedValues = (field) => [...new Set(selectedCards.map((item) => item.filter[field]).filter(Boolean))]
  const conditions = selectedValues('condition')
  const handlings = selectedValues('handling')
  const hasConflict = conditions.length > 1 || handlings.length > 1
  const includesClosed = selectedCards.length === 0 || selectedCards.some((item) => item.filter.includeClosed)
  const closedToday = selectedCards.some((item) => item.filter.closedToday)

  filters.value = {
    search: filters.value.search,
    severity: filters.value.severity,
    condition: hasConflict ? '__no_matching_alarm__' : (conditions[0] || ''),
    handling: hasConflict ? '' : (handlings[0] || ''),
    includeClosed: includesClosed,
    closedToday
  }
  page.value = 1
  activeView.value = 'queue'
  loadQueue()
}
function openHistory() { activeView.value = 'history'; if (!legacyEvents.value.length && !historyLoading.value) loadLegacyHistory() }
function goQueuePage(target) { const next = Math.min(Math.max(1, Math.ceil(total.value / pageSize.value)), Math.max(1, Number(target) || 1)); if (next === page.value) return; page.value = next; loadQueue() }
function changeQueuePageSize(size) { pageSize.value = Number(size) || 10; page.value = 1; loadQueue() }
function goLegacyPage(target) { const next = Math.min(Math.max(1, Math.ceil(legacyTotal.value / legacyPageSize.value)), Math.max(1, Number(target) || 1)); if (next === legacyPage.value) return; legacyPage.value = next; loadLegacyHistory() }
function changeLegacyPageSize(size) { legacyPageSize.value = Number(size) || 10; legacyPage.value = 1; loadLegacyHistory() }

async function openDetail(item) {
  const requestVersion = ++detailLoadVersion
  if (!selected.value) detailReturnFocus.value = document.activeElement instanceof HTMLElement ? document.activeElement : null
  selected.value = item
  activeEvidenceTab.value = (hasClearedEvidence(item) && !hasAlarmEvidence(item)) ? 'cleared' : 'alarm'
  detailLoading.value = true
  selectedEvents.value = []
  focusModal(detailDialog)
  try {
    const id = alarmOf(item).public_id
    const [detailResponse, eventsResponse] = await Promise.all([axios.get(`/api/alarm-instances/${id}`), axios.get(`/api/alarm-instances/${id}/events`)])
    if (requestVersion !== detailLoadVersion) return
    if (detailResponse.data?.code === 0) {
      selected.value = detailResponse.data.data
      if (!snapshotUrl(selected.value) && clearedSnapshotUrl(selected.value)) {
        activeEvidenceTab.value = 'cleared'
      }
    }
    if (eventsResponse.data?.code === 0) selectedEvents.value = eventsResponse.data.data || []
  } catch (error) {
    if (requestVersion !== detailLoadVersion) return
    requestError.value = errorMessage(error)
  } finally {
    if (requestVersion === detailLoadVersion) detailLoading.value = false
  }
}
function closeDetail() { detailLoadVersion++; selected.value = null; selectedEvents.value = []; nextTick(() => detailReturnFocus.value?.focus?.()); detailReturnFocus.value = null }
async function acknowledge(item) { try { await axios.post(`/api/alarm-instances/${alarmOf(item).public_id}/acknowledge`, { comment: '' }); await loadQueue(); if (selected.value) await openDetail(selected.value) } catch (error) { requestError.value = errorMessage(error) } }

function captureModalFocus() { modalReturnFocus.value = document.activeElement instanceof HTMLElement ? document.activeElement : null }
function focusModal(dialog) { nextTick(() => dialog.value?.querySelector('[autofocus], input, select, textarea, button')?.focus()) }
function restoreModalFocus() { nextTick(() => modalReturnFocus.value?.focus?.()); modalReturnFocus.value = null }
function topModal() { if (legacyDetail.value) return legacyDialog.value; if (operation.value) return operationDialog.value; if (showWorkOrder.value) return workOrderDialog.value; if (showPolicies.value) return policyDialog.value; if (selected.value) return detailDialog.value; return null }
function closeTopModal() { if (legacyDetail.value) closeLegacyDetail(); else if (operation.value) closeOperation(); else if (showWorkOrder.value) closeWorkOrder(); else if (showPolicies.value) closePolicies(); else if (selected.value) closeDetail() }
function handleModalKeyboard(event) {
  const dialog = topModal()
  if (!dialog) return
  if (event.key === 'Escape') { event.preventDefault(); closeTopModal(); return }
  if (event.key !== 'Tab') return
  const focusable = Array.from(dialog.querySelectorAll('button:not([disabled]), [href], input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])'))
  if (!focusable.length) return
  const first = focusable[0]
  const last = focusable[focusable.length - 1]
  if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last.focus() }
  if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus() }
}
function openOperation(kind) { captureModalFocus(); operationError.value = ''; operation.value = kind; operationForm.value = { ownerUserID: 0, reason: '', until: '', disposition: selectedAlarm.value.condition_status === 'recovered' ? 'resolved' : 'false_positive', comment: '' }; focusModal(operationDialog) }
function closeOperation() { operation.value = ''; restoreModalFocus() }
async function submitOperation() { const id = selectedAlarm.value.public_id; operationSubmitting.value = true; operationError.value = ''; try { if (operation.value === 'assign') await axios.post(`/api/alarm-instances/${id}/assign`, { owner_user_id: operationForm.value.ownerUserID, comment: operationForm.value.comment || '' }); if (operation.value === 'shelve') await axios.post(`/api/alarm-instances/${id}/shelve`, { reason: operationForm.value.reason, until: operationForm.value.until ? new Date(operationForm.value.until).toISOString() : '' }); if (operation.value === 'close') await axios.post(`/api/alarm-instances/${id}/close`, { disposition: operationForm.value.disposition, comment: operationForm.value.comment }); closeOperation(); await loadQueue(); await openDetail(selected.value) } catch (error) { operationError.value = errorMessage(error) } finally { operationSubmitting.value = false } }
async function unshelve() { try { await axios.post(`/api/alarm-instances/${selectedAlarm.value.public_id}/unshelve`, { comment: '' }); await loadQueue(); await openDetail(selected.value) } catch (error) { requestError.value = errorMessage(error) } }

async function openWorkOrder(item) {
  captureModalFocus()
  workOrderError.value = ''
  workOrderSuccess.value = ''
  workOrderTemplate.value = null
  const alarm = alarmOf(item)
  workOrder.value = { templateId: 0, title: titleOf(item), summary: `${t('source')}: ${alarmContextText(item)}\n${t('firstSeen')}: ${formatTime(alarm.first_occurred_at)}`, priority: ({ critical: 'urgent', major: 'high', warning: 'normal', info: 'low' })[alarm.severity] || 'normal', formData: {}, alarm: item }
  showWorkOrder.value = true
  focusModal(workOrderDialog)
  try {
    const response = await axios.get('/api/work-order-templates')
    if (response.data?.code !== 0) throw new Error(response.data?.message)
    workOrderTemplates.value = response.data?.data || []
    if (workOrderTemplates.value.length) { workOrder.value.templateId = workOrderTemplates.value[0].ID; await loadWorkOrderTemplate() } else workOrderError.value = language.value === 'zh' ? '当前项目尚未配置工单模板。' : 'No work-order template is configured for this project.'
  } catch (error) { workOrderError.value = errorMessage(error) }
}
function closeWorkOrder() { showWorkOrder.value = false; restoreModalFocus() }
async function loadWorkOrderTemplate() { workOrderTemplate.value = null; workOrder.value.formData = {}; if (!workOrder.value.templateId) return; try { const response = await axios.get(`/api/work-order-templates/${workOrder.value.templateId}`); if (response.data?.code !== 0) throw new Error(response.data?.message); workOrderTemplate.value = response.data.data } catch (error) { workOrderError.value = errorMessage(error) } }
async function createWorkOrder() { const alarm = alarmOf(workOrder.value.alarm); if (!workOrder.value.templateId || !workOrder.value.title) { workOrderError.value = language.value === 'zh' ? '请选择模板并填写工单标题。' : 'Select a template and enter a work-order title.'; return } workOrderSubmitting.value = true; workOrderError.value = ''; try { const response = await axios.post('/api/work-orders/from-alarm', { template_id: workOrder.value.templateId, title: workOrder.value.title, summary: workOrder.value.summary, priority: workOrder.value.priority, alarm_instance_id: alarm.public_id, generation: alarm.generation, idempotency_key: `alarm:${alarm.public_id}`, form_data: workOrder.value.formData }); if (response.data?.code !== 0) throw new Error(response.data?.message); workOrderSuccess.value = response.data?.created ? (language.value === 'zh' ? '工单已创建并关联告警。' : 'Work order created and linked to the alarm.') : (language.value === 'zh' ? '该告警已关联既有工单。' : 'This alarm is already linked to a work order.'); await loadQueue(); if (selected.value) await openDetail(selected.value) } catch (error) { workOrderError.value = errorMessage(error) } finally { workOrderSubmitting.value = false } }
function goWorkOrder(publicID) { router.push({ path: '/work-orders', query: { highlight: publicID } }) }

async function openPolicies() {
  if (!showPolicies.value) captureModalFocus()
  showPolicies.value = true
  policyError.value = ''
  focusModal(policyDialog)
  const [policiesResult, templatesResult, productsResult] = await Promise.allSettled([
    axios.get('/api/alarm-policies'),
    axios.get('/api/work-order-templates'),
    axios.get('/api/products', { params: { page: 1, pageSize: 1000 } })
  ])
  if (policiesResult.status !== 'fulfilled' || policiesResult.value.data?.code !== 0) {
    policyError.value = errorMessage(policiesResult.status === 'fulfilled' ? new Error(policiesResult.value.data?.message) : policiesResult.reason)
    return
  }
  policies.value = policiesResult.value.data?.data || []
  policyTemplates.value = templatesResult.status === 'fulfilled' && templatesResult.value.data?.code === 0 ? templatesResult.value.data?.data || [] : []
  if (productsResult.status === 'fulfilled' && productsResult.value.data?.code === 0) catalogProducts.value = productsResult.value.data?.data || []
  policySourceEvents.value = extractPolicySourceEvents(catalogProducts.value)
}
function closePolicies() { showPolicies.value = false; editingPolicy.value = null; policyEventSearch.value = ''; policyProductGroup.value = ''; hidePolicyHelp(); restoreModalFocus() }
function policyDraftCode(policy) { return policy.code || `device-event-${Date.now()}` }
function editPolicy(policy) {
  const sourceRefs = policySourceRefs(policy)
  const matchMode = (policy.match_mode || policy.ID) ? policyMatchMode(policy) : 'events'
  const acknowledgeSLASeconds = Math.max(0, Number(policy.acknowledge_sla_seconds) || 0)
  const resolutionSLASeconds = Math.max(0, Number(policy.resolution_sla_seconds) || 0)
  editingPolicy.value = {
    ...policy,
    code: policyDraftCode(policy),
    name: policy.name || '',
    source_type: policy.source_type || 'device_event',
    source_ref: sourceRefs[0] || '',
    source_refs: sourceRefs,
    match_mode: matchMode,
    event_levels: policyLevels(policy),
    severity: policy.severity || 'warning',
    priority: policy.priority ?? 0,
    enabled: policy.enabled ?? true,
    auto_close: policy.auto_close ?? true,
    recovery_hold_seconds: policy.recovery_hold_seconds ?? 0,
    acknowledge_sla_seconds: acknowledgeSLASeconds,
    acknowledge_sla_parts: durationPartsFromSeconds(acknowledgeSLASeconds),
    acknowledge_sla_original_seconds: acknowledgeSLASeconds,
    acknowledge_sla_changed: false,
    acknowledge_sla_remainder: acknowledgeSLASeconds % 60,
    resolution_sla_seconds: resolutionSLASeconds,
    resolution_sla_parts: durationPartsFromSeconds(resolutionSLASeconds),
    resolution_sla_original_seconds: resolutionSLASeconds,
    resolution_sla_changed: false,
    resolution_sla_remainder: resolutionSLASeconds % 60,
    escalate_after_seconds: policy.escalate_after_seconds ?? 0,
    allow_exception_close: policy.allow_exception_close ?? false,
    require_work_order: policy.require_work_order ?? false,
    auto_create_work_order: policy.auto_create_work_order ?? false,
    work_order_template_id: policy.work_order_template_id ?? 0,
    work_order_mode: policyWorkOrderMode(policy)
  }
  policyError.value = ''
  policyEventSearch.value = ''
  nextTick(() => {
    const selectedOption = policyEventOptions.value.find(option => sourceRefs.includes(option.value))
    policyProductGroup.value = selectedOption?.groupKey || policyEventGroups.value[0]?.key || ''
    document.getElementById('policy-name')?.focus()
  })
}
function syncPolicyWorkOrderOptions() {
  if (!editingPolicy.value) return
  const mode = editingPolicy.value.work_order_mode || 'none'
  editingPolicy.value.require_work_order = mode !== 'none'
  editingPolicy.value.auto_create_work_order = mode === 'auto'
  if (mode !== 'auto') editingPolicy.value.work_order_template_id = 0
}
async function savePolicy() {
  try {
    if (!editingPolicy.value) return
    policyError.value = ''
    const name = String(editingPolicy.value.name || '').trim()
    if (!name) {
      policyError.value = t('policyNameRequired')
      nextTick(() => document.getElementById('policy-name')?.focus())
      return
    }
    const matchMode = policyMatchMode(editingPolicy.value)
    const sourceRefs = matchMode === 'events' ? policySourceRefs(editingPolicy.value) : []
    const eventLevels = matchMode === 'levels' ? policyLevels(editingPolicy.value) : []
    if (matchMode === 'events' && !sourceRefs.length) {
      policyError.value = t('eventSelectionRequired')
      nextTick(() => document.getElementById('policy-event-search')?.focus())
      return
    }
    if (matchMode === 'levels' && !eventLevels.length) {
      policyError.value = t('eventLevelSelectionRequired')
      return
    }
    const acknowledgeSLASeconds = policyDurationSeconds('acknowledge')
    const resolutionSLASeconds = policyDurationSeconds('resolution')
    if (acknowledgeSLASeconds === null || resolutionSLASeconds === null) {
      policyError.value = t('invalidDuration')
      return
    }
    syncPolicyWorkOrderOptions()
    if (editingPolicy.value.auto_create_work_order && !editingPolicy.value.work_order_template_id) {
      policyError.value = t('workOrderTemplateRequired')
      nextTick(() => document.getElementById('policy-work-order-template')?.focus())
      return
    }
    const payload = {
      ...editingPolicy.value,
      code: policyDraftCode(editingPolicy.value),
      name,
      match_mode: matchMode,
      source_ref: sourceRefs[0] || '',
      source_refs: sourceRefs,
      event_levels: eventLevels,
      acknowledge_sla_seconds: acknowledgeSLASeconds,
      resolution_sla_seconds: resolutionSLASeconds
    }
    for (const key of ['work_order_mode', 'all_events', 'acknowledge_sla_parts', 'acknowledge_sla_original_seconds', 'acknowledge_sla_changed', 'acknowledge_sla_remainder', 'resolution_sla_parts', 'resolution_sla_original_seconds', 'resolution_sla_changed', 'resolution_sla_remainder']) delete payload[key]
    const response = await axios.post('/api/alarm-policies', payload)
    if (response.data?.code !== 0) throw new Error(response.data?.message)
    editingPolicy.value = null
    await openPolicies()
  } catch (error) { policyError.value = errorMessage(error) }
}
async function deletePolicy(policy) {
  if (!window.confirm(formatCopy('deletePolicyConfirm', { name: policy.name }))) return
  policyError.value = ''
  try {
    const response = await axios.delete(`/api/alarm-policies/${policy.ID}`)
    if (response.data?.code !== 0) throw new Error(response.data?.message)
    if (editingPolicy.value?.ID === policy.ID) editingPolicy.value = null
    await openPolicies()
  } catch (error) { policyError.value = errorMessage(error) }
}
function openLegacyDetail(event) { captureModalFocus(); legacyDetail.value = event; focusModal(legacyDialog) }
function closeLegacyDetail() { legacyDetail.value = null; restoreModalFocus() }

onMounted(async () => {
  window.addEventListener('keydown', handleModalKeyboard)
  await refreshAll()
  try { participants.value = await loadAllWorkOrderParticipants() } catch (_) { participants.value = [] }
})
onBeforeUnmount(() => window.removeEventListener('keydown', handleModalKeyboard))
</script>

<style scoped>
.alarm-filter-toolbar,
.alarm-filter-query {
  display: flex;
  align-items: center;
  gap: 8px;
}

.alarm-filter-toolbar {
  flex-wrap: wrap;
  justify-content: space-between;
}

.alarm-filter-metrics {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.alarm-filter-query {
  flex: 0 1 480px;
  min-width: 0;
  margin-left: auto;
}

.alarm-filter-query > .input-group {
  flex: 1 1 160px;
  width: auto;
  min-width: 0;
}

.alarm-filter-query :deep(.device-filter-trigger) {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  white-space: nowrap;
}

.alarm-filter-query :deep(.btn),
.alarm-filter-query .input-group {
  min-height: 32px;
}

@media (max-width: 575.98px) {
  .alarm-filter-query {
    flex-basis: 100%;
    flex-wrap: wrap;
  }
}

.alarm-center-page {
  --alarm-surface: var(--bg-surface, var(--bs-body-bg));
  --alarm-surface-muted: var(--bs-tertiary-bg);
  --alarm-muted: var(--bs-secondary-color);
  --alarm-timeline: rgba(var(--bs-secondary-rgb), 0.28);
  --alarm-row-hover: rgba(var(--bs-primary-rgb), 0.045);
}

.cursor-pointer {
  cursor: pointer;
}

.min-w-0 {
  min-width: 0;
}

.kpi-card {
  transition: transform 0.2s ease, box-shadow 0.2s ease, border-color 0.2s ease;
  user-select: none;
}

.kpi-card:hover {
  transform: translateY(-2px);
}

.kpi-card--active {
  border-color: var(--color-primary, #3b82f6) !important;
  box-shadow: 0 0 0 2px rgba(59, 130, 246, 0.25) !important;
}

.severity-dot {
  width: 0.65rem;
  height: 0.65rem;
  border-radius: 50%;
  flex: 0 0 auto;
  background: var(--bs-secondary);
}

.severity-dot--critical,
.severity-dot--major {
  background: var(--bs-danger, #ef4444);
  box-shadow: 0 0 6px rgba(239, 68, 68, 0.5);
}

.severity-dot--warning {
  background: var(--bs-warning, #f59e0b);
  box-shadow: 0 0 6px rgba(245, 158, 11, 0.5);
}

.severity-dot--info {
  background: var(--bs-primary, #3b82f6);
  box-shadow: 0 0 6px rgba(59, 130, 246, 0.5);
}

.badge-handling {
  color: var(--bs-body-color);
  background: var(--bs-tertiary-bg);
  border: 1px solid var(--bs-border-color-translucent);
}

.alarm-table thead th {
  background: var(--bs-tertiary-bg);
  color: var(--bs-secondary-color);
  font-size: 0.8rem;
  font-weight: 600;
  letter-spacing: 0.02em;
  white-space: nowrap;
  border-bottom: 1px solid var(--bs-border-color);
}

.alarm-row {
  transition: background-color 0.15s ease;
}

.alarm-row:hover > td {
  background: var(--alarm-row-hover);
}

.alarm-drawer {
  position: fixed;
  z-index: 1051;
  top: 0;
  right: 0;
  width: min(540px, 100vw);
  height: 100dvh;
  background: var(--bg-drawer, #ffffff) !important;
  color: var(--bs-body-color);
  display: flex;
  flex-direction: column;
  border-left: 1px solid var(--border-color, var(--bs-border-color)) !important;
  box-shadow: -16px 0 48px rgba(0, 0, 0, 0.3) !important;
}

:global([data-bs-theme='dark']) .alarm-drawer {
  background: var(--bg-drawer, #171a20) !important;
}

.alarm-drawer__head {
  padding: 1.25rem;
  background: var(--bg-drawer-header, var(--bs-tertiary-bg)) !important;
  border-bottom: 1px solid var(--bs-border-color) !important;
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 1rem;
}

.alarm-drawer__body {
  padding: 1.25rem;
  overflow-y: auto;
}

.alarm-detail-summary {
  background: var(--bg-drawer-section, var(--bs-tertiary-bg)) !important;
}

.section-title {
  font-size: 0.85rem;
  font-weight: 700;
  color: var(--bs-secondary-color);
}

.timeline {
  border-left: 2px solid var(--alarm-timeline);
  list-style: none;
  margin: 0 0 0 0.4rem;
  padding: 0 0 0 1rem;
}

.timeline li {
  position: relative;
  padding-bottom: 0.85rem;
  font-size: 0.85rem;
}

.timeline li::before {
  background: var(--bs-primary);
  border-radius: 50%;
  content: '';
  height: 0.5rem;
  left: -1.32rem;
  position: absolute;
  top: 0.28rem;
  width: 0.5rem;
}

.timeline time {
  display: block;
  color: var(--bs-secondary-color);
  font-size: 0.76rem;
}

.modal-backdrop-layer {
  background: rgba(0, 0, 0, 0.55);
  z-index: 1060;
}

.modal-backdrop-layer .modal {
  z-index: 1061;
}

.alarm-more-actions summary {
  list-style: none;
}

.alarm-more-actions summary::-webkit-details-marker {
  display: none;
}

.alarm-more-actions__menu {
  background: var(--bs-body-bg);
}

.alarm-history-help {
  position: relative;
  display: inline-flex;
}

.alarm-history-help__trigger {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 1.5rem;
  height: 1.5rem;
  padding: 0;
  border: 0;
  border-radius: 50%;
  background: transparent;
  color: var(--bs-secondary-color);
}

.alarm-history-help__trigger:hover {
  color: var(--bs-primary);
  background: var(--bs-tertiary-bg);
}

.alarm-history-help__popover {
  position: absolute;
  z-index: 10;
  top: calc(100% + 0.4rem);
  left: 0;
  width: min(22rem, calc(100vw - 2rem));
  padding: 0.65rem 0.75rem;
  border: 1px solid var(--bs-border-color);
  border-radius: 0.5rem;
  background: var(--bs-body-bg);
  box-shadow: 0 0.5rem 1.25rem rgba(0, 0, 0, 0.2);
  color: var(--bs-body-color);
  font-size: 0.78rem;
  line-height: 1.45;
  opacity: 0;
  pointer-events: none;
  transform: translateY(-0.2rem);
  visibility: hidden;
  transition: opacity 0.15s ease, transform 0.15s ease, visibility 0.15s ease;
}

.alarm-history-help:hover .alarm-history-help__popover,
.alarm-history-help:focus-within .alarm-history-help__popover {
  opacity: 1;
  transform: translateY(0);
  visibility: visible;
}

.policy-help {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 1.1rem;
  height: 1.1rem;
  margin-left: 0.35rem;
  padding: 0;
  border: 1px solid var(--bs-secondary-color);
  border-radius: 50%;
  background: transparent;
  color: var(--bs-secondary-color);
  font-size: 0.72rem;
  font-weight: 700;
}

.policy-help-popover {
  position: fixed;
  z-index: 20000;
  width: min(21rem, calc(100vw - 1.5rem));
  padding: 0.65rem 0.75rem;
  border: 1px solid var(--bs-border-color);
  border-radius: 0.45rem;
  background: var(--bs-body-bg);
  box-shadow: 0 0.65rem 1.5rem rgba(0, 0, 0, 0.3);
  color: var(--bs-body-color);
  font-size: 0.8rem;
  line-height: 1.5;
  pointer-events: none;
}

.policy-event-picker {
  overflow: hidden;
  border: 1px solid var(--bs-border-color);
  border-radius: 0.5rem;
  background: var(--bs-body-bg);
}

.policy-event-search {
  padding: 0.5rem;
  border-block: 1px solid var(--bs-border-color-translucent);
}

.policy-event-browser {
  padding: 0.65rem;
  border-bottom: 1px solid var(--bs-border-color-translucent);
  background: var(--bs-tertiary-bg);
}

.policy-selected-events {
  display: flex;
  flex-wrap: wrap;
  gap: 0.35rem;
  padding: 0.55rem 0.65rem;
  border-bottom: 1px solid var(--bs-border-color-translucent);
}

.policy-event-options {
  max-height: 13rem;
  overflow-y: auto;
}

.policy-event-option {
  display: flex;
  align-items: flex-start;
  gap: 0.55rem;
  margin: 0;
  padding: 0.48rem 0.65rem;
  cursor: pointer;
  font-size: 0.82rem;
  line-height: 1.35;
}

.policy-event-option:hover {
  background: rgba(var(--bs-primary-rgb), 0.07);
}

.policy-event-option--all {
  background: var(--bs-tertiary-bg);
  font-weight: 650;
}

.policy-duration {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 0.4rem;
}

.policy-table thead th {
  background: var(--bs-tertiary-bg);
  color: var(--bs-secondary-color);
  font-size: 0.78rem;
  white-space: nowrap;
}

.legacy-snapshot-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(10rem, 1fr));
  gap: 0.75rem;
}

.legacy-snapshot-grid img {
  display: block;
  width: 100%;
  max-height: 22rem;
  object-fit: contain;
  background: var(--bs-tertiary-bg);
}

@media (max-width: 767.98px) {
  .alarm-drawer {
    width: 100vw;
  }
}
</style>
