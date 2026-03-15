# Acceptance

## Seed-Level Criteria (Every Realization)

Every realization of this seed must satisfy the following:

1. The surface is strictly read-only with no mutation controls or mutating
   HTTP verbs.
2. The UI distinguishes convenience summaries from authoritative registry
   resources instead of collapsing them into one ambiguous surface.
3. The product gives explicit instructions for agents to access the same data
   through API routes and does not require HTML scraping for authoritative use.
4. The browser exposes or links to authoritative list and detail API routes
   for the resources it presents.
5. If public/private visibility boundaries exist, the realization explains
   them explicitly rather than presenting an unexplained partial view as
   complete truth.
6. If claims, schema versions, change sets, or rows are not yet backed by real
   authoritative routes, the UI does not fake those surfaces as authoritative.

## First Realization Criteria (a-authoritative-browser)

The first realization must additionally satisfy:

7. The realization uses the existing registry catalog routes wherever they
   already expose the needed information, rather than introducing replacement
   browse APIs for the same surfaces.
8. A new visitor can reach a simple overview that explains what the registry
   is, what resource types exist, and where to start browsing.
9. List views for realizations, commands, projections, objects, and schemas
   support search or facet-based narrowing without forcing a user to know
   internal implementation details first.
10. Detail views for realizations, commands, projections, objects, and schemas
    expose stable identifiers and links to the authoritative catalog routes
    that back them.
11. The realization includes evidence that mutating verbs or mutation controls
    are not exposed through this browser surface.

## Later Realization Criteria (When Ledger Routes Exist)

Later realizations should additionally satisfy:

12. The browser provides browsable views for claims, schema versions, change
    sets, and rows backed by their real authoritative routes.
13. When claim detail exists, it makes supersession and governing schema
    version explicit.
