# Decisions

## Realization Carries The App

The runnable notepad implementation lives under the realization's `artifacts/`
tree instead of under `kernel/`.

Reason:

- this seed exists to prove that realizations can hold real code
- keeping the app in the seed avoids prematurely hard-coding app behavior into
  trusted machinery

## In-Memory Shared State First

The first implementation uses process-local memory and no database.

Reason:

- it keeps the teaching surface small
- it isolates runtime-shape questions from persistence questions
- it is enough to prove the "shared room" interaction model

## Go Plus HTMX

The first realization uses a single-process Go server with server-rendered HTML
and HTMX.

Reason:

- fast to build and inspect
- no frontend build chain required
- easy to keep inside a realization artifact tree
