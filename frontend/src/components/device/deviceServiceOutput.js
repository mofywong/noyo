export const hasServiceOutputParams = (srv) => Array.isArray(srv?.outputData) && srv.outputData.length > 0

export const normalizeServiceOutputData = (data) => {
  if (typeof data !== 'string') return data

  const trimmed = data.trim()
  if (!trimmed) return data
  if (!looksLikeJson(trimmed)) return data

  try {
    return JSON.parse(trimmed)
  } catch (_) {
    return data
  }
}

const looksLikeJson = (value) => {
  return (value.startsWith('{') && value.endsWith('}')) || (value.startsWith('[') && value.endsWith(']'))
}

const findNestedOutputObject = (data, identifier) => {
  if (!data || typeof data !== 'object' || Array.isArray(data) || !identifier) return null

  for (const value of Object.values(data)) {
    const normalized = normalizeServiceOutputData(value)
    if (
      normalized &&
      typeof normalized === 'object' &&
      !Array.isArray(normalized) &&
      Object.prototype.hasOwnProperty.call(normalized, identifier)
    ) {
      return normalized
    }
  }

  return null
}

export const getServiceOutputRawValue = (data, outParam, outputCount = 1) => {
  if (data === null || data === undefined) return undefined

  const identifier = outParam?.identifier
  const normalizedData = normalizeServiceOutputData(data)

  if (identifier && normalizedData && typeof normalizedData === 'object' && !Array.isArray(normalizedData)) {
    if (Object.prototype.hasOwnProperty.call(normalizedData, identifier)) {
      const directValue = normalizeServiceOutputData(normalizedData[identifier])
      if (
        directValue &&
        typeof directValue === 'object' &&
        !Array.isArray(directValue) &&
        Object.prototype.hasOwnProperty.call(directValue, identifier)
      ) {
        return directValue[identifier]
      }
      return directValue
    }

    const nestedOutput = findNestedOutputObject(normalizedData, identifier)
    if (nestedOutput) {
      return nestedOutput[identifier]
    }

    return undefined
  }

  return outputCount === 1 ? normalizedData : undefined
}

export const formatServiceOutputValue = (value, outParam) => {
  if (value === null || value === undefined) return '-'

  const dataType = outParam?.dataType || {}
  const type = dataType.type
  const specs = dataType.specs || {}

  if (type === 'enum' && specs && value !== '-') {
    const enumName = specs[value] ?? specs[String(value)]
    if (enumName !== undefined) {
      return `${enumName} (${value})`
    }
  }

  if ((type === 'int' || type === 'float' || type === 'double') && specs.unit) {
    return `${value} ${specs.unit}`
  }

  if (typeof value === 'object') {
    return JSON.stringify(value)
  }

  return String(value)
}

export const getOutputValue = (data, outParam, outputCount = 1) => {
  return formatServiceOutputValue(getServiceOutputRawValue(data, outParam, outputCount), outParam)
}

export const formatServiceOutputData = (data, srv) => {
  if (!hasServiceOutputParams(srv)) {
    return normalizeServiceOutputData(data)
  }

  const outputCount = srv.outputData.length
  return srv.outputData.reduce((acc, outParam) => {
    acc[outParam.identifier] = getOutputValue(data, outParam, outputCount)
    return acc
  }, {})
}
