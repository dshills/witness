# Witness Architecture Document

## 1. System Overview

Witness is a terminal-first observability and monitoring platform for AI-driven software development workflows, implemented in Go as a single dependency-light CLI binary. It captures structured, append-only telemetry events from coding agents, LLM API calls, CLI tools, Git repositories, test runners, and workflow orchestration systems, then aggregates that event stream into a normalized run/session state model that is persisted locally and rendered in a live terminal dashboard. Operators use Witness to gain real-time visibility into what an autonomous coding workflow is doing, what it has changed, what it has cost, and whether it is making genuine progress or looping, stalling, or drifting — and to replay and perform postmortems on completed or failed runs without requiring any external database or network service.

---

## 2. Components

> **Note:** The machine-extracted fact model for this repository returned an empty component list (`"components": []`), indicating that static analysis did not resolve individual source files at extraction time. All component descriptions below are derived exclusively from the authoritative `SPEC.md` and `PLAN.md` context documents provided alongside the fact model. No source-file-level claims are made beyond what those documents specify.

### 2.1 `cmd/witness` — CLI Entry Point

**Planned source:** `cmd/witness/main.go`

The top-level Cobra command tree and program entry point. Registers all subcommands, loads configuration, and delegates to internal packages. In v1 the binary is a single unified CLI with no daemon split.

**Subcommands defined here:**

| Command | Purpose |
|---|---|
| `witness version` | Print build version, commit, and date |
| `witness runs` | List known runs with summary metadata |
| `witness inspect <run-id>` | Print detailed textual run summary |
| `witness stats <run-id>` | Print aggregated operational metrics |
| `witness export <run-id>` | Export run data as JSON, NDJSON, or Markdown |
| `witness doctor` | Validate local setup and system readiness |
| `witness config show` | Display effective configuration |
| `witness run -- <cmd>` | Execute a child command under Witness instrumentation |
| `witness watch` | Open live TUI for the current or specified run |
| `witness attach --run <id>` | Attach TUI to an existing active run |
| `witness replay <run-id>` | Replay a historical run through the TUI or textual timeline |

---

### 2.2 `internal/models` — Domain Model Types

**Planned sources:** `internal/models/run.go`, `stage.go`, `tool.go`, `model_request.go`, `file_change.go`, `commit.go`, `alert.go`, `enums.go`

Defines all core entity structs and their associated enumerations. These types are the shared vocabulary used by every other internal package.

| Type | Purpose |
|---|---|
| `Run` | Top-level unit of workflow execution; carries `run_id`, status, repo metadata, labels |
| `Stage` | A major workflow phase within a run (e.g., Implement, Verify, Test) |
| `ToolInvocation` | A single execution of a CLI tool or sub-tool |
| `ModelRequest` | A single LLM/API request with token counts and cost |
| `FileChange` | A meaningful filesystem change observed during the run |
| `Commit` | A Git commit created during the run |
| `Alert` | A health or anomaly signal with severity and acknowledgement state |
| `RunStatus`, `StageStatus`, `ChangeType`, `Severity` | Typed enumerations with `String()`, `MarshalJSON()`, `UnmarshalJSON()` |

---

### 2.3 `internal/events` — Event Envelope and Schema

**Planned sources:** `internal/events/event.go`, `types.go`, `sink.go`, `validate.go`

Defines the immutable, append-only event model that is the authoritative source of truth for all run state. Every observable occurrence in a workflow is represented as an `Event`.

- `event.go`: The `Event` struct with required fields (`event_id`, `schema_version`, `timestamp`, `run_id`, `type`, `source`, `payload`) and optional fields (`stage_id`, `status`, `summary`, `trace_id`, `span_id`, `parent_event_id`, `tags`, `labels`). `Payload` is `json.RawMessage` to preserve unknown fields.
- `types.go`: `EventType` string constants covering all event categories: run lifecycle, stage lifecycle, tool lifecycle, model lifecycle, Git/repository, test/validation, findings/review, system/health, and narrative/annotation.
- `sink.go`: The `EventSink` interface (`Append(ctx, evt) error`) — the single write contract consumed by all event-producing subsystems.
- `validate.go`: `Validate(evt Event) error` — checks all required fields, known schema version, non-nil payload, and recognized event type. Returns a structured multi-violation error.

Also provides `NewID(prefix string) string` using time-ordered ULIDs for all entity identifiers.

---

### 2.4 `internal/config` — Configuration Loading

**Planned sources:** `internal/config/config.go`, `defaults.go`, `pricing.go`, `config_test.go`

Loads and merges configuration from three sources in precedence order: CLI flags (handled at the Cobra layer), environment variables (`WITNESS_*`), and a YAML config file (`~/.witness/config.yaml`), falling back to compiled-in defaults.

- `config.go`: `Config` struct with sub-structs for `StorageConfig`, `UIConfig`, `AlertsConfig`, `FilesConfig`, `PrivacyConfig`, `GitConfig`, `PricingConfig`, `CaptureConfig`. `AlertsConfig` uses `time.Duration` for threshold fields.
- `defaults.go`: Compiled-in defaults — storage root `~/.witness`, UI refresh 500ms, stall threshold 10 minutes, loop window 8, max run cost $25.00, max stage cost $8.00, default file ignore patterns.
- `pricing.go`: Built-in model pricing table (Anthropic Claude family, OpenAI GPT-4/4o/o1/o3 family) with `EstimateCost(provider, model, inputTokens, outputTokens, cachedTokens)`. User-overridable via config. Unknown models return 0 and log a one-time warning.

---

### 2.5 `internal/store` / `internal/store/fsstore` — Event Persistence

**Planned sources:** `internal/store/` (interface), `internal/store/fsstore/` (implementation)

Implements durable local storage with no external database dependency.

The `Store` interface exposes:
- Run CRUD: `CreateRun`, `GetRun`, `UpdateRun`, `ListRuns`, `DeleteRun`
- Event operations: `AppendEvent`, `ReadEvents`, `StreamEvents`
- Snapshot operations: `SaveSnapshot`, `LoadSnapshot`

The `fsstore` implementation uses the following on-disk layout:

```
~/.witness/
  config.yaml
  runs/
    <run-id>/
      run.json        # Run metadata (atomic write via temp-file rename)
      events.ndjson   # Append-only newline-delimited JSON event log
      snapshot.json   # Optional aggregated state snapshot (atomic write)
      export/         # Export output directory
```

Key durability behaviors:
- `events.ndjson` opened with `O_APPEND|O_WRONLY|O_CREATE`; each write is followed by `f.Sync()`
- Partial final lines on recovery are silently discarded
- Snapshot writes use `snapshot.json.tmp` → `os.Rename` for atomicity
- Duplicate event IDs deduplicated via an in-memory ring buffer (capacity 256)
- `StreamEvents` tails the NDJSON file via `fsnotify`; slow consumers drop events rather than blocking the append path
- All appends serialized via `sync.Mutex` for concurrent-caller safety

---

### 2.6 `internal/aggregate` — State Aggregation and Metrics

**Planned sources:** `internal/aggregate/` (aggregator, RunState, metrics)

Derives live `RunState` from the append-only event stream. This is the single source of truth for all rendered and exported state.

- `RunState`: Comprehensive snapshot struct holding the current `Run`, all `Stages`, `ActiveStage`, `ActiveTool`, `ActiveModel`, token/cost totals broken down by provider/model/stage, tool invocation history, Git/file state, alert sets, a 200-event ring buffer of recent events, and timing fields (`LastEventAt`, `LastFileChangeAt`, `LastCommitAt`, `LastStageChangeAt`).
- `Aggregator`: Thread-safe (`sync.RWMutex`) event processor. `Apply(evt)` dispatches on `EventType` to update the appropriate state fields. `Snapshot()` returns a copy under read lock.
- Metrics methods on `RunState`: `Duration()`, `StageDurations()`, `TokenBurnRate(window)`, `CostBurnRate(window)`, `AvgToolLatency()`, `AvgModelLatency()`, `MeanTimeBetweenCommits()`, `UniqueFilesTouched()`.
- `Rebuild(run, events)`: Reconstructs a `RunState` from scratch by replaying all events through a fresh `Aggregator`. Used for historical run loading and snapshot verification.
- `RunState` is JSON-serializable for persistence as `snapshot.json`.

---

### 2.7 `internal/git` — Git Repository Observer

**Planned sources:** `internal/git/` (observer, helpers)

Polls Git state at a configurable interval (default 5 seconds) and emits events to the `EventSink`.

- `Observer`: Polls `git symbolic-ref`, `git status --porcelain`, and `git log` via `os/exec` with timeouts. Emits `git.branch.changed`, `repo.status.changed`, and `git.commit.created` events on detected changes. On first poll, records current HEAD without emitting history.
- Helper functions: `DetectRepoRoot(path)`, `CurrentBranch(repoRoot)`, `ParseCommit(repoRoot, sha)` — used by both the observer and CLI inspection commands.
- Failures are logged and non-fatal per the graceful degradation requirement.

---

### 2.8 `internal/files` — Filesystem Watcher

**Planned sources:** `internal/files/` (watcher)

Monitors the repository root for file changes using `fsnotify` and emits `file.created`, `file.modified`, and `file.deleted` events.

- Applies include/exclude glob patterns using `github.com/bmatcuk/doublestar/v4`.
- Default ignores: `.git/`, `node_modules/`, `vendor/`, `dist/`, `build/`, `.next/`, editor swap files, `.DS_Store`.
- Debounces events for the same path within 100ms to suppress editor save-sequence noise.
- Failures degrade gracefully; file watching is non-fatal.

---

### 2.9 `internal/ingest` — Event and Tool Output Ingestion

**Planned sources:** `internal/ingest/` (scanner, tool result parser, adapters)

Parses external input and normalizes it into Witness events.

- `Scanner`: Reads lines from subprocess stdout. Lines that parse as valid `events.Event` (with `event_id` and `type`) are ingested directly (with `run_id` overridden to the current run). Lines matching the structured tool result schema are converted. Other lines pass through.
- `ToolResult`: Structured tool output contract (`tool`, `status`, `summary`, `findings`, `artifacts`, `duration_ms`, `tokens`, `model`, `provider`).
- `ToolResultToEvents`: Converts a `ToolResult` into `tool.completed`, optionally `model.request.completed`, and `finding.recorded` events.
- `ToolAdapter` interface with a registry: Generic JSON adapter (primary), with optional Prism and SpecCritic/PlanCritic adapters deferred pending schema documentation.

---

### 2.10 `internal/privacy` — Redaction

**Planned sources:** `internal/privacy/redactor.go`, `redactor_test.go`

Applies configurable regex-based redaction to event content before persistence.

- `Redactor`: Compiled from configured patterns plus built-in defaults (API keys matching `sk-...`, AWS `AKIA...` keys, bearer tokens, generic `password=`/`secret=` patterns).
- Applied to event summaries, tool output, and string values within event payloads (walking the JSON object). Never applied to structural field names, IDs, or timestamps.

---

### 2.11 `internal/alerts` — Anomaly Detection Engine

**Planned sources:** `internal/alerts/` (engine, rules)

Evaluates aggregated `RunState` against heuristic rules and emits `alert.raised` events for new anomalies.

- `Rule` interface: `Name() string`, `Evaluate(state, cfg) []models.Alert`
- `Engine`: Runs all registered rules, deduplicates against previously raised alert IDs, returns only new alerts.

**Implemented rules:**

| Rule | Trigger | Severity |
|---|---|---|
| Stall Detection | No file changes or stage transitions for `cfg.StallDuration` while run is active | `warning` |
| Loop Detection | Same tool invoked ≥75% of last N invocations with no stage transitions | `warning` |
| Budget Threshold | Run cost > `MaxRunCostUSD` or stage cost > `MaxStageCostUSD` | `error` / `warning` |
| Retry Storm | ≥5 of last 10 tool invocations for the same tool have status=failed | `warning` |
| Failure Density | ≥3 failure events within a 60-second window | `warning` |

Stall and loop scores are computed as normalized floats (0.0–1.0) and exposed on `RunState` for TUI display.

---

### 2.12 `internal/replay` — Replay Controller

**Planned sources:** `internal/replay/` (controller, postmortem)

Provides time-ordered playback of historical event streams.

- `Controller`: Holds the full event slice and current index. Supports `Play`, `Pause`, `SetSpeed`, `StepForward`, `StepBackward` (O(n) rebuild), `JumpToIndex`, `JumpToTime`, and navigation shortcuts (`JumpToNextStageTransition`, `JumpToNextAlert`, `JumpToNextCommit`).
- `Play` runs in a goroutine, advancing events with delays proportional to actual timestamp gaps scaled by `speed`. Sends `RunState` updates via a buffered channel (capacity 1) with non-blocking send to avoid stalling on slow TUI consumers.
- `GeneratePostmortem(state RunState) string`: Produces a concise textual summary of run objective, outcome, stage durations, failures, cost, Git activity, and notable alerts.

---

### 2.13 `internal/export` — Export Formatters

**Planned sources:** `internal/export/` (json.go, ndjson.go, markdown.go)

Three `Exporter` implementations writing to `io.Writer`:

- **JSON**: Single structured object with `run`, `stages`, `tools`, `model_requests`, `file_changes`, `commits`, `alerts`, `metrics`, `events`.
- **NDJSON**: Raw event stream, one validated JSON line per event.
- **Markdown**: Human-readable postmortem report with summary table, stage table, token usage, commits, and alerts sections.

---

### 2.14 `internal/tui` — Terminal UI Shell

**Planned sources:** `internal/tui/` (app, keys), `internal/ui/panels/`, `internal/ui/state/`

Bubble Tea (`charmbracelet/bubbletea`) application implementing the live dashboard and replay UI.

- `App`: Root Bubble Tea model. Owns layout calculation, panel focus routing, and keyboard dispatch.
- **Panels** (each implements `Panel` interface with `Init`, `Update`, `View`, `Title`, `Focusable`):
  - `HeaderPanel`: Run name, ID, repo, branch, live duration, color-coded status
  - `StagePanel`: Ordered stage list with status icons, progress bar, current step summary
  - `ActiveWorkPanel`: Active tool/model, request count, latency, retry count
  - `TokenCostPanel`: Token totals, cost, per-provider/model breakdown, burn rate, budget warning
  - `GitFilePanel`: File change counts, last commit, recent/hot files, dirty state
  - `AlertsPanel`: Active alerts with severity-colored markers