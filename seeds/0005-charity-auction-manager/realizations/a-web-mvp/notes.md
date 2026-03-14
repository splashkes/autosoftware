# Realization Notes

Implementation should protect bid integrity before adding convenience features.

Recommended build order:

1. auction, lot, bidder, and bid models with close semantics
2. lot detail, registration, and terms-acceptance flows
3. bid placement rules, stale-bid rejection, and leader calculation
4. admin auction closeout, default handling, and reporting
