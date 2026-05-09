<template>
  <div v-if="schema && schema.properties">
    <div v-for="{ key, prop } in sortedProperties" :key="key" class="mb-3">
      <label class="form-label">
        {{ getTitle(prop, key) }}
        <span v-if="isRequired(key)" class="text-danger">*</span>
      </label>
      
      <!-- Enum Select -->
      <select 
        v-if="prop.enum" 
        :value="modelValue[key]" 
        @change="updateField(key, $event.target.value)"
        class="form-select"
      >
        <option v-for="(opt, idx) in prop.enum" :key="opt" :value="opt">
          {{ getEnumLabel(prop, opt, idx) }}
        </option>
      </select>

      <!-- Array Enum Type (Multi Select) -->
      <div v-else-if="isArrayEnum(prop)" class="border p-2 rounded bg-light">
        <div class="d-flex justify-content-between align-items-center mb-2">
          <small class="text-muted">{{ selectedArrayValues(key, prop).length }} / {{ prop.items.enum.length }}</small>
          <div class="btn-group btn-group-sm">
            <button type="button" class="btn btn-outline-secondary" @click="selectAllArrayEnum(key, prop)">
              {{ $t('select_all') }}
            </button>
            <button type="button" class="btn btn-outline-secondary" @click="updateField(key, [])">
              {{ $t('deselect_all') }}
            </button>
          </div>
        </div>
        <div class="schema-enum-grid">
          <div v-for="(opt, idx) in prop.items.enum" :key="opt" class="form-check">
            <input
              class="form-check-input"
              type="checkbox"
              :id="`${key}_${idx}`"
              :checked="selectedArrayValues(key, prop).includes(opt)"
              @change="toggleArrayEnumValue(key, prop, opt, $event.target.checked)"
            >
            <label class="form-check-label" :for="`${key}_${idx}`">
              {{ getArrayEnumLabel(prop, opt, idx) }}
            </label>
          </div>
        </div>
      </div>

      <!-- Array Type (Simple List) -->
      <div v-else-if="prop.type === 'array'" class="border p-2 rounded bg-light">
        <div v-for="(item, index) in (modelValue[key] || [])" :key="index" class="mb-2 border-bottom pb-2">
          <div class="d-flex justify-content-between mb-2">
            <small class="fw-bold">{{ $t('item') }} {{ index + 1 }}</small>
            <button type="button" class="btn btn-xs btn-outline-danger" @click="removeArrayItem(key, index)">
              <i class="bi bi-trash"></i>
            </button>
          </div>
          <!-- Recursive Component for Array Items -->
          <SchemaForm 
            :schema="prop.items" 
            :modelValue="item" 
            @update:modelValue="(val) => updateArrayItem(key, index, val)" 
          />
        </div>
        <button type="button" class="btn btn-sm btn-outline-primary mt-1" @click="addArrayItem(key, prop.items)">
          <i class="bi bi-plus"></i> {{ $t('tsl_add') }} {{ getTitle(prop, key) }}
        </button>
      </div>

      <!-- Object Type (Nested) -->
      <div v-else-if="prop.type === 'object'" class="border p-2 rounded">
        <SchemaForm 
          :schema="prop" 
          :modelValue="modelValue[key] || {}" 
          @update:modelValue="(val) => updateField(key, val)" 
        />
      </div>

      <!-- Boolean Type -->
      <div v-else-if="prop.type === 'boolean'" class="form-check">
        <input 
          class="form-check-input" 
          type="checkbox" 
          :checked="modelValue[key]" 
          @change="updateField(key, $event.target.checked)"
        >
      </div>

      <!-- Integer/Number Type -->
      <input 
        v-else-if="prop.type === 'integer' || prop.type === 'number'" 
        type="number" 
        class="form-control" 
        :value="modelValue[key] !== undefined ? modelValue[key] : prop.default"
        @input="updateField(key, Number($event.target.value))"
      >

      <!-- Code/JSON Type -->
      <textarea 
        v-else-if="prop.format === 'code' || prop.format === 'json'"
        class="form-control font-monospace" 
        rows="10"
        :value="modelValue[key] !== undefined ? modelValue[key] : prop.default"
        @input="updateField(key, $event.target.value)"
      ></textarea>

      <!-- String Type -->
      <input 
        v-else 
        type="text" 
        class="form-control" 
        :value="modelValue[key] !== undefined ? modelValue[key] : prop.default"
        @input="updateField(key, $event.target.value)"
      >
      
      <div v-if="getDescription(prop)" class="form-text">{{ getDescription(prop) }}</div>
    </div>
  </div>
</template>

<script>
export default {
  name: 'SchemaForm' // Necessary for recursive calls
}
</script>

<script setup>
import { computed } from 'vue';
import { useI18n } from 'vue-i18n';

const { t, locale } = useI18n();

const isZh = computed(() => locale.value === 'zh' || locale.value === 'zh-CN');

const getTitle = (prop, key) => {
  if (isZh.value && prop.title_zh) {
    return prop.title_zh;
  }
  return t(prop.title || key);
};

const getDescription = (prop) => {
  if (isZh.value && prop.description_zh) {
    return prop.description_zh;
  }
  return prop.description ? t(prop.description) : '';
};

const getEnumLabel = (prop, opt, idx) => {
  if (isZh.value && prop.enumNames_zh && prop.enumNames_zh[idx]) {
    return prop.enumNames_zh[idx];
  }
  if (prop.enumNames && prop.enumNames[idx]) {
    return prop.enumNames[idx];
  }
  return opt;
};

const props = defineProps({
  schema: {
    type: Object,
    required: true
  },
  modelValue: {
    type: Object,
    default: () => ({})
  }
});

const emit = defineEmits(['update:modelValue']);

const isRequired = (key) => {
  return props.schema.required && props.schema.required.includes(key);
};

const isArrayEnum = (prop) => {
  return prop.type === 'array' && prop.items && Array.isArray(prop.items.enum);
};

const selectedArrayValues = (key, prop) => {
  const value = props.modelValue[key];
  if (Array.isArray(value)) return value;
  if (Array.isArray(prop.default)) return prop.default;
  return [];
};

const getArrayEnumLabel = (prop, opt, idx) => {
  if (isZh.value && prop.items.enumNames_zh && prop.items.enumNames_zh[idx]) {
    return prop.items.enumNames_zh[idx];
  }
  if (prop.items.enumNames && prop.items.enumNames[idx]) {
    return prop.items.enumNames[idx];
  }
  return opt;
};

const toggleArrayEnumValue = (key, prop, opt, checked) => {
  const current = selectedArrayValues(key, prop);
  const next = checked
    ? [...new Set([...current, opt])]
    : current.filter(item => item !== opt);
  updateField(key, next);
};

const selectAllArrayEnum = (key, prop) => {
  updateField(key, [...prop.items.enum]);
};

const sortedProperties = computed(() => {
  if (!props.schema || !props.schema.properties) return [];
  
  const properties = props.schema.properties;
  const required = props.schema.required || [];
  
  const keys = Object.keys(properties);
  
  // Split into required and optional
  const requiredKeys = keys.filter(k => required.includes(k));
  const optionalKeys = keys.filter(k => !required.includes(k));
  
  // Concatenate: required first
  const sortedKeys = [...requiredKeys, ...optionalKeys];
  
  // Map to objects for v-for
  return sortedKeys.map(key => ({
    key,
    prop: properties[key]
  }));
});

const updateField = (key, value) => {
  const newValue = { ...props.modelValue, [key]: value };
  emit('update:modelValue', newValue);
};

const addArrayItem = (key, itemSchema) => {
  const currentArray = props.modelValue[key] || [];
  // Initialize new item with defaults if possible
  const newItem = {};
  if (itemSchema.type === 'object' && itemSchema.properties) {
    for (const k in itemSchema.properties) {
      if (itemSchema.properties[k].default !== undefined) {
        newItem[k] = itemSchema.properties[k].default;
      }
    }
  }
  emit('update:modelValue', { ...props.modelValue, [key]: [...currentArray, newItem] });
};

const removeArrayItem = (key, index) => {
  const currentArray = props.modelValue[key] || [];
  const newArray = currentArray.filter((_, i) => i !== index);
  emit('update:modelValue', { ...props.modelValue, [key]: newArray });
};

const updateArrayItem = (key, index, val) => {
  const currentArray = props.modelValue[key] || [];
  const newArray = [...currentArray];
  newArray[index] = val;
  emit('update:modelValue', { ...props.modelValue, [key]: newArray });
};
</script>

<style scoped>
.schema-enum-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(160px, 1fr));
  gap: 0.25rem 0.75rem;
  max-height: 260px;
  overflow: auto;
}
</style>
