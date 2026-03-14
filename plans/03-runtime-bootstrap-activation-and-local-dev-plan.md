# AS Plan: Runtime Bootstrap, Activation, and Local Development

## Purpose

This document captures the practical runtime model for early AS and the contributor workflow that makes AI-assisted coding viable.

The seed runtime should be simple, fully runnable locally, and designed so the kernel can eventually stabilize while app-level evolution remains fast.

---

## Single-Owner Bootstrap

The initial AS instance should assume a single owner.

That means:
- no community governance required
- no voting model required
- no complex trust graph required
- one owner can propose, approve, and deploy mutations

Later systems may add community governance, but early systems should start with direct owner control.

---

## Seed Runtime Shape

The initial seed runtime should be a pair of Go services plus Postgres.

Components:
- BFF
- worker
- Postgres

Roles:
- BFF serves the app, stores sessions, accepts user intent, and creates new objects/claims locally
- worker syncs registry rows, resolves artifacts, verifies integrity, replays accepted changes, and materializes runtime state
- Postgres stores the local mirror, materialized state, sessions, and worker checkpoints

This is the first host organism.

---

## Postgres Responsibilities

Postgres should hold three broad categories of state:

### 1. Local mirror of accepted rows
- registry objects
- registry claims
- row/offset cursors

### 2. Materialized runtime state
- pages
- components
- routes
- forms
- active algorithms
- workflow configuration
- view models for HTMX rendering

### 3. Session and transient application state
- sessions
- session context
- temporary user interaction state
- worker checkpoints
- snapshots

The worker is the only writer of materialized state.
The BFF may write sessions and intent.

---

## Worker as Compiler of Current Reality

The worker should be described explicitly as the compiler of current reality.

On boot or update, it:
1. syncs accepted rows
2. resolves artifacts
3. decrypts if required
4. verifies hashes
5. replays relevant claims
6. materializes runtime state into Postgres

This materialized state is what the BFF serves.

The BFF should not interpret raw lineage on every request.

---

## HTMX and App Configuration

The worker should populate the major app configuration on boot and update.

Examples:
- HTMX-facing view models
- resolved component configs
- active CSS bundle references
- route definitions
- form definitions
- workflow state machines
- active algorithms

Prefer materializing structured view state over pre-rendering only final HTML.
That keeps the BFF flexible while preserving fast reads.

---

## Stable Kernel vs Fast-Evolving Surface

This principle should be elevated, not implied.

### Stable kernel
These should become boring and change slowly:
- registry sync
- artifact resolution
- hash verification
- worker replay engine
- materialization engine
- session plumbing
- basic render/runtime shell
- test harness

### Fast-evolving app surface
These should be easy to change:
- page definitions
- component configs
- CSS bundles
- workflow definitions
- algorithms
- schemas
- UI affordances
- seed structure and mutation tooling

The AI coding loop gets dramatically easier when the mutable surface is bounded.

---

## Local Dev World

Early supporters should be able to run the whole system locally.

A local dev world should include:
- local registry or registry mirror
- worker
- BFF
- Postgres
- artifact resolution
- fixture data
- replay and test harness

One command should bring up the full world.

The point is to let contributors and agents work against a functioning host program, not against an abstract spec.

---

## Local Mutation Workflow

Recommended loop:

1. start from a snapshot or current repo state
2. boot local dev world
3. make changes to artifacts/schemas/tests/etc.
4. simulate mutation locally
5. run replay and tests against fixture data
6. inspect diffs/screenshots/results
7. open PR when satisfied
8. cloud CI re-validates
9. merge
10. ingest and deploy

This is the bridge between local coding and cloud mutation.

---

## Mutation Simulation

A mutation should be testable before it becomes canonical.

The local simulator should be able to:
- apply proposed rows or mutation bundle to a temp ledger state
- run worker replay
- build runtime tables
- compare old vs new state
- run tests
- show render/config/algorithm diffs

This is the AS equivalent of the autoresearch loop: propose, simulate, score, iterate.

---

## Activation and Deployment

A version existing is not the same as a version being active.

The runtime should support explicit activation state.

Useful deployment modes:
- auto
- staged
- manual
- local_only

Examples:
- a new algorithm version may exist but not be active yet
- a new CSS version may be materialized but not selected
- a candidate view may be staged for preview before activation

Deployment should occur after:
- accepted rows are ingested
- worker replay succeeds
- activation rules allow the new state to become active

---

## Rollback and Pinning

Because lineage is append-only and activation is explicit, the system should support:
- rollback to a previous active realization/version
- pinning a system, tenant, or owner to a specific version/state
- staged candidate vs current active state

This should be documented as a first-class operational feature, not as a side effect.

---

## Boot UX

Before boot completion, the app may show a small plant/throbber with honest status lines.

Examples:
- syncing registry
- fetching artifacts
- decrypting private artifacts
- verifying hashes
- growing interface
- starting app

This is optional but conceptually aligned: the app is growing into current form.

---

## Design Rule to Carry Forward

The seed runtime should be small, fully runnable, and easy to replay.
The kernel should settle into a slow-moving compiler and execution shell.
Everything AI is likely to change often should live above that kernel boundary.
