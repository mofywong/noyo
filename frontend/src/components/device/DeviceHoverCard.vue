<template>
  <div class="device-hover-card" :class="`device-hover-card--${statusTone}`">
    <span class="device-hover-card__accent" aria-hidden="true"></span>

    <!-- 顶部：设备身份栏 -->
    <header class="device-hover-card__head">
      <span class="device-hover-card__icon" aria-hidden="true">
        <i :class="isCamera ? 'bi bi-camera-video' : 'bi bi-hdd-network'"></i>
      </span>
      <div class="device-hover-card__identity">
        <p class="device-hover-card__name" :title="device?.name || device?.code">{{ device?.name || device?.code }}</p>
        <p class="device-hover-card__code font-mono" :title="device?.code">{{ device?.code }}</p>
      </div>
      <span class="dash-pill" :class="device?.online ? 'dash-pill--success' : 'dash-pill--neutral'">
        <span class="dash-pill-dot"></span>
        {{ device?.online ? $t('dev_online') : $t('dev_offline') }}
      </span>
    </header>

    <!-- 核心 Hero 区域：设备属性实时数据看板 -->
    <section class="device-hover-card__section device-hover-card__hero">
      <div class="device-hover-card__section-header">
        <h6 class="device-hover-card__section-title">
          <span class="device-hover-card__pulse-dot" :class="{ 'device-hover-card__pulse-dot--active': device?.online }"></span>
          {{ $t('dev_hover_realtime_data') }}
          <span v-if="realtimeProperties?.length" class="device-hover-card__metric-count font-mono">
            ({{ realtimeProperties.length }})
          </span>
        </h6>
        <span v-if="updatedAtText" class="device-hover-card__freshness font-mono" :title="updatedAtText">
          {{ updatedAtText }}
        </span>
      </div>

      <!-- 加载态骨架 -->
      <div v-if="realtimeLoading" class="device-hover-card__metric-grid">
        <div v-for="n in 4" :key="n" class="device-hover-card__metric-tile skeleton" style="height: 52px;"></div>
      </div>

      <!-- 实时属性指标网格（支持多属性平滑滚动） -->
      <div v-else-if="realtimeProperties?.length > 0" class="device-hover-card__metric-scroll">
        <div class="device-hover-card__metric-grid">
          <div
            v-for="prop in realtimeProperties"
            :key="prop.key"
            class="device-hover-card__metric-tile"
            :class="{ 'device-hover-card__metric-tile--empty': !prop.hasValue }"
          >
            <div class="device-hover-card__metric-header">
              <span class="device-hover-card__metric-name" :title="prop.name">{{ prop.name }}</span>
            </div>
            <div class="device-hover-card__metric-body">
              <span
                class="device-hover-card__metric-val font-mono"
                :class="{ 'device-hover-card__metric-val--live': prop.hasValue && device?.online, 'text-muted': !prop.hasValue }"
              >
                {{ formatPropertyValue(prop.value) }}
              </span>
              <span v-if="prop.unit" class="device-hover-card__metric-unit">{{ prop.unit }}</span>
            </div>
          </div>
        </div>
      </div>

      <!-- 暂无数据空状态 -->
      <div v-else class="device-hover-card__metric-empty">
        <i class="bi bi-activity me-1" aria-hidden="true"></i>
        <span>{{ $t('dev_hover_no_data') }}</span>
      </div>
    </section>

    <!-- 次级元数据收敛栏（基本信息） -->
    <section class="device-hover-card__section device-hover-card__meta-section">
      <div class="device-hover-card__meta-grid">
        <div class="device-hover-card__meta-item">
          <span class="device-hover-card__meta-label">{{ $t('dev_product') }}</span>
          <span class="device-hover-card__meta-val" :title="productName">{{ productName || '-' }}</span>
        </div>
        <div class="device-hover-card__meta-item">
          <span class="device-hover-card__meta-label">{{ $t('dev_hover_driver') }}</span>
          <span class="device-hover-card__meta-val" :title="driverName || device?.protocol_name">
            {{ driverName || device?.protocol_name || '-' }}
          </span>
        </div>
        <div v-if="device?.project_name" class="device-hover-card__meta-item">
          <span class="device-hover-card__meta-label">{{ $t('project_name') }}</span>
          <span class="device-hover-card__meta-val" :title="device.project_name">{{ device.project_name }}</span>
        </div>
        <div v-if="parentName" class="device-hover-card__meta-item">
          <span class="device-hover-card__meta-label">{{ $t('dev_parent') }}</span>
          <span class="device-hover-card__meta-val" :title="parentName">{{ parentName }}</span>
        </div>
      </div>
    </section>

    <!-- 标签 -->
    <section v-if="device?.tags?.length" class="device-hover-card__section device-hover-card__tags-section">
      <div class="device-hover-card__tags">
        <span v-for="tag in device.tags.slice(0, 4)" :key="tag.ID" class="device-hover-card__tag" :style="tagStyle(tag)">
          <i :class="tag.icon || 'bi-tag'" aria-hidden="true"></i>
          <span class="device-hover-card__tag-name">{{ tag.name }}</span>
        </span>
        <span v-if="device.tags.length > 4" class="device-hover-card__tag-more font-mono">
          +{{ device.tags.length - 4 }}
        </span>
      </div>
    </section>

    <!-- 页脚：AI健康度 + 最近活跃 + 行详情引导 -->
    <footer class="device-hover-card__foot">
      <div class="device-hover-card__foot-row">
        <span class="device-hover-card__health" :class="`device-hover-card__health--${health.tone}`">
          <i :class="health.icon" aria-hidden="true"></i>
          <span v-if="health.value" class="device-hover-card__health-value font-mono">{{ health.value }}</span>
          <span class="device-hover-card__health-label">{{ health.label }}</span>
        </span>
        <span v-if="lastActiveText" class="device-hover-card__meta-time font-mono">{{ lastActiveText }}</span>
      </div>
      <p class="device-hover-card__hint">
        <i class="bi bi-box-arrow-up-right" aria-hidden="true"></i>
        {{ $t('dev_hover_hint') }}
      </p>
    </footer>
  </div>
</template>

<script setup>
import { computed } from 'vue';
import { useI18n } from 'vue-i18n';

/**
 * 设备属性悬浮卡片（Hero 实时遥测重构版）。
 *
 * 核心设计：
 * 1. 强化设备实时属性遥测（Hero Section），突出大号数值、单位与动态高亮。
 * 2. 压缩次要静态配置与技术参数至底部紧凑栏。
 * 3. 严格使用 CSS 变量令牌，支持 Light/Dark 双主题自适应。
 */
const props = defineProps({
  /** 设备对象（含 code/name/product_code/online/tags/config 等字段） */
  device: { type: Object, default: null },
  /** 已解析的产品名称 */
  productName: { type: String, default: '' },
  /** 已解析的接入驱动名称 */
  driverName: { type: String, default: '' },
  /** 已解析的父设备名称 */
  parentName: { type: String, default: '' },
  /** 是否已配置 AI 设备守护 */
  aiConfigured: { type: Boolean, default: false },
  /** 已格式化的「最近活跃」文本 */
  lastActiveText: { type: String, default: '' },
  /** 实时遥测属性列表 Array<{ key, name, value, unit, type, hasValue }> */
  realtimeProperties: { type: Array, default: () => [] },
  /** 实时数据加载状态 */
  realtimeLoading: { type: Boolean, default: false },
  /** 数据更新时间或新鲜度提示 */
  updatedAtText: { type: String, default: '' },
});

const { t } = useI18n();

/** 状态语义：未启用优先于离线，避免禁用设备被误读为故障 */
const statusTone = computed(() => {
  if (!props.device?.enabled) return 'disabled';
  return props.device?.online ? 'online' : 'offline';
});

/** 是否为视频类设备（GB28181/Camera） */
const isCamera = computed(() => {
  const code = (props.device?.protocol_profile_code || '').toLowerCase();
  const proto = (props.device?.protocol_name || '').toLowerCase();
  return code.includes('gb28181') || code.includes('camera') || proto.includes('gb28181');
});

/** 格式化属性数值 */
const formatPropertyValue = (val) => {
  if (val === undefined || val === null || val === '') return '--';
  if (typeof val === 'boolean') return val ? 'TRUE' : 'FALSE';
  if (typeof val === 'number') {
    if (Number.isInteger(val)) return String(val);
    return Number(val.toFixed(2));
  }
  if (typeof val === 'object') {
    try {
      return JSON.stringify(val);
    } catch {
      return String(val);
    }
  }
  return String(val);
};

const health = computed(() => {
  const device = props.device || {};

  if (device.ai_latched) {
    return {
      tone: 'danger',
      icon: 'bi bi-shield-fill-x',
      value: device.ai_health_trigger != null ? Number(device.ai_health_trigger).toFixed(1) : '',
      label: t('ai_latched'),
    };
  }

  const score = device.ai_health_score;
  if (device.online && score !== undefined && score !== null) {
    const tone = score > 80 ? 'success' : score > 60 ? 'warning' : 'danger';
    return { tone, icon: 'bi bi-shield-fill-check', value: Number(score).toFixed(1), label: t('ai_health') };
  }

  if (!device.online && (device.ai_health_details || props.aiConfigured)) {
    return { tone: 'muted', icon: 'bi bi-shield-slash', value: '', label: t('ai_offline') };
  }

  if (props.aiConfigured) {
    return { tone: 'primary', icon: 'bi bi-shield-check', value: '', label: t('ai_generating') };
  }

  return { tone: 'muted', icon: 'bi bi-shield', value: '', label: t('ai_not_configured') };
});

const hexToRgb = (color) => {
  if (!/^#([0-9a-f]{3}|[0-9a-f]{6})$/i.test(color || '')) return null;
  let hex = color.slice(1);
  if (hex.length === 3) {
    hex = hex.split('').map((char) => char + char).join('');
  }
  const value = Number.parseInt(hex, 16);
  return { r: (value >> 16) & 0xff, g: (value >> 8) & 0xff, b: value & 0xff };
};

/** 标签着色沿用列表页的 `--tag-color` / `--tag-rgb` 约定 */
const tagStyle = (tag) => {
  const color = tag?.color || 'var(--color-brand)';
  const rgb = hexToRgb(tag?.color) || { r: 59, g: 130, b: 246 };
  return {
    '--tag-color': color,
    '--tag-rgb': `${rgb.r}, ${rgb.g}, ${rgb.b}`,
  };
};
</script>

<style scoped>
.device-hover-card {
  --device-hover-tone: var(--color-brand);

  position: relative;
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 14px 16px;
  overflow: hidden;
  color: var(--text-main);
  font-size: 0.8125rem;
  line-height: 1.4;
  pointer-events: auto;
}

.device-hover-card--online {
  --device-hover-tone: var(--color-success);
}

.device-hover-card--offline {
  --device-hover-tone: var(--color-danger);
}

.device-hover-card--disabled {
  --device-hover-tone: var(--text-muted);
}

.device-hover-card__accent {
  position: absolute;
  inset: 0 0 auto 0;
  height: 2px;
  background: linear-gradient(
    90deg,
    var(--device-hover-tone) 0%,
    color-mix(in srgb, var(--device-hover-tone) 35%, transparent) 58%,
    transparent 100%
  );
}

/* ── 头部：图标 + 名称/编码 + 状态胶囊 ───────────────────────────── */

.device-hover-card__head {
  display: flex;
  align-items: flex-start;
  gap: 10px;
}

.device-hover-card__icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  width: 32px;
  height: 32px;
  border: 1px solid color-mix(in srgb, var(--device-hover-tone) 30%, transparent);
  border-radius: var(--noyo-radius-control, 8px);
  background: color-mix(in srgb, var(--device-hover-tone) 12%, transparent);
  color: var(--device-hover-tone);
  font-size: 0.95rem;
}

.device-hover-card__identity {
  flex: 1 1 auto;
  min-width: 0;
}

.device-hover-card__name {
  margin: 0;
  overflow: hidden;
  color: var(--text-main);
  font-size: 0.875rem;
  font-weight: 600;
  letter-spacing: -0.01em;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.device-hover-card__code {
  margin: 1px 0 0;
  overflow: hidden;
  color: var(--text-muted);
  font-size: 0.6875rem;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.device-hover-card__head .dash-pill {
  flex-shrink: 0;
  margin-top: 1px;
}

/* ── 分组结构 ───────────────────────────────────────────────────── */

.device-hover-card__section {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.device-hover-card__section-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.device-hover-card__section-title {
  display: flex;
  align-items: center;
  gap: 6px;
  margin: 0;
  color: var(--text-secondary);
  font-size: 0.75rem;
  font-weight: 600;
  letter-spacing: 0.02em;
}

/* 脉冲呼吸灯指示器 */
.device-hover-card__pulse-dot {
  display: inline-block;
  flex-shrink: 0;
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--text-muted);
  opacity: 0.6;
}

.device-hover-card__pulse-dot--active {
  background: var(--color-success);
  opacity: 1;
  box-shadow: 0 0 0 0 rgba(34, 197, 94, 0.7);
  animation: hover-pulse 2s infinite cubic-bezier(0.4, 0, 0.6, 1);
}

@keyframes hover-pulse {
  0% {
    box-shadow: 0 0 0 0 rgba(34, 197, 94, 0.6);
  }
  70% {
    box-shadow: 0 0 0 6px rgba(34, 197, 94, 0);
  }
  100% {
    box-shadow: 0 0 0 0 rgba(34, 197, 94, 0);
  }
}

.device-hover-card__metric-count {
  font-size: 0.6875rem;
  color: var(--text-muted);
  font-weight: 400;
}

.device-hover-card__freshness {
  color: var(--text-muted);
  font-size: 0.625rem;
}

/* ── Hero 核心区：实时指标网格与滚动容器 ──────────────────────────── */

.device-hover-card__metric-scroll {
  max-height: 230px;
  overflow-y: auto;
  overflow-x: hidden;
  padding-right: 3px;
  margin-right: -3px;
  scrollbar-width: thin;
  scrollbar-color: var(--border-color) transparent;
  overscroll-behavior: contain;
}

.device-hover-card__metric-scroll::-webkit-scrollbar {
  width: 4px;
}

.device-hover-card__metric-scroll::-webkit-scrollbar-track {
  background: transparent;
}

.device-hover-card__metric-scroll::-webkit-scrollbar-thumb {
  background: var(--border-color);
  border-radius: 4px;
}

.device-hover-card__metric-scroll::-webkit-scrollbar-thumb:hover {
  background: var(--text-muted);
}

.device-hover-card__metric-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 6px;
}

.device-hover-card__metric-tile {
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  gap: 4px;
  min-width: 0;
  padding: 6px 8px;
  border: 1px solid var(--border-color);
  border-radius: var(--noyo-radius-control, 8px);
  background: var(--bg-input);
  transition: border-color 0.15s ease, background-color 0.15s ease;
}

.device-hover-card__metric-tile--empty {
  opacity: 0.75;
}

.device-hover-card__metric-header {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 4px;
}

.device-hover-card__metric-name {
  overflow: hidden;
  color: var(--text-secondary);
  font-size: 0.72rem;
  font-weight: 500;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.device-hover-card__metric-body {
  display: flex;
  align-items: baseline;
  gap: 3px;
  overflow: hidden;
}

.device-hover-card__metric-val {
  overflow: hidden;
  color: var(--text-main);
  font-size: 1.0625rem;
  font-weight: 700;
  line-height: 1.2;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.device-hover-card__metric-val--live {
  color: var(--color-brand);
}

.device-hover-card__metric-unit {
  flex-shrink: 0;
  color: var(--text-muted);
  font-size: 0.6875rem;
  font-weight: 400;
}

.device-hover-card__metric-empty {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 14px 10px;
  border: 1px dashed var(--border-color);
  border-radius: var(--noyo-radius-control, 8px);
  background: var(--bg-input);
  color: var(--text-muted);
  font-size: 0.75rem;
}

/* ── 次级元数据栏（基本信息） ───────────────────────────────────── */

.device-hover-card__meta-section {
  padding-top: 6px;
  border-top: 1px solid var(--border-color);
}

.device-hover-card__meta-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 4px 8px;
}

.device-hover-card__meta-item {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 6px;
  overflow: hidden;
}

.device-hover-card__meta-label {
  flex-shrink: 0;
  color: var(--text-muted);
  font-size: 0.6875rem;
}

.device-hover-card__meta-val {
  overflow: hidden;
  color: var(--text-secondary);
  font-size: 0.72rem;
  font-weight: 500;
  text-align: right;
  text-overflow: ellipsis;
  white-space: nowrap;
}

/* ── 标签 ───────────────────────────────────────────────────────── */

.device-hover-card__tags {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 4px;
}

.device-hover-card__tag {
  --tag-color: var(--color-brand);
  --tag-rgb: 59, 130, 246;
  display: inline-flex;
  align-items: center;
  gap: 3px;
  max-width: 100%;
  padding: 1px 6px;
  border: 1px solid rgba(var(--tag-rgb), 0.15);
  border-left-width: 2px;
  border-left-color: rgb(var(--tag-rgb));
  border-radius: 3px 4px 4px 3px;
  background: linear-gradient(135deg, rgba(var(--tag-rgb), 0.1) 0%, rgba(var(--tag-rgb), 0.04) 100%);
  color: rgb(var(--tag-rgb));
  font-size: 0.6875rem;
  font-weight: 500;
  line-height: 1.5;
}

.device-hover-card__tag-name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.device-hover-card__tag-more {
  color: var(--text-muted);
  font-size: 0.65rem;
}

/* ── 页脚：健康度 + 引导 ────────────────────────────────────────── */

.device-hover-card__foot {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding-top: 8px;
  border-top: 1px solid var(--border-color);
}

.device-hover-card__foot-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.device-hover-card__health {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  min-width: 0;
  font-size: 0.72rem;
  font-weight: 600;
}

.device-hover-card__health--success {
  color: var(--color-success);
}

.device-hover-card__health--warning {
  color: var(--color-warning);
}

.device-hover-card__health--danger {
  color: var(--color-danger);
}

.device-hover-card__health--primary {
  color: var(--color-brand);
}

.device-hover-card__health--muted {
  color: var(--text-muted);
}

.device-hover-card__health-label {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.device-hover-card__meta-time {
  flex-shrink: 0;
  color: var(--text-muted);
  font-size: 0.65rem;
}

.device-hover-card__hint {
  display: flex;
  align-items: center;
  gap: 4px;
  margin: 0;
  color: var(--text-muted);
  font-size: 0.65rem;
}
</style>
