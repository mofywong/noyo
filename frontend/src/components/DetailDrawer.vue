<template>
  <Transition name="drawer-fade">
    <div v-if="visible" class="noyo-drawer-overlay" @click.self="$emit('close')" @keydown.esc="$emit('close')">
      <div class="noyo-drawer" role="dialog" aria-modal="true" tabindex="-1" :aria-label="title">
        <div class="noyo-drawer-header">
          <div class="d-flex align-items-center gap-2 min-w-0">
            <slot name="header-badge"></slot>
            <h5 class="mb-0 text-truncate">{{ title }}</h5>
          </div>
          <button type="button" class="btn-close" @click="$emit('close')" :aria-label="$t('common_close', '关闭 Close')"></button>
        </div>
        <div class="noyo-drawer-body">
          <slot></slot>
        </div>
        <div v-if="$slots.footer" class="noyo-drawer-footer">
          <slot name="footer"></slot>
        </div>
      </div>
    </div>
  </Transition>
</template>

<script setup>
import { watch, nextTick } from 'vue';

const props = defineProps({
  visible: Boolean,
  title: String
});

const emit = defineEmits(['close']);

// 焦点管理（§12：打开时聚焦抽屉，Esc 关闭）
watch(() => props.visible, async (v) => {
  if (v) {
    await nextTick();
    document.querySelector('.noyo-drawer')?.focus?.();
  } else {
    document.activeElement?.blur?.();
  }
});
</script>
