export function buildAlarmTimeline(events) {
  // Open/repeat records are the authoritative occurrences. signal_observed is
  // supplementary enrichment, persisted separately for the same occurrence.
  const hasOccurrences = events.some(item => ['opened', 'repeated'].includes(item.event?.type))
  let occurrenceIndex = 0
  return events.filter(item => !hasOccurrences || item.event?.type !== 'signal_observed').map(item => {
    const type = item.event?.type
    if (!['opened', 'repeated', 'signal_observed'].includes(type)) return item
    // Repeated events retain the instance's initial EvidenceSnapshot. Their
    // payload is the complete evidence for this occurrence, including no media.
    const evidence = item.payload?.payload ?? item.evidence ?? {}
    return {
      ...item,
      evidence,
      created_at: item.payload?.occurred_at || evidence.reported_at || item.created_at,
      occurrenceIndex: occurrenceIndex++,
      recordingId: evidence.params?.recording_id || evidence.recording_id || '',
    }
  })
}
