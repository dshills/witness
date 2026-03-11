package aggregate

import (
	"encoding/json"
	"sync"

	"github.com/dshills/witness/internal/events"
	"github.com/dshills/witness/internal/models"
)

// --- internal payload structs for JSON unmarshalling ---

type runPayload struct {
	Name         string            `json:"name,omitempty"`
	RepoRoot     string            `json:"repo_root,omitempty"`
	Branch       string            `json:"branch,omitempty"`
	Entrypoint   string            `json:"entrypoint,omitempty"`
	Command      []string          `json:"command,omitempty"`
	WorkingDir   string            `json:"working_dir,omitempty"`
	Host         string            `json:"host,omitempty"`
	User         string            `json:"user,omitempty"`
	WorkflowType string            `json:"workflow_type,omitempty"`
	Labels       map[string]string `json:"labels,omitempty"`
}

type stagePayload struct {
	StageID         string   `json:"stage_id"`
	Name            string   `json:"name"`
	Order           int      `json:"order"`
	ProgressPercent *float64 `json:"progress_percent,omitempty"`
	Summary         string   `json:"summary,omitempty"`
}

type toolStartedPayload struct {
	InvocationID string   `json:"invocation_id"`
	ToolName     string   `json:"tool_name"`
	Command      []string `json:"command,omitempty"`
}

type toolCompletedPayload struct {
	InvocationID string `json:"invocation_id"`
	ExitCode     *int   `json:"exit_code,omitempty"`
	Summary      string `json:"summary,omitempty"`
	DurationMS   *int   `json:"duration_ms,omitempty"`
}

type toolOutputPayload struct {
	InvocationID string `json:"invocation_id"`
	Summary      string `json:"summary,omitempty"`
	Output       string `json:"output,omitempty"`
}

type modelRequestStartedPayload struct {
	RequestID string `json:"request_id"`
	Provider  string `json:"provider"`
	Model     string `json:"model"`
	Purpose   string `json:"purpose,omitempty"`
	ToolName  string `json:"tool_name,omitempty"`
}

type modelRequestCompletedPayload struct {
	RequestID    string  `json:"request_id"`
	Provider     string  `json:"provider"`
	Model        string  `json:"model"`
	InputTokens  int64   `json:"input_tokens"`
	OutputTokens int64   `json:"output_tokens"`
	CachedTokens int64   `json:"cached_tokens"`
	CostUSD      float64 `json:"cost_usd"`
	LatencyMS    *int    `json:"latency_ms,omitempty"`
}

type modelRequestFailedPayload struct {
	RequestID string `json:"request_id"`
	Provider  string `json:"provider"`
	Model     string `json:"model"`
	Error     string `json:"error,omitempty"`
}

type fileChangePayload struct {
	ChangeID        string `json:"change_id"`
	Path            string `json:"path"`
	SizeBefore      *int64 `json:"size_before,omitempty"`
	SizeAfter       *int64 `json:"size_after,omitempty"`
	LineDeltaAdd    *int   `json:"line_delta_add,omitempty"`
	LineDeltaRemove *int   `json:"line_delta_remove,omitempty"`
	ContentHash     string `json:"content_hash,omitempty"`
}

type commitPayload struct {
	CommitID     string `json:"commit_id"`
	SHA          string `json:"sha"`
	Message      string `json:"message"`
	AuthorName   string `json:"author_name,omitempty"`
	AuthorEmail  string `json:"author_email,omitempty"`
	FilesChanged *int   `json:"files_changed,omitempty"`
	Insertions   *int   `json:"insertions,omitempty"`
	Deletions    *int   `json:"deletions,omitempty"`
}

type branchPayload struct {
	Branch string `json:"branch"`
}

type repoStatusPayload struct {
	DirtyFiles int `json:"dirty_files"`
}

type alertPayload struct {
	AlertID      string          `json:"alert_id"`
	Severity     models.Severity `json:"severity"`
	Type         string          `json:"type"`
	Title        string          `json:"title"`
	Description  string          `json:"description"`
	RelatedIDs   []string        `json:"related_ids,omitempty"`
	Acknowledged bool            `json:"acknowledged"`
	Metadata     json.RawMessage `json:"metadata,omitempty"`
}

type alertClearedPayload struct {
	AlertID string `json:"alert_id"`
}

// Aggregator applies events to produce a RunState.
type Aggregator struct {
	mu       sync.RWMutex
	state    RunState
	ringHead int // dedicated ring buffer head index
}

// NewAggregator creates a new Aggregator seeded with the given run.
func NewAggregator(run models.Run) *Aggregator {
	a := &Aggregator{
		state: RunState{
			Run: run,
		},
	}
	a.state.initMaps()
	return a
}

// Snapshot returns a copy of the current RunState under a read lock.
func (a *Aggregator) Snapshot() RunState {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.copyState()
}

// Apply processes a single event and updates the aggregated state.
func (a *Aggregator) Apply(evt events.Event) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Dispatch on event type.
	switch evt.Type { //nolint:exhaustive // unknown types handled by default
	// --- Run lifecycle ---
	case events.EventRunCreated:
		a.applyRunCreated(evt)
	case events.EventRunStarted:
		a.applyRunStarted(evt)
	case events.EventRunCompleted:
		a.applyRunTerminal(evt, models.RunStatusCompleted)
	case events.EventRunFailed:
		a.applyRunTerminal(evt, models.RunStatusFailed)
	case events.EventRunCancelled:
		a.applyRunTerminal(evt, models.RunStatusCancelled)
	case events.EventRunStalled:
		a.state.Run.Status = models.RunStatusStalled

	// --- Stage lifecycle ---
	case events.EventStageCreated:
		a.applyStageCreated(evt)
	case events.EventStageStarted:
		a.applyStageStarted(evt)
	case events.EventStageProgress:
		a.applyStageProgress(evt)
	case events.EventStageCompleted:
		a.applyStageTerminal(evt, models.StageStatusCompleted)
	case events.EventStageFailed:
		a.applyStageTerminal(evt, models.StageStatusFailed)
	case events.EventStageSkipped:
		a.applyStageTerminal(evt, models.StageStatusSkipped)

	// --- Tool lifecycle ---
	case events.EventToolStarted:
		a.applyToolStarted(evt)
	case events.EventToolCompleted:
		a.applyToolCompleted(evt, models.InvocationStatusCompleted)
	case events.EventToolFailed:
		a.applyToolCompleted(evt, models.InvocationStatusFailed)
	case events.EventToolOutput:
		a.applyToolOutput(evt)

	// --- Model lifecycle ---
	case events.EventModelRequestStarted:
		a.applyModelRequestStarted(evt)
	case events.EventModelRequestCompleted:
		a.applyModelRequestCompleted(evt)
	case events.EventModelRequestFailed:
		a.applyModelRequestFailed(evt)

	// --- File/Git ---
	case events.EventFileCreated:
		a.applyFileChange(evt, models.ChangeTypeCreated)
	case events.EventFileModified:
		a.applyFileChange(evt, models.ChangeTypeModified)
	case events.EventFileDeleted:
		a.applyFileChange(evt, models.ChangeTypeDeleted)
	case events.EventGitCommitCreated:
		a.applyCommit(evt)
	case events.EventGitBranchChanged:
		a.applyBranchChanged(evt)
	case events.EventRepoStatusChanged:
		a.applyRepoStatusChanged(evt)

	// --- Alerts ---
	case events.EventAlertRaised:
		a.applyAlertRaised(evt)
	case events.EventAlertCleared:
		a.applyAlertCleared(evt)

	// --- Tests ---
	case events.EventTestFailed:
		a.state.FailureCount++

	default:
		// Unknown event type: no error, just accumulate.
	}

	// Common bookkeeping for every event.
	a.appendRecentEvent(evt)
	a.state.EventCount++
	a.state.LastEventAt = evt.Timestamp

	return nil
}

// --- Run handlers ---

func (a *Aggregator) applyRunCreated(evt events.Event) {
	var p runPayload
	_ = json.Unmarshal(evt.Payload, &p)
	s := &a.state
	s.Run.RunID = evt.RunID
	if p.Name != "" {
		s.Run.Name = p.Name
	}
	if p.RepoRoot != "" {
		s.Run.RepoRoot = p.RepoRoot
	}
	if p.Branch != "" {
		s.Run.Branch = p.Branch
		s.Branch = p.Branch
	}
	if p.Entrypoint != "" {
		s.Run.Entrypoint = p.Entrypoint
	}
	if len(p.Command) > 0 {
		s.Run.Command = p.Command
	}
	if p.WorkingDir != "" {
		s.Run.WorkingDir = p.WorkingDir
	}
	if p.Host != "" {
		s.Run.Host = p.Host
	}
	if p.User != "" {
		s.Run.User = p.User
	}
	if p.WorkflowType != "" {
		s.Run.WorkflowType = p.WorkflowType
	}
	if p.Labels != nil {
		s.Run.Labels = p.Labels
	}
}

func (a *Aggregator) applyRunStarted(evt events.Event) {
	a.state.Run.Status = models.RunStatusRunning
	if a.state.Run.StartedAt.IsZero() {
		a.state.Run.StartedAt = evt.Timestamp
	}
}

func (a *Aggregator) applyRunTerminal(evt events.Event, status models.RunStatus) {
	a.state.Run.Status = status
	ts := evt.Timestamp
	a.state.Run.EndedAt = &ts
}

// --- Stage handlers ---

func (a *Aggregator) applyStageCreated(evt events.Event) {
	var p stagePayload
	_ = json.Unmarshal(evt.Payload, &p)
	stage := models.Stage{
		StageID: p.StageID,
		RunID:   evt.RunID,
		Name:    p.Name,
		Order:   p.Order,
		Status:  models.StageStatusPending,
	}
	if stage.StageID == "" {
		stage.StageID = evt.StageID
	}
	a.state.Stages = append(a.state.Stages, stage)
}

func (a *Aggregator) applyStageStarted(evt events.Event) {
	stageID := a.resolveStageID(evt)
	if st := a.findStage(stageID); st != nil {
		st.Status = models.StageStatusRunning
		ts := evt.Timestamp
		st.StartedAt = &ts
		clone := *st
		a.state.ActiveStage = &clone
		a.state.LastStageChangeAt = evt.Timestamp
	}
}

func (a *Aggregator) applyStageProgress(evt events.Event) {
	var p stagePayload
	_ = json.Unmarshal(evt.Payload, &p)
	stageID := a.resolveStageID(evt)
	if st := a.findStage(stageID); st != nil {
		if p.ProgressPercent != nil {
			st.ProgressPercent = p.ProgressPercent
		}
		if p.Summary != "" {
			st.Summary = p.Summary
		}
		// Keep ActiveStage in sync
		if a.state.ActiveStage != nil && a.state.ActiveStage.StageID == stageID {
			clone := *st
			a.state.ActiveStage = &clone
		}
	}
}

func (a *Aggregator) applyStageTerminal(evt events.Event, status models.StageStatus) {
	stageID := a.resolveStageID(evt)
	if st := a.findStage(stageID); st != nil {
		st.Status = status
		ts := evt.Timestamp
		st.EndedAt = &ts
		a.state.LastStageChangeAt = evt.Timestamp
	}
	// Clear ActiveStage if it matches.
	if a.state.ActiveStage != nil && a.state.ActiveStage.StageID == stageID {
		a.state.ActiveStage = nil
	}
}

func (a *Aggregator) resolveStageID(evt events.Event) string {
	if evt.StageID != "" {
		return evt.StageID
	}
	// Try payload.
	var p struct {
		StageID string `json:"stage_id"`
	}
	_ = json.Unmarshal(evt.Payload, &p)
	return p.StageID
}

func (a *Aggregator) findStage(stageID string) *models.Stage {
	for i := range a.state.Stages {
		if a.state.Stages[i].StageID == stageID {
			return &a.state.Stages[i]
		}
	}
	return nil
}

// --- Tool handlers ---

func (a *Aggregator) applyToolStarted(evt events.Event) {
	var p toolStartedPayload
	_ = json.Unmarshal(evt.Payload, &p)
	inv := models.ToolInvocation{
		InvocationID: p.InvocationID,
		RunID:        evt.RunID,
		StageID:      evt.StageID,
		ToolName:     p.ToolName,
		Command:      p.Command,
		StartedAt:    evt.Timestamp,
		Status:       models.InvocationStatusRunning,
	}
	a.state.ToolInvocations = append(a.state.ToolInvocations, inv)
	clone := inv
	a.state.ActiveTool = &clone
	a.state.ToolCounts[p.ToolName]++
}

func (a *Aggregator) applyToolCompleted(evt events.Event, status models.InvocationStatus) {
	var p toolCompletedPayload
	_ = json.Unmarshal(evt.Payload, &p)
	ts := evt.Timestamp
	if inv := a.findToolInvocation(p.InvocationID); inv != nil {
		inv.Status = status
		inv.EndedAt = &ts
		inv.ExitCode = p.ExitCode
		if p.Summary != "" {
			inv.Summary = p.Summary
		}
		inv.DurationMS = p.DurationMS
		dur := ts.Sub(inv.StartedAt)
		a.state.ToolDurations[inv.ToolName] += dur
	}
	// Clear ActiveTool if it matches.
	if a.state.ActiveTool != nil && a.state.ActiveTool.InvocationID == p.InvocationID {
		a.state.ActiveTool = nil
	}
}

func (a *Aggregator) applyToolOutput(evt events.Event) {
	var p toolOutputPayload
	_ = json.Unmarshal(evt.Payload, &p)
	if inv := a.findToolInvocation(p.InvocationID); inv != nil {
		if p.Summary != "" {
			inv.Summary = p.Summary
		}
	}
	// Also update ActiveTool if matched.
	if a.state.ActiveTool != nil && a.state.ActiveTool.InvocationID == p.InvocationID {
		if p.Summary != "" {
			a.state.ActiveTool.Summary = p.Summary
		}
	}
}

func (a *Aggregator) findToolInvocation(id string) *models.ToolInvocation {
	for i := range a.state.ToolInvocations {
		if a.state.ToolInvocations[i].InvocationID == id {
			return &a.state.ToolInvocations[i]
		}
	}
	return nil
}

// --- Model handlers ---

func (a *Aggregator) applyModelRequestStarted(evt events.Event) {
	var p modelRequestStartedPayload
	_ = json.Unmarshal(evt.Payload, &p)
	req := models.ModelRequest{
		RequestID: p.RequestID,
		RunID:     evt.RunID,
		StageID:   evt.StageID,
		Provider:  p.Provider,
		Model:     p.Model,
		StartedAt: evt.Timestamp,
		Status:    models.InvocationStatusRunning,
		Purpose:   p.Purpose,
		ToolName:  p.ToolName,
	}
	a.state.ModelRequests = append(a.state.ModelRequests, req)
	clone := req
	a.state.ActiveModel = &clone
	a.state.ModelCounts[p.Model]++
}

func (a *Aggregator) applyModelRequestCompleted(evt events.Event) {
	var p modelRequestCompletedPayload
	_ = json.Unmarshal(evt.Payload, &p)
	ts := evt.Timestamp
	if req := a.findModelRequest(p.RequestID); req != nil {
		req.Status = models.InvocationStatusCompleted
		req.EndedAt = &ts
		req.InputTokens = &p.InputTokens
		req.OutputTokens = &p.OutputTokens
		req.CachedTokens = &p.CachedTokens
		req.CostUSD = &p.CostUSD
		req.LatencyMS = p.LatencyMS
	}

	// Accumulate totals.
	a.state.TotalInputTokens += p.InputTokens
	a.state.TotalOutputTokens += p.OutputTokens
	a.state.TotalCachedTokens += p.CachedTokens
	a.state.TotalCostUSD += p.CostUSD

	// By provider.
	tc := a.state.TokensByProvider[p.Provider]
	tc.Input += p.InputTokens
	tc.Output += p.OutputTokens
	tc.Cached += p.CachedTokens
	a.state.TokensByProvider[p.Provider] = tc
	a.state.CostByProvider[p.Provider] += p.CostUSD

	// By model.
	tc = a.state.TokensByModel[p.Model]
	tc.Input += p.InputTokens
	tc.Output += p.OutputTokens
	tc.Cached += p.CachedTokens
	a.state.TokensByModel[p.Model] = tc
	a.state.CostByModel[p.Model] += p.CostUSD

	// By stage (use event's StageID).
	if evt.StageID != "" {
		tc = a.state.TokensByStage[evt.StageID]
		tc.Input += p.InputTokens
		tc.Output += p.OutputTokens
		tc.Cached += p.CachedTokens
		a.state.TokensByStage[evt.StageID] = tc
	}

	// Clear ActiveModel if matched.
	if a.state.ActiveModel != nil && a.state.ActiveModel.RequestID == p.RequestID {
		a.state.ActiveModel = nil
	}
}

func (a *Aggregator) applyModelRequestFailed(evt events.Event) {
	var p modelRequestFailedPayload
	_ = json.Unmarshal(evt.Payload, &p)
	ts := evt.Timestamp
	if req := a.findModelRequest(p.RequestID); req != nil {
		req.Status = models.InvocationStatusFailed
		req.EndedAt = &ts
	}
	a.state.FailureCount++
	if a.state.ActiveModel != nil && a.state.ActiveModel.RequestID == p.RequestID {
		a.state.ActiveModel = nil
	}
}

func (a *Aggregator) findModelRequest(id string) *models.ModelRequest {
	for i := range a.state.ModelRequests {
		if a.state.ModelRequests[i].RequestID == id {
			return &a.state.ModelRequests[i]
		}
	}
	return nil
}

// --- File/Git handlers ---

func (a *Aggregator) applyFileChange(evt events.Event, changeType models.ChangeType) {
	var p fileChangePayload
	_ = json.Unmarshal(evt.Payload, &p)
	fc := models.FileChange{
		ChangeID:        p.ChangeID,
		RunID:           evt.RunID,
		Path:            p.Path,
		ChangeType:      changeType,
		Timestamp:       evt.Timestamp,
		SizeBefore:      p.SizeBefore,
		SizeAfter:       p.SizeAfter,
		LineDeltaAdd:    p.LineDeltaAdd,
		LineDeltaRemove: p.LineDeltaRemove,
		ContentHash:     p.ContentHash,
	}
	a.state.FileChanges = append(a.state.FileChanges, fc)
	a.state.HotFiles[p.Path]++
	a.state.LastFileChangeAt = evt.Timestamp
}

func (a *Aggregator) applyCommit(evt events.Event) {
	var p commitPayload
	_ = json.Unmarshal(evt.Payload, &p)
	c := models.Commit{
		CommitID:     p.CommitID,
		RunID:        evt.RunID,
		SHA:          p.SHA,
		Timestamp:    evt.Timestamp,
		Message:      p.Message,
		AuthorName:   p.AuthorName,
		AuthorEmail:  p.AuthorEmail,
		FilesChanged: p.FilesChanged,
		Insertions:   p.Insertions,
		Deletions:    p.Deletions,
	}
	a.state.Commits = append(a.state.Commits, c)
	a.state.LastCommitAt = evt.Timestamp
}

func (a *Aggregator) applyBranchChanged(evt events.Event) {
	var p branchPayload
	_ = json.Unmarshal(evt.Payload, &p)
	a.state.Branch = p.Branch
	a.state.Run.Branch = p.Branch
}

func (a *Aggregator) applyRepoStatusChanged(evt events.Event) {
	var p repoStatusPayload
	_ = json.Unmarshal(evt.Payload, &p)
	a.state.DirtyFiles = p.DirtyFiles
}

// --- Alert handlers ---

func (a *Aggregator) applyAlertRaised(evt events.Event) {
	var p alertPayload
	_ = json.Unmarshal(evt.Payload, &p)
	alert := models.Alert{
		AlertID:      p.AlertID,
		RunID:        evt.RunID,
		Timestamp:    evt.Timestamp,
		Severity:     p.Severity,
		Type:         p.Type,
		Title:        p.Title,
		Description:  p.Description,
		RelatedIDs:   p.RelatedIDs,
		Acknowledged: p.Acknowledged,
		Metadata:     p.Metadata,
	}
	a.state.Alerts = append(a.state.Alerts, alert)
	a.state.ActiveAlerts = append(a.state.ActiveAlerts, alert)
}

func (a *Aggregator) applyAlertCleared(evt events.Event) {
	var p alertClearedPayload
	_ = json.Unmarshal(evt.Payload, &p)
	filtered := make([]models.Alert, 0, len(a.state.ActiveAlerts))
	for _, al := range a.state.ActiveAlerts {
		if al.AlertID != p.AlertID {
			filtered = append(filtered, al)
		}
	}
	a.state.ActiveAlerts = filtered
}

// --- Ring buffer ---

func (a *Aggregator) appendRecentEvent(evt events.Event) {
	if len(a.state.RecentEvents) < maxRecentEvents {
		a.state.RecentEvents = append(a.state.RecentEvents, evt)
	} else {
		a.state.RecentEvents[a.ringHead] = evt
	}
	a.ringHead = (a.ringHead + 1) % maxRecentEvents
}

// copyState returns a deep-enough copy of the state for safe external use.
func (a *Aggregator) copyState() RunState {
	s := a.state

	// Copy slices.
	s.Stages = copySlice(a.state.Stages)
	s.ToolInvocations = copySlice(a.state.ToolInvocations)
	s.ModelRequests = copySlice(a.state.ModelRequests)
	s.FileChanges = copySlice(a.state.FileChanges)
	s.Commits = copySlice(a.state.Commits)
	s.Alerts = copySlice(a.state.Alerts)
	s.ActiveAlerts = copySlice(a.state.ActiveAlerts)
	s.RecentEvents = copySlice(a.state.RecentEvents)

	// Copy maps.
	s.TokensByProvider = copyMap(a.state.TokensByProvider)
	s.TokensByModel = copyMap(a.state.TokensByModel)
	s.TokensByStage = copyMap(a.state.TokensByStage)
	s.CostByProvider = copyMap(a.state.CostByProvider)
	s.CostByModel = copyMap(a.state.CostByModel)
	s.ToolCounts = copyMap(a.state.ToolCounts)
	s.ToolDurations = copyMap(a.state.ToolDurations)
	s.ModelCounts = copyMap(a.state.ModelCounts)
	s.HotFiles = copyMap(a.state.HotFiles)

	// Copy pointers.
	if a.state.ActiveStage != nil {
		clone := *a.state.ActiveStage
		s.ActiveStage = &clone
	}
	if a.state.ActiveTool != nil {
		clone := *a.state.ActiveTool
		s.ActiveTool = &clone
	}
	if a.state.ActiveModel != nil {
		clone := *a.state.ActiveModel
		s.ActiveModel = &clone
	}

	return s
}

func copySlice[T any](src []T) []T {
	if src == nil {
		return nil
	}
	dst := make([]T, len(src))
	copy(dst, src)
	return dst
}

func copyMap[K comparable, V any](src map[K]V) map[K]V {
	if src == nil {
		return nil
	}
	dst := make(map[K]V, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
