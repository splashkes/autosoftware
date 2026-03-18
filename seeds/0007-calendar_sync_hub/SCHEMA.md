# Schema Specification

## calendar_source

Fields:

- id
- name
- type
- credentials_reference
- last_sync_time
- status

## calendar_destination

Fields:

- id
- name
- type
- credentials_reference

## event

Fields:

- id
- title
- description
- start_time
- end_time
- source_calendar_id
- origin_type
- last_modified

## propagation_edge

Fields:

- event_id
- destination_calendar_id
- status
- last_push_time

## event_revision

Tracks history of changes to events.