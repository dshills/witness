# Witness

Flight recorder for AI coding sessions. Observe what Claude Code (or any agentic tool) does in real time — tool calls, model costs, file changes, git activity, stalls, loops — via a live terminal dashboard or replayable timeline.

## Quick Start with Claude Code

```bash
# Observe a Claude Code session (stdin auto-connected)
witness claude

# Name the session for easy lookup later
witness claude --name "refactor auth module"

# Resume the last Claude conversation
witness claude --resume

# Pass extra flags to Claude
witness claude -- --model sonnet

# In another terminal, watch the live dashboard
witness watch
```

That's it. Witness auto-detects your repo and branch, names the run accordingly, and tunes alert thresholds for agentic sessions (longer stall timeout, higher cost limits, wider loop window).

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

## What It Captures

When Claude Code (or any subprocess) runs under Witness:

- **Tool calls** — parsed from structured JSON output on stdout
- **Model usage** — tokens, cost, provider, latency
- **Git activity** — commits, branch changes, dirty file count
- **File changes** — created, modified, deleted (with debounce)
- **Alerts** — stalls, loops, budget overruns, retry storms, failure clusters
- **Everything** — as an append-only event log you can replay and export

## Commands

| Command | Description |
|---------|-------------|
| `witness claude` | Observe a Claude Code session (recommended) |
| `witness run -- <cmd>` | Observe any command |
| `witness wrap` | Pipe stdin through Witness, capturing events |
| `witness watch` | Live TUI dashboard |
| `witness attach --run <id>` | Attach TUI to an active run |
| `witness replay <run-id>` | Replay a historical run |
| `witness runs` | List recorded runs |
| `witness inspect <run-id>` | Detailed run summary |
| `witness stats <run-id>` | Aggregated metrics |
| `witness export <run-id>` | Export (JSON, NDJSON, Markdown) |
| `witness doctor` | System health check |
| `witness config show` | Show effective config |

### `witness claude`

```
witness claude [flags] [-- extra-args...]

Flags:
  --name <string>   Run name (default: auto from repo/branch)
  --resume           Resume last Claude conversation
  --no-git           Disable git observation
```

Purpose-built for Claude Code. Automatically:
- Connects stdin (interactive mode)
- Names the run from your repo and branch (e.g., `witness/main`)
- Applies agentic defaults: 30min stall threshold, $100 cost limit, wider loop window

### `witness run`

```
witness run [flags] -- <command> [args...]

Flags:
  -i, --interactive    Force stdin passthrough (auto-detected for terminals)
  --name <string>      Human-readable run name
  --no-git             Disable git observation
  --no-files           Disable filesystem observation
```

General-purpose command observer. Stdin is auto-connected when running from a terminal — no `-i` needed for interactive commands:

```bash
witness run -- make build
witness run --name "deploy" -- ./deploy.sh staging
witness run -- python -i           # stdin auto-connected
```

### `witness wrap`

```
witness wrap [flags]

Flags:
  --run <id>        Attach to an existing run
  --name <string>   Create a new run
```

Lightweight stdin/stdout pipe. Captures events without being the parent process:

```bash
prism review --json | witness wrap --name "code-review"
golangci-lint run ./... 2>&1 | witness wrap --run run_01J...
```

### `witness watch` / `witness attach`

```
witness watch [--run <id|latest>]
witness attach --run <run-id>
```

Opens the live TUI. `watch` finds the most recent active run. `attach` requires the run to be active.

### `witness replay`

```
witness replay <run-id> [flags]

Flags:
  --speed <float>    Playback speed (default 1.0 = real time)
  --summary          Print postmortem only
  --text             Text timeline (no TUI)
```

Replay keys: `space` play/pause, `left`/`right` step, `<`/`>` speed, `n`/`N` stage, `c`/`C` commit, `A` alert.

## TUI Dashboard

Seven panels updating at 500ms, with two-column layout on wide terminals:

```
+----------------- Header ------------------+
+-----------+---------+---------------------+
| Stages    | Active Tool/Model             |
+-----------+-------------------------------+
| Tokens/$  | Git/Files                     |
+-----------+-------------------------------+
| Alerts                                    |
+-------------------------------------------+
| Event Stream                              |
+-------------------------------------------+
```

Keys: `q` quit, `Tab` focus, `j`/`k` scroll, `p`/`r` pause/resume, `/` filter, `?` help, `s`/`t`/`g`/`a`/`e`/`m` drill-down.

## Configuration

Config file: `~/.witness/config.yaml`

```yaml
storage:
  root: ~/.witness

ui:
  refresh_ms: 500
  theme: auto

alerts:
  stall_duration: 10m            # witness claude uses 30m
  loop_window: 8                 # witness claude uses 15
  max_run_cost_usd: 25.00       # witness claude uses 100.00
  max_stage_cost_usd: 8.00      # witness claude uses 25.00

files:
  ignore:
    - ".git/**"
    - "node_modules/**"
    - "vendor/**"
    - "*.swp"
    - ".DS_Store"

privacy:
  redact_patterns:
    - '(?i)(sk-[a-zA-Z0-9]{20,}|AKIA[A-Z0-9]{16})'
    - '(?i)bearer\s+[A-Za-z0-9\-._~+/=]{20,}'

git:
  poll_interval_seconds: 5

pricing:
  models:
    - provider: anthropic
      model: claude-sonnet-4-6
      input_per_m_token: 3.00
      output_per_m_token: 15.00
```

Environment overrides: `WITNESS_STORAGE_ROOT`, `WITNESS_UI_REFRESH_MS`, `WITNESS_STALL_DURATION`, `WITNESS_MAX_RUN_COST_USD`.

## Claude Code Hooks Integration

Claude Code supports hooks that fire on tool use. You can configure hooks to emit Witness events directly, without wrapping the process:

```json
// ~/.claude/hooks.json
{
  "hooks": {
    "tool_use": {
      "command": "witness wrap --run $WITNESS_RUN_ID"
    }
  }
}
```

Or start a Witness run first and reference its ID:

```bash
# Start Claude under Witness
witness claude

# Or manually: start a run, export the ID, then use it in hooks
export WITNESS_RUN_ID=$(witness run --name "session" -- echo "started" 2>&1 | grep -o 'run_[^ ]*')
```

## Tool Integration

Three levels, from zero effort to full control:

**Level 0** — No integration. Witness captures stdout/stderr as-is.

**Level 1** — Emit structured JSON with `tool` + `status`:

```json
{"tool":"golangci-lint","status":"pass","summary":"0 issues","findings":{"warning":2},"duration_ms":1500}
```

Witness auto-generates `tool.completed`, `model.request.completed`, and `finding.recorded` events.

**Level 2** — Emit native Witness events (full `event_id`, `type`, `payload`). See [`docs/event-schema.md`](docs/event-schema.md).

## Alerts

| Rule | Trigger | Severity |
|------|---------|----------|
| **Stall** | No activity for `stall_duration` | warning |
| **Loop** | Same tool 75%+ of last `loop_window` calls | warning |
| **Budget** | Cost exceeds `max_run_cost_usd` | error |
| **Retry Storm** | 5+ same-tool failures in 10 calls | warning |
| **Failure Density** | 3+ failures in 60 seconds | warning |

Deduplicated — each alert fires at most once per run.

## Storage

```
~/.witness/runs/<run-id>/
  run.json          # metadata
  events.ndjson     # append-only event log
  snapshot.json     # cached state
```

Crash-tolerant: malformed trailing lines discarded, snapshots atomically replaced.

## Architecture

Event-sourced: append-only event log is the source of truth. State is derived by replaying events through the aggregator. Snapshots are periodic caches.

```
witness run/claude
       |
  +---------+----------+-----------+
  |         |          |           |
  Git    Stdout     File        Stdin
  Poll   Scanner   Watcher   (interactive)
  |         |          |
  +----+----+----------+
       |
   StoreSink (validate -> redact -> persist -> aggregate -> alert)
       |
  +----+----+-------+
  |         |       |
FSStore   Agg    Alerts
(NDJSON)    |
         TUI / Export / Replay
```

## Built-in Pricing

| Provider | Models |
|----------|--------|
| Anthropic | claude-opus-4-6, claude-sonnet-4-6, claude-haiku-4-5 |
| OpenAI | gpt-4o, gpt-4o-mini, o1, o3, o3-mini, o4-mini |

Override via `pricing.models` in config.

## License

MIT — see [LICENSE](LICENSE).
