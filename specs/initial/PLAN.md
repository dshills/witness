# PLAN.md

## Project: Witness

Implementation plan derived from `specs/SPEC.md`. Each phase is designed to be independently testable and produce a working increment.

---

## Design Decisions

These resolve the open questions from SPEC §35 and establish constraints for all phases.

| # | Decision | Rationale |
|---|----------|-----------|
| 1 | No daemon/collector mode in v1. CLI-driven capture only. | Fewer moving parts; daemon adds IPC complexity with no v1 payoff. |
| 2 | File watching via `fsnotify` with polling fallback disabled by default but configurable. | fsnotify covers macOS/Linux/Windows. Polling is expensive and rarely needed. |
| 3 | Snapshots written every 500 events and on clean shutdown. | Balances recovery speed against write overhead. |
| 4 | TUI library: `charmbracelet/bubbletea` with `lipgloss` for styling and `bubbles` for common components. | Mature, composable, large ecosystem, active maintenance. The Elm architecture maps well to event-sourced state. |
| 5 | Minimum tool integration contract: `{"tool","status","summary","findings","duration_ms"}`. | Small enough that any tool can emit it. Witness parses what it gets, ignores unknown fields. |
| 6 | Model pricing: configurable YAML table shipped with sensible defaults, user-overridable in config. | Avoids external network dependency. Users update when prices change. |
| 7 | Cost estimation uses pricing table at event recording time. Historical runs keep their original costs. | Simple, predictable, no retroactive recalculation. |
| 8 | Replay is text/panel-focused in v1. No terminal charts. | Charts add complexity (braille rendering, sizing) with marginal v1 value. |
| 9 | Stdout/stderr: capture last 200 lines per tool invocation, configurable. Full streams not persisted. | Enough for diagnostics without disk/memory bloat. |
| 10 | Single binary with subcommands. No split. | Simpler distribution, simpler user mental model. |

### Dependency Choices

| Dependency | Purpose | Notes |
|------------|---------|-------|
| `github.com/spf13/cobra` | CLI framework | Standard for Go CLIs |
| `github.com/oklog/ulid/v2` | ID generation | Time-ordered, sortable, URL-safe |
| `gopkg.in/yaml.v3` | Config parsing | Lightweight, no viper |
| `github.com/fsnotify/fsnotify` | File system watching | Cross-platform |
| `github.com/charmbracelet/bubbletea` | TUI framework | Elm architecture |
| `github.com/charmbracelet/lipgloss` | TUI styling | Pairs with bubbletea |
| `github.com/charmbracelet/bubbles` | TUI components | Viewport, table, spinner, etc. |
| `github.com/bmatcuk/doublestar/v4` | Glob matching | Supports `**` patterns for ignore/include rules |

---

## Phase 1: Project Scaffolding and Domain Models

**Goal**: Establish the Go module, package layout, domain types, and configuration loading. No persistence, no CLI commands beyond `version`. This is the skeleton everything else hangs on.

**Spec sections**: §9 (Data Model), §10.2–10.4 (Event Envelope), §21 (Configuration), §26 (Architecture)

### Deliverables

#### 1.1 Go Module and Package Structure

Go module path: `github.com/dshills/witness` (update if a different org is used).

Create the module and empty packages:

```
go.mod
go.sum
cmd/witness/main.go
internal/app/app.go
internal/config/config.go
internal/config/config_test.go
internal/config/defaults.go
internal/config/pricing.go
internal/events/event.go
internal/events/types.go
internal/events/sink.go
internal/events/validate.go
internal/events/validate_test.go
internal/models/run.go
internal/models/stage.go
internal/models/tool.go
internal/models/model_request.go
internal/models/file_change.go
internal/models/commit.go
internal/models/alert.go
internal/models/enums.go
internal/privacy/redactor.go
internal/privacy/redactor_test.go
internal/version/version.go
```

#### 1.2 Domain Models (`internal/models`)

Implement all core entity structs per SPEC §9.1:

- `Run` with all fields including `Labels map[string]string`
- `Stage` with order, status, progress
- `ToolInvocation` with findings, metadata
- `ModelRequest` with token fields, cost
- `FileChange` with change type enum, line deltas
- `Commit` with sha, message, diff stats
- `Alert` with severity enum, acknowledged flag
- Status enums: `RunStatus`, `StageStatus`, `ChangeType`, `Severity`
- `RunStatus` must include: `pending`, `running`, `completed`, `failed`, `cancelled`, `stalled`, `unknown`

Each enum type must implement `String()`, `MarshalJSON()`, and `UnmarshalJSON()`. Use `iota`-based constants, not raw strings, for type safety.

#### 1.3 Event Envelope (`internal/events`)

Implement the base event envelope per SPEC §10.2–10.4:

```go
type Event struct {
    EventID       string          `json:"event_id"`
    SchemaVersion string          `json:"schema_version"`
    Timestamp     time.Time       `json:"timestamp"`
    RunID         string          `json:"run_id"`
    Type          EventType       `json:"type"`
    Source        string          `json:"source"`
    Payload       json.RawMessage `json:"payload"`

    // Optional
    StageID       string            `json:"stage_id,omitempty"`
    Status        string            `json:"status,omitempty"`
    Summary       string            `json:"summary,omitempty"`
    TraceID       string            `json:"trace_id,omitempty"`
    SpanID        string            `json:"span_id,omitempty"`
    ParentEventID string            `json:"parent_event_id,omitempty"`
    Tags          []string          `json:"tags,omitempty"`
    Labels        map[string]string `json:"labels,omitempty"`
}
```

Use `json.RawMessage` for `Payload` to preserve unknown fields per SPEC §10.1.7.

Define `EventType` as a string type with constants for all types in SPEC §10.5 (run lifecycle, stage lifecycle, tool lifecycle, model lifecycle, git/repo, test/validation, findings/review, system/health, narrative/annotation).

#### 1.4 Event Sink Interface (`internal/events/sink.go`)

```go
type EventSink interface {
    Append(ctx context.Context, evt Event) error
}
```

This interface is consumed by `internal/git`, `internal/files`, and `internal/ingest` in Phase 5. Defining it in Phase 1 ensures all packages compile against a stable contract.

#### 1.5 Event Validation (`internal/events`)

`Validate(evt Event) error` checks:
- `event_id` non-empty
- `schema_version` is "1.0" (or known version)
- `timestamp` is non-zero
- `run_id` non-empty
- `type` is a recognized EventType
- `source` non-empty
- `payload` is valid JSON (not nil)

Return a structured validation error listing all violations, not just the first.

#### 1.6 ID Generation

Helper function `NewID(prefix string) string` using ULID:
- `NewID("run")` → `"run_01JXXXXXX..."`
- `NewID("evt")` → `"evt_01JXXXXXX..."`
- `NewID("stage")` → `"stage_01JXXXXXX..."`

Thread-safe: use a package-level `sync.Mutex`-protected `ulid.MonotonicEntropy` source. This ensures monotonically increasing IDs within the same millisecond under concurrent access.

#### 1.7 Configuration (`internal/config`)

```go
type Config struct {
    Storage  StorageConfig  `yaml:"storage"`
    UI       UIConfig       `yaml:"ui"`
    Alerts   AlertsConfig   `yaml:"alerts"`
    Files    FilesConfig    `yaml:"files"`
    Privacy  PrivacyConfig  `yaml:"privacy"`
    Git      GitConfig      `yaml:"git"`
    Pricing  PricingConfig  `yaml:"pricing"`
    Capture  CaptureConfig  `yaml:"capture"`
}
```

Loading precedence per SPEC §21.1:
1. `Load(path string) (*Config, error)` — reads YAML, merges over defaults
2. `ApplyEnv(cfg *Config)` — override from `WITNESS_*` env vars
3. CLI flags override at command level (handled in cobra, not here)

Defaults in `defaults.go`: storage root `~/.witness`, refresh 500ms, stall 10m, loop window 8, max run cost $25, max stage cost $8, default ignore patterns.

`AlertsConfig` must use `time.Duration` for time-based thresholds (e.g., `StallDuration time.Duration`, not `StallSeconds int`) to avoid type conversion ambiguity when compared with `time.Since()` results. In YAML, durations are specified as Go duration strings (e.g., `stall_duration: 10m`, `stall_duration: 600s`). Config validation must reject suspiciously small durations (<1s) with a warning.

Pricing table in `pricing.go`: built-in map of `provider/model → InputCostPerMToken, OutputCostPerMToken`. Cover Anthropic Claude family, OpenAI GPT-4/4o/o1/o3 family. User-overridable via config.

#### 1.8 Version and CLI Entry Point

`internal/version/version.go`: exported `Version`, `Commit`, `Date` vars set via `-ldflags`.

`cmd/witness/main.go`: cobra root command with `version` subcommand only. Sets up config loading.

### Tests

- `internal/events/validate_test.go`: valid event passes, missing each required field fails, unknown event type fails, malformed payload fails
- `internal/config/config_test.go`: defaults load correctly, YAML override works, env var override works, missing file uses defaults
- `internal/models/*_test.go`: enum marshaling/unmarshaling round-trips correctly, unknown enum values handled

### Acceptance Criteria

- [ ] `go build ./cmd/witness/` succeeds
- [ ] `go test ./...` passes
- [ ] `golangci-lint run ./...` clean
- [ ] `witness version` prints version info
- [ ] All domain model types are defined with JSON tags
- [ ] Event envelope serializes/deserializes with unknown payload fields preserved
- [ ] Config loads from YAML with correct precedence

---

## Phase 2: Event Persistence and Run Storage

**Goal**: Implement the storage layer — append-only NDJSON event log, run metadata, run index, crash-tolerant recovery. After this phase, events can be created, persisted, and read back.

**Spec sections**: §12 (Persistence), §10.6 (Event Ordering), §10.7 (Idempotency)

### Deliverables

#### 2.1 Storage Interface (`internal/store`)

```go
type Store interface {
    // Run management
    CreateRun(ctx context.Context, run models.Run) error
    GetRun(ctx context.Context, runID string) (models.Run, error)
    UpdateRun(ctx context.Context, run models.Run) error
    ListRuns(ctx context.Context) ([]models.Run, error)
    DeleteRun(ctx context.Context, runID string) error

    // Event operations
    AppendEvent(ctx context.Context, runID string, evt events.Event) error
    ReadEvents(ctx context.Context, runID string) ([]events.Event, error)
    StreamEvents(ctx context.Context, runID string) (<-chan events.Event, error)

    // Snapshots
    SaveSnapshot(ctx context.Context, runID string, data []byte) error
    LoadSnapshot(ctx context.Context, runID string) ([]byte, error)

    Close() error
}
```

#### 2.2 Filesystem Store (`internal/store/fsstore`)

Implements `Store` using the local filesystem layout from SPEC §12.2:

```
~/.witness/
  runs/
    <run-id>/
      run.json       # Run metadata
      events.ndjson   # Append-only event log
      snapshot.json   # Optional aggregated state
      export/         # Export output directory
```

**Run metadata** (`run.json`): `models.Run` serialized as JSON. Written on create, updated on status changes. Uses atomic write (write to temp file, rename) per SPEC §12.4.

**Event log** (`events.ndjson`): One JSON line per event. File opened in append mode with `O_APPEND|O_WRONLY|O_CREATE`. Each append ends with `\n`. File is synced after each write (`f.Sync()`).

**Crash tolerance** (SPEC §12.4):
- On read, use a line scanner that discards the final line if it does not parse as valid JSON (partial write recovery).
- On snapshot write, write to `snapshot.json.tmp` then `os.Rename` for atomicity.

**Run index**: `ListRuns` scans the `runs/` directory, reads each `run.json`. For v1 this is sufficient — the number of runs on a local machine is manageable. No separate index file needed. `ListRuns` is only called by CLI commands (`runs`, `watch`), never in the TUI refresh loop.

**Event deduplication** (SPEC §10.7): `AppendEvent` checks if the last N events (N=256, in-memory ring buffer) contain the same `event_id`. If duplicate, skip silently. N=256 ensures high-throughput event streams during crash-recovery replay don't produce false negatives.

#### 2.3 Event Streaming

`StreamEvents` returns a buffered channel (capacity 256) that:
1. Reads all existing events from `events.ndjson`
2. Then tails the file for new appends using `fsnotify` on the ndjson file
3. Closes when context is cancelled
4. If a slow consumer lets the buffer fill, new events are dropped (with a logged warning) rather than blocking the append path

This enables live consumers (TUI, aggregator) to subscribe to a run's event stream.

#### 2.4 Storage Directory Bootstrap

`EnsureStorageDir(root string) error`: creates `~/.witness/` and `~/.witness/runs/` if they don't exist, with `0700` permissions.

### Tests

- `store/fsstore_test.go`:
  - Create run, read it back, fields match
  - Append 100 events, read all back, order preserved
  - Append event with duplicate ID, only one stored
  - Write partial final line, recovery reads all complete events
  - Atomic snapshot write/read round-trip
  - ListRuns returns all runs sorted by start time
  - StreamEvents delivers existing + new events
  - StreamEvents: context cancellation closes channel
  - StreamEvents: slow consumer does not block AppendEvent (events dropped after buffer fills)
  - DeleteRun removes directory
  - Concurrent appends don't corrupt the file (use `sync.Mutex` internally)

### Acceptance Criteria

- [ ] Events round-trip through write/read with no data loss
- [ ] Partial final NDJSON line is tolerated on recovery
- [ ] Snapshot write is atomic
- [ ] Duplicate event IDs are deduplicated
- [ ] StreamEvents tails new events in real time
- [ ] `go test ./internal/store/...` passes
- [ ] `golangci-lint run ./...` clean

---

## Phase 3: State Aggregation and Metrics

**Goal**: Build the aggregator that derives live state from the event stream. After this phase, any sequence of events can be reduced to a `RunState` snapshot with all metrics.

**Spec sections**: §11 (State Aggregation), §14 (Metrics)

### Deliverables

#### 3.1 RunState Model (`internal/aggregate`)

```go
type RunState struct {
    Run            models.Run
    Stages         []models.Stage
    ActiveStage    *models.Stage
    ActiveTool     *models.ToolInvocation
    ActiveModel    *models.ModelRequest

    // Counters
    TotalInputTokens   int64
    TotalOutputTokens  int64
    TotalCachedTokens  int64
    TotalCostUSD       float64
    TokensByProvider   map[string]TokenCount
    TokensByModel      map[string]TokenCount
    TokensByStage      map[string]TokenCount
    CostByProvider     map[string]float64
    CostByModel        map[string]float64

    // Tool stats
    ToolInvocations    []models.ToolInvocation
    ToolCounts         map[string]int
    ToolDurations      map[string]time.Duration

    // Model stats
    ModelRequests      []models.ModelRequest
    ModelCounts        map[string]int

    // Git/Files
    Branch             string
    DirtyFiles         int
    FileChanges        []models.FileChange
    Commits            []models.Commit
    HotFiles           map[string]int  // path → touch count

    // Alerts
    Alerts             []models.Alert
    ActiveAlerts       []models.Alert

    // Health scores
    FailureCount       int
    RetryCount         int

    // Event stream
    RecentEvents       []events.Event  // ring buffer, last 200
    EventCount         int64

    // Timing
    LastEventAt        time.Time
    LastFileChangeAt   time.Time
    LastCommitAt       time.Time
    LastStageChangeAt  time.Time
}

type TokenCount struct {
    Input  int64
    Output int64
    Cached int64
}
```

#### 3.2 Aggregator Implementation (`internal/aggregate`)

```go
type Aggregator struct {
    mu    sync.RWMutex
    state RunState
}

func NewAggregator(run models.Run) *Aggregator
func (a *Aggregator) Apply(evt events.Event) error
func (a *Aggregator) Snapshot() RunState  // returns a copy under read lock
```

`Apply` dispatches on `evt.Type` to handler methods:

| Event Type | Handler Action |
|------------|---------------|
| `run.created` | Set run metadata |
| `run.started` | Set status=running, started_at |
| `run.completed/failed/cancelled` | Set status, ended_at |
| `run.stalled` | Set status=stalled |
| `stage.created` | Append stage to list |
| `stage.started` | Set stage status=running, set ActiveStage |
| `stage.progress` | Update progress_percent, summary |
| `stage.completed/failed/skipped` | Set stage status, ended_at, clear ActiveStage if matched |
| `tool.started` | Create ToolInvocation, set ActiveTool, increment ToolCounts |
| `tool.completed/failed` | Update invocation end/status/exit_code, clear ActiveTool, update ToolDurations |
| `tool.output` | Store summary/output snippet on active tool |
| `model.request.started` | Create ModelRequest, set ActiveModel, increment ModelCounts |
| `model.request.completed` | Update request, accumulate tokens/cost into totals and breakdowns, clear ActiveModel |
| `model.request.failed` | Update request status, increment FailureCount, clear ActiveModel |
| `file.created/modified/deleted` | Append FileChange, increment HotFiles, update LastFileChangeAt |
| `git.commit.created` | Append Commit, update LastCommitAt |
| `git.branch.changed` | Update Branch |
| `repo.status.changed` | Update DirtyFiles |
| `alert.raised` | Append to Alerts and ActiveAlerts |
| `alert.cleared` | Remove from ActiveAlerts by alert_id |
| `test.failed` | Increment FailureCount |
| `*` (unknown) | Append to RecentEvents, increment EventCount, no error |

Every call to `Apply` also:
- Appends to `RecentEvents` (ring buffer, drop oldest past 200)
- Increments `EventCount`
- Updates `LastEventAt`

#### 3.3 Metrics Computation (`internal/aggregate`)

Methods on `RunState`:

```go
func (s *RunState) Duration() time.Duration
func (s *RunState) StageDurations() map[string]time.Duration
func (s *RunState) TokenBurnRate(window time.Duration) float64  // tokens/min over window
func (s *RunState) CostBurnRate(window time.Duration) float64
// Burn rate algorithm: scan RecentEvents for model.request.completed events
// within the last `window` duration. Sum their token counts (or costs) and
// divide by the actual elapsed time of those events. Return 0 if no
// qualifying events exist in the window.
func (s *RunState) AvgToolLatency() time.Duration
func (s *RunState) AvgModelLatency() time.Duration
func (s *RunState) MeanTimeBetweenCommits() time.Duration
func (s *RunState) UniqueFilesTouched() int
```

These derive from the accumulated state — no separate metrics store.

#### 3.4 Snapshot Serialization

`RunState` must be JSON-serializable for persistence as `snapshot.json`:
```go
func (s *RunState) MarshalJSON() ([]byte, error)
func UnmarshalRunState(data []byte) (*RunState, error)
```

#### 3.5 Rebuild from Event Log

```go
func Rebuild(run models.Run, events []events.Event) (*RunState, error)
```

Creates a new `Aggregator`, applies all events in order, returns the final `RunState`. Used for:
- Loading historical runs
- Verifying snapshot correctness
- Recovery after crash

### Tests

- `aggregate_test.go`:
  - Empty run produces zero-state snapshot
  - Run lifecycle events update status correctly
  - Stage events update stage list and active stage
  - Tool events accumulate counts and durations
  - Model events accumulate tokens and costs correctly across providers/models
  - File/commit events populate Git state
  - Alert raised/cleared lifecycle works
  - Out-of-order events handled gracefully (SPEC §10.6)
  - Unknown event types don't error, just accumulate
  - Rebuild from event list matches incremental Apply
  - RecentEvents ring buffer doesn't exceed 200
  - TokenBurnRate and CostBurnRate compute correctly over window
  - Concurrent Apply calls are safe

### Acceptance Criteria

- [ ] Aggregator processes all defined event types
- [ ] Snapshot is reconstructable from event log (Rebuild matches incremental)
- [ ] Metrics compute correctly from accumulated state
- [ ] Thread-safe under concurrent Apply
- [ ] `go test ./internal/aggregate/...` passes
- [ ] `golangci-lint run ./...` clean

---

## Phase 4: CLI Commands — Inspection and Export

**Goal**: Implement the non-live CLI commands: `runs`, `inspect`, `stats`, `export`, `doctor`, `config show`. After this phase, a user can create runs programmatically, persist events, and inspect them via the CLI.

**Spec sections**: §7.2 (Command Responsibilities), §22 (Export Formats)

### Deliverables

#### 4.1 CLI Framework (`cmd/witness`)

Cobra command tree:

```
witness
├── version
├── runs
├── inspect <run-id>
├── stats <run-id>
├── export <run-id> --format <json|ndjson|markdown>
├── doctor
├── config
│   └── show
├── run -- <command>      (Phase 5)
├── watch                 (Phase 7)
├── attach --run <id>     (Phase 7)
└── replay <run-id>       (Phase 8)
```

Phase 4 implements `runs`, `inspect`, `stats`, `export`, `doctor`, `config show`.

#### 4.2 `witness runs`

- Load all runs via `store.ListRuns()`
- Sort by `started_at` descending
- Print table: `ID | Name | Status | Started | Duration | Stages | Cost | Commits | Alerts`
- Use tab writer for alignment
- Support `--status` filter (e.g., `--status running`)
- Support `--limit N` (default 20)

#### 4.3 `witness inspect <run-id>`

- Load run and rebuild state via aggregator
- Print structured textual summary:
  - Run metadata (id, name, repo, branch, command, status, duration)
  - Stage progression list with status icons and durations
  - Token/cost summary by provider/model
  - Git summary (commits, files changed)
  - Active alerts
  - Last 10 events

#### 4.4 `witness stats <run-id>`

- Load run and rebuild state
- Print metrics per SPEC §14.1–14.2:
  - Duration by stage
  - Tokens by provider/model
  - Tools used with invocation counts and avg latency
  - Findings by severity
  - Files changed (total and unique)
  - Commits count
  - Alerts count
  - Derived rates (token burn, cost burn, mean time between commits)

#### 4.5 `witness export <run-id>` (`internal/export`)

Three exporters implementing:

```go
type Exporter interface {
    Export(ctx context.Context, state aggregate.RunState, events []events.Event, w io.Writer) error
}
```

**JSON exporter** (`json.go`): Single JSON object with `run`, `stages`, `tools`, `model_requests`, `file_changes`, `commits`, `alerts`, `metrics`, `events`.

**NDJSON exporter** (`ndjson.go`): Raw event stream, one JSON line per event. Essentially a copy of `events.ndjson` but filtered/validated.

**Markdown exporter** (`markdown.go`): Human-readable summary per SPEC §23.3:

```markdown
# Run Report: <name>

## Summary
- **Run ID**: ...
- **Status**: ...
- **Duration**: ...
- **Total Cost**: $X.XX

## Stages
| Stage | Status | Duration |
|-------|--------|----------|
| ...   | ...    | ...      |

## Token Usage
...

## Commits
...

## Alerts
...
```

CLI flag `--output` writes to file instead of stdout. Default is stdout.

#### 4.6 `witness doctor`

Check and report:
- Storage directory exists and is writable
- Git binary available (`git --version`)
- Terminal supports colors (check `$NO_COLOR` env var and `isatty` on stdout; use lipgloss's built-in detection)
- Config file parseable (if exists)
- Run count and total disk usage
- Print pass/fail for each check

#### 4.7 `witness config show`

- Load effective config (defaults + file + env)
- Print as YAML to stdout
- Source annotation is out of scope for v1; print effective values only

### Tests

- `export/json_test.go`: golden test — known RunState + events → expected JSON output
- `export/ndjson_test.go`: golden test — events round-trip through export
- `export/markdown_test.go`: golden test — known state → expected markdown
- Integration test: create run + events via store, run `witness inspect` and `witness stats`, verify output contains expected data

### Acceptance Criteria

- [ ] `witness runs` lists runs with correct formatting
- [ ] `witness inspect` prints meaningful run summary
- [ ] `witness stats` prints all required metrics
- [ ] `witness export --format json` produces valid, complete JSON
- [ ] `witness export --format ndjson` produces valid NDJSON with one event per line
- [ ] `witness export --format markdown` produces readable summary
- [ ] `witness doctor` reports system readiness
- [ ] `witness config show` displays effective config
- [ ] `go test ./...` passes
- [ ] `golangci-lint run ./...` clean

---

## Phase 5: Live Capture — `witness run`

**Goal**: Implement `witness run -- <command>` for subprocess observation. After this phase, a user can run a command under Witness instrumentation and have events recorded.

**Spec sections**: §7.2 `witness run`, §8 (Execution Modes), §16 (Git Integration), §17 (File System Observation), §20 (Privacy)

### Deliverables

#### 5.1 Subprocess Runner (`internal/app`)

`witness run -- <command> [args...]`:

1. Create a new run via store (`status=pending`)
2. Register signal handler for SIGINT/SIGTERM (see §5.1.1)
3. Emit `run.created` event
4. Start observation goroutines (Git poller, file watcher, stdin ingestion)
5. Start subprocess via `os/exec` in its own process group (POSIX: `cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}`), capture stdout/stderr in separate goroutines. Use build tags (`runner_unix.go` / `runner_windows.go`) for platform-specific process group and signal forwarding. Windows is best-effort in v1.
6. Emit `run.started` event
7. Wait for subprocess exit
8. Emit `run.completed` or `run.failed` based on exit code
9. Cancel context to stop observation goroutines; wait for goroutine group to finish
10. Write final snapshot
11. Print run ID and summary to stderr

**§5.1.1 Signal Handling and Graceful Shutdown:**

On SIGINT or SIGTERM:
1. Forward signal to subprocess process group via `syscall.Kill(-pid, sig)`
2. Start a 10-second deadline timer
3. If subprocess exits within deadline: emit `run.cancelled` event, write snapshot, exit
4. If deadline expires: send SIGKILL to process group, emit `run.cancelled` with metadata noting forced kill, write snapshot, exit

All observation goroutines must accept a `context.Context` and exit promptly when it is cancelled. Use `sync.WaitGroup` or `errgroup.Group` to wait for clean goroutine shutdown before writing the final snapshot.

Subprocess stdout/stderr:
- Relay to the terminal in real time (the user still sees their command output)
- Stdout and stderr must be consumed in separate goroutines to prevent deadlock when both pipes fill their OS buffers simultaneously
- Use `io.TeeReader` to split stdout: one path writes to `os.Stdout` for terminal relay, the other feeds the ingest scanner. This avoids consuming the pipe twice.
- If the subprocess fails to start (e.g., binary not found), emit `run.failed` with `summary="subprocess failed to start: <error>"`, write snapshot, and exit
- Scan for Witness JSON event lines (lines matching `{"event_id":...}`)
- Lines that parse as valid Witness events are ingested; others pass through

Flag `--name <name>` to set run name. Default: the command string.
Flag `--no-git` to disable Git observation.
Flag `--no-files` to disable file watching.
Flag `--tui` to auto-open TUI (implemented in Phase 7, flag registered here).

#### 5.2 Git Observer (`internal/git`)

Polls Git state at a configurable interval (default: 5 seconds).

```go
type Observer struct {
    repoRoot string
    interval time.Duration
    sink     events.EventSink
    runID    string
}

func (o *Observer) Start(ctx context.Context) error
func (o *Observer) Stop()
```

On each poll:
1. `git rev-parse --show-toplevel` — detect repo root (once)
2. `git symbolic-ref --short HEAD` — current branch. Emit `git.branch.changed` if changed.
3. `git status --porcelain` — dirty files. Emit `repo.status.changed` if changed.
4. `git log --oneline <last-known-sha>..HEAD` — new commits. Emit `git.commit.created` for each. On first poll, record current HEAD as `last-known-sha` without emitting any events (avoids replaying entire repo history).
5. For each new commit: `git show --stat <sha>` — get files changed, insertions, deletions.

All Git commands executed via `os/exec` with timeouts. Failures logged, not fatal (SPEC §25.2).

Helper functions (used by observer and by `inspect`/`stats`):
- `DetectRepoRoot(path string) (string, error)`
- `CurrentBranch(repoRoot string) (string, error)`
- `ParseCommit(repoRoot, sha string) (models.Commit, error)`

#### 5.3 File System Watcher (`internal/files`)

```go
type Watcher struct {
    root     string
    patterns FilesConfig  // ignore/include
    sink     events.EventSink
    runID    string
}

func (w *Watcher) Start(ctx context.Context) error
func (w *Watcher) Stop()
```

Uses `fsnotify` to watch the repo root recursively. On file events:
1. Check against ignore patterns (SPEC §17.2 defaults + config overrides)
2. Classify as `file.created`, `file.modified`, or `file.deleted`
3. Emit event with path, timestamp, change type

Ignore pattern matching: use `github.com/bmatcuk/doublestar/v4` for `**` glob support (listed in Dependency Choices). Default ignores: `.git/`, `node_modules/`, `vendor/`, `dist/`, `build/`, `.next/`, `*.swp`, `*.swo`, `*~`, `.DS_Store`.

Debounce: coalesce events for the same path within 100ms to avoid duplicate events from editor save sequences.

#### 5.4 Stdin Event Ingestion (`internal/ingest`)

```go
type Scanner struct {
    reader io.Reader
    sink   events.EventSink
    runID  string
}

func (s *Scanner) Scan(ctx context.Context) error
```

Reads lines from subprocess stdout. For each line:
1. Attempt JSON parse as `events.Event`
2. If valid and has `event_id` + `type`: ingest as Witness event (override `run_id` to current run)
3. If not a Witness event, attempt parse as structured tool result (SPEC §18.4): convert to `tool.completed` event
4. Otherwise: pass through as raw output, optionally store in tool output buffer

#### 5.5 Redaction (`internal/privacy`)

```go
func NewRedactor(patterns []string) (*Redactor, error)
func (r *Redactor) Redact(s string) string
```

Compiles configured regex patterns (SPEC §20.3). Applied to:
- Event summaries before persistence
- Tool output before persistence
- Event `payload` string values (redactor walks the JSON object and applies patterns to all string values)
- Never applied to event IDs, timestamps, or structural field names/keys

Default patterns (compiled at startup):
- API keys: `(?i)(sk-[a-zA-Z0-9]{20,}|AKIA[A-Z0-9]{16})`
- Bearer tokens: `(?i)bearer\s+[A-Za-z0-9\-._~+/=]{20,}`
- Generic secrets: `(?i)(password|secret|apikey)\s*[=:]\s*\S{8,}` (note: `token` excluded to avoid false positives on telemetry fields like `input_tokens`)

#### 5.6 EventSink Adapter

Bridge between observers and the store:

```go
type StoreSink struct {
    store        store.Store
    runID        string
    aggregator   *aggregate.Aggregator
    redactor     *privacy.Redactor
    alertEngine  AlertHook  // nil = no alert evaluation (wired in Phase 8)
    mu           sync.Mutex
    count        int64
}

// AlertHook is an optional interface for Phase 8 integration.
// When nil, no alert evaluation occurs.
type AlertHook interface {
    Evaluate(state aggregate.RunState) []models.Alert
}

func (s *StoreSink) Append(ctx context.Context, evt events.Event) error
```

On each append:
1. Validate event
2. Redact sensitive fields
3. Persist to store
4. Apply to aggregator
5. If `alertEngine != nil`, evaluate alerts and append any new alerts as events
6. Increment count; if count % 500 == 0, save snapshot

### Tests

- `git/observer_test.go`: parse commit output, detect branch change, handle non-git directory
- `files/watcher_test.go`: ignore patterns match correctly, debounce works (create temp dir, write files, verify events)
- `ingest/scanner_test.go`: Witness JSON line parsed, structured tool result parsed, raw line passed through
- `privacy/redactor_test.go`: API keys redacted, bearer tokens redacted, normal text unchanged, empty patterns produce no-op redactor
- `app/storesink_test.go`: append 500+ events via StoreSink, verify snapshot file is written after the 500th event
- Integration test: `witness run -- echo hello` creates a run, records start/complete events, exit code 0
- Integration test: `witness run -- <cmd-that-writes-to-both-stdout-and-stderr>` completes without deadlock

### Acceptance Criteria

- [ ] `witness run -- <cmd>` creates a run, records events, prints run ID
- [ ] Subprocess stdout/stderr relayed to terminal
- [ ] Git changes detected and recorded during run
- [ ] File changes detected and recorded with correct ignore patterns
- [ ] Structured JSON events from subprocess ingested
- [ ] Sensitive patterns redacted before persistence
- [ ] Run completes with `run.completed` or `run.failed` event
- [ ] SIGINT during `witness run` forwards signal to subprocess, emits `run.cancelled`, writes snapshot, and all goroutines exit cleanly
- [ ] Snapshot written on completion
- [ ] `witness inspect <run-id>` shows the captured run
- [ ] `go test ./...` passes
- [ ] `golangci-lint run ./...` clean

---

## Phase 6: Replay Engine

**Goal**: Implement replay — loading a historical run and stepping through its events in time order. This is the foundation for both the `replay` CLI command and the TUI replay mode.

**Spec sections**: §23 (Replay and Postmortem)

### Deliverables

#### 6.1 Replay Controller (`internal/replay`)

```go
type Controller struct {
    events    []events.Event
    index     int
    state     *aggregate.Aggregator
    speed     float64  // 1.0 = real time, 2.0 = double, 0 = manual step
    playing   bool
    mu        sync.RWMutex
}

func NewController(run models.Run, events []events.Event) *Controller

// Playback
func (c *Controller) Play(ctx context.Context)
func (c *Controller) Pause()
func (c *Controller) SetSpeed(speed float64)

// Stepping
func (c *Controller) StepForward() (*events.Event, error)
func (c *Controller) StepBackward() error  // rebuild from 0 to index-1
func (c *Controller) JumpToIndex(i int) error
func (c *Controller) JumpToTime(t time.Time) error

// Navigation
func (c *Controller) JumpToNextStageTransition() error
func (c *Controller) JumpToNextAlert() error
func (c *Controller) JumpToNextCommit() error
func (c *Controller) JumpToPrevStageTransition() error

// State
func (c *Controller) CurrentEvent() *events.Event
func (c *Controller) CurrentState() aggregate.RunState
func (c *Controller) Progress() (current, total int)
func (c *Controller) IsPlaying() bool
```

`StepBackward` and `JumpToIndex` (backward) work by rebuilding state from event 0 up to the target index. This is O(n) but acceptable for v1 event volumes (thousands, not millions).

`Play` runs in a goroutine, advancing events with delays proportional to actual timestamp gaps scaled by `speed`. Sends state updates via `func (c *Controller) Updates() <-chan aggregate.RunState` — a buffered channel (capacity 1) that the TUI consumes as `StateMsg`. Uses non-blocking send (`select { case ch <- state: default: }`) to drop intermediate states when the TUI is behind, ensuring high-speed replay doesn't stall. This is consistent with the live TUI state bridge design (Phase 7 §7.2).

#### 6.2 Postmortem Summary Generator

```go
func GeneratePostmortem(state aggregate.RunState) string
```

Produces a concise text summary per SPEC §23.3:
- Run objective (name, command, repo)
- Duration and outcome
- Stage progression with time spent in each
- Where failures occurred
- Cost summary
- Commits and file change summary
- Notable alerts

#### 6.3 `witness replay <run-id>` Command

Initial implementation (pre-TUI): textual timeline mode.
- Load run and events from store
- Print events one at a time with timestamps, types, and summaries
- Support `--speed` flag for automatic playback
- Support `--summary` flag to just print the postmortem summary

Full TUI replay added in Phase 8.

### Tests

- `replay/controller_test.go`:
  - StepForward advances index and applies event
  - StepBackward rebuilds correctly
  - JumpToNextStageTransition finds correct event
  - JumpToNextAlert finds correct event
  - JumpToNextCommit finds correct event
  - JumpToIndex at boundary (0, len-1) works
  - Play with speed=0 does not auto-advance
  - CurrentState matches expected state at each step
- `replay/postmortem_test.go`: known run produces expected summary text

### Acceptance Criteria

- [ ] Replay controller steps forward/backward through events
- [ ] State at any point matches what Rebuild would produce for those events
- [ ] Jump-to-stage/alert/commit navigation works
- [ ] `witness replay <run-id>` prints event timeline
- [ ] `witness replay <run-id> --summary` prints postmortem
- [ ] `go test ./internal/replay/...` passes
- [ ] `golangci-lint run ./...` clean

---

## Phase 7: Terminal UI — Live Dashboard

**Goal**: Build the live TUI using Bubble Tea. After this phase, `witness watch` and `witness attach` work, showing real-time run state.

**Spec sections**: §13 (TUI Requirements), §30 (Human Factors)

### Deliverables

#### 7.1 TUI Architecture (`internal/tui`)

Bubble Tea model hierarchy:

```
App (root model)
├── HeaderPanel
├── StagePanel
├── ActiveWorkPanel
├── TokenCostPanel
├── GitFilePanel
├── AlertsPanel
└── EventStreamPanel
```

Each panel implements:

```go
type Panel interface {
    Init() tea.Cmd
    Update(msg tea.Msg) (Panel, tea.Cmd)
    View(width, height int) string
    Title() string
    Focusable() bool
}
```

The `App` model owns the layout, routes key events to the focused panel, and refreshes on state changes.

#### 7.2 State Bridge

Connect the aggregator to the TUI:

```go
type StateMsg struct {
    State aggregate.RunState
}
```

A background goroutine subscribes to the event stream (`store.StreamEvents`), applies events to the aggregator, and sends `StateMsg` to the Bubble Tea program at the configured refresh interval (default 500ms). The TUI only renders on `StateMsg`, not on every event.

#### 7.3 Layout Engine

Two layout modes:

**Full layout** (≥100 cols × ≥28 rows): Two-column layout per SPEC §13.2.

```
┌─────────────── Header ───────────────────┐
├──────────────┬───────────────────────────┤
│ Stages       │ Active Tool/Model        │
├──────────────┤───────────────────────────┤
│ Tokens/Cost  │ Git/Files                │
├──────────────┴───────────────────────────┤
│ Alerts                                   │
├──────────────────────────────────────────┤
│ Event Stream                             │
└──────────────────────────────────────────┘
```

**Compact layout** (<100 cols): Single-column stacked panels. Show only Header, Active Work, Alerts, Event Stream. Others accessible via keyboard shortcuts.

Resize handling: on `tea.WindowSizeMsg`, recalculate layout and re-render.

#### 7.4 Panel Implementations (`internal/ui/panels`)

**HeaderPanel** (SPEC §13.3): Run name, ID, repo, branch, duration (live-updating), status with color coding (green=running, red=failed, yellow=stalled, blue=completed).

**StagePanel** (SPEC §13.4): Ordered list of stages. Icons: `✓` completed, `▸` running, `✗` failed, `–` skipped, `○` pending. Progress bar if `progress_percent` available. Current step summary text.

**ActiveWorkPanel** (SPEC §13.5): Active tool name and command. Active model provider/model. Request count, latency. Summary text. Retry count if >0.

**TokenCostPanel** (SPEC §13.6): Total input/output tokens. Total cost formatted as `$X.XX`. Cost by provider/model (top 3). Token burn rate (tokens/min over last 5 minutes). Warning marker if budget threshold exceeded.

**GitFilePanel** (SPEC §13.7): Modified/added/deleted file counts. Last commit sha (7 chars) and message (truncated). Recent files touched (last 5). Hot files by frequency (top 3). Dirty state indicator.

**AlertsPanel** (SPEC §13.8): Active alerts with severity colors (red=critical/error, yellow=warning, blue=info). Most recent first. Truncate to fit height.

**EventStreamPanel** (SPEC §13.9): Scrollable viewport using `bubbles.viewport`. Time-ordered events. Compact format: `[HH:MM:SS] TYPE source: summary`. Support pause/freeze (stop auto-scroll), filter by type. Viewport supports j/k scrolling.

#### 7.5 Keyboard Navigation (SPEC §13.11)

| Key | Action |
|-----|--------|
| `q` / `ctrl+c` | Quit |
| `tab` | Focus next panel |
| `shift+tab` | Focus previous panel |
| `j` / `↓` | Scroll down in focused panel |
| `k` / `↑` | Scroll up in focused panel |
| `p` | Pause event stream auto-scroll |
| `r` | Resume live tail |
| `/` | Open filter input for event stream |
| `esc` | Close filter / exit drill-down / unfocus |
| `?` | Toggle help overlay |

Drill-down shortcuts (switch to focused view):
| Key | View |
|-----|------|
| `s` | Stage detail |
| `t` | Token/cost detail |
| `g` | Git detail |
| `a` | Alerts detail |
| `e` | Event stream full screen |
| `m` | Model request history |

Drill-down views use the full terminal and `esc` returns to the dashboard.

#### 7.6 Help Overlay

Translucent overlay listing all keyboard shortcuts. Toggled by `?`. Rendered on top of the current view using lipgloss overlay positioning.

#### 7.7 Commands

**`witness watch`**: Open TUI for the most recent active run. `--run <id>` or `--run latest` to specify. `--repo .` to find active run for current repo. If no active runs exist and no `--run` flag is given, exit with message `"No active runs found. Use 'witness watch --run <id>' to view a completed run."` With `--run latest`, show the most recent run regardless of status.

**`witness attach --run <run-id>`**: Same as `watch --run <run-id>` but requires the run to be active (status=running/pending). If the run is already in a terminal state (completed/failed/cancelled — note: `stalled` and `unknown` are considered active), exit with code 1 and message `"run <id> is not active (status: <status>)"`. If the run transitions to a terminal state while attached, the TUI displays the final state and remains open (user can quit with `q`).

Both connect to the event stream via `store.StreamEvents` and feed the aggregator → TUI pipeline.

### Tests

- `tui/app_test.go`: Init produces correct initial model, WindowSizeMsg triggers layout recalc
- `ui/panels/*_test.go`: Each panel renders without panic given empty state, each panel renders correctly given populated state (snapshot testing with lipgloss `NewRenderer` in test mode)
- `tui/keys_test.go`: Key dispatch routes to correct panel, `q` produces quit cmd, `tab` advances focus

### Acceptance Criteria

- [ ] `witness watch` opens TUI showing live run state
- [ ] All 7 panels render with correct data
- [ ] Keyboard navigation works (tab, j/k, drill-downs)
- [ ] Layout adapts to terminal resize
- [ ] Compact layout works for narrow terminals
- [ ] Event stream pauses/resumes
- [ ] Help overlay shows shortcuts
- [ ] TUI exits cleanly without corrupting run data
- [ ] `go test ./...` passes
- [ ] `golangci-lint run ./...` clean

---

## Phase 8: Alerts and Heuristics

**Goal**: Implement the anomaly detection engine that evaluates aggregated state and raises alerts.

**Spec sections**: §15 (Alerts and Anomaly Detection), §14.3 (Heuristic Metrics)

### Deliverables

#### 8.1 Alert Evaluator Framework (`internal/alerts`)

```go
type Rule interface {
    Name() string
    Evaluate(state aggregate.RunState, cfg config.AlertsConfig) []models.Alert
}

type Engine struct {
    rules []Rule
    cfg   config.AlertsConfig
    known map[string]bool  // alert IDs already raised
}

func NewEngine(cfg config.AlertsConfig) *Engine
func (e *Engine) RegisterRule(r Rule)
func (e *Engine) Evaluate(state aggregate.RunState) []models.Alert
```

`Evaluate` runs all rules, deduplicates against previously raised alerts, returns only new alerts. Each new alert is also emitted as an `alert.raised` event.

#### 8.2 Stall Detection Rule

Trigger conditions (all must be true):
- Run status is `running`
- `time.Since(state.LastFileChangeAt) > cfg.StallDuration` AND `time.Since(state.LastStageChangeAt) > cfg.StallDuration`
- At least one event has been received (run isn't brand new)

Produces `warning` severity alert with type `stall.detected`.

Compute stall score: `min(1.0, stalledDuration / (2 * stallThreshold))`. Exposed in RunState for TUI health panel.

#### 8.3 Loop Detection Rule

Trigger conditions:
- Within the last `cfg.LoopWindow` tool invocations, the same tool was invoked ≥ `int(math.Ceil(float64(cfg.LoopWindow) * 0.75))` times
- OR within the last `cfg.LoopWindow` model requests, the same model+purpose combination repeated ≥ `int(math.Ceil(float64(cfg.LoopWindow) * 0.75))` times
- AND no stage transitions occurred among those last `cfg.LoopWindow` invocations (i.e., all invocations in the window belong to the same stage)

Produces `warning` severity alert with type `loop.detected`.

Compute loop score: `repetitionCount / windowSize`. Exposed in RunState.

#### 8.4 Budget Threshold Rule

Three checks:
- `state.TotalCostUSD > cfg.MaxRunCostUSD` → `error` severity, type `budget.threshold.exceeded`
- Per-stage cost > `cfg.MaxStageCostUSD` → `warning` severity
- Total tokens > `cfg.MaxTokens` (if configured) → `warning` severity

#### 8.5 Retry Storm Rule

Trigger: within the last 10 tool invocations, ≥ 5 have the same tool name AND status=failed.

Produces `warning` severity alert with type `retry.storm`.

#### 8.6 Failure Density Rule

Trigger: ≥ 3 failure events (`test.failed`, `tool.failed`, `model.request.failed`) within a 60-second window.

Produces `warning` severity alert with type `failure.density.high`.

**Note**: Drift detection (SPEC §15.1) is deferred to post-v1. It requires correlating intent metadata with artifact state, which adds significant complexity beyond the core alert rules. SPEC §34 acceptance criterion #5 requires "basic stalls, loops, retries, and budget threshold warnings" — drift is not in that list.

#### 8.7 Integration with Live Pipeline

Phase 5's `StoreSink` defines an optional `AlertHook` interface (nil by default). In Phase 8, create `alerts.Engine` implementing `AlertHook`, and wire it into `StoreSink` at construction time in `witness run`. The alert engine evaluates after each `aggregator.Apply` call; new alerts are appended as `alert.raised` events.

Alerts appear in the TUI alerts panel immediately.

### Tests

- `alerts/stall_test.go`: stall fires after threshold, doesn't fire when recent file changes exist, doesn't fire for completed runs
- `alerts/loop_test.go`: loop fires for repeated tool calls, doesn't fire when stage transitions occur within window
- `alerts/budget_test.go`: fires when cost exceeds threshold, fires per-stage, doesn't fire below threshold
- `alerts/retry_test.go`: fires for repeated failures of same tool, doesn't fire for different tools
- `alerts/density_test.go`: fires for clustered failures, doesn't fire for spread-out failures
- `alerts/engine_test.go`: deduplication prevents double-raise, multiple rules can fire independently

### Acceptance Criteria

- [ ] Stall alert fires after configurable inactivity period
- [ ] Loop alert fires for repetitive tool/model patterns
- [ ] Budget alert fires when cost or token thresholds exceeded
- [ ] Retry storm alert fires for repeated tool failures
- [ ] Failure density alert fires for clustered errors
- [ ] Alerts appear in TUI and are persisted as events
- [ ] Alerts are not duplicated on re-evaluation
- [ ] `go test ./internal/alerts/...` passes
- [ ] `golangci-lint run ./...` clean

---

## Phase 9: TUI Replay Mode

**Goal**: Connect the replay controller (Phase 6) to the TUI (Phase 7) for visual replay of historical runs.

**Spec sections**: §23.2 (Replay Controls), §13.10 (Drill-Down Views)

### Deliverables

#### 9.1 Replay TUI Mode

`witness replay <run-id>` opens the TUI in replay mode:

- Same panel layout as live mode
- Additional replay control bar at the bottom:
  ```
  ▸ Playing 2x | Event 142/891 | 14:23:19 | [space] play/pause [←/→] step [</>] speed [n] next stage [c] next commit
  ```
- State updates driven by the replay controller instead of live events

#### 9.2 Replay Key Bindings

| Key | Action |
|-----|--------|
| `space` | Play/pause |
| `→` / `l` | Step forward one event |
| `←` / `h` | Step backward one event |
| `>` / `.` | Increase speed (1x → 2x → 4x → 8x → 16x) |
| `<` / `,` | Decrease speed |
| `n` | Jump to next stage transition |
| `N` | Jump to previous stage transition |
| `c` | Jump to next commit |
| `C` | Jump to previous commit |
| `A` | Jump to next alert |

All existing dashboard keys (`tab`, `j/k`, drill-downs) still work.

#### 9.3 Timeline Scrubber

Show a text-based progress bar in the replay control bar:
```
[████████░░░░░░░░░░░░] 16%
```

Based on `controller.Progress()`.

### Tests

- Integration test: load a fixture run with known events, step through replay, verify state at key points matches expected
- Replay control bar renders correctly at various terminal widths

### Acceptance Criteria

- [ ] `witness replay <run-id>` opens TUI in replay mode
- [ ] Play/pause/step controls work
- [ ] Speed adjustment works
- [ ] Jump-to navigation works
- [ ] Dashboard panels update correctly during replay
- [ ] Progress bar reflects current position
- [ ] `go test ./...` passes
- [ ] `golangci-lint run ./...` clean

---

## Phase 10: External Tool Integration

**Goal**: Implement structured parsing for external tool output and define the integration contract.

**Spec sections**: §18 (External Tool Integration Contract), §19 (Model Telemetry)

### Deliverables

#### 10.1 Tool Result Parser (`internal/ingest`)

```go
type ToolResult struct {
    Tool       string                    `json:"tool"`
    Status     string                    `json:"status"`
    Summary    string                    `json:"summary"`
    Findings   map[string]int            `json:"findings,omitempty"`
    Artifacts  *ToolArtifacts            `json:"artifacts,omitempty"`
    DurationMS int                       `json:"duration_ms,omitempty"`
    Tokens     *ToolTokens              `json:"tokens,omitempty"`
    Model      string                    `json:"model,omitempty"`
    Provider   string                    `json:"provider,omitempty"`
    Extra      map[string]json.RawMessage `json:"extra,omitempty"`
}

func ParseToolResult(line []byte) (*ToolResult, error)
func ToolResultToEvents(result ToolResult, runID, stageID string) []events.Event
```

`ToolResultToEvents` converts a structured result into:
- `tool.completed` event with findings and duration
- `model.request.completed` event if tokens/model info present
- `finding.recorded` events for each finding category with count > 0

#### 10.2 Known Tool Adapters

Parsing adapters for specific tools that may emit non-standard structured output:

```go
type ToolAdapter interface {
    CanParse(line []byte) bool
    Parse(line []byte) (*ToolResult, error)
}
```

Initial adapters:
- **Generic JSON**: Parses the standard `ToolResult` schema. This is the primary adapter and handles any tool that emits the contract defined in §10.1.
- **Prism**: Parses prism `--json` output format. Adapter implementation requires documenting the actual Prism JSON schema by running `prism review --json` and inspecting output. If the schema is not available at implementation time, defer to Generic JSON only.
- **SpecCritic/PlanCritic**: Parses `--json` output format. Same approach — document actual schema from tool output. Defer if unavailable.

The Generic JSON adapter is the v1 minimum. Tool-specific adapters are best-effort and may be deferred to post-v1 if output format documentation is not readily available.

The adapter registry tries each adapter in order; first match wins.

#### 10.3 Model Pricing Lookup

Enhance `internal/config/pricing.go`:

```go
func EstimateCost(provider, model string, inputTokens, outputTokens, cachedTokens int64) float64
```

Uses the pricing table from config. Returns 0 if model not found (don't guess). Log a warning once per unknown model.

Built-in pricing for:
- Anthropic: claude-opus-4-6, claude-sonnet-4-6, claude-haiku-4-5
- OpenAI: gpt-4o, gpt-4o-mini, o1, o3, o3-mini, o4-mini

#### 10.4 Event Schema Documentation

Write `docs/event-schema.md` documenting:
- Event envelope fields (required/optional)
- All event types with their expected payload shapes
- Structured tool result contract
- Examples for each integration level

This is the integration guide for tool authors.

### Tests

- `ingest/toolresult_test.go`: Parse standard tool result, convert to events, handle missing optional fields
- `ingest/adapters_test.go`: Each adapter parses its format, rejects other formats
- `config/pricing_test.go`: Known models return correct cost, unknown models return 0

### Acceptance Criteria

- [ ] Structured tool JSON parsed from subprocess output
- [ ] Tool results converted to appropriate Witness events
- [ ] Model cost estimation works for known providers
- [ ] Unknown tool output formats handled gracefully (passed through, not error)
- [ ] `docs/event-schema.md` covers all event types from SPEC §10.5 with payload shapes and at least one example per integration level
- [ ] `go test ./...` passes
- [ ] `golangci-lint run ./...` clean

---

## Phase Summary

| Phase | Name | Key Packages | Depends On |
|-------|------|-------------|-----------|
| 1 | Project Scaffolding & Domain Models | `models`, `events`, `config`, `version` | — |
| 2 | Event Persistence & Run Storage | `store`, `store/fsstore` | Phase 1 |
| 3 | State Aggregation & Metrics | `aggregate` | Phases 1, 2 |
| 4 | CLI Commands — Inspection & Export | `export`, `cmd/witness` | Phases 1–3 |
| 5 | Live Capture — `witness run` | `app`, `git`, `files`, `ingest`, `privacy` | Phases 1–3 |
| 6 | Replay Engine | `replay` | Phases 1–3 |
| 7 | Terminal UI — Live Dashboard | `tui`, `ui/panels`, `ui/state` | Phases 1–5 |
| 8 | Alerts & Heuristics | `alerts` | Phases 1–3, 5 (StoreSink integration) |
| 9 | TUI Replay Mode | `tui` (extension) | Phases 6, 7 |
| 10 | External Tool Integration | `ingest` (extension), `config/pricing` | Phases 1–5 |

Phases 4, 5, and 6 can be developed in parallel after Phase 3 completes. Phase 7 depends on Phase 5 (live event stream). Phase 8 depends on Phases 1–3 for the alert rules themselves, plus Phase 5 for `StoreSink` integration. Phase 9 depends on Phases 6 and 7. Phase 10 can begin any time after Phase 5.

---

## Risk Register

| Risk | Impact | Mitigation |
|------|--------|------------|
| Bubble Tea learning curve delays TUI phase | Medium | Phase 7 is isolated; core functionality works without TUI via CLI commands |
| fsnotify misses events on macOS (known issue with kqueue) | Low | Debouncing and Git polling provide redundant detection |
| Event log files grow large for long runs | Medium | Snapshot-based rehydration avoids full replay on startup; retention config for cleanup |
| Aggregator state drift from event log | High | Rebuild-from-log test in every phase; snapshot verification against rebuild |
| Subprocess stdout parsing misidentifies non-JSON lines as events | Low | Strict JSON validation + require `event_id` field present before treating as event |
| Concurrent file writes to events.ndjson | Medium | `sync.Mutex` in fsstore serializes all appends; callers may invoke from multiple goroutines safely |
