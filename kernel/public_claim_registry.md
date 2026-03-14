# Claim Registry

This document explains the claim layer as its own useful lens on the registry.

Boundary:

- explain why claims deserve separate attention
- explain the utility of claim history and supersession
- make clear that this is not a second physical registry
- do not restate protocol fields line-by-line

## One Registry, Claim-Focused View

Autosoftware still has one append-only registry.

This document isolates the claim layer because claims do most of the work of
describing accepted state over time.

Objects provide stable identity.
Claims express accepted assertions about those objects.

## Why Claims Matter

Claims are the reason the system can evolve without pretending the application
is one overwritten snapshot.

Because claims are append-only:

- accepted state can move forward without erasing history
- old and new interpretations can be compared
- the materializer can rebuild current state from ordered accepted assertions

## Perspective: Change Without Silent Overwrite

In a conventional app, state is often updated in place.

With claims:

- the earlier accepted assertion remains in history
- the newer accepted assertion supersedes it
- the path from old behavior to new behavior stays visible

This is one of the key reasons the system remains explainable while evolving.

## Perspective: Shared Data Across Interfaces

Claims let several interfaces depend on the same underlying accepted state.

For example:

- one interface may read a task title claim into a kanban card
- another may read the same claim into a chat-oriented summary
- another may combine that claim with status and relationship claims in an API

The shared object stays stable.
Several interfaces simply interpret the accepted claims around it.

## Perspective: Safer Agent Work

Claims are useful for agent-driven development because they make accepted state
explicit.

Instead of asking an agent to infer "what changed" from a replaced file tree,
the system can say:

- which object was affected
- which claim was added
- which earlier claim was superseded
- which schema interprets the accepted assertion

That shortens review and improves traceability.

## Relationship To Materialization

Claims are not the running app by themselves.

They matter because the materializer reads ordered accepted claims and derives
current state from them.

That means:

- the registry stores accepted claim history
- the materializer computes current state from that history
- interface state is derived, not the only truth

## Read Next

- `kernel/protocol/v1/claims.md`
- `kernel/public_object_registry.md`
- `kernel/public_schema_object_registry.md`
