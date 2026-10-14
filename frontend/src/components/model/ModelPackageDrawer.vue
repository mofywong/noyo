<template>
  <DetailDrawer :visible="visible" :title="drawerTitle" @close="$emit('close')">
    <template #header-badge><i class="bi bi-layers model-pkg-drawer__badge" aria-hidden="true"></i></template>
    <p v-if="error" class="model-pkg-drawer__error" role="alert">{{ error }}</p>
    <ul v-if="row?.releases?.length" class="model-pkg-drawer__versions">
      <li v-for="release in row.releases" :key="release.version" class="model-pkg-drawer__version">
        <div class="model-pkg-drawer__head">
          <div><span class="model-pkg-drawer__label">{{ copy.versionNumber }}</span><h3>{{ release.version }}</h3></div>
          <span class="model-pkg-drawer__pill" :class="{ 'model-pkg-drawer__pill--active': release.active }">{{ release.active ? copy.activeVersion : release.installed ? copy.candidateVersion : copy.notInstalled }}</span>
        </div>
        <dl class="model-pkg-drawer__meta">
          <dt>{{ copy.versionDescription }}</dt><dd>{{ description(release) }}</dd>
          <dt>{{ copy.installedAt }}</dt><dd><time v-if="release.installedAt" :datetime="release.installedAt">{{ installedTime(release) }}</time><span v-else>{{ copy.unspecified }}</span></dd>
        </dl>
        <div v-if="catalog.canManage && release.installed" class="model-pkg-drawer__actions">
          <LiquidGlassButton v-if="canSwitch(release)" variant="outline-primary" size="sm" icon="bi bi-arrow-repeat" :disabled="busy" @click="activateRelease(release)">{{ copy.versionSwitch }}</LiquidGlassButton>
          <LiquidGlassButton variant="outline-secondary" size="sm" icon="bi bi-shield-check" :disabled="busy" @click="verifyRelease(release)">{{ copy.verify }}</LiquidGlassButton>
          <LiquidGlassButton v-if="canDelete(release)" variant="outline-danger" size="sm" icon="bi bi-trash" :disabled="busy" @click="deleteRelease(release)">{{ copy.deleteVersion }}</LiquidGlassButton>
        </div>
      </li>
    </ul>
    <p v-else class="model-pkg-drawer__empty">{{ copy.noVersions }}</p>
  </DetailDrawer>
</template>

<script setup>
import { computed } from 'vue';
import { useI18n } from 'vue-i18n';
import DetailDrawer from '../DetailDrawer.vue';
import LiquidGlassButton from '../liquid-glass/LiquidGlassButton.vue';
import { modelCenterCopy, modelPackageActionAllowed, modelText, modelTimestamp } from '../../utils/modelCenter.js';

const props = defineProps({
  visible: { type: Boolean, default: false },
  row: { type: Object, default: null },
  catalog: { type: Object, required: true },
  busy: { type: Boolean, default: false },
  error: { type: String, default: '' },
});
const emit = defineEmits(['close', 'activate', 'uninstall', 'verify']);
const { locale } = useI18n();
const copy = computed(() => modelCenterCopy(locale.value));
const drawerTitle = computed(() => `${modelText(props.row?.title, locale.value, props.row?.id || '')} · ${copy.value.viewVersions}`);
const description = release => modelText(release.description, locale.value, copy.value.noVersionDescription);
const installedTime = release => modelTimestamp(release.installedAt, locale.value);
const canSwitch = release => props.catalog.canManage && modelPackageActionAllowed(release, 'activate');
const canDelete = release => props.catalog.canManage && modelPackageActionAllowed(release, 'uninstall');
const activateRelease = release => { if (!props.busy && canSwitch(release)) emit('activate', release); };
const deleteRelease = release => { if (!props.busy && canDelete(release)) emit('uninstall', release); };
const verifyRelease = release => { if (!props.busy && props.catalog.canManage && modelPackageActionAllowed(release, 'verify')) emit('verify', release); };
</script>

<style scoped>
.model-pkg-drawer__badge { color: var(--color-brand); }
.model-pkg-drawer__versions { display: grid; gap: 16px; margin: 0; padding: 0; list-style: none; }
.model-pkg-drawer__version { padding: 24px; border: 1px solid var(--border-color); border-radius: var(--radius-card); background: var(--bg-surface); }
.model-pkg-drawer__head { display: flex; align-items: flex-start; justify-content: space-between; flex-wrap: wrap; gap: 16px; margin-bottom: 16px; }
.model-pkg-drawer__head h3 { margin: 8px 0 0; color: var(--text-primary); font-size: 20px; font-weight: 700; overflow-wrap: anywhere; }
.model-pkg-drawer__label, .model-pkg-drawer__meta dt { color: var(--text-secondary); font-size: 14px; }
.model-pkg-drawer__pill { padding: 8px 16px; border-radius: var(--radius-control); background: var(--bg-hover); color: var(--text-secondary); font-size: 12px; }
.model-pkg-drawer__pill--active { background: color-mix(in srgb, var(--color-success) 12%, var(--bg-surface)); color: var(--color-success); }
.model-pkg-drawer__meta { display: grid; grid-template-columns: minmax(96px, 1fr) minmax(0, 3fr); gap: 16px; margin: 0; font-size: 14px; line-height: 1.6; }
.model-pkg-drawer__meta dd { margin: 0; color: var(--text-primary); overflow-wrap: anywhere; white-space: pre-wrap; }
.model-pkg-drawer__actions { display: flex; flex-wrap: wrap; gap: 8px; margin-top: 24px; }
.model-pkg-drawer__empty { margin: 0; padding: 96px 24px; color: var(--text-secondary); text-align: center; }
.model-pkg-drawer__error { margin: 0 0 16px; padding: 16px; border-radius: var(--radius-control); background: color-mix(in srgb, var(--color-danger) 8%, var(--bg-surface)); color: var(--color-danger); }
@media (max-width: 480px) { .model-pkg-drawer__meta { grid-template-columns: 1fr; gap: 8px; } .model-pkg-drawer__meta dd + dt { margin-top: 8px; } }
</style>
