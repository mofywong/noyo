package workorder

import (
	"fmt"
	"sort"
	"strings"
)

// WorkflowDefinition is the transport-safe workflow DSL. It deliberately
// contains no persistence IDs; a published revision is immutable.
type WorkflowDefinition struct {
	InitialStatus string         `json:"initial_status"`
	Nodes         []WorkflowNode `json:"nodes"`
	Edges         []WorkflowEdge `json:"edges"`
	EndStatuses   []string       `json:"end_statuses"`
}

type WorkflowNode struct {
	Key      string `json:"key"`
	Name     string `json:"name"`
	Category string `json:"category"`
}

type WorkflowEdge struct {
	Key      string         `json:"key"`
	From     string         `json:"from"`
	To       string         `json:"to"`
	Guard    map[string]any `json:"guard,omitempty"`
	Approval *ApprovalSpec  `json:"approval,omitempty"`
}

type ApprovalSpec struct {
	Mode           string   `json:"mode"`
	ApproverRefs   []string `json:"approver_refs"`
	RejectTo       string   `json:"reject_to"`
	TimeoutSeconds int      `json:"timeout_seconds"`
}

func ValidateWorkflowDefinition(def WorkflowDefinition) error {
	if strings.TrimSpace(def.InitialStatus) == "" {
		return fmt.Errorf("initial status is required")
	}
	nodes := map[string]WorkflowNode{}
	for _, node := range def.Nodes {
		if strings.TrimSpace(node.Key) == "" || nodes[node.Key].Key != "" {
			return fmt.Errorf("workflow node key is missing or duplicated")
		}
		nodes[node.Key] = node
	}
	if _, ok := nodes[def.InitialStatus]; !ok {
		return fmt.Errorf("initial status %q is not defined", def.InitialStatus)
	}
	ends := map[string]bool{}
	for _, key := range def.EndStatuses {
		if _, ok := nodes[key]; !ok {
			return fmt.Errorf("end status %q is not defined", key)
		}
		ends[key] = true
	}
	if len(ends) == 0 {
		return fmt.Errorf("workflow requires an end status")
	}
	adj := map[string][]string{}
	for _, edge := range def.Edges {
		if edge.Key == "" || nodes[edge.From].Key == "" || nodes[edge.To].Key == "" {
			return fmt.Errorf("workflow edge %q references an unknown node", edge.Key)
		}
		if edge.Guard != nil {
			if _, ok := edge.Guard["field"]; !ok {
				return fmt.Errorf("workflow guard on %q requires field", edge.Key)
			}
			if _, ok := edge.Guard["equals"]; !ok {
				return fmt.Errorf("workflow guard on %q requires equals", edge.Key)
			}
		}
		if edge.Approval != nil {
			mode := strings.ToLower(edge.Approval.Mode)
			if mode != "any" && mode != "all" {
				return fmt.Errorf("workflow approval %q has invalid mode", edge.Key)
			}
			if len(edge.Approval.ApproverRefs) == 0 || edge.Approval.RejectTo == "" {
				return fmt.Errorf("workflow approval %q is incomplete", edge.Key)
			}
			if _, ok := nodes[edge.Approval.RejectTo]; !ok {
				return fmt.Errorf("workflow approval %q reject status is unknown", edge.Key)
			}
		}
		adj[edge.From] = append(adj[edge.From], edge.To)
	}
	seen := map[string]bool{}
	var visit func(string)
	visit = func(key string) {
		if seen[key] {
			return
		}
		seen[key] = true
		for _, next := range adj[key] {
			visit(next)
		}
	}
	visit(def.InitialStatus)
	for key := range nodes {
		if !seen[key] {
			return fmt.Errorf("workflow node %q is unreachable", key)
		}
	}
	return nil
}

func (d WorkflowDefinition) AvailableEdges(status string, values map[string]any) []WorkflowEdge {
	result := make([]WorkflowEdge, 0)
	for _, edge := range d.Edges {
		if edge.From != status || !guardMatches(edge.Guard, values) {
			continue
		}
		result = append(result, edge)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Key < result[j].Key })
	return result
}

func guardMatches(guard map[string]any, values map[string]any) bool {
	if len(guard) == 0 {
		return true
	}
	field, _ := guard["field"].(string)
	return fmt.Sprint(values[field]) == fmt.Sprint(guard["equals"])
}
