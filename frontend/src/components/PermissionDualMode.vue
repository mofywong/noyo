<template>
  <div class="permission-dual-mode">
    <!-- 模式切换 -->
    <div class="d-flex flex-wrap gap-3 justify-content-between align-items-center mb-4 bg-white p-3 rounded shadow-sm">
      <div class="fw-bold fs-5 text-primary">
        <i class="bi bi-shield-check me-2"></i>{{ title || $t('role_menu_perm', '菜单权限') }}
      </div>
      <div class="btn-group" role="group">
        <input type="radio" class="btn-check" :name="'mode'+componentId" :id="'modeQuick'+componentId" value="quick" v-model="currentMode" :disabled="isReadOnly">
        <label class="btn btn-outline-primary px-4" :for="'modeQuick'+componentId">
          <i class="bi bi-lightning-charge me-1"></i>{{ $t('perm_quick_mode', '快速模式') }}
        </label>

        <input type="radio" class="btn-check" :name="'mode'+componentId" :id="'modeAdv'+componentId" value="advanced" v-model="currentMode" :disabled="isReadOnly">
        <label class="btn btn-outline-primary px-4" :for="'modeAdv'+componentId">
          <i class="bi bi-sliders me-1"></i>{{ $t('perm_advanced_mode', '高级模式') }}
        </label>
      </div>
    </div>

    <!-- ================= 快速模式 ================= -->
    <div v-if="currentMode === 'quick'" class="quick-mode-container">
      <div class="d-flex flex-wrap justify-content-end mb-3 gap-2">
        <button type="button" class="btn btn-sm btn-outline-secondary" @click="setAllQuickMode('none')" :disabled="isReadOnly">{{ $t('perm_batch_clear', '清空所有') }}</button>
        <button type="button" class="btn btn-sm btn-outline-secondary" @click="setAllQuickMode('readonly')" :disabled="isReadOnly">{{ $t('perm_batch_readonly', '全部只读') }}</button>
        <button type="button" class="btn btn-sm btn-outline-primary" @click="setAllQuickMode('edit')" :disabled="isReadOnly">{{ $t('perm_batch_edit', '全部编辑') }}</button>
        <button type="button" class="btn btn-sm btn-outline-danger" @click="setAllQuickMode('full')" :disabled="isReadOnly">{{ $t('perm_batch_full', '全部完全控制') }}</button>
      </div>

      <div class="card shadow-sm border-0 mb-4 mt-3">
        <div class="list-group list-group-flush">
          <div class="list-group-item d-flex flex-column flex-sm-row align-items-sm-center py-3" v-for="mod in activeModules" :key="mod.module">
            <div class="d-flex align-items-center fw-medium mb-2 mb-sm-0" style="flex: 0 0 180px;">
              <div class="icon-box bg-primary bg-opacity-10 text-primary rounded d-flex justify-content-center align-items-center me-3 flex-shrink-0" style="width: 36px; height: 36px;">
                <i :class="['bi', mod.icon, 'fs-5']"></i>
              </div>
              <span class="text-truncate">{{ $t(mod.nameKey, translateModule(mod.module)) }}</span>
            </div>
            <div class="flex-grow-1 text-end">
              <div class="btn-group btn-group-sm mt-auto ms-auto" role="group">
                <input v-if="quickLevels[mod.module] === 'custom'" type="radio" class="btn-check" :id="'cust_'+mod.module+componentId" value="custom" checked disabled>
                <label v-if="quickLevels[mod.module] === 'custom'" class="btn btn-outline-warning text-dark" :for="'cust_'+mod.module+componentId" :title="$t('perm_level_custom', '自定义')">
                  <i class="bi bi-gear-fill me-1"></i>{{ $t('perm_level_custom', '自定义') }}
                </label>

                <input v-if="mod.levels.includes('none')" type="radio" class="btn-check" :name="'qm_'+mod.module+componentId" :id="'none_'+mod.module+componentId" value="none" :checked="quickLevels[mod.module] === 'none'" @change="onQuickLevelChange(mod, 'none')" :disabled="isReadOnly">
                <label v-if="mod.levels.includes('none')" class="btn btn-outline-secondary" :for="'none_'+mod.module+componentId">{{ $t('perm_level_none', '无权限') }}</label>

                <input v-if="mod.levels.includes('readonly')" type="radio" class="btn-check" :name="'qm_'+mod.module+componentId" :id="'read_'+mod.module+componentId" value="readonly" :checked="quickLevels[mod.module] === 'readonly'" @change="onQuickLevelChange(mod, 'readonly')" :disabled="isReadOnly">
                <label v-if="mod.levels.includes('readonly')" class="btn btn-outline-info" :for="'read_'+mod.module+componentId">{{ $t('perm_level_readonly', '只读') }}</label>

                <input v-if="mod.levels.includes('edit')" type="radio" class="btn-check" :name="'qm_'+mod.module+componentId" :id="'edit_'+mod.module+componentId" value="edit" :checked="quickLevels[mod.module] === 'edit'" @change="onQuickLevelChange(mod, 'edit')" :disabled="isReadOnly">
                <label v-if="mod.levels.includes('edit')" class="btn btn-outline-primary" :for="'edit_'+mod.module+componentId">{{ $t('perm_level_edit', '编辑') }}</label>

                <input v-if="mod.levels.includes('full')" type="radio" class="btn-check" :name="'qm_'+mod.module+componentId" :id="'full_'+mod.module+componentId" value="full" :checked="quickLevels[mod.module] === 'full'" @change="onQuickLevelChange(mod, 'full')" :disabled="isReadOnly">
                <label v-if="mod.levels.includes('full')" class="btn btn-outline-danger" :for="'full_'+mod.module+componentId">{{ $t('perm_level_full', '完全控制') }}</label>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- ================= 高级模式 ================= -->
    <div v-else class="advanced-mode-container">
      <div class="d-flex flex-wrap gap-3 justify-content-between mb-3">
        <div class="input-group input-group-sm w-50">
          <span class="input-group-text bg-white"><i class="bi bi-search"></i></span>
          <input type="text" class="form-control" v-model="searchQuery" :placeholder="$t('search_text')">
        </div>
        <div class="d-flex flex-wrap gap-2">
          <button type="button" class="btn btn-sm btn-outline-secondary" @click="expandAll(true)">{{ $t('perm_expand_all', '展开全部') }}</button>
          <button type="button" class="btn btn-sm btn-outline-secondary" @click="expandAll(false)">{{ $t('perm_collapse_all', '折叠全部') }}</button>
          <button type="button" class="btn btn-sm btn-outline-primary ms-2" @click="selectAllPerms" :disabled="isReadOnly">{{ $t('perm_select_visible') }}</button>
          <button type="button" class="btn btn-sm btn-outline-secondary" @click="invertAllPerms" :disabled="isReadOnly">{{ $t('perm_invert_visible') }}</button>
        </div>
      </div>
      
      <div class="row row-cols-1 row-cols-md-2 g-3">
        <div class="col" v-for="(group, module) in filteredGroupedPermissions" :key="module">
          <div class="card h-100 shadow-sm border-0">
            <div class="card-header bg-white d-flex justify-content-between align-items-center" style="cursor: pointer;" @click="toggleCollapse(module)">
              <div class="fw-bold d-flex align-items-center">
                <i class="bi me-2 text-muted" :class="collapsed[module] ? 'bi-chevron-right' : 'bi-chevron-down'"></i>
                {{ translateModule(module) }}
                <span class="badge bg-primary bg-opacity-10 text-primary ms-2 rounded-pill px-2">
                  {{ getSelectedCountForModule(group) }} / {{ group.length }}
                </span>
              </div>
              <div @click.stop>
                <button type="button" class="btn btn-sm btn-link text-primary p-0 text-decoration-none me-2" @click="selectAllModule(group)" :disabled="isReadOnly">{{ $t('select_all', '全选') }}</button>
                <button type="button" class="btn btn-sm btn-link text-secondary p-0 text-decoration-none" @click="invertModule(group)" :disabled="isReadOnly">{{ $t('invert_selection', '反选') }}</button>
              </div>
            </div>
            <div class="collapse" :class="{ show: !collapsed[module] }">
              <ul class="list-group list-group-flush">
                <li class="list-group-item" v-for="p in group" :key="p.ID">
                  <div class="form-check">
                    <input class="form-check-input" type="checkbox" :value="p.code" :id="'perm'+p.ID+componentId" v-model="selectedPermCodes" :disabled="isReadOnly">
                    <label class="form-check-label d-flex justify-content-between" :for="'perm'+p.ID+componentId">
                      <span>{{ permissionName(p) }} <span class="text-muted small ms-1">({{ p.code }})</span></span>
                      <span class="badge bg-secondary opacity-75 fw-normal" style="font-size: 0.7em">{{ translateType(p.type) }}</span>
                    </label>
                  </div>
                </li>
              </ul>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { getAvailableQuickModeModules, inferQuickModeLevel, getCodesForLevel, getPermissionModule } from '../utils/permissionQuickMode'

const props = defineProps({
  title: { type: String, default: '' },
  allPermissions: { type: Array, required: true },
  modelValue: { type: Array, required: true }, // Array of selected permission IDs
  isReadOnly: { type: Boolean, default: false },
  hiddenModules: { type: Array, default: () => [] }
})

const emit = defineEmits(['update:modelValue'])
const { t, locale } = useI18n()

// Generate a random ID for this component instance to ensure unique HTML IDs
const componentId = Math.random().toString(36).substring(2, 9)

const currentMode = ref('quick')
const searchQuery = ref('')
const collapsed = ref({})

const activeModules = computed(() => {
  return getAvailableQuickModeModules(props.allPermissions, props.hiddenModules || [])
})

const MODULE_ORDER = [
  'dashboard', 'screen', 'plugin', 'product', 'gateway', 'device', 'topology', 'device_map', 'geofence', 'device_tag', 'rule', 'alarm', 'work_order', 'ai_scene', 'inspection', 'semantic_review', 'ai_model', 'ai_assistant', 'ai_brain', 'history', 'audit', 'llm_request', 'system_logs', 'user', 'tenant', 'project', 'role', 'app', 'system'
]

const normalizedPermissions = computed(() => {
  return props.allPermissions.map(p => ({ ...p, module: getPermissionModule(p) }))
})

// Derive selection from both props so a late permission catalog cannot clear
// existing assignments. Preserve IDs outside the currently assignable catalog.
const selectedPermCodes = computed({
  get: () => normalizedPermissions.value.filter(p => props.modelValue.includes(p.ID)).map(p => p.code),
  set: codes => {
    if (props.isReadOnly) return
    const knownIds = new Set(normalizedPermissions.value.map(p => p.ID))
    const newIds = [
      ...props.modelValue.filter(id => !knownIds.has(id)),
      ...normalizedPermissions.value.filter(p => codes.includes(p.code)).map(p => p.ID)
    ]
    if (newIds.length !== props.modelValue.length || newIds.some(id => !props.modelValue.includes(id))) {
      emit('update:modelValue', newIds)
    }
  }
})

const quickLevels = computed(() => Object.fromEntries(activeModules.value.map(mod => [
  mod.module, inferQuickModeLevel(mod, selectedPermCodes.value)
])))

// Watch allPermissions to initialize collapse state
watch(normalizedPermissions, (newVal) => {
  newVal.forEach(p => {
    if (collapsed.value[p.module] === undefined) {
      collapsed.value[p.module] = false // 默认全展开
    }
  })
}, { immediate: true })

// Actions
const onQuickLevelChange = (modConfig, level) => {
  if (props.isReadOnly || !modConfig.levels.includes(level)) return
  const codesToSet = getCodesForLevel(modConfig, level)
  const allModCodes = [...modConfig.readonly, ...modConfig.edit, ...modConfig.full]
  
  let newSelected = selectedPermCodes.value.filter(c => !allModCodes.includes(c))
  codesToSet.forEach(c => {
    if (!newSelected.includes(c)) newSelected.push(c)
  })
  
  selectedPermCodes.value = newSelected
}

const setAllQuickMode = (level) => {
  if (props.isReadOnly) return
  const selected = new Set(selectedPermCodes.value)
  activeModules.value.forEach(mod => {
    ;[...mod.readonly, ...mod.edit, ...mod.full].forEach(code => selected.delete(code))
  })
  activeModules.value.forEach(mod => {
    const levels = ['none', 'readonly', 'edit', 'full']
    const availableLevel = levels.slice(0, levels.indexOf(level) + 1).reverse().find(candidate => mod.levels.includes(candidate))
    getCodesForLevel(mod, availableLevel || 'none').forEach(code => selected.add(code))
  })
  selectedPermCodes.value = [...selected]
}



const groupedPermissions = computed(() => {
  const groups = {}
  normalizedPermissions.value.forEach(p => {
    if (props.hiddenModules.includes(p.module)) return
    if (!groups[p.module]) {
      groups[p.module] = []
    }
    groups[p.module].push(p)
  })

  const orderedGroups = {}
  const displayOrder = MODULE_ORDER
  for (const mod of displayOrder) {
    if (groups[mod] && groups[mod].length > 0) {
      orderedGroups[mod] = groups[mod]
    }
  }
  for (const mod in groups) {
    if (!displayOrder.includes(mod) && groups[mod].length > 0) {
      orderedGroups[mod] = groups[mod]
    }
  }

  return orderedGroups
})

const filteredGroupedPermissions = computed(() => {
  if (!searchQuery.value) return groupedPermissions.value
  const q = searchQuery.value.toLowerCase()
  const filtered = {}
  for (const [mod, perms] of Object.entries(groupedPermissions.value)) {
    const modTrans = translateModule(mod).toLowerCase()
    if (modTrans.includes(q)) {
      filtered[mod] = perms
      continue
    }
    const filteredPerms = perms.filter(p => permissionName(p).toLowerCase().includes(q) || p.code.toLowerCase().includes(q))
    if (filteredPerms.length > 0) {
      filtered[mod] = filteredPerms
    }
  }
  return filtered
})

const toggleCollapse = (module) => {
  collapsed.value[module] = !collapsed.value[module]
}

const expandAll = (expand) => {
  for (const key in collapsed.value) {
    collapsed.value[key] = !expand
  }
}

const getSelectedCountForModule = (perms) => {
  return perms.filter(p => selectedPermCodes.value.includes(p.code)).length
}

const selectAllPerms = () => {
  selectAllModule(Object.values(filteredGroupedPermissions.value).flat())
}

const invertAllPerms = () => {
  invertModule(Object.values(filteredGroupedPermissions.value).flat())
}

const selectAllModule = (perms) => {
  const newSelected = [...selectedPermCodes.value]
  perms.forEach(p => {
    if (!newSelected.includes(p.code)) {
      newSelected.push(p.code)
    }
  })
  selectedPermCodes.value = newSelected
}

const invertModule = (perms) => {
  const newSelected = [...selectedPermCodes.value]
  perms.forEach(p => {
    const idx = newSelected.indexOf(p.code)
    if (idx > -1) {
      newSelected.splice(idx, 1)
    } else {
      newSelected.push(p.code)
    }
  })
  selectedPermCodes.value = newSelected
}

const translateModule = (module) => {
  const key = ({ llm_request: 'llm_requests', system_logs: 'system_logs' })[module] || `perm_mod_${module}`
  return t(key, t('perm_mod_other', { module }))
}

const translateType = (type) => {
  const t_key = type ? type.toLowerCase() : ''
  return t('perm_type_' + t_key, type)
}

const permissionName = (permission) => {
  const key = `perm_code_${permission.code.replaceAll(':', '_')}`
  const names = String(permission.name || permission.code).split(' / ')
  const fallback = locale.value.startsWith('zh') ? names[0] : names[1] || permission.code
  return t(key, fallback)
}
</script>

<style scoped>
.quick-mode-container .btn-group {
  box-shadow: var(--shadow-card);
}

.quick-mode-container .btn-group .btn {
  min-width: 80px;
}

.advanced-mode-container .collapse.show {
  border-top: 1px solid var(--border-color);
}

.advanced-mode-container .list-group-item {
  border-left: none;
  border-right: none;
  border-bottom: 1px solid var(--border-color);
}

.advanced-mode-container .list-group-item:last-child {
  border-bottom: none;
}
</style>
