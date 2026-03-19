# AS Plan: Gap Closure and Brainstorming Capture

## Purpose

This document groups the missing pieces identified in review and points to the follow-up plan documents that close them.

The goal is not to re-explain AS from scratch. The goal is to ensure the repo captures the strongest parts of the design work so far and does not drift into a mushy or incomplete model.

---

## What Needed to Be Added

The review surfaced nine major gaps:

1. The artifact model was under-specified.
2. The GitHub mutation workflow was not explicit enough.
3. Commit order and semantic lineage were too easy to confuse.
4. The single-owner bootstrap story was not prominent enough.
5. The stable-kernel / fast-evolving-app split needed to be elevated.
6. The local-first contributor workflow needed a concrete operating model.
7. Activation and deployment needed a clearer state model.
8. The privacy model needed a stronger object / claim / artifact split.
9. Propagation needed a cleaner registry-vs-streams separation.

---

## New Plan Documents

### 01-artifacts-storage-and-integrity-plan.md
Covers:
- artifact key + hash model
- schema-defined prefixes
- canonical hashing rules
- portability across GitHub / S3 / OCI / IPFS
- encryption and re-encryption
- large artifact handling

### 02-github-mutations-and-review-plan.md
Covers:
- PR as mutation unit
- commits as exploration only
- GitHub checks as ingest gate
- mutation metadata vs claims
- local dev to PR to merge to ingest
- why GitHub is the human control plane, not the canonical ledger

### 03-runtime-bootstrap-activation-and-local-dev-plan.md
Covers:
- single-owner v1
- seed Go runtime shape: BFF + worker + Postgres
- stable kernel vs fast-evolving surfaces
- local full-app dev world
- mutation simulation and replay
- activation, deployment modes, rollback and pinning

### 04-registry-lineage-privacy-and-propagation-plan.md
Covers:
- object identity vs claim lineage
- row order vs supersession
- active-head indexes rather than naive version numbers
- private claims and artifact deletion
- registry rows vs derived streams
- multi-registry federation

### 14-onsite-event-admin-and-entry-reclassification-plan.md
Covers:
- the live onsite flower-show operator workflow
- class correction during judging
- intake without photos
- show credits versus permissions
- full-show thumbnail board expectations
- scoped judge-support and intake-operator access

### 15-flowershow-public-ui-cleanup-plan.md
Covers:
- public-home restructuring into upcoming shows, active clubs, and past shows
- photo-led show cards and browse cards
- simpler responsive nav
- clubs as a first-class public surface
- browse retention without over-promoting niche nav items
- stronger public exhibitor-profile navigation

---

## Cross-Cutting Principles

These principles should now appear consistently across docs.

### 1. Claims are append-only; artifacts are movable
Claims preserve lineage.
Artifacts preserve content identity through hashes.
Artifact locations may change through schema prefix updates.

### 2. GitHub is for authoring and review
The registry is for accepted runtime truth.
Do not push GitHub process metadata into claims.

### 3. The worker is the compiler of current reality
The worker syncs accepted rows, resolves artifacts, verifies hashes, and materializes the runtime state.

### 4. The runtime kernel should become boring
The kernel should eventually change slowly.
App definitions, UI artifacts, workflows, algorithms, and schemas should change quickly.

### 5. Start with a single owner
Early systems do not need community governance.
They need reliable mutation, replay, review, and rollback.

---

## Suggested Near-Term Repo Follow-Through

1. Add protocol docs for artifacts, lineage, and activation.
2. Add a GitHub integration section under kernel or plans.
3. Add a local-dev document showing the full dev-world loop.
4. Make artifact hashing and prefix-based relocation explicit in the protocol.
5. Make PR-as-mutation and merge-as-ingest explicit in the README or kernel docs.

---

## One-Sentence Summary

AS should be documented as a system where accepted mutations flow from GitHub review into an append-only registry, where a worker compiles current reality from claims and artifacts, and where storage, privacy, activation, and portability are all handled by explicit protocol rules rather than hidden conventions.
