# Claims

Claims are append-only accepted assertions about objects.

## Purpose

A claim expresses some accepted state, relation, or interpretation about an
object.

Examples include:

- a title claim on a task object
- a status claim on a workflow object
- a binding claim that points an interface object at a realization artifact

## Core Rules

- claims are append-only
- claims may supersede earlier claims
- claims should be interpreted through explicit schema/version rules
- a claim belongs to accepted history only after it is appended through the
  registry

## Why Claims Matter

Claims let Autosoftware evolve without pretending the current system state is a
single overwritten codebase snapshot.

Because claims are separate from objects:

- identity stays stable
- accepted state can change over time
- interfaces can share the same underlying objects
- the materializer can rebuild state from ordered accepted assertions

## Supersession

Supersession is how the model expresses replacement without deletion.

Instead of mutating an accepted claim in place:

- a new accepted claim is appended
- that new claim may supersede the earlier one

This preserves accepted history while still allowing current state to move
forward.

## Interpretation

Claims are not useful by themselves.
They matter because schemas and materialization rules interpret them.

That is why:

- claims belong in the registry
- interpretation belongs in the kernel/materializer model
- current state is derived, not stored as the only truth
