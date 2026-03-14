# Brief

Build a charity auction manager that helps organizations run online auctions
for charitable causes.

Desired initial outcome:

- admins can create auctions and manage lots
- bidders can register, browse items, and place bids under deterministic rules
- the system can track current leaders and winning bidders
- the first release supports online fundraising without requiring integrated
  payment processing
- admins can track post-close contact, payment, and fulfillment status inside
  the system even when payment happens off-platform
- the scope stays focused on auction operations rather than broad donor CRM

Primary users:

- charity admin running an auction
- bidder participating in an auction
- public visitor browsing auction items

Scope:

- auction CRUD with draft, scheduled, open, and closed states plus open and
  close times in an authoritative timezone
- lot management with title, description, image, starting bid, minimum
  increment, and withdrawal state
- bidder registration with required contact details, terms acceptance, and
  authenticated bidding
- live or near-live bid updates
- winner reporting, default handling, and fulfillment tracking after close

Constraints:

- keep the first release understandable and legally cautious
- define deterministic server-side bid ordering and close-time rules
- defer payment processing and tax receipt generation
- defer reserve prices, proxy bidding, bid retraction, and anti-sniping
  extensions if they threaten delivery
- defer multi-auction enterprise administration
- prefer one web application over a fragmented stack
