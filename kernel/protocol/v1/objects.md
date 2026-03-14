# Objects

Objects provide stable identity inside the registry model.

## Purpose

An object is the thing that claims are about.
Objects stay stable while claims express mutable accepted state about them.

Examples include:

- domain entities such as tasks or people
- interface entities such as UI components
- structural entities such as schemas or stores

## Core Rules

- objects have stable IDs
- objects are created explicitly
- objects are never deleted from accepted history
- mutable accepted state does not live on the object record

## Immutable Object Metadata

Objects may carry immutable creation metadata such as:

- object ID
- object kind
- creator principal
- created-at timestamp

These are creation facts, not mutable application state.

## Why Objects Matter

The registry needs something more stable than files or table rows.

An object lets the system say:

- this later claim is about the same thing
- this interface and that interface refer to the same entity
- this state evolved rather than being silently replaced
