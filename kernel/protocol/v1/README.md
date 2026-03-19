# Protocol v1

Initial protocol notes and draft specs for the kernel-facing ledger model.

Key distinctions to preserve:

- seeds are source intent
- realizations are compiled intent
- accepted change sets are derived from accepted realizations

Read this after `kernel/public_object_registry.md` if you want the normative
shape rather than the conceptual explanation.

Current protocol focus:

- `registry.md` defines append-only change-set and row rules
- `registry_query_api.md` defines the Phase 2 read/query surface for accepted
  registry state
- `objects.md` defines stable object identity and immutable creation metadata
- `claims.md` defines append-only assertions, supersession, and interpretation
- `schemas.md` defines schema identity, versioning, and interpretation rules
- `permissions.md` defines the system-native authority, delegation, and
  effective-access materialization model that seeds should build on when they
  need meaningful control
- `interactions.md` defines the normalized operational API contract that every
  runnable realization must expose for both UI and machine clients
- `growth.md` defines the kernel growth-console contract from seed docs to
  growth jobs and readiness states
