<template>
  <div>
    <div class="d-flex justify-content-between align-items-center mb-3">
      <h6 class="mb-0">{{ $t('tsl_prop_proto_map') }}</h6>
      <div>
        <span v-if="protocolName" class="badge bg-info me-2">{{ protocolName }}</span>
        <span v-else class="badge bg-secondary">{{ $t('map_mode_direct') }}</span>
      </div>
    </div>

    <!-- Collection Mode Selection (Only for GatewayPolling) -->
    <div v-if="strategy === 'GatewayPolling' && !isSubDevice" class="mb-3">
      <label class="form-label me-3 fw-bold">{{ $t('collection_mode') }}:</label>
      <div class="btn-group" role="group">
        <input type="radio" class="btn-check" name="collectionMode" id="modeManual" value="manual" v-model="localCollectionMode">
        <label class="btn btn-outline-primary btn-sm" for="modeManual">
          <i class="bi bi-list-task"></i> {{ $t('mode_manual_group') }}
        </label>

        <input type="radio" class="btn-check" name="collectionMode" id="modeAuto" value="auto" v-model="localCollectionMode">
        <label class="btn btn-outline-primary btn-sm" for="modeAuto">
          <i class="bi bi-magic"></i> {{ $t('mode_auto_point') }}
        </label>
      </div>
      <div class="form-text mt-1" v-if="localCollectionMode === 'manual'">
        {{ $t('desc_mode_manual') }}
      </div>
      <div class="form-text mt-1" v-else>
        {{ $t('desc_mode_auto') }}
      </div>
    </div>

    <!-- Polling Group Management (Only for Gateway/Direct Devices in GatewayPolling mode AND Manual Mode) -->
    <div v-if="strategy === 'GatewayPolling' && !isSubDevice && localCollectionMode === 'manual'" class="mb-4 border rounded p-3 bg-body-tertiary">
      <div class="d-flex justify-content-between align-items-center mb-2">
        <h6 class="mb-0">{{ $t('dev_poll_groups') }}</h6>
        <button class="btn btn-sm btn-outline-primary" @click="addPollingGroup">
          <i class="bi bi-plus"></i> {{ $t('add') }}
        </button>
      </div>
      
      <div v-if="localPollingGroups.length === 0" class="text-muted small text-center py-2">
        {{ $t('tsl_warn_no_groups') }}
      </div>
      <div v-else class="table-responsive">
        <table class="table table-sm table-bordered mb-0 bg-white">
          <thead>
            <tr>
              <th>{{ $t('name') }}</th>
              <th class="text-center">{{ $t('enable') }}</th>
              <th>{{ $t('slave_id') }}</th>
              <th>{{ $t('func_code') }}</th>
              <th>{{ $t('start_addr') }}</th>
              <th>{{ $t('length') }}</th>
              <th>{{ $t('interval') }}</th>
              <th>{{ $t('description') }}</th>
              <th style="width: 80px">{{ $t('action') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="(group, index) in localPollingGroups" :key="index">
               <td>
                 <div class="d-flex align-items-center">
                    <input v-model="group.name" class="form-control form-control-sm me-1" :placeholder="$t('name')" :disabled="!group._isNew">
                    <i v-if="!group._isNew" class="bi bi-lock-fill text-muted small" :title="$t('tsl_group_name_locked')"></i>
                 </div>
               </td>
               <td class="text-center">
                 <div class="form-check d-flex justify-content-center">
                   <input class="form-check-input" type="checkbox" v-model="group.enable">
                 </div>
               </td>
               <td><input v-model.number="group.slave_id" type="number" class="form-control form-control-sm" style="width: 60px"></td>
               <td>
                 <select v-model.number="group.function_code" class="form-select form-select-sm">
                   <option :value="1">{{ $t('modbus_fc_coil') }}</option>
                   <option :value="2">{{ $t('modbus_fc_input') }}</option>
                   <option :value="3">{{ $t('modbus_fc_hold') }}</option>
                   <option :value="4">{{ $t('modbus_fc_input_reg') }}</option>
                 </select>
               </td>
               <td><input v-model.number="group.start_address" type="number" class="form-control form-control-sm"></td>
               <td><input v-model.number="group.length" type="number" class="form-control form-control-sm"></td>
               <td><input v-model.number="group.interval" type="number" class="form-control form-control-sm"></td>
               <td><input v-model="group.description" class="form-control form-control-sm"></td>
               <td class="text-center">
                 <button class="btn btn-sm btn-link text-danger p-0" @click="removePollingGroup(index)">
                   <i class="bi bi-trash"></i>
                 </button>
               </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- Auto Grouping Preview (Only for Gateway/Direct Devices in GatewayPolling mode AND Auto Mode) -->
    <div v-if="strategy === 'GatewayPolling' && !isSubDevice && localCollectionMode === 'auto'" class="mb-4 border rounded p-3 bg-body-tertiary">
       <div class="d-flex justify-content-between align-items-center mb-2">
        <div class="d-flex align-items-center">
            <h6 class="mb-0 text-primary me-2"><i class="bi bi-lightning-charge me-1"></i>{{ $t('auto_group_preview') }}</h6>
            <span class="badge bg-primary">{{ autoGeneratedGroups.length }} Groups</span>
        </div>
        <button class="btn btn-sm btn-link text-decoration-none" @click="isAutoGroupPreviewExpanded = !isAutoGroupPreviewExpanded">
            <i class="bi" :class="isAutoGroupPreviewExpanded ? 'bi-chevron-up' : 'bi-chevron-down'"></i>
            {{ isAutoGroupPreviewExpanded ? $t('collapse') : $t('expand') }}
        </button>
      </div>
      
      <div v-if="isAutoGroupPreviewExpanded">
          <div v-if="autoGeneratedGroups.length === 0" class="text-muted small text-center py-2">
            {{ $t('tsl_warn_no_groups_auto') }}
          </div>
          <div v-else class="table-responsive">
            <table class="table table-sm table-bordered mb-0 bg-white">
              <thead class="table-light">
                <tr>
                  <th>{{ $t('name') }}</th>
                  <th>{{ $t('slave_id') }}</th>
                  <th>{{ $t('func_code') }}</th>
                  <th>{{ $t('start_addr') }}</th>
                  <th>{{ $t('length') }}</th>
                  <th>{{ $t('interval') }}</th>
                  <th>{{ $t('point_count') }}</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="(group, index) in autoGeneratedGroups" :key="index">
                   <td>{{ group.name }}</td>
                   <td>{{ group.slave_id }}</td>
                   <td>
                     <span v-if="group.function_code === 1">{{ $t('modbus_fc_coil') }}</span>
                     <span v-else-if="group.function_code === 2">{{ $t('modbus_fc_input') }}</span>
                     <span v-else-if="group.function_code === 3">{{ $t('modbus_fc_hold') }}</span>
                     <span v-else-if="group.function_code === 4">{{ $t('modbus_fc_input_reg') }}</span>
                     <span v-else>{{ group.function_code }}</span>
                   </td>
                   <td>{{ group.start_address }}</td>
                   <td>{{ group.length }}</td>
                            <td>{{ group.interval }}</td>
                            <td :title="group.pointNames">
                                <span class="text-primary" style="cursor: help; text-decoration: underline dotted;">{{ group.pointCount }}</span>
                            </td>
                        </tr>
              </tbody>
            </table>
          </div>
      </div>
    </div>

    <!-- Parent Polling Group Display (For SubDevices) -->
    <div v-if="strategy === 'GatewayPolling' && isSubDevice" class="mb-4 border rounded p-3 bg-body-tertiary">
      <div class="d-flex justify-content-between align-items-center mb-2">
        <h6 class="mb-0 text-secondary"><i class="bi bi-diagram-3 me-1"></i>{{ $t('parent_poll_groups') }}</h6>
      </div>
      
      <div v-if="!pollingGroups || pollingGroups.length === 0" class="text-muted small text-center py-2">
        {{ $t('tsl_warn_no_groups') }}
      </div>
      <div v-else class="table-responsive">
        <table class="table table-sm table-bordered mb-0 bg-white">
          <thead class="table-light">
            <tr>
              <th>{{ $t('name') }}</th>
              <th>{{ $t('slave_id') }}</th>
              <th>{{ $t('func_code') }}</th>
              <th>{{ $t('start_addr') }}</th>
              <th>{{ $t('length') }}</th>
              <th>{{ $t('interval') }}</th>
              <th>{{ $t('description') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="(group, index) in pollingGroups" :key="index">
               <td>{{ group.name }}</td>
               <td>{{ group.slave_id }}</td>
               <td>
                 <span v-if="group.function_code === 1">{{ $t('modbus_fc_coil') }}</span>
                 <span v-else-if="group.function_code === 2">{{ $t('modbus_fc_input') }}</span>
                 <span v-else-if="group.function_code === 3">{{ $t('modbus_fc_hold') }}</span>
                 <span v-else-if="group.function_code === 4">{{ $t('modbus_fc_input_reg') }}</span>
                 <span v-else>{{ group.function_code }}</span>
               </td>
               <td>{{ group.start_address }}</td>
               <td>{{ group.length }}</td>
               <td>{{ group.interval }}</td>
               <td>{{ group.description }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- SubDevice/Self Point Mapping (Requires available polling groups) -->
    <div v-if="(strategy === 'GatewayPolling' && (availablePollingGroups.length > 0 || localCollectionMode === 'auto')) || strategy === 'DirectAccess'">
      
      <ul class="nav nav-tabs mb-3">
        <li class="nav-item">
          <a class="nav-link" :class="{ active: activeTab === 'properties' }" href="#" @click.prevent="activeTab = 'properties'">{{ $t('tsl_properties') }}</a>
        </li>
        <li class="nav-item">
          <a class="nav-link" :class="{ active: activeTab === 'events' }" href="#" @click.prevent="activeTab = 'events'">{{ $t('tsl_events') }}</a>
        </li>
        <li class="nav-item">
          <a class="nav-link" :class="{ active: activeTab === 'online' }" href="#" @click.prevent="activeTab = 'online'">{{ $t('online_status_config') }}</a>
        </li>
      </ul>

      <!-- Properties Tab -->
      <div v-if="activeTab === 'properties'">
        
        <!-- 1. Standard TSL Properties Section -->
        <div class="card mb-4 shadow-sm">
            <div class="card-header bg-body-tertiary fw-bold py-2">
                <i class="bi bi-box-seam me-1"></i> {{ $t('tsl_prop_source_model') }}
            </div>
            <div class="card-body p-0">
                <div v-if="tslPointsList.length === 0" class="text-center py-3 text-muted">
                    {{ $t('tsl_no_data') }}
                </div>
                <div v-else class="table-responsive">
                    <table class="table table-hover align-middle mb-0">
                        <thead class="table-light">
                            <tr>
                                <th style="width: 20%">{{ $t('tsl_name') }}</th>
                                <th style="width: 20%">{{ $t('tsl_identifier') }}</th>
                                <th style="width: 10%">{{ $t('tsl_type') }}</th>
                                <th v-if="strategy === 'GatewayPolling'" style="width: 30%">
                                    <span v-if="localCollectionMode === 'manual'">{{ $t('tsl_prop_poll_group') }} / {{ $t('start_addr') }}</span>
                                    <span v-else>{{ $t('tsl_prop_direct_addr_detail') }}</span>
                                </th>
                                <th v-else style="width: 30%">{{ $t('tsl_prop_address') }}</th>
                                <th class="text-center" style="width: 10%">{{ $t('tsl_prop_report') }}</th>
                                <th class="text-end" style="width: 10%; min-width: 100px">{{ $t('tsl_actions') }}</th>
                            </tr>
                        </thead>
                        <tbody>
                            <tr v-for="pt in tslPointsList" :key="pt.identifier">
                                <td>{{ pt.name }}</td>
                                <td class="font-monospace text-muted small">{{ pt.identifier }}</td>
                                <td><span class="badge bg-secondary">{{ pt.dataType }}</span></td>
                                <td v-if="strategy === 'GatewayPolling'" class="small">
                                    <div v-if="pt.isMapped">
                                        <div v-if="pt.rawPoint.polling_group">
                                            <span class="badge bg-secondary me-1">{{ pt.rawPoint.polling_group }}</span>
                                            <span class="text-muted">{{ $t('tsl_offset_short') }}: {{ pt.rawPoint.offset }}</span>
                                        </div>
                                        <div v-else class="text-muted">
                                            Slave: {{ pt.rawPoint.slave_id }} / Func: {{ pt.rawPoint.function_code }} / Addr: {{ pt.rawPoint.address }}
                                        </div>
                                    </div>
                                    <span v-else class="badge bg-secondary opacity-50">{{ $t('tsl_prop_unmapped') }}</span>
                                </td>
                                <td v-else class="font-monospace small">
                                    <span v-if="pt.isMapped">{{ pt.rawPoint.address }}</span>
                                    <span v-else class="badge bg-secondary opacity-50">{{ $t('tsl_prop_unmapped') }}</span>
                                </td>
                                <td class="text-center">
                                    <div class="form-check form-switch d-flex justify-content-center">
                                        <input class="form-check-input" type="checkbox" 
                                            :checked="pt.isProperty" 
                                            @change="togglePointProperty(pt, $event.target.checked)"
                                            :disabled="!pt.isMapped">
                                    </div>
                                </td>
                                <td class="text-end">
                                    <div class="d-flex justify-content-end">
                                        <button v-if="pt.rawProp" class="btn btn-sm btn-outline-primary" @click="openEditModal(pt.rawProp)">
                                            <i class="bi bi-pencil"></i> {{ $t('tsl_edit') }}
                                        </button>
                                        <button v-else class="btn btn-sm btn-outline-primary" @click="editCustomPoint(pt.rawPoint)">
                                            <i class="bi bi-pencil"></i> {{ $t('tsl_edit') }}
                                        </button>
                                    </div>
                                </td>
                            </tr>
                        </tbody>
                    </table>
                </div>
            </div>
        </div>

        <!-- 2. Custom Points Section -->
        <div class="card shadow-sm">
            <div class="card-header bg-body-tertiary py-2 d-flex justify-content-between align-items-center">
                <span class="fw-bold text-primary"><i class="bi bi-cpu me-1"></i> {{ $t('tsl_custom_points') }}</span>
                <button class="btn btn-sm btn-primary" @click="openAddCustomPointModal">
                    <i class="bi bi-plus-lg"></i> {{ $t('tsl_add_custom_point') }}
                </button>
            </div>
            <div class="card-body p-0">
                <div v-if="customPointsList.length === 0" class="text-center py-4 text-muted bg-body-tertiary">
                    {{ $t('tsl_no_data') }}
                </div>
                <div v-else class="table-responsive">
                    <table class="table table-hover align-middle mb-0">
                        <thead class="table-light">
                            <tr>
                                <th style="width: 20%">{{ $t('tsl_name') }}</th>
                                <th style="width: 20%">{{ $t('tsl_identifier') }}</th>
                                <th style="width: 10%">{{ $t('tsl_type') }}</th>
                                <th v-if="strategy === 'GatewayPolling'" style="width: 30%">
                                    <span v-if="localCollectionMode === 'manual'">{{ $t('tsl_prop_poll_group') }} / {{ $t('start_addr') }}</span>
                                    <span v-else>{{ $t('tsl_prop_direct_addr_detail') }}</span>
                                </th>
                                <th v-else style="width: 30%">{{ $t('tsl_prop_address') }}</th>
                                <th class="text-center" style="width: 10%">{{ $t('tsl_prop_report') }}</th>
                                <th class="text-end" style="width: 10%; min-width: 100px">{{ $t('tsl_actions') }}</th>
                            </tr>
                        </thead>
                        <tbody>
                            <tr v-for="pt in customPointsList" :key="pt.identifier">
                                <td>{{ pt.name }}</td>
                                <td class="font-monospace text-muted small">{{ pt.identifier }}</td>
                                <td><span class="badge bg-secondary">{{ pt.dataType }}</span></td>
                                <td v-if="strategy === 'GatewayPolling'" class="small">
                                    <div v-if="pt.rawPoint.polling_group">
                                        <span class="badge bg-secondary me-1">{{ pt.rawPoint.polling_group }}</span>
                                        <span class="text-muted">{{ $t('tsl_offset_short') }}: {{ pt.rawPoint.offset }}</span>
                                    </div>
                                    <div v-else class="text-muted">
                                        <span v-if="pt.rawPoint.slave_id">Slave: {{ pt.rawPoint.slave_id }} / Func: {{ pt.rawPoint.function_code }} / Addr: {{ pt.rawPoint.address }}</span>
                                        <span v-else class="text-danger small"><i class="bi bi-exclamation-circle"></i> Invalid</span>
                                    </div>
                                </td>
                                <td v-else class="font-monospace small">{{ pt.rawPoint.address }}</td>
                                <td class="text-center">
                                    <div class="form-check form-switch d-flex justify-content-center">
                                        <input class="form-check-input" type="checkbox" 
                                            :checked="pt.isProperty" 
                                            @change="togglePointProperty(pt, $event.target.checked)">
                                    </div>
                                </td>
                                <td class="text-end">
                                    <div class="d-flex justify-content-end gap-1">
                                        <button class="btn btn-sm btn-outline-primary" @click="editCustomPoint(pt.rawPoint)">
                                            <i class="bi bi-pencil"></i>
                                        </button>
                                        <button class="btn btn-sm btn-outline-danger" @click="deleteCustomPoint(pt.rawPoint)">
                                            <i class="bi bi-trash"></i>
                                        </button>
                                    </div>
                                </td>
                            </tr>
                        </tbody>
                    </table>
                </div>
            </div>
        </div>

      </div>

      <!-- Online Status Config Tab -->
      <div v-if="activeTab === 'online'" class="p-3 border rounded bg-white">
        <div class="mb-4">
          <label class="form-label">{{ $t('online_check_strategy') }}</label>
          <div class="d-flex flex-column gap-2">
            <div class="form-check">
              <input class="form-check-input" type="radio" name="onlineStrategy" id="strategyAuto" value="communication" v-model="onlineRule.strategy">
              <label class="form-check-label" for="strategyAuto">
                {{ $t('strategy_auto_comm') }}
                <div class="form-text small">{{ $t('strategy_auto_desc') }}</div>
              </label>
            </div>
            <div class="form-check">
              <input class="form-check-input" type="radio" name="onlineStrategy" id="strategyPoint" value="custom_point" v-model="onlineRule.strategy">
              <label class="form-check-label" for="strategyPoint">
                {{ $t('strategy_custom_point') }}
                <div class="form-text small">{{ $t('strategy_custom_desc') }}</div>
              </label>
            </div>
            <div class="form-check">
              <input class="form-check-input" type="radio" name="onlineStrategy" id="strategyValue" value="value_change" v-model="onlineRule.strategy">
              <label class="form-check-label" for="strategyValue">
                {{ $t('strategy_value_change') }}
                <div class="form-text small">{{ $t('strategy_value_change_desc') }}</div>
              </label>
            </div>
          </div>
        </div>

        <!-- Custom Point Strategy Config -->
        <div v-if="onlineRule.strategy === 'custom_point'" class="card bg-body-tertiary border-0 mb-4">
          <div class="card-body">
            <h6 class="card-title mb-3">{{ $t('online_judgment_rule') }}</h6>
            <div class="row g-3 align-items-center">
              <div class="col-auto">
                <span class="fw-bold">{{ $t('when_point') }}</span>
              </div>
              <div class="col-md-3">
                <select v-model="onlineRule.point" class="form-select">
                  <option value="" disabled>{{ $t('select_point') }}</option>
                  <option v-for="pt in availablePoints" :key="pt.value" :value="pt.value">{{ pt.label }}</option>
                </select>
              </div>
              <div class="col-md-2">
                <select v-model="onlineRule.operator" class="form-select">
                  <option value="==">{{ $t('op_eq') }} (==)</option>
                  <option value="!=">{{ $t('op_neq') }} (!=)</option>
                  <option value=">">{{ $t('op_gt') }} (&gt;)</option>
                  <option value="bit_and">{{ $t('op_bit_and') }} (&)</option>
                </select>
              </div>
              <div class="col-md-2">
                <input v-model.number="onlineRule.value" type="number" class="form-control" :placeholder="$t('dev_data_val_placeholder')">
              </div>
              <div class="col-auto">
                <span class="fw-bold"> ==> {{ $t('status_online') }} 🟢</span>
              </div>
            </div>
            <div class="mt-2 text-muted small">
              <i class="bi bi-info-circle me-1"></i>
              {{ $t('online_rule_hint') }}
            </div>
          </div>
        </div>

        <!-- Value Change Strategy Config -->
        <div v-if="onlineRule.strategy === 'value_change'" class="card bg-body-tertiary border-0 mb-4">
          <div class="card-body">
             <h6 class="card-title mb-3">{{ $t('value_check_config') }}</h6>
             <div class="row g-3 align-items-end">
                <div class="col-md-6">
                  <label class="form-label small">{{ $t('monitor_point') }}</label>
                  <select v-model="onlineRule.monitor_point" class="form-select">
                    <option value="" disabled>{{ $t('select_point') }}</option>
                    <option v-for="pt in availablePoints" :key="pt.value" :value="pt.value">{{ pt.label }}</option>
                  </select>
                </div>
                <div class="col-md-6">
                  <label class="form-label small">{{ $t('max_unchanged_time') }}</label>
                  <div class="input-group">
                    <input v-model.number="onlineRule.max_unchanged_interval" type="number" min="1" class="form-control">
                    <span class="input-group-text">Seconds</span>
                  </div>
                </div>
                <div class="col-12">
                   <div class="form-text small">{{ $t('value_check_desc') }}</div>
                </div>
             </div>
          </div>
        </div>

        <hr class="my-4">

        <!-- Debounce Settings (Universal) -->
        <h6 class="mb-3 text-secondary">{{ $t('debounce_config') }}</h6>
        
        <div class="row g-3">
          <div class="col-md-6">
            <div class="card h-100">
              <div class="card-body">
                <div class="mb-0">
                  <label class="form-label small">{{ $t('offline_debounce') }}</label>
                  <div class="input-group input-group-sm">
                    <input v-model.number="onlineRule.offline_debounce" type="number" min="0" class="form-control">
                    <span class="input-group-text">Times</span>
                  </div>
                  <div class="form-text small mt-2">{{ $t('debounce_hint') }}</div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- Events Tab -->
      <div v-if="activeTab === 'events'">
        <div class="row h-100">
            <!-- Left: Event List -->
            <div class="col-md-4 border-end">
                <div class="list-group list-group-flush">
                    <button v-for="evt in tslEvents" :key="evt.identifier" 
                            class="list-group-item list-group-item-action d-flex justify-content-between align-items-center"
                            :class="{ active: selectedEventIdentifier === evt.identifier }"
                            @click="selectEvent(evt)">
                        <div>
                            <div class="fw-bold">{{ evt.name }}</div>
                            <small :class="selectedEventIdentifier === evt.identifier ? 'text-white-50' : 'text-muted'">{{ evt.identifier }}</small>
                        </div>
                        <i v-if="hasRule(evt.identifier)" class="bi bi-check-circle-fill" :class="selectedEventIdentifier === evt.identifier ? 'text-white' : 'text-success'"></i>
                    </button>
                    <div v-if="!tslEvents || tslEvents.length === 0" class="p-3 text-center text-muted">
                        {{ $t('tsl_no_data') }}
                    </div>
                </div>
            </div>

            <!-- Right: Configuration -->
            <div class="col-md-8">
                <div v-if="selectedEventIdentifier" class="p-3">
                    <div class="d-flex justify-content-between align-items-center mb-4">
                        <h5 class="mb-0">{{ getEventName(selectedEventIdentifier) }}</h5>
                        <div class="form-check form-switch">
                            <input class="form-check-input" type="checkbox" :id="'enableRule-'+selectedEventIdentifier"
                                   :checked="hasRule(selectedEventIdentifier)"
                                   @change="toggleRule(selectedEventIdentifier, $event.target.checked)">
                            <label class="form-check-label" :for="'enableRule-'+selectedEventIdentifier">{{ $t('enable_event_rule') }}</label>
                        </div>
                    </div>

                    <div v-if="activeEventRule">
                        <!-- Triggers -->
                        <div class="card mb-3 border-light bg-light">
                            <div class="card-header bg-transparent border-0 d-flex justify-content-between align-items-center">
                                <div class="d-flex align-items-center gap-2">
                                    <span class="fw-bold small text-uppercase text-muted">{{ $t('trigger_condition') }}</span>
                                    <select v-if="activeEventRule.trigger_logic !== undefined" v-model="activeEventRule.trigger_logic" class="form-select form-select-sm py-0 ps-2 pe-4" style="width: auto; height: 24px; font-size: 0.8rem;">
                                        <option value="or">{{ $t('logic_or') }}</option>
                                        <option value="and">{{ $t('logic_and') }}</option>
                                    </select>
                                    <select v-else :value="'or'" @change="activeEventRule.trigger_logic = $event.target.value" class="form-select form-select-sm py-0 ps-2 pe-4" style="width: auto; height: 24px; font-size: 0.8rem;">
                                        <option value="or">{{ $t('logic_or') }}</option>
                                        <option value="and">{{ $t('logic_and') }}</option>
                                    </select>
                                </div>
                                <button class="btn btn-sm btn-outline-primary py-0" @click="addTrigger">
                                    <i class="bi bi-plus"></i> {{ $t('add') }}
                                </button>
                            </div>
                            <div class="card-body pt-0">
                                <div v-if="!activeEventRule.triggers || activeEventRule.triggers.length === 0" class="text-muted small text-center py-2">
                                    {{ $t('tsl_no_data') }}
                                </div>
                                <div v-else v-for="(trigger, idx) in activeEventRule.triggers" :key="idx" class="d-flex gap-2 mb-2 align-items-center">
                                     <select v-model="trigger.point" class="form-select form-select-sm">
                                        <option value="" disabled>{{ $t('select_point') }}</option>
                                        <option v-for="pt in availablePoints" :key="pt.value" :value="pt.value">{{ pt.label }}</option>
                                     </select>
                                     <select v-model="trigger.operator" class="form-select form-select-sm" style="max-width: 100px">
                                        <option value=">">&gt;</option>
                                        <option value="<">&lt;</option>
                                        <option value="==">==</option>
                                        <option value="!=">!=</option>
                                        <option value=">=">&gt;=</option>
                                        <option value="<=">&lt;=</option>
                                        <option value="change">{{ $t('op_change') }}</option>
                                     </select>
                                     <input v-if="trigger.operator !== 'change'" v-model.number="trigger.value" type="number" class="form-control form-control-sm" :placeholder="$t('dev_data_val_placeholder')">
                                     <button class="btn btn-sm btn-link text-danger p-0" @click="removeTrigger(idx)">
                                        <i class="bi bi-trash"></i>
                                     </button>
                                </div>
                            </div>
                        </div>

                        <!-- Conditions -->
                        <div class="card mb-3 border-light bg-light">
                            <div class="card-header bg-transparent border-0 d-flex justify-content-between align-items-center">
                                <div class="d-flex align-items-center gap-2">
                                    <span class="fw-bold small text-uppercase text-muted">{{ $t('judgment_condition') }}</span>
                                    <select v-if="activeEventRule.condition_logic !== undefined" v-model="activeEventRule.condition_logic" class="form-select form-select-sm py-0 ps-2 pe-4" style="width: auto; height: 24px; font-size: 0.8rem;">
                                        <option value="and">{{ $t('logic_and') }}</option>
                                        <option value="or">{{ $t('logic_or') }}</option>
                                    </select>
                                    <select v-else :value="'and'" @change="activeEventRule.condition_logic = $event.target.value" class="form-select form-select-sm py-0 ps-2 pe-4" style="width: auto; height: 24px; font-size: 0.8rem;">
                                        <option value="and">{{ $t('logic_and') }}</option>
                                        <option value="or">{{ $t('logic_or') }}</option>
                                    </select>
                                </div>
                                <button class="btn btn-sm btn-outline-primary py-0" @click="addCondition">
                                    <i class="bi bi-plus"></i> {{ $t('add') }}
                                </button>
                            </div>
                            <div class="card-body pt-0">
                                <div v-if="!activeEventRule.conditions || activeEventRule.conditions.length === 0" class="text-muted small text-center py-2">
                                    {{ $t('tsl_no_data') }}
                                </div>
                                <div v-else v-for="(cond, idx) in activeEventRule.conditions" :key="idx" class="d-flex gap-2 mb-2 align-items-center">
                                     <select v-model="cond.point" class="form-select form-select-sm">
                                        <option value="" disabled>{{ $t('select_point') }}</option>
                                        <option v-for="pt in availablePoints" :key="pt.value" :value="pt.value">{{ pt.label }}</option>
                                     </select>
                                     <select v-model="cond.operator" class="form-select form-select-sm" style="max-width: 100px">
                                        <option value=">">&gt;</option>
                                        <option value="<">&lt;</option>
                                        <option value="==">==</option>
                                        <option value="!=">!=</option>
                                        <option value=">=">&gt;=</option>
                                        <option value="<=">&lt;=</option>
                                     </select>
                                     <input v-model.number="cond.value" type="number" class="form-control form-control-sm" :placeholder="$t('dev_data_val_placeholder')">
                                     <button class="btn btn-sm btn-link text-danger p-0" @click="removeCondition(idx)">
                                        <i class="bi bi-trash"></i>
                                     </button>
                                </div>
                            </div>
                        </div>

                        <!-- Report Interval -->
                        <div class="card mb-3 border-light bg-light">
                            <div class="card-header bg-transparent border-0 d-flex justify-content-between align-items-center">
                                <div class="d-flex align-items-center gap-2">
                                    <span class="fw-bold small text-uppercase text-muted">{{ $t('report_interval') }}</span>
                                </div>
                            </div>
                            <div class="card-body pt-0">
                                <div class="input-group input-group-sm">
                                    <input v-model.number="activeEventRule.report_interval" type="number" class="form-control" placeholder="-1">
                                    <span class="input-group-text">{{ $t('seconds') }}</span>
                                </div>
                                <div class="form-text small mt-1">
                                    {{ $t('report_interval_hint') }}
                                </div>
                            </div>
                        </div>

                        <!-- Params -->
                        <div class="mb-3" v-if="getEventParams(selectedEventIdentifier).length > 0">
                            <label class="form-label small text-muted mb-1">{{ $t('event_params_binding') }}</label>
                            <div v-for="param in getEventParams(selectedEventIdentifier)" :key="param.identifier" class="d-flex align-items-center mb-2">
                               <span class="small me-2 text-nowrap" style="width: 120px; overflow:hidden; text-overflow:ellipsis" :title="param.name">{{ param.name }} ({{ param.identifier }})</span>
                               <select v-model="activeEventRule.params[param.identifier]" class="form-select form-select-sm">
                                  <option value="">{{ $t('select_point_optional') }}</option>
                                  <option v-for="pt in availablePoints" :key="pt.value" :value="pt.value">{{ pt.label }}</option>
                               </select>
                            </div>
                        </div>

                    </div>
                    <div v-else class="text-center text-muted py-5">
                        {{ $t('no_event_rules') }}
                    </div>
                </div>
                <div v-else class="h-100 d-flex align-items-center justify-content-center text-muted">
                    <div class="text-center">
                        <i class="bi bi-hand-index-thumb fs-1 mb-2"></i>
                        <p>{{ $t('select_event') }}</p>
                    </div>
                </div>
            </div>
        </div>
      </div>

    </div>

    <!-- Warning if GatewayPolling but no groups defined -->
    <div v-if="strategy === 'GatewayPolling' && localCollectionMode === 'manual' && availablePollingGroups.length === 0" class="alert alert-warning">
       {{ isSubDevice ? $t('warn_no_parent_poll_group') : $t('warn_setup_poll_group') }}
    </div>

    <!-- Add/Edit Custom Point Modal -->
    <div v-if="showCustomPointModal" class="modal fade show d-block" style="background: rgba(0,0,0,0.5)">
      <div class="modal-dialog">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title">{{ editingCustomPointName ? $t('tsl_edit') : $t('add_point') }}</h5>
            <button type="button" class="btn-close" @click="closeCustomPointModal"></button>
          </div>
          <div class="modal-body">
            <div class="mb-3">
                <label class="form-label required">{{ $t('point_name') }}</label>
                <input v-model="currentCustomPointData.display_name" type="text" class="form-control" :placeholder="$t('point_name_hint')">
            </div>
            <div class="mb-3">
                <label class="form-label required">{{ $t('tsl_identifier') }}</label>
                <input v-model="currentCustomPointData.name" type="text" class="form-control" :placeholder="$t('tsl_identifier')" :disabled="!!editingCustomPointName">
            </div>

            <!-- Gateway Polling Strategy -->
            <div v-if="strategy === 'GatewayPolling'">
                <!-- Manual Mode -->
                <div v-if="localCollectionMode === 'manual'">
                  <div class="mb-3">
                    <label class="form-label">{{ $t('tsl_prop_poll_group') }}</label>
                    <select v-if="availablePollingGroups && availablePollingGroups.length > 0" v-model="currentCustomPointData.polling_group" class="form-select">
                       <option value="" disabled>{{ $t('tsl_placeholder_group') }}</option>
                       <option v-for="g in availablePollingGroups" :key="g.name" :value="g.name">{{ g.name }} (Slave: {{ g.slave_id }})</option>
                    </select>
                    <input v-else v-model="currentCustomPointData.polling_group" type="text" class="form-control" :placeholder="$t('tsl_placeholder_group')">
                  </div>
                  <div class="mb-3">
                    <label class="form-label">{{ $t('tsl_prop_offset') }}</label>
                    <input v-model.number="currentCustomPointData.offset" type="number" class="form-control">
                  </div>
                </div>

                <!-- Auto Mode -->
                <div v-else>
                    <div class="row">
                        <div class="col-md-6 mb-3">
                            <label class="form-label">{{ $t('slave_id') }}</label>
                            <input v-model.number="currentCustomPointData.slave_id" type="number" class="form-control">
                        </div>
                        <div class="col-md-6 mb-3">
                            <label class="form-label">{{ $t('func_code') }}</label>
                            <select v-model.number="currentCustomPointData.function_code" class="form-select">
                                <option :value="1">{{ $t('modbus_fc_coil') }}</option>
                                <option :value="2">{{ $t('modbus_fc_input') }}</option>
                                <option :value="3">{{ $t('modbus_fc_hold') }}</option>
                                <option :value="4">{{ $t('modbus_fc_input_reg') }}</option>
                            </select>
                        </div>
                    </div>
                    <div class="mb-3">
                        <label class="form-label">{{ $t('start_addr') }} {{ $t('hint_register') }}</label>
                        <input v-model.number="currentCustomPointData.address" type="number" class="form-control">
                    </div>
                    <div class="mb-3">
                        <label class="form-label">{{ $t('interval') }} (ms)</label>
                        <input v-model.number="currentCustomPointData.interval" type="number" class="form-control" placeholder="1000">
                    </div>
                </div>

                <div class="mb-3">
                  <label class="form-label">{{ $t('tsl_prop_raw_type') }}</label>
                  <select v-model="currentCustomPointData.data_type" class="form-select">
                    <option value="uint16">uint16</option>
                    <option value="int16">int16</option>
                    <option value="uint32">uint32</option>
                    <option value="int32">int32</option>
                    <option value="float32">float32</option>
                    <option value="float64">float64</option>
                    <option value="bool">bool</option>
                  </select>
                </div>
                <div class="mb-3">
                  <label class="form-label">{{ $t('tsl_prop_byte_order') }}</label>
                  <select v-model="currentCustomPointData.byte_order" class="form-select">
                    <option value="ABCD">{{ $t('tsl_prop_byte_order_big') }}</option>
                    <option value="DCBA">{{ $t('tsl_prop_byte_order_little') }}</option>
                    <option value="CDAB">{{ $t('tsl_prop_byte_order_cdab') }}</option>
                    <option value="BADC">{{ $t('tsl_prop_byte_order_badc') }}</option>
                  </select>
                </div>
                <div class="mb-3">
                  <label class="form-label">{{ $t('tsl_prop_read_expr') }}</label>
                  <input v-model="currentCustomPointData.read_expr" type="text" class="form-control" :placeholder="$t('hint_read_expr')">
                </div>
                <div v-if="shouldShowPrecision(currentCustomPointData.data_type, currentCustomPointData.read_expr)" class="mb-3">
                  <label class="form-label">{{ $t('tsl_prop_precision') }}</label>
                  <input v-model.number="currentCustomPointData.precision" type="number" class="form-control" placeholder="e.g. 2">
                </div>

                <div v-if="currentCustomPointData.data_type === 'bool'" class="mb-3">
                  <label class="form-label">{{ $t('tsl_prop_bit_index') }}</label>
                  <input v-model.number="currentCustomPointData.extract_rule.bit_index" type="number" class="form-control">
                </div>

                <div v-if="currentCustomPointData.data_type === 'string' || currentCustomPointData.data_type === 'hex'" class="mb-3">
                  <label class="form-label">{{ $t('tsl_prop_length') }}</label>
                  <input v-model.number="currentCustomPointData.extract_rule.length" type="number" class="form-control">
                </div>

            </div>

            <!-- Direct Access Strategy -->
            <div v-else>
                <div class="mb-3">
                  <label class="form-label">{{ $t('tsl_prop_address') }}</label>
                  <input v-model="currentCustomPointData.address" type="text" class="form-control" :placeholder="$t('tsl_placeholder_address')">
                </div>
                <div class="mb-3">
                  <label class="form-label">{{ $t('tsl_prop_data_type') }}</label>
                  <input v-model="currentCustomPointData.data_type" type="text" class="form-control" :placeholder="$t('hint_direct_access_type')">
                </div>
            </div>

          </div>
          <div class="modal-footer">
            <button type="button" class="btn btn-secondary" @click="closeCustomPointModal">{{ $t('tsl_cancel') }}</button>
            <button type="button" class="btn btn-primary" @click="saveCustomPoint">{{ $t('common_save') }}</button>
          </div>
        </div>
      </div>
    </div>

    <!-- Edit Mapping Modal -->
    <div v-if="showModal" class="modal fade show d-block" style="background: rgba(0,0,0,0.5)">
      <div class="modal-dialog">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title">{{ $t('tsl_prop_proto_map') }} - {{ currentProp?.name }}</h5>
            <button type="button" class="btn-close" @click="closeModal"></button>
          </div>
          <div class="modal-body">
            <div class="form-check mb-3">
              <input class="form-check-input" type="checkbox" id="enableMapping" v-model="hasMapping">
              <label class="form-check-label" for="enableMapping">
                {{ $t('tsl_prop_enable_map') }}
              </label>
            </div>

            <div v-if="hasMapping">
              <!-- Gateway Polling Strategy (Modbus) -->
              <div v-if="strategy === 'GatewayPolling'">
                <!-- Manual Mode: Group + Offset -->
                <div v-if="localCollectionMode === 'manual'">
                  <div class="mb-3">
                    <label class="form-label">
                      {{ $t('tsl_prop_poll_group') }}
                      <i class="bi bi-question-circle ms-1" :title="$t('desc_poll_group')" style="cursor: help;"></i>
                    </label>
                    <select v-if="availablePollingGroups && availablePollingGroups.length > 0" v-model="currentMapping.polling_group" class="form-select">
                       <option value="" disabled>{{ $t('tsl_placeholder_group') }}</option>
                       <option v-for="g in availablePollingGroups" :key="g.name" :value="g.name">{{ g.name }} (Slave: {{ g.slave_id }})</option>
                    </select>
                    <input v-else v-model="currentMapping.polling_group" type="text" class="form-control" :placeholder="$t('tsl_placeholder_group')">
                    <div v-if="!availablePollingGroups || availablePollingGroups.length === 0" class="form-text text-warning">
                      {{ $t('tsl_warn_no_groups') }}
                    </div>
                  </div>
                  <div class="mb-3">
                    <label class="form-label">
                      {{ $t('tsl_prop_offset') }}
                      <i class="bi bi-question-circle ms-1" :title="$t('desc_offset')" style="cursor: help;"></i>
                    </label>
                    <input v-model.number="currentMapping.offset" type="number" class="form-control">
                  </div>
                </div>

                <!-- Auto Mode: Direct Address -->
                <div v-else>
                    <div class="row">
                        <div class="col-md-6 mb-3">
                            <label class="form-label">
                                {{ $t('slave_id') }}
                                <i class="bi bi-question-circle ms-1" :title="$t('desc_slave_id')" style="cursor: help;"></i>
                            </label>
                            <input v-model.number="currentMapping.slave_id" type="number" class="form-control">
                        </div>
                        <div class="col-md-6 mb-3">
                            <label class="form-label">
                                {{ $t('func_code') }}
                                <i class="bi bi-question-circle ms-1" :title="$t('desc_func_code')" style="cursor: help;"></i>
                            </label>
                            <select v-model.number="currentMapping.function_code" class="form-select">
                                <option :value="1">{{ $t('modbus_fc_coil') }}</option>
                                <option :value="2">{{ $t('modbus_fc_input') }}</option>
                                <option :value="3">{{ $t('modbus_fc_hold') }}</option>
                                <option :value="4">{{ $t('modbus_fc_input_reg') }}</option>
                            </select>
                        </div>
                    </div>
                    <div class="mb-3">
                        <label class="form-label">
                            {{ $t('start_addr') }} {{ $t('hint_register') }}
                            <i class="bi bi-question-circle ms-1" :title="$t('desc_start_addr')" style="cursor: help;"></i>
                        </label>
                        <input v-model.number="currentMapping.address" type="number" class="form-control">
                    </div>
                    <div class="mb-3">
                        <label class="form-label">
                            {{ $t('interval') }} (ms)
                            <i class="bi bi-question-circle ms-1" :title="$t('desc_interval')" style="cursor: help;"></i>
                        </label>
                        <input v-model.number="currentMapping.interval" type="number" class="form-control" placeholder="1000">
                    </div>
                </div>

                <div class="mb-3">
                  <label class="form-label">
                    {{ $t('tsl_prop_raw_type') }}
                    <i class="bi bi-question-circle ms-1" :title="$t('desc_raw_type')" style="cursor: help;"></i>
                  </label>
                  <select v-model="currentMapping.data_type" class="form-select">
                    <option value="uint16">uint16</option>
                    <option value="int16">int16</option>
                    <option value="uint32">uint32</option>
                    <option value="int32">int32</option>
                    <option value="float32">float32</option>
                    <option value="float64">float64</option>
                    <option value="bool">bool</option>
                  </select>
                </div>
                <div class="mb-3">
                  <label class="form-label">
                    {{ $t('tsl_prop_byte_order') }}
                    <i class="bi bi-question-circle ms-1" :title="$t('desc_byte_order')" style="cursor: help;"></i>
                  </label>
                  <select v-model="currentMapping.byte_order" class="form-select">
                    <option value="ABCD">{{ $t('tsl_prop_byte_order_big') }}</option>
                    <option value="DCBA">{{ $t('tsl_prop_byte_order_little') }}</option>
                    <option value="CDAB">{{ $t('tsl_prop_byte_order_cdab') }}</option>
                    <option value="BADC">{{ $t('tsl_prop_byte_order_badc') }}</option>
                  </select>
                </div>
                <!-- V2 Advanced Read -->
        <div class="mb-3">
          <label class="form-label">
            {{ $t('tsl_prop_read_expr') }}
            <i class="bi bi-question-circle ms-1" :title="$t('desc_read_expr')" style="cursor: help;"></i>
          </label>
          <input v-model="currentMapping.read_expr" type="text" class="form-control" placeholder="e.g. x * 0.1">
        </div>
        <div v-if="shouldShowPrecision(currentMapping.data_type, currentMapping.read_expr)" class="mb-3">
          <label class="form-label">
            {{ $t('tsl_prop_precision') }}
            <i class="bi bi-question-circle ms-1" :title="$t('desc_precision')" style="cursor: help;"></i>
          </label>
          <input v-model.number="currentMapping.precision" type="number" class="form-control" placeholder="e.g. 2">
        </div>

                <div v-if="currentMapping.data_type === 'bool' && !isFC1or2" class="mb-3">
                  <label class="form-label">
                    {{ $t('tsl_prop_bit_index') }}
                    <i class="bi bi-question-circle ms-1" :title="$t('desc_bit_index')" style="cursor: help;"></i>
                  </label>
                  <input v-model.number="currentMapping.extract_rule.bit_index" type="number" class="form-control" min="0" max="15">
                </div>

                <div v-if="currentMapping.data_type === 'string' || currentMapping.data_type === 'hex'" class="mb-3">
                  <label class="form-label">
                    {{ $t('tsl_prop_length') }}
                    <i class="bi bi-question-circle ms-1" :title="$t('desc_length')" style="cursor: help;"></i>
                  </label>
                  <input v-model.number="currentMapping.extract_rule.length" type="number" class="form-control">
                </div>


                <!-- Write Configuration -->
                <div class="border-top pt-3 mt-3">
                   <div class="form-check mb-3">
                      <input class="form-check-input" type="checkbox" id="enableWrite" v-model="currentMapping.enable_write" :disabled="currentProp?.accessMode === 'r'">
                      <label class="form-check-label fw-bold" for="enableWrite">
                        {{ $t('tsl_prop_enable_write') }}
                        <i class="bi bi-question-circle ms-1" :title="$t('desc_enable_write')" style="cursor: help;"></i>
                      </label>
                      <span v-if="currentProp?.accessMode === 'r'" class="badge bg-secondary ms-2">{{ $t('tsl_prop_access_r') }}</span>
                   </div>

                   <div v-if="currentMapping.enable_write" class="ps-3 border-start">
                      <div class="mb-3">
                        <label class="form-label">
                          {{ $t('tsl_prop_write_mode') }}
                          <i class="bi bi-question-circle ms-1" :title="$t('desc_write_mode')" style="cursor: help;"></i>
                        </label>
                        <select v-model="currentMapping.write_mode" class="form-select">
                          <option value="same_as_read">{{ $t('tsl_prop_write_mode_same') }}</option>
                          <option value="custom">{{ $t('tsl_prop_write_mode_custom') }}</option>
                        </select>
                      </div>




                      <div v-if="currentMapping.write_mode === 'custom'">
                         <div class="mb-3">
                            <label class="form-label">
                               {{ $t('tsl_prop_write_addr') }}
                               <i class="bi bi-question-circle ms-1" :title="$t('desc_write_addr')" style="cursor: help;"></i>
                            </label>
                            <input v-model.number="currentMapping.write_address" type="number" class="form-control">
                         </div>
                         <div class="mb-3">
                            <label class="form-label">
                               {{ $t('tsl_prop_write_func') }}
                               <i class="bi bi-question-circle ms-1" :title="$t('desc_write_func')" style="cursor: help;"></i>
                            </label>
                            <select v-model.number="currentMapping.write_function_code" class="form-select">
                               <option :value="5">{{ $t('modbus_fc_write_single_coil') }}</option>
                               <option :value="6">{{ $t('modbus_fc_write_single_reg') }}</option>
                               <option :value="15">{{ $t('modbus_fc_write_multi_coil') }}</option>
                               <option :value="16">{{ $t('modbus_fc_write_multi_reg') }}</option>
                            </select>
                         </div>
                         <div class="mb-3">
                            <label class="form-label">
                               {{ $t('tsl_prop_write_slave') }}
                               <i class="bi bi-question-circle ms-1" :title="$t('desc_write_slave')" style="cursor: help;"></i>
                            </label>
                            <input v-model.number="currentMapping.write_slave_id" type="number" class="form-control" :placeholder="$t('hint_write_slave_default')">
                         </div>
                      </div>

                      <div class="mb-3">
                        <label class="form-label">
                          {{ $t('tsl_prop_write_expr') }}
                          <i class="bi bi-question-circle ms-1" :title="$t('desc_write_expr')" style="cursor: help;"></i>
                        </label>
                        <input v-model="currentMapping.write_expr" type="text" class="form-control" placeholder="e.g. x * 10">
                      </div>
                   </div>
                </div>
              </div>

              <!-- Direct Access Strategy (BACnet, etc.) -->
              <div v-else>
                <!-- Custom BACnet Form -->
                <div v-if="isBacnet">
                    <div class="mb-3">
                        <label class="form-label">Object Type</label>
                        <select v-model.number="currentMapping.object_type" class="form-select">
                            <option :value="0">Analog Input (AI)</option>
                            <option :value="1">Analog Output (AO)</option>
                            <option :value="2">Analog Value (AV)</option>
                            <option :value="3">Binary Input (BI)</option>
                            <option :value="4">Binary Output (BO)</option>
                            <option :value="5">Binary Value (BV)</option>
                            <option :value="8">Device</option>
                            <option :value="13">Multi-state Input (MI)</option>
                            <option :value="14">Multi-state Output (MO)</option>
                            <option :value="19">Multi-state Value (MV)</option>
                        </select>
                    </div>
                    <div class="row">
                        <div class="col-md-6 mb-3">
                            <label class="form-label">Instance ID</label>
                            <input v-model.number="currentMapping.instance_id" type="number" class="form-control" min="0">
                        </div>
                        <div class="col-md-6 mb-3">
                            <label class="form-label">Property ID</label>
                            <input v-model.number="currentMapping.property_id" type="number" class="form-control" placeholder="Default 85">
                        </div>
                    </div>
                    <div class="mb-3">
                        <label class="form-label">Interval (ms)</label>
                        <input v-model.number="currentMapping.poll_interval" type="number" class="form-control" placeholder="Default 5000">
                    </div>
                    <div class="mb-3">
                        <label class="form-label">{{ $t('tsl_prop_read_expr') }}</label>
                        <input v-model="currentMapping.read_expr" type="text" class="form-control" placeholder="e.g. x * 0.1">
                    </div>

                    <!-- Write Configuration -->
                    <div class="border-top pt-3 mt-3">
                       <div class="form-check mb-3">
                          <input class="form-check-input" type="checkbox" id="bacnetEnableWrite" v-model="currentMapping.enable_write" :disabled="currentProp?.accessMode === 'r'">
                          <label class="form-check-label fw-bold" for="bacnetEnableWrite">
                            {{ $t('tsl_prop_enable_write') }}
                          </label>
                       </div>

                       <div v-if="currentMapping.enable_write" class="ps-3 border-start">
                           <div class="mb-3">
                               <label class="form-label">Write Priority</label>
                               <input v-model.number="currentMapping.write_priority" type="number" class="form-control" min="1" max="16" placeholder="Default 16">
                           </div>
                           <div class="mb-3">
                               <label class="form-label">{{ $t('tsl_prop_write_expr') }}</label>
                               <input v-model="currentMapping.write_expr" type="text" class="form-control" placeholder="e.g. x * 10">
                           </div>
                       </div>
                    </div>
                </div>

                <!-- Generic Direct Access (Script, etc.) -->
                <div v-else>
                    <div v-if="pointSchema">
                      <SchemaForm :schema="pointSchema" v-model="currentMapping" />
                    </div>
                    <div v-else>
                      <div class="mb-3">
                        <label class="form-label">{{ $t('tsl_prop_address') }}</label>
                        <input v-model="currentMapping.address" type="text" class="form-control" :placeholder="$t('tsl_placeholder_address')">
                        <div class="form-text">{{ $t('tsl_hint_address') }}</div>
                      </div>
                      <div class="mb-3">
                        <label class="form-label">{{ $t('tsl_prop_data_type') }}</label>
                        <input v-model="currentMapping.data_type" type="text" class="form-control" :placeholder="$t('hint_direct_access_type')">
                      </div>
                    </div>
                </div>
              </div>
            </div>
          </div>
          <div class="modal-footer">
            <button type="button" class="btn btn-secondary" style="min-width: 80px" @click="closeModal">{{ $t('tsl_cancel') }}</button>
            <button type="button" class="btn btn-primary" style="min-width: 80px" @click="saveMapping">{{ $t('common_save') }}</button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, watch, onMounted } from 'vue';
import { useI18n } from 'vue-i18n';
import SchemaForm from '../SchemaForm.vue';
import axios from 'axios';

const { t } = useI18n();

const getTypeLength = (dataType) => {
    switch (String(dataType).toLowerCase()) {
        case 'bool': return 1;
        case 'int16':
        case 'uint16': return 2;
        case 'int32':
        case 'uint32':
        case 'float32': return 4;
        case 'int64':
        case 'uint64':
        case 'float64': return 8;
        default: return 0; // Should be handled, maybe 1 or 2? Backend defaults to 1 if 0.
    }
};

const autoGeneratedGroups = computed(() => {
    if (localCollectionMode.value !== 'auto') return [];
    
    const points = [];
    if (props.deviceConfig && props.deviceConfig.points) {
        points.push(...props.deviceConfig.points);
    }
    
    if (points.length === 0) return [];

    const maxLen = props.deviceConfig.max_group_length || 120;
    const maxGap = props.deviceConfig.max_address_gap || 20;
    const defaultSlaveID = props.deviceConfig.slave_id || 1;

    // Group by key
    const groups = {};
    points.forEach(pt => {
        const sid = pt.slave_id || defaultSlaveID;
        const fc = pt.function_code || 3;

        let interval = pt.interval;
        if (!interval || interval <= 0) {
             interval = 1000;
        }

        const key = `${sid}_${fc}_${interval}`;
        if (!groups[key]) groups[key] = [];
        groups[key].push({
            ...pt,
            slave_id: sid,
            function_code: fc,
            interval: interval,
            address: Number(pt.address)
        });
    });

    const result = [];

    Object.keys(groups).forEach(key => {
        const pts = groups[key];
        pts.sort((a, b) => a.address - b.address);

        const [sid, fc, interval] = key.split('_').map(Number);
        
        let currentStart = -1;
        let currentEnd = -1;
        let chunkPoints = [];

        const flushChunk = () => {
            if (chunkPoints.length === 0) return;
            const length = currentEnd - currentStart;
            
            // Generate list of point names for tooltip
            const pointNames = chunkPoints.map(p => {
                const name = p.display_name || p.name;
                return `${name}（${p.name}）：${p.address}`;
            }).join(', ');

            result.push({
                name: `Auto_${sid}_${fc}_${interval}_${currentStart}_${length}`,
                slave_id: sid,
                function_code: fc,
                start_address: currentStart,
                length: length,
                interval: interval,
                pointCount: chunkPoints.length,
                pointNames: pointNames
            });
            chunkPoints = [];
        };

        pts.forEach(pt => {
            let ptLen = 0;
            if (fc === 1 || fc === 2) {
                ptLen = 1;
            } else {
                const bytes = getTypeLength(pt.data_type);
                ptLen = Math.floor((bytes + 1) / 2);
                if (ptLen === 0) ptLen = 1;
            }

            if (chunkPoints.length === 0) {
                currentStart = pt.address;
                currentEnd = pt.address + ptLen;
                chunkPoints.push(pt);
                return;
            }

            if (pt.address > currentEnd + maxGap) {
                flushChunk();
                currentStart = pt.address;
                currentEnd = pt.address + ptLen;
                chunkPoints.push(pt);
                return;
            }

            let newEnd = pt.address + ptLen;
            if (newEnd < currentEnd) newEnd = currentEnd;

            const newLen = newEnd - currentStart;
            if (newLen > maxLen) {
                flushChunk();
                currentStart = pt.address;
                currentEnd = pt.address + ptLen;
                chunkPoints.push(pt);
                return;
            }

            chunkPoints.push(pt);
            currentEnd = newEnd;
        });
        flushChunk();
    });

    return result;
});

const shouldShowPrecision = (dataType, readExpr) => {
  if (!dataType) return false;
  const type = String(dataType).toLowerCase();
  // 1. If it's explicitly a float type
  if (type.includes('float') || type.includes('double')) {
    return true;
  }
  // 2. If there's a read expression, the result is likely a float
  if (readExpr && String(readExpr).trim() !== '') {
    return true;
  }
  return false;
};

const props = defineProps({
  deviceConfig: {
    type: Object,
    default: () => ({})
  },
  tslProperties: {
    type: Array,
    default: () => []
  },
  tslEvents: {
    type: Array,
    default: () => []
  },
  pollingGroups: {
    type: Array,
    default: () => []
  },
  protocolName: {
    type: String,
    default: ''
  },
  isSubDevice: {
      type: Boolean,
      default: false
  },
  productCode: {
      type: String,
      default: ''
  },
  parentCode: {
      type: String,
      default: ''
  }
});

const emit = defineEmits(['update:deviceConfig']);

const showModal = ref(false);
const currentProp = ref(null);
const hasMapping = ref(false);
const activeTab = ref('properties');

const strategy = computed(() => {
  if (props.protocolName && props.protocolName.toLowerCase().includes('modbus')) {
    return 'GatewayPolling';
  }
  return 'DirectAccess';
});

const isBacnet = computed(() => {
    return props.protocolName && props.protocolName.toLowerCase().includes('bacnet');
});

const onlineRule = ref({
    strategy: 'communication',
    point: '',
    operator: '==',
    value: 1,
    online_debounce: 1,
    offline_debounce: 3,
    enable_value_check: false,
    monitor_point: '',
    max_unchanged_interval: 60
  });

const currentMapping = ref({
  polling_group: '',
  offset: 0,
  data_type: 'uint16',
  byte_order: 'ABCD',
  address: '',
  read_expr: '',
  extract_rule: {
    bit_index: 0,
    length: 0
  },
  enable_write: false,
  write_config: {}
});

const pointSchema = ref(null);

const fetchPointSchema = async () => {
    if (!props.productCode) {
        pointSchema.value = null;
        return;
    }
    // Only fetch for DirectAccess strategies (like BACnet), Modbus handles its own form
    if (strategy.value === 'GatewayPolling' || isBacnet.value) {
        pointSchema.value = null;
        return;
    }


    try {
        const params = new URLSearchParams();
        params.append('productCode', props.productCode);
        if (props.parentCode) params.append('parentCode', props.parentCode);
        params.append('type', 'point');
        
        const res = await axios.get(`/api/devices/config-schema?${params.toString()}`);
        if (res.data.code === 0 && res.data.data.schema) {
            pointSchema.value = res.data.data.schema;
        } else {
            pointSchema.value = null;
        }
    } catch (e) {
        console.error("Failed to fetch point schema", e);
        pointSchema.value = null;
    }
};

watch(() => props.protocolName, fetchPointSchema, { immediate: true });
watch(() => strategy.value, fetchPointSchema);


// Local state for polling groups (for Gateway/Direct devices)
const localPollingGroups = ref([]);
const isAutoGroupPreviewExpanded = ref(true);
const localEvents = ref([]);
const localCollectionMode = ref('manual');
const localConnection = ref({
    ip: '',
    port: 502,
    slave_id: 1,
    timeout_ms: 1000
});

const availablePollingGroups = computed(() => {
    if (props.isSubDevice) {
        return props.pollingGroups; // From parent
    }
    return localPollingGroups.value; // From self
});

const isFC1or2 = computed(() => {
    // Check Manual Mode (using polling_group)
    if (localCollectionMode.value === 'manual' && currentMapping.value.polling_group) {
        const group = availablePollingGroups.value.find(g => g.name === currentMapping.value.polling_group);
        if (group && (group.function_code === 1 || group.function_code === 2)) {
            return true;
        }
    }
    // Check Auto Mode (direct function_code)
    if (localCollectionMode.value === 'auto' && (currentMapping.value.function_code === 1 || currentMapping.value.function_code === 2)) {
        return true;
    }
    return false;
});

// Custom Points State
const showCustomPointModal = ref(false);
const editingCustomPointName = ref('');
const currentCustomPointData = ref({
    name: '',
    display_name: '',
    polling_group: '',
    offset: 0,
    data_type: 'uint16',
    byte_order: 'ABCD',
    slave_id: 1,
    function_code: 3,
    address: 0,
    length: 1
});

const getMapping = (identifier) => {
  if (!props.deviceConfig || !props.deviceConfig.points) return null;
  return props.deviceConfig.points.find(p => p.name === identifier);
};

const tslPointsList = computed(() => {
    const list = [];
    if (props.tslProperties && props.tslProperties.length > 0) {
        props.tslProperties.forEach(prop => {
            const mapping = getMapping(prop.identifier);
            list.push({
                identifier: prop.identifier,
                name: prop.name,
                dataType: prop.dataType?.type || '-',
                isMapped: !!mapping,
                isProperty: mapping ? (mapping.is_property !== false) : true, // Default to true if not set
                rawProp: prop,
                rawPoint: mapping
            });
        });
    } else if (props.deviceConfig && props.deviceConfig.points) {
        props.deviceConfig.points.forEach(pt => {
            // Accept points explicitly marked as properties, OR points with type 'property' that aren't marked false
            if (pt.is_property === true || (pt.type === 'property' && pt.is_property !== false)) {
                list.push({
                    identifier: pt.name,
                    name: pt.display_name || pt.name,
                    dataType: pt.data_type || '-',
                    isMapped: true,
                    isProperty: true,
                    rawProp: null,
                    rawPoint: pt
                });
            }
        });
    }
    return list;
});

const customPointsList = computed(() => {
    const list = [];
    if (props.deviceConfig && props.deviceConfig.points) {
        props.deviceConfig.points.forEach(pt => {
            const isTreatAsProperty = pt.is_property === true || (pt.type === 'property' && pt.is_property !== false);
            if ((!props.tslProperties || props.tslProperties.length === 0) && isTreatAsProperty) {
                return;
            }
            // Check if this point is already covered by TSL
            const isTslProp = props.tslProperties?.some(p => p.identifier === pt.name);
            if (!isTslProp) {
                list.push({
                    identifier: pt.name,
                    name: pt.display_name || pt.name,
                    dataType: pt.data_type,
                    isMapped: true,
                    isProperty: isTreatAsProperty, // Default to false for custom points usually, but respect config
                    rawProp: null,
                    rawPoint: pt
                });
            }
        });
    }
    return list;
});

const togglePointProperty = (pt, checked) => {
    const newConfig = JSON.parse(JSON.stringify(props.deviceConfig));
    if (!newConfig.points) return;
    
    const point = newConfig.points.find(p => p.name === pt.identifier);
    if (point) {
        point.is_property = checked;
        emit('update:deviceConfig', newConfig);
    }
};

const openAddCustomPointModal = () => {
    editingCustomPointName.value = '';
    // Reset defaults
    if (strategy.value === 'GatewayPolling') {
        currentCustomPointData.value = {
          name: '',
          display_name: '',
          polling_group: '',
          offset: 0,
          data_type: 'uint16',
          byte_order: 'ABCD',
          read_expr: '',
          extract_rule: {},
          slave_id: 1,
          function_code: 3,
          address: 0,
          length: 1,
          interval: 1000,
          write_expr: ''
        };
    } else {
        currentCustomPointData.value = {
            name: '',
            display_name: '',
            address: '',
            data_type: '',
            read_expr: '',
            write_expr: '',
            extract_rule: {}
        };
    }
    showCustomPointModal.value = true;
};

const editCustomPoint = (pt) => {
    editingCustomPointName.value = pt.name;
    currentCustomPointData.value = JSON.parse(JSON.stringify(pt));
    
    // Ensure V2 objects
    if (!currentCustomPointData.value.extract_rule) currentCustomPointData.value.extract_rule = {};
    
    if (!currentCustomPointData.value.display_name) {
        currentCustomPointData.value.display_name = pt.name;
    }
    showCustomPointModal.value = true;
};

const closeCustomPointModal = () => {
    showCustomPointModal.value = false;
};

const saveCustomPoint = () => {
    if (!currentCustomPointData.value.name) {
        alert(t('tsl_identifier') + ' required');
        return;
    }
    if (!currentCustomPointData.value.display_name) {
        alert(t('point_name_hint'));
        return;
    }

    // Check uniqueness if adding new
    if (!editingCustomPointName.value) {
        // Check against TSL Properties
        const isTsl = props.tslProperties?.some(p => p.identifier === currentCustomPointData.value.name);
        // Check against existing points
        const exists = props.deviceConfig.points?.some(p => p.name === currentCustomPointData.value.name);
        
        if (isTsl || exists) {
            alert(t('tsl_prop_id_exists'));
            return;
        }
    }

    // Validation (reuse logic roughly)
    if (strategy.value === 'GatewayPolling') {
        if (localCollectionMode.value === 'manual' && !currentCustomPointData.value.polling_group) {
             alert(t('tsl_poll_group_required'));
             return;
        }
    } else {
        if (!currentCustomPointData.value.address) {
             alert(t('tsl_address_required'));
             return;
        }
    }

    const newConfig = JSON.parse(JSON.stringify(props.deviceConfig));
    if (!newConfig.points) newConfig.points = [];

    // Remove old if editing
    if (editingCustomPointName.value) {
        const idx = newConfig.points.findIndex(p => p.name === editingCustomPointName.value);
        if (idx !== -1) newConfig.points.splice(idx, 1);
    }

    // Add new with is_property: false by default for custom points, but user can toggle later
    // We set it to false initially so it doesn't pollute properties unless explicitly enabled
    newConfig.points.push({
        ...currentCustomPointData.value,
        is_property: false
    });

    emit('update:deviceConfig', newConfig);
    closeCustomPointModal();
};

const deleteCustomPoint = (pt) => {
    if (!confirm(t('delete_point_confirm'))) return;
    
    const newConfig = JSON.parse(JSON.stringify(props.deviceConfig));
    const idx = newConfig.points.findIndex(p => p.name === pt.name);
    if (idx !== -1) {
        newConfig.points.splice(idx, 1);
        emit('update:deviceConfig', newConfig);
    }
};

const isInitializing = ref(false);

const initLocalState = () => {
    isInitializing.value = true;
    const newVal = props.deviceConfig || {};
    if (newVal) {
        if (newVal.polling_groups) {
             localPollingGroups.value = JSON.parse(JSON.stringify(newVal.polling_groups));
        } else {
            localPollingGroups.value = [];
        }
        
        if (newVal.events) {
            // Deep copy and initialize defaults
            localEvents.value = newVal.events.map(evt => ({
                ...evt,
                report_interval: (evt.report_interval !== undefined && evt.report_interval !== null) ? evt.report_interval : -1,
                triggers: evt.triggers || [],
                conditions: evt.conditions || []
            }));
        } else {
            localEvents.value = [];
        }

        const newMode = newVal.collection_mode === 'AutoReport' ? 'auto' : (newVal.collection_mode || 'manual');
        localCollectionMode.value = newMode;

        // Sync connection info if present
        if (newVal.ip) localConnection.value.ip = newVal.ip;
        if (newVal.port) localConnection.value.port = newVal.port;

        const defaultRule = {
            strategy: 'communication',
            point: '',
            operator: '==',
            value: 1,
            offline_debounce: 2,
            monitor_point: '',
            max_unchanged_interval: 60
        };
        
        onlineRule.value = {
            ...defaultRule,
            ...(newVal.online_rule || {})
        };
    }
    // Ensure watchers don't fire during init
    setTimeout(() => {
        isInitializing.value = false;
    }, 0);
};

onMounted(() => {
    initLocalState();
});

// Simple debounce utility
const debounce = (fn, delay) => {
    let timeoutId;
    return (...args) => {
        clearTimeout(timeoutId);
        timeoutId = setTimeout(() => fn(...args), delay);
    };
};

const emitUpdate = (newConfig) => {
    emit('update:deviceConfig', newConfig);
};

const debouncedEmitUpdate = debounce(emitUpdate, 300);

// Sync local polling groups back to device config
watch(localPollingGroups, (newGroups) => {
    if (isInitializing.value) return;
    if (!props.isSubDevice && strategy.value === 'GatewayPolling') {
        // Strip internal flags before emitting
        const cleanGroups = newGroups.map(g => {
            const { _isNew, ...rest } = g;
            return rest;
        });
        
        // Deep compare with prop to avoid echo
        if (JSON.stringify(cleanGroups) !== JSON.stringify(props.deviceConfig.polling_groups)) {
            const newConfig = { ...props.deviceConfig, polling_groups: cleanGroups };
            debouncedEmitUpdate(newConfig);
        }
    }
}, { deep: true });

// Sync local events back to device config
watch(localEvents, (newEvents) => {
    if (isInitializing.value) return;
    if (JSON.stringify(newEvents) !== JSON.stringify(props.deviceConfig.events)) {
        const newConfig = { ...props.deviceConfig, events: newEvents };
        debouncedEmitUpdate(newConfig);
    }
}, { deep: true });

// Sync collection mode
watch(localCollectionMode, (newMode) => {
    if (isInitializing.value) return;
    if (!props.isSubDevice && strategy.value === 'GatewayPolling') {
        // Check if different from prop
        // Prop logic: 'AutoReport' -> 'auto', else 'manual'
        const currentPropMode = props.deviceConfig.collection_mode === 'AutoReport' ? 'auto' : (props.deviceConfig.collection_mode || 'manual');
        
        if (newMode !== currentPropMode) {
            const newConfig = { ...props.deviceConfig, collection_mode: newMode };
            // Force sync connection info as well to ensure it's not lost
            if (props.deviceConfig.host && !newConfig.ip) newConfig.ip = props.deviceConfig.host;
            debouncedEmitUpdate(newConfig);
        }
    }
});

watch(onlineRule, (newVal) => {
    if (isInitializing.value) return;
    // Deep compare
    if (JSON.stringify(newVal) !== JSON.stringify(props.deviceConfig.online_rule)) {
        const newConfig = { ...props.deviceConfig, online_rule: newVal };
        debouncedEmitUpdate(newConfig);
    }
}, { deep: true });



const addPollingGroup = () => {
    localPollingGroups.value.push({
        name: `Group_${localPollingGroups.value.length + 1}`,
        enable: true,
        slave_id: 1,
        function_code: 3,
        start_address: 0,
        length: 10,
        interval: 1000,
        description: '',
        _isNew: true
    });
};

const removePollingGroup = (index) => {
    localPollingGroups.value.splice(index, 1);
};



const openEditModal = (prop) => {
  currentProp.value = prop;
  const existing = getMapping(prop.identifier);
  if (existing) {
    hasMapping.value = true;
    currentMapping.value = JSON.parse(JSON.stringify(existing));
    
    // Ensure V2 objects exist
    if (!currentMapping.value.extract_rule) currentMapping.value.extract_rule = {};
    
    // Remove name from currentMapping as it's stored separately or added on save
    delete currentMapping.value.name;
  } else {
    hasMapping.value = false;
    // Reset based on strategy
    if (strategy.value === 'GatewayPolling') {
        currentMapping.value = {
          polling_group: '',
          offset: 0,
          data_type: 'uint16',
          byte_order: 'ABCD',
          read_expr: '',
          extract_rule: {},
          slave_id: 1,
          function_code: 3,
          address: 0,
          length: 1,
          interval: 1000,
          write_expr: ''
        };
    } else if (isBacnet.value) {
        currentMapping.value = {
            object_type: 0,
            instance_id: 1,
            property_id: 85,
            read_expr: '',
            enable_write: false,
            write_priority: 16,
            write_expr: ''
        };
    } else {
        currentMapping.value = {
            address: '',
            data_type: '',
            read_expr: '',
            write_expr: '',
            extract_rule: {}
        };
    }
  }
  
  // Enforce Read-Only
  if (prop.accessMode === 'r') {
      currentMapping.value.enable_write = false;
  }

  showModal.value = true;
};

const availablePoints = computed(() => {
    if (props.deviceConfig && props.deviceConfig.points) {
        return props.deviceConfig.points.map(p => {
            const tslProp = props.tslProperties?.find(tp => tp.identifier === p.name);
            const name = tslProp ? tslProp.name : p.name;
            return {
                label: `${name} (${p.name})`,
                value: p.name
            };
        });
    }
    return [];
});

const selectedEventIdentifier = ref('');

const getEventName = (id) => {
    const evt = props.tslEvents?.find(e => e.identifier === id);
    return evt ? evt.name : id;
};

const hasRule = (identifier) => {
    return localEvents.value.some(r => r.identifier === identifier);
};

const activeEventRule = computed(() => {
    if (!selectedEventIdentifier.value) return null;
    return localEvents.value.find(r => r.identifier === selectedEventIdentifier.value);
});

const selectEvent = (evt) => {
    selectedEventIdentifier.value = evt.identifier;
};

const toggleRule = (identifier, enabled) => {
    if (enabled) {
        if (!hasRule(identifier)) {
            localEvents.value.push({
                identifier: identifier,
                triggers: [], 
                conditions: [],
                report_interval: -1,
                params: {}
            });
        }
    } else {
        const idx = localEvents.value.findIndex(r => r.identifier === identifier);
        if (idx !== -1) {
            localEvents.value.splice(idx, 1);
        }
    }
};

const addTrigger = () => {
    if (activeEventRule.value) {
        if (!activeEventRule.value.triggers) activeEventRule.value.triggers = [];
        activeEventRule.value.triggers.push({
            point: '',
            operator: '>',
            value: 0
        });
    }
};

const removeTrigger = (idx) => {
    if (activeEventRule.value && activeEventRule.value.triggers) {
        activeEventRule.value.triggers.splice(idx, 1);
    }
};

const addCondition = () => {
    if (activeEventRule.value) {
        if (!activeEventRule.value.conditions) activeEventRule.value.conditions = [];
        activeEventRule.value.conditions.push({
            point: '',
            operator: '>',
            value: 0
        });
    }
};

const removeCondition = (idx) => {
    if (activeEventRule.value && activeEventRule.value.conditions) {
        activeEventRule.value.conditions.splice(idx, 1);
    }
};

const getEventParams = (identifier) => {
    if (!props.tslEvents) return [];
    const evt = props.tslEvents.find(e => e.identifier === identifier);
    return evt ? (evt.outputData || []) : [];
};

const closeModal = () => {
  showModal.value = false;
  currentProp.value = null;
};

    const saveMapping = () => {
      const newConfig = JSON.parse(JSON.stringify(props.deviceConfig));
      if (!newConfig.points) newConfig.points = [];
    
      // Remove existing
      const idx = newConfig.points.findIndex(p => p.name === currentProp.value.identifier);
      if (idx !== -1) {
        newConfig.points.splice(idx, 1);
      }
    
      if (hasMapping.value) {
        // Validation based on strategy
        if (strategy.value === 'GatewayPolling') {
            // Fix: Use availablePollingGroups instead of props.pollingGroups
            if (availablePollingGroups.value && availablePollingGroups.value.length > 0 && !currentMapping.value.polling_group) {
                alert(t('tsl_poll_group_required'));
                return;
            }
        } else if (isBacnet.value) {
            if (currentMapping.value.object_type === undefined || currentMapping.value.instance_id === undefined) {
                 alert('Object Type and Instance ID are required');
                 return;
            }
        } else {
            if (!currentMapping.value.address) {
                alert(t('tsl_address_required'));
                return;
            }
        }

        // Clean up empty extract_rule
        if (currentMapping.value.extract_rule) {
             const rule = currentMapping.value.extract_rule;
             // Only delete if both are truly empty (undefined or null or '')
             // Note: bit_index: 0 is a valid value for bit extraction
             const hasBitIndex = rule.bit_index !== undefined && rule.bit_index !== null && rule.bit_index !== '';
             const hasLength = rule.length !== undefined && rule.length !== null && rule.length !== '';
             
             if (!hasBitIndex && !hasLength) {
                 delete currentMapping.value.extract_rule;
             }
        }

        newConfig.points.push({
          name: currentProp.value.identifier,
          ...currentMapping.value
        });
      }
    
      emit('update:deviceConfig', newConfig);
      closeModal();
    };
</script>
