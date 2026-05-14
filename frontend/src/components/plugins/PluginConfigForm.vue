<template>
  <div class="card mb-4">
    <div class="card-header py-3">
      <div class="d-flex align-items-center justify-content-between w-100">
        <div class="d-flex align-items-center">
          <div class="bg-primary bg-opacity-10 p-2 rounded me-3 text-primary">
            <i class="bi bi-sliders fs-5"></i>
          </div>
          <div>
            <h5 class="mb-0">{{ title }}</h5>
            <small class="text-muted">{{ hint }}</small>
          </div>
        </div>

        <div class="d-flex align-items-center gap-3">
          <div class="form-check form-switch mb-0 d-flex align-items-center">
            <input
              class="form-check-input me-2"
              type="checkbox"
              role="switch"
              style="cursor: pointer;"
              :id="switchId"
              :checked="plugin?.status === 'running'"
              @change="$emit('toggle-status', $event.target.checked)"
            >
            <label class="form-check-label small fw-medium text-muted" :for="switchId">
              {{ plugin?.status === 'running' ? enabledText : disabledText }}
            </label>
          </div>
          <div class="vr mx-1"></div>
          <span v-if="plugin?.status === 'running'" class="badge bg-success">{{ runningText }}</span>
          <span v-else class="badge bg-secondary">{{ stoppedText }}</span>
        </div>
      </div>
    </div>

    <div class="card-body">
      <slot name="before"></slot>
      <slot name="custom"></slot>

      <template v-if="!hasCustomContent">
        <div v-if="!schema?.fields || schema.fields.length === 0" class="alert alert-info">
          {{ noConfigText }}
        </div>

        <form v-else @submit.prevent="$emit('save')">
          <div v-for="field in schema.fields" :key="field.name">
            <template v-if="field.name !== 'enabled'">
              <div v-if="field.type === 'switch' || field.type === 'bool'" class="mb-3">
                <label class="fw-bold d-block mb-1">{{ getLocalized(field.title) || field.name }}</label>
                <div v-if="getLocalized(field.description)" class="form-text text-muted mb-2 mt-0">
                  {{ getLocalized(field.description) }}
                </div>
                <div class="form-check form-switch ps-0">
                  <input
                    class="form-check-input ms-0"
                    type="checkbox"
                    role="switch"
                    :id="'field-'+field.name"
                    :checked="formData[field.name]"
                    @change="updateField(field.name, $event.target.checked)"
                  >
                </div>
              </div>

              <div v-else-if="field.type === 'int' || field.type === 'float' || field.type === 'number'" class="mb-3">
                <label :for="'field-'+field.name" class="form-label fw-bold d-block mb-1">
                  {{ getLocalized(field.title) || field.name }}
                </label>
                <div v-if="getLocalized(field.description)" class="form-text text-muted mb-2 mt-0">
                  {{ getLocalized(field.description) }}
                </div>
                <input
                  type="number"
                  class="form-control"
                  :id="'field-'+field.name"
                  :value="formData[field.name]"
                  @input="updateField(field.name, numberValue($event.target.value))"
                >
              </div>

              <div v-else-if="field.type === 'select'" class="mb-3">
                <label :for="'field-'+field.name" class="form-label fw-bold d-block mb-1">
                  {{ getLocalized(field.title) || field.name }}
                </label>
                <div v-if="getLocalized(field.description)" class="form-text text-muted mb-2 mt-0">
                  {{ getLocalized(field.description) }}
                </div>
                <select
                  class="form-select"
                  :id="'field-'+field.name"
                  :value="formData[field.name]"
                  @change="updateField(field.name, $event.target.value)"
                >
                  <option v-for="opt in field.options" :key="opt.value" :value="opt.value">
                    {{ getLocalized(opt.label) || opt.value }}
                  </option>
                </select>
              </div>

              <div v-else class="mb-3">
                <label :for="'field-'+field.name" class="form-label fw-bold d-block mb-1">
                  {{ getLocalized(field.title) || field.name }}
                </label>
                <div v-if="getLocalized(field.description)" class="form-text text-muted mb-2 mt-0">
                  {{ getLocalized(field.description) }}
                </div>
                <input
                  type="text"
                  class="form-control"
                  :id="'field-'+field.name"
                  :value="formData[field.name]"
                  @input="updateField(field.name, $event.target.value)"
                >
              </div>
            </template>
          </div>

          <div class="d-flex justify-content-end mt-4 pt-3 border-top">
            <button type="button" class="btn btn-light me-2" @click="$emit('cancel')">{{ cancelText }}</button>
            <button type="submit" class="btn btn-primary px-4" :disabled="saving">
              <span v-if="saving" class="spinner-border spinner-border-sm me-2" role="status" aria-hidden="true"></span>
              <i v-else :class="saveIconClass"></i>
              {{ saving ? savingText : saveText }}
            </button>
          </div>
        </form>
      </template>
    </div>
  </div>
</template>

<script setup>
import { computed, useSlots } from 'vue';

const props = defineProps({
  plugin: Object,
  schema: Object,
  formData: {
    type: Object,
    default: () => ({})
  },
  locale: {
    type: String,
    default: 'en'
  },
  saving: Boolean,
  switchIdPrefix: {
    type: String,
    default: 'config-switch'
  },
  title: String,
  hint: String,
  enabledText: String,
  disabledText: String,
  runningText: String,
  stoppedText: String,
  noConfigText: String,
  cancelText: String,
  saveText: String,
  savingText: String,
  saveIconClass: {
    type: String,
    default: 'bi bi-save me-2'
  }
});

const emit = defineEmits(['update:form-data', 'save', 'cancel', 'toggle-status']);
const slots = useSlots();
const hasCustomContent = computed(() => Boolean(slots.custom));
const switchId = computed(() => `${props.switchIdPrefix}-${props.plugin?.name || 'plugin'}`);

const getLocalized = (obj) => {
  if (!obj) return '';
  if (typeof obj === 'string') return obj;
  return obj[props.locale] || obj.en || '';
};

const updateField = (name, value) => {
  emit('update:form-data', { ...props.formData, [name]: value });
};

const numberValue = (value) => {
  if (value === '') return '';
  const parsed = Number(value);
  return Number.isNaN(parsed) ? value : parsed;
};
</script>
