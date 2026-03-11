# Witness Event Schema

This document defines the Witness event envelope, all recognized event types,
the structured tool result contract, and integration examples.

## Event Envelope

Every Witness event is a JSON object with the following fields:

| Field            | Type     | Required | Description                                      |
|------------------|----------|----------|--------------------------------------------------|
| `event_id`       | string   | yes      | Unique ID (ULID with `evt_` prefix)              |
| `schema_version` | string   | yes      | Schema version (currently `"1.0"`)               |
| `timestamp`      | string   | yes      | ISO 8601 UTC timestamp                           |
| `run_id`         | string   | yes      | ID of the run this event belongs to              |
| `type`           | string   | yes      | Event type (see below)                           |
| `source`         | string   | yes      | Component or tool that emitted the event         |
| `payload`        | object   | yes      | Type-specific payload (see per-type docs below)  |
| `stage_id`       | string   | no       | ID of the stage (if applicable)                  |
| `status`         | string   | no       | Status string (e.g., "pass", "fail")             |
| `summary`        | string   | no       | Human-readable summary                           |
| `trace_id`       | string   | no       | Distributed trace ID                             |
| `span_id`        | string   | no       | Distributed span ID                              |
| `parent_event_id`| string   | no       | ID of parent event for causal linking            |
| `tags`           | string[] | no       | Freeform tags                                    |
| `labels`         | object   | no       | Key-value labels                                 |

### Example Envelope

```json
{
  "event_id": "evt_01JXXXXXX",
  "schema_version": "1.0",
  "timestamp": "2026-03-11T14:30:00Z",
  "run_id": "run_01JXXXXXX",
  "type": "tool.completed",
  "source": "golangci-lint",
  "payload": { "tool": "golangci-lint", "status": "pass" },
  "stage_id": "stage_01JXXXXXX",
  "status": "pass",
  "summary": "No issues found"
}
```

## Event Types

### Run Lifecycle

| Type             | Description                              |
|------------------|------------------------------------------|
| `run.created`    | Run record created                       |
| `run.started`    | Subprocess execution began               |
| `run.completed`  | Run finished successfully                |
| `run.failed`     | Run finished with error                  |
| `run.cancelled`  | Run was cancelled by user                |
| `run.stalled`    | No activity detected for stall threshold |

**Payload shape** (`run.started`):
```json
{
  "command": ["claude", "code"],
  "working_dir": "/path/to/project"
}
```

### Stage Lifecycle

| Type               | Description                        |
|--------------------|------------------------------------|
| `stage.created`    | Stage defined in workflow          |
| `stage.started`    | Stage execution began              |
| `stage.progress`   | Progress update within a stage     |
| `stage.completed`  | Stage finished successfully        |
| `stage.failed`     | Stage finished with error          |
| `stage.skipped`    | Stage was skipped                  |

**Payload shape** (`stage.progress`):
```json
{
  "message": "Running tests...",
  "percent": 50
}
```

### Tool Lifecycle

| Type              | Description                         |
|-------------------|-------------------------------------|
| `tool.started`    | External tool invocation began      |
| `tool.output`     | Captured output line from tool      |
| `tool.completed`  | Tool finished (parsed from result)  |
| `tool.failed`     | Tool invocation failed              |

**Payload shape** (`tool.completed`):
```json
{
  "tool": "golangci-lint",
  "status": "pass",
  "summary": "No issues found",
  "duration_ms": 2500,
  "findings": { "error": 0, "warning": 2 },
  "artifacts": { "files": ["report.json"] }
}
```

### Model Telemetry

| Type                       | Description                    |
|----------------------------|--------------------------------|
| `model.request.started`    | LLM API request began          |
| `model.request.completed`  | LLM API request finished       |
| `model.request.failed`     | LLM API request failed         |

**Payload shape** (`model.request.completed`):
```json
{
  "model": "claude-sonnet-4-6",
  "provider": "anthropic",
  "input_tokens": 5000,
  "output_tokens": 1200,
  "cached_tokens": 300
}
```

### Repository / File Events

| Type                  | Description                        |
|-----------------------|------------------------------------|
| `repo.status.changed` | Git working tree status changed   |
| `file.created`        | File was created                  |
| `file.modified`       | File was modified                 |
| `file.deleted`        | File was deleted                  |
| `git.commit.created`  | New commit detected               |
| `git.branch.changed`  | Branch changed                    |

**Payload shape** (`file.modified`):
```json
{
  "path": "internal/config/pricing.go",
  "op": "modified"
}
```

### Test / Validation Events

| Type                  | Description                      |
|-----------------------|----------------------------------|
| `test.started`        | Test suite execution began       |
| `test.completed`      | Test suite passed                |
| `test.failed`         | Test suite failed                |
| `validation.warning`  | Non-fatal validation issue       |
| `validation.error`    | Fatal validation issue           |

**Payload shape** (`test.completed`):
```json
{
  "suite": "go test ./...",
  "passed": 42,
  "failed": 0,
  "duration_ms": 3200
}
```

### Findings / Review Events

| Type                | Description                         |
|---------------------|-------------------------------------|
| `finding.recorded`  | A finding from a review tool        |
| `review.completed`  | A review process completed          |
| `drift.detected`    | Drift between spec and code found   |

**Payload shape** (`finding.recorded`):
```json
{
  "tool": "prism",
  "category": "warning",
  "count": 3
}
```

### System / Health Events

| Type                          | Description                          |
|-------------------------------|--------------------------------------|
| `alert.raised`                | Alert condition detected             |
| `alert.cleared`               | Alert condition resolved             |
| `budget.threshold.exceeded`   | Cost threshold exceeded              |
| `loop.detected`               | Repetitive behavior detected         |
| `stall.detected`              | No progress for stall threshold      |

**Payload shape** (`budget.threshold.exceeded`):
```json
{
  "threshold_usd": 25.00,
  "current_usd": 26.50,
  "scope": "run"
}
```

### Annotation Events

| Type               | Description                      |
|--------------------|----------------------------------|
| `note.recorded`    | Freeform note or annotation      |
| `summary.updated`  | Summary text updated             |

**Payload shape** (`note.recorded`):
```json
{
  "message": "Switched approach from X to Y"
}
```

## Structured Tool Result Contract

External tools can emit a JSON object on stdout that Witness automatically
parses and converts into events. This is the primary integration mechanism.

### Tool Result Schema

```json
{
  "tool": "tool-name",
  "status": "pass|fail|error|warning",
  "summary": "Human-readable summary",
  "findings": {
    "error": 0,
    "warning": 2,
    "info": 5
  },
  "artifacts": {
    "files": ["report.json", "coverage.html"]
  },
  "duration_ms": 1500,
  "tokens": {
    "input": 5000,
    "output": 1200,
    "cached": 300
  },
  "model": "claude-sonnet-4-6",
  "provider": "anthropic",
  "extra": {}
}
```

**Required fields**: `tool`, `status`

**Optional fields**: All others. Omit fields that don't apply to your tool.

### Event Conversion

When Witness parses a tool result, it generates:

1. **`tool.completed`** -- Always emitted. Contains tool name, status, summary,
   duration, findings, and artifacts in the payload.

2. **`model.request.completed`** -- Emitted only if both `tokens` and `model`
   are present. Contains model name, provider, and token counts.

3. **`finding.recorded`** -- One event per finding category with count > 0.
   Contains tool name, category, and count.

## Integration Levels

### Level 1: Zero Integration (Plain Text)

Tools that emit plain text on stdout work out of the box. Witness captures
and displays their output but does not generate structured events.

```
$ golangci-lint run ./...
main.go:10:2: unused variable (SA4006)
```

No events are generated from this output.

### Level 2: Structured Tool Result

Tools emit the tool result JSON schema on stdout. Witness parses it and
generates `tool.completed`, `model.request.completed`, and `finding.recorded`
events automatically.

```json
{"tool":"golangci-lint","status":"fail","summary":"1 issue found","findings":{"error":1},"duration_ms":2000}
```

This generates:
- `tool.completed` with findings and duration
- `finding.recorded` for the "error" category

### Level 3: Native Witness Events

Tools emit full Witness event JSON on stdout. Witness ingests them directly
with no conversion needed. This gives tools full control over event types,
payloads, and metadata.

```json
{"event_id":"evt_01JXXXXXX","schema_version":"1.0","timestamp":"2026-03-11T14:30:00Z","run_id":"run_01JXXXXXX","type":"tool.completed","source":"my-tool","payload":{"custom":"data"}}
```

The `run_id` is overridden to match the current Witness run regardless of what
the tool emits.
