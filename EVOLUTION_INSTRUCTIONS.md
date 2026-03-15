# Evolution Instructions

> **Boundary rule — read this first.**
> Your work happens in `seeds/`. Do not modify anything under `kernel/`,
> including `kernel/go.mod`, `kernel/go.sum`, `kernel/internal/`, or
> `kernel/cmd/`. The kernel is trusted infrastructure maintained separately.
> Do not modify `compose.yaml` or shared root-level config files.
> All application code, runtime configuration, and artifacts belong under
> `seeds/<seed_id>/realizations/<realization_id>/artifacts/`.

> **Socket assignments.** Each realization listens on a unix domain socket
> to avoid port collisions when multiple realizations run concurrently.
> Use the convention `/tmp/as-realizations/<seed-id>--<realization-id>.sock`.
>
> | Seed | `AS_ADDR` |
> |------|-----------|
> | 0001-shared-notepad | `/tmp/as-realizations/0001-shared-notepad--a-go-htmx-room.sock` |
> | 0003-customer-service-app | `/tmp/as-realizations/0003-customer-service-app--a-web-mvp.sock` |
> | 0004-event-listings | `/tmp/as-realizations/0004-event-listings--a-web-mvp.sock` |

The entire Autosoftware project exists to make this process work. Every other
component — the registry, the materializer, the kernel, federation — is
infrastructure in service of one thing: a seed becomes running software, and
that software keeps evolving from within.

This document is the primary guide for anyone or anything that participates
in that process. Human, AI agent, automated pipeline, or any combination.

For a one-page summary, see [EVOLUTION_QUICK.md](EVOLUTION_QUICK.md).

---

## The Evolution Model

Evolution is not deployment. It is not shipping. It is not CI/CD.

A **seed** is living intent — what a user wants, why they want it, and what
"done" looks like. A **realization** is compiled intent — a concrete
implementation produced from that seed by a specific approach. An
**approach** defines the strategy and tooling used to compile intent into
working software.

The same seed can produce many realizations. Different approaches, different
agents, different models, different coding rulesets. This is a feature. The
registry keeps every accepted realization. The materializer picks the active
one. Nothing is lost.

When the running software produces feedback — errors, incidents, user
signals — that feedback becomes input for the next evolution cycle. The loop
is: **seed → realization → running software → feedback → better
realization**. The system is designed so this loop can run continuously,
with decreasing human involvement as trust in the process increases.

---

## The Context Sequence

The single most impactful factor in evolution quality is the order in which
context is consumed. Whether you are a human reading files or an AI agent
processing a seed packet, this sequence produces better realizations than
any other:

### Step 1: seed.yaml

Identity and status. What seed is this? What stage is it in? This takes
five seconds and orients everything that follows.

```yaml
seed_id: 0003-customer-service-app
version: 1
summary: Web-first support product with tickets, live chat, and a knowledge base.
status: proposed
```

### Step 2: brief.md

What the user actually wants. Written in outcome language, not
implementation language. The brief answers: who is this for, what should it
do, and why does it matter?

Read this first because it prevents you from solving the wrong problem.
Every decision that follows should trace back to something in the brief.

### Step 3: acceptance.md

**Read this before design.** This is counterintuitive but critical.

Acceptance criteria define what "done" looks like. If you read design first,
you will start forming implementation ideas before you know how they will be
judged. Reading acceptance first means every design choice and every line of
code is written with the finish line in view.

Acceptance criteria are seed-level, not realization-level. Every realization
of this seed must satisfy the same criteria regardless of approach or author.

### Step 4: design.md

The shape, boundaries, and constraints of this change. Design responds to
the brief and operates within the acceptance criteria. It may define actors,
state models, scope boundaries, and early validation checkpoints.

Design is where you learn what is in scope and what is explicitly out of
scope. Respecting these boundaries is how realizations stay coherent.

### Step 5: The Approach

Each seed has one or more named approaches in `approaches/`. An approach
is the build configuration for the evolution:

```yaml
approach_id: a-web-mvp
summary: Conservative web MVP with server-rendered pages and light dynamic behavior.
status: active
strategy:
  implementation_style: conservative
  review_style: human-validated-first
  interface_bias: server-rendered-web
realizer:
  model: codex-gpt-5
  prompt_profile: seed-first-product-build
  coding_ruleset: go-html-htmx-mvp
```

The **strategy** block defines how the realization should be built —
conservative or aggressive, server-rendered or client-heavy, human-reviewed
or agent-validated. The **realizer** block defines who or what does the
building — which model, which prompt profile, which coding conventions.

Different approaches produce different realizations of the same seed. A
`minimal` approach and an `ornate` approach can coexist. The registry and
comparison tools help decide which realization to accept.

### Step 6: interaction_contract.yaml

The operational promise. Before writing code, understand what the
realization must expose to the world:

- **Domain objects** — what exists at runtime (tickets, notes, articles)
- **Commands** — what users and integrations can do (create, update, close)
- **Projections** — what they can see (inbox, detail view, search results)
- **Capabilities** — which kernel services the realization uses (sessions,
  search, state machines, communications)
- **Auth modes** — how callers authenticate
- **Consistency semantics** — what guarantees the realization makes

The contract is what makes a realization testable and integrable. A
realization without a contract is a realization nobody can trust.

### Step 7: Existing Realization Artifacts

If you are iterating on an existing realization (not starting fresh), read
what already exists. The `realization.yaml` manifest, the `README.md` and
`notes.md` within the realization directory, and the artifacts themselves.

If artifacts already exist, understand them before changing them. Growth
operations that ignore existing work produce incoherent output.

### Step 8: Kernel Capabilities

The kernel provides shared primitives that realizations should use rather
than reinvent:

- Identity (principals, identifiers, memberships)
- Sessions and auth challenges
- Handles and access links
- Publications and state transitions
- Activity events and jobs
- Communications (threads, messages, assignments, subscriptions)
- Search (documents, facets)
- Guardrails (rate limits, risk events)
- Uploads and file references

These are declared in the interaction contract's `capabilities` list. Using
kernel capabilities means your realization gets identity, access control,
notifications, and search without building them from scratch.

### Why This Order

Brief before design: solve the right problem.
Acceptance before design: know what done looks like.
Approach before code: know the strategy and constraints.
Contract before implementation: define the promise, then fulfill it.
Existing work before changes: don't destroy coherence.
Capabilities before reinventing: use what exists.

The kernel's seed packet API (`GET /v1/projections/realization-growth/seed-packet`)
pre-assembles this context in the correct order. If you are building tooling
that consumes seed packets, preserve this ordering.

---

## Who Does This Work

All of these are valid:

| Method | How It Works |
|--------|-------------|
| **Human with editor** | Clone the repo, read the seed docs, write code, open a PR |
| **AI agent with seed packet** | Consume the seed packet from the growth API, produce artifacts, submit via PR or API |
| **Growth job pipeline** | Queue a job through the boot console or API, a worker (human or agent) claims and executes it |
| **Embedded self-evolution** | The kernel calls AI APIs directly, passing the seed packet and approach config, producing and validating artifacts autonomously |
| **Hybrid** | Human writes the contract, agent builds the implementation. Or agent drafts, human refines. Or agent builds, human validates |
| **External automation** | CI/CD, webhooks, or scheduled jobs trigger growth operations |

The seed packet is identical regardless of method. The approach YAML tells
the system (or the human) which strategy and tooling to use. The interaction
contract defines the same testable promise no matter who fulfills it.

### Embedded Self-Evolution

The approach YAML's `realizer` block is designed for this case:

```yaml
realizer:
  model: codex-gpt-5
  prompt_profile: seed-first-product-build
  coding_ruleset: go-html-htmx-mvp
```

When the kernel operates in self-evolution mode, it reads this block to
determine which AI model to call, which prompt strategy to use, and which
coding conventions to enforce. The kernel assembles the seed packet, calls
the model, receives artifacts, validates them against the contract and
acceptance criteria, and either accepts the result or feeds the failures
back for another pass.

This mode requires API credentials and resource limits configured at the
kernel level. It is the most autonomous method but also the one that
benefits most from high-quality seed docs — the AI has nothing to ask
clarifying questions about, so the brief, design, and acceptance criteria
must be precise.

### Choosing a Method

| Seed complexity | Trust level | Recommended method |
|----------------|-------------|-------------------|
| Simple, well-specified | High | Embedded self-evolution or agent with seed packet |
| Simple, ambiguous brief | Medium | Hybrid — human clarifies brief, agent builds |
| Complex, well-specified | Medium | Agent drafts, human reviews |
| Complex, ambiguous | Low | Human-led with agent assistance |
| First-ever realization of a new seed | Any | Human writes contract first, then any method for implementation |

The interaction contract is the trust boundary. Once a contract exists and
has been validated, autonomous methods become safer because the contract
defines what "correct" means mechanically.

---

## The Interaction Contract

The contract comes before code. Always.

A realization without a contract can exist (status: designed), but it cannot
be trusted, tested, integrated, or compared. Writing the contract is the
first act of realization, not a documentation step after the fact.

### Contract Anatomy

```yaml
contract_version: 1
surface_kind: interactive    # interactive | read_only | bootstrap_only

auth_modes:
  - anonymous
  - session
  - service_token

capabilities:
  - sessions
  - state_transitions
  - activity_events

domain_objects:
  support_ticket:
    schema_ref: "design.md#ticket-state-model"
    capabilities:
      - state_transitions
      - activity_events

commands:
  - name: tickets.create
    path: /v1/commands/0003-customer-service-app/tickets.create
    auth: [session, service_token]
    consistency: read_your_writes

projections:
  - name: support_tickets.detail
    path: /v1/projections/0003-customer-service-app/support_tickets/detail
    auth: [session, service_token, access_link]
```

### What the Contract Enforces

- Every command and projection has a declared path, auth requirement, and
  consistency semantic
- Domain objects reference back to seed design docs via `schema_ref`
- Capabilities are drawn from the kernel's shared primitives
- The same contract is consumed by the first-party UI, third-party
  integrations, AI bots, and operator tools — nobody gets a private
  backdoor

### Contract-First Development

1. Read brief, acceptance, and design
2. Identify domain objects from the design
3. Map them to kernel capabilities
4. Declare commands (what can be done) and projections (what can be seen)
5. Write `interaction_contract.yaml`
6. Validate it: `GET /v1/contracts` shows the kernel's parsed view
7. Then build the implementation that fulfills these promises

This order means your code is shaped by the contract, not the other way
around. The contract is testable — a conformance checker can verify that
every declared command returns a response and every projection is
reachable.

---

## The Realization Pass

A pass is one coherent unit of evolution work. It takes a realization from
one readiness stage to the next (or improves it within a stage).

### Readiness Stages

| Stage | What Exists | What's Possible |
|-------|------------|----------------|
| **Designed** | Seed docs only | Inspect seed docs, plan approach |
| **Defined** | Seed docs + interaction contract | Inspect, test contract, plan implementation |
| **Runnable** | Contract + artifacts + runtime manifest | Inspect, test, run, serve traffic |
| **Accepted** | Runnable + validation evidence + approval | Full production use, registry append |

### Transitions

**Designed → Defined:** Write `interaction_contract.yaml`. This is often
the highest-leverage single file in a realization — it forces design
decisions into a testable format.

**Defined → Runnable:** Add `artifacts/` with implementation code and
`runtime.yaml` with boot instructions. The kernel must be able to start the
realization from `runtime.yaml` and route traffic to it.

```yaml
kind: runtime
version: 1
runtime: go
entrypoint: main.go
working_directory: artifacts/notepad-app
run:
  command: go
  args: [run, main.go]
environment:
  AS_ADDR: /tmp/as-realizations/0001-shared-notepad--a-go-htmx-room.sock
```

**Runnable → Accepted:** Pass acceptance criteria, contract conformance,
and review (human, agent, or both). The realization is appended to the
registry and becomes a candidate for materialization.

### Artifact Conventions

- Implementation code lives under `realizations/<id>/artifacts/`
- Runtime manifest is `artifacts/runtime.yaml`
- Keep artifacts self-contained — a realization should be buildable and
  runnable from its own directory
- Point `realization.yaml`'s `artifacts` list at the files the kernel
  needs to know about

### What One Pass Should Produce

A good pass is coherent and reviewable. It should:

- Move the realization forward by at least one readiness stage, or
  meaningfully improve it within a stage
- Not leave the realization in a broken intermediate state
- Include validation evidence for what was built
- Record durable decisions in `decision_log.md`; use `notes.md` for
  transient working context only

---

## Approaches and Variations

A seed may have multiple named approaches. Each approach is a different
strategy for compiling the same intent into working software.

### When to Create a New Approach

- Different technology stack (Go+HTMX vs React vs server-only)
- Different complexity target (minimal vs full-featured)
- Different AI model or prompt strategy
- Different review philosophy (agent-validated vs human-first)

### When to Create a New Realization Under the Same Approach

- Iterating on an existing implementation
- Branching to try a variation without losing the original
- Targeting a different growth profile (minimal, balanced, ornate)

### Approach Configuration

The `strategy` block guides implementation decisions:

| Field | Purpose | Examples |
|-------|---------|---------|
| `implementation_style` | Overall complexity posture | `conservative`, `aggressive`, `server-rendered-htmx` |
| `review_style` | Who validates the output | `human-validated-first`, `seed-lab-review`, `agent-only` |
| `interface_bias` | UI technology preference | `server-rendered-web`, `htmx-first`, `spa`, `api-only` |
| `state_model` | How runtime state is managed | `in-memory-shared-room`, `database-backed`, `event-sourced` |

The `realizer` block configures automated execution:

| Field | Purpose | Examples |
|-------|---------|---------|
| `model` | Which AI model produces the realization | `codex-gpt-5`, `claude-opus-4-6`, `human` |
| `prompt_profile` | Which prompt strategy to use | `seed-first-product-build`, `realization-first`, `test-driven` |
| `coding_ruleset` | Coding conventions and constraints | `go-html-htmx-mvp`, `react-typescript-strict`, `keep-app-in-seed` |

When the kernel (or a worker) processes a growth job, it reads the approach
YAML to configure the execution environment. When a human does the work,
the approach YAML communicates the intended style and constraints.

---

## Iteration and Mutation

### Operations

| Operation | When to Use |
|-----------|------------|
| **Grow** | First pass, major new capability, or substantial expansion |
| **Tweak** | Targeted adjustment — fix a bug, refine a UI element, adjust behavior |
| **Validate** | Verify current state against acceptance criteria without changing anything |

### The Growth API

Queue a job programmatically:

```
POST /v1/commands/realizations.grow

{
  "reference": "0003-customer-service-app/a-web-mvp",
  "operation": "grow",
  "profile": "balanced",
  "target": "runnable_mvp",
  "developer_instructions": "Focus on the ticket lifecycle first."
}
```

Or use the boot console at `http://localhost:8090/` — select a tile, choose
Grow, configure the job, and submit.

### Mutations from Inside the Running Experience

The sprout flow allows mutations to originate from within a running
realization:

1. Review current seed specs
2. Describe the desired change
3. Review the proposed approach
4. Define UAT criteria for the change
5. Confirm and queue the growth job

This is evolution from within — the running software proposes changes to
itself, mediated by the seed model so nothing happens without visibility
and approval.

### Creating New Realization Variants

To try a fundamentally different approach without losing existing work:

```
POST /v1/commands/realizations.grow

{
  "reference": "0003-customer-service-app/a-web-mvp",
  "operation": "grow",
  "create_new": true,
  "new_realization_id": "b-minimal-api",
  "new_summary": "API-first variant with no server-rendered UI"
}
```

The new realization inherits the seed context but starts with a clean
artifact slate.

---

## Validation and Feedback

### Acceptance Criteria Mapping

Every piece of validation evidence should trace back to a numbered criterion
in `acceptance.md`. When writing validation results, reference the criteria
explicitly:

```
ac-01: PASS — Interface loads without login prompt
ac-02: PASS — Created note via POST /notes, verified in GET /notes
ac-03: PASS — Edited note title and body, changes persisted
```

### Contract Conformance

The interaction contract is mechanically testable:

- Issue each declared command with minimal valid input
- Hit each declared projection and verify response shape
- Verify declared capabilities are actually used
- Flag commands in the contract that return 404 or wrong content types

### The Feedback Loop

Running realizations continuously emit signals:

| Signal | Source | Tagged With |
|--------|--------|------------|
| Client incidents | Browser errors, console.error, HTMX failures | realization_id, seed_id, request_id |
| Request events | HTTP status codes, latency, routes | realization_id, seed_id |
| Test runs | Automated and manual test results | realization_id, suite |
| Agent reviews | AI-generated code review findings | realization_id, reviewer, severity |

All feedback is stored against the realization that produced it. This means
you can compare error rates across realizations of the same seed — a
structured quality signal that goes beyond pass/fail.

### Cross-Realization Comparison

When multiple realizations of the same seed exist, compare them on:

- Acceptance criteria pass rate
- Client incident rate
- Request latency (p50, p95, p99)
- Agent review finding severity distribution
- Artifact size and dependency count

The better realization is the one that satisfies the seed's intent with
fewer problems, not necessarily the one with more features.

---

## Quality of Evolution

### Readiness Stages Are Checkpoints, Not Bureaucracy

Each stage transition represents a real capability gain:

- **Designed** means someone can read and understand the intent
- **Defined** means the operational promise is explicit and testable
- **Runnable** means the kernel can start it and route traffic to it
- **Accepted** means it has passed validation and is trusted for production

Do not skip stages. A realization that jumps from designed to runnable
without a contract is harder to test, harder to compare, and harder to
trust.

### Failed Realizations Are Cheap

The registry keeps everything. A failed realization attempt does not
destroy the seed, does not block other approaches, and does not pollute
the accepted history. The materializer only acts on accepted realizations.

This means the cost of trying is low. Queue a growth job, let an agent
attempt it, review the output. If it fails acceptance, the seed is
unchanged and another attempt can be made with different parameters,
a different model, or a different approach entirely.

### The Best Realization Wins

When multiple realizations of the same seed pass acceptance, the system
does not need to merge them. One is accepted for active use. Others remain
in the registry as alternatives that can be activated later if conditions
change (different performance requirements, different capability needs,
different user feedback).

---

## Worked Examples

### Example 1: 0001-shared-notepad (Simple, One Pass)

**Context sequence:**

1. `seed.yaml` — seed 0001, status proposed, "prove a runnable realization"
2. `brief.md` — shared notepad, no login, in-memory, one room
3. `acceptance.md` — 7 criteria: no-login UI, CRUD notes, shared across
   clients, runnable source in artifacts, machine-readable runtime artifact
4. `design.md` — single shared room, broadcast edits, server-rendered
5. `approaches/a-go-htmx-room.yaml` — Go + HTMX, in-memory shared room,
   realization-first prompt profile
6. `interaction_contract.yaml` — 3 commands (create, update, delete),
   1 projection (room view), anonymous auth
7. No existing artifacts (first pass)
8. No kernel capabilities needed (anonymous, in-memory)

**What one pass produces:**

- `artifacts/notepad-app/main.go` — Go HTTP server with HTMX
- `artifacts/notepad-app/go.mod` — module definition
- `artifacts/runtime.yaml` — `go run main.go` on port 8094
- `realization.yaml` updated with artifact list
- `validation/README.md` with manual test steps mapped to acceptance criteria

**Result:** Designed → Runnable in one pass.

### Example 2: 0003-customer-service-app (Complex, Phased)

**Context sequence:**

1. `seed.yaml` — seed 0003, status proposed
2. `brief.md` — support product for real teams: tickets, chat, knowledge
   base. Multiple actor roles (customer, agent, admin)
3. `acceptance.md` — 10 criteria covering ticket lifecycle, secure access,
   live chat, escalation, KB management
4. `design.md` — defines actors, state machines, scope boundaries (no
   billing, no custom fields in v1), live chat with offline fallback
5. `approaches/a-web-mvp.yaml` — conservative, server-rendered, human-
   validated-first
6. `interaction_contract.yaml` — 3 domain objects (support_ticket,
   chat_session, knowledge_article), 10+ commands, 8+ projections,
   multiple auth modes, 10+ kernel capabilities
7. No existing artifacts (first pass)
8. Kernel capabilities: sessions, state_transitions, activity_events,
   threads, messages, assignments, subscriptions, publications,
   search_documents, access_links

**Phased approach:**

- **Pass 1:** Implement ticket CRUD and state machine (ac-01, ac-04).
  Designed → Runnable for tickets only.
- **Pass 2:** Add secure customer access via access links (ac-02, ac-03).
  Tweak pass.
- **Pass 3:** Add live chat with agent workspace (ac-05, ac-06, ac-07).
  Grow pass.
- **Pass 4:** Add KB authoring and public search (ac-08, ac-09). Grow pass.
- **Pass 5:** Validate all 10 acceptance criteria end-to-end. Validate
  operation.

Each pass is coherent on its own. The realization is runnable after Pass 1
and gets more complete with each subsequent pass.

---

## Reference

### Seed Document Structure

```
seeds/<seed-id>/
├── seed.yaml                    # Identity and status
├── brief.md                     # User request (what and why)
├── design.md                    # Shape, boundaries, constraints
├── acceptance.md                # Success criteria (seed-level)
├── decision_log.md              # Durable rationale
├── notes.md                     # Working notes
├── approaches/
│   └── <approach-id>.yaml       # Build configuration
└── realizations/
    └── <realization-id>/
        ├── realization.yaml     # Realization metadata
        ├── interaction_contract.yaml  # Operational promise
        ├── README.md            # Realization orientation
        ├── notes.md             # Iteration notes
        ├── artifacts/
        │   ├── runtime.yaml     # Boot instructions
        │   └── <app-code>/      # Implementation
        └── validation/
            └── README.md        # Acceptance evidence
```

### realization.yaml

```yaml
realization_id: a-go-htmx-room
seed_id: 0001-shared-notepad
approach_id: a-go-htmx-room
summary: Shared notepad with Go backend and HTMX-driven edits.
status: draft                    # draft | defined | runnable | accepted
subdomain: notepad               # optional: kernel routes this subdomain here
path_prefix: /notepad/           # optional: kernel routes this path here
artifacts:
  - artifacts/runtime.yaml
  - artifacts/notepad-app/main.go
```

### runtime.yaml

```yaml
kind: runtime
version: 1
runtime: go
entrypoint: main.go
working_directory: artifacts/notepad-app
run:
  command: go
  args: [run, main.go]
environment:
  AS_ADDR: /tmp/as-realizations/0001-shared-notepad--a-go-htmx-room.sock
notes:
  - In-memory state only; restart clears all notes
```

### Kernel Capability Catalog

| Capability | What It Provides |
|-----------|-----------------|
| `principals` | User and service account identity |
| `identifiers` | Email, phone, and other identity bindings |
| `memberships` | Principal membership in scopes and groups |
| `sessions` | Authenticated sessions with TTL |
| `auth_challenges` | Verification tokens and challenge flows |
| `consents` | User consent tracking |
| `handles` | Friendly URLs and namespaced identifiers |
| `access_links` | Token-based sharing without full accounts |
| `publications` | Content visibility and scheduling |
| `state_transitions` | Audited state machine transitions |
| `activity_events` | User action logging with visibility levels |
| `jobs` | Background work queue with priority and retry |
| `outbox` | Notification delivery with deduplication |
| `threads` | Conversation containers |
| `messages` | Messages within threads with edit/delete |
| `assignments` | Task assignment to principals |
| `subscriptions` | Watch patterns for change notification |
| `notification_preferences` | Per-principal notification configuration |
| `uploads` | File upload with content hashing |
| `search_documents` | Full-text search with faceting |
| `search_facets` | Structured facet metadata |
| `guard_decisions` | Security check audit trail |
| `risk_events` | Threat detection and response records |

### Growth API Endpoints

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/v1/projections/realization-growth/seed-packet?reference=<ref>` | GET | Retrieve the pre-assembled seed packet |
| `/v1/commands/realizations.grow` | POST | Queue a growth, tweak, or validate job |
| `/v1/projections/realization-growth/jobs/<job_id>` | GET | Poll job status |

### Glossary

| Term | Definition |
|------|-----------|
| **Seed** | A full change capsule: user intent, design, acceptance criteria, approaches, and realizations |
| **Realization** | A concrete implementation compiled from a seed by a specific approach |
| **Approach** | A named strategy and tooling configuration for producing a realization |
| **Claim** | An append-only assertion about an object in the registry |
| **Registry** | The authoritative append-only ledger of accepted changes |
| **Materializer** | The component that reads the registry and rebuilds current running state |
| **Surface** | A user-facing interface (web UI, API, agent interface) |
| **Capability** | A shared kernel primitive that realizations declare and use |
| **Seed packet** | The kernel's pre-assembled bundle of all context needed to produce a realization |
| **Growth job** | A queued unit of evolution work (grow, tweak, or validate) |
| **Interaction contract** | The machine-readable operational promise a realization makes |
| **Feedback loop** | Client incidents, request events, test runs, and agent reviews tied to a realization |
