package inspection

import (
	"encoding/json"

	"gorm.io/gorm"
)

// RunSummary describes observation results independently of execution status.
type RunSummary struct {
	Total    int `json:"total"`
	Checked  int `json:"checked"`
	Passed   int `json:"passed"`
	Issues   int `json:"issues"`
	Rejected int `json:"rejected"`
	Unable   int `json:"unable"`
	Pending  int `json:"pending"`
}

// EffectiveItemResult applies the review without replacing the original AI fact.
// Dismissing a detection does not prove that unobserved scenes passed.
func EffectiveItemResult(item Item) string {
	if item.Result == "issue" && item.ReviewStatus == "rejected" {
		if item.Coverage == "partial" || item.Coverage == "none" {
			return "unable"
		}
		return "rejected"
	}
	return item.Result
}

// NeedsReinspection preserves incomplete observations even when another scene
// found an issue. Missing coverage and scene data remain compatible with older
// completed records, which only stored an aggregate result.
func NeedsReinspection(item Item) bool {
	if item.Status != "done" || item.Result == "unable" || item.Coverage == "partial" || item.Coverage == "none" {
		return true
	}
	var scenes []SceneResult
	if json.Unmarshal([]byte(item.SceneResultsJSON), &scenes) == nil {
		for _, scene := range scenes {
			if scene.Result == "unable" {
				return true
			}
		}
	}
	return false
}

// PopulateRunSummaries derives historical and current outcomes in one bounded
// query. Incomplete or failed observations can never produce a clean result.
func PopulateRunSummaries(db *gorm.DB, runs []Run) error {
	if len(runs) == 0 {
		return nil
	}
	ids := make([]string, 0, len(runs))
	for _, run := range runs {
		ids = append(ids, run.ID)
	}
	var counts []struct {
		RunID, Status, Result, ReviewStatus, Coverage string
		Count                                         int
	}
	if err := db.Model(&Item{}).Select("run_id, status, result, review_status, coverage, COUNT(*) AS count").Where("run_id IN ?", ids).Group("run_id, status, result, review_status, coverage").Scan(&counts).Error; err != nil {
		return err
	}
	summaries := map[string]RunSummary{}
	for _, count := range counts {
		summary := summaries[count.RunID]
		summary.Total += count.Count
		if count.Status != "done" {
			summary.Pending += count.Count
		} else {
			summary.Checked += count.Count
			switch EffectiveItemResult(Item{Result: count.Result, ReviewStatus: count.ReviewStatus, Coverage: count.Coverage}) {
			case "pass":
				summary.Passed += count.Count
			case "issue":
				summary.Issues += count.Count
			case "rejected":
				summary.Rejected += count.Count
			default:
				summary.Unable += count.Count
			}
		}
		summaries[count.RunID] = summary
	}
	for n := range runs {
		run := &runs[n]
		run.Summary = summaries[run.ID]
		s := run.Summary
		switch {
		case s.Issues > 0:
			run.Result = "issue"
		case run.Status == "queued" || run.Status == "running" || run.Status == "paused":
			run.Result = "pending"
		case s.Pending > 0 || s.Total == 0 || run.Status == "cancelled":
			run.Result = "incomplete"
		case s.Unable > 0 || run.Status == "failed" || run.Status == "partial":
			run.Result = "unable"
		case s.Rejected > 0 && s.Passed+s.Rejected == s.Total:
			run.Result = "rejected"
		case s.Passed == s.Total:
			run.Result = "pass"
		default:
			run.Result = "incomplete"
		}
	}
	return nil
}

func migrateExecutionCounts(db *gorm.DB) error {
	// Never reduce a durable counter if old runs were subsequently removed.
	count := "(SELECT COUNT(*) FROM inspection_runs WHERE inspection_runs.plan_id = inspection_plans.id AND inspection_runs.tenant_id = inspection_plans.tenant_id AND inspection_runs.project_id = inspection_plans.project_id)"
	return db.Model(&Plan{}).Where("execution_count < "+count).UpdateColumn("execution_count", gorm.Expr(count)).Error
}
