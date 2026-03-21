# Flowershow Live UI/UX Audit

Date: 2026-03-21
Environment: live production at `https://autosoftware.app/flowershow`
Method: manual Playwright walkthrough across public desktop, public mobile, and authenticated admin surfaces

## Coverage

- Homepage
- Classes index
- Clubs index
- Show detail
- Admin show workspace
- Intake workflow shell
- Mobile navigation

## Screenshot References

- Homepage desktop:
  - `/var/folders/x6/w41swp_n32j7c95b70skk6b80000gn/T/playwright-mcp-output/1774107547670/page-2026-03-21T18-52-19-391Z.png`
- Classes desktop:
  - `/var/folders/x6/w41swp_n32j7c95b70skk6b80000gn/T/playwright-mcp-output/1774107547670/page-2026-03-21T18-52-26-162Z.png`
- Clubs desktop:
  - `/var/folders/x6/w41swp_n32j7c95b70skk6b80000gn/T/playwright-mcp-output/1774107547670/page-2026-03-21T18-52-36-042Z.png`
- Homepage mobile:
  - `/var/folders/x6/w41swp_n32j7c95b70skk6b80000gn/T/playwright-mcp-output/1774107547670/page-2026-03-21T18-52-47-372Z.png`
- Show detail desktop:
  - `/var/folders/x6/w41swp_n32j7c95b70skk6b80000gn/T/playwright-mcp-output/1774107547670/page-2026-03-21T18-52-12-458Z.png`
- Admin setup desktop:
  - `/var/folders/x6/w41swp_n32j7c95b70skk6b80000gn/T/playwright-mcp-output/1774107547670/page-2026-03-21T18-53-11-920Z.png`
- Admin intake desktop:
  - `/var/folders/x6/w41swp_n32j7c95b70skk6b80000gn/T/playwright-mcp-output/1774107547670/page-2026-03-21T18-53-31-946Z.png`

## Top Findings

### 1. Homepage cards are visually strong but semantically noisy

The homepage has become much more polished, but the first show card still communicates class fragments more strongly than the show itself. The hero card for `UHS API Demo Show` is image-led and attractive, but the overlaid text `Class 16 · Miniature / Class 2 · Pink / Class 9 · Any colour` reads like leftover internal metadata instead of a public-facing summary.

Suggested fix:

- Keep one strong over-image label only.
- Prefer either:
  - `Featured classes`
  - or a single synthesized label like `Peonies, roses, dahlias, and tomatoes`
- Move the detailed sample classes into secondary metadata or remove them entirely.

### 2. Public class identity is still duplicated too often

The classes page is much improved visually, especially with photography, but class identity is repeated in multiple forms on each card:

- badge: `Class 1 · White`
- heading: `1 · Double or semi-double`
- body line: `White`

That is too much decoding work for one tile.

Suggested fix:

- Make the title the canonical form:
  - `Class 1`
  - `Double or semi-double`
  - `White`
- Stop mixing number and qualifier into both the badge and the heading.
- Keep one summary line for the rule detail:
  - `1 bloom · Paeonia lactiflora`

### 3. Club index has uneven information density

The clubs page works, but the cards do not scale well across clubs with very different amounts of data.

Observed problems:

- `District 17` and `Ontario Horticultural Society` feel mostly empty and visually wasteful.
- `Uxbridge Horticultural Society` is much denser and more useful.
- The first upcoming show entry for Uxbridge still leads with `UHS API Demo Show`, while the rest lead with dates, so the formatting is inconsistent.

Suggested fix:

- Standardize upcoming shows to date-first everywhere.
- Collapse truly empty cards into a more compact summary variant.
- Consider replacing `Top exhibitors` empty text with:
  - `No public exhibitors yet`
  - and a smaller visual footprint.

### 4. Show detail entries table is informative but overloaded

The show detail page is already richer than before, but the entries table still asks the user to parse too much repeated class and note content in a cramped row:

- entry image
- generated entry title
- class name
- qualifier
- presentation
- taxon
- notes

The result is useful but heavy.

Suggested fix:

- Keep the current image-first row.
- Reduce each class cell to:
  - `Class 4: Single`
  - `Any colour`
  - `1 bloom · Paeonia lactiflora`
- Only show `Notes:` when the note materially adds something beyond taxon repetition.
- For public-facing generated entry names, prefer a cleaner display label if one exists rather than repeating class wording.

### 5. Admin show setup still feels too verbose for operational use

The show admin shell is moving in the right direction, but `Setup` still feels like a long document rather than a fast workspace. The left nav is useful. The top tab strip is useful. The content density underneath is still too high.

Observed friction:

- verbose helper text under most sections
- repeated `Expand` affordances
- schedule structure area is still hard to skim
- duplicated division text is still visible in admin:
  - `Division 1: Division 1 - Horticulture Demo`

Suggested fix:

- shorten helper text in admin by about 30-50%
- make disclosure labels more specific:
  - `Assign judge`
  - `Add credit`
  - `Edit class`
  instead of generic `Expand`
- normalize division rendering in admin to match public:
  - `Division 1 - Horticulture Demo`

### 6. Intake tab is the strongest new operator surface, but the sidebar labels are still too mechanical

The intake grid is directionally correct and much more operationally useful than the old form stack. It is now visually scannable and supports the right mental model: class first, entry tiles second, add tile last.

Remaining issues:

- sidebar labels still use numbered sections like `1. Peony`, `2. Rose`
- division heading in content still shows duplicated label text
- the explanatory copy at the top can be shorter now that the grid itself is self-explanatory

Suggested fix:

- use division group headings in the sidebar, with plain section names below them
- remove numeric prefixes from the section nav
- trim the intro copy to a single line
- fix division heading duplication here too

## Mobile Findings

### 7. Mobile shell is working, but the homepage still feels long and top-heavy

The hamburger menu is present and functional. That is good. The hero and stats stack cleanly. But the homepage still feels like too much scrolling before the user gets to the most useful show content.

Suggested fix:

- reduce hero copy length on mobile
- tighten spacing between hero, stats, and first show card
- consider moving the metrics strip below the first upcoming show on mobile

### 8. Footer widget is still a bit too dominant on mobile

The inset is better than before, but the `AS / Agent + access / (re)design` block still reads as a large, product-level footer component competing with page content instead of quietly supporting it.

Suggested fix:

- reduce vertical padding on mobile
- reduce tab chrome prominence
- keep the content available but demote its visual weight

## Technical / Interaction Findings

### 9. Mobile homepage emits an htmx console error

Observed on mobile homepage load:

- console error source: `https://unpkg.com/htmx.org@2.0.4`
- browser reported:
  - `[ERROR] Event @ https://unpkg.com/htmx.org@2.0.4:0`

This may be harmless, but it should be treated as a real regression until understood.

Suggested fix:

- reproduce with explicit console logging in development
- identify which event is being emitted
- remove the spurious htmx error if it is expected behavior

## Strongest Immediate Improvements

If only a short next pass is available, the highest-value fixes are:

1. Clean up public card semantics on homepage and classes.
2. Standardize club upcoming-show formatting to date-first everywhere.
3. Normalize duplicated admin division headings.
4. Reduce verbose admin helper copy and generic disclosure text.
5. Investigate the mobile htmx console error.

## Overall Assessment

The system is now much more coherent than it was a few weeks ago. The design language is recognizable, the photography is doing real work, the club/show/class surfaces are starting to feel connected, and the intake workspace is materially better than the old workflow. The remaining issues are less about foundational brokenness and more about information architecture discipline:

- too much repeated metadata
- too much explanatory text in admin
- inconsistent summary patterns between similar cards
- a few lingering rendering/debug leftovers

That is a good place to be, but it means the next round should focus on editorial tightening and consistency, not just more features.
