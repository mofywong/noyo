// Persist references, never expand a product into thousands of device IDs.
const codes = values => [...new Set((Array.isArray(values) ? values : []).filter(value => typeof value === 'string' && value.length > 0))].sort()

export function assistantScope(value) {
  return value?.version === 1 && value.mode === 'selected'
    ? { version: 1, mode: 'selected', productCodes: codes(value.productCodes), deviceCodes: codes(value.deviceCodes) }
    : { version: 1, mode: 'project', productCodes: [], deviceCodes: [] }
}

export function assistantScopeRequest(value) {
  const scope = assistantScope(value)
  return scope.mode === 'project' ? {} : { productCodes: scope.productCodes, deviceCodes: scope.deviceCodes }
}

export function assistantScopeCount(value) {
  // Product definitions describe capabilities, not a device selection.
  return assistantScope(value).deviceCodes.length
}
