package panels

import (
	"fmt"
	"sort"
	"strings"

	"github.com/dshills/witness/internal/aggregate"
	"github.com/dshills/witness/internal/models"
	"github.com/dshills/witness/internal/tui"

	tea "github.com/charmbracelet/bubbletea"
)

// GitFilePanel shows file change counts, recent commits, and hot files.
type GitFilePanel struct {
	state aggregate.RunState
}

// NewGitFilePanel creates a new GitFilePanel.
func NewGitFilePanel() *GitFilePanel {
	return &GitFilePanel{}
}

func (p *GitFilePanel) Init() tea.Cmd { return nil }

func (p *GitFilePanel) Update(msg tea.Msg) (tui.Panel, tea.Cmd) {
	if sm, ok := msg.(tui.StateMsg); ok {
		p.state = sm.State
	}
	return p, nil
}

func (p *GitFilePanel) View(width, height int) string {
	var lines []string

	// File counts.
	var created, modified, deleted int
	for i := range p.state.FileChanges {
		switch p.state.FileChanges[i].ChangeType {
		case models.ChangeTypeCreated:
			created++
		case models.ChangeTypeModified:
			modified++
		case models.ChangeTypeDeleted:
			deleted++
		}
	}
	lines = append(lines, padRight(
		fmt.Sprintf(" Files: +%d ~%d -%d  unique:%d", created, modified, deleted, p.state.UniqueFilesTouched()), width))

	// Dirty state.
	if p.state.DirtyFiles > 0 {
		lines = append(lines, padRight(
			fmt.Sprintf(" Dirty: %d files", p.state.DirtyFiles), width))
	}

	// Last commit.
	if len(p.state.Commits) > 0 {
		c := p.state.Commits[len(p.state.Commits)-1]
		sha := c.SHA
		if len(sha) > 7 {
			sha = sha[:7]
		}
		msg := c.Message
		maxMsg := width - len(sha) - 12
		if maxMsg > 0 && len(msg) > maxMsg {
			msg = msg[:maxMsg-3] + "..."
		}
		lines = append(lines, padRight(
			fmt.Sprintf(" Last: %s %s", sha, msg), width))
	}

	// Recent files (last 5).
	recentFiles := p.state.FileChanges
	if len(recentFiles) > 5 {
		recentFiles = recentFiles[len(recentFiles)-5:]
	}
	for i := len(recentFiles) - 1; i >= 0 && len(lines) < height-1; i-- {
		fc := &recentFiles[i]
		path := fc.Path
		maxPath := width - 6
		if maxPath > 0 && len(path) > maxPath {
			path = "..." + path[len(path)-maxPath+3:]
		}
		icon := "~"
		switch fc.ChangeType {
		case models.ChangeTypeCreated:
			icon = "+"
		case models.ChangeTypeDeleted:
			icon = "-"
		}
		lines = append(lines, padRight(fmt.Sprintf("   %s %s", icon, path), width))
	}

	// Hot files (top 3).
	if len(p.state.HotFiles) > 0 && len(lines) < height {
		type hotFile struct {
			path  string
			count int
		}
		var hf []hotFile
		for path, count := range p.state.HotFiles {
			hf = append(hf, hotFile{path: path, count: count})
		}
		sort.Slice(hf, func(i, j int) bool { return hf[i].count > hf[j].count })
		if len(hf) > 3 {
			hf = hf[:3]
		}
		lines = append(lines, padRight(" Hot:", width))
		for _, f := range hf {
			if len(lines) >= height {
				break
			}
			lines = append(lines, padRight(
				fmt.Sprintf("   %dx %s", f.count, f.path), width))
		}
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

func (p *GitFilePanel) Title() string   { return "Git / Files" }
func (p *GitFilePanel) Focusable() bool { return true }
