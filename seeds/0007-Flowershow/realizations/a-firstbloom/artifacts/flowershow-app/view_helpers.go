package main

import (
	"fmt"
	"sort"
	"strings"
)

type showJudgeView struct {
	Assignment *ShowJudgeAssignment
	Person     *Person
}

type showCreditView struct {
	Credit *ShowCredit
	Person *Person
}

type personLookupView struct {
	Person           *Person
	OrganizationID   string
	OrganizationName string
	AffiliationRole  string
	Label            string
}

type boardStats struct {
	TotalEntries       int `json:"total_entries"`
	ClassesWithEntries int `json:"classes_with_entries"`
	MissingPhotos      int `json:"missing_photos"`
	SuppressedEntries  int `json:"suppressed_entries"`
	PlacedEntries      int `json:"placed_entries"`
}

type boardDivisionView struct {
	Division *Division           `json:"division"`
	Sections []*boardSectionView `json:"sections"`
}

type boardSectionView struct {
	Section *Section          `json:"section"`
	Classes []*boardClassView `json:"classes"`
}

type boardClassView struct {
	Class             *ShowClass   `json:"class"`
	Entries           []*entryView `json:"entries"`
	EntryCount        int          `json:"entry_count"`
	MissingPhotoCount int          `json:"missing_photo_count"`
	PlacedCount       int          `json:"placed_count"`
}

type standardView struct {
	Standard *StandardDocument
	Editions []*StandardEdition
}

type rubricView struct {
	Rubric   *JudgingRubric
	Criteria []*JudgingCriterion
}

type classRuleView struct {
	Class     *ShowClass
	Rules     []effectiveRule
	Citations []*SourceCitation
}

type citationTargetOption struct {
	Value string
	Label string
}

func isPublicEntry(entry *Entry) bool {
	return entry != nil && !entry.Suppressed
}

func (a *app) standardViews() []*standardView {
	standards := a.store.allStandardDocuments()
	out := make([]*standardView, 0, len(standards))
	for _, std := range standards {
		view := &standardView{
			Standard: std,
			Editions: a.store.editionsByStandard(std.ID),
		}
		sort.Slice(view.Editions, func(i, j int) bool {
			if view.Editions[i].PublicationYear == view.Editions[j].PublicationYear {
				return view.Editions[i].EditionLabel < view.Editions[j].EditionLabel
			}
			return view.Editions[i].PublicationYear > view.Editions[j].PublicationYear
		})
		out = append(out, view)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Standard.Name < out[j].Standard.Name })
	return out
}

func (a *app) judgeViewsForShow(showID string) []*showJudgeView {
	assignments := a.store.judgesByShow(showID)
	out := make([]*showJudgeView, 0, len(assignments))
	for _, assignment := range assignments {
		person, _ := a.store.personByID(assignment.PersonID)
		out = append(out, &showJudgeView{
			Assignment: assignment,
			Person:     person,
		})
	}
	return out
}

func (a *app) availableJudgesForShow(showID string) []*Person {
	assigned := map[string]bool{}
	for _, assignment := range a.store.judgesByShow(showID) {
		assigned[assignment.PersonID] = true
	}
	var out []*Person
	for _, person := range a.store.allPersons() {
		if !assigned[person.ID] {
			out = append(out, person)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].LastName+out[i].FirstName < out[j].LastName+out[j].FirstName
	})
	return out
}

func (a *app) personLookupViewsForShow(showID, query string) []*personLookupView {
	items := a.store.lookupPersonsForShow(showID, query)
	out := make([]*personLookupView, 0, len(items))
	for _, item := range items {
		person, ok := a.store.personByID(item.PersonID)
		if !ok {
			continue
		}
		org, _ := a.store.organizationByID(item.OrganizationID)
		orgName := item.OrganizationID
		if org != nil {
			orgName = org.Name
		}
		label := fmt.Sprintf("%s %s · %s · %s", person.FirstName, person.LastName, item.Role, orgName)
		out = append(out, &personLookupView{
			Person:           person,
			OrganizationID:   item.OrganizationID,
			OrganizationName: orgName,
			AffiliationRole:  item.Role,
			Label:            label,
		})
	}
	return out
}

func (a *app) recentEntryViews(entries []*entryView, limit int) []*entryView {
	if limit <= 0 {
		return nil
	}
	out := append([]*entryView(nil), entries...)
	sort.Slice(out, func(i, j int) bool {
		return out[i].Entry.CreatedAt.After(out[j].Entry.CreatedAt)
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

func (a *app) entriesNeedingPhotos(entries []*entryView) []*entryView {
	out := make([]*entryView, 0)
	for _, item := range entries {
		if len(item.Media) == 0 {
			out = append(out, item)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Entry.CreatedAt.After(out[j].Entry.CreatedAt)
	})
	return out
}

func (a *app) boardDataForShow(showID string, entries []*entryView) ([]*boardDivisionView, boardStats) {
	byClass := make(map[string][]*entryView)
	stats := boardStats{TotalEntries: len(entries)}
	classesWithEntries := make(map[string]struct{})
	for _, entry := range entries {
		byClass[entry.Entry.ClassID] = append(byClass[entry.Entry.ClassID], entry)
		if len(entry.Media) == 0 {
			stats.MissingPhotos++
		}
		if entry.Entry.Suppressed {
			stats.SuppressedEntries++
		}
		if entry.Entry.Placement > 0 {
			stats.PlacedEntries++
		}
	}

	show, ok := a.store.showByID(showID)
	if !ok {
		return nil, stats
	}
	schedule, ok := a.store.scheduleByShowID(show.ID)
	if !ok {
		return nil, stats
	}

	var divisions []*boardDivisionView
	for _, div := range a.store.divisionsBySchedule(schedule.ID) {
		divView := &boardDivisionView{Division: div}
		for _, sec := range a.store.sectionsByDivision(div.ID) {
			secView := &boardSectionView{Section: sec}
			for _, cls := range a.store.classesBySection(sec.ID) {
				classEntries := append([]*entryView(nil), byClass[cls.ID]...)
				sort.Slice(classEntries, func(i, j int) bool {
					if classEntries[i].Entry.Placement != classEntries[j].Entry.Placement {
						return classEntries[i].Entry.Placement < classEntries[j].Entry.Placement
					}
					return classEntries[i].Entry.CreatedAt.Before(classEntries[j].Entry.CreatedAt)
				})
				classView := &boardClassView{
					Class:      cls,
					Entries:    classEntries,
					EntryCount: len(classEntries),
				}
				for _, entry := range classEntries {
					if len(entry.Media) == 0 {
						classView.MissingPhotoCount++
					}
					if entry.Entry.Placement > 0 {
						classView.PlacedCount++
					}
				}
				if classView.EntryCount > 0 {
					classesWithEntries[cls.ID] = struct{}{}
				}
				secView.Classes = append(secView.Classes, classView)
			}
			divView.Sections = append(divView.Sections, secView)
		}
		divisions = append(divisions, divView)
	}
	stats.ClassesWithEntries = len(classesWithEntries)
	return divisions, stats
}

func (a *app) rubricViewsForShow(showID string) []*rubricView {
	rubrics := a.store.allRubrics()
	out := make([]*rubricView, 0, len(rubrics))
	for _, rubric := range rubrics {
		if strings.TrimSpace(rubric.ShowID) != "" && rubric.ShowID != showID {
			continue
		}
		out = append(out, &rubricView{
			Rubric:   rubric,
			Criteria: a.store.criteriaByRubric(rubric.ID),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Rubric.Title < out[j].Rubric.Title })
	return out
}

func (a *app) classRuleViews(showID, editionID string) []*classRuleView {
	if strings.TrimSpace(editionID) == "" {
		return nil
	}
	classes := a.store.classesByShowID(showID)
	out := make([]*classRuleView, 0, len(classes))
	for _, cls := range classes {
		rules := a.store.effectiveRulesForClass(cls.ID, editionID)
		if len(rules) == 0 {
			continue
		}
		citations := a.store.citationsByTarget("show_class", cls.ID)
		out = append(out, &classRuleView{
			Class:     cls,
			Rules:     rules,
			Citations: citations,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Class.ClassNumber < out[j].Class.ClassNumber })
	return out
}

func (a *app) citationTargetsForShow(showID string, ruleViews []*classRuleView) []citationTargetOption {
	var out []citationTargetOption
	for _, cls := range a.store.classesByShowID(showID) {
		out = append(out, citationTargetOption{
			Value: fmt.Sprintf("show_class:%s", cls.ID),
			Label: fmt.Sprintf("Class %s — %s", cls.ClassNumber, cls.Title),
		})
	}
	for _, entry := range a.store.entriesByShow(showID) {
		out = append(out, citationTargetOption{
			Value: fmt.Sprintf("entry:%s", entry.ID),
			Label: fmt.Sprintf("Entry — %s", entry.Name),
		})
	}
	for _, view := range ruleViews {
		for _, rule := range view.Rules {
			if rule.Rule != nil {
				out = append(out, citationTargetOption{
					Value: fmt.Sprintf("standard_rule:%s", rule.Rule.ID),
					Label: fmt.Sprintf("Rule — %s", rule.Rule.SubjectLabel),
				})
			}
			if rule.Override != nil {
				out = append(out, citationTargetOption{
					Value: fmt.Sprintf("class_rule_override:%s", rule.Override.ID),
					Label: fmt.Sprintf("Override — %s class %s", rule.Override.OverrideType, view.Class.ClassNumber),
				})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Label < out[j].Label })
	return out
}
