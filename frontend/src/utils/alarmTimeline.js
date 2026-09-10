const occurrenceEventTypes = new Set(['opened', 'repeated', 'signal_observed'])
const observationEventTypes = new Set(['opened', 'repeated', 'signal_observed', 'recovered', 'condition_recovered', 'cleared'])

function timelineEvidence(item, type) {
  // Event rows currently carry the immutable alarm snapshot for auditability.
  // Only occurrence and condition-observation rows represent a scene; handling
  // rows (including closed) must not present that snapshot as new evidence.
  if (!observationEventTypes.has(type)) return {}
  if (occurrenceEventTypes.has(type)) return item.payload?.payload ?? item.evidence ?? {}
  return item.evidence ?? {}
}

export function buildAlarmTimeline(events) {
  // Open/repeat records are the authoritative occurrences. signal_observed is
  // supplementary enrichment, persisted separately for the same occurrence.
  const hasOccurrences = events.some(item => ['opened', 'repeated'].includes(item.event?.type))
  let occurrenceIndex = 0
  return events.filter(item => !hasOccurrences || item.event?.type !== 'signal_observed').map(item => {
    const type = item.event?.type
    const evidence = timelineEvidence(item, type)
    const timelineItem = {
      ...item,
      evidence,
    }
    if (!occurrenceEventTypes.has(type)) return timelineItem
    // Repeated events retain the instance's initial EvidenceSnapshot. Their
    // payload is the complete evidence for this occurrence, including no media.
    return {
      ...timelineItem,
      created_at: item.payload?.occurred_at || evidence.reported_at || item.created_at,
      occurrenceIndex: occurrenceIndex++,
      recordingId: evidence.params?.recording_id || evidence.recording_id || '',
    }
  })
}
