# Brief

Build a public registry browser that makes the append-only registry legible,
auditable, and trustworthy for both people and agents.

Full seed vision:

- a person can browse and search registry objects, claims, schemas, change
  sets, and rows from one clear UI
- the UI has a simple top layer for orientation and a deeper layer for full
  authoritative detail
- every detail screen links back to accepted history instead of hiding the
  ledger behind a convenience view
- agents can access the same information through stable API routes without
  scraping HTML
- the surface stays strictly read-only

First realization scope (current catalog routes only):

- browse and search realizations, commands, projections, objects, and schemas
- claims, schema versions, change sets, and rows are deferred until their
  authoritative registry routes exist

Constraints:

- this seed must not introduce mutation paths into the registry browser
- the UI should distinguish convenience navigation from authoritative registry
  resources when both exist
- public versus private visibility rules must be explicit rather than implied
- the first version should build trust through clarity, not visual complexity
