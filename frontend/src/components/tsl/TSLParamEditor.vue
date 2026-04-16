<template>
  <div class="border rounded p-3 bg-light">
    <!-- List View -->
    <div>
      <div class="d-flex justify-content-between align-items-center mb-2">
        <h6 class="mb-0 text-muted">{{ title || $t('tsl_param') }}</h6>
        <button v-if="!isEditing" class="btn btn-sm btn-outline-primary" @click="startAdd">
          <i class="bi bi-plus-lg"></i> {{ $t('tsl_add') }}
        </button>
      </div>
      
      <table class="table table-sm table-bordered bg-white mb-3">
        <thead>
          <tr>
            <th>{{ $t('tsl_name') }}</th>
            <th>{{ $t('tsl_identifier') }}</th>
            <th>{{ $t('tsl_type') }}</th>
            <th style="width: 80px" class="text-center">必填</th>
            <th style="width: 100px">{{ $t('tsl_actions') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="(!params || params.length === 0) && !(isEditing && editingIndex === -1)">
            <td colspan="5" class="text-center text-muted small">{{ $t('tsl_no_data') }}</td>
          </tr>
          <template v-for="(param, index) in params" :key="index">
            <tr :class="{'table-primary': isEditing && editingIndex === index}">
              <td>{{ param.name }}</td>
              <td><small class="font-monospace">{{ param.identifier }}</small></td>
              <td><span class="badge bg-secondary">{{ param.dataType?.type }}</span></td>
              <td class="text-center">
                 <i v-if="param.required" class="bi bi-check-lg text-success"></i>
                 <i v-else class="bi bi-dash text-muted"></i>
              </td>
              <td>
                <button class="btn btn-xs btn-link p-0 me-2" @click="startEdit(index)" :disabled="isEditing">{{ $t('tsl_edit') }}</button>
                <button class="btn btn-xs btn-link p-0 text-danger" @click="removeParam(index)" :disabled="isEditing">{{ $t('tsl_delete') }}</button>
              </td>
            </tr>
            <!-- Inline Edit Form -->
            <tr v-if="isEditing && editingIndex === index">
              <td colspan="5" class="p-0 border-primary border-2">
                <div class="bg-white p-3 m-2 border rounded shadow-sm">
                  <h6 class="mb-3">{{ $t('tsl_edit') }} {{ $t('tsl_param') }}</h6>
                  <div class="row g-2">
                    <div class="col-md-5">
                      <label class="form-label small">{{ $t('tsl_name') }}</label>
                      <input v-model="currentParam.name" type="text" class="form-control form-control-sm">
                    </div>
                    <div class="col-md-5">
                      <label class="form-label small">{{ $t('tsl_identifier') }}</label>
                      <input v-model="currentParam.identifier" type="text" class="form-control form-control-sm" :disabled="editingIndex !== -1">
                    </div>
                    <div class="col-md-2 d-flex align-items-center mb-1">
                      <div class="form-check form-switch mt-4">
                         <input class="form-check-input" type="checkbox" v-model="currentParam.required" :id="'paramRequiredSwitch_' + index">
                         <label class="form-check-label small text-nowrap" :for="'paramRequiredSwitch_' + index">必填</label>
                      </div>
                    </div>
                    <div class="col-md-12">
                      <label class="form-label small">{{ $t('tsl_datatype') }}</label>
                      <select v-model="currentParam.dataType.type" class="form-select form-select-sm">
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
                    
                    <!-- Numeric Specs -->
                    <div class="col-md-3" v-if="['int', 'float', 'double'].includes(currentParam.dataType.type)">
                      <label class="form-label small">{{ $t('tsl_prop_unit') }}</label>
                      <input v-model="currentParam.dataType.specs.unit" type="text" class="form-control form-control-sm">
                    </div>
                    <div class="col-md-3" v-if="['int', 'float', 'double'].includes(currentParam.dataType.type)">
                      <label class="form-label small">{{ $t('tsl_prop_min') }}</label>
                      <input v-model.number="currentParam.dataType.specs.min" type="number" class="form-control form-control-sm">
                    </div>
                    <div class="col-md-3" v-if="['int', 'float', 'double'].includes(currentParam.dataType.type)">
                      <label class="form-label small">{{ $t('tsl_prop_max') }}</label>
                      <input v-model.number="currentParam.dataType.specs.max" type="number" class="form-control form-control-sm">
                    </div>
                    <div class="col-md-3" v-if="['int', 'float', 'double'].includes(currentParam.dataType.type)">
                      <label class="form-label small">{{ $t('tsl_prop_step') }}</label>
                      <input v-model.number="currentParam.dataType.specs.step" type="number" class="form-control form-control-sm">
                    </div>
                    
                    <!-- Text Specs -->
                    <div class="col-md-6" v-if="['text'].includes(currentParam.dataType.type)">
                       <label class="form-label small">{{ $t('tsl_prop_max_len') }}</label>
                       <input v-model.number="currentParam.dataType.specs.length" type="number" class="form-control form-control-sm">
                    </div>

                    <!-- Bool Specs -->
                    <div class="col-md-6" v-if="currentParam.dataType.type === 'bool'">
                       <label class="form-label small">{{ $t('tsl_bool_false') }}</label>
                       <input v-model="currentParam.dataType.specs['0']" type="text" class="form-control form-control-sm" placeholder="e.g. OFF">
                    </div>
                    <div class="col-md-6" v-if="currentParam.dataType.type === 'bool'">
                       <label class="form-label small">{{ $t('tsl_bool_true') }}</label>
                       <input v-model="currentParam.dataType.specs['1']" type="text" class="form-control form-control-sm" placeholder="e.g. ON">
                    </div>

                    <!-- Enum Specs -->
                    <div class="col-md-12" v-if="currentParam.dataType.type === 'enum'">
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
                  <div class="mt-3 text-end">
                    <button type="button" class="btn btn-sm btn-secondary me-2" @click="cancelEdit">{{ $t('tsl_cancel') }}</button>
                    <button type="button" class="btn btn-sm btn-primary" @click="saveParam">保存参数</button>
                  </div>
                </div>
              </td>
            </tr>
          </template>

          <!-- Inline Edit Form for Add -->
          <tr v-if="isEditing && editingIndex === -1">
            <td colspan="5" class="p-0 border-primary border-2">
              <div class="bg-white p-3 m-2 border rounded shadow-sm">
                <h6 class="mb-3">{{ $t('tsl_add') }} {{ $t('tsl_param') }}</h6>
                <div class="row g-2">
                  <div class="col-md-5">
                    <label class="form-label small">{{ $t('tsl_name') }}</label>
                    <input v-model="currentParam.name" type="text" class="form-control form-control-sm">
                  </div>
                  <div class="col-md-5">
                    <label class="form-label small">{{ $t('tsl_identifier') }}</label>
                    <input v-model="currentParam.identifier" type="text" class="form-control form-control-sm">
                  </div>
                  <div class="col-md-2 d-flex align-items-center mb-1">
                    <div class="form-check form-switch mt-4">
                       <input class="form-check-input" type="checkbox" v-model="currentParam.required" id="paramRequiredSwitch_add">
                       <label class="form-check-label small text-nowrap" for="paramRequiredSwitch_add">必填</label>
                    </div>
                  </div>
                  <div class="col-md-12">
                    <label class="form-label small">{{ $t('tsl_datatype') }}</label>
                    <select v-model="currentParam.dataType.type" class="form-select form-select-sm">
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
                  
                  <!-- Numeric Specs -->
                  <div class="col-md-3" v-if="['int', 'float', 'double'].includes(currentParam.dataType.type)">
                    <label class="form-label small">{{ $t('tsl_prop_unit') }}</label>
                    <input v-model="currentParam.dataType.specs.unit" type="text" class="form-control form-control-sm">
                  </div>
                  <div class="col-md-3" v-if="['int', 'float', 'double'].includes(currentParam.dataType.type)">
                    <label class="form-label small">{{ $t('tsl_prop_min') }}</label>
                    <input v-model.number="currentParam.dataType.specs.min" type="number" class="form-control form-control-sm">
                  </div>
                  <div class="col-md-3" v-if="['int', 'float', 'double'].includes(currentParam.dataType.type)">
                    <label class="form-label small">{{ $t('tsl_prop_max') }}</label>
                    <input v-model.number="currentParam.dataType.specs.max" type="number" class="form-control form-control-sm">
                  </div>
                  <div class="col-md-3" v-if="['int', 'float', 'double'].includes(currentParam.dataType.type)">
                    <label class="form-label small">{{ $t('tsl_prop_step') }}</label>
                    <input v-model.number="currentParam.dataType.specs.step" type="number" class="form-control form-control-sm">
                  </div>
                  
                  <!-- Text Specs -->
                  <div class="col-md-6" v-if="['text'].includes(currentParam.dataType.type)">
                     <label class="form-label small">{{ $t('tsl_prop_max_len') }}</label>
                     <input v-model.number="currentParam.dataType.specs.length" type="number" class="form-control form-control-sm">
                  </div>

                  <!-- Bool Specs -->
                  <div class="col-md-6" v-if="currentParam.dataType.type === 'bool'">
                     <label class="form-label small">{{ $t('tsl_bool_false') }}</label>
                     <input v-model="currentParam.dataType.specs['0']" type="text" class="form-control form-control-sm" placeholder="e.g. OFF">
                  </div>
                  <div class="col-md-6" v-if="currentParam.dataType.type === 'bool'">
                     <label class="form-label small">{{ $t('tsl_bool_true') }}</label>
                     <input v-model="currentParam.dataType.specs['1']" type="text" class="form-control form-control-sm" placeholder="e.g. ON">
                  </div>

                  <!-- Enum Specs -->
                  <div class="col-md-12" v-if="currentParam.dataType.type === 'enum'">
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
                <div class="mt-3 text-end">
                  <button type="button" class="btn btn-sm btn-secondary me-2" @click="cancelEdit">{{ $t('tsl_cancel') }}</button>
                  <button type="button" class="btn btn-sm btn-primary" @click="saveParam">保存参数</button>
                </div>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
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
  },
  title: {
    type: String,
    default: ''
  }
});

const emit = defineEmits(['update:modelValue']);

const params = computed({
  get: () => props.modelValue || [],
  set: (val) => emit('update:modelValue', val)
});

const isEditing = ref(false);
const editingIndex = ref(-1);
const currentParam = ref({
  name: '',
  identifier: '',
  required: false,
  dataType: { type: 'int', specs: {} }
});
const currentParamSpecsJson = ref('');
const enumList = ref([]);

const updateSpecsFromJson = () => {
  try {
    if (currentParamSpecsJson.value && !['enum', 'bool'].includes(currentParam.value.dataType.type)) {
        // No-op
    }
  } catch (e) {
    console.warn("Invalid JSON specs");
  }
};

// Watch for data type changes to initialize specs
watch(() => currentParam.value.dataType.type, (newType) => {
  if (!currentParam.value.dataType.specs) {
    currentParam.value.dataType.specs = {};
  }
  if (newType === 'enum') {
    parseEnumSpecs();
  }
});

const parseEnumSpecs = () => {
  enumList.value = [];
  const specs = currentParam.value.dataType.specs;
  if (specs) {
    Object.keys(specs).forEach(k => {
      enumList.value.push({ key: k, value: specs[k] });
    });
  }
};

const saveEnumSpecs = () => {
  if (currentParam.value.dataType.type === 'enum') {
    const newSpecs = {};
    enumList.value.forEach(item => {
      if (item.key) newSpecs[item.key] = item.value;
    });
    currentParam.value.dataType.specs = newSpecs;
  }
};

const addEnumItem = () => {
  enumList.value.push({ key: '', value: '' });
};

const removeEnumItem = (idx) => {
  enumList.value.splice(idx, 1);
};

const startAdd = () => {
  currentParam.value = { name: '', identifier: '', required: false, dataType: { type: 'int', specs: {} } };
  currentParamSpecsJson.value = '';
  enumList.value = [];
  editingIndex.value = -1;
  isEditing.value = true;
};

const startEdit = (index) => {
  currentParam.value = JSON.parse(JSON.stringify(params.value[index]));
  if (!currentParam.value.dataType) {
    currentParam.value.dataType = { type: 'int', specs: {} };
  } else if (!currentParam.value.dataType.specs) {
    currentParam.value.dataType.specs = {};
  }

  // Normalize backend data types to TSL abstract types for echoing
  const t = currentParam.value.dataType.type;
  if (['string'].includes(t)) currentParam.value.dataType.type = 'text';
  if (['integer', 'int32', 'int16', 'uint32', 'uint16'].includes(t)) currentParam.value.dataType.type = 'int';
  if (['number', 'float32', 'float64'].includes(t)) currentParam.value.dataType.type = 'float';
  if (['boolean'].includes(t)) currentParam.value.dataType.type = 'bool';

  if (currentParam.value.dataType.type === 'enum') {
    parseEnumSpecs();
  }
  currentParamSpecsJson.value = ''; // Reset
  editingIndex.value = index;
  isEditing.value = true;
};

const cancelEdit = () => {
  isEditing.value = false;
};

const saveParam = () => {
  if (!currentParam.value.identifier) {
    alert(t('tsl_param_id_required'));
    return;
  }
  // Ensure specs are synced
  saveEnumSpecs();
  
  const newParams = [...params.value];
  if (editingIndex.value === -1) {
    newParams.push(JSON.parse(JSON.stringify(currentParam.value)));
  } else {
    newParams[editingIndex.value] = JSON.parse(JSON.stringify(currentParam.value));
  }
  emit('update:modelValue', newParams);
  isEditing.value = false;
};

const removeParam = (index) => {
  const newParams = [...params.value];
  newParams.splice(index, 1);
  emit('update:modelValue', newParams);
};
</script>
