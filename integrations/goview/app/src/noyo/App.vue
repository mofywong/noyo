<template>
  <n-config-provider :theme="state.theme === 'dark' ? darkTheme : null" :locale="english ? enUS : zhCN" :date-locale="english ? dateEnUS : dateZhCN" :theme-overrides="overrides">
    <go-app-provider><router-view /></go-app-provider>
  </n-config-provider>
</template>
<script setup lang="ts">
import { computed, watchEffect } from 'vue'
import { NConfigProvider, darkTheme, enUS, zhCN, dateEnUS, dateZhCN } from 'naive-ui'
import { GoAppProvider } from '@/components/GoAppProvider'
import { state } from './bridge'
const english = computed(() => state.locale.startsWith('en'))
// The editor shell reads `:root[data-theme=dark]` for every design token. Until
// this ran, the child app styled its chrome from `state.theme` but left the CSS
// tokens on the light set, so the canvas stayed pale while the panels went dark.
watchEffect(() => {
  document.documentElement.setAttribute('data-theme', state.theme === 'light' ? 'light' : 'dark')
})
const overrides = computed(() => ({ common: { primaryColor: getComputedStyle(document.documentElement).getPropertyValue('--primary-color').trim() || undefined, borderRadius: '8px' } }))
</script>
