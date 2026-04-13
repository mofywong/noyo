<template>
  <div>
    <div class="d-flex justify-content-between align-items-center mb-3">
      <h6 class="mb-0 text-muted">{{ $t('tsl_events') }}</h6>
      <button class="btn btn-sm btn-primary" @click="openModal()">
        <i class="bi bi-plus-lg"></i> {{ $t('tsl_add') }}
      </button>
    </div>

    <table class="table table-hover table-sm align-middle">
      <thead class="table-light">
        <tr>
          <th>{{ $t('tsl_name') }}</th>
          <th>{{ $t('tsl_identifier') }}</th>
          <th>{{ $t('tsl_type') }}</th>
          <th class="text-end">{{ $t('tsl_actions') }}</th>
        </tr>
      </thead>
      <tbody>
        <tr v-if="!events || events.length === 0">
          <td colspan="4" class="text-center text-muted py-3">{{ $t('tsl_no_data') }}</td>
        </tr>
        <tr v-for="(evt, index) in events" :key="evt.identifier">
          <td>{{ evt.name }}</td>
          <td class="font-monospace">{{ evt.identifier }}</td>
          <td>{{ evt.type || 'info' }}</td>
          <td class="text-end">
            <button class="btn btn-sm btn-link text-decoration-none" @click="openModal(evt, index)">{{ $t('tsl_edit') }}</button>
            <button class="btn btn-sm btn-link text-danger text-decoration-none" @click="removeEvent(index)">{{ $t('tsl_delete') }}</button>
          </td>
        </tr>
      </tbody>
    </table>

    <!-- Modal -->
    <div v-if="showModal" class="modal fade show d-block" style="background: rgba(0,0,0,0.5)">
      <div class="modal-dialog modal-lg">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title">{{ editingIndex === -1 ? $t('tsl_add') : $t('tsl_edit') }} {{ $t('tsl_events') }}</h5>
            <button type="button" class="btn-close" @click="closeModal"></button>
          </div>
          <div class="modal-body">
            <div class="mb-3">
              <label class="form-label">{{ $t('tsl_name') }}</label>
              <input v-model="currentEvt.name" type="text" class="form-control">
            </div>
            <div class="mb-3">
              <label class="form-label">{{ $t('tsl_identifier') }}</label>
              <input v-model="currentEvt.identifier" type="text" class="form-control" :disabled="editingIndex !== -1">
            </div>
             <div class="mb-3">
              <label class="form-label">{{ $t('tsl_type') }}</label>
              <select v-model="currentEvt.type" class="form-select">
                <option value="info">{{ $t('tsl_evt_type_info') }}</option>
                <option value="alert">{{ $t('tsl_evt_type_alert') }}</option>
                <option value="error">{{ $t('tsl_evt_type_error') }}</option>
              </select>
            </div>
            
            <div class="mb-3">
              <TSLParamEditor 
                :title="$t('tsl_svc_output')" 
                v-model="currentEvt.outputData" 
              />
            </div>
          </div>
          <div class="modal-footer">
            <button type="button" class="btn btn-secondary" @click="closeModal">{{ $t('tsl_cancel') }}</button>
            <button type="button" class="btn btn-primary" @click="saveEvent">{{ $t('tsl_confirm') }}</button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed } from 'vue';
import { useI18n } from 'vue-i18n';
import TSLParamEditor from './TSLParamEditor.vue';

const { t } = useI18n();

const props = defineProps({
  modelValue: {
    type: Array,
    default: () => []
  }
});

const emit = defineEmits(['update:modelValue']);

const events = computed({
  get: () => props.modelValue || [],
  set: (val) => emit('update:modelValue', val)
});

const showModal = ref(false);
const editingIndex = ref(-1);
const currentEvt = ref({ name: '', identifier: '', type: 'info', outputData: [] });

const openModal = (evt, index = -1) => {
  editingIndex.value = index;
  if (evt) {
    currentEvt.value = JSON.parse(JSON.stringify(evt));
    if (!currentEvt.value.outputData) currentEvt.value.outputData = [];
  } else {
    currentEvt.value = { name: '', identifier: '', type: 'info', outputData: [] };
  }
  showModal.value = true;
};

const closeModal = () => {
  showModal.value = false;
};

const saveEvent = () => {
  const newEvents = [...events.value];
  if (editingIndex.value === -1) {
    newEvents.push(JSON.parse(JSON.stringify(currentEvt.value)));
  } else {
    newEvents[editingIndex.value] = JSON.parse(JSON.stringify(currentEvt.value));
  }
  emit('update:modelValue', newEvents);
  closeModal();
};

const removeEvent = (index) => {
  if(!confirm(t('common_delete_confirm'))) return;
  const newEvents = [...events.value];
  newEvents.splice(index, 1);
  emit('update:modelValue', newEvents);
};
</script>
