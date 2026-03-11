# Witness

Terminal-first observability platform for AI-driven software development workflows.

Witness wraps any command, captures what happens — tool calls, model requests, file changes, git activity, cost accumulation — and presents it as a live dashboard or replayable timeline. Think of it as flight-recorder meets `htop` for agentic coding sessions.

## Install

```bash
go install github.com/dshills/witness/cmd/witness@latest
```

Or build from source:

```bash
go build -ldflags \
  "-X github.com/dshills/witness/internal/version.Version=0.1.0 \
   -X github.com/dshills/witness/internal/version.Commit=$(git rev-parse HEAD) \
   -X github.com/dshills/witness/internal/version.Date=$(date -u +'%Y-%m-%dT%H:%M:%SZ')" \
  ./cmd/witness/
```

Requires Go 1.26+.

## Quick Start

```bash
# Run a command under observation
witness run -- make build

# Run with a name and disable git polling
witness run --name "deploy staging" --no-git -- ./deploy.sh staging

# Open live dashboard for the most recent active run
witness watch

# List past runs
witness runs

# Replay a historical run in the TUI
witness replay run_01JXXXXXX

# Export a run as Markdown
witness export run_01JXXXXXX --format markdown
```

## Commands

| Command | Description |
|---------|-------------|
| `witness run -- <cmd>` | Run a command with live observation |
| `witness watch` | Open live TUI dashboard |
| `witness attach --run <id>` | Attach TUI to an active run |
| `witness replay <run-id>` | Replay a historical run |
| `witness runs` | List recorded runs |
| `witness inspect <run-id>` | Print detailed run summary |
| `witness stats <run-id>` | Print aggregated metrics |
| `witness export <run-id>` | Export run data (JSON, NDJSON, Markdown) |
| `witness doctor` | Check system health |
| `witness config show` | Display effective configuration |
| `witness version` | Print version info |

### `witness run`

```
witness run [flags] -- <command> [args...]

Flags:
  --name <string>    Human-readable run name
  --no-git           Disable git observation
  --no-files         Disable filesystem observation
```

Captures subprocess stdout/stderr (relayed to your terminal), monitors git for new commits and branch changes, watches the filesystem for file modifications, and detects structured tool output on stdout.

Signal handling: SIGINT and SIGTERM are forwarded to the subprocess process group. If the process doesn't exit within 10 seconds, the group is killed.

### `witness watch` / `witness attach`

```
witness watch [--run <id|latest>]
witness attach --run <run-id>
```

Opens a Bubble Tea TUI with 7 panels: header, stages, active work, token/cost, git/files, alerts, and event stream. `attach` requires the run to be active.

### `witness replay`

```
witness replay <run-id> [flags]

Flags:
  --speed <float>    Playback speed (default: 1.0 in TUI, 0 = instant)
  --summary          Print postmortem summary only
  --text             Text-based timeline (no TUI)
```

Default opens the TUI in replay mode with playback controls:

| Key | Action |
|-----|--------|
| `space` | Play/pause |
| `right`/`l` | Step forward |
| `left`/`h` | Step backward |
| `>`/`.` | Increase speed |
| `<`/`,` | Decrease speed |
| `n`/`N` | Next/previous stage transition |
| `c`/`C` | Next/previous commit |
| `A` | Next alert |

### `witness runs`

```
witness runs [--status <filter>] [--limit <n>]
```

### `witness export`

```
witness export <run-id> [--format json|ndjson|markdown] [--output <file>]
```

## TUI Dashboard

The live dashboard updates at 500ms intervals (configurable) and adapts to terminal size:

**Full layout** (100+ columns):
```
┌─────────────── Header ───────────────────┐
├──────────────┬───────────────────────────┤
│ Stages       │ Active Tool/Model         │
├──────────────┤───────────────────────────┤
│ Tokens/Cost  │ Git/Files                 │
├──────────────┴───────────────────────────┤
│ Alerts                                   │
├──────────────────────────────────────────┤
│ Event Stream                             │
└──────────────────────────────────────────┘
```

**Compact layout** (<100 columns): single-column stacked panels.

**Keyboard shortcuts:**

| Key | Action |
|-----|--------|
| `q` / `Ctrl-C` | Quit |
| `Tab` / `Shift-Tab` | Cycle panel focus |
| `j`/`k` | Scroll in focused panel |
| `p` / `r` | Pause / resume event stream |
| `/` | Filter event stream |
| `?` | Toggle help overlay |
| `s`/`t`/`g`/`a`/`e`/`m` | Drill-down views |

## Configuration

Witness loads config from `~/.witness/config.yaml`, with environment variable overrides.

```yaml
storage:
  root: ~/.witness

ui:
  refresh_ms: 500
  theme: auto                    # auto, light, dark

alerts:
  stall_duration: 10m
  loop_window: 8
  max_run_cost_usd: 25.00
  max_stage_cost_usd: 8.00
  # max_tokens: 1000000          # optional token limit

files:
  ignore:
    - ".git/**"
    - "node_modules/**"
    - "vendor/**"
    - "dist/**"
    - "build/**"
    - ".next/**"
    - "*.swp"
    - "*~"
    - ".DS_Store"

privacy:
  redact_patterns:
    - '(?i)(sk-[a-zA-Z0-9]{20,}|AKIA[A-Z0-9]{16})'
    - '(?i)bearer\s+[A-Za-z0-9\-._~+/=]{20,}'
    - '(?i)(password|secret|apikey)\s*[=:]\s*\S{8,}'

git:
  poll_interval_seconds: 5

pricing:
  models:
    - provider: anthropic
      model: claude-sonnet-4-6
      input_per_m_token: 3.00
      output_per_m_token: 15.00
```

**Environment overrides:**

| Variable | Config path |
|----------|-------------|
| `WITNESS_STORAGE_ROOT` | `storage.root` |
| `WITNESS_UI_REFRESH_MS` | `ui.refresh_ms` |
| `WITNESS_STALL_DURATION` | `alerts.stall_duration` |
| `WITNESS_MAX_RUN_COST_USD` | `alerts.max_run_cost_usd` |

## Tool Integration

Tools can integrate with Witness at three levels:

### Level 0: No Integration

Witness captures stdout/stderr as-is. No structured events are generated.

### Level 1: Structured Tool Result

Emit a single JSON object on stdout with at minimum `tool` and `status`:

```json
{
  "tool": "golangci-lint",
  "status": "pass",
  "summary": "0 issues found",
  "findings": { "error": 0, "warning": 2 },
  "duration_ms": 1500,
  "tokens": { "input": 5000, "output": 1200 },
  "model": "claude-sonnet-4-6",
  "provider": "anthropic"
}
```

Witness automatically converts this into `tool.completed`, `model.request.completed`, and `finding.recorded` events.

### Level 2: Native Events

Emit full Witness event JSON on stdout:

```json
{
  "event_id": "evt_01J...",
  "schema_version": "1.0",
  "timestamp": "2026-03-11T14:30:00Z",
  "type": "tool.completed",
  "source": "my-tool",
  "payload": { "findings": { "error": 0 } },
  "summary": "All checks passed"
}
```

The `run_id` is overridden to match the current Witness run. Events must have both `event_id` and `type` to be recognized.

See [`docs/event-schema.md`](docs/event-schema.md) for the complete event type reference.

## Alert Heuristics

Witness evaluates alert rules after every event:

| Rule | Trigger | Severity |
|------|---------|----------|
| **Stall** | No file or stage changes for `stall_duration` | warning |
| **Loop** | Same tool repeated 75%+ of last `loop_window` invocations | warning |
| **Budget** | Run cost exceeds `max_run_cost_usd` | error |
| **Stage Budget** | Stage cost exceeds `max_stage_cost_usd` | warning |
| **Retry Storm** | 5+ failures of same tool in last 10 invocations | warning |
| **Failure Density** | 3+ failure events within 60 seconds | warning |

Alerts are deduplicated — each alert type fires at most once per run.

## Storage

All data is stored on the local filesystem:

```
~/.witness/
└── runs/
    └── run_01JXXXXXX/
        ├── run.json           # Run metadata
        ├── events.ndjson      # Append-only event log
        └── snapshot.json      # Cached aggregated state
```

- **events.ndjson** — append-only, O_APPEND, synced after each write
- **snapshot.json** — atomically replaced via temp file + rename
- **Crash-tolerant** — malformed trailing lines are discarded on read

## Architecture

```
                    ┌──────────────┐
                    │ witness run  │
                    └──────┬───────┘
                           │
              ┌────────────┼────────────┐
              │            │            │
        ┌─────┴─────┐ ┌───┴───┐ ┌─────┴─────┐
        │ Git       │ │Stdout │ │ File      │
        │ Observer  │ │Scanner│ │ Watcher   │
        └─────┬─────┘ └───┬───┘ └─────┬─────┘
              │            │            │
              └────────────┼────────────┘
                           │
                    ┌──────┴───────┐
                    │  StoreSink   │
                    │  (validate,  │
                    │   redact,    │
                    │   persist,   │
                    │   aggregate, │
                    │   alert)     │
                    └──────┬───────┘
                           │
              ┌────────────┼────────────┐
              │            │            │
        ┌─────┴─────┐ ┌───┴───┐ ┌─────┴─────┐
        │ FSStore   │ │ Agg   │ │  Alert    │
        │ (NDJSON)  │ │       │ │  Engine   │
        └───────────┘ └───┬───┘ └───────────┘
                          │
                    ┌─────┴──────┐
                    │   TUI /    │
                    │   Export   │
                    └────────────┘
```

Event-sourced design: the append-only event log is the source of truth. State is derived by replaying events through the aggregator. Snapshots are periodic caches for fast startup.

## Built-in Pricing

Cost estimation is included for common models:

| Provider | Models |
|----------|--------|
| Anthropic | claude-opus-4-6, claude-sonnet-4-6, claude-haiku-4-5 |
| OpenAI | gpt-4o, gpt-4o-mini, o1, o3, o3-mini, o4-mini |

Override or extend via `pricing.models` in config. Unknown models return $0 cost with a logged warning.

## License

See [LICENSE](LICENSE) for details.
