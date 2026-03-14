# Brief

Build an event listings product that lets organizers create and manage public
event calendars.

Desired initial outcome:

- organizers can add, edit, publish, and archive events
- the public can browse upcoming events by list or calendar
- each event has a clean public detail page
- visitors can search by keyword and filter events by date, category, or
  location
- the first release stays focused on publishing and discovery, not ticketing

Primary users:

- organizer managing a calendar
- public visitor discovering events

Scope:

- event CRUD with draft, published, canceled, and archived states
- public event list and month-calendar views
- event detail pages with start and end time, timezone, all-day handling,
  venue, description, and links
- basic taxonomy for category, organizer, and venue
- keyword search and filtering for upcoming events by date, category, and
  location
- stable public URLs for event detail pages across edits

Constraints:

- keep the first version centered on public listings
- require explicit time and status semantics so list and calendar views behave
  consistently
- defer payments, ticket sales, and heavy marketing automation
- defer complex recurrence rules if they threaten delivery
- prefer a single web app that can be operated by one small team
