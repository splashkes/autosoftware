# Realizations

`realizations/` holds compiled intent for the registry browser seed.

The first planned realization is `a-authoritative-browser`, a read-only
browser that makes authoritative registry state understandable to both humans
and agents.

Every realization intended to run must include:

- `realization.yaml` for machine-readable realization metadata
- `interaction_contract.yaml` for the normalized command/projection contract
- `artifacts/` for concrete outputs
