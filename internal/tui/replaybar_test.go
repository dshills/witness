package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestRenderReplayBar_ZeroWidth(t *testing.T) {
	status := ReplayStatusMsg{Playing: true, Speed: 1, Current: 0, Total: 100}
	got := RenderReplayBar(status, 0)
	if got != "" {
		t.Errorf("expected empty string for zero width, got %q", got)
	}
}

func TestRenderReplayBar_PlayingState(t *testing.T) {
	status := ReplayStatusMsg{
		Playing:   true,
		Speed:     2,
		Current:   141,
		Total:     891,
		EventTime: time.Date(2024, 1, 15, 14, 23, 19, 0, time.UTC),
	}
	got := RenderReplayBar(status, 120)
	if !strings.Contains(got, "Playing") {
		t.Errorf("expected 'Playing' in bar, got %q", got)
	}
	if !strings.Contains(got, "2x") {
		t.Errorf("expected '2x' speed in bar, got %q", got)
	}
	if !strings.Contains(got, "142/891") {
		t.Errorf("expected event position '142/891' in bar, got %q", got)
	}
	if !strings.Contains(got, "14:23:19") {
		t.Errorf("expected timestamp in bar, got %q", got)
	}
}

func TestRenderReplayBar_PausedState(t *testing.T) {
	status := ReplayStatusMsg{
		Playing: false,
		Speed:   1,
		Current: 50,
		Total:   100,
	}
	got := RenderReplayBar(status, 100)
	if !strings.Contains(got, "Paused") {
		t.Errorf("expected 'Paused' in bar, got %q", got)
	}
}

func TestRenderReplayBar_ProgressBar(t *testing.T) {
	status := ReplayStatusMsg{
		Playing: false,
		Speed:   1,
		Current: 49, // 50/100 = 50%
		Total:   100,
	}
	got := RenderReplayBar(status, 80)
	if !strings.Contains(got, "50%") {
		t.Errorf("expected '50%%' in progress bar, got %q", got)
	}
	if !strings.Contains(got, "[") || !strings.Contains(got, "]") {
		t.Errorf("expected progress bar brackets in output, got %q", got)
	}
}

func TestRenderReplayBar_NarrowWidth(t *testing.T) {
	status := ReplayStatusMsg{
		Playing: true,
		Speed:   4,
		Current: 10,
		Total:   20,
	}
	// Should not panic at narrow widths.
	for _, w := range []int{1, 5, 10, 20, 30, 40} {
		got := RenderReplayBar(status, w)
		if got == "" && w > 0 {
			// At very narrow widths the control line may still produce output.
			// Just verify no panic.
			_ = got
		}
	}
}

func TestRenderReplayBar_AtStart(t *testing.T) {
	status := ReplayStatusMsg{
		Playing: false,
		Speed:   1,
		Current: -1,
		Total:   100,
	}
	got := RenderReplayBar(status, 80)
	if !strings.Contains(got, "0%") {
		t.Errorf("expected '0%%' at start, got %q", got)
	}
}

func TestRenderReplayBar_AtEnd(t *testing.T) {
	status := ReplayStatusMsg{
		Playing: false,
		Speed:   1,
		Current: 99,
		Total:   100,
	}
	got := RenderReplayBar(status, 80)
	if !strings.Contains(got, "100%") {
		t.Errorf("expected '100%%' at end, got %q", got)
	}
}

func TestRenderReplayBar_VariousWidths(t *testing.T) {
	status := ReplayStatusMsg{
		Playing:   true,
		Speed:     2,
		Current:   50,
		Total:     200,
		EventTime: time.Date(2024, 6, 1, 10, 30, 0, 0, time.UTC),
	}

	widths := []int{40, 60, 80, 100, 120, 160, 200}
	for _, w := range widths {
		t.Run(fmt.Sprintf("width_%d", w), func(t *testing.T) {
			got := RenderReplayBar(status, w)
			lines := strings.Split(got, "\n")
			if len(lines) != 2 {
				t.Errorf("expected 2 lines at width %d, got %d", w, len(lines))
			}
		})
	}
}

func TestFormatSpeed(t *testing.T) {
	tests := []struct {
		speed float64
		want  string
	}{
		{1, "1x"},
		{2, "2x"},
		{4, "4x"},
		{8, "8x"},
		{16, "16x"},
		{0.5, "0.5x"},
		{1.5, "1.5x"},
	}
	for _, tt := range tests {
		got := formatSpeed(tt.speed)
		if got != tt.want {
			t.Errorf("formatSpeed(%v) = %q, want %q", tt.speed, got, tt.want)
		}
	}
}
