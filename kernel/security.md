# Kernel Security Model

This document describes the kernel's security posture, trust boundaries,
existing protections, hardening steps, and regression testing expectations.

The line references below link to the current `main` branch on GitHub.

**Base URL for references:**
`https://github.com/splashkes/autosoftware/blob/main/`

---

## Trust Architecture

### The Kernel-Seed Boundary

Seeds are untrusted. A seed's Go code, YAML manifests, and artifact
declarations could be authored by any contributor and must be treated as
potentially hostile input. The kernel reads seed metadata but must never
trust it to be well-formed, bounded, or honest.

**What seeds control:**

| Seed-authored artifact | Consumed by kernel at | Risk |
|------------------------|----------------------|------|
| `realization.yaml` — `artifacts` list | [`catalog.go:23`](https://github.com/splashkes/autosoftware/blob/main/kernel/internal/realizations/catalog.go#L23) | Path traversal via `../` in artifact paths |
| `realization.yaml` — `seed_id`, `realization_id` | [`catalog.go:18-19`](https://github.com/splashkes/autosoftware/blob/main/kernel/internal/realizations/catalog.go#L18-L19) | Path traversal in persistence output paths |
| `runtime.yaml` — `command`, `args`, `entrypoint` | [`meta.go:19-33`](https://github.com/splashkes/autosoftware/blob/main/kernel/internal/realizations/meta.go#L19-L33) | Arbitrary command execution (if ever invoked) |
| `runtime.yaml` — `environment` map | [`meta.go:26`](https://github.com/splashkes/autosoftware/blob/main/kernel/internal/realizations/meta.go#L26) | Environment variable injection |
| `interaction_contract.yaml` — `$ref` paths | [`contracts.go:420-434`](https://github.com/splashkes/autosoftware/blob/main/kernel/internal/realizations/contracts.go#L420-L434) | File existence probing outside repo |
| Seed Go source code | Not executed by kernel | Full compromise if kernel ever shells out |

**What the kernel owns (trusted):**

| Resource | Protection |
|----------|------------|
| Runtime database (Postgres) | Connection string passed only to kernel services via env vars; seeds have no access |
| Kernel service binaries | Built from `kernel/cmd/`; never from seed code |
| HTTP endpoints | Registered only in kernel packages (`http/json/`, `cmd/webd/`) |
| Migration SQL | Read from `kernel/db/runtime/*.sql` — kernel-authored, checksummed |

**Current isolation status:** The kernel does not execute seed code. The
`local-run.sh` script only launches kernel services (`apid`, `registryd`,
`materializerd`, `webd`). Seeds are read as static artifacts. However, the
`RuntimeManifest` struct already models a `Run.Command` + `Run.Args` field
([`meta.go:30-33`](https://github.com/splashkes/autosoftware/blob/main/kernel/internal/realizations/meta.go#L30-L33)),
so execution is clearly on the roadmap — and must be sandboxed before it
ships.

### Design Principle

> The kernel must resist attack from within. Any data that originates from
> seed-authored files is untrusted input, even though it lives inside the
> repository.

---

## Current Protections

### SQL Injection Prevention

All database queries use parameterized placeholders (`$1`, `$2`, ...) via the
`pgx` driver. No SQL is constructed through string concatenation.

| Location | Example |
|----------|---------|
| [`kernel/internal/interactions/runtime_identity.go:27-33`](https://github.com/splashkes/autosoftware/blob/main/kernel/internal/interactions/runtime_identity.go#L27-L33) | `CreatePrincipal` — five bind parameters |
| [`kernel/internal/interactions/runtime_discovery.go:22-43`](https://github.com/splashkes/autosoftware/blob/main/kernel/internal/interactions/runtime_discovery.go#L22-L43) | `UpsertSearchDocument` — twelve bind parameters |
| [`kernel/internal/interactions/runtime_communications.go:24-32`](https://github.com/splashkes/autosoftware/blob/main/kernel/internal/interactions/runtime_communications.go#L24-L32) | `CreateThread` — eight bind parameters |
| [`kernel/internal/interactions/runtime_guardrails.go:24-34`](https://github.com/splashkes/autosoftware/blob/main/kernel/internal/interactions/runtime_guardrails.go#L24-L34) | `RecordGuardDecision` — ten bind parameters |

### XSS Prevention

All HTML rendering uses Go's `html/template` package, which auto-escapes
template variables by default. No use of `template.HTML` or raw string
injection into HTML output.

| Location | Detail |
|----------|--------|
| [`kernel/cmd/webd/main.go:7`](https://github.com/splashkes/autosoftware/blob/main/kernel/cmd/webd/main.go#L7) | `html/template` import |
| [`kernel/cmd/webd/main.go:23-238`](https://github.com/splashkes/autosoftware/blob/main/kernel/cmd/webd/main.go#L23-L238) | Template definitions via `template.Must(template.New(...).Parse(...))` |
| [`kernel/cmd/webd/bootloader.go:5`](https://github.com/splashkes/autosoftware/blob/main/kernel/cmd/webd/bootloader.go#L5) | `html/template` import for boot page |

### Content Security Policy

Per-request CSP nonces are generated with `crypto/rand` (18 bytes,
base64-encoded). Scripts and styles require the nonce to execute.

| Location | Detail |
|----------|--------|
| [`kernel/internal/http/server/security.go:133-139`](https://github.com/splashkes/autosoftware/blob/main/kernel/internal/http/server/security.go#L133-L139) | `newCSPNonce()` — `crypto/rand.Read` with 18-byte buffer |
| [`kernel/internal/http/server/security.go:118-131`](https://github.com/splashkes/autosoftware/blob/main/kernel/internal/http/server/security.go#L118-L131) | Default CSP policy with `script-src 'nonce-...'` and `style-src 'nonce-...'` |
| [`kernel/internal/http/server/security.go:47`](https://github.com/splashkes/autosoftware/blob/main/kernel/internal/http/server/security.go#L47) | CSP header set on every response |

### Security Headers

The `DefaultMiddlewareStack` applies three layers to every request:

[`kernel/internal/http/server/security.go:26-32`](https://github.com/splashkes/autosoftware/blob/main/kernel/internal/http/server/security.go#L26-L32)

```
SecurityHeadersMiddleware
  -> CorrelationMiddleware
    -> SameOriginUnsafeMethodsMiddleware
```

Headers set on every response:

| Header | Value | Line |
|--------|-------|------|
| `Content-Security-Policy` | Nonce-gated, self-only | [`security.go:47`](https://github.com/splashkes/autosoftware/blob/main/kernel/internal/http/server/security.go#L47) |
| `Referrer-Policy` | `strict-origin-when-cross-origin` | [`security.go:48`](https://github.com/splashkes/autosoftware/blob/main/kernel/internal/http/server/security.go#L48) |
| `X-Content-Type-Options` | `nosniff` | [`security.go:49`](https://github.com/splashkes/autosoftware/blob/main/kernel/internal/http/server/security.go#L49) |
| `X-Frame-Options` | `DENY` | [`security.go:50`](https://github.com/splashkes/autosoftware/blob/main/kernel/internal/http/server/security.go#L50) |

### CSRF Protection (Same-Origin Enforcement)

The `SameOriginUnsafeMethodsMiddleware` blocks cross-origin state-changing
requests from browsers using the Fetch Metadata pattern:

[`kernel/internal/http/server/security.go:63-106`](https://github.com/splashkes/autosoftware/blob/main/kernel/internal/http/server/security.go#L63-L106)

1. **`Sec-Fetch-Site`** — rejects anything other than `same-origin`, `none`,
   or empty (line [77-83](https://github.com/splashkes/autosoftware/blob/main/kernel/internal/http/server/security.go#L77-L83))
2. **`Origin`** — must match the request target (line [87-94](https://github.com/splashkes/autosoftware/blob/main/kernel/internal/http/server/security.go#L87-L94))
3. **`Referer`** — fallback check when Origin is absent (line [96-102](https://github.com/splashkes/autosoftware/blob/main/kernel/internal/http/server/security.go#L96-L102))

Non-browser clients (which omit all three headers) pass through by design,
since CSRF is a browser-only attack vector.

### Cryptographic Randomness

All identifiers and tokens use `crypto/rand`. No use of `math/rand`.

| Location | Usage |
|----------|-------|
| [`kernel/internal/interactions/runtime_helpers.go:56-62`](https://github.com/splashkes/autosoftware/blob/main/kernel/internal/interactions/runtime_helpers.go#L56-L62) | `newID()` — 8-byte hex entity IDs |
| [`kernel/internal/interactions/runtime_helpers.go:64-70`](https://github.com/splashkes/autosoftware/blob/main/kernel/internal/interactions/runtime_helpers.go#L64-L70) | `newToken()` — 32-byte base64url auth tokens |
| [`kernel/internal/http/server/request_context.go:148-155`](https://github.com/splashkes/autosoftware/blob/main/kernel/internal/http/server/request_context.go#L148-L155) | `newOpaqueID()` — 8-byte hex request/correlation IDs |

### Token Storage

Auth tokens (challenge verifiers, access link tokens) are hashed with SHA-256
before database storage. Raw tokens are never persisted.

[`kernel/internal/interactions/runtime_helpers.go:72-75`](https://github.com/splashkes/autosoftware/blob/main/kernel/internal/interactions/runtime_helpers.go#L72-L75)

```go
func hashToken(token string) string {
    sum := sha256.Sum256([]byte(token))
    return hex.EncodeToString(sum[:])
}
```

### Auth Challenge & Access Link Replay Protection

Both `ConsumeAuthChallenge` and `ConsumeAccessLink` use transactional locking
(`SELECT ... FOR UPDATE`) to prevent concurrent replay, and check status,
expiration, and use counts atomically before granting access.

| Flow | Location | Protections |
|------|----------|-------------|
| Auth challenge | [`runtime_identity.go:220-234`](https://github.com/splashkes/autosoftware/blob/main/kernel/internal/interactions/runtime_identity.go#L220-L234) | `FOR UPDATE` lock, status/expiry/used_at check, hash comparison |
| Access link | [`runtime_webstate.go:124-140`](https://github.com/splashkes/autosoftware/blob/main/kernel/internal/interactions/runtime_webstate.go#L124-L140) | `FOR UPDATE` lock, status/expiry/revocation/max_uses check |

### Input Validation

JSON request decoding rejects unknown fields and enforces single-object bodies:

[`kernel/internal/http/json/runtime.go:326-339`](https://github.com/splashkes/autosoftware/blob/main/kernel/internal/http/json/runtime.go#L326-L339)

```go
decoder := json.NewDecoder(r.Body)
decoder.DisallowUnknownFields()
```

### No Command Injection Surface

No use of `os/exec`, `exec.Command`, or shell invocations in any kernel Go
code. All operations use Go standard library APIs directly.

---

## Hardening Steps

The items below are defense-in-depth improvements. Items marked with a seed
icon are directly related to the kernel-seed trust boundary.

### H1. Default bind addresses should prefer loopback

**Status:** Open
**Impact:** If a binary is run directly (bypassing `local-run.sh`), it binds
to all interfaces.

| Location | Current Default |
|----------|----------------|
| [`kernel/cmd/apid/main.go:55`](https://github.com/splashkes/autosoftware/blob/main/kernel/cmd/apid/main.go#L55) | `":8092"` |
| [`kernel/cmd/webd/main.go:367`](https://github.com/splashkes/autosoftware/blob/main/kernel/cmd/webd/main.go#L367) | `":8090"` |

The `local-run.sh` script overrides these to `127.0.0.1`, but the Go defaults
should match for safe-by-default behavior.

**Recommendation:** Change defaults from `":port"` to `"127.0.0.1:port"`.

### H2. Path containment for artifact file reads (seed boundary)

**Status:** Open
**Impact:** A `realization.yaml` with a crafted artifact path
(e.g. `../../../etc/passwd`) causes the kernel to read and return arbitrary
file contents via `os.ReadFile`.

| Location | Operation |
|----------|-----------|
| [`kernel/internal/realizations/growth.go:99`](https://github.com/splashkes/autosoftware/blob/main/kernel/internal/realizations/growth.go#L99) | `filepath.Join(realizationDir, filepath.FromSlash(artifact))` |
| [`kernel/internal/realizations/growth.go:111`](https://github.com/splashkes/autosoftware/blob/main/kernel/internal/realizations/growth.go#L111) | `os.ReadFile(path)` with no boundary check |
| [`kernel/internal/materializer/service.go:280`](https://github.com/splashkes/autosoftware/blob/main/kernel/internal/materializer/service.go#L280) | `filepath.Join(entry.RootDir, filepath.FromSlash(artifact))` |
| [`kernel/internal/materializer/service.go:420`](https://github.com/splashkes/autosoftware/blob/main/kernel/internal/materializer/service.go#L420) | `readPreview()` calls `os.ReadFile` |

**Recommendation:** After resolving the full path, verify it stays within the
repository root:

```go
absPath, _ := filepath.Abs(fullPath)
absRoot, _ := filepath.Abs(repoRoot)
if !strings.HasPrefix(absPath, absRoot+string(filepath.Separator)) {
    continue // skip — path escapes repo boundary
}
```

### H3. Path containment for materialization persistence (seed boundary)

**Status:** Open
**Impact:** The `persist()` function constructs output paths from `SeedID` and
`RealizationID`, which originate from seed YAML manifests. The
`NormalizeReference` function strips whitespace and leading/trailing slashes
but does not reject `..` segments.

| Location | Operation |
|----------|-----------|
| [`kernel/internal/realizations/catalog.go:96-98`](https://github.com/splashkes/autosoftware/blob/main/kernel/internal/realizations/catalog.go#L96-L98) | `NormalizeReference` — no `..` rejection |
| [`kernel/internal/materializer/service.go:293`](https://github.com/splashkes/autosoftware/blob/main/kernel/internal/materializer/service.go#L293) | `os.MkdirAll` with unsanitized `SeedID`/`RealizationID` |
| [`kernel/internal/materializer/service.go:304`](https://github.com/splashkes/autosoftware/blob/main/kernel/internal/materializer/service.go#L304) | `os.WriteFile` to constructed path |

**Recommendation:** Reject references containing `..` in `NormalizeReference`
or validate the resolved output path stays within `OutputRoot` before writing.

### H4. Path containment for contract schema references (seed boundary)

**Status:** Open
**Impact:** `validateContractRef` resolves `$ref` values from
`interaction_contract.yaml` files and calls `os.Stat` on the result, leaking
file existence outside the repo boundary.

| Location | Operation |
|----------|-----------|
| [`kernel/internal/realizations/contracts.go:420-434`](https://github.com/splashkes/autosoftware/blob/main/kernel/internal/realizations/contracts.go#L420-L434) | `os.Stat(target)` on seed-authored `$ref` path |

**Recommendation:** After resolving `target`, verify it is under the repo root
before calling `os.Stat`.

### H5. Document the authentication model

**Status:** Open
**Impact:** The session resolution middleware is intentionally non-blocking —
it enriches request context but never rejects unauthenticated requests.

| Location | Behavior |
|----------|----------|
| [`kernel/internal/http/server/session.go:26-51`](https://github.com/splashkes/autosoftware/blob/main/kernel/internal/http/server/session.go#L26-L51) | `SessionResolutionMiddleware` — continues on missing/invalid session |
| [`kernel/internal/http/json/runtime.go:29-59`](https://github.com/splashkes/autosoftware/blob/main/kernel/internal/http/json/runtime.go#L29-L59) | 29 endpoints registered without auth gates |
| [`kernel/cmd/apid/main.go:51-53`](https://github.com/splashkes/autosoftware/blob/main/kernel/cmd/apid/main.go#L51-L53) | Middleware applied without enforcement |

This is safe because services bind to `127.0.0.1` via `local-run.sh`, making
them unreachable from the network. However:

- The Go code defaults to `:port` (all interfaces), which would expose
  unauthenticated endpoints if the binary is run directly.
- Future deployment scenarios will require authentication middleware before
  these endpoints are network-accessible.

**Recommendation:** Add a comment in `runtime.go` documenting the intentional
decision. When the project moves toward deployment, add an auth enforcement
middleware before the route handler.

### H6. Error response sanitization

**Status:** Open
**Impact:** `respondError` passes `err.Error()` directly to clients. Database
errors, file paths, and internal details can leak.

[`kernel/internal/http/json/respond.go:14-16`](https://github.com/splashkes/autosoftware/blob/main/kernel/internal/http/json/respond.go#L14-L16)

```go
func respondError(w http.ResponseWriter, status int, err error) {
    respondJSON(w, status, map[string]string{"error": err.Error()})
}
```

Callers like [`runtime_identity.go:240`](https://github.com/splashkes/autosoftware/blob/main/kernel/internal/interactions/runtime_identity.go#L240)
wrap database errors with `wrapErr()`, which preserves the full error chain
including table names, constraint names, and file paths.

**Recommendation:** Map known error types (not found, conflict, bad input) to
safe client messages. Log the full error server-side. Return only a generic
message for unexpected errors.

### H7. Constant-time token comparison

**Status:** Open
**Impact:** Auth challenge verification uses `!=` to compare hashes, which
could leak timing information.

| Location | Operation |
|----------|-----------|
| [`kernel/internal/interactions/runtime_identity.go:249`](https://github.com/splashkes/autosoftware/blob/main/kernel/internal/interactions/runtime_identity.go#L249) | `if verifierHash != hashToken(verifier)` |

Access link lookup uses `token_hash = $1` as a database `WHERE` clause
([`runtime_webstate.go:127`](https://github.com/splashkes/autosoftware/blob/main/kernel/internal/interactions/runtime_webstate.go#L127)),
which is equivalent to constant-time at the application layer since the
database handles the comparison. However, the auth challenge path compares
in Go.

**Recommendation:** Use `crypto/subtle.ConstantTimeCompare` for the
in-application hash comparison:

```go
if subtle.ConstantTimeCompare([]byte(verifierHash), []byte(hashToken(verifier))) != 1 {
    return AuthChallenge{}, ErrNotFound
}
```

### H8. Runtime manifest execution safety (seed boundary)

**Status:** Open — not yet exploitable (no exec path exists)
**Impact:** The `RuntimeManifest` struct models arbitrary command execution
via seed-authored YAML. When the kernel eventually invokes seed processes,
all fields must be treated as hostile.

[`kernel/internal/realizations/meta.go:19-33`](https://github.com/splashkes/autosoftware/blob/main/kernel/internal/realizations/meta.go#L19-L33)

```go
type RuntimeManifest struct {
    Command          string            // seed-controlled
    Args             []string          // seed-controlled
    Entrypoint       string            // seed-controlled path
    WorkingDirectory string            // seed-controlled path
    Environment      map[string]string // seed-controlled key-value pairs
}
```

**Recommendation:** Before implementing seed execution:

1. **Command allowlist** — only permit known runtimes (`go`, `node`, `python`)
2. **Entrypoint containment** — verify entrypoint path resolves within the
   seed's own directory tree
3. **Working directory containment** — same boundary check
4. **Environment allowlist** — only pass declared, kernel-approved env vars;
   never propagate kernel secrets (`AS_RUNTIME_DATABASE_URL`, etc.)
5. **Process isolation** — run seed processes in a container, chroot, or
   namespace with no access to the kernel's network ports or database

### H9. Remote registry SSRF controls

**Status:** Open
**Impact:** The `RemoteRegistryClient` makes HTTP requests to a URL from the
`AS_REMOTE_REGISTRY_URL` environment variable. While env vars are trusted,
the URL target is not validated for scheme or host restrictions.

| Location | Detail |
|----------|--------|
| [`kernel/internal/materializer/service.go:381`](https://github.com/splashkes/autosoftware/blob/main/kernel/internal/materializer/service.go#L381) | `client = &http.Client{Timeout: 4 * time.Second}` — 4s timeout set |
| [`kernel/internal/materializer/service.go:394`](https://github.com/splashkes/autosoftware/blob/main/kernel/internal/materializer/service.go#L394) | Response body limited to 2048 bytes |

**Recommendation:** If the remote registry URL is ever derived from
non-environment sources, validate scheme (`https://` only) and reject
private/loopback addresses.

---

## Security Regression Testing

### Scope

Security tests should cover two classes of input:

1. **Network input** — HTTP requests to kernel endpoints
2. **Seed-authored input** — YAML manifests and artifact paths that the kernel
   reads from disk

### Test Categories

#### T1. Path traversal containment

Verify that seed-authored paths cannot escape the repository root.

```
Test: artifact path with "../" resolving outside repo root
  Given a realization.yaml with artifacts: ["../../../../etc/passwd"]
  When LoadGrowthContext is called for that realization
  Then the artifact is skipped or an error is returned
  And os.ReadFile is never called on a path outside repoRoot

Test: artifact path with symlink escape
  Given an artifact path that resolves to a symlink pointing outside repo
  When the kernel reads the artifact
  Then the read is rejected

Test: materialization persist with traversal reference
  Given SeedID = "../../tmp" and RealizationID = "evil"
  When persist() is called
  Then os.MkdirAll and os.WriteFile target paths within OutputRoot only

Test: contract $ref with traversal
  Given interaction_contract.yaml with schema_ref: "../../../../etc/shadow"
  When validateContractRef is called
  Then the ref is rejected before os.Stat is reached

Test: approach_id with traversal
  Given realization.yaml with approach_id: "../../../etc/passwd"
  When LoadGrowthContext builds the approach doc path
  Then the path is contained within the seed directory
```

#### T2. SQL injection resistance

Verify no query construction uses string concatenation.

```
Test: special characters in principal display_name
  Given display_name = "'; DROP TABLE runtime_principals; --"
  When CreatePrincipal is called
  Then the principal is created with the literal string as its name
  And no SQL error or table drop occurs

Test: LIKE wildcards in search query
  Given query = "%_[^a]%"
  When SearchDocuments is called
  Then results reflect literal pattern matching via parameterized $2
  And no SQL syntax error occurs
```

#### T3. XSS resistance

Verify template rendering escapes all dynamic content.

```
Test: HTML in seed summary rendered in webd
  Given a realization with summary = "<script>alert(1)</script>"
  When the boot page or growth page renders
  Then the output contains &lt;script&gt; (escaped), not raw <script>

Test: CSP nonce uniqueness
  Given two sequential requests to webd
  Then each response has a different CSP nonce value
  And the nonce is 24+ characters of base64
```

#### T4. Authentication and token security

Verify auth flows resist replay, brute force, and timing attacks.

```
Test: auth challenge consumed twice
  Given a valid challenge that has been consumed
  When ConsumeAuthChallenge is called again with the same verifier
  Then ErrNotFound is returned (status != "pending")

Test: auth challenge with wrong verifier
  Given a pending challenge
  When ConsumeAuthChallenge is called with an incorrect verifier
  Then ErrNotFound is returned
  And the challenge remains pending (not marked used)

Test: expired auth challenge
  Given a challenge with expires_at in the past
  When ConsumeAuthChallenge is called with the correct verifier
  Then ErrNotFound is returned

Test: access link max uses exhausted
  Given an access link with max_uses = 1 and use_count = 1
  When ConsumeAccessLink is called
  Then ErrNotFound is returned

Test: access link revoked
  Given an access link with revoked_at set
  When ConsumeAccessLink is called
  Then ErrNotFound is returned

Test: token never stored in plaintext
  Given a newly created auth challenge or access link
  When the database row is inspected
  Then only the SHA-256 hash is stored, not the raw token
```

#### T5. CSRF enforcement

Verify browser-originated cross-origin mutations are blocked.

```
Test: cross-origin POST with Sec-Fetch-Site: cross-site
  Given a POST request with header Sec-Fetch-Site: cross-site
  When the request reaches SameOriginUnsafeMethodsMiddleware
  Then a 403 is returned

Test: POST with mismatched Origin header
  Given a POST to localhost:8090 with Origin: https://evil.com
  When the request reaches the middleware
  Then a 403 is returned

Test: safe methods bypass CSRF check
  Given a GET request with Sec-Fetch-Site: cross-site
  When the request reaches the middleware
  Then the request passes through
```

#### T6. Error response sanitization

Verify internal details do not leak to clients.

```
Test: database constraint violation
  Given a request that triggers a unique constraint error
  When the error is returned to the client
  Then the response does not contain table names, column names, or SQL

Test: file not found during artifact load
  Given an artifact path that does not exist
  When the kernel attempts to read it
  Then the HTTP response does not reveal the absolute filesystem path
```

#### T7. Input validation

Verify JSON decoding rejects malformed or oversize input.

```
Test: unknown JSON field
  Given a POST body with {"principal_id": "x", "evil_field": true}
  When decodeJSON is called
  Then a 400 error is returned

Test: multiple JSON objects in body
  Given a POST body with two JSON objects concatenated
  When decodeJSON is called
  Then a 400 error is returned
```

#### T8. Runtime manifest safety (seed boundary)

Verify the kernel never blindly trusts seed execution metadata.

```
Test: runtime manifest with command = "rm -rf /"
  Given a runtime.yaml with run.command: "rm" and args: ["-rf", "/"]
  When the kernel loads the manifest
  Then the manifest is loaded for inspection only
  And no process is spawned

Test: runtime environment with kernel secrets
  Given a runtime.yaml with environment: {AS_RUNTIME_DATABASE_URL: "..."}
  When seed execution is eventually implemented
  Then kernel-internal env vars are never propagated to the seed process

Test: entrypoint path traversal
  Given a runtime.yaml with entrypoint: "../../../kernel/cmd/apid/main.go"
  When the kernel resolves the entrypoint
  Then the path is rejected (escapes the seed directory)
```

#### T9. Bind address safety

Verify kernel services default to loopback.

```
Test: apid default address
  When apid starts without AS_APID_ADDR set
  Then it binds to 127.0.0.1:8092, not 0.0.0.0:8092

Test: webd default address
  When webd starts without AS_WEBD_ADDR set
  Then it binds to 127.0.0.1:8090, not 0.0.0.0:8090
```

---

## Review Checklist

When reviewing kernel PRs, verify:

- [ ] New SQL uses `$N` bind parameters — no string interpolation
- [ ] New HTML output uses `html/template` — no raw string rendering
- [ ] New file reads validate the resolved path stays within repo root
- [ ] New endpoints either document their auth model or use an auth middleware
- [ ] New tokens/IDs use `crypto/rand` — never `math/rand`
- [ ] Secrets are hashed before storage — never stored in plaintext
- [ ] Error responses do not leak internal details (paths, SQL, stack traces)
- [ ] Seed-authored data (YAML fields, artifact paths) is never trusted
- [ ] No seed-controlled value is passed to `os/exec` or used in a shell command
- [ ] Token comparisons use constant-time operations
