> **Before you start.** Read these first:
> [EVOLUTION_QUICK.md](../../EVOLUTION_QUICK.md) (one page) |
> [EVOLUTION_INSTRUCTIONS.md](../../EVOLUTION_INSTRUCTIONS.md) (full guide) |
> [Seed Execution Model](../../docs/seed_execution_model.md) (how seeds become running software)

# Event Listings Seed

This seed defines a public event publishing product with an organizer admin
surface and public calendar views.

Its canonical long-term model is graph-first: events, versions, venues,
organizers, media, and related objects should remain reusable by alternate
clients rather than being trapped in one flattened event record.

Primary surfaces in this seed:

- organizer event management
- public calendar and list views
- event detail pages
- category and date-based discovery

Boundary docs in this seed:

- [PUBLIC_PRIVATE_DATA.md](PUBLIC_PRIVATE_DATA.md) for the seed-local
  shared/public/private/runtime split across the event graph
