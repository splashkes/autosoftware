# Notes

Initial implementation slices to consider:

1. landing page, overview counts, and API discovery guidance
2. searchable list pages for objects, claims, schemas, change sets, and rows
3. deep detail pages with provenance, supersession, and schema-version links
4. explicit read-only verification and validation evidence

Open product questions for later refinement:

- whether row browsing should default to newest-first for humans while
  preserving authoritative ascending cursor semantics for agents
- whether raw JSON payload views should be visible inline or behind an explicit
  "show raw" action
- how strongly the first realization should surface public redaction language
  when the public/private model is still evolving
