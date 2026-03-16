# Acceptance

## Seed-Level Criteria (Every Realization)

Every realization of this seed must satisfy the following:

1. The product makes the authority model explicit:
   upstream sources are imported, the hub owns production truth, and
   destinations receive propagated copies.
2. Every event shown by the system remains traceable to its source calendar,
   source event identity, and most recent sync metadata.
3. The hub preserves event revision history so upstream changes and operator
   decisions do not silently overwrite earlier state.
4. Production events are explicit objects or states, distinct from raw inbound
   source records.
5. Propagation state is visible per destination edge with at least pending,
   pushed, and failed semantics plus retry visibility.
6. The interaction contract exposes commands and projections for source sync,
   production control, and propagation inspection.
7. Operators can filter or locate at least these issue classes:
   upstream changes, conflicts or review-needed items, unsynced records, and
   propagation failures.
8. Credentials are referenced through secure handles or external secret
   bindings rather than stored as plaintext seed artifacts.
9. The first implementation path does not require major kernel redesign.

## Operations Realization Criteria (a-network-operations-hub)

The operations realization must additionally satisfy:

10. The default landing surface shows the connected source, hub, and
    destination topology with visible sync-health status.
11. An operator can move from a failing source or destination into the exact
    affected event lineage and delivery edges without losing provenance.
12. Incident and retry workflows are promoted ahead of calendar editing or
    curation affordances.

## Workbench Realization Criteria (b-production-calendar-workbench)

The workbench realization must additionally satisfy:

13. The default landing surface centers the production calendar and its intake
    queue rather than the raw connector graph.
14. An operator can review normalized events and explicitly decide whether they
    become production truth, remain deferred, or require further attention.
15. Timeline or calendar views of production events remain traceable back to
    their source lineage and destination propagation state.
