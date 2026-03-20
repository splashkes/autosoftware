package main

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
)

var flowershowProjectionTables = []string{
	"as_flowershow_m_organizations",
	"as_flowershow_m_shows",
	"as_flowershow_m_show_judges",
	"as_flowershow_m_persons",
	"as_flowershow_m_person_organizations",
	"as_flowershow_m_organization_invites",
	"as_flowershow_m_schedules",
	"as_flowershow_m_divisions",
	"as_flowershow_m_sections",
	"as_flowershow_m_classes",
	"as_flowershow_m_entries",
	"as_flowershow_m_show_credits",
	"as_flowershow_m_media",
	"as_flowershow_m_taxons",
	"as_flowershow_m_awards",
	"as_flowershow_m_standard_documents",
	"as_flowershow_m_standard_editions",
	"as_flowershow_m_source_documents",
	"as_flowershow_m_source_citations",
	"as_flowershow_m_standard_rules",
	"as_flowershow_m_class_rule_overrides",
	"as_flowershow_m_rubrics",
	"as_flowershow_m_criteria",
	"as_flowershow_m_scorecards",
	"as_flowershow_m_criterion_scores",
}

func flowershowProjectionExpectedCounts(mem *memoryStore) map[string]int {
	if mem == nil {
		return map[string]int{}
	}
	return map[string]int{
		"as_flowershow_m_organizations":        len(mem.organizations),
		"as_flowershow_m_shows":                len(mem.shows),
		"as_flowershow_m_show_judges":          len(mem.showJudges),
		"as_flowershow_m_persons":              len(mem.persons),
		"as_flowershow_m_person_organizations": len(mem.personOrgs),
		"as_flowershow_m_organization_invites": len(mem.orgInvites),
		"as_flowershow_m_schedules":            len(mem.schedules),
		"as_flowershow_m_divisions":            len(mem.divisions),
		"as_flowershow_m_sections":             len(mem.sections),
		"as_flowershow_m_classes":              len(mem.classes),
		"as_flowershow_m_entries":              len(mem.entries),
		"as_flowershow_m_show_credits":         len(mem.showCredits),
		"as_flowershow_m_media":                len(mem.media),
		"as_flowershow_m_taxons":               len(mem.taxons),
		"as_flowershow_m_awards":               len(mem.awards),
		"as_flowershow_m_standard_documents":   len(mem.stdDocs),
		"as_flowershow_m_standard_editions":    len(mem.stdEditions),
		"as_flowershow_m_source_documents":     len(mem.srcDocs),
		"as_flowershow_m_source_citations":     len(mem.srcCitations),
		"as_flowershow_m_standard_rules":       len(mem.stdRules),
		"as_flowershow_m_class_rule_overrides": len(mem.classOverrides),
		"as_flowershow_m_rubrics":              len(mem.rubrics),
		"as_flowershow_m_criteria":             len(mem.criteria),
		"as_flowershow_m_scorecards":           len(mem.scorecards),
		"as_flowershow_m_criterion_scores":     len(mem.critScores),
	}
}

func sortedMapKeys[T any](items map[string]*T) []string {
	keys := make([]string, 0, len(items))
	for key := range items {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func stringSliceOrEmpty(items []string) []string {
	if items == nil {
		return []string{}
	}
	return items
}

func (s *postgresFlowershowStore) projectionCounts(ctx context.Context) (map[string]int, error) {
	counts := make(map[string]int, len(flowershowProjectionTables))
	for _, table := range flowershowProjectionTables {
		var count int
		if err := s.pool.QueryRow(ctx, "SELECT COUNT(*) FROM "+table).Scan(&count); err != nil {
			return nil, fmt.Errorf("count %s: %w", table, err)
		}
		counts[table] = count
	}
	return counts, nil
}

func (s *postgresFlowershowStore) projectionsNeedRebuild(ctx context.Context, mem *memoryStore) (bool, error) {
	expected := flowershowProjectionExpectedCounts(mem)
	counts, err := s.projectionCounts(ctx)
	if err != nil {
		return false, err
	}
	for _, table := range flowershowProjectionTables {
		if counts[table] != expected[table] {
			return true, nil
		}
	}
	return false, nil
}

func (s *postgresFlowershowStore) rebuildProjectionTablesFromSnapshot(ctx context.Context, mem *memoryStore) error {
	if s == nil || s.pool == nil || mem == nil {
		return nil
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin projection rebuild tx: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	if err := s.rebuildProjectionTablesFromSnapshotTx(ctx, tx, mem); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit projection rebuild tx: %w", err)
	}
	return nil
}

func (s *postgresFlowershowStore) rebuildProjectionTablesFromSnapshotTx(ctx context.Context, tx pgx.Tx, mem *memoryStore) error {
	if _, err := tx.Exec(ctx, "TRUNCATE "+strings.Join(flowershowProjectionTables, ", ")); err != nil {
		return fmt.Errorf("truncate flowershow projections: %w", err)
	}

	for _, id := range sortedMapKeys(mem.organizations) {
		item := mem.organizations[id]
		if _, err := tx.Exec(ctx, `INSERT INTO as_flowershow_m_organizations (id, name, level, parent_id) VALUES ($1,$2,$3,$4)`,
			item.ID, item.Name, item.Level, nullableString(item.ParentID)); err != nil {
			return fmt.Errorf("insert organization projection %s: %w", item.ID, err)
		}
	}
	for _, id := range sortedMapKeys(mem.shows) {
		item := mem.shows[id]
		if _, err := tx.Exec(ctx, `INSERT INTO as_flowershow_m_shows (id, slug, organization_id, name, location, show_date, season, status, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
			item.ID, item.Slug, item.OrganizationID, item.Name, item.Location, item.Date, item.Season, item.Status, item.CreatedAt, item.UpdatedAt); err != nil {
			return fmt.Errorf("insert show projection %s: %w", item.ID, err)
		}
	}
	for _, id := range sortedMapKeys(mem.showJudges) {
		item := mem.showJudges[id]
		if _, err := tx.Exec(ctx, `INSERT INTO as_flowershow_m_show_judges (id, show_id, person_id, assigned_at) VALUES ($1,$2,$3,$4)`,
			item.ID, item.ShowID, item.PersonID, item.AssignedAt); err != nil {
			return fmt.Errorf("insert show judge projection %s: %w", item.ID, err)
		}
	}
	for _, id := range sortedMapKeys(mem.persons) {
		item := mem.persons[id]
		if _, err := tx.Exec(ctx, `INSERT INTO as_flowershow_m_persons (id, first_name, last_name, initials, email, public_display_mode) VALUES ($1,$2,$3,$4,$5,$6)`,
			item.ID, item.FirstName, item.LastName, item.Initials, item.Email, item.PublicDisplayMode); err != nil {
			return fmt.Errorf("insert person projection %s: %w", item.ID, err)
		}
	}
	personOrgKeys := make([]string, 0, len(mem.personOrgs))
	for key := range mem.personOrgs {
		personOrgKeys = append(personOrgKeys, key)
	}
	sort.Strings(personOrgKeys)
	for _, key := range personOrgKeys {
		item := mem.personOrgs[key]
		if _, err := tx.Exec(ctx, `INSERT INTO as_flowershow_m_person_organizations (person_id, organization_id, role) VALUES ($1,$2,$3)`,
			item.PersonID, item.OrganizationID, item.Role); err != nil {
			return fmt.Errorf("insert person organization projection %s: %w", key, err)
		}
	}
	for _, id := range sortedMapKeys(mem.orgInvites) {
		item := mem.orgInvites[id]
		claimedAt := any(nil)
		if !item.ClaimedAt.IsZero() {
			claimedAt = item.ClaimedAt
		}
		if _, err := tx.Exec(ctx, `INSERT INTO as_flowershow_m_organization_invites (id, organization_id, first_name, last_name, email, organization_role, permission_roles, status, invited_by_subject, invited_by_name, invited_at, claimed_subject_id, claimed_cognito_sub, claimed_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`,
			item.ID, item.OrganizationID, item.FirstName, item.LastName, item.Email, item.OrganizationRole, stringSliceOrEmpty(item.PermissionRoles), item.Status, item.InvitedBySubject, item.InvitedByName, item.InvitedAt, item.ClaimedSubjectID, item.ClaimedCognitoSub, claimedAt); err != nil {
			return fmt.Errorf("insert organization invite projection %s: %w", item.ID, err)
		}
	}
	for _, id := range sortedMapKeys(mem.schedules) {
		item := mem.schedules[id]
		if _, err := tx.Exec(ctx, `INSERT INTO as_flowershow_m_schedules (id, show_id, source_document_id, effective_standard_edition_id, notes) VALUES ($1,$2,$3,$4,$5)`,
			item.ID, item.ShowID, nullableString(item.SourceDocumentID), nullableString(item.EffectiveStandardEditionID), item.Notes); err != nil {
			return fmt.Errorf("insert schedule projection %s: %w", item.ID, err)
		}
	}
	for _, id := range sortedMapKeys(mem.divisions) {
		item := mem.divisions[id]
		if _, err := tx.Exec(ctx, `INSERT INTO as_flowershow_m_divisions (id, show_schedule_id, code, title, domain, sort_order) VALUES ($1,$2,$3,$4,$5,$6)`,
			item.ID, item.ShowScheduleID, item.Code, item.Title, item.Domain, item.SortOrder); err != nil {
			return fmt.Errorf("insert division projection %s: %w", item.ID, err)
		}
	}
	for _, id := range sortedMapKeys(mem.sections) {
		item := mem.sections[id]
		if _, err := tx.Exec(ctx, `INSERT INTO as_flowershow_m_sections (id, division_id, code, title, sort_order) VALUES ($1,$2,$3,$4,$5)`,
			item.ID, item.DivisionID, item.Code, item.Title, item.SortOrder); err != nil {
			return fmt.Errorf("insert section projection %s: %w", item.ID, err)
		}
	}
	for _, id := range sortedMapKeys(mem.classes) {
		item := mem.classes[id]
		if _, err := tx.Exec(ctx, `INSERT INTO as_flowershow_m_classes (id, section_id, class_number, sort_order, title, domain, description, specimen_count, unit, measurement_rule, naming_requirement, container_rule, eligibility_rule, schedule_notes, taxon_refs) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)`,
			item.ID, item.SectionID, item.ClassNumber, item.SortOrder, item.Title, item.Domain, item.Description, item.SpecimenCount, item.Unit, item.MeasurementRule, item.NamingRequirement, item.ContainerRule, item.EligibilityRule, item.ScheduleNotes, stringSliceOrEmpty(item.TaxonRefs)); err != nil {
			return fmt.Errorf("insert class projection %s: %w", item.ID, err)
		}
	}
	for _, id := range sortedMapKeys(mem.entries) {
		item := mem.entries[id]
		if _, err := tx.Exec(ctx, `INSERT INTO as_flowershow_m_entries (id, show_id, class_id, person_id, name, notes, suppressed, placement, points, taxon_refs, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
			item.ID, item.ShowID, item.ClassID, item.PersonID, item.Name, item.Notes, item.Suppressed, item.Placement, item.Points, stringSliceOrEmpty(item.TaxonRefs), item.CreatedAt); err != nil {
			return fmt.Errorf("insert entry projection %s: %w", item.ID, err)
		}
	}
	for _, id := range sortedMapKeys(mem.showCredits) {
		item := mem.showCredits[id]
		if _, err := tx.Exec(ctx, `INSERT INTO as_flowershow_m_show_credits (id, show_id, person_id, display_name, credit_label, notes, sort_order, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
			item.ID, item.ShowID, item.PersonID, item.DisplayName, item.CreditLabel, item.Notes, item.SortOrder, item.CreatedAt); err != nil {
			return fmt.Errorf("insert show credit projection %s: %w", item.ID, err)
		}
	}
	for _, id := range sortedMapKeys(mem.media) {
		item := mem.media[id]
		if _, err := tx.Exec(ctx, `INSERT INTO as_flowershow_m_media (id, entry_id, media_type, url, content_type, thumbnail_url, file_name, storage_key, file_size, width, height, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
			item.ID, item.EntryID, item.MediaType, item.URL, item.ContentType, item.ThumbnailURL, item.FileName, item.StorageKey, item.FileSize, item.Width, item.Height, item.CreatedAt); err != nil {
			return fmt.Errorf("insert media projection %s: %w", item.ID, err)
		}
	}
	for _, id := range sortedMapKeys(mem.taxons) {
		item := mem.taxons[id]
		if _, err := tx.Exec(ctx, `INSERT INTO as_flowershow_m_taxons (id, taxon_type, name, scientific_name, description, parent_id) VALUES ($1,$2,$3,$4,$5,$6)`,
			item.ID, item.TaxonType, item.Name, item.ScientificName, item.Description, nullableString(item.ParentID)); err != nil {
			return fmt.Errorf("insert taxon projection %s: %w", item.ID, err)
		}
	}
	for _, id := range sortedMapKeys(mem.awards) {
		item := mem.awards[id]
		if _, err := tx.Exec(ctx, `INSERT INTO as_flowershow_m_awards (id, organization_id, name, description, season, taxon_filters, scoring_rule, min_entries) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
			item.ID, item.OrganizationID, item.Name, item.Description, item.Season, stringSliceOrEmpty(item.TaxonFilters), item.ScoringRule, item.MinEntries); err != nil {
			return fmt.Errorf("insert award projection %s: %w", item.ID, err)
		}
	}
	for _, id := range sortedMapKeys(mem.stdDocs) {
		item := mem.stdDocs[id]
		if _, err := tx.Exec(ctx, `INSERT INTO as_flowershow_m_standard_documents (id, name, issuing_org_id, domain_scope, description) VALUES ($1,$2,$3,$4,$5)`,
			item.ID, item.Name, item.IssuingOrg, item.DomainScope, item.Description); err != nil {
			return fmt.Errorf("insert standard document projection %s: %w", item.ID, err)
		}
	}
	for _, id := range sortedMapKeys(mem.stdEditions) {
		item := mem.stdEditions[id]
		if _, err := tx.Exec(ctx, `INSERT INTO as_flowershow_m_standard_editions (id, standard_document_id, edition_label, publication_year, revision_date, status, source_url, source_kind) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
			item.ID, item.StandardDocumentID, item.EditionLabel, item.PublicationYear, item.RevisionDate, item.Status, item.SourceURL, item.SourceKind); err != nil {
			return fmt.Errorf("insert standard edition projection %s: %w", item.ID, err)
		}
	}
	for _, id := range sortedMapKeys(mem.srcDocs) {
		item := mem.srcDocs[id]
		if _, err := tx.Exec(ctx, `INSERT INTO as_flowershow_m_source_documents (id, organization_id, show_id, title, document_type, publication_date, source_url, local_path, checksum) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
			item.ID, item.OrganizationID, nullableString(item.ShowID), item.Title, item.DocumentType, item.PublicationDate, item.SourceURL, item.LocalPath, item.Checksum); err != nil {
			return fmt.Errorf("insert source document projection %s: %w", item.ID, err)
		}
	}
	for _, id := range sortedMapKeys(mem.srcCitations) {
		item := mem.srcCitations[id]
		if _, err := tx.Exec(ctx, `INSERT INTO as_flowershow_m_source_citations (id, source_document_id, target_type, target_id, page_from, page_to, quoted_text, extraction_confidence) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
			item.ID, item.SourceDocumentID, item.TargetType, item.TargetID, item.PageFrom, item.PageTo, item.QuotedText, item.ExtractionConfidence); err != nil {
			return fmt.Errorf("insert source citation projection %s: %w", item.ID, err)
		}
	}
	for _, id := range sortedMapKeys(mem.stdRules) {
		item := mem.stdRules[id]
		if _, err := tx.Exec(ctx, `INSERT INTO as_flowershow_m_standard_rules (id, standard_edition_id, domain, rule_type, subject_label, body, page_ref) VALUES ($1,$2,$3,$4,$5,$6,$7)`,
			item.ID, item.StandardEditionID, item.Domain, item.RuleType, item.SubjectLabel, item.Body, item.PageRef); err != nil {
			return fmt.Errorf("insert standard rule projection %s: %w", item.ID, err)
		}
	}
	for _, id := range sortedMapKeys(mem.classOverrides) {
		item := mem.classOverrides[id]
		if _, err := tx.Exec(ctx, `INSERT INTO as_flowershow_m_class_rule_overrides (id, show_class_id, base_standard_rule_id, override_type, body, rationale) VALUES ($1,$2,$3,$4,$5,$6)`,
			item.ID, item.ShowClassID, nullableString(item.BaseStandardRuleID), item.OverrideType, item.Body, item.Rationale); err != nil {
			return fmt.Errorf("insert class override projection %s: %w", item.ID, err)
		}
	}
	for _, id := range sortedMapKeys(mem.rubrics) {
		item := mem.rubrics[id]
		if _, err := tx.Exec(ctx, `INSERT INTO as_flowershow_m_rubrics (id, standard_edition_id, show_id, domain, title) VALUES ($1,$2,$3,$4,$5)`,
			item.ID, nullableString(item.StandardEditionID), nullableString(item.ShowID), item.Domain, item.Title); err != nil {
			return fmt.Errorf("insert rubric projection %s: %w", item.ID, err)
		}
	}
	for _, id := range sortedMapKeys(mem.criteria) {
		item := mem.criteria[id]
		if _, err := tx.Exec(ctx, `INSERT INTO as_flowershow_m_criteria (id, judging_rubric_id, name, max_points, sort_order) VALUES ($1,$2,$3,$4,$5)`,
			item.ID, item.JudgingRubricID, item.Name, item.MaxPoints, item.SortOrder); err != nil {
			return fmt.Errorf("insert criterion projection %s: %w", item.ID, err)
		}
	}
	for _, id := range sortedMapKeys(mem.scorecards) {
		item := mem.scorecards[id]
		if _, err := tx.Exec(ctx, `INSERT INTO as_flowershow_m_scorecards (id, entry_id, judge_id, rubric_id, total_score, notes) VALUES ($1,$2,$3,$4,$5,$6)`,
			item.ID, item.EntryID, item.JudgeID, item.RubricID, item.TotalScore, item.Notes); err != nil {
			return fmt.Errorf("insert scorecard projection %s: %w", item.ID, err)
		}
	}
	for _, id := range sortedMapKeys(mem.critScores) {
		item := mem.critScores[id]
		if _, err := tx.Exec(ctx, `INSERT INTO as_flowershow_m_criterion_scores (id, scorecard_id, criterion_id, score, comment) VALUES ($1,$2,$3,$4,$5)`,
			item.ID, item.ScorecardID, item.CriterionID, item.Score, item.Comment); err != nil {
			return fmt.Errorf("insert criterion score projection %s: %w", item.ID, err)
		}
	}

	return nil
}
