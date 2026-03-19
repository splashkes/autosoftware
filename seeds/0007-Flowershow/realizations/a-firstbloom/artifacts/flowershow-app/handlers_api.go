package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

// --- Projections ---

func (a *app) handleAPIShowsDirectory(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.store.allShows())
}

func (a *app) handleAPIShowDetail(w http.ResponseWriter, r *http.Request) {
	show, ok := a.apiShowByIdentifier(r.PathValue("id"))
	if !ok {
		a.writeAPIError(w, r, http.StatusNotFound, "show_not_found", "Show not found.", "Use a stable show id or a known slug from the shows directory projection.", nil)
		return
	}
	writeJSON(w, http.StatusOK, a.showDetailProjection(show, false))
}

func (a *app) handleAPIShowWorkspace(w http.ResponseWriter, r *http.Request) {
	if !a.isAdmin(r) && !a.isServiceToken(r) {
		a.writeAPIError(w, r, http.StatusUnauthorized, "unauthorized", "Authentication required.", "Use an admin session or a Bearer service token to inspect the private show workspace.", nil)
		return
	}
	data, err := a.adminShowDetailData(r.PathValue("id"))
	if err != nil {
		a.writeAPIError(w, r, http.StatusNotFound, "show_not_found", "Show workspace not found.", "Use a stable show id from the shows directory projection.", nil)
		return
	}
	writeJSON(w, http.StatusOK, data)
}

func (a *app) handleAPIEntries(w http.ResponseWriter, r *http.Request) {
	showID := r.URL.Query().Get("show_id")
	classID := r.URL.Query().Get("class_id")
	filterVisible := func(items []*Entry) []*Entry {
		var out []*Entry
		for _, entry := range items {
			if isPublicEntry(entry) {
				out = append(out, entry)
			}
		}
		return out
	}
	if classID != "" {
		writeJSON(w, http.StatusOK, filterVisible(a.store.entriesByClass(classID)))
		return
	}
	if showID != "" {
		writeJSON(w, http.StatusOK, filterVisible(a.store.entriesByShow(showID)))
		return
	}
	a.writeAPIError(w, r, http.StatusBadRequest, "missing_query_parameter", "A show_id or class_id query parameter is required.", "Pass show_id to list all public entries for a show, or class_id to list entries for one class.", []apiFieldError{
		{Field: "show_id", Message: "required when class_id is absent"},
		{Field: "class_id", Message: "required when show_id is absent"},
	})
}

func (a *app) handleAPIEntryDetail(w http.ResponseWriter, r *http.Request) {
	entry, ok := a.store.entryByID(r.PathValue("id"))
	if !ok || (!isPublicEntry(entry) && !a.isAdmin(r) && !a.isServiceToken(r)) {
		a.writeAPIError(w, r, http.StatusNotFound, "entry_not_found", "Entry not found.", "Use a stable entry id from an entry list or workspace projection.", nil)
		return
	}

	payload := map[string]any{
		"entry": entry,
		"media": a.store.mediaByEntry(entry.ID),
	}
	if cls, ok := a.store.classByID(entry.ClassID); ok {
		payload["class"] = cls
	}
	if show, ok := a.store.showByID(entry.ShowID); ok {
		payload["show"] = show
	}
	if person, ok := a.store.personByID(entry.PersonID); ok {
		payload["person"] = a.personProjection(person, a.isAdmin(r) || a.isServiceToken(r))
	}
	if a.isAdmin(r) || a.isServiceToken(r) {
		payload["scorecards"] = a.store.scorecardsByEntry(entry.ID)
	}
	writeJSON(w, http.StatusOK, payload)
}

func (a *app) handleAPIClasses(w http.ResponseWriter, r *http.Request) {
	showID := r.URL.Query().Get("show_id")
	sectionID := r.URL.Query().Get("section_id")
	if sectionID != "" {
		writeJSON(w, http.StatusOK, a.store.classesBySection(sectionID))
		return
	}
	if showID != "" {
		writeJSON(w, http.StatusOK, a.store.classesByShowID(showID))
		return
	}
	a.writeAPIError(w, r, http.StatusBadRequest, "missing_query_parameter", "A show_id or section_id query parameter is required.", "Pass show_id to list classes for a show, or section_id to list classes for one section.", []apiFieldError{
		{Field: "show_id", Message: "required when section_id is absent"},
		{Field: "section_id", Message: "required when show_id is absent"},
	})
}

func (a *app) handleAPIClassDetail(w http.ResponseWriter, r *http.Request) {
	cls, ok := a.store.classByID(r.PathValue("id"))
	if !ok {
		a.writeAPIError(w, r, http.StatusNotFound, "class_not_found", "Class not found.", "Use a stable class id from the class list or show workspace projection.", nil)
		return
	}

	var section *Section
	var division *Division
	var schedule *ShowSchedule
	var show *Show

	if current, ok := a.store.sectionByID(cls.SectionID); ok {
		section = current
	}
	if section != nil {
		if current, ok := a.store.divisionByID(section.DivisionID); ok {
			division = current
		}
	}
	if division != nil {
		show, schedule = a.showByScheduleID(division.ShowScheduleID)
	}

	var entries []*Entry
	for _, entry := range a.store.entriesByClass(cls.ID) {
		if isPublicEntry(entry) || a.isAdmin(r) || a.isServiceToken(r) {
			entries = append(entries, entry)
		}
	}

	payload := map[string]any{
		"class":     cls,
		"section":   section,
		"division":  division,
		"schedule":  schedule,
		"show":      show,
		"entries":   entries,
		"citations": a.store.citationsByTarget("show_class", cls.ID),
	}
	if schedule != nil {
		payload["effective_rules"] = a.store.effectiveRulesForClass(cls.ID, schedule.EffectiveStandardEditionID)
	}
	writeJSON(w, http.StatusOK, payload)
}

func (a *app) handleAPITaxonomy(w http.ResponseWriter, r *http.Request) {
	taxonType := r.URL.Query().Get("type")
	if taxonType != "" {
		writeJSON(w, http.StatusOK, a.store.taxonsByType(taxonType))
		return
	}
	writeJSON(w, http.StatusOK, a.store.allTaxons())
}

func (a *app) handleAPILeaderboard(w http.ResponseWriter, r *http.Request) {
	orgID := r.URL.Query().Get("org_id")
	season := r.URL.Query().Get("season")
	if season == "" {
		season = "2025"
	}
	if orgID == "" {
		orgs := a.store.allOrganizations()
		if len(orgs) > 0 {
			orgID = orgs[0].ID
		}
	}
	writeJSON(w, http.StatusOK, a.store.leaderboard(orgID, season))
}

func (a *app) handleAPILedger(w http.ResponseWriter, r *http.Request) {
	if !a.isAdmin(r) && !a.isServiceToken(r) {
		a.writeAPIError(w, r, http.StatusUnauthorized, "unauthorized", "Authentication required.", "Use an admin session or a Bearer service token to inspect ledger history.", nil)
		return
	}
	objectID := r.PathValue("objectID")
	claims, err := a.store.ledgerByObjectID(objectID)
	if err != nil {
		a.writeAPIError(w, r, http.StatusInternalServerError, "ledger_lookup_failed", "Ledger lookup failed.", "Retry with a stable object id from a projection or workspace response.", nil)
		return
	}
	writeJSON(w, http.StatusOK, claims)
}

func (a *app) handleAPIAdminDashboard(w http.ResponseWriter, r *http.Request) {
	if !a.isAdmin(r) && !a.isServiceToken(r) {
		a.writeAPIError(w, r, http.StatusUnauthorized, "unauthorized", "Authentication required.", "Use an admin session or a Bearer service token to inspect the admin dashboard projection.", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"shows":   len(a.store.allShows()),
		"persons": len(a.store.allPersons()),
		"taxons":  len(a.store.allTaxons()),
		"roles":   len(a.store.allUserRoles()),
	})
}

// --- Commands ---

func (a *app) handleAPICommand(w http.ResponseWriter, r *http.Request) {
	if !a.isAdmin(r) && !a.isServiceToken(r) {
		a.writeAPIError(w, r, http.StatusUnauthorized, "unauthorized", "Authentication required.", "Use an admin session or a Bearer service token to execute commands.", nil)
		return
	}

	command := commandNameFromPath(r.URL.Path)

	switch command {
	case "shows.create":
		var input ShowInput
		if !a.decodeAPIJSON(w, r, &input) {
			return
		}
		show, err := a.store.createShow(input)
		if err != nil {
			a.writeAPIError(w, r, http.StatusBadRequest, "show_create_failed", err.Error(), "Provide organization_id, name, and season using the shows.create schema from the contract.", nil)
			return
		}
		writeJSON(w, http.StatusCreated, show)

	case "shows.update":
		var req struct {
			ID string `json:"id"`
			ShowInput
		}
		if !a.decodeAPIJSON(w, r, &req) {
			return
		}
		show, err := a.store.updateShow(req.ID, req.ShowInput)
		if err != nil {
			a.writeAPIError(w, r, http.StatusBadRequest, "show_update_failed", err.Error(), "Pass id plus the fields to update using the shows.update schema from the contract.", []apiFieldError{{Field: "id", Message: "required stable show id"}})
			return
		}
		writeJSON(w, http.StatusOK, show)

	case "schedules.upsert":
		var req ShowSchedule
		if !a.decodeAPIJSON(w, r, &req) {
			return
		}
		if strings.TrimSpace(req.ShowID) == "" {
			a.writeAPIError(w, r, http.StatusBadRequest, "schedule_show_required", "show_id is required.", "Pass the stable show id and any schedule governance fields to upsert schedule state.", []apiFieldError{{Field: "show_id", Message: "required stable show id"}})
			return
		}
		var (
			schedule *ShowSchedule
			err      error
		)
		if _, ok := a.store.scheduleByShowID(req.ShowID); ok {
			schedule, err = a.store.updateSchedule(req.ShowID, ShowSchedule{
				SourceDocumentID:           req.SourceDocumentID,
				EffectiveStandardEditionID: req.EffectiveStandardEditionID,
				Notes:                      req.Notes,
			})
		} else {
			schedule, err = a.store.createSchedule(ShowSchedule{
				ShowID:                     req.ShowID,
				SourceDocumentID:           req.SourceDocumentID,
				EffectiveStandardEditionID: req.EffectiveStandardEditionID,
				Notes:                      req.Notes,
			})
		}
		if err != nil {
			a.writeAPIError(w, r, http.StatusBadRequest, "schedule_upsert_failed", err.Error(), "Ensure the referenced show exists and the standard/source ids are valid for this schedule.", nil)
			return
		}
		writeJSON(w, http.StatusOK, schedule)

	case "judges.assign":
		var req struct {
			ShowID   string `json:"show_id"`
			PersonID string `json:"person_id"`
		}
		if !a.decodeAPIJSON(w, r, &req) {
			return
		}
		assignment, err := a.store.assignJudgeToShow(req.ShowID, req.PersonID)
		if err != nil {
			a.writeAPIError(w, r, http.StatusBadRequest, "judge_assign_failed", err.Error(), "Pass stable show_id and person_id values from projections or workspace responses.", []apiFieldError{
				{Field: "show_id", Message: "required stable show id"},
				{Field: "person_id", Message: "required stable person id"},
			})
			return
		}
		writeJSON(w, http.StatusCreated, assignment)

	case "divisions.create":
		var input DivisionInput
		if !a.decodeAPIJSON(w, r, &input) {
			return
		}
		division, err := a.store.createDivision(input)
		if err != nil {
			a.writeAPIError(w, r, http.StatusBadRequest, "division_create_failed", err.Error(), "Pass a valid show_schedule_id plus title, domain, and sort_order.", nil)
			return
		}
		writeJSON(w, http.StatusCreated, division)

	case "sections.create":
		var input SectionInput
		if !a.decodeAPIJSON(w, r, &input) {
			return
		}
		section, err := a.store.createSection(input)
		if err != nil {
			a.writeAPIError(w, r, http.StatusBadRequest, "section_create_failed", err.Error(), "Pass a valid division_id plus title and sort_order.", nil)
			return
		}
		writeJSON(w, http.StatusCreated, section)

	case "entries.create":
		var input EntryInput
		if !a.decodeAPIJSON(w, r, &input) {
			return
		}
		entry, err := a.store.createEntry(input)
		if err != nil {
			a.writeAPIError(w, r, http.StatusBadRequest, "entry_create_failed", err.Error(), "Pass stable show_id, class_id, and person_id values when creating an entry.", nil)
			return
		}
		writeJSON(w, http.StatusCreated, entry)

	case "entries.update":
		var req struct {
			ID string `json:"id"`
			EntryInput
		}
		if !a.decodeAPIJSON(w, r, &req) {
			return
		}
		entry, err := a.store.updateEntry(req.ID, req.EntryInput)
		if err != nil {
			a.writeAPIError(w, r, http.StatusBadRequest, "entry_update_failed", err.Error(), "Pass id plus any updated entry fields using stable object ids.", []apiFieldError{{Field: "id", Message: "required stable entry id"}})
			return
		}
		writeJSON(w, http.StatusOK, entry)

	case "entries.set_placement":
		var req struct {
			ID        string  `json:"id"`
			Placement int     `json:"placement"`
			Points    float64 `json:"points"`
		}
		if !a.decodeAPIJSON(w, r, &req) {
			return
		}
		if err := a.store.setPlacement(req.ID, req.Placement, req.Points); err != nil {
			a.writeAPIError(w, r, http.StatusBadRequest, "entry_placement_failed", err.Error(), "Pass a stable entry id plus placement and optional points.", []apiFieldError{{Field: "id", Message: "required stable entry id"}})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})

	case "entries.set_visibility":
		var req struct {
			ID         string `json:"id"`
			Suppressed bool   `json:"suppressed"`
		}
		if !a.decodeAPIJSON(w, r, &req) {
			return
		}
		if err := a.store.setEntrySuppressed(req.ID, req.Suppressed); err != nil {
			a.writeAPIError(w, r, http.StatusBadRequest, "entry_visibility_failed", err.Error(), "Pass a stable entry id and the desired suppressed boolean.", []apiFieldError{{Field: "id", Message: "required stable entry id"}})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "suppressed": req.Suppressed})

	case "classes.create":
		var input ShowClassInput
		if !a.decodeAPIJSON(w, r, &input) {
			return
		}
		cls, err := a.store.createClass(input)
		if err != nil {
			a.writeAPIError(w, r, http.StatusBadRequest, "class_create_failed", err.Error(), "Pass a valid section_id plus class_number, title, and domain.", nil)
			return
		}
		writeJSON(w, http.StatusCreated, cls)

	case "classes.compute_placements":
		var req struct {
			ClassID string `json:"class_id"`
		}
		if !a.decodeAPIJSON(w, r, &req) {
			return
		}
		if err := a.store.computePlacementsFromScores(req.ClassID); err != nil {
			a.writeAPIError(w, r, http.StatusBadRequest, "placement_compute_failed", err.Error(), "Pass a stable class id with submitted scorecards before recomputing placements.", []apiFieldError{{Field: "class_id", Message: "required stable class id"}})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})

	case "persons.create":
		var input PersonInput
		if !a.decodeAPIJSON(w, r, &input) {
			return
		}
		person, err := a.store.createPerson(input)
		if err != nil {
			a.writeAPIError(w, r, http.StatusBadRequest, "person_create_failed", err.Error(), "Pass first_name and last_name to register a person record.", nil)
			return
		}
		writeJSON(w, http.StatusCreated, person)

	case "awards.create":
		var input AwardInput
		if !a.decodeAPIJSON(w, r, &input) {
			return
		}
		award, err := a.store.createAward(input)
		if err != nil {
			a.writeAPIError(w, r, http.StatusBadRequest, "award_create_failed", err.Error(), "Pass organization_id, name, season, and scoring_rule to define an award.", nil)
			return
		}
		writeJSON(w, http.StatusCreated, award)

	case "awards.compute":
		var req struct {
			AwardID string `json:"award_id"`
		}
		if !a.decodeAPIJSON(w, r, &req) {
			return
		}
		results, err := a.store.computeAward(req.AwardID)
		if err != nil {
			a.writeAPIError(w, r, http.StatusBadRequest, "award_compute_failed", err.Error(), "Pass a stable award_id and ensure there are matching entries to score.", []apiFieldError{{Field: "award_id", Message: "required stable award id"}})
			return
		}
		writeJSON(w, http.StatusOK, results)

	case "taxons.create":
		var input TaxonInput
		if !a.decodeAPIJSON(w, r, &input) {
			return
		}
		taxon, err := a.store.createTaxon(input)
		if err != nil {
			a.writeAPIError(w, r, http.StatusBadRequest, "taxon_create_failed", err.Error(), "Pass taxon_type and name to add a taxonomy node.", nil)
			return
		}
		writeJSON(w, http.StatusCreated, taxon)

	case "media.attach":
		var input Media
		if !a.decodeAPIJSON(w, r, &input) {
			return
		}
		if _, ok := a.store.entryByID(input.EntryID); !ok {
			a.writeAPIError(w, r, http.StatusBadRequest, "media_entry_missing", "entry_id must reference an existing entry.", "Use a stable entry id from an entry list or workspace projection before attaching media metadata.", []apiFieldError{{Field: "entry_id", Message: "must reference an existing entry"}})
			return
		}
		media, err := a.store.attachMedia(input)
		if err != nil {
			a.writeAPIError(w, r, http.StatusBadRequest, "media_attach_failed", err.Error(), "Pass entry_id, media_type, url, and file_name when attaching media metadata.", nil)
			return
		}
		writeJSON(w, http.StatusCreated, media)

	case "media.delete":
		var req struct {
			MediaID string `json:"media_id"`
		}
		if !a.decodeAPIJSON(w, r, &req) {
			return
		}
		if err := a.store.deleteMedia(req.MediaID); err != nil {
			a.writeAPIError(w, r, http.StatusBadRequest, "media_delete_failed", err.Error(), "Pass a stable media_id from an entry detail or workspace projection.", []apiFieldError{{Field: "media_id", Message: "required stable media id"}})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})

	case "rubrics.create":
		var input JudgingRubric
		if !a.decodeAPIJSON(w, r, &input) {
			return
		}
		rubric, err := a.store.createRubric(input)
		if err != nil {
			a.writeAPIError(w, r, http.StatusBadRequest, "rubric_create_failed", err.Error(), "Pass domain and title, plus optional show or standard linkage.", nil)
			return
		}
		writeJSON(w, http.StatusCreated, rubric)

	case "criteria.create":
		var input JudgingCriterion
		if !a.decodeAPIJSON(w, r, &input) {
			return
		}
		criterion, err := a.store.createCriterion(input)
		if err != nil {
			a.writeAPIError(w, r, http.StatusBadRequest, "criterion_create_failed", err.Error(), "Pass judging_rubric_id, name, max_points, and sort_order.", nil)
			return
		}
		writeJSON(w, http.StatusCreated, criterion)

	case "scorecards.submit":
		var req struct {
			EntryID  string                `json:"entry_id"`
			JudgeID  string                `json:"judge_id"`
			RubricID string                `json:"rubric_id"`
			Notes    string                `json:"notes"`
			Scores   []EntryCriterionScore `json:"scores"`
		}
		if !a.decodeAPIJSON(w, r, &req) {
			return
		}
		sc, err := a.store.submitScorecard(EntryScorecard{
			EntryID:  req.EntryID,
			JudgeID:  req.JudgeID,
			RubricID: req.RubricID,
			Notes:    req.Notes,
		}, req.Scores)
		if err != nil {
			a.writeAPIError(w, r, http.StatusBadRequest, "scorecard_submit_failed", err.Error(), "Pass stable entry_id, judge_id, rubric_id, and per-criterion scores.", nil)
			return
		}
		writeJSON(w, http.StatusCreated, sc)

	case "standards.create":
		var input StandardDocument
		if !a.decodeAPIJSON(w, r, &input) {
			return
		}
		doc, err := a.store.createStandardDocument(input)
		if err != nil {
			a.writeAPIError(w, r, http.StatusBadRequest, "standard_create_failed", err.Error(), "Pass name, issuing_org_id, and domain_scope for a standard document.", nil)
			return
		}
		writeJSON(w, http.StatusCreated, doc)

	case "editions.create":
		var input StandardEdition
		if !a.decodeAPIJSON(w, r, &input) {
			return
		}
		ed, err := a.store.createStandardEdition(input)
		if err != nil {
			a.writeAPIError(w, r, http.StatusBadRequest, "edition_create_failed", err.Error(), "Pass standard_document_id and edition metadata when creating an edition.", nil)
			return
		}
		writeJSON(w, http.StatusCreated, ed)

	case "sources.create":
		var input SourceDocument
		if !a.decodeAPIJSON(w, r, &input) {
			return
		}
		doc, err := a.store.createSourceDocument(input)
		if err != nil {
			a.writeAPIError(w, r, http.StatusBadRequest, "source_create_failed", err.Error(), "Pass title, document_type, and any linked organization_id or show_id for a source document.", nil)
			return
		}
		writeJSON(w, http.StatusCreated, doc)

	case "citations.create":
		var input SourceCitation
		if !a.decodeAPIJSON(w, r, &input) {
			return
		}
		cite, err := a.store.createSourceCitation(input)
		if err != nil {
			a.writeAPIError(w, r, http.StatusBadRequest, "citation_create_failed", err.Error(), "Pass source_document_id plus target_type and target_id when creating a citation.", nil)
			return
		}
		writeJSON(w, http.StatusCreated, cite)

	case "ingestions.import":
		var input ingestionImportRequest
		if !a.decodeAPIJSON(w, r, &input) {
			return
		}
		doc, citations, err := a.importIngestion(input, input.SourceDocument.ShowID)
		if err != nil {
			a.writeAPIError(w, r, http.StatusBadRequest, "ingestion_import_failed", err.Error(), "This command imports pre-structured cited data, not raw PDF bytes.", nil)
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{
			"source_document": doc,
			"citations":       citations,
		})

	case "rules.create":
		var input StandardRule
		if !a.decodeAPIJSON(w, r, &input) {
			return
		}
		rule, err := a.store.createStandardRule(input)
		if err != nil {
			a.writeAPIError(w, r, http.StatusBadRequest, "rule_create_failed", err.Error(), "Pass standard_edition_id, domain, rule_type, subject_label, and body for a rule.", nil)
			return
		}
		writeJSON(w, http.StatusCreated, rule)

	case "overrides.create":
		var input ClassRuleOverride
		if !a.decodeAPIJSON(w, r, &input) {
			return
		}
		override, err := a.store.createClassRuleOverride(input)
		if err != nil {
			a.writeAPIError(w, r, http.StatusBadRequest, "override_create_failed", err.Error(), "Pass show_class_id, override_type, and body when creating a local rule override.", nil)
			return
		}
		writeJSON(w, http.StatusCreated, override)

	case "roles.assign":
		var input UserRoleInput
		if !a.decodeAPIJSON(w, r, &input) {
			return
		}
		role, err := a.store.assignUserRole(input)
		if err != nil {
			a.writeAPIError(w, r, http.StatusBadRequest, "role_assign_failed", err.Error(), "Pass cognito_sub and role, plus optional organization_id or show_id scope.", nil)
			return
		}
		writeJSON(w, http.StatusCreated, role)

	default:
		a.writeAPIError(w, r, http.StatusNotFound, "unknown_command", "Unknown command.", "Inspect the contract detail endpoint for supported command names in this realization.", []apiFieldError{{Field: "command", Message: command}})
	}
}

func (a *app) decodeAPIJSON(w http.ResponseWriter, r *http.Request, dest any) bool {
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		a.writeAPIError(w, r, http.StatusBadRequest, "invalid_json", "Request body must be valid JSON matching the command schema.", "Check the contract schema, property names, and JSON syntax before retrying.", nil)
		return false
	}
	payload, ok := a.unwrapRuntimeContextEnvelope(w, r, raw)
	if !ok {
		return false
	}
	dec := json.NewDecoder(bytes.NewReader(payload))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dest); err != nil {
		a.writeAPIError(w, r, http.StatusBadRequest, "invalid_json", "Request body must be valid JSON matching the command schema.", "Check the contract schema, property names, and JSON syntax before retrying.", nil)
		return false
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		a.writeAPIError(w, r, http.StatusBadRequest, "invalid_json", "Request body must contain exactly one JSON value.", "Remove trailing content after the JSON object and retry.", nil)
		return false
	}
	return true
}

func (a *app) unwrapRuntimeContextEnvelope(w http.ResponseWriter, r *http.Request, raw []byte) ([]byte, bool) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		a.writeAPIError(w, r, http.StatusBadRequest, "invalid_json", "Request body must be valid JSON matching the command schema.", "Check the contract schema, property names, and JSON syntax before retrying.", nil)
		return nil, false
	}

	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &envelope); err != nil {
		return trimmed, true
	}

	inputRaw, wrapped := envelope["input"]
	if !wrapped {
		return trimmed, true
	}

	for key := range envelope {
		if key != "input" && key != "runtime_context" {
			a.writeAPIError(w, r, http.StatusBadRequest, "invalid_json", "Wrapped command payloads may only include input and runtime_context.", "Move command fields into input and keep prompt-like guidance inside runtime_context.", []apiFieldError{{Field: key, Message: "unexpected top-level field"}})
			return nil, false
		}
	}

	if len(bytes.TrimSpace(inputRaw)) == 0 || bytes.Equal(bytes.TrimSpace(inputRaw), []byte("null")) {
		a.writeAPIError(w, r, http.StatusBadRequest, "invalid_json", "Wrapped command payloads require a non-null input object.", "Provide command fields inside the input object and retry.", []apiFieldError{{Field: "input", Message: "required command payload"}})
		return nil, false
	}

	if runtimeRaw, ok := envelope["runtime_context"]; ok && !bytes.Equal(bytes.TrimSpace(runtimeRaw), []byte("null")) {
		var runtimeContext map[string]any
		dec := json.NewDecoder(bytes.NewReader(runtimeRaw))
		if err := dec.Decode(&runtimeContext); err != nil {
			a.writeAPIError(w, r, http.StatusBadRequest, "invalid_json", "runtime_context must be a JSON object when provided.", "Pass prompt-like authoring guidance as a JSON object or omit runtime_context.", []apiFieldError{{Field: "runtime_context", Message: "must be a JSON object"}})
			return nil, false
		}
		if err := dec.Decode(&struct{}{}); err != io.EOF {
			a.writeAPIError(w, r, http.StatusBadRequest, "invalid_json", "runtime_context must contain exactly one JSON object.", "Remove trailing content from runtime_context and retry.", []apiFieldError{{Field: "runtime_context", Message: "must contain exactly one JSON object"}})
			return nil, false
		}
	}

	return inputRaw, true
}

func commandNameFromPath(path string) string {
	parts := strings.Split(path, "/")
	return parts[len(parts)-1]
}

func (a *app) apiShowByIdentifier(id string) (*Show, bool) {
	show, ok := a.store.showByID(id)
	if ok {
		return show, true
	}
	return a.store.showBySlug(id)
}

func (a *app) showDetailProjection(show *Show, includePrivate bool) map[string]any {
	org, _ := a.store.organizationByID(show.OrganizationID)
	schedule, divisions := a.divisionViewsForShow(show.ID)
	payload := map[string]any{
		"show":         show,
		"organization": org,
		"schedule":     schedule,
		"divisions":    divisions,
		"entries":      a.entryViewsForShow(show.ID, includePrivate),
		"awards":       a.store.awardsByOrganization(show.OrganizationID),
	}
	if includePrivate {
		payload["judges"] = a.judgeViewsForShow(show.ID)
		payload["sources"] = a.store.allSourceDocuments()
		payload["rubric_views"] = a.rubricViewsForShow(show.ID)
		payload["class_rule_views"] = a.classRuleViews(show.ID, func() string {
			if schedule == nil {
				return ""
			}
			return schedule.EffectiveStandardEditionID
		}())
	}
	return payload
}

func (a *app) divisionViewsForShow(showID string) (*ShowSchedule, []*divisionView) {
	schedule, _ := a.store.scheduleByShowID(showID)
	if schedule == nil {
		return nil, nil
	}
	var divisions []*divisionView
	for _, div := range a.store.divisionsBySchedule(schedule.ID) {
		dv := &divisionView{Division: div}
		for _, sec := range a.store.sectionsByDivision(div.ID) {
			dv.Sections = append(dv.Sections, &sectionView{
				Section: sec,
				Classes: a.store.classesBySection(sec.ID),
			})
		}
		divisions = append(divisions, dv)
	}
	return schedule, divisions
}

func (a *app) entryViewsForShow(showID string, includePrivate bool) []*entryView {
	var out []*entryView
	for _, entry := range a.store.entriesByShow(showID) {
		if !includePrivate && !isPublicEntry(entry) {
			continue
		}
		person, _ := a.store.personByID(entry.PersonID)
		if person != nil && !includePrivate {
			masked := *person
			masked.FirstName = ""
			masked.LastName = ""
			masked.Email = ""
			person = &masked
		}
		class, _ := a.store.classByID(entry.ClassID)
		out = append(out, &entryView{
			Entry:  entry,
			Person: person,
			Class:  class,
			Media:  a.store.mediaByEntry(entry.ID),
		})
	}
	return out
}

func (a *app) showByScheduleID(scheduleID string) (*Show, *ShowSchedule) {
	for _, show := range a.store.allShows() {
		schedule, ok := a.store.scheduleByShowID(show.ID)
		if ok && schedule.ID == scheduleID {
			return show, schedule
		}
	}
	return nil, nil
}

func (a *app) personProjection(person *Person, includePrivate bool) map[string]any {
	payload := map[string]any{
		"id":       person.ID,
		"initials": person.Initials,
	}
	if includePrivate {
		payload["first_name"] = person.FirstName
		payload["last_name"] = person.LastName
		payload["email"] = person.Email
	}
	return payload
}
