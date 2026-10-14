<template>
  <!-- 骨架屏：复用全局 .skeleton 类（§8.7：--bg-hover 底 + 1.2s 高光扫过） -->
  <div class="model-skeleton" role="status" :aria-label="label">
    <!-- 能力卡网格 / Capability card grid -->
    <template v-if="variant === 'cards'">
      <div class="model-skeleton__grid">
        <div v-for="index in count" :key="index" class="skeleton model-skeleton__card" />
      </div>
    </template>

    <!-- 表格 / Table -->
    <template v-else-if="variant === 'table'">
      <div class="model-skeleton__table">
        <div class="skeleton model-skeleton__thead" />
        <div v-for="index in count" :key="index" class="skeleton model-skeleton__row" />
      </div>
    </template>

    <!-- 两栏（发布包选择器）/ Two-column (release picker) -->
    <template v-else-if="variant === 'split'">
      <div class="model-skeleton__split">
        <div class="skeleton model-skeleton__panel" />
        <div class="skeleton model-skeleton__panel model-skeleton__panel--tall" />
      </div>
    </template>

    <!-- 单行文本 / Single text line -->
    <template v-else>
      <div class="skeleton model-skeleton__line" />
    </template>

    <span class="model-skeleton__sr">{{ label }}</span>
  </div>
</template>

<script setup>
defineProps({
  /** cards | table | split | line */
  variant: { type: String, default: 'cards' },
  count: { type: Number, default: 6 },
  label: { type: String, default: '' },
});
</script>

<style scoped>
.model-skeleton { display: block; min-width: 0; }

.model-skeleton__grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(min(320px, 100%), 1fr)); gap: 16px; }
.model-skeleton__card { height: 148px; border-radius: var(--radius-card); }

.model-skeleton__table { display: flex; flex-direction: column; gap: 1px; border: 1px solid var(--border-color); border-radius: var(--radius-card); overflow: hidden; }
.model-skeleton__thead { height: 42px; border-radius: 0; }
.model-skeleton__row { height: 56px; border-radius: 0; }

.model-skeleton__split { display: grid; grid-template-columns: minmax(0, 1fr) minmax(0, 1.15fr); gap: 16px; }
.model-skeleton__panel { height: 220px; border-radius: var(--radius-card); }
.model-skeleton__panel--tall { height: 420px; }

.model-skeleton__line { height: 20px; border-radius: var(--radius-control); }

/* 文字标签仅给读屏器 / Screen-reader-only label */
.model-skeleton__sr { position: absolute; width: 1px; height: 1px; padding: 0; margin: -1px; overflow: hidden; clip: rect(0, 0, 0, 0); white-space: nowrap; border: 0; }

@media (max-width: 1100px) {
  .model-skeleton__split { grid-template-columns: minmax(0, 1fr); }
  .model-skeleton__panel--tall { height: 260px; }
}
</style>
