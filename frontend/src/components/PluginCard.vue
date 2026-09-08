<template>
  <div
    class="card h-100 plugin-card"
    :class="{ 'plugin-card-running': plugin.status === 'running' }"
  >
    <div class="card-body d-flex flex-column" :class="{ 'opacity-50': plugin.isPro && plugin.isUnauthorized }">
      <div class="d-flex justify-content-between align-items-start mb-3">
        <div class="plugin-icon d-flex align-items-center justify-content-center">
          <img v-if="pluginIconUrl(plugin.icon)" :src="pluginIconUrl(plugin.icon)" class="plugin-icon-img" alt="">
          <i v-else class="bi bi-box-seam fs-3"></i>
        </div>
        <div class="d-flex flex-column align-items-end gap-1">
          <span
            class="dash-pill"
            :class="plugin.status === 'running' ? 'dash-pill--success' : 'dash-pill--neutral'"
          >
            <span class="dash-pill-dot"></span>
            {{ plugin.status === 'running' ? $t('status_running') : $t('status_stopped') }}
          </span>
          <span v-if="plugin.isPro && plugin.isUnauthorized" class="dash-pill dash-pill--warning">
            <i class="bi bi-lock-fill me-1"></i> {{ $t('pro_feature_locked') }}
          </span>
        </div>
      </div>

      <h5 class="card-title fw-bold mb-1">
        {{ plugin.title ? (plugin.title[locale] || plugin.title['en'] || plugin.name) : plugin.name }}
        <span v-if="plugin.isPro" class="badge text-bg-danger ms-1 align-middle" style="padding: 2px 6px; font-size: 0.62rem; font-weight: 700;">PRO</span>
      </h5>
      <p class="card-text text-secondary small flex-grow-1 mb-0">
        {{ plugin.description ? (plugin.description[locale] || plugin.description['en'] || '') : $t('plugin_desc_default', { category: plugin.category ? plugin.category.toUpperCase() : 'PLUGIN', name: plugin.name }) }}
      </p>

      <div v-if="plugin.isPro && plugin.isUnauthorized" class="small text-warning d-flex align-items-center gap-1 mt-2">
        <i class="bi bi-stars"></i>{{ $t('plugin_card_pro_locked') }}
      </div>

      <div class="d-flex align-items-center justify-content-between mt-3 pt-3 border-top">
        <LiquidGlassButton
          variant="outline-primary"
          size="sm"
          icon="bi bi-gear-fill"
          @click="$emit('configure')"
          :disabled="plugin.isPro && plugin.isUnauthorized"
          v-permission="'plugin:config'"
        >
          {{ $t('plugin_card_configure') }}
        </LiquidGlassButton>
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
  width: 52px;
  height: 52px;
  background: var(--glass-tint);
  border: 1px solid var(--glass-border);
  border-radius: 14px;
  box-shadow: var(--glass-highlight), 0 4px 12px rgba(0, 0, 0, 0.06);
  color: var(--color-brand);
  font-size: 1.35rem;
  flex-shrink: 0;
}

[data-bs-theme="dark"] .plugin-icon {
  background: rgba(255, 255, 255, 0.04);
  border-color: rgba(255, 255, 255, 0.12);
  box-shadow: inset 0 1px 1px rgba(255, 255, 255, 0.15), 0 4px 12px rgba(0, 0, 0, 0.3);
}

.plugin-icon-img {
  width: 30px;
  height: 30px;
  object-fit: contain;
}

.plugin-switch:checked {
  background-color: var(--color-brand);
  border-color: var(--color-brand);
}

/* 晶体发光状态胶囊 */
.dash-pill {
  display: inline-flex;
  align-items: center;
  gap: 0.3rem;
  padding: 0.2rem 0.6rem;
  border-radius: 9999px;
  font-size: 0.72rem;
  font-weight: 600;
  letter-spacing: 0.02em;
  line-height: 1.2;
  transition: all var(--noyo-duration-fast) var(--noyo-ease-standard);
  backdrop-filter: blur(8px);
  -webkit-backdrop-filter: blur(8px);
}

.dash-pill-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background-color: currentColor;
  box-shadow: 0 0 6px currentColor;
  flex-shrink: 0;
  display: inline-block;
}

.dash-pill--success {
  color: #15803d !important;
  background: rgba(22, 163, 74, 0.12);
  border: 1px solid rgba(22, 163, 74, 0.28);
}

[data-bs-theme="dark"] .dash-pill--success {
  color: #4ade80 !important;
  background: rgba(74, 222, 128, 0.15);
  border: 1px solid rgba(74, 222, 128, 0.35);
}

.dash-pill--neutral {
  color: var(--text-secondary) !important;
  background: rgba(100, 116, 139, 0.12);
  border: 1px solid rgba(100, 116, 139, 0.24);
}

.dash-pill--warning {
  color: #b45309 !important;
  background: rgba(245, 158, 11, 0.12);
  border: 1px solid rgba(245, 158, 11, 0.28);
}

[data-bs-theme="dark"] .dash-pill--warning {
  color: #fbbf24 !important;
  background: rgba(251, 191, 36, 0.15);
  border: 1px solid rgba(251, 191, 36, 0.35);
}

/* 运行态：成功语义色呼吸光晕与物理弹性微跳动 */
.plugin-card {
  transition: transform 0.4s cubic-bezier(0.34, 1.56, 0.64, 1), box-shadow 0.35s ease, border-color 0.35s ease;
}

@media (hover: hover) and (pointer: fine) {
  .plugin-card:hover {
    transform: translateY(-6px) scale(1.012);
    border-color: rgba(147, 197, 253, 0.75);
    box-shadow: 0 16px 36px -6px color-mix(in srgb, var(--color-brand) 18%, transparent), 0 4px 14px rgba(0, 0, 0, 0.1);
  }
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
