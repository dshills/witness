package panels

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/dshills/witness/internal/aggregate"
	"github.com/dshills/witness/internal/tui"

	tea "github.com/charmbracelet/bubbletea"
)

// TokenCostPanel shows total tokens, cost, burn rate, and budget warnings.
type TokenCostPanel struct {
	state       aggregate.RunState
	budgetLimit float64
}

// NewTokenCostPanel creates a new TokenCostPanel with an optional budget limit.
func NewTokenCostPanel(budgetLimit float64) *TokenCostPanel {
	return &TokenCostPanel{budgetLimit: budgetLimit}
}

func (p *TokenCostPanel) Init() tea.Cmd { return nil }

func (p *TokenCostPanel) Update(msg tea.Msg) (tui.Panel, tea.Cmd) {
	if sm, ok := msg.(tui.StateMsg); ok {
		p.state = sm.State
	}
	return p, nil
}

func (p *TokenCostPanel) View(width, height int) string {
	var lines []string

	total := p.state.TotalInputTokens + p.state.TotalOutputTokens + p.state.TotalCachedTokens
	lines = append(lines, padRight(
		fmt.Sprintf(" Tokens: %s (in:%s out:%s cached:%s)",
			formatTokens(total),
			formatTokens(p.state.TotalInputTokens),
			formatTokens(p.state.TotalOutputTokens),
			formatTokens(p.state.TotalCachedTokens)),
		width))

	// Cost.
	costLine := fmt.Sprintf(" Cost: $%.4f", p.state.TotalCostUSD)
	if p.budgetLimit > 0 && p.state.TotalCostUSD > p.budgetLimit {
		warnStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
		costLine += warnStyle.Render(" OVER BUDGET")
	}
	lines = append(lines, padRight(costLine, width))

	// Burn rate.
	burnRate := p.state.TokenBurnRate(5 * time.Minute)
	costBurn := p.state.CostBurnRate(5 * time.Minute)
	if burnRate > 0 {
		lines = append(lines, padRight(
			fmt.Sprintf(" Burn: %.0f tok/min  $%.4f/min", burnRate*60, costBurn*60), width))
	} else {
		lines = append(lines, padRight(" Burn: -", width))
	}

	// Top models by cost (up to 3).
	type modelCost struct {
		name string
		cost float64
	}
	var mc []modelCost
	for m, c := range p.state.CostByModel {
		mc = append(mc, modelCost{name: m, cost: c})
	}
	sort.Slice(mc, func(i, j int) bool { return mc[i].cost > mc[j].cost })
	if len(mc) > 3 {
		mc = mc[:3]
	}
	for _, m := range mc {
		lines = append(lines, padRight(
			fmt.Sprintf("   %s: $%.4f", m.name, m.cost), width))
	}

	// Pad remaining.
	for len(lines) < height {
		lines = append(lines, padRight("", width))
	}
	if len(lines) > height {
		lines = lines[:height]
	}

	return strings.Join(lines, "\n")
}

func (p *TokenCostPanel) Title() string   { return "Tokens / Cost" }
func (p *TokenCostPanel) Focusable() bool { return true }

func formatTokens(n int64) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%.1fK", float64(n)/1_000)
	default:
		return fmt.Sprintf("%d", n)
	}
}
