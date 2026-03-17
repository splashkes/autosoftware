# Acceptance

Every realization of this seed must satisfy the following:

1. An organizer can create, edit, publish, unpublish, cancel, and archive
   events without changing the event's stable machine identity or public URL.
2. The realization stores and renders each event with explicit start time, end
   time, timezone, and optional all-day behavior.
3. A public visitor can browse upcoming published events in a list view.
4. A public visitor can browse events in a calendar view, including correct
   rendering of simple multi-day events.
5. A public visitor can open a dedicated event detail page with clear time,
   venue, and status information.
6. Public discovery supports keyword search and filtering by date, category,
   and location.
7. Canceled events remain publicly understandable with a clear canceled status,
   while archived events are removed from default upcoming discovery surfaces.
8. Alternate clients can manage and read the same event objects through
   declared commands plus projections, including a stable by-ID event read
   surface and a private organizer workspace read surface.
9. The seed's canonical data model is graph-first: events, versions, venues,
   organizers, series, media, and actor provenance must be documented as
   explicit nodes or edges rather than left as permanent flattened prose.
10. Content-bearing objects that can change over time declare transparent
    version semantics instead of silently overwriting canonical truth.
11. The seed docs make snapshot-versus-reference rules explicit anywhere a
    related object may change after publication.
12. Draft or otherwise private organizer state is not exposed anonymously, and
    any public-metadata versus private-content boundary is explicit in the seed
    docs and realization contract.
13. If data is classified as anonymously public for this seed, it is reachable
    through authoritative APIs as well as the first-party UI, subject only to
    declared anonymous-client safety controls such as rate limiting or
    crawl-abuse protections.
14. Each event instance is registrable through a stable registry-layer record
    whose public boundary is explicit even when the event's fuller content is
    private, draft, or metadata-only.
15. The realization classifies canonical event data as `shared_metadata`,
    `public_payload`, `private_payload`, or `runtime_only`, and that
    classification is visible in both the seed docs and
    `interaction_contract.yaml`.
16. If a realization adds structured truth beyond the current seed core, it
    does so additively as a new canonical node, edge, or namespaced extension
    rather than by silently repurposing an older field.
17. If participation data is added later, it is modeled as an explicit
    attendance subgraph such as intent, registration, or check-in rather than as
    an undifferentiated attendee blob on the event.
18. The realization stays focused on event publishing and discovery rather than
    bundling ticketing or attendee-management promises it does not implement.
