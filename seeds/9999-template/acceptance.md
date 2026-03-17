# Acceptance

List the acceptance criteria required for this seed, such as replay
expectations, permission assertions, and interface expectations.

These are seed-level requirements.
Realization-specific evidence belongs under `realizations/<id>/validation/`.

Boundary:

- define what must be true for this seed to be accepted
- capture expected results or evidence pointers
- do not use this as a generic kernel runbook
- do not restate the design here

Typical acceptance requirements should cover:

- first-party private workflows are reachable through declared commands and
  projections, not hidden server-only reads or writes
- alternate clients can use a stable object identity even when the UX centers a
  handle or slug
- any public-metadata or digest-only registration surface is explicit about its
  boundary relative to private content
