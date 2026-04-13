<template>
  <div class="card h-100">
    <div class="card-body d-flex flex-column">
      <div class="d-flex justify-content-between align-items-start mb-3">
        <div class="bg-light rounded p-3 text-primary d-flex align-items-center justify-content-center" style="width: 64px; height: 64px;">
          <img v-if="plugin.icon" :src="plugin.icon" style="width: 32px; height: 32px; object-fit: contain;">
          <i v-else class="bi bi-box-seam fs-3"></i>
        </div>
        <span v-if="plugin.status === 'running'" class="badge bg-success bg-opacity-10 text-success rounded-pill">{{ $t('status_running') }}</span>
        <span v-else class="badge bg-secondary bg-opacity-10 text-secondary rounded-pill">{{ $t('status_stopped') }}</span>
      </div>
      
      <h5 class="card-title fw-bold mb-1">{{ plugin.title ? (plugin.title[locale] || plugin.title['en'] || plugin.name) : plugin.name }}</h5>
      <p class="card-text text-muted small flex-grow-1">
        {{ plugin.description ? (plugin.description[locale] || plugin.description['en'] || '') : $t('plugin_desc_default', { category: plugin.category ? plugin.category.toUpperCase() : 'PLUGIN', name: plugin.name }) }}
      </p>
      
      <div class="d-flex align-items-center justify-content-between mt-3 pt-3 border-top">
        <button class="btn btn-sm btn-outline-primary" @click="$emit('configure')">
          <i class="bi bi-gear-fill me-1"></i> {{ $t('configure') }}
        </button>
        <div class="form-check form-switch">
          <input class="form-check-input" type="checkbox" role="switch" 
                 :id="'switch-' + plugin.name" 
                 :checked="plugin.status === 'running'"
                 @change="$emit('update-status', plugin.name, $event.target.checked)">
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { useI18n } from 'vue-i18n';

const { locale } = useI18n();

defineProps({
  plugin: Object
});

defineEmits(['configure', 'update-status']);
</script>
