# Growth Contract

This document defines the kernel-level contract for moving a seed from docs and
draft realization definitions toward runnable software.

Boundary:

- define how draft or partial realizations are surfaced before they are runnable
- define the normalized seed packet used by both humans and agents
- define the growth command and job handoff into shared runtime state
- do not redefine the operational app contract for finished realizations

## Core Rule

Draft seeds should still be visible and inspectable before they are runnable.

The kernel must not pretend every visible realization can already be launched.
Instead it should expose:

- current readiness
- the seed packet that explains what exists
- the normalized command that asks for the next growth pass
- the runtime job record a worker can claim

This keeps the process legible for both humans and agents.

## Readiness Stages

The kernel currently uses these readiness stages:

- `Designed`: seed docs exist, but the realization is not yet normalized for
  growth
- `Defined`: docs and `interaction_contract.yaml` exist, but there is no
  runtime artifact yet
- `Runnable`: a runtime manifest exists and the realization can be launched
- `Accepted`: a runnable realization marked as accepted
- `Bootstrap`: a foundational inspection-only realization

Only `Runnable` and `Accepted` realizations should expose a real `Run` action.

## Seed Packet Projection

Endpoint:

- `GET /v1/projections/realization-growth/seed-packet?reference={seed_id}/{realization_id}`

The packet should include:

- selected realization identity and readiness
- seed summary and status
- seed docs such as `README.md`, `brief.md`, `design.md`, `acceptance.md`,
  `decision_log.md`, `notes.md`, and `seed.yaml`
- selected approach docs
- realization docs such as `README.md`, `notes.md`, `realization.yaml`, and
  `interaction_contract.yaml`
- runtime docs or artifacts when they already exist

The packet is the minimum context a future agent should need if it starts from
only the seed directory plus kernel specs.

## Growth Command

Endpoint:

- `POST /v1/commands/realizations.grow`

Command fields:

- `reference`
- `operation`: `grow`, `tweak`, or `validate`
- `create_new`
- `new_realization_id`
- `new_summary`
- `profile`: `minimal`, `balanced`, `ornate`, or `custom`
- `target`: `runnable_mvp`, `api_first`, `ux_surface`, or `validation_only`
- `developer_instructions`
- `idempotency_key`

Semantics:

- `grow` advances the realization toward a more complete state
- `tweak` makes a narrower follow-up pass on an existing realization
- `validate` requests review or verification without broad product changes
- `create_new=true` means the job should target a new realization variant
  instead of modifying the current one in place

The UI and any third-party tool should use this same command shape.

## Job Handoff

The growth command writes an entry into `runtime_jobs`.

Current queue rules:

- queue: `realization-growth`
- kinds: `realizations.grow`, `realizations.tweak`, `realizations.validate`

The job payload should include:

- request metadata such as request id, session id, principal id, and seed id
- source and target realization references
- chosen operation, profile, and target
- developer instructions
- the full seed packet
- a concise `prompt_brief`

This makes the queued job self-describing enough for a worker or agent to claim
without separately rediscovering repo state first.

## Worker Contract

A worker that claims a realization-growth job should:

1. load the embedded seed packet and prompt brief
2. read the referenced realization and seed docs from the repo
3. produce the requested growth, tweak, or validation pass
4. update the target realization or produce validation evidence
5. complete or fail the job with a clear result summary

The worker should treat the selected realization as the target of work, not the
seed in the abstract.

## Multiple Realization Variants

Profiles such as `minimal`, `balanced`, and `ornate` are not hidden global
styles.

If a seed needs durable variants, the growth flow should allow creating
multiple realization definitions so each variant has:

- its own `realization.yaml`
- its own `interaction_contract.yaml`
- its own artifacts and notes
- its own review and acceptance history

## Verification

The intended verification path is:

1. `go test ./...` to validate realization contracts and growth helpers
2. `./scripts/local-run.sh` to verify:
   - Postgres bootstrap
   - runtime service health
   - contract discovery
   - growth seed packet projection
   - growth job enqueue and job projection
   - materialization output
3. inspect `http://127.0.0.1:8090/` to confirm the console distinguishes
   `Inspect`, `Grow`, and `Run`

The critical rule is that the path from seed docs to growth work must stay
explicit, normalized, and testable.
