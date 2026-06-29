export const isFilledDriverDefault = (value) => {
  if (value === undefined || value === null) return false;
  if (typeof value === 'string') return value.trim() !== '';
  if (Array.isArray(value)) return value.length > 0;
  if (typeof value === 'object') return Object.keys(value).length > 0;
  return true;
};

export const applyDriverDefaults = (currentConfig = {}, driverDefaults = null, schema = null) => {
  const config = currentConfig && typeof currentConfig === 'object' ? { ...currentConfig } : {};
  if (!driverDefaults || typeof driverDefaults !== 'object' || !schema?.properties) {
    return config;
  }
  for (const key of Object.keys(schema.properties)) {
    if (config[key] === undefined && isFilledDriverDefault(driverDefaults[key])) {
      config[key] = driverDefaults[key];
    }
  }
  return config;
};
