# AS Plan: Kernel-Native Authority, Delegation, and Effective-Access Materialization

## Purpose

Autosoftware currently has the right conceptual pieces for append-only
accepted history, semantic commands, and materialized projections, but it does
not yet have a canonical kernel-native authority model.

Some realizations still decide permissions through seed-local role tables,
deployment overrides, or auth-provider-adjacent assumptions.

That is not durable enough for:

- delegated control between humans and agents
- auditable grant and revoke history
- cross-seed authority consistency
- effective-access inspection by UI, operators, and remote agents

This plan closes that gap.

It should be read together with:

- `kernel/protocol/v1/permissions.md`
- `kernel/protocol/v1/registry.md`
- `kernel/protocol/v1/claims.md`
- `kernel/protocol/v1/interactions.md`
- `seeds/9999-template/README.md`
- `seeds/0007-Flowershow/design.md`

## Goal

Make authority in Autosoftware work like this:

- external auth proves identity
- internal subject bindings stabilize principals
- accepted registry history records authority grants, revocations, and
  delegation
- the kernel materializes current effective access
- seeds declare capability and scope semantics
- commands ask the kernel whether the acting subject is allowed

## Non-Negotiable Constraints

### 1. Identity And Authority Stay Separate

Auth providers may prove who the caller is.
They should not remain the canonical source of long-term authority.

### 2. Authority Is Accepted History

Grants, revocations, supersession, and expiry should be recorded as accepted
history, not hidden mutable table state.

### 3. Normal Clients Use Semantic Commands

External callers should not append raw registry authority rows directly.
They should call semantic authority commands.

### 4. Effective Access Is Derived

The current allow/deny answer should come from kernel materialization over
accepted history, not from ad hoc seed-local checks.

### 5. Seeds Still Define Semantics

The kernel evaluates authority generically.
Seeds still define:

- scope kinds
- capability vocabulary
- bundle vocabulary
- command capability requirements

## Deliverables

### Kernel Deliverables

- canonical subject-binding model
- canonical authority grant lifecycle model
- authority-related schema definitions
- authority materializer
- effective-access projections
- authority ledger projections
- kernel authorization evaluator used by commands

### Template Deliverables

- required authority section for seeds that model control
- required capability/scope documentation
- required contract declaration for authority-sensitive commands
- acceptance expectations for derived effective access

### Flower-Show Deliverables

- replace the current simple seed-local role path with kernel-native authority
  evaluation
- model club and show control through authority bundles and scopes
- expose authority ledger/effective-access views
- use flower-show as the exemplar seed for future authority-bearing seeds

## Phases

### Phase 0: Canonical Doctrine

Status target:

- complete the living kernel authority doc
- thread it into template and seed docs

Outputs:

- `kernel/protocol/v1/permissions.md`
- template updates
- flower-show design and acceptance updates

Exit condition:

- the repository has one canonical source for system-native authority

### Phase 1: Subject Binding And Authority Schema

Add kernel-native models for:

- external identity binding to stable subject objects
- scope object references
- capability bundle definitions
- authority grant lifecycle claims

Define:

- accepted schema versions for authority claims
- revocation and supersession rules
- minimum projection payloads

Exit condition:

- authority has explicit registry-native object and claim vocabulary

### Phase 2: Materializer And Effective-Access Projections

Build the kernel materializer support for:

- replaying accepted authority history
- expanding bundles into capabilities
- removing revoked, expired, and superseded grants
- enforcing delegation constraints

Expose projections such as:

- effective access by subject
- effective access by scope
- authority ledger by subject
- authority ledger by scope

Exit condition:

- the kernel can deterministically answer current effective authority from
  accepted history

### Phase 3: Command Authorization Integration

Add the kernel-facing evaluator path:

- resolve acting `subject_id`
- resolve required capability
- resolve target scope
- authorize allow/deny through the effective-access view

Require interaction contracts to declare authority requirements for
authority-sensitive commands.

Exit condition:

- command authorization can be performed by the shared kernel model rather than
  by seed-local role checks

### Phase 4: Flower-Show Adoption

Adopt the canonical model in `0007-Flowershow`.

Map flower-show authority vocabulary:

- `organization_member`
- `organization_executive`
- `organization_admin`
- `show_editor`
- `show_judge`
- `show_steward`
- `show_scoring_operator`
- `show_awards_operator`

Replace:

- current seed-local role assignment as the source of truth
- auth-provider-adjacent shortcuts as the durable authority model

Preserve:

- semantic commands
- agent parity
- ledger visibility

Exit condition:

- flower-show becomes the first serious seed proving the authority model

### Phase 5: Template Enforcement And Wider Seed Adoption

Update contract-load and validation checks so seeds that model authority must
document and expose it correctly.

Add:

- template requirements
- validation rules
- example authority contract fragments

Exit condition:

- new seeds do not regress into undocumented local role logic

## Recommended First Execution Slice

The first real execution slice should be small enough to finish cleanly.

Recommended Slice A:

- define kernel-level subject binding shape
- define kernel-level authority grant schema
- define the first authority ledger projection
- define the first effective-access-by-subject projection
- document contract fields for capability and scope requirements

This slice avoids jumping straight into a full seed migration while still
making the model real.

## Proposed Command And Projection Family

Initial semantic commands:

- `authority.grants.propose`
- `authority.grants.accept`
- `authority.grants.revoke`
- `authority.subjects.bind_identity`

Initial projections:

- `effective-authority/subjects/{subject_id}`
- `effective-authority/scopes/{scope_type}/{scope_id}`
- `authority-ledger/subjects/{subject_id}`
- `authority-ledger/scopes/{scope_type}/{scope_id}`

These names may change, but the family should exist.

## Flower-Show Migration Notes

Flower-show should not start by modeling every nuance of every club office.

Start with:

- organization membership
- organization executive/admin control
- show editor/judge/scoring control
- revocation visibility

Only after that should it add narrower class or workflow authority if needed.

## Risks

### Risk 1: Recreating A Seed-Local Role Table In Kernel Clothing

Avoid a design where the kernel simply stores another mutable `roles` table.
That would miss the point of accepted history and materialization.

### Risk 2: Over-Coupling Seeds To One Global Role Vocabulary

The kernel should standardize mechanics, not force every seed to share the
same bundle names.

### Risk 3: Direct Registry Writes For Normal Clients

Do not make agents or UI clients append raw authority rows directly.
Keep semantic commands as the operational surface.

### Risk 4: Unbounded Delegation

Delegation must be explicit, scoped, and evaluable.
Otherwise authority sprawl will become impossible to reason about.

## Validation Strategy

Validation should happen at three levels:

1. docs and contract validation
2. kernel replay/materialization tests
3. live seed conformance tests

Concrete checks:

- subject binding is stable
- accepted grant history replays deterministically
- revocation and supersession remove current access but preserve history
- delegation rules are enforced
- projections expose enough provenance to explain allow/deny decisions
- seeds cannot bypass the kernel path for normal authority-sensitive commands

## Completion Condition

This plan is complete when:

- authority is modeled as accepted history in the kernel
- effective access is materialized centrally
- seeds declare semantics rather than reimplementing mechanics
- flower-show proves the model in a real authority-bearing seed
- the template prevents new seeds from drifting back to local ad hoc roles
