import { computed, onMounted, onUnmounted, ref, watch } from 'vue';
import axios from 'axios';
import { configurationFailureReason, configurationResourceIndex, configurationStatus, pendingConfigurationCreates, resourceConfiguration } from '../utils/configurationState.js';

// Load once for the list, rather than once per row. Runtime results are separate
// from persistence acknowledgements and must not be inferred from sync_state.
export function useConfigurationStates(entityType, resources = null) {
  const data = ref(null);
  const configurationStatesUnavailable = ref(false);
  const hasConfigurationStates = computed(() => data.value !== null);
  const resourceCodes = () => [...new Set((typeof resources === 'function' ? resources() : resources?.value || []).map((row) => row.code).filter(Boolean))];
  const indexed = computed(() => ({ resources: configurationResourceIndex(data.value), summaries: new Map() }));
  let timer;
  let controller;
  let generation = 0;

  async function reload() {
    const current = ++generation;
    controller?.abort();
    controller = new AbortController();
    try {
      const response = await axios.get('/api/extension/cascade/configuration/resources', {
        params: {
          entity_type: entityType, summary: true,
          ...(resources ? { entity_codes: JSON.stringify(resourceCodes()), include_pending_creates: true } : {}),
        }, signal: controller.signal,
      });
      if (response.data?.code && response.data.code !== 200) throw new Error('configuration state unavailable');
      if (current === generation) {
        data.value = response.data?.data || null;
        configurationStatesUnavailable.value = false;
      }
    } catch {
      // Retain confirmed failures through an outage; absence of fresh status is
      // not evidence that the configuration became healthy.
      if (current === generation && !controller.signal.aborted) configurationStatesUnavailable.value = true;
    }
  }

  function summaryFor(resource) {
    const pendingEditAt = resource.UpdatedAt ?? resource.updated_at ?? null;
    const key = JSON.stringify([resource.code, resource.gateway_code || '', resource.sync_state || '', pendingEditAt]);
    const cache = indexed.value.summaries;
    if (!cache.has(key)) {
      const state = resourceConfiguration(indexed.value.resources, entityType, resource.code, resource.gateway_code || '');
      const status = configurationStatus(state, resource.sync_state, pendingEditAt);
      cache.set(key, {
        status: status === 'pending' && state.intents.some((entry) => entry.kind === 'delete' && entry.status === 'pending') ? 'pending_delete' : status,
        reason: configurationFailureReason(state),
      });
    }
    return cache.get(key);
  }

  function configurationStateFor(resource) {
    if (!resource?.code) return '';
    return summaryFor(resource).status;
  }

  function configurationErrorFor(resource) {
    if (!resource?.code) return '';
    return summaryFor(resource).reason;
  }

  onMounted(() => {
    reload();
    timer = window.setInterval(reload, 15000);
    window.addEventListener('noyo-data-updated', reload);
  });
  if (resources) watch(() => JSON.stringify(resourceCodes()), reload);
  onUnmounted(() => {
    ++generation;
    controller?.abort();
    window.clearInterval(timer);
    window.removeEventListener('noyo-data-updated', reload);
  });
  const pendingConfigurationIntents = computed(() => pendingConfigurationCreates(data.value, entityType));
  return { configurationStateFor, configurationErrorFor, pendingConfigurationIntents, configurationStatesUnavailable, hasConfigurationStates, reloadConfigurationStates: reload };
}
