# Decisions

## One Team First

The first product version assumes one organization and one support team.

Reason:

- it removes multi-tenant complexity from the first build
- it keeps data ownership and permissions understandable
- it is enough to validate the product shape

## Portalless Customer Access

Customers do not need a full account in v1, but ticket access must still be
secure and repeatable.

Reason:

- requiring account creation adds friction at the exact moment people need help
- signed ticket links keep the request flow lightweight
- link recovery via email plus ticket code is enough for the MVP

## Unified Help Surface

Tickets, chat, and knowledge base should live in one application instead of
three disconnected tools.

Reason:

- chat should escalate cleanly into tickets
- knowledge base content should reduce inbound ticket volume
- one codebase is the fastest way to reach a usable MVP

## Chat Falls Back To Tickets

The chat experience should degrade into a ticket when no agent is available.

Reason:

- small teams cannot guarantee instant response at all times
- fallback is more honest than presenting an unattended live surface
- preserving the transcript prevents customers from repeating themselves

## Mailbox Sync Deferred

Mailbox sync, phone, SMS, and AI assistance are explicitly deferred.

Reason:

- each adds large integration and workflow cost
- transactional notifications are enough for secure ticket access in v1
- the seed should define a buildable v1, not a broad support platform fantasy
