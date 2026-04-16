<template>
  <div>
    <div class="d-flex justify-content-between align-items-center mb-3">
      <h6 class="mb-0 text-muted">{{ $t('tsl_properties') }}</h6>
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
          <th>{{ $t('tsl_prop_access') }}</th>
          <th class="text-end">{{ $t('tsl_actions') }}</th>
        </tr>
      </thead>
      <tbody>
        <tr v-if="!properties || properties.length === 0">
          <td colspan="5" class="text-center text-muted py-3">{{ $t('tsl_no_data') }}</td>
        </tr>
        <tr v-for="(prop, index) in properties" :key="prop.identifier">
          <td>{{ prop.name }}</td>
          <td class="font-monospace">{{ prop.identifier }}</td>
          <td><span class="badge bg-secondary">{{ prop.dataType?.type }}</span></td>
          <td>{{ prop.accessMode === 'r' ? $t('tsl_prop_access_r') : $t('tsl_prop_access_rw') }}</td>
          <td class="text-end">
            <button class="btn btn-sm btn-link text-decoration-none" @click="openModal(prop, index)">{{ $t('tsl_edit') }}</button>
            <button class="btn btn-sm btn-link text-danger text-decoration-none" @click="removeProperty(index)">{{ $t('tsl_delete') }}</button>
          </td>
        </tr>
      </tbody>
    </table>

    <!-- Modal -->
    <div v-if="showModal" class="modal fade show d-block" style="background: rgba(0,0,0,0.5)">
      <div class="modal-dialog modal-lg">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title">{{ editingIndex === -1 ? $t('tsl_add') : $t('tsl_edit') }} {{ $t('tsl_properties') }}</h5>
            <button type="button" class="btn-close" @click="closeModal"></button>
          </div>
          <div class="modal-body">
            <div class="row">
              <!-- Basic Info -->
              <div class="col-md-12">
                <h6 class="mb-3 border-bottom pb-2">{{ $t('tsl_basic_info') }}</h6>
                <div class="row">
                  <div class="col-md-6 mb-3">
                    <label class="form-label">{{ $t('tsl_name') }}</label>
                    <input v-model="currentProp.name" type="text" class="form-control" :placeholder="$t('tsl_placeholder_name')">
                  </div>
                  <div class="col-md-6 mb-3">
                    <label class="form-label">{{ $t('tsl_identifier') }}</label>
                    <input v-model="currentProp.identifier" type="text" class="form-control" :placeholder="$t('tsl_placeholder_id')" :disabled="editingIndex !== -1">
                  </div>
                </div>

                <div class="row">
                  <div class="col-md-6 mb-3">
                    <label class="form-label">{{ $t('tsl_datatype') }}</label>
                    <select v-model="currentProp.dataType.type" class="form-select">
                      <option value="int">{{ $t('tsl_type_int') }}</option>
                      <option value="float">{{ $t('tsl_type_float') }}</option>
                      <option value="double">{{ $t('tsl_type_double') }}</option>
                      <option value="text">{{ $t('tsl_type_text') }}</option>
                      <option value="bool">{{ $t('tsl_type_bool') }}</option>
                      <option value="enum">{{ $t('tsl_type_enum') }}</option>
                      <option value="object">{{ $t('tsl_type_json') }}</option>
                      <option value="date">{{ $t('tsl_type_date') }}</option>
                    </select>
                  </div>
                  <div class="col-md-6 mb-3">
                    <label class="form-label">{{ $t('tsl_prop_access') }}</label>
                    <select v-model="currentProp.accessMode" class="form-select">
                      <option value="rw">{{ $t('tsl_prop_access_rw') }}</option>
                      <option value="r">{{ $t('tsl_prop_access_r') }}</option>
                    </select>
                  </div>
                </div>

                <!-- Numeric Specs -->
                <div class="row" v-if="['int', 'float', 'double'].includes(currentProp.dataType.type)">
                  <div class="col-md-3 mb-3">
                    <label class="form-label">{{ $t('tsl_prop_unit') }}</label>
                    <input v-model="currentProp.dataType.specs.unit" type="text" class="form-control">
                  </div>
                  <div class="col-md-3 mb-3">
                    <label class="form-label">{{ $t('tsl_prop_min') }}</label>
                    <input v-model.number="currentProp.dataType.specs.min" type="number" class="form-control">
                  </div>
                  <div class="col-md-3 mb-3">
                    <label class="form-label">{{ $t('tsl_prop_max') }}</label>
                    <input v-model.number="currentProp.dataType.specs.max" type="number" class="form-control">
                  </div>
                  <div class="col-md-3 mb-3">
                    <label class="form-label">{{ $t('tsl_prop_step') }}</label>
                    <input v-model.number="currentProp.dataType.specs.step" type="number" class="form-control">
                  </div>
                </div>
                
                <!-- Text Specs -->
                <div class="row" v-if="['text'].includes(currentProp.dataType.type)">
                  <div class="col-md-6 mb-3">
                    <label class="form-label">{{ $t('tsl_prop_max_len') }}</label>
                     <input v-model.number="currentProp.dataType.specs.length" type="number" class="form-control">
                  </div>
                </div>

                <!-- Bool Specs -->
                <div class="row" v-if="currentProp.dataType.type === 'bool'">
                   <div class="col-md-6 mb-3">
                      <label class="form-label">{{ $t('tsl_bool_false') }}</label>
                      <input v-model="currentProp.dataType.specs['0']" type="text" class="form-control" placeholder="e.g. OFF">
                   </div>
                   <div class="col-md-6 mb-3">
                      <label class="form-label">{{ $t('tsl_bool_true') }}</label>
                      <input v-model="currentProp.dataType.specs['1']" type="text" class="form-control" placeholder="e.g. ON">
                   </div>
                </div>

                <!-- Enum Specs -->
                <div v-if="currentProp.dataType.type === 'enum'">
                  <label class="form-label small">{{ $t('tsl_prop_specs') }}</label>
                  <table class="table table-sm table-bordered mb-2">
                    <thead>
                      <tr>
                        <th style="width: 120px">{{ $t('tsl_enum_key') }}</th>
                        <th>{{ $t('tsl_enum_value') }}</th>
                        <th style="width: 50px"></th>
                      </tr>
                    </thead>
                    <tbody>
                       <tr v-for="(item, idx) in enumList" :key="idx">
                          <td><input v-model="item.key" type="text" class="form-control form-control-sm"></td>
                          <td><input v-model="item.value" type="text" class="form-control form-control-sm"></td>
                          <td class="text-center align-middle">
                            <button class="btn btn-xs btn-link text-danger p-0" @click="removeEnumItem(idx)">
                              <i class="bi bi-x-lg"></i>
                            </button>
                          </td>
                       </tr>
                    </tbody>
                  </table>
                  <button class="btn btn-sm btn-outline-secondary" @click="addEnumItem">
                    <i class="bi bi-plus"></i> {{ $t('tsl_add_item') }}
                  </button>
                </div>

              </div>
            </div>
          </div>
          <div class="modal-footer">
            <button type="button" class="btn btn-secondary" @click="closeModal">{{ $t('tsl_cancel') }}</button>
            <button type="button" class="btn btn-primary" @click="saveProperty">{{ $t('tsl_confirm') }}</button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, watch } from 'vue';
import { useI18n } from 'vue-i18n';

const { t } = useI18n();

const props = defineProps({
  modelValue: {
    type: Array,
    default: () => []
  }
});

const emit = defineEmits(['update:modelValue']);

const properties = computed({
  get: () => props.modelValue || [],
  set: (val) => emit('update:modelValue', val)
});

const showModal = ref(false);
const editingIndex = ref(-1);
const currentProp = ref({
  name: '',
  identifier: '',
  accessMode: 'rw',
  dataType: { type: 'int', specs: {} }
});
const currentPropSpecsJson = ref('');
const enumList = ref([]);

const updateSpecsFromJson = () => {
  // Deprecated JSON area for Enums, but kept if we need to support other future types
  try {
    if (currentPropSpecsJson.value && !['enum', 'bool'].includes(currentProp.value.dataType.type)) {
       // No-op for now as we removed struct/array
    }
  } catch (e) {
    console.warn("Invalid JSON specs");
  }
};

// Watch for data type changes to initialize specs
watch(() => currentProp.value.dataType.type, (newType) => {
  if (!currentProp.value.dataType.specs) {
    currentProp.value.dataType.specs = {};
  }
  if (newType === 'enum') {
    parseEnumSpecs();
  }
});

const parseEnumSpecs = () => {
  enumList.value = [];
  const specs = currentProp.value.dataType.specs;
  if (specs) {
    Object.keys(specs).forEach(k => {
      enumList.value.push({ key: k, value: specs[k] });
    });
  }
};

const saveEnumSpecs = () => {
  if (currentProp.value.dataType.type === 'enum') {
    const newSpecs = {};
    enumList.value.forEach(item => {
      if (item.key) newSpecs[item.key] = item.value;
    });
    currentProp.value.dataType.specs = newSpecs;
  }
};

const addEnumItem = () => {
  enumList.value.push({ key: '', value: '' });
};

const removeEnumItem = (idx) => {
  enumList.value.splice(idx, 1);
};

const openModal = (prop, index = -1) => {
  editingIndex.value = index;
  if (prop) {
    currentProp.value = JSON.parse(JSON.stringify(prop));
    if (!currentProp.value.dataType) {
      currentProp.value.dataType = { type: 'int', specs: {} };
    } else if (!currentProp.value.dataType.specs) {
      currentProp.value.dataType.specs = {};
    }

    // Normalize backend data types to TSL abstract types for echoing
    const t = currentProp.value.dataType.type;
    if (['string'].includes(t)) currentProp.value.dataType.type = 'text';
    if (['integer', 'int32', 'int16', 'uint32', 'uint16'].includes(t)) currentProp.value.dataType.type = 'int';
    if (['number', 'float32', 'float64'].includes(t)) currentProp.value.dataType.type = 'float';
    if (['boolean'].includes(t)) currentProp.value.dataType.type = 'bool';
    
    if (currentProp.value.dataType.type === 'enum') {
       parseEnumSpecs();
    }
    currentPropSpecsJson.value = ''; // Reset
  } else {
    currentProp.value = { 
      name: '', 
      identifier: '', 
      accessMode: 'rw', 
      dataType: { type: 'int', specs: {} } 
    };
    currentPropSpecsJson.value = '';
    enumList.value = [];
  }
  showModal.value = true;
};

const closeModal = () => {
  showModal.value = false;
};

const saveProperty = () => {
  saveEnumSpecs();
  const newProps = [...properties.value];
  const propData = JSON.parse(JSON.stringify(currentProp.value));

  if (editingIndex.value === -1) {
    newProps.push(propData);
  } else {
    newProps[editingIndex.value] = propData;
  }
  emit('update:modelValue', newProps);
  closeModal();
};

const removeProperty = (index) => {
  if(!confirm(t('common_delete_confirm'))) return;
  const newProps = [...properties.value];
  newProps.splice(index, 1);
  emit('update:modelValue', newProps);
};

</script>
