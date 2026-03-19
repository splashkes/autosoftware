# AS Plan: Shared Runtime Session Persistence And Identity Binding

## Purpose

Autosoftware needs browser and agent login state that survives reboots,
redeploys, and app restarts without turning every seed into its own secret and
session silo.

The immediate problem showed up in flower-show:

- browser login state depended on a process-local cookie signing secret
- authorization state could still disappear because some role data lived in
  memory

This plan turns persistent session handling into a kernel-aligned runtime
pattern instead of a seed-local workaround.

## Goal

Make login persistence work like this across apps:

- external auth provider proves identity
- runtime binds provider identity to a stable internal subject
- runtime stores durable session rows in Postgres
- browser stores only an opaque session handle cookie
- current permissions are resolved from durable internal state
- reboot does not discard valid signed-in state

## Non-Negotiable Constraints

### 1. Registry Is Not Secret Storage

Do not put live auth secrets or cookie-master keys in the public registry.

### 2. Browser Cookies Stay Opaque

Do not embed the full identity or permission payload in browser cookies for
the durable model.

### 3. Identity And Authority Stay Separate

Auth provider identity proves who the caller is.
System-native authority still decides what the caller may do.

### 4. Multi-App Reuse Matters

The session mechanism should serve many apps with different auth providers.

## Deliverables

### Kernel-Level Deliverables

- durable runtime subject-binding model
- durable runtime session model
- provider registration and identity binding guidance
- runtime secret-storage guidance
- projections or APIs for session lookup and revocation

### Seed-Level Deliverables

- opaque browser session cookies
- durable role or authority lookup
- no process-random login invalidation on restart
- tests covering continuity across a fresh app instance

## Phases

### Phase 0: Doctrine

Update the living permissions/auth doc so the durable session model is
explicit:

- runtime sessions
- runtime auth identities
- runtime principals
- runtime secrets versus registry truth
- multi-app provider mapping

Exit condition:

- the repository has one canonical explanation of durable runtime sessions

### Phase 1: Runtime Session Plumbing

Make the kernel runtime tables and APIs the canonical shape for:

- auth provider identity bindings
- session creation
- session resolution
- session ending and revocation

Exit condition:

- the runtime layer can own durable session rows for many apps

### Phase 2: First Seed Adoption

Adopt the model in flower-show:

- create or resolve internal subject on login
- create runtime-backed session row
- issue opaque browser session cookie
- resolve signed-in user from runtime storage
- persist role state in Postgres instead of memory

Exit condition:

- flower-show sessions and permissions survive normal reboots

### Phase 3: Shared Runtime Client Or Adapter

Choose and standardize one reuse path for seeds:

- direct runtime-table adapter where the seed shares the runtime database
- or kernel runtime HTTP client for app-to-kernel calls

Exit condition:

- new seeds have a clear integration path instead of inventing local auth
  persistence

### Phase 4: Revocation And Session Management

Add operator-visible and account-visible controls for:

- session listing
- session revocation
- device or browser naming
- rolling or idle expiry if desired

Exit condition:

- session persistence is not just durable, but manageable

## Flower-Show Slice Delivered

The first execution slice implemented here covers:

- opaque runtime-backed browser sessions
- runtime subject binding from Cognito identity
- Postgres-backed role persistence
- local and browser tests proving the new session flow

It intentionally does not complete the full kernel-native authority ledger
migration yet.

## Validation Expectations

- Go tests verify opaque session cookies and continuity across a fresh app
  instance using the same session store
- Playwright local tests authenticate through a gated local test session path
  that mints real opaque sessions instead of forging cookies
- deployed environments should keep users signed in across normal restarts once
  runtime storage remains intact
