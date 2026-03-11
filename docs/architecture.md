# Witness Architecture Document

## 1. System Overview

Witness is a terminal-first observability and monitoring platform for AI-driven software development workflows, implemented in Go as a single dependency-light CLI binary. It captures structured, append-only telemetry events from coding agents, LLM API calls, CLI tools, Git repositories, test runners, and workflow orchestration systems, then aggregates that event stream into a normalized run/session state model that is persisted locally and rendered in a live terminal dashboard. Operators use Witness to gain real-time visibility into what an autonomous AI coding workflow is doing, how much it is costing, whether it is making meaningful progress, and what artifacts it has changed — and to replay, inspect, and export historical runs for postmortem analysis.

---

## 2. Components

> **Note:** The machine-extracted fact model for this repository returned an empty component list (`"components": []`), indicating that static analysis did not resolve individual source files at extraction time. All component descriptions below are derived directly from the authoritative `SPEC.md` and `PLAN.md` documents provided as spec and plan context. No endpoints, datastores, or integrations were fabricated beyond what those documents specify.

### 2.1 `cmd/witness` — CLI Entry Point

**Source files:** `cmd/witness/main.go`

The top-level binary entry point. Constructs the Cobra command tree and wires together all internal packages. Registers all subcommands (`run`, `watch`, `attach`, `runs`, `inspect`, `replay`, `export`, `stats`, `doctor`, `config show`, `version`). Loads configuration and passes it downstream. Sets version metadata via `-ldflags` at build time.

### 2.2 `internal/app` — Application Orchestration

**Source files:** `internal/app/app.go`

Owns the `witness run -- <command>` execution lifecycle. Responsibilities include creating a new run record, registering OS signal handlers (SIGINT/SIGTERM with graceful subprocess forwarding and a 10-second forced-kill deadline), launching the subprocess in its own process group, starting observation goroutines (Git poller, file watcher, stdin ingestion), relaying subprocess stdout/stderr to the terminal, and emitting `run.created`, `run.started`, `run.completed`, and `run.failed` events. Also contains the `StoreSink` adapter that bridges all event producers to the store and aggregator.

### 2.3 `internal/config` — Configuration Loading

**Source files:** `internal/config/config.go`, `internal/config/config_test.go`, `internal/config/defaults.go`, `internal/config/pricing.go`

Defines the `Config` struct and all sub-domain config types (`StorageConfig`, `UIConfig`, `AlertsConfig`, `FilesConfig`, `PrivacyConfig`, `GitConfig`, `PricingConfig`, `CaptureConfig`). Implements a three-layer loading precedence: YAML config file → `WITNESS_*` environment variables → compiled-in defaults. `defaults.go` provides baseline values (storage root `~/.witness`, 500ms refresh, 10-minute stall threshold, $25 max run cost, $8 max stage cost, default file ignore patterns). `pricing.go` provides a built-in model pricing table covering Anthropic Claude and OpenAI GPT families, used for cost estimation; user-overridable via config.

### 2.4 `internal/events` — Event Schema and Validation

**Source files:** `internal/events/event.go`, `internal/events/types.go`, `internal/events/sink.go`, `internal/events/validate.go`, `internal/events/validate_test.go`

Defines the canonical `Event` struct (the base envelope per SPEC §10.2), the `EventType` string-typed constant set covering all lifecycle categories (run, stage, tool, model, git/repo, test/validation, findings/review, system/health, narrative/annotation), and the `EventSink` interface (`Append(ctx, evt) error`). `validate.go` implements `Validate(evt Event) error`, checking all required fields and returning a structured multi-violation error. Uses `json.RawMessage` for `Payload` to preserve unknown fields across serialization boundaries.

### 2.5 `internal/models` — Domain Entity Types

**Source files:** `internal/models/run.go`, `internal/models/stage.go`, `internal/models/tool.go`, `internal/models/model_request.go`, `internal/models/file_change.go`, `internal/models/commit.go`, `internal/models/alert.go`, `internal/models/enums.go`

Defines all core domain structs: `Run`, `Stage`, `ToolInvocation`, `ModelRequest`, `FileChange`, `Commit`, and `Alert`, with all fields specified in SPEC §9.1. `enums.go` defines typed enum constants (`RunStatus`, `StageStatus`, `ChangeType`, `Severity`) using `iota`-based constants, each implementing `String()`, `MarshalJSON()`, and `UnmarshalJSON()` for type-safe JSON round-tripping.

### 2.6 `internal/store` / `internal/store/fsstore` — Event and Run Persistence

**Source files:** `internal/store/` (interface), `internal/store/fsstore/` (implementation)

Defines the `Store` interface covering run CRUD, event append/read/stream, and snapshot save/load. The `fsstore` implementation persists data to the local filesystem under `~/.witness/runs/<run-id>/` using three files: `run.json` (atomic-write run metadata), `events.ndjson` (append-only, `O_APPEND`-mode event log, synced after each write), and `snapshot.json` (atomically replaced via temp-file rename). Implements crash tolerance by discarding unparseable final lines on read. Deduplicates events using an in-memory ring buffer of the last 256 event IDs. `StreamEvents` tails the NDJSON file via `fsnotify` and delivers events on a buffered channel (capacity 256), dropping events on slow consumers rather than blocking the append path.

### 2.7 `internal/aggregate` — State Aggregation and Metrics

**Source files:** `internal/aggregate/` (aggregator, RunState, metrics)

Defines `RunState` — the complete derived view of a run including active stage/tool/model, token and cost totals broken down by provider/model/stage, Git state, file change history, alert set, a 200-event ring buffer of recent events, and timing fields. The `Aggregator` dispatches incoming events by type to handler methods that mutate state under a `sync.RWMutex`. `Snapshot()` returns a copy under read lock. `Rebuild(run, events)` reconstructs state from scratch for historical runs and snapshot verification. Metric methods (`TokenBurnRate`, `CostBurnRate`, `StageDurations`, `AvgToolLatency`, etc.) derive values from accumulated state without a separate metrics store.

### 2.8 `internal/alerts` — Anomaly Detection Engine

**Source files:** `internal/alerts/` (engine, stall rule, loop rule, budget rule, retry rule, density rule)

Defines the `Rule` interface and `Engine` struct. The engine runs all registered rules against the current `RunState`, deduplicates against previously raised alert IDs, and returns only new alerts. Implements five rules: **Stall Detection** (no file changes or stage transitions for configurable duration while running), **Loop Detection** (same tool or model+purpose repeated ≥75% of the last N invocations without stage transitions), **Budget Threshold** (run cost, stage cost, or token count exceeds configured limits), **Retry Storm** (≥5 failures of the same tool in the last 10 invocations), and **Failure Density** (≥3 failure events within a 60-second window). Wired into `StoreSink` via the `AlertHook` interface.

### 2.9 `internal/git` — Git Repository Observer

**Source files:** `internal/git/` (observer, helpers)

Implements a polling `Observer` that runs `git` subcommands at a configurable interval (default 5 seconds) to detect branch changes, dirty working tree state, and new commits. Emits `git.branch.changed`, `repo.status.changed`, and `git.commit.created` events via `EventSink`. Helper functions (`DetectRepoRoot`, `CurrentBranch`, `ParseCommit`) are also used by CLI inspection commands. All Git commands run via `os/exec` with timeouts; failures degrade gracefully per SPEC §25.2.

### 2.10 `internal/files` — Filesystem Watcher

**Source files:** `internal/files/` (watcher)

Implements a `Watcher` using `fsnotify` to observe file changes under the repo root. Classifies events as `file.created`, `file.modified`, or `file.deleted`. Applies include/exclude glob patterns using `doublestar` for `**` support. Default ignores: `.git/`, `node_modules/`, `vendor/`, `dist/`, `build/`, `.next/`, editor swap files. Debounces events for the same path within 100ms to suppress editor save noise.

### 2.11 `internal/ingest` — External Event and Tool Output Ingestion

**Source files:** `internal/ingest/` (scanner, tool result parser, adapters)

The `Scanner` reads lines from subprocess stdout, attempting to parse each as a full Witness `Event` (requires `event_id` + `type`), then as a structured `ToolResult` (the minimum integration contract: `tool`, `status`, `summary`, `findings`, `duration_ms`), and otherwise passing the line through. `ToolResultToEvents` converts a `ToolResult` into `tool.completed`, optionally `model.request.completed`, and `finding.recorded` events. A `ToolAdapter` registry supports pluggable parsers; the Generic JSON adapter is the v1 baseline, with tool-specific adapters (Prism, SpecCritic/PlanCritic) as best-effort additions.

### 2.12 `internal/privacy` — Redaction Engine

**Source files:** `internal/privacy/redactor.go`, `internal/privacy/redactor_test.go`

Compiles configured regex patterns into a `Redactor` that scrubs sensitive strings from event summaries, tool output, and payload string values before persistence. Default patterns cover OpenAI/Anthropic API keys, AWS access keys, bearer tokens, and generic `password`/`secret`/`apikey` assignments. Applied by `StoreSink` on every event before it reaches the store.

### 2.13 `internal/replay` — Replay Controller

**Source files:** `internal/replay/` (controller, postmortem)

The `Controller` holds a loaded event slice and an `Aggregator`, supporting forward/backward stepping, time-based jumping, and automatic playback at configurable speed multipliers. `StepBackward` and backward jumps rebuild state from event 0 to the target index (O(n), acceptable for v1 volumes). `Play` runs in a goroutine, advancing events with delays proportional to actual timestamp gaps scaled by speed, sending `RunState` updates on a non-blocking buffered channel. `GeneratePostmortem` produces a concise textual summary of run outcome, time distribution, failures, cost, and Git activity.

### 2.14 `internal/tui` — Terminal UI Shell

**Source files:** `internal/tui/` (app model, layout engine, key dispatch)

The Bubble Tea root model (`App`) owns the panel hierarchy, layout engine, and keyboard routing. Supports two layout modes: full (≥100×28) with a two-column panel grid, and compact (<100 cols) with a single-column stacked subset. A background goroutine subscribes to `store.StreamEvents`, applies events to the aggregator, and sends `StateMsg` to the Bubble Tea program at the configured refresh interval (default 500ms). In replay mode, the state bridge is replaced by the replay controller's update channel.

### 2.15 `internal/ui/panels` — TUI Panel Implementations

**Source files:** `internal/ui/panels/` (header, stage, active work, token/cost, git/file, alerts, event stream)

Seven panel implementations, each satisfying the `Panel` interface (`Init`, `Update`, `View(width, height)`, `Title`, `Focusable`): **HeaderPanel** (run name, ID, repo, branch, live duration, color-coded status), **StagePanel** (ordered stage list with icons and progress bars), **ActiveWorkPanel** (active tool/model, latency, retry count), **TokenCostPanel** (token totals, cost by provider/model, burn rate, budget warnings), **GitFilePanel** (file change counts, recent commits, hot files, dirty state), **AlertsPanel** (active alerts with severity colors), and **EventStreamPanel** (scrollable viewport with pause/filter support).

### 2.16 `internal/export` — Run Export Formatters

**Source files:** `internal/export/` (json, ndjson, markdown exporters)

Three `Exporter` implementations writing to `io.Writer`. The **JSON exporter** produces a single structured object with all run entities and metrics. The **NDJSON exporter** writes the raw event stream one line per event. The **Markdown exporter** produces a human-readable postmortem report with tables for stages, token usage, commits, and alerts — suitable for issue trackers and documentation.

### 2.17 `internal/version` — Build Metadata

**Source files:** `internal/version/version.go`

Exports `Version`, `Commit`, and `Date` variables populated via `-ldflags` at build time. Used by the `witness version` subcommand.

---

## 3. Data Flow

### 3.1 Live Capture Flow (`witness run -- <command>`)

```
External Command (subprocess)
        │
        ├─ stdout ──► io.TeeReader ──► Terminal (relay)
        │                    │
        │                    └──► ingest.Scanner
        │                              │
        │                    ┌─────────┴──────────┐
        │                    │ Witness Event?       │ ToolResult?
        │                    ▼                      ▼
        │             events.Validate()    ingest.ToolResultToEvents()
        │                    │                      │
        ├─ stderr ──► Terminal (relay)              │
        │                                           │
        ├─ git.Observer (poll every 5s) ────────────┤
        │                                           │
        └─ files.Watcher (fsnotify) ────────────────┤
                                                    │
                                                    ▼
                                          app.StoreSink.Append()
                                                    │
                                          privacy.Redactor.Redact()
                                                    │
                                          events.Validate()
                                                    │
                                    ┌───────────────┼───────────────┐
                                    ▼               ▼               ▼
                             store.AppendEvent  aggregate.Apply  alerts.Engine.Evaluate()
                             (events.ndjson)         │                    │
                                    │          RunState snapshot    alert.raised events
                                    │                │              (fed back into Sink)
                             every 500 events        │
                             store.SaveSnapshot       │
                             (snapshot.json)          │
                                                      ▼
                                              StateMsg (500ms tick)
                                                      │
                                                      ▼
                                               tui.App (Bubble Tea)
                                                      │
                                              Panel rendering
                                              (terminal output)
```

### 3.2 Inspection Flow (`witness inspect / stats / export`)

```
CLI Command
    │
    ▼
store.GetRun(runID)          ← run.json
store.ReadEvents(runID)      ← events.ndjson
    │
    ▼
aggregate.Rebuild(run, events)
    │
    ▼
RunState
    │
    ├──► Text formatter (inspect / stats) ──► stdout
    └──► export.Exporter (json/ndjson/md) ──► stdout or file
```

### 3.3 Replay Flow (`witness replay <run-id>`)

```
store.GetRun(runID)
store.ReadEvents(runID)
        │
        ▼
replay.NewController(run, events)
        │
        ├─ Play goroutine (timestamp-paced)
        │         │
        │         ▼
        │   controller.StepForward()
        │         │
        │         ▼
        │   aggregate.Apply(evt)
        │         │
        │         ▼
        │   RunState ──► StateMsg channel (non-blocking)
        │
        └─ tui.App (Bubble Tea) ◄── State