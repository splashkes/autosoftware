package main

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

// --- Projections (Public) ---

func (a *app) handleAPIShowsDirectory(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.store.allShows())
}

func (a *app) handleAPIShowDetail(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	show, ok := a.store.showByID(id)
	if !ok {
		show, ok = a.store.showBySlug(id)
	}
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "show not found"})
		return
	}
	writeJSON(w, http.StatusOK, show)
}

func (a *app) handleAPIEntries(w http.ResponseWriter, r *http.Request) {
	showID := r.URL.Query().Get("show_id")
	classID := r.URL.Query().Get("class_id")
	if classID != "" {
		writeJSON(w, http.StatusOK, a.store.entriesByClass(classID))
		return
	}
	if showID != "" {
		writeJSON(w, http.StatusOK, a.store.entriesByShow(showID))
		return
	}
	writeJSON(w, http.StatusBadRequest, map[string]string{"error": "show_id or class_id required"})
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
	writeJSON(w, http.StatusBadRequest, map[string]string{"error": "show_id or section_id required"})
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
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	objectID := r.PathValue("objectID")
	claims, err := a.store.ledgerByObjectID(objectID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, claims)
}

func (a *app) handleAPIAdminDashboard(w http.ResponseWriter, r *http.Request) {
	if !a.isAdmin(r) && !a.isServiceToken(r) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"shows":   len(a.store.allShows()),
		"persons": len(a.store.allPersons()),
		"taxons":  len(a.store.allTaxons()),
	})
}

// --- Commands ---

func (a *app) handleAPICommand(w http.ResponseWriter, r *http.Request) {
	if !a.isAdmin(r) && !a.isServiceToken(r) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	// Extract command name from path
	path := r.URL.Path
	parts := strings.Split(path, "/")
	command := parts[len(parts)-1]

	switch command {
	case "shows.create":
		var input ShowInput
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		show, err := a.store.createShow(input)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, show)

	case "shows.update":
		var req struct {
			ID string `json:"id"`
			ShowInput
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		show, err := a.store.updateShow(req.ID, req.ShowInput)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, show)

	case "entries.create":
		var input EntryInput
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		entry, err := a.store.createEntry(input)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, entry)

	case "entries.update":
		var req struct {
			ID string `json:"id"`
			EntryInput
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		entry, err := a.store.updateEntry(req.ID, req.EntryInput)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, entry)

	case "entries.set_placement":
		var req struct {
			ID        string  `json:"id"`
			Placement int     `json:"placement"`
			Points    float64 `json:"points"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		if err := a.store.setPlacement(req.ID, req.Placement, req.Points); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})

	case "classes.create":
		var input ShowClassInput
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		cls, err := a.store.createClass(input)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, cls)

	case "persons.create":
		var input PersonInput
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		person, err := a.store.createPerson(input)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, person)

	case "awards.create":
		var input AwardInput
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		award, err := a.store.createAward(input)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, award)

	case "awards.compute":
		var req struct {
			AwardID string `json:"award_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		results, err := a.store.computeAward(req.AwardID)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, results)

	case "taxons.create":
		var input TaxonInput
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		taxon, err := a.store.createTaxon(input)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, taxon)

	case "rubrics.create":
		var input JudgingRubric
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		rubric, err := a.store.createRubric(input)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, rubric)

	case "criteria.create":
		var input JudgingCriterion
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		criterion, err := a.store.createCriterion(input)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
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
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		sc, err := a.store.submitScorecard(EntryScorecard{
			EntryID:  req.EntryID,
			JudgeID:  req.JudgeID,
			RubricID: req.RubricID,
			Notes:    req.Notes,
		}, req.Scores)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, sc)

	case "standards.create":
		var input StandardDocument
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		doc, err := a.store.createStandardDocument(input)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, doc)

	case "editions.create":
		var input StandardEdition
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		ed, err := a.store.createStandardEdition(input)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, ed)

	case "sources.create":
		var input SourceDocument
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		doc, err := a.store.createSourceDocument(input)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, doc)

	case "citations.create":
		var input SourceCitation
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		cite, err := a.store.createSourceCitation(input)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, cite)

	case "rules.create":
		var input StandardRule
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		rule, err := a.store.createStandardRule(input)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, rule)

	case "overrides.create":
		var input ClassRuleOverride
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		override, err := a.store.createClassRuleOverride(input)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, override)

	default:
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "unknown command: " + command})
	}
}

// Suppress unused import
var _ = strconv.Atoi
