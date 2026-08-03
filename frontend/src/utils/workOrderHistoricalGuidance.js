function asObject(value) {
  if (value && typeof value === 'object' && !Array.isArray(value)) return value
  if (typeof value !== 'string' || !value.trim()) return null
  try {
    const parsed = JSON.parse(value)
    return parsed && typeof parsed === 'object' && !Array.isArray(parsed) ? parsed : null
  } catch (_) {
    return null
  }
}

function cleanText(value) {
  return String(value ?? '').trim()
}

function positiveCount(value) {
  const count = Number(value)
  return Number.isFinite(count) && count > 0 ? Math.trunc(count) : 0
}

export function historicalExperienceDetails(experience = {}) {
  const candidates = [
    ['handlingOpinion', experience.handling_opinion],
    ['rootCause', experience.root_cause],
    ['handlingProcess', experience.handling_process],
    ['handlingResult', experience.handling_result]
  ]
  const seen = new Set()
  const details = []
  for (const [labelKey, rawValue] of candidates) {
    const value = cleanText(rawValue)
    const fingerprint = value.toLocaleLowerCase()
    if (!value || seen.has(fingerprint)) continue
    seen.add(fingerprint)
    details.push({ labelKey, value })
  }
  return details
}

export function normalizeWorkOrderHistoricalGuidance(detail) {
  if (detail?.work_order?.source_type !== 'ai') return null
  const snapshot = asObject(detail?.source_snapshot) || {}
  const proposal = asObject(snapshot.proposal) || {}
  const history = asObject(proposal.historical_resolution) || asObject(snapshot.historical_resolution)
  if (!history) return null

  const rawExperiences = Array.isArray(history.experiences) && history.experiences.length
    ? history.experiences
    : [history]
  const experiences = rawExperiences.map((experience, index) => ({
    id: `${positiveCount(history.memory_id)}-${index}`,
    details: historicalExperienceDetails(experience),
    verificationCount: positiveCount(experience?.verification_count),
    lastVerifiedAt: cleanText(experience?.last_verified_at)
  })).filter(experience => experience.details.length > 0)
  if (!experiences.length) return null

  return {
    memoryId: positiveCount(history.memory_id),
    title: cleanText(history.title),
    summary: cleanText(history.summary),
    verificationCount: positiveCount(history.verification_count),
    lastVerifiedAt: cleanText(history.last_verified_at),
    experiences
  }
}
