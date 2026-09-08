<template>
  <div
    class="liquid-glass-card"
    :class="[
      `liquid-glass-card--${variant}`,
      `liquid-glass-card--elevation-${elevation}`,
      { 'liquid-glass-card--interactive': interactive },
      customClass
    ]"
    :style="customStyle"
  >
    <header v-if="$slots.header || title" class="liquid-glass-card__header" :class="headerClass">
      <slot name="header">
        <div class="liquid-glass-card__title-wrap">
          <i v-if="icon" :class="[icon, 'liquid-glass-card__icon']"></i>
          <h3 v-if="title" class="liquid-glass-card__title">{{ title }}</h3>
          <span v-if="subtitle" class="liquid-glass-card__subtitle">{{ subtitle }}</span>
        </div>
        <div v-if="$slots.actions" class="liquid-glass-card__actions">
          <slot name="actions" />
        </div>
      </slot>
    </header>

    <div class="liquid-glass-card__body" :class="bodyClass">
      <slot />
    </div>

    <footer v-if="$slots.footer" class="liquid-glass-card__footer" :class="footerClass">
      <slot name="footer" />
    </footer>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'

interface LiquidGlassCardProps {
  variant?: 'surface' | 'table' | 'compact' | 'kpi' | 'plain'
  elevation?: 'flat' | 'regular' | 'floating' | 'island'
  title?: string
  subtitle?: string
  icon?: string
  interactive?: boolean
  customClass?: string
  headerClass?: string
  bodyClass?: string
  footerClass?: string
  customStyle?: Record<string, any>
}

const props = withDefaults(defineProps<LiquidGlassCardProps>(), {
  variant: 'surface',
  elevation: 'regular',
  title: '',
  subtitle: '',
  icon: '',
  interactive: false,
  customClass: '',
  headerClass: '',
  bodyClass: '',
  footerClass: '',
  customStyle: () => ({})
})
</script>

<style scoped>
.liquid-glass-card {
  position: relative;
  overflow: hidden;
  border-radius: var(--radius-card, 16px);
  background: var(--noyo-dashboard-liquid-tint, var(--bg-surface));
  backdrop-filter: blur(var(--noyo-dashboard-liquid-blur, 10px)) saturate(175%) brightness(var(--noyo-dashboard-liquid-backdrop-brightness, 1.05));
  -webkit-backdrop-filter: blur(var(--noyo-dashboard-liquid-blur, 10px)) saturate(175%) brightness(var(--noyo-dashboard-liquid-backdrop-brightness, 1.05));
  border: 1px solid var(--noyo-dashboard-liquid-edge, var(--border-color));
  box-shadow: var(--card-shadow);
  transition: transform var(--noyo-duration-standard, 0.25s) var(--noyo-ease-standard, cubic-bezier(0.16, 1, 0.3, 1)),
              box-shadow var(--noyo-duration-standard, 0.25s) var(--noyo-ease-standard, cubic-bezier(0.16, 1, 0.3, 1)),
              border-color var(--noyo-duration-standard, 0.25s) var(--noyo-ease-standard, cubic-bezier(0.16, 1, 0.3, 1));
  display: flex;
  flex-direction: column;
}

/* 顶部白金镜面反光导光条 */
.liquid-glass-card::before {
  content: '';
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  height: 1px;
  background: linear-gradient(
    90deg,
    rgba(255, 255, 255, 0) 0%,
    var(--noyo-dashboard-liquid-highlight, rgba(255, 255, 255, 0.45)) 35%,
    var(--noyo-dashboard-liquid-highlight, rgba(255, 255, 255, 0.65)) 50%,
    var(--noyo-dashboard-liquid-highlight, rgba(255, 255, 255, 0.45)) 65%,
    rgba(255, 255, 255, 0) 100%
  );
  pointer-events: none;
  z-index: 1;
}

.liquid-glass-card--interactive:hover {
  border-color: rgba(147, 197, 253, 0.7);
  box-shadow: var(--shadow-floating, 0 12px 32px rgba(15, 23, 42, 0.12));
}

.liquid-glass-card--table {
  flex: 1;
  min-height: 0;
}

.liquid-glass-card--table > .liquid-glass-card__body {
  padding: 0;
  display: flex;
  flex-direction: column;
  height: 100%;
  overflow: hidden;
}

.liquid-glass-card--compact {
  border-radius: var(--radius-control, 12px);
  padding: 0.5rem 0.85rem;
}

.liquid-glass-card--kpi {
  border-radius: var(--radius-card, 14px);
  padding: 0.85rem 1rem;
}

.liquid-glass-card__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 14px 18px;
  border-bottom: 1px solid var(--border-color);
  background: transparent;
  flex-shrink: 0;
}

.liquid-glass-card__title-wrap {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.liquid-glass-card__title {
  margin: 0;
  font-size: 1rem;
  font-weight: 600;
  color: var(--text-main);
}

.liquid-glass-card__subtitle {
  font-size: 0.8rem;
  color: var(--text-secondary);
}

.liquid-glass-card__icon {
  font-size: 1.1rem;
  color: var(--color-brand);
}

.liquid-glass-card__body {
  flex: 1;
  min-height: 0;
  position: relative;
}

.liquid-glass-card__footer {
  padding: 10px 18px;
  border-top: 1px solid var(--border-color);
  background: transparent;
  flex-shrink: 0;
}
</style>
