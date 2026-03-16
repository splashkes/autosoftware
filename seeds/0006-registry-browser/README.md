> **Before you start.** Read these first:
> [EVOLUTION_QUICK.md](../../EVOLUTION_QUICK.md) (one page) |
> [EVOLUTION_INSTRUCTIONS.md](../../EVOLUTION_INSTRUCTIONS.md) (full guide) |
> [Seed Execution Model](../../docs/seed_execution_model.md) (how seeds become running software)

# Registry Browser Seed

This seed defines an authoritative, read-only browser for the public registry.

Primary surfaces in this seed:

- simple registry overview with clear entry points for non-experts
- browsable and searchable lists of objects, claims, schemas, change sets, and
  rows
- deep detail views showing provenance, supersession, schema versions, and
  accepted history
- explicit API guidance so agents can use the same information without
  scraping HTML

Current planned realizations:

- `a-authoritative-browser` as the direct catalog browser
- `a-ledger-reading-room` as a human-first refinement that reorganizes the
  same accepted registry state around systems, governed things, actions, read
  models, contracts, and registry internals
