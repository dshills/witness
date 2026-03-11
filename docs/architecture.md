# Witness Architecture Document

## 1. System Overview

Witness is a terminal-first observability and monitoring platform for AI-driven software development workflows, implemented in Go as a single dependency-light CLI binary. It captures structured, append-only telemetry events from coding agents, LLM API calls, CLI tools, Git repositories, test runners, and workflow orchestration systems, then aggregates that event stream into a normalized run/session state model that is persisted locally and rendered in a live terminal dashboard. Operators use Witness to gain real-time visibility into what an autonomous AI coding workflow is doing, how much it is costing, whether it is making meaningful progress, and what artifacts it has changed — and to replay, inspect, and export historical runs for postmortem analysis.

---

## 2. Components

> **Note:** The machine-extracted fact model for this repository returned an empty component list (`"components": []`), indicating that static analysis did not resolve individual source files at extraction time. All component descriptions below are derived exclusively from the authoritative `SPEC.md` and `PLAN.md` context documents provided alongside the fact model. No source file paths are available to prefix claims against; all claims are attributed to the specification and plan documents instead.

### 2.1 `cmd/witness` — CLI Entry Point

**Source:** `cmd/witness/main.go` (specified in `PLAN.md §1.1`)
**Purpose:** Cobra-based command tree that is the single user-facing binary. Registers all subcommands, loads configuration, and delegates to internal packages. Subcommands include `version`, `runs`, `inspect`, `stats`, `export`, `doctor`, `config show`, `run`, `watch`, `attach`, and `replay`.

---

### 2.2 `internal/app` — Application Orchestration

**Source:** `internal/app/app.go` (specified in `PLAN.md §1.1`, `§5.1`)
**Purpose:** Owns the top-level lifecycle of a `witness run` invocation. Creates a new run record, registers OS signal handlers, starts observation goroutines (Git poller, file watcher, stdin ingestion), launches the child subprocess in its own process group, waits for exit, emits lifecycle events, and coordinates clean shutdown. Also contains `StoreSink`, the adapter that bridges all event producers to the store and aggregator.

---

### 2.3 `internal/config` — Configuration Loading

**Source:** `internal/config/config.go`, `internal/config/defaults.go`, `internal/config/pricing.go` (specified in `PLAN.md §1.1`, `§1.7`)
**Purpose:** Defines the `Config` struct hierarchy covering storage paths, UI settings, alert thresholds, file ignore patterns, Git polling settings, privacy/redaction rules, and model pricing tables. Implements a three-layer loading precedence: YAML config file → `WITNESS_*` environment variables → compiled-in defaults. The pricing sub-module (`pricing.go`) holds a built-in cost-per-million-token table for Anthropic and OpenAI model families and exposes `EstimateCost(provider, model, inputTokens, outputTokens, cachedTokens)`.

---

### 2.4 `internal/events` — Event Schema and Validation

**Source:** `internal/events/event.go`, `internal/events/types.go`, `internal/events/sink.go`, `internal/events/validate.go` (specified in `PLAN.md §1.1`, `§1.3`–`§1.5`)
**Purpose:** Defines the canonical `Event` envelope struct with `json.RawMessage` payload to preserve unknown fields. Declares the `EventType` string-typed constant set covering all lifecycle categories (run, stage, tool, model, git/repo, test/validation, findings/review, system/health, narrative). Provides `Validate(evt Event) error` that checks all required fields and returns a structured multi-violation error. Defines the `EventSink` interface (`Append(ctx, evt) error`) consumed by all event-producing subsystems.

---

### 2.5 `internal/models` — Domain Entity Types

**Source:** `internal/models/run.go`, `internal/models/stage.go`, `internal/models/tool.go`, `internal/models/model_request.go`, `internal/models/file_change.go`, `internal/models/commit.go`, `internal/models/alert.go`, `internal/models/enums.go` (specified in `PLAN.md §1.1`, `§1.2`)
**Purpose:** Pure data structs for all core domain entities: `Run`, `Stage`, `ToolInvocation`, `ModelRequest`, `FileChange`, `Commit`, and `Alert`. All enum types (`RunStatus`, `StageStatus`, `ChangeType`, `Severity`) are `iota`-based constants with `String()`, `MarshalJSON()`, and `UnmarshalJSON()` implementations for type-safe serialization.

---

### 2.6 `internal/store` / `internal/store/fsstore` — Event Persistence

**Source:** `internal/store/` (specified in `PLAN.md §2.1`–`§2.4`)
**Purpose:** Defines the `Store` interface covering run CRUD, event append/read/stream, and snapshot save/load. The `fsstore` implementation persists data to the local filesystem under `~/.witness/runs/<run-id>/` using three files: `run.json` (atomic-write run metadata), `events.ndjson` (append-only, `O_APPEND`-mode event log, synced after each write), and `snapshot.json` (atomically replaced aggregated state). Implements crash-tolerant recovery by discarding partial final NDJSON lines. Provides `StreamEvents` which tails the NDJSON file via `fsnotify` and delivers events on a buffered channel. Deduplicates events using an in-memory ring buffer of the last 256 event IDs.

---

### 2.7 `internal/aggregate` — State Aggregation and Metrics

**Source:** `internal/aggregate/` (specified in `PLAN.md §3.1`–`§3.5`)
**Purpose:** Defines `RunState`, the fully derived in-memory view of a run, including active stage/tool/model, token and cost totals broken down by provider/model/stage, Git state, file change history, alert set, a 200-event ring buffer of recent events, and timing fields. The `Aggregator` struct applies events one at a time via `Apply(evt)` dispatching to per-type handlers, and exposes `Snapshot()` returning a copy under a read lock. `Rebuild(run, events)` reconstructs state from scratch for historical runs. Metric methods on `RunState` compute burn rates, average latencies, and mean time between commits.

---

### 2.8 `internal/alerts` — Anomaly Detection Engine

**Source:** `internal/alerts/` (specified in `PLAN.md §8.1`–`§8.7`)
**Purpose:** Implements the `Rule` interface and an `Engine` that evaluates all registered rules against the current `RunState` after each event. Built-in rules cover stall detection (no file changes or stage transitions beyond a configurable duration), loop detection (repeated tool or model invocations within a sliding window without stage movement), budget threshold breaches (run cost, stage cost, token count), retry storms (repeated failures of the same tool), and failure density (clustered errors within a 60-second window). The engine deduplicates alerts against previously raised IDs and emits new alerts as `alert.raised` events back through the `EventSink`.

---

### 2.9 `internal/git` — Git Repository Observer

**Source:** `internal/git/` (specified in `PLAN.md §5.2`)
**Purpose:** Polls Git state at a configurable interval (default 5 seconds) by shelling out to `git` with timeouts. Detects branch changes, dirty working tree changes, and new commits since the last known HEAD. Emits `git.branch.changed`, `repo.status.changed`, and `git.commit.created` events. Provides helper functions `DetectRepoRoot`, `CurrentBranch`, and `ParseCommit` used by both the observer and CLI inspection commands. Failures degrade gracefully without aborting the run.

---

### 2.10 `internal/files` — Filesystem Watcher

**Source:** `internal/files/` (specified in `PLAN.md §5.3`)
**Purpose:** Uses `fsnotify` to watch the repository root recursively. Classifies filesystem events as `file.created`, `file.modified`, or `file.deleted`, applies include/exclude glob patterns (using `doublestar` for `**` support), debounces events for the same path within 100ms, and emits the classified events through the `EventSink`. Default ignore list covers `.git/`, `node_modules/`, `vendor/`, `dist/`, `build/`, `.next/`, and common editor temporary files.

---

### 2.11 `internal/ingest` — External Event and Tool Output Ingestion

**Source:** `internal/ingest/` (specified in `PLAN.md §5.4`, `§10.1`–`§10.2`)
**Purpose:** Scans subprocess stdout line by line. Lines that parse as valid `events.Event` JSON (with `event_id` and `type` present) are ingested directly as Witness events with the current `run_id` injected. Lines that match the structured `ToolResult` schema are converted to `tool.completed`, `model.request.completed`, and `finding.recorded` events via `ToolResultToEvents`. A `ToolAdapter` registry allows tool-specific parsers (Generic JSON, Prism, SpecCritic/PlanCritic) to be tried in order. Unrecognized lines pass through to the terminal.

---

### 2.12 `internal/privacy` — Redaction

**Source:** `internal/privacy/redactor.go` (specified in `PLAN.md §1.1`, `§5.5`)
**Purpose:** Compiles a set of configurable regex patterns at startup and exposes `Redact(s string) string`. Applied to event summaries, tool output, and all string values within event payloads before persistence. Default patterns cover OpenAI/Anthropic API keys, AWS access keys, bearer tokens, and generic `password=`/`secret=` patterns. Never applied to structural field names, event IDs, or timestamps.

---

### 2.13 `internal/export` — Run Export Formatters

**Source:** `internal/export/` (specified in `PLAN.md §4.5`)
**Purpose:** Implements the `Exporter` interface (`Export(ctx, state, events, w io.Writer) error`) in three formats. The JSON exporter produces a single structured object with all run entities and metrics. The NDJSON exporter writes the raw validated event stream one line per event. The Markdown exporter produces a human-readable postmortem report with summary tables for stages, token usage, commits, and alerts.

---

### 2.14 `internal/replay` — Replay Controller

**Source:** `internal/replay/` (specified in `PLAN.md §6.1`–`§6.3`)
**Purpose:** Holds a loaded event slice and an `Aggregator`, and provides playback controls: `Play`, `Pause`, `SetSpeed`, `StepForward`, `StepBackward`, `JumpToIndex`, `JumpToTime`, and jump-to-next/prev navigation for stage transitions, alerts, and commits. `StepBackward` rebuilds state from event 0 to the target index. `Play` runs in a goroutine, advancing events with real-time-proportional delays scaled by speed, and sends `RunState` updates on a non-blocking buffered channel. Also provides `GeneratePostmortem(state)` for textual run summaries.

---

### 2.15 `internal/tui` — Terminal UI Shell

**Source:** `internal/tui/` (specified in `PLAN.md §7.1`–`§7.7`)
**Purpose:** Bubble Tea root application model. Owns the panel hierarchy, layout engine (full ≥100×28 two-column layout vs. compact single-column fallback), keyboard routing, drill-down view switching, and the help overlay. Connects to the live event stream via a background goroutine that feeds `StateMsg` updates at the configured refresh interval. In replay mode, receives state from the replay controller instead.

---

### 2.16 `internal/ui/panels` — TUI Panel Implementations

**Source:** `internal/ui/panels/` (specified in `PLAN.md §7.4`)
**Purpose:** Individual panel implementations for Header, Stage Progress, Active Work, Token/Cost, Git/File Activity, Alerts, and Event Stream. Each implements the `Panel` interface (`Init`, `Update`, `View(width, height)`, `Title`, `Focusable`). The Event Stream panel uses a `bubbles.viewport` for scrollable, pauseable, filterable event display.

---

### 2.17 `internal/version` — Build Metadata

**Source:** `internal/version/version.go` (specified in `PLAN.md §1.1`, `§1.8`)
**Purpose:** Exports `Version`, `Commit`, and `Date` variables populated at build time via `-ldflags`. Used by the `witness version` command.

---

## 3. Data Flow

Because the machine-extracted fact model returned no detected API endpoints or datastore connections, the following data flow is derived entirely from the `SPEC.md` and `PLAN.md` specification documents.

### 3.1 Live Capture Flow (`witness run -- <command>`)

```
External Command (subprocess)
        │
        │ stdout/stderr (piped via io.TeeReader)
        ▼
internal/ingest.Scanner
        │ parses Witness JSON events and ToolResult JSON
        │ passes raw lines to terminal relay
        ▼
internal/app.StoreSink  ◄── internal/git.Observer (polls every 5s)
        │               ◄── internal/files.Watcher (fsnotify)
        │               ◄── internal/alerts.Engine (evaluates after each Apply)
        │
        ├─► internal/privacy.Redactor (redacts sensitive strings)
        ├─► internal/events.Validate (validates envelope)
        ├─► internal/store/fsstore.AppendEvent (appends to events.ndjson)
        └─► internal/aggregate.Aggregator.Apply (updates in-memory RunState)
                │
                └─► every 500 events: store.SaveSnapshot (writes snapshot.json)
```

### 3.2 Live TUI Feed Flow (`witness watch`)

```
internal/store/fsstore.StreamEvents (tails events.ndjson via fsnotify)
        │
        ▼
background goroutine in internal/tui
        │ applies events to Aggregator
        │ sends StateMsg every 500ms
        ▼
internal/tui.App (Bubble Tea model)
        │
        ├─► internal/ui/panels.HeaderPanel
        ├─► internal/ui/panels.StagePanel
        ├─► internal/ui/panels.ActiveWorkPanel
        ├─► internal/ui/panels.TokenCostPanel
        ├─► internal/ui/panels.GitFilePanel
        ├─► internal/ui/panels.AlertsPanel
        └─► internal/ui/panels.EventStreamPanel
                        │
                        ▼
                   Terminal (stdout)
```

### 3.3 Inspection and Export Flow (`witness inspect`, `witness stats`, `witness export`)

```
internal/store/fsstore.GetRun + ReadEvents
        │
        ▼
internal/aggregate.Rebuild(run, events) → RunState
        │
        ├─► (inspect/stats) formatted text output → stdout
        └─► (export) internal/export.{JSON|NDJSON|Markdown}Exporter → io.Writer (stdout or file)
```

### 3.4 Replay Flow (`witness replay`)

```
internal/store/fsstore.GetRun + ReadEvents
        │
        ▼
internal/replay.NewController(run, events)
        │
        ├─► Play/Pause/Step controls (keyboard via TUI or CLI flags)
        │
        ▼
internal/replay.Controller.Updates() channel → StateMsg
        │
        ▼
internal/tui.App (replay mode) → panels → Terminal
```

### 3.5 Alert Evaluation Flow

```
internal/app.StoreSink.Append(evt)
        │
        ├─► internal/aggregate.Aggregator.Apply(evt)
        │           │
        │           └─► updated RunState
        │
        └─► internal/alerts.Engine.Evaluate(RunState)
                    │
                    ├─► StallRule, LoopRule, BudgetRule, RetryStormRule, FailureDensityRule
                    