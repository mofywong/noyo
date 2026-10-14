import { newerModelVersion } from './modelCenter.js';

const unique = items => [...new Set(items)].sort();

// A delivery always includes the full closure, even when the customer already
// has resources installed. Local install plans must not be used for this export.
export function modelDemand(catalog, selected, enhancements = []) {
  const capabilities = unique(selected);
  const rows = [], optional = new Set();
  const collect = (capabilityId, seenCaps, packages, stack = new Set()) => {
    if (stack.has(capabilityId)) throw new Error('cyclic dependency');
    if (seenCaps.has(capabilityId)) return;
    const cap = catalog.capabilities.find(item => item.id === capabilityId);
    if (!cap) throw new Error('unknown capability');
    stack.add(capabilityId);
    for (const id of cap.dependencies || []) collect(id, seenCaps, packages, stack);
    for (const id of cap.packages || []) collectPackage(id, packages);
    for (const id of cap.optionalPackages || []) optional.add(id);
    stack.delete(capabilityId);
    seenCaps.add(capabilityId);
  };
  const collectPackage = (id, packages, stack = new Set()) => {
    if (stack.has(id)) throw new Error('cyclic dependency');
    if (packages.has(id)) return;
    stack.add(id);
    const release = catalog.packages.filter(p => p.id === id).reduce((latest, p) => !latest || newerModelVersion(p.version, latest.version) ? p : latest, null);
    for (const dep of release?.dependencies || []) collectPackage(dep, packages, stack);
    stack.delete(id);
    packages.add(id);
  };
  for (const id of capabilities) {
    const packages = new Set();
    collect(id, new Set(), packages);
    rows.push({ capability: catalog.capabilities.find(c => c.id === id), packages: [...packages].sort() });
  }
  const optionalPackages = unique(enhancements.filter(id => optional.has(id)));
  const packageIds = new Set(rows.flatMap(row => row.packages));
  for (const id of optionalPackages) collectPackage(id, packageIds);
  return {
    request: { schemaVersion: 1, capabilities, optionalPackages, supportedContracts: unique(catalog.packages.map(p => p.contract).filter(Boolean)) },
    rows, packageIds: [...packageIds].sort(), optionalIds: [...optional].sort(),
  };
}

export function modelDemandCSV(demand) {
  const cell = value => `"${String(value ?? '').replace(/^[=+@-]/, "'$&").replaceAll('"', '""')}"`;
  const rows = [['算法 ID / Algorithm ID', '中文名称 / Chinese name', '英文名称 / English name', '必需模型 / Required models', '已选增强模型 / Selected enhancements', '程序支持 / Application support']];
  for (const { capability: cap, packages } of demand.rows) {
    rows.push([cap.id, cap.title?.zh || '', cap.title?.en || '', packages.join(';'), demand.request.optionalPackages.filter(id => (cap.optionalPackages || []).includes(id)).join(';'), cap.state === 'unsupported' || cap.unsupportedReason ? '需升级 / Update required' : '支持 / Supported']);
  }
  return '\uFEFF' + rows.map(row => row.map(cell).join(',')).join('\r\n') + '\r\n';
}

export function downloadModelFile(name, content, type = 'application/json') {
  const url = URL.createObjectURL(new Blob([content], { type }));
  const link = document.createElement('a');
  link.href = url;
  link.download = name;
  link.click();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}
