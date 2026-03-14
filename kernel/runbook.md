# Kernel Runbook

This document is for operational procedures.

Boundary:

- describe how to run, validate, inspect, and operate the kernel
- describe step-by-step sequences
- do not restate architecture rationale here
- do not duplicate protocol definitions here

## Scope

The runbook should answer operational questions such as:

- how to start local kernel services
- how to pin a local run to a realization
- how to validate a seed before merge
- how to validate a realization before merge
- how an accepted realization is appended to the registry
- how to trigger or inspect materialization
- how to inspect registry and materializer state
- how to inspect browser and request incidents for one realization

## Planned Operational Flow

1. Prepare the founding seed and kernel configuration.
2. Start the registry service.
3. Start the materializer.
4. Start web and API surfaces as needed.
5. Select or pin the realization to test or serve.
6. Author or refine one seed in `seeds/`.
7. Produce or refine a realization for that seed.
8. Run the realization and capture browser or request failures in runtime
   feedback-loop storage.
9. Validate the realization against the seed-level acceptance criteria.
10. Review incidents, test runs, and agent findings tied to that realization.
11. Merge the realization through review.
12. Append the accepted realization to the registry.
13. Confirm materialization completed successfully.

As the implementation solidifies, this file should become the source of truth
for actual commands and procedures.
