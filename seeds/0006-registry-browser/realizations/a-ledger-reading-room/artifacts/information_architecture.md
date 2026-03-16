# Information Architecture

## Intent

This realization treats the registry browser as a public ledger reading room.

The authoritative browser remains the direct catalog surface.
The reading room is the human legibility layer built on top of the same
accepted registry state.

## Primary Principles

1. World before machinery.
   A person should encounter software systems and governed things before
   realization, projection, or schema jargon.
2. Meaning before identifiers.
   Plain-language purpose and summary come before stable IDs and raw route
   strings.
3. Relationships before raw fields.
   Each page should first answer what this is connected to and why it matters.
4. Trust through traceability.
   Every page must still reveal the authoritative route or routes that ground
   the view.
5. Meta separated from domain.
   Registry-internal entities should be visible, but not mixed into the
   default browse layer for domain software.

## Top-Level Navigation

### Systems

Primary entry point.
Grouped by `seed_id`, with display titles, summaries, maturity, governed
things, actions, read models, and contracts.

### Governed Things

Human-facing entry point for objects.
Answers: what exists, what governs it, what can happen to it, and where it is
seen.

### Actions

Human-facing entry point for commands.
Answers: what can happen, who can do it, what it affects, and which read model
shows the result.

### Read Models

Human-facing entry point for projections.
Answers: what question this read model answers, how fresh it is, and which
system and governed things it reflects.

### Contracts

Entry point for realizations and schemas as governing documents.
Answers: which behavior is implemented, which meaning is governed, and how
authoritative the current state is.

### Registry Internals

Separate area for self-describing registry resources such as registry objects,
registry commands, registry projections, and registry schemas.

## Home Page Order

1. What this registry is in plain language
2. Systems overview
3. How maturity and trust should be read
4. Browse entry points
5. How to verify each view through the authoritative API
6. Registry internals as a secondary route

## Status Framing

The reading room should not flatten all metadata into one row.
It should separate:

- acceptance status: draft, accepted, retired
- runtime availability: planned, runnable, read-only
- surface kind: interactive, read-only, bootstrap-only

## Content Rules

- show display title first, stable identifier second
- show one-sentence summary on every list row
- show relation counts only after the summary is clear
- do not lead a page with raw route strings
- show authoritative routes in a dedicated "Source of truth" block
- preserve exact machine names, but do not force them as the main label
