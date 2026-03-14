# AS Plan: GitHub, Mutations, Review, and Ingestion

## Purpose

This document defines the GitHub-side mutation workflow.

The key principle is that GitHub should handle authoring, collaboration, validation, and review, while the registry should record only accepted runtime truth.

---

## Core Mapping

The clean mapping is:

- commit = exploration step
- pull request = mutation proposal
- merge = accepted mutation
- registry ingest = canonicalization of the accepted mutation

A mutation is therefore analogous to a PR, not a commit.

This allows an agent to make many commits while searching, fixing, rewriting, and retesting, but keeps the registry free of exploratory thrash.

---

## Repository Model

Each AS app should have its own repo.

Examples:
- `as-crm`
- `as-todo`
- `as-support`
- `as-artbattle`

These repos are not the registry.
They are the human- and AI-editable projection of the app.

Typical contents:
- artifacts
- schemas
- algorithms
- UI definitions
- tests
- mutation templates

---

## Mutation Unit

A PR should carry the whole proposed mutation.

That may include:
- new objects
- new claims
- new artifacts
- superseding claims
- tests
- screenshots or render diffs
- a mutation manifest

After review and merge, only the final merged tree is ingested.

---

## Why PR, Not Commit

A single commit is too small and too noisy.

The AI coding tool may need many commits to reach a good result.
Those commits are useful for exploration but should not become canonical system history.

The reviewed and merged PR is the right level of acceptance.

---

## CI and Security Gates

Registry ingest should only happen after required GitHub checks pass.

Baseline checks:
- unit tests
- integration tests
- worker replay/materialization test
- artifact hash validation
- secret scanning
- dependency vulnerability scan
- static analysis/linting

Higher-risk mutations may add:
- schema compatibility checks
- policy/permissions diff checks
- algorithm replay or simulation checks
- UI snapshot and render diffs

No successful checks means no merge.
No merge means no ingest.

---

## Important Separation

GitHub check results are not claims.

Keep these as mutation/process metadata:
- PR number
- merge commit SHA
- CI status
- scan results
- reviewers
- approvals

Claims should represent application state only.

Examples of claim-worthy facts:
- algorithm.active
- css.version
- workflow.definition
- task.title

Examples of non-claim metadata:
- tests_passed
- security_scan_passed
- PR approved by Alice

Do not pollute the registry with governance-process state.

---

## Ingestion Flow

After merge:

1. ingester reads the merged PR snapshot
2. parses mutation contents
3. resolves and hashes artifacts
4. generates object/claim rows
5. appends rows to the registry atomically
6. emits a wake signal/event for workers

The final merged state is the only state that matters to the registry.

---

## Mutation Manifest

Each PR/mutation should include machine-readable metadata.

Recommended contents:
- mutation title
- target app
- scope/impact summary
- affected objects
- affected schemas
- deployment mode
- risk level
- tests expected to pass
- rollback anchor if available

This is review metadata, not system-state metadata.

---

## Local Development Loop

An early supporter or coding agent should be able to:

1. clone the app repo
2. boot the full local dev world
3. make changes
4. run replay + tests locally
5. open a PR
6. let cloud CI re-validate
7. merge when acceptable
8. let ingest update the registry

This creates a practical bridge between local coding and cloud deployment.

---

## Deployment Trigger

Deployment is not triggered by commit.
Deployment is not triggered by PR open.

Deployment is triggered by:
- merge of an approved PR
- successful ingest into the registry
- successful worker replay/materialization

The worker is the actual deployment engine.
GitHub is the mutation review layer.

---

## GitHub’s Proper Role

GitHub should be treated as the human control plane for mutation.

It gives AS, for free:
- branching
- PR review
- CI
- security scanning
- contributor identity
- comments/discussion
- audit of the authoring process

The registry should remain the machine control plane for accepted state.

---

## Design Rule to Carry Forward

GitHub handles proposed change.
The registry handles accepted reality.
The worker handles deployment by replay.

That keeps authoring, truth, and execution cleanly separated.
