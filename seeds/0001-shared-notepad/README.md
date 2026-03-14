# Shared Notepad Seed

`0001-shared-notepad` is the first post-genesis proving-ground seed.

It exists to force the kernel model to carry a real runnable app realization
instead of only prose and placeholder artifacts.

Document boundaries:

- `README.md` explains this seed only.
- `brief.md` states the shared-notepad request and constraints.
- `design.md` explains the notepad design and what is deferred.
- `approaches/` defines named realization approaches for this seed.
- `decision_log.md` records durable seed-local choices and rationale.
- `acceptance.md` defines what every shared-notepad realization must satisfy.
- `seed.yaml` records machine-readable seed metadata.
- `realizations/` holds concrete notepad realizations and their runtime
  artifacts.

This seed intentionally pushes implementation into `realizations/`.
Kernel changes should only happen when the seed proves a capability that truly
belongs in shared trusted machinery.
