<template>
  <Transition name="lg-panel">
    <GlassGroup
      v-if="open"
      profile="floating"
      shape="panel"
      class="lg-panel"
      role="dialog"
      aria-modal="true"
      :aria-label="title || undefined"
    >
      <header v-if="title || closable" class="lg-panel__header">
        <h3 v-if="title" class="lg-panel__title">{{ title }}</h3>
        <button
          v-if="closable"
          type="button"
          class="lg-btn lg-panel__close"
          :aria-label="resolvedCloseLabel"
          @click="onClose"
        >
          <i class="bi bi-x-lg" aria-hidden="true"></i>
        </button>
      </header>
      <SolidSurface density="compact" elevation="flat" class="lg-panel__surface">
        <div class="lg-panel__body">
          <slot />
        </div>
        <footer v-if="$slots.footer" class="lg-panel__footer">
          <slot name="footer" />
        </footer>
      </SolidSurface>
    </GlassGroup>
  </Transition>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import GlassGroup from './GlassGroup.vue'
import SolidSurface from './SolidSurface.vue'

interface LiquidGlassPanelProps {
  open?: boolean
  title?: string
  closable?: boolean
  closeLabel?: string
}

const props = withDefaults(defineProps<LiquidGlassPanelProps>(), {
  open: false,
  title: '',
  closable: true,
  closeLabel: '',
})

const emit = defineEmits<{
  (event: 'update:open', value: boolean): void
  (event: 'close'): void
}>()

const { t } = useI18n()
const resolvedCloseLabel = computed(
  () => props.closeLabel || t('common_close', 'Close'),
)

const onClose = () => {
  emit('update:open', false)
  emit('close')
}

const onKeydown = (event: KeyboardEvent) => {
  if (event.key === 'Escape' && props.open) onClose()
}

onMounted(() => window.addEventListener('keydown', onKeydown))
onBeforeUnmount(() => window.removeEventListener('keydown', onKeydown))
</script>

<style scoped>
.lg-panel__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 16px 24px;
}

.lg-panel__title {
  margin: 0;
  color: var(--text-main);
  font-size: 1rem;
  font-weight: 600;
}

.lg-panel__close {
  min-width: 40px;
  padding-inline: 12px;
}

.lg-panel__surface {
  margin: 0 8px 8px;
}

.lg-panel__body {
  max-height: min(60vh, 480px);
  overflow-y: auto;
}

.lg-panel__footer {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  padding-top: 16px;
  border-top: 1px solid var(--noyo-solid-border);
}
</style>
