# Acceptance

Every realization of this seed must satisfy the following:

1. The system can ingest events from more than one external calendar source or
   feed type.
2. Imported events are normalized into a single canonical event model with
   preserved source identity and provenance.
3. Operators can inspect sync status for each source and destination.
4. The system records propagation state for each outbound delivery target,
   including at least pending, pushed, and failed.
5. The authority model clearly distinguishes imported source calendars from the
   production calendar used for outbound propagation.
6. Operators can inspect event lineage, including origin, transformation, and
   downstream propagation targets.
7. The UI or API exposes conflicts, unsynced changes, or failed propagation in
   an operationally useful way.
8. The realization does not pretend to be a full personal scheduling product if
   that scope is still deferred.
