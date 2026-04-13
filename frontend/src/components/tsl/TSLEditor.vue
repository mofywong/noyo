<template>
  <div class="d-flex flex-column h-100">
    <ul class="nav nav-tabs mb-3">
      <li class="nav-item">
        <button class="nav-link" :class="{ active: activeTab === 'properties' }" @click="activeTab = 'properties'">
          {{ $t('tsl_properties') }}
        </button>
      </li>
      <li class="nav-item">
        <button class="nav-link" :class="{ active: activeTab === 'events' }" @click="activeTab = 'events'">
          {{ $t('tsl_events') }}
        </button>
      </li>
      <li class="nav-item">
        <button class="nav-link" :class="{ active: activeTab === 'services' }" @click="activeTab = 'services'">
          {{ $t('tsl_services') }}
        </button>
      </li>
      <li class="nav-item ms-auto">
        <button class="btn btn-sm btn-outline-primary" @click="$emit('save')">
          <i class="bi bi-save me-1"></i> {{ $t('tsl_save') }}
        </button>
      </li>
    </ul>

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
