import axios from 'axios'

export const SCREEN_PROTOCOL = 'noyo.screen/1'
export const SCREEN_ENGINE = 'goview-5ab05b4-noyo19'
export const screenId = (value) => String(value?.id ?? value?.ID ?? value ?? '')
export const draftVersion = (value) => Number(value?.draftVersion ?? value?.draft_version ?? 0)
export const metadataVersion = (value) => Number(value?.metadataVersion ?? value?.metadata_version ?? 0)
export const publishedReleaseId = (value) => value?.publishedReleaseId ?? value?.published_release_id ?? value?.activeReleaseId ?? null
export const releaseNumber = (value) => value?.releaseNo ?? value?.release_no ?? value?.version ?? value?.id ?? value?.ID
export const requestId = () => {
  if (globalThis.crypto?.randomUUID) return globalThis.crypto.randomUUID()
  // Private-network HTTP deployments can use getRandomValues without randomUUID.
  const bytes = globalThis.crypto.getRandomValues(new Uint8Array(16))
  return Array.from(bytes, (value) => value.toString(16).padStart(2, '0')).join('')
}

export async function screenRequest(method, path = '', data, config = {}) {
  const response = await axios({ method, url: `/api/screens${path}`, data, ...config })
  if (response.data?.code !== 0) {
    const error = new Error('Screen request failed')
    error.code = response.data?.code || 'REQUEST_FAILED'
    error.response = response
    throw error
  }
  return response.data.data
}

export function errorKey(error) {
  const status = error?.response?.status
  const code = String(error?.code || error?.response?.data?.error?.code || '')
  if (status === 409 || status === 412 || /CONFLICT|VERSION_MISMATCH/.test(code)) return 'conflict'
  if (status === 401 || status === 403) return 'unauthorized'
  if (status === 503) return 'serviceUnavailable'
  if (status === 404 || status === 410) return 'notFound'
  if (status === 413) return 'requestTooLarge'
  if (status === 400 || status === 422) return 'invalidRequest'
  if (status === 428) return 'versionRequired'
  if (code === 'ECONNABORTED' || code === 'ETIMEDOUT') return 'requestTimeout'
  if (code === 'ERR_NETWORK') return 'networkFailed'
  return 'failed'
}

export function revisionHeaders(version) { return { 'If-Match': `"draft-${version}"` } }

export const screensApi = {
  list: (params, signal) => screenRequest('get', '', undefined, { params, signal }),
  create: (name) => screenRequest('post', '', { name }),
  rename: (id, name, version) => screenRequest('put', `/${encodeURIComponent(id)}/metadata`, { name }, { headers: { 'If-Match': `"metadata-${version}"` } }),
  copy: (id, name, version, operationId) => screenRequest('post', `/${encodeURIComponent(id)}/copy`, { name, draftVersion: version, requestId: operationId }),
  remove: (id) => screenRequest('delete', `/${encodeURIComponent(id)}`),
  load: (id, mode = 'edit', releaseId) => releaseId
    ? screenRequest('get', `/${encodeURIComponent(id)}/releases/${encodeURIComponent(releaseId)}`)
    : screenRequest('get', `/${encodeURIComponent(id)}/${mode === 'view' ? 'published' : 'draft'}`),
  save: (id, document, version) => screenRequest('put', `/${encodeURIComponent(id)}/draft`, { document }, { headers: revisionHeaders(version) }),
  validate: (id, document) => screenRequest('post', `/${encodeURIComponent(id)}/validate`, { document }),
  publish: (id, version) => screenRequest('post', `/${encodeURIComponent(id)}/publish`, { draftVersion: version, requestId: requestId() }),
  unpublish: (id) => screenRequest('post', `/${encodeURIComponent(id)}/unpublish`, { requestId: requestId() }),
  releases: (id) => screenRequest('get', `/${encodeURIComponent(id)}/releases`),
  restore: (id, releaseId, version) => screenRequest('post', `/${encodeURIComponent(id)}/releases/${encodeURIComponent(releaseId)}/restore`, { draftVersion: version, requestId: requestId() }),
}
