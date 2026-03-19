# Acceptance Checklist

This checklist combines:

- the seed-level acceptance criteria in
  [`../../acceptance.md`](../../acceptance.md)
- the agent/API parity work merged in PR `#76`

Use it as the release acceptance list for `a-firstbloom`.

## Test Modes

- `Local deterministic`: run the Go app in demo mode and use the built-in admin
  password flow.
- `API auth`: call the semantic endpoints with `Authorization: Bearer ...`.
- `Remote admin`: hit the deployed app and complete the OTP login for
  `simon@plashkes.com`.

## Release-Specific Checks From The Agent/API Parity Work

- [ ] Contract discovery returns both `GET /v1/contracts` and
  `GET /v1/contracts/0007-Flowershow/a-firstbloom`.
  Current automated evidence: `TestContractsEndpointsReturnLocalContract`.
  Manual follow-up: open the home-page widget and verify the links render and
  load.

- [ ] The contract includes `kernel_agent_principles`,
  `seed_agent_principles`, `ui_surfaces`, `runtime_context`, and
  `error_codes`.
  Current automated evidence: `TestContractsEndpointsReturnLocalContract`,
  `kernel/internal/realizations/contracts_test.go`.
  Gap: add a runtime conformance check that the declared `ui_surfaces` still
  match live handlers.

- [ ] The shared agent-access widget appears on public and operator-facing
  pages.
  Current automated evidence: `TestHomePageLoads`.
  Gap: add browser checks for show detail, class detail, and admin pages.

- [ ] Authenticated callers receive structured errors with `code`, `hint`,
  `request_id`, `auth_mode`, and `contract_ref`.
  Current automated evidence: `TestCommandEndpointsRequireAuth`,
  `TestCommandEndpointsReturnUsefulStructuredErrorsForAuthenticatedCallers`.
  Gap: add projection and malformed-path error coverage.

- [ ] Authenticated agents can inspect private workspace and by-id projections
  with stable ids.
  Current automated evidence: `TestShowWorkspaceProjectionAcceptsServiceToken`,
  `TestProjectionsReturnJSON`.
  Gap: add explicit tests for `entries/{id}` and `classes/{id}`.

- [ ] The admin operations promoted into the semantic API are callable through
  session or `service_token` auth.
  Verify at minimum: `shows.create`, `shows.update`, `schedules.upsert`,
  `judges.assign`, `divisions.create`, `sections.create`,
  `entries.set_visibility`, `classes.compute_placements`, `media.attach`,
  `media.delete`, `roles.assign`.
  Current automated evidence: `TestCommandEndpointsAcceptServiceToken`,
  `TestScheduleUpsertCommandCreatesSchedule`.
  Gap: dedicated tests are still needed for most of the new parity commands.

## Core Functionality

- [ ] Shows can be created and browsed.
  Current automated evidence: `TestHomePageLoads`, `TestShowDetailBySlug`,
  `TestAPIShowsDirectory`, Playwright `"home page loads with seeded shows"`,
  `"show detail page displays schedule and entries"`.

- [ ] Entries can be created, linked, and ranked with placements.
  Current automated evidence: `TestFullAPIFlow`, `TestStoreMemoryBasics`,
  `TestScorecardAndPlacement`, Playwright `"admin CRUD: create show, add classes, add entries, set placements"`.

- [ ] Entries link to people and classes.
  Current automated evidence: `TestEntryDetail`, `TestClassBrowse`,
  `TestStoreMemoryBasics`.

## Schedule Hierarchy

- [ ] A show exposes a schedule with divisions, sections, and classes.
  Current automated evidence: `TestClassBrowse`,
  `TestScheduleUpsertCommandCreatesSchedule`, Playwright
  `"class browse shows schedule hierarchy"`.

- [ ] Entries resolve through `show_class_id`-style class identity instead of a
  free-floating category.
  Current automated evidence: `TestStoreMemoryBasics`.
  Gap: add an API-level assertion on the entry payload shape.

## Governance, Provenance, And Rules

- [ ] Standard editions can be registered and cited.
  Current automated evidence: `TestEffectiveRulesForClass`,
  `TestStoreMemoryBasics`.

- [ ] Source documents and citations can be attached with provenance metadata.
  Current automated evidence: `TestStoreMemoryBasics`, `TestIngestionImportAPI`.
  Gap: add API-level create/read tests for source documents and citations.

- [ ] Standard rules and class overrides combine correctly.
  Current automated evidence: `TestEffectiveRulesForClass`.

- [ ] Runtime-only authoring context can travel with commands without becoming
  canonical flower-show data.
  Current automated evidence: contract declaration only.
  Gap: add request/response tests proving runtime context is accepted and not
  persisted into projections.

## Rubric Scoring, Awards, And Leaderboards

- [ ] Rubrics, criteria, and scorecards support per-criterion scoring.
  Current automated evidence: `TestScorecardAndPlacement`,
  `TestScorecardRequiresAssignedJudge`.

- [ ] Placements and awards compute correctly.
  Current automated evidence: `TestScorecardAndPlacement`,
  `TestStoreMemoryBasics`.
  Gap: add explicit API command coverage for `awards.compute` and
  `classes.compute_placements`.

- [ ] Leaderboards render correctly.
  Current automated evidence: `TestLeaderboard`, `TestLeaderboardAllOrganizations`,
  Playwright `"leaderboard displays rankings"`.

## Taxonomy, Privacy, And Media

- [ ] Taxonomy browse and detail views cross-link related entries.
  Current automated evidence: `TestTaxonomyBrowse`, Playwright
  `"taxonomy browse lists taxons"`, `"taxon detail shows related entries"`.

- [ ] Public pages show initials only and suppression hides entries without
  deletion.
  Current automated evidence: `TestEntryDetail`,
  `TestSuppressedEntryHiddenFromPublicViews`, Playwright
  `"entry detail shows initials only (privacy)"`.

- [ ] Media upload and rendering works with expected restrictions.
  Current automated evidence: `TestMediaUploadAndRender`,
  `TestMediaUploadRejectsHEIC`.
  Gap: add browser coverage for upload/delete flows and video handling.

## Authentication And Admin

- [ ] Session-based admin login works locally.
  Current automated evidence: `TestAdminLoginFlow`,
  Playwright `"admin login and dashboard"`.

- [ ] Cognito/OTP admin login works on the deployed app for
  `simon@plashkes.com`.
  Current automated evidence: none.
  Manual acceptance path: complete OTP login in headed Playwright and save
  storage state for the authenticated suite.

- [ ] Admin pages enforce auth and render governance/scoring controls.
  Current automated evidence: `TestAdminRequiresAuth`,
  `TestAdminAllowsCognitoSessionWithAdminRole`,
  `TestAdminShowDetailIncludesGovernanceAndScoringControls`,
  `TestHTMXJudgeAssignReturnsInfoPanel`.

- [ ] `service_token` auth can perform the normal remote-agent operations.
  Current automated evidence: `TestCommandEndpointsAcceptServiceToken`,
  `TestShowWorkspaceProjectionAcceptsServiceToken`.
  Gap: broaden to the rest of the parity commands and private projections.
