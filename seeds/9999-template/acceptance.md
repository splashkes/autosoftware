*Quick `acceptance.md` map: top = concrete outcomes; middle = graph, identity, versioning, and public/private assertions; bottom = evidence notes, extensions, or clarifications.*

# Acceptance

List the acceptance criteria required for this seed, such as replay
expectations, permission assertions, and interface expectations.

These are seed-level requirements.
Realization-specific evidence belongs under `realizations/<id>/validation/`.

In a finished seed, start with the actual acceptance bullets rather than a long
preface.

Boundary:

- define what must be true for this seed to be accepted
- capture expected results or evidence pointers
- do not use this as a generic kernel runbook
- do not restate the design here

Typical acceptance requirements should cover:

- first-party private workflows are reachable through declared commands and
  projections, not hidden server-only reads or writes
- authenticated agent callers have parity with the first-party UI for normal
  workflows, or a documented reason when a surface is intentionally narrower
- the canonical graph is explicit about nodes and relations rather than hiding
  durable truth in one flattened payload
- alternate clients can use a stable object identity even when the UX centers a
  handle or slug
- content-bearing objects that matter over time have explicit version semantics
- authorship or edit provenance is modeled explicitly when it matters
- any public-metadata or digest-only registration surface is explicit about its
  boundary relative to private content
- anonymously public data is available through authoritative APIs as well as the
  UI, subject only to declared anonymous-client safety controls
- the seed docs and interaction contract agree on which fields are
  `shared_metadata`, `public_payload`, `private_payload`, or `runtime_only`
- the seed docs and interaction contract agree on which runtime-only
  authoring-context fields may be sent by agents without becoming canonical
  shared truth
- useful authenticated errors are intentional, structured, and testable in
  flight rather than incidental implementation detail
- realization-specific extra structured truth is added additively instead of by
  silently repurposing older canonical fields
- when the seed has meaningful authority semantics, the docs and interaction
  contract define subject, scope, bundle, grant, and delegation rules clearly
- when the seed has meaningful authority semantics, effective access is derived
  from explicit accepted history rather than hidden local-only role checks
