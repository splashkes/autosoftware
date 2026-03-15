# Decisions

## Publishing Before Commerce

The first version is an event publishing tool, not a ticketing system.

Reason:

- publishing and discovery are the core jobs
- paid registration expands product scope dramatically
- a clean listings product is easier to build and validate first

## Explicit Time Semantics

Events must have deterministic start, end, timezone, and all-day behavior.

Reason:

- calendar products fail quickly when list and calendar views disagree
- time-zone ambiguity breaks public trust
- explicit rules are necessary even in a small MVP

## Public Pages Matter As Much As Admin

The public browse experience is a first-class part of the product.

Reason:

- event software fails if discovery pages are weak
- organizers care about presentation, not only data entry

## Canceled Stays Public, Archived Leaves Discovery

Canceled and archived are different public states and should not be merged.

Reason:

- visitors need to understand that a once-published event was canceled
- organizers still need a way to remove old events from upcoming discovery
- stable URLs are easier to preserve when state semantics are clear

## Recurrence Deferred

Complex recurring event rules are deferred from v1.

Reason:

- recurrence logic creates disproportionate complexity
- single and simple multi-day events are enough for the first useful release

## Stable Slugs Generated Once

The first runnable realization generates a slug when an event is created and
keeps that slug fixed through later edits.

Reason:

- public detail URLs must remain stable across organizer edits
- the simplest trustworthy rule is "slug is write-once"
- a stored slug avoids title-change edge cases in links, search, and archives

## Server-Rendered MVP First

The initial runnable realization is a self-contained server-rendered Go web app
with in-memory state and minimal JSON projections.

Reason:

- it satisfies the seed quickly without changing kernel infrastructure
- it keeps organizer and public surfaces coherent in one code path
- it leaves room for later persistence and kernel capability integration
