# Kernel Decision Log

This document records stable kernel decisions and the reason they were made.

Boundary:

- record durable design choices and tradeoffs
- explain why a boundary exists
- do not describe runtime procedures here
- do not duplicate architecture walkthroughs here
- do not turn this into a changelog

## Current Decisions

### 1. The kernel and the evolving app layer are separate

The kernel stays generic and trusted.
The app evolves through seeds in `seeds/`.

This keeps the mechanism stable while allowing application behavior to change.

### 2. `genesis/` is the canonical first seed

The founding truth is represented using the same seed shape as later changes.
`seeds/0000-genesis` is only a linked authoring view.

This keeps the model uniform from the first change onward.

### 3. Seeds are source intent; realizations are compiled intent

Humans and agents work on seeds.
AI coding runs produce realizations from those seeds.

This keeps authored intent separate from concrete implementation output.

### 4. Change sets are emitted from accepted realizations

The registry stores accepted changes as change sets and rows derived from
accepted realizations.

This keeps the accepted runtime truth tied to a concrete reviewed output.

### 5. PRs review realizations, not raw seeds

A seed may produce multiple realizations over time.
A PR should normally correspond to one realization of one seed.

This makes GitHub the review surface for compiled intent.

### 6. Runtime pins realizations, not unresolved seeds

Boot and materialization should always target a selected realization.
If a seed is selected, the system must resolve it to a realization by explicit
policy.

This keeps runtime deterministic while still allowing seed-level entry points.

### 7. Canonical kernel docs live under `kernel/`

Kernel structure, rationale, and operations are documented close to the code.
Top-level docs should explain the whole system, not repeat kernel internals.

### 8. The generic top-level `docs/` layer is removed for now

Until there is real cross-cutting documentation that cannot live in
`genesis/`, the root README, or `kernel/`, a generic docs area is more
confusing than helpful.

### 9. The generic top-level `testdata/` layer is removed for now

Kernel fixtures should live with kernel code when they become real.
Seed-specific validation inputs should live with the seed that uses them.

This keeps test structure attached to responsibility instead of creating a
catch-all root bucket.

### 10. Feedback-loop signals stay in kernel runtime storage, not the registry

Browser crashes, HTMX failures, request traces, test runs, and agent review
artifacts are part of the development loop, not accepted product history.

The kernel should collect them in runtime storage and tie them to request IDs,
seed IDs, and realization IDs so agents can shorten the loop without polluting
the append-only registry.
