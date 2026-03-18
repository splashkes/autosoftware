# PLAN — Calendar Sync Hub

## Concept

Calendar data exists across many fragmented systems.

Examples:

- Google calendars
- Apple calendars
- ICS feeds
- Event platforms
- Venue calendars
- Internal production calendars

The Calendar Sync Hub acts as a **time network router**.

Sources → Hub → Destinations

## Key Concepts

### Aggregation

External sources feed events into the system.

### Normalization

Events are mapped into a common event model.

### Authority

One calendar is designated as the **production calendar**.

### Propagation

Production events can be pushed into downstream calendars.

### Observability

Every event maintains provenance and propagation state.