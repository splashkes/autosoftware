# Notes

Initial implementation slices to consider:

1. auction, lot, bidder, and bid schema with explicit close semantics
2. public lot pages, bidder registration, and terms acceptance
3. bid placement, stale-bid rejection, and leader calculation
4. admin monitoring, closeout statuses, and default handling

Open product questions for later refinement:

- whether outbid notifications should wait until after the first usable build
- whether winner-contact exports or print views matter for operators
- whether hybrid live-plus-silent events should reuse this model later
