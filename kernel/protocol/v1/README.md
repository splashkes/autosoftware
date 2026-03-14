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
- `objects.md` defines stable object identity and immutable creation metadata
- `claims.md` defines append-only assertions, supersession, and interpretation
- `schemas.md` defines schema identity, versioning, and interpretation rules
