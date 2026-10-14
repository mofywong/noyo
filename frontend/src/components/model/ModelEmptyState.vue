<template>
  <!-- 空状态：复用全局 .empty-state 结构（§8.6） -->
  <div class="empty-state model-empty-state" :class="{ 'model-empty-state--inline': inline }" role="status">
    <i :class="['bi', icon]" aria-hidden="true"></i>
    <p>{{ message }}</p>
    <slot>
      <LiquidGlassButton
        v-if="actionLabel"
        :variant="actionVariant"
        size="sm"
        :icon="actionIcon"
        @click="$emit('action')"
      >{{ actionLabel }}</LiquidGlassButton>
    </slot>
  </div>
</template>

<script setup>
import LiquidGlassButton from '../liquid-glass/LiquidGlassButton.vue';

defineProps({
  icon: { type: String, default: 'bi-inbox' },
  message: { type: String, required: true },
  actionLabel: { type: String, default: '' },
  actionIcon: { type: String, default: '' },
  actionVariant: { type: String, default: 'outline-primary' },
  /** 内嵌在卡片/表格内时收紧留白 */
  inline: { type: Boolean, default: false },
});

defineEmits(['action']);
</script>

<style scoped>
/* 全局 .empty-state 的 padding 为 96px，内嵌场景需要收紧。
   注意 .empty-state 的 `> i` / `> p` 是全局选择器，scoped 下必须用 :deep() 才能改到。 */
.model-empty-state--inline { padding: 40px 24px; }
.model-empty-state--inline :deep(> i) { font-size: 40px; margin-bottom: 16px; }
</style>
