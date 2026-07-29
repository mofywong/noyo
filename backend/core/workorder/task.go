package workorder

import (
	"fmt"
	"strings"
)

const (
	TaskPending   = "pending"
	TaskApproved  = "approved"
	TaskRejected  = "rejected"
	TaskCancelled = "cancelled"
	TaskExpired   = "expired"
)

type ApprovalTask struct {
	PublicID          string   `json:"public_id"`
	WorkOrderPublicID string   `json:"work_order_public_id"`
	NodeKey           string   `json:"node_key"`
	Mode              string   `json:"mode"`
	CandidateRefs     []string `json:"candidate_refs"`
	Status            string   `json:"status"`
	Version           int      `json:"version"`
}

func (t ApprovalTask) CanDecide(principal string) bool {
	if t.Status != TaskPending {
		return false
	}
	for _, ref := range t.CandidateRefs {
		if ref == principal {
			return true
		}
	}
	return false
}

func DecideTask(task *ApprovalTask, principal, decision string) error {
	if task == nil || !task.CanDecide(principal) {
		return NewError(CodePermissionDenied, "principal is not a pending candidate")
	}
	switch strings.ToLower(decision) {
	case "approve":
		task.Status = TaskApproved
	case "reject":
		task.Status = TaskRejected
	default:
		return NewError(CodeValidationFailed, fmt.Sprintf("unsupported decision %q", decision))
	}
	task.Version++
	return nil
}

func ApprovalComplete(mode string, tasks []ApprovalTask) bool {
	if len(tasks) == 0 {
		return false
	}
	approved := 0
	for _, task := range tasks {
		if task.Status == TaskApproved {
			approved++
		}
	}
	if strings.EqualFold(mode, "any") {
		return approved >= 1
	}
	if strings.EqualFold(mode, "all") {
		return approved == len(tasks)
	}
	return false
}
