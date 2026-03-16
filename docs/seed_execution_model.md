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
  realization becomes launchable
       ↓
  execution command enqueued
       ↓
  executor launches process, records route bindings
       ↓
  kernel proxies traffic from live execution state
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
- Validates whether the runtime manifest is launchable on the available backend
- Classifies readiness stage and launch capability

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
inspected and grown. One with a runtime manifest can surface a Run action, but
actual launch depends on executor validation instead of a guessed socket path
from repo metadata.

**Code:** `kernel/internal/realizations/meta.go` — `classifyReadiness()`
**Code:** `kernel/internal/realizations/runtime_launch.go`

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
├── runtime.yaml          # author-owned launch recipe
└── <app>/                # implementation code
```

The `runtime.yaml` declares what the realization needs to start, but it does
not own kernel-assigned values such as upstream addresses or kernel capability
URLs.

```yaml
kind: runtime
version: 1
runtime: go
entrypoint: artifacts/app/main.go
working_directory: artifacts/app
run:
  command: go
  args:
    - run
    - .
environment:
  AS_DATA_FILE: data/events.json
```

Once this file exists, the readiness stage advances to **Runnable** and the
Run action becomes available on the boot console when the executor can validate
the manifest for the active backend.

## 7. Running

Running is now a kernel-managed local execution flow:

1. The boot console posts `realizations.launch`.
2. The kernel records a realization execution session in Postgres.
3. `execd` claims the queued launch job.
4. The executor assigns the upstream address and injects kernel-owned
   environment such as `AS_ADDR`, `AS_REGISTRY_URL`, `AS_PUBLIC_API_URL`,
   `AS_INTERNAL_API_URL`, `AS_EXECUTION_ID`, `AS_SEED_ID`, and
   `AS_REALIZATION_ID`.
5. After the process is healthy, the executor records live route bindings.
6. `webd` proxies requests from those route bindings instead of deriving routes
   from repo metadata at startup.

Preview routes are execution-scoped:

- `localhost:8090/__runs/<execution-id>/`

Stable subdomain and path routes only exist when a healthy execution is
activated and its bindings are recorded in runtime state.

**Code:** `kernel/cmd/execd/main.go`
**Code:** `kernel/internal/execution/local_worker.go`
**Code:** `kernel/cmd/webd/main.go` — `dynamicRealizationRoutingMiddleware()`

### Shared deployment note

The current AS DOKS deployment still uses this same execution model in a
transitional form:

- the runtime image contains the repo tree
- the runtime image contains the Go toolchain
- `execd` is colocated with `webd` so localhost-backed upstreams are reachable
- realizations are launched from source rather than from sealed packages

This works, and it is now good enough for live execution, but it is not the
final shared-environment architecture.

The long-term target remains:

- packaged realization artifacts or small per-realization images
- separate workload identity per execution
- executor-assigned capabilities and secret injection at launch time
- no dependency on raw seed-authored `go run` in shared production

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
| Run | Kernel executor | Launch process, bind routes, track health | Runnable (running) |
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
- Local kernel-managed execution via `realizations.launch`
- Shared DOKS execution using the source-backed transitional executor model
- Runtime-backed route bindings for preview and activated routes
- Feedback loop (incidents, request events, test runs to Postgres)

**Not yet automated:** shared packaged execution backends, agent workers
claiming growth jobs, validation execution, registry write-back on acceptance.
