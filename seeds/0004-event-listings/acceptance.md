# Acceptance

Every realization of this seed must satisfy the following:

1. An organizer can create, edit, publish, unpublish, cancel, and archive
   events without changing the event's public URL.
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
9. Draft or otherwise private organizer state is not exposed anonymously, and
   any public-metadata versus private-content boundary is explicit in the
   realization contract.
10. The realization stays focused on event publishing and discovery rather than
   bundling ticketing or attendee-management promises it does not implement.
