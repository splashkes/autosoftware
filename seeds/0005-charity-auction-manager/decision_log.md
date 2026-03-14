# Decisions

## Payments Deferred

The first version tracks auction outcomes without integrated payment handling.

Reason:

- payment processing adds legal, security, and operational complexity
- the auction workflow is still valuable without it
- winner and fulfillment tracking can be built first

## Bid Integrity Is Core

Bid validation and auction state correctness are more important than flashy
presentation in the first release.

Reason:

- trust in the bidding rules is the product's foundation
- correctness failures would invalidate the whole platform

## Registration Required For Bidding

Browsing can be public, but placing bids requires bidder registration.

Reason:

- admins need to know who won
- anonymous bidding creates avoidable operational risk

## Deterministic Bid Ordering

The server's receive order is authoritative when bids compete.

Reason:

- clients cannot be trusted to agree on timing
- race conditions are inevitable near close
- admins need a simple rule they can explain after disputes

## Scheduled Close Is Authoritative

Auctions stop accepting bids automatically at the configured close time, with
optional manual early close.

Reason:

- bidders need predictable rules about when bidding ends
- automatic close removes operator hesitation at the deadline
- manual early close still gives admins an escape hatch for exceptional cases
