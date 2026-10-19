<template>
  <div class="device-map-page page-fixed-height d-flex flex-column h-100">
    <div class="page-header">
      <div>
        <h1>{{ $t('sidebar_device_map') }}</h1>
        <p class="page-subtitle">{{ $t('device_map_subtitle') }}</p>
      </div>
      <div class="d-flex align-items-center gap-2">
        <div class="basemap-switch" ref="basemapSwitchRef">
          <LiquidGlassButton variant="secondary" icon="bi bi-layers" :aria-expanded="basemapMenuOpen" aria-controls="basemap-menu" @click="basemapMenuOpen = !basemapMenuOpen">
            {{ activeProviderLabel }}
          </LiquidGlassButton>
          <div v-if="basemapMenuOpen" id="basemap-menu" class="basemap-menu">
            <button
              v-for="provider in BASEMAP_PROVIDERS"
              :key="provider.id"
              type="button"
              class="basemap-menu-item"
              :class="{ active: provider.id === basemapId }"
              @click="selectBasemap(provider.id)"
            >
              <span class="basemap-menu-title">{{ locale === 'en' ? provider.label.en : provider.label.zh }}</span>
              <span class="basemap-menu-hint">{{ locale === 'en' ? provider.hint.en : provider.hint.zh }}</span>
            </button>
            <form class="basemap-token" @submit.prevent="applyTiandituToken">
              <label for="tianditu-key" class="basemap-token-label">{{ $t('device_map_tianditu_token') }}</label>
              <input
                id="tianditu-key"
                v-model="tiandituTokenDraft"
                type="text"
                class="form-control form-control-sm"
                autocomplete="off"
                spellcheck="false"
                aria-describedby="tianditu-key-help"
                :placeholder="$t('device_map_tianditu_token_placeholder')"
              />
              <p id="tianditu-key-help" class="basemap-token-help">{{ $t('device_map_tianditu_token_help') }}</p>
              <button type="submit" class="btn btn-sm btn-primary w-100 mt-2">
                {{ $t('device_map_apply_token') }}
              </button>
            </form>
          </div>
        </div>
        <LiquidGlassButton
          variant="secondary"
          icon="bi bi-crosshair"
          :disabled="!locatedDevices.length"
          @click="fitAllDevices"
        >
          {{ $t('device_map_zoom_to_fit') }}
        </LiquidGlassButton>
        <div v-permission="'geofence:create'" class="fence-create" ref="fenceCreateRef">
          <LiquidGlassButton variant="primary" icon="bi bi-plus-lg" @click="fenceMenuOpen = !fenceMenuOpen">
            {{ $t('device_map_new_fence') }}
            <i class="bi bi-chevron-down ms-1"></i>
          </LiquidGlassButton>
          <div v-if="fenceMenuOpen" class="fence-create-menu">
            <button type="button" class="fence-create-item" @click="startFenceDrawing">
              <i class="bi bi-pencil-square"></i>
              <span>
                <strong>{{ $t('device_map_fence_draw_polygon') }}</strong>
                <small>{{ $t('device_map_fence_draw_polygon_hint') }}</small>
              </span>
            </button>
            <button type="button" class="fence-create-item" @click="openRegionDialog">
              <i class="bi bi-geo"></i>
              <span>
                <strong>{{ $t('device_map_fence_by_region') }}</strong>
                <small>{{ $t('device_map_fence_by_region_hint') }}</small>
              </span>
            </button>
          </div>
        </div>
      </div>
    </div>

    <div class="device-map-body flex-grow-1">
      <div class="map-surface">
        <div ref="mapContainer" class="map-canvas"></div>

        <!--
          地点搜索。放在地图左上角而不是顶栏：搜的是"地图上的位置"，
          视线不该离开地图。结果面板用绝对定位，不挤压画布。
        -->
        <div ref="mapSearchRef" class="map-search">
          <div class="map-search-box">
            <i class="bi bi-search"></i>
            <input
              v-model="placeQuery"
              type="text"
              class="map-search-input"
              :placeholder="$t('device_map_search_placeholder')"
              autocomplete="off"
              @keydown.enter="runPlaceSearch"
              @focus="placePanelOpen = placeResults.length > 0 || !!placeError"
            />
            <span v-if="placeSearching" class="spinner-border spinner-border-sm text-secondary"></span>
            <button v-else-if="placeQuery" type="button" class="map-search-clear" @click="clearPlaceSearch">
              <i class="bi bi-x-lg"></i>
            </button>
          </div>
          <div v-if="placePanelOpen" class="map-search-results">
            <div v-if="placeSearching" class="map-search-hint">{{ $t('device_map_searching') }}</div>
            <div v-else-if="placeError" class="map-search-hint text-danger">{{ placeError }}</div>
            <button
              v-for="(place, index) in placeResults"
              :key="`${place.name}-${index}`"
              type="button"
              class="map-search-item"
              @click="goToPlace(place)"
            >
              <i class="bi" :class="place.boundary ? 'bi-geo-alt' : 'bi-pin-map'"></i>
              <span class="map-search-item-text">
                <span class="map-search-item-name">{{ place.name }}</span>
                <span class="map-search-item-address">{{ place.address }}</span>
              </span>
              <span class="map-search-item-kind">{{ $t(`device_map_place_kind_${place.kind}`) }}</span>
            </button>
          </div>
          <div v-if="placeHighlight && !placePanelOpen" class="map-search-picked">
            <i class="bi bi-geo-alt-fill"></i>
            <span>{{ placeHighlight.name }}</span>
            <button type="button" class="map-search-clear" @click="placeHighlight = null; renderPlaceMarker()">
              <i class="bi bi-x-lg"></i>
            </button>
          </div>
        </div>

        <!-- 瓦片失败：底图挂了，但设备点与围栏仍在 -->
        <div v-if="tileError" class="map-banner map-banner-warning">
          <i class="bi bi-exclamation-triangle-fill"></i>
          <span>{{ $t('device_map_tile_failed') }}</span>
          <button type="button" class="btn btn-sm btn-outline-warning ms-2" @click="retryBasemap">
            {{ $t('device_map_retry') }}
          </button>
        </div>

        <!--
          绘制提示条。围栏只剩「自由绘制多边形」一种模式（圆形已移除），
          手动标注有它自己的一条（下面）。
        -->
        <div v-if="drawing.mode === 'fence-polygon'" class="map-banner map-banner-active">
          <i class="bi" :class="drawing.finished ? 'bi-check2-circle' : 'bi-pencil-square'"></i>
          <span class="map-banner-text">{{ drawingHint }}</span>
          <span class="drawing-readout ms-2">{{ drawing.points.length }} {{ $t('device_map_points') }}</span>

          <!-- 绘制中：请用户先"定稿"；定稿后才出现「下一步」。
               两步分开是刻意的 —— 右键之后范围还留在图上，用户可以先看清再走。 -->
          <button
            v-if="!drawing.finished"
            type="button"
            class="btn btn-sm btn-primary ms-2"
            :disabled="drawing.points.length < 3"
            @click="finishFenceDrawing"
          >
            {{ $t('device_map_finish_draw') }}
          </button>
          <template v-else>
            <button type="button" class="btn btn-sm btn-primary ms-2" @click="proceedToFenceForm">
              {{ $t('device_map_next') }}
            </button>
            <button type="button" class="btn btn-sm btn-outline-secondary ms-2" @click="resumeFenceDrawing">
              {{ $t('device_map_continue_draw') }}
            </button>
          </template>

          <button
            v-if="!drawing.finished && drawing.points.length"
            type="button"
            class="btn btn-sm btn-outline-secondary ms-2"
            @click="undoDrawingPoint"
          >
            {{ $t('device_map_undo_point') }}
          </button>
          <button type="button" class="btn btn-sm btn-outline-secondary ms-2" @click="cancelDrawing">
            {{ $t('cancel') }}
          </button>
        </div>

        <!-- 手动标注提示条 -->
        <div v-if="drawing.mode === 'device-location'" class="map-banner map-banner-active">
          <i class="bi bi-geo-alt-fill"></i>
          <span>{{ $t('device_map_pick_location_hint', { name: drawing.deviceName || '' }) }}</span>
          <button type="button" class="btn btn-sm btn-outline-secondary ms-2" @click="cancelDrawing">
            {{ $t('cancel') }}
          </button>
        </div>

        <!-- 地图控件 -->
        <div class="map-controls">
          <button type="button" class="map-control" :title="$t('device_map_zoom_in')" @click="zoomBy(1)">
            <i class="bi bi-plus-lg"></i>
          </button>
          <button type="button" class="map-control" :title="$t('device_map_zoom_out')" @click="zoomBy(-1)">
            <i class="bi bi-dash-lg"></i>
          </button>
          <button type="button" class="map-control" :title="$t('device_map_reset')" @click="fitAllDevices">
            <i class="bi bi-crosshair"></i>
          </button>
          <button
            v-permission="'geofence:create'"
            type="button"
            class="map-control"
            :class="{ active: drawing.mode === 'fence-polygon' }"
            :title="$t('device_map_draw_fence')"
            @click="startFenceDrawing"
          >
            <i class="bi bi-bounding-box"></i>
          </button>
        </div>

        <!--
          图层与视角。这两个是**地图交互类**控件，放在地图右下角、尺寸与缩放按钮一致：
          顶栏只留页面级动作（换底图 / 全览 / 新建围栏），视线不必在顶栏与画布之间来回跳。
          菜单向上展开 —— 下方就是版权条，向下没有空间。
        -->
        <div class="map-toolbox" ref="layerPanelRef">
          <div class="layer-switch">
            <button
              type="button"
              class="map-control"
              :class="{ active: layerPanelOpen }"
              :title="$t('device_map_layers')"
              :aria-label="$t('device_map_layers')"
              :aria-expanded="layerPanelOpen"
              aria-controls="layer-menu"
              @click="toggleLayerPanel"
            >
              <i class="bi bi-sliders2" aria-hidden="true"></i>
            </button>
            <div v-if="layerPanelOpen" id="layer-menu" class="layer-menu">
              <p class="layer-group-title">{{ $t('device_map_layers_base') }}</p>
              <div v-for="group in baseElementGroups" :key="group.id" class="form-check form-switch layer-row">
                <input
                  :id="`layer-${group.id}`"
                  class="form-check-input"
                  type="checkbox"
                  role="switch"
                  :checked="elementPrefs[group.id] !== false"
                  @change="toggleElement(group.id)"
                />
                <label class="form-check-label" :for="`layer-${group.id}`">{{ $t(group.key) }}</label>
              </div>
              <p v-if="!vectorElements" class="layer-hint">{{ $t('device_map_layer_raster_hint') }}</p>
              <p class="layer-group-title">{{ $t('device_map_layers_business') }}</p>
              <div v-for="group in BUSINESS_LAYER_GROUPS" :key="group.id" class="form-check form-switch layer-row">
                <input
                  :id="`layer-${group.id}`"
                  class="form-check-input"
                  type="checkbox"
                  role="switch"
                  :checked="elementPrefs[group.id] !== false"
                  @change="toggleElement(group.id)"
                />
                <label class="form-check-label" :for="`layer-${group.id}`">{{ $t(group.key) }}</label>
              </div>
            </div>
          </div>
          <!--
            状态脉冲高亮。与「图层」并列而不是塞进图层菜单里：
            它改的是**显示强调**，不是图层可见性，语义不同；混在一起会让用户以为
            关掉某项就等于"不显示该状态设备"。
          -->
          <div class="pulse-switch" ref="pulsePanelRef">
            <button
              type="button"
              class="map-control"
              :class="{ active: pulseStatuses.length > 0 }"
              :title="$t('device_map_pulse')"
              :aria-label="$t('device_map_pulse')"
              :aria-expanded="pulsePanelOpen"
              aria-controls="pulse-menu"
              @click="togglePulsePanel"
            >
              <i class="bi bi-broadcast" aria-hidden="true"></i>
            </button>
            <div v-if="pulsePanelOpen" id="pulse-menu" class="layer-menu pulse-menu">
              <p class="layer-group-title">{{ $t('device_map_pulse_title') }}</p>
              <div v-for="status in DEVICE_STATUS_ORDER" :key="status" class="form-check layer-row pulse-row">
                <input
                  :id="`pulse-${status}`"
                  class="form-check-input"
                  type="checkbox"
                  :checked="pulseStatuses.includes(status)"
                  @change="togglePulseStatus(status)"
                />
                <span class="pulse-dot" :style="{ background: deviceStatusColor(status) }" aria-hidden="true"></span>
                <label class="form-check-label" :for="`pulse-${status}`">{{ $t(deviceStatusLabelKey(status)) }}</label>
              </div>
              <div class="pulse-actions">
                <button type="button" class="pulse-action" @click="setPulseStatuses(DEVICE_STATUS_ORDER)">
                  {{ $t('device_map_pulse_select_all') }}
                </button>
                <button type="button" class="pulse-action" @click="setPulseStatuses([])">
                  {{ $t('device_map_pulse_clear') }}
                </button>
              </div>
              <p class="layer-hint">
                {{ pulseReducedMotion ? $t('device_map_pulse_reduced') : $t('device_map_pulse_hint') }}
              </p>
            </div>
          </div>
          <div class="map-dimension" role="group" :aria-label="$t('device_map_dimension')">
            <button
              type="button"
              class="map-control map-control-text"
              :class="{ active: !is3D }"
              :title="$t('device_map_view_2d')"
              :aria-pressed="!is3D"
              @click="setMapDimension('2d')"
            >
              2D
            </button>
            <button
              type="button"
              class="map-control map-control-text"
              :class="{ active: is3D }"
              :title="$t('device_map_3d_hint')"
              :aria-pressed="is3D"
              @click="setMapDimension('3d')"
            >
              3D
            </button>
          </div>
        </div>

        <!-- 图例 -->
        <div class="map-legend">
          <div v-for="status in DEVICE_STATUS_ORDER" :key="status" class="map-legend-item">
            <i
              class="bi"
              :class="{ online: 'bi-check-circle-fill', warning: 'bi-exclamation-circle-fill', alarm: 'bi-exclamation-square-fill', offline: 'bi-dash-circle-fill' }[status]"
              :style="{ color: deviceStatusColor(status, isDark) }"
              aria-hidden="true"
            ></i>
            <span>{{ $t(deviceStatusLabelKey(status)) }}</span>
          </div>
        </div>

        <!-- 版权与坐标读数 -->
        <div class="map-attribution">
          <span>{{ activeProvider?.attribution || '' }}</span>
          <span class="map-cursor-readout">{{ cursorReadout }}</span>
        </div>

        <!--
          围栏悬停标签。只读：填色块上悬停就能知道"这是哪条围栏"，
          不必回到右侧列表逐条比对。pointer-events:none 让光标始终落在地图上，
          否则标签自己会持续触发 mouseleave/mousemove，出现闪烁。
        -->
        <div v-if="hoverFence" class="fence-hover-label" :style="hoverFenceStyle">
          <span class="fence-hover-name">{{ hoverFence.name }}</span>
          <span class="fence-hover-meta">
            {{ $t(`device_map_rule_${hoverFence.rule_type}`) }}<template v-if="hoverFence.scope"> · {{ hoverFence.scope }}</template>
          </span>
        </div>

        <!--
          设备悬停卡。卡片可交互（视频点播 / 移动图钉）⇒ 移出圆点后延迟关闭，
          光标进入卡片时取消关闭，否则按钮永远点不到。
        -->
        <div
          v-if="hoverDevice"
          ref="hoverCardRef"
          class="device-hover-card"
          :style="hoverCardStyle"
          @mouseenter="cancelHoverClose"
          @mouseleave="scheduleHoverClose"
        >
          <div class="d-flex align-items-center justify-content-between gap-2">
            <span class="device-hover-name">{{ hoverDevice.device_name || hoverDevice.device_code }}</span>
            <span class="device-hover-status" :style="{ color: deviceStatusColor(hoverStatus, isDark) }">
              {{ $t(deviceStatusLabelKey(hoverStatus)) }}
            </span>
          </div>
          <div class="device-hover-code">{{ hoverDevice.device_code }}</div>

          <!--
            摄像机点播入口。判据完全来自协议插件注册的 deviceActions.condition，
            页面不认识任何具体协议名 —— 换成别的视频协议插件，这里自动生效。
          -->
          <button v-if="canPlayVideo(hoverDevice)" type="button" class="device-hover-video" @click="playDeviceVideo(hoverDevice)">
            <i class="bi bi-camera-video-fill" aria-hidden="true"></i>
            <span>{{ $t('device_map_hover_video') }}</span>
          </button>

          <div class="device-hover-divider"></div>

          <div class="device-hover-section-head">
            <span>{{ $t('device_map_hover_realtime') }}</span>
            <span v-if="hoverSnapshotAt" class="device-hover-fresh font-mono">{{ formatTime(hoverSnapshotAt) }}</span>
          </div>
          <div v-if="hoverPropertiesLoading" class="device-hover-props">
            <span v-for="n in 4" :key="n" class="device-hover-prop-skeleton"></span>
          </div>
          <div v-else-if="hoverProperties.length" class="device-hover-props">
            <div v-for="prop in hoverProperties.slice(0, HOVER_PROPERTY_LIMIT)" :key="prop.key" class="device-hover-prop">
              <span class="device-hover-prop-name" :title="prop.name">{{ prop.name }}</span>
              <span class="device-hover-prop-value font-mono" :class="{ 'is-empty': !prop.hasValue }">
                {{ formatPropertyValue(prop.value) }}<span v-if="prop.unit" class="device-hover-prop-unit">{{ prop.unit }}</span>
              </span>
            </div>
          </div>
          <p v-else class="device-hover-empty">
            <i class="bi bi-activity" aria-hidden="true"></i>
            <span>{{ hoverProbeMessage }}</span>
          </p>
          <p v-if="hoverProperties.length > HOVER_PROPERTY_LIMIT" class="device-hover-more">
            {{ $t('device_map_hover_more', { count: hoverProperties.length - HOVER_PROPERTY_LIMIT }) }}
          </p>

          <div class="device-hover-divider"></div>
          <div class="device-hover-meta">
            <span>{{ $t('device_map_meta_coords') }}</span>
            <span>{{ formatCoord(hoverDevice.longitude, hoverDevice.latitude) }}</span>
          </div>
          <div class="device-hover-meta">
            <span>{{ $t('device_map_meta_source') }}</span>
            <span>{{ hoverDevice.source === 'telemetry' ? $t('device_map_source_telemetry') : $t('device_map_source_manual') }}</span>
          </div>
          <div class="device-hover-meta">
            <span>{{ $t('device_map_meta_last_fix') }}</span>
            <span>{{ formatTime(hoverDevice.fix_at) }}</span>
          </div>
          <button
            v-if="hoverDevice.source === 'manual'"
            v-permission="'device_location:edit'"
            type="button"
            class="btn btn-sm btn-outline-primary w-100 mt-2"
            @click="startDeviceLocationPick(hoverDevice)"
          >
            {{ $t('device_map_move_pin') }}
          </button>
        </div>

        <!--
          空状态：一台设备都没有绑定经纬度。
          必须是一张**居中卡片**而不是整面蒙版 —— 地图仍然是可用的：
          手动标注要先在地图上点选位置，绘制围栏也要点地图。
          蒙版一旦铺满，既糊掉底图又吃掉全部点击，还会盖住绘制提示条。
          drawing.mode 非空时直接让位，避免与"请在地图上点选"冲突。

          卡片必须可关闭：它压在视野正中，而这一页在没有任何设备时依然要能
          自由浏览、搜索地点、画围栏。关掉的记忆按会话保存（sessionStorage），
          设备重新有位置后自动失效 —— 下次再变空时提示会回来。
        -->
        <div v-if="showEmptyState" class="map-empty-state">
          <button
            type="button"
            class="map-empty-close"
            :aria-label="$t('device_map_empty_dismiss')"
            :title="$t('device_map_empty_dismiss')"
            @click="dismissEmptyState"
          >
            <i class="bi bi-x-lg" aria-hidden="true"></i>
          </button>
          <svg viewBox="0 0 48 48" class="map-empty-icon" aria-hidden="true">
            <path
              d="M24 6c-6.6 0-12 5.4-12 12 0 9 12 24 12 24s12-15 12-24c0-6.6-5.4-12-12-12zm0 16.4a4.4 4.4 0 1 1 0-8.8 4.4 4.4 0 0 1 0 8.8z"
              fill="currentColor"
            />
          </svg>
          <h3>{{ $t('device_map_empty_title') }}</h3>
          <p>{{ $t('device_map_empty_desc') }}</p>
          <LiquidGlassButton variant="secondary" icon="bi bi-cpu" @click="goToDevices">
            {{ $t('device_map_empty_action') }}
          </LiquidGlassButton>
        </div>

        <!-- 加载态 -->
        <div v-if="loading" class="map-loading">
          <div class="spinner-border text-primary" role="status"></div>
        </div>
      </div>

      <aside class="map-panel">
        <div class="map-panel-header">
          <h6 class="mb-0 fw-bold">{{ $t('device_map_panel_title') }}</h6>
          <span class="map-panel-count">{{ devices.length }}</span>
        </div>

        <div class="map-panel-tabs">
          <button
            type="button"
            class="map-panel-tab"
            :class="{ active: panelTab === 'devices' }"
            @click="panelTab = 'devices'"
          >
            {{ $t('device_map_tab_devices') }} {{ devices.length }}
          </button>
          <button
            type="button"
            class="map-panel-tab"
            :class="{ active: panelTab === 'fences' }"
            @click="panelTab = 'fences'"
          >
            {{ $t('device_map_tab_fences') }} {{ fences.length }}
          </button>
        </div>

        <!--
          这里曾经有一块「当前地理围栏告警」。已移除：告警统一由告警中心（/alarms）管理，
          同一份违规信息在两处出现，会在处置节奏上互相干扰 —— 页面内的区块无法确认/关闭，
          却又是最抢眼的一块，反而把"去告警中心处理"的路径挡住了。
          ⚠️ 设备点位颜色仍按围栏判定着色（在线/预警/告警），那是空间视图的状态编码，
          不是告警条目，不随本次移除。
        -->

        <template v-if="panelTab === 'devices'">
          <div class="map-panel-search">
            <input v-model="search" type="search" class="form-control form-control-sm" :placeholder="$t('search')" />
          </div>
          <div class="map-panel-filter">
            <button
              v-for="status in DEVICE_STATUS_ORDER"
              :key="status"
              type="button"
              class="map-filter-chip"
              :class="{ active: statusFilter === status }"
              :style="statusFilter === status ? { borderColor: deviceStatusColor(status, isDark) } : {}"
              @click="toggleStatusFilter(status)"
            >
              <span class="map-filter-dot" :style="{ backgroundColor: deviceStatusColor(status, isDark) }"></span>
              {{ $t(deviceStatusLabelKey(status)) }}
            </button>
          </div>

          <div class="map-panel-scroll">
            <div v-if="filteredDevices.length === 0" class="map-panel-empty">
              {{ search ? $t('device_map_no_match') : $t('device_map_no_devices') }}
            </div>
            <div
              v-for="device in filteredDevices"
              :key="device.device_code"
              class="map-device-row"
              :class="{ active: selectedCode === device.device_code }"
              role="button"
              tabindex="0"
              @click="focusDevice(device)"
              @keydown.enter="focusDevice(device)"
            >
              <span
                class="map-device-dot"
                :style="{ backgroundColor: deviceStatusColor(deviceDisplayStatus(device), isDark) }"
              ></span>
              <span class="map-device-info">
                <span class="map-device-name">{{ device.device_name || device.device_code }}</span>
                <span class="map-device-sub">
                  <template v-if="device.has_location">
                    {{ formatCoord(device.longitude, device.latitude) }} ·
                    {{ device.source === 'telemetry' ? $t('device_map_source_telemetry') : $t('device_map_source_manual') }}
                    <template v-if="isStaleLocation(device.fix_at)"> · {{ $t('device_map_stale') }}</template>
                  </template>
                  <template v-else>{{ $t('device_map_no_location') }}</template>
                </span>
              </span>
              <span
                v-if="device.mode === 'telemetry'"
                class="badge text-bg-light"
                :title="$t('device_map_telemetry_locked')"
              >
                <i class="bi bi-broadcast"></i>
              </span>
              <span v-else class="map-device-actions">
                <button
                  v-permission="'device_location:edit'"
                  type="button"
                  class="btn btn-sm btn-link p-0"
                  :title="device.has_location ? $t('device_map_move_pin') : $t('device_map_set_pin')"
                  @click.stop="startDeviceLocationPick(device)"
                >
                  <i class="bi" :class="device.has_location ? 'bi-geo-alt' : 'bi-plus-circle'"></i>
                </button>
                <button
                  v-if="device.has_location"
                  v-permission="'device_location:edit'"
                  type="button"
                  class="btn btn-sm btn-link p-0 text-danger"
                  :title="$t('device_map_clear_pin')"
                  @click.stop="clearLocationFor(device)"
                >
                  <i class="bi bi-x-circle"></i>
                </button>
              </span>
            </div>
          </div>
        </template>

        <template v-else>
          <div class="map-panel-scroll">
            <div v-if="fences.length === 0" class="map-panel-empty">{{ $t('device_map_no_fences') }}</div>
            <div
              v-for="fence in fences"
              :key="fence.ID"
              class="map-fence-row"
              :class="{ active: isFenceSelected(fence) }"
              role="button"
              tabindex="0"
              @click="focusFence(fence)"
              @keydown.enter="focusFence(fence)"
            >
              <span class="map-fence-swatch" :style="{ backgroundColor: fenceAccent(fence.rule_type, isDark) }"></span>
              <span class="map-device-info">
                <span class="map-device-name">{{ fence.name }}</span>
                <span class="map-device-sub">
                  <template v-if="fence.source === 'region'">
                    {{ $t('device_map_fence_region') }} · {{ fence.region_name }}
                  </template>
                  <template v-else-if="fence.kind === 'circle'">{{ $t('device_map_fence_legacy_circle') }}</template>
                  <template v-else>{{ $t('device_map_fence_polygon') }}</template>
                  · {{ $t(`device_map_rule_${fence.rule_type}`) }}
                  · {{ fenceScopeLabel(fence) }}
                  <template v-if="!fence.enabled"> · {{ $t('device_map_fence_disabled') }}</template>
                </span>
                <!-- 「N 台在内」既是数量也是入口：点它看围栏设备清单。
                     比再塞第三个图标按钮更好点，也不用为它单独占一行。 -->
                <button type="button" class="map-fence-count" @click.stop="openFenceDevices(fence)">
                  <i class="bi bi-box-seam"></i>
                  {{ $t('device_map_fence_inside_count', { count: fenceInsideCount(fence.ID) }) }}
                  <i class="bi bi-chevron-right"></i>
                </button>
              </span>
              <span class="map-fence-actions">
                <button
                  v-permission="'geofence:edit'"
                  type="button"
                  class="btn btn-sm btn-link p-0"
                  @click.stop="openFenceEdit(fence)"
                >
                  <i class="bi bi-pencil"></i>
                </button>
                <button
                  v-permission="'geofence:delete'"
                  type="button"
                  class="btn btn-sm btn-link p-0 text-danger"
                  @click.stop="confirmDeleteFence(fence)"
                >
                  <i class="bi bi-trash"></i>
                </button>
              </span>
            </div>
          </div>
        </template>
      </aside>
    </div>

    <!-- 围栏表单 -->
    <Teleport to="body">
      <div v-if="fenceForm.open" class="modal fade show d-block" tabindex="-1" role="dialog" aria-modal="true">
        <div class="modal-dialog modal-dialog-centered">
          <div class="modal-content">
            <div class="modal-header">
              <h5 class="modal-title">
                {{ fenceForm.id ? $t('device_map_fence_edit') : $t('device_map_fence_create') }}
              </h5>
              <button type="button" class="btn-close" @click="closeFenceForm"></button>
            </div>
            <div class="modal-body">
              <div class="mb-3">
                <label class="form-label">{{ $t('device_map_fence_name') }}</label>
                <input v-model="fenceForm.name" type="text" class="form-control" :placeholder="$t('device_map_fence_name_placeholder')" />
              </div>
              <div class="mb-3">
                <label class="form-label">{{ $t('device_map_fence_rule') }}</label>
                <select v-model="fenceForm.rule_type" class="form-select">
                  <option value="forbid">{{ $t('device_map_rule_forbid') }}</option>
                  <option value="keep_in">{{ $t('device_map_rule_keep_in') }}</option>
                  <option value="notify">{{ $t('device_map_rule_notify') }}</option>
                </select>
                <p class="form-text mb-0">{{ $t('device_map_fence_rule_hint') }}</p>
              </div>
              <div class="row g-2 mb-3">
                <div class="col-6">
                  <label class="form-label">{{ $t('device_map_fence_enter_dwell') }}</label>
                  <input v-model.number="fenceForm.enter_dwell_s" type="number" min="0" class="form-control" />
                </div>
                <div class="col-6">
                  <label class="form-label">{{ $t('device_map_fence_exit_dwell') }}</label>
                  <input v-model.number="fenceForm.exit_dwell_s" type="number" min="0" class="form-control" />
                </div>
              </div>
              <p class="text-muted small mb-3">{{ $t('device_map_fence_dwell_hint') }}</p>

              <!-- 范围来源：手绘 / 地区。地区围栏不能拖顶点，只能重新选地区。 -->
              <div class="fence-summary">
                <template v-if="fenceForm.kind === 'circle'">
                  <span>{{ $t('device_map_fence_legacy_circle') }} · {{ Math.round(fenceForm.radius_m || 0) }} m</span>
                  <button type="button" class="btn btn-sm btn-outline-primary ms-auto" @click="convertCircleToPolygon">
                    {{ $t('device_map_fence_convert_polygon') }}
                  </button>
                </template>
                <template v-else-if="fenceForm.source === 'region'">
                  <span>
                    {{ $t('device_map_fence_region') }} · {{ fenceForm.region_name }}
                    · {{ $t('device_map_fence_region_stats', { polygons: fenceFormShapeSummary.polygons, points: fenceFormShapeSummary.vertices }) }}
                  </span>
                  <button type="button" class="btn btn-sm btn-outline-primary ms-auto" @click="openRegionDialogForForm">
                    {{ $t('device_map_fence_reselect_region') }}
                  </button>
                </template>
                <template v-else>
                  <span>{{ $t('device_map_fence_polygon_summary', { count: fenceRingVertexCount }) }}</span>
                </template>
              </div>

              <!-- 适用设备：不选 = 对全部已定位设备生效 -->
              <div class="mt-3">
                <label class="form-label">
                  {{ $t('device_map_fence_devices') }}
                  <span class="text-muted fw-normal">· {{ fenceScopeLabel({ assigned_devices: fenceForm.device_codes }) }}</span>
                </label>
                <input
                  v-model="fenceForm.deviceSearch"
                  type="search"
                  class="form-control form-control-sm mb-2"
                  :placeholder="$t('device_map_fence_devices_search')"
                />
                <div class="fence-device-picker">
                  <div v-if="fenceDeviceCandidates.length === 0" class="fence-device-empty">
                    {{ $t('device_map_fence_devices_empty') }}
                  </div>
                  <label v-for="device in fenceDeviceCandidates" :key="device.device_code" class="fence-device-item">
                    <input
                      type="checkbox"
                      class="form-check-input"
                      :checked="selectedFenceDeviceCodes.has(device.device_code)"
                      @change="toggleFenceDevice(device.device_code)"
                    />
                    <span class="fence-device-name">{{ device.device_name || device.device_code }}</span>
                    <span class="fence-device-code">{{ device.device_code }}</span>
                  </label>
                </div>
                <p class="form-text mb-0">{{ $t('device_map_fence_devices_hint') }}</p>
              </div>

              <div class="form-check mt-3">
                <input id="fence-enabled" v-model="fenceForm.enabled" class="form-check-input" type="checkbox" />
                <label class="form-check-label" for="fence-enabled">{{ $t('device_map_fence_enabled') }}</label>
              </div>
            </div>
            <div class="modal-footer">
              <button type="button" class="btn btn-outline-secondary" @click="closeFenceForm">{{ $t('cancel') }}</button>
              <button type="button" class="btn btn-primary" :disabled="fenceForm.saving || !fenceForm.name" @click="submitFenceForm">
                <span v-if="fenceForm.saving" class="spinner-border spinner-border-sm me-1"></span>
                {{ $t('save') }}
              </button>
            </div>
          </div>
        </div>
      </div>
    </Teleport>

    <!-- 按地区名称创建围栏 -->
    <Teleport to="body">
      <div v-if="regionDialog.open" class="modal fade show d-block" tabindex="-1" role="dialog" aria-modal="true">
        <div class="modal-dialog modal-dialog-centered">
          <div class="modal-content">
            <div class="modal-header">
              <h5 class="modal-title">{{ $t('device_map_fence_by_region') }}</h5>
              <button type="button" class="btn-close" @click="closeRegionDialog"></button>
            </div>
            <div class="modal-body">
              <label class="form-label">{{ $t('device_map_region_query') }}</label>
              <div class="d-flex gap-2">
                <input
                  v-model="regionDialog.query"
                  type="text"
                  class="form-control"
                  :placeholder="$t('device_map_region_placeholder')"
                  @keydown.enter="runRegionLookup"
                />
                <button
                  type="button"
                  class="btn btn-primary flex-shrink-0"
                  :disabled="regionDialog.loading || !regionDialog.query.trim()"
                  @click="runRegionLookup"
                >
                  <span v-if="regionDialog.loading" class="spinner-border spinner-border-sm me-1"></span>
                  {{ $t('device_map_region_search') }}
                </button>
              </div>
              <p class="form-text">{{ $t('device_map_region_hint') }}</p>

              <div v-if="regionDialog.error" class="alert alert-warning small mb-0 mt-2">{{ regionDialog.error }}</div>

              <div v-else-if="regionDialog.result" class="region-result mt-2">
                <div class="region-result-name">{{ regionDialog.result.display_name }}</div>
                <div class="region-result-meta">
                  {{ $t(`device_map_place_kind_${regionDialog.result.level}`) }}
                  · {{ $t('device_map_fence_region_stats', { polygons: regionDialog.result.polygons, points: regionDialog.result.points }) }}
                  <template v-if="regionDialog.result.simplified">
                    · {{ $t('device_map_region_simplified') }}
                  </template>
                </div>
                <p class="region-result-note">{{ $t('device_map_region_preview_note') }}</p>
              </div>
            </div>
            <div class="modal-footer">
              <button type="button" class="btn btn-outline-secondary" @click="closeRegionDialog">{{ $t('cancel') }}</button>
              <button
                type="button"
                class="btn btn-primary"
                :disabled="!regionDialog.result"
                @click="useRegionBoundary"
              >
                {{ regionDialog.forForm ? $t('device_map_region_apply') : $t('device_map_next') }}
              </button>
            </div>
          </div>
        </div>
      </div>
    </Teleport>

    <!-- 围栏设备清单 -->
    <Teleport to="body">
      <div v-if="fenceDeviceDialog.open" class="modal fade show d-block" tabindex="-1" role="dialog" aria-modal="true">
        <div class="modal-dialog modal-dialog-centered">
          <div class="modal-content">
            <div class="modal-header">
              <h5 class="modal-title">{{ $t('device_map_fence_view_devices') }}</h5>
              <button type="button" class="btn-close" @click="closeFenceDevices"></button>
            </div>
            <div class="modal-body">
              <div class="fence-device-dialog-head">
                <span class="fence-device-dialog-name">{{ fenceDeviceDialog.fence?.name }}</span>
                <span class="fence-device-dialog-scope">
                  <template v-if="fenceDeviceDialog.fence">{{ fenceScopeLabel(fenceDeviceDialog.fence) }}</template>
                </span>
              </div>

              <div v-if="!fenceDeviceRows.length" class="map-panel-empty">
                {{ $t('device_map_fence_no_devices') }}
              </div>
              <div v-else class="fence-device-view">
                <div v-for="row in fenceDeviceRows" :key="row.device_code" class="fence-device-view-row">
                  <span class="fence-device-view-name">{{ row.device_name }}</span>
                  <span class="fence-device-view-code">{{ row.device_code }}</span>
                  <span class="fence-state-tag" :class="`is-${row.state}`">{{ fenceStateLabel(row.state) }}</span>
                </div>
              </div>

              <p class="form-text mt-2 mb-0">{{ $t('device_map_fence_state_hint') }}</p>
            </div>
            <div class="modal-footer">
              <button type="button" class="btn btn-outline-secondary" @click="closeFenceDevices">
                {{ $t('cancel') }}
              </button>
            </div>
          </div>
        </div>
      </div>
    </Teleport>

    <!--
      协议插件提供的设备弹窗（当前是 GB28181 实时视频）。
      `mediaPlayer: true` 的动作把要打开的组件挂在 action.component 上，
      这里只负责按需渲染 —— 地图页不 import 任何视频组件，插件被裁掉时自然没有入口。
    -->
    <Teleport to="body">
      <component
        :is="modal.component"
        v-for="modal in activeExtensionModals"
        :key="modal.name"
        v-bind="modal.props"
        @close="closeExtensionModal(modal.name)"
      />
    </Teleport>
  </div>
</template>

<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
// maplibre-gl v6 只提供具名导出（无 default），且 `Map` 会遮蔽原生 Map 构造器，
// 因此这里统一用命名空间式别名引入。
import { Map as MapLibreMap, LngLatBounds, Marker, NavigationControl, setWorkerUrl } from 'maplibre-gl'
import 'maplibre-gl/dist/maplibre-gl.css'
// 瓦片解析 worker 必须显式产出。
//
// MapLibre 默认按 `new URL('maplibre-gl-worker.mjs', import.meta.url)` 就地找 worker，
// 打进产物后这个地址是 `/assets/maplibre-gl-worker.mjs`，但 **vite 不会自动把
// node_modules 里的这个文件作为资源产出**；后端静态服务对缺失路径又会走 SPA 回退
// 返回 index.html —— worker 拿到一段 HTML，启动失败。
//
// 症状很有迷惑性：栅格浮雕底图照常显示，所以"地图看起来是好的"，但**矢量瓦片
// 永远解析不出来**——没有任何路网与注记，放大到街区级只剩一片底色。
// 用 ?url 让 vite 明确产出该文件，再把地址交给 maplibre。
import maplibreWorkerUrl from 'maplibre-gl/dist/maplibre-gl-worker.mjs?url'

setWorkerUrl(maplibreWorkerUrl)

import LiquidGlassButton from '../components/liquid-glass/LiquidGlassButton.vue'
import { useToast } from '../composables/useToast'
import { useConfirm } from '../composables/useConfirm'
import { subscribeDeviceEvents } from '../composables/useDeviceEventStream'
import { useAuthStore } from '../stores/auth'
import { installDeviceMarkerImages } from '../utils/deviceMapMarkers'
import { usePlugins } from '../plugins/registry'
import {
  assembleDeviceProperties,
  formatPropertyValue,
  loadDeviceSnapshot,
  loadProductProperties,
} from '../utils/deviceRealtime'
import {
  PULSE_FRAME_INTERVAL_MS,
  PULSE_STILL_INTENSITY,
  buildPulseFilter,
  glowColor,
  loadPulseStatuses,
  prefersReducedMotion,
  pulseGeometry,
  pulseIntensity,
  savePulseStatuses,
} from '../utils/devicePulse'
import {
  BASE_ELEMENT_GROUPS,
  BUSINESS_LAYER_GROUPS,
  applyElementVisibility,
  loadElementPrefs,
  saveElementPrefs,
} from '../utils/mapElementLayers'
import {
  BASEMAP_PROVIDERS,
  DARK_OVERLAY,
  getProvider,
  loadBasemapPreference,
  loadTiandituToken,
  resolveBasemap,
  saveBasemapPreference,
  saveTiandituToken,
} from '../utils/mapProviders'
import {
  DEVICE_STATUS_ORDER,
  circleRing,
  createGeofence,
  deleteGeofence,
  deviceStatusColor,
  deviceStatusLabelKey,
  devicesToFeatureCollection,
  fencesToFeatureCollection,
  fetchGeofences,
  fetchIpLocation,
  fetchMapDevices,
  fenceAccent,
  fenceGeometry,
  fenceGeometryStats,
  isStaleLocation,
  lookupRegionBoundary,
  mapTokenColor,
  readIpLocationCache,
  removeDeviceLocation,
  resolveDeviceStatus,
  saveDeviceLocation,
  searchPlaces,
  updateGeofence,
} from '../utils/deviceMap'

const { t, locale } = useI18n()
const { showToast } = useToast()
const { confirmDialog } = useConfirm()
const router = useRouter()
const authStore = useAuthStore()

const mapContainer = ref(null)
const basemapSwitchRef = ref(null)
const basemapMenuOpen = ref(false)

const loading = ref(true)
const tileError = ref(false)
const devices = ref([])
const fences = ref([])
const fenceStates = ref([])
const search = ref('')
const statusFilter = ref('')
const panelTab = ref('devices')
const selectedCode = ref('')
// 当前在地图上被"聚焦"的围栏。为空表示没有聚焦，全部围栏同等显示。
const selectedFenceId = ref(null)
const hoverDevice = ref(null)
const hoverPoint = reactive({ x: 0, y: 0 })
// 围栏悬停标签：与设备悬停卡并存会互相遮挡，两者互斥（见 handleFenceHover）。
const hoverFence = ref(null)
const hoverFencePoint = reactive({ x: 0, y: 0 })
const cursorPoint = reactive({ lng: null, lat: null })
const isDark = ref(false)

// ---- 悬停卡的实时属性与视频点播 ----
//
// 属性必须异步取（物模型在产品上、值在设备最新快照里），而地图上的 hover 是高频动作：
// 用户扫一眼会瞬间掠过十几个点。所以这里做了三件事：
//   1. 停留 180ms 才发请求 —— 扫过就取消，不发无用的网络请求；
//   2. 结果按设备比较 seq，慢响应回来时若已经换了设备就丢弃，绝不覆盖新卡片；
//   3. 产品物模型与最新值各有 5s~5min 缓存，来回悬停同一台设备不再重复请求。
const HOVER_PROBE_DELAY = 180
// 移出圆点到卡片消失之间留的宽限。卡片里现在有按钮（视频、移动图钉），
// 没有这段宽限，光标从圆点移到卡片上的一瞬间卡片就没了，按钮永远点不到。
const HOVER_CLOSE_DELAY = 220
const HOVER_PROPERTY_LIMIT = 6
const hoverProperties = ref([])
const hoverPropertiesLoading = ref(false)
const hoverPropertiesError = ref('')
const hoverSnapshotAt = ref(null)
const hoverCardRef = ref(null)
// 卡片的实测尺寸：用于边缘翻转。估一个初值，测量在卡片渲染后补上。
const hoverCardBox = reactive({ width: 268, height: 240 })
let hoverProbeTimer = 0
let hoverProbeSeq = 0
let hoverCloseTimer = 0

// ---- 协议插件提供的设备动作（摄像机 ⇒ 可点播实时视频）----
const { extensions } = usePlugins()
const mediaPlayerActions = computed(() => (extensions.value.deviceActions || []).filter((action) => action.mediaPlayer))
const activeExtensionModals = ref([])

const basemapId = ref(loadBasemapPreference())
const tiandituToken = ref(loadTiandituToken())
const tiandituTokenDraft = ref(tiandituToken.value)

// ---- 图层元素可见性 ----
const layerPanelOpen = ref(false)
const layerPanelRef = ref(null)
const elementPrefs = reactive(loadElementPrefs())
// 只有矢量底图能逐元素开关；天地图是栅格瓦片，可切的只有注记层。
const vectorElements = computed(() => activeProvider.value?.kind === 'vector')
// 栅格底图下不列"点了也没用"的开关，只留注记 —— 灰掉 6 行比不显示更让人困惑。
const baseElementGroups = computed(() =>
  vectorElements.value ? BASE_ELEMENT_GROUPS : BASE_ELEMENT_GROUPS.filter((group) => group.id === 'labels'),
)

// ---- 2D / 3D 视角 ----
const DIMENSION_STORAGE_KEY = 'noyo.map.dimension'
// 3D 只是相机俯角，不含地形起伏（没有可用的 DEM 来源）。50° 足够看出前后关系又不过度变形。
const MAP_3D_PITCH = 50
const is3D = ref(loadDimensionPreference() === '3d')
let compassControl = null

// ---- 状态脉冲高亮 ----
//
// 只画外圈光晕，核心圆点保持静止 —— 圆点的颜色/填充/尺寸是状态编码（详见
// utils/deviceDotCanvas.js），跟着动画变形会把"告警更大"读成"告警正在放大"。
// 几何与相位算法在 utils/devicePulse.js，这里只负责挂图层和驱动逐帧更新。
//
// 双层结构：`device-pulse-wave` 是向外推出的描边环（负责"扩"），
// `device-pulse` 是贴住圆点的实心柔光（负责"亮"）。单层做不出"闪"：
// 只有亮度没有方向感，看起来就是"一团颜色在浓淡变化"。
const PULSE_LAYER_ID = 'device-pulse'
const PULSE_WAVE_LAYER_ID = 'device-pulse-wave'
const pulsePanelOpen = ref(false)
const pulsePanelRef = ref(null)
const pulseStatuses = ref(loadPulseStatuses())
// 系统级"减少动态效果"：开启后不跑逐帧循环，改为静态光环（无障碍要求）。
const pulseReducedMotion = ref(prefersReducedMotion())
let pulseFrame = 0
let pulseStartedAt = 0
let pulseLastPaint = 0

function pulseColorExpression() {
  const stops = []
  // 光晕用提亮加饱和后的派生色，而不是直接复用圆点填充色 ——
  // 圆点色偏沉稳（要看起来像实体），光晕要的是"发光"，两者诉求不同。
  for (const status of DEVICE_STATUS_ORDER) stops.push(status, glowColor(deviceStatusColor(status)))
  return ['match', ['get', 'status'], ...stops, glowColor(deviceStatusColor('online'))]
}

function paintPulseFrame(now, { still = false } = {}) {
  if (!map) return
  const hasCore = Boolean(map.getLayer(PULSE_LAYER_ID))
  const hasWave = Boolean(map.getLayer(PULSE_WAVE_LAYER_ID))
  if (!hasCore && !hasWave) return
  // 30fps 足够：半径总变化只有 10px 量级，60fps 的额外帧只是白烧 GPU。
  if (!still && now - pulseLastPaint < PULSE_FRAME_INTERVAL_MS) return
  pulseLastPaint = now
  const intensity = still ? PULSE_STILL_INTENSITY : pulseIntensity(now - pulseStartedAt)
  const geometry = pulseGeometry(intensity, map.getZoom())
  // 只逐帧改四个常量型 paint 属性：评估是 O(1)，不会逐 feature 重算。
  if (hasCore) {
    map.setPaintProperty(PULSE_LAYER_ID, 'circle-radius', geometry.coreRadius)
    map.setPaintProperty(PULSE_LAYER_ID, 'circle-opacity', geometry.coreOpacity)
  }
  if (hasWave) {
    map.setPaintProperty(PULSE_WAVE_LAYER_ID, 'circle-radius', geometry.waveRadius)
    map.setPaintProperty(PULSE_WAVE_LAYER_ID, 'circle-stroke-opacity', geometry.waveOpacity)
  }
}

/** 脉冲是否应该运行：任何一条不满足都必须彻底停掉 rAF，不能"空转"。 */
function pulseShouldAnimate() {
  return Boolean(
    map &&
      (map.getLayer(PULSE_LAYER_ID) || map.getLayer(PULSE_WAVE_LAYER_ID)) &&
      pulseStatuses.value.length > 0 &&
      elementPrefs.devices !== false &&
      !pulseReducedMotion.value &&
      !document.hidden,
  )
}

function loopPulse(now) {
  pulseFrame = 0
  if (!pulseShouldAnimate()) return
  paintPulseFrame(now)
  pulseFrame = requestAnimationFrame(loopPulse)
}

function syncPulseAnimation() {
  if (pulseShouldAnimate()) {
    if (!pulseFrame) {
      pulseStartedAt = performance.now()
      pulseLastPaint = 0
      pulseFrame = requestAnimationFrame(loopPulse)
    }
    return
  }
  if (pulseFrame) {
    cancelAnimationFrame(pulseFrame)
    pulseFrame = 0
  }
  // 不跑动画时仍落一帧静态值：关掉设备图层或系统要求减少动效时，
  // 用户看到的应该是"淡淡的固定光环"，而不是"选了却毫无反应"。
  paintPulseFrame(performance.now(), { still: true })
}

/**
 * 回到前台：恢复脉冲 + 补一次校准。
 *
 * 后台期间定时器会被浏览器节流（通常压到 1 次/分钟），SSE 事件本身能到，
 * 但校准轮询基本停摆。切回来时补一次，避免"看到的是几分钟前的状态"。
 */
function handleVisibilityChange() {
  syncPulseAnimation()
  if (!document.hidden) refreshDevicesSilently()
}

/** 把 filter 与配色落到已有图层上。切底图（setStyle）后图层会重建，必须重跑。 */
function applyPulseLayer() {
  if (!map) return
  const filter = buildPulseFilter(pulseStatuses.value)
  const color = pulseColorExpression()
  if (map.getLayer(PULSE_LAYER_ID)) {
    map.setFilter(PULSE_LAYER_ID, filter)
    map.setPaintProperty(PULSE_LAYER_ID, 'circle-color', color)
  }
  if (map.getLayer(PULSE_WAVE_LAYER_ID)) {
    map.setFilter(PULSE_WAVE_LAYER_ID, filter)
    map.setPaintProperty(PULSE_WAVE_LAYER_ID, 'circle-stroke-color', color)
  }
  paintPulseFrame(performance.now(), { still: true })
}

function setPulseStatuses(list) {
  pulseStatuses.value = [...list]
  savePulseStatuses(pulseStatuses.value)
  applyPulseLayer()
  syncPulseAnimation()
}

function togglePulseStatus(status) {
  const current = pulseStatuses.value
  setPulseStatuses(current.includes(status) ? current.filter((item) => item !== status) : [...current, status])
}

/** 两个弹出菜单共用右下角同一块空间，同时打开会互相遮挡 ⇒ 互斥。 */
function togglePulsePanel() {
  pulsePanelOpen.value = !pulsePanelOpen.value
  if (pulsePanelOpen.value) layerPanelOpen.value = false
}

function toggleLayerPanel() {
  layerPanelOpen.value = !layerPanelOpen.value
  if (layerPanelOpen.value) pulsePanelOpen.value = false
}

// ---- 地点搜索 ----
const placeQuery = ref('')
const placeResults = ref([])
const placeSearching = ref(false)
const placeError = ref('')
const placePanelOpen = ref(false)
const placeHighlight = ref(null)
const mapSearchRef = ref(null)

// ---- 围栏创建入口 ----
const fenceMenuOpen = ref(false)
const fenceCreateRef = ref(null)

// ---- 按地区名称建围栏 ----
const regionDialog = reactive({
  open: false,
  query: '',
  loading: false,
  error: '',
  result: null,
  // true 表示是"编辑既有地区围栏时重新选地区"，保存时回填到表单而不是新建
  forForm: false,
})

// ---- 围栏设备清单 ----
const fenceDeviceDialog = reactive({
  open: false,
  fence: null,
})

const fenceDeviceRows = computed(() => (fenceDeviceDialog.fence ? fenceDevices(fenceDeviceDialog.fence) : []))

const drawing = reactive({
  mode: '',
  points: [],
  // hover 是"光标当前所在位置"，用来画跟随鼠标的那一段线和一个空心点。
  // 它**不进 points** —— 否则右键完成时会把光标所在的半路坐标当成一个顶点。
  hover: null,
  // finished：右键/双击已定稿。此时预览冻结、光标恢复默认，
  // 由用户自己决定什么时候点「下一步」进表单。
  finished: false,
  radiusM: 0,
  centerLng: null,
  centerLat: null,
  deviceCode: '',
  deviceName: '',
})

const fenceForm = reactive({
  open: false,
  saving: false,
  id: null,
  name: '',
  kind: 'polygon',
  rule_type: 'keep_in',
  center_lng: null,
  center_lat: null,
  radius_m: 0,
  ring: [],
  // geometry 是「表单最终要提交的 GeoJSON 字符串」。手绘围栏由 ring 生成，
  // 地区围栏直接沿用后端返回的 MultiPolygon —— 两者不能互相推导。
  geometry: '',
  source: 'manual',
  region_name: '',
  region_level: '',
  device_codes: [],
  deviceSearch: '',
  enter_dwell_s: 0,
  exit_dwell_s: 0,
  enabled: true,
})

let map = null
// 地点搜索结果的定位标记（DOM Marker，不是业务图层）。
let placeMarker = null
// 绘制预览的合帧句柄。mousemove 每秒可能来上百次，但预览只需要跟得上帧，
// 用 rAF 合帧，避免每次都做一次 GeoJSON setData + 重绘。
let drawingHoverFrame = 0
let themeObserver = null
let resizeObserver = null
// 地图「样式 + 首帧」是否就绪。样式是异步拉取的（provider 给的是 styleUrl），
// 数据往往先到 —— 拟合会先做一次，等 load 之后再用同一视窗补一次。
let mapReady = false
// 用户是否自己动过相机（拖动 / 滚轮 / 双击缩放）。异步回来的默认视野据此让路。
let userMovedMap = false
// 默认视野的序号：每发起一次拟合自增，用来丢掉过期的 IP 定位回调。
let defaultViewSeq = 0
// IP 粗定位结果（{ longitude, latitude }）与进行中的请求。
// 初值取会话缓存：30 分钟内的上一次结果够用，而且首帧就能落到城市上，
// 不必"先铺中国全境再收窄"地跳两次。
let ipLocation = readIpLocationCache()
let ipLocationRequest = null

const activeProvider = computed(() => BASEMAP_PROVIDERS.find((item) => item.id === basemapId.value) || null)
const activeProviderLabel = computed(() => {
  const provider = activeProvider.value
  if (!provider) return t('device_map_basemap')
  return locale.value === 'en' ? provider.label.en : provider.label.zh
})

const locatedDevices = computed(() =>
  devices.value.filter((device) => device.has_location && device.longitude != null && device.latitude != null),
)

// ---- 空状态提示卡的可关闭状态 ----
// 按会话记忆：同一轮浏览里关掉就不再弹；设备重新有位置后记忆失效，
// 下次再变空时提示会回来（否则这条引导一旦关掉就永久消失）。
const EMPTY_STATE_STORAGE_KEY = 'noyo.map.emptyStateDismissed'
const emptyStateDismissed = ref(readEmptyStateDismissed())

const showEmptyState = computed(
  () =>
    !loading.value &&
    !drawing.mode &&
    devices.value.length > 0 &&
    locatedDevices.value.length === 0 &&
    !emptyStateDismissed.value,
)

function readEmptyStateDismissed() {
  try {
    return sessionStorage.getItem(EMPTY_STATE_STORAGE_KEY) === '1'
  } catch (error) {
    // 隐私模式 / 禁用存储：退化为"本次页面内生效"。
    return false
  }
}

function persistEmptyStateDismissed(dismissed) {
  try {
    if (dismissed) sessionStorage.setItem(EMPTY_STATE_STORAGE_KEY, '1')
    else sessionStorage.removeItem(EMPTY_STATE_STORAGE_KEY)
  } catch (error) {
    /* ignore */
  }
}

function dismissEmptyState() {
  emptyStateDismissed.value = true
  persistEmptyStateDismissed(true)
}

function deviceDisplayStatus(device) {
  return resolveDeviceStatus(device, fenceStates.value, fences.value)
}

const filteredDevices = computed(() => {
  const keyword = search.value.trim().toLowerCase()
  return devices.value.filter((device) => {
    if (keyword) {
      const haystack = `${device.device_name || ''} ${device.device_code || ''}`.toLowerCase()
      if (!haystack.includes(keyword)) return false
    }
    if (statusFilter.value && deviceDisplayStatus(device) !== statusFilter.value) return false
    return true
  })
})

const fenceRingVertexCount = computed(() => {
  const ring = fenceForm.ring || []
  if (ring.length < 2) return ring.length
  const first = ring[0]
  const last = ring[ring.length - 1]
  // ring 是闭合环（首尾同点），顶点数要减一，否则三角形会显示成 4 个点
  const closed = first[0] === last[0] && first[1] === last[1]
  return closed ? ring.length - 1 : ring.length
})

// 表单里的几何摘要：手绘看顶点数，地区围栏看"几个区块 + 多少顶点"。
const fenceFormShapeSummary = computed(() => {
  if (fenceForm.kind === 'circle') {
    return { kind: 'circle', vertices: 0, polygons: 1 }
  }
  if (fenceForm.source === 'region' && fenceForm.geometry) {
    const stats = fenceGeometryStats({ kind: 'polygon', geometry: fenceForm.geometry })
    return { kind: 'region', ...stats }
  }
  return { kind: 'polygon', vertices: fenceRingVertexCount.value, polygons: 1 }
})

// 设备多选：按关键词过滤，只列已定位设备 —— 没定位的设备进不了围栏判定。
const fenceDeviceCandidates = computed(() => {
  const keyword = (fenceForm.deviceSearch || '').trim().toLowerCase()
  return locatedDevices.value.filter((device) => {
    if (!keyword) return true
    const haystack = `${device.device_name || ''} ${device.device_code || ''}`.toLowerCase()
    return haystack.includes(keyword)
  })
})

const selectedFenceDeviceCodes = computed(() => new Set(fenceForm.device_codes || []))

const drawingHint = computed(() => {
  if (drawing.mode === 'fence-polygon') {
    return drawing.finished ? t('device_map_draw_finished_hint') : t('device_map_draw_polygon_hint')
  }
  return ''
})

const cursorReadout = computed(() => {
  if (cursorPoint.lng == null) return ''
  return `${formatNumber(cursorPoint.lng, 6)}, ${formatNumber(cursorPoint.lat, 6)}`
})

const hoverStatus = computed(() => {
  const device = hoverDevice.value
  if (!device) return 'offline'
  return deviceDisplayStatus(device)
})

/** 空态文案三态：无权 / 读取失败 / 设备确实没上报。把"失败"说成"没数据"会误导排障。 */
const hoverProbeMessage = computed(() => {
  if (hoverPropertiesError.value === 'forbidden') return t('device_map_hover_no_permission')
  if (hoverPropertiesError.value === 'failed') return t('device_map_hover_load_failed')
  return t('device_map_hover_no_data')
})

/**
 * 悬停卡定位：默认落在光标右下；贴近画布右/下边缘时翻到另一侧。
 *
 * 翻转是必须的 —— 卡片在加了实时属性区之后高度可达 300px 以上，
 * 而 `.map-surface` 是 overflow:hidden：落在右下角的设备，卡片下半截
 * （正好是视频按钮与属性值）会被直接裁掉，看起来像"没渲染完"。
 * 尺寸用实测值（measureHoverCard），不靠估算：属性条数、双语换行都会改变高度。
 */
const hoverCardStyle = computed(() => {
  const width = mapContainer.value?.clientWidth || 0
  const height = mapContainer.value?.clientHeight || 0
  const cardWidth = hoverCardBox.width || 268
  const cardHeight = hoverCardBox.height || 240
  const gap = 16
  const margin = 8

  let left = hoverPoint.x + gap
  if (width && left + cardWidth > width - margin) {
    left = Math.max(margin, hoverPoint.x - gap - cardWidth)
  }
  let top = hoverPoint.y + gap
  if (height && top + cardHeight > height - margin) {
    top = Math.max(margin, hoverPoint.y - gap - cardHeight)
  }
  return { left: `${left}px`, top: `${top}px` }
})

/** 卡片实测尺寸。只在卡片渲染后读一次布局，hover 频率下这点 reflow 可以忽略。 */
function measureHoverCard() {
  const element = hoverCardRef.value
  if (!element) return
  hoverCardBox.width = element.offsetWidth || hoverCardBox.width
  hoverCardBox.height = element.offsetHeight || hoverCardBox.height
}

function cancelHoverClose() {
  if (hoverCloseTimer) {
    clearTimeout(hoverCloseTimer)
    hoverCloseTimer = 0
  }
}

function scheduleHoverClose() {
  cancelHoverClose()
  hoverCloseTimer = window.setTimeout(() => {
    hoverCloseTimer = 0
    hoverDevice.value = null
    resetHoverProbe()
  }, HOVER_CLOSE_DELAY)
}

/** 悬停卡内容全部清空：换设备、关闭、打开弹窗时都要走这里，避免闪出上一台的数值。 */
function resetHoverProbe() {
  if (hoverProbeTimer) {
    clearTimeout(hoverProbeTimer)
    hoverProbeTimer = 0
  }
  hoverProbeSeq += 1
  hoverProperties.value = []
  hoverPropertiesLoading.value = false
  hoverPropertiesError.value = ''
  hoverSnapshotAt.value = null
}

function scheduleHoverProbe(device) {
  resetHoverProbe()
  const seq = hoverProbeSeq
  hoverProbeTimer = window.setTimeout(() => {
    hoverProbeTimer = 0
    probeHoverDevice(device, seq)
  }, HOVER_PROBE_DELAY)
}

async function probeHoverDevice(device, seq) {
  hoverPropertiesLoading.value = true
  const [tslProperties, snapshot] = await Promise.all([
    loadProductProperties(device.product_code),
    loadDeviceSnapshot(device.device_code),
  ])
  // 慢响应回来时用户可能已经换了设备（甚至关了卡）⇒ 直接丢弃。
  if (seq !== hoverProbeSeq || hoverDevice.value?.device_code !== device.device_code) return
  hoverProperties.value = assembleDeviceProperties(tslProperties, snapshot.data)
  hoverSnapshotAt.value = snapshot.updatedAt
  hoverPropertiesError.value = snapshot.error || ''
  hoverPropertiesLoading.value = false
  await nextTick()
  measureHoverCard()
}

/**
 * 围栏标签默认出现在光标右上；靠近右边缘时翻到左侧，
 * 否则会被 .map-surface 的 overflow:hidden 裁掉半截。
 */
const hoverFenceStyle = computed(() => {
  const width = mapContainer.value?.clientWidth || 0
  const flip = width > 0 && hoverFencePoint.x > width - 200
  return {
    left: `${hoverFencePoint.x + (flip ? -14 : 14)}px`,
    top: `${hoverFencePoint.y - 10}px`,
    transform: flip ? 'translate(-100%, -100%)' : 'translate(0, -100%)',
  }
})

// 围栏对哪些设备生效：绑定为空 = 全部已定位设备。
function fenceScopeLabel(fence) {
  const assigned = fence.assigned_devices || []
  if (assigned.length === 0) return t('device_map_fence_scope_all')
  return t('device_map_fence_scope_count', { count: assigned.length })
}

/**
 * 这台设备现在是不是在这条围栏「里面」。
 *
 * 只看判定状态（inside），不重新算几何 —— 几何判定在后端，前端再算一遍
 * 必然出现两边口径不一致（迟滞、精度带都在后端）。
 */
function fenceStateOf(fenceId, deviceCode) {
  const hit = fenceStates.value.find(
    (state) => Number(state.fence_id) === Number(fenceId) && state.device_code === deviceCode,
  )
  return hit ? hit.state : 'unknown'
}

/** 当前位于围栏内部的设备数。没有判定记录的设备不计入（它可能根本没上报过位置）。 */
function fenceInsideCount(fenceId) {
  return fenceStates.value.filter(
    (state) => Number(state.fence_id) === Number(fenceId) && state.state === 'inside',
  ).length
}

/** 这条围栏要展示的设备清单：绑定为空时按"对全部已定位设备生效"处理。 */
function fenceDevices(fence) {
  const assigned = fence.assigned_devices || []
  const pool = assigned.length
    ? locatedDevices.value.filter((device) => assigned.includes(device.device_code))
    : locatedDevices.value
  return pool.map((device) => ({
    device_code: device.device_code,
    device_name: device.device_name || device.device_code,
    state: fenceStateOf(fence.ID ?? fence.id, device.device_code),
  }))
}

/** 计算围栏几何的包围盒（用于"点一下就定位过去"）。 */
function fenceBounds(fence) {
  const geometry = fenceGeometry(fence)
  if (!geometry) return null
  let minLng = Infinity
  let minLat = Infinity
  let maxLng = -Infinity
  let maxLat = -Infinity
  const visitRing = (ring) => {
    ring.forEach(([lng, lat]) => {
      if (lng < minLng) minLng = lng
      if (lng > maxLng) maxLng = lng
      if (lat < minLat) minLat = lat
      if (lat > maxLat) maxLat = lat
    })
  }
  if (geometry.type === 'Polygon') {
    geometry.coordinates.forEach(visitRing)
  } else if (geometry.type === 'MultiPolygon') {
    geometry.coordinates.forEach((polygon) => polygon.forEach(visitRing))
  } else {
    return null
  }
  if (!Number.isFinite(minLng) || !Number.isFinite(minLat)) return null
  return [
    [minLng, minLat],
    [maxLng, maxLat],
  ]
}

/** 点击围栏行：定位到它并把其余围栏压淡。再点一次取消聚焦。 */
function focusFence(fence) {
  const id = Number(fence.ID ?? fence.id)
  if (Number(selectedFenceId.value) === id) {
    selectedFenceId.value = null
    syncSources()
    return
  }
  selectedFenceId.value = id
  syncSources()
  const bounds = fenceBounds(fence)
  if (bounds && map) {
    // maxZoom 防止一条小围栏被放大到只剩几个像素的底图色块。
    map.fitBounds(bounds, { padding: 88, maxZoom: 14, duration: 600 })
  }
}

function isFenceSelected(fence) {
  return Number(selectedFenceId.value) === Number(fence.ID ?? fence.id)
}

function fenceStateLabel(state) {
  const key = state === 'inside' || state === 'outside' ? state : 'unknown'
  return t(`device_map_fence_state_${key}`)
}

function openFenceDevices(fence) {
  fenceDeviceDialog.fence = fence
  fenceDeviceDialog.open = true
}

function closeFenceDevices() {
  fenceDeviceDialog.open = false
}

const EMPTY_FC = { type: 'FeatureCollection', features: [] }

// ===================== 生命周期 =====================

onMounted(async () => {
  observeTheme()
  await bootstrapMap()
  await loadData()
  // 数据到手之后再订阅：先有基线，事件才是"在此基础上打补丁"。
  startRealtimeUpdates()
  window.addEventListener('click', handleGlobalClick)
  window.addEventListener('keydown', handleKeydown)
  // 标签页切到后台时暂停脉冲——不可见还在跑 rAF 是纯耗电，没有视觉收益。
  document.addEventListener('visibilitychange', handleVisibilityChange)
})

onBeforeUnmount(() => {
  window.removeEventListener('click', handleGlobalClick)
  window.removeEventListener('keydown', handleKeydown)
  document.removeEventListener('visibilitychange', handleVisibilityChange)
  stopRealtimeUpdates()
  // 脉冲动画循环必须掐掉：它每帧都在 map 上 setPaintProperty，地图 remove 之后
  // 继续跑会持续报错并拖慢整个页面。
  if (pulseFrame) {
    cancelAnimationFrame(pulseFrame)
    pulseFrame = 0
  }
  // 悬停卡的延迟取数/延迟关闭计时器同理：组件销毁后回调不该再碰响应式状态。
  if (hoverProbeTimer) {
    clearTimeout(hoverProbeTimer)
    hoverProbeTimer = 0
  }
  if (hoverCloseTimer) {
    clearTimeout(hoverCloseTimer)
    hoverCloseTimer = 0
  }
  activeExtensionModals.value = []
  if (themeObserver) themeObserver.disconnect()
  if (resizeObserver) resizeObserver.disconnect()
  compassControl = null
  // 挂起的合帧回调必须取消：它会在组件销毁后访问已 remove 的地图。
  if (drawingHoverFrame) {
    cancelAnimationFrame(drawingHoverFrame)
    drawingHoverFrame = 0
  }
  // 任一步抛错都不能留下一个还活着的地图实例：它持有 WebGL 上下文与
  // styledata 处理链，漏掉会把后续所有页面一起拖慢。
  try {
    clearClusterLabels()
  } catch (error) {
    /* ignore */
  }
  if (placeMarker) {
    try {
      placeMarker.remove()
    } catch (error) {
      /* ignore */
    }
    placeMarker = null
  }
  if (map) {
    try {
      map.remove()
    } catch (error) {
      /* ignore */
    }
  }
  map = null
  mapReady = false
})

/** Esc 统一退出：先退绘制，再收搜索面板，最后关浮层。 */
function handleKeydown(event) {
  if (event.key !== 'Escape') return
  if (drawing.mode) {
    cancelDrawing()
    return
  }
  if (placePanelOpen.value) {
    placePanelOpen.value = false
    return
  }
  if (fenceMenuOpen.value) fenceMenuOpen.value = false
}

function observeTheme() {
  const read = () => {
    const root = document.querySelector('[data-bs-theme]')
    isDark.value = root?.getAttribute('data-bs-theme') === 'dark'
  }
  read()
  themeObserver = new MutationObserver(read)
  themeObserver.observe(document.documentElement, { attributes: true, subtree: true, attributeFilter: ['data-bs-theme'] })
}

function handleGlobalClick(event) {
  if (basemapMenuOpen.value) {
    if (!(basemapSwitchRef.value && basemapSwitchRef.value.contains(event.target))) {
      basemapMenuOpen.value = false
    }
  }
  if (fenceMenuOpen.value) {
    if (!(fenceCreateRef.value && fenceCreateRef.value.contains(event.target))) {
      fenceMenuOpen.value = false
    }
  }
  if (placePanelOpen.value) {
    if (!(mapSearchRef.value && mapSearchRef.value.contains(event.target))) {
      placePanelOpen.value = false
    }
  }
  if (layerPanelOpen.value) {
    if (!(layerPanelRef.value && layerPanelRef.value.contains(event.target))) {
      layerPanelOpen.value = false
    }
  }
  if (pulsePanelOpen.value) {
    if (!(pulsePanelRef.value && pulsePanelRef.value.contains(event.target))) {
      pulsePanelOpen.value = false
    }
  }
}

async function bootstrapMap() {
  const resolved = resolveBasemap(basemapId.value, { token: tiandituToken.value })
  if (!resolved || resolved.unavailable) {
    // 天地图需要 Key，回退到开发期底图并提示
    basemapId.value = 'openfreemap'
    saveBasemapPreference(basemapId.value)
    basemapMenuOpen.value = true
    showToast('warning', t('device_map_token_required'))
  }

  const style = resolveStyleFor(basemapId.value)
  map = new MapLibreMap({
    container: mapContainer.value,
    style: style || resolveStyleFor('openfreemap'),
    // 初始视野给中国全境，别用某个城市：没有设备时这一页的第一印象
    // 就是"我的系统覆盖哪里"，落在单个城市会让人以为只支持那一个点。
    center: [104.0, 30.5],
    zoom: 3,
    // 记住上次的视角：2D/3D 是用户偏好，不该每次进页面都重置。
    pitch: is3D.value ? MAP_3D_PITCH : 0,
    attributionControl: false,
    // 底图为栅格时不需要字形服务；显式关闭可避免离线部署时请求外部字体
    fadeDuration: 0,
  })

  if (is3D.value) ensureCompass()

  // style.load means layers can be added; isStyleLoaded also waits for raster
  // tiles. Business overlays must survive slow or failed basemap requests.
  map.on('style.load', () => {
    mapReady = true
    installOverlayLayers()
    syncSources()
  })
  map.on('error', (event) => {
    const message = String(event?.error?.message || '')
    // 只把瓦片相关的错误当作"底图加载失败"，其余（如样式解析）不打断使用
    if (message.includes('tile') || message.includes('Tile') || message.includes('403') || message.includes('401')) {
      tileError.value = true
    }
  })
  map.on('click', handleMapClick)
  // 双击与右键都能结束绘制。绘制期间 doubleClickZoom 已关掉，所以双击不会顺带缩放。
  map.on('dblclick', () => {
    finishFenceDrawing()
  })
  map.on('contextmenu', (event) => {
    if (drawing.mode !== 'fence-polygon') return
    // 右键在地图上的语义是"结束绘制"，必须把浏览器/系统菜单拦掉；
    // MapLibre 不会替我们 preventDefault。
    event.preventDefault?.()
    event.originalEvent?.preventDefault?.()
    finishFenceDrawing()
  })
  map.on('mousemove', (event) => {
    cursorPoint.lng = event.lngLat.lng
    cursorPoint.lat = event.lngLat.lat
    // 画围栏时让最后一段线跟着鼠标走，用户才能看出"现在点下去会画成什么样"。
    if (drawing.mode === 'fence-polygon' && drawing.points.length) {
      trackDrawingHover(event.lngLat.lng, event.lngLat.lat)
    }
  })
  map.on('mouseleave', () => {
    cursorPoint.lng = null
    cursorPoint.lat = null
    // 光标离开画布时图层事件不会再触发，两块悬停浮层必须在这里兜底收掉。
    hoverFence.value = null
    cancelHoverClose()
    hoverDevice.value = null
    resetHoverProbe()
    if (drawing.hover) {
      drawing.hover = null
      refreshDrawingPreview()
    }
  })

  // 用户一旦自己动过地图，异步回来的 IP 定位就不许再抢相机。
  // dragstart / zoomstart 只由用户操作触发；程序化的 easeTo / fitBounds 不会发出这两个事件。
  map.on('dragstart', markUserMovedMap)
  map.on('zoomstart', markUserMovedMap)

  if (typeof ResizeObserver !== 'undefined') {
    resizeObserver = new ResizeObserver(() => map?.resize())
    resizeObserver.observe(mapContainer.value)
  }
}

function resolveStyleFor(id) {
  const resolved = resolveBasemap(id, { token: tiandituToken.value })
  if (!resolved || resolved.unavailable || !resolved.style) return null
  return resolved.style
}

// ===================== 图层安装 =====================

function installOverlayLayers() {
  if (!map || !mapReady) return

  // setStyle 会重建所有 source，聚合 id 随之重算 ⇒ 旧的 DOM 数字标注必须清掉，
  // 否则会留在旧位置直到下一次 moveend。
  const freshStyle = !map.getSource('devices')
  if (freshStyle) clearClusterLabels()

  // 暗色方案甲：底图之上叠半透明暗层。
  // 天地图没有官方暗色层，栅格瓦片也无法改样式 ⇒ 这是唯一可控且可预期的做法。
  if (!map.getLayer('dark-overlay')) {
    map.addLayer({
      id: 'dark-overlay',
      type: 'background',
      paint: { 'background-color': DARK_OVERLAY },
      layout: { visibility: isDark.value ? 'visible' : 'none' },
    })
  }

  if (!map.getSource('fences')) {
    map.addSource('fences', { type: 'geojson', data: EMPTY_FC })
  }
  if (!map.getLayer('fence-fill')) {
    map.addLayer({
      id: 'fence-fill',
      type: 'fill',
      source: 'fences',
      paint: {
        'fill-color': [
          'match',
          ['get', 'rule_type'],
          'forbid',
          fenceAccent('forbid', isDark.value),
          'keep_in',
          fenceAccent('keep_in', isDark.value),
          fenceAccent('notify', isDark.value),
        ],
        // 选中围栏明显加深；未选中轻微淡出，让"我点的是哪一条"一眼可见。
        'fill-opacity': [
          'case',
          ['==', ['get', 'selected'], 1],
          0.26,
          ['==', ['get', 'enabled'], 1],
          0.11,
          0.04,
        ],
      },
    })
  }
  if (!map.getLayer('fence-line')) {
    map.addLayer({
      id: 'fence-line',
      type: 'line',
      source: 'fences',
      paint: {
        'line-color': [
          'match',
          ['get', 'rule_type'],
          'forbid',
          fenceAccent('forbid', isDark.value),
          'keep_in',
          fenceAccent('keep_in', isDark.value),
          fenceAccent('notify', isDark.value),
        ],
        'line-width': ['case', ['==', ['get', 'selected'], 1], 3.5, ['==', ['get', 'enabled'], 1], 2, 1],
        'line-opacity': ['case', ['==', ['get', 'selected'], 1], 1, 0.55],
        'line-dasharray': ['case', ['==', ['get', 'enabled'], 1], ['literal', [1, 0]], ['literal', [2, 2]]],
      },
    })
  }
  // 选中围栏的外发光。只画一条粗而透明的描边，比调色更容易在影像底图上认出来。
  if (!map.getLayer('fence-line-halo')) {
    map.addLayer({
      id: 'fence-line-halo',
      type: 'line',
      source: 'fences',
      filter: ['==', ['get', 'selected'], 1],
      paint: {
        'line-color': [
          'match',
          ['get', 'rule_type'],
          'forbid',
          fenceAccent('forbid', isDark.value),
          'keep_in',
          fenceAccent('keep_in', isDark.value),
          fenceAccent('notify', isDark.value),
        ],
        'line-width': 10,
        'line-opacity': 0.22,
        'line-blur': 4,
      },
    })
    // 光晕要垫在描边下层，否则半透明粗线会把细描边糊淡。
    if (map.getLayer('fence-line')) map.moveLayer('fence-line-halo', 'fence-line')
  }

  // 地区预览：按名称查到的行政区边界，在"确认设为围栏"之前先画出来。
  // 单独一条源，不和 draw-preview（手绘折线）共用 —— 两边同时用会互相清空。
  if (!map.getSource('region-preview')) {
    map.addSource('region-preview', { type: 'geojson', data: EMPTY_FC })
  }
  if (!map.getLayer('region-preview-fill')) {
    map.addLayer({
      id: 'region-preview-fill',
      type: 'fill',
      source: 'region-preview',
      paint: { 'fill-color': '#3B82F6', 'fill-opacity': 0.16 },
    })
  }
  if (!map.getLayer('region-preview-line')) {
    map.addLayer({
      id: 'region-preview-line',
      type: 'line',
      source: 'region-preview',
      paint: { 'line-color': '#2563EB', 'line-width': 2 },
    })
  }

  if (!map.getSource('devices')) {
    map.addSource('devices', {
      type: 'geojson',
      data: EMPTY_FC,
      cluster: true,
      clusterRadius: 50,
      clusterMaxZoom: 14,
    })
  }
  // 聚合气泡刻意不用 symbol 图层写数字：symbol 依赖样式里的 glyphs，
  // 而天地图是栅格底图、根本没有字形服务（OpenFreeMap 的字体栈也是 Noto Sans），
  // 一旦字体栈对不上，数字会静默不渲染。改由 DOM 标注叠加，两种底图行为一致。
  if (!map.getLayer('device-cluster')) {
    map.addLayer({
      id: 'device-cluster',
      type: 'circle',
      source: 'devices',
      filter: ['has', 'point_count'],
      paint: {
        'circle-color': mapTokenColor('--color-brand'),
        'circle-opacity': 0.9,
        'circle-radius': ['step', ['get', 'point_count'], 16, 10, 20, 50, 26],
        'circle-stroke-width': 2,
        'circle-stroke-color': mapTokenColor('--map-device-outline'),
      },
    })
  }
  installDeviceMarkerImages(map)
  if (!map.getLayer('device-point')) {
    map.addLayer({
      id: 'device-point',
      type: 'symbol',
      source: 'devices',
      filter: ['!', ['has', 'point_count']],
      layout: {
        'icon-image': ['concat', 'device-dot-', ['get', 'status']],
        // 圆点以坐标为中心对位：不需要针尖偏移，点位就是圆心。
        'icon-anchor': 'center',
        'icon-size': ['interpolate', ['linear'], ['zoom'], 3, 0.8, 18, 1],
        'icon-allow-overlap': true,
        'icon-ignore-placement': true,
      },
      paint: {
        'icon-opacity': ['case', ['==', ['get', 'stale'], 1], 0.7, 1],
      },
    })
  }
  // 状态脉冲的外圈光晕。
  //
  // 必须**垫在 device-point 下层**：光晕半径最大能到圆点的 4 倍，压在上面会把
  // 圆点的咬边（surface 色描边）糊掉，看起来像圆点"发虚"。
  // 必须用 circle 而不是再画一张 symbol 图：radius/opacity 是 paint 属性，
  // 逐帧更新不会触发重新布局；icon-size 是 layout 属性，每帧改会让整张图的
  // 碰撞检测重算一遍。
  //
  // 两层顺序：wave 先画（在下），core 后画（在上）。同心的两个圆里，
  // 让柔光压住扩散环的内沿，环就只剩"外扩的一段"，边界更利落。
  const pulseFilter = buildPulseFilter(pulseStatuses.value)
  if (!map.getLayer(PULSE_WAVE_LAYER_ID)) {
    map.addLayer(
      {
        id: PULSE_WAVE_LAYER_ID,
        type: 'circle',
        source: 'devices',
        filter: pulseFilter,
        paint: {
          // 只要描边：填充透明，避免把圆点周围糊成一块色斑。
          'circle-color': 'rgba(0, 0, 0, 0)',
          'circle-opacity': 0,
          'circle-stroke-color': pulseColorExpression(),
          // 2.4px 是"看得见但不粗糙"的下限；再细在浅色路网上会被压没。
          'circle-stroke-width': 2.4,
          'circle-stroke-opacity': 0.9,
          'circle-radius': 12,
          // 柔化让环的边缘有光感，但比旧版的 0.45 更收敛 ——
          // 过度模糊会把环糊成"一片灰"，那正是旧版看不出来的原因之一。
          'circle-blur': 0.42,
          // 3D 下必须用视口对齐：circle 默认按地图平面绘制，pitch 后会被压成椭圆，
          // 而 symbol 图层的圆点在任何俯角下都是正圆 —— 不一致就是"光晕歪了"。
          'circle-pitch-alignment': 'viewport',
        },
      },
      'device-point',
    )
  }
  if (!map.getLayer(PULSE_LAYER_ID)) {
    map.addLayer(
      {
        id: PULSE_LAYER_ID,
        type: 'circle',
        source: 'devices',
        filter: pulseFilter,
        paint: {
          'circle-color': pulseColorExpression(),
          'circle-radius': 12,
          'circle-opacity': 0.5,
          // 0.18：比扩散环更实，作为"闪"的亮度主体。
          'circle-blur': 0.18,
          'circle-pitch-alignment': 'viewport',
        },
      },
      'device-point',
    )
  }
  if (!map.getLayer('device-selected')) {
    map.addLayer({
      id: 'device-selected',
      type: 'symbol',
      source: 'devices',
      filter: ['==', ['get', 'device_code'], selectedCode.value || '__none__'],
      layout: {
        'icon-image': ['concat', 'device-dot-selected-', ['get', 'status']],
        'icon-anchor': 'center',
        'icon-size': ['interpolate', ['linear'], ['zoom'], 3, 0.8, 18, 1],
        'icon-allow-overlap': true,
        'icon-ignore-placement': true,
      },
    })
  }
  if (!map.getSource('draw-preview')) {
    map.addSource('draw-preview', { type: 'geojson', data: EMPTY_FC })
  }
  // 半透明填充：只要轮廓的话，用户看不出"这个范围到底圈住了多少个设备"。
  if (!map.getLayer('draw-preview-fill')) {
    map.addLayer({
      id: 'draw-preview-fill',
      type: 'fill',
      source: 'draw-preview',
      filter: ['==', ['geometry-type'], 'Polygon'],
      paint: {
        'fill-color': isDark.value ? '#409CFF' : '#3B82F6',
        'fill-opacity': 0.12,
      },
    })
  }
  if (!map.getLayer('draw-preview')) {
    map.addLayer({
      id: 'draw-preview',
      type: 'line',
      source: 'draw-preview',
      paint: {
        'line-color': isDark.value ? '#409CFF' : '#3B82F6',
        'line-width': 2,
        'line-dasharray': [2, 2],
      },
    })
  }

  // 顶点：已点下的点是实心，跟随鼠标的点是空心 —— 这样"我点到哪儿了"和
  // "光标现在在哪儿"不会混在一起，也不会有人以为已经点下去了。
  if (!map.getSource('draw-vertices')) {
    map.addSource('draw-vertices', { type: 'geojson', data: EMPTY_FC })
  }
  if (!map.getLayer('draw-vertices')) {
    map.addLayer({
      id: 'draw-vertices',
      type: 'circle',
      source: 'draw-vertices',
      paint: {
        'circle-radius': ['case', ['==', ['get', 'kind'], 'hover'], 5, 4],
        'circle-color': ['case', ['==', ['get', 'kind'], 'hover'], '#FFFFFF', isDark.value ? '#409CFF' : '#3B82F6'],
        'circle-stroke-width': 2,
        'circle-stroke-color': ['case', ['==', ['get', 'kind'], 'hover'], isDark.value ? '#409CFF' : '#3B82F6', '#FFFFFF'],
      },
    })
  }

  // 暗层必须夹在「底图」与「业务图层」之间。setStyle 会重建图层顺序，
  // 所以每次安装后都要重新确认位置 —— 否则半透明暗层会盖住设备点。
  //
  // 但必须是**幂等**的：moveLayer 会置 _layerOrderChanged 并让 style 重新
  // 下发 styledata，无条件调用就变成 styledata → moveLayer → styledata 的
  // 逐帧自激，整页跟着卡。只在顺序真的不对时才动它。
  const layerOrder = map.getLayersOrder()
  const darkIndex = layerOrder.indexOf('dark-overlay')
  const fenceIndex = layerOrder.indexOf('fence-fill')
  if (darkIndex !== -1 && fenceIndex !== -1 && darkIndex !== fenceIndex - 1) {
    map.moveLayer('dark-overlay', 'fence-fill')
  }

  // setStyle 会重建所有图层 ⇒ 每次安装后都要把用户的元素开关重放一遍。
  applyElementPreferences()
  // 脉冲图层的 filter / 配色 / 动画状态同样依赖"图层刚被重建"这件事。
  applyPulseLayer()
  syncPulseAnimation()
  bindLayerEvents()
  bindClusterLabelEvents()
}

// ===================== 聚合数字标注（DOM） =====================

/**
 * 聚合数字用 DOM 标注而非 symbol 图层。
 *
 * 原因见 installOverlayLayers 中的注释：栅格底图没有 glyphs。
 * 这里维护 id -> Marker 的映射，增量增删改，避免每次重建导致闪动。
 */
const clusterLabels = new Map()
let clusterLabelFrame = null

function bindClusterLabelEvents() {
  if (!map) return
  const pairs = [
    ['moveend', scheduleClusterLabels],
    ['zoomend', scheduleClusterLabels],
    ['sourcedata', handleSourceData],
  ]
  pairs.forEach(([type, listener]) => {
    map.off(type, listener)
    map.on(type, listener)
  })
}

function handleSourceData(event) {
  if (event.sourceId === 'devices' && event.isSourceLoaded) scheduleClusterLabels()
}

function scheduleClusterLabels() {
  if (clusterLabelFrame != null) return
  clusterLabelFrame = requestAnimationFrame(() => {
    clusterLabelFrame = null
    refreshClusterLabels()
  })
}

function refreshClusterLabels() {
  if (!map || !map.getSource('devices')) return
  // 设备图层被关掉时不要再造 DOM 标注，否则会出现"数字飘在空地图上"。
  if (elementPrefs.devices === false) {
    clearClusterLabels()
    return
  }
  // 只在具备聚合能力时查询，否则 querySourceFeatures 会抛出
  let features = []
  try {
    features = map.querySourceFeatures('devices', { filter: ['has', 'point_count'] }) || []
  } catch (error) {
    return
  }

  const seen = new Set()
  features.forEach((feature) => {
    const clusterId = feature.properties?.cluster_id
    const coordinates = feature.geometry?.coordinates
    const count = feature.properties?.point_count
    if (clusterId == null || !coordinates || !count) return
    if (seen.has(clusterId)) return
    seen.add(clusterId)

    let marker = clusterLabels.get(clusterId)
    const text = count > 999 ? '999+' : String(count)
    if (!marker) {
      // 先写好文本再 addTo：Marker 用元素的布局尺寸换算 center 锚点，
      // 空 div 会被量成 0×0，导致数字偏到圆右下角。
      const element = document.createElement('div')
      element.className = 'map-cluster-label'
      element.textContent = text
      marker = new Marker({ element, anchor: 'center' }).setLngLat(coordinates).addTo(map)
      clusterLabels.set(clusterId, marker)
    }
    marker.setLngLat(coordinates)
    const element = marker.getElement()
    if (element.textContent !== text) element.textContent = text
  })

  clusterLabels.forEach((marker, clusterId) => {
    if (seen.has(clusterId)) return
    marker.remove()
    clusterLabels.delete(clusterId)
  })
}

function clearClusterLabels() {
  clusterLabels.forEach((marker) => marker.remove())
  clusterLabels.clear()
  if (clusterLabelFrame != null) {
    cancelAnimationFrame(clusterLabelFrame)
    clusterLabelFrame = null
  }
}

/**
 * 图层事件用「先解绑再绑定」而非「只绑一次」。
 *
 * installOverlayLayers 会在每次 style 变化时被调用（幂等安装图层）。
 * 不用 once 标志位是因为不确定 setStyle 是否会连带清掉委派监听器
 * （清了会丢交互、没清会重复触发），先 off 再 on 在两种情况下都正确。
 */
function bindLayerEvents() {
  if (!map) return
  const pairs = [
    ['mouseenter', 'device-point', handlePointEnter],
    ['mouseleave', 'device-point', handlePointLeave],
    ['click', 'device-point', handlePointClick],
    ['click', 'device-cluster', handleClusterClick],
    ['mouseenter', 'device-point', handlePointCursorIn],
    ['mouseleave', 'device-point', handlePointCursorOut],
    // 围栏用 mousemove 而非 mouseenter：标签跟随光标，不会停在"进入点"上。
    ['mousemove', 'fence-fill', handleFenceHover],
    ['mouseleave', 'fence-fill', handleFenceLeave],
  ]
  pairs.forEach(([type, layerId, listener]) => {
    map.off(type, layerId, listener)
    map.on(type, layerId, listener)
  })
}

function handlePointCursorIn() {
  if (map && !drawing.mode) map.getCanvas().style.cursor = 'pointer'
}

function handlePointCursorOut() {
  if (map && !drawing.mode) map.getCanvas().style.cursor = ''
}

function setPaintIfExists(layerId, paint) {
  if (map && map.getLayer(layerId)) map.setPaintProperty(layerId, Object.keys(paint)[0], Object.values(paint)[0])
}

// 主题变化时刷新所有依颜色的图层
watch(isDark, () => {
  if (!map || !mapReady) return
  if (map.getLayer('dark-overlay')) {
    map.setLayoutProperty('dark-overlay', 'visibility', isDark.value ? 'visible' : 'none')
  }
  installDeviceMarkerImages(map, true)
  // 脉冲光晕的配色来自状态令牌，主题切换后必须重取一次。
  applyPulseLayer()
  const stroke = mapTokenColor('--map-device-outline')
  setPaintIfExists('device-cluster', { 'circle-stroke-color': stroke })
  setPaintIfExists('device-cluster', { 'circle-color': mapTokenColor('--color-brand') })
  for (const layerId of ['fence-fill', 'fence-line', 'fence-line-halo']) {
    if (map.getLayer(layerId)) {
      map.setPaintProperty(layerId, layerId === 'fence-fill' ? 'fill-color' : 'line-color', [
        'match',
        ['get', 'rule_type'],
        'forbid',
        fenceAccent('forbid', isDark.value),
        'keep_in',
        fenceAccent('keep_in', isDark.value),
        fenceAccent('notify', isDark.value),
      ])
    }
  }
  if (map.getLayer('draw-preview')) {
    map.setPaintProperty('draw-preview', 'line-color', isDark.value ? '#409CFF' : '#3B82F6')
  }
  if (map.getLayer('draw-preview-fill')) {
    map.setPaintProperty('draw-preview-fill', 'fill-color', isDark.value ? '#409CFF' : '#3B82F6')
  }
  if (map.getLayer('draw-vertices')) {
    const accent = isDark.value ? '#409CFF' : '#3B82F6'
    map.setPaintProperty('draw-vertices', 'circle-color', ['case', ['==', ['get', 'kind'], 'hover'], '#FFFFFF', accent])
    map.setPaintProperty('draw-vertices', 'circle-stroke-color', [
      'case',
      ['==', ['get', 'kind'], 'hover'],
      accent,
      '#FFFFFF',
    ])
  }
})

watch(
  () => selectedCode.value,
  (code) => {
    if (!map || !map.getLayer('device-selected')) return
    map.setFilter('device-selected', ['==', ['get', 'device_code'], code || '__none__'])
  },
)

// 设备重新有位置后清掉"提示卡已关闭"的记忆：否则下次再一台都没定位时，
// 这条引导就永久消失了（用户只关过一次，不该等同于"再也不想看"）。
watch(
  () => locatedDevices.value.length > 0,
  (hasLocation) => {
    if (!hasLocation || !emptyStateDismissed.value) return
    emptyStateDismissed.value = false
    persistEmptyStateDismissed(false)
  },
)

// ===================== 数据 =====================

/**
 * 拉取设备与围栏。
 *
 * `fit: false` 用于**用户已经站在正确视角**的场景（手动落点、清除落点）：
 * 重新拟合视野会把刚点下去的位置甩走，属于纯打扰。
 */
async function loadData({ fit = true } = {}) {
  loading.value = true
  try {
    const [mapData, fenceData] = await Promise.all([fetchMapDevices(), fetchGeofences()])
    devices.value = mapData.devices
    fences.value = fenceData.fences
    fenceStates.value = fenceData.states
    syncSources()
    // 首次加载必须拟合：没有已定位设备时它会退到中国全境视野，
    // 否则地图会停在初始的中国中心但缩放不准，看起来像"没定位"。
    if (fit) fitAllDevices()
  } catch (error) {
    showToast('danger', `${t('device_map_load_failed')}: ${error.message}`)
  } finally {
    loading.value = false
  }
}

function syncSources() {
  if (!map) return
  const deviceSource = map.getSource('devices')
  if (deviceSource) {
    deviceSource.setData(
      devicesToFeatureCollection(devices.value, {
        fenceStates: fenceStates.value,
        fences: fences.value,
      }),
    )
  }
  const fenceSource = map.getSource('fences')
  if (fenceSource) fenceSource.setData(fencesToFeatureCollection(fences.value, selectedFenceId.value))
  refreshDrawingPreview()
  scheduleClusterLabels()
}

async function reloadFences() {
  const fenceData = await fetchGeofences()
  fences.value = fenceData.fences
  fenceStates.value = fenceData.states
  syncSources()
}

// ===================== 实时推送 =====================
//
// 这一页是"盯着看"的场景：设备上下线、围栏跃迁、位置移动如果都要等用户手动刷新，
// 地图上的颜色就只是"进入页面那一刻的快照"，越看越不可信。
//
// 两条通道分工明确，不要互相替代：
//   · SSE（即时）：后端把事件推进来，前端**就地改本地数据**，不发网络请求，
//     所以能秒级可见，也不会因为一个事件引发一次全量重绘的雪崩。
//   · 低频校准（兜底）：60s 一次静默全量拉取。覆盖三类缺口 —— SSE 丢事件、
//     断线重连窗口内的变更、以及事件里没带全的字段。
//
// 权限不足（缺 device:list）时 SSE 会 403，直接退化为 20s 轮询：
// 宁可慢一点，也不能让页面停在打开时那一刻。
const REALTIME_EVENTS = [
  'device.status.changed',
  'device.list.changed',
  'geofence.triggered',
  'device.location.changed',
]
// 一台设备组网批量上下线时事件会成串到达，合并成一次 GeoJSON 重设。
const LAYER_SYNC_DELAY_MS = 90
// 设备增删没法就地合并（缺整条记录），需要重拉；同样做合并，避免风暴式请求。
const DEVICE_RELOAD_DELAY_MS = 800
const SILENT_REFRESH_MS = 60 * 1000
const SILENT_REFRESH_FALLBACK_MS = 20 * 1000

let unsubscribeRealtime = null
let layerSyncTimer = 0
let deviceReloadTimer = 0
let silentRefreshTimer = 0
// onMounted 里有 await（地图初始化 + 首次加载）。用户在这段时间内切走页面时，
// 卸载钩子会先跑完，之后才轮到 startRealtimeUpdates —— 没有这个标记，
// 连接会在组件已销毁后建立，而且再也没人负责关掉它。
let realtimeDisposed = false

function findMapDevice(code) {
  return devices.value.find((device) => device.device_code === code)
}

function scheduleLayerSync() {
  if (layerSyncTimer) return
  layerSyncTimer = window.setTimeout(() => {
    layerSyncTimer = 0
    syncSources()
  }, LAYER_SYNC_DELAY_MS)
}

function scheduleDeviceReload() {
  if (deviceReloadTimer) return
  deviceReloadTimer = window.setTimeout(() => {
    deviceReloadTimer = 0
    refreshDevicesSilently()
  }, DEVICE_RELOAD_DELAY_MS)
}

/**
 * 用新列表整体替换设备数据，并把悬停卡重新指到新对象上。
 *
 * 不 remap 的话，卡片会继续持有已经不在列表里的旧对象 —— 数值定格、状态不再
 * 更新，看起来像"卡住了"，而用户的第一反应是"实时推送失效了"。
 */
function applyDeviceList(next) {
  const hoverCode = hoverDevice.value?.device_code || ''
  devices.value = next
  if (!hoverCode) return
  const stillThere = next.find((device) => device.device_code === hoverCode)
  if (stillThere) {
    hoverDevice.value = stillThere
  } else {
    hoverDevice.value = null
    resetHoverProbe()
  }
}

/** 设备在线状态：后端推的是 `{Topic: <code>, Payload: 'online' | 'offline'}`。 */
function applyDeviceStatusEvent(payload) {
  const code = payload?.Topic
  if (!code) return
  const device = findMapDevice(code)
  if (!device) {
    // 列表里没有这台 ⇒ 是本端还没见过的新设备（或已被删），只能重拉。
    scheduleDeviceReload()
    return
  }
  const online = payload.Payload === 'online' || payload.Payload === true
  if (device.online === online) return
  device.online = online
  if (online && payload.Timestamp) device.last_active = new Date(payload.Timestamp).toISOString()
  scheduleLayerSync()
  // 悬停卡的"状态"读的是同一个对象，Vue 的响应式会跟着刷新；
  // 但卡片的实时属性区是悬停时一次性取的，这里不强刷 —— 强刷会让数值突跳。
}

/**
 * 围栏跃迁：`{device_code, fence_id, from, to}`。
 *
 * 直接 upsert 本地判定表，不重拉 /fences/states：一次跃迁只需要改一行，
 * 重拉会把整张表（可能上千行）搬一遍，而跃迁是高频事件。
 */
function applyGeofenceEvent(payload) {
  const code = payload?.device_code
  const fenceId = Number(payload?.fence_id)
  const state = payload?.to
  if (!code || !Number.isFinite(fenceId)) return
  if (state !== 'inside' && state !== 'outside') return

  const index = fenceStates.value.findIndex(
    (entry) => Number(entry.fence_id) === fenceId && entry.device_code === code,
  )
  if (index >= 0) {
    if (fenceStates.value[index].state === state) return
    // 整项替换而不是就地改：fenceStates 是 ref 里的数组，
    // 替换元素能让依赖它的 computed 可靠地重新求值。
    fenceStates.value[index] = { ...fenceStates.value[index], state }
  } else {
    fenceStates.value.push({ fence_id: fenceId, device_code: code, state })
  }
  scheduleLayerSync()
}

/** 位置变更：坐标 / 来源 / 精度 / fix_at 一起落到本地设备对象上。 */
function applyDeviceLocationEvent(payload) {
  const code = payload?.device_code
  if (!code) return
  const device = findMapDevice(code)
  if (!device) return
  const longitude = Number(payload.longitude)
  const latitude = Number(payload.latitude)
  if (!Number.isFinite(longitude) || !Number.isFinite(latitude)) return
  // 静止设备也有 30s 一次的心跳落库（保证 last fix 不过期），那些事件带的是
  // 完全相同的坐标。没有这个守卫，地图会每 30s 无谓地重建一次点位数据。
  const nextFixAt = payload.fix_at || 0
  const nextSource = payload.source || device.source
  const unchanged =
    device.longitude === longitude &&
    device.latitude === latitude &&
    (device.fix_at || 0) === nextFixAt &&
    device.source === nextSource
  if (unchanged) return

  device.has_location = true
  device.longitude = longitude
  device.latitude = latitude
  device.source = nextSource
  if (payload.accuracy_m != null) device.accuracy_m = payload.accuracy_m
  if (payload.fix_at) device.fix_at = payload.fix_at
  if (payload.received_at) device.received_at = payload.received_at
  scheduleLayerSync()
}

function handleRealtimeEvent(type, payload) {
  switch (type) {
    case 'device.status.changed':
      applyDeviceStatusEvent(payload)
      break
    case 'device.list.changed':
      scheduleDeviceReload()
      break
    case 'geofence.triggered':
      applyGeofenceEvent(payload)
      break
    case 'device.location.changed':
      applyDeviceLocationEvent(payload)
      break
    default:
      break
  }
}

/** 设备集合指纹：用来判断静默拉取的结果是否真的变了，没变就不碰图层。 */
function deviceSignature(list) {
  return list
    .map(
      (device) =>
        `${device.device_code}:${device.online ? 1 : 0}:${device.fix_at || 0}:${device.longitude ?? ''}:${device.latitude ?? ''}`,
    )
    .join('|')
}

/**
 * 静默全量刷新。
 *
 * 与 loadData 的区别是三条：不设 loading（不盖加载层）、不拟合视野（不抢相机）、
 * 失败不弹提示（校准是后台动作，失败就等下一轮）。
 */
async function refreshDevicesSilently() {
  if (document.hidden || loading.value) return
  try {
    const mapData = await fetchMapDevices()
    const next = mapData.devices || []
    if (deviceSignature(next) === deviceSignature(devices.value)) return
    applyDeviceList(next)
    syncSources()
  } catch (error) {
    /* 静默失败：下一轮再来 */
  }
}

function startSilentRefresh(interval) {
  if (silentRefreshTimer) clearInterval(silentRefreshTimer)
  silentRefreshTimer = window.setInterval(refreshDevicesSilently, interval)
}

function startRealtimeUpdates() {
  if (realtimeDisposed) return
  // 事件流接口要 device:list 权限（地图页本身只要求 device_map:view），
  // 权限不够时不必尝试建连 —— EventSource 会拿着 403 无限重连，
  // 既刷日志又占连接槽。
  if (!authStore.hasPermission('device:list')) {
    startSilentRefresh(SILENT_REFRESH_FALLBACK_MS)
    return
  }

  unsubscribeRealtime = subscribeDeviceEvents({
    events: REALTIME_EVENTS,
    onEvent: handleRealtimeEvent,
    // 每次（重）连成功都校准一次：断线期间的事件是永久丢失的，
    // 只靠后续事件补不回来。
    onOpen: () => {
      startSilentRefresh(SILENT_REFRESH_MS)
      refreshDevicesSilently()
    },
    onError: () => {
      startSilentRefresh(SILENT_REFRESH_FALLBACK_MS)
    },
  })
  startSilentRefresh(SILENT_REFRESH_MS)
}

function stopRealtimeUpdates() {
  realtimeDisposed = true
  if (unsubscribeRealtime) {
    unsubscribeRealtime()
    unsubscribeRealtime = null
  }
  if (layerSyncTimer) {
    clearTimeout(layerSyncTimer)
    layerSyncTimer = 0
  }
  if (deviceReloadTimer) {
    clearTimeout(deviceReloadTimer)
    deviceReloadTimer = 0
  }
  if (silentRefreshTimer) {
    clearInterval(silentRefreshTimer)
    silentRefreshTimer = 0
  }
}

// ===================== 底图切换 =====================

function selectBasemap(id) {
  if (!resolveStyleFor(id)) {
    basemapMenuOpen.value = true
    showToast('warning', t('device_map_token_required'))
    return
  }
  basemapId.value = id
  saveBasemapPreference(id)
  basemapMenuOpen.value = false
  applyBasemap()
}

function applyTiandituToken() {
  const token = tiandituTokenDraft.value.trim()
  if (!token) {
    showToast('warning', t('device_map_token_empty'))
    return
  }
  tiandituToken.value = token
  tiandituTokenDraft.value = token
  saveTiandituToken(token)
  // 填完 Key 后：如果当前就是需要 Key 的底图（矢量/影像），保持在原底图上；
  // 否则跳到默认矢量底图 —— 用户填 Key 的目的就是要看天地图。
  selectBasemap(getProvider(basemapId.value)?.requiresToken ? basemapId.value : 'tianditu')
  showToast('success', t('device_map_token_saved'))
}

// ===================== 图层元素可见性 =====================

function applyElementPreferences() {
  if (!map) return
  applyElementVisibility(map, elementPrefs)
}

function toggleElement(id) {
  elementPrefs[id] = elementPrefs[id] === false
  saveElementPrefs(elementPrefs)
  applyElementPreferences()
  // 聚合数字是 DOM 标注，不在样式里，得单独收放。
  if (id === 'devices') {
    if (elementPrefs.devices) scheduleClusterLabels()
    else clearClusterLabels()
    // 点位被关掉时脉冲圈没有任何意义，必须连动画一起停，不能让它空转。
    syncPulseAnimation()
  }
}

// ===================== 2D / 3D 视角 =====================

function loadDimensionPreference() {
  try {
    return window.localStorage.getItem(DIMENSION_STORAGE_KEY) === '3d' ? '3d' : '2d'
  } catch (error) {
    return '2d'
  }
}

function saveDimensionPreference(value) {
  try {
    window.localStorage.setItem(DIMENSION_STORAGE_KEY, value)
  } catch (error) {
    /* ignore */
  }
}

function setMapDimension(mode) {
  const next = mode === '3d'
  if (next === is3D.value) return
  is3D.value = next
  saveDimensionPreference(next ? '3d' : '2d')
  if (!map) return
  if (next) {
    map.easeTo({ pitch: MAP_3D_PITCH, duration: 700 })
    ensureCompass()
  } else {
    // 回到 2D 必须同时把正北复位，否则"平视但地图是斜的"会让人以为导航坏了。
    map.easeTo({ pitch: 0, bearing: 0, duration: 700 })
    removeCompass()
  }
}

function ensureCompass() {
  if (!map || compassControl) return
  compassControl = new NavigationControl({ showZoom: false, showCompass: true, visualizePitch: true })
  map.addControl(compassControl, 'top-right')
}

function removeCompass() {
  if (!map || !compassControl) return
  map.removeControl(compassControl)
  compassControl = null
}

function applyBasemap() {
  if (!map) return
  const style = resolveStyleFor(basemapId.value)
  if (!style) {
    showToast('warning', t('device_map_token_required'))
    return
  }
  tileError.value = false
  // 换底图 = 样式重来一遍，在就绪之前同样不能拟合（同 fitAllDevices 里的原因）。
  mapReady = false
  clearClusterLabels()
  map.setStyle(style, { diff: false })
}

function retryBasemap() {
  tileError.value = false
  applyBasemap()
}

// ===================== 交互 =====================

function zoomBy(delta) {
  if (!map) return
  map.easeTo({ zoom: map.getZoom() + delta, duration: 200 })
}

// ===================== 视窗拟合 =====================

// 拟合留白（px）。右上角压着缩放/全览/围栏四个控件，留够 72 才不会把设备点压在控件底下。
const FIT_PADDING = 72
// 「能显示大部分设备」所用的最大缩放：再放大就会把设备挤出视野。
const FIT_MAX_ZOOM = 16
// 只有一台设备（或所有设备重合）时的定位缩放。
const SINGLE_DEVICE_ZOOM = 15
// 没有任何设备时的默认视野：中国全境。
//
// 南界取到 3.4° 是为了**完整包含南海诸岛**，东界含台湾 —— 大陆合规要求
// 中国地图必须完整表达领土，缺一块就是"问题地图"。
const CHINA_BOUNDS = [
  [73.4, 3.4],
  [135.1, 53.6],
]
// IP 粗定位在城市级（误差数公里到数十公里），对应的缩放要能看清一座城市
// 及其周边。10 级在 1280px 宽的画布上大约覆盖 250 公里，正好是"我在这座城市"。
const IP_LOCATION_ZOOM = 10
// 至少要让这么多比例的设备同时进入视野。
const FOCUS_KEEP_RATIO = 0.8
// 设备总数不超过这个值时一律全保留 —— 数量这么少时任何一台都可能是主角，裁掉就是丢信息。
const FOCUS_TRIM_MIN_TOTAL = 5
// (0,0) 一律视为无有效定位。
const ZERO_COORD_EPSILON = 1e-6

function isValidLngLat(longitude, latitude) {
  if (!Number.isFinite(longitude) || !Number.isFinite(latitude)) return false
  if (Math.abs(latitude) > 90 || Math.abs(longitude) > 180) return false
  // 物模型点位未上报时经常是 0；(0,0) 会把视窗硬拉到几内亚湾，
  // 于是真实设备全部被挤成一个点，表现为「地图缩到了一个莫名其妙的地方」。
  return !(Math.abs(longitude) < ZERO_COORD_EPSILON && Math.abs(latitude) < ZERO_COORD_EPSILON)
}

function medianOf(values) {
  const sorted = [...values].sort((left, right) => left - right)
  const middle = Math.floor(sorted.length / 2)
  return sorted.length % 2 ? sorted[middle] : (sorted[middle - 1] + sorted[middle]) / 2
}

/**
 * 计算「大多数设备」所在的视窗。
 *
 * 为什么不直接对全部设备 fitBounds：只要有一台坐标是错的（或部署在另一个省），
 * 整张图就会被压到国家级视野，真正关心的设备全挤成一个小点 —— 看上去就是
 * 「缩得太远，什么都看不清」。
 *
 * 规则：
 *  1. 以经纬度**中位数**作为稳健中心（平均值会被单个离群点直接拉走，中位数不会）；
 *  2. 按到中心的距离升序，保留 K = max(2, ceil(n × 0.8)) 台；
 *  3. 用这 K 台的包围盒作为视窗。在「至少显示 80% 设备」这个前提下，该盒子最小，
 *     因此 fitBounds 得到的缩放，就是**能同时显示大部分设备的最大放大倍数**；
 *  4. 设备数 ≤ 4 时不裁剪。
 *
 * @returns {null | { single: [number, number] } | { bounds: LngLatBounds, kept: number, total: number }}
 */
function computeFocusBounds(points) {
  const valid = points.filter(([longitude, latitude]) => isValidLngLat(longitude, latitude))
  if (!valid.length) return null
  if (valid.length === 1) return { single: valid[0] }

  let kept = valid
  if (valid.length > FOCUS_TRIM_MIN_TOTAL) {
    const centerLongitude = medianOf(valid.map((point) => point[0]))
    const centerLatitude = medianOf(valid.map((point) => point[1]))
    // 经度差按纬度收敛：同样的经度差，纬度越高对应的地面距离越短。
    const longitudeScale = Math.max(Math.cos((centerLatitude * Math.PI) / 180), 0.1)
    const distanceSquare = ([longitude, latitude]) => {
      const deltaLongitude = (longitude - centerLongitude) * longitudeScale
      const deltaLatitude = latitude - centerLatitude
      return deltaLongitude * deltaLongitude + deltaLatitude * deltaLatitude
    }
    const keepCount = Math.max(2, Math.ceil(valid.length * FOCUS_KEEP_RATIO))
    kept = [...valid].sort((left, right) => distanceSquare(left) - distanceSquare(right)).slice(0, keepCount)
  }

  const bounds = new LngLatBounds()
  kept.forEach((point) => bounds.extend(point))
  return { bounds, kept: kept.length, total: valid.length }
}

/** 把算好的视窗真正应用到相机上。 */
function applyFocus(focus, duration) {
  if (!map) return
  if (focus.single) {
    map.easeTo({ center: focus.single, zoom: SINGLE_DEVICE_ZOOM, duration })
    return
  }

  // 所有保留点重合（例如整栋楼的设备都在同一个坐标）时包围盒面积为零。
  // fitBounds 对这种退化盒子的处理依赖实现细节，直接走定点定位更可控。
  const southWest = focus.bounds.getSouthWest()
  const northEast = focus.bounds.getNorthEast()
  const spansNothing =
    Math.abs(northEast.lng - southWest.lng) < ZERO_COORD_EPSILON &&
    Math.abs(northEast.lat - southWest.lat) < ZERO_COORD_EPSILON
  if (spansNothing) {
    map.easeTo({ center: [southWest.lng, southWest.lat], zoom: SINGLE_DEVICE_ZOOM, duration })
    return
  }

  map.fitBounds(focus.bounds, { padding: FIT_PADDING, maxZoom: FIT_MAX_ZOOM, duration })
}

/** 把视野拉到中国全境。IP 粗定位拿不到时它就是"无设备"的默认含义。 */
function fitChinaView(duration = 600) {
  if (!map) return
  map.fitBounds(CHINA_BOUNDS, { padding: 0, duration })
  // 同 fitAllDevices：样式未就绪时相机设对了但瓦片覆盖不重算，会露白。
  if (!mapReady) {
    map.once('load', () => {
      mapReady = true
      map.fitBounds(CHINA_BOUNDS, { padding: 0, duration: 0 })
    })
  }
}

function markUserMovedMap() {
  userMovedMap = true
}

/**
 * 取一次「浏览器所在网络的公网 IP」粗定位。
 *
 * 只问一次：结果对整个页面生命周期有效，失败也不再重试 —— 它只决定首次视野，
 * 失败时中国全境就是答案，没有重试的必要。
 */
function requestIpLocation() {
  if (ipLocationRequest) return ipLocationRequest
  ipLocationRequest = fetchIpLocation().then((location) => {
    ipLocation = location
    return location
  })
  return ipLocationRequest
}

/** 把视野收到 IP 粗定位的城市上。IP 定位拿不到时什么都不做（保持原来的视野）。 */
function focusIpLocation(duration) {
  if (!map || !ipLocation) return
  const center = [ipLocation.longitude, ipLocation.latitude]
  map.easeTo({ center, zoom: IP_LOCATION_ZOOM, duration })
  // 同 fitChinaView：样式未就绪时相机对了但瓦片覆盖不重算，load 后再补一次。
  if (!mapReady) {
    map.once('load', () => {
      mapReady = true
      map.easeTo({ center, zoom: IP_LOCATION_ZOOM, duration: 0 })
    })
  }
}

/**
 * 一台有位置的设备都没有时的默认视野。
 *
 * 顺序：先按公网 IP 粗定位到"当前所处的城市"，拿不到才退回中国全境。
 * 不在拿到 IP 之前干等 —— 先把中国全境铺好，结果回来再平滑收窄，
 * 期间用户看到的一直是一张有内容的图。
 *
 * 收窄有三个退出条件，缺一个就会变成"地图自己乱跳"：
 *   1. 这次拟合已被后来的拟合取代（defaultViewSeq 变了）；
 *   2. 期间设备已经有位置了（IP 视野没有意义了）；
 *   3. 用户自己动过地图（他显然已经知道自己在看哪，抢相机就是纯打扰）。
 */
function fitDefaultView() {
  const duration = mapReady ? 600 : 0
  const seq = ++defaultViewSeq

  if (ipLocation) {
    focusIpLocation(duration)
    return
  }

  fitChinaView(duration)
  requestIpLocation().then((location) => {
    if (!location) return
    if (seq !== defaultViewSeq) return
    if (locatedDevices.value.length > 0) return
    if (userMovedMap) return
    focusIpLocation(600)
  })
}

function fitAllDevices() {
  if (!map) return

  const points = locatedDevices.value.map((device) => [Number(device.longitude), Number(device.latitude)])
  const focus = computeFocusBounds(points)
  if (!focus) {
    // 一台有位置的设备都没有：先按 IP 粗定位到所在城市，拿不到就退到中国全境。
    // 空白底图（或停在某个随机城市）会让人以为地图坏了。
    fitDefaultView()
    return
  }

  // 有设备了：默认视野的异步回调一律作废，别在设备拟好之后再抢相机。
  defaultViewSeq += 1

  // 先无条件拟合：视野正确是硬要求，任何"等就绪再拟合"的门都可能因事件顺序
  // 丢掉这次拟合（踩过一次：地图停在默认 zoom 4，看起来像没生效）。
  applyFocus(focus, mapReady ? 600 : 0)

  // 样式仍在加载时，相机能设对，但底图的瓦片覆盖不会按新相机重算 ——
  // 表现为「视野对了却一片空白，随手缩放一下就又出来了」。低缩放时因为世界
  // 瓦片本来就能盖住整个视野，这个缺陷被掩盖；拟合到街区级就必然暴露。
  // 所以 load 之后用同一个视窗再补一次（瞬时、幂等）。
  if (!mapReady) {
    map.once('load', () => {
      mapReady = true
      applyFocus(focus, 0)
    })
  }
}

function focusDevice(device) {
  selectedCode.value = device.device_code
  if (device.has_location && map) {
    map.easeTo({ center: [device.longitude, device.latitude], zoom: Math.max(map.getZoom(), 14), duration: 400 })
  } else if (device.mode !== 'telemetry') {
    startDeviceLocationPick(device)
  }
}

function toggleStatusFilter(status) {
  statusFilter.value = statusFilter.value === status ? '' : status
}

function goToDevices() {
  router.push('/devices')
}

// ===================== 地图事件 =====================

function handlePointEnter(event) {
  const feature = event.features?.[0]
  if (!feature) return
  const code = feature.properties?.device_code
  const device = devices.value.find((item) => item.device_code === code)
  if (!device) return
  // 设备点常常压在围栏填充区里，两块浮层同时出现会互相遮挡。
  hoverFence.value = null
  cancelHoverClose()
  const sameDevice = hoverDevice.value?.device_code === device.device_code
  hoverDevice.value = device
  hoverPoint.x = event.point.x
  hoverPoint.y = event.point.y
  // 同一台设备来回微动不重复取数：卡片内容没变，刷新只会让数值闪一下。
  if (!sameDevice) scheduleHoverProbe(device)
}

function handlePointLeave() {
  // 不立即关闭：光标马上会移到卡片上去点按钮（视频 / 移动图钉）。
  scheduleHoverClose()
}

// ===================== 悬停卡：视频点播 =====================

/**
 * 把地图接口的设备对象映射成「协议插件认得的设备对象」。
 *
 * 地图接口用 `device_code` / `device_name` 命名（位置表与设备表的历史差异），
 * 而 deviceActions.condition 与播放器组件都读 `code` / `name`。
 * 这一层映射是刻意保留的边界：不去改动设备表 DTO 的字段名，避免影响其他调用方。
 */
function toActionDevice(device) {
  return {
    ...device,
    code: device.device_code,
    name: device.device_name,
  }
}

/** 能否点播视频，完全由插件声明的 condition 决定，页面不硬编码任何协议名。 */
function canPlayVideo(device) {
  if (!device) return false
  const target = toActionDevice(device)
  return mediaPlayerActions.value.some((action) => (typeof action.condition === 'function' ? action.condition(target) : true))
}

function playDeviceVideo(device) {
  const target = toActionDevice(device)
  const action = mediaPlayerActions.value.find((item) =>
    typeof item.condition === 'function' ? item.condition(target) : true,
  )
  if (!action || typeof action.action !== 'function') {
    showToast('warning', t('device_map_video_unavailable'))
    return
  }
  // 打开弹窗的同时把卡片收掉：弹窗是模态的，卡片留在底下只会挡住视频左上角。
  // 但必须先关掉关闭计时器，否则 220ms 后它再来清一次，会把刚建的弹窗对象状态搅乱。
  resetHoverProbe()
  cancelHoverClose()
  hoverDevice.value = null
  action.action(target, (eventName, payload) => {
    if (eventName === 'open-modal') {
      openExtensionModal(payload.name, action.component, { device: payload.device })
    }
  })
}

function openExtensionModal(name, component, modalProps) {
  const existing = activeExtensionModals.value.find((item) => item.name === name)
  if (existing) {
    existing.props = modalProps
    return
  }
  activeExtensionModals.value.push({ name, component, props: modalProps })
}

function closeExtensionModal(name) {
  activeExtensionModals.value = activeExtensionModals.value.filter((item) => item.name !== name)
}

/**
 * 围栏填充区悬停 ⇒ 显示围栏名称。
 *
 * 名称取要素属性（fencesToFeatureCollection 已带 name），
 * 但"对哪些设备生效"只在原始对象里，所以按 fence_id 回查一次 —— 语料小，无需缓存。
 */
function handleFenceHover(event) {
  if (!map) return
  // 绘制态下每一次移动都属于绘制动作，弹名称会挡住顶点与提示条。
  if (drawing.mode) {
    hoverFence.value = null
    return
  }
  if (hoverDevice.value) {
    hoverFence.value = null
    return
  }
  const feature = event.features?.[0]
  if (!feature) return
  const props = feature.properties || {}
  const id = Number(props.fence_id)
  const fence = fences.value.find((item) => Number(item.ID ?? item.id) === id) || null
  hoverFence.value = {
    name: props.name || fence?.name || t('device_map_fence_polygon'),
    rule_type: props.rule_type || fence?.rule_type || 'keep_in',
    scope: fence ? fenceScopeLabel(fence) : '',
  }
  hoverFencePoint.x = event.point.x
  hoverFencePoint.y = event.point.y
}

function handleFenceLeave() {
  hoverFence.value = null
}

function handlePointClick(event) {
  const feature = event.features?.[0]
  if (!feature) return
  const code = feature.properties?.device_code
  selectedCode.value = code
  panelTab.value = 'devices'
}

function handleClusterClick(event) {
  const feature = event.features?.[0]
  if (!feature || !map) return
  const clusterId = feature.properties?.cluster_id
  map.getSource('devices').getClusterExpansionZoom(clusterId, (error, zoom) => {
    if (error) return
    map.easeTo({ center: feature.geometry.coordinates, zoom, duration: 400 })
  })
}

function handleMapClick(event) {
  const { lng, lat } = event.lngLat
  if (drawing.mode === 'device-location') {
    commitDeviceLocation(lng, lat)
    return
  }
  if (drawing.mode === 'fence-polygon') {
    // 定稿后又点了一下地图 = 想继续加顶点，自动退回绘制态。
    if (drawing.finished) {
      drawing.finished = false
      setDrawPreviewSolid(false)
      if (map) map.getCanvas().style.cursor = 'crosshair'
    }
    drawing.points.push([lng, lat])
    // 刚点下的这个点就是光标位置，避免预览里出现一个原地重叠的空心点。
    drawing.hover = null
    refreshDrawingPreview()
  }
}

// ===================== 地点搜索 =====================

/**
 * 搜索地点并飞到该位置。
 *
 * 结果来自本端后端代理（Photon / OSM），坐标是 WGS-84 —— 与设备、围栏同一套
 * 坐标系，所以直接 flyTo 即可，不做任何偏移转换（延续"坐标单一真源"铁律）。
 */
async function runPlaceSearch() {
  const keyword = placeQuery.value.trim()
  if (!keyword) return
  placeSearching.value = true
  placeError.value = ''
  placePanelOpen.value = true
  try {
    placeResults.value = await searchPlaces(keyword, 8)
    if (placeResults.value.length === 0) placeError.value = t('device_map_search_empty')
  } catch (error) {
    placeResults.value = []
    placeError.value = geoErrorMessage(error, 'search', keyword)
  } finally {
    placeSearching.value = false
  }
}

function clearPlaceSearch() {
  placeQuery.value = ''
  placeResults.value = []
  placeError.value = ''
  placePanelOpen.value = false
}

function goToPlace(place) {
  if (!place) return
  placePanelOpen.value = false
  placeHighlight.value = place
  renderPlaceMarker()
  if (!map) return
  map.flyTo({ center: [Number(place.lng), Number(place.lat)], zoom: 15, duration: 700 })
}

function renderPlaceMarker() {
  if (!map) return
  if (placeMarker) {
    placeMarker.remove()
    placeMarker = null
  }
  const place = placeHighlight.value
  if (!place) return
  // 与聚合数字标注同样的约束：元素不能自带 position（maplibre 用 absolute +
  // transform 定位），也不要吃掉鼠标事件，否则会挡住底下的设备点。
  const element = document.createElement('div')
  element.className = 'map-place-marker'
  element.style.pointerEvents = 'none'
  placeMarker = new Marker({ element, anchor: 'center' })
    .setLngLat([Number(place.lng), Number(place.lat)])
    .addTo(map)
}

// ===================== 按地区名称建围栏 =====================

function openRegionDialog() {
  fenceMenuOpen.value = false
  closeFenceForm()
  Object.assign(regionDialog, { open: true, loading: false, error: '', result: null, forForm: false, query: '' })
  clearRegionPreview()
}

/** 编辑地区围栏时"重新选地区"：带上原名称，结果回填到当前表单。 */
function openRegionDialogForForm() {
  Object.assign(regionDialog, {
    open: true,
    loading: false,
    error: '',
    result: null,
    forForm: true,
    query: fenceForm.region_name || fenceForm.name || '',
  })
}

function closeRegionDialog() {
  regionDialog.open = false
  regionDialog.loading = false
  clearRegionPreview()
}

/**
 * 把地理服务的失败原因翻成用户能行动的一句话。
 *
 * 后端只回原因码，文案在这里决定 —— 这样中英文都归本地管，也不会出现
 * 把上游 URL/EOF 直接摊给用户的情况。「不可达」和「没这个地区」要给
 * 完全不同的下一步动作，二者混成一句"查询失败"等于什么都没说。
 */
function geoErrorMessage(error, scope, query) {
  const reason = error?.reason || ''
  if (reason === 'unreachable') {
    return scope === 'search' ? t('device_map_search_unreachable') : t('device_map_region_unreachable')
  }
  if (reason === 'no_match') {
    return t('device_map_region_no_match', { query })
  }
  if (reason === 'no_boundary') {
    return t('device_map_region_no_boundary')
  }
  return scope === 'search' ? t('device_map_search_failed') : t('device_map_region_failed')
}

async function runRegionLookup() {
  const keyword = regionDialog.query.trim()
  if (!keyword) return
  regionDialog.loading = true
  regionDialog.error = ''
  regionDialog.result = null
  clearRegionPreview()
  try {
    const boundary = await lookupRegionBoundary(keyword)
    regionDialog.result = boundary
    showRegionPreview(boundary)
  } catch (error) {
    regionDialog.error = geoErrorMessage(error, 'region', keyword)
  } finally {
    regionDialog.loading = false
  }
}

/** 把地区边界画到地图上并把视野收进去，让用户在保存前先看清范围。 */
function showRegionPreview(boundary) {
  if (!map || !boundary) return
  let geometry = null
  try {
    geometry = typeof boundary.geometry === 'string' ? JSON.parse(boundary.geometry) : boundary.geometry
  } catch (error) {
    geometry = null
  }
  if (!geometry) return
  const source = map.getSource('region-preview')
  if (source) source.setData({ type: 'FeatureCollection', features: [{ type: 'Feature', geometry, properties: {} }] })
  const bbox = boundary.bbox
  if (Array.isArray(bbox) && bbox.length === 4) {
    map.fitBounds(
      [
        [Number(bbox[0]), Number(bbox[1])],
        [Number(bbox[2]), Number(bbox[3])],
      ],
      { padding: 56, maxZoom: 14, duration: 600 },
    )
  }
}

function clearRegionPreview() {
  if (!map) return
  const source = map.getSource('region-preview')
  if (source) source.setData(EMPTY_FC)
}

/** 把查到的边界带入围栏表单：新建则重置表单，编辑则只替换几何。 */
function useRegionBoundary() {
  const boundary = regionDialog.result
  if (!boundary) return
  const editing = regionDialog.forForm && fenceForm.id
  if (editing) {
    fenceForm.kind = 'polygon'
    fenceForm.source = 'region'
    fenceForm.region_name = boundary.name
    fenceForm.region_level = boundary.level
    fenceForm.geometry = boundary.geometry
    fenceForm.ring = []
    fenceForm.center_lng = null
    fenceForm.center_lat = null
    fenceForm.radius_m = 0
    if (!fenceForm.name) fenceForm.name = boundary.name
  } else {
    openFenceFormForRegion(boundary)
  }
  regionDialog.open = false
  clearRegionPreview()
  cancelDrawing()
}

// ===================== 围栏绘制 =====================

function trackDrawingHover(lng, lat) {
  // 定稿后预览就冻结了，光标移动不再改画面。
  if (drawing.finished) return
  drawing.hover = [lng, lat]
  if (drawingHoverFrame) return
  drawingHoverFrame = requestAnimationFrame(() => {
    drawingHoverFrame = 0
    refreshDrawingPreview()
  })
}

function stopDrawingHover() {
  if (drawingHoverFrame) {
    cancelAnimationFrame(drawingHoverFrame)
    drawingHoverFrame = 0
  }
  drawing.hover = null
}

/** 预览线：绘制中用虚线（"还没定"），定稿后转实线（"就这样了"）。 */
function setDrawPreviewSolid(solid) {
  if (!map || !map.getLayer('draw-preview')) return
  map.setPaintProperty('draw-preview', 'line-dasharray', solid ? [1, 0] : [2, 2])
}

/**
 * 结束绘制并**冻结**预览。
 *
 * 这里刻意不直接打开表单：一条围栏画完，用户常要再看一眼范围对不对，
 * 或者补一个顶点。跳转过去就只能先取消再重画。
 */
function finishFenceDrawing() {
  if (drawing.mode !== 'fence-polygon') return
  if (drawing.finished) return
  if (drawing.points.length < 3) {
    // 少于三点没有面积，不能当围栏。不要静默无视 —— 用户会以为右键坏了。
    showToast('warning', t('device_map_draw_need_three'))
    return
  }
  drawing.finished = true
  stopDrawingHover()
  setDrawPreviewSolid(true)
  if (map) map.getCanvas().style.cursor = ''
  refreshDrawingPreview()
}

/** 从定稿态回到绘制态（"继续加点"）。 */
function resumeFenceDrawing() {
  if (drawing.mode !== 'fence-polygon') return
  drawing.finished = false
  setDrawPreviewSolid(false)
  if (map) map.getCanvas().style.cursor = 'crosshair'
  refreshDrawingPreview()
}

/** 撤销上一个顶点。定稿后撤销要退回绘制态，否则预览停在旧形状上。 */
function undoDrawingPoint() {
  if (!drawing.points.length) return
  drawing.points.pop()
  if (drawing.finished) {
    drawing.finished = false
    setDrawPreviewSolid(false)
    if (map) map.getCanvas().style.cursor = 'crosshair'
  }
  refreshDrawingPreview()
}

/** 提示条上的「下一步」：把当前几何交给表单。 */
function proceedToFenceForm() {
  openFenceFormForDrawing()
}

function startFenceDrawing() {
  if (!map) {
    showToast('danger', t('device_map_map_not_ready'))
    return
  }
  fenceMenuOpen.value = false
  drawing.mode = 'fence-polygon'
  drawing.points = []
  drawing.hover = null
  drawing.finished = false
  drawing.centerLng = null
  drawing.centerLat = null
  drawing.radiusM = 0
  closeFenceForm()
  clearRegionPreview()
  map.getCanvas().style.cursor = 'crosshair'
  setDrawPreviewSolid(false)
  // 绘制期间关掉双击缩放：双击要用来"结束绘制"，否则每画完一笔视野就跳一下。
  if (map.doubleClickZoom) map.doubleClickZoom.disable()
}

function cancelDrawing() {
  drawing.mode = ''
  drawing.points = []
  stopDrawingHover()
  drawing.finished = false
  drawing.centerLng = null
  drawing.centerLat = null
  drawing.radiusM = 0
  drawing.deviceCode = ''
  drawing.deviceName = ''
  if (map) {
    map.getCanvas().style.cursor = ''
    if (map.doubleClickZoom) map.doubleClickZoom.enable()
  }
  setDrawPreviewSolid(false)
  refreshDrawingPreview()
}

function refreshDrawingPreview() {
  if (!map) return
  const polygonFeatures = []
  const vertexFeatures = []
  if (drawing.mode === 'fence-polygon') {
    // 预览环 = 已点下的顶点 + 光标位置。光标点只是"预告"，不进 points。
    const ring = drawing.hover ? drawing.points.concat([drawing.hover]) : drawing.points.slice()
    if (ring.length >= 3) {
      // 三点起直接闭合：用户立刻能看到"围起来的范围"有多大，比折线更接近最终结果。
      polygonFeatures.push({
        type: 'Feature',
        geometry: { type: 'Polygon', coordinates: [ring.concat([ring[0]])] },
        properties: {},
      })
    } else if (ring.length === 2) {
      polygonFeatures.push({ type: 'Feature', geometry: { type: 'LineString', coordinates: ring }, properties: {} })
    }
    drawing.points.forEach(([lng, lat]) => {
      vertexFeatures.push({
        type: 'Feature',
        geometry: { type: 'Point', coordinates: [lng, lat] },
        properties: { kind: 'vertex' },
      })
    })
    if (drawing.hover) {
      vertexFeatures.push({
        type: 'Feature',
        geometry: { type: 'Point', coordinates: drawing.hover },
        properties: { kind: 'hover' },
      })
    }
  }

  const source = map.getSource('draw-preview')
  if (source) source.setData({ type: 'FeatureCollection', features: polygonFeatures })
  const vertexSource = map.getSource('draw-vertices')
  if (vertexSource) vertexSource.setData({ type: 'FeatureCollection', features: vertexFeatures })
}

function openFenceFormForDrawing() {
  if (drawing.points.length < 3) return
  // 先把几何拷出来，再退出拾取：cancelDrawing 会清空 points，
  // 顺序反了就变成"提交一个空环"。
  const ring = drawing.points.slice()
  ring.push([ring[0][0], ring[0][1]])
  stopDrawingHover()
  applyNewFenceForm({
    name: '',
    kind: 'polygon',
    source: 'manual',
    ring,
    geometry: JSON.stringify({ type: 'Polygon', coordinates: [ring] }),
  })
  // 进表单即退出拾取：留着预览会让用户以为还能继续加点，
  // 而表单里的几何是快照，不会跟着变。
  cancelDrawing()
}

function openFenceFormForRegion(boundary) {
  applyNewFenceForm({
    name: boundary.name,
    kind: 'polygon',
    source: 'region',
    region_name: boundary.name,
    region_level: boundary.level,
    ring: [],
    geometry: boundary.geometry,
  })
}

function applyNewFenceForm({ name, kind, source, ring, geometry, region_name = '', region_level = '' }) {
  Object.assign(fenceForm, {
    open: true,
    saving: false,
    id: null,
    name,
    kind,
    source,
    region_name,
    region_level,
    rule_type: 'keep_in',
    center_lng: null,
    center_lat: null,
    radius_m: 0,
    ring,
    geometry,
    device_codes: [],
    deviceSearch: '',
    enter_dwell_s: 60,
    exit_dwell_s: 60,
    enabled: true,
  })
}

function openFenceEdit(fence) {
  // 手绘围栏需要把已存的 GeoJSON 反解回 ring：表单提交时是拿 fenceForm.ring
  // 重新序列化 geometry 的，留空数组会把原几何直接抹掉（只剩改个名字就丢了范围）。
  const geometry = fenceGeometry(fence)
  const ring =
    fence.kind === 'polygon' && fence.source !== 'region' && geometry?.type === 'Polygon' && geometry.coordinates?.[0]
      ? geometry.coordinates[0].map((point) => [Number(point[0]), Number(point[1])])
      : []
  Object.assign(fenceForm, {
    open: true,
    saving: false,
    id: fence.ID,
    name: fence.name,
    kind: fence.kind,
    source: fence.source || 'manual',
    region_name: fence.region_name || '',
    region_level: fence.region_level || '',
    rule_type: fence.rule_type,
    center_lng: fence.center_lng,
    center_lat: fence.center_lat,
    radius_m: fence.radius_m ?? 0,
    ring,
    geometry: fence.kind === 'polygon' ? fence.geometry || '' : '',
    // 绑定关系以列表接口返回的 assigned_devices 为准。
    device_codes: Array.isArray(fence.assigned_devices) ? fence.assigned_devices.slice() : [],
    deviceSearch: '',
    enter_dwell_s: fence.enter_dwell_s ?? 0,
    exit_dwell_s: fence.exit_dwell_s ?? 0,
    enabled: !!fence.enabled,
  })
}

function closeFenceForm() {
  fenceForm.open = false
  fenceForm.saving = false
}

function toggleFenceDevice(code) {
  const index = fenceForm.device_codes.indexOf(code)
  if (index >= 0) fenceForm.device_codes.splice(index, 1)
  else fenceForm.device_codes.push(code)
}

/** 历史圆形围栏转多边形：用当前圆离散成 64 边形，保留原语义。 */
function convertCircleToPolygon() {
  const lng = Number(fenceForm.center_lng)
  const lat = Number(fenceForm.center_lat)
  const radius = Number(fenceForm.radius_m) || 0
  if (!Number.isFinite(lng) || !Number.isFinite(lat) || radius <= 0) return
  const ring = circleRing(lng, lat, radius)
  fenceForm.kind = 'polygon'
  fenceForm.source = 'manual'
  fenceForm.ring = ring
  fenceForm.geometry = JSON.stringify({ type: 'Polygon', coordinates: [ring] })
  fenceForm.center_lng = null
  fenceForm.center_lat = null
  fenceForm.radius_m = 0
  showToast('info', t('device_map_fence_circle_converted'))
}

async function submitFenceForm() {
  if (!fenceForm.name) return
  if (fenceForm.kind === 'polygon' && fenceForm.source === 'region' && !fenceForm.geometry) {
    showToast('warning', t('device_map_region_required'))
    return
  }
  if (fenceForm.kind === 'polygon' && fenceForm.source !== 'region' && fenceRingVertexCount.value < 3) {
    showToast('warning', t('device_map_draw_polygon_hint'))
    return
  }
  fenceForm.saving = true
  const geometry =
    fenceForm.kind === 'polygon' && fenceForm.source !== 'region'
      ? JSON.stringify({ type: 'Polygon', coordinates: [fenceForm.ring] })
      : fenceForm.geometry
  const payload = {
    name: fenceForm.name,
    kind: fenceForm.kind,
    source: fenceForm.kind === 'circle' ? 'manual' : fenceForm.source,
    region_name: fenceForm.source === 'region' ? fenceForm.region_name : '',
    region_level: fenceForm.source === 'region' ? fenceForm.region_level : '',
    rule_type: fenceForm.rule_type,
    enter_dwell_s: Number(fenceForm.enter_dwell_s) || 0,
    exit_dwell_s: Number(fenceForm.exit_dwell_s) || 0,
    enabled: fenceForm.enabled,
    // 表单是绑定关系的唯一来源：每次都整体提交，后端整体替换。
    device_codes: fenceForm.device_codes.slice(),
  }
  if (fenceForm.kind === 'circle') {
    payload.center_lng = fenceForm.center_lng
    payload.center_lat = fenceForm.center_lat
    payload.radius_m = Number(fenceForm.radius_m) || 0
  } else {
    payload.geometry = geometry
  }

  try {
    if (fenceForm.id) {
      await updateGeofence(fenceForm.id, payload)
      showToast('success', t('device_map_fence_updated'))
    } else {
      await createGeofence(payload)
      showToast('success', t('device_map_fence_created'))
    }
    closeFenceForm()
    cancelDrawing()
    await reloadFences()
  } catch (error) {
    showToast('danger', error.message)
  } finally {
    fenceForm.saving = false
  }
}

async function confirmDeleteFence(fence) {
  const confirmed = await confirmDialog({
    title: t('device_map_fence_delete'),
    message: t('device_map_fence_delete_message', { name: fence.name }),
  })
  if (!confirmed) return
  try {
    await deleteGeofence(fence.ID)
    showToast('success', t('device_map_fence_deleted'))
    await reloadFences()
  } catch (error) {
    showToast('danger', error.message)
  }
}
// ===================== 手动标注 =====================

function startDeviceLocationPick(device) {
  if (device.mode === 'telemetry') {
    showToast('warning', t('device_map_telemetry_locked'))
    return
  }
  // 底图没起来时"进入拾取模式"会是一个死状态：提示条在、但地图点不动。
  // 明确报出来，不要让它表现成"按钮没反应"。
  if (!map) {
    showToast('danger', t('device_map_map_not_ready'))
    return
  }
  drawing.mode = 'device-location'
  drawing.deviceCode = device.device_code
  drawing.deviceName = device.device_name || device.device_code
  cancelHoverClose()
  hoverDevice.value = null
  resetHoverProbe()
  if (map) map.getCanvas().style.cursor = 'crosshair'
}

async function commitDeviceLocation(lng, lat) {
  const code = drawing.deviceCode
  if (!code) return
  try {
    await saveDeviceLocation(code, lng, lat)
    showToast('success', t('device_map_location_saved'))
    cancelDrawing()
    // 人已经站在要放点的位置上了，刷新数据不要再动相机。
    await loadData({ fit: false })
  } catch (error) {
    showToast('danger', error.message)
    cancelDrawing()
  }
}

async function clearLocationFor(device) {
  try {
    await removeDeviceLocation(device.device_code)
    showToast('success', t('device_map_location_cleared'))
    await loadData({ fit: false })
  } catch (error) {
    showToast('danger', error.message)
  }
}

// ===================== 格式化 =====================

function formatNumber(value, digits = 4) {
  if (value == null || Number.isNaN(Number(value))) return '—'
  return Number(value).toFixed(digits)
}

function formatCoord(lng, lat) {
  if (lng == null || lat == null) return '—'
  return `${formatNumber(lng, 5)}, ${formatNumber(lat, 5)}`
}

function formatTime(value) {
  if (!value) return '—'
  const date = new Date(Number(value))
  if (Number.isNaN(date.getTime())) return '—'
  return date.toLocaleString(locale.value === 'en' ? 'en-US' : 'zh-CN', { hour12: false })
}
</script>

<style scoped>
.device-map-body {
  display: flex;
  gap: 16px;
  min-height: 0;
}

.map-surface {
  position: relative;
  flex: 1 1 auto;
  min-width: 0;
  border-radius: var(--radius-card, 24px);
  overflow: hidden;
  background: var(--bg-body);
}

.map-canvas {
  position: absolute;
  inset: 0;
}

/*
 * 聚合数字（DOM 标注，见 refreshClusterLabels）。
 * 注意：不要在这里声明 position —— maplibregl-marker 用 position:absolute + transform 定位，
 * 覆盖 position 会让标注跑到画布左上角。pointer-events:none 让点击穿透到 canvas，
 * 否则点数字不会触发聚合气泡的放大。
 */
.map-cluster-label {
  pointer-events: none;
  user-select: none;
  font-size: 12px;
  font-weight: 700;
  color: #fff;
  text-shadow: 0 1px 2px rgba(0, 0, 0, 0.4);
  font-variant-numeric: tabular-nums;
}

.map-panel {
  width: 340px;
  flex: 0 0 340px;
  display: flex;
  flex-direction: column;
  background: var(--glass-strong-tint);
  border-radius: var(--radius-card, 24px);
  border: 1px solid var(--glass-strong-border);
  overflow: hidden;
}

.map-panel-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px 16px 8px;
}

.map-panel-count {
  font-size: 12px;
  color: var(--text-secondary, #64748b);
}

.map-panel-tabs {
  display: flex;
  gap: 4px;
  padding: 0 12px 8px;
}

.map-panel-tab {
  flex: 1;
  border: none;
  background: transparent;
  border-radius: 10px;
  padding: 6px 8px;
  font-size: 13px;
  color: var(--text-secondary, #64748b);
}

.map-panel-tab.active {
  background: var(--color-brand, #3b82f6);
  color: #fff;
}

.map-panel-search {
  padding: 0 12px 8px;
}

.map-panel-filter {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  padding: 0 12px 8px;
}

.map-filter-chip {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  border: 1px solid var(--noyo-solid-border);
  background: transparent;
  border-radius: 999px;
  padding: 2px 10px;
  font-size: 12px;
  color: var(--text-secondary, #64748b);
}

.map-filter-chip.active {
  color: var(--text-primary, #0f172a);
}

.map-filter-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
}

.map-panel-scroll {
  flex: 1 1 auto;
  /* 没有 min-height:0 时，flex 子项的自动最小高度等于内容高度，
     列表一长就把告警区挤出面板（面板 overflow:hidden 直接裁掉）。 */
  min-height: 0;
  overflow-y: auto;
  padding: 0 8px 12px;
}

.map-panel-empty {
  padding: 32px 16px;
  text-align: center;
  font-size: 13px;
  color: var(--text-secondary, #94a3b8);
}

.map-device-row,
.map-fence-row {
  display: flex;
  align-items: center;
  gap: 10px;
  width: 100%;
  border: none;
  background: transparent;
  border-radius: 12px;
  padding: 9px 10px;
  text-align: left;
}

.map-device-row:hover,
.map-fence-row:hover {
  background: var(--bg-hover);
}

.map-device-row.active {
  background: var(--color-brand-subtle);
}

/* 围栏被选中 = 地图上正在聚焦它。左侧加一道竖条，与"设备行选中"区分开：
   两者可能同时存在，靠背景色区分不开。 */
.map-fence-row {
  cursor: pointer;
}

.map-fence-row.active {
  background: var(--color-brand-subtle);
  box-shadow: inset 3px 0 0 var(--color-brand, #3b82f6);
}

/* 「N 台在内」：数量兼入口。做成胶囊而不是纯文字，避免被当成副标题忽略。 */
.map-fence-count {
  align-self: flex-start;
  display: inline-flex;
  align-items: center;
  gap: 4px;
  margin-top: 3px;
  padding: 1px 8px;
  border: 1px solid var(--noyo-solid-border);
  border-radius: 999px;
  background: transparent;
  font-size: 11px;
  line-height: 18px;
  color: var(--text-secondary, #64748b);
}

.map-fence-count:hover {
  color: var(--color-brand, #3b82f6);
  border-color: var(--color-brand, #3b82f6);
}

.map-fence-count .bi-chevron-right {
  font-size: 9px;
  opacity: 0.7;
}

/* 围栏设备清单 */
.fence-device-dialog-head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 8px;
  padding-bottom: 8px;
  margin-bottom: 4px;
  border-bottom: 1px solid var(--noyo-solid-border);
}

.fence-device-dialog-name {
  font-size: 14px;
  font-weight: 600;
  color: var(--text-primary, #0f172a);
}

.fence-device-dialog-scope {
  font-size: 12px;
  color: var(--text-secondary, #64748b);
}

.fence-device-view {
  max-height: 320px;
  overflow-y: auto;
}

.fence-device-view-row {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 7px 2px;
  border-bottom: 1px solid var(--noyo-solid-border);
}

.fence-device-view-row:last-child {
  border-bottom: none;
}

.fence-device-view-name {
  font-size: 13px;
  color: var(--text-primary, #0f172a);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  flex: 1 1 auto;
}

.fence-device-view-code {
  font-size: 11px;
  color: var(--text-secondary, #94a3b8);
  font-variant-numeric: tabular-nums;
  flex: 0 0 auto;
}

.fence-state-tag {
  flex: 0 0 auto;
  font-size: 11px;
  line-height: 18px;
  padding: 0 8px;
  border-radius: 999px;
  border: 1px solid transparent;
}

.fence-state-tag.is-inside {
  color: var(--color-success, #16a34a);
  border-color: currentColor;
}

.fence-state-tag.is-outside {
  color: var(--color-danger, #dc2626);
  border-color: currentColor;
}

.fence-state-tag.is-unknown {
  color: var(--text-secondary, #94a3b8);
  border-color: var(--noyo-solid-border);
}

.map-device-dot {
  width: 9px;
  height: 9px;
  border-radius: 50%;
  flex: 0 0 9px;
}

.map-fence-swatch {
  width: 9px;
  height: 9px;
  border-radius: 3px;
  flex: 0 0 9px;
}

.map-device-info {
  display: flex;
  flex-direction: column;
  min-width: 0;
  flex: 1 1 auto;
}

.map-device-name {
  font-size: 13px;
  font-weight: 600;
  color: var(--text-primary, #0f172a);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.map-device-sub {
  font-size: 11px;
  color: var(--text-secondary, #94a3b8);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.map-fence-actions {
  display: inline-flex;
  gap: 8px;
}

.map-device-actions {
  display: inline-flex;
  gap: 8px;
}

/* 地图浮层 */

.map-banner {  position: absolute;
  top: 12px;
  left: 50%;
  transform: translateX(-50%);
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 14px;
  border-radius: 12px;
  font-size: 13px;
  /* 必须压过空状态卡片(6)与加载态(7)：绘制提示条是"已进入拾取模式"的
     唯一可见反馈，被盖住就等于点了按钮没反应。 */
  z-index: 9;
  backdrop-filter: blur(12px);
  max-width: calc(100% - 24px);
  /* 深色遮罩 + 白字：这条横幅永远压在底图影像上，不落在应用表面上，
     因此刻意不随明暗主题翻转（与地图自带弹窗同性质）。 */
  background: rgba(15, 23, 42, 0.86);
  color: #fff;
  /* 地图区被右侧面板挤窄时，宁可整行折到第二行，也不要把提示文字
     压成"一字一行" —— 那样看起来像渲染坏了。 */
  flex-wrap: wrap;
  justify-content: center;
}

.map-banner-text {
  white-space: nowrap;
}

.map-banner button {
  flex-shrink: 0;
}

/* 横幅底色是深色遮罩，Bootstrap 的 outline-secondary 文字是深灰 ——
   放上去几乎看不见（实测只剩一个圆角方块）。这里强制浅色。 */
.map-banner .btn-outline-secondary {
  color: #f1f5f9;
  border-color: rgba(226, 232, 240, 0.55);
  /* 只给边框不够：13px 字在深色横幅上仍然偏淡，加一层极浅底把它"托"起来。 */
  background: rgba(255, 255, 255, 0.1);
}

.map-banner .btn-outline-secondary:hover {
  color: #fff;
  border-color: #fff;
  background: rgba(255, 255, 255, 0.14);
}

/* 语义色只落在图标与描边上。白字压亮橙在暗色主题下对比度不足（#ff9f0a 上不到 3:1），
   而这里正是"底图挂了"的警报位，可读性优先。 */
.map-banner-warning {
  border: 1px solid var(--color-warning);
}

.map-banner-warning i {
  color: var(--color-warning);
}

.map-banner-active i {
  color: var(--color-brand);
}

.drawing-readout {
  font-variant-numeric: tabular-nums;
  font-size: 12px;
}

.map-controls {
  position: absolute;
  top: 16px;
  right: 16px;
  display: flex;
  flex-direction: column;
  gap: 6px;
  z-index: 4;
}

.map-control {
  width: 34px;
  height: 34px;
  border-radius: 10px;
  border: 1px solid var(--glass-strong-border);
  background: var(--glass-strong-tint);
  color: var(--text-primary, #0f172a);
  display: inline-flex;
  align-items: center;
  justify-content: center;
  backdrop-filter: blur(12px);
  cursor: pointer;
  transition: background-color .16s ease, color .16s ease;
}

/* 悬停只描边不改底色：底色是玻璃令牌，被 --bg-hover 整体替换会让按钮突然"变透明"，
   在底图上看起来像闪烁。 */
.map-control:hover {
  border-color: var(--color-brand, #3b82f6);
}

.map-control.active,
.map-control.active:hover {
  background: var(--color-brand, #3b82f6);
  color: #fff;
}

/*
 * 右下角工具组（图层 / 2D / 3D）。
 * 与缩放按钮同为 34×34 —— 同一张地图上的控件尺寸必须一致，否则看起来像两套设计。
 * bottom:40px 是给底部版权条让位（它占满整宽、z-index 同为 4）。
 */
.map-toolbox {
  position: absolute;
  right: 16px;
  bottom: 40px;
  display: flex;
  flex-direction: column;
  gap: 6px;
  align-items: flex-end;
  z-index: 4;
}

.map-dimension {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.map-control-text {
  font-size: 12px;
  font-weight: 600;
  letter-spacing: .3px;
  font-variant-numeric: tabular-nums;
}

.map-legend {
  position: absolute;
  left: 16px;
  bottom: 40px;
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 10px 12px;
  border-radius: 12px;
  background: var(--glass-strong-tint);
  border: 1px solid var(--glass-strong-border);
  font-size: 12px;
  z-index: 4;
  backdrop-filter: blur(12px);
}

.map-legend-item {
  display: flex;
  align-items: center;
  gap: 6px;
}

.map-legend-dot {
  width: 9px;
  height: 9px;
  border-radius: 50%;
}

.map-attribution {
  position: absolute;
  left: 0;
  right: 0;
  bottom: 0;
  display: flex;
  justify-content: space-between;
  gap: 12px;
  padding: 4px 12px;
  font-size: 11px;
  color: var(--text-secondary, #475569);
  background: var(--glass-strong-tint);
  z-index: 4;
}

.map-cursor-readout {
  font-variant-numeric: tabular-nums;
}

/*
 * 围栏悬停标签：只读小卡片，位置由 hoverFenceStyle 计算（含右边缘翻转）。
 * pointer-events:none 是必须的 —— 否则标签会挡住光标下的填充区，
 * mouseleave 与 mousemove 交替触发，表现为标签疯狂闪烁。
 */
.fence-hover-label {
  position: absolute;
  max-width: 260px;
  padding: 6px 10px;
  border-radius: 10px;
  background: var(--glass-strong-tint);
  border: 1px solid var(--glass-strong-border);
  box-shadow: var(--glass-strong-shadow);
  font-size: 12px;
  line-height: 1.35;
  z-index: 6;
  pointer-events: none;
  backdrop-filter: blur(16px);
}

.fence-hover-name {
  display: block;
  font-weight: 600;
  color: var(--text-main);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.fence-hover-meta {
  display: block;
  margin-top: 1px;
  font-size: 11px;
  color: var(--text-secondary, #64748b);
  white-space: nowrap;
}

.device-hover-card {
  position: absolute;
  width: 260px;
  padding: 12px 14px;
  border-radius: 16px;
  background: var(--glass-strong-tint);
  border: 1px solid var(--glass-strong-border);
  box-shadow: var(--glass-strong-shadow);
  font-size: 12px;
  z-index: 6;
  pointer-events: auto;
  backdrop-filter: blur(16px);
}

.device-hover-name {
  font-weight: 600;
  font-size: 13px;
}

.device-hover-code {
  font-size: 11px;
  color: var(--text-secondary, #94a3b8);
}

.device-hover-divider {
  height: 1px;
  margin: 8px 0;
  background: var(--noyo-solid-border);
}

.device-hover-meta {
  display: flex;
  justify-content: space-between;
  gap: 8px;
  color: var(--text-secondary, #475569);
  padding: 1px 0;
}

/* ===================== 悬停卡：实时属性与视频 ===================== */

/*
 * 属性区刻意做成"两列小方块"而不是一行一条：
 * 一行一条在 260px 宽里放不下「点位名 + 数值 + 单位」，数值会被省略号吃掉，
 * 而数值正是这张卡存在的理由。两列方块牺牲一点留白，换来数值完整可读。
 */
.device-hover-section-head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 5px;
  color: var(--text-muted);
  font-size: 10px;
  font-weight: 600;
  letter-spacing: .3px;
}

.device-hover-fresh {
  font-size: 10px;
  font-weight: 400;
  color: var(--text-muted);
}

.device-hover-props {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 5px;
}

.device-hover-prop {
  display: flex;
  flex-direction: column;
  gap: 1px;
  min-width: 0;
  padding: 4px 6px 5px;
  border-radius: 8px;
  background: var(--bg-input, rgba(148, 163, 184, .12));
}

.device-hover-prop-name {
  overflow: hidden;
  color: var(--text-muted);
  font-size: 10px;
  line-height: 1.3;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.device-hover-prop-value {
  overflow: hidden;
  color: var(--text-main);
  font-size: 13px;
  font-weight: 600;
  line-height: 1.3;
  text-overflow: ellipsis;
  white-space: nowrap;
}

/* 没上报过的点位不隐藏、只压灰：隐藏会让人以为"没有这个点位"。 */
.device-hover-prop-value.is-empty {
  color: var(--text-muted);
  font-weight: 400;
}

.device-hover-prop-unit {
  margin-left: 2px;
  font-size: 10px;
  font-weight: 400;
  color: var(--text-muted);
}

.device-hover-prop-skeleton {
  height: 34px;
  border-radius: 8px;
  background: var(--bg-input, rgba(148, 163, 184, .12));
  animation: hover-prop-breathe 1.4s ease-in-out infinite;
}

@keyframes hover-prop-breathe {
  0%, 100% { opacity: .55; }
  50% { opacity: 1; }
}

.device-hover-empty {
  display: flex;
  align-items: center;
  gap: 6px;
  margin: 0;
  padding: 8px 10px;
  border-radius: 8px;
  background: var(--bg-input, rgba(148, 163, 184, .12));
  color: var(--text-muted);
  font-size: 11px;
}

.device-hover-more {
  margin: 4px 0 0;
  color: var(--text-muted);
  font-size: 10px;
  text-align: right;
}

.device-hover-video {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  width: 100%;
  min-height: 28px;
  margin-top: 7px;
  border-radius: 8px;
  border: 1px solid color-mix(in srgb, var(--color-brand, #3b82f6) 45%, transparent);
  background: color-mix(in srgb, var(--color-brand, #3b82f6) 12%, transparent);
  color: var(--color-brand, #3b82f6);
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
  transition: background-color .16s ease, border-color .16s ease;
}

.device-hover-video:hover {
  background: color-mix(in srgb, var(--color-brand, #3b82f6) 20%, transparent);
  border-color: var(--color-brand, #3b82f6);
}

.device-hover-video:focus-visible {
  outline: 2px solid var(--color-brand, #3b82f6);
  outline-offset: 2px;
}

/*
 * 空状态是浮在地图上的一张卡片，**不是整面蒙版**。
 * 曾经写成 inset:0 + 半透明玻璃，后果有三：
 *   1. 72% 白 + blur(8px) 把底图糊成一片白 —— 看起来"地图没渲染出来"；
 *   2. 铺满后吃掉地图的全部点击 —— 手动标注点不下去；
 *   3. 与 .map-banner 同 z-index 但排在后面 —— 绘制提示条被永久盖住，
 *      于是点"+"后界面毫无变化，表现为"按钮没反应"。
 * 卡片化之后，四周地图保持可点、可看。
 */
.map-empty-state {
  position: absolute;
  left: 50%;
  top: 50%;
  transform: translate(-50%, -50%);
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
  padding: 28px 32px 26px;
  max-width: min(420px, calc(100% - 32px));
  text-align: center;
  border-radius: var(--radius-card, 24px);
  border: 1px solid var(--glass-strong-border);
  background: var(--glass-strong-tint);
  box-shadow: var(--glass-strong-shadow);
  backdrop-filter: blur(16px);
  z-index: 6;
}

/*
 * 关闭按钮压在卡片右上角而不是占一行：卡片高度直接决定它挡住多少底图，
 * 多一行按钮就多挡 40px。28×28 是 8pt 网格内可点的最小实用尺寸。
 */
.map-empty-close {
  position: absolute;
  top: 8px;
  right: 8px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  padding: 0;
  border: none;
  border-radius: 50%;
  background: transparent;
  color: var(--text-secondary, #94a3b8);
  font-size: 12px;
  line-height: 1;
}

.map-empty-close:hover {
  background: var(--bg-hover);
  color: var(--text-primary);
}

.map-empty-close:focus-visible {
  outline: 2px solid var(--color-brand);
  outline-offset: 1px;
}

.map-empty-state h3 {
  font-size: 16px;
  margin: 0;
}

.map-empty-state p {
  font-size: 13px;
  color: var(--text-secondary, #64748b);
  margin: 0;
  max-width: 380px;
}

.map-empty-icon {
  width: 56px;
  height: 56px;
  color: var(--text-secondary, #94a3b8);
  opacity: 0.5;
}

.map-loading {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--glass-strong-tint);
  z-index: 7;
}

.basemap-switch {
  position: relative;
}

.basemap-menu {
  position: absolute;
  right: 0;
  top: calc(100% + 8px);
  width: min(360px, calc(100vw - 48px));
  max-height: calc(100vh - 160px);
  overflow-y: auto;
  padding: 8px;
  border-radius: 16px;
  background: var(--glass-strong-tint);
  border: 1px solid var(--glass-strong-border);
  box-shadow: var(--glass-strong-shadow);
  z-index: 20;
  backdrop-filter: blur(16px);
}

.basemap-menu-item {
  display: flex;
  flex-direction: column;
  gap: 2px;
  width: 100%;
  border: none;
  background: transparent;
  border-radius: 10px;
  padding: 8px 10px;
  text-align: left;
}

.basemap-menu-item:hover {
  background: var(--bg-hover);
}

.basemap-menu-item.active {
  background: var(--color-brand-subtle);
}

.basemap-menu-title {
  font-size: 13px;
  font-weight: 600;
}

.basemap-menu-hint {
  font-size: 11px;
  color: var(--text-secondary, #94a3b8);
  line-height: 1.35;
}

.basemap-token {
  padding: 16px 8px 8px;
  border-top: 1px solid var(--noyo-solid-border);
  margin-top: 8px;
}

.basemap-token-label {
  font-size: 12px;
  color: var(--text-secondary);
  display: block;
  margin-bottom: 8px;
}

.basemap-token-help {
  margin: 8px 0 0;
  font-size: 12px;
  line-height: 1.5;
  color: var(--text-secondary);
}

/* ===================== 图层与视角 ===================== */

.layer-switch {
  position: relative;
}

.layer-menu {
  position: absolute;
  right: 0;
  /* 按钮在地图右下角 ⇒ 菜单只能向上展开（下方是版权条，没有空间）。
     菜单被 .map-surface 的 overflow:hidden 裁剪，自身高度一旦超过可用空间，
     顶部几行就变成"看得见却滚不到"——所以上限按视口算，而不是按 100vh 直接给。 */
  bottom: calc(100% + 8px);
  width: min(276px, calc(100vw - 48px));
  max-height: min(420px, calc(100vh - 300px));
  overflow-y: auto;
  padding: 10px 12px 12px;
  border-radius: 16px;
  background: var(--glass-strong-tint);
  border: 1px solid var(--glass-strong-border);
  box-shadow: var(--glass-strong-shadow);
  z-index: 20;
  backdrop-filter: blur(16px);
}

.layer-group-title {
  margin: 2px 0 8px;
  font-size: 11px;
  font-weight: 600;
  letter-spacing: .4px;
  color: var(--text-muted);
}

.layer-row {
  display: flex;
  align-items: center;
  gap: 8px;
  min-height: 28px;
  padding-left: 0;
  margin: 0 0 2px;
}

.layer-row .form-check-input {
  flex: 0 0 auto;
  margin: 0;
  cursor: pointer;
}

.layer-row .form-check-label {
  font-size: 13px;
  color: var(--text-main);
  cursor: pointer;
  user-select: none;
}

.layer-row .form-check-input:disabled {
  cursor: not-allowed;
}

.layer-row .form-check-input:disabled + .form-check-label {
  color: var(--text-muted);
  cursor: not-allowed;
}

.layer-hint {
  margin: 6px 0 12px;
  padding: 8px 10px;
  border-radius: 10px;
  background: var(--bg-hover);
  font-size: 11px;
  line-height: 1.5;
  color: var(--text-secondary);
}

/* ===================== 状态脉冲高亮 ===================== */

/* 菜单定位基准。与 .layer-switch 同样的向上展开约束（见 .layer-menu）。 */
.pulse-switch {
  position: relative;
}

.pulse-menu {
  width: min(262px, calc(100vw - 48px));
  /* 菜单比图层菜单矮得多，不必按视口算上限 */
  max-height: none;
}

.pulse-row {
  gap: 7px;
}

/* 状态色示意圆点：勾选与否都能立刻看出"这一项对应哪种颜色"，
   否则四行文字在没有图标的情况下完全靠记忆区分。 */
.pulse-dot {
  flex: 0 0 auto;
  width: 8px;
  height: 8px;
  border-radius: 50%;
}

.pulse-actions {
  display: flex;
  gap: 8px;
  margin: 10px 0 4px;
}

.pulse-action {
  flex: 1 1 0;
  min-height: 26px;
  padding: 0 8px;
  border-radius: 8px;
  border: 1px solid var(--noyo-solid-border);
  background: transparent;
  color: var(--text-secondary);
  font-size: 12px;
  cursor: pointer;
  transition: border-color .16s ease, color .16s ease;
}

.pulse-action:hover {
  border-color: var(--color-brand, #3b82f6);
  color: var(--color-brand, #3b82f6);
}

.pulse-action:focus-visible,
.pulse-switch .map-control:focus-visible {
  outline: 2px solid var(--color-brand, #3b82f6);
  outline-offset: 2px;
}

/* ===================== 地点搜索 ===================== */

/*
 * 搜索框浮在地图左上角。z-index 必须高于设备悬停卡(4)、空状态卡片(6)、
 * 加载态(7)，低于 .map-banner(9) —— 绘制提示条出现时应当压住搜索面板，
 * 否则用户会以为"点在面板上"其实点在提示条上。
 */
.map-search {
  position: absolute;
  left: 16px;
  top: 16px;
  width: min(360px, calc(100% - 32px));
  z-index: 8;
}

.map-search-box {
  display: flex;
  align-items: center;
  gap: 8px;
  height: 40px;
  padding: 0 12px;
  border-radius: 999px;
  background: var(--glass-strong-tint);
  border: 1px solid var(--glass-strong-border);
  box-shadow: var(--glass-strong-shadow);
  backdrop-filter: blur(16px);
  color: var(--text-secondary, #64748b);
}

.map-search-input {
  flex: 1;
  min-width: 0;
  border: none;
  outline: none;
  background: transparent;
  font-size: 13px;
  color: var(--text-primary);
}

.map-search-input::placeholder {
  color: var(--text-secondary, #94a3b8);
}

.map-search-clear {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 20px;
  height: 20px;
  padding: 0;
  border: none;
  border-radius: 50%;
  background: transparent;
  color: var(--text-secondary, #94a3b8);
  font-size: 11px;
}

.map-search-clear:hover {
  background: var(--bg-hover);
  color: var(--text-primary);
}

.map-search-results {
  margin-top: 8px;
  max-height: 320px;
  overflow-y: auto;
  padding: 6px;
  border-radius: 16px;
  background: var(--glass-strong-tint);
  border: 1px solid var(--glass-strong-border);
  box-shadow: var(--glass-strong-shadow);
  backdrop-filter: blur(16px);
}

.map-search-item {
  display: flex;
  align-items: center;
  gap: 10px;
  width: 100%;
  padding: 8px 10px;
  border: none;
  border-radius: 10px;
  background: transparent;
  text-align: left;
  color: var(--text-primary);
}

.map-search-item:hover {
  background: var(--bg-hover);
}

.map-search-item-text {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 1px;
}

.map-search-item-name {
  font-size: 13px;
  font-weight: 600;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.map-search-item-address {
  font-size: 11px;
  color: var(--text-secondary, #94a3b8);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.map-search-item-kind {
  flex-shrink: 0;
  font-size: 10px;
  padding: 2px 6px;
  border-radius: 999px;
  background: var(--color-brand-subtle);
  color: var(--color-brand);
}

.map-search-hint {
  padding: 10px;
  font-size: 12px;
  color: var(--text-secondary, #94a3b8);
  text-align: center;
}

.map-search-picked {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  margin-top: 8px;
  padding: 5px 8px 5px 10px;
  border-radius: 999px;
  font-size: 12px;
  background: var(--color-brand-subtle);
  color: var(--color-brand);
  border: 1px solid var(--glass-strong-border);
}

/* 搜索定位标记：只有形状，靠 CSS 画。元素本身不能声明 position —— 
   maplibre 用 absolute + transform 定位，覆盖后标记会跑到画布左上角。 */
.map-place-marker {
  width: 14px;
  height: 14px;
  border-radius: 50%;
  background: var(--color-brand);
  border: 2px solid #ffffff;
  box-shadow: 0 0 0 4px rgba(59, 130, 246, 0.28);
}

/* ===================== 新建围栏菜单 ===================== */

.fence-create {
  position: relative;
}

.fence-create-menu {
  position: absolute;
  right: 0;
  top: calc(100% + 8px);
  width: 288px;
  padding: 6px;
  border-radius: 16px;
  background: var(--glass-strong-tint);
  border: 1px solid var(--glass-strong-border);
  box-shadow: var(--glass-strong-shadow);
  backdrop-filter: blur(16px);
  z-index: 20;
}

.fence-create-item {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  width: 100%;
  padding: 9px 10px;
  border: none;
  border-radius: 10px;
  background: transparent;
  text-align: left;
  color: var(--text-primary);
}

.fence-create-item:hover {
  background: var(--bg-hover);
}

.fence-create-item i {
  margin-top: 2px;
  color: var(--color-brand);
}

.fence-create-item span {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.fence-create-item strong {
  font-size: 13px;
  font-weight: 600;
}

.fence-create-item small {
  font-size: 11px;
  color: var(--text-secondary, #94a3b8);
  line-height: 1.35;
}

/* ===================== 围栏表单 ===================== */

.fence-summary {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px 12px;
  border-radius: 12px;
  border: 1px solid var(--noyo-solid-border);
  background: var(--bg-hover);
  font-size: 12px;
  color: var(--text-secondary, #64748b);
}

.fence-device-picker {
  max-height: 180px;
  overflow-y: auto;
  padding: 4px;
  border-radius: 12px;
  border: 1px solid var(--noyo-solid-border);
}

.fence-device-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 8px;
  border-radius: 8px;
  font-size: 12px;
  cursor: pointer;
}

.fence-device-item:hover {
  background: var(--bg-hover);
}

.fence-device-name {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.fence-device-code {
  flex-shrink: 0;
  font-size: 11px;
  color: var(--text-secondary, #94a3b8);
}

.fence-device-empty {
  padding: 12px;
  text-align: center;
  font-size: 12px;
  color: var(--text-secondary, #94a3b8);
}

/* ===================== 地区围栏结果 ===================== */

.region-result {
  padding: 10px 12px;
  border-radius: 12px;
  border: 1px solid var(--noyo-solid-border);
  background: var(--bg-hover);
}

.region-result-name {
  font-size: 13px;
  font-weight: 600;
  color: var(--text-primary);
}

.region-result-meta {
  margin-top: 2px;
  font-size: 11px;
  color: var(--text-secondary, #64748b);
}

.region-result-note {
  margin: 6px 0 0;
  font-size: 11px;
  color: var(--text-secondary, #94a3b8);
}
</style>
