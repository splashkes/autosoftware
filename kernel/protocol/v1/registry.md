# Registry

The registry is the authoritative append-only record of accepted mutations.

## Purpose

The registry exists to record accepted object and claim changes in a form that
can be replayed and materialized.

It is the source of truth for accepted history.
Events, webhooks, and GitHub actions may notify the system that something
happened, but they are not the authoritative record.

## Core Model

The registry records:

- change sets as the accepted unit of mutation
- rows as the atomic records inside a change set

An accepted realization emits one or more rows through one accepted change set.

## Invariants

- the registry is append-only
- accepted rows are ordered
- past accepted rows are never rewritten in place
- replay order comes from registry order, not timestamps
- workers synchronize by cursoring through ordered accepted rows

## Row Types

Initial row types:

- `object.create`
- `claim.create`

The registry should stay small at the row-type level.
Most application meaning belongs in claims and their interpretation, not in a
large kernel-level row taxonomy.

## Change Sets

The change set is the accepted mutation boundary.

It should carry:

- accepted provenance
- idempotency identity
- one or more ordered rows

This keeps review and acceptance attached to a concrete realization while still
letting the registry remain generic.

## Synchronization

Workers should synchronize by reading accepted rows after the last known
cursor.

Conceptually:

- `row_id > last_cursor`

Wake signals are useful for latency, but replay should always trust the
registry, not the wake mechanism.

## Relationship To Runtime State

The registry is not the same as the current application state.

The registry stores accepted history.
The materializer interprets that history and rebuilds current state from it.
