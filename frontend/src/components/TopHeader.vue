<template>
  <header class="top-header">
    <div class="d-flex align-items-center">
      <button class="btn btn-link text-body d-md-none me-3" @click="$emit('toggleSidebar')">
        <i class="bi bi-list fs-4"></i>
      </button>
      <div class="header-title">{{ title }}</div>
    </div>
    <div class="d-flex align-items-center gap-3">
      <!-- Theme Switcher -->
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

      <!-- Language Switcher -->
      <div class="dropdown">
        <button class="btn btn-sm btn-outline-secondary d-flex align-items-center gap-2" type="button" data-bs-toggle="dropdown" aria-expanded="false">
          <i class="bi bi-translate"></i> <span>{{ currentLangName }}</span>
        </button>
        <ul class="dropdown-menu dropdown-menu-end shadow-sm">
          <li><button class="dropdown-item d-flex align-items-center gap-2" @click="$emit('setLanguage', 'en')">
            <span>English</span>
          </button></li>
          <li><button class="dropdown-item d-flex align-items-center gap-2" @click="$emit('setLanguage', 'zh')">
            <span>中文</span>
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

const props = defineProps({
  title: String,
  currentTheme: String
});

defineEmits(['toggleSidebar', 'setTheme', 'setLanguage']);

const { locale } = useI18n();

const currentLangName = computed(() => {
  return locale.value === 'zh' ? '中文' : 'English';
});
</script>
