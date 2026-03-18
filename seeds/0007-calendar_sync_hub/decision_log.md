# Decisions

## One Internal Production Authority

The hub should own one production calendar as the write authority for
downstream propagation.

Reason:

- multi-master sync would make correctness and conflict handling much harder
- operators need one clear answer to "what is the current approved event?"
- propagation semantics are simpler when outbound copies derive from one
  internal authority

Rejected alternative:

- treating every connected calendar as a peer that can overwrite every other
  calendar

## Import First, Propagate Outward

V1 should focus on importing upstream changes and propagating approved events
outward.

Reason:

- it matches the current seed concept and roadmap
- it keeps the trust boundary understandable
- it avoids silent write loops into upstream systems

Rejected alternative:

- allowing freeform bidirectional editing from the first realization

## Normalized And Production Events Stay Distinct

The system should distinguish normalized inbound interpretation from approved
production truth.

Reason:

- upstream changes may need review before they become operational truth
- operators need to see drift between raw input and approved output
- propagation should point to a stable production state, not a mutable import

Rejected alternative:

- treating inbound source events and production events as the same record

## Poll First Adapter Strategy

The first adapter model should favor explicit polling and incremental fetches.

Reason:

- it is easier to reason about, validate, and replay
- it keeps worker behavior predictable across adapter types
- webhooks can be added later once the normalized model is stable

Rejected alternative:

- requiring event-driven webhook infrastructure before validating the core hub

## Keep Two Realizations Separate

The seed should preserve two distinct realization paths:
one operations-first and one production-workbench-first.

Reason:

- reliability operators and production coordinators optimize for different
  workflows
- forcing both into one first UI would blur the evaluation
- both can share the same authority model and contract while testing different
  information architecture

Rejected alternative:

- one blended first realization that mixes graph operations and calendar
  curation into one ambiguous surface
