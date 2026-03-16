# Realizations

`realizations/` holds compiled intent for this seed.

Each realization is one concrete implementation candidate produced from the
calendar sync hub seed.
Each realization should implement one named approach from `../approaches/`.

Current realization tracks:

- `a-network-operations-hub`
- `b-production-calendar-workbench`

Every realization intended to run must include:

- `realization.yaml` for machine-readable realization metadata
- `interaction_contract.yaml` for the normalized operational contract
- `artifacts/` for concrete outputs
