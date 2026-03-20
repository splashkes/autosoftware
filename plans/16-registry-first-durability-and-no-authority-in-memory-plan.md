# AS Plan: Registry-First Durability and No Authority in Memory

## Purpose

This plan closes the durability gap that allowed accepted-looking writes to exist
only in process memory or only in seed-local Postgres tables.

The goal is to make this impossible across AS:

- no authoritative mutable state living only in memory
- no domain facts living only in seed-local Postgres tables
- no seed command path succeeding unless the resulting fact is durably captured
- no production realization starting in a mode that can lose accepted writes on
  reboot or cross-instance routing

This is not only a Flowershow problem. Flowershow exposed a repo-wide
architecture risk.

---

## Problem Statement

Two different failure classes currently exist:

1. Some realizations still store accepted-looking domain writes only in process
   memory.
2. Some durable writes may land only in seed-local Postgres tables instead of
   the kernel registry, which makes them hard to audit, replay, migrate, or
   rebuild correctly.

That creates multiple unacceptable states:

- API success responses that are not durable
- writes visible on one instance and absent on another
- facts lost on restart
- facts impossible to reconstruct from the kernel registry
- projections that are functioning as hidden sources of truth

The architecture must instead guarantee:

- accepted domain facts go to the registry first
- runtime operational state goes to durable runtime storage
- Postgres read models are disposable projections
- memory is cache, transport, or request-local only

---

## Non-Negotiable Invariants

### 1. No authoritative mutable state in memory

Process memory may be used only for:

- per-request computation
- derived caches of durable state
- transport-local state like SSE subscribers or websocket sessions
- static boot caches such as parsed templates or loaded contracts
- best-effort protective state whose loss cannot corrupt truth

Process memory may not be the only durable home for:

- organizations
- shows
- schedules
- classes
- entries
- citations
- credits
- invites
- roles or authority grants
- sessions
- tokens
- jobs
- accepted claim history

### 2. Domain facts must be registry-first

If a field or object is a domain fact that should survive reboot, audit, share,
replay, or migrate, it must enter the system through an accepted registry change
set.

Domain facts may not be stored only in seed-local Postgres tables.

### 3. Runtime state must be durable

Operational state that is not registry truth must still be durable in kernel
runtime storage, typically Postgres.

Examples:

- sessions
- OTP challenges
- token hashes
- job leases
- web state
- delivery attempts

### 4. Projections must be disposable

If projection tables are dropped and rebuilt from registry plus runtime state,
no domain fact should be lost.

### 5. Production realizations must fail closed

If a realization in durable mode still has mutating command paths that write
only to memory or to non-registry fact tables, it must fail CI and eventually
fail startup in production mode.

---

## Canonical State Model

Every persisted thing in AS must belong to exactly one of these buckets.

### A. Registry Fact

Accepted durable history.

Examples:

- organization created
- show created
- show updated
- schedule created
- division added
- class reordered
- entry moved
- citation attached
- authority grant accepted
- invite created

Storage rule:

- append accepted change set to kernel registry

### B. Runtime Durable State

Durable operational state that is not registry truth.

Examples:

- user sessions
- login challenges
- access tokens
- runtime jobs
- temporary locks
- outbox delivery state

Storage rule:

- write to kernel runtime store

### C. Projection / Cache

Rebuildable current-state views.

Examples:

- public club cards
- show detail views
- leaderboard rows
- search documents
- fast lookup indexes

Storage rule:

- materialized from registry and runtime durable state

Anything that does not clearly fit B or C should default to A.

---

## Required Architecture Changes

## 1. Command Path Reform

All semantic commands that create or mutate domain facts must:

1. validate input
2. construct one accepted change set
3. append it to the kernel registry
4. let a materializer update projections
5. return identifiers and read-your-writes projection handles

They must not directly persist domain truth into seed tables.

## 2. Seed Store Reform

Seed-local stores must stop pretending to be authoritative durable stores for
domain objects.

Allowed seed storage roles:

- projection reader
- projection writer used by a materializer
- runtime helper for non-domain state only if explicitly approved

Disallowed seed storage roles:

- direct source of truth for accepted domain facts
- fallback mutable memory store in production

Naming rule:

- if it can mutate accepted facts, it must not be called a generic store unless
  it is the actual registry or runtime durable store
- in-memory helpers must be called caches, fixtures, or dev stores

## 3. Materializer Reform

Materializers must own projection writes.

They should:

- replay ordered registry rows
- build or rebuild seed projections
- support full rebuild from empty projection tables
- support incremental cursor-based updates

## 4. Durable Startup Rules

In durable mode, startup must verify:

- runtime database reachable
- registry append path reachable
- required materializers registered
- no blocked mutators remain on direct memory-only code paths

If not, startup should fail rather than run in a false-durable state.

---

## Enforcement Mechanisms

## 1. Static CI Audits

Add CI checks that fail if:

- any production Postgres-backed mutator returns `s.mem.create...`,
  `s.mem.update...`, `s.mem.delete...`, or equivalent
- realization handlers perform direct domain `INSERT`, `UPDATE`, or `DELETE`
  outside the registry/materializer/runtime boundary
- a command contract exists without a declared durability class
- a command that mutates domain facts is not wired through a registry append path

## 2. Restart-Survival Tests

For every mutating command:

1. create or mutate through the public command
2. tear down and rebuild app/store/materializer state
3. read the by-id projection
4. confirm the fact still exists

## 3. Projection Rebuild Tests

For every seed:

1. create several domain objects
2. drop or clear projection tables
3. replay registry
4. confirm all objects and relationships reappear correctly

## 4. Multi-Instance Consistency Tests

For every interactive seed:

1. write through instance A
2. read through instance B
3. restart both
4. read again
5. confirm no divergence

## 5. Contract-Level Acceptance Gates

Interactive seeds should fail acceptance if:

- accepted writes cannot survive restart
- accepted writes cannot be replayed from registry
- mutable production paths still rely on memory-only authority

---

## Repo-Wide Migration Program

## Phase 1. Emergency Containment

Goal:

- stop further data loss

Actions:

- identify every memory-only production mutator across all seeds
- block new feature work that adds direct seed-local fact persistence
- patch the highest-risk active seed paths first
- publish the durable-state doctrine in kernel and template docs

Exit criteria:

- every currently exposed production mutation path is classified
- every memory-only production mutator is listed and tracked

## Phase 2. Flowershow Durability Repair

Goal:

- make the active flower-show realization restart-safe for all existing command
  surfaces

Actions:

- patch all Flowershow mutators currently delegating to memory
- add hydration for every durable projection table currently used
- add restart-survival coverage for every published command
- add multi-instance consistency coverage for the live app
- recover or replay lost live data from any available external source

Exit criteria:

- no published Flowershow mutator is memory-only
- a reboot does not lose accepted writes
- multi-instance reads converge

## Phase 3. Registry-First Domain Writes

Goal:

- move from “durable seed-local Postgres” to “registry-first accepted facts”

Actions:

- define canonical accepted change set shapes for seed domain facts
- add kernel append interfaces usable by realization commands
- implement Flowershow append path for:
  - organizations
  - shows
  - schedules
  - divisions
  - sections
  - classes
  - entries
  - citations
  - credits
  - invites
  - authority grants
- materialize seed projection tables from registry rows

Exit criteria:

- dropping projection tables and replaying registry recovers all supported facts

## Phase 4. Template and Seed Enforcement

Goal:

- prevent future seeds from drifting back to direct fact storage

Actions:

- update the seed template with explicit registry/runtime/projection boundaries
- require durability classification in every new interaction contract
- add CI policy checks repo-wide

Exit criteria:

- new seeds cannot ship without registry-first mutation design

---

## Flowershow Emergency Recovery Track

Because Flowershow already lost live writes, there must be a separate recovery
track for existing data.

### Recovery Sources

Possible recovery sources:

- external agent payloads that created the records
- source PDFs or schedules
- browser/API transcripts
- uploaded documents already stored durably
- any partial SQL rows that did persist

### Recovery Procedure

1. inventory lost objects by organization and show
2. identify whether any durable references still exist
3. reconstruct from source docs or agent payloads
4. replay through the repaired command path
5. verify through by-id projections and UI surfaces

### Recovery Rule

No production restart should occur again on a seed with known memory-only
mutators until recovery and repair status is understood.

---

## Registry Integration Requirements

To stop Postgres-only truth from creeping back in, implement these rules:

### Rule 1

If losing projection tables and rebuilding from registry would lose the fact,
the fact was stored in the wrong place.

### Rule 2

Seed domain tables are projections, not authoritative truth tables.

### Rule 3

Realization command handlers may write:

- registry change sets for domain facts
- runtime durable state for operational control

They may not directly author domain truth in seed SQL tables.

### Rule 4

Registry append and replay must be observable:

- cursor position
- last applied change set
- replay lag
- rebuild status

---

## Allowed Exceptions

Exceptions are narrow and explicit.

### Allowed

- request-local working memory
- parsed template caches
- contract metadata caches
- SSE subscriber lists
- websocket connection state
- short-lived read-through caches whose loss does not lose truth

### Not Allowed

- domain object writes only in memory
- auth or permission truth only in memory
- accepted mutation history only in memory
- seed-local Postgres fact writes that bypass registry

Any proposed exception must prove:

- loss of the process does not lose truth
- multi-instance routing does not create divergence
- the exception is operational rather than domain truth

---

## Required Documentation Changes

Update and keep aligned:

- `kernel/protocol/v1/registry.md`
- `kernel/protocol/v1/permissions.md`
- `kernel/protocol/v1/AUTOSOFTWARE_AGENT_PRINCIPLES.md`
- `seeds/9999-template/*`
- active seed acceptance docs

Each interactive seed should explicitly declare:

- which data is registry fact
- which data is runtime durable state
- which tables are projections
- which rebuild tests prove correctness

---

## Acceptance Criteria

This plan is complete only when all of the following are true:

1. No production seed mutator stores accepted facts only in memory.
2. No production seed uses seed-local Postgres tables as the sole source of
   durable domain truth.
3. All domain-fact commands append accepted change sets to the registry.
4. All projections can be rebuilt from registry plus runtime durable state.
5. Restart-survival tests exist for all published mutating commands.
6. Multi-instance consistency tests exist for active interactive seeds.
7. CI blocks new memory-only or direct fact-table mutation paths.
8. Flowershow lost-data recovery is completed or explicitly documented as
   unrecoverable by source.

---

## Immediate Execution Order

1. Audit every current production mutator for memory-only or projection-only
   truth writes.
2. Patch Flowershow durable persistence gaps to stop ongoing loss.
3. Add CI audit for memory-only Postgres mutators.
4. Add restart-survival tests for the repaired Flowershow command set.
5. Design and implement registry-first append path for Flowershow domain facts.
6. Convert Flowershow seed tables into explicit projections.
7. Roll the same enforcement into the seed template and other active seeds.

---

## One-Sentence Summary

AS must treat the registry as the only durable home for accepted domain facts,
runtime storage as the durable home for operational state, and Postgres plus
memory as rebuildable projections and caches rather than hidden sources of
truth.
