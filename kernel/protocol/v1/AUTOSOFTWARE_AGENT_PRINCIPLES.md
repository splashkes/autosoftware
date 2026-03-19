# Autosoftware Agent Principles

This document defines the stable system-light rules for agent-facing
Autosoftware realizations.

It is intended to stay durable while individual seeds and realizations evolve.
Each seed should carry a thin local overlay in
`AUTOSOFTWARE_AGENT_PRINCIPLES.md` that links back here and records only the
seed-specific constraints.

## Core Rule

- Agents, first-party UI flows, operator tools, and third-party integrations
  should use the same semantic commands and projections for normal work.
- A private HTMX or server-rendered screen is not an excuse to create a
  stronger human-only interface.
- Authenticated agents should usually have equal or better operational
  observability than the first-party UI, even when the UI remains the main
  human surface.

## Registry First

- Agents should discover capabilities through `GET /v1/contracts`.
- The authoritative interface is the declared interaction contract, not a DOM
  scrape, not a hidden admin route, and not direct projection-table CRUD.
- Stable object identities and declared projections should be preferred over
  mutable handles or route fragments.

## Auth Paths

- Realizations must make their supported auth modes explicit.
- `service_token` access is the normal path for remote agent operators unless a
  stronger or narrower mode is declared for the seed.
- Anonymous access, session access, and service-token access must describe
  their different read boundaries clearly.
- External authentication proves identity.
- Durable authority should belong to system-native subjects, scopes, bundles,
  grants, and materialized effective access rather than being permanently
  outsourced to the auth provider.

## Runtime Context

- Prompt-like authoring instructions sent to an AI interactor belong in
  runtime-only request context.
- Runtime context can shape how an agent composes a command, but it is not
  canonical domain truth.
- Canonical truth belongs in the seed graph, accepted projections, and cited
  records.

## Registry And DB Discipline

- Agents should use the shared registry and declared API surface before any
  direct datastore path.
- Direct database access is a system-internal or maintenance path, not the
  normal operational contract, unless the seed explicitly declares otherwise.
- Hidden server-only reads and writes should be treated as implementation
  debt, not as valid long-term agent surfaces.

## Errors

- Authenticated callers should receive useful structured errors.
- Error payloads should include a stable code, a human-readable message, a
  request identifier, and enough hinting to recover without guessing.
- Anonymous callers may receive a smaller or more defensive error slice when
  exposure risk is higher.

## Observability

- Minimal UI is acceptable.
- Invisible capability is not.
- Interactive realizations should expose enough on-page contract and API
  affordance that a human can inspect the live surface an agent would use.

## Testability

- Parity requirements should be checked twice:
- once at contract-load time, so missing links and broken schema references
  fail quickly
- once against the running app, so auth, command shape, error shape, and
  read-your-writes behavior are proven in flight

## Seed Overlays

Each seed-local `AUTOSOFTWARE_AGENT_PRINCIPLES.md` should record:

- the seed's primary agent workflows
- the normal auth path for agents
- any declared internal-only or maintenance-only paths
- any seed-specific registry, provenance, upload, or review rules
- what "equal or better than the UI" means for that seed
