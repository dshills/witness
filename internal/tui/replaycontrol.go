package tui

import (
	"context"

	"github.com/dshills/witness/internal/events"
)

// ReplayControl is the interface the TUI uses to control replay playback.
// It is satisfied by *replay.Controller.
type ReplayControl interface {
	Play(ctx context.Context)
	Pause()
	StepForward() (*events.Event, error)
	StepBackward() error
	SetSpeed(speed float64)
	Speed() float64
	IsPlaying() bool
	Progress() (current, total int)
	JumpToNextStageTransition() error
	JumpToPrevStageTransition() error
	JumpToNextCommit() error
	JumpToPrevCommit() error
	JumpToNextAlert() error
	JumpToPrevAlert() error
}

// speedSteps defines the allowed replay speed multipliers.
var speedSteps = []float64{1, 2, 4, 8, 16}

// nextSpeed returns the next higher speed in the step ladder.
func nextSpeed(current float64) float64 {
	for _, s := range speedSteps {
		if s > current {
			return s
		}
	}
	return speedSteps[len(speedSteps)-1]
}

// prevSpeed returns the next lower speed in the step ladder.
func prevSpeed(current float64) float64 {
	for i := len(speedSteps) - 1; i >= 0; i-- {
		if speedSteps[i] < current {
			return speedSteps[i]
		}
	}
	return speedSteps[0]
}
