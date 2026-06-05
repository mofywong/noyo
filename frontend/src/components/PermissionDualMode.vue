<template>
  <div class="permission-dual-mode">
    <!-- 模式切换 -->
    <div class="d-flex justify-content-between align-items-center mb-4 bg-white p-3 rounded shadow-sm">
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
      <div class="d-flex justify-content-end mb-3 gap-2">
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
              <span class="text-truncate">{{ $t(mod.nameKey) }}</span>
            </div>
            <div class="flex-grow-1 text-end">
              <div class="btn-group btn-group-sm mt-auto ms-auto" role="group">
                <input v-if="quickLevels[mod.module] === 'custom'" type="radio" class="btn-check" :id="'cust_'+mod.module+componentId" value="custom" v-model="quickLevels[mod.module]" disabled>
                <label v-if="quickLevels[mod.module] === 'custom'" class="btn btn-outline-warning text-dark" :for="'cust_'+mod.module+componentId" :title="$t('perm_level_custom', '自定义')">
                  <i class="bi bi-gear-fill me-1"></i>{{ $t('perm_level_custom', '自定义') }}
                </label>

                <input v-if="mod.levels.includes('none')" type="radio" class="btn-check" :name="'qm_'+mod.module+componentId" :id="'none_'+mod.module+componentId" value="none" v-model="quickLevels[mod.module]" @change="onQuickLevelChange(mod, 'none')" :disabled="isReadOnly">
                <label v-if="mod.levels.includes('none')" class="btn btn-outline-secondary" :for="'none_'+mod.module+componentId">{{ $t('perm_level_none', '无权限') }}</label>

                <input v-if="mod.levels.includes('readonly')" type="radio" class="btn-check" :name="'qm_'+mod.module+componentId" :id="'read_'+mod.module+componentId" value="readonly" v-model="quickLevels[mod.module]" @change="onQuickLevelChange(mod, 'readonly')" :disabled="isReadOnly">
                <label v-if="mod.levels.includes('readonly')" class="btn btn-outline-info" :for="'read_'+mod.module+componentId">{{ $t('perm_level_readonly', '只读') }}</label>

                <input v-if="mod.levels.includes('edit')" type="radio" class="btn-check" :name="'qm_'+mod.module+componentId" :id="'edit_'+mod.module+componentId" value="edit" v-model="quickLevels[mod.module]" @change="onQuickLevelChange(mod, 'edit')" :disabled="isReadOnly">
                <label v-if="mod.levels.includes('edit')" class="btn btn-outline-primary" :for="'edit_'+mod.module+componentId">{{ $t('perm_level_edit', '编辑') }}</label>

                <input v-if="mod.levels.includes('full')" type="radio" class="btn-check" :name="'qm_'+mod.module+componentId" :id="'full_'+mod.module+componentId" value="full" v-model="quickLevels[mod.module]" @change="onQuickLevelChange(mod, 'full')" :disabled="isReadOnly">
                <label v-if="mod.levels.includes('full')" class="btn btn-outline-danger" :for="'full_'+mod.module+componentId">{{ $t('perm_level_full', '完全控制') }}</label>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- ================= 高级模式 ================= -->
    <div v-else class="advanced-mode-container">
      <div class="d-flex justify-content-between mb-3">
        <div class="input-group input-group-sm w-50">
          <span class="input-group-text bg-white"><i class="bi bi-search"></i></span>
          <input type="text" class="form-control" v-model="searchQuery" :placeholder="$t('search_text')">
        </div>
        <div class="d-flex gap-2">
          <button type="button" class="btn btn-sm btn-outline-secondary" @click="expandAll(true)">{{ $t('perm_expand_all', '展开全部') }}</button>
          <button type="button" class="btn btn-sm btn-outline-secondary" @click="expandAll(false)">{{ $t('perm_collapse_all', '折叠全部') }}</button>
          <button type="button" class="btn btn-sm btn-outline-primary ms-2" @click="selectAllPerms" :disabled="isReadOnly">{{ $t('select_all', '全选') }}</button>
          <button type="button" class="btn btn-sm btn-outline-secondary" @click="invertAllPerms" :disabled="isReadOnly">{{ $t('invert_selection', '反选') }}</button>
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
                      <span>{{ p.name }} <span class="text-muted small ms-1">({{ p.code }})</span></span>
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
import { ref, computed, watch, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { getAvailableQuickModeModules, inferQuickModeLevel, getCodesForLevel } from '../utils/permissionQuickMode'

const props = defineProps({
  title: { type: String, default: '' },
  allPermissions: { type: Array, required: true },
  modelValue: { type: Array, required: true }, // Array of selected permission IDs
  isReadOnly: { type: Boolean, default: false },
  hiddenModules: { type: Array, default: () => [] }
})

const emit = defineEmits(['update:modelValue'])
const { t } = useI18n()

// Generate a random ID for this component instance to ensure unique HTML IDs
const componentId = Math.random().toString(36).substring(2, 9)

const currentMode = ref('quick')
const searchQuery = ref('')
const collapsed = ref({})
const selectedPermCodes = ref([])
const quickLevels = ref({})

const activeModules = computed(() => {
  return getAvailableQuickModeModules(props.allPermissions, props.hiddenModules || [])
})

const MODULE_ORDER = [
  'dashboard', 'plugin', 'product', 'gateway', 'device', 'topology', 'device_tag', 'alarm', 'audit', 'system_logs', 'user', 'tenant', 'project', 'role', 'app', 'system'
]

const normalizedPermissions = computed(() => {
  return props.allPermissions.map(p => {
    // Keep topology mapping if backend doesn't output it correctly, though we fixed dashboard
    if (p.code === 'dashboard:view') return { ...p, module: 'dashboard' }
    if (p.code === 'device:topology') return { ...p, module: 'topology' }
    return p
  })
})

// Watch modelValue (prop from parent) -> selectedPermCodes
watch(() => props.modelValue, (newVal) => {
  const codes = []
  normalizedPermissions.value.forEach(p => {
    if (newVal.includes(p.ID)) {
      codes.push(p.code)
    }
  })
  if (JSON.stringify(codes) !== JSON.stringify(selectedPermCodes.value)) {
    selectedPermCodes.value = codes
  }
}, { immediate: true, deep: true })

// Watch allPermissions to initialize collapse state
watch(normalizedPermissions, (newVal) => {
  newVal.forEach(p => {
    if (collapsed.value[p.module] === undefined) {
      collapsed.value[p.module] = false // 默认全展开
    }
  })
}, { immediate: true })

// Sync selectedPermCodes -> quickLevels AND emit back to parent
watch(selectedPermCodes, (newCodes) => {
  activeModules.value.forEach(modConfig => {
    quickLevels.value[modConfig.module] = inferQuickModeLevel(modConfig, newCodes, quickLevels.value[modConfig.module])
  })
  
  // emit back to parent
  const newIds = normalizedPermissions.value
    .filter(p => newCodes.includes(p.code))
    .map(p => p.ID)
  
  // To avoid infinite loops, only emit if different
  if (JSON.stringify(newIds) !== JSON.stringify(props.modelValue)) {
    emit('update:modelValue', newIds)
  }
}, { deep: true })


// Actions
const onQuickLevelChange = (modConfig, level) => {
  quickLevels.value[modConfig.module] = level
  const codesToSet = getCodesForLevel(modConfig, level)
  const allModCodes = [...modConfig.readonly, ...modConfig.edit, ...modConfig.full]
  
  let newSelected = selectedPermCodes.value.filter(c => !allModCodes.includes(c))
  codesToSet.forEach(c => {
    if (!newSelected.includes(c)) newSelected.push(c)
  })
  
  selectedPermCodes.value = newSelected
}

const setAllQuickMode = (level) => {
  activeModules.value.forEach(mod => {
    if (mod.levels.includes(level)) {
      onQuickLevelChange(mod, level)
    } else if (level === 'full' && mod.levels.includes('edit')) {
      onQuickLevelChange(mod, 'edit')
    } else if (level === 'edit' && mod.levels.includes('readonly')) {
      onQuickLevelChange(mod, 'readonly')
    } else if (level === 'none') {
      onQuickLevelChange(mod, 'none')
    }
  })
}



const groupedPermissions = computed(() => {
  const groups = {}
  normalizedPermissions.value.forEach(p => {
    if (!groups[p.module]) {
      groups[p.module] = []
    }
    groups[p.module].push(p)
  })

  const orderedGroups = {}
  const displayOrder = [
    'dashboard', 'plugin', 'product', 'gateway', 'device', 'topology', 'device_tag', 'alarm', 'audit', 'system_logs', 'user', 'tenant', 'project', 'role', 'app', 'system'
  ]
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
    if (props.hiddenModules && props.hiddenModules.includes(mod)) continue
    const modTrans = translateModule(mod).toLowerCase()
    if (modTrans.includes(q)) {
      filtered[mod] = perms
      continue
    }
    const filteredPerms = perms.filter(p => p.name.toLowerCase().includes(q) || p.code.toLowerCase().includes(q))
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
  selectedPermCodes.value = normalizedPermissions.value.map(p => p.code)
}

const invertAllPerms = () => {
  const allCodes = normalizedPermissions.value.map(p => p.code)
  selectedPermCodes.value = allCodes.filter(c => !selectedPermCodes.value.includes(c))
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
  const m = module ? module.toLowerCase() : ''
  const keyMap = {
    'tenant': 'perm_mod_tenant',
    'project': 'perm_mod_project',
    'system': 'perm_mod_system',
    'audit': 'perm_mod_audit',
    'system_logs': 'system_logs',
    'device': 'perm_mod_device',
    'topology': 'perm_mod_topology',
    'device_tag': 'perm_mod_device_tag',
    'gateway': 'perm_mod_gateway',
    'history': 'perm_mod_history',
    'rule': 'perm_mod_rule',
    'alarm': 'perm_mod_alarm',
    'auth': 'perm_mod_auth',
    'product': 'perm_mod_product',
    'user': 'perm_mod_user',
    'role': 'perm_mod_role',
    'plugin': 'perm_mod_plugin',
    'audit': 'perm_mod_audit',
    'app': 'perm_mod_app',
    'dashboard': 'perm_mod_dashboard'
  }
  
  const defaultZh = {
    'tenant': '租户管理',
    'system': '系统配置',
    'rule': '规则引擎',
    'auth': '认证授权'
  }
  
  const key = keyMap[m]
  if (key && t(key) !== key) {
    return t(key)
  }
  return defaultZh[m] || module
}

const translateType = (type) => {
  const t_key = type ? type.toLowerCase() : ''
  const keyMap = {
    'menu': '菜单',
    'button': '按钮',
    'api': '接口',
    'data': '数据'
  }
  return t('perm_type_' + t_key, keyMap[t_key] || type)
}
</script>

<style scoped>
.quick-mode-container .btn-group {
  box-shadow: 0 1px 3px rgba(0,0,0,0.05);
}

.quick-mode-container .btn-group .btn {
  min-width: 80px;
}

.advanced-mode-container .collapse.show {
  border-top: 1px solid rgba(0,0,0,0.05);
}

.advanced-mode-container .list-group-item {
  border-left: none;
  border-right: none;
  border-bottom: 1px solid rgba(0,0,0,0.03);
}

.advanced-mode-container .list-group-item:last-child {
  border-bottom: none;
}
</style>
