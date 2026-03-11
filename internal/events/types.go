package events

// EventType represents the type of a Witness event.
type EventType string

// Run lifecycle events.
const (
	EventRunCreated   EventType = "run.created"
	EventRunStarted   EventType = "run.started"
	EventRunCompleted EventType = "run.completed"
	EventRunFailed    EventType = "run.failed"
	EventRunCancelled EventType = "run.cancelled"
	EventRunStalled   EventType = "run.stalled"
)

// Stage lifecycle events.
const (
	EventStageCreated   EventType = "stage.created"
	EventStageStarted   EventType = "stage.started"
	EventStageProgress  EventType = "stage.progress"
	EventStageCompleted EventType = "stage.completed"
	EventStageFailed    EventType = "stage.failed"
	EventStageSkipped   EventType = "stage.skipped"
)

// Tool lifecycle events.
const (
	EventToolStarted   EventType = "tool.started"
	EventToolOutput    EventType = "tool.output"
	EventToolCompleted EventType = "tool.completed"
	EventToolFailed    EventType = "tool.failed"
)

// Model lifecycle events.
const (
	EventModelRequestStarted   EventType = "model.request.started"
	EventModelRequestCompleted EventType = "model.request.completed"
	EventModelRequestFailed    EventType = "model.request.failed"
)

// Git / Repository events.
const (
	EventRepoStatusChanged EventType = "repo.status.changed"
	EventFileCreated       EventType = "file.created"
	EventFileModified      EventType = "file.modified"
	EventFileDeleted       EventType = "file.deleted"
	EventGitCommitCreated  EventType = "git.commit.created"
	EventGitBranchChanged  EventType = "git.branch.changed"
)

// Test / Validation events.
const (
	EventTestStarted       EventType = "test.started"
	EventTestCompleted     EventType = "test.completed"
	EventTestFailed        EventType = "test.failed"
	EventValidationWarning EventType = "validation.warning"
	EventValidationError   EventType = "validation.error"
)

// Findings / Review events.
const (
	EventFindingRecorded EventType = "finding.recorded"
	EventReviewCompleted EventType = "review.completed"
	EventDriftDetected   EventType = "drift.detected"
)

// System / Health events.
const (
	EventAlertRaised             EventType = "alert.raised"
	EventAlertCleared            EventType = "alert.cleared"
	EventBudgetThresholdExceeded EventType = "budget.threshold.exceeded"
	EventLoopDetected            EventType = "loop.detected"
	EventStallDetected           EventType = "stall.detected"
)

// Generic narrative / annotation events.
const (
	EventNoteRecorded   EventType = "note.recorded"
	EventSummaryUpdated EventType = "summary.updated"
)

// knownEventTypes is the set of all recognized event types.
var knownEventTypes = func() map[EventType]struct{} {
	types := []EventType{
		EventRunCreated, EventRunStarted, EventRunCompleted, EventRunFailed, EventRunCancelled, EventRunStalled,
		EventStageCreated, EventStageStarted, EventStageProgress, EventStageCompleted, EventStageFailed, EventStageSkipped,
		EventToolStarted, EventToolOutput, EventToolCompleted, EventToolFailed,
		EventModelRequestStarted, EventModelRequestCompleted, EventModelRequestFailed,
		EventRepoStatusChanged, EventFileCreated, EventFileModified, EventFileDeleted, EventGitCommitCreated, EventGitBranchChanged,
		EventTestStarted, EventTestCompleted, EventTestFailed, EventValidationWarning, EventValidationError,
		EventFindingRecorded, EventReviewCompleted, EventDriftDetected,
		EventAlertRaised, EventAlertCleared, EventBudgetThresholdExceeded, EventLoopDetected, EventStallDetected,
		EventNoteRecorded, EventSummaryUpdated,
	}
	m := make(map[EventType]struct{}, len(types))
	for _, t := range types {
		m[t] = struct{}{}
	}
	return m
}()

// IsKnownEventType returns true if the event type is recognized.
func IsKnownEventType(t EventType) bool {
	_, ok := knownEventTypes[t]
	return ok
}
