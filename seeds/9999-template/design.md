*Quick `design.md` map: top = request interpretation plus the canonical graph and interface shape; middle = projections, commands, visibility, and version/provenance rules; bottom = boundaries, extensions, and rollback notes.*

# Design

Describe the local design of this seed.

This file is the first design response to the request in `brief.md`.
It should be good enough for the user to validate the direction before
implementation begins.
It should also narrow the set of viable approaches that later get named in
`approaches/`.

In a finished seed, put the interesting product-specific answer first.
Use the rest of this file to clarify and constrain that answer, not to delay
it.

Boundary:

- explain what this seed is changing
- explain what remains outside the scope of this seed
- describe affected objects, relations, claims, artifacts, interfaces, and
  projections
- make the intended command and projection surface clear enough to encode in
  `realizations/<id>/interaction_contract.yaml`
- explain what "agent equal or better than human access" means for this seed
- when the seed has meaningful control or membership semantics, define the
  subject, scope, capability, bundle, grant, and delegation vocabulary using
  the kernel authority model from `kernel/protocol/v1/permissions.md`
- define the canonical graph before describing flattened convenience payloads
- identify which projections are public, which are private, and whether any
  public surface is metadata-only or digest-only
- classify the object data as `shared_metadata`, `public_payload`,
  `private_payload`, or `runtime_only`
- call out relation visibility when it differs between audiences
- call out the stable object identity that alternate clients should use if the
  UX also exposes a public handle or slug
- explain version and provenance rules for content-bearing objects
- explain snapshot-versus-reference rules when related objects can change later
- explain whether commands accept any runtime-only authoring context for agent
  guidance, and make clear that it is not canonical shared truth
- explain the expected authenticated error shape when alternate clients need to
  recover from validation or permission failures
- make the early design checkpoint explicit so time is not wasted on the wrong
  implementation path
- identify the named approaches that deserve realization
- do not restate general kernel architecture here
- move durable yes/no tradeoff records to `decision_log.md`

Suggested section order:

- request interpretation
- canonical graph
- versions and provenance
- public/private boundary
- authority and delegation
- agent access and runtime-only context
- early design check
- approach candidates
- scope
- affected surfaces
- data and claim changes
- interface changes
- artifact changes
- rollback or supersession notes

## Canonical Graph

Describe the stable node kinds and relation kinds this seed introduces or
changes.

## Versions and Provenance

Describe which content-bearing objects have immutable versions, who created or
accepted them, and whether any historical snapshot rules matter.

## Authority and Delegation

If the seed has meaningful permission, control, membership, office, or
delegation behavior, describe:

- the stable subject kinds
- the stable scope kinds
- the capability or bundle vocabulary
- which commands require which capabilities
- whether grants can be delegated, narrowed, revoked, or time-boxed
- whether authority history must be visible through ledger or effective-access
  projections

## Data and Claim Changes

Describe the concrete object, relation, or claim-shape changes introduced by
this seed.

## Interface Changes

Describe the commands, projections, routes, or other operational surfaces this
seed expects realizations to expose.
