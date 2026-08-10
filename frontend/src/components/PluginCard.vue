<template>
  <div
    class="card h-100 plugin-card"
    :class="{ 'plugin-card-running': plugin.status === 'running' }"
  >
    <div class="card-body d-flex flex-column" :class="{ 'opacity-50': plugin.isPro && plugin.isUnauthorized }">
      <div class="d-flex justify-content-between align-items-start mb-3">
        <div class="plugin-icon rounded-3 d-flex align-items-center justify-content-center">
          <img v-if="pluginIconUrl(plugin.icon)" :src="pluginIconUrl(plugin.icon)" class="plugin-icon-img" alt="">
          <i v-else class="bi bi-box-seam fs-3"></i>
        </div>
        <div class="d-flex flex-column align-items-end gap-1">
          <span
            class="spec-badge"
            :class="plugin.status === 'running' ? 'spec-badge--success' : 'spec-badge--neutral'"
          >
            <span
              class="status-dot"
              :class="plugin.status === 'running' ? 'status-dot--online' : 'status-dot--offline'"
            ></span>
            {{ plugin.status === 'running' ? $t('status_running') : $t('status_stopped') }}
          </span>
          <span v-if="plugin.isPro && plugin.isUnauthorized" class="spec-badge spec-badge--warning">
            <i class="bi bi-lock-fill me-1"></i> {{ $t('pro_feature_locked') }}
          </span>
        </div>
      </div>

      <h5 class="card-title fw-bold mb-1">
        {{ plugin.title ? (plugin.title[locale] || plugin.title['en'] || plugin.name) : plugin.name }}
        <span v-if="plugin.isPro" class="spec-badge spec-badge--danger ms-1 align-middle" style="padding: 1px 6px; font-size: 0.6rem;">PRO</span>
      </h5>
      <p class="card-text text-secondary small flex-grow-1 mb-0">
        {{ plugin.description ? (plugin.description[locale] || plugin.description['en'] || '') : $t('plugin_desc_default', { category: plugin.category ? plugin.category.toUpperCase() : 'PLUGIN', name: plugin.name }) }}
      </p>

      <div v-if="plugin.isPro && plugin.isUnauthorized" class="small text-warning d-flex align-items-center gap-1 mt-2">
        <i class="bi bi-stars"></i>{{ $t('plugin_card_pro_locked') }}
      </div>

      <div class="d-flex align-items-center justify-content-between mt-3 pt-3 border-top">
        <button
          class="btn btn-sm btn-outline-primary"
          @click="$emit('configure')"
          :disabled="plugin.isPro && plugin.isUnauthorized"
          v-permission="'plugin:config'"
        >
          <i class="bi bi-gear-fill me-1"></i> {{ $t('plugin_card_configure') }}
        </button>
        <div class="form-check form-switch mb-0" v-permission="'plugin:config'">
          <input
            class="form-check-input plugin-switch"
            type="checkbox"
            role="switch"
            :id="'switch-' + plugin.name"
            :checked="plugin.status === 'running'"
            :disabled="plugin.isPro && plugin.isUnauthorized"
            :aria-label="plugin.name"
            @change="$emit('update-status', plugin.name, $event.target.checked)"
          >
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
/* Noyo UX Guidelines §8.1 / §8.5 / §3.3 */
.plugin-icon {
  width: 64px;
  height: 64px;
  background: var(--bg-hover);
  color: var(--color-brand);
  font-size: 1.5rem;
}

.plugin-icon-img {
  width: 32px;
  height: 32px;
  object-fit: contain;
}

.plugin-switch:checked {
  background-color: var(--color-brand);
  border-color: var(--color-brand);
}

/* 运行态：成功语义色呼吸光晕（§3.3，2s 周期，40% 幅度） */
.plugin-card {
  transition: border-color 0.15s ease, box-shadow 0.15s ease;
}

.plugin-card-running {
  border-color: color-mix(in srgb, var(--color-success) 34%, var(--border-color));
  animation: pluginRunningBreath 2s ease-in-out infinite;
}

@keyframes pluginRunningBreath {
  0%,
  100% {
    box-shadow:
      0 0 0 1px color-mix(in srgb, var(--color-success) 14%, transparent),
      0 0 14px color-mix(in srgb, var(--color-success) 14%, transparent),
      var(--shadow-card);
  }

  50% {
    box-shadow:
      0 0 0 1px color-mix(in srgb, var(--color-success) 28%, transparent),
      0 0 26px color-mix(in srgb, var(--color-success) 24%, transparent),
      var(--shadow-card);
  }
}
</style>
