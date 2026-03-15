# Seed Execution Model

How a seed on disk becomes a running system.

```
seed folder on disk
       ↓
  kernel discovery
       ↓
  boot console (inspect, grow, run)
       ↓
  growth job enqueued
       ↓
  worker produces artifacts ← (human, agent, or pipeline)
       ↓
  realization becomes runnable
       ↓
  process launched, kernel proxies traffic
       ↓
  registry records acceptance ← (append-only ledger)
```

---

## 1. Seed Folder

Everything starts as files on disk. A seed directory holds:

- `seed.yaml` — identity, status, version
- `brief.md` — what the user wants
- `acceptance.md` — what "done" looks like
- `design.md` — shape, boundaries, constraints
- `approaches/*.yaml` — strategy and realizer configuration

No code yet. No runtime. Just the request and the criteria for success.

## 2. Kernel Discovery

At startup, the kernel walks `seeds/` and `genesis/realizations/` looking for
`realization.yaml` files. Each one produces a `LocalRealization` with its
manifest, reference (`seed-id/realization-id`), and root directory.

**Code:** `kernel/internal/realizations/catalog.go` — `Discover()`

## 3. Boot Console

The web server calls `ListRealizations()` to enrich discovered realizations
with metadata:

- Loads seed docs (brief, acceptance, design)
- Loads interaction contract if present
- Finds and parses `runtime.yaml` if present
- Extracts `AS_ADDR` for proxy routing
- Classifies readiness stage

The boot console at `http://localhost:8090/` renders tiles grouped by seed.
Each tile shows status, readiness, and available actions.

**Code:** `kernel/internal/materializer/service.go` — `ListRealizations()`
**Code:** `kernel/cmd/webd/bootloader.go` — `newBootPageView()`

## 4. Readiness Stages

The kernel determines what stage a realization is in automatically:

| Stage | Condition | Actions Available |
|-------|-----------|-------------------|
| **Designed** | Seed docs exist, no contract | Inspect, Grow |
| **Defined** | Has `interaction_contract.yaml` | Inspect, Grow |
| **Runnable** | Has `runtime.yaml` + artifacts | Inspect, Grow, Run |
| **Accepted** | Runnable + status accepted | Inspect, Grow, Run |

Each stage unlocks more actions. A realization that only has docs can be
inspected and grown. One with a runtime manifest can also be run.

**Code:** `kernel/internal/realizations/meta.go` — `classifyReadiness()`

## 5. Growth

Clicking "Grow" on the boot console opens a form with:

- **Operation:** grow (first pass), tweak (fix), or validate (verify only)
- **Profile:** minimal, balanced, ornate, or custom
- **Target:** runnable_mvp, api_first, ux_surface, or validation_only
- **Developer instructions:** free text guidance

Submitting the form enqueues a job to the `runtime_jobs` table with the full
seed packet — every document the worker needs to produce a realization.

```
POST /v1/commands/realizations.grow
→ Job queued: job_abc123, status: pending
→ Payload contains: seed docs, contract, approach config, instructions
```

A worker (human, AI agent, or pipeline) claims the job, reads the seed packet,
and produces artifacts: application code, `runtime.yaml`, validation evidence.
When done, the worker writes artifacts to disk and marks the job complete.

**Code:** `kernel/internal/http/json/growth.go`
**Code:** `kernel/internal/interactions/runtime_webstate.go`

## 6. Artifacts and Runtime

A runnable realization has:

```
realizations/<id>/artifacts/
├── runtime.yaml          # how to boot
└── <app>/                # implementation code
```

The `runtime.yaml` declares everything the kernel needs to launch:

```yaml
kind: runtime
version: 1
runtime: go
entrypoint: main.go
working_directory: artifacts/<app>
run_command: go run .
environment:
  AS_ADDR: /tmp/as-realizations/<seed-id>--<realization-id>.sock
```

Once this file exists, the readiness stage advances to **Runnable** and the
Run action becomes available on the boot console.

## 7. Running

Today, running is manual. The boot console shows the runtime recipe — command,
environment, working directory — and the operator launches it. The kernel does
not yet manage processes directly.

Once the realization process is listening on its `AS_ADDR`, the kernel's
routing middleware proxies traffic to it via unix domain socket:

- **Subdomain routing:** `notepad.localhost:8090` → `/tmp/as-realizations/<seed>--<realization>.sock`
- **Path prefix routing:** `localhost:8090/notepad/` → `/tmp/as-realizations/<seed>--<realization>.sock`

The kernel injects `X-AS-Seed-ID` and `X-AS-Realization-ID` headers so the
realization knows its own identity.

**Code:** `kernel/cmd/webd/main.go` — `realizationRoutingMiddleware()`

## 8. Registry and Acceptance

The registry is not in the critical path of getting a realization running.
It enters the picture at **acceptance** — when a realization is reviewed,
validated, and promoted.

Accepted mutations are appended to the registry as immutable records. The
registry provides:

- Append-only history of every accepted change
- Deterministic replay (rebuild current state from the ledger)
- Traceability from running behavior back to the mutation that produced it
- Supersession instead of silent overwrite

The materializer reads the registry and rebuilds derived views. The running
application is always a projection of accepted history.

## 9. The Full Sequence with Timing

| Phase | Who | What Happens | Advances To |
|-------|-----|-------------|-------------|
| Author | Human | Write seed docs: brief, acceptance, design | Designed |
| Define | Human or agent | Write `interaction_contract.yaml` | Defined |
| Grow | Agent or human | Produce code + `runtime.yaml` | Runnable |
| Run | Operator | Launch process, kernel proxies | Runnable (running) |
| Validate | Agent or human | Test against acceptance criteria | Runnable (validated) |
| Accept | Reviewer | Approve, record in registry | Accepted |

Not every realization goes through every phase linearly. A growth pass might
jump from Designed straight to Runnable if the worker produces both the
contract and the implementation in one pass.

## 10. What Works Today

- Seed discovery from disk
- Boot console with tile grid, status, readiness
- Inspect action (materializes snapshot to disk)
- Growth job enqueueing with full seed packet
- Routing middleware (subdomain + path prefix proxying)
- Manual process launch from runtime recipe
- Feedback loop (incidents, request events, test runs to Postgres)

**Not yet automated:** agent workers claiming growth jobs, process management,
validation execution, registry write-back on acceptance.
