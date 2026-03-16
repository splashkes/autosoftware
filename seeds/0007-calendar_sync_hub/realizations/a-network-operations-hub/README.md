# Network Operations Hub Realization

This realization is the operations-first variant of the calendar sync hub
seed.

It should make the network of calendars legible to an operator responsible for
ingestion health, propagation reliability, and event lineage.

## Why This Exists

The seed includes many moving parts:

- source connectors
- normalized events
- production events
- destination calendars
- propagation retries
- conflicts and upstream drift

This realization leads with the operational network itself rather than the
production calendar as the main workspace.

## Core Emphasis

- source, hub, and destination topology
- sync freshness and connector health
- incident and retry queues
- per-event lineage across ingestion, normalization, and propagation

## Boundary

This directory defines the draft contract and approach boundary for the
operations-first realization.
It does not yet include runnable artifacts.
