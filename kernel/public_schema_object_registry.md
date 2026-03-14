# Schema Object Registry

This document explains schemas as first-class objects inside the registry
model.

Boundary:

- explain why schemas should be treated as objects
- explain why schema versioning matters to replay and materialization
- make clear that this is not a separate physical registry
- do not restate protocol fields line-by-line

## One Registry, Schema-Focused View

Autosoftware still has one append-only registry.

This document isolates the schema layer because schema objects define how
claims are interpreted over time.

Schemas deserve object-like treatment because they have:

- stable identity
- versioned evolution
- publication and permission concerns
- artifact bindings
- downstream consumers in the materializer

## Why Schemas Matter

Claims are only useful if the system knows how to interpret them.

Schema objects let the system say:

- what kind of claim this is
- how it should be validated
- how it should be interpreted during materialization
- which version is being relied on

Without schema objects, claim interpretation drifts into ad hoc code and
replay becomes harder to trust.

## Perspective: Replayable Meaning

Autosoftware is not just preserving accepted bytes.
It is preserving accepted meaning.

If a claim says "task title" or "UI binding," the system must know which
schema version interprets that statement.

That is why schemas need:

- stable object identity
- explicit versioning
- explicit publication
- explicit references from claims

## Perspective: Rapid UI Evolution

Rapid UI evolution is one of the clearest reasons to keep schemas separate from
realizations.

Several realizations may:

- present the same UI-related object differently
- depend on the same structural schema
- publish new schema versions for richer interpretation

Because schemas are first-class objects, UI evolution can move quickly without
losing the structural meaning of accepted claims.

## Perspective: Shared Structure Across Systems

The same underlying objects may be used by:

- a web interface
- an agent-facing API
- a background workflow

Schema objects give those systems a shared interpretation layer.
They keep structure from being trapped inside one interface implementation.

## Relationship To Claims

Objects provide stable identity.
Claims provide accepted assertions.
Schema objects provide the interpretation rules that make those accepted
assertions meaningful.

The three layers work together:

- object: what the claim is about
- claim: the accepted assertion
- schema object: how that assertion is interpreted

## Relationship To Materialization

The materializer depends on schema objects to know how accepted claims should
turn into current state.

That is why schema evolution must be explicit and versioned.
Otherwise the same accepted claim could mean different things at different
times with no clear boundary.

## Read Next

- `kernel/protocol/v1/schemas.md`
- `kernel/protocol/v1/claims.md`
- `kernel/public_object_registry.md`
