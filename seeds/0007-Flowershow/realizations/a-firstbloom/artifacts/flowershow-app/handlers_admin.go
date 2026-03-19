package main

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

func (a *app) handleAdminLogout(w http.ResponseWriter, r *http.Request) {
	a.clearPendingAuth(w, r)
	a.clearUserSession(w, r)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// --- Dashboard ---

type adminDashboardData struct {
	Title       string
	CurrentPath string
	Shows       []*Show
	Persons     []*Person
	Orgs        []*Organization
	Awards      []*AwardDefinition
	Rubrics     []*JudgingRubric
}

func (a *app) handleAdminDashboard(w http.ResponseWriter, r *http.Request) {
	orgs := a.store.allOrganizations()
	var awards []*AwardDefinition
	for _, org := range orgs {
		awards = append(awards, a.store.awardsByOrganization(org.ID)...)
	}
	a.render(w, "admin_dashboard.html", adminDashboardData{
		Title:       "Admin Dashboard",
		CurrentPath: "/admin",
		Shows:       a.store.allShows(),
		Persons:     a.store.allPersons(),
		Orgs:        orgs,
		Awards:      awards,
		Rubrics:     a.store.allRubrics(),
	})
}

// --- Show CRUD ---

func (a *app) handleAdminShowNew(w http.ResponseWriter, r *http.Request) {
	a.render(w, "admin_show_new.html", map[string]any{
		"Title":       "New Show",
		"CurrentPath": "/admin/shows/new",
		"Orgs":        a.store.allOrganizations(),
	})
}

func (a *app) handleAdminShowCreate(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	show, err := a.store.createShow(ShowInput{
		OrganizationID: r.FormValue("organization_id"),
		Name:           r.FormValue("name"),
		Location:       r.FormValue("location"),
		Date:           r.FormValue("date"),
		Season:         r.FormValue("season"),
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, "/admin/shows/"+show.ID, http.StatusSeeOther)
}

type adminShowDetailData struct {
	Title            string
	CurrentPath      string
	Show             *Show
	Schedule         *ShowSchedule
	ScheduleEdition  *StandardEdition
	Divisions        []*divisionView
	Entries          []*entryView
	Persons          []*Person
	Classes          []*ShowClass
	Awards           []*AwardDefinition
	Rubrics          []*JudgingRubric
	RubricViews      []*rubricView
	Orgs             []*Organization
	Standards        []*StandardDocument
	StandardViews    []*standardView
	StandardEditions []*StandardEdition
	Sources          []*SourceDocument
	Judges           []*showJudgeView
	AvailableJudges  []*Person
	ClassRuleViews   []*classRuleView
	CitationTargets  []citationTargetOption
}

func (a *app) handleAdminShowDetail(w http.ResponseWriter, r *http.Request) {
	showID := r.PathValue("showID")
	data, err := a.adminShowDetailData(showID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	a.render(w, "show_admin.html", data)
}

func (a *app) adminShowDetailData(showID string) (adminShowDetailData, error) {
	show, ok := a.store.showByID(showID)
	if !ok {
		return adminShowDetailData{}, fmt.Errorf("show not found")
	}

	sched, _ := a.store.scheduleByShowID(show.ID)
	var divisions []*divisionView
	if sched != nil {
		for _, div := range a.store.divisionsBySchedule(sched.ID) {
			dv := &divisionView{Division: div}
			for _, sec := range a.store.sectionsByDivision(div.ID) {
				sv := &sectionView{
					Section: sec,
					Classes: a.store.classesBySection(sec.ID),
				}
				dv.Sections = append(dv.Sections, sv)
			}
			divisions = append(divisions, dv)
		}
	}

	var entries []*entryView
	for _, e := range a.store.entriesByShow(show.ID) {
		person, _ := a.store.personByID(e.PersonID)
		cls, _ := a.store.classByID(e.ClassID)
		entries = append(entries, &entryView{
			Entry:  e,
			Person: person,
			Class:  cls,
			Media:  a.store.mediaByEntry(e.ID),
		})
	}

	orgs := a.store.allOrganizations()
	var awards []*AwardDefinition
	for _, org := range orgs {
		awards = append(awards, a.store.awardsByOrganization(org.ID)...)
	}
	standardViews := a.standardViews()
	rubricViews := a.rubricViewsForShow(show.ID)
	var scheduleEdition *StandardEdition
	if sched != nil && sched.EffectiveStandardEditionID != "" {
		scheduleEdition, _ = a.store.standardEditionByID(sched.EffectiveStandardEditionID)
	}
	classRuleViews := a.classRuleViews(show.ID, func() string {
		if sched == nil {
			return ""
		}
		return sched.EffectiveStandardEditionID
	}())

	return adminShowDetailData{
		Title:            "Admin: " + show.Name,
		CurrentPath:      "/admin/shows/" + show.ID,
		Show:             show,
		Schedule:         sched,
		ScheduleEdition:  scheduleEdition,
		Divisions:        divisions,
		Entries:          entries,
		Persons:          a.store.allPersons(),
		Classes:          a.store.classesByShowID(show.ID),
		Awards:           awards,
		Rubrics:          a.store.allRubrics(),
		RubricViews:      rubricViews,
		Orgs:             orgs,
		Standards:        a.store.allStandardDocuments(),
		StandardViews:    standardViews,
		StandardEditions: a.store.allStandardEditions(),
		Sources:          a.store.allSourceDocuments(),
		Judges:           a.judgeViewsForShow(show.ID),
		AvailableJudges:  a.availableJudgesForShow(show.ID),
		ClassRuleViews:   classRuleViews,
		CitationTargets:  a.citationTargetsForShow(show.ID, classRuleViews),
	}, nil
}

func (a *app) handleAdminShowUpdate(w http.ResponseWriter, r *http.Request) {
	showID := r.PathValue("showID")
	r.ParseForm()
	_, err := a.store.updateShow(showID, ShowInput{
		OrganizationID: r.FormValue("organization_id"),
		Name:           r.FormValue("name"),
		Location:       r.FormValue("location"),
		Date:           r.FormValue("date"),
		Season:         r.FormValue("season"),
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	a.sseBroker.publish(showID, "show-updated", `<div class="toast">Show updated</div>`)
	a.publishAdminSections(showID, "info")
	a.respondAdminSectionOrRedirect(w, r, showID, "info")
}

// --- Schedule & Hierarchy ---

func (a *app) handleAdminScheduleCreate(w http.ResponseWriter, r *http.Request) {
	showID := r.PathValue("showID")
	r.ParseForm()
	if _, ok := a.store.scheduleByShowID(showID); ok {
		_, err := a.store.updateSchedule(showID, ShowSchedule{
			SourceDocumentID:           r.FormValue("source_document_id"),
			EffectiveStandardEditionID: r.FormValue("edition_id"),
			Notes:                      r.FormValue("notes"),
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		a.sseBroker.publish(showID, "schedule-created", `<div class="toast">Schedule governance updated</div>`)
		a.publishAdminSections(showID, "schedule", "governance", "scoring")
		a.respondAdminSectionOrRedirect(w, r, showID, "schedule")
		return
	}
	_, err := a.store.createSchedule(ShowSchedule{
		ShowID:                     showID,
		SourceDocumentID:           r.FormValue("source_document_id"),
		EffectiveStandardEditionID: r.FormValue("edition_id"),
		Notes:                      r.FormValue("notes"),
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	a.sseBroker.publish(showID, "schedule-created", `<div class="toast">Schedule created</div>`)
	a.publishAdminSections(showID, "schedule", "governance", "scoring")
	a.respondAdminSectionOrRedirect(w, r, showID, "schedule")
}

func (a *app) handleAdminJudgeAssign(w http.ResponseWriter, r *http.Request) {
	showID := r.PathValue("showID")
	r.ParseForm()
	if _, err := a.store.assignJudgeToShow(showID, r.FormValue("person_id")); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	a.sseBroker.publish(showID, "show-updated", `<div class="toast">Judge assigned</div>`)
	a.publishAdminSections(showID, "info", "scoring")
	a.respondAdminSectionOrRedirect(w, r, showID, "info")
}

func (a *app) handleAdminDivisionCreate(w http.ResponseWriter, r *http.Request) {
	showID := r.PathValue("showID")
	r.ParseForm()
	schedID := r.FormValue("schedule_id")
	sortOrder, _ := strconv.Atoi(r.FormValue("sort_order"))
	_, err := a.store.createDivision(DivisionInput{
		ShowScheduleID: schedID,
		Code:           r.FormValue("code"),
		Title:          r.FormValue("title"),
		Domain:         r.FormValue("domain"),
		SortOrder:      sortOrder,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	a.sseBroker.publish(showID, "division-created", `<div class="toast">Division added</div>`)
	a.publishAdminSections(showID, "schedule", "entries", "scoring", "governance")
	a.respondAdminSectionOrRedirect(w, r, showID, "schedule")
}

func (a *app) handleAdminSectionCreate(w http.ResponseWriter, r *http.Request) {
	showID := r.PathValue("showID")
	r.ParseForm()
	sortOrder, _ := strconv.Atoi(r.FormValue("sort_order"))
	_, err := a.store.createSection(SectionInput{
		DivisionID: r.FormValue("division_id"),
		Code:       r.FormValue("code"),
		Title:      r.FormValue("title"),
		SortOrder:  sortOrder,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	a.sseBroker.publish(showID, "section-created", `<div class="toast">Section added</div>`)
	a.publishAdminSections(showID, "schedule", "entries", "scoring", "governance")
	a.respondAdminSectionOrRedirect(w, r, showID, "schedule")
}

func (a *app) handleAdminClassCreate(w http.ResponseWriter, r *http.Request) {
	showID := r.PathValue("showID")
	r.ParseForm()
	specimenCount, _ := strconv.Atoi(r.FormValue("specimen_count"))
	_, err := a.store.createClass(ShowClassInput{
		SectionID:     r.FormValue("section_id"),
		ClassNumber:   r.FormValue("class_number"),
		Title:         r.FormValue("title"),
		Domain:        r.FormValue("domain"),
		Description:   r.FormValue("description"),
		SpecimenCount: specimenCount,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	a.sseBroker.publish(showID, "class-created", `<div class="toast">Class added</div>`)
	a.publishAdminSections(showID, "schedule", "entries", "scoring", "governance")
	a.respondAdminSectionOrRedirect(w, r, showID, "schedule")
}

// --- Entries ---

func (a *app) handleAdminEntryCreate(w http.ResponseWriter, r *http.Request) {
	showID := r.PathValue("showID")
	r.ParseForm()
	_, err := a.store.createEntry(EntryInput{
		ShowID:   showID,
		ClassID:  r.FormValue("class_id"),
		PersonID: r.FormValue("person_id"),
		Name:     r.FormValue("name"),
		Notes:    r.FormValue("notes"),
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	a.sseBroker.publish(showID, "entry-created", `<div class="toast">Entry added</div>`)
	a.publishAdminSections(showID, "entries", "winners", "scoring", "governance")
	a.publishShowSummary(showID)
	a.respondAdminSectionOrRedirect(w, r, showID, "entries")
}

func (a *app) handleAdminEntryPlacement(w http.ResponseWriter, r *http.Request) {
	entryID := r.PathValue("entryID")
	r.ParseForm()
	placement, _ := strconv.Atoi(r.FormValue("placement"))
	points, _ := strconv.ParseFloat(r.FormValue("points"), 64)
	if points == 0 {
		pointsMap := map[int]float64{1: 6, 2: 4, 3: 2}
		points = pointsMap[placement]
	}
	if err := a.store.setPlacement(entryID, placement, points); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Find show for SSE
	if entry, ok := a.store.entryByID(entryID); ok {
		a.sseBroker.publish(entry.ShowID, "placement-set",
			fmt.Sprintf(`<div class="toast">Placement set: %s → %d</div>`, entry.Name, placement))
		a.publishAdminSections(entry.ShowID, "entries", "winners")
		a.publishShowSummary(entry.ShowID)
		a.respondAdminSectionOrRedirect(w, r, entry.ShowID, "winners")
		return
	}

	referer := r.Header.Get("Referer")
	if referer == "" {
		referer = "/admin"
	}
	http.Redirect(w, r, referer, http.StatusSeeOther)
}

func (a *app) handleAdminEntryVisibility(w http.ResponseWriter, r *http.Request) {
	entryID := r.PathValue("entryID")
	r.ParseForm()
	suppressed := r.FormValue("suppressed") == "true" || r.FormValue("suppressed") == "on"
	if err := a.store.setEntrySuppressed(entryID, suppressed); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if entry, ok := a.store.entryByID(entryID); ok {
		label := "Entry visible publicly"
		if suppressed {
			label = "Entry suppressed from public view"
		}
		a.sseBroker.publish(entry.ShowID, "placement-set", fmt.Sprintf(`<div class="toast">%s</div>`, label))
		a.publishAdminSections(entry.ShowID, "entries", "winners", "scoring", "governance")
		a.publishShowSummary(entry.ShowID)
		a.respondAdminSectionOrRedirect(w, r, entry.ShowID, "entries")
		return
	}
	redirect(w, r)
}

func (a *app) handleMediaUpload(w http.ResponseWriter, r *http.Request) {
	entryID := r.PathValue("entryID")
	if _, ok := a.store.entryByID(entryID); !ok {
		http.NotFound(w, r)
		return
	}
	if err := r.ParseMultipartForm(maxFormMemory); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	headers := r.MultipartForm.File["media"]
	if len(headers) == 0 {
		http.Error(w, "media file required", http.StatusBadRequest)
		return
	}
	for _, header := range headers {
		media, err := a.media.Store(r.Context(), entryID, header)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if _, err := a.store.attachMedia(*media); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}
	if entry, ok := a.store.entryByID(entryID); ok {
		a.publishAdminSections(entry.ShowID, "entries")
		a.publishShowSummary(entry.ShowID)
		a.respondAdminSectionOrRedirect(w, r, entry.ShowID, "entries")
		return
	}
	redirect(w, r)
}

func (a *app) handleMediaDelete(w http.ResponseWriter, r *http.Request) {
	mediaID := r.PathValue("mediaID")
	media, ok := a.store.mediaByID(mediaID)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if err := a.media.Delete(r.Context(), media); err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	if err := a.store.deleteMedia(mediaID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if entry, ok := a.store.entryByID(media.EntryID); ok {
		a.publishAdminSections(entry.ShowID, "entries")
		a.publishShowSummary(entry.ShowID)
		a.respondAdminSectionOrRedirect(w, r, entry.ShowID, "entries")
		return
	}
	redirect(w, r)
}

// --- Persons ---

func (a *app) handleAdminPersons(w http.ResponseWriter, r *http.Request) {
	a.render(w, "admin_persons.html", map[string]any{
		"Title":       "Manage Persons",
		"CurrentPath": "/admin/persons",
		"Persons":     a.store.allPersons(),
	})
}

func (a *app) handleAdminPersonCreate(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	_, err := a.store.createPerson(PersonInput{
		FirstName: r.FormValue("first_name"),
		LastName:  r.FormValue("last_name"),
		Email:     r.FormValue("email"),
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	referer := r.Header.Get("Referer")
	if referer == "" {
		referer = "/admin/persons"
	}
	http.Redirect(w, r, referer, http.StatusSeeOther)
}

// --- Awards ---

func (a *app) handleAdminAwardCreate(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	minEntries, _ := strconv.Atoi(r.FormValue("min_entries"))
	_, err := a.store.createAward(AwardInput{
		OrganizationID: r.FormValue("organization_id"),
		Name:           r.FormValue("name"),
		Description:    r.FormValue("description"),
		Season:         r.FormValue("season"),
		ScoringRule:    r.FormValue("scoring_rule"),
		MinEntries:     minEntries,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	referer := r.Header.Get("Referer")
	if referer == "" {
		referer = "/admin"
	}
	http.Redirect(w, r, referer, http.StatusSeeOther)
}

func (a *app) handleAdminAwardCompute(w http.ResponseWriter, r *http.Request) {
	awardID := r.PathValue("awardID")
	results, err := a.store.computeAward(awardID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, results)
}

// --- Standards & Rules ---

func (a *app) handleAdminStandardCreate(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	_, err := a.store.createStandardDocument(StandardDocument{
		Name:        r.FormValue("name"),
		IssuingOrg:  r.FormValue("issuing_org_id"),
		DomainScope: r.FormValue("domain_scope"),
		Description: r.FormValue("description"),
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if showID := r.FormValue("show_id"); showID != "" {
		a.publishAdminSections(showID, "governance", "schedule", "scoring")
		a.respondAdminSectionOrRedirect(w, r, showID, "governance")
		return
	}
	redirect(w, r)
}

func (a *app) handleAdminEditionCreate(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	pubYear, _ := strconv.Atoi(r.FormValue("publication_year"))
	_, err := a.store.createStandardEdition(StandardEdition{
		StandardDocumentID: r.FormValue("standard_document_id"),
		EditionLabel:       r.FormValue("edition_label"),
		PublicationYear:    pubYear,
		Status:             r.FormValue("status"),
		SourceURL:          r.FormValue("source_url"),
		SourceKind:         r.FormValue("source_kind"),
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if showID := r.FormValue("show_id"); showID != "" {
		a.publishAdminSections(showID, "governance", "schedule", "scoring")
		a.respondAdminSectionOrRedirect(w, r, showID, "governance")
		return
	}
	redirect(w, r)
}

func (a *app) handleAdminSourceCreate(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	_, err := a.store.createSourceDocument(SourceDocument{
		OrganizationID: r.FormValue("organization_id"),
		ShowID:         r.FormValue("show_id"),
		Title:          r.FormValue("title"),
		DocumentType:   r.FormValue("document_type"),
		SourceURL:      r.FormValue("source_url"),
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if showID := r.FormValue("show_id"); showID != "" {
		a.publishAdminSections(showID, "governance", "schedule")
		a.respondAdminSectionOrRedirect(w, r, showID, "governance")
		return
	}
	redirect(w, r)
}

func (a *app) handleAdminCitationCreate(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	targetType := r.FormValue("target_type")
	targetID := r.FormValue("target_id")
	if ref := strings.TrimSpace(r.FormValue("target_ref")); ref != "" {
		parts := strings.SplitN(ref, ":", 2)
		if len(parts) == 2 {
			targetType = parts[0]
			targetID = parts[1]
		}
	}
	confidence, _ := strconv.ParseFloat(r.FormValue("extraction_confidence"), 64)
	_, err := a.store.createSourceCitation(SourceCitation{
		SourceDocumentID:     r.FormValue("source_document_id"),
		TargetType:           targetType,
		TargetID:             targetID,
		PageFrom:             r.FormValue("page_from"),
		PageTo:               r.FormValue("page_to"),
		QuotedText:           r.FormValue("quoted_text"),
		ExtractionConfidence: confidence,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if showID := r.FormValue("show_id"); showID != "" {
		a.publishAdminSections(showID, "governance")
		a.respondAdminSectionOrRedirect(w, r, showID, "governance")
		return
	}
	for _, doc := range a.store.allSourceDocuments() {
		if doc.ID == r.FormValue("source_document_id") && doc.ShowID != "" {
			a.publishAdminSections(doc.ShowID, "governance")
			a.respondAdminSectionOrRedirect(w, r, doc.ShowID, "governance")
			return
		}
	}
	redirect(w, r)
}

func (a *app) handleAdminRuleCreate(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	_, err := a.store.createStandardRule(StandardRule{
		StandardEditionID: r.FormValue("standard_edition_id"),
		Domain:            r.FormValue("domain"),
		RuleType:          r.FormValue("rule_type"),
		SubjectLabel:      r.FormValue("subject_label"),
		Body:              r.FormValue("body"),
		PageRef:           r.FormValue("page_ref"),
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if showID := r.FormValue("show_id"); showID != "" {
		a.publishAdminSections(showID, "governance", "schedule")
		a.respondAdminSectionOrRedirect(w, r, showID, "governance")
		return
	}
	for _, show := range a.store.allShows() {
		if sched, ok := a.store.scheduleByShowID(show.ID); ok && sched.EffectiveStandardEditionID == r.FormValue("standard_edition_id") {
			a.publishAdminSections(show.ID, "governance", "schedule")
			a.respondAdminSectionOrRedirect(w, r, show.ID, "governance")
			return
		}
	}
	redirect(w, r)
}

func (a *app) handleAdminOverrideCreate(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	_, err := a.store.createClassRuleOverride(ClassRuleOverride{
		ShowClassID:        r.FormValue("show_class_id"),
		BaseStandardRuleID: r.FormValue("base_standard_rule_id"),
		OverrideType:       r.FormValue("override_type"),
		Body:               r.FormValue("body"),
		Rationale:          r.FormValue("rationale"),
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if showID := r.FormValue("show_id"); showID != "" {
		a.publishAdminSections(showID, "governance")
		a.respondAdminSectionOrRedirect(w, r, showID, "governance")
		return
	}
	if cls, ok := a.store.classByID(r.FormValue("show_class_id")); ok {
		if showID := showIDForClass(a.store, cls.ID); showID != "" {
			a.publishAdminSections(showID, "governance")
			a.respondAdminSectionOrRedirect(w, r, showID, "governance")
			return
		}
	}
	redirect(w, r)
}

// --- Rubrics & Scoring ---

func (a *app) handleAdminRubricCreate(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	_, err := a.store.createRubric(JudgingRubric{
		StandardEditionID: r.FormValue("standard_edition_id"),
		ShowID:            r.FormValue("show_id"),
		Domain:            r.FormValue("domain"),
		Title:             r.FormValue("title"),
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if showID := r.FormValue("show_id"); showID != "" {
		a.publishAdminSections(showID, "scoring")
		a.respondAdminSectionOrRedirect(w, r, showID, "scoring")
		return
	}
	redirect(w, r)
}

func (a *app) handleAdminCriterionCreate(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	maxPts, _ := strconv.Atoi(r.FormValue("max_points"))
	sortOrd, _ := strconv.Atoi(r.FormValue("sort_order"))
	_, err := a.store.createCriterion(JudgingCriterion{
		JudgingRubricID: r.FormValue("rubric_id"),
		Name:            r.FormValue("name"),
		MaxPoints:       maxPts,
		SortOrder:       sortOrd,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if showID := r.FormValue("show_id"); showID != "" {
		a.publishAdminSections(showID, "scoring")
		a.respondAdminSectionOrRedirect(w, r, showID, "scoring")
		return
	}
	if rubric, ok := a.store.rubricByID(r.FormValue("rubric_id")); ok && rubric.ShowID != "" {
		a.publishAdminSections(rubric.ShowID, "scoring")
		a.respondAdminSectionOrRedirect(w, r, rubric.ShowID, "scoring")
		return
	}
	redirect(w, r)
}

func (a *app) handleAdminScorecardSubmit(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	entryID := r.FormValue("entry_id")
	judgeID := r.FormValue("judge_id")
	rubricID := r.FormValue("rubric_id")
	notes := r.FormValue("notes")
	entry, ok := a.store.entryByID(entryID)
	if !ok {
		http.Error(w, "entry not found", http.StatusBadRequest)
		return
	}
	judgeAllowed := false
	for _, assignment := range a.store.judgesByShow(entry.ShowID) {
		if assignment.PersonID == judgeID {
			judgeAllowed = true
			break
		}
	}
	if !judgeAllowed {
		http.Error(w, "judge is not assigned to this show", http.StatusBadRequest)
		return
	}

	criteria := a.store.criteriaByRubric(rubricID)
	var scores []EntryCriterionScore
	for _, c := range criteria {
		score, _ := strconv.ParseFloat(r.FormValue("score_"+c.ID), 64)
		scores = append(scores, EntryCriterionScore{
			CriterionID: c.ID,
			Score:       score,
		})
	}

	_, err := a.store.submitScorecard(EntryScorecard{
		EntryID:  entryID,
		JudgeID:  judgeID,
		RubricID: rubricID,
		Notes:    notes,
	}, scores)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	a.sseBroker.publish(entry.ShowID, "scorecard-submitted",
		fmt.Sprintf(`<div class="toast">Scorecard submitted for %s</div>`, entry.Name))
	a.publishAdminSections(entry.ShowID, "scoring", "winners", "entries")
	a.publishShowSummary(entry.ShowID)
	a.respondAdminSectionOrRedirect(w, r, entry.ShowID, "scoring")
}

func (a *app) handleAdminComputePlacements(w http.ResponseWriter, r *http.Request) {
	classID := r.PathValue("classID")
	if err := a.store.computePlacementsFromScores(classID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if showID := showIDForClass(a.store, classID); showID != "" {
		a.publishAdminSections(showID, "scoring", "winners", "entries")
		a.publishShowSummary(showID)
		a.respondAdminSectionOrRedirect(w, r, showID, "scoring")
		return
	}
	redirect(w, r)
}

func redirect(w http.ResponseWriter, r *http.Request) {
	referer := r.Header.Get("Referer")
	if referer == "" {
		referer = "/admin"
	}
	http.Redirect(w, r, referer, http.StatusSeeOther)
}
