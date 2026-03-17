# Brief

Build an event listings product that lets organizers create and manage public
event calendars.

Desired initial outcome:

- organizers can add, edit, publish, and archive events
- the public can browse upcoming events by list or calendar
- each event has a clean public detail page
- separate teams can build alternate event experiences on the same shared graph
  of event, venue, organizer, media, and version data
- visitors can search by keyword and filter events by date, category, or
  location
- the first release stays focused on publishing and discovery, not ticketing

Primary users:

- organizer managing a calendar
- public visitor discovering events

Scope:

- event CRUD with draft, published, canceled, and archived states
- stable object identity and transparent version history for event content
- public event list and month-calendar views
- event detail pages with start and end time, timezone, all-day handling,
  venue, description, and links
- graph-first related objects for category, organizer, venue, and media even if
  the first UI still edits some of that through event-centric screens
- support for connected `series` of events
- keyword search and filtering for upcoming events by date, category, and
  location
- stable public URLs for event detail pages across edits

Constraints:

- keep the first version centered on public listings
- make the canonical seed model graph-first so later realizations can expose
  richer venue, organizer, series, media, or attendee subgraphs without
  breaking shared identity
- require explicit time and status semantics so list and calendar views behave
  consistently
- make public versus private data boundaries explicit enough that a different
  team could operate another UI on the same registry plus runtime data
- defer payments, ticket sales, and heavy marketing automation
- defer advanced attendance, registration, and check-in flows unless they are
  needed for discovery or publishing
- defer complex recurrence rules if they threaten delivery, but do not block the
  introduction of first-class `series` identity
- prefer a single web app that can be operated by one small team
