package panels

import (
	"fmt"
	"strings"
	"time"

	"github.com/dshills/witness/internal/aggregate"
	"github.com/dshills/witness/internal/tui"

	tea "github.com/charmbracelet/bubbletea"
)

// ActiveWorkPanel shows the currently active tool and model with latency info.
type ActiveWorkPanel struct {
	state aggregate.RunState
}

// NewActiveWorkPanel creates a new ActiveWorkPanel.
func NewActiveWorkPanel() *ActiveWorkPanel {
	return &ActiveWorkPanel{}
}

func (p *ActiveWorkPanel) Init() tea.Cmd { return nil }

func (p *ActiveWorkPanel) Update(msg tea.Msg) (tui.Panel, tea.Cmd) {
	if sm, ok := msg.(tui.StateMsg); ok {
		p.state = sm.State
	}
	return p, nil
}

func (p *ActiveWorkPanel) View(width, height int) string {
	var lines []string

	// Active tool.
	if t := p.state.ActiveTool; t != nil {
		dur := time.Since(t.StartedAt).Truncate(time.Millisecond)
		line := fmt.Sprintf(" Tool: %s (%s)", t.ToolName, dur)
		if t.Summary != "" {
			maxS := width - len(line) - 2
			if maxS > 5 {
				s := t.Summary
				if len(s) > maxS {
					s = s[:maxS-3] + "..."
				}
				line += " " + s
			}
		}
		lines = append(lines, padRight(line, width))
	} else {
		lines = append(lines, padRight(" Tool: (idle)", width))
	}

	// Active model.
	if m := p.state.ActiveModel; m != nil {
		dur := time.Since(m.StartedAt).Truncate(time.Millisecond)
		line := fmt.Sprintf(" Model: %s/%s (%s)", m.Provider, m.Model, dur)
		if m.Purpose != "" {
			line += fmt.Sprintf(" [%s]", m.Purpose)
		}
		lines = append(lines, padRight(line, width))
	} else {
		lines = append(lines, padRight(" Model: (idle)", width))
	}

	// Request counts.
	toolCount := len(p.state.ToolInvocations)
	modelCount := len(p.state.ModelRequests)
	lines = append(lines, padRight(
		fmt.Sprintf(" Requests: tools=%d models=%d", toolCount, modelCount), width))

	// Latency.
	avgTool := p.state.AvgToolLatency().Truncate(time.Millisecond)
	avgModel := p.state.AvgModelLatency().Truncate(time.Millisecond)
	lines = append(lines, padRight(
		fmt.Sprintf(" Avg latency: tool=%s model=%s", avgTool, avgModel), width))

	// Retries.
	if p.state.RetryCount > 0 {
		lines = append(lines, padRight(
			fmt.Sprintf(" Retries: %d", p.state.RetryCount), width))
	}

	// Pad remaining height.
	for len(lines) < height {
		lines = append(lines, padRight("", width))
	}
	if len(lines) > height {
		lines = lines[:height]
	}

	return strings.Join(lines, "\n")
}

func (p *ActiveWorkPanel) Title() string   { return "Active Work" }
func (p *ActiveWorkPanel) Focusable() bool { return true }
