<template>
  <div class="card h-100 position-relative overflow-hidden plugin-card" :class="{ 'plugin-card-running': plugin.status === 'running' }">
    <!-- Overlay for unauthorized pro plugins -->
    <div v-if="plugin.isPro && plugin.isUnauthorized" class="position-absolute w-100 h-100" style="background: rgba(0, 0, 0, 0.02); z-index: 10; top: 0; left: 0; pointer-events: none;">
      <!-- Removed center badge to prevent blocking content -->
    </div>

    <div class="card-body d-flex flex-column" :class="{ 'opacity-50': plugin.isPro && plugin.isUnauthorized }">
      <div class="d-flex justify-content-between align-items-start mb-3">
        <div class="bg-light rounded p-3 text-primary d-flex align-items-center justify-content-center" style="width: 64px; height: 64px;">
          <img v-if="pluginIconUrl(plugin.icon)" :src="pluginIconUrl(plugin.icon)" style="width: 32px; height: 32px; object-fit: contain;">
          <i v-else class="bi bi-box-seam fs-3"></i>
        </div>
        <div class="d-flex flex-column align-items-end">
          <span v-if="plugin.status === 'running'" class="badge bg-success bg-opacity-10 text-success rounded-pill mb-1">{{ $t('status_running') }}</span>
          <span v-else class="badge bg-secondary bg-opacity-10 text-secondary rounded-pill mb-1">{{ $t('status_stopped') }}</span>
          <span v-if="plugin.isPro && plugin.isUnauthorized" class="badge bg-warning text-dark rounded-pill d-flex align-items-center shadow-sm" style="font-size: 0.65rem;">
            <i class="bi bi-lock-fill me-1"></i> {{ $t('pro_feature_locked') || '专业版功能' }}
          </span>
        </div>
      </div>
      
      <h5 class="card-title fw-bold mb-1">
        {{ plugin.title ? (plugin.title[locale] || plugin.title['en'] || plugin.name) : plugin.name }}
        <span v-if="plugin.isPro" class="badge bg-danger text-white ms-1" style="font-size: 0.6rem; vertical-align: middle;">PRO</span>
      </h5>
      <p class="card-text text-muted small flex-grow-1">
        {{ plugin.description ? (plugin.description[locale] || plugin.description['en'] || '') : $t('plugin_desc_default', { category: plugin.category ? plugin.category.toUpperCase() : 'PLUGIN', name: plugin.name }) }}
      </p>
      
      <div class="d-flex align-items-center justify-content-between mt-3 pt-3 border-top">
        <button class="btn btn-sm btn-outline-primary" @click="$emit('configure')" :disabled="plugin.isPro && plugin.isUnauthorized" v-permission="'plugin:config'">
          <i class="bi bi-gear-fill me-1"></i> {{ $t('configure') }}
        </button>
        <div class="form-check form-switch" v-permission="'plugin:config'">
          <input class="form-check-input" type="checkbox" role="switch"
                 :id="'switch-' + plugin.name"
                 :checked="plugin.status === 'running'"
                 :disabled="plugin.isPro && plugin.isUnauthorized"
                 @change="$emit('update-status', plugin.name, $event.target.checked)">
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { useI18n } from 'vue-i18n';
import { pluginIconUrl } from '../utils/pluginIconBranding.js';

const { locale } = useI18n();

defineProps({
  plugin: Object
});

defineEmits(['configure', 'update-status']);
</script>

<style scoped>
.plugin-card {
  transition: border-color 0.18s ease, box-shadow 0.18s ease, transform 0.18s ease;
}

.plugin-card-running {
  animation: pluginRunningBreath 2.8s ease-in-out infinite;
  border-color: color-mix(in srgb, #22c55e 34%, var(--border-color));
  box-shadow:
    0 0 0 1px rgb(34 197 94 / 0.18),
    0 0 18px rgb(34 197 94 / 0.18),
    var(--card-shadow);
}

@keyframes pluginRunningBreath {
  0%,
  100% {
    box-shadow:
      0 0 0 1px rgb(34 197 94 / 0.14),
      0 0 14px rgb(34 197 94 / 0.14),
      var(--card-shadow);
  }

  50% {
    box-shadow:
      0 0 0 1px rgb(34 197 94 / 0.28),
      0 0 26px rgb(34 197 94 / 0.24),
      var(--card-shadow);
  }
}
</style>
