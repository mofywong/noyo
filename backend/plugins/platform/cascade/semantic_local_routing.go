package cascade

import (
	"noyo/core/semanticreview"
	"sort"
)

type localSemanticClient interface {
	semanticreview.Client
	LocalSemanticStatus(string) semanticreview.Status
}

func (p *CascadePlugin) localSemanticClient() localSemanticClient {
	ctx := p.ctx
	if ctx == nil {
		ctx = p.Ctx
	}
	if ctx == nil {
		return nil
	}
	c, _ := ctx.GetPlugin("ai_copilot").(localSemanticClient)
	return c
}
func localUnavailableReason(status semanticreview.Status) string {
	if status.Status == "disabled" {
		return "local_disabled"
	}
	if status.Status == "no_provider" {
		return "local_no_provider"
	}
	return ""
}
func mergedSemanticResults(local, platform []semanticreview.Result) []semanticreview.Result {
	out := append([]semanticreview.Result(nil), local...)
	indexes := map[string]int{}
	for i, r := range out {
		indexes[r.ID] = i
	}
	for _, r := range platform {
		if i, exists := indexes[r.ID]; exists {
			if r.AttemptsUsed > out[i].AttemptsUsed || r.CompletedAt.After(out[i].CompletedAt) {
				out[i] = r
			}
		} else {
			indexes[r.ID] = len(out)
			out = append(out, r)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].ObservedAt.After(out[j].ObservedAt) })
	if len(out) > 50 {
		out = out[:50]
	}
	return out
}
