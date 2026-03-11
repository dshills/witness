# Example: Building a Feature with Witness + Full Toolchain

This walkthrough shows a complete development cycle using Witness to observe a Claude Code session, with speccritic, plancritic, prism, realitycheck, clarion, and verifier validating each step.

## Scenario

You're adding a webhook notification system to an existing Go microservice. The full workflow: spec → plan → implement → review → validate → document → test → commit.

---

## 1. Start Your Observed Session

Open a Witness-observed Claude Code session:

```bash
witness claude --name "add-webhooks"
```

Witness auto-detects your repo (`myservice/main`), connects stdin, and starts capturing git activity, file changes, and tool output. In a second terminal, open the live dashboard:

```bash
witness watch
```

You now have real-time visibility into everything that happens during development.

---

## 2. Write the Spec

Ask Claude to write the spec, then validate it:

```
> Write a SPEC.md for a webhook notification system. Events: user.created, order.completed, payment.failed. Support retry with exponential backoff, HMAC signing, and a delivery log.
```

Claude writes `specs/SPEC.md`. Validate it:

```bash
speccritic check
```

```
speccritic v1.2.0 — specs/SPEC.md

[DEFECT] §4.2: Retry policy says "reasonable backoff" — must specify exact
         intervals or formula (e.g., min(2^attempt * 1s, 5m))
[DEFECT] §6.1: HMAC signing mentions "a secret" — must specify per-endpoint
         or global, rotation policy, and which hash algorithm
[WARNING] §3: No rate limiting specified for outbound webhook delivery
[WARNING] §7: No mention of payload size limits

2 defects, 2 warnings — iterate before proceeding
```

Fix the defects, re-run until clean:

```bash
speccritic check
```

```
speccritic v1.2.0 — specs/SPEC.md
0 defects, 0 warnings — spec is ready
```

---

## 3. Generate the Plan

Ask Claude to create a phased implementation plan, then validate:

```
> Create a PLAN.md from the spec. 4 phases: domain models, delivery engine, API endpoints, retry worker.
```

Claude writes `specs/PLAN.md`. Validate it:

```bash
plancritic check
```

```
plancritic v1.1.0 — specs/PLAN.md

[RISK] Phase 3 depends on Phase 2 delivery engine but doesn't list it
       as a dependency — add explicit dependency
[AMBIGUITY] Phase 4 says "handle failures gracefully" — must specify:
            dead-letter queue? alert after N failures? disable endpoint?
[OK] No contradictions found
[OK] Phase ordering is acyclic

1 risk, 1 ambiguity — iterate before proceeding
```

Fix and re-run until clean.

---

## 4. Implement Phase by Phase

Work through each phase with Claude. Witness captures everything — the files Claude creates, the git commits, and the tool invocations.

```
> Implement Phase 1 from the plan — domain models for webhook endpoints, events, and delivery attempts.
```

Claude creates files. After each phase, run the validation pipeline:

### Lint and Test

```bash
golangci-lint run ./...
go test ./...
```

### Code Review with Prism

Pipe the review through `witness wrap` so findings are captured in your Witness run:

```bash
prism review staged --provider openai --model gpt-4o 2>&1 | witness wrap --run $(witness runs --status running --limit 1 -q)
```

Or review directly:

```bash
prism review staged --provider openai --model gpt-4o
```

```
Prism Code Review — staged mode
Findings: 3 total (1 high, 1 medium, 1 low)

[!] HIGH
  internal/webhook/deliver.go:45  HTTP client has no timeout
  Suggestion: Use &http.Client{Timeout: 30 * time.Second}

[!] MEDIUM
  internal/webhook/sign.go:12  HMAC key stored as string, not []byte
  Suggestion: Accept []byte to avoid encoding assumptions

[-] LOW
  internal/webhook/models.go:8  EndpointID could use a type alias for clarity
```

Fix the HIGH and MEDIUM findings, re-run prism until clean.

### Spec Conformance with Realitycheck

```bash
realitycheck check --spec ./specs/SPEC.md --plan ./specs/PLAN.md
```

```
realitycheck v0.9.0

Phase 1: domain models
  [PASS] Endpoint model matches spec §3.1
  [PASS] Event types match spec §2
  [DRIFT] DeliveryAttempt missing `response_body` field from spec §5.3

Score: 92/100 — 1 drift item
```

Fix the drift, re-run until score is satisfactory.

### Documentation with Clarion

```bash
clarion pack enterprise --spec ./specs/SPEC.md --plan ./specs/PLAN.md
```

Clarion generates/updates docs. Check for drift:

```bash
clarion drift
```

If drift detected, regenerate:

```bash
clarion gen --section api-reference
```

### Test Coverage with Verifier

```bash
verifier analyze
```

```
Verifier Report — Risk Score: 45

Critical Gaps:
  TESTREC-A1B2: Concurrency test for DeliverWebhook (confidence: 0.8)
  TESTREC-C3D4: Unit test for HMAC signature verification (confidence: 0.9)

Medium Gaps:
  TESTREC-E5F6: Table-driven test for retry backoff calculation
  TESTREC-G7H8: Error path test for endpoint not found
```

Scaffold the critical tests:

```bash
verifier scaffold --rec TESTREC-A1B2,TESTREC-C3D4
```

Then implement the test logic and run:

```bash
go test -race ./...
```

---

## 5. Commit the Phase

Once all validation passes:

```bash
git add internal/webhook/
git commit -m "feat: Phase 1 — webhook domain models and delivery types"
```

Repeat steps 4-5 for each phase.

---

## 6. After the Session

End the Claude session (`/exit` or Ctrl-C). Witness prints a summary:

```
witness: run run_01JXYZ completed (847 events)
```

### Review the Session

```bash
# Quick stats
witness stats run_01JXYZ

# Full postmortem
witness replay run_01JXYZ --summary

# Detailed event replay in TUI
witness replay run_01JXYZ

# Export for team review
witness export run_01JXYZ --format markdown --output session-report.md
```

### Example Stats Output

```
witness stats run_01JXYZ

Duration by Stage:
  domain-models     12m 34s
  delivery-engine   28m 11s
  api-endpoints     19m 45s
  retry-worker      15m 22s

Tokens by Provider:
  PROVIDER    INPUT     OUTPUT    CACHED    COST
  anthropic   125,400   48,200    31,000    $2.14
  openai      42,100    15,800    0         $0.38

Tools Used:
  TOOL              COUNT   AVG LATENCY
  golangci-lint     8       1.2s
  prism             4       38.4s
  realitycheck      4       12.1s
  go-test           12      3.8s
  verifier          2       8.5s
  speccritic        3       6.2s
  clarion           2       15.3s

Summary:
  Duration:        1h 15m 52s
  Total Cost:      $2.52
  Files Changed:   34
  Commits:         4
  Alerts:          1 (budget warning at $2.00)
  Test Failures:   2 (resolved)
```

---

## Workflow Summary

```
speccritic   ──→  Validate SPEC.md (defects, ambiguities)
plancritic   ──→  Validate PLAN.md (risks, contradictions)
prism        ──→  Code review (bugs, security, style)
realitycheck ──→  Spec/plan conformance (drift detection)
clarion      ──→  Generate and maintain documentation
verifier     ──→  Find test gaps, scaffold test stubs

witness claude  ──→  Observe the entire session
witness watch   ──→  Live dashboard in another terminal
witness stats   ──→  Post-session metrics
witness replay  ──→  Step through what happened
witness export  ──→  Share the report
```

Each tool's output flows through Witness as events, giving you a complete record of not just what code was written, but how it was validated.
