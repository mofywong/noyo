package core

import (
	"encoding/json"
	"noyo/core/store"
)

// Public progress contains validated outcomes, never model reasoning or raw
// checkpoint content. It is rebuilt from durable artifacts after reconnects.
type screenAssistantStepStatus struct {
	ID     string `json:"id"`
	State  string `json:"state"`
	Count  int    `json:"count,omitempty"`
	Detail string `json:"detail,omitempty"`
}

func screenAssistantTaskSteps(job store.IOCScreenAssistantJob) []screenAssistantStepStatus {
	var checkpoints map[string]screenAssistantCheckpoint
	_ = json.Unmarshal([]byte(job.Checkpoints), &checkpoints)
	ids := []string{"intention", "metrics", "checking", "generating", "reviewing"}
	var outcome struct {
		Clarification string `json:"clarification"`
	}
	_ = json.Unmarshal([]byte(job.Result), &outcome)
	stage := job.Stage
	if stage == "validating" { // Earlier tasks used the same name twice.
		stage = "checking"
		if _, ok := checkpoints["generating"]; ok {
			stage = "reviewing"
		}
	}
	steps := make([]screenAssistantStepStatus, 0, len(ids))
	for _, id := range ids {
		step := screenAssistantStepStatus{ID: id, State: "pending"}
		if checkpoint, ok := checkpoints[id]; ok {
			step.State = "succeeded"
			var value struct {
				Goal       string            `json:"goal"`
				Selected   []json.RawMessage `json:"selected"`
				Metrics    []json.RawMessage `json:"metrics"`
				Sections   []json.RawMessage `json:"sections"`
				Operations []json.RawMessage `json:"operations"`
				Count      int               `json:"count"`
			}
			if json.Unmarshal([]byte(checkpoint.Content), &value) == nil {
				step.Count = value.Count + len(value.Selected) + len(value.Metrics) + len(value.Sections) + len(value.Operations)
				step.Detail = screenAssistantText(value.Goal, 240)
			}
		}
		if id == stage && step.State != "succeeded" {
			switch job.State {
			case "running":
				step.State = "running"
			case "failed":
				step.State = "failed"
			case "canceled":
				step.State = "canceled"
			}
		}
		if job.State == "completed" {
			if outcome.Clarification != "" && step.State != "succeeded" {
				step.State = "skipped"
			} else {
				step.State = "succeeded"
			}
		}
		steps = append(steps, step)
	}
	return steps
}
