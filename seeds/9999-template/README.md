# Seed Template

Copy this seed when starting a new change.
You can also copy a nearby existing seed if it is the closest starting point.

The goal is to make the path from idea to accepted change explicit and easy to
follow for both humans and agents.

That copy step is only an authoring shortcut.
The kernel should not infer semantic lineage from it.

This template also includes a starter realization under `realizations/`.
Use descriptive realization IDs such as `a-author-approach` or
`b-structured-html`.

Document boundaries:

- `README.md` explains the structure of this seed only.
- `brief.md` captures the incoming request in user language.
- `design.md` explains the design response, including the early user-validation
  checkpoint before implementation.
- `AUTOSOFTWARE_AGENT_PRINCIPLES.md` records the seed-local agent-access rules
  that specialize the stable kernel doctrine.
- `PUBLIC_PRIVATE_DATA.md` defines the seed-local shared/public/private/runtime
  data boundary when the seed needs one, ideally in graph-first terms.
- `approaches/` defines the named YAML approaches that realizations may
  implement.
- `decision_log.md` records durable seed-local choices and tradeoffs.
- `notes.md` captures working notes, review context, and open questions.
- `acceptance.md` defines what every realization of this seed must satisfy.
- `seed.yaml` holds machine-readable metadata for publication.
- `realizations/` holds compiled realizations of this seed and their artifacts.

Do not duplicate kernel architecture, kernel decisions, or kernel runbook
content here. A seed should only explain the change it carries.
Concrete artifacts should live under `realizations/<id>/artifacts/`, not beside
the seed documents.
Every runnable realization must also define `interaction_contract.yaml` so the
same commands and projections can serve the UI and machine clients, and so the
seed's objects plus relations are machine-readable.
That contract should also make the seed's agent-access doctrine concrete enough
to test.

The normal collaboration path is:

1. capture the user request in `brief.md`
2. answer it with an early design in `design.md`
3. validate that design with the user before implementation begins
4. define one or more named approaches under `approaches/`
5. record durable choices in `decision_log.md`
6. record end-state success criteria in `acceptance.md`
7. keep iteration detail in `notes.md`
8. write or update `AUTOSOFTWARE_AGENT_PRINCIPLES.md` for seed-local agent
   expectations
9. define the realization interaction contract under `realizations/<id>/`
10. place concrete outputs under `realizations/`
