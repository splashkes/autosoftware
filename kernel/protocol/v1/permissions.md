# Authority, Permissions, and Delegation

This is the living protocol note for system-native authority in Autosoftware.

It explains how identity from external auth providers should connect to
registry-backed authority, how delegation should work, and how current
effective access should be materialized.

This document is intentionally cross-seed.
Seeds should define their local subject kinds, scopes, capability bundles, and
operational rules without reinventing the kernel model.

## Purpose

Autosoftware needs a model where:

- external authentication proves who is acting
- internal registry history decides what that actor may do
- authority can be delegated and revoked without losing history
- UI clients, remote agents, and operator tools all hit the same semantic
  command surface
- effective access is derived, inspectable, and reproducible

This document is about that model.

## Core Rule

Identity and authority are separate.

- auth providers establish identity
- the Autosoftware registry establishes authority
- current access is derived from accepted authority history

Do not treat Cognito groups, session cookies, or seed-local tables as the
authoritative long-term permission model.
Those may help bootstrap or bridge the system, but the durable platform model
is registry-native.

## Non-Goals

This document does not define:

- a specific OAuth or Cognito setup
- UI wireframes
- seed-specific role names
- low-level storage table design
- every possible policy combinator

It defines the conceptual and protocol boundary that seeds and realizations
should share.

## Terms

### Identity

The externally authenticated actor.

Examples:

- a human who signed in with Cognito
- a service authenticated by bearer token
- a remote agent acting under an operator-owned service identity

Identity answers:

- who is making the request

It does not answer:

- what that requester is allowed to do

### Subject

The stable internal principal object used by the system for authority.

Typical subject kinds:

- `person`
- `service`
- `agent`
- `group`
- `office`

Subjects are what grants attach to.

### Scope

The region of authority where a capability applies.

Typical scope kinds:

- `platform`
- `organization`
- `show`
- `division`
- `section`
- `class`
- `entry`
- `document`
- `workflow`

### Capability

A concrete permission, not a vague label.

Examples:

- `organization.member.read_private`
- `organization.executive.manage_roster`
- `show.judge.assign`
- `show.entry.score`
- `show.publish`
- `authority.grant`

Capabilities should be fine-grained enough to evaluate commands directly.

### Bundle

A named collection of capabilities.

Examples:

- `organization_admin`
- `show_judge`
- `show_editor`

Bundles are convenience vocabulary.
The evaluator should resolve them into concrete capabilities.

### Grant

An accepted authority record saying one subject grants a bundle or capability
set to another subject over a declared scope.

### Delegation

The rule that determines whether an effective grant holder may create a
further grant for another subject.

### Effective Access

The current derived answer to:

- may subject S perform capability C on scope X right now

## Why This Belongs In The Registry Model

The registry is already the append-only source of accepted history.

Authority fits naturally in that model because:

- grants should be auditable
- revocations should not erase earlier acceptance
- current access should be replayable from accepted rows
- agents and operators should be able to inspect how authority emerged

That means authority history should follow the same pattern as other accepted
system truth:

- object identity is stable
- claims are append-only
- supersession and revocation are explicit
- current state is materialized from accepted history

## Authority Objects And Claims

The kernel-level authority model should stay small and generic.

Suggested first-class authority-related objects:

- `subject`
- `scope`
- `capability_bundle`
- `authority_grant`

Suggested first-class claim shapes:

- subject identity binding
- bundle definition
- grant acceptance
- grant revocation
- grant supersession
- grant expiry
- delegation constraint

The kernel should not need a seed-specific row taxonomy for every role shape.
Most meaning belongs in claims interpreted through explicit schemas.

## Recommended Grant Shape

An authority grant should be able to express at least:

- `grant_id`
- `grantor_subject_id`
- `grantee_subject_id`
- `bundle_id` or explicit capability set
- `scope_type`
- `scope_id`
- `delegation_mode`
- `basis`
- `status`
- `effective_at`
- `expires_at`
- `supersedes_grant_id`
- `reason`
- `evidence_refs`

Optional but valuable:

- `approval_required`
- `accepted_by_subject_id`
- `max_delegation_depth`
- `allowed_capability_subset`
- `narrower_scope_only`

## Grant Lifecycle

Recommended lifecycle states:

- `proposed`
- `accepted`
- `rejected`
- `revoked`
- `expired`
- `superseded`

Not every asserted grant should become effective immediately.

The key distinction is:

- the registry may record a proposal or revocation event
- the materialized effective-access view should expose only accepted,
  non-revoked, non-expired authority

## Revocation And Supersession

Do not model revocation by mutating a grant row in place.

Do not model revocation by inventing role strings such as:

- `REVOKED executive`

Instead:

- append a later accepted revocation or superseding grant claim
- preserve the earlier accepted grant in history
- derive the current state by replay

This keeps the ledger intact while allowing current access to change cleanly.

## Delegation Model

Delegation must be explicit.

A subject should not be able to create downstream grants merely because it has
some powerful capability.
Its effective authority should also permit delegation.

Recommended delegation modes:

- `none`
- `same_scope`
- `narrower_scope`
- `subset_only`

Examples:

- a club admin may grant organization-scoped roles inside that same club
- a show editor may grant only show-scoped contributor roles inside that one
  show
- a judge may score entries but may not delegate judge power

Delegation should always be checked against current effective access, not
against stale snapshots.

## Bundles Versus Capabilities

Use capabilities as the evaluation primitive.

Use bundles as the human-facing and seed-facing vocabulary.

This means:

- commands should ultimately require concrete capabilities
- bundles expand into one or more capabilities
- seeds can expose user-friendly labels without weakening evaluation clarity

For example:

- bundle `organization_executive`
- capabilities:
  - `organization.member.read_private`
  - `organization.executive.manage_roster`
  - `show.create`

## Materialization

The kernel materializer should derive effective access from accepted authority
history.

Conceptually:

1. read accepted authority rows in registry order
2. create stable objects as needed
3. expand accepted bundles into capability sets
4. remove revoked, expired, or superseded grants
5. enforce delegation constraints
6. build effective-access projections by subject and by scope

The materializer should not guess seed semantics.
It should resolve through explicit schema and bundle definitions.

## Authorization Evaluation

Normal command execution should not append raw grant claims directly from
external clients.

Instead:

1. the caller authenticates
2. identity resolves to a stable `subject_id`
3. the command contract declares the capability it needs
4. the seed resolves the relevant scope from command input
5. the kernel evaluator answers allow or deny
6. if allowed, the command runs
7. resulting accepted change sets append to the registry

This preserves the rule from `interactions.md` that normal callers use
semantic commands rather than direct claim-writing.

## Kernel Versus Seed Responsibility

### Kernel Responsibilities

- subject identity binding model
- accepted authority change-set and claim interpretation
- bundle expansion
- delegation evaluation
- effective-access materialization
- authority history and effective-access projections
- common structured error semantics for denial

### Seed Responsibilities

- declare subject kinds when seed-local ones matter
- declare supported scope kinds
- define bundle vocabulary
- define command-to-capability requirements
- resolve command input into a concrete scope target
- define any seed-local approval or review rules

The kernel should know how to evaluate authority.
The seed should know what authority means in its domain.

## Contract Implications

Interactive contracts should be able to declare:

- supported subject kinds
- supported scope kinds
- declared bundles
- command capability requirements
- authority-sensitive projections

Seeds do not need to expose raw registry authority writes as their public API.
They should expose semantic authority commands such as:

- `authority.grants.propose`
- `authority.grants.accept`
- `authority.grants.revoke`
- `authority.grants.list`

The contract should also expose projections such as:

- effective access by subject
- effective access by scope
- authority ledger by subject
- authority ledger by scope

## Error Model

Permission denial should be structured and useful.

At minimum:

- `code`
- `message`
- `required_capability`
- `resolved_scope`
- `subject_id`
- `request_id`

When safe for the caller:

- denial basis
- missing delegation reason
- expiry reason
- related authority projection link

Do not return only a bare `403 forbidden`.

## Bootstrap

There still has to be an origin for authority.

Bootstrap should be:

- explicit
- narrow
- reviewable
- transitional

Recommended rule:

- bootstrap creates one or a few initial accepted grants
- everything after that uses the normal semantic authority commands and
  registry history

Bootstrap is the exception.
It should not become the permanent permission path.

## Identity Binding

The system needs a durable mapping from external auth identity to internal
subject.

That mapping should itself be modeled, not hidden.

Typical flow:

- Cognito `sub` authenticates
- runtime resolves or creates a stable internal subject binding
- authorization runs on `subject_id`

This keeps the platform free to change auth providers without losing authority
history.

## Durable Browser Sessions

Browser login persistence is a runtime concern, not a registry concern.

The durable browser-session model should be:

- the browser stores only an opaque session handle cookie
- kernel runtime storage holds the session row
- the session row points at the stable internal subject
- authority is resolved from durable internal state, not from cookie contents

Recommended runtime records:

- `runtime_principals`
- `runtime_principal_identifiers`
- `runtime_auth_identities`
- `runtime_sessions`

Recommended browser behavior:

- cookie value is a random opaque `session_id`
- cookie does not embed full identity or permission payload
- session expiry is enforced from runtime state
- logout revokes or ends the runtime session row
- reboot or redeploy does not invalidate valid sessions merely because a
  process-local secret changed

This means a normal reboot path should be:

1. browser sends the same session cookie
2. app or kernel resolves `session_id` from runtime storage
3. session resolves to `subject_id`
4. effective access resolves from durable authority data
5. the user remains signed in with the same current permissions

If any one of those layers is volatile, reboot persistence is incomplete.

## Runtime Secrets Versus Registry Truth

Do not store session master secrets, auth-provider client secrets, or other
live secret material in the public registry.

The registry is for accepted truth and replayable history.
Secrets are runtime control material.

The recommended split is:

- registry or contract config for non-secret declarative auth/provider shape
- runtime secret storage for sensitive provider credentials and cookie-sealing
  or envelope-encryption material
- runtime Postgres for live sessions, auth bindings, and short-lived auth
  workflow state

Encrypted blobs in the registry do not remove the need for a runtime root of
trust.
They usually just move the secret-management problem elsewhere.

## Multi-App Auth Provider Shape

Autosoftware should assume that many apps may use different auth providers or
different Cognito pools and clients.

The stable cross-app rule is:

- provider config identifies where identity came from
- `runtime_auth_identities` binds provider identity to internal subject
- `runtime_sessions` records the authenticated browser or agent session
- authority remains system-native and provider-neutral

That allows:

- one subject to authenticate through multiple providers over time
- many apps to share kernel runtime session and identity machinery
- seeds to keep provider-specific login UX without making the provider the
  authority model

## Flower-Show Adoption Note

`0007-Flowershow` is the first live seed adopting the durable-session side of
this model.

Its current bridge shape is:

- Cognito still proves identity
- flower-show browser sessions are stored as opaque runtime-backed sessions
- flower-show role state is persisted in Postgres instead of memory
- current role evaluation still includes a seed-local bridge while the full
  kernel-native authority ledger materialization continues

That is an acceptable transition step because it improves durability and
separates provider identity from browser-session persistence without claiming
that the full authority migration is complete.

## Seed Guidance

If a seed models membership, office, or operational control, it should make
three distinctions clear:

- membership
- office
- operational authority

Sometimes one implies another.
Often it should not.

Seeds should not collapse those into one vague `role` string unless the seed
is truly trivial.

## Flower-Show Example Vocabulary

The flower-show seed is a good exemplar because it has both club-level and
show-level control.

Natural bundle vocabulary includes:

- `organization_member`
- `organization_executive`
- `organization_admin`
- `show_editor`
- `show_judge`
- `show_steward`
- `show_scoring_operator`
- `show_awards_operator`

Examples:

- admin at club
- judge at show
- member at club
- executive at club
- revoked executive at club

These should all be modeled as scoped authority history plus current derived
effective access, not as ad hoc seed-local booleans.

## Template Expectations

Seeds that need authority should document:

- subject vocabulary
- scope vocabulary
- bundle vocabulary
- grant lifecycle rules
- delegation rules
- command capability requirements
- public versus private authority projections

The template should require these fields whenever a seed has meaningful
authority semantics.

## Current Repository State

Today some realizations still use seed-local role tables or deployment
overrides.

That is acceptable only as an interim bridge.

The intended long-term direction is:

- auth provider proves identity
- registry history proves authority
- kernel materialization derives current access
- seeds declare semantics through contracts and docs

## Living-Doc Rule

This document is expected to evolve.

When it changes:

- update the template expectations
- update relevant seed overlays
- update execution plans
- keep the kernel/seed boundary explicit
