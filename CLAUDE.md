# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Witness is a terminal-first observability platform for AI-driven software development workflows, implemented in Go. It captures structured events from coding agents, LLM calls, CLI tools, repositories, tests, and workflow orchestration, then presents them in a live terminal dashboard with durable run history.

The spec lives at `specs/SPEC.md`.

## Build & Development Commands

```bash
# Build
go build ./cmd/witness/

# Run tests
go test ./...

# Run a single test
go test ./internal/events/ -run TestEventValidation

# Lint (required after Go changes)
golangci-lint run ./...
```

## Architecture

Two conceptual layers with strict separation — the TUI is a consumer of the core, not the core itself.

### Witness Core
Event ingestion, validation, state aggregation, persistence, exports, and alert heuristics.

### Witness TUI
Terminal UI for live monitoring, replay, and drill-down views. Keyboard-driven.

### Package Layout

```
cmd/witness/          # CLI entrypoint
internal/events/      # Event types, envelope, validation, schema versioning
internal/ingest/      # Parse external tool output, stdin/file/pipe ingestion
internal/store/       # Event persistence (ndjson), run metadata, snapshots
internal/aggregate/   # Derive live state from events, calculate metrics
internal/alerts/      # Alert heuristics, threshold evaluation
internal/git/         # Repo detection, status polling, commit summarization
internal/files/       # Filesystem watch, include/exclude handling
internal/models/      # Domain models (Run, Stage, ToolInvocation, ModelRequest, etc.)
internal/export/      # JSON, NDJSON, Markdown exporters
internal/replay/      # Replay state stepping, timeline iteration
internal/tui/         # Terminal app shell, navigation, layouts
internal/ui/panels/   # Individual TUI panels
internal/ui/state/    # UI state management
internal/config/      # Config loading (CLI flags > env vars > config file > defaults)
internal/version/     # Version info
```

### Key Design Principles

- **Event-sourced**: Append-only event log is the source of truth. State is derived from events, never the reverse.
- **Events are immutable**, timestamped, self-describing, and include a `run_id`.
- **Storage**: Local filesystem at `~/.witness/runs/<run-id>/` with `run.json`, `events.ndjson`, and optional `snapshot.json`. No external database in v1.
- **Crash tolerance**: Partial ndjson lines must be tolerated on recovery. Snapshots replaced atomically.
- **Graceful degradation**: Malformed events produce warnings, not crashes. Git/file watch failures degrade gracefully.

### Key Interfaces

```go
EventSink       — Append(ctx, event) error
EventSource     — Subscribe(ctx, runID) (<-chan Event, error)
Aggregator      — Apply(event) error; Snapshot() RunState
Exporter        — Export(ctx, runID, writer) error
AlertEvaluator  — Evaluate(state) []Alert
```

### CLI Commands

`witness run`, `watch`, `attach`, `runs`, `inspect`, `replay`, `export`, `stats`, `doctor`, `config show`

### External Tool Integration

Three levels: passive observation (subprocess start/end), structured JSON summary parsing, and full Witness event emission. Designed to integrate with SpecCritic, PlanCritic, RealityCheck, Prism, Clarion, and Verifier.

## Development Phases

1. Core telemetry foundation (events, storage, run model)
2. Aggregation and CLI inspection (`runs`, `inspect`, `stats`, `export`)
3. Live capture (`run -- <cmd>`, file watch, Git integration)
4. TUI (live dashboard, panels, keyboard navigation)
5. Alerts and heuristics (stall, loop, retry storm, budget)
6. Tool integrations (structured JSON parsing, helper package)
