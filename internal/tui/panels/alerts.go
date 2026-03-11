package panels

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/dshills/witness/internal/aggregate"
	"github.com/dshills/witness/internal/models"
	"github.com/dshills/witness/internal/tui"

	tea "github.com/charmbracelet/bubbletea"
)

// AlertsPanel shows active alerts with severity colors.
type AlertsPanel struct {
	state aggregate.RunState
}

// NewAlertsPanel creates a new AlertsPanel.
func NewAlertsPanel() *AlertsPanel {
	return &AlertsPanel{}
}

func (p *AlertsPanel) Init() tea.Cmd { return nil }

func (p *AlertsPanel) Update(msg tea.Msg) (tui.Panel, tea.Cmd) {
	if sm, ok := msg.(tui.StateMsg); ok {
		p.state = sm.State
	}
	return p, nil
}

func (p *AlertsPanel) View(width, height int) string {
	alerts := p.state.ActiveAlerts
	if len(alerts) == 0 {
		line := padRight("  (no active alerts)", width)
		lines := []string{line}
		for len(lines) < height {
			lines = append(lines, padRight("", width))
		}
		return strings.Join(lines, "\n")
	}

	var lines []string
	// Most recent first.
	for i := len(alerts) - 1; i >= 0 && len(lines) < height; i-- {
		a := &alerts[i]
		sev := severityStyle(a.Severity)
		label := sev.Render(fmt.Sprintf("[%s]", a.Severity))
		text := fmt.Sprintf(" %s %s: %s", label, a.Title, a.Description)
		if len(text) > width {
			text = text[:width-3] + "..."
		}
		lines = append(lines, padRight(text, width))
	}

	for len(lines) < height {
		lines = append(lines, padRight("", width))
	}
	return strings.Join(lines, "\n")
}

func (p *AlertsPanel) Title() string   { return "Alerts" }
func (p *AlertsPanel) Focusable() bool { return true }

func severityStyle(sev models.Severity) lipgloss.Style {
	switch sev {
	case models.SeverityCritical, models.SeverityError:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("9")) // red
	case models.SeverityWarning:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("11")) // yellow
	default:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("12")) // blue
	}
}
