# Autosoftware

Software that evolves from within.

In the historical model, a change request leaves the product and disappears
into a separate software process. In Autosoftware, the product is where change
begins. Users describe what they want inside the running experience, and the
system designs, builds, validates, and ships the change without leaving.

## How It Works

A **seed** captures a user request — what they want and why. The kernel
validates the design with the user, then produces one or more **realizations**:
concrete implementations compiled from the seed by different agents, models, or
strategies. The best realization is accepted, and the **materializer** rebuilds
the live experience automatically.

Seeds are full change capsules. Each one holds the brief, design, acceptance
criteria, approaches, and every realization compiled from them. The same seed
can be realized many times as tools improve.

## Who This Is Not For

Yes, AI can generate a working app in minutes. Yes, there are thousands of
starter repos, boilerplates, and "ship your SaaS this weekend" templates.
That is a solved problem and we are not solving it again.

This project is not a code generator. It is not a shortcut for solo founders.
It is not trying to replace your favorite framework or make developers
obsolete. If you want an app built and deployed by Friday, you already have
great options.

What does not exist yet is a **shared interaction platform that evolves
continuously** — where users participate in the change process, where
competing realizations coexist safely, where the history of every accepted
mutation is preserved and replayable, and where the whole thing is designed
from the ground up for an era where AI is a first-class participant in
software creation.

We are building a public software commons. The people who use it should own
its evolution, not just its interface. That is a different problem, and it is
the one we are here for.

## Active Seeds

| Seed | Summary |
|------|---------|
| `0001-shared-notepad` | Shared in-memory notepad — the first runnable realization |
| `0003-customer-service-app` | Web-first support product with tickets, live chat, and knowledge base |
| `0004-event-listings` | Public event calendar with organizer publishing and discovery |
| `0005-charity-auction-manager` | Online auction product for charities with lot management and bidding |

## Repo Layout

```
genesis/        founding seed — the first complete example of the model
kernel/         trusted machinery: registry, materializer, growth engine, web surfaces
seeds/          change capsules: request → design → approaches → realizations
materialized/   hydrated outputs derived from accepted registry history
scripts/        local dev helpers
```

Most change happens in `seeds/`. The kernel enforces correctness — registry
validation, artifact verification, materialization determinism — and should
change less often.

## The Registry

Accepted changes are not committed to a repo and forgotten. They are appended
to an **object registry** — an ordered, append-only ledger of every accepted
mutation. Objects have stable identity. **Claims** record what is true about
them: a title, a UI binding, a moderation decision, a schema version.
**Schemas** are first-class objects too, so the system preserves not just
accepted bytes but accepted meaning.

The materializer reads the registry and rebuilds current state from it. The
running application is always a derived view of accepted history, not an
in-place overwrite. Old and new interpretations can coexist. A UI redesign
is a new realization accepted alongside the previous one, not a replacement
that erases it.

This separation has practical consequences: the same underlying objects can
power a kanban board, an HTMX workflow, and an agent-facing API
simultaneously. Shared structured data lives in the registry, not buried
inside one interface implementation.

### Public Registry

We maintain a public registry as the canonical authority for Autosoftware
objects. Any accepted seed, realization, schema, or claim published here
is openly addressable and replayable by anyone.

### Federation

Registries are designed to be federated. Each registry acts as its own
authority with a signing identity, and object references are globally
qualified (`as:<id>`, `acme:<id>`, `artbattle:<id>`). A worker can trust
and sync from multiple registries at once.

Different participants can fork the UI, workflows, local schemas, and
algorithms while still collaborating on the same objects — as long as they
share stable identities and the core claims they understand. Unknown claims
and schemas are safely ignored by systems that don't recognize them.

This means a charity auction platform and an art marketplace could share
the same object model for lots, bids, and payouts, each with their own
surfaces and business rules, while federation keeps the underlying data
portable. Private claims (payout status, moderation notes, internal
reliability scores) stay private — the ledger records that they exist and
who authored them, but the content itself remains under the authority's
control.

### Why This Matters

Without the registry layer, this project would collapse into "agents editing
a normal application repo." The registry is what makes continuous evolution
safe: append-only history, explicit supersession instead of silent overwrite,
deterministic replay, and traceability from current behavior back to the
accepted mutation that produced it.

## Running Locally

No external dependencies required. Start the full kernel stack with:

```
./scripts/local-run.sh
```

The boot console is at `http://localhost:8090/`. From there you can inspect
seeds, queue growth jobs, and launch runnable realizations.

Realizations can declare their own subdomain or path prefix. When running,
the kernel routes `notepad.localhost:8090` or `localhost:8090/notepad/`
directly to the realization's process.

A runtime database (Postgres) is optional and only needed for identity,
sessions, and communications features. See [kernel/runbook.md](kernel/runbook.md)
for configuration.

## Production Shape

`main` now includes a production release workflow for the AS stack:

- build on push to `main` after PR merge
- build one kernel image with `docker buildx`
- push a SHA-tagged image to the configured registry repository
- deploy by immutable image digest, not by mutable tag
- keep deployment config in GitHub environment vars and secrets, not in the repo

The production topology currently deployed from `main` is:

- `as-webd`
- `as-registryd`
- `as-apid`
- `as-materializerd`
- `execd` colocated with `webd` in the same pod

That last point matters. Realization execution in shared deployment is still
source-backed today: the runtime image contains the repo tree and Go toolchain,
and realizations are launched from source rather than from sealed per-
realization images.

See [kernel/runbook.md](kernel/runbook.md) for the operational release flow and
[docs/seed_execution_model.md](docs/seed_execution_model.md) for the current
execution model and its remaining gap to packaged realization runtimes.

## Learn More

- [seeds/README.md](seeds/README.md) — seed model and authoring guide
- [kernel/architecture.md](kernel/architecture.md) — kernel structure and responsibilities
- [kernel/public_object_registry.md](kernel/public_object_registry.md) — the registry model in depth
- [kernel/public_schema_object_registry.md](kernel/public_schema_object_registry.md) — schemas as first-class objects
- [kernel/philosophy.md](kernel/philosophy.md) — runtime selection, feedback loops, and deeper concepts
- [kernel/protocol/v1/growth.md](kernel/protocol/v1/growth.md) — seed packet and growth-job contract
- [genesis/](genesis/) — the founding seed
