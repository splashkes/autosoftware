# Evolution Quick Reference

> **Boundary rule.** Work in `seeds/` only. Do not modify `kernel/`, `compose.yaml`, or root config files.
> All code and artifacts go in `seeds/<seed_id>/realizations/<realization_id>/artifacts/`.

> **Socket assignments.** Each realization listens on a unix domain socket. Use the convention `/tmp/as-realizations/<seed-id>--<realization-id>.sock` in `runtime.yaml`:
>
> | Seed | `AS_ADDR` |
> |------|-----------|
> | 0003 | `/tmp/as-realizations/0003-customer-service-app--a-web-mvp.sock` |
> | 0004 | `/tmp/as-realizations/0004-event-listings--a-web-mvp.sock` |

One-page companion to [EVOLUTION_INSTRUCTIONS.md](EVOLUTION_INSTRUCTIONS.md).

## Contents

| Section | Line |
|---------|------|
| **Context Sequence:** The ordered reading list that produces better realizations | 30 |
| **Readiness Transitions:** What you need to advance a realization to the next stage | 41 |
| **Operations:** The three types of evolution passes and when to use each | 49 |
| **One Pass Should:** The minimum bar every evolution pass must clear | 57 |
| **Growth API:** HTTP endpoints for triggering and monitoring realization growth | 64 |
| **File Structure:** Canonical layout of a seed directory and its realizations | 72 |

---

## Context Sequence: The ordered reading list that produces better realizations

1. **seed.yaml** — identity and status
2. **brief.md** — what the user wants
3. **acceptance.md** — what "done" looks like (read BEFORE design)
4. **design.md** — shape, boundaries, constraints
5. **approaches/*.yaml** — strategy and realizer config
6. **interaction_contract.yaml** — commands, projections, capabilities
7. **Existing artifacts** — what's already built (if iterating)
8. **Kernel capabilities** — what the platform provides for free

## Readiness Transitions: What you need to advance a realization to the next stage

| From | To | You Need |
|------|----|----------|
| Designed | Defined | `interaction_contract.yaml` |
| Defined | Runnable | `artifacts/` + `runtime.yaml` |
| Runnable | Accepted | Validation evidence + review |

## Operations: The three types of evolution passes and when to use each

| Op | Use When |
|----|----------|
| **Grow** | First pass or major expansion |
| **Tweak** | Targeted fix or refinement |
| **Validate** | Verify without changing |

## One Pass Should: The minimum bar every evolution pass must clear

- Move forward at least one readiness stage (or meaningfully improve within one)
- Not leave things broken
- Include validation evidence mapped to acceptance criteria
- Record durable decisions in `decision_log.md`

## Growth API: HTTP endpoints for triggering and monitoring realization growth

```
GET  /v1/projections/realization-growth/seed-packet?reference=<seed>/<realization>
POST /v1/commands/realizations.grow
GET  /v1/projections/realization-growth/jobs/<job_id>
```

## File Structure: Canonical layout of a seed directory and its realizations

```
seeds/<id>/
├── seed.yaml
├── brief.md
├── acceptance.md
├── design.md
├── approaches/<approach>.yaml
└── realizations/<id>/
    ├── realization.yaml
    ├── interaction_contract.yaml
    ├── artifacts/
    │   ├── runtime.yaml          # how to boot
    │   └── <app>/                # implementation
    └── validation/README.md      # acceptance evidence
```
