# Witness Architecture Document

## 1. System Overview

Witness is a terminal-first observability and monitoring platform for AI-driven software development workflows, implemented as a single Go CLI binary. It captures structured, append-only telemetry events from coding agents, LLM API calls, CLI tools, Git repositories, test runners, and workflow orchestration systems, then aggregates that event stream into a normalized run-state model that is persisted locally and rendered in a live terminal dashboard. Operators use Witness to gain real-time visibility into what an autonomous coding workflow is doing, how much it is costing, whether it is making progress, and what artifacts it has changed — and to replay, inspect, and export historical runs for postmortem analysis.

---

## 2. Components

> **Note:** The machine-extracted fact model for this repository returned an empty component list (`"components": []`), indicating that static analysis did not resolve individual source files at extraction time. The component descriptions below are derived entirely from the authoritative `SPEC.md` and `PLAN.md` context documents. All claims are attributed to those sources.

### 2.1 `cmd/witness` — CLI Entry Point

**Source:** `cmd/witness/main.go`
**Inferred from:** `PLAN.md §1.8`, `SPEC.md §7`

- `PLAN.md §1.8` specifies this as the Cobra root command tree, initially exposing only `version` in Phase 1 and expanding to the full subcommand set across subsequent phases.
- `SPEC.md §7.1` defines the full command surface: `watch`, `run`, `attach`, `runs`, `inspect`, `replay`, `export`, `stats`, `doctor`, `config show`.
- Responsible for parsing CLI flags, loading configuration, and delegating to internal packages. Does not contain business logic.

### 2.2 `internal/app` — Application Orchestration

**Source:** `internal/app/app.go`
**Inferred from:** `PLAN.md §5.1`

- `PLAN.md §5.1` describes this package as the subprocess runner for `witness run -- <command>`.
- Responsibilities include: creating a new run in the store, registering signal handlers (SIGINT/SIGTERM with 10-second graceful shutdown deadline), starting observation goroutines (Git poller, file watcher, stdin ingestion), launching the child process in its own process group, relaying subprocess stdout/stderr to the terminal, and emitting `run.created`, `run.started`, `run.completed`/`run.failed` lifecycle events.
- Also contains `StoreSink` (`internal/app/storesink`), the bridge that validates, redacts, persists, aggregates, and optionally alert-evaluates each incoming event.

### 2.3 `internal/config` — Configuration Loading

**Source:** `internal/config/config.go`, `internal/config/defaults.go`, `internal/config/pricing.go`
**Inferred from:** `PLAN.md §1.7`, `SPEC.md §21`

- `PLAN.md §1.7` defines the `Config` struct with sub-domains: `StorageConfig`, `UIConfig`, `AlertsConfig`, `FilesConfig`, `PrivacyConfig`, `GitConfig`, `PricingConfig`, `CaptureConfig`.
- `SPEC.md §21.1` specifies the precedence order: CLI flags → environment variables (`WITNESS_*`) → config file (YAML) → defaults.
- `internal/config/defaults.go` holds compiled-in defaults: storage root `~/.witness`, UI refresh 500ms, stall threshold 10 minutes, loop window 8, max run cost $25.00, max stage cost $8.00, default file ignore patterns.
- `internal/config/pricing.go` holds the built-in model pricing table (Anthropic Claude family, OpenAI GPT-4/4o/o1/o3 family) and the `EstimateCost(provider, model string, inputTokens, outputTokens, cachedTokens int64) float64` function.

### 2.4 `internal/events` — Event Schema and Validation

**Source:** `internal/events/event.go`, `internal/events/types.go`, `internal/events/sink.go`, `internal/events/validate.go`
**Inferred from:** `PLAN.md §1.3–1.5`, `SPEC.md §10`

- `PLAN.md §1.3` defines the `Event` struct with required fields (`event_id`, `schema_version`, `timestamp`, `run_id`, `type`, `source`, `payload`) and optional fields (`stage_id`, `status`, `summary`, `trace_id`, `span_id`, `parent_event_id`, `tags`, `labels`). `payload` is typed as `json.RawMessage` to preserve unknown fields.
- `PLAN.md §1.4` defines the `EventSink` interface (`Append(ctx context.Context, evt Event) error`), the stable contract consumed by `internal/git`, `internal/files`, and `internal/ingest`.
- `PLAN.md §1.5` specifies `Validate(evt Event) error`, which checks all required fields and returns a structured multi-violation error.
- `internal/events/types.go` defines `EventType` as a string type with constants for all ~35 event types across run lifecycle, stage lifecycle, tool lifecycle, model lifecycle, Git/repo, test/validation, findings/review, system/health, and narrative/annotation categories.

### 2.5 `internal/models` — Domain Entities

**Source:** `internal/models/run.go`, `internal/models/stage.go`, `internal/models/tool.go`, `internal/models/model_request.go`, `internal/models/file_change.go`, `internal/models/commit.go`, `internal/models/alert.go`, `internal/models/enums.go`
**Inferred from:** `PLAN.md §1.2`, `SPEC.md §9`

- `SPEC.md §9.1` defines seven core entity types. `PLAN.md §1.2` maps these to Go structs:
  - `Run`: top-level workflow execution unit with `run_id`, `status` (`RunStatus` enum), `labels map[string]string`, timestamps, repo/branch/host metadata.
  - `Stage`: major workflow phase with `order`, `status` (`StageStatus` enum), `progress_percent`, `summary`.
  - `ToolInvocation`: CLI tool execution with `invocation_id`, `tool_name`, `exit_code`, `duration_ms`, `findings`, `metadata`.
  - `ModelRequest`: LLM API call with `request_id`, `provider`, `model`, token counts (`input_tokens`, `output_tokens`, `cached_tokens`, `reasoning_tokens`), `cost_usd`, `latency_ms`.
  - `FileChange`: file system event with `change_type` (`ChangeType` enum: `created`, `modified`, `deleted`, `renamed`), `line_delta_add`, `line_delta_remove`, `content_hash`.
  - `Commit`: Git commit with `sha`, `message`, `files_changed`, `insertions`, `deletions`.
  - `Alert`: anomaly signal with `severity` (`Severity` enum: `info`, `warning`, `error`, `critical`), `type`, `related_ids`, `acknowledged`.
- All enum types implement `String()`, `MarshalJSON()`, and `UnmarshalJSON()`.

### 2.6 `internal/store` / `internal/store/fsstore` — Event Persistence

**Source:** `internal/store/` (interface), `internal/store/fsstore/` (implementation)
**Inferred from:** `PLAN.md §2`, `SPEC.md §12`

- `PLAN.md §2.1` defines the `Store` interface covering run CRUD, event append/read/stream, and snapshot save/load.
- `PLAN.md §2.2` specifies the filesystem implementation using the layout `~/.witness/runs/<run-id>/` with three files:
  - `run.json`: run metadata, written atomically (write-to-temp + rename).
  - `events.ndjson`: append-only newline-delimited JSON event log, opened with `O_APPEND|O_WRONLY|O_CREATE`, synced after each write.
  - `snapshot.json`: optional aggregated state snapshot, written atomically.
- `PLAN.md §2.3` specifies `StreamEvents` as a buffered channel (capacity 256) that first replays existing events then tails the file via `fsnotify`. Slow consumers receive dropped events (logged warning) rather than blocking the append path.
- `PLAN.md §2.2` specifies crash tolerance: partial final NDJSON lines are discarded on recovery; snapshot writes use atomic rename.
- Event deduplication uses an in-memory ring buffer of the last 256 event IDs.

### 2.7 `internal/aggregate` — State Aggregation and Metrics

**Source:** `internal/aggregate/`
**Inferred from:** `PLAN.md §3`, `SPEC.md §11`, `SPEC.md §14`

- `PLAN.md §3.1` defines `RunState`, the derived state model holding: active run/stage/tool/model, token and cost totals broken down by provider/model/stage, tool invocation history, Git/file state, alert set, a 200-event ring buffer of recent events, and timing fields (`LastEventAt`, `LastFileChangeAt`, `LastCommitAt`, `LastStageChangeAt`).
- `PLAN.md §3.2` defines `Aggregator` with `Apply(evt events.Event) error` (dispatches on event type to handler methods) and `Snapshot() RunState` (returns a copy under read lock). Thread-safe via `sync.RWMutex`.
- `PLAN.md §3.3` defines derived metric methods on `RunState`: `Duration()`, `StageDurations()`, `TokenBurnRate(window)`, `CostBurnRate(window)`, `AvgToolLatency()`, `AvgModelLatency()`, `MeanTimeBetweenCommits()`, `UniqueFilesTouched()`.
- `PLAN.md §3.5` defines `Rebuild(run models.Run, events []events.Event) (*RunState, error)` for reconstructing state from the full event log — used for historical runs, crash recovery, and snapshot verification.

### 2.8 `internal/alerts` — Anomaly Detection

**Source:** `internal/alerts/`
**Inferred from:** `PLAN.md §8`, `SPEC.md §15`

- `PLAN.md §8.1` defines the `Rule` interface and `Engine` struct. The engine runs all registered rules, deduplicates against previously raised alerts, and returns only new alerts.
- `PLAN.md §8.2–8.6` specifies five initial rules:
  - **Stall Detection**: fires when no file changes or stage transitions have occurred within `cfg.StallDuration` while the run is active.
  - **Loop Detection**: fires when the same tool or model+purpose combination repeats ≥ 75% of the last `cfg.LoopWindow` invocations with no intervening stage transitions.
  - **Budget Threshold**: fires when total run cost exceeds `cfg.MaxRunCostUSD`, per-stage cost exceeds `cfg.MaxStageCostUSD`, or total tokens exceed `cfg.MaxTokens`.
  - **Retry Storm**: fires when ≥ 5 of the last 10 tool invocations for the same tool name have `status=failed`.
  - **Failure Density**: fires when ≥ 3 failure events occur within a 60-second window.
- `PLAN.md §8.7` specifies integration via the `AlertHook` interface on `StoreSink`; the engine is wired in at `witness run` construction time.

### 2.9 `internal/git` — Git Repository Observer

**Source:** `internal/git/`
**Inferred from:** `PLAN.md §5.2`, `SPEC.md §16`

- `PLAN.md §5.2` defines `Observer`, which polls Git state at a configurable interval (default 5 seconds) by executing `git` subprocesses with timeouts.
- On each poll: detects branch changes (emits `git.branch.changed`), detects dirty file count changes (emits `repo.status.changed`), detects new commits via `git log <last-sha>..HEAD` (emits `git.commit.created` with stats from `git show --stat`).
- Helper functions: `DetectRepoRoot`, `CurrentBranch`, `ParseCommit`.
- `SPEC.md §16.3` constrains this package to read-only Git observation; it must never mutate repository state.

### 2.10 `internal/files` — File System Watcher

**Source:** `internal/files/`
**Inferred from:** `PLAN.md §5.3`, `SPEC.md §17`

- `PLAN.md §5.3` defines `Watcher`, which uses `fsnotify` to watch the repo root recursively.
- Classifies events as `file.created`, `file.modified`, or `file.deleted` and emits them to the `EventSink`.
- Applies ignore pattern matching using `github.com/bmatcuk/doublestar/v4` for `**` glob support.
- Default ignores: `.git/`, `node_modules/`, `vendor/`, `dist/`, `build/`, `.next/`, editor swap files (`.swp`, `.swo`, `*~`), `.DS_Store`.
- Debounces events for the same path within 100ms to suppress editor save-sequence duplicates.

### 2.11 `internal/ingest` — External Event Ingestion

**Source:** `internal/ingest/`
**Inferred from:** `PLAN.md §5.4`, `PLAN.md §10.1–10.2`, `SPEC.md §18`

- `PLAN.md §5.4` defines `Scanner`, which reads lines from subprocess stdout and attempts to parse each as: (1) a native Witness `Event`, (2) a structured `ToolResult` (SPEC §18.4 contract), or (3) raw output passed through.
- `PLAN.md §10.1` defines `ToolResult` and `ToolResultToEvents`, which converts a structured result into `tool.completed`, optionally `model.request.completed`, and `finding.recorded` events.
- `PLAN.md §10.2` defines the `ToolAdapter` interface and an adapter registry. The Generic JSON adapter handles the standard `ToolResult` schema; tool-specific adapters (Prism, SpecCritic/PlanCritic) are best-effort and may be deferred.

### 2.12 `internal/privacy` — Redaction

**Source:** `internal/privacy/redactor.go`
**Inferred from:** `PLAN.md §5.5`, `SPEC.md §20`

- `PLAN.md §5.5` defines `Redactor`, which compiles configured regex patterns and applies them to event summaries, tool output, and all string values within event payloads before persistence.
- Default patterns cover: API keys (`sk-*`, `AKIA*`), bearer tokens, and generic `password`/`secret`/`apikey` assignments.
- `SPEC.md §20.1` constrains the redactor to never apply to event IDs, timestamps, or structural field names.

### 2.13 `internal/replay` — Replay Engine

**Source:** `internal/replay/`
**Inferred from:** `PLAN.md §6`, `SPEC.md §23`

- `PLAN.md §6.1` defines `Controller`, which holds the full event slice for a historical run and an `Aggregator` for state reconstruction.
- Supports: `Play`/`Pause`, `SetSpeed`, `StepForward`/`StepBackward`, `JumpToIndex`/`JumpToTime`, and semantic navigation (`JumpToNextStageTransition`, `JumpToNextAlert`, `JumpToNextCommit`).
- `StepBackward` and backward `JumpToIndex` rebuild state from event 0 to the target index (O(n), acceptable for v1 volumes).
- `Play` runs in a goroutine, advancing events with delays proportional to actual timestamp gaps scaled by `speed`, sending `RunState` updates via a non-blocking buffered channel.
- `PLAN.md §6.2` defines `GeneratePostmortem(state RunState) string` for concise textual run summaries.

### 2.14 `internal/tui` — Terminal UI Shell

**Source:** `internal/tui/`
**Inferred from:** `PLAN.md §7.1–7.7`, `SPEC