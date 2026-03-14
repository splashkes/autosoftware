# Design

This seed establishes a deliberately small application whose main job is to
teach the kernel what a runnable realization needs to contain.

## Scope

The shared notepad should provide:

- a single shared room
- note creation with title and body
- note editing
- note deletion
- automatic browser refresh so several open tabs converge on the same state

The storage model is process-local memory only.
Restarting the process clears the note list.

## Realization Shape

The realization should carry:

- machine-readable realization metadata in `realization.yaml`
- a runtime manifest artifact that describes how to launch the app
- the application source itself under realization artifacts
- validation notes describing how to check the result manually

This is the core exercise:
the realization must be code-bearing and runnable, not merely descriptive.

## Runtime Shape

The app should be a small Go HTTP server with HTMX-enhanced HTML.

Reasons:

- one binary shape
- no separate build pipeline required
- easy to inspect inside a realization artifact tree
- straightforward to evolve toward later kernel boot semantics

## Deferred Areas

This seed intentionally defers:

- persistence
- authentication
- authorization
- multi-room or per-user workspaces
- conflict resolution beyond last writer wins
- registry append semantics for note mutations
- automatic kernel-managed boot from a realization manifest

Those are valuable later, but they should not obscure the first question:
what does a runnable realization actually look like?
