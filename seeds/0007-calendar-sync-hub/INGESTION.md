# Calendar Ingestion

Adapters ingest events from external sources.

Supported adapters:

- Google Calendar API
- CalDAV / Apple Calendar
- ICS feeds
- Event platform APIs

## Sync Strategy

Adapters should:

- fetch incremental updates
- track source event IDs
- detect changes upstream

Polling intervals configurable per source.