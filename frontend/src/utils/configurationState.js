import { localizeServerPayload } from './serverText.js';

export function resourceConfiguration(data, entityType, entityCode, gatewayCode = '') {
  if (data instanceof Map) {
    const resource = data.get(JSON.stringify([entityType, entityCode]));
    if (!resource) return { projections: [], intents: [], runtime: [] };
    const matchesGateway = (entry) => !gatewayCode || !entry.gateway_code || entry.gateway_code === gatewayCode;
    return {
      projections: resource.projections.filter(matchesGateway),
      intents: resource.intents.filter(matchesGateway),
      runtime: resource.runtime.filter(matchesGateway),
    };
  }
  const matches = (entry) => entry.entity_type === entityType && entry.entity_code === entityCode
    && (!gatewayCode || !entry.gateway_code || entry.gateway_code === gatewayCode);
  return {
    projections: (data?.projections || []).filter(matches),
    intents: (data?.intents || []).filter(matches),
    runtime: [...(data?.runtime || []), ...(data?.runtime_apply || [])].filter(matches),
  };
}

// History remains available in the detail panel. The list summarizes the
// newest operation for each target, so a resolved failure cannot mask recovery.
function latestConfigurationIntents(intents) {
  const latest = new Map();
  for (const intent of intents || []) {
    const targets = intent.gateway_code ? [intent.gateway_code] : String(intent.target_node || "").split(",");
    for (const target of targets) {
      const key = JSON.stringify([intent.entity_type, intent.entity_code, target.trim()]);
      const previous = latest.get(key);
      const timeDifference = Number(intent.created_at || 0) - Number(previous?.created_at || 0);
      const sequenceDifference = String(intent.operation_code || "").localeCompare(String(previous?.operation_code || ""), undefined, { numeric: true });
      if (!previous || timeDifference > 0 || (timeDifference === 0 && sequenceDifference > 0)) latest.set(key, intent);
    }
  }
  return [...new Set(latest.values())];
}

function configurationTimestamp(value) {
  if (value === null || value === undefined || value === '') return 0;
  const timestamp = typeof value === 'string' && !/^\d+$/.test(value) ? Date.parse(value) : Number(value);
  return Number.isFinite(timestamp) ? timestamp : 0;
}

export function configurationStatus(resource, fallback = '', pendingEditAt = null) {
  const intents = latestConfigurationIntents(resource.intents);
  const runtime = resource.runtime || [];
  // A replication conflict and a superseded platform intent are different
  // states that used to share one badge. The replication layer keeps both
  // versions and is arbitrated on the gateway page; a superseded intent is
  // resolved by editing the resource again. Keep them apart so each badge
  // points at the surface that owns the decision.
  if (fallback === 'conflict') return 'sync_conflict';
  const failedIntents = intents.filter((entry) => ['failed', 'rejected'].includes(entry.status));
  if (failedIntents.length) {
    // An old row may retain "pending" after a terminal acknowledgement. Only a
    // demonstrably newer local edit can supersede that confirmed failure while
    // its intent has not yet reached this response.
    const newestIntentAt = Math.max(...intents.map((entry) => configurationTimestamp(entry.created_at)));
    const newerPendingEdit = fallback === 'pending'
      && failedIntents.every((entry) => configurationTimestamp(entry.created_at) > 0)
      && configurationTimestamp(pendingEditAt) > newestIntentAt;
    return newerPendingEdit ? 'pending' : 'failed';
  }
  // List mutation responses can arrive before the status poll; keep their
  // pending/conflict signal until the freshly confirmed state is fetched.
  if (fallback === 'pending') return 'pending';
  if (intents.some((entry) => entry.status === 'conflict')) return 'conflict';
  if (runtime.some((entry) => entry.status === 'failed')) return 'failed';
  if (intents.some((entry) => ['pending', 'queued', 'submitted'].includes(entry.status))) {
    return intents.some((entry) => entry.kind === 'delete' && entry.status === 'pending') ? 'pending_delete' : 'pending';
  }
  if (runtime.some((entry) => entry.status === 'pending')) return 'applying';
  const projections = resource.projections || [];
  const confirmationsMissing = projections.some((projection) => !runtime.some((entry) =>
    (!projection.gateway_code || !entry.gateway_code || projection.gateway_code === entry.gateway_code)
    && Number(entry.revision) >= Number(projection.revision)
    && Number(entry.lifecycle_version) === Number(projection.lifecycle_version)));
  if (runtime.length && runtime.every((entry) => entry.status === 'active') && !confirmationsMissing) return 'active';
  if (projections.length) return 'committed';
  return fallback;
}

// Build once per response. A list row should not rescan every resource and its
// history separately for the badge and the failure explanation.
export function configurationResourceIndex(data) {
  const index = new Map();
  for (const [field, entries] of [
    ['projections', data?.projections], ['intents', data?.intents],
    ['runtime', data?.runtime], ['runtime', data?.runtime_apply],
  ]) {
    for (const entry of entries || []) {
      const key = JSON.stringify([entry.entity_type, entry.entity_code]);
      if (!index.has(key)) index.set(key, { projections: [], intents: [], runtime: [] });
      index.get(key)[field].push(entry);
    }
  }
  return index;
}

// Use the same current-intent selection as the badge status so a recovered
// operation cannot keep showing its historical failure reason in the list.
export function configurationFailureReason(resource) {
  const failures = [
    ...latestConfigurationIntents(resource.intents).filter((entry) => ['failed', 'rejected'].includes(entry.status)),
    ...(resource.runtime || []).filter((entry) => entry.status === 'failed'),
  ];
  return failures.find((entry) => entry.last_error)?.last_error || '';
}

// A settled resource needs no marker. "committed" (the gateway confirmed the
// configuration) and "active" (the runtime loaded it) are the healthy outcomes,
// so lists and the detail drawer stay silent for them; only states an operator
// may still have to look at — in flight, conflicted, failed — earn screen space.
export const QUIET_CONFIGURATION_STATES = ['committed', 'active'];

export function configurationNeedsAttention(status) {
  const normalized = String(status || '').toLowerCase();
  return normalized !== '' && !QUIET_CONFIGURATION_STATES.includes(normalized);
}

export function pendingConfigurationCreates(data, entityType) {
  const projections = data?.projections || [];
  const latest = new Map();
  for (const intent of latestConfigurationIntents(data?.intents)) {
    if (intent.entity_type !== entityType || intent.kind === 'delete' || intent.status === 'committed') continue;
    if (intent.base_payload && intent.base_payload !== '{}' && intent.base_payload !== 'null') continue;
    if (projections.some((entry) => entry.entity_type === entityType && entry.entity_code === intent.entity_code && !entry.deleted)) continue;
    const previous = latest.get(intent.entity_code);
    if (!previous || Number(intent.created_at) > Number(previous.created_at)) latest.set(intent.entity_code, intent);
  }
  return Array.from(latest.values()).sort((a, b) => Number(b.created_at) - Number(a.created_at));
}

// Snapshots carry a per-language copy of user-visible fields (TSL names with
// `name_zh`, `{"zh":…,"en":…}` pairs, JSON strings nested in JSON strings), so
// the raw text renders both languages at once. Passing the active locale
// collapses each field to the console language; omitting it keeps the payload
// byte-faithful for callers that only need structural formatting.
export function formatConfigurationPayload(payload, locale = '') {
  if (payload === null || payload === undefined || payload === '') return '';
  try {
    const parsed = typeof payload === 'string' ? JSON.parse(payload) : payload;
    const rendered = locale ? localizeServerPayload(parsed, locale) : parsed;
    return JSON.stringify(rendered, null, 2);
  } catch {
    return String(payload);
  }
}
