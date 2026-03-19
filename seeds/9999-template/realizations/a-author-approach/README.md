# Realization Template

This directory is a starter realization for the seed template.

Use one realization directory for one concrete implementation attempt or one
accepted implementation of the seed.

Document boundaries:

- `README.md` explains this realization only.
- `realization.yaml` records machine-readable realization metadata, including
  the source approach.
- `interaction_contract.yaml` records the operational API that both the UI and
  machine clients must use, including UI-to-command parity, runtime-only agent
  context, and useful authenticated error expectations when those matter.
- `artifacts/` holds concrete generated output for this realization.
- `validation/` holds realization-specific validation evidence.
- `notes.md` holds realization-local notes.
