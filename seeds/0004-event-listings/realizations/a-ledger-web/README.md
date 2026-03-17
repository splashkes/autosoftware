# Event Listings Ledger Web

This realization is the registry-native flagship surface for the event listings
seed.

It keeps the public/admin web experience from the MVP direction, but changes
the storage model underneath:

- durable event objects in the shared runtime database
- append-only claims for snapshots and lifecycle transitions
- materialized current-state read models for directory, calendar, and detail
- human-readable ledger pages so the accepted history is visible in-product
