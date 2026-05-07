package main

import (
	"net/http"
	"strconv"
	"strings"
	"time"
)

// People roster CTAs (above the intake grid) — three admin form-post handlers
// plus a JSON search endpoint. These re-use the existing store methods
// (createPerson, updatePerson, assignJudgeToShow) and the runtime authority
// AssignRole path; no new contract commands are introduced.

type peopleRosterSearchResult struct {
	ID             string `json:"id"`
	FirstName      string `json:"first_name"`
	LastName       string `json:"last_name"`
	Email          string `json:"email,omitempty"`
	Phone          string `json:"phone,omitempty"`
	IsJudge        bool   `json:"is_judge"`
	Qualifications string `json:"qualifications,omitempty"`
	Specialties    string `json:"specialties,omitempty"`
}

// handlePeopleRosterSearch returns up to 12 person matches for an admin-side
// autocomplete used by the Add Judge / Add Helper / Add Member modals.
func (a *app) handlePeopleRosterSearch(w http.ResponseWriter, r *http.Request) {
	q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	out := struct {
		Results []peopleRosterSearchResult `json:"results"`
	}{Results: []peopleRosterSearchResult{}}
	if q == "" {
		writeJSON(w, http.StatusOK, out)
		return
	}
	for _, person := range a.store.allPersons() {
		if person == nil {
			continue
		}
		fullName := strings.ToLower(strings.TrimSpace(person.FirstName + " " + person.LastName))
		email := strings.ToLower(strings.TrimSpace(person.Email))
		if !strings.Contains(fullName, q) && (email == "" || !strings.Contains(email, q)) {
			continue
		}
		out.Results = append(out.Results, peopleRosterSearchResult{
			ID:             person.ID,
			FirstName:      person.FirstName,
			LastName:       person.LastName,
			Email:          person.Email,
			Phone:          person.Phone,
			IsJudge:        person.IsJudge,
			Qualifications: person.Qualifications,
			Specialties:    person.Specialties,
		})
		if len(out.Results) >= 12 {
			break
		}
	}
	writeJSON(w, http.StatusOK, out)
}

// peopleRosterResolveOrCreate resolves a person from a posted form. If
// person_id is set and refers to an existing person, that person is returned
// (and the optional follow-up update fields are applied). Otherwise a new
// person is created from the form fields. firstName + lastName must be
// present when no person_id is provided.
func (a *app) peopleRosterResolveOrCreate(r *http.Request, makeJudge bool) (*Person, error) {
	personID := strings.TrimSpace(r.FormValue("person_id"))
	firstName := strings.TrimSpace(r.FormValue("first_name"))
	lastName := strings.TrimSpace(r.FormValue("last_name"))
	email := strings.TrimSpace(r.FormValue("email"))
	phone := strings.TrimSpace(r.FormValue("phone"))
	qualifications := strings.TrimSpace(r.FormValue("qualifications"))
	specialties := strings.TrimSpace(r.FormValue("specialties"))

	yearsRaw := strings.TrimSpace(r.FormValue("years_in_flower_shows"))
	judgingStartedYear := 0
	if makeJudge && yearsRaw != "" {
		years, err := strconv.Atoi(yearsRaw)
		if err == nil && years > 0 {
			judgingStartedYear = time.Now().UTC().Year() - years
		}
	}

	if personID != "" {
		existing, ok := a.store.personByID(personID)
		if !ok {
			return nil, errPersonRosterMissing
		}
		// Optional incremental updates: judge promotion, qualifications,
		// years experience. Don't clobber unrelated fields.
		needsUpdate := false
		next := PersonInput{
			FirstName:          existing.FirstName,
			LastName:           existing.LastName,
			Email:              existing.Email,
			Phone:              existing.Phone,
			Specialties:        existing.Specialties,
			Qualifications:     existing.Qualifications,
			Notes:              existing.Notes,
			IsJudge:            existing.IsJudge,
			JudgingStartedYear: existing.JudgingStartedYear,
			PublicDisplayMode:  existing.PublicDisplayMode,
		}
		if makeJudge && !existing.IsJudge {
			next.IsJudge = true
			needsUpdate = true
		}
		if makeJudge && qualifications != "" && qualifications != existing.Qualifications {
			next.Qualifications = qualifications
			needsUpdate = true
		}
		if makeJudge && judgingStartedYear != 0 && judgingStartedYear != existing.JudgingStartedYear {
			next.JudgingStartedYear = judgingStartedYear
			needsUpdate = true
		}
		if needsUpdate {
			updated, err := a.store.updatePerson(existing.ID, next)
			if err != nil {
				return nil, err
			}
			return updated, nil
		}
		return existing, nil
	}

	if firstName == "" || lastName == "" {
		return nil, errPersonRosterNameRequired
	}
	person, err := a.store.createPerson(PersonInput{
		FirstName:          firstName,
		LastName:           lastName,
		Email:              email,
		Phone:              phone,
		Qualifications:     qualifications,
		Specialties:        specialties,
		IsJudge:            makeJudge,
		JudgingStartedYear: judgingStartedYear,
	})
	if err != nil {
		return nil, err
	}
	return person, nil
}

var (
	errPersonRosterMissing      = &peopleRosterError{Status: http.StatusNotFound, Msg: "selected person not found"}
	errPersonRosterNameRequired = &peopleRosterError{Status: http.StatusBadRequest, Msg: "first name and last name are required"}
)

type peopleRosterError struct {
	Status int
	Msg    string
}

func (e *peopleRosterError) Error() string { return e.Msg }

func writePeopleRosterError(w http.ResponseWriter, err error) {
	if rerr, ok := err.(*peopleRosterError); ok {
		http.Error(w, rerr.Msg, rerr.Status)
		return
	}
	http.Error(w, err.Error(), http.StatusBadRequest)
}

// handleAdminAddJudgeForm — POST /admin/shows/{showID}/people/judge
func (a *app) handleAdminAddJudgeForm(w http.ResponseWriter, r *http.Request) {
	showID := r.PathValue("showID")
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	person, err := a.peopleRosterResolveOrCreate(r, true)
	if err != nil {
		writePeopleRosterError(w, err)
		return
	}
	if _, err := a.store.assignJudgeToShow(showID, person.ID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	a.sseBroker.publish(showID, "show-updated", `<div class="toast">Judge added</div>`)
	a.publishAdminSections(showID, "intake", "setup", "scoring")
	a.respondAdminSectionOrRedirect(w, r, showID, "intake")
}

// handleAdminAddShowHelperAdminForm — POST /admin/shows/{showID}/people/show-admin
// Grants the flowershow_show_intake_operator runtime bundle scoped to this show.
func (a *app) handleAdminAddShowHelperAdminForm(w http.ResponseWriter, r *http.Request) {
	showID := r.PathValue("showID")
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	person, err := a.peopleRosterResolveOrCreate(r, false)
	if err != nil {
		writePeopleRosterError(w, err)
		return
	}
	if err := a.assignPersonRoleForShow(r, person, showID, "show_intake_operator"); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	a.sseBroker.publish(showID, "show-updated", `<div class="toast">Show helper admin added</div>`)
	a.publishAdminSections(showID, "intake", "setup", "governance")
	a.respondAdminSectionOrRedirect(w, r, showID, "intake")
}

// handleAdminAddMemberEntrantForm — POST /admin/shows/{showID}/people/member
// Posts with role=flowershow_entrant or role=flowershow_show_intake_operator.
func (a *app) handleAdminAddMemberEntrantForm(w http.ResponseWriter, r *http.Request) {
	showID := r.PathValue("showID")
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	roleRaw := strings.TrimSpace(r.FormValue("role"))
	roleKey, ok := normalizePeopleRosterRole(roleRaw)
	if !ok {
		http.Error(w, "unsupported role", http.StatusBadRequest)
		return
	}
	person, err := a.peopleRosterResolveOrCreate(r, false)
	if err != nil {
		writePeopleRosterError(w, err)
		return
	}
	if err := a.assignPersonRoleForShow(r, person, showID, roleKey); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	a.sseBroker.publish(showID, "show-updated", `<div class="toast">Member added</div>`)
	a.publishAdminSections(showID, "intake", "setup", "governance")
	a.respondAdminSectionOrRedirect(w, r, showID, "intake")
}

// normalizePeopleRosterRole accepts both the bundle id (flowershow_*) and the
// short bundle key (entrant / show_intake_operator) and returns the bundle key
// understood by AssignRole. Returns ok=false for unsupported values.
func normalizePeopleRosterRole(value string) (string, bool) {
	v := strings.TrimSpace(value)
	switch v {
	case "flowershow_entrant", "entrant":
		return "entrant", true
	case "flowershow_show_intake_operator", "show_intake_operator":
		return "show_intake_operator", true
	default:
		return "", false
	}
}

// assignPersonRoleForShow grants a runtime authority bundle for a person
// scoped to the given show. The grantor is the currently signed-in admin (if
// any). The memory authority resolver only requires a SubjectID; the postgres
// resolver additionally needs the target user to have signed in at least once
// (it dereferences via cognito_sub). For the admin-add-ahead-of-time flow the
// person id is used as a subject id placeholder so memory tests round-trip
// cleanly; in production the grant is finalized when the invited user signs
// in (see runtime authority migration path).
func (a *app) assignPersonRoleForShow(r *http.Request, person *Person, showID, roleKey string) error {
	if a.authority == nil {
		return errPersonRosterMissing
	}
	grantor := ""
	if user, ok := a.currentUser(r); ok && user != nil {
		grantor = user.SubjectID
	}
	cognitoSub := strings.TrimSpace(person.Email)
	subjectID := strings.TrimSpace(person.ID)
	_, err := a.authority.AssignRole(r.Context(), UserRoleInput{
		SubjectID:  subjectID,
		CognitoSub: cognitoSub,
		ShowID:     showID,
		Role:       roleKey,
	}, grantor)
	return err
}
