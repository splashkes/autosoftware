# API And Playwright Plan

This plan is for the next validation pass on `a-firstbloom`. The goal is not
just "more tests"; it is to prove that the declared contract, the HTMX UI, and
the remote-agent API stay at parity or better for authenticated agents.

## Objectives

1. Prove the interaction contract is load-valid and operationally truthful.
2. Prove normal admin work can be done through the semantic API with either a
   session or a `service_token`.
3. Prove the public and admin UIs expose the same core surface that agents use.
4. Add a real-browser validation path for the deployed admin OTP flow using
   `simon@plashkes.com`.

## Test Layers

### 1. Contract-Load And Schema Tests

Owner: Go tests in `kernel/internal/realizations/contracts_test.go`

Add or extend checks for:

- every flower-show `ui_surface` references a declared command or projection
- every projection path declared in the contract has a live handler in the app
- every command with `runtime_context` declares at least one non-runtime result
  projection
- every command with `error_codes` has matching negative-path tests

Outcome: contract drift fails before runtime.

### 2. App-Local API Conformance Tests

Owner: `artifacts/flowershow-app/main_test.go`

Expand coverage from smoke tests to a command/projection matrix:

- command success path for each parity command introduced in PR `#76`
- command auth matrix: anonymous denied, session allowed, `service_token`
  allowed when declared
- command validation matrix: malformed JSON, missing required fields, invalid
  referenced ids, forbidden role
- projection auth matrix: anonymous/public, session/private, service token
  private
- read-your-writes checks: command result is visible in the declared projection
- runtime-context checks: accepted in request, absent from persisted
  projections

High-priority commands to add next:

- `judges.assign`
- `divisions.create`
- `sections.create`
- `entries.set_visibility`
- `classes.compute_placements`
- `media.attach`
- `media.delete`
- `roles.assign`

High-priority projections to add next:

- `GET /v1/projections/0007-Flowershow/entries/{id}`
- `GET /v1/projections/0007-Flowershow/classes/{id}`
- `GET /v1/projections/0007-Flowershow/shows/{id}/workspace`

### 3. Local Deterministic Playwright

Owner: `/Users/splash/as-flower-agent/autosoftware/tests`

Split the current `tests/flowershow.spec.ts` into smaller concern-driven specs:

- `flowershow.public.spec.ts`
  Public pages, taxonomy, privacy, leaderboard, widget visibility.
- `flowershow.admin.local.spec.ts`
  Local password-admin flow, HTMX mutations, schedule edits, visibility toggles.
- `flowershow.api.spec.ts`
  Contract discovery, structured error rendering, service-token fetches via
  `request`.
- `flowershow.widget.spec.ts`
  Shared agent-access widget, contract links, and stable-id affordances.

Why split:

- failures will localize by concern
- public checks can stay fast and parallel-friendly
- admin flows can own setup/teardown state explicitly

Local deterministic browser coverage should add:

- widget presence on home, show detail, class detail, and admin pages
- contract link navigation from the widget
- admin creation of division, section, class, and judge assignment
- visibility toggles reflected immediately in public views
- placement recomputation reflected in workspace and public summary
- structured error rendering for failed authenticated calls surfaced in the UI

### 4. Remote Admin OTP Playwright

Owner: manual or headed local run against the deployed environment

This suite should validate the real auth stack instead of the local password
shortcut.

Recommended approach:

1. Add a setup spec that runs headed with `PLAYWRIGHT_SKIP_WEBSERVER=1`.
2. Open the deployed flower-show admin login page.
3. Start the Cognito/OTP flow for `simon@plashkes.com`.
4. Pause for manual OTP entry.
5. Save Playwright `storageState` to a local temp file such as
   `tests/.auth/flowershow-admin.json`.
6. Reuse that storage state in the authenticated remote specs.

Recommended environment variables:

- `FLOWERSHOW_BASE_URL`
- `FLOWERSHOW_ADMIN_EMAIL=simon@plashkes.com`
- `PLAYWRIGHT_SKIP_WEBSERVER=1`

Recommended remote spec split:

- `flowershow.remote-auth.setup.ts`
- `flowershow.remote-admin.spec.ts`
- `flowershow.remote-agent-api.spec.ts`

Remote admin checks should cover:

- OTP login succeeds and lands in the admin dashboard
- admin pages expose the widget and contract links
- admin show workspace loads with live data
- HTMX mutations still work on the deployed stack
- role-gated pages reject non-admin state if a downgraded session is used

This suite should stay out of default CI because OTP is human-mediated.

### 5. Remote Service-Token API Smoke

Owner: local operator run or workflow-dispatch job with secrets

Use a real deployed `service_token` to verify:

- `/v1/contracts` and the self contract resolve remotely
- a private workspace projection is reachable
- a mutation command succeeds
- malformed JSON and permission failures return the structured error envelope

This can be automated separately from OTP because it does not require manual
interaction once the secret exists.

## Coverage Matrix To Reach

### Anonymous

- public projections
- public pages
- denied access to admin/workspace/private projections
- denied access to commands

### Session Admin

- admin dashboard and show workspace
- HTMX partial flows
- page widget parity and contract discoverability
- all normal admin mutations

### Service Token

- contract discovery
- private by-id projections
- command parity with admin for normal operational tasks
- structured error envelope on failures

## Execution Order

1. Expand Go API tests for every new parity command and private projection.
2. Split the current Playwright flower-show file into public/admin/api/widget
   suites.
3. Add browser coverage for widget visibility, visibility toggles, and
   placement recomputation.
4. Add a headed OTP setup path for `simon@plashkes.com`.
5. Add remote service-token smoke coverage.
6. Wire the deterministic Playwright suites into CI once split and stable.
7. Keep the OTP suite as manual or workflow-dispatch until a safe auth harness
   exists.

## Exit Criteria

This plan is complete when:

- every command introduced for UI/API parity has both success and negative-path
  API coverage
- every private projection has auth and payload-shape coverage
- the shared agent-access widget is covered on both public and admin pages
- local Playwright proves the HTMX UI uses the same surface remote agents use
- a headed remote run proves the real admin OTP flow works for
  `simon@plashkes.com`
