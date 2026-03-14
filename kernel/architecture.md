# Kernel Architecture

This document explains what the kernel is and how its major parts fit
together.

Boundary:

- describe structure and responsibilities
- describe component relationships
- do not record decision rationale here
- do not put operational step-by-step procedures here
- do not restate protocol specs line-by-line

## Role

`kernel/` is the trusted machinery of AS.

It is responsible for:

- validating accepted changes
- appending them to the registry
- verifying artifacts
- enforcing permission rules
- materializing current state
- serving web and API interfaces
- capturing local and preview feedback-loop signals that shorten the
  development loop

## Main Runtime Components

- `cmd/registryd/` is the registry-serving authority for one logical registry.
- `cmd/materializerd/` replays accepted history and builds current state.
- `cmd/webd/` serves the human-facing web surface.
- `cmd/apid/` serves the machine-facing API surface.

## Internal Packages

- `internal/registry/` owns append, ordering, idempotency, and ledger access.
- `internal/authz/` owns grants, scopes, and pre-state permission checks.
- `internal/artifacts/` owns fetch, decrypt, canonicalize, hash, and verify.
- `internal/boot/` owns kernel-only boot surfaces, including preview
  feedback-loop script injection.
- `internal/materializer/` owns replay and materialization flow.
- `internal/interactions/` is the shared use-case layer for web and API.
- `internal/projections/` holds shared read-model logic.
- `internal/feedback_loop/` owns structured runtime incidents, test runs, and
  agent review records for the feedback loop.
- `internal/http/` holds transport-specific server code, including request
  correlation and feedback-loop ingest handlers.

## Adjacent Areas

- `genesis/` is outside the kernel and defines founding truth.
- `seeds/` is outside the kernel and contains evolving app changes.
- `materialized/` is outside the kernel and contains local hydrated outputs.
- `protocol/v1/` contains the normative protocol drafts the kernel should
  enforce.
