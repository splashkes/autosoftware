package main

import (
	"net/http"
	"net/url"
	"strings"
	"time"
)

// --- View data ---

type publicShowHelpLandingData struct {
	Title       string
	CurrentPath string
	Show        *Show
	Org         *Organization
	HasToken    bool
}

type publicShowHelpRedeemData struct {
	Title          string
	CurrentPath    string
	Show           *Show
	Org            *Organization
	Token          string
	Next           string
	Error          string
	Notice         string
	PrefillName    string
	PrefillEmail   string
	PersonOptions  []*personLookupView
	IsExhibitor    bool
	MatchedPerson  string
	TokenFromQuery bool
}

type adminShowHelpersData struct {
	Title         string
	CurrentPath   string
	Show          *Show
	Org           *Organization
	Invites       []adminShowHelperInviteView
	BadgeSessions []adminShowBadgeSessionView
	Notice        string
	Error         string
}

type adminShowHelperInviteView struct {
	ID          string
	Label       string
	CreatedBy   string
	CreatedAt   time.Time
	ExpiresAt   time.Time
	Active      bool
	StatusLabel string
	RevokedAt   *time.Time
}

type adminShowBadgeSessionView struct {
	ID            string
	Name          string
	Email         string
	MatchedPerson string
	CreatedAt     time.Time
	LastSeenAt    time.Time
	ExpiresAt     time.Time
	Active        bool
	StatusLabel   string
}

type adminShowHelperCreatedData struct {
	Title       string
	CurrentPath string
	Show        *Show
	Org         *Organization
	ShareURL    string
	Token       string
	Invite      *ShowHelperInvite
}

// --- Public routes ---

// handlePublicShowHelpLanding renders /shows/{slug}/help. Public, no auth.
// If `?token=...` is present, redirect to /help-redeem so the same token
// works whether the admin shared the bare /help URL or the full URL with
// the token in the query.
func (a *app) handlePublicShowHelpLanding(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	show, ok := a.store.showBySlug(slug)
	if !ok {
		http.NotFound(w, r)
		return
	}

	token := strings.TrimSpace(r.URL.Query().Get("token"))
	if token != "" {
		target := "/shows/" + slug + "/help-redeem?token=" + url.QueryEscape(token)
		http.Redirect(w, r, target, http.StatusSeeOther)
		return
	}

	org, _ := a.store.organizationByID(show.OrganizationID)
	a.render(w, r, "public_show_help_landing.html", publicShowHelpLandingData{
		Title:       "Help Out · " + show.Name,
		CurrentPath: "/shows/" + slug + "/help",
		Show:        show,
		Org:         org,
		HasToken:    false,
	})
}

// handlePublicShowHelpRedeem renders the GET form on /help-redeem and also
// handles the POST that creates the badge session.
func (a *app) handlePublicShowHelpRedeem(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	show, ok := a.store.showBySlug(slug)
	if !ok {
		http.NotFound(w, r)
		return
	}
	org, _ := a.store.organizationByID(show.OrganizationID)

	if r.Method == http.MethodPost {
		a.processShowHelpRedeem(w, r, show, org)
		return
	}

	tokenFromQuery := strings.TrimSpace(r.URL.Query().Get("token"))
	a.render(w, r, "public_show_help_redeem.html", publicShowHelpRedeemData{
		Title:          "Join " + show.Name,
		CurrentPath:    "/shows/" + slug + "/help-redeem",
		Show:           show,
		Org:            org,
		Token:          tokenFromQuery,
		Next:           strings.TrimSpace(r.URL.Query().Get("next")),
		PersonOptions:  a.personLookupViewsForShow(show.ID, ""),
		TokenFromQuery: tokenFromQuery != "",
	})
}

func (a *app) processShowHelpRedeem(w http.ResponseWriter, r *http.Request, show *Show, org *Organization) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	token := strings.TrimSpace(r.FormValue("token"))
	name := strings.TrimSpace(r.FormValue("name"))
	email := strings.TrimSpace(r.FormValue("email"))
	role := strings.TrimSpace(r.FormValue("role"))
	matchedPersonID := strings.TrimSpace(r.FormValue("matched_person_id"))
	next := strings.TrimSpace(r.FormValue("next"))

	renderError := func(message string) {
		a.render(w, r, "public_show_help_redeem.html", publicShowHelpRedeemData{
			Title:          "Join " + show.Name,
			CurrentPath:    "/shows/" + show.Slug + "/help-redeem",
			Show:           show,
			Org:            org,
			Token:          token,
			Next:           next,
			PrefillName:    name,
			PrefillEmail:   email,
			IsExhibitor:    role == "exhibitor",
			MatchedPerson:  matchedPersonID,
			PersonOptions:  a.personLookupViewsForShow(show.ID, ""),
			TokenFromQuery: false,
			Error:          message,
		})
	}

	if token == "" {
		w.WriteHeader(http.StatusBadRequest)
		renderError("Paste the share link or token from your show admin to continue.")
		return
	}
	if name == "" || email == "" {
		w.WriteHeader(http.StatusBadRequest)
		renderError("Both your name and email are required so admins can recognize you.")
		return
	}

	invite, ok := a.store.findShowHelperInviteByToken(token)
	if !ok || invite == nil || invite.ShowID != show.ID {
		w.WriteHeader(http.StatusBadRequest)
		renderError("That share link is no longer active. Ask an admin for a fresh one.")
		return
	}

	if role != "exhibitor" {
		matchedPersonID = ""
	}

	issued, err := a.store.createShowBadgeSession(ShowBadgeSessionInput{
		ShowID:          show.ID,
		InviteID:        invite.ID,
		Name:            name,
		Email:           email,
		MatchedPersonID: matchedPersonID,
	})
	if err != nil || issued == nil || issued.Session == nil {
		w.WriteHeader(http.StatusBadRequest)
		renderError("We could not finish setting up your helper badge. Try again or ask an admin.")
		return
	}

	a.setShowBadgeCookie(w, r, issued.Token, issued.Session.ExpiresAt)

	target := "/shows/" + show.Slug + "?notice=" + url.QueryEscape("You're set up. Find an admin to start adding photos.")
	if next != "" {
		decoded, err := url.QueryUnescape(next)
		if err == nil && strings.HasPrefix(decoded, "/shows/"+show.Slug) {
			target = decoded
		}
	}
	http.Redirect(w, r, target, http.StatusSeeOther)
}

// handlePublicShowHelpEnd lets a helper voluntarily end their badge session.
// POST only — clears the cookie and revokes the session record.
func (a *app) handlePublicShowHelpEnd(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	show, ok := a.store.showBySlug(slug)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if session, ok := a.currentShowBadge(r); ok && session != nil && session.ShowID == show.ID {
		_ = a.store.revokeShowBadgeSession(session.ID)
	}
	a.clearShowBadgeCookie(w, r)
	target := "/shows/" + show.Slug + "/help?notice=" + url.QueryEscape("You're signed out as a helper.")
	http.Redirect(w, r, target, http.StatusSeeOther)
}

// --- Admin routes ---

func (a *app) handleAdminShowHelpers(w http.ResponseWriter, r *http.Request) {
	showID := strings.TrimSpace(r.PathValue("showID"))
	show, ok := a.store.showByID(showID)
	if !ok {
		http.NotFound(w, r)
		return
	}
	org, _ := a.store.organizationByID(show.OrganizationID)
	notice := strings.TrimSpace(r.URL.Query().Get("notice"))

	a.render(w, r, "admin_show_helpers.html", a.buildAdminShowHelpersData(show, org, notice, ""))
}

func (a *app) buildAdminShowHelpersData(show *Show, org *Organization, notice, errMsg string) adminShowHelpersData {
	now := time.Now().UTC()
	invites := make([]adminShowHelperInviteView, 0)
	for _, item := range a.store.listShowHelperInvitesByShow(show.ID) {
		active := item.RevokedAt == nil && now.Before(item.ExpiresAt)
		statusLabel := "active"
		switch {
		case item.RevokedAt != nil:
			statusLabel = "revoked"
		case !now.Before(item.ExpiresAt):
			statusLabel = "expired"
		}
		invites = append(invites, adminShowHelperInviteView{
			ID:          item.ID,
			Label:       item.Label,
			CreatedBy:   item.CreatedBy,
			CreatedAt:   item.CreatedAt,
			ExpiresAt:   item.ExpiresAt,
			Active:      active,
			StatusLabel: statusLabel,
			RevokedAt:   item.RevokedAt,
		})
	}

	sessions := make([]adminShowBadgeSessionView, 0)
	for _, session := range a.store.listShowBadgeSessionsByShow(show.ID) {
		active := session.RevokedAt == nil && now.Before(session.ExpiresAt)
		statusLabel := "active"
		switch {
		case session.RevokedAt != nil:
			statusLabel = "ended"
		case !now.Before(session.ExpiresAt):
			statusLabel = "expired"
		}
		matchedPerson := ""
		if session.MatchedPersonID != "" {
			if person, ok := a.store.personByID(session.MatchedPersonID); ok && person != nil {
				matchedPerson = strings.TrimSpace(person.FirstName + " " + person.LastName)
			}
		}
		sessions = append(sessions, adminShowBadgeSessionView{
			ID:            session.ID,
			Name:          session.Name,
			Email:         session.Email,
			MatchedPerson: matchedPerson,
			CreatedAt:     session.CreatedAt,
			LastSeenAt:    session.LastSeenAt,
			ExpiresAt:     session.ExpiresAt,
			Active:        active,
			StatusLabel:   statusLabel,
		})
	}

	return adminShowHelpersData{
		Title:         "Helper Share Links · " + show.Name,
		CurrentPath:   "/admin/shows/" + show.ID + "/helpers",
		Show:          show,
		Org:           org,
		Invites:       invites,
		BadgeSessions: sessions,
		Notice:        notice,
		Error:         errMsg,
	}
}

func (a *app) handleAdminShowHelperCreate(w http.ResponseWriter, r *http.Request) {
	showID := strings.TrimSpace(r.PathValue("showID"))
	show, ok := a.store.showByID(showID)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	user, _ := a.currentUser(r)
	createdBy := ""
	if user != nil {
		createdBy = strings.TrimSpace(user.SubjectID)
		if createdBy == "" {
			createdBy = strings.TrimSpace(user.CognitoSub)
		}
	}
	label := strings.TrimSpace(r.FormValue("label"))

	issued, err := a.store.createShowHelperInvite(ShowHelperInviteInput{
		ShowID:        show.ID,
		Label:         label,
		CreatedBy:     createdBy,
		ExpiresInDays: defaultShowHelperInviteDays,
	})
	if err != nil || issued == nil || issued.Invite == nil {
		http.Error(w, "could not create helper invite", http.StatusBadRequest)
		return
	}

	org, _ := a.store.organizationByID(show.OrganizationID)
	shareURL := requestBasePath(r) + "/shows/" + show.Slug + "/help-redeem?token=" + url.QueryEscape(issued.Token)

	a.render(w, r, "admin_show_helper_created.html", adminShowHelperCreatedData{
		Title:       "Share link created · " + show.Name,
		CurrentPath: "/admin/shows/" + show.ID + "/helpers",
		Show:        show,
		Org:         org,
		ShareURL:    shareURL,
		Token:       issued.Token,
		Invite:      issued.Invite,
	})
}

func (a *app) handleAdminShowHelperRevoke(w http.ResponseWriter, r *http.Request) {
	showID := strings.TrimSpace(r.PathValue("showID"))
	inviteID := strings.TrimSpace(r.PathValue("inviteID"))
	show, ok := a.store.showByID(showID)
	if !ok {
		http.NotFound(w, r)
		return
	}
	invite, ok := a.store.showHelperInviteByID(inviteID)
	if !ok || invite == nil || invite.ShowID != show.ID {
		http.NotFound(w, r)
		return
	}
	if err := a.store.revokeShowHelperInvite(invite.ID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	target := "/admin/shows/" + show.ID + "/helpers?notice=" + url.QueryEscape("Share link revoked.")
	http.Redirect(w, r, target, http.StatusSeeOther)
}
