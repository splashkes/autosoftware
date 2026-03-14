# Realizations

`realizations/` holds compiled intent for this seed.

Each realization is one concrete implementation candidate produced from the
seed's request, design, and constraints.
Each realization should implement one named approach from `../approaches/`.

Use short descriptive realization IDs such as `a-author-approach` or
`b-structured-html`, not numeric sequence folders.

A PR should normally review one realization of one seed.
If the same seed is rerun later, add a new realization instead of overwriting
the old one.

At runtime, the system should pin one realization.
If a seed is selected instead, resolve it to a realization before boot or
materialization.
