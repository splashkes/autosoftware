package main

import (
	"encoding/json"
	"fmt"
	"sort"
)

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
