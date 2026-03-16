# Realizations

`realizations/` holds compiled intent for the registry browser seed.

Current planned realizations:

- `a-authoritative-browser`, a direct read-only catalog browser that keeps the
  authoritative routes close to the surface
- `a-ledger-reading-room`, a refinement realization that reorganizes the same
  registry state for human understanding without changing kernel behavior

Every realization intended to run must include:

- `realization.yaml` for machine-readable realization metadata
- `interaction_contract.yaml` for the normalized command/projection contract
- `artifacts/` for concrete outputs
