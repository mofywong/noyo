package workorder

const (
	ActionAssign   = "assign"
	ActionReassign = "reassign"
	ActionStart    = "start"
	ActionPause    = "pause"
	ActionResume   = "resume"
	ActionResolve  = "resolve"
	ActionAccept   = "accept"
	ActionClose    = "close"
	ActionReopen   = "reopen"
	ActionCancel   = "cancel"
	ActionWithdraw = "withdraw"
)

// AllowedActions is shared by API and UI so the action list cannot advertise
// a transition that a pending approval blocks.
func AllowedActions(status string, pendingApproval bool) []string {
	if pendingApproval {
		return []string{ActionWithdraw}
	}
	switch status {
	case "open":
		return []string{ActionAssign, ActionStart, ActionCancel}
	case "assigned":
		return []string{ActionReassign, ActionStart, ActionCancel}
	case "in_progress":
		return []string{ActionPause, ActionResolve, ActionCancel}
	case "paused":
		return []string{ActionResume, ActionCancel}
	case "resolved":
		return []string{ActionAccept, ActionReopen}
	case "accepted":
		return []string{ActionClose, ActionReopen}
	case "closed", "cancelled":
		return []string{ActionReopen}
	default:
		return nil
	}
}
