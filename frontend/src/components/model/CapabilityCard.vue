<template>
  <article class="model-scene-card" role="listitem" :aria-label="cardLabel">
    <div class="model-scene-card__head">
      <span class="model-scene-card__icon" aria-hidden="true"><i :class="['bi', capability.icon || 'bi-cpu']"></i></span>
      <span class="model-scene-card__status" :class="{ 'model-scene-card__status--available': available }">{{ statusText }}</span>
    </div>
    <h3>{{ title }}</h3>
    <p>{{ description }}</p>
  </article>
</template>

<script setup>
import { computed } from 'vue';
import { useI18n } from 'vue-i18n';
import { modelCenterCopy, modelSceneAvailable, modelText } from '../../utils/modelCenter.js';
const props = defineProps({ capability: { type: Object, required: true } });
const { locale } = useI18n();
const copy = computed(() => modelCenterCopy(locale.value));
const title = computed(() => modelText(props.capability.title, locale.value, props.capability.id));
const description = computed(() => modelText(props.capability.description, locale.value));
const available = computed(() => modelSceneAvailable(props.capability));
const statusText = computed(() => available.value ? copy.value.sceneAvailable : copy.value.sceneUnavailable);
const cardLabel = computed(() => copy.value.a11yCapabilityCard.replace('{name}', title.value).replace('{status}', statusText.value));
</script>

<style scoped>
.model-scene-card { padding: 24px; border: 1px solid var(--border-color); border-radius: var(--radius-card); background: var(--bg-surface); min-width: 0; }
.model-scene-card__head { display: flex; align-items: flex-start; justify-content: space-between; flex-wrap: wrap; gap: 16px; }
.model-scene-card__icon { display: grid; place-items: center; width: 40px; height: 40px; flex: 0 0 40px; border-radius: 8px; background: var(--color-brand-subtle); color: var(--color-brand); font-size: 20px; }
h3 { margin: 16px 0 0; font-size: 1.25rem; font-weight: 700; color: var(--text-primary); overflow-wrap: anywhere; }
p { margin: 8px 0 0; color: var(--text-secondary); font-size: 0.875rem; line-height: 1.6; }
.model-scene-card__status { padding: 0 8px; border-radius: var(--radius-control); color: var(--text-secondary); background: var(--bg-elevated); font-size: 0.8125rem; }
.model-scene-card__status--available { color: var(--color-success); background: color-mix(in srgb, var(--color-success) 12%, var(--bg-surface)); }
</style>
