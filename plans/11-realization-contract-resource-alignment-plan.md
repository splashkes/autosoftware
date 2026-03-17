# Realization and Contract Resource Alignment Plan

## Purpose

This plan resolves the naming drift between `realization` and `contract`
across the registry API, reading-room UI, protocol docs, and future canonical
resources.

The goal is to stop teaching two different mental models for the same thing.
Today the system mostly has one resource with two names. That is manageable in
the short term, but it will become expensive once registry instances, version
history, and long-lived external clients depend on the surface.

---

## Problem

The live system currently mixes two terms:

- the registry API exposes `realization` resources
- the reading room uses `contracts` as the primary human label for the same
  detail page
- the browser accepts both `/realizations/...` and `/contracts/...`
- the main human-facing artifact inside a realization is
  `interaction_contract.yaml`

That creates three kinds of ambiguity:

1. People cannot tell whether a realization and a contract are actually the
   same resource.
2. Agents and external clients cannot tell which term is canonical for links,
   API traversal, and future automation.
3. The system has no clean place to grow if contracts ever become truly
   first-class resources instead of just one important artifact inside a
   realization.

---

## Current State

### What is actually canonical today

Today the registry resource is a realization.

It has:

- a stable `reference` such as `0004-event-listings/a-web-mvp`
- realization metadata such as status, approach, surface kind, and summary
- the operational contract materialized from `interaction_contract.yaml`
- linked commands, projections, objects, relations, and governing schemas

The current API shape reflects that:

- `GET /v1/registry/realizations`
- `GET /v1/registry/realization?reference=...`

### Where the drift appears

The reading room currently presents that same resource under `contracts`:

- navigation label: `Contracts`
- canonical browse path: `/registry/reading-room/contracts/...`
- handler aliases both `/realizations/` and `/contracts/`

This was understandable when the most important human concern was "what
contract does this app expose?" but the resource has already grown beyond a
pure contract document.

---

## Recommended End State

The recommended end state is:

- `realization` is the canonical resource term across protocol, API, and UI
- `interaction_contract.yaml` is the canonical operational-contract artifact
  inside a realization
- `contract` remains a useful explanatory word, but not the primary resource
  identity

This aligns best with the larger system model:

- a seed is source intent
- a realization is compiled intent
- a realization carries an operational contract
- accepted change sets are derived from accepted realizations

Under this model:

- the registry resource is the realization
- the contract is a structured promise or artifact of that realization
- commands, projections, objects, and graph relations are aspects of the
  realization detail surface

---

## Why This Direction Is Better

### 1. It matches the architecture already in use

The API, internal catalog, and discovery model already treat realizations as
the resource. Renaming the API to `contract` would throw away coherence for the
sake of a narrower human label.

### 2. It leaves room for first-class contracts later

If the system later needs separate contract resources, there should be a real
reason, such as:

- multiple contract revisions attached to one realization
- signed contract bundles distinct from implementation metadata
- contract comparison across realizations as its own resource family

Those are valid future directions, but they should create genuinely separate
resources, not reuse the current realization identity with a second name.

### 3. It makes the human and machine surfaces agree

A user should be able to read a page, copy its API route, and get the same
resource concept back.

The current state almost does that, but the labels still suggest:

- "this page is a contract"
- "this API resource is a realization"

That mismatch is unnecessary.

---

## Canonical Naming Rules

The system should adopt these rules:

### Resource terms

- `realization`: the canonical registry resource
- `command`: a public write surface exposed by a realization
- `projection`: a public read surface exposed by a realization
- `object`: a seed-local governed thing
- `relation`: a graph edge declared by a realization contract
- `schema`: a governing schema reference

### Artifact terms

- `interaction_contract.yaml`: the canonical operational-contract artifact
- `runtime.yaml`: the canonical launch artifact
- realization-local `README.md`, validation docs, and code artifacts remain
  artifacts, not standalone registry resources by default

### Human wording

- "operational contract" is acceptable as a section or explanatory phrase
- "contract page" is acceptable informally only if the page is clearly labeled
  as realization detail
- `Contracts` should not remain the primary top-level navigation label once the
  rename is executed

---

## URL and API Policy

### Canonical URLs

Canonical reading-room URLs should eventually use realization language:

- `/registry/reading-room/realizations/{seed_id}/{realization_id}`

API routes should remain:

- `GET /v1/registry/realizations`
- `GET /v1/registry/realization?reference=...`

### Compatibility URLs

The older `contracts` paths should be treated as compatibility aliases:

- `/registry/reading-room/contracts/{seed_id}/{realization_id}`

Compatibility behavior should be:

- short term: serve successfully
- medium term: redirect to the realization route
- long term: keep only if external link stability demands it

If redirects are used, they should preserve content-hash permalinks too.

### Payload wording

The realization detail payload should keep the top-level key `realization`.

If helpful, it may add clearer subordinate fields such as:

- `contract_artifact_ref`
- `contract_schema_version`
- `contract_summary`

But it should not rename the resource itself to `contract`.

---

## UI Changes

The reading room should be updated to present realization detail as realization
detail.

Recommended changes:

1. Rename the nav item from `Contracts` to `Realizations`.
2. Rename page headings and breadcrumb language from contract-centric wording
   to realization-centric wording.
3. Keep a clearly labeled section for:
   - operational contract
   - auth modes
   - capabilities
   - commands
   - projections
   - graph relations
4. Keep the contract artifact link visible and explicit.

The important distinction is:

- the page is about the realization
- one major section of the page is its operational contract

---

## Documentation Changes

The following docs should be aligned as part of this plan:

- `kernel/protocol/v1/realizations.md`
- `kernel/protocol/v1/interactions.md`
- `README.md`
- `seeds/README.md`
- `EVOLUTION_QUICK.md`
- `EVOLUTION_INSTRUCTIONS.md`

Key doc rules:

- explain that the realization is the compiled resource
- explain that `interaction_contract.yaml` is the operational promise of a
  realization
- stop using `contract` and `realization` as interchangeable nouns unless the
  sentence is explicitly describing the contract artifact

---

## Phase Plan

### Phase 1: Terminology Audit

Inventory all usages of:

- `contract`
- `contracts`
- `realization`
- `realizations`

Classify each usage as one of:

- canonical resource name
- artifact name
- human shorthand
- compatibility alias

Success criteria:

- every use of the words is intentional
- drift is identified before renaming anything

### Phase 2: Canonical Spec Update

Fill in `kernel/protocol/v1/realizations.md` and align the surrounding docs.

Define:

- what a realization resource is
- how it relates to approaches and accepted change sets
- how the operational contract fits inside it
- when a separate contract resource would be justified in the future

Success criteria:

- the protocol docs teach one canonical model
- future UI and API work has a stable spec target

### Phase 3: API Clarification

Keep the existing registry API resource names, but clarify them in:

- discovery metadata
- response fields
- tests

Optionally add explicit artifact-level fields on realization detail to point at
the contract file without implying a second resource identity.

Success criteria:

- agents can traverse realization resources without guessing terminology
- the API does not imply that `contract` is a separate resource when it is not

### Phase 4: Reading-Room Rename

Update the reading room to use realization-first naming.

Work includes:

- navigation rename
- route rename
- heading and copy updates
- compatibility aliases or redirects from `/contracts/...`

Success criteria:

- the UI and API describe the same resource with the same primary term
- old links continue to work during migration

### Phase 5: Validation and Link Hygiene

Validate:

- live reading-room pages
- linked API routes
- canonical and permalink behavior
- compatibility redirects or aliases

Also audit other generated links so the older contract-path shape is not
reintroduced accidentally.

Success criteria:

- copied UI links and copied API routes refer to the same conceptual resource
- external clients do not need private naming knowledge to navigate

---

## Non-Goals

This plan does not by itself:

- create first-class versioned contract resources
- redesign the realization catalog model
- change the semantics of commands, projections, objects, or schemas
- remove compatibility aliases immediately

Those are separate choices.

---

## Decision Rule

If there is still disagreement during implementation, use this rule:

- if the thing being described is the compiled app-level resource, call it a
  realization
- if the thing being described is the promise expressed in
  `interaction_contract.yaml`, call it the operational contract

Do not use `contract` as the primary name for the realization resource unless
the protocol is intentionally changed to make contracts first-class resources
with their own identity and lifecycle.

---

## Execution Order

1. Add this plan.
2. Fill `kernel/protocol/v1/realizations.md`.
3. Align the main docs and evolution docs.
4. Rename the reading-room primary language and canonical paths to
   realization-first terms.
5. Keep `contracts` as a temporary compatibility alias.
6. Validate live UI and API parity after deployment.
