const clean = (value) => String(value ?? '').trim();

export const formatNamedReference = (name, code) => {
  const displayName = clean(name);
  const referenceCode = clean(code);
  if (!displayName) return referenceCode;
  if (!referenceCode || displayName === referenceCode) return displayName;
  return `${displayName}（${referenceCode}）`;
};

export const referenceNameMap = (entities, codeKey = 'code', nameKey = 'name') => {
  const names = new Map();
  for (const entity of Array.isArray(entities) ? entities : []) {
    const code = clean(entity?.[codeKey]);
    if (code) names.set(code, clean(entity?.[nameKey]));
  }
  return names;
};

export const formatReferenceFromMap = (code, names) => formatNamedReference(names?.get(clean(code)), code);
