package core

import (
	"strings"
	"unicode/utf8"
)

// Selectable goals are constrained by the executable shape. A task never
// implies an additional query, threshold, target, relation or data provider.
func screenAssistantCandidateTasks(shape string) []string {
	switch shape {
	case "scalar", "states":
		return []string{"current"}
	case "series":
		return []string{"change", "compare", "diagnose"}
	case "distribution":
		return []string{"compare", "composition"}
	case "events", "alarms":
		return []string{"respond"}
	case "map":
		return []string{"locate"}
	}
	return nil
}

// A boolean property is a per-device on/off state. Its distribution counts how
// many devices sit in each state, and devices that never reported are excluded,
// so the two buckets are not parts of a whole: offering "composition" would let
// the host render a single remaining state as a 100% ring. Boolean states are
// only ever compared. A multi-value enumeration can still be a composition.
func screenAssistantStateDistribution(c screenAssistantMetricCandidate) bool {
	if c.Shape != "distribution" || c.Field == nil {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(c.Field.Type)) {
	case "bool", "boolean":
		return true
	}
	return false
}

func screenAssistantCandidateTasksFor(c screenAssistantMetricCandidate) []string {
	if screenAssistantStateDistribution(c) {
		return []string{"compare"}
	}
	return screenAssistantCandidateTasks(c.Shape)
}

func screenAssistantCandidateTask(c screenAssistantMetricCandidate) string {
	tasks := screenAssistantCandidateTasksFor(c)
	if len(tasks) == 0 {
		return ""
	}
	if screenAssistantContains(tasks, c.Metric.Task) {
		return c.Metric.Task
	}
	return tasks[0]
}

// A declared state list is emitted in full, so its length is the exact category
// count. Undeclared cardinality stays zero and cannot justify automatically
// selecting a part-to-whole graphic.
func screenAssistantCategoryCount(c screenAssistantMetricCandidate) int {
	if c.Shape != "distribution" || c.Field == nil {
		return 0
	}
	if len(c.Field.Values) > 0 {
		return len(c.Field.Values)
	}
	if c.Field.Type == "bool" || c.Field.Type == "boolean" {
		return 2
	}
	return 0
}

func screenAssistantDesignQuestion(value string) string {
	value = strings.TrimSpace(value)
	// Sections have a byte limit; approved metric questions may be longer.
	if len(value) > 256 {
		value = value[:256]
		for !utf8.ValidString(value) {
			value = value[:len(value)-1]
		}
	}
	return value
}
