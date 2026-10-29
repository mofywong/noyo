<template>
  <div class="noyo-tabs">
    <div ref="tablist" class="noyo-tabs__list" role="tablist" :aria-label="label" aria-orientation="horizontal">
      <button v-for="item in items" :id="item.id || `${baseId}-${item.value}-tab`" :key="item.value" type="button" role="tab" class="noyo-tabs__tab" :aria-selected="modelValue === item.value" :aria-controls="item.panelId" :tabindex="focusValue === item.value ? 0 : -1" :disabled="disabled || item.disabled" @click="select(item)" @keydown="onKeydown($event, item.value)">
        <i v-if="item.icon" :class="item.icon" aria-hidden="true"></i>
        <span>{{ item.label }}</span>
        <span v-if="item.count !== undefined && item.count !== null" class="noyo-tabs__count">{{ item.count }}</span>
      </button>
    </div>
    <div v-if="$slots.actions" class="noyo-tabs__actions"><slot name="actions" /></div>
  </div>
</template>

<script setup>
import { computed, onMounted, ref, useId, watch } from 'vue';

const props = defineProps({
  modelValue: { type: [String, Number], required: true },
  items: { type: Array, required: true },
  label: { type: String, required: true },
  id: { type: String, default: '' },
  disabled: { type: Boolean, default: false },
});
const emit = defineEmits(['update:modelValue']);
const generatedId = useId();
const baseId = computed(() => props.id || `tabs-${generatedId}`);
const tablist = ref(null);
const enabledItems = computed(() => props.disabled ? [] : props.items.filter(item => !item.disabled));
const focusValue = computed(() => enabledItems.value.some(item => item.value === props.modelValue) ? props.modelValue : enabledItems.value[0]?.value);

function revealActiveTab() {
  const list = tablist.value;
  const button = list?.querySelector('[aria-selected="true"]');
  if (!button) return;
  const container = list.getBoundingClientRect(), active = button.getBoundingClientRect();
  const delta = active.left < container.left ? active.left - container.left : active.right > container.right ? active.right - container.right : 0;
  if (delta) list.scrollTo({ left: list.scrollLeft + delta, behavior: 'auto' });
}
onMounted(revealActiveTab);
watch(() => [props.modelValue, props.items], revealActiveTab, { flush: 'post' });

function select(item) {
  if (!props.disabled && !item.disabled && item.value !== props.modelValue) emit('update:modelValue', item.value);
}
function onKeydown(event, value) {
  if (!['ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(event.key) || !enabledItems.value.length) return;
  event.preventDefault();
  const items = enabledItems.value;
  const index = items.findIndex(item => item.value === value);
  const next = event.key === 'Home' ? items[0] : event.key === 'End' ? items.at(-1) : items[(index + (event.key === 'ArrowRight' ? 1 : items.length - 1)) % items.length];
  const buttonIndex = props.items.findIndex(item => item.value === next.value);
  const button = tablist.value?.querySelectorAll('[role="tab"]')[buttonIndex];
  button?.focus({ preventScroll: true });
  button?.scrollIntoView({ block: 'nearest', inline: 'nearest' });
  select(next);
}
</script>

<style scoped>
.noyo-tabs { display: flex; align-items: center; flex-wrap: wrap; gap: 8px 16px; min-width: 0; max-width: 100%; margin-bottom: 24px; flex-shrink: 0; border-bottom: 1px solid var(--border-color); }
.noyo-tabs__list { display: flex; flex: 1 1 0; gap: 8px; min-width: 0; overflow-x: auto; padding: 8px 0; overscroll-behavior-x: contain; scrollbar-width: thin; }
.noyo-tabs__tab { display: inline-flex; position: relative; flex-shrink: 0; align-items: center; justify-content: center; gap: 8px; min-height: 48px; margin: 0; padding: 8px 16px; border: 0; border-radius: 8px; background: var(--noyo-color-transparent); color: var(--text-secondary); font: inherit; font-weight: 500; white-space: nowrap; cursor: pointer; transition: color var(--noyo-duration-fast) var(--noyo-ease-standard), background-color var(--noyo-duration-fast) var(--noyo-ease-standard); }
.noyo-tabs__tab:hover:not(:disabled) { background: var(--bg-hover); color: var(--text-primary); }
.noyo-tabs__tab[aria-selected="true"] { background: var(--color-brand-subtle); color: var(--color-brand); font-weight: 600; }
.noyo-tabs__tab[aria-selected="true"]::after { content: ''; position: absolute; bottom: 0; right: 16px; left: 16px; height: 2px; border-radius: 8px; background: var(--color-brand); }
.noyo-tabs__tab:focus-visible { outline: 2px solid var(--color-brand); outline-offset: -2px; }
.noyo-tabs__tab:disabled { cursor: default; opacity: var(--disabled-opacity, 0.5); }
.noyo-tabs__count { min-width: 24px; padding: 0 8px; border-radius: 16px; background: var(--bg-hover); color: var(--text-secondary); font-size: 12px; line-height: 24px; font-variant-numeric: tabular-nums; }
.noyo-tabs__tab[aria-selected="true"] .noyo-tabs__count { color: var(--color-brand); }
.noyo-tabs__actions { display: flex; align-items: center; flex-wrap: wrap; gap: 8px; padding: 8px 0; }
@media (max-width: 640px) { .noyo-tabs__list { flex-basis: 100%; } .noyo-tabs__actions { margin-left: auto; } }
@media (prefers-reduced-motion: reduce) { .noyo-tabs__tab { transition: none; } }
</style>
