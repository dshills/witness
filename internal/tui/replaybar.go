package tui

import (
	"fmt"
	"strings"
	"time"
)

// RenderReplayBar renders the replay control bar for the given width.
// It shows play/pause state, speed, progress, timeline scrubber, and key hints.
func RenderReplayBar(status ReplayStatusMsg, width int) string {
	if width <= 0 {
		return ""
	}

	// Build the two lines: control info and progress bar.
	controlLine := renderControlLine(status, width)
	progressLine := renderProgressLine(status, width)

	return controlLine + "\n" + progressLine
}

// renderControlLine builds the main control info line.
func renderControlLine(status ReplayStatusMsg, width int) string {
	// Play/pause indicator.
	var playIcon string
	if status.Playing {
		playIcon = "\u25b8 Playing"
	} else {
		playIcon = "\u25a0 Paused"
	}

	// Speed display.
	speedStr := formatSpeed(status.Speed)

	// Event position.
	posStr := fmt.Sprintf("Event %d/%d", status.Current+1, status.Total)

	// Timestamp.
	var timeStr string
	if !status.EventTime.IsZero() {
		timeStr = status.EventTime.Format(time.TimeOnly)
	} else {
		timeStr = "--:--:--"
	}

	// Key hints.
	hints := "[space] play/pause [\u2190/\u2192] step [</>] speed [n] stage [c] commit"

	// Compose the line.
	info := fmt.Sprintf("%s %s | %s | %s", playIcon, speedStr, posStr, timeStr)

	// Check if we have room for hints.
	separator := " | "
	totalLen := len(info) + len(separator) + len(hints)

	if totalLen <= width {
		gap := width - totalLen
		return info + separator + hints + strings.Repeat(" ", gap)
	}

	// Truncate hints if needed.
	if len(info) < width {
		remaining := width - len(info) - len(separator)
		if remaining > 3 {
			return info + separator + hints[:min(remaining, len(hints))]
		}
		return padToWidth(info, width)
	}

	// Very narrow: just show info truncated.
	if len(info) > width {
		return info[:width]
	}
	return padToWidth(info, width)
}

// renderProgressLine builds the progress bar line.
func renderProgressLine(status ReplayStatusMsg, width int) string {
	if width < 5 {
		return ""
	}

	// [████████░░░░░░░░░░░░] 16%
	pct := 0.0
	if status.Total > 0 {
		pct = float64(status.Current+1) / float64(status.Total)
	}
	if status.Current < 0 {
		pct = 0
	}

	pctStr := fmt.Sprintf(" %d%%", int(pct*100))

	// Brackets + percent + spaces.
	barWidth := width - 2 - len(pctStr) // 2 for [ and ]
	if barWidth < 1 {
		return fmt.Sprintf("%d%%", int(pct*100))
	}

	filled := int(float64(barWidth) * pct)
	if filled > barWidth {
		filled = barWidth
	}

	bar := "[" + strings.Repeat("\u2588", filled) + strings.Repeat("\u2591", barWidth-filled) + "]" + pctStr

	if len(bar) < width {
		bar += strings.Repeat(" ", width-len(bar))
	}

	return bar
}

// formatSpeed formats the speed multiplier for display.
func formatSpeed(speed float64) string {
	if speed == float64(int(speed)) {
		return fmt.Sprintf("%dx", int(speed))
	}
	return fmt.Sprintf("%.1fx", speed)
}
