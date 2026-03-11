package panels

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/dshills/witness/internal/aggregate"
	"github.com/dshills/witness/internal/models"
	"github.com/dshills/witness/internal/tui"

	tea "github.com/charmbracelet/bubbletea"
)

// StagePanel shows an ordered list of stages with status icons.
type StagePanel struct {
	state  aggregate.RunState
	offset int
}

// NewStagePanel creates a new StagePanel.
func NewStagePanel() *StagePanel {
	return &StagePanel{}
}

func (p *StagePanel) Init() tea.Cmd { return nil }

func (p *StagePanel) Update(msg tea.Msg) (tui.Panel, tea.Cmd) {
	switch m := msg.(type) {
	case tui.StateMsg:
		p.state = m.State
	case tea.KeyMsg:
		switch m.String() {
		case "j", "down":
			p.offset++
		case "k", "up":
			if p.offset > 0 {
				p.offset--
			}
		}
	}
	return p, nil
}

func (p *StagePanel) View(width, height int) string {
	stages := p.state.Stages
	if len(stages) == 0 {
		return padRight("  (no stages)", width)
	}

	// Sort by order.
	sorted := make([]models.Stage, len(stages))
	copy(sorted, stages)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Order < sorted[j].Order
	})

	// Clamp offset.
	maxOffset := len(sorted) - height
	if maxOffset < 0 {
		maxOffset = 0
	}
	if p.offset > maxOffset {
		p.offset = maxOffset
	}

	var lines []string
	for i := p.offset; i < len(sorted) && len(lines) < height; i++ {
		st := &sorted[i]
		icon := stageIcon(st.Status)
		line := fmt.Sprintf(" %s %s", icon, st.Name)

		if st.ProgressPercent != nil {
			line += fmt.Sprintf(" %s", progressBar(*st.ProgressPercent, 10))
		}
		if st.Status == models.StageStatusRunning && st.StartedAt != nil {
			dur := time.Since(*st.StartedAt).Truncate(time.Second)
			line += fmt.Sprintf(" %s", dur)
		}
		if st.Summary != "" {
			maxSummary := width - len(line) - 2
			if maxSummary > 5 {
				summary := st.Summary
				if len(summary) > maxSummary {
					summary = summary[:maxSummary-3] + "..."
				}
				line += " " + summary
			}
		}

		lines = append(lines, padRight(line, width))
	}

	// Pad remaining height.
	for len(lines) < height {
		lines = append(lines, padRight("", width))
	}

	return strings.Join(lines, "\n")
}

func (p *StagePanel) Title() string   { return "Stages" }
func (p *StagePanel) Focusable() bool { return true }

func stageIcon(status models.StageStatus) string {
	switch status {
	case models.StageStatusCompleted:
		return "\u2713" // checkmark
	case models.StageStatusRunning:
		return "\u25b8" // right triangle
	case models.StageStatusFailed:
		return "\u2717" // X mark
	case models.StageStatusSkipped:
		return "\u2013" // en dash
	default:
		return "\u25cb" // circle
	}
}

func progressBar(pct float64, width int) string {
	filled := int(pct / 100.0 * float64(width))
	if filled > width {
		filled = width
	}
	if filled < 0 {
		filled = 0
	}
	return "[" + strings.Repeat("#", filled) + strings.Repeat("-", width-filled) + "]"
}

func padRight(s string, width int) string {
	if len(s) >= width {
		return s[:width]
	}
	return s + strings.Repeat(" ", width-len(s))
}
