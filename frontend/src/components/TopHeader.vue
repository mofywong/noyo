<template>
  <header class="top-header">
    <div class="d-flex align-items-center">
      <button class="btn btn-link text-body d-md-none me-3" @click="$emit('toggleSidebar')">
        <i class="bi bi-list fs-4"></i>
      </button>
      <div class="header-title">{{ title }}</div>
    </div>
    <div class="d-flex align-items-center gap-3">
      <div
        v-if="mqttStatus"
        class="mqtt-status-pill"
        :class="mqttStatus.connected ? 'is-connected' : 'is-disconnected'"
        :title="mqttStatus.broker || ''"
      >
        <span class="mqtt-status-dot"></span>
        <span class="mqtt-status-label">MQTT</span>
        <span class="mqtt-status-value">{{ mqttStatus.connected ? 'Connected' : 'Disconnected' }}</span>
      </div>

      <div class="dropdown">
        <button class="btn btn-sm btn-outline-secondary d-flex align-items-center gap-2" type="button" data-bs-toggle="dropdown" aria-expanded="false">
          <i class="bi bi-circle-half"></i>
        </button>
        <ul class="dropdown-menu dropdown-menu-end shadow-sm">
          <li><button class="dropdown-item d-flex align-items-center gap-2" @click="$emit('setTheme', 'light')">
            <i class="bi bi-sun"></i> <span>{{ $t('theme_light') }}</span>
          </button></li>
          <li><button class="dropdown-item d-flex align-items-center gap-2" @click="$emit('setTheme', 'dark')">
            <i class="bi bi-moon"></i> <span>{{ $t('theme_dark') }}</span>
          </button></li>
          <li><button class="dropdown-item d-flex align-items-center gap-2" @click="$emit('setTheme', 'system')">
            <i class="bi bi-circle-half"></i> <span>{{ $t('theme_system') }}</span>
          </button></li>
        </ul>
      </div>

      <div class="dropdown">
        <button class="btn btn-sm btn-outline-secondary d-flex align-items-center gap-2" type="button" data-bs-toggle="dropdown" aria-expanded="false">
          <i class="bi bi-translate"></i> <span>{{ currentLangName }}</span>
        </button>
        <ul class="dropdown-menu dropdown-menu-end shadow-sm">
          <li><button class="dropdown-item d-flex align-items-center gap-2" @click="$emit('setLanguage', 'en')">
            <span>{{ languageEnglish }}</span>
          </button></li>
          <li><button class="dropdown-item d-flex align-items-center gap-2" @click="$emit('setLanguage', 'zh')">
            <span>{{ languageChinese }}</span>
          </button></li>
        </ul>
      </div>

      <div class="dropdown">
        <a href="#" class="d-flex align-items-center text-decoration-none dropdown-toggle text-body" data-bs-toggle="dropdown">
          <div class="bg-body rounded-circle d-flex align-items-center justify-content-center border" style="width: 32px; height: 32px;">
            <i class="bi bi-person-fill text-secondary"></i>
          </div>
        </a>
        <ul class="dropdown-menu dropdown-menu-end shadow-sm border-0">
          <li><a class="dropdown-item" href="#">{{ $t('header_profile') }}</a></li>
          <li><hr class="dropdown-divider"></li>
          <li><a class="dropdown-item text-danger" href="#">{{ $t('header_logout') }}</a></li>
        </ul>
      </div>
    </div>
  </header>
</template>

<script setup>
import { computed } from 'vue';
import { useI18n } from 'vue-i18n';
import { gatewayText } from '../utils/gatewayLocale';

defineProps({
  title: String,
  currentTheme: String,
  mqttStatus: Object
});

defineEmits(['toggleSidebar', 'setTheme', 'setLanguage']);

const { locale } = useI18n();

const languageEnglish = computed(() => gatewayText(locale.value, 'language_english'));
const languageChinese = computed(() => gatewayText(locale.value, 'language_chinese'));

const currentLangName = computed(() => {
  return locale.value === 'zh' ? languageChinese.value : languageEnglish.value;
});
</script>

<style scoped>
.mqtt-status-pill {
  align-items: center;
  border: 1px solid transparent;
  border-radius: 999px;
  display: inline-flex;
  font-size: 0.75rem;
  font-weight: 700;
  gap: 0.4rem;
  min-height: 2rem;
  padding: 0 0.7rem;
  white-space: nowrap;
}

.mqtt-status-pill.is-connected {
  background: rgba(16, 185, 129, 0.12);
  border-color: rgba(16, 185, 129, 0.24);
  color: #047857;
}

.mqtt-status-pill.is-disconnected {
  background: rgba(239, 68, 68, 0.1);
  border-color: rgba(239, 68, 68, 0.22);
  color: #b91c1c;
}

.mqtt-status-dot {
  border-radius: 50%;
  display: inline-block;
  height: 0.48rem;
  width: 0.48rem;
}

.is-connected .mqtt-status-dot {
  background: #10b981;
  box-shadow: 0 0 0 0.22rem rgba(16, 185, 129, 0.16);
}

.is-disconnected .mqtt-status-dot {
  background: #ef4444;
  box-shadow: 0 0 0 0.22rem rgba(239, 68, 68, 0.14);
}

.mqtt-status-label {
  color: inherit;
}

.mqtt-status-value {
  color: color-mix(in srgb, currentColor 82%, var(--text-secondary));
}

@media (max-width: 768px) {
  .mqtt-status-value {
    display: none;
  }
}
</style>
