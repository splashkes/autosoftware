# AS Plan: Registry, Lineage, Privacy, and Propagation

## Purpose

This document closes the core protocol gaps around lineage, private data, version semantics, propagation, and federation.

The goal is to keep the registry simple and authoritative while allowing systems to scale, federate, and handle private data sanely.

---

## Registry as Append-Only Row Ledger

The registry should be described as an append-only row ledger.

Each row is durable and ordered.

Examples:
- `object.create`
- `claim.create`

The important point is that append order is not the same as semantic version lineage.

---

## Commit Order vs Semantic Lineage

This distinction should be made explicit.

### Commit order
Represented by:
- `row_id`

Meaning:
- what was committed later in registry time

### Semantic lineage
Represented by:
- `supersedes_claim_id`
- active-head indexes
- activation choices

Meaning:
- what replaces what
- what is current for a given subject/schema

A higher `row_id` does not necessarily mean “newest version” in a semantic sense.
Rows for many unrelated subjects can be interleaved.

---

## Do We Need Version Numbers?

Do not make version numbers a core primitive of truth.

Why:
- lineage may branch
- two claims can both supersede the same earlier claim
- “highest version” may not mean canonical or active

Instead:
- use `supersedes_claim_id` as truth
- materialize active-head indexes for fast lookup
- optionally materialize lineage depth if convenient

This avoids pretending the world is always a clean linear sequence.

---

## Objects, Claims, and Privacy

A crucial rule from the brainstorming:

- objects may be publicly addressable
- claims may be private
- artifacts may be private, encrypted, deleted, mirrored, or moved

This is important because a public object registry is relatively easy, but claims often carry sensitive meaning.

Examples of sensitive claims:
- payout status
- internal reliability note
- customer support note
- moderation decision
- private algorithm definition

The protocol should not assume claims are public by default.

---

## What the Claims Ledger Actually Resolves

A claims ledger should preserve lineage and authorship even when claim content is secret.

In the registry, a claim should be able to preserve:
- that a claim exists
- what object it is about
- which schema it uses
- who authored it
- when it was created
- what it supersedes
- what hash the artifact had

The artifact content itself may remain private or later be deleted.

This allows:
- auditability
- accountability
- replay where possible
- privacy where required

---

## Artifact Deletion and Redaction

Claims should remain append-only.
Artifacts may be removed according to policy.

If a private artifact is deleted:
- the claim remains in the ledger
- lineage remains
- the worker should treat the claim as unavailable/inert unless mirrored

This keeps historical responsibility without forcing permanent retention of sensitive content.

---

## Propagation Model

Do not force the registry itself to “be Kafka.”

Cleaner split:
- registry = authoritative durable store
- streams/events = derived propagation layer

The registry stores truth.
Streams notify interested workers.

---

## Wake Signals vs Truth

Events should be treated as wake signals, not as authoritative truth.

Worker pattern:
1. receive event or wake signal
2. inspect row/offset progress
3. pull from the registry or local mirror
4. replay accepted rows
5. update runtime state

The ledger remains the source of truth.

---

## Scaling Propagation

At large scale, one global stream is too blunt.

Use derived channels/topics partitioned by interest, such as:
- claim type
- object type
- tenant
- authority
- subject hash
- domain family

Workers should keep offsets per channel/topic.

This gives:
- bounded replays
- selective subscriptions
- practical scale

But again: streams are derived; the registry is canonical.

---

## Federation and Multiple Registries

AS should allow multiple registries.

Each registry acts as an authority and should have:
- `registry_id`
- signing identity/public key

Object and claim references should be globally qualified by authority.

Examples:
- `as:<uuid>`
- `acme:<uuid>`
- `artbattle:<uuid>`

Workers should be able to trust and sync from multiple registries.

This allows:
- public registries
- private organization registries
- community registries
- local experimental registries

---

## Interoperability Rule

Different systems may fork:
- UI
- workflows
- local schemas
- algorithms

But they can still collaborate if they share:
- stable object identities
- core claims they understand
- claim/artifact integrity rules

Unknown claims and schemas can be ignored by systems that do not understand them.

---

## Design Rule to Carry Forward

Keep these concepts cleanly separated:

- `row_id` = commit order
- `supersedes_claim_id` = semantic lineage
- active-head index = fast current-state lookup
- claims = append-only truth records
- artifacts = content behind claims
- streams = propagation convenience
- registry = canonical history

That separation removes a lot of hidden confusion and makes replay, privacy, and federation much easier to reason about.
