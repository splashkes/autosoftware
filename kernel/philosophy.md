# Kernel Philosophy

This document is for deeper conceptual ideas that shape the system but do not
belong in the main README, the kernel architecture, the decision log, or the
runbook.

Boundary:

- explain why certain Autosoftware mechanics are interesting or useful
- capture deeper conceptual patterns behind the kernel
- do not restate operational procedures here
- do not duplicate stable decisions line-by-line here
- do not turn this into a product manifesto

## Runtime Selection

Runtime should always boot from a selected realization.

A user, environment, or local test harness may select:

- a realization directly, or
- a seed, which must resolve to a realization by explicit rule

The default seed-selection rule is:

- use the latest accepted realization for that seed

This keeps runtime deterministic while still allowing seed-level entry points.

## Short Evolution Loop

Autosoftware should shorten the path from a failing interaction to an improved
realization.

The kernel should capture feedback-loop signals such as:

- browser crashes and unhandled promise rejections
- HTMX request failures
- request-level traces tied to the pinned realization
- local test runs
- agent review findings

Those signals belong in kernel runtime storage, not in the registry.
They should always be tied to request IDs, seed IDs, and realization IDs so a
coding agent can review what broke and fix the right compiled output.

## Working Discipline

Most work should happen in `seeds/`, usually by creating or refining a seed and
then producing or updating a realization for it.

Changes to `kernel/` should be rarer and should tighten one of:

- registry correctness
- permission validation
- artifact verification
- replay/materialization determinism
- transport or interface plumbing
