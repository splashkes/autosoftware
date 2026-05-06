# Live Anonymous Red-Team Harness

This directory holds the repeatable harness, manifests, and reporting templates
for live anonymous testing against the public Autosoftware surface.

## Layout

- `scenarios.json` defines the shared scenario schema used by the HTTP runner
  and the Playwright browser suite.
- `ownership-map.md` maps scenario owner keys to likely code owners in the repo.
- `report-template.md` is the concise finding format for each wave.
- `reports/` is the runtime output area for baselines, evidence, and wave runs.

## Default safety model

- Run during `01:00` to `05:00` in `America/Toronto` unless the operator
  explicitly overrides the window.
- Anonymous only: no credentials, no off-platform account creation, no login
  guessing, and no social engineering.
- Manual-only scenarios are skipped unless the operator passes an explicit
  opt-in.
- Registry-host scenarios are health-gated. If `registry.autosoftware.app` is
  degraded, those scenarios skip while the main-site and API dependency checks
  still run.

## Shared scenario schema

Each scenario entry in `scenarios.json` uses the same top-level shape:

- `id`
- `wave`
- `driver`
- `host`
- `class`
- `requestTemplate`
- `preconditions`
- `maxRequests`
- `maxConcurrency`
- `abortCondition`
- `expectedSafeBehavior`
- `evidenceRequirements`

The harness also uses optional fields such as `ownerKey`, `expectedStatuses`,
`manualOnly`, and `healthGate` to make runs repeatable and machine-checkable.

## Commands

List HTTP scenarios:

```bash
node scripts/live-redteam-http.mjs --list
```

Run one HTTP wave:

```bash
node scripts/live-redteam-http.mjs --wave 1
```

Run the live browser suite:

```bash
cd tests
npm run test:live
```

Run the hybrid wave wrapper:

```bash
scripts/live-redteam-wave.sh --wave 3
```

Include manual launch probes:

```bash
scripts/live-redteam-wave.sh --wave 2 --include-manual
```

## Evidence output

The default output root is:

`plans/live-anonymous-red-team/reports/runs/<run_id>/`

Each run writes:

- `http/summary.json` and `http/evidence.jsonl`
- `browser/` Playwright screenshots, HAR files, and per-scenario JSON evidence
- `baseline.json` when Wave 1 scenarios are part of the run

The latest computed Wave 1 baseline is also copied to:

`plans/live-anonymous-red-team/reports/latest-baseline.json`

This file is ignored by git and can be reused by later waves with
`--baseline`.
