package panels

import (
	"fmt"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/dshills/witness/internal/aggregate"
	"github.com/dshills/witness/internal/models"
	"github.com/dshills/witness/internal/tui"

	tea "github.com/charmbracelet/bubbletea"
)

// HeaderPanel shows run name, ID, branch, duration, and status.
type HeaderPanel struct {
	state aggregate.RunState
}

// NewHeaderPanel creates a new HeaderPanel.
func NewHeaderPanel() *HeaderPanel {
	return &HeaderPanel{}
}

func (p *HeaderPanel) Init() tea.Cmd { return nil }

func (p *HeaderPanel) Update(msg tea.Msg) (tui.Panel, tea.Cmd) {
	if sm, ok := msg.(tui.StateMsg); ok {
		p.state = sm.State
	}
	return p, nil
}

func (p *HeaderPanel) View(width, _ int) string {
	run := p.state.Run

	name := run.Name
	if name == "" {
		name = run.RunID
	}
	if name == "" {
		name = "(no run)"
	}

	statusStyle := statusColor(run.Status)
	statusStr := statusStyle.Render(run.Status.String())

	dur := p.state.Duration().Truncate(time.Second).String()

	left := fmt.Sprintf(" %s  [%s]", name, truncate(run.RunID, 20))
	right := fmt.Sprintf("%s  %s ", statusStr, dur)

	if run.Branch != "" {
		left += fmt.Sprintf("  branch:%s", run.Branch)
	}

	gap := width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}

	padding := ""
	for range gap {
		padding += " "
	}

	return left + padding + right
}

func (p *HeaderPanel) Title() string   { return "Header" }
func (p *HeaderPanel) Focusable() bool { return false }

func statusColor(status models.RunStatus) lipgloss.Style {
	switch status {
	case models.RunStatusRunning:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("10")) // green
	case models.RunStatusFailed:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("9")) // red
	case models.RunStatusStalled:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("11")) // yellow
	case models.RunStatusCompleted:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("12")) // blue
	default:
		return lipgloss.NewStyle()
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}
