# AS Plan: Artifacts, Storage, and Integrity

## Purpose

This document closes the storage and artifact-model gaps.

The central design decision is that claims should identify content, while schemas should define storage policy.

---

## Core Model

A claim should not store a full location URI.
A claim should store:

- `artifact_key`
- `artifact_hash`

A schema should define:

- `artifact_prefix`
- optional encryption and canonicalization rules
- optional hash algorithm rules

Artifact resolution is:

`artifact_locator = artifact_prefix + artifact_key`

This keeps claims stable while allowing storage backends to change.

---

## Why Prefixes Belong in Schemas

If 50,000 images move from GitHub to S3, or from S3 to another provider, the system should not need 50,000 new claims.

Instead:
- existing claims keep the same `artifact_key`
- existing claims keep the same `artifact_hash`
- schema updates the `artifact_prefix`

This works as long as the artifact bytes do not change.

---

## Artifact Hashing Rule

The hash must be computed over the canonical artifact bytes only.

Included in the hash:
- file contents
- deterministic normalized JSON/YAML bytes, if canonicalization is defined
- exact binary bytes

Excluded from the hash:
- file paths
- Git commit metadata
- timestamps
- HTTP headers
- storage provider metadata
- compression container metadata not part of the canonical artifact

The rule is:

`artifact_hash = hash(canonical_artifact_bytes)`

This is what makes artifact relocation safe.

---

## Canonicalization

Canonicalization should be schema-defined and deterministic.

Examples:
- plain text: UTF-8 bytes only
- JSON: normalized ordering if required
- YAML: normalized form if required
- images and binaries: raw file bytes

If canonicalization is not explicitly defined, use the artifact bytes exactly as stored.

---

## Artifact Sources

The protocol should support multiple backends through prefixes.

Examples:
- `github://as-crm/<commit_sha>/artifacts/`
- `s3://crm-images-prod/`
- `oci://registry.example.com/as/`
- `ipfs://<cid>/`
- `file:///var/as/artifacts/`

The worker only needs two things:
- a resolver for the prefix family
- the expected `artifact_hash`

---

## GitHub as Early Artifact Store

In early phases, many artifacts should simply be files in the repo.

Examples:
- HTML fragments
- CSS bundles
- workflow YAML
- seed definitions
- algorithm code
- schema definitions

A practical early locator is:

`github://as-crm/<commit_sha>/artifacts/ui/task_form.html`

This gives:
- immutability through commit pinning
- strong reviewability
- easy local development
- easy migration later

---

## Large Artifacts

Large artifacts should use the same model.

The claim still stores:
- `artifact_key`
- `artifact_hash`

The schema still stores:
- `artifact_prefix`

The backend may change depending on size.

Examples:
- small text/config artifacts in Git
- large images in object storage
- large datasets or models in OCI, LFS, or cloud buckets

The worker may stream or reference large artifacts rather than fully loading them into memory.

---

## Encryption Model

Artifacts may be stored publicly but encrypted.

The worker resolves, fetches, decrypts, verifies, and applies.

Recommended model:
- encrypted blob in storage
- decryption keys held by worker/KMS context
- schema identifies encryption policy or key class

For clarity, support both:
- `ciphertext_hash`
- `plaintext_hash`

This is especially useful when the same plaintext is re-encrypted under a new key or method.

---

## Re-Encryption Plan

If encryption changes, do not mutate accepted claims in place.

Instead:
1. fetch old artifact
2. decrypt
3. re-encrypt with new key/method
4. write new artifact
5. create a new claim referencing the new artifact
6. optionally supersede the old claim

If the plaintext is unchanged, the `plaintext_hash` should remain the same while the `ciphertext_hash` changes.

---

## Artifact Deletion

Claims are append-only.
Artifacts may be deleted if policy allows.

If an artifact is deleted:
- the claim still exists
- lineage still exists
- the worker should treat the claim as unavailable/inert unless mirrored or restored

This allows private data to be removed without erasing the fact that a claim once existed.

---

## Worker Responsibilities

For every artifact-bearing claim, the worker should:

1. resolve `artifact_locator` from schema prefix + artifact key
2. fetch the artifact
3. decrypt if required
4. canonicalize if required
5. compute hash
6. compare with expected hash
7. accept or reject the artifact

Artifacts failing verification must never be applied.

---

## Design Rule to Carry Forward

Claims hold identity of content.
Schemas hold location policy.
Hashes prove content.
Workers enforce integrity.

That separation keeps storage portable, replay deterministic, and claims stable even when providers change.
