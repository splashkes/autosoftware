# AS Kernel Review: Mission, Seeds, and Gap Analysis

## Context

Autosoftware (AS) is an experiment where software evolves from user requests made inside the running experience itself. The kernel is the trusted, generic platform machinery for 1000+ apps. Seeds define specific apps. This review checks whether the kernel provides the base functionality the 3 real seeds imply.

---

## The Wisdom

The core insight: software development and software use shouldn't be separate activities. In AS, a user request becomes a **seed** (source intent), gets compiled into a **realization** (runnable code), and the **materializer** rebuilds the running experience. An append-only **registry** keeps accepted history auditable and replayable. The kernel provides boring, generic primitives (identity, access, state, messaging, jobs, search) so seeds can focus on domain logic without reinventing infrastructure.

Key architectural principle: **capability-shaped, not app-shaped**. The kernel exposes principals, access links, state transitions, publications, threads, jobs, and outbox delivery. Seeds define tickets, events, auctions, and bids on top of these primitives.

---

## The 3 Real Seeds

| Seed | What It Is | Core Needs |
|---|---|---|
| **0003 - Customer Service** | Tickets + live chat + knowledge base for small support teams | Auth, signed access links (portalless customer access), threads/messages, state machine, search, email notifications |
| **0004 - Event Listings** | Public event calendar for organizers | Auth, timezone-aware publications (start/end, all-day), handles/slugs for stable URLs, state machine, search/filter |
| **0005 - Charity Auction** | Online auction with deterministic bidding | Auth, scheduled jobs (auction close authority), state machine, activity events (bid audit trail), idempotency, guardrails |

All three use the Go+HTMX server-rendered approach. All three defer payments, advanced integrations, and recurrence.

---

## Gap Analysis: What's Built vs. What's Needed

### Actually Working (usable today)

| Capability | Notes |
|---|---|
| Realization discovery + materialization | `catalog.go`, `service.go` -- full local+remote merge, file previews, persistence |
| Web loader UI (`webd` :8090) | Dark-themed realization selector, boot button, materialization display |
| Materializer API (`materializerd` :8091) | `/v1/realizations`, `/v1/materializations` endpoints |
| Browser feedback loop | Injected script captures window errors, console.error, HTMX failures, posts to `/feedback/incidents` |
| Security middleware stack | CSP with nonces, X-Frame-Options DENY, same-origin CSRF enforcement, correlation IDs |
| Repo structure validation (`asvalidate`) | Manifests, directory structure, no-CDN-scripts rule, doc sparsity warnings |

**Honest verdict**: All working code is meta-platform (dev loop, structure validation, error capture). None serves end-user application functionality. No seed can store a ticket, show an event, or accept a bid using what exists today.

### Schema-Ready, Zero Go Code (the big gap)

The `kernel/db/runtime/` directory contains 8 well-designed SQL files covering all 10 documented capability families. **None are wired to Go code.** `go.mod` has no database driver. There is no repository layer, no service layer, no migration runner.

| Schema File | What It Covers | Which Seeds Need It | Blocking? |
|---|---|---|---|
| `sessions.sql` | Principals, identifiers, memberships, sessions, consent | ALL | **Critical** -- no auth without this |
| `web_state.sql` | Handles, access links, publications, state transitions, activity events, jobs, outbox, idempotency | ALL | **Critical** -- the largest and most important schema |
| `communications.sql` | Threads, messages, assignments, subscriptions, notification prefs | 0003 primarily | **Critical for 0003** |
| `uploads.sql` | Upload blobs, file references | All (but deferrable for v1) | Low |
| `discovery.sql` | Search documents, facets | 0003, 0004 | Medium-High |
| `guardrails.sql` | Rate limits, guard decisions, risk events | 0005 primarily | Medium |
| `oauth.sql` | Auth providers, linked identities, challenges | All (v1 could use simpler auth) | Low for v1 |
| `feedback_loop.sql` | Browser crashes, request traces, test runs | Meta-platform | Low (in-memory store works for now) |

### Completely Missing (not even in schema)

| Gap | Impact |
|---|---|
| **Database connectivity** | `go.mod` only has `yaml.v3`. No `pgx`, no `database/sql`, no migration tool. Blocks everything. |
| **`registryd` implementation** | Empty `main()`. The append-only registry has no code. (Acceptable for v1 -- seeds can ship against runtime layer) |
| **`apid` implementation** | Empty `main()`. No machine-facing API. |
| **Seed route registration** | `webd` only serves the loader UI. No mechanism for seeds to register handlers, templates, or routes. Open architectural question. |
| **Real-time transport** | No SSE, WebSocket, or long-polling. Seed 0003 live chat and Seed 0005 live bidding need this. |
| **Full-text search** | Schema has `runtime_search_documents` but no `tsvector`, no query logic. |
| **Email sending** | Outbox schema exists but no SMTP/provider integration. |
| **Password/credential storage** | No hashing, no verification. |

### Correctly Excluded from Kernel (domain logic that belongs in seeds)

The boundary is well-drawn. These are correctly absent:

- Ticket lifecycle rules, chat-to-ticket fallback (0003)
- Event entity model, calendar rendering, category taxonomy (0004)
- Bid validation rules (min increment, first-received-wins), auction/lot entities, winner promotion (0005)
- All domain-specific entity storage and business workflows

---

## Design Assessment

### What's Sound

1. **Publication model** with `timezone`, `all_day`, `starts_at`, `ends_at` directly serves Seed 0004 events and Seed 0005 auction windows without domain leakage.
2. **State transitions** as generic auditable records with `machine` field means one table serves ticket states, event states, and auction states.
3. **Access links** as a first-class kernel primitive correctly anticipates Seed 0003's portalless customer access.
4. **Subject-anchored threads** keyed on `subject_kind/subject_id` means the same messaging infrastructure works for ticket conversations, live chat, and bid-related notifications.
5. **Transactional outbox** is the right architecture for reliable email/notification delivery across all seeds.

### What Needs Decision Before Building

1. **How do seed realizations plug into the web server?** Currently `webd` is monolithic. Seeds need route registration, template rendering, and static asset serving. Options: (a) each seed runs its own HTTP server, (b) `webd` gains a plugin/mount system, (c) reverse proxy composition.
2. **Real-time transport**: SSE vs WebSocket vs polling for Seed 0003 chat and Seed 0005 bid updates.
3. **Auth for v1**: Full OAuth vs email magic-link vs simple password. The `auth_challenges` table could support email codes for v1.

---

## Overall Verdict

**The kernel's design is excellent. Its implementation is ~15% complete.**

The schema layer maps precisely to what the three seeds need. The capability-vs-domain boundary is correct. The architecture documents are thoughtful and consistent. But between the well-designed SQL schemas and the working meta-platform shell, there is a vast gap: no database driver, no auth, no session validation, no state machine service, no messaging, no jobs, no search, no outbox. Every one of these has a schema file ready to go, but zero Go code behind it.

The 3 seeds are **well-specified and ready to build** -- their briefs, designs, acceptance criteria, and decision logs are thorough. The kernel's design correctly anticipates their needs. But the kernel is not yet ready to support them. The foundation is a blueprint, not a building.

### Priority Path to Enable Seeds

1. **Tier 0** (blocks everything): Database driver + connectivity, migration runner, principal/session service
2. **Tier 1** (blocks any seed from shipping): Cookie-to-session wiring, state transition service, access link service, publication service, handle service, job queue runner
3. **Tier 2** (supporting): Thread/message service, search service, outbox dispatcher, activity events, uploads
4. **Tier 3** (architectural): Seed route registration decision, real-time transport decision, `apid` scope decision
