package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// --- ID generation ---

func newID(prefix string) string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("%s_%d", prefix, time.Now().UTC().UnixNano())
	}
	return prefix + "_" + hex.EncodeToString(buf)
}

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			return r
		}
		if r == ' ' || r == '-' || r == '_' {
			return '-'
		}
		return -1
	}, s)
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	return strings.Trim(s, "-")
}

func organizationInitials(name string) string {
	words := strings.Fields(strings.TrimSpace(name))
	initials := make([]rune, 0, len(words))
	for _, word := range words {
		word = strings.TrimSpace(word)
		if word == "" {
			continue
		}
		runes := []rune(word)
		if len(runes) == 0 {
			continue
		}
		initials = append(initials, runes[0])
	}
	if len(initials) == 0 {
		return "show"
	}
	return strings.ToLower(string(initials))
}

func showSlugForInput(org *Organization, input ShowInput) string {
	dateDigits := strings.NewReplacer("-", "", "/", "", " ", "").Replace(strings.TrimSpace(input.Date))
	datePart := ""
	if len(dateDigits) >= 8 {
		datePart = dateDigits[:4] + "-" + dateDigits[4:8]
	} else if strings.TrimSpace(input.Season) != "" {
		datePart = strings.TrimSpace(input.Season)
	}
	base := "show"
	if org != nil {
		base = organizationInitials(org.Name)
	}
	if datePart == "" {
		return base
	}
	return base + datePart
}

func uniqueShowSlug(existing map[string]*Show, preferred, currentID string) string {
	preferred = slugify(preferred)
	if preferred == "" {
		preferred = "show"
	}
	candidate := preferred
	seq := 2
	for {
		conflict := false
		for _, show := range existing {
			if show == nil || show.ID == currentID {
				continue
			}
			if show.Slug == candidate {
				conflict = true
				break
			}
		}
		if !conflict {
			return candidate
		}
		candidate = fmt.Sprintf("%s-%d", preferred, seq)
		seq++
	}
}

// --- Store interface ---

type flowershowStore interface {
	// Organizations
	createOrganization(Organization) (*Organization, error)
	organizationByID(id string) (*Organization, bool)
	allOrganizations() []*Organization

	// Shows
	createShow(ShowInput) (*Show, error)
	updateShow(id string, input ShowInput) (*Show, error)
	showByID(id string) (*Show, bool)
	showBySlug(slug string) (*Show, bool)
	allShows() []*Show
	assignJudgeToShow(showID, personID string) (*ShowJudgeAssignment, error)
	judgesByShow(showID string) []*ShowJudgeAssignment

	// Persons
	createPerson(PersonInput) (*Person, error)
	updatePerson(id string, input PersonInput) (*Person, error)
	personByID(id string) (*Person, bool)
	personByEmail(email string) (*Person, bool)
	allPersons() []*Person
	linkPersonOrganization(PersonOrganization) (*PersonOrganization, error)
	personOrganizationsByPerson(personID string) []*PersonOrganization
	lookupPersonsForShow(showID, query string) []*PersonOrganization

	// Organization invites
	createOrganizationInvite(OrganizationInviteInput) (*OrganizationInvite, error)
	organizationInvitesByOrganization(organizationID string) []*OrganizationInvite
	claimOrganizationInvites(email, subjectID, cognitoSub string, assignRole func(UserRoleInput) error) ([]*OrganizationInvite, error)

	// Schedule
	createSchedule(ShowSchedule) (*ShowSchedule, error)
	updateSchedule(showID string, input ShowSchedule) (*ShowSchedule, error)
	scheduleByShowID(showID string) (*ShowSchedule, bool)

	// Divisions
	createDivision(DivisionInput) (*Division, error)
	divisionsBySchedule(scheduleID string) []*Division
	divisionByID(id string) (*Division, bool)

	// Sections
	createSection(SectionInput) (*Section, error)
	sectionsByDivision(divisionID string) []*Section
	sectionByID(id string) (*Section, bool)

	// Classes
	createClass(ShowClassInput) (*ShowClass, error)
	updateClass(id string, input ShowClassInput) (*ShowClass, error)
	reorderClass(id string, sortOrder int) (*ShowClass, error)
	classesBySection(sectionID string) []*ShowClass
	classByID(id string) (*ShowClass, bool)
	classesByShowID(showID string) []*ShowClass

	// Entries
	createEntry(EntryInput) (*Entry, error)
	updateEntry(id string, input EntryInput) (*Entry, error)
	moveEntry(entryID, classID, reason string) (*Entry, error)
	reassignEntry(entryID, personID string) (*Entry, error)
	deleteEntry(entryID string) error
	setEntrySuppressed(entryID string, suppressed bool) error
	setPlacement(entryID string, placement int, points float64) error
	entryByID(id string) (*Entry, bool)
	entriesByShow(showID string) []*Entry
	entriesByClass(classID string) []*Entry
	entriesByPerson(personID string) []*Entry

	// Show credits
	createShowCredit(ShowCreditInput) (*ShowCredit, error)
	showCreditByID(id string) (*ShowCredit, bool)
	deleteShowCredit(id string) error
	showCreditsByShow(showID string) []*ShowCredit

	// Media
	attachMedia(Media) (*Media, error)
	mediaByEntry(entryID string) []*Media
	mediaByID(id string) (*Media, bool)
	deleteMedia(id string) error

	// Taxonomy
	createTaxon(TaxonInput) (*Taxon, error)
	taxonByID(id string) (*Taxon, bool)
	allTaxons() []*Taxon
	taxonsByType(taxonType string) []*Taxon

	// Awards
	createAward(AwardInput) (*AwardDefinition, error)
	awardByID(id string) (*AwardDefinition, bool)
	awardsByOrganization(orgID string) []*AwardDefinition
	computeAward(awardID string) ([]AwardResult, error)

	// Standards
	createStandardDocument(StandardDocument) (*StandardDocument, error)
	allStandardDocuments() []*StandardDocument
	createStandardEdition(StandardEdition) (*StandardEdition, error)
	standardEditionByID(id string) (*StandardEdition, bool)
	allStandardEditions() []*StandardEdition
	editionsByStandard(standardDocID string) []*StandardEdition

	// Sources
	createSourceDocument(SourceDocument) (*SourceDocument, error)
	allSourceDocuments() []*SourceDocument
	createSourceCitation(SourceCitation) (*SourceCitation, error)
	citationsByTarget(targetType, targetID string) []*SourceCitation

	// Rules
	createStandardRule(StandardRule) (*StandardRule, error)
	rulesByEdition(editionID string) []*StandardRule
	createClassRuleOverride(ClassRuleOverride) (*ClassRuleOverride, error)
	overridesByClass(classID string) []*ClassRuleOverride
	effectiveRulesForClass(classID string, editionID string) []effectiveRule

	// Rubrics
	createRubric(JudgingRubric) (*JudgingRubric, error)
	rubricByID(id string) (*JudgingRubric, bool)
	allRubrics() []*JudgingRubric
	createCriterion(JudgingCriterion) (*JudgingCriterion, error)
	criteriaByRubric(rubricID string) []*JudgingCriterion

	// Scorecards
	submitScorecard(EntryScorecard, []EntryCriterionScore) (*EntryScorecard, error)
	scorecardsByEntry(entryID string) []*EntryScorecard
	criterionScoresByScorecard(scorecardID string) []*EntryCriterionScore
	computePlacementsFromScores(classID string) error

	// Leaderboard
	leaderboard(orgID, season string) []LeaderboardEntry

	// Agent tokens
	issueAgentToken(AgentTokenIssueInput) (*IssuedAgentToken, error)
	listAgentTokensBySubject(cognitoSub string) []*AgentToken
	revokeAgentToken(tokenID, ownerCognitoSub string) (*AgentToken, error)
	authenticateAgentToken(raw string) (*AgentToken, bool)

	// Ledger
	ledgerByObjectID(objectID string) ([]FlowershowClaim, error)

	Close()
}

type effectiveRule struct {
	Rule     *StandardRule      `json:"rule,omitempty"`
	Override *ClassRuleOverride `json:"override,omitempty"`
	Source   string             `json:"source"` // "standard", "override", "local_only"
}

// ============================================================================
// In-memory store (development & tests)
// ============================================================================

type memoryStore struct {
	mu             sync.RWMutex
	organizations  map[string]*Organization
	shows          map[string]*Show
	persons        map[string]*Person
	personOrgs     map[string]*PersonOrganization
	orgInvites     map[string]*OrganizationInvite
	showJudges     map[string]*ShowJudgeAssignment
	schedules      map[string]*ShowSchedule
	divisions      map[string]*Division
	sections       map[string]*Section
	classes        map[string]*ShowClass
	entries        map[string]*Entry
	showCredits    map[string]*ShowCredit
	media          map[string]*Media
	taxons         map[string]*Taxon
	awards         map[string]*AwardDefinition
	stdDocs        map[string]*StandardDocument
	stdEditions    map[string]*StandardEdition
	srcDocs        map[string]*SourceDocument
	srcCitations   map[string]*SourceCitation
	stdRules       map[string]*StandardRule
	classOverrides map[string]*ClassRuleOverride
	rubrics        map[string]*JudgingRubric
	criteria       map[string]*JudgingCriterion
	scorecards     map[string]*EntryScorecard
	critScores     map[string]*EntryCriterionScore
	agentTokens    map[string]*AgentToken
	agentTokenHash map[string]string
	objects        map[string]*FlowershowObject
	claims         []FlowershowClaim
	claimSeq       int64
}

func newMemoryStore() *memoryStore {
	s := &memoryStore{
		organizations:  make(map[string]*Organization),
		shows:          make(map[string]*Show),
		persons:        make(map[string]*Person),
		personOrgs:     make(map[string]*PersonOrganization),
		orgInvites:     make(map[string]*OrganizationInvite),
		showJudges:     make(map[string]*ShowJudgeAssignment),
		schedules:      make(map[string]*ShowSchedule),
		divisions:      make(map[string]*Division),
		sections:       make(map[string]*Section),
		classes:        make(map[string]*ShowClass),
		entries:        make(map[string]*Entry),
		showCredits:    make(map[string]*ShowCredit),
		media:          make(map[string]*Media),
		taxons:         make(map[string]*Taxon),
		awards:         make(map[string]*AwardDefinition),
		stdDocs:        make(map[string]*StandardDocument),
		stdEditions:    make(map[string]*StandardEdition),
		srcDocs:        make(map[string]*SourceDocument),
		srcCitations:   make(map[string]*SourceCitation),
		stdRules:       make(map[string]*StandardRule),
		classOverrides: make(map[string]*ClassRuleOverride),
		rubrics:        make(map[string]*JudgingRubric),
		criteria:       make(map[string]*JudgingCriterion),
		scorecards:     make(map[string]*EntryScorecard),
		critScores:     make(map[string]*EntryCriterionScore),
		agentTokens:    make(map[string]*AgentToken),
		agentTokenHash: make(map[string]string),
		objects:        make(map[string]*FlowershowObject),
	}
	s.seedDemoData()
	return s
}

func (s *memoryStore) Close() {}

func (s *memoryStore) appendClaim(objectID, objectType, claimType string, payload any) {
	s.claimSeq++
	if _, ok := s.objects[objectID]; !ok {
		s.objects[objectID] = &FlowershowObject{
			ID:         objectID,
			ObjectType: objectType,
			CreatedAt:  time.Now().UTC(),
			CreatedBy:  "system",
		}
	}
	s.claims = append(s.claims, FlowershowClaim{
		ID:         newID("claim"),
		ObjectID:   objectID,
		ClaimSeq:   s.claimSeq,
		ClaimType:  claimType,
		AcceptedAt: time.Now().UTC(),
		AcceptedBy: "system",
		Payload:    payload,
	})
}

// --- Organizations ---

func (s *memoryStore) createOrganization(o Organization) (*Organization, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if o.ID == "" {
		o.ID = newID("org")
	}
	s.organizations[o.ID] = &o
	s.appendClaim(o.ID, "organization", "organization.created", o)
	return &o, nil
}

func (s *memoryStore) organizationByID(id string) (*Organization, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	o, ok := s.organizations[id]
	return o, ok
}

func (s *memoryStore) allOrganizations() []*Organization {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []*Organization
	for _, o := range s.organizations {
		out = append(out, o)
	}
	return out
}

// --- Shows ---

func (s *memoryStore) createShow(input ShowInput) (*Show, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	org, _ := s.organizations[strings.TrimSpace(input.OrganizationID)]
	show := &Show{
		ID:             newID("show"),
		Slug:           uniqueShowSlug(s.shows, showSlugForInput(org, input), ""),
		OrganizationID: input.OrganizationID,
		Name:           input.Name,
		Location:       input.Location,
		Date:           input.Date,
		Season:         input.Season,
		Status:         "draft",
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	s.shows[show.ID] = show
	s.appendClaim(show.ID, "show", "show.created", show)
	return show, nil
}

func (s *memoryStore) updateShow(id string, input ShowInput) (*Show, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	show, ok := s.shows[id]
	if !ok {
		return nil, errors.New("show not found")
	}
	show.Name = input.Name
	show.Location = input.Location
	show.Date = input.Date
	show.Season = input.Season
	if input.OrganizationID != "" {
		show.OrganizationID = input.OrganizationID
	}
	org, _ := s.organizations[strings.TrimSpace(show.OrganizationID)]
	show.Slug = uniqueShowSlug(s.shows, showSlugForInput(org, ShowInput{
		OrganizationID: show.OrganizationID,
		Name:           show.Name,
		Location:       show.Location,
		Date:           show.Date,
		Season:         show.Season,
	}), show.ID)
	show.UpdatedAt = time.Now().UTC()
	s.appendClaim(show.ID, "show", "show.updated", show)
	return show, nil
}

func (s *memoryStore) showByID(id string) (*Show, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	show, ok := s.shows[id]
	return show, ok
}

func (s *memoryStore) showBySlug(slug string) (*Show, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, show := range s.shows {
		if show.Slug == slug {
			return show, true
		}
	}
	return nil, false
}

func (s *memoryStore) allShows() []*Show {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []*Show
	for _, show := range s.shows {
		out = append(out, show)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Date > out[j].Date })
	return out
}

func (s *memoryStore) assignJudgeToShow(showID, personID string) (*ShowJudgeAssignment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.shows[showID]; !ok {
		return nil, errors.New("show not found")
	}
	if _, ok := s.persons[personID]; !ok {
		return nil, errors.New("person not found")
	}
	for _, assignment := range s.showJudges {
		if assignment.ShowID == showID && assignment.PersonID == personID {
			return assignment, nil
		}
	}
	assignment := &ShowJudgeAssignment{
		ID:         newID("judge"),
		ShowID:     showID,
		PersonID:   personID,
		AssignedAt: time.Now().UTC(),
	}
	s.showJudges[assignment.ID] = assignment
	s.appendClaim(assignment.ID, "show_judge_assignment", "show_judge.assigned", assignment)
	return assignment, nil
}

func (s *memoryStore) judgesByShow(showID string) []*ShowJudgeAssignment {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []*ShowJudgeAssignment
	for _, assignment := range s.showJudges {
		if assignment.ShowID == showID {
			out = append(out, assignment)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].AssignedAt.Before(out[j].AssignedAt) })
	return out
}

// --- Persons ---

func (s *memoryStore) createPerson(input PersonInput) (*Person, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	initials := ""
	if len([]rune(strings.TrimSpace(input.FirstName))) > 0 {
		initials += string([]rune(strings.TrimSpace(input.FirstName))[:1])
	}
	if len([]rune(strings.TrimSpace(input.LastName))) > 0 {
		initials += string([]rune(strings.TrimSpace(input.LastName))[:1])
	}
	publicDisplayMode := strings.TrimSpace(input.PublicDisplayMode)
	if publicDisplayMode == "" {
		publicDisplayMode = "initials"
	}
	p := &Person{
		ID:                newID("person"),
		FirstName:         input.FirstName,
		LastName:          input.LastName,
		Initials:          initials,
		Email:             input.Email,
		PublicDisplayMode: publicDisplayMode,
	}
	s.persons[p.ID] = p
	s.appendClaim(p.ID, "person", "person.created", p)
	if strings.TrimSpace(input.OrganizationID) != "" {
		role := strings.TrimSpace(input.OrganizationRole)
		if role == "" {
			role = "member"
		}
		link := &PersonOrganization{
			PersonID:       p.ID,
			OrganizationID: strings.TrimSpace(input.OrganizationID),
			Role:           role,
		}
		s.personOrgs[p.ID+"|"+link.OrganizationID+"|"+link.Role] = link
		s.appendClaim(p.ID, "person", "person.organization_linked", link)
	}
	return p, nil
}

func (s *memoryStore) updatePerson(id string, input PersonInput) (*Person, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.persons[id]
	if !ok {
		return nil, errors.New("person not found")
	}
	p.FirstName = input.FirstName
	p.LastName = input.LastName
	p.Email = input.Email
	if strings.TrimSpace(input.PublicDisplayMode) != "" {
		p.PublicDisplayMode = strings.TrimSpace(input.PublicDisplayMode)
	}
	if len(input.FirstName) > 0 && len(input.LastName) > 0 {
		p.Initials = string([]rune(input.FirstName)[:1]) + string([]rune(input.LastName)[:1])
	}
	s.appendClaim(p.ID, "person", "person.updated", p)
	return p, nil
}

func (s *memoryStore) personByID(id string) (*Person, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.persons[id]
	return p, ok
}

func (s *memoryStore) personByEmail(email string) (*Person, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	email = normalizeAuthIdentifier(email)
	if email == "" {
		return nil, false
	}
	for _, person := range s.persons {
		if normalizeAuthIdentifier(person.Email) == email {
			return person, true
		}
	}
	return nil, false
}

func (s *memoryStore) allPersons() []*Person {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []*Person
	for _, p := range s.persons {
		out = append(out, p)
	}
	return out
}

func (s *memoryStore) linkPersonOrganization(link PersonOrganization) (*PersonOrganization, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.persons[link.PersonID]; !ok {
		return nil, errors.New("person not found")
	}
	if _, ok := s.organizations[link.OrganizationID]; !ok {
		return nil, errors.New("organization not found")
	}
	if strings.TrimSpace(link.Role) == "" {
		link.Role = "member"
	}
	copy := link
	s.personOrgs[copy.PersonID+"|"+copy.OrganizationID+"|"+copy.Role] = &copy
	s.appendClaim(copy.PersonID, "person", "person.organization_linked", copy)
	return &copy, nil
}

func (s *memoryStore) personOrganizationsByPerson(personID string) []*PersonOrganization {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []*PersonOrganization
	for _, item := range s.personOrgs {
		if item.PersonID == personID {
			copy := *item
			out = append(out, &copy)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].OrganizationID == out[j].OrganizationID {
			return out[i].Role < out[j].Role
		}
		return out[i].OrganizationID < out[j].OrganizationID
	})
	return out
}

func (s *memoryStore) lookupPersonsForShow(showID, query string) []*PersonOrganization {
	s.mu.RLock()
	defer s.mu.RUnlock()
	show, ok := s.shows[showID]
	if !ok {
		return nil
	}
	query = strings.ToLower(strings.TrimSpace(query))
	var out []*PersonOrganization
	for _, link := range s.personOrgs {
		if link.OrganizationID != show.OrganizationID {
			continue
		}
		person, ok := s.persons[link.PersonID]
		if !ok {
			continue
		}
		haystack := strings.ToLower(strings.TrimSpace(person.FirstName + " " + person.LastName + " " + person.Email + " " + person.Initials + " " + link.Role))
		if query != "" && !strings.Contains(haystack, query) {
			continue
		}
		copy := *link
		out = append(out, &copy)
	}
	sort.Slice(out, func(i, j int) bool {
		pi := s.persons[out[i].PersonID]
		pj := s.persons[out[j].PersonID]
		if pi == nil || pj == nil {
			return out[i].PersonID < out[j].PersonID
		}
		return pi.LastName+pi.FirstName < pj.LastName+pj.FirstName
	})
	return out
}

// --- Schedule ---

func (s *memoryStore) createSchedule(sched ShowSchedule) (*ShowSchedule, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sched.ID == "" {
		sched.ID = newID("sched")
	}
	s.schedules[sched.ID] = &sched
	s.appendClaim(sched.ID, "schedule", "schedule.created", sched)
	return &sched, nil
}

func (s *memoryStore) updateSchedule(showID string, input ShowSchedule) (*ShowSchedule, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sched, ok := s.scheduleByShowIDLocked(showID)
	if !ok {
		return nil, errors.New("schedule not found")
	}
	sched.SourceDocumentID = input.SourceDocumentID
	sched.EffectiveStandardEditionID = input.EffectiveStandardEditionID
	sched.Notes = input.Notes
	s.appendClaim(sched.ID, "schedule", "schedule.updated", sched)
	return sched, nil
}

func (s *memoryStore) scheduleByShowID(showID string) (*ShowSchedule, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, sched := range s.schedules {
		if sched.ShowID == showID {
			return sched, true
		}
	}
	return nil, false
}

// --- Divisions ---

func (s *memoryStore) createDivision(input DivisionInput) (*Division, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d := &Division{
		ID:             newID("div"),
		ShowScheduleID: input.ShowScheduleID,
		Code:           input.Code,
		Title:          input.Title,
		Domain:         input.Domain,
		SortOrder:      input.SortOrder,
	}
	s.divisions[d.ID] = d
	s.appendClaim(d.ID, "division", "division.created", d)
	return d, nil
}

func (s *memoryStore) divisionsBySchedule(scheduleID string) []*Division {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []*Division
	for _, d := range s.divisions {
		if d.ShowScheduleID == scheduleID {
			out = append(out, d)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].SortOrder < out[j].SortOrder })
	return out
}

func (s *memoryStore) divisionByID(id string) (*Division, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	d, ok := s.divisions[id]
	return d, ok
}

// --- Sections ---

func (s *memoryStore) createSection(input SectionInput) (*Section, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sec := &Section{
		ID:         newID("sec"),
		DivisionID: input.DivisionID,
		Code:       input.Code,
		Title:      input.Title,
		SortOrder:  input.SortOrder,
	}
	s.sections[sec.ID] = sec
	s.appendClaim(sec.ID, "section", "section.created", sec)
	return sec, nil
}

func (s *memoryStore) sectionsByDivision(divisionID string) []*Section {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []*Section
	for _, sec := range s.sections {
		if sec.DivisionID == divisionID {
			out = append(out, sec)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].SortOrder < out[j].SortOrder })
	return out
}

func (s *memoryStore) sectionByID(id string) (*Section, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sec, ok := s.sections[id]
	return sec, ok
}

// --- Classes ---

func (s *memoryStore) createClass(input ShowClassInput) (*ShowClass, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c := &ShowClass{
		ID:                newID("class"),
		SectionID:         input.SectionID,
		ClassNumber:       input.ClassNumber,
		SortOrder:         input.SortOrder,
		Title:             input.Title,
		Domain:            input.Domain,
		Description:       input.Description,
		SpecimenCount:     input.SpecimenCount,
		Unit:              input.Unit,
		MeasurementRule:   input.MeasurementRule,
		NamingRequirement: input.NamingRequirement,
		ContainerRule:     input.ContainerRule,
		EligibilityRule:   input.EligibilityRule,
		ScheduleNotes:     input.ScheduleNotes,
		TaxonRefs:         input.TaxonRefs,
	}
	s.classes[c.ID] = c
	s.appendClaim(c.ID, "class", "class.created", c)
	return c, nil
}

func (s *memoryStore) updateClass(id string, input ShowClassInput) (*ShowClass, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.classes[id]
	if !ok {
		return nil, errors.New("class not found")
	}
	c.ClassNumber = input.ClassNumber
	c.SortOrder = input.SortOrder
	c.Title = input.Title
	c.Domain = input.Domain
	c.Description = input.Description
	c.SpecimenCount = input.SpecimenCount
	c.Unit = input.Unit
	c.MeasurementRule = input.MeasurementRule
	c.NamingRequirement = input.NamingRequirement
	c.ContainerRule = input.ContainerRule
	c.EligibilityRule = input.EligibilityRule
	c.ScheduleNotes = input.ScheduleNotes
	c.TaxonRefs = input.TaxonRefs
	s.appendClaim(c.ID, "class", "class.updated", c)
	return c, nil
}

func (s *memoryStore) reorderClass(id string, sortOrder int) (*ShowClass, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.classes[id]
	if !ok {
		return nil, errors.New("class not found")
	}
	c.SortOrder = sortOrder
	s.appendClaim(c.ID, "class", "class.reordered", map[string]any{
		"sort_order": sortOrder,
	})
	return c, nil
}

func (s *memoryStore) classesBySection(sectionID string) []*ShowClass {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []*ShowClass
	for _, c := range s.classes {
		if c.SectionID == sectionID {
			out = append(out, c)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].SortOrder == out[j].SortOrder {
			return out[i].ClassNumber < out[j].ClassNumber
		}
		return out[i].SortOrder < out[j].SortOrder
	})
	return out
}

func (s *memoryStore) classByID(id string) (*ShowClass, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.classes[id]
	return c, ok
}

func (s *memoryStore) classesByShowID(showID string) []*ShowClass {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sched, ok := s.scheduleByShowIDLocked(showID)
	if !ok {
		return nil
	}
	var out []*ShowClass
	for _, d := range s.divisions {
		if d.ShowScheduleID != sched.ID {
			continue
		}
		for _, sec := range s.sections {
			if sec.DivisionID != d.ID {
				continue
			}
			for _, c := range s.classes {
				if c.SectionID == sec.ID {
					out = append(out, c)
				}
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].SortOrder == out[j].SortOrder {
			return out[i].ClassNumber < out[j].ClassNumber
		}
		return out[i].SortOrder < out[j].SortOrder
	})
	return out
}

func (s *memoryStore) scheduleByShowIDLocked(showID string) (*ShowSchedule, bool) {
	for _, sched := range s.schedules {
		if sched.ShowID == showID {
			return sched, true
		}
	}
	return nil, false
}

// --- Entries ---

func (s *memoryStore) createEntry(input EntryInput) (*Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e := &Entry{
		ID:        newID("entry"),
		ShowID:    input.ShowID,
		ClassID:   input.ClassID,
		PersonID:  input.PersonID,
		Name:      input.Name,
		Notes:     input.Notes,
		TaxonRefs: input.TaxonRefs,
		CreatedAt: time.Now().UTC(),
	}
	s.entries[e.ID] = e
	s.appendClaim(e.ID, "entry", "entry.created", e)
	return e, nil
}

func (s *memoryStore) updateEntry(id string, input EntryInput) (*Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.entries[id]
	if !ok {
		return nil, errors.New("entry not found")
	}
	e.Name = input.Name
	e.Notes = input.Notes
	e.ClassID = input.ClassID
	e.PersonID = input.PersonID
	e.TaxonRefs = input.TaxonRefs
	s.appendClaim(e.ID, "entry", "entry.updated", e)
	return e, nil
}

func (s *memoryStore) moveEntry(entryID, classID, reason string) (*Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.entries[entryID]
	if !ok {
		return nil, errors.New("entry not found")
	}
	if _, ok := s.classes[classID]; !ok {
		return nil, errors.New("class not found")
	}
	previousClassID := e.ClassID
	e.ClassID = classID
	s.appendClaim(e.ID, "entry", "entry.moved", map[string]any{
		"previous_class_id": previousClassID,
		"class_id":          classID,
		"reason":            strings.TrimSpace(reason),
	})
	return e, nil
}

func (s *memoryStore) reassignEntry(entryID, personID string) (*Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.entries[entryID]
	if !ok {
		return nil, errors.New("entry not found")
	}
	if _, ok := s.persons[personID]; !ok {
		return nil, errors.New("person not found")
	}
	previousPersonID := e.PersonID
	e.PersonID = personID
	s.appendClaim(e.ID, "entry", "entry.reassigned", map[string]any{
		"previous_person_id": previousPersonID,
		"person_id":          personID,
	})
	return e, nil
}

func (s *memoryStore) deleteEntry(entryID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.entries[entryID]
	if !ok {
		return errors.New("entry not found")
	}
	for _, scorecard := range s.scorecards {
		if scorecard.EntryID == entryID {
			return errors.New("cannot delete entry with scorecards")
		}
	}
	for id, media := range s.media {
		if media.EntryID == entryID {
			delete(s.media, id)
		}
	}
	delete(s.entries, entryID)
	s.appendClaim(entryID, "entry", "entry.deleted", map[string]any{
		"show_id":  e.ShowID,
		"class_id": e.ClassID,
	})
	return nil
}

func (s *memoryStore) setEntrySuppressed(entryID string, suppressed bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.entries[entryID]
	if !ok {
		return errors.New("entry not found")
	}
	e.Suppressed = suppressed
	s.appendClaim(e.ID, "entry", "entry.visibility_set", map[string]any{
		"suppressed": suppressed,
	})
	return nil
}

func (s *memoryStore) setPlacement(entryID string, placement int, points float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.entries[entryID]
	if !ok {
		return errors.New("entry not found")
	}
	e.Placement = placement
	e.Points = points
	s.appendClaim(e.ID, "entry", "entry.placement_set", map[string]any{
		"placement": placement, "points": points,
	})
	return nil
}

func (s *memoryStore) entryByID(id string) (*Entry, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	e, ok := s.entries[id]
	return e, ok
}

func (s *memoryStore) entriesByShow(showID string) []*Entry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []*Entry
	for _, e := range s.entries {
		if e.ShowID == showID {
			out = append(out, e)
		}
	}
	return out
}

func (s *memoryStore) entriesByClass(classID string) []*Entry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []*Entry
	for _, e := range s.entries {
		if e.ClassID == classID {
			out = append(out, e)
		}
	}
	return out
}

func (s *memoryStore) entriesByPerson(personID string) []*Entry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []*Entry
	for _, e := range s.entries {
		if e.PersonID == personID {
			out = append(out, e)
		}
	}
	return out
}

func (s *memoryStore) createShowCredit(input ShowCreditInput) (*ShowCredit, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if strings.TrimSpace(input.ShowID) == "" {
		return nil, errors.New("show id required")
	}
	if strings.TrimSpace(input.CreditLabel) == "" {
		return nil, errors.New("credit label required")
	}
	if strings.TrimSpace(input.PersonID) == "" && strings.TrimSpace(input.DisplayName) == "" {
		return nil, errors.New("person or display name required")
	}
	if input.PersonID != "" {
		if _, ok := s.persons[input.PersonID]; !ok {
			return nil, errors.New("person not found")
		}
	}
	credit := &ShowCredit{
		ID:          newID("credit"),
		ShowID:      input.ShowID,
		PersonID:    strings.TrimSpace(input.PersonID),
		DisplayName: strings.TrimSpace(input.DisplayName),
		CreditLabel: strings.TrimSpace(input.CreditLabel),
		Notes:       strings.TrimSpace(input.Notes),
		SortOrder:   input.SortOrder,
		CreatedAt:   time.Now().UTC(),
	}
	s.showCredits[credit.ID] = credit
	s.appendClaim(credit.ID, "show_credit", "show_credit.created", credit)
	return credit, nil
}

func (s *memoryStore) deleteShowCredit(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.showCredits[id]; !ok {
		return errors.New("show credit not found")
	}
	delete(s.showCredits, id)
	s.appendClaim(id, "show_credit", "show_credit.deleted", map[string]any{"id": id})
	return nil
}

func (s *memoryStore) showCreditByID(id string) (*ShowCredit, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	credit, ok := s.showCredits[id]
	return credit, ok
}

func (s *memoryStore) showCreditsByShow(showID string) []*ShowCredit {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []*ShowCredit
	for _, credit := range s.showCredits {
		if credit.ShowID == showID {
			out = append(out, credit)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].SortOrder == out[j].SortOrder {
			if out[i].CreditLabel == out[j].CreditLabel {
				return out[i].CreatedAt.Before(out[j].CreatedAt)
			}
			return out[i].CreditLabel < out[j].CreditLabel
		}
		return out[i].SortOrder < out[j].SortOrder
	})
	return out
}

// --- Media ---

func (s *memoryStore) attachMedia(m Media) (*Media, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if m.ID == "" {
		m.ID = newID("media")
	}
	if m.CreatedAt.IsZero() {
		m.CreatedAt = time.Now().UTC()
	}
	cp := m
	s.media[cp.ID] = &cp
	s.appendClaim(cp.ID, "media", "media.attached", cp)
	return &cp, nil
}

func (s *memoryStore) mediaByEntry(entryID string) []*Media {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []*Media
	for _, m := range s.media {
		if m.EntryID == entryID {
			out = append(out, m)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out
}

func (s *memoryStore) mediaByID(id string) (*Media, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	m, ok := s.media[id]
	return m, ok
}

func (s *memoryStore) deleteMedia(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, ok := s.media[id]
	if !ok {
		return errors.New("media not found")
	}
	delete(s.media, id)
	s.appendClaim(id, "media", "media.deleted", m)
	return nil
}

// --- Taxonomy ---

func (s *memoryStore) createTaxon(input TaxonInput) (*Taxon, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t := &Taxon{
		ID:             newID("taxon"),
		TaxonType:      input.TaxonType,
		Name:           input.Name,
		ScientificName: input.ScientificName,
		Description:    input.Description,
		ParentID:       input.ParentID,
	}
	s.taxons[t.ID] = t
	s.appendClaim(t.ID, "taxon", "taxon.created", t)
	return t, nil
}

func (s *memoryStore) taxonByID(id string) (*Taxon, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.taxons[id]
	return t, ok
}

func (s *memoryStore) allTaxons() []*Taxon {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []*Taxon
	for _, t := range s.taxons {
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func (s *memoryStore) taxonsByType(taxonType string) []*Taxon {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []*Taxon
	for _, t := range s.taxons {
		if t.TaxonType == taxonType {
			out = append(out, t)
		}
	}
	return out
}

// --- Awards ---

func (s *memoryStore) createAward(input AwardInput) (*AwardDefinition, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a := &AwardDefinition{
		ID:             newID("award"),
		OrganizationID: input.OrganizationID,
		Name:           input.Name,
		Description:    input.Description,
		Season:         input.Season,
		TaxonFilters:   input.TaxonFilters,
		ScoringRule:    input.ScoringRule,
		MinEntries:     input.MinEntries,
	}
	s.awards[a.ID] = a
	s.appendClaim(a.ID, "award", "award.created", a)
	return a, nil
}

func (s *memoryStore) awardByID(id string) (*AwardDefinition, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	a, ok := s.awards[id]
	return a, ok
}

func (s *memoryStore) awardsByOrganization(orgID string) []*AwardDefinition {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []*AwardDefinition
	for _, a := range s.awards {
		if a.OrganizationID == orgID {
			out = append(out, a)
		}
	}
	return out
}

func (s *memoryStore) computeAward(awardID string) ([]AwardResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	award, ok := s.awards[awardID]
	if !ok {
		return nil, errors.New("award not found")
	}

	// Collect entries matching taxon filters
	matchingEntries := s.filterEntriesByTaxons(award.TaxonFilters, award.OrganizationID, award.Season)

	// Aggregate per person
	personScores := make(map[string]float64)
	personCounts := make(map[string]int)
	for _, e := range matchingEntries {
		personCounts[e.PersonID]++
		switch award.ScoringRule {
		case "sum":
			personScores[e.PersonID] += e.Points
		case "max":
			if e.Points > personScores[e.PersonID] {
				personScores[e.PersonID] = e.Points
			}
		case "count":
			if e.Placement > 0 && e.Placement <= 3 {
				personScores[e.PersonID]++
			}
		}
	}

	var results []AwardResult
	for pid, score := range personScores {
		if award.MinEntries > 0 && personCounts[pid] < award.MinEntries {
			continue
		}
		results = append(results, AwardResult{
			AwardID:  awardID,
			PersonID: pid,
			Score:    score,
		})
	}
	sort.Slice(results, func(i, j int) bool { return results[i].Score > results[j].Score })
	for i := range results {
		results[i].Rank = i + 1
	}
	return results, nil
}

func (s *memoryStore) filterEntriesByTaxons(taxonFilters []string, orgID, season string) []*Entry {
	if len(taxonFilters) == 0 {
		// No filter — return all entries for org+season
		var out []*Entry
		for _, e := range s.entries {
			if e.Suppressed {
				continue
			}
			show, ok := s.shows[e.ShowID]
			if ok && show.OrganizationID == orgID && show.Season == season {
				out = append(out, e)
			}
		}
		return out
	}
	filterSet := make(map[string]bool)
	for _, t := range taxonFilters {
		filterSet[t] = true
	}
	var out []*Entry
	for _, e := range s.entries {
		if e.Suppressed {
			continue
		}
		show, ok := s.shows[e.ShowID]
		if !ok || show.OrganizationID != orgID || show.Season != season {
			continue
		}
		for _, ref := range e.TaxonRefs {
			if filterSet[ref] {
				out = append(out, e)
				break
			}
		}
	}
	return out
}

// --- Standards ---

func (s *memoryStore) createStandardDocument(doc StandardDocument) (*StandardDocument, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if doc.ID == "" {
		doc.ID = newID("std")
	}
	s.stdDocs[doc.ID] = &doc
	s.appendClaim(doc.ID, "standard_document", "standard_document.created", doc)
	return &doc, nil
}

func (s *memoryStore) allStandardDocuments() []*StandardDocument {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []*StandardDocument
	for _, d := range s.stdDocs {
		out = append(out, d)
	}
	return out
}

func (s *memoryStore) createStandardEdition(ed StandardEdition) (*StandardEdition, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ed.ID == "" {
		ed.ID = newID("edition")
	}
	s.stdEditions[ed.ID] = &ed
	s.appendClaim(ed.ID, "standard_edition", "standard_edition.created", ed)
	return &ed, nil
}

func (s *memoryStore) standardEditionByID(id string) (*StandardEdition, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ed, ok := s.stdEditions[id]
	return ed, ok
}

func (s *memoryStore) allStandardEditions() []*StandardEdition {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []*StandardEdition
	for _, ed := range s.stdEditions {
		out = append(out, ed)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].PublicationYear == out[j].PublicationYear {
			return out[i].EditionLabel < out[j].EditionLabel
		}
		return out[i].PublicationYear > out[j].PublicationYear
	})
	return out
}

func (s *memoryStore) editionsByStandard(standardDocID string) []*StandardEdition {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []*StandardEdition
	for _, ed := range s.stdEditions {
		if ed.StandardDocumentID == standardDocID {
			out = append(out, ed)
		}
	}
	return out
}

// --- Sources ---

func (s *memoryStore) createSourceDocument(doc SourceDocument) (*SourceDocument, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if doc.ID == "" {
		doc.ID = newID("source")
	}
	s.srcDocs[doc.ID] = &doc
	s.appendClaim(doc.ID, "source_document", "source_document.created", doc)
	return &doc, nil
}

func (s *memoryStore) allSourceDocuments() []*SourceDocument {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []*SourceDocument
	for _, d := range s.srcDocs {
		out = append(out, d)
	}
	return out
}

func (s *memoryStore) createSourceCitation(cite SourceCitation) (*SourceCitation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if cite.ID == "" {
		cite.ID = newID("cite")
	}
	s.srcCitations[cite.ID] = &cite
	s.appendClaim(cite.ID, "source_citation", "source_citation.created", cite)
	return &cite, nil
}

func (s *memoryStore) citationsByTarget(targetType, targetID string) []*SourceCitation {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []*SourceCitation
	for _, c := range s.srcCitations {
		if c.TargetType == targetType && c.TargetID == targetID {
			out = append(out, c)
		}
	}
	return out
}

// --- Rules ---

func (s *memoryStore) createStandardRule(rule StandardRule) (*StandardRule, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if rule.ID == "" {
		rule.ID = newID("rule")
	}
	s.stdRules[rule.ID] = &rule
	s.appendClaim(rule.ID, "standard_rule", "standard_rule.created", rule)
	return &rule, nil
}

func (s *memoryStore) rulesByEdition(editionID string) []*StandardRule {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []*StandardRule
	for _, r := range s.stdRules {
		if r.StandardEditionID == editionID {
			out = append(out, r)
		}
	}
	return out
}

func (s *memoryStore) createClassRuleOverride(override ClassRuleOverride) (*ClassRuleOverride, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if override.ID == "" {
		override.ID = newID("override")
	}
	s.classOverrides[override.ID] = &override
	s.appendClaim(override.ID, "class_rule_override", "class_rule_override.created", override)
	return &override, nil
}

func (s *memoryStore) overridesByClass(classID string) []*ClassRuleOverride {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []*ClassRuleOverride
	for _, o := range s.classOverrides {
		if o.ShowClassID == classID {
			out = append(out, o)
		}
	}
	return out
}

func (s *memoryStore) effectiveRulesForClass(classID string, editionID string) []effectiveRule {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Get standard rules
	ruleMap := make(map[string]*StandardRule)
	for _, r := range s.stdRules {
		if r.StandardEditionID == editionID {
			ruleMap[r.ID] = r
		}
	}

	// Get overrides for class
	overridesByBase := make(map[string]*ClassRuleOverride)
	var localOnlyOverrides []*ClassRuleOverride
	for _, o := range s.classOverrides {
		if o.ShowClassID != classID {
			continue
		}
		if o.BaseStandardRuleID != "" {
			overridesByBase[o.BaseStandardRuleID] = o
		} else {
			localOnlyOverrides = append(localOnlyOverrides, o)
		}
	}

	var results []effectiveRule
	for _, rule := range ruleMap {
		if override, ok := overridesByBase[rule.ID]; ok {
			results = append(results, effectiveRule{
				Rule:     rule,
				Override: override,
				Source:   "override",
			})
		} else {
			results = append(results, effectiveRule{
				Rule:   rule,
				Source: "standard",
			})
		}
	}
	for _, o := range localOnlyOverrides {
		results = append(results, effectiveRule{
			Override: o,
			Source:   "local_only",
		})
	}
	return results
}

// --- Rubrics ---

func (s *memoryStore) createRubric(r JudgingRubric) (*JudgingRubric, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if r.ID == "" {
		r.ID = newID("rubric")
	}
	s.rubrics[r.ID] = &r
	s.appendClaim(r.ID, "rubric", "rubric.created", r)
	return &r, nil
}

func (s *memoryStore) rubricByID(id string) (*JudgingRubric, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.rubrics[id]
	return r, ok
}

func (s *memoryStore) allRubrics() []*JudgingRubric {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []*JudgingRubric
	for _, r := range s.rubrics {
		out = append(out, r)
	}
	return out
}

func (s *memoryStore) createCriterion(c JudgingCriterion) (*JudgingCriterion, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if c.ID == "" {
		c.ID = newID("crit")
	}
	s.criteria[c.ID] = &c
	return &c, nil
}

func (s *memoryStore) criteriaByRubric(rubricID string) []*JudgingCriterion {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []*JudgingCriterion
	for _, c := range s.criteria {
		if c.JudgingRubricID == rubricID {
			out = append(out, c)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].SortOrder < out[j].SortOrder })
	return out
}

// --- Scorecards ---

func (s *memoryStore) submitScorecard(sc EntryScorecard, scores []EntryCriterionScore) (*EntryScorecard, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sc.ID == "" {
		sc.ID = newID("scorecard")
	}
	var total float64
	for i := range scores {
		if scores[i].ID == "" {
			scores[i].ID = newID("crit_score")
		}
		scores[i].ScorecardID = sc.ID
		total += scores[i].Score
		s.critScores[scores[i].ID] = &scores[i]
	}
	sc.TotalScore = total
	s.scorecards[sc.ID] = &sc
	s.appendClaim(sc.ID, "scorecard", "scorecard.submitted", map[string]any{
		"scorecard": sc, "scores": scores,
	})
	return &sc, nil
}

func (s *memoryStore) scorecardsByEntry(entryID string) []*EntryScorecard {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []*EntryScorecard
	for _, sc := range s.scorecards {
		if sc.EntryID == entryID {
			out = append(out, sc)
		}
	}
	return out
}

func (s *memoryStore) criterionScoresByScorecard(scorecardID string) []*EntryCriterionScore {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []*EntryCriterionScore
	for _, cs := range s.critScores {
		if cs.ScorecardID == scorecardID {
			out = append(out, cs)
		}
	}
	return out
}

func (s *memoryStore) computePlacementsFromScores(classID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Find entries in class
	var classEntries []*Entry
	for _, e := range s.entries {
		if e.ClassID == classID {
			classEntries = append(classEntries, e)
		}
	}

	// Average score per entry
	type entryScore struct {
		entry    *Entry
		avgScore float64
	}
	var scored []entryScore
	for _, e := range classEntries {
		var total float64
		var count int
		for _, sc := range s.scorecards {
			if sc.EntryID == e.ID {
				total += sc.TotalScore
				count++
			}
		}
		if count > 0 {
			scored = append(scored, entryScore{entry: e, avgScore: total / float64(count)})
		}
	}

	sort.Slice(scored, func(i, j int) bool { return scored[i].avgScore > scored[j].avgScore })

	pointsMap := map[int]float64{1: 6, 2: 4, 3: 2}
	for i, es := range scored {
		placement := i + 1
		if placement <= 3 {
			es.entry.Placement = placement
			es.entry.Points = pointsMap[placement]
		}
	}
	return nil
}

// --- Leaderboard ---

func (s *memoryStore) leaderboard(orgID, season string) []LeaderboardEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	type personStats struct {
		points     float64
		entryCount int
		firstCount int
	}
	stats := make(map[string]*personStats)

	for _, e := range s.entries {
		if e.Suppressed {
			continue
		}
		show, ok := s.shows[e.ShowID]
		if !ok || show.OrganizationID != orgID || show.Season != season {
			continue
		}
		ps, ok := stats[e.PersonID]
		if !ok {
			ps = &personStats{}
			stats[e.PersonID] = ps
		}
		ps.entryCount++
		ps.points += e.Points
		if e.Placement == 1 {
			ps.firstCount++
		}
	}

	var out []LeaderboardEntry
	for pid, ps := range stats {
		person, ok := s.persons[pid]
		if !ok {
			continue
		}
		out = append(out, LeaderboardEntry{
			PersonID:    pid,
			PersonName:  person.FirstName + " " + person.LastName,
			Initials:    person.Initials,
			TotalPoints: ps.points,
			EntryCount:  ps.entryCount,
			FirstCount:  ps.firstCount,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].TotalPoints > out[j].TotalPoints })
	for i := range out {
		out[i].Rank = i + 1
	}
	return out
}

// --- Ledger ---

func (s *memoryStore) ledgerByObjectID(objectID string) ([]FlowershowClaim, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []FlowershowClaim
	for _, c := range s.claims {
		if c.ObjectID == objectID {
			out = append(out, c)
		}
	}
	return out, nil
}

// --- Seed Demo Data ---

func (s *memoryStore) seedDemoData() {
	// Organization
	org := &Organization{ID: "org_demo1", Name: "Metro Rose Society", Level: "society"}
	s.organizations[org.ID] = org

	// Persons
	persons := []Person{
		{ID: "person_01", FirstName: "Margaret", LastName: "Chen", Initials: "MC", PublicDisplayMode: "initials"},
		{ID: "person_02", FirstName: "Robert", LastName: "Williams", Initials: "RW", PublicDisplayMode: "initials"},
		{ID: "person_03", FirstName: "Susan", LastName: "Park", Initials: "SP", PublicDisplayMode: "initials"},
	}
	for i := range persons {
		s.persons[persons[i].ID] = &persons[i]
	}
	personOrgs := []PersonOrganization{
		{PersonID: "person_01", OrganizationID: org.ID, Role: "member"},
		{PersonID: "person_02", OrganizationID: org.ID, Role: "member"},
		{PersonID: "person_03", OrganizationID: org.ID, Role: "guest"},
	}
	for i := range personOrgs {
		item := personOrgs[i]
		s.personOrgs[item.PersonID+"|"+item.OrganizationID+"|"+item.Role] = &item
	}

	// Shows
	show1 := &Show{
		ID: "show_spring2025", Slug: "spring-rose-show-2025", OrganizationID: org.ID,
		Name: "Spring Rose Show 2025", Location: "Community Garden Center",
		Date: "2025-04-15", Season: "2025", Status: "published",
		CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}
	show2 := &Show{
		ID: "show_fall2025", Slug: "fall-garden-festival-2025", OrganizationID: org.ID,
		Name: "Fall Garden Festival 2025", Location: "Botanical Gardens Pavilion",
		Date: "2025-09-20", Season: "2025", Status: "draft",
		CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}
	s.shows[show1.ID] = show1
	s.shows[show2.ID] = show2

	// Schedule for spring show
	sched := &ShowSchedule{ID: "sched_spring", ShowID: show1.ID}
	s.schedules[sched.ID] = sched

	// Divisions
	divs := []Division{
		{ID: "div_hort", ShowScheduleID: sched.ID, Code: "I", Title: "Horticulture Specimens", Domain: "horticulture", SortOrder: 1},
		{ID: "div_design", ShowScheduleID: sched.ID, Code: "II", Title: "Floral Design", Domain: "design", SortOrder: 2},
		{ID: "div_special", ShowScheduleID: sched.ID, Code: "III", Title: "Special Exhibits", Domain: "special", SortOrder: 3},
	}
	for i := range divs {
		s.divisions[divs[i].ID] = &divs[i]
	}

	// Sections
	secs := []Section{
		{ID: "sec_hybrid_tea", DivisionID: "div_hort", Code: "A", Title: "Hybrid Tea Roses", SortOrder: 1},
		{ID: "sec_floribunda", DivisionID: "div_hort", Code: "B", Title: "Floribunda Roses", SortOrder: 2},
		{ID: "sec_miniature", DivisionID: "div_hort", Code: "C", Title: "Miniature Roses", SortOrder: 3},
		{ID: "sec_traditional", DivisionID: "div_design", Code: "A", Title: "Traditional Arrangements", SortOrder: 1},
		{ID: "sec_modern", DivisionID: "div_design", Code: "B", Title: "Modern Design", SortOrder: 2},
		{ID: "sec_peoples", DivisionID: "div_special", Code: "A", Title: "People's Choice", SortOrder: 1},
	}
	for i := range secs {
		s.sections[secs[i].ID] = &secs[i]
	}

	// Classes
	classes := []ShowClass{
		{ID: "class_01", SectionID: "sec_hybrid_tea", ClassNumber: "101", Title: "One Hybrid Tea Bloom", Domain: "horticulture", Description: "One bloom, hybrid tea, any variety", SpecimenCount: 1},
		{ID: "class_02", SectionID: "sec_hybrid_tea", ClassNumber: "102", Title: "Three Hybrid Tea Blooms", Domain: "horticulture", Description: "Three blooms, one variety, own root", SpecimenCount: 3},
		{ID: "class_03", SectionID: "sec_floribunda", ClassNumber: "201", Title: "One Floribunda Spray", Domain: "horticulture", Description: "One spray, any floribunda variety", SpecimenCount: 1},
		{ID: "class_04", SectionID: "sec_floribunda", ClassNumber: "202", Title: "Three Floribunda Sprays", Domain: "horticulture", Description: "Three sprays, one or more varieties", SpecimenCount: 3},
		{ID: "class_05", SectionID: "sec_miniature", ClassNumber: "301", Title: "One Miniature Bloom", Domain: "horticulture", Description: "One bloom, any miniature variety", SpecimenCount: 1},
		{ID: "class_06", SectionID: "sec_miniature", ClassNumber: "302", Title: "Three Miniature Blooms", Domain: "horticulture", Description: "Three blooms, miniature varieties", SpecimenCount: 3},
		{ID: "class_07", SectionID: "sec_traditional", ClassNumber: "401", Title: "Traditional Line Arrangement", Domain: "design", Description: "A traditional line arrangement using roses"},
		{ID: "class_08", SectionID: "sec_traditional", ClassNumber: "402", Title: "Mass Arrangement", Domain: "design", Description: "Mass arrangement featuring garden roses"},
		{ID: "class_09", SectionID: "sec_modern", ClassNumber: "501", Title: "Crescent Design", Domain: "design", Description: "Modern crescent design with roses"},
		{ID: "class_10", SectionID: "sec_modern", ClassNumber: "502", Title: "Abstract Design", Domain: "design", Description: "Abstract design incorporating roses"},
		{ID: "class_11", SectionID: "sec_peoples", ClassNumber: "601", Title: "Best Single Rose", Domain: "special", Description: "People's choice — best single rose specimen"},
		{ID: "class_12", SectionID: "sec_peoples", ClassNumber: "602", Title: "Best Arrangement", Domain: "special", Description: "People's choice — best floral arrangement"},
	}
	for i := range classes {
		s.classes[classes[i].ID] = &classes[i]
	}

	// Taxons
	taxons := []Taxon{
		{ID: "taxon_rose", TaxonType: "botanical", Name: "Rose", ScientificName: "Rosa", Description: "Genus Rosa"},
		{ID: "taxon_ht", TaxonType: "botanical", Name: "Hybrid Tea", Description: "Hybrid Tea rose class", ParentID: "taxon_rose"},
		{ID: "taxon_flori", TaxonType: "botanical", Name: "Floribunda", Description: "Floribunda rose class", ParentID: "taxon_rose"},
		{ID: "taxon_mini", TaxonType: "botanical", Name: "Miniature", Description: "Miniature rose class", ParentID: "taxon_rose"},
		{ID: "taxon_crescent", TaxonType: "design_type", Name: "Crescent Design", Description: "A curved, crescent-shaped arrangement"},
	}
	for i := range taxons {
		s.taxons[taxons[i].ID] = &taxons[i]
	}

	// Entries
	entries := []Entry{
		{ID: "entry_01", ShowID: show1.ID, ClassID: "class_01", PersonID: "person_01", Name: "Peace", TaxonRefs: []string{"taxon_ht"}, Placement: 1, Points: 6, CreatedAt: time.Now().UTC()},
		{ID: "entry_02", ShowID: show1.ID, ClassID: "class_01", PersonID: "person_02", Name: "Mr. Lincoln", TaxonRefs: []string{"taxon_ht"}, Placement: 2, Points: 4, CreatedAt: time.Now().UTC()},
		{ID: "entry_03", ShowID: show1.ID, ClassID: "class_02", PersonID: "person_01", Name: "Double Delight Trio", TaxonRefs: []string{"taxon_ht"}, Placement: 1, Points: 6, CreatedAt: time.Now().UTC()},
		{ID: "entry_04", ShowID: show1.ID, ClassID: "class_03", PersonID: "person_03", Name: "Iceberg Spray", TaxonRefs: []string{"taxon_flori"}, Placement: 1, Points: 6, CreatedAt: time.Now().UTC()},
		{ID: "entry_05", ShowID: show1.ID, ClassID: "class_05", PersonID: "person_02", Name: "Baby Love", TaxonRefs: []string{"taxon_mini"}, Placement: 1, Points: 6, CreatedAt: time.Now().UTC()},
		{ID: "entry_06", ShowID: show1.ID, ClassID: "class_07", PersonID: "person_03", Name: "Garden Elegance", TaxonRefs: []string{"taxon_rose"}, Placement: 2, Points: 4, CreatedAt: time.Now().UTC()},
		{ID: "entry_07", ShowID: show1.ID, ClassID: "class_09", PersonID: "person_01", Name: "Moonlight Crescent", TaxonRefs: []string{"taxon_crescent", "taxon_rose"}, Placement: 1, Points: 6, CreatedAt: time.Now().UTC()},
		{ID: "entry_08", ShowID: show1.ID, ClassID: "class_11", PersonID: "person_02", Name: "Crimson Glory", TaxonRefs: []string{"taxon_ht"}, CreatedAt: time.Now().UTC()},
	}
	for i := range entries {
		s.entries[entries[i].ID] = &entries[i]
	}

	// Show judges
	assignments := []ShowJudgeAssignment{
		{ID: "judge_assign_01", ShowID: show1.ID, PersonID: "person_03", AssignedAt: time.Now().UTC()},
		{ID: "judge_assign_02", ShowID: show1.ID, PersonID: "person_02", AssignedAt: time.Now().UTC()},
	}
	for i := range assignments {
		s.showJudges[assignments[i].ID] = &assignments[i]
	}

	// Awards
	awards := []AwardDefinition{
		{ID: "award_hp", OrganizationID: org.ID, Name: "High Points", Season: "2025", ScoringRule: "sum"},
		{ID: "award_bestrose", OrganizationID: org.ID, Name: "Best Rose", Season: "2025", TaxonFilters: []string{"taxon_rose"}, ScoringRule: "max"},
	}
	for i := range awards {
		s.awards[awards[i].ID] = &awards[i]
	}

	// Rubric
	rubric := &JudgingRubric{ID: "rubric_hort", Domain: "horticulture", Title: "Horticulture Specimen Judging"}
	s.rubrics[rubric.ID] = rubric
	criteria := []JudgingCriterion{
		{ID: "crit_form", JudgingRubricID: rubric.ID, Name: "Form", MaxPoints: 25, SortOrder: 1},
		{ID: "crit_color", JudgingRubricID: rubric.ID, Name: "Color", MaxPoints: 25, SortOrder: 2},
		{ID: "crit_substance", JudgingRubricID: rubric.ID, Name: "Substance & Texture", MaxPoints: 15, SortOrder: 3},
		{ID: "crit_stem", JudgingRubricID: rubric.ID, Name: "Stem & Foliage", MaxPoints: 15, SortOrder: 4},
		{ID: "crit_size", JudgingRubricID: rubric.ID, Name: "Size", MaxPoints: 10, SortOrder: 5},
		{ID: "crit_grooming", JudgingRubricID: rubric.ID, Name: "Grooming & Condition", MaxPoints: 10, SortOrder: 6},
	}
	for i := range criteria {
		s.criteria[criteria[i].ID] = &criteria[i]
	}

	// Standards, provenance, and rules
	std := &StandardDocument{
		ID:          "std_ojes",
		Name:        "Official Judging and Exhibiting Standards",
		IssuingOrg:  org.ID,
		DomainScope: "horticulture",
		Description: "National judging and exhibiting standard for rose shows.",
	}
	s.stdDocs[std.ID] = std
	edition := &StandardEdition{
		ID:                 "edition_ojes_2019",
		StandardDocumentID: std.ID,
		EditionLabel:       "2019",
		PublicationYear:    2019,
		Status:             "current",
		SourceURL:          "https://example.com/ojes-2019.pdf",
		SourceKind:         "official_pdf",
	}
	s.stdEditions[edition.ID] = edition
	source := &SourceDocument{
		ID:             "source_spring_schedule",
		OrganizationID: org.ID,
		ShowID:         show1.ID,
		Title:          "Spring Rose Show 2025 Schedule",
		DocumentType:   "schedule",
		SourceURL:      "https://example.com/spring-rose-show-2025-schedule.pdf",
		Checksum:       "demo-schedule-checksum",
	}
	s.srcDocs[source.ID] = source
	sched.EffectiveStandardEditionID = edition.ID
	sched.SourceDocumentID = source.ID
	sched.Notes = "Governed by OJES 2019 and the spring schedule."

	rules := []StandardRule{
		{
			ID:                "rule_ojes_container",
			StandardEditionID: edition.ID,
			Domain:            "horticulture",
			RuleType:          "presentation",
			SubjectLabel:      "Hybrid Tea specimens",
			Body:              "Specimens must be exhibited in a clear container with foliage below the water line removed.",
			PageRef:           "p. 42",
		},
		{
			ID:                "rule_ojes_naming",
			StandardEditionID: edition.ID,
			Domain:            "horticulture",
			RuleType:          "naming",
			SubjectLabel:      "Variety naming",
			Body:              "Exhibitor must identify the cultivar name on the entry card when known.",
			PageRef:           "p. 19",
		},
	}
	for i := range rules {
		s.stdRules[rules[i].ID] = &rules[i]
	}
	override := &ClassRuleOverride{
		ID:                 "override_class_01_tag",
		ShowClassID:        "class_01",
		BaseStandardRuleID: "rule_ojes_container",
		OverrideType:       "narrow",
		Body:               "Use the provided green bottles for this venue to fit the staging tables.",
		Rationale:          "Venue staging depth is limited for class 101.",
	}
	s.classOverrides[override.ID] = override
	citations := []SourceCitation{
		{
			ID:                   "cite_class_01",
			SourceDocumentID:     source.ID,
			TargetType:           "show_class",
			TargetID:             "class_01",
			PageFrom:             "7",
			PageTo:               "7",
			QuotedText:           "Class 101 is for one hybrid tea bloom staged in the provided green bottle.",
			ExtractionConfidence: 0.94,
		},
		{
			ID:                   "cite_rule_01",
			SourceDocumentID:     source.ID,
			TargetType:           "standard_rule",
			TargetID:             "rule_ojes_container",
			PageFrom:             "11",
			PageTo:               "11",
			QuotedText:           "Exhibitors must use society-provided containers for class 101.",
			ExtractionConfidence: 0.89,
		},
	}
	for i := range citations {
		s.srcCitations[citations[i].ID] = &citations[i]
	}
}

// ============================================================================
// Postgres store
// ============================================================================

type postgresFlowershowStore struct {
	pool *pgxpool.Pool
	mem  *memoryStore // fallback for non-migrated methods
}

func newFlowershowStore(databaseURL string) (flowershowStore, error) {
	if strings.TrimSpace(databaseURL) == "" {
		s := newMemoryStore()
		return s, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("open flowershow database: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping flowershow database: %w", err)
	}

	store := &postgresFlowershowStore{pool: pool, mem: newMemoryStore()}
	if err := store.migrate(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	if err := store.seedIfEmpty(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	if err := store.hydratePersons(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return store, nil
}

func (s *postgresFlowershowStore) Close() {
	if s != nil && s.pool != nil {
		s.pool.Close()
	}
}

func (s *postgresFlowershowStore) migrate(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, `
CREATE TABLE IF NOT EXISTS as_flowershow_objects (
  object_id TEXT PRIMARY KEY,
  object_type TEXT NOT NULL,
  slug TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  created_by TEXT NOT NULL DEFAULT 'system'
);

CREATE TABLE IF NOT EXISTS as_flowershow_claims (
  claim_id TEXT PRIMARY KEY,
  object_id TEXT NOT NULL REFERENCES as_flowershow_objects(object_id) ON DELETE CASCADE,
  claim_seq BIGINT GENERATED ALWAYS AS IDENTITY UNIQUE,
  claim_type TEXT NOT NULL,
  accepted_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  accepted_by TEXT NOT NULL DEFAULT 'system',
  supersedes_claim_id TEXT REFERENCES as_flowershow_claims(claim_id),
  payload JSONB NOT NULL DEFAULT '{}'::jsonb
);

CREATE INDEX IF NOT EXISTS as_flowershow_claims_obj_idx
  ON as_flowershow_claims (object_id, claim_seq DESC);

CREATE TABLE IF NOT EXISTS as_flowershow_agent_tokens (
  id TEXT PRIMARY KEY,
  owner_cognito_sub TEXT NOT NULL,
  owner_email TEXT NOT NULL DEFAULT '',
  owner_name TEXT NOT NULL DEFAULT '',
  label TEXT NOT NULL,
  token_prefix TEXT NOT NULL,
  token_hash TEXT NOT NULL UNIQUE,
  permission_profile TEXT NOT NULL,
  permissions JSONB NOT NULL DEFAULT '[]'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  expires_at TIMESTAMPTZ NOT NULL,
  last_used_at TIMESTAMPTZ,
  revoked_at TIMESTAMPTZ,
  revoked_reason TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS as_flowershow_agent_tokens_owner_idx
  ON as_flowershow_agent_tokens (owner_cognito_sub, created_at DESC);

CREATE TABLE IF NOT EXISTS as_flowershow_auth_pending (
  pending_id TEXT PRIMARY KEY,
  flow TEXT NOT NULL,
  email TEXT NOT NULL,
  cognito_session TEXT NOT NULL DEFAULT '',
  expires_at TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS as_flowershow_auth_pending_expires_idx
  ON as_flowershow_auth_pending (expires_at);

CREATE TABLE IF NOT EXISTS as_flowershow_m_organizations (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  level TEXT NOT NULL DEFAULT 'society',
  parent_id TEXT
);

CREATE TABLE IF NOT EXISTS as_flowershow_m_shows (
  id TEXT PRIMARY KEY,
  slug TEXT NOT NULL UNIQUE,
  organization_id TEXT NOT NULL,
  name TEXT NOT NULL,
  location TEXT NOT NULL DEFAULT '',
  show_date TEXT NOT NULL DEFAULT '',
  season TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'draft',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS as_flowershow_m_show_judges (
  id TEXT PRIMARY KEY,
  show_id TEXT NOT NULL,
  person_id TEXT NOT NULL,
  assigned_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS as_flowershow_m_persons (
  id TEXT PRIMARY KEY,
  first_name TEXT NOT NULL,
  last_name TEXT NOT NULL,
  initials TEXT NOT NULL DEFAULT '',
  email TEXT NOT NULL DEFAULT '',
  public_display_mode TEXT NOT NULL DEFAULT 'initials'
);

ALTER TABLE as_flowershow_m_persons
  ADD COLUMN IF NOT EXISTS public_display_mode TEXT NOT NULL DEFAULT 'initials';

CREATE TABLE IF NOT EXISTS as_flowershow_m_person_organizations (
  person_id TEXT NOT NULL,
  organization_id TEXT NOT NULL,
  role TEXT NOT NULL DEFAULT 'member',
  PRIMARY KEY (person_id, organization_id, role)
);

CREATE TABLE IF NOT EXISTS as_flowershow_m_organization_invites (
  id TEXT PRIMARY KEY,
  organization_id TEXT NOT NULL,
  first_name TEXT NOT NULL DEFAULT '',
  last_name TEXT NOT NULL DEFAULT '',
  email TEXT NOT NULL,
  organization_role TEXT NOT NULL DEFAULT '',
  permission_roles TEXT[] NOT NULL DEFAULT '{}'::text[],
  status TEXT NOT NULL DEFAULT 'pending',
  invited_by_subject TEXT NOT NULL DEFAULT '',
  invited_by_name TEXT NOT NULL DEFAULT '',
  invited_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  claimed_subject_id TEXT NOT NULL DEFAULT '',
  claimed_cognito_sub TEXT NOT NULL DEFAULT '',
  claimed_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS as_flowershow_org_invites_org_idx
  ON as_flowershow_m_organization_invites (organization_id, invited_at DESC);

CREATE INDEX IF NOT EXISTS as_flowershow_org_invites_email_idx
  ON as_flowershow_m_organization_invites ((lower(email)), status);

CREATE TABLE IF NOT EXISTS as_flowershow_m_schedules (
  id TEXT PRIMARY KEY,
  show_id TEXT NOT NULL,
  source_document_id TEXT,
  effective_standard_edition_id TEXT,
  notes TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS as_flowershow_m_divisions (
  id TEXT PRIMARY KEY,
  show_schedule_id TEXT NOT NULL,
  code TEXT NOT NULL DEFAULT '',
  title TEXT NOT NULL,
  domain TEXT NOT NULL DEFAULT 'horticulture',
  sort_order INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS as_flowershow_m_sections (
  id TEXT PRIMARY KEY,
  division_id TEXT NOT NULL,
  code TEXT NOT NULL DEFAULT '',
  title TEXT NOT NULL,
  sort_order INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS as_flowershow_m_classes (
  id TEXT PRIMARY KEY,
  section_id TEXT NOT NULL,
  class_number TEXT NOT NULL DEFAULT '',
  sort_order INTEGER NOT NULL DEFAULT 0,
  title TEXT NOT NULL,
  domain TEXT NOT NULL DEFAULT 'horticulture',
  description TEXT NOT NULL DEFAULT '',
  specimen_count INTEGER NOT NULL DEFAULT 0,
  unit TEXT NOT NULL DEFAULT '',
  measurement_rule TEXT NOT NULL DEFAULT '',
  naming_requirement TEXT NOT NULL DEFAULT '',
  container_rule TEXT NOT NULL DEFAULT '',
  eligibility_rule TEXT NOT NULL DEFAULT '',
  schedule_notes TEXT NOT NULL DEFAULT '',
  taxon_refs TEXT[] NOT NULL DEFAULT '{}'
);

CREATE TABLE IF NOT EXISTS as_flowershow_m_entries (
  id TEXT PRIMARY KEY,
  show_id TEXT NOT NULL,
  class_id TEXT NOT NULL,
  person_id TEXT NOT NULL,
  name TEXT NOT NULL,
  notes TEXT NOT NULL DEFAULT '',
  suppressed BOOLEAN NOT NULL DEFAULT FALSE,
  placement INTEGER NOT NULL DEFAULT 0,
  points DOUBLE PRECISION NOT NULL DEFAULT 0,
  taxon_refs TEXT[] NOT NULL DEFAULT '{}',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS as_flowershow_m_show_credits (
  id TEXT PRIMARY KEY,
  show_id TEXT NOT NULL,
  person_id TEXT NOT NULL DEFAULT '',
  display_name TEXT NOT NULL DEFAULT '',
  credit_label TEXT NOT NULL,
  notes TEXT NOT NULL DEFAULT '',
  sort_order INTEGER NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS as_flowershow_m_taxons (
  id TEXT PRIMARY KEY,
  taxon_type TEXT NOT NULL,
  name TEXT NOT NULL,
  scientific_name TEXT NOT NULL DEFAULT '',
  description TEXT NOT NULL DEFAULT '',
  parent_id TEXT
);

CREATE TABLE IF NOT EXISTS as_flowershow_m_awards (
  id TEXT PRIMARY KEY,
  organization_id TEXT NOT NULL,
  name TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  season TEXT NOT NULL DEFAULT '',
  taxon_filters TEXT[] NOT NULL DEFAULT '{}',
  scoring_rule TEXT NOT NULL DEFAULT 'sum',
  min_entries INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS as_flowershow_m_standard_documents (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  issuing_org_id TEXT NOT NULL DEFAULT '',
  domain_scope TEXT NOT NULL DEFAULT '',
  description TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS as_flowershow_m_standard_editions (
  id TEXT PRIMARY KEY,
  standard_document_id TEXT NOT NULL,
  edition_label TEXT NOT NULL DEFAULT '',
  publication_year INTEGER NOT NULL DEFAULT 0,
  revision_date TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'current',
  source_url TEXT NOT NULL DEFAULT '',
  source_kind TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS as_flowershow_m_source_documents (
  id TEXT PRIMARY KEY,
  organization_id TEXT NOT NULL DEFAULT '',
  show_id TEXT,
  title TEXT NOT NULL,
  document_type TEXT NOT NULL DEFAULT '',
  publication_date TEXT NOT NULL DEFAULT '',
  source_url TEXT NOT NULL DEFAULT '',
  local_path TEXT NOT NULL DEFAULT '',
  checksum TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS as_flowershow_m_source_citations (
  id TEXT PRIMARY KEY,
  source_document_id TEXT NOT NULL,
  target_type TEXT NOT NULL,
  target_id TEXT NOT NULL,
  page_from TEXT NOT NULL DEFAULT '',
  page_to TEXT NOT NULL DEFAULT '',
  quoted_text TEXT NOT NULL DEFAULT '',
  extraction_confidence DOUBLE PRECISION NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS as_flowershow_m_standard_rules (
  id TEXT PRIMARY KEY,
  standard_edition_id TEXT NOT NULL,
  domain TEXT NOT NULL DEFAULT '',
  rule_type TEXT NOT NULL DEFAULT '',
  subject_label TEXT NOT NULL DEFAULT '',
  body TEXT NOT NULL DEFAULT '',
  page_ref TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS as_flowershow_m_class_rule_overrides (
  id TEXT PRIMARY KEY,
  show_class_id TEXT NOT NULL,
  base_standard_rule_id TEXT,
  override_type TEXT NOT NULL DEFAULT 'local_only',
  body TEXT NOT NULL DEFAULT '',
  rationale TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS as_flowershow_m_rubrics (
  id TEXT PRIMARY KEY,
  standard_edition_id TEXT,
  show_id TEXT,
  domain TEXT NOT NULL DEFAULT '',
  title TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS as_flowershow_m_criteria (
  id TEXT PRIMARY KEY,
  judging_rubric_id TEXT NOT NULL,
  name TEXT NOT NULL,
  max_points INTEGER NOT NULL DEFAULT 0,
  sort_order INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS as_flowershow_m_scorecards (
  id TEXT PRIMARY KEY,
  entry_id TEXT NOT NULL,
  judge_id TEXT NOT NULL,
  rubric_id TEXT NOT NULL,
  total_score DOUBLE PRECISION NOT NULL DEFAULT 0,
  notes TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS as_flowershow_m_criterion_scores (
  id TEXT PRIMARY KEY,
  scorecard_id TEXT NOT NULL,
  criterion_id TEXT NOT NULL,
  score DOUBLE PRECISION NOT NULL DEFAULT 0,
  comment TEXT NOT NULL DEFAULT ''
);
`)
	if err != nil {
		return fmt.Errorf("migrate flowershow store: %w", err)
	}
	return nil
}

func (s *postgresFlowershowStore) seedIfEmpty(ctx context.Context) error {
	var count int
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM as_flowershow_m_shows`).Scan(&count); err != nil {
		return fmt.Errorf("count shows: %w", err)
	}
	if count > 0 {
		return nil
	}
	// Seed from memory store's demo data by inserting into Postgres
	mem := newMemoryStore()
	for _, org := range mem.organizations {
		_, _ = s.pool.Exec(ctx, `INSERT INTO as_flowershow_m_organizations (id, name, level, parent_id) VALUES ($1,$2,$3,$4) ON CONFLICT DO NOTHING`,
			org.ID, org.Name, org.Level, org.ParentID)
	}
	for _, show := range mem.shows {
		_, _ = s.pool.Exec(ctx, `INSERT INTO as_flowershow_m_shows (id, slug, organization_id, name, location, show_date, season, status, created_at, updated_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) ON CONFLICT DO NOTHING`,
			show.ID, show.Slug, show.OrganizationID, show.Name, show.Location, show.Date, show.Season, show.Status, show.CreatedAt, show.UpdatedAt)
	}
	for _, p := range mem.persons {
		_, _ = s.pool.Exec(ctx, `INSERT INTO as_flowershow_m_persons (id, first_name, last_name, initials, email, public_display_mode) VALUES ($1,$2,$3,$4,$5,$6) ON CONFLICT DO NOTHING`,
			p.ID, p.FirstName, p.LastName, p.Initials, p.Email, p.PublicDisplayMode)
	}
	for _, po := range mem.personOrgs {
		_, _ = s.pool.Exec(ctx, `INSERT INTO as_flowershow_m_person_organizations (person_id, organization_id, role) VALUES ($1,$2,$3) ON CONFLICT DO NOTHING`,
			po.PersonID, po.OrganizationID, po.Role)
	}
	for _, assignment := range mem.showJudges {
		_, _ = s.pool.Exec(ctx, `INSERT INTO as_flowershow_m_show_judges (id, show_id, person_id, assigned_at) VALUES ($1,$2,$3,$4) ON CONFLICT DO NOTHING`,
			assignment.ID, assignment.ShowID, assignment.PersonID, assignment.AssignedAt)
	}
	for _, sched := range mem.schedules {
		_, _ = s.pool.Exec(ctx, `INSERT INTO as_flowershow_m_schedules (id, show_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`,
			sched.ID, sched.ShowID)
	}
	for _, d := range mem.divisions {
		_, _ = s.pool.Exec(ctx, `INSERT INTO as_flowershow_m_divisions (id, show_schedule_id, code, title, domain, sort_order) VALUES ($1,$2,$3,$4,$5,$6) ON CONFLICT DO NOTHING`,
			d.ID, d.ShowScheduleID, d.Code, d.Title, d.Domain, d.SortOrder)
	}
	for _, sec := range mem.sections {
		_, _ = s.pool.Exec(ctx, `INSERT INTO as_flowershow_m_sections (id, division_id, code, title, sort_order) VALUES ($1,$2,$3,$4,$5) ON CONFLICT DO NOTHING`,
			sec.ID, sec.DivisionID, sec.Code, sec.Title, sec.SortOrder)
	}
	for _, c := range mem.classes {
		_, _ = s.pool.Exec(ctx, `INSERT INTO as_flowershow_m_classes (id, section_id, class_number, sort_order, title, domain, description, specimen_count, taxon_refs) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9) ON CONFLICT DO NOTHING`,
			c.ID, c.SectionID, c.ClassNumber, c.SortOrder, c.Title, c.Domain, c.Description, c.SpecimenCount, c.TaxonRefs)
	}
	for _, e := range mem.entries {
		_, _ = s.pool.Exec(ctx, `INSERT INTO as_flowershow_m_entries (id, show_id, class_id, person_id, name, suppressed, placement, points, taxon_refs, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) ON CONFLICT DO NOTHING`,
			e.ID, e.ShowID, e.ClassID, e.PersonID, e.Name, e.Suppressed, e.Placement, e.Points, e.TaxonRefs, e.CreatedAt)
	}
	for _, credit := range mem.showCredits {
		_, _ = s.pool.Exec(ctx, `INSERT INTO as_flowershow_m_show_credits (id, show_id, person_id, display_name, credit_label, notes, sort_order, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8) ON CONFLICT DO NOTHING`,
			credit.ID, credit.ShowID, credit.PersonID, credit.DisplayName, credit.CreditLabel, credit.Notes, credit.SortOrder, credit.CreatedAt)
	}
	for _, t := range mem.taxons {
		_, _ = s.pool.Exec(ctx, `INSERT INTO as_flowershow_m_taxons (id, taxon_type, name, scientific_name, description, parent_id) VALUES ($1,$2,$3,$4,$5,$6) ON CONFLICT DO NOTHING`,
			t.ID, t.TaxonType, t.Name, t.ScientificName, t.Description, t.ParentID)
	}
	for _, a := range mem.awards {
		_, _ = s.pool.Exec(ctx, `INSERT INTO as_flowershow_m_awards (id, organization_id, name, season, taxon_filters, scoring_rule) VALUES ($1,$2,$3,$4,$5,$6) ON CONFLICT DO NOTHING`,
			a.ID, a.OrganizationID, a.Name, a.Season, a.TaxonFilters, a.ScoringRule)
	}
	for _, std := range mem.stdDocs {
		_, _ = s.pool.Exec(ctx, `INSERT INTO as_flowershow_m_standard_documents (id, name, issuing_org_id, domain_scope, description) VALUES ($1,$2,$3,$4,$5) ON CONFLICT DO NOTHING`,
			std.ID, std.Name, std.IssuingOrg, std.DomainScope, std.Description)
	}
	for _, ed := range mem.stdEditions {
		_, _ = s.pool.Exec(ctx, `INSERT INTO as_flowershow_m_standard_editions (id, standard_document_id, edition_label, publication_year, revision_date, status, source_url, source_kind) VALUES ($1,$2,$3,$4,$5,$6,$7,$8) ON CONFLICT DO NOTHING`,
			ed.ID, ed.StandardDocumentID, ed.EditionLabel, ed.PublicationYear, ed.RevisionDate, ed.Status, ed.SourceURL, ed.SourceKind)
	}
	for _, doc := range mem.srcDocs {
		_, _ = s.pool.Exec(ctx, `INSERT INTO as_flowershow_m_source_documents (id, organization_id, show_id, title, document_type, publication_date, source_url, local_path, checksum) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9) ON CONFLICT DO NOTHING`,
			doc.ID, doc.OrganizationID, doc.ShowID, doc.Title, doc.DocumentType, doc.PublicationDate, doc.SourceURL, doc.LocalPath, doc.Checksum)
	}
	for _, cite := range mem.srcCitations {
		_, _ = s.pool.Exec(ctx, `INSERT INTO as_flowershow_m_source_citations (id, source_document_id, target_type, target_id, page_from, page_to, quoted_text, extraction_confidence) VALUES ($1,$2,$3,$4,$5,$6,$7,$8) ON CONFLICT DO NOTHING`,
			cite.ID, cite.SourceDocumentID, cite.TargetType, cite.TargetID, cite.PageFrom, cite.PageTo, cite.QuotedText, cite.ExtractionConfidence)
	}
	for _, rule := range mem.stdRules {
		_, _ = s.pool.Exec(ctx, `INSERT INTO as_flowershow_m_standard_rules (id, standard_edition_id, domain, rule_type, subject_label, body, page_ref) VALUES ($1,$2,$3,$4,$5,$6,$7) ON CONFLICT DO NOTHING`,
			rule.ID, rule.StandardEditionID, rule.Domain, rule.RuleType, rule.SubjectLabel, rule.Body, rule.PageRef)
	}
	for _, override := range mem.classOverrides {
		_, _ = s.pool.Exec(ctx, `INSERT INTO as_flowershow_m_class_rule_overrides (id, show_class_id, base_standard_rule_id, override_type, body, rationale) VALUES ($1,$2,$3,$4,$5,$6) ON CONFLICT DO NOTHING`,
			override.ID, override.ShowClassID, override.BaseStandardRuleID, override.OverrideType, override.Body, override.Rationale)
	}
	for _, r := range mem.rubrics {
		_, _ = s.pool.Exec(ctx, `INSERT INTO as_flowershow_m_rubrics (id, domain, title) VALUES ($1,$2,$3) ON CONFLICT DO NOTHING`,
			r.ID, r.Domain, r.Title)
	}
	for _, c := range mem.criteria {
		_, _ = s.pool.Exec(ctx, `INSERT INTO as_flowershow_m_criteria (id, judging_rubric_id, name, max_points, sort_order) VALUES ($1,$2,$3,$4,$5) ON CONFLICT DO NOTHING`,
			c.ID, c.JudgingRubricID, c.Name, c.MaxPoints, c.SortOrder)
	}
	return nil
}

func (s *postgresFlowershowStore) hydratePersons(ctx context.Context) error {
	rows, err := s.pool.Query(ctx, `SELECT id, first_name, last_name, initials, email, public_display_mode FROM as_flowershow_m_persons`)
	if err != nil {
		return fmt.Errorf("load persons: %w", err)
	}
	defer rows.Close()

	persons := make(map[string]*Person)
	for rows.Next() {
		var p Person
		if err := rows.Scan(&p.ID, &p.FirstName, &p.LastName, &p.Initials, &p.Email, &p.PublicDisplayMode); err != nil {
			return fmt.Errorf("scan person: %w", err)
		}
		person := p
		persons[p.ID] = &person
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate persons: %w", err)
	}

	linkRows, err := s.pool.Query(ctx, `SELECT person_id, organization_id, role FROM as_flowershow_m_person_organizations`)
	if err != nil {
		return fmt.Errorf("load person organizations: %w", err)
	}
	defer linkRows.Close()

	personOrgs := make(map[string]*PersonOrganization)
	for linkRows.Next() {
		var link PersonOrganization
		if err := linkRows.Scan(&link.PersonID, &link.OrganizationID, &link.Role); err != nil {
			return fmt.Errorf("scan person organization: %w", err)
		}
		item := link
		personOrgs[item.PersonID+"|"+item.OrganizationID+"|"+item.Role] = &item
	}
	if err := linkRows.Err(); err != nil {
		return fmt.Errorf("iterate person organizations: %w", err)
	}

	s.mem.mu.Lock()
	for id, person := range persons {
		s.mem.persons[id] = person
	}
	for key, link := range personOrgs {
		s.mem.personOrgs[key] = link
	}
	s.mem.mu.Unlock()
	return nil
}

// For now, the Postgres store delegates to the memory store for all operations.
// A full Postgres implementation would replace each method with SQL queries.
// This allows the app to start with Postgres (migrated schema) while using
// the memory store pattern for development speed.

func (s *postgresFlowershowStore) createOrganization(o Organization) (*Organization, error) {
	return s.mem.createOrganization(o)
}
func (s *postgresFlowershowStore) organizationByID(id string) (*Organization, bool) {
	return s.mem.organizationByID(id)
}
func (s *postgresFlowershowStore) allOrganizations() []*Organization { return s.mem.allOrganizations() }
func (s *postgresFlowershowStore) createShow(input ShowInput) (*Show, error) {
	return s.mem.createShow(input)
}
func (s *postgresFlowershowStore) updateShow(id string, input ShowInput) (*Show, error) {
	return s.mem.updateShow(id, input)
}
func (s *postgresFlowershowStore) showByID(id string) (*Show, bool) { return s.mem.showByID(id) }
func (s *postgresFlowershowStore) showBySlug(slug string) (*Show, bool) {
	return s.mem.showBySlug(slug)
}
func (s *postgresFlowershowStore) allShows() []*Show { return s.mem.allShows() }
func (s *postgresFlowershowStore) assignJudgeToShow(showID, personID string) (*ShowJudgeAssignment, error) {
	return s.mem.assignJudgeToShow(showID, personID)
}
func (s *postgresFlowershowStore) judgesByShow(showID string) []*ShowJudgeAssignment {
	return s.mem.judgesByShow(showID)
}
func (s *postgresFlowershowStore) createPerson(input PersonInput) (*Person, error) {
	person, err := s.mem.createPerson(input)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err = s.pool.Exec(ctx, `INSERT INTO as_flowershow_m_persons (id, first_name, last_name, initials, email, public_display_mode)
		VALUES ($1,$2,$3,$4,$5,$6)`,
		person.ID, person.FirstName, person.LastName, person.Initials, person.Email, person.PublicDisplayMode)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(input.OrganizationID) != "" {
		role := strings.TrimSpace(input.OrganizationRole)
		if role == "" {
			role = "member"
		}
		_, err = s.pool.Exec(ctx, `INSERT INTO as_flowershow_m_person_organizations (person_id, organization_id, role)
			VALUES ($1,$2,$3) ON CONFLICT DO NOTHING`,
			person.ID, strings.TrimSpace(input.OrganizationID), role)
		if err != nil {
			return nil, err
		}
	}
	return person, nil
}
func (s *postgresFlowershowStore) updatePerson(id string, input PersonInput) (*Person, error) {
	person, err := s.mem.updatePerson(id, input)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	tag, err := s.pool.Exec(ctx, `UPDATE as_flowershow_m_persons
		SET first_name = $2, last_name = $3, initials = $4, email = $5, public_display_mode = $6
		WHERE id = $1`,
		person.ID, person.FirstName, person.LastName, person.Initials, person.Email, person.PublicDisplayMode)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, errors.New("person not found")
	}
	return person, nil
}
func (s *postgresFlowershowStore) personByID(id string) (*Person, bool) { return s.mem.personByID(id) }
func (s *postgresFlowershowStore) allPersons() []*Person                { return s.mem.allPersons() }
func (s *postgresFlowershowStore) linkPersonOrganization(link PersonOrganization) (*PersonOrganization, error) {
	item, err := s.mem.linkPersonOrganization(link)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err = s.pool.Exec(ctx, `INSERT INTO as_flowershow_m_person_organizations (person_id, organization_id, role)
		VALUES ($1,$2,$3) ON CONFLICT DO NOTHING`,
		item.PersonID, item.OrganizationID, item.Role)
	if err != nil {
		return nil, err
	}
	return item, nil
}
func (s *postgresFlowershowStore) personOrganizationsByPerson(personID string) []*PersonOrganization {
	return s.mem.personOrganizationsByPerson(personID)
}
func (s *postgresFlowershowStore) lookupPersonsForShow(showID, query string) []*PersonOrganization {
	return s.mem.lookupPersonsForShow(showID, query)
}
func (s *postgresFlowershowStore) createSchedule(sched ShowSchedule) (*ShowSchedule, error) {
	return s.mem.createSchedule(sched)
}
func (s *postgresFlowershowStore) updateSchedule(showID string, input ShowSchedule) (*ShowSchedule, error) {
	return s.mem.updateSchedule(showID, input)
}
func (s *postgresFlowershowStore) scheduleByShowID(showID string) (*ShowSchedule, bool) {
	return s.mem.scheduleByShowID(showID)
}
func (s *postgresFlowershowStore) createDivision(input DivisionInput) (*Division, error) {
	return s.mem.createDivision(input)
}
func (s *postgresFlowershowStore) divisionsBySchedule(scheduleID string) []*Division {
	return s.mem.divisionsBySchedule(scheduleID)
}
func (s *postgresFlowershowStore) divisionByID(id string) (*Division, bool) {
	return s.mem.divisionByID(id)
}
func (s *postgresFlowershowStore) createSection(input SectionInput) (*Section, error) {
	return s.mem.createSection(input)
}
func (s *postgresFlowershowStore) sectionsByDivision(divisionID string) []*Section {
	return s.mem.sectionsByDivision(divisionID)
}
func (s *postgresFlowershowStore) sectionByID(id string) (*Section, bool) {
	return s.mem.sectionByID(id)
}
func (s *postgresFlowershowStore) createClass(input ShowClassInput) (*ShowClass, error) {
	return s.mem.createClass(input)
}
func (s *postgresFlowershowStore) updateClass(id string, input ShowClassInput) (*ShowClass, error) {
	return s.mem.updateClass(id, input)
}
func (s *postgresFlowershowStore) reorderClass(id string, sortOrder int) (*ShowClass, error) {
	return s.mem.reorderClass(id, sortOrder)
}
func (s *postgresFlowershowStore) classesBySection(sectionID string) []*ShowClass {
	return s.mem.classesBySection(sectionID)
}
func (s *postgresFlowershowStore) classByID(id string) (*ShowClass, bool) { return s.mem.classByID(id) }
func (s *postgresFlowershowStore) classesByShowID(showID string) []*ShowClass {
	return s.mem.classesByShowID(showID)
}
func (s *postgresFlowershowStore) createEntry(input EntryInput) (*Entry, error) {
	return s.mem.createEntry(input)
}
func (s *postgresFlowershowStore) updateEntry(id string, input EntryInput) (*Entry, error) {
	return s.mem.updateEntry(id, input)
}
func (s *postgresFlowershowStore) moveEntry(entryID, classID, reason string) (*Entry, error) {
	return s.mem.moveEntry(entryID, classID, reason)
}
func (s *postgresFlowershowStore) reassignEntry(entryID, personID string) (*Entry, error) {
	return s.mem.reassignEntry(entryID, personID)
}
func (s *postgresFlowershowStore) deleteEntry(entryID string) error { return s.mem.deleteEntry(entryID) }
func (s *postgresFlowershowStore) setEntrySuppressed(entryID string, suppressed bool) error {
	return s.mem.setEntrySuppressed(entryID, suppressed)
}
func (s *postgresFlowershowStore) setPlacement(entryID string, placement int, points float64) error {
	return s.mem.setPlacement(entryID, placement, points)
}
func (s *postgresFlowershowStore) entryByID(id string) (*Entry, bool) { return s.mem.entryByID(id) }
func (s *postgresFlowershowStore) entriesByShow(showID string) []*Entry {
	return s.mem.entriesByShow(showID)
}
func (s *postgresFlowershowStore) entriesByClass(classID string) []*Entry {
	return s.mem.entriesByClass(classID)
}
func (s *postgresFlowershowStore) entriesByPerson(personID string) []*Entry {
	return s.mem.entriesByPerson(personID)
}
func (s *postgresFlowershowStore) createShowCredit(input ShowCreditInput) (*ShowCredit, error) {
	return s.mem.createShowCredit(input)
}
func (s *postgresFlowershowStore) showCreditByID(id string) (*ShowCredit, bool) {
	return s.mem.showCreditByID(id)
}
func (s *postgresFlowershowStore) deleteShowCredit(id string) error { return s.mem.deleteShowCredit(id) }
func (s *postgresFlowershowStore) showCreditsByShow(showID string) []*ShowCredit {
	return s.mem.showCreditsByShow(showID)
}
func (s *postgresFlowershowStore) attachMedia(m Media) (*Media, error) { return s.mem.attachMedia(m) }
func (s *postgresFlowershowStore) mediaByEntry(entryID string) []*Media {
	return s.mem.mediaByEntry(entryID)
}
func (s *postgresFlowershowStore) mediaByID(id string) (*Media, bool) { return s.mem.mediaByID(id) }
func (s *postgresFlowershowStore) deleteMedia(id string) error        { return s.mem.deleteMedia(id) }
func (s *postgresFlowershowStore) createTaxon(input TaxonInput) (*Taxon, error) {
	return s.mem.createTaxon(input)
}
func (s *postgresFlowershowStore) taxonByID(id string) (*Taxon, bool) { return s.mem.taxonByID(id) }
func (s *postgresFlowershowStore) allTaxons() []*Taxon                { return s.mem.allTaxons() }
func (s *postgresFlowershowStore) taxonsByType(taxonType string) []*Taxon {
	return s.mem.taxonsByType(taxonType)
}
func (s *postgresFlowershowStore) createAward(input AwardInput) (*AwardDefinition, error) {
	return s.mem.createAward(input)
}
func (s *postgresFlowershowStore) awardByID(id string) (*AwardDefinition, bool) {
	return s.mem.awardByID(id)
}
func (s *postgresFlowershowStore) awardsByOrganization(orgID string) []*AwardDefinition {
	return s.mem.awardsByOrganization(orgID)
}
func (s *postgresFlowershowStore) computeAward(awardID string) ([]AwardResult, error) {
	return s.mem.computeAward(awardID)
}
func (s *postgresFlowershowStore) createStandardDocument(doc StandardDocument) (*StandardDocument, error) {
	return s.mem.createStandardDocument(doc)
}
func (s *postgresFlowershowStore) allStandardDocuments() []*StandardDocument {
	return s.mem.allStandardDocuments()
}
func (s *postgresFlowershowStore) createStandardEdition(ed StandardEdition) (*StandardEdition, error) {
	return s.mem.createStandardEdition(ed)
}
func (s *postgresFlowershowStore) standardEditionByID(id string) (*StandardEdition, bool) {
	return s.mem.standardEditionByID(id)
}
func (s *postgresFlowershowStore) allStandardEditions() []*StandardEdition {
	return s.mem.allStandardEditions()
}
func (s *postgresFlowershowStore) editionsByStandard(standardDocID string) []*StandardEdition {
	return s.mem.editionsByStandard(standardDocID)
}
func (s *postgresFlowershowStore) createSourceDocument(doc SourceDocument) (*SourceDocument, error) {
	return s.mem.createSourceDocument(doc)
}
func (s *postgresFlowershowStore) allSourceDocuments() []*SourceDocument {
	return s.mem.allSourceDocuments()
}
func (s *postgresFlowershowStore) createSourceCitation(cite SourceCitation) (*SourceCitation, error) {
	return s.mem.createSourceCitation(cite)
}
func (s *postgresFlowershowStore) citationsByTarget(targetType, targetID string) []*SourceCitation {
	return s.mem.citationsByTarget(targetType, targetID)
}
func (s *postgresFlowershowStore) createStandardRule(rule StandardRule) (*StandardRule, error) {
	return s.mem.createStandardRule(rule)
}
func (s *postgresFlowershowStore) rulesByEdition(editionID string) []*StandardRule {
	return s.mem.rulesByEdition(editionID)
}
func (s *postgresFlowershowStore) createClassRuleOverride(override ClassRuleOverride) (*ClassRuleOverride, error) {
	return s.mem.createClassRuleOverride(override)
}
func (s *postgresFlowershowStore) overridesByClass(classID string) []*ClassRuleOverride {
	return s.mem.overridesByClass(classID)
}
func (s *postgresFlowershowStore) effectiveRulesForClass(classID string, editionID string) []effectiveRule {
	return s.mem.effectiveRulesForClass(classID, editionID)
}
func (s *postgresFlowershowStore) createRubric(r JudgingRubric) (*JudgingRubric, error) {
	return s.mem.createRubric(r)
}
func (s *postgresFlowershowStore) rubricByID(id string) (*JudgingRubric, bool) {
	return s.mem.rubricByID(id)
}
func (s *postgresFlowershowStore) allRubrics() []*JudgingRubric { return s.mem.allRubrics() }
func (s *postgresFlowershowStore) createCriterion(c JudgingCriterion) (*JudgingCriterion, error) {
	return s.mem.createCriterion(c)
}
func (s *postgresFlowershowStore) criteriaByRubric(rubricID string) []*JudgingCriterion {
	return s.mem.criteriaByRubric(rubricID)
}
func (s *postgresFlowershowStore) submitScorecard(sc EntryScorecard, scores []EntryCriterionScore) (*EntryScorecard, error) {
	return s.mem.submitScorecard(sc, scores)
}
func (s *postgresFlowershowStore) scorecardsByEntry(entryID string) []*EntryScorecard {
	return s.mem.scorecardsByEntry(entryID)
}
func (s *postgresFlowershowStore) criterionScoresByScorecard(scorecardID string) []*EntryCriterionScore {
	return s.mem.criterionScoresByScorecard(scorecardID)
}
func (s *postgresFlowershowStore) computePlacementsFromScores(classID string) error {
	return s.mem.computePlacementsFromScores(classID)
}
func (s *postgresFlowershowStore) leaderboard(orgID, season string) []LeaderboardEntry {
	return s.mem.leaderboard(orgID, season)
}
func (s *postgresFlowershowStore) ledgerByObjectID(objectID string) ([]FlowershowClaim, error) {
	return s.mem.ledgerByObjectID(objectID)
}

// Ensure compile-time check
var _ flowershowStore = (*memoryStore)(nil)
var _ flowershowStore = (*postgresFlowershowStore)(nil)

// Suppress unused import warnings
var (
	_ = pgx.ErrNoRows
	_ = json.Marshal
)
