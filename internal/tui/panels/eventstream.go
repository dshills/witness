package panels

import (
	"fmt"
	"strings"
	"time"

	"github.com/dshills/witness/internal/aggregate"
	"github.com/dshills/witness/internal/events"
	"github.com/dshills/witness/internal/tui"

	tea "github.com/charmbracelet/bubbletea"
)

// EventStreamPanel shows a scrollable list of recent events.
type EventStreamPanel struct {
	state   aggregate.RunState
	offset  int
	paused  bool
	filter  string
	editing bool // true when typing a filter
	input   string
}

// NewEventStreamPanel creates a new EventStreamPanel.
func NewEventStreamPanel() *EventStreamPanel {
	return &EventStreamPanel{}
}

func (p *EventStreamPanel) Init() tea.Cmd { return nil }

func (p *EventStreamPanel) Update(msg tea.Msg) (tui.Panel, tea.Cmd) {
	switch m := msg.(type) {
	case tui.StateMsg:
		p.state = m.State
		if !p.paused {
			// Auto-scroll to bottom.
			p.offset = len(p.filteredEvents()) // will be clamped in View
		}
	case tea.KeyMsg:
		if p.editing {
			return p.updateFilter(m)
		}
		switch m.String() {
		case "j", "down":
			p.offset++
		case "k", "up":
			if p.offset > 0 {
				p.offset--
			}
		case "p":
			p.paused = true
		case "r":
			p.paused = false
			p.offset = len(p.filteredEvents())
		case "/":
			p.editing = true
			p.input = p.filter
		}
	}
	return p, nil
}

func (p *EventStreamPanel) updateFilter(msg tea.KeyMsg) (tui.Panel, tea.Cmd) {
	switch msg.String() {
	case "enter":
		p.filter = p.input
		p.editing = false
		p.offset = 0
	case "esc":
		p.editing = false
	case "backspace":
		if len(p.input) > 0 {
			p.input = p.input[:len(p.input)-1]
		}
	default:
		if len(msg.String()) == 1 {
			p.input += msg.String()
		}
	}
	return p, nil
}

func (p *EventStreamPanel) filteredEvents() []events.Event {
	evts := p.state.RecentEvents
	if p.filter == "" {
		return evts
	}
	var filtered []events.Event
	for i := range evts {
		if strings.Contains(string(evts[i].Type), p.filter) ||
			strings.Contains(evts[i].Source, p.filter) ||
			strings.Contains(evts[i].Summary, p.filter) {
			filtered = append(filtered, evts[i])
		}
	}
	return filtered
}

func (p *EventStreamPanel) View(width, height int) string {
	if p.editing {
		return p.filterView(width, height)
	}

	evts := p.filteredEvents()
	if len(evts) == 0 {
		line := padRight("  (no events)", width)
		lines := []string{line}
		for len(lines) < height {
			lines = append(lines, padRight("", width))
		}
		return strings.Join(lines, "\n")
	}

	// Clamp offset: show last `height` events by default.
	maxOffset := len(evts) - height
	if maxOffset < 0 {
		maxOffset = 0
	}
	if p.offset > maxOffset {
		p.offset = maxOffset
	}
	if p.offset < 0 {
		p.offset = 0
	}

	var lines []string
	for i := p.offset; i < len(evts) && len(lines) < height; i++ {
		e := &evts[i]
		ts := e.Timestamp.Format(time.TimeOnly)
		line := fmt.Sprintf(" [%s] %s %s", ts, e.Type, e.Source)
		if e.Summary != "" {
			line += ": " + e.Summary
		}
		if len(line) > width {
			line = line[:width-3] + "..."
		}
		lines = append(lines, padRight(line, width))
	}

	statusLine := ""
	if p.paused {
		statusLine = " [PAUSED]"
	}
	if p.filter != "" {
		statusLine += fmt.Sprintf(" filter:%s", p.filter)
	}
	if statusLine != "" && len(lines) < height {
		lines = append(lines, padRight(statusLine, width))
	}

	for len(lines) < height {
		lines = append(lines, padRight("", width))
	}
	if len(lines) > height {
		lines = lines[:height]
	}

	return strings.Join(lines, "\n")
}

func (p *EventStreamPanel) filterView(width, height int) string {
	var lines []string
	lines = append(lines, padRight(fmt.Sprintf(" Filter: %s_", p.input), width))
	for len(lines) < height {
		lines = append(lines, padRight("", width))
	}
	return strings.Join(lines, "\n")
}

func (p *EventStreamPanel) Title() string   { return "Event Stream" }
func (p *EventStreamPanel) Focusable() bool { return true }
