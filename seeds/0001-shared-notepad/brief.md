# Brief

Build a shared notepad app that anyone can open and use immediately.

Desired initial outcome:

- one URL
- one shared room
- note create, edit, and delete
- no login
- no database
- no ownership model
- no private state

The notepad is intentionally disposable.
Its purpose is to make the kernel prove that a realization can hold actual
runnable application code.

Scope:

- a browser UI for writing and editing notes
- in-memory shared state for the running process
- a realization that contains runnable source code under its own `artifacts/`
  tree
- a machine-readable runtime artifact that says how the realization should boot

Constraints:

- keep the app intentionally small
- prefer Go + HTMX for speed and inspectability
- keep kernel changes minimal and justified
- defer persistence, auth, permissions, and multi-room behavior
