# Background Workers

## ingestion_worker

Fetches updates from external calendars.

## normalization_worker

Maps events into unified schema.

## propagation_worker

Pushes production events to destination calendars.

## conflict_worker

Detects overlapping events.

## health_worker

Monitors sync health and alerts on failures.