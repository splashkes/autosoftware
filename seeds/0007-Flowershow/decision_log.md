# Decision Log

## 1. API-First Ingestion
All data entry is done via API and external agents.

## 2. No Admin UI (v1)
Avoid building forms; rely on structured ingestion.

## 3. Append-Only Model
No deletion; only suppression.

## 4. Flexible Taxonomy
Graph-like tagging system instead of rigid schema.

## 5. Separate Domains
Horticulture and Design modeled separately.

## 6. Organization Hierarchy
Support multi-level org structure (club → province → etc).

## 7. Integer Scoring
All monetary values handled in cents (no floats).

## 8. Identity Privacy
Public = initials, private = full identity mapping.
