/**
 * propertyName.js — resolves a monitored point identifier to the label the
 * product author configured for it.
 *
 * Why this module exists
 * ----------------------
 * Plugins that watch a single telemetry point identify it by its TSL property
 * identifier, e.g. "AI_OUTPUT_CURRENT". That identifier is transport data: it is
 * unreadable in any language and carries nothing to translate. The operator-facing
 * label lives in the product TSL (products.config.tsl.properties[].name) and may
 * carry an explicit per-language pair via name_zh / name_en.
 *
 * The backend resolves the same ladder at alarm-write time
 * (noyo/backend/core/property_name.go) so newly written alarm titles already hold
 * the display name. This module handles the rows written before that, which still
 * embed the raw identifier in the stored title, and it renders the `property`
 * event parameter. Keep both ladders in sync.
 *
 * Resolution ladder, per language:
 *   zh: name_zh -> name (Chinese) -> name (bilingual payload) -> prettified identifier
 *   en: name_en -> name (Latin)   -> name (bilingual payload) -> prettified identifier
 *
 * The rule guarantees neither language renders the other language's characters,
 * and that a raw underscore identifier never reaches the operator.
 */

import { normalizeTextLocale } from './serverText.js'

const CJK_PATTERN = /[\u3040-\u30ff\u3400-\u4dbf\u4e00-\u9fff\uf900-\ufaff]/

const asText = (value) => (typeof value === 'string' ? value.trim() : '')

/** "AI_OUTPUT_CURRENT" -> "AI OUTPUT CURRENT" */
export const prettifyPointCode = (code) => (
  String(code ?? '').trim().replace(/[_\-.]+/g, ' ').replace(/\s+/g, ' ').trim()
)

const splitBilingualPayload = (value) => {
  const text = asText(value)
  if (!text.startsWith('{') || !text.includes('"')) return null
  try {
    const parsed = JSON.parse(text)
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) return null
    const zh = asText(parsed.zh)
    const en = asText(parsed.en)
    return zh || en ? { zh, en } : null
  } catch (_) {
    return null
  }
}

/**
 * Resolves a single value for `locale` from a TSL property definition. Returns
 * the prettified identifier when the definition is missing, so a raw code is
 * never the result.
 */
export const resolvePointNameFromProperty = (property, identifier, locale) => {
  const fallback = prettifyPointCode(identifier) || String(identifier ?? '').trim()
  const source = property && typeof property === 'object' && !Array.isArray(property) ? property : {}
  const name = asText(source.name)
  const payload = splitBilingualPayload(name)

  let zh = asText(source.name_zh) || payload?.zh || ''
  let en = asText(source.name_en) || payload?.en || ''

  if (!payload) {
    // `name` is the product-authored label: the Chinese copy whenever the author
    // wrote Chinese, and only reusable for English when it is already Latin text.
    if (!zh) zh = name
    if (!en && name && !CJK_PATTERN.test(name)) en = name
  }

  // No cross-language fallback: an English console must not be handed Chinese
  // text. Each side falls back to the prettified identifier independently, which
  // mirrors the backend ladder in core/property_name.go.
  const resolved = normalizeTextLocale(locale) === 'en' ? en : zh
  return resolved || fallback
}

const productPointDefinitions = (product) => {
  let config = product?.config
  if (typeof config === 'string') {
    try {
      config = JSON.parse(config)
    } catch (_) {
      return []
    }
  }
  if (!config || typeof config !== 'object' || Array.isArray(config)) return []
  const tsl = config.tsl && typeof config.tsl === 'object' && !Array.isArray(config.tsl) ? config.tsl : config
  return Array.isArray(tsl.properties) ? tsl.properties : []
}

/**
 * Builds productCode -> (lower-cased point identifier -> TSL property) so display
 * boundaries can resolve a point name without re-parsing product config.
 */
export const buildProductPointIndex = (products) => {
  const index = new Map()
  for (const product of products || []) {
    const code = String(product?.code ?? '').trim()
    if (!code) continue
    const points = new Map()
    for (const property of productPointDefinitions(product)) {
      const identifier = String(property?.identifier ?? '').trim().toLowerCase()
      if (identifier) points.set(identifier, property)
    }
    index.set(code, points)
  }
  return index
}

export const resolvePointName = (productPointIndex, productCode, identifier, locale) => {
  const identifierText = String(identifier ?? '').trim()
  if (!identifierText) return ''
  const points = productPointIndex?.get?.(String(productCode ?? '').trim())
  return resolvePointNameFromProperty(points?.get?.(identifierText.toLowerCase()), identifierText, locale)
}

/**
 * Replaces a raw point identifier inside already-rendered text (e.g. an alarm
 * title stored before the backend resolved names) with its display name.
 */
export const localizePointCodeInText = (text, identifier, name) => {
  const source = String(text ?? '')
  const code = String(identifier ?? '').trim()
  const replacement = String(name ?? '').trim()
  if (!source || !code || !replacement || !source.includes(code)) return source
  return source.split(code).join(replacement)
}
