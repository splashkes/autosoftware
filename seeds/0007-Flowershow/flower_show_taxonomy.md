# Flower Show Competition – Taxonomy & Data Model (v1.0)

## Overview
This document defines the full taxonomy and data model for a federated flower show competition system.

It supports:
- Multiple organizations (clubs, societies, federations)
- Flexible taxonomy (botanical + design)
- Cross-linking
- Awards and scoring
- Media and entries
- Privacy and suppression

---

# 1. Core Axes

## 1.1 Domain
- horticulture
- design
- special
- other

## 1.2 Taxonomy
Cross-linkable concepts:
- rose
- crescent design
- novice
- yellow tipped

## 1.3 Organization
Hierarchical:
- society
- district
- region
- province
- country
- global

---

# 2. Organization Model

## Organization
- id
- name
- type (club, society, federation)
- parent_organization_id (nullable)
- location

Supports hierarchy:
Society → District → Region → Province → Country → Global

---

# 3. Person Model

## Person
- id
- name
- initials
- privacy flag

## PersonOrganization
- person_id
- organization_id
- role (member, judge, admin, guest)

---

# 4. Show Model

## Show
- id
- organization_id
- name
- location
- datetime
- season

---

# 5. Entry Model

## Entry
- id
- show_id
- category_id
- person_id
- placement
- points
- media_ids[]

---

# 6. Taxonomy Model

## Taxon
- id
- type
- label
- canonical_label
- aliases[]
- metadata

## Taxon Types
- botanical
- scientific_name
- cultivar
- characteristic
- design_type
- presentation_rule
- award_dimension
- free_tag

## TaxonRelation
- from_taxon_id
- to_taxon_id
- relation_type

---

# 7. Domain-Specific Models

## FlowerCategory
- category_number
- common_name
- scientific_name
- cultivar
- characteristics[]
- presentation[]

## DesignCategory
- type
- title
- criteria[]
- specifics[]

---

# 8. Scientific Names

Structured:
- genus
- species
- subspecies
- cultivar

---

# 9. Awards

## AwardDefinition
- name
- organization_id
- criteria:
    - includes_taxons[]
    - excludes_taxons[]
    - scoring (sum, max, custom)

Examples:
- High Points
- High Points Novice
- Best Rose

---

# 10. Voting

## Vote
- entry_id
- user_id (optional)
- weight

---

# 11. Media

## Media
- id
- entry_id
- url
- type
- metadata

---

# 12. Privacy & Suppression

- Public: entries, results, media
- Private: identity linkage
- Suppression: hide without deletion

---

# 13. Key Design Principles

- Graph-first taxonomy
- Structured domains
- Flexible awards
- API-first ingestion
- Append-only with suppression

---

# 14. Future Extensions

- Cross-organization awards
- Federation analytics
- AI-assisted taxonomy normalization
