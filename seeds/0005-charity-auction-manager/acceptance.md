# Acceptance

Every realization of this seed must satisfy the following:

1. An admin can create and manage auctions and lots, including explicit
   timezone, open time, close time, publish state, and withdrawn state.
2. A bidder can register with required contact details, accept terms, and place
   a valid bid on an active lot.
3. The system accepts an opening bid only when it meets or exceeds the starting
   bid and accepts subsequent bids only when they meet or exceed the current
   high bid plus the minimum increment.
4. The system rejects bids that are stale, below the current minimum valid
   amount, or received at or after the authoritative close time.
5. Competing bids are resolved by server-side receive order, not client-side
   timing.
6. A bidder can see the current high bid and whether they are leading or
   outbid.
7. Bidding stops automatically at the scheduled close time unless an admin
   closes the auction earlier.
8. An admin can review winners by lot and track post-close status including
   contacted, payment pending, fulfilled, defaulted, and next-bidder
   promotion.
9. The realization clearly defines off-platform payment tracking without
   pretending to implement integrated payments or receipt generation if those
   are still deferred.
