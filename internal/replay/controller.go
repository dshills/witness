package replay

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/dshills/witness/internal/aggregate"
	"github.com/dshills/witness/internal/events"
	"github.com/dshills/witness/internal/models"
)

// Sentinel errors for replay controller operations.
var (
	ErrNoEvents        = errors.New("no events to replay")
	ErrAtEnd           = errors.New("already at last event")
	ErrAtStart         = errors.New("already at first event")
	ErrIndexOutOfRange = errors.New("index out of range")
	ErrNotFound        = errors.New("no matching event found")
	ErrAlreadyPlaying  = errors.New("already playing")
)

// Controller replays a historical run by stepping through its events
// and maintaining aggregated state at each point.
type Controller struct {
	events  []events.Event
	index   int // index of the last applied event, -1 means no events applied
	run     models.Run
	agg     *aggregate.Aggregator
	speed   float64 // 1.0 = real time, 2.0 = double, 0 = manual step
	playing bool
	mu      sync.RWMutex
	updates chan aggregate.RunState
	cancel  context.CancelFunc
}

// NewController creates a replay controller for the given run and events.
// Events should be in chronological order. The controller starts at index -1
// (before the first event). Call StepForward or Play to begin replay.
func NewController(run models.Run, evts []events.Event) *Controller {
	return &Controller{
		events:  evts,
		index:   -1,
		run:     run,
		agg:     aggregate.NewAggregator(run),
		speed:   1.0,
		updates: make(chan aggregate.RunState, 1),
	}
}

// Play starts automatic playback in a goroutine. Events are advanced with
// delays proportional to timestamp gaps scaled by speed. If speed is 0,
// Play returns immediately without advancing (manual step mode).
// Play is non-blocking; it launches a goroutine and returns immediately.
func (c *Controller) Play(ctx context.Context) {
	c.mu.Lock()
	if c.playing {
		c.mu.Unlock()
		return
	}
	if c.speed == 0 {
		c.mu.Unlock()
		return
	}
	c.playing = true
	playCtx, cancel := context.WithCancel(ctx)
	c.cancel = cancel
	c.mu.Unlock()

	go c.playLoop(playCtx)
}

func (c *Controller) playLoop(ctx context.Context) {
	defer func() {
		c.mu.Lock()
		c.playing = false
		c.mu.Unlock()
	}()

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		c.mu.Lock()
		if c.index+1 >= len(c.events) {
			c.mu.Unlock()
			return
		}

		// Calculate delay based on timestamp gap.
		var delay time.Duration
		if c.index >= 0 && c.index+1 < len(c.events) {
			gap := c.events[c.index+1].Timestamp.Sub(c.events[c.index].Timestamp)
			if gap > 0 && c.speed > 0 {
				delay = time.Duration(float64(gap) / c.speed)
			}
		}
		speed := c.speed
		c.mu.Unlock()

		if speed == 0 {
			return
		}

		// Cap delay to avoid excessively long waits.
		const maxDelay = 2 * time.Second
		if delay > maxDelay {
			delay = maxDelay
		}

		if delay > 0 {
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
		}

		// Advance one step.
		_, err := c.StepForward()
		if err != nil {
			return
		}
	}
}

// Pause stops automatic playback.
func (c *Controller) Pause() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cancel != nil {
		c.cancel()
		c.cancel = nil
	}
	c.playing = false
}

// SetSpeed changes the playback speed. 1.0 = real time, 2.0 = double speed.
// Setting speed to 0 effectively pauses playback.
func (c *Controller) SetSpeed(speed float64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.speed = speed
}

// StepForward advances to the next event, applies it, and returns it.
func (c *Controller) StepForward() (*events.Event, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.events) == 0 {
		return nil, ErrNoEvents
	}
	nextIdx := c.index + 1
	if nextIdx >= len(c.events) {
		return nil, ErrAtEnd
	}

	evt := c.events[nextIdx]
	if err := c.agg.Apply(evt); err != nil {
		return nil, fmt.Errorf("applying event %d: %w", nextIdx, err)
	}
	c.index = nextIdx

	c.sendUpdate()

	return &evt, nil
}

// StepBackward moves to the previous event by rebuilding state from scratch
// up to index-1. This is O(n) but acceptable for typical event volumes.
func (c *Controller) StepBackward() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.index <= 0 {
		if c.index == 0 {
			// Go back to before the first event.
			c.agg = aggregate.NewAggregator(c.run)
			c.index = -1
			c.sendUpdate()
			return nil
		}
		return ErrAtStart
	}

	targetIdx := c.index - 1
	c.rebuildTo(targetIdx)

	c.sendUpdate()

	return nil
}

// JumpToIndex moves to the specified event index by rebuilding state.
func (c *Controller) JumpToIndex(i int) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.events) == 0 {
		return ErrNoEvents
	}
	if i < 0 || i >= len(c.events) {
		return fmt.Errorf("%w: %d (range 0-%d)", ErrIndexOutOfRange, i, len(c.events)-1)
	}

	if i == c.index {
		return nil
	}

	if i > c.index {
		// Forward: apply events from current+1 to i.
		for idx := c.index + 1; idx <= i; idx++ {
			if err := c.agg.Apply(c.events[idx]); err != nil {
				return fmt.Errorf("applying event %d: %w", idx, err)
			}
		}
		c.index = i
	} else {
		// Backward: rebuild from scratch.
		c.rebuildTo(i)
	}

	c.sendUpdate()

	return nil
}

// JumpToTime moves to the last event at or before the given time.
func (c *Controller) JumpToTime(t time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.events) == 0 {
		return ErrNoEvents
	}

	targetIdx := -1
	for i, evt := range c.events {
		if evt.Timestamp.After(t) {
			break
		}
		targetIdx = i
	}

	if targetIdx < 0 {
		return fmt.Errorf("%w: no events at or before %s", ErrNotFound, t.Format(time.RFC3339))
	}

	if targetIdx == c.index {
		return nil
	}

	if targetIdx > c.index {
		for idx := c.index + 1; idx <= targetIdx; idx++ {
			if err := c.agg.Apply(c.events[idx]); err != nil {
				return fmt.Errorf("applying event %d: %w", idx, err)
			}
		}
		c.index = targetIdx
	} else {
		c.rebuildTo(targetIdx)
	}

	c.sendUpdate()

	return nil
}

// JumpToNextStageTransition jumps to the next stage lifecycle event
// (created, started, completed, failed, skipped) after the current index.
func (c *Controller) JumpToNextStageTransition() error {
	return c.jumpToNextByType(func(et events.EventType) bool {
		return isStageTransition(et)
	})
}

// JumpToNextAlert jumps to the next alert.raised event after the current index.
func (c *Controller) JumpToNextAlert() error {
	return c.jumpToNextByType(func(et events.EventType) bool {
		return et == events.EventAlertRaised
	})
}

// JumpToNextCommit jumps to the next git.commit.created event after the current index.
func (c *Controller) JumpToNextCommit() error {
	return c.jumpToNextByType(func(et events.EventType) bool {
		return et == events.EventGitCommitCreated
	})
}

// JumpToPrevStageTransition jumps to the most recent stage transition event
// before the current index.
func (c *Controller) JumpToPrevStageTransition() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	searchEnd := c.index
	if searchEnd < 0 {
		return ErrNotFound
	}

	targetIdx := -1
	for i := searchEnd - 1; i >= 0; i-- {
		if isStageTransition(c.events[i].Type) {
			targetIdx = i
			break
		}
	}

	if targetIdx < 0 {
		return ErrNotFound
	}

	c.rebuildTo(targetIdx)
	c.sendUpdate()

	return nil
}

// CurrentEvent returns the event at the current index, or nil if before start.
func (c *Controller) CurrentEvent() *events.Event {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.index < 0 || c.index >= len(c.events) {
		return nil
	}
	evt := c.events[c.index]
	return &evt
}

// CurrentState returns a snapshot of the aggregated state at the current position.
func (c *Controller) CurrentState() aggregate.RunState {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.agg.Snapshot()
}

// Progress returns the current event index and total event count.
// When before the first event, current is -1.
func (c *Controller) Progress() (current, total int) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.index, len(c.events)
}

// IsPlaying returns whether automatic playback is active.
func (c *Controller) IsPlaying() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.playing
}

// Updates returns a channel that receives state updates during playback.
// The channel has capacity 1; intermediate states are dropped if the consumer
// is behind. This prevents high-speed replay from stalling.
func (c *Controller) Updates() <-chan aggregate.RunState {
	return c.updates
}

// rebuildTo rebuilds the aggregator state from scratch up to the target index.
// Caller must hold c.mu.Lock().
func (c *Controller) rebuildTo(targetIdx int) {
	c.agg = aggregate.NewAggregator(c.run)
	for i := 0; i <= targetIdx; i++ {
		_ = c.agg.Apply(c.events[i])
	}
	c.index = targetIdx
}

// sendUpdate performs a non-blocking send of the current state on the updates channel.
// Caller must hold c.mu (either Lock or RLock, since Snapshot acquires its own lock).
func (c *Controller) sendUpdate() {
	state := c.agg.Snapshot()
	select {
	case c.updates <- state:
	default:
		// Drop if consumer is behind.
	}
}

// jumpToNextByType finds the next event matching the predicate after current index.
func (c *Controller) jumpToNextByType(match func(events.EventType) bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	startIdx := c.index + 1
	if startIdx >= len(c.events) {
		return ErrNotFound
	}

	for i := startIdx; i < len(c.events); i++ {
		if match(c.events[i].Type) {
			// Apply all events up to and including i.
			for idx := c.index + 1; idx <= i; idx++ {
				if err := c.agg.Apply(c.events[idx]); err != nil {
					return fmt.Errorf("applying event %d: %w", idx, err)
				}
			}
			c.index = i
			c.sendUpdate()
			return nil
		}
	}

	return ErrNotFound
}

// isStageTransition returns true if the event type is a stage lifecycle event.
func isStageTransition(et events.EventType) bool {
	switch et {
	case events.EventStageCreated,
		events.EventStageStarted,
		events.EventStageCompleted,
		events.EventStageFailed,
		events.EventStageSkipped:
		return true
	}
	return false
}
