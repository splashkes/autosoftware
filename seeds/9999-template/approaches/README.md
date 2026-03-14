# Approaches

`approaches/` holds named, machine-readable approaches for this seed.
Each approach should usually be one YAML file such as
`a-author-approach.yaml`.

Use one approach when different realization strategies should be compared
explicitly, such as:

- different coding rules or rewrite strategies
- different model selections
- different prompt profiles
- different UI or data-shape approaches

Each realization should point back to one approach through
`realization.yaml:approach_id`.
