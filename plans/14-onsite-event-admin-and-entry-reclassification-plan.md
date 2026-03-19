# AS Plan: Onsite Event Admin And Entry Reclassification

## Purpose

Flower-show already has the beginnings of a live operator workspace, but the
current admin surface is still closer to a generic control panel than to the
actual "people around a table during a show" workflow.

This plan closes the specific gap you called out:

- judges sometimes change the class of an entry
- an easy onsite support role must be able to make that change quickly
- the same interface should work before the show and during the show
- setup, intake, photo capture, class correction, and live operations should
  feel like one coherent workspace

## Goal

Make the event admin page work as the operational workspace for the whole show
lifecycle:

- before the show: schedule setup, class ordering, judge assignment, credits
- during intake: add entries quickly, optionally without photos
- during judging: move entries between classes, correct entrant assignment,
  attach media later, and keep all operators in sync via SSE
- during presentation: inspect the whole show in thumbnail mode across all
  classes and entries

All normal operations must remain reachable through declared API commands and
projections, not just the HTMX interface.

## Current State Snapshot

The current realization already provides some of the needed foundation.

### What Exists Now

- The main operator page already exists at
  `/admin/shows/{showID}` in
  [`show_admin.html`](/Users/splash/as-flower-agent/autosoftware/seeds/0007-Flowershow/realizations/a-firstbloom/artifacts/flowershow-app/templates/show_admin.html).
- It already supports SSE-driven partial refresh for the operator panels and
  toast notifications.
- Setup operations already exist for:
  - show profile update
  - schedule governance create/update
  - division create
  - section create
  - class create
  - show-level judge assignment
- Entry operations already exist for:
  - entry create
  - placement set
  - visibility suppress/restore
  - photo upload
  - media delete
- The current entry form already allows adding an entry without photos.
- The API already exposes semantic commands for:
  - `schedules.upsert`
  - `judges.assign`
  - `divisions.create`
  - `sections.create`
  - `classes.create`
  - `entries.create`
  - `entries.update`
  - `entries.set_visibility`
  - `classes.compute_placements`
  - `media.attach`
  - `media.delete`

### What Is Only Partial

- Entries can be updated through `entries.update`, but there is no operator UI
  for move-entry or change-entrant.
- The store supports `updateClass`, but the current admin UI does not expose
  class editing.
- Divisions and sections have `sort_order`, but classes do not. That means
  class reordering is not modeled properly yet.
- Judges are assigned at the show level. There is not yet a clear class-level
  judging assignment or judging-workflow workspace.
- SSE exists, but it is panel-refresh oriented. There is not yet a richer
  event-wide operational stream or a dedicated thumbnail board view.

### What Is Missing

- Move entry to a different class as a first-class operator action
- Delete entry
- Change assigned entrant on an entry
- Fast filtered entrant lookup by club members and guests
- Explicit support for intake without photos followed by later photo attach
  as a visible workflow, not just an implicit possibility
- Show credits such as designer, host, scribe, announcer, or photographer
- Full-show thumbnail mode across all classes and all entries
- Class reordering UI and API
- A scoped onsite support role designed for floor operations and judge support
- Explicit audit trail and explanation when a class is corrected during judging

## Non-Negotiable Constraints

### 1. Class Correction Is A Normal Workflow

Reclassifying an entry should not be treated as an edge case or hidden admin
repair. It is a normal live-show operation and should be obvious and fast.

### 2. Operational Roles And Credits Must Stay Separate

Permissions determine what a user may do.
Credits describe who contributed to the show page.

Examples:

- `show_judge_support` is a permission bundle
- `scribe` or `designer` is a show credit label

Do not overload credits as permission state.

### 3. The Same Workspace Must Work Before And During The Show

Do not split "setup UI" and "live event UI" into unrelated surfaces unless a
very strong reason emerges. Operators should learn one workspace.

### 4. UI/API/Agent Parity Still Applies

Any new operator action must have:

- a semantic command
- a projection or workspace read model
- useful authenticated errors
- test coverage for both UI and API use

### 5. Broadcast Every Material Live Change

If one operator changes a class, entrant, media state, placement, or credits,
other operators looking at the same show should see the update without reload.

## Recommended Domain Additions

### A. Entry Reclassification Event

Add a first-class way to record that an entry moved from one class to another.

This should preserve:

- entry id
- prior class id
- new class id
- actor subject id
- timestamp
- optional reason

This can be modeled as:

- append-only ledger claim
- plus current materialized entry state

The important point is that class correction remains inspectable.

### B. Show Credit Record

Add a free-form `show_credit` style object, for example:

- `show_id`
- `person_id` or free-text display name
- `credit_label`
- `sort_order`
- optional notes

Examples:

- host
- designer
- scribe
- announcer
- photographer
- registrar

This belongs on the page and in the API, but not in the permissions model.

### C. Onsite Support Capability Bundle

Define a scoped authority bundle for easy floor operations.

Suggested bundle:

- `show_intake_operator`
  - add entry
  - update entry
  - move entry between classes
  - change entrant assignment
  - attach/delete media
  - view private workspace
  - not allowed to grant roles
  - not necessarily allowed to publish awards or manage org authority

Optional narrower variant:

- `show_judge_support`
  - everything above except broader show setup mutations

The exact bundle split can be refined later, but the plan should assume this is
not equivalent to full admin.

### D. Entrant Lookup Model

Entry intake should resolve people quickly from club context.

The eventual lookup should support:

- members of the hosting club
- guests attached to the show or club
- fast filtered name search
- "not found" path to create a temporary guest/person record

This implies a stronger people-plus-organization relationship than the current
plain person select.

## Workspace Shape

The current tabbed admin page should evolve into one onsite operations
workspace with clear task lanes.

Suggested lanes:

- `Setup`
  - show profile
  - schedule
  - class ordering
  - judges
  - show credits
- `Intake`
  - add entry starting from a selected class
  - fast entrant lookup
  - capture cultivar / entrant wording exactly as provided
  - add entry without photos
  - flag entries needing photos
- `Floor`
  - move entry
  - change entrant
  - delete mistaken entry
  - attach/remove photos
  - quick class correction after judging review
- `Scoring`
  - scorecards
  - placements
  - compute placements
- `Board`
  - full-show thumbnail mode
  - all classes
  - all entries
  - visual status of missing photos / moved entries / judged entries
- `Governance`
  - standards
  - sources
  - citations
  - rules
  - overrides

This does not require six separate routes. It does require a clearer workflow
than the current generic tabs.

## Commands And Projections To Add Or Tighten

### Commands

Add or formalize these semantic commands:

- `classes.update`
- `classes.reorder`
- `entries.move`
- `entries.delete`
- `entries.reassign_entrant`
- `show_credits.create`
- `show_credits.update`
- `show_credits.delete`
- `persons.lookup`
  or an equivalent private projection optimized for typeahead

`entries.update` may technically cover some of this already, but separate
commands are preferred for the live workflow because they make intent clearer,
validation better, and audit trails more legible.

### Projections

Add or improve these projections:

- `shows/{id}/workspace`
  with explicit intake, floor, and board slices
- `shows/{id}/entries/board`
  full-show thumbnail mode
- `shows/{id}/people/lookup`
  private filtered lookup for members/guests
- `entries/{id}`
  with visible reclassification history
- `shows/{id}/credits`

## Data Model Changes

### Phase-Critical Changes

- add class-level `sort_order`
- add delete semantics for entries
- add explicit move/reassign entry support
- add show-credit records

### Likely Supporting Changes

- stronger person-to-organization relationship for member/guest lookup
- optional entry workflow fields such as:
  - `photo_status`
  - `intake_status`
  - `needs_review`
  - `last_class_change_at`

These do not all need to land immediately, but the plan should not ignore the
operational state model.

## Phases

### Phase 0: Acceptance And Contract Alignment

Update flower-show acceptance and contract expectations so this onsite workflow
is explicit.

Add acceptance language for:

- class correction during judging
- onsite support role
- show credits
- class reordering
- entry move / delete / entrant reassignment
- full-show thumbnail board
- filtered entrant lookup

Exit condition:

- the desired live operator workflow is documented as required behavior rather
  than informal chat intent

### Phase 1: Data Model And Command Surface

Implement the missing domain and command primitives:

- class `sort_order`
- `entries.move`
- `entries.delete`
- `entries.reassign_entrant`
- `classes.update`
- `classes.reorder`
- `show_credits.*`

Exit condition:

- the semantic API can express every target workflow mutation cleanly

### Phase 2: Authority And Role Bundles

Add the scoped onsite operator support bundle and map commands to capability
requirements.

Focus on:

- `show_admin`
- `show_intake_operator`
- `show_judge_support`

Exit condition:

- the floor-support workflow no longer requires giving broad admin access to
  everyone helping during a show

### Phase 3: Workspace Redesign

Refactor the current admin page into a clearer operational workspace:

- setup lane
- intake lane
- floor correction lane
- board / thumbnail lane

Keep it usable before and during the show.

Exit condition:

- operators can do intake, correction, media attach, and setup without hunting
  across unrelated tabs

### Phase 4: People Lookup And Credits

Implement:

- fast filtered person lookup
- member/guest scoping
- quick-create guest path
- free-form show credits

Exit condition:

- entry intake is fast and credits are visible without polluting permissions

### Phase 5: SSE And Thumbnail Board

Broaden live updates so all material show-state changes propagate to:

- intake workspace
- floor workspace
- board / thumbnail view

Build the full-show thumbnail board.

Exit condition:

- multiple operators can watch the entire show change in real time

### Phase 6: Tests And Live Validation

Add comprehensive coverage for:

- API command tests
- UI Playwright flows
- concurrent SSE update behavior
- role-specific permission tests

Required scenarios:

- add entry without photo, then attach photo later
- judge-support operator moves an entry to a different class
- operator changes entrant on an entry
- mistaken entry is deleted
- classes are reordered and public browse reflects it
- full-show thumbnail board updates live after intake changes

Exit condition:

- the onsite workflow is proven locally and in live headed validation

## Recommended Execution Order

1. Acceptance and contract updates
2. Data-model and command additions
3. Scoped onsite support role
4. Workspace redesign
5. People lookup and credits
6. Thumbnail board and SSE broadening
7. Full API and Playwright validation

## Success Criteria

This plan is complete when all of these are true:

- a judge-support operator can reclassify an entry quickly without broad admin
  authority
- class changes are visible and auditable
- entries can be added before photos and completed later
- classes can be reordered explicitly
- entrants can be corrected quickly
- mistaken entries can be deleted cleanly
- show credits exist as free-form display records
- the operator workspace is the same surface used for setup and live admin
- the full show can be inspected in thumbnail mode across all classes and
  entries
- all of the above are available through both the UI and the semantic API
