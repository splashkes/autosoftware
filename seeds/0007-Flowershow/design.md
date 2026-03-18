# Design

## Core Structure

### 1. Organization Hierarchy
Organizations form a hierarchy:

Society → District → Region → Province → Country → Global

Each Show is hosted by an Organization.

---

### 2. Show

A Show represents a single event:

- organization
- location
- datetime
- categories

---

### 3. People

A Person:
- can be an entrant
- can be a judge
- may belong to zero or more organizations

Identity:
- private (full name)
- public (initials)

---

### 4. Entries

Entries are submissions into a category within a show.

Each Entry:
- belongs to a Show
- belongs to a Category
- belongs to a Person
- has placement and points
- has media
- has taxonomy references

---

### 5. Domains

#### Horticulture
Describes physical specimens:
- common name
- scientific name (structured)
- cultivar
- characteristics
- presentation rules

#### Design
Describes compositions:
- type (crescent design, etc)
- title
- criteria
- specifics (taxonomic references)

---

### 6. Taxonomy (Core System)

A flexible graph-like system:

- Taxons represent concepts (rose, crescent design, novice)
- Entries, categories, and awards reference taxons
- Taxons can relate to each other

Enables:
- cross-linking
- filtering
- discovery

---

### 7. Awards

Awards are defined per organization and season.

Each award:
- defines filters (taxonomy-based)
- defines scoring rules (sum, max, custom)

Examples:
- High Points
- Best Rose
- High Points Novice
- Memorial awards

---

### 8. Media

- Attached to entries
- Captured by multiple users
- Client-side downscaled
- Stored and linked

---

### 9. Privacy & Suppression

- System is append-only
- Content can be suppressed (hidden)
- Identity mapping is private

---

## Key Design Decisions

- API-first ingestion (no admin UI initially)
- Graph-like taxonomy over rigid schema
- Separate structured domains (horticulture vs design)
- Organization-scoped everything
