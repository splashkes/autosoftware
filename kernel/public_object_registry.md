# Public Object Registry

This document explains the registry as a system concept, not just a protocol
surface.

Boundary:

- explain what the registry is for
- explain why the registry matters to Autosoftware
- illustrate the utility of the model from several perspectives
- do not restate field-by-field protocol specs here
- do not turn this into an operations manual

## Core Idea

Autosoftware separates:

- objects with stable identity
- claims about those objects
- realizations that generate accepted changes
- a materializer that rebuilds current state from accepted history

The registry is the authoritative append-only record of accepted changes.
It is where accepted object and claim mutations go after review.

The application does not become current because files were overwritten in a
repository.
It becomes current because accepted changes were appended to the registry and
the materializer rebuilt state from that history.

## Why This Exists

The registry is useful because it decouples three things that are usually
collapsed together:

- the user-facing request for change
- the concrete implementation that satisfies that request
- the current running state of the application

That separation gives Autosoftware a better change surface for both humans and
agents.

## Perspective: Rapidly Evolving UI

In a conventional app, a UI redesign usually means replacing templates,
components, CSS, and behavior in place.
The old state is gone unless a separate release process preserved it.

With the registry model:

- a new UI realization is accepted as new object/claim changes
- prior accepted UI state remains part of the history
- the materializer can rebuild older or newer accepted UI states
- several UI approaches can be compared against the same underlying objects

This matters for Autosoftware because the UI may evolve quickly while the
underlying object model remains stable.

## Perspective: Shared Data And Structures

The same objects can support several interfaces at once.

For example:

- one realization may present work as a kanban board
- another may present the same objects through an HTMX workflow
- another may expose the same underlying structure to an agent-first API

The registry helps here because shared data and shared structure are not buried
inside one interface implementation.
They are recorded as objects and claims that multiple realizations can rely on.

## Perspective: Safer Continuous Evolution

Autosoftware is not only about generating code quickly.
It is about preserving accepted history while the implementation keeps moving.

The registry gives the system:

- append-only accepted change history
- deterministic replay targets
- explicit supersession instead of silent overwrite
- traceability from current behavior back to accepted mutations

Without that layer, the project risks collapsing into "agents editing a normal
application repo."

## Perspective: Materialization As A First-Class Step

The materializer is not a cache warmer.
It is the part of the system that interprets accepted history and turns it into
current runnable state.

That means:

- the registry stores accepted truth
- the materializer derives current state
- runtime state can be rebuilt from accepted history

This is the main reason the registry belongs inside the kernel model rather
than as an incidental backend detail.

## What The Registry Is Not

The registry is not:

- a replacement for every runtime storage table
- a dump of browser telemetry or test output
- a place for unreviewed draft work
- a synonym for the Git repository

Runtime diagnostics belong in kernel runtime storage.
Draft seed work belongs in `seeds/`.
Accepted object and claim mutations belong in the registry.

## Read Next

- `kernel/protocol/v1/registry.md` for append and row/change-set rules
- `kernel/protocol/v1/objects.md` for object identity
- `kernel/protocol/v1/claims.md` for assertion semantics
- `kernel/public_claim_registry.md` for the claim-focused view
- `kernel/public_schema_object_registry.md` for the schema-focused view
- `kernel/architecture.md` for how `registryd` and `materializerd` fit together
