<template>
  <section class="scope-permission-policy">
    <div class="policy-heading">
      <div>
        <h6 class="mb-1">{{ title }}</h6>
        <p class="text-muted small mb-0">{{ $t('scope_permission_policy_hint') }}</p>
      </div>
      <span class="badge text-bg-light border">{{ $t(`scope_permission_mode_${mode}`) }}</span>
    </div>

    <div class="row g-2 mb-3">
      <div v-if="scope === 'project'" class="col-12 col-md-4">
        <label class="policy-option" :class="{ active: mode === 'inherit' }">
          <input v-model="mode" class="visually-hidden" type="radio" value="inherit" :disabled="isReadOnly">
          <i class="bi bi-diagram-3"></i>
          <span>{{ $t('scope_permission_mode_inherit') }}</span>
          <small>{{ $t('scope_permission_mode_inherit_hint') }}</small>
        </label>
      </div>
      <div class="col-12" :class="scope === 'project' ? 'col-md-4' : 'col-md-6'">
        <label class="policy-option" :class="{ active: mode === 'all' }">
          <input v-model="mode" class="visually-hidden" type="radio" value="all" :disabled="isReadOnly">
          <i class="bi bi-stars"></i>
          <span>{{ $t('scope_permission_mode_all') }}</span>
          <small>{{ $t('scope_permission_mode_all_hint') }}</small>
        </label>
      </div>
      <div class="col-12" :class="scope === 'project' ? 'col-md-4' : 'col-md-6'">
        <label class="policy-option" :class="{ active: mode === 'custom' }">
          <input v-model="mode" class="visually-hidden" type="radio" value="custom" :disabled="isReadOnly">
          <i class="bi bi-sliders"></i>
          <span>{{ $t('scope_permission_mode_custom') }}</span>
          <small>{{ $t('scope_permission_mode_custom_hint') }}</small>
        </label>
      </div>
    </div>

    <div v-if="mode === 'inherit'" class="policy-note policy-note-inherit">
      <i class="bi bi-info-circle"></i>
      <span>{{ $t('scope_permission_inherit_note') }}</span>
    </div>
    <div v-else-if="mode === 'all'" class="policy-note policy-note-all">
      <i class="bi bi-check2-circle"></i>
      <span>{{ $t('scope_permission_all_note') }}</span>
    </div>
    <PermissionDualMode
      v-else
      :all-permissions="allPermissions"
      :is-read-only="isReadOnly"
      :title="$t('scope_permission_custom_title')"
      v-model="permissionIDs"
    />
  </section>
</template>

<script setup>
import { computed, ref, watch } from 'vue'
import PermissionDualMode from './PermissionDualMode.vue'

const props = defineProps({
  scope: { type: String, required: true, validator: (value) => ['tenant', 'project'].includes(value) },
  title: { type: String, required: true },
  allPermissions: { type: Array, required: true },
  mode: { type: String, required: true },
  permissionIds: { type: Array, required: true },
  isReadOnly: { type: Boolean, default: false }
})

const emit = defineEmits(['update:mode', 'update:permissionIds'])
const lastCustomPermissionIDs = ref([...props.permissionIds])

const mode = computed({
  get: () => props.mode,
  set: (nextMode) => {
    if (nextMode === props.mode) return
    if (props.mode === 'custom') {
      lastCustomPermissionIDs.value = [...props.permissionIds]
    }
    emit('update:mode', nextMode)
    emit('update:permissionIds', nextMode === 'custom' ? [...lastCustomPermissionIDs.value] : [])
  }
})

const permissionIDs = computed({
  get: () => props.permissionIds,
  set: (nextIDs) => emit('update:permissionIds', nextIDs)
})

watch(() => props.permissionIds, (nextIDs) => {
  if (props.mode === 'custom') {
    lastCustomPermissionIDs.value = [...nextIDs]
  }
}, { deep: true })
</script>

<style scoped>
.scope-permission-policy {
  display: grid;
  gap: 1rem;
}

.policy-heading {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
}

.policy-option {
  display: flex;
  min-height: 104px;
  flex-direction: column;
  gap: 0.35rem;
  padding: 0.9rem;
  border: 1px solid var(--bs-border-color);
  border-radius: 0.65rem;
  cursor: pointer;
  transition: border-color 0.15s ease, box-shadow 0.15s ease, background-color 0.15s ease;
}

.policy-option:hover,
.policy-option.active {
  border-color: var(--bs-primary);
  background: var(--bs-primary-bg-subtle);
}

.policy-option.active {
  box-shadow: inset 0 0 0 1px var(--bs-primary);
}

.policy-option > i {
  color: var(--bs-primary);
  font-size: 1.2rem;
}

.policy-option > span {
  font-weight: 600;
}

.policy-option > small {
  color: var(--bs-secondary-color);
  line-height: 1.35;
}

.policy-note {
  display: flex;
  align-items: flex-start;
  gap: 0.55rem;
  padding: 0.8rem 0.9rem;
  border-radius: 0.5rem;
  font-size: 0.875rem;
}

.policy-note-all {
  color: var(--bs-success-text-emphasis);
  background: var(--bs-success-bg-subtle);
}

.policy-note-inherit {
  color: var(--bs-info-text-emphasis);
  background: var(--bs-info-bg-subtle);
}
</style>
