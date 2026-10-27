<template>
  <LiquidGlassButton ref="button" class="ai-assistant-fab" type="button" variant="crystal" size="lg" :refract="true" :title="label" :aria-label="label">
    <span class="ai-assistant-fab-core" aria-hidden="true"><i class="bi bi-robot" /></span>
    <span class="ai-assistant-fab-label">{{ label }}</span>
  </LiquidGlassButton>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import LiquidGlassButton from './LiquidGlassButton.vue'
import 'bootstrap-icons/font/bootstrap-icons.css'
import '../../styles/liquid-glass.css'

defineProps<{ label: string }>()
const button = ref<InstanceType<typeof LiquidGlassButton>>()
defineExpose({ focus: () => button.value?.$el.querySelector('button')?.focus() })
</script>

<style scoped>
.ai-assistant-fab {
  --ai-fab-brand: var(--color-brand, var(--ioc-ai-brand));
  --ai-fab-white: var(--bs-white, var(--ioc-ai-on-accent));
  --ai-fab-float-duration: 3s;
  --ai-fab-pulse-duration: 2s;
  position: relative;
  width: 56px;
  height: 56px;
}
.ai-assistant-fab::before {
  content: '';
  position: absolute;
  inset: -12px;
  border-radius: 50%;
  background: radial-gradient(circle, color-mix(in srgb, var(--ai-fab-brand) 48%, var(--noyo-color-transparent)), var(--noyo-color-transparent) 68%);
  filter: blur(8px);
  opacity: .9;
  pointer-events: none;
}
.ai-assistant-fab :deep(.noyo-glass-btn) {
  --lg-btn-bg: color-mix(in srgb, var(--ai-fab-brand) 48%, var(--noyo-glass-regular-tint));
  --lg-btn-bg-hover: color-mix(in srgb, var(--ai-fab-brand) 62%, var(--noyo-glass-regular-tint));
  --lg-btn-border: color-mix(in srgb, var(--ai-fab-brand) 64%, var(--noyo-glass-regular-border));
  --lg-btn-text: var(--ai-fab-white);
  --lg-btn-shadow: var(--shadow-floating, var(--ioc-ai-shadow));
  z-index: 1;
  width: 56px;
  height: 56px;
  min-height: 56px;
  padding: 0;
  border-radius: 50%;
  animation: ai-assistant-fab-float var(--ai-fab-float-duration) ease-in-out infinite;
}
.ai-assistant-fab:hover :deep(.noyo-glass-btn) { transform: scale(1.08); animation-play-state: paused; }
.ai-assistant-fab:active :deep(.noyo-glass-btn) { transform: scale(.95); animation: none; }
.ai-assistant-fab-core {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 34px;
  height: 34px;
  border: 1px solid color-mix(in srgb, var(--ai-fab-white) 56%, var(--noyo-color-transparent));
  border-radius: 50%;
  background: color-mix(in srgb, var(--ai-fab-brand) 68%, var(--noyo-color-transparent));
  box-shadow: inset 0 1px 0 color-mix(in srgb, var(--ai-fab-white) 48%, var(--noyo-color-transparent));
}
.ai-assistant-fab-core i { animation: ai-assistant-fab-pulse var(--ai-fab-pulse-duration) ease-in-out infinite; }
.ai-assistant-fab-label { position: absolute; width: 1px; height: 1px; overflow: hidden; clip-path: inset(50%); }
@keyframes ai-assistant-fab-float { 0%, 100% { transform: translateY(0); } 50% { transform: translateY(-6px); } }
@keyframes ai-assistant-fab-pulse { 0%, 100% { opacity: 1; } 50% { opacity: .85; } }
@media (prefers-reduced-motion: reduce) {
  .ai-assistant-fab :deep(.noyo-glass-btn), .ai-assistant-fab-core i { animation: none; }
}
</style>
