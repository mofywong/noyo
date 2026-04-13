<template>
  <div>
    <div class="d-flex justify-content-between align-items-center mb-3">
      <h6 class="mb-0 text-muted">{{ $t('tsl_services') }}</h6>
      <button class="btn btn-sm btn-primary" @click="openModal()">
        <i class="bi bi-plus-lg"></i> {{ $t('tsl_add') }}
      </button>
    </div>

    <table class="table table-hover table-sm align-middle">
      <thead class="table-light">
        <tr>
          <th>{{ $t('tsl_name') }}</th>
          <th>{{ $t('tsl_identifier') }}</th>
          <th>{{ $t('tsl_svc_call_type') }}</th>
          <th class="text-end">{{ $t('tsl_actions') }}</th>
        </tr>
      </thead>
      <tbody>
        <tr v-if="!services || services.length === 0">
          <td colspan="4" class="text-center text-muted py-3">{{ $t('tsl_no_data') }}</td>
        </tr>
        <tr v-for="(svc, index) in services" :key="svc.identifier">
          <td>{{ svc.name }}</td>
          <td class="font-monospace">{{ svc.identifier }}</td>
          <td>{{ svc.callType === 'sync' ? $t('tsl_svc_sync') : $t('tsl_svc_async') }}</td>
          <td class="text-end">
            <button class="btn btn-sm btn-link text-decoration-none" @click="openModal(svc, index)">{{ $t('tsl_edit') }}</button>
            <button class="btn btn-sm btn-link text-danger text-decoration-none" @click="removeService(index)">{{ $t('tsl_delete') }}</button>
          </td>
        </tr>
      </tbody>
    </table>

    <!-- Modal -->
    <div v-if="showModal" class="modal fade show d-block" style="background: rgba(0,0,0,0.5)">
      <div class="modal-dialog modal-lg">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title">{{ editingIndex === -1 ? $t('tsl_add') : $t('tsl_edit') }} {{ $t('tsl_services') }}</h5>
            <button type="button" class="btn-close" @click="closeModal"></button>
          </div>
          <div class="modal-body">
            <div class="mb-3">
              <label class="form-label">{{ $t('tsl_name') }}</label>
              <input v-model="currentSvc.name" type="text" class="form-control">
            </div>
            <div class="mb-3">
              <label class="form-label">{{ $t('tsl_identifier') }}</label>
              <input v-model="currentSvc.identifier" type="text" class="form-control" :disabled="editingIndex !== -1">
            </div>
             <div class="mb-3">
              <label class="form-label">{{ $t('tsl_svc_call_type') }}</label>
              <select v-model="currentSvc.callType" class="form-select">
                <option value="async">{{ $t('tsl_svc_async') }}</option>
                <option value="sync">{{ $t('tsl_svc_sync') }}</option>
              </select>
            </div>
            
            <div class="mb-3">
              <TSLParamEditor 
                :title="$t('tsl_svc_input')" 
                v-model="currentSvc.inputData" 
              />
            </div>

            <div class="mb-3">
              <TSLParamEditor 
                :title="$t('tsl_svc_output')" 
                v-model="currentSvc.outputData" 
              />
            </div>
          </div>
          <div class="modal-footer">
            <button type="button" class="btn btn-secondary" @click="closeModal">{{ $t('tsl_cancel') }}</button>
            <button type="button" class="btn btn-primary" @click="saveService">{{ $t('tsl_confirm') }}</button>
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

const services = computed({
  get: () => props.modelValue || [],
  set: (val) => emit('update:modelValue', val)
});

const showModal = ref(false);
const editingIndex = ref(-1);
const currentSvc = ref({ name: '', identifier: '', callType: 'async', inputData: [], outputData: [] });

const openModal = (svc, index = -1) => {
  editingIndex.value = index;
  if (svc) {
    currentSvc.value = JSON.parse(JSON.stringify(svc));
    if (!currentSvc.value.inputData) currentSvc.value.inputData = [];
    if (!currentSvc.value.outputData) currentSvc.value.outputData = [];
  } else {
    currentSvc.value = { name: '', identifier: '', callType: 'async', inputData: [], outputData: [] };
  }
  showModal.value = true;
};

const closeModal = () => {
  showModal.value = false;
};

const saveService = () => {
  const newServices = [...services.value];
  if (editingIndex.value === -1) {
    newServices.push(JSON.parse(JSON.stringify(currentSvc.value)));
  } else {
    newServices[editingIndex.value] = JSON.parse(JSON.stringify(currentSvc.value));
  }
  emit('update:modelValue', newServices);
  closeModal();
};

const removeService = (index) => {
  if(!confirm(t('common_delete_confirm'))) return;
  const newServices = [...services.value];
  newServices.splice(index, 1);
  emit('update:modelValue', newServices);
};
</script>
