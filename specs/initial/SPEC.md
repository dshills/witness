
# SPEC.md

## Project: Witness

## Tagline

Witness is a terminal-first observability and monitoring platform for AI-driven software development workflows. It captures structured events from coding agents, LLM calls, CLI tools, repositories, tests, and workflow orchestration systems, then presents that information in a live terminal dashboard and durable run history.

Witness is not a code generator, not a planner, and not a reviewer. It is the system of record and live operational cockpit for AI software delivery.

---

## 1. Purpose

Modern AI coding workflows are noisy, expensive, and opaque. An operator can often see that “something is happening,” but not:

* what stage the workflow is currently in
* what tools have been called
* how many tokens have been consumed
* how much money has been burned
* what files changed
* whether the agent is making real progress
* whether the workflow is looping, stalled, or drifting away from intent
* when commits occurred
* which model or tool is responsible for a failure

Witness exists to solve that problem.

Witness provides:

* a structured event ingestion layer for AI workflow telemetry
* a normalized run/session model
* a durable event log and aggregated run state
* a terminal UI for live monitoring
* a replay and postmortem mode for previous runs
* machine-readable exports for downstream reporting and analytics
* a set of heuristics for detecting stalls, loops, drift, and cost anomalies

Witness should feel like a cross between:

* `top` / `htop`
* a CI pipeline monitor
* a distributed trace viewer
* a Git activity monitor
* an LLM token and cost dashboard
* a live debugger for autonomous coding workflows

---

## 2. Goals

### 2.1 Primary Goals

1. Provide a real-time, terminal-native view of an AI coding workflow.
2. Capture structured telemetry for tools, models, Git activity, tests, and orchestration phases.
3. Allow operators to quickly determine whether a workflow is progressing, blocked, looping, or failing.
4. Record durable event streams and summarized state for replay and postmortem analysis.
5. Support integration with the user’s ecosystem of AI development CLI tools, including SpecCritic, PlanCritic, RealityCheck, Prism, Clarion, and future tools.
6. Be usable as both a standalone monitor and a reusable telemetry/event substrate.
7. Be implemented in Go as a dependency-light CLI application.

### 2.2 Secondary Goals

1. Provide a clean contract for external tools to emit structured telemetry.
2. Support both local interactive workflows and non-interactive CI/agent runs.
3. Enable optional future expansion to remote dashboards, multi-run views, and web UIs.
4. Make autonomous workflow behavior more trustworthy through visibility and receipts.

---

## 3. Non-Goals

Witness must not attempt to become all things to all goblins.

### 3.1 Explicit Non-Goals

1. Witness is not an LLM orchestration engine.
2. Witness is not itself a coding agent.
3. Witness is not a replacement for SpecCritic, PlanCritic, Prism, RealityCheck, Clarion, or Verifier.
4. Witness is not a source code editor.
5. Witness is not a Git client replacement.
6. Witness is not a full distributed tracing backend in the first release.
7. Witness is not a web app in the initial version.
8. Witness is not responsible for storing chain-of-thought or private model reasoning.
9. Witness must not attempt to capture secret prompt content unless explicitly configured and properly redacted.
10. Witness must not depend on a database server for the initial local version.

---

## 4. Target Users

### 4.1 Primary Users

* software engineers using AI coding agents
* open source maintainers running AI-assisted workflows
* CTOs / principal engineers supervising autonomous code generation
* operators of agentic development pipelines
* consultants using AI workflows to accelerate architecture or implementation work

### 4.2 Secondary Users

* teams running internal AI coding platforms
* DevOps / platform engineers integrating AI workflows into CI
* reviewers performing postmortems on failed AI-assisted runs

---

## 5. Core Use Cases

### 5.1 Live Run Monitoring

An engineer launches an AI workflow and opens Witness to see:

* current workflow stage
* active tool or model call
* files being modified
* commits being created
* tests passing/failing
* token usage and cost growth
* warnings for loops or stalled progress

### 5.2 Replay / Postmortem

After a workflow completes or fails, the engineer replays the run to understand:

* what happened in what order
* where time was spent
* which tools failed
* whether costs ballooned unexpectedly
* what code changes occurred before failure

### 5.3 Toolchain Integration

A workflow consisting of SpecCritic → PlanCritic → coding agent → RealityCheck → Prism → Clarion emits events to Witness, allowing a single coherent operational view.

### 5.4 CI/CD Integration

A non-interactive run emits events in CI. Witness stores the run and later produces summaries or exports.

### 5.5 Loop / Stall Detection

Witness detects that an agent has made repeated model calls and tool invocations over several minutes without creating meaningful file changes or moving stages.

### 5.6 Budget Monitoring

Witness shows total tokens and estimated cost by model/provider and warns when thresholds are exceeded.

---

## 6. Product Overview

Witness consists of two conceptual layers:

### 6.1 Witness Core

The core telemetry/event subsystem responsible for:

* event schema definitions
* event ingestion
* event validation
* state aggregation
* persistence
* exports
* alert heuristics

### 6.2 Witness TUI

The terminal UI responsible for:

* live rendering of run state
* event stream display
* stage progress visualization
* token/cost panels
* Git activity visualization
* alerts and drill-downs
* keyboard-driven navigation

The architecture must allow the TUI to be one consumer of the core, not the core itself.

---

## 7. Command Line Interface

Witness must be a terminal-first CLI.

### 7.1 Primary Commands

```bash
witness watch
witness watch --run latest
witness watch --run <run-id>
witness run -- <command>
witness attach --run <run-id>
witness runs
witness inspect <run-id>
witness replay <run-id>
witness export <run-id> --format json
witness stats <run-id>
witness doctor
witness config show
```

### 7.2 Command Responsibilities

#### `witness run -- <command>`

Runs a child command under Witness instrumentation when possible.

Responsibilities:

* create a new run/session
* record workflow start/stop events
* collect subprocess timing/exit data
* optionally watch the repo for file changes and Git activity
* optionally ingest structured JSON events emitted by child tools
* optionally auto-open the TUI when interactive

Example:

```bash
witness run -- my-agent build
```

#### `witness watch`

Open the live TUI for the current or specified run.

Examples:

```bash
witness watch
witness watch --run latest
witness watch --repo .
```

#### `witness attach --run <run-id>`

Attach to an existing active run and open the TUI.

#### `witness runs`

List known runs with summary info:

* run id
* repo
* start time
* duration
* status
* stages completed
* cost
* commits
* alerts

#### `witness inspect <run-id>`

Print a detailed textual summary of a run without the live TUI.

#### `witness replay <run-id>`

Replay a previous run in time order through the TUI or a textual timeline view.

#### `witness export <run-id> --format <format>`

Export run data in machine-readable format.

Supported initial formats:

* json
* ndjson
* markdown

#### `witness stats <run-id>`

Print aggregated metrics:

* duration by phase
* tokens by provider/model
* tools used
* findings count by severity
* files changed
* commits
* alert counts

#### `witness doctor`

Validate local setup:

* writable storage paths
* Git availability
* terminal compatibility
* config sanity
* optional tool integrations

#### `witness config show`

Display effective config values.

---

## 8. Execution Modes

Witness must support multiple operating modes.

### 8.1 Mode: Live Interactive

A user is present in the terminal and wants to watch a run in real time.

### 8.2 Mode: Headless Capture

A workflow emits events without an active TUI. Witness records events to disk for later viewing.

### 8.3 Mode: Replay

Witness rehydrates a historical run from persisted events and allows the user to inspect progression.

### 8.4 Mode: Passive Repository Observation

Witness observes repository changes, commits, and optionally tool output even if not every tool is instrumented.

### 8.5 Mode: External Event Ingestion

External tools write events to stdout/stderr or to an event file/socket/pipe using Witness JSON schema.

---

## 9. Data Model

Witness must distinguish between raw events and aggregated state.

### 9.1 Core Entities

#### Run

A run is the top-level unit of workflow execution.

Fields:

* `run_id` string
* `name` optional string
* `repo_root` optional string
* `branch` optional string
* `started_at` timestamp
* `ended_at` optional timestamp
* `status` enum
* `entrypoint` optional string
* `command` optional string array
* `working_dir` optional string
* `host` optional string
* `user` optional string
* `workflow_type` optional string
* `labels` map[string]string

Status enum:

* `pending`
* `running`
* `completed`
* `failed`
* `cancelled`
* `stalled`
* `unknown`

#### Stage

A stage is a major workflow phase.

Fields:

* `stage_id` string
* `run_id` string
* `name` string
* `order` integer
* `status` enum
* `started_at` optional timestamp
* `ended_at` optional timestamp
* `progress_percent` optional number
* `summary` optional string

Examples:

* Load Spec
* Critique Spec
* Build Plan
* Critique Plan
* Implement
* Verify
* Review
* Document
* Test
* Package

#### Tool Invocation

Represents the execution of a CLI tool or sub-tool.

Fields:

* `invocation_id` string
* `run_id` string
* `stage_id` optional string
* `tool_name` string
* `command` optional string array
* `started_at` timestamp
* `ended_at` optional timestamp
* `status` enum
* `exit_code` optional integer
* `duration_ms` optional integer
* `summary` optional string
* `findings` optional object
* `metadata` object

#### Model Request

Represents an LLM/API request.

Fields:

* `request_id` string
* `run_id` string
* `stage_id` optional string
* `provider` string
* `model` string
* `started_at` timestamp
* `ended_at` optional timestamp
* `status` enum
* `input_tokens` optional integer
* `output_tokens` optional integer
* `cached_tokens` optional integer
* `reasoning_tokens` optional integer if provider supports it
* `cost_usd` optional decimal
* `latency_ms` optional integer
* `purpose` optional string
* `tool_name` optional string
* `metadata` object

#### File Change

Represents a meaningful file system change relevant to the run.

Fields:

* `change_id` string
* `run_id` string
* `path` string
* `change_type` enum
* `timestamp` timestamp
* `size_before` optional integer
* `size_after` optional integer
* `line_delta_add` optional integer
* `line_delta_remove` optional integer
* `content_hash` optional string

Change type enum:

* `created`
* `modified`
* `deleted`
* `renamed`

#### Commit

Represents a Git commit created during the run.

Fields:

* `commit_id` string
* `run_id` string
* `sha` string
* `timestamp` timestamp
* `message` string
* `author_name` optional string
* `author_email` optional string
* `files_changed` optional integer
* `insertions` optional integer
* `deletions` optional integer

#### Alert

Represents a health or anomaly signal.

Fields:

* `alert_id` string
* `run_id` string
* `timestamp` timestamp
* `severity` enum
* `type` string
* `title` string
* `description` string
* `related_ids` []string
* `acknowledged` bool
* `metadata` object

Severity enum:

* `info`
* `warning`
* `error`
* `critical`

---

## 10. Event Model

Witness must use an append-only event stream as the source of truth.

### 10.1 Event Design Principles

1. Events are immutable.
2. Events must be timestamped.
3. Events must be self-describing enough to be useful independently.
4. Events must include a `run_id`.
5. Event payloads must be structured JSON-serializable objects.
6. Event schemas must be versioned.
7. Unknown fields should be preserved where feasible.
8. State is derived from events, not the other way around.

### 10.2 Base Event Envelope

```json
{
  "event_id": "evt_01J...",
  "schema_version": "1.0",
  "timestamp": "2026-03-11T14:23:19.123Z",
  "run_id": "run_01J...",
  "stage_id": "stage_implement",
  "type": "tool.completed",
  "source": "prism",
  "status": "success",
  "summary": "Prism review completed with 2 warnings",
  "payload": {
    "tool_name": "prism",
    "issues_found": 2,
    "severity_counts": {
      "warning": 2
    }
  }
}
```

### 10.3 Required Event Envelope Fields

* `event_id`
* `schema_version`
* `timestamp`
* `run_id`
* `type`
* `source`
* `payload`

### 10.4 Optional Event Envelope Fields

* `stage_id`
* `status`
* `summary`
* `trace_id`
* `span_id`
* `parent_event_id`
* `tags`
* `labels`

### 10.5 Initial Event Types

#### Run Lifecycle

* `run.created`
* `run.started`
* `run.completed`
* `run.failed`
* `run.cancelled`
* `run.stalled`

#### Stage Lifecycle

* `stage.created`
* `stage.started`
* `stage.progress`
* `stage.completed`
* `stage.failed`
* `stage.skipped`

#### Tool Lifecycle

* `tool.started`
* `tool.output`
* `tool.completed`
* `tool.failed`

#### Model Lifecycle

* `model.request.started`
* `model.request.completed`
* `model.request.failed`

#### Git / Repository

* `repo.status.changed`
* `file.created`
* `file.modified`
* `file.deleted`
* `git.commit.created`
* `git.branch.changed`

#### Test / Validation

* `test.started`
* `test.completed`
* `test.failed`
* `validation.warning`
* `validation.error`

#### Findings / Review

* `finding.recorded`
* `review.completed`
* `drift.detected`

#### System / Health

* `alert.raised`
* `alert.cleared`
* `budget.threshold.exceeded`
* `loop.detected`
* `stall.detected`

#### Generic Narrative / Annotation

* `note.recorded`
* `summary.updated`

### 10.6 Event Ordering

Witness must not assume perfect event ordering across all sources, but must:

* preserve ingestion order
* retain event timestamps
* sort by timestamp for replay where appropriate
* tolerate late-arriving events

### 10.7 Event Idempotency

Witness should support deduplication where an event carries a stable ID.

---

## 11. State Aggregation

Witness must maintain a derived state model for efficient live rendering.

### 11.1 Aggregation Responsibilities

* track current run status
* determine active stage
* determine active tool and active model request
* maintain token and cost totals
* maintain Git summary and file activity summary
* maintain alert set
* maintain recent event stream buffer
* maintain duration calculations
* maintain tool usage counts
* maintain stage progression

### 11.2 Aggregated Views

#### Run Summary View

* run metadata
* total duration
* overall status
* stage completion count
* total tokens
* total cost
* commits count
* alerts count

#### Active Work View

* current stage
* current step/summary
* active tool
* active model request
* elapsed time in current stage

#### Token / Cost View

* total input tokens
* total output tokens
* by provider
* by model
* by stage
* by tool if available

#### Git Activity View

* current branch
* dirty/clean status
* files modified/added/deleted
* recent commits
* hot files by update frequency

#### Health View

* current alerts
* loop suspicion score
* stall suspicion score
* retry counts
* failure counts
* budget threshold status

### 11.3 Consistency Rules

* derived state must be reconstructable from the event log
* the event log is authoritative
* store corruption or aggregator bugs must never mutate historical events

---

## 12. Persistence

Witness must persist data locally in the initial version.

### 12.1 Initial Storage Requirements

* append-only event log per run
* summarized run metadata index
* optional snapshots for faster startup
* no external database dependency required

### 12.2 Suggested Local Layout

```text
~/.witness/
  config.yaml
  runs/
    <run-id>/
      run.json
      events.ndjson
      snapshot.json
      export/
```

### 12.3 File Responsibilities

#### `run.json`

Basic run metadata and summary.

#### `events.ndjson`

Append-only newline-delimited JSON event stream.

#### `snapshot.json`

Optional aggregated state snapshot for quick rehydration.

### 12.4 Durability Rules

* event append must be crash-tolerant to a reasonable local standard
* partial final lines in ndjson must be tolerated on recovery
* snapshots must be replaceable atomically

### 12.5 Future Persistence Options

The design should allow future storage backends:

* sqlite
* embedded key-value store
* remote collector backend

But these are not required in v1.

---

## 13. TUI Requirements

Witness must provide a rich terminal UI.

### 13.1 TUI Principles

1. Must be useful first, pretty second.
2. Must work in standard modern terminals.
3. Must support keyboard-only navigation.
4. Must handle resize events.
5. Must degrade gracefully on narrow terminals.
6. Must make important problems obvious without requiring spelunking.

### 13.2 Initial Main Layout

The main live view should include the following panels:

1. Header / Run Status
2. Stage Progress
3. Active Tool / Model
4. Token & Cost Summary
5. Git / File Activity
6. Alerts
7. Event Stream

### 13.3 Header Panel

Displays:

* workflow/run name
* run id
* repo path
* branch
* uptime/duration
* overall status

### 13.4 Stage Progress Panel

Displays:

* list of stages in order
* completion state icons
* current stage
* progress percentage if available
* current step summary

### 13.5 Active Tool / Model Panel

Displays:

* active tool name
* active command summary
* active model provider/model
* request count
* latency/duration
* current summary text
* retry count

### 13.6 Token & Cost Panel

Displays:

* total input/output tokens
* total estimated cost
* cost by provider/model
* token burn rate over recent window
* warning if thresholds exceeded

### 13.7 Git / File Activity Panel

Displays:

* modified/added/deleted counts
* last commit sha/message (shortened)
* recent files touched
* hot files by frequency
* dirty state

### 13.8 Alerts Panel

Displays active alerts with severity markers.

Examples:

* repeated failure in stage
* no file changes for 10 minutes
* token burn high relative to progress
* same tool invoked repeatedly without stage movement
* test failures accumulating

### 13.9 Event Stream Panel

Displays a scrollable time-ordered feed of recent events.

Must support:

* scrolling
* pause/freeze
* filter by type/severity/source
* compact and verbose modes

### 13.10 Drill-Down Views

The TUI should support switching to focused views for:

* stage detail
* tool history
* model request history
* Git activity / commit list
* alerts detail
* metrics view
* replay timeline

### 13.11 Keyboard Shortcuts

Initial shortcuts should include something like:

* `q` quit
* `tab` next panel
* `shift+tab` previous panel
* `j/k` move selection
* `/` filter
* `p` pause event stream
* `r` refresh / resume live tail
* `g` Git detail view
* `t` token view
* `a` alerts view
* `e` event stream focus
* `s` stage view
* `m` model view
* `?` help

Exact bindings may evolve, but keyboard control is mandatory.

### 13.12 Terminal Constraints

Initial minimum supported terminal size should be approximately:

* 100 columns x 28 rows for full view

Narrower terminals should fall back to simplified stacked layouts.

---

## 14. Metrics and Analytics

Witness must compute meaningful operational metrics.

### 14.1 Required Metrics

* total run duration
* duration per stage
* duration per tool
* tool invocation counts
* model request counts
* input/output tokens by provider/model
* estimated cost by provider/model
* files touched count
* unique files touched count
* commits count
* failures count
* alerts count
* retries count

### 14.2 Derived Metrics

* token burn rate per minute
* cost burn rate per minute
* average tool latency
* average model latency
* mean time between commits
* stage throughput
* findings per tool invocation
* progress-to-cost ratio

### 14.3 Heuristic Metrics

* stall score
* loop score
* drift suspicion score
* productivity score (experimental)

Witness must treat these heuristic scores as guidance, not truth handed down from the robot mountain.

---

## 15. Alerts and Anomaly Detection

Witness must provide a minimal but useful anomaly engine.

### 15.1 Initial Alert Types

#### Stall Detection

Trigger when:

* run remains active
* no meaningful file changes or stage transitions for configurable duration
* tools/model calls continue or run remains idle unexpectedly

#### Loop Detection

Trigger when:

* repeated invocation of same tool/model/pattern
* same stage persists across repeated operations
* low evidence of progress between attempts

#### Budget Threshold Exceeded

Trigger when:

* run cost exceeds configured threshold
* stage cost exceeds configured threshold
* token usage exceeds configured threshold

#### Retry Storm

Trigger when:

* same tool or request repeatedly fails within a window

#### Failure Density

Trigger when:

* multiple errors or test failures occur in short succession

#### Drift Detection

Trigger when:

* workflow intent/progress metadata claims progress inconsistent with artifact changes
* planned milestone not evidenced by files/tests/commits after configurable interval

### 15.2 Alert Rules Engine

Initial rules may be hard-coded plus configurable thresholds.
A more generic user-defined rules system is future scope.

### 15.3 Alert Behavior

Alerts should:

* appear in the TUI
* be persisted as events
* be exportable
* include enough explanation to be actionable

---

## 16. Git Integration

Witness must understand repository activity.

### 16.1 Required Git Capabilities

* detect repo root
* read current branch
* detect dirty/clean working tree
* detect changed files
* detect commits created during run
* summarize commit metadata

### 16.2 Optional Git Enhancements

* line diff counts
* file churn ranking
* branch change events
* stash detection

### 16.3 Git Constraints

Witness must not rewrite history or mutate Git state except where explicitly running a child command that itself does so. Witness is an observer, not a chaos goblin with a chainsaw.

---

## 17. File System Observation

Witness should monitor file activity relevant to the run.

### 17.1 Required Behavior

* observe file changes under repo root or configured paths
* classify create/modify/delete
* ignore noisy irrelevant paths by default

### 17.2 Default Ignore Candidates

* `.git/`
* `node_modules/`
* `vendor/`
* `dist/`
* `build/`
* `.next/`
* temporary editor swap files

### 17.3 Configuration

Users must be able to override include/exclude patterns.

---

## 18. External Tool Integration Contract

Witness must integrate with existing and future CLI tools.

### 18.1 Integration Philosophy

The cleanest integration is structured event emission.

Tools such as:

* SpecCritic
* PlanCritic
* RealityCheck
* Prism
* Clarion
* Verifier

should ideally support a JSON output mode or direct Witness event emission mode.

### 18.2 Supported Integration Levels

#### Level 0: Passive Observation

Witness only sees subprocess start/end, stdout/stderr, Git changes, and file changes.

#### Level 1: Structured Summary Output

Tool emits machine-readable JSON summary that Witness parses.

#### Level 2: Full Event Emission

Tool emits detailed Witness-compatible events.

### 18.3 Tool Output Expectations

For structured summary mode, tools should emit fields like:

* tool name
* start time
* end time
* status
* summary
* findings counts by severity
* artifacts read
* artifacts written
* model/provider info if applicable
* token usage if applicable

### 18.4 Example Structured Tool Result

```json
{
  "tool": "speccritic",
  "status": "success",
  "summary": "Specification critique complete",
  "findings": {
    "critical": 0,
    "error": 1,
    "warning": 3,
    "info": 4
  },
  "artifacts": {
    "read": ["SPEC.md"],
    "written": ["SPEC_REPORT.md"]
  },
  "duration_ms": 1820
}
```

### 18.5 SDK / Helper Package

Witness should eventually provide a small Go helper package for emitting events cleanly from Go CLIs.
This helper package is desirable but not mandatory in v1.

---

## 19. Model Telemetry

Witness must support telemetry for LLM requests.

### 19.1 Required Model Metadata

* provider
* model
* start/end times
* success/failure
* input tokens
* output tokens
* cached tokens if available
* estimated cost
* purpose/stage if available

### 19.2 Nice-to-Have Metadata

* request label
* temperature
* tool choice mode
* prompt template name
* cache hit info
* streaming duration

### 19.3 Privacy Constraints

Witness must not require storing raw prompts or raw responses. Those may be optionally captured in redacted form, but the default should lean conservative.

---

## 20. Privacy and Secret Handling

This matters. Terminal glamour is not an excuse to leak secrets like a haunted colander.

### 20.1 Required Safety Behavior

* do not store secrets intentionally
* redact known sensitive patterns where feasible
* avoid persisting raw API keys, tokens, cookies, or Authorization headers
* avoid storing full prompt contents by default
* avoid storing environment variables by default

### 20.2 Configurable Redaction

Witness should support configurable redaction patterns.

### 20.3 Example Redaction Targets

* OpenAI / Anthropic API keys
* AWS access keys
* bearer tokens
* JWTs
* private URLs with embedded credentials

---

## 21. Configuration

Witness must be configurable, but not a labyrinthine shrine to configuration files.

### 21.1 Config Sources

Precedence order:

1. CLI flags
2. environment variables
3. config file
4. defaults

### 21.2 Config Domains

* storage paths
* default repo detection behavior
* event retention
* file ignore/include patterns
* Git polling/watch settings
* token/cost thresholds
* alert thresholds
* UI refresh intervals
* theme preferences
* privacy/redaction rules

### 21.3 Example Config File

```yaml
storage:
  root: ~/.witness
ui:
  refresh_ms: 500
  theme: auto
alerts:
  stall_seconds: 600
  loop_window: 8
  max_run_cost_usd: 25.00
  max_stage_cost_usd: 8.00
files:
  ignore:
    - .git/**
    - node_modules/**
    - dist/**
privacy:
  redact_patterns:
    - '(?i)authorization:\s*bearer\s+[A-Za-z0-9\-\._~\+\/=]+'
```

---

## 22. Export Formats

Witness must support exporting run data.

### 22.1 JSON Export

Complete structured export for machine consumption.

### 22.2 NDJSON Export

Raw event stream export.

### 22.3 Markdown Export

Human-readable run summary for issues, docs, or postmortems.

### 22.4 Future Formats

Future support may include:

* HTML
* CSV summaries
* OpenTelemetry-compatible mappings

---

## 23. Replay and Postmortem

Replay is a major feature, not an afterthought.

### 23.1 Replay Capabilities

* load historical event log
* step through time in order
* show stage evolution
* show alerts as they appeared
* show token/cost growth over time
* show commits/files as they appeared

### 23.2 Replay Controls

* play/pause
* step forward/backward by event
* jump to stage transitions
* jump to alerts
* jump to commits
* speed adjustment

### 23.3 Postmortem Summary

Witness should be able to generate a concise textual summary of:

* what the run attempted
* where it spent time
* what failed or stalled
* cost summary
* final artifact/Git summary
* notable alerts

---

## 24. Performance Requirements

### 24.1 Startup

* opening a recent active run should feel fast
* historical run loading should be acceptable for thousands of events

### 24.2 Event Ingestion

* handle at least hundreds of events per minute comfortably in local workflows
* maintain UI responsiveness during bursts

### 24.3 Resource Usage

Witness should remain lightweight enough for normal developer machines.

### 24.4 Scalability Expectations for v1

v1 is optimized for local and small-team runs, not huge fleet-scale telemetry. Do not prematurely summon the distributed systems demon.

---

## 25. Reliability Requirements

### 25.1 Crash Handling

* partial event logs must be recoverable
* TUI crash must not destroy run data
* failed event parsing should not abort the whole run if possible

### 25.2 Error Isolation

* a malformed tool output event should produce a warning, not total collapse
* Git observation failure should degrade gracefully
* file watch failure should degrade gracefully

### 25.3 Recovery

Witness should be able to reopen incomplete or active runs.

---

## 26. Implementation Architecture

### 26.1 Language and Packaging

* implementation language: Go
* keep dependency footprint reasonable
* prefer standard library where sensible
* use mature terminal UI library only if it clearly improves velocity and maintainability

### 26.2 Suggested Package Layout

```text
cmd/witness/
internal/app/
internal/config/
internal/events/
internal/ingest/
internal/store/
internal/aggregate/
internal/alerts/
internal/git/
internal/files/
internal/models/
internal/export/
internal/replay/
internal/tui/
internal/ui/panels/
internal/ui/state/
internal/version/
```

### 26.3 Package Responsibilities

#### `internal/events`

* event types
* envelope definitions
* validation
* schema versioning

#### `internal/ingest`

* parse external tool output
* receive/stdin/file/pipe ingestion
* normalize events

#### `internal/store`

* event persistence
* run metadata persistence
* snapshots

#### `internal/aggregate`

* derive live state from events
* calculate metrics

#### `internal/alerts`

* alert heuristics
* threshold evaluation

#### `internal/git`

* repo detection
* status polling/watching
* commit summarization

#### `internal/files`

* filesystem watch
* include/exclude handling

#### `internal/replay`

* replay state stepping
* timeline iteration

#### `internal/tui`

* terminal app shell
* navigation
* layouts
* panel orchestration

---

## 27. Suggested Internal Interfaces

These are illustrative, not mandatory exact signatures.

### 27.1 Event Sink

```go
type EventSink interface {
    Append(ctx context.Context, evt events.Event) error
}
```

### 27.2 Event Source

```go
type EventSource interface {
    Subscribe(ctx context.Context, runID string) (<-chan events.Event, error)
}
```

### 27.3 Aggregator

```go
type Aggregator interface {
    Apply(evt events.Event) error
    Snapshot() aggregate.RunState
}
```

### 27.4 Exporter

```go
type Exporter interface {
    Export(ctx context.Context, runID string, w io.Writer) error
}
```

### 27.5 Alert Evaluator

```go
type AlertEvaluator interface {
    Evaluate(state aggregate.RunState) []models.Alert
}
```

---

## 28. Status and Progress Semantics

Witness must avoid fake precision. Progress bars full of lies are the decorative gourds of developer tooling.

### 28.1 Progress Sources

Progress may come from:

* explicit stage progress events
* known workflow stage counts
* heuristics based on milestones

### 28.2 Rules

* if progress is heuristic, label it as such where practical
* do not fabricate percentage progress without basis
* allow unknown progress

---

## 29. Workflow Semantics

Witness should understand staged workflows but must not require a rigid one.

### 29.1 Default Stage Model

For AI coding workflows, a default stage list may include:

* intake
* spec
* spec critique
* planning
* plan critique
* implementation
* verification
* review
* testing
* documentation
* packaging

### 29.2 Custom Workflows

Users/tools must be able to define custom stage names and order.

---

## 30. Human Factors

Witness should improve trust, not create a louder casino.

### 30.1 UX Goals

* make the current state obvious
* surface anomalies early
* reward quick situational understanding
* keep noisy details accessible but not dominant

### 30.2 Information Hierarchy

Priority order:

1. current status / health
2. stage and progress
3. active work
4. cost/tokens
5. Git/file changes
6. recent event stream
7. deep metrics/history

---

## 31. Testing Requirements

Witness must be well tested because observability tools that lie are cursed objects.

### 31.1 Required Test Areas

* event validation
* event append and recovery
* state aggregation correctness
* alert heuristic behavior
* Git integration parsing
* config loading precedence
* export correctness
* replay correctness
* TUI state transitions where testable

### 31.2 Test Types

* unit tests
* golden tests for exports
* replay reconstruction tests
* fixture-based event stream tests
* integration tests for subprocess observation

### 31.3 Failure Cases to Cover

* out-of-order events
* missing stage completion
* malformed JSON event line
* abrupt process exit
* partial event log file
* non-Git directory
* huge noisy stdout

---

## 32. Documentation Requirements

Witness documentation should include:

* getting started guide
* architecture overview
* event schema reference
* integration guide for external tools
* config reference
* TUI keyboard shortcuts
* privacy/redaction guide
* troubleshooting guide

---

## 33. Future Enhancements

These are explicitly out of scope for v1 unless easy to land cleanly.

### 33.1 Possible Future Features

* remote collector / daemon mode
* multi-run dashboard
* side-by-side run comparison
* web dashboard sharing the same event backend
* OpenTelemetry export/import bridge
* rule DSL for custom alerts
* file diff drill-down in TUI
* tmux integration
* collaborative/shared runs
* hosted SaaS mode
* SQLite or server-backed history search
* plugin system for custom panels

---

## 34. Acceptance Criteria for v1

Witness v1 is considered successful if it can:

1. Create and persist a run with append-only structured events.
2. Observe and display run lifecycle, stages, tool activity, and model telemetry.
3. Show token/cost summaries in the TUI.
4. Observe Git/file activity and display recent commits/changed files.
5. Detect and surface at least basic stalls, loops, retries, and budget threshold warnings.
6. Reopen and replay a previous run from persisted data.
7. Export a run to JSON and Markdown.
8. Operate locally without external services.
9. Integrate at least passively with arbitrary commands and structurally with at least one or two AI workflow tools.
10. Remain stable under normal local developer usage.

---

## 35. Open Questions

These should be resolved during design/implementation planning.

1. Should Witness include a daemon/collector mode in v1 or only direct CLI-driven capture?
2. Should passive file watching use OS-native fsnotify only, polling fallback, or hybrid?
3. Should snapshots be written periodically, on clean shutdown, or both?
4. Which TUI library offers the best balance of control and simplicity in Go?
5. What is the minimum structured JSON contract needed for external tool integration?
6. Should model pricing tables be built in, configurable, or fetched externally?
7. How should cost estimation handle provider changes over time?
8. Should replay mode support visual charts in terminal or remain text/panel focused in v1?
9. What degree of raw stdout/stderr retention is useful versus dangerous/noisy?
10. Should Witness eventually split into `witness` core and a separate `watchtower` UI binary, or remain one CLI with subcommands?

---

## 36. Recommended Initial Development Phases

### Phase 1: Core Telemetry Foundation

* run model
* event schema
* ndjson storage
* run index
* event append/recovery

### Phase 2: Aggregation and CLI Inspection

* state aggregation
* `runs`, `inspect`, `stats`, `export`
* replay foundation

### Phase 3: Live Capture

* `run -- <command>`
* subprocess observation
* file watch
* Git integration

### Phase 4: TUI

* live dashboard
* event stream
* stage/tool/token/Git panels
* keyboard navigation

### Phase 5: Alerts and Heuristics

* stall detection
* loop detection
* retry storm detection
* budget thresholds

### Phase 6: Tool Integrations

* structured JSON parsing for selected tools
* helper package or schema docs

---

## 37. Product Positioning Summary

Witness is the observability layer for AI software delivery.

It gives operators live visibility into what an AI workflow is doing, what it has changed, what it has cost, and whether it is actually making progress. It turns opaque agentic development into something inspectable, replayable, and governable.

That is the whole point.

Without Witness, autonomous coding workflows are just vibes, token invoices, and hope.
With Witness, they start becoming engineering systems.
