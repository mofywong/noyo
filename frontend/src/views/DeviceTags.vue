<template>
  <div class="device-tags-page page-fixed-height d-flex flex-column h-100">
    <div class="page-header">
      <div>
        <h1>{{ $t('sidebar_device_tags') }}</h1>
        <p class="page-subtitle">{{ $t('dev_tag_manage_subtitle', '管理设备业务分组与多维标签绑定') }}</p>
      </div>
      <LiquidGlassButton
        variant="primary"
        icon="bi bi-plus-lg"
        @click="openTagModal()"
        v-permission="'device_tag:create'"
      >
        {{ $t('dev_tag_create') }}
      </LiquidGlassButton>
    </div>

    <div class="device-tags-layout flex-grow-1 overflow-hidden">
      <aside class="tag-sidebar h-100">
        <div class="card border-0 shadow-sm table-glass-card h-100 d-flex flex-column">
          <div class="card-header bg-transparent border-0 py-3 d-flex justify-content-between align-items-center">
            <h6 class="mb-0 fw-bold">{{ $t('dev_tag_manage') }}</h6>
          </div>
          <div class="card-body p-0">
            <div v-if="loadingTags" class="text-center py-4 text-muted">{{ $t('loading') }}</div>
            <div v-else-if="tags.length === 0" class="text-center py-5 text-muted">
              <i class="bi bi-tags fs-1 d-block mb-2"></i>
              {{ $t('dev_no_tags') }}
            </div>
            <div v-else class="tag-list">
              <div
                v-for="tag in tags"
                :key="tag.ID"
                class="tag-list-item"
                :class="{ active: selectedTag?.ID === tag.ID }"
                role="button"
                tabindex="0"
                @click="selectTag(tag)"
                @keydown.enter="selectTag(tag)"
              >
                <span class="tag-color-dot" :style="{ backgroundColor: tag.color || 'var(--color-brand)' }"></span>
                <span class="tag-list-item__icon">
                  <i :class="tag.icon || 'bi-tag'"></i>
                </span>
                <span class="tag-list-item__text">
                  <span class="fw-semibold text-truncate d-block">{{ tag.name }}</span>
                  <span v-if="tag.description" class="d-block small opacity-75 text-truncate">{{ tag.description }}</span>
                </span>
                <span class="badge rounded-pill" :class="selectedTag?.ID === tag.ID ? 'bg-light text-primary' : 'bg-secondary-subtle text-secondary'">
                  {{ tag.device_count || 0 }}
                </span>
                <span class="table-actions" @click.stop>
                  <button class="table-action-btn table-action-btn--primary" @click="openTagModal(tag)" :title="$t('tsl_edit')" v-permission="'device_tag:edit'">
                    <i class="bi bi-pencil"></i>
                  </button>
                  <button class="table-action-btn table-action-btn--danger" @click="deleteTag(tag)" :title="$t('tsl_delete')" v-permission="'device_tag:delete'">
                    <i class="bi bi-trash"></i>
                  </button>
                </span>
              </div>
            </div>
          </div>
        </div>
      </aside>

      <section class="tag-workspace h-100">
        <div class="card border-0 shadow-sm table-glass-card h-100 d-flex flex-column">
          <div class="card-header bg-transparent border-0 py-3">
            <div class="d-flex justify-content-between align-items-center gap-3">
              <div class="min-w-0">
                <h5 class="mb-1 text-truncate">{{ selectedTag ? selectedTag.name : $t('dev_tag_assign_title') }}</h5>
                <div class="text-muted small">{{ $t('dev_tag_assign_hint') }}</div>
              </div>
              <LiquidGlassButton
                size="sm"
                class="flex-shrink-0"
                :variant="isDirty ? 'warning' : 'primary'"
                :disabled="!selectedTag || isSaving"
                :icon="isSaving ? 'bi-arrow-repeat spin' : 'bi-check2'"
                @click="saveAssignments"
                v-permission="'device_tag:edit'"
              >
                {{ $t('common_save') }}
              </LiquidGlassButton>
            </div>
            <div class="row g-2 mt-3">
              <div class="col-md-7">
                <input class="form-control form-control-sm" v-model="deviceSearch" :placeholder="$t('dev_tag_device_search')">
              </div>
              <div class="col-md-5">
                <select class="form-select form-select-sm" v-model="productFilter">
                  <option value="">{{ $t('dev_product') }}: {{ $t('all') }}</option>
                  <option v-for="code in productOptions" :key="code" :value="code">{{ productDisplay(code) }}</option>
                </select>
              </div>
            </div>
          </div>
          <div class="card-body p-0 d-flex flex-column flex-grow-1 overflow-hidden min-h-0">
            <div v-if="!selectedTag" class="h-100 d-flex align-items-center justify-content-center text-muted py-5">
              {{ $t('dev_tag_select_hint') }}
            </div>
            <div v-else class="d-flex flex-column flex-grow-1 overflow-hidden min-h-0">
              <div class="assignment-summary">
                <span class="d-inline-flex align-items-center gap-1"><i class="bi bi-link-45deg text-primary"></i>{{ $t('dev_tag_bound_devices') }} <strong class="text-primary">{{ assignedDeviceCodes.length }}</strong></span>
                <span class="d-inline-flex align-items-center gap-1"><i class="bi bi-link text-secondary"></i>{{ $t('dev_tag_unbound_devices') }} <strong>{{ unassignedDevices.length }}</strong></span>
              </div>

              <div class="assignment-board flex-grow-1 min-h-0">
                <section class="assignment-panel">
                  <div class="assignment-panel__header">
                    <div class="d-flex align-items-center gap-2">
                      <h6 class="mb-0 fw-semibold">{{ $t('dev_tag_bound_devices') }}</h6>
                      <span class="badge rounded-pill bg-primary-subtle text-primary border border-primary-subtle">{{ assignedDevices.length }}</span>
                    </div>
                    <button class="btn btn-outline-secondary btn-sm" :disabled="assignedVisibleDevices.length === 0" @click="clearVisibleDevices" v-permission="'device_tag:edit'">
                      <i class="bi bi-dash-circle me-1"></i>{{ $t('dev_tag_unbind_visible') }}
                    </button>
                  </div>
                  <div v-if="loadingDevices" class="assignment-empty">{{ $t('loading') }}</div>
                  <div v-else-if="assignedDevices.length === 0" class="assignment-empty">{{ $t('dev_tag_bound_empty') }}</div>
                  <div v-else class="device-list">
                    <article v-for="device in assignedVisibleDevices" :key="device.code" class="device-list-item">
                      <div class="device-list-item__main">
                        <div class="device-name" :title="device.name || '-'">{{ device.name || '-' }}</div>
                        <div class="device-code" :title="deviceDisplay(device)">{{ deviceDisplay(device) }}</div>
                        <div class="device-list-item__meta">
                          <span class="product-pill" :title="productDisplay(device.product_code)">{{ productDisplay(device.product_code) || '-' }}</span>
                          <span class="badge rounded-pill" :class="device.online ? 'bg-success' : 'bg-secondary'">
                            {{ device.online ? $t('dev_online') : $t('dev_offline') }}
                          </span>
                        </div>
                      </div>
                      <button class="btn btn-outline-danger btn-sm rounded-pill px-2 py-1" @click="unbindDevice(device)" v-permission="'device_tag:edit'">
                        <i class="bi bi-dash-lg me-1"></i>{{ $t('dev_tag_unbind') }}
                      </button>
                    </article>
                    <div v-if="assignedOverflow > 0" class="assignment-limit-note">
                      {{ $t('dev_tag_result_limited', { count: assignmentListLimit, total: assignedDevices.length }) }}
                    </div>
                  </div>
                </section>

                <section class="assignment-panel">
                  <div class="assignment-panel__header">
                    <div class="d-flex align-items-center gap-2">
                      <h6 class="mb-0 fw-semibold">{{ $t('dev_tag_unbound_devices') }}</h6>
                      <span class="badge rounded-pill bg-secondary-subtle text-secondary border border-secondary-subtle">{{ unassignedDevices.length }}</span>
                    </div>
                    <button class="btn btn-outline-primary btn-sm" :disabled="unassignedVisibleDevices.length === 0" @click="selectVisibleDevices" v-permission="'device_tag:edit'">
                      <i class="bi bi-plus-circle me-1"></i>{{ $t('dev_tag_bind_visible') }}
                    </button>
                  </div>
                  <div v-if="loadingDevices" class="assignment-empty">{{ $t('loading') }}</div>
                  <div v-else-if="unassignedDevices.length === 0" class="assignment-empty">{{ $t('dev_tag_unbound_empty') }}</div>
                  <div v-else class="device-list">
                    <article v-for="device in unassignedVisibleDevices" :key="device.code" class="device-list-item">
                      <div class="device-list-item__main">
                        <div class="device-name" :title="device.name || '-'">{{ device.name || '-' }}</div>
                        <div class="device-code" :title="deviceDisplay(device)">{{ deviceDisplay(device) }}</div>
                        <div class="device-list-item__meta">
                          <span class="product-pill" :title="productDisplay(device.product_code)">{{ productDisplay(device.product_code) || '-' }}</span>
                          <span class="badge rounded-pill" :class="device.online ? 'bg-success' : 'bg-secondary'">
                            {{ device.online ? $t('dev_online') : $t('dev_offline') }}
                          </span>
                        </div>
                      </div>
                      <button class="btn btn-outline-primary btn-sm rounded-pill px-2 py-1" @click="bindDevice(device)" v-permission="'device_tag:edit'">
                        <i class="bi bi-plus-lg me-1"></i>{{ $t('dev_tag_bind') }}
                      </button>
                    </article>
                    <div v-if="unassignedOverflow > 0" class="assignment-limit-note">
                      {{ $t('dev_tag_result_limited', { count: assignmentListLimit, total: unassignedDevices.length }) }}
                    </div>
                  </div>
                </section>
              </div>
            </div>
          </div>
        </div>
      </section>
    </div>

    <Teleport to="body">
      <div v-if="showTagModal" class="modal fade show d-block" style="background: rgba(0,0,0,0.5); z-index: 1060;">
        <div class="modal-dialog modal-dialog-centered">
          <div class="modal-content">
            <div class="modal-header">
              <h5 class="modal-title">{{ editingTag ? $t('dev_tag_edit') : $t('dev_tag_create') }}</h5>
            <button type="button" class="btn-close" @click="closeTagModal"></button>
          </div>
          <div class="modal-body">
            <div class="mb-3">
              <label class="form-label">{{ $t('dev_tag_name') }}</label>
              <input class="form-control" v-model.trim="tagForm.name" maxlength="64">
            </div>
            <div class="mb-3">
              <label class="form-label">{{ $t('dev_tag_color') }}</label>
              <div class="d-flex align-items-center gap-2 mb-2">
                <input class="form-control form-control-color" type="color" v-model="tagForm.color" style="width:42px;flex-shrink:0;padding:2px;">
                <span class="text-muted small font-monospace">{{ tagForm.color }}</span>
              </div>
              <div class="color-presets">
                <span v-for="c in PRESET_COLORS" :key="c" class="color-swatch"
                      :class="{ active: tagForm.color === c }"
                      :style="{ backgroundColor: c }"
                      @click="tagForm.color = c">
                  <i v-if="tagForm.color === c" class="bi bi-check"></i>
                </span>
              </div>
            </div>
            <div class="mb-3">
              <label class="form-label">{{ $t('dev_tag_icon') }}</label>
              <div class="icon-presets">
                <span
                  v-for="icon in PRESET_ICONS"
                  :key="icon"
                  class="icon-option"
                  :class="{ active: tagForm.icon === icon }"
                  :title="icon"
                  @click="tagForm.icon = icon"
                >
                  <i :class="icon"></i>
                  <i v-if="tagForm.icon === icon" class="bi bi-check-circle-fill icon-checked"></i>
                </span>
              </div>
            </div>
            <div class="mb-3">
              <label class="form-label">{{ $t('dev_tag_description') }}</label>
              <textarea class="form-control" rows="3" v-model.trim="tagForm.description"></textarea>
            </div>
          </div>
          <div class="modal-footer">
            <LiquidGlassButton variant="secondary" @click="closeTagModal">{{ $t('tsl_cancel') }}</LiquidGlassButton>
            <LiquidGlassButton variant="primary" :disabled="!tagForm.name" @click="saveTag">{{ $t('common_save') }}</LiquidGlassButton>
          </div>
        </div>
      </div>
    </div>
    </Teleport>
  </div>
</template>

<script setup>
import { computed, onBeforeUnmount, onMounted, ref } from 'vue';
import { onBeforeRouteLeave } from 'vue-router';
import axios from 'axios';
import { useI18n } from 'vue-i18n';
import { formatNamedReference, formatReferenceFromMap, referenceNameMap } from '../utils/entityDisplay.js';

const { t } = useI18n();

const tags = ref([]);
const devices = ref([]);
const products = ref([]);
const selectedTag = ref(null);
const assignedDeviceCodes = ref([]);
const savedDeviceCodes = ref([]);
const isSaving = ref(false);
const loadingTags = ref(false);
const loadingDevices = ref(false);
const showTagModal = ref(false);
const editingTag = ref(null);
const tagForm = ref({ name: '', color: '#0d6efd', icon: 'bi-tag', description: '' });
const PRESET_COLORS = [
  '#0d6efd', '#6610f2', '#6f42c1', '#d63384',
  '#dc3545', '#fd7e14', '#ffc107', '#198754',
  '#20c997', '#0dcaf0', '#6c757d', '#212529',
];
const PRESET_ICONS = [
  'bi-tag', 'bi-bookmark', 'bi-star', 'bi-heart', 'bi-flag',
  'bi-bell', 'bi-shield', 'bi-shield-check', 'bi-lock', 'bi-unlock',
  'bi-gear', 'bi-sliders', 'bi-tools', 'bi-wrench',
  'bi-camera', 'bi-camera-video', 'bi-wifi', 'bi-bluetooth',
  'bi-thermometer', 'bi-droplet', 'bi-lightning', 'bi-sun',
  'bi-cloud', 'bi-moon', 'bi-fire', 'bi-snow',
  'bi-house', 'bi-building', 'bi-factory', 'bi-hdd-stack',
  'bi-diagram-3', 'bi-boxes', 'bi-pc-display', 'bi-cpu',
  'bi-arrow-repeat', 'bi-arrow-clockwise', 'bi-activity', 'bi-graph-up',
  'bi-exclamation-triangle', 'bi-check-circle', 'bi-x-circle', 'bi-info-circle',
  'bi-people', 'bi-person-badge', 'bi-shield-lock', 'bi-eye',
];
const deviceSearch = ref('');
const productFilter = ref('');
const assignmentListLimit = 80;

const productOptions = computed(() => {
  return Array.from(new Set(devices.value.map(device => device.product_code).filter(Boolean))).sort();
});
const productNameMap = computed(() => referenceNameMap(products.value));
const deviceDisplay = (device) => formatNamedReference(device?.name, device?.code);
const productDisplay = (code) => formatReferenceFromMap(code, productNameMap.value);

const filteredDevices = computed(() => {
  const keyword = deviceSearch.value.trim().toLowerCase();
  return devices.value.filter(device => {
    if (productFilter.value && device.product_code !== productFilter.value) return false;
    if (!keyword) return true;
    return [device.code, device.name, device.product_code]
      .filter(Boolean)
      .some(value => String(value).toLowerCase().includes(keyword));
  });
});

const assignedSet = computed(() => new Set(assignedDeviceCodes.value));
const assignedDevices = computed(() => filteredDevices.value.filter(device => assignedSet.value.has(device.code)));
const unassignedDevices = computed(() => filteredDevices.value.filter(device => !assignedSet.value.has(device.code)));
const assignedVisibleDevices = computed(() => assignedDevices.value.slice(0, assignmentListLimit));
const unassignedVisibleDevices = computed(() => unassignedDevices.value.slice(0, assignmentListLimit));
const assignedOverflow = computed(() => Math.max(assignedDevices.value.length - assignedVisibleDevices.value.length, 0));
const unassignedOverflow = computed(() => Math.max(unassignedDevices.value.length - unassignedVisibleDevices.value.length, 0));

const isDirty = computed(() => {
  const curr = assignedDeviceCodes.value;
  const saved = savedDeviceCodes.value;
  if (curr.length !== saved.length) return true;
  return curr.some((code, i) => code !== saved[i]);
});

const normalizeDeviceCodes = (codes) => Array.from(new Set(codes.filter(Boolean))).sort();

const fetchTags = async () => {
  loadingTags.value = true;
  try {
    const res = await axios.get('/api/device-tags', { params: { _t: Date.now() } });
    if (res.data.code === 0) {
      tags.value = res.data.data || [];
      if (selectedTag.value) {
        selectedTag.value = tags.value.find(tag => tag.ID === selectedTag.value.ID) || null;
      }
    }
  } catch (e) {
    console.error(e);
  } finally {
    loadingTags.value = false;
  }
};

const fetchDevices = async () => {
  loadingDevices.value = true;
  try {
    const res = await axios.get('/api/devices', { params: { page: 0, _t: Date.now() } });
    if (res.data.code === 0) {
      devices.value = res.data.data || [];
    }
  } catch (e) {
    console.error(e);
  } finally {
    loadingDevices.value = false;
  }
};

const fetchProducts = async () => {
  try {
    const res = await axios.get('/api/products', { params: { pageSize: 1000, _t: Date.now() } });
    if (res.data.code === 0) {
      const data = res.data.data;
      products.value = Array.isArray(data) ? data : (data?.list || []);
    }
  } catch (e) {
    console.error(e);
    products.value = [];
  }
};

const selectTag = async (tag) => {
  if (selectedTag.value && selectedTag.value.ID !== tag.ID && isDirty.value) {
    await saveAssignments();
  }
  selectedTag.value = tag;
  try {
    const res = await axios.get(`/api/device-tags/${tag.ID}/devices`, { params: { _t: Date.now() } });
    if (res.data.code === 0) {
      const codes = normalizeDeviceCodes(res.data.data?.device_codes || []);
      assignedDeviceCodes.value = codes;
      savedDeviceCodes.value = [...codes];
    }
  } catch (e) {
    console.error(e);
    assignedDeviceCodes.value = [];
    savedDeviceCodes.value = [];
  }
};

const openTagModal = (tag = null) => {
  editingTag.value = tag;
  tagForm.value = tag
    ? { name: tag.name, color: tag.color || '#0d6efd', icon: tag.icon || 'bi-tag', description: tag.description || '' }
    : { name: '', color: '#0d6efd', icon: 'bi-tag', description: '' };
  showTagModal.value = true;
};

const closeTagModal = () => {
  showTagModal.value = false;
  editingTag.value = null;
};

const saveTag = async () => {
  try {
    const payload = { ...tagForm.value };
    const res = editingTag.value
      ? await axios.put(`/api/device-tags/${editingTag.value.ID}`, payload)
      : await axios.post('/api/device-tags', payload);
    if (res.data.code === 0) {
      closeTagModal();
      await fetchTags();
      if (!selectedTag.value && res.data.data) {
        await selectTag(res.data.data);
      }
    } else {
      alert(res.data.message);
    }
  } catch (e) {
    console.error(e);
    alert(t('common_save_fail'));
  }
};

const deleteTag = async (tag) => {
  if (!confirm(t('common_delete_confirm'))) return;
  try {
    const res = await axios.delete(`/api/device-tags/${tag.ID}`);
    if (res.data.code === 0) {
      if (selectedTag.value?.ID === tag.ID) {
        selectedTag.value = null;
        assignedDeviceCodes.value = [];
      }
      await fetchTags();
      await fetchDevices();
    } else {
      alert(res.data.message);
    }
  } catch (e) {
    console.error(e);
    alert(t('common_delete_fail'));
  }
};

const bindDevice = (device) => {
  assignedDeviceCodes.value = normalizeDeviceCodes([...assignedDeviceCodes.value, device.code]);
};

const unbindDevice = (device) => {
  assignedDeviceCodes.value = assignedDeviceCodes.value.filter(code => code !== device.code);
};

const selectVisibleDevices = () => {
  assignedDeviceCodes.value = normalizeDeviceCodes([
    ...assignedDeviceCodes.value,
    ...unassignedVisibleDevices.value.map(device => device.code)
  ]);
};

const clearVisibleDevices = () => {
  const visible = new Set(assignedVisibleDevices.value.map(device => device.code));
  assignedDeviceCodes.value = assignedDeviceCodes.value.filter(code => !visible.has(code));
};

const saveAssignments = async () => {
  if (!selectedTag.value) return;
  isSaving.value = true;
  try {
    const res = await axios.put(`/api/device-tags/${selectedTag.value.ID}/devices`, {
      device_codes: assignedDeviceCodes.value
    });
    if (res.data.code === 0) {
      savedDeviceCodes.value = [...assignedDeviceCodes.value];
      await fetchTags();
      await fetchDevices();
    } else {
      alert(res.data.message);
    }
  } catch (e) {
    console.error(e);
    alert(t('common_save_fail'));
  } finally {
    isSaving.value = false;
  }
};

onMounted(async () => {
  await Promise.all([fetchTags(), fetchDevices(), fetchProducts()]);
  if (tags.value.length > 0) {
    selectTag(tags.value[0]);
  }
});

// Navigation guard — block SPA route change if dirty
onBeforeRouteLeave((_to, _from, next) => {
  if (isDirty.value) {
    const answer = window.confirm(t('dev_tag_unsaved_confirm'));
    if (!answer) {
      next(false);
      return;
    }
  }
  next();
});

// Browser/tab close guard
const handleBeforeUnload = (e) => {
  if (isDirty.value) {
    e.preventDefault();
    e.returnValue = '';
  }
};
window.addEventListener('beforeunload', handleBeforeUnload);

onBeforeUnmount(() => {
  window.removeEventListener('beforeunload', handleBeforeUnload);
});
</script>

<style scoped>
.device-tags-layout {
  display: flex;
  gap: 16px;
  height: 100%;
  min-height: 0;
}

.tag-sidebar {
  flex: 0 0 320px;
  max-width: 320px;
  min-width: 280px;
}

.tag-workspace {
  flex: 1 1 auto;
  min-width: 0;
}

.tag-list {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: 8px 12px;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.tag-list-item {
  display: grid;
  grid-template-columns: auto auto minmax(0, 1fr) auto auto;
  align-items: center;
  gap: 8px;
  padding: 10px 12px;
  border: 1px solid var(--border-color);
  border-radius: var(--radius-control, 10px);
  background: var(--noyo-glass-island-reading, rgba(var(--bs-body-bg-rgb), 0.35));
  backdrop-filter: blur(8px);
  -webkit-backdrop-filter: blur(8px);
  cursor: pointer;
  transition: all var(--noyo-duration-standard, 0.2s) ease;
  color: var(--text-primary, var(--bs-body-color));
}

.tag-list-item:hover {
  background: var(--noyo-glass-island-action-hover, rgba(var(--bs-primary-rgb), 0.08));
  border-color: rgba(var(--bs-primary-rgb), 0.35);
  transform: translateY(-1px);
}

.tag-list-item.active {
  background: rgba(var(--bs-primary-rgb), 0.14);
  border-color: var(--bs-primary);
  color: var(--bs-primary);
  box-shadow: 0 0 0 1px var(--bs-primary);
}

.tag-list-item__text {
  min-width: 0;
}

.tag-color-dot {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  flex: 0 0 auto;
  box-shadow: 0 0 0 2px rgba(var(--bs-body-bg-rgb), 0.8);
}

.tag-list-item__icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 20px;
  font-size: 0.9rem;
  color: inherit;
  flex-shrink: 0;
}

.assignment-summary {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  padding: 10px 16px;
  border-top: 1px solid var(--border-color);
  border-bottom: 1px solid var(--border-color);
  background: var(--noyo-glass-island-action, rgba(var(--bs-body-bg-rgb), 0.4));
  color: var(--text-secondary, var(--bs-secondary-color));
  font-size: 0.85rem;
  font-weight: 500;
  backdrop-filter: blur(12px);
  -webkit-backdrop-filter: blur(12px);
  flex-shrink: 0;
}

.assignment-board {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
  gap: 16px;
  padding: 16px;
  flex: 1;
  min-height: 0;
}

.assignment-panel {
  min-width: 0;
  height: 100%;
  display: flex;
  flex-direction: column;
  border: 1px solid var(--noyo-dashboard-liquid-edge, var(--border-color));
  border-radius: var(--radius-card, 14px);
  background: var(--noyo-dashboard-liquid-tint, var(--bg-surface));
  backdrop-filter: blur(var(--noyo-dashboard-liquid-blur, 10px)) saturate(175%) brightness(var(--noyo-dashboard-liquid-backdrop-brightness, 1.05));
  -webkit-backdrop-filter: blur(var(--noyo-dashboard-liquid-blur, 10px)) saturate(175%) brightness(var(--noyo-dashboard-liquid-backdrop-brightness, 1.05));
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.04);
  overflow: hidden;
}

.assignment-panel__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 12px 16px;
  border-bottom: 1px solid var(--border-color);
  background: var(--noyo-glass-island-action, rgba(var(--bs-body-bg-rgb), 0.45));
  backdrop-filter: blur(8px);
  -webkit-backdrop-filter: blur(8px);
  flex-shrink: 0;
}

.device-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  flex: 1;
  min-height: 0;
  padding: 12px;
  overflow-y: auto;
}

.device-list-item {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  align-items: center;
  gap: 10px;
  padding: 10px 14px;
  border: 1px solid var(--border-color);
  border-radius: var(--radius-control, 10px);
  background: var(--noyo-glass-island-reading, rgba(var(--bs-body-bg-rgb), 0.45));
  backdrop-filter: blur(8px);
  -webkit-backdrop-filter: blur(8px);
  transition: all var(--noyo-duration-standard, 0.2s) cubic-bezier(0.16, 1, 0.3, 1);
}

.device-list-item:hover {
  border-color: rgba(var(--bs-primary-rgb), 0.4);
  background: var(--noyo-glass-island-action-hover, rgba(var(--bs-primary-rgb), 0.06));
  transform: translateY(-1px);
  box-shadow: 0 4px 12px rgba(var(--bs-primary-rgb), 0.08);
}

.device-list-item__main {
  min-width: 0;
}

.device-list-item__meta {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 6px;
  margin-top: 6px;
}

.device-name {
  overflow: hidden;
  color: var(--bs-primary);
  font-family: var(--bs-font-monospace);
  font-weight: 700;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.device-code {
  overflow: hidden;
  color: var(--text-secondary, var(--bs-secondary-color));
  font-size: 0.82rem;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.product-pill {
  display: inline-block;
  max-width: 120px;
  overflow: hidden;
  padding: 2px 8px;
  border: 1px solid var(--border-color);
  border-radius: 999px;
  background: var(--noyo-glass-island-action, rgba(var(--bs-body-bg-rgb), 0.5));
  color: var(--text-primary, var(--bs-body-color));
  font-size: 0.76rem;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.assignment-empty {
  display: flex;
  align-items: center;
  justify-content: center;
  flex: 1;
  min-height: 200px;
  padding: 24px;
  color: var(--text-secondary, var(--bs-secondary-color));
  text-align: center;
}

.assignment-limit-note {
  padding: 6px 2px 2px;
  color: var(--text-secondary, var(--bs-secondary-color));
  font-size: 0.78rem;
  text-align: center;
}

@keyframes spin {
  from { transform: rotate(0deg); }
  to   { transform: rotate(360deg); }
}
.bi-arrow-repeat.spin {
  display: inline-block;
  animation: spin 1s linear infinite;
}

.min-w-0 {
  min-width: 0;
}

.color-presets {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}
.color-swatch {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  border-radius: 50%;
  cursor: pointer;
  border: 2px solid transparent;
  transition: all 0.15s ease;
  box-shadow: 0 1px 3px rgba(0,0,0,0.12);
}
.color-swatch:hover {
  transform: scale(1.15);
  box-shadow: 0 2px 6px rgba(0,0,0,0.18);
}
.color-swatch.active {
  border-color: #fff;
  box-shadow: 0 0 0 2px var(--bs-primary), 0 2px 6px rgba(0,0,0,0.18);
}
.color-swatch i {
  font-size: 0.8rem;
  color: #fff;
  filter: drop-shadow(0 1px 1px rgba(0,0,0,0.4));
}

.icon-presets {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  max-height: 180px;
  overflow-y: auto;
  padding: 4px 2px;
}
.icon-option {
  position: relative;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 38px;
  height: 38px;
  border-radius: 8px;
  cursor: pointer;
  border: 1px solid var(--border-color);
  transition: all 0.15s ease;
  background: var(--noyo-glass-island-action, rgba(var(--bs-body-bg-rgb), 0.4));
  color: var(--bs-body-color);
  font-size: 1rem;
}
.icon-option:hover {
  border-color: var(--bs-primary);
  background: var(--noyo-glass-island-action-hover, rgba(var(--bs-primary-rgb), 0.08));
  transform: scale(1.12);
}
.icon-option.active {
  border-color: var(--bs-primary);
  background: rgba(var(--bs-primary-rgb), 0.14);
  color: var(--bs-primary);
  box-shadow: 0 0 0 1px var(--bs-primary);
}
.icon-checked {
  position: absolute;
  top: -5px;
  right: -5px;
  font-size: 0.6rem;
  color: var(--bs-primary);
  background: #fff;
  border-radius: 50%;
}

@media (max-width: 992px) {
  .device-tags-layout {
    flex-direction: column;
  }

  .tag-sidebar {
    flex: none;
    max-width: none;
    width: 100%;
  }

  .assignment-board {
    grid-template-columns: 1fr;
  }

  .device-list {
    max-height: none;
  }
}
</style>
