# Design

This seed defines a focused charity auction platform for publishing lots,
accepting bids, and closing auctions cleanly.

## Product Shape

The application should have:

- an admin workspace for managing auctions and lots
- a public auction catalog
- authenticated bidder flows for registration and bidding
- post-close reporting for winners and next steps

## Actors and Access

- Public visitor: can browse active auction pages and lot details.
- Bidder: registered user with required contact details and accepted bidding
  terms; can place bids and see whether they are leading or outbid.
- Admin: authenticated charity staff member who manages auctions, lots, and
  closeout status.

Bidder registration in v1 should capture enough identity to complete a manual
handoff after the auction, at minimum name, email, and phone number. The
product does not need to become a full donor CRM.

## Core Workflows

### Admin Workflow

Admins should be able to:

- create an auction with open and close times in a defined timezone
- add, edit, publish, and withdraw lots
- set starting bids and minimum increment rules
- monitor active bidding and current leaders
- manually close an auction early if needed and view winners

### Bidder Workflow

Bidders should be able to:

- register with enough identity to place bids and accept auction terms
- browse active lots
- open a lot detail page
- place a valid bid above the current amount
- see whether they are currently winning or outbid

### Closeout Workflow

After an auction closes, admins should be able to:

- see winning bidders by lot
- mark winners as contacted
- track payment pending versus fulfilled outside the system if needed
- mark a winner as defaulted or unreachable and promote the next highest valid
  bidder with an audit note

## Auction and Bid Rules

### Auction States

- `draft`: editable and not yet publicly open for bidding
- `scheduled`: published with a future open time
- `open`: currently accepting bids
- `closed`: no longer accepting bids

Expected behavior:

- an auction opens automatically at `open_at`
- bidding stops automatically at `close_at`
- admins may close an auction early, and the earlier of manual close or
  scheduled close is authoritative
- reopening is out of scope for v1

### Lot States

- `draft`: not public
- `published`: publicly visible and bid-eligible when the parent auction is
  open
- `withdrawn`: publicly understandable but no longer bid-eligible
- `closed`: inherited outcome after auction close

### Bid Validity

- if a lot has no bids yet, the first valid bid is at least the starting bid
- once a valid high bid exists, the next valid bid is at least current high bid
  plus the lot's minimum increment
- server receive time is authoritative for competing submissions
- if two bids of the same amount race, the first accepted bid remains the
  leader and the later same-amount bid is rejected as below the new minimum
- bids received at or after the authoritative close time are invalid
- accepted bids are immutable in v1; bidder-side bid withdrawal is out of scope

Reserve prices, proxy bidding, and anti-sniping extensions are all deferred
from the MVP so the close rules remain simple and auditable.

## MVP Boundaries

Included in v1:

- one organization
- online lot browsing
- bidder registration
- bid placement with validation
- closeout reporting
- deterministic close and bid-ordering rules
- manual post-close status tracking and winner promotion

Deferred beyond v1:

- integrated payments
- automatic tax receipts
- shipping workflows
- proxy bidding
- reserve prices
- anti-sniping or automatic time extensions
- bidder-initiated bid retraction
- silent-plus-live hybrid event orchestration
- donor CRM features beyond what the auction itself needs

## Realization Guidance

Realizations should prioritize correctness, auditability, and clear operator
handoff over flashy interaction. The most important thing is that bidding rules
and close behavior are deterministic and explainable.

Technology choice belongs in the realization approach documents. Whatever stack
is used, it should make accepted bids, rejected bids, close timing, and
post-close status changes easy to validate.
