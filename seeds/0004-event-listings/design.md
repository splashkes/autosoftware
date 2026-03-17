# Design

This seed defines an event publishing and discovery product, not a full
ticketing platform.

## Product Shape

The application should have:

- an organizer admin workspace
- a public-facing event directory
- a calendar view and event detail pages

The key job is making event publishing fast and public discovery pleasant.

## Actors and Access

- Organizer: authenticated staff member who creates and manages events.
- Public visitor: anonymous visitor who browses, searches, filters, and opens
  event pages.

## Event Model

Each event should have:

- stable event object identity for machine clients and alternate UIs
- title, summary, and description
- stable public URL that does not change when the event is edited
- start timestamp, end timestamp, and authoritative timezone
- optional `all_day` behavior
- venue or location information for display and filtering
- category and optional external destination link

Time semantics for v1:

- all rendering and filtering should use the event's own timezone
- an all-day event is shown without clock times
- a single-day event has a start and end on the same date
- a simple multi-day event spans consecutive dates without recurrence logic
- a multi-day event appears on every covered day in the month calendar and
  matches any overlapping date-range filter

## State Model

- `draft`: editable and not public
- `published`: publicly discoverable in list, calendar, search, and filters
- `canceled`: still publicly visible with a clear canceled label so visitors do
  not assume the event disappeared by mistake
- `archived`: removed from default upcoming discovery surfaces, while the event
  detail page may still remain reachable at the same URL with an archived
  notice

Unpublishing an event returns it to `draft`.

## Core Workflows

### Organizer Workflow

Organizers should be able to:

- create an event draft
- add event title, summary, description, start and end time, timezone, venue,
  category, and link
- publish or unpublish an event
- edit an existing event without losing its public URL
- mark an event as canceled or archived

These organizer workflows must not be private implementation details of the
first-party UI. A realization should expose the same semantic commands and
private organizer read models so another UI or agent can manage the same event
objects.

### Public Discovery Workflow

Visitors should be able to:

- browse upcoming events as a list
- switch to a month-calendar view
- search by title or keyword
- filter by category, date range, and location
- open a dedicated event page with clear time, venue, and status information

Public discovery may expose only the metadata appropriate for public visitors,
while organizer-facing projections may expose fuller draft or editorial state.
That split should be explicit in the realization contract rather than hidden in
server-only code.

## MVP Boundaries

Included in v1:

- one organization managing one public calendar
- single events and simple multi-day events
- list and calendar views
- keyword search plus filtering by date, category, and location
- stable event URLs and explicit status behavior

Deferred beyond v1:

- ticket sales
- waitlists
- sponsor packages
- attendee accounts
- advanced recurrence rules
- ICS sync and external calendar import

## Realization Guidance

Realizations should make public discovery trustworthy first: time display,
status behavior, URL stability, and filter semantics should be deterministic.

Realizations should also declare:

- a stable by-ID event projection for alternate clients
- a public handle-based detail projection for the public site
- a private organizer workspace projection for draft and editorial state

Technology choice belongs in the realization approach documents. Whatever stack
is used, the MVP should keep organizer publishing simple while ensuring that
public list, calendar, search, and detail views agree on the same event model.
