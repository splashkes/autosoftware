# Flower Show Competition – Taxonomy & Data Model (v2.0)

## Overview
Full taxonomy and data model for a federated flower show competition system with standards-backed schedule hierarchy, rubric scoring, and source provenance.

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

# 5. Standards & Editions

## standard_document
- id
- name
- issuing_org_id
- domain_scope
- description

Examples: OJES (Ontario Judging and Exhibiting Standards), Publication 34

## standard_edition
- id
- standard_document_id
- edition_label
- publication_year
- revision_date
- status (current, superseded, draft)
- source_url
- source_kind (official_pdf, print_only, excerpt_pdf, catalog_record)

---

# 6. Source Documents & Citations

## source_document
- id
- organization_id
- show_id (nullable)
- title
- document_type (rulebook, schedule, fair_book, newsletter, results_sheet, catalog_record)
- publication_date
- source_url
- local_path (nullable)
- checksum (nullable)

## source_citation
- id
- source_document_id
- target_type
- target_id
- page_from
- page_to
- quoted_text (nullable)
- extraction_confidence

---

# 7. Schedule Hierarchy

## show_schedule
- id
- show_id
- source_document_id
- effective_standard_edition_id (nullable)
- notes

## division
- id
- show_schedule_id
- code (nullable)
- title
- domain (horticulture, design, special, other)
- sort_order

## section
- id
- division_id
- code (nullable)
- title
- sort_order

## show_class
- id
- section_id
- class_number
- title
- domain
- description
- specimen_count (nullable)
- unit (nullable)
- measurement_rule (nullable)
- naming_requirement (nullable)
- container_rule (nullable)
- eligibility_rule (nullable)
- schedule_notes (nullable)
- taxon_refs[]

Replaces the earlier generic `category` concept.

---

# 8. Standard Rules & Overrides

## standard_rule
- id
- standard_edition_id
- domain
- rule_type (definition, presentation, measurement, eligibility, scale_of_points, naming)
- subject_label
- body
- page_ref (nullable)

## class_rule_override
- id
- show_class_id
- base_standard_rule_id (nullable)
- override_type (replace, narrow, extend, local_only)
- body
- rationale (nullable)

---

# 9. Entry Model

## Entry
- id
- show_id
- show_class_id
- person_id
- placement
- points
- media_ids[]

---

# 10. Taxonomy Model

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

# 11. Domain-Specific Models

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

# 12. Scientific Names

Structured:
- genus
- species
- subspecies
- cultivar

---

# 13. Judging & Rubric Scoring

## judging_rubric
- id
- standard_edition_id (nullable)
- show_id (nullable)
- domain
- title

## judging_criterion
- id
- judging_rubric_id
- name
- max_points
- sort_order

## entry_scorecard
- id
- entry_id
- judge_id
- rubric_id
- total_score
- notes (nullable)

## entry_criterion_score
- id
- entry_scorecard_id
- criterion_id
- score
- comment (nullable)

Placements and awards computed from scorecards when present.

---

# 14. Awards

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

# 15. Voting

## Vote
- entry_id
- user_id (optional)
- weight

---

# 16. Media

## Media
- id
- entry_id
- url
- type (photo, video)
- metadata (dimensions, duration, original_filename)
- storage_key (S3 key)

Multiple photos and videos per entry. Client sends optimized files; server transcodes if exceeding threshold.

---

# 17. Privacy & Suppression

- Public: entries, results, media
- Private: identity linkage
- Suppression: hide without deletion

---

# 18. Key Design Principles

- Standards and provenance as first-class layers
- Schedule hierarchy (division → section → class)
- Rule inheritance with local overrides
- Graph-first taxonomy
- Rubric-capable scoring
- Structured domains
- Flexible awards
- API-first ingestion
- Append-only with suppression

---

# 19. Future Extensions

- Cross-organization awards
- Federation analytics
- AI-assisted taxonomy normalization
- Real-time ingestion tools
- Historical data migration
