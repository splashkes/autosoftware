# Realization Testing and Live Validation

This plan covers the full testing lifecycle for realizations: what happens
before acceptance, during deployment, while running live, and on an ongoing
basis. The goal is a testing model that matches the registry's append-only
nature — tests are claims about realizations, not transient CI artifacts
that disappear after a green check.

---

## Principles

1. **Tests are claims.** A test result is an assertion about a realization at
   a point in time. It should be recorded in the registry like any other
   claim, not buried in CI logs. This makes test history replayable and
   auditable.

2. **Acceptance criteria are seed-level, not realization-level.** The seed
   defines what "done" means. Every realization compiled from that seed must
   satisfy the same acceptance criteria, regardless of approach or agent.

3. **Live validation is not optional.** A realization that passes pre-deploy
   checks but fails under real traffic has not been validated. The feedback
   loop (client incidents, request events) is part of the test surface.

4. **Comparison across realizations is a feature.** The same seed may produce
   multiple realizations. Testing should make it easy to compare quality,
   performance, and correctness across them — not just pass/fail one at a time.

---

## Phase 1: Pre-Acceptance Validation

Before a realization is accepted into the registry.

### 1.1 Contract Conformance

Every realization that declares an `interaction_contract.yaml` makes a
testable promise: these are my commands, projections, and capabilities.

- **Automated contract checker:** parse the contract, issue each declared
  command and projection against the running realization, verify response
  shapes match declared schemas.
- **Missing endpoint detection:** flag commands or projections declared in the
  contract but returning 404 or wrong content types.
- **Capability verification:** if the contract declares `principals` or
  `sessions` as required capabilities, verify the realization actually calls
  the kernel's identity endpoints rather than rolling its own.

### 1.2 Acceptance Criteria Automation

Each seed has an `acceptance.md` with human-readable criteria. These should
become machine-executable.

- **Structured acceptance format:** extend `acceptance.md` with a parseable
  checklist block (or add `acceptance.yaml` alongside it) so criteria can be
  enumerated programmatically.
- **Acceptance test runner:** a kernel tool that reads the structured criteria,
  maps each to a test function or HTTP probe, and produces a pass/fail report
  per criterion.
- **Agent-authored tests:** when a growth agent produces a realization, it
  should also produce acceptance test stubs that map back to the seed's
  criteria. These live in `realizations/<id>/validation/`.

### 1.3 Sandbox Execution

Run the realization in isolation before it touches shared state.

- **Ephemeral runtime:** spin up the realization using its `runtime.yaml`
  manifest in a throwaway environment (container, temp directory, ephemeral
  port).
- **Test data fixtures:** each seed can declare fixture data in
  `seeds/<id>/fixtures/` — initial state the realization should handle
  correctly.
- **Deterministic replay:** materialize registry state up to the point just
  before this realization, inject the candidate realization's claims, replay
  the materializer, and compare output against expected state.

### 1.4 Security Scan

Automated checks before acceptance, not after.

- **Static analysis:** run the realization's artifacts through language-
  specific linters and security scanners (go vet, gosec, eslint-security).
- **Dependency audit:** check declared dependencies against known
  vulnerability databases.
- **Seed boundary enforcement:** verify the realization does not attempt to
  read kernel-internal paths, access the runtime database directly, or
  shell out to undeclared commands.

### 1.5 Agent Review

AI-assisted code review as a recorded claim.

- **Review agent:** an automated reviewer that reads the seed docs, the
  interaction contract, and the realization artifacts, then produces a
  structured review (findings, severity, recommendations).
- **Review claim:** the review is appended to the registry as an
  `agent_review` claim against the realization object. This makes it
  queryable and auditable — not a PR comment that gets lost.
- **Blocking findings:** findings marked `blocking` prevent acceptance
  until resolved or overridden by a human.

---

## Phase 2: Deployment

After acceptance, before live traffic.

### 2.1 Staged Activation

Not every accepted realization should go live immediately.

- **Activation modes:** `auto` (immediate), `staged` (canary first),
  `manual` (explicit operator action), `local_only` (never deployed).
  Configured per-seed or per-realization in the manifest.
- **Canary routing:** use the existing subdomain/path-prefix routing to
  direct a subset of traffic to the new realization while the previous
  version continues serving the rest. Example: `canary-notepad.localhost`
  routes to the candidate while `notepad.localhost` stays on the current
  accepted version.
- **Promotion gate:** canary must run for a configurable window (time or
  request count) with zero blocking incidents before promotion to primary.

### 2.2 Artifact Integrity

Verify what's running is what was accepted.

- **Hash verification:** on deploy, the kernel computes artifact hashes and
  compares them against the hashes recorded in the registry at acceptance
  time. Mismatch = abort.
- **Provenance chain:** every deployed artifact should be traceable back to:
  which growth job produced it, which seed packet it consumed, which agent
  or human authored it, and which registry row accepted it.

### 2.3 Health Gate

The realization must prove it can start before receiving traffic.

- **Startup probe:** after launching the realization process, the kernel
  polls a health endpoint (declared in `runtime.yaml` or defaulting to
  `GET /`) with a configurable timeout. Failure = rollback.
- **Dependency readiness:** if the realization declares kernel capabilities
  (sessions, search, communications), verify those kernel services are
  reachable from the realization's runtime before opening traffic.

---

## Phase 3: Live Validation

While the realization is serving real traffic.

### 3.1 Feedback Loop Activation

The client-side incident reporter and request event recorder already exist.
Wire them to the testing surface.

- **SQL backing for MemoryStore:** replace the in-memory feedback loop store
  with the Postgres-backed `runtime_client_incidents` and
  `runtime_request_events` tables. This is already schematized; the Go
  implementation needs the persistence layer.
- **Realization-scoped dashboards:** every incident and request event is
  tagged with `realization_id` and `seed_id`. The boot console should
  surface per-realization error rates and latency distributions, not just
  a global log.
- **Alert thresholds:** configurable per-realization. If error rate exceeds
  threshold within a window, the kernel can automatically deactivate the
  realization and fall back to the previous accepted version.

### 3.2 Synthetic Monitoring

Automated probes against the live realization, not just passive observation.

- **Smoke probes:** lightweight HTTP checks against declared commands and
  projections on a schedule (every 30s–5m). These are the same contract
  conformance checks from Phase 1, run continuously.
- **User flow probes:** scripted multi-step sequences that exercise critical
  paths. For the notepad seed: create note, verify it appears, edit it,
  verify the edit, delete it. These scripts live in
  `realizations/<id>/validation/probes/`.
- **Cross-realization comparison probes:** if two realizations of the same
  seed are both running, issue the same probe to both and compare response
  correctness, latency, and payload size.

### 3.3 Shadow Traffic

Test a candidate realization against real requests without exposing users
to it.

- **Request mirroring:** the kernel's routing middleware can duplicate
  incoming requests to a shadow realization. Responses from the shadow are
  recorded but not returned to the user.
- **Diff analysis:** compare shadow responses against primary responses.
  Differences in structure, status codes, or payload semantics are flagged
  for review.
- **Safe by default:** shadow traffic is read-only. If the realization has
  write side effects (creating records, sending messages), the shadow
  instance runs against an isolated database or a dry-run mode declared
  in its runtime manifest.

---

## Phase 4: Ongoing

Continuous validation after the realization is established.

### 4.1 Drift Detection

Does the materialized state still match what the registry says it should be?

- **Periodic replay:** on a schedule, the materializer replays accepted
  registry history from scratch and compares the result against current
  materialized state. Any divergence is a drift incident.
- **Incremental checksum:** after each materialization pass, compute a
  checksum of the output. Store it as a claim. Compare against the next
  pass. If the same registry state produces different checksums, something
  is non-deterministic.

### 4.2 Acceptance Re-Verification

Acceptance criteria can regress even without code changes (dependency
updates, data growth, infrastructure shifts).

- **Scheduled acceptance runs:** re-run the seed's acceptance test suite
  against the live realization on a cadence (daily, weekly). Failures
  create incidents, not silent logs.
- **Cross-version comparison:** when a new realization is accepted for
  the same seed, run the acceptance suite against both old and new
  versions and produce a comparison report.

### 4.3 Performance Baselines

Detect regressions before users notice them.

- **Baseline capture:** after a realization is promoted to primary, record
  p50/p95/p99 latency and error rates from the first N hours as the
  baseline.
- **Regression detection:** continuous comparison against baseline. If p95
  latency increases by more than a configurable threshold (e.g., 2x),
  create an incident and optionally trigger canary rollback.
- **Baseline as claim:** store performance baselines as registry claims
  against the realization object. This makes them part of the historical
  record and enables cross-realization performance comparison.

### 4.4 Agent-Driven Exploration

Let AI agents probe running realizations for issues humans wouldn't think
to test.

- **Exploration agent:** given the seed docs and interaction contract, an
  agent generates novel inputs, edge cases, and adversarial sequences
  against the running realization. Findings are recorded as `agent_review`
  claims.
- **Mutation testing:** the agent proposes small perturbations to the
  realization (remove a validation, change a default, swap an algorithm)
  and observes whether the acceptance tests catch the regression. This
  measures test quality, not just code quality.
- **Regression hunting:** after each new acceptance, the agent specifically
  targets areas changed between the old and new realization to verify no
  regressions in adjacent behavior.

---

## Implementation Priority

| Item | Phase | Dependencies | Effort |
|------|-------|-------------|--------|
| SQL-backed feedback loop store | 3.1 | Schema exists, needs Go persistence layer | Small |
| Contract conformance checker | 1.1 | interaction_contract.yaml parser exists | Medium |
| Structured acceptance.yaml | 1.2 | Convention decision | Small |
| Health gate on startup | 2.3 | runtime.yaml already declares process | Small |
| Realization-scoped error view in boot console | 3.1 | SQL-backed feedback loop | Medium |
| Synthetic smoke probes | 3.2 | Contract conformance checker | Medium |
| Canary routing via subdomain | 2.1 | Routing middleware exists | Medium |
| Sandbox execution runner | 1.3 | runtime.yaml launcher | Large |
| Agent review pipeline | 1.5 | Growth job system exists | Large |
| Shadow traffic mirroring | 3.3 | Routing middleware exists | Large |
| Drift detection via replay | 4.1 | Deterministic materializer | Large |
| Performance baseline claims | 4.3 | SQL-backed request events | Medium |

The smallest high-value starting point is **SQL-backed feedback loop** (3.1)
because the schema and client-side reporter already exist. Once incidents
persist, realization-scoped error views and alert thresholds follow naturally.

The second priority is **contract conformance** (1.1) because it turns the
interaction contract from documentation into an executable test suite with
no manual test authoring required.

---

## Relationship to Existing Plans

- **Plan 02 (GitHub Mutations):** CI gates described there become the
  automation layer for Phase 1. This plan defines *what* those gates
  should check.
- **Plan 03 (Runtime Bootstrap):** activation modes and deployment triggers
  described there align with Phase 2. This plan adds health gates, canary
  routing, and artifact verification.
- **Plan 05 (Additional Architecture):** mutation safety, deterministic
  replay, and observability sections in that plan are inputs to Phases 1
  and 4 here. This plan makes them concrete.

---

## Open Questions

1. **Test claim schema:** what does an `acceptance_result` or `test_run`
   claim look like in the registry? Fields: realization reference, test
   suite ID, pass/fail per criterion, execution timestamp, agent/human
   attribution, environment fingerprint.

2. **Canary traffic split:** percentage-based (10% canary) or deterministic
   (specific sessions routed to canary)? Deterministic is easier to debug.
   Percentage-based is more realistic.

3. **Shadow traffic scope:** should shadow mirroring be opt-in per
   realization, or always-on for staged deployments? Opt-in is safer but
   requires configuration discipline.

4. **Baseline window:** how long should a new realization run before its
   performance is captured as baseline? Too short = noisy. Too long =
   delayed regression detection.

5. **Cross-registry test claims:** if a federated registry accepts a
   realization, should test claims from our public registry be trusted?
   Or must each authority run its own validation?
