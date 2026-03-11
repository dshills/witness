package export

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/dshills/witness/internal/aggregate"
	"github.com/dshills/witness/internal/events"
)

// MarkdownExporter writes a human-readable Markdown report.
type MarkdownExporter struct{}

// Export writes the run report as Markdown.
func (e *MarkdownExporter) Export(_ context.Context, state aggregate.RunState, _ []events.Event, w io.Writer) error {
	var b strings.Builder

	name := state.Run.Name
	if name == "" {
		name = state.Run.RunID
	}
	fmt.Fprintf(&b, "# Run Report: %s\n\n", name)

	// Summary
	b.WriteString("## Summary\n\n")
	fmt.Fprintf(&b, "- **Run ID**: %s\n", state.Run.RunID)
	fmt.Fprintf(&b, "- **Status**: %s\n", state.Run.Status)
	fmt.Fprintf(&b, "- **Duration**: %s\n", state.Duration().Truncate(time.Millisecond)) // truncate to ms
	fmt.Fprintf(&b, "- **Total Cost**: $%.2f\n", state.TotalCostUSD)
	if state.Run.RepoRoot != "" {
		fmt.Fprintf(&b, "- **Repository**: %s\n", state.Run.RepoRoot)
	}
	if state.Run.Branch != "" {
		fmt.Fprintf(&b, "- **Branch**: %s\n", state.Run.Branch)
	}
	if len(state.Run.Command) > 0 {
		fmt.Fprintf(&b, "- **Command**: `%s`\n", strings.Join(state.Run.Command, " "))
	}
	b.WriteString("\n")

	// Stages
	if len(state.Stages) > 0 {
		b.WriteString("## Stages\n\n")
		b.WriteString("| Stage | Status | Duration |\n")
		b.WriteString("|-------|--------|----------|\n")
		for i := range state.Stages {
			st := &state.Stages[i]
			dur := "-"
			if st.StartedAt != nil {
				if st.EndedAt != nil {
					dur = st.EndedAt.Sub(*st.StartedAt).Truncate(time.Millisecond).String()
				} else {
					dur = "running"
				}
			}
			fmt.Fprintf(&b, "| %s | %s | %s |\n", st.Name, st.Status, dur)
		}
		b.WriteString("\n")
	}

	// Token Usage
	if state.TotalInputTokens > 0 || state.TotalOutputTokens > 0 {
		b.WriteString("## Token Usage\n\n")
		fmt.Fprintf(&b, "- **Input Tokens**: %d\n", state.TotalInputTokens)
		fmt.Fprintf(&b, "- **Output Tokens**: %d\n", state.TotalOutputTokens)
		fmt.Fprintf(&b, "- **Cached Tokens**: %d\n", state.TotalCachedTokens)
		b.WriteString("\n")

		if len(state.TokensByModel) > 0 {
			b.WriteString("### By Model\n\n")
			b.WriteString("| Model | Input | Output | Cached | Cost |\n")
			b.WriteString("|-------|-------|--------|--------|------|\n")
			models := make([]string, 0, len(state.TokensByModel))
			for k := range state.TokensByModel {
				models = append(models, k)
			}
			sort.Strings(models)
			for _, model := range models {
				tc := state.TokensByModel[model]
				cost := state.CostByModel[model]
				fmt.Fprintf(&b, "| %s | %d | %d | %d | $%.4f |\n", model, tc.Input, tc.Output, tc.Cached, cost)
			}
			b.WriteString("\n")
		}
	}

	// Commits
	if len(state.Commits) > 0 {
		b.WriteString("## Commits\n\n")
		for i := range state.Commits {
			c := &state.Commits[i]
			msg := c.Message
			if idx := strings.IndexByte(msg, '\n'); idx >= 0 {
				msg = msg[:idx]
			}
			fmt.Fprintf(&b, "- `%s` %s\n", shortSHA(c.SHA), msg)
		}
		b.WriteString("\n")
	}

	// Alerts
	if len(state.Alerts) > 0 {
		b.WriteString("## Alerts\n\n")
		for i := range state.Alerts {
			a := &state.Alerts[i]
			fmt.Fprintf(&b, "- **[%s]** %s: %s\n", a.Severity, a.Title, a.Description)
		}
		b.WriteString("\n")
	}

	_, err := io.WriteString(w, b.String())
	return err
}

func shortSHA(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}
