# Seeds

Each seed is a full change capsule for one requested change.
It holds source intent plus the realizations compiled from that intent.

It should be easy for a human or agent to understand the intent, design,
acceptance, realizations, and publication metadata by opening one seed
directory.

`0000-genesis` is a linked view of the founding seed stored at `genesis/`.
`9999-template` is the copyable starter seed for new work.

To start a new seed:

- copy `9999-template/`, or
- copy the closest existing seed and adapt it

To start a new realization inside a seed:

- copy the closest existing realization, or
- start from the template realization inside `9999-template/`

Name realization directories with short descriptive IDs such as
`a-author-approach` or `b-structured-html`, not numeric sequence folders.

Copying another seed is an authoring convenience, not kernel-level lineage.
Seeds should be treated as self-contained change capsules unless they
explicitly encode relationships in their own contents.

Copying another realization is also an authoring convenience, not kernel-level
lineage.

The kernel should process the accepted realization, the accepted change set,
and the claims they carry, not infer semantics from which folder was copied to
make the seed or realization.

Runtime should still target realizations directly.
If a user or tool selects a seed, that seed must resolve to a realization by
explicit policy.

## Seed Model

A seed is source intent.
A realization is compiled intent.

The intended seed document split is:

- `README.md` for the seed-local map and boundaries
- `brief.md` for request, purpose, and success criteria
- `design.md` for the shape and boundary of this specific change
- `approaches/` for named realization approaches
- `decision_log.md` for durable seed-local rationale
- `notes.md` for working notes and review context
- `acceptance.md` for acceptance criteria that apply across realizations
- `realizations/` for concrete compiled outputs and their realization-local
  evidence

The intended collaboration split is:

- `brief.md` captures what the user is asking for
- `design.md` is where the user and agent confirm the initial design direction
- `approaches/` records the named approaches available for realization
- `decision_log.md` records only the durable choices that survive iteration
- `acceptance.md` records how the seed will be judged at the end
- `notes.md` keeps the transient journey out of the durable docs

A seed may produce multiple realizations over time.
As the kernel, prompts, or coding agents improve, the same seed may compile to
a different codebase.
That is expected.

The runtime target is still the realization, not the unresolved seed.

Concrete files and artifact payloads belong under `realizations/<id>/artifacts/`,
not at the seed root.
Each realization should point back to one approach.
Every realization intended to run as an app must also declare
`interaction_contract.yaml` beside `realization.yaml` so its operational API is
traceable from seed docs to shared kernel capabilities.

The founding seed is special:

- `genesis/` is the canonical first seed
- `seeds/0000-genesis` points to the same seed from the normal authoring
  surface
