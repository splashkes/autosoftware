# Flowershow Public UI Cleanup Plan

## Goal

Tighten the public Flowershow experience so it feels more photo-led,
organization-aware, and mobile-usable, while keeping agent/API visibility and
existing browse depth.

This plan is based on a live review of the current public pages and a rough UX
note-taking pass.

## Current State

### Home

Current home page behavior:

- one `Shows` section
- cards are text-only
- no separation between upcoming and past shows
- no organization or club landing section

### Navigation

Current top nav:

- `Flowershow`
- `Shows`
- `Browse`
- `Taxonomy`
- `Leaderboard`
- `Standards`
- `Account`
- `Admin`

Issues:

- too many top-level items
- poor mobile behavior
- private or niche surfaces are overexposed
- signed-in state does not simplify to a person-centric affordance

### Browse

Current browse page is valuable and should remain.

It already supports:

- club filter
- domain filter
- taxon filter
- judge filter
- search

But it is still text-heavy and not photo-led enough.

### Leaderboard

Current leaderboard works as a standalone page, but it likely belongs more
naturally within organization or club context than as a top-level primary nav
surface.

### Account

Current account page is functionally strong but too long and too operational
for normal users.

Current issues:

- top sections are vertically stretched by long permission listings
- `Identity` contains actions that do not really belong in the body content
- `Delegable Permissions` is too prominent for a feature most users will never
  use
- token issuance and token history overwhelm the page for ordinary exhibitors
- the page does not yet feel like a reusable profile shell that can grow into
  balances, participation history, upcoming shows, or other personal views

### Standards

Current standards page is still incomplete as a public surface.

Observed issue:

- it largely acts as a thin list with source-document opening behavior
- it does not yet feel like a real browseable standards information surface

### Admin New Show

Current new-show defaults need cleanup.

Observed issue:

- `Season` is still defaulting to `2025`
- since the current date is in 2026, the default should now be `2026`

### Person Profiles

This should not be lost:

- browse results already link initials to a person page
- show detail entries already link to a person page

So the public exhibitor-profile direction is already partially present and
should be strengthened rather than reinvented.

## Product Direction

### 1. Make Home Show-Centric And Club-Aware

Home should become a richer landing page with:

- upcoming shows
- past shows
- active clubs

Likely order:

1. hero
2. upcoming shows
3. active clubs
4. past shows
5. browse callout

### 2. Make Shows Photo-Led

Show cards should gain a left-side media rail or joined image panel.

Initial direction:

- left-side green-accented image area integrated into the card
- slow rotator between show-related flower images
- approximately 5-second cadence
- graceful placeholder/mock images until real coverage is broader

Important:

- this should work on both home and browse/result cards where relevant
- it should degrade cleanly on mobile

### 3. Simplify Top Navigation

Proposed primary nav:

- `Shows`
- `Clubs`
- `Classes`
- `Browse`

Right side:

- if signed out: `Log in`
- if signed in: person name
- only show `Admin` when the signed-in user actually has admin access

Remove from top-level primary nav:

- `Taxonomy`
- `Leaderboard`
- `Standards`
- `Account`

These should remain reachable, but not elevated as primary global nav items.

### 4. Keep Browse, But Demote It Slightly

Browse should remain because it is useful and distinct.

But its relationship to `Shows` should be clarified:

- home can include a strong browse entry point
- browse should feel like a deeper exploration surface, not the main landing
  model

### 5. Introduce Clubs As A First-Class Public Surface

`Clubs` is the user-facing word.
Underneath, it can still be powered by `organization`.

Club pages should eventually carry:

- club name and parent context
- active shows
- seasonal leaderboard summary
- exhibitors tied to that club
- links into show results and public people profiles

The existing leaderboard should likely be folded into this experience rather
than emphasized as a separate top-level destination.

### 6. Classes Likely Deserve A Public Surface

`Classes` is easier for normal users than `taxonomy` or `standards`.

This likely becomes:

- a browseable public class index
- a route into show classes across shows or within a show

### 7. Strengthen Public Person Profiles

Even with private-name constraints, public profile navigation should be
stronger.

Expected direction:

- public initials or masked identity remain in list views
- clicking through should show the exhibitor’s public history
- past entries, placements, and media should be visible where privacy rules
  allow

### 8. Reframe Account As A Real Profile Surface

The account page should become a reusable profile shell rather than a long
single-column dump.

Initial direction:

- add a left-column profile navigation
- move deep operational content behind its own profile sub-pages or sections
- keep the default profile landing focused on the person and their flowershow
  participation

Likely left-nav items:

- `Overview`
- `Shows`
- `Entries`
- `Clubs`
- `Tokens / API`
- later: balances, payouts, notifications, settings

Important UX rule:

- most users should never have to parse token or capability detail unless they
  explicitly choose that section

### 9. Reduce Agent-Token Prominence In Account

Agent token management should stay available but should not dominate the main
profile page.

Expected direction:

- hide or isolate token management behind the profile nav
- collapse long capability lists by default
- keep operational/admin detail available on demand
- treat this as advanced tooling, not the main account experience

### 10. Move Body Actions Out Of Basic Identity Blocks

The account page currently mixes identity display with too many actions.

Direction:

- keep sign-out available on the profile surface, likely top-right of the page
- remove `Browse Shows`, `Open Admin`, and similar navigation actions from the
  identity card itself
- let the page shell and left nav carry navigation instead

### 11. Improve Standards Without Losing It

Standards should remain part of the system, but it needs to be linked from a
more appropriate place and eventually become more explorable.

Direction:

- do not lose the page
- remove it from primary top nav
- link it from show rules, class detail, club context, or a deeper knowledge
  area
- later improve it into an actual browse/detail experience instead of mostly a
  source-document jump

## Next PR Scope

Recommended next PR should stay focused on public UX and not mix in more admin
or authority work.

### Slice A

- simplify global nav
- signed-in right-side identity affordance
- hide admin for non-admins
- responsive nav cleanup
- fix season default on new-show creation to match the current year

### Slice B

- split home into upcoming shows, active clubs, and past shows
- add browse callout on home

### Slice C

- add mock or placeholder image rails to show cards
- implement slow rotation treatment
- apply same card language to browse results where useful

### Slice D

- create first `Clubs` public page
- move leaderboard emphasis into club context

### Slice E

- introduce reusable left-nav profile shell
- move token management into its own profile area
- minimize or collapse long advanced-permission content

### Slice F

- improve standards discoverability and placement without deleting the surface

### Slice G

- strengthen public person-profile linking and presentation

## Constraints

- do not lose existing browse capability
- do not overexpose admin or private surfaces in nav
- keep mobile behavior intentional rather than collapsing into a broken wrap
- treat photos as a first-class design element, even while some assets are
  still mocked
- preserve agent/API footer visibility, but avoid making it visually compete
  with the main public content

## Current Confirmation Points

These were explicitly called out in the review pass and should be treated as
accepted direction unless changed later:

- upcoming and past shows should both exist on home
- active clubs should exist on home
- shows should feel image-led
- `Browse` should remain
- `Clubs` should become top-level
- `Taxonomy`, `Leaderboard`, `Standards`, and `Account` should not remain
  top-level public nav items
- signed-in identity should appear as the person name on the far right
- `Admin` should only appear for admins
- public exhibitor profiles should be reachable from visible entry surfaces
- account should become a left-nav profile shell rather than a long single
  page of mixed concerns
- advanced agent-token capability detail should be hidden or deferred for most
  users
- standards should remain but should not stay in the primary top nav
- new-show season should default to the current year, which is now `2026`
