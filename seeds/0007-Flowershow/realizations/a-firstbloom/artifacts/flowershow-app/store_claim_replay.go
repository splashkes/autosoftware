package main

import (
	"encoding/json"
	"fmt"
	"sort"
)

var replayableFlowershowClaimTypes = map[string]struct{}{
	"organization.created":           {},
	"show.created":                   {},
	"show.updated":                   {},
	"show.schedule_reset":            {},
	"show_judge.assigned":            {},
	"person.created":                 {},
	"person.updated":                 {},
	"person.organization_linked":     {},
	"organization.invite_created":    {},
	"organization.invite_claimed":    {},
	"schedule.created":               {},
	"schedule.updated":               {},
	"division.created":               {},
	"section.created":                {},
	"class.created":                  {},
	"class.updated":                  {},
	"class.reordered":                {},
	"entry.created":                  {},
	"entry.updated":                  {},
	"entry.moved":                    {},
	"entry.reassigned":               {},
	"entry.deleted":                  {},
	"entry.visibility_set":           {},
	"entry.placement_set":            {},
	"show_credit.created":            {},
	"show_credit.deleted":            {},
	"media.attached":                 {},
	"media.deleted":                  {},
	"taxon.created":                  {},
	"award.created":                  {},
	"standard_document.created":      {},
	"standard_edition.created":       {},
	"source_document.created":        {},
	"source_citation.created":        {},
	"standard_rule.created":          {},
	"class_rule_override.created":    {},
	"rubric.created":                 {},
	"criterion.created":              {},
	"scorecard.submitted":            {},
	"show_class.placements_computed": {},
}

func decodeFlowershowClaimPayload[T any](claim FlowershowClaim) (T, error) {
	var out T
	payload, err := json.Marshal(claim.Payload)
	if err != nil {
		return out, fmt.Errorf("marshal claim payload for %s: %w", claim.ClaimType, err)
	}
	if err := json.Unmarshal(payload, &out); err != nil {
		return out, fmt.Errorf("decode claim payload for %s: %w", claim.ClaimType, err)
	}
	return out, nil
}

func replayFlowershowSnapshotFromClaims(objects map[string]*FlowershowObject, claims []FlowershowClaim) (*memoryStore, error) {
	fresh := newEmptyMemoryStore()
	for id, object := range objects {
		copy := *object
		fresh.objects[id] = &copy
	}
	fresh.claims = append([]FlowershowClaim(nil), claims...)
	classPlacementRecomputes := make(map[string]struct{})
	for _, claim := range claims {
		if claim.ClaimSeq > fresh.claimSeq {
			fresh.claimSeq = claim.ClaimSeq
		}
		switch claim.ClaimType {
		case "organization.created":
			item, err := decodeFlowershowClaimPayload[Organization](claim)
			if err != nil {
				return nil, err
			}
			fresh.organizations[item.ID] = &item
		case "show.created", "show.updated":
			item, err := decodeFlowershowClaimPayload[Show](claim)
			if err != nil {
				return nil, err
			}
			fresh.shows[item.ID] = &item
		case "show.schedule_reset":
			payload, err := decodeFlowershowClaimPayload[struct {
				ScheduleIDs        []string `json:"schedule_ids"`
				DivisionIDs        []string `json:"division_ids"`
				SectionIDs         []string `json:"section_ids"`
				ClassIDs           []string `json:"class_ids"`
				EntryIDs           []string `json:"entry_ids"`
				MediaIDs           []string `json:"media_ids"`
				ScorecardIDs       []string `json:"scorecard_ids"`
				CriterionScoreIDs  []string `json:"criterion_score_ids"`
				JudgeAssignmentIDs []string `json:"judge_assignment_ids"`
				ClassOverrideIDs   []string `json:"class_override_ids"`
				SourceCitationIDs  []string `json:"source_citation_ids"`
			}](claim)
			if err != nil {
				return nil, err
			}
			for _, id := range payload.MediaIDs {
				delete(fresh.media, id)
			}
			for _, id := range payload.CriterionScoreIDs {
				delete(fresh.critScores, id)
			}
			for _, id := range payload.ScorecardIDs {
				delete(fresh.scorecards, id)
			}
			for _, id := range payload.EntryIDs {
				delete(fresh.entries, id)
			}
			for _, id := range payload.ClassOverrideIDs {
				delete(fresh.classOverrides, id)
			}
			for _, id := range payload.SourceCitationIDs {
				delete(fresh.srcCitations, id)
			}
			for _, id := range payload.ClassIDs {
				delete(fresh.classes, id)
			}
			for _, id := range payload.SectionIDs {
				delete(fresh.sections, id)
			}
			for _, id := range payload.DivisionIDs {
				delete(fresh.divisions, id)
			}
			for _, id := range payload.ScheduleIDs {
				delete(fresh.schedules, id)
			}
			for _, id := range payload.JudgeAssignmentIDs {
				delete(fresh.showJudges, id)
			}
		case "show_judge.assigned":
			item, err := decodeFlowershowClaimPayload[ShowJudgeAssignment](claim)
			if err != nil {
				return nil, err
			}
			fresh.showJudges[item.ID] = &item
		case "person.created", "person.updated":
			item, err := decodeFlowershowClaimPayload[Person](claim)
			if err != nil {
				return nil, err
			}
			fresh.persons[item.ID] = &item
		case "person.organization_linked":
			item, err := decodeFlowershowClaimPayload[PersonOrganization](claim)
			if err != nil {
				return nil, err
			}
			copy := item
			fresh.personOrgs[copy.PersonID+"|"+copy.OrganizationID+"|"+copy.Role] = &copy
		case "organization.invite_created", "organization.invite_claimed":
			item, err := decodeFlowershowClaimPayload[OrganizationInvite](claim)
			if err != nil {
				return nil, err
			}
			item.PermissionRoles = append([]string(nil), item.PermissionRoles...)
			fresh.orgInvites[item.ID] = &item
		case "schedule.created", "schedule.updated":
			item, err := decodeFlowershowClaimPayload[ShowSchedule](claim)
			if err != nil {
				return nil, err
			}
			fresh.schedules[item.ID] = &item
		case "division.created":
			item, err := decodeFlowershowClaimPayload[Division](claim)
			if err != nil {
				return nil, err
			}
			fresh.divisions[item.ID] = &item
		case "section.created":
			item, err := decodeFlowershowClaimPayload[Section](claim)
			if err != nil {
				return nil, err
			}
			fresh.sections[item.ID] = &item
		case "class.created", "class.updated":
			item, err := decodeFlowershowClaimPayload[ShowClass](claim)
			if err != nil {
				return nil, err
			}
			item.TaxonRefs = append([]string(nil), item.TaxonRefs...)
			fresh.classes[item.ID] = &item
		case "class.reordered":
			payload, err := decodeFlowershowClaimPayload[struct {
				SortOrder int `json:"sort_order"`
			}](claim)
			if err != nil {
				return nil, err
			}
			if item, ok := fresh.classes[claim.ObjectID]; ok {
				item.SortOrder = payload.SortOrder
			}
		case "entry.created", "entry.updated":
			item, err := decodeFlowershowClaimPayload[Entry](claim)
			if err != nil {
				return nil, err
			}
			item.TaxonRefs = append([]string(nil), item.TaxonRefs...)
			fresh.entries[item.ID] = &item
		case "entry.moved":
			payload, err := decodeFlowershowClaimPayload[struct {
				ClassID string `json:"class_id"`
			}](claim)
			if err != nil {
				return nil, err
			}
			if item, ok := fresh.entries[claim.ObjectID]; ok {
				item.ClassID = payload.ClassID
			}
		case "entry.reassigned":
			payload, err := decodeFlowershowClaimPayload[struct {
				PersonID string `json:"person_id"`
			}](claim)
			if err != nil {
				return nil, err
			}
			if item, ok := fresh.entries[claim.ObjectID]; ok {
				item.PersonID = payload.PersonID
			}
		case "entry.deleted":
			delete(fresh.entries, claim.ObjectID)
		case "entry.visibility_set":
			payload, err := decodeFlowershowClaimPayload[struct {
				Suppressed bool `json:"suppressed"`
			}](claim)
			if err != nil {
				return nil, err
			}
			if item, ok := fresh.entries[claim.ObjectID]; ok {
				item.Suppressed = payload.Suppressed
			}
		case "entry.placement_set":
			payload, err := decodeFlowershowClaimPayload[struct {
				Placement int     `json:"placement"`
				Points    float64 `json:"points"`
			}](claim)
			if err != nil {
				return nil, err
			}
			if item, ok := fresh.entries[claim.ObjectID]; ok {
				item.Placement = payload.Placement
				item.Points = payload.Points
			}
		case "show_credit.created":
			item, err := decodeFlowershowClaimPayload[ShowCredit](claim)
			if err != nil {
				return nil, err
			}
			fresh.showCredits[item.ID] = &item
		case "show_credit.deleted":
			delete(fresh.showCredits, claim.ObjectID)
		case "media.attached":
			item, err := decodeFlowershowClaimPayload[Media](claim)
			if err != nil {
				return nil, err
			}
			fresh.media[item.ID] = &item
		case "media.deleted":
			delete(fresh.media, claim.ObjectID)
		case "taxon.created":
			item, err := decodeFlowershowClaimPayload[Taxon](claim)
			if err != nil {
				return nil, err
			}
			fresh.taxons[item.ID] = &item
		case "award.created":
			item, err := decodeFlowershowClaimPayload[AwardDefinition](claim)
			if err != nil {
				return nil, err
			}
			item.TaxonFilters = append([]string(nil), item.TaxonFilters...)
			fresh.awards[item.ID] = &item
		case "standard_document.created":
			item, err := decodeFlowershowClaimPayload[StandardDocument](claim)
			if err != nil {
				return nil, err
			}
			fresh.stdDocs[item.ID] = &item
		case "standard_edition.created":
			item, err := decodeFlowershowClaimPayload[StandardEdition](claim)
			if err != nil {
				return nil, err
			}
			fresh.stdEditions[item.ID] = &item
		case "source_document.created":
			item, err := decodeFlowershowClaimPayload[SourceDocument](claim)
			if err != nil {
				return nil, err
			}
			fresh.srcDocs[item.ID] = &item
		case "source_citation.created":
			item, err := decodeFlowershowClaimPayload[SourceCitation](claim)
			if err != nil {
				return nil, err
			}
			fresh.srcCitations[item.ID] = &item
		case "standard_rule.created":
			item, err := decodeFlowershowClaimPayload[StandardRule](claim)
			if err != nil {
				return nil, err
			}
			fresh.stdRules[item.ID] = &item
		case "class_rule_override.created":
			item, err := decodeFlowershowClaimPayload[ClassRuleOverride](claim)
			if err != nil {
				return nil, err
			}
			fresh.classOverrides[item.ID] = &item
		case "rubric.created":
			item, err := decodeFlowershowClaimPayload[JudgingRubric](claim)
			if err != nil {
				return nil, err
			}
			fresh.rubrics[item.ID] = &item
		case "criterion.created":
			item, err := decodeFlowershowClaimPayload[JudgingCriterion](claim)
			if err != nil {
				return nil, err
			}
			fresh.criteria[item.ID] = &item
		case "scorecard.submitted":
			payload, err := decodeFlowershowClaimPayload[struct {
				Scorecard EntryScorecard        `json:"scorecard"`
				Scores    []EntryCriterionScore `json:"scores"`
			}](claim)
			if err != nil {
				return nil, err
			}
			scorecard := payload.Scorecard
			fresh.scorecards[scorecard.ID] = &scorecard
			for _, score := range payload.Scores {
				copy := score
				fresh.critScores[copy.ID] = &copy
			}
		case "show_class.placements_computed":
			payload, err := decodeFlowershowClaimPayload[struct {
				ClassID string `json:"class_id"`
			}](claim)
			if err != nil {
				return nil, err
			}
			if payload.ClassID != "" {
				classPlacementRecomputes[payload.ClassID] = struct{}{}
			}
		case "agent_token.issued", "agent_token.revoked":
			// Runtime state lives in dedicated tables and is not reconstructed from claims.
		default:
			return nil, fmt.Errorf("unsupported flowershow claim type %q", claim.ClaimType)
		}
	}
	for classID := range classPlacementRecomputes {
		recomputeEntryPlacementsFromScoresNoClaim(fresh, classID)
	}
	return fresh, nil
}

func recomputeEntryPlacementsFromScoresNoClaim(s *memoryStore, classID string) {
	classEntries := make([]*Entry, 0)
	for _, entry := range s.entries {
		if entry.ClassID == classID {
			entry.Placement = 0
			entry.Points = 0
			classEntries = append(classEntries, entry)
		}
	}
	type entryScore struct {
		entry    *Entry
		avgScore float64
	}
	scored := make([]entryScore, 0, len(classEntries))
	for _, entry := range classEntries {
		var total float64
		var count int
		for _, scorecard := range s.scorecards {
			if scorecard.EntryID == entry.ID {
				total += scorecard.TotalScore
				count++
			}
		}
		if count > 0 {
			scored = append(scored, entryScore{entry: entry, avgScore: total / float64(count)})
		}
	}
	sort.Slice(scored, func(i, j int) bool { return scored[i].avgScore > scored[j].avgScore })
	pointsMap := map[int]float64{1: 6, 2: 4, 3: 2}
	for index, item := range scored {
		placement := index + 1
		if placement <= 3 {
			item.entry.Placement = placement
			item.entry.Points = pointsMap[placement]
		}
	}
}
