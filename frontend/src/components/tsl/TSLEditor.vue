<template>
  <div class="d-flex flex-column h-100">
    <AppTabs v-model="activeTab" :label="$t('tsl_properties')" :items="[{ value: 'properties', label: $t('tsl_properties') }, { value: 'events', label: $t('tsl_events') }, { value: 'services', label: $t('tsl_services') }]">
      <template #actions><button data-dialog-confirm type="button" class="btn btn-sm btn-outline-primary" @click="$emit('save')"><i class="bi bi-save me-1" aria-hidden="true"></i>{{ $t('tsl_save') }}</button></template>
    </AppTabs>

    <div class="flex-grow-1 overflow-auto">
      <div v-if="activeTab === 'properties'">
        <TSLPropertyEditor 
          v-model="modelValue.properties" 
          :protocolSchema="protocolSchema"
          :mapping="mapping"
          @update:mapping="updateMapping"
        />
      </div>
      <div v-else-if="activeTab === 'events'">
        <TSLEventEditor v-model="modelValue.events" />
      </div>
      <div v-else-if="activeTab === 'services'">
        <TSLServiceEditor v-model="modelValue.services" />
      </div>
    </div>
  </div>
</template>

<script setup>
import AppTabs from '@/components/AppTabs.vue';
import { ref } from 'vue';
import TSLPropertyEditor from './TSLPropertyEditor.vue';
import TSLEventEditor from './TSLEventEditor.vue';
import TSLServiceEditor from './TSLServiceEditor.vue';

const props = defineProps({
  modelValue: {
    type: Object,
    default: () => ({ properties: [], events: [], services: [] })
  },
  protocolSchema: {
    type: Object, // The full protocol schema (product config schema)
    default: null
  },
  mapping: {
    type: Object, // The protocol-specific mapping (e.g., points array)
    default: () => ({ points: [] })
  }
});

const emit = defineEmits(['update:modelValue', 'update:mapping', 'save']);

const activeTab = ref('properties');

const updateMapping = (newMapping) => {
  emit('update:mapping', newMapping);
};

</script>
