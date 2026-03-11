package replay

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/dshills/witness/internal/aggregate"
	"github.com/dshills/witness/internal/models"
)

// GeneratePostmortem produces a concise text summary of a completed run,
// covering run info, duration, outcome, stage progression, failures,
// cost summary, commits, and notable alerts.
func GeneratePostmortem(state aggregate.RunState) string {
	var b strings.Builder

	// Run info
	b.WriteString("=== Postmortem Summary ===\n\n")
	b.WriteString("--- Run ---\n")
	fmt.Fprintf(&b, "  Name:     %s\n", state.Run.Name)
	fmt.Fprintf(&b, "  ID:       %s\n", state.Run.RunID)
	if state.Run.RepoRoot != "" {
		fmt.Fprintf(&b, "  Repo:     %s\n", state.Run.RepoRoot)
	}
	if state.Run.Branch != "" {
		fmt.Fprintf(&b, "  Branch:   %s\n", state.Run.Branch)
	}
	if len(state.Run.Command) > 0 {
		fmt.Fprintf(&b, "  Command:  %s\n", strings.Join(state.Run.Command, " "))
	}
	b.WriteString("\n")

	// Duration and outcome
	b.WriteString("--- Outcome ---\n")
	fmt.Fprintf(&b, "  Status:   %s\n", state.Run.Status)
	dur := state.Duration()
	fmt.Fprintf(&b, "  Duration: %s\n", dur.Truncate(time.Second))
	if state.FailureCount > 0 {
		fmt.Fprintf(&b, "  Failures: %d\n", state.FailureCount)
	}
	b.WriteString("\n")

	// Stage progression
	if len(state.Stages) > 0 {
		b.WriteString("--- Stages ---\n")
		stageDurations := state.StageDurations()
		for i := range state.Stages {
			st := &state.Stages[i]
			icon := stageIcon(st.Status)
			durStr := "-"
			if d, ok := stageDurations[st.Name]; ok {
				durStr = d.Truncate(time.Second).String()
			}
			fmt.Fprintf(&b, "  %s %s (%s) [%s]\n", icon, st.Name, st.Status, durStr)
		}
		b.WriteString("\n")
	}

	// Cost summary
	if state.TotalInputTokens > 0 || state.TotalOutputTokens > 0 || state.TotalCostUSD > 0 {
		b.WriteString("--- Cost ---\n")
		fmt.Fprintf(&b, "  Input tokens:  %d\n", state.TotalInputTokens)
		fmt.Fprintf(&b, "  Output tokens: %d\n", state.TotalOutputTokens)
		if state.TotalCachedTokens > 0 {
			fmt.Fprintf(&b, "  Cached tokens: %d\n", state.TotalCachedTokens)
		}
		fmt.Fprintf(&b, "  Total cost:    $%.4f\n", state.TotalCostUSD)

		// Cost by model (top 3)
		if len(state.CostByModel) > 0 {
			type modelCost struct {
				model string
				cost  float64
			}
			var mc []modelCost
			for m, c := range state.CostByModel {
				mc = append(mc, modelCost{m, c})
			}
			sort.Slice(mc, func(i, j int) bool {
				return mc[i].cost > mc[j].cost
			})
			limit := 3
			if len(mc) < limit {
				limit = len(mc)
			}
			b.WriteString("  By model:\n")
			for _, m := range mc[:limit] {
				fmt.Fprintf(&b, "    %s: $%.4f\n", m.model, m.cost)
			}
		}
		b.WriteString("\n")
	}

	// Commits
	if len(state.Commits) > 0 {
		b.WriteString("--- Commits ---\n")
		fmt.Fprintf(&b, "  Total: %d\n", len(state.Commits))
		fmt.Fprintf(&b, "  Files touched: %d\n", state.UniqueFilesTouched())
		limit := 5
		if len(state.Commits) < limit {
			limit = len(state.Commits)
		}
		for _, c := range state.Commits[:limit] {
			sha := c.SHA
			if len(sha) > 7 {
				sha = sha[:7]
			}
			msg := c.Message
			if len(msg) > 60 {
				msg = msg[:57] + "..."
			}
			fmt.Fprintf(&b, "  %s %s\n", sha, msg)
		}
		b.WriteString("\n")
	}

	// Notable alerts
	if len(state.Alerts) > 0 {
		b.WriteString("--- Alerts ---\n")
		fmt.Fprintf(&b, "  Total: %d\n", len(state.Alerts))
		limit := 10
		if len(state.Alerts) < limit {
			limit = len(state.Alerts)
		}
		for _, a := range state.Alerts[:limit] {
			fmt.Fprintf(&b, "  [%s] %s: %s\n", a.Severity, a.Title, a.Description)
		}
		b.WriteString("\n")
	}

	return b.String()
}

func stageIcon(status models.StageStatus) string {
	switch status {
	case models.StageStatusCompleted:
		return "[OK]"
	case models.StageStatusFailed:
		return "[FAIL]"
	case models.StageStatusRunning:
		return "[..]"
	case models.StageStatusSkipped:
		return "[SKIP]"
	default:
		return "[  ]"
	}
}
