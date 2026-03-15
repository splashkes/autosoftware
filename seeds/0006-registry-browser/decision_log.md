# Decisions

## Read-Only Is A Product Requirement

The registry browser must not expose mutation capability.

Reason:

- trust collapses if a browsing surface can also mutate accepted history
- public inspection should be safe by default
- agents need a clear rule: observe here, do not write here

Rejected alternative:

- a blended "admin browser" that mixes browse and mutation controls

## UI And API Must Share One Truth

The browser should point at the authoritative registry API rather than invent
an unrelated read model for agents.

Reason:

- humans and agents should be able to verify the same state
- a separate hidden model would create drift and confusion
- linking each screen to the backing route improves trust

Rejected alternative:

- a UI-only browser with undocumented internal queries

## Existing Registry Routes Come First

The first realization should use the registry catalog routes that already
exist before asking for new backend browse APIs.

Reason:

- current objects, schemas, realizations, commands, and projections are
  already exposed
- building on existing routes keeps the seam cleaner and reduces drift
- new routes should be reserved for true ledger resources, not duplicated list
  surfaces

Rejected alternative:

- immediately creating seed-local APIs for browsing data that is already
  available

## Progressive Disclosure Beats Density

The first realization should prefer a simple top layer with deeper detail on
demand.

Reason:

- the audience includes curious non-experts, not just operators
- an all-detail interface would make the registry feel opaque
- deep inspection is still required for disputes and audits

Rejected alternative:

- shipping only a dense power-user console first

## Claims And Schema Versions Are First-Class

The browser must expose claims and schema versions directly, not only through
object pages.

Reason:

- truth in the registry lives in claims
- schemas govern meaning, not just formatting
- disputes often depend on exact schema version and supersession history

Rejected alternative:

- object-only browsing with claims hidden inside object summaries

## Public Redaction Must Be Explicit

If some accepted history is not public, the browser should explain that
visibility boundary rather than pretending the visible subset is the whole
story.

Reason:

- trust requires honesty about what is omitted
- agents need clear expectations for public versus private authority

Rejected alternative:

- silent omission with no visibility explanation
