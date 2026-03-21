package main

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

func (a *app) handleAdminLogout(w http.ResponseWriter, r *http.Request) {
	a.clearPendingAuth(w, r)
	a.clearUserSession(w, r)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// --- Dashboard ---

type adminDashboardData struct {
	Title         string
	CurrentPath   string
	ActiveSection string
	Sections      []accountSectionView
	Shows         []*Show
	Persons       []*Person
	Orgs          []*Organization
	Judges        []adminJudgePersonView
	References    []adminReferenceLink
	Awards        []*AwardDefinition
	Rubrics       []*JudgingRubric
	SearchQuery   string
	SearchHits    []adminSearchHit
}

type adminSearchHit struct {
	TypeLabel string
	Title     string
	Meta      string
	Href      string
}

type adminJudgeShowView struct {
	ShowName    string
	ShowHref    string
	ShowDate    string
	StatusLabel string
	RoleLabel   string
}

type adminJudgePersonView struct {
	PersonID       string
	PersonName     string
	PersonHref     string
	Email          string
	Phone          string
	Initials       string
	Specialties    string
	Qualifications string
	Notes          string
	Affiliations   []string
	Shows          []adminJudgeShowView
}

type adminJudgePickView struct {
	EntryName    string
	EntryHref    string
	ShowName     string
	ShowHref     string
	ShowDate     string
	ClassLabel   string
	EntrantLabel string
	MediaPath    string
}

type adminJudgeProfileData struct {
	Title         string
	CurrentPath   string
	Sections      []accountSectionView
	Judge         adminJudgePersonView
	UpcomingShows []adminJudgeShowView
	PastShows     []adminJudgeShowView
	FirstPicks    []adminJudgePickView
}

type adminReferenceLink struct {
	Label       string
	Href        string
	Description string
}

type clubAdminData struct {
	Title             string
	CurrentPath       string
	OrganizationID    string
	Organization      *Organization
	CurrentUser       *UserIdentity
	Sections          []accountSectionView
	ActiveSection     string
	ManagedShows      []*Show
	Members           []clubMemberView
	Invites           []organizationInviteView
	InviteRoleOptions []inviteRoleOptionView
	Notice            string
}

type clubMemberView struct {
	FullName         string
	Email            string
	OrganizationRole string
	PermissionRoles  []string
}

type organizationInviteView struct {
	ID               string
	FullName         string
	Email            string
	OrganizationRole string
	PermissionLabels []string
	StatusLabel      string
	ClaimedLabel     string
}

type inviteRoleOptionView struct {
	Role        string
	Label       string
	Description string
}

type adminClubCreateData struct {
	Title         string
	CurrentPath   string
	Organizations []*Organization
}

func (a *app) handleAdminDashboard(w http.ResponseWriter, r *http.Request) {
	orgs := a.store.allOrganizations()
	section := adminDashboardSection(r.URL.Query().Get("section"))
	var awards []*AwardDefinition
	for _, org := range orgs {
		awards = append(awards, a.store.awardsByOrganization(org.ID)...)
	}
	searchQuery := strings.TrimSpace(r.URL.Query().Get("q"))
	a.render(w, r, "admin_dashboard.html", adminDashboardData{
		Title:         "Admin Dashboard",
		CurrentPath:   "/admin",
		ActiveSection: section,
		Sections:      adminDashboardSections(section),
		Shows:         a.store.allShows(),
		Persons:       a.store.allPersons(),
		Orgs:          orgs,
		Judges:        a.adminJudgePersonViews(),
		References:    adminReferenceLinks(),
		Awards:        awards,
		Rubrics:       a.store.allRubrics(),
		SearchQuery:   searchQuery,
		SearchHits:    a.adminSearchHits(searchQuery),
	})
}

func (a *app) handleAdminJudgeCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	firstName := strings.TrimSpace(r.FormValue("first_name"))
	lastName := strings.TrimSpace(r.FormValue("last_name"))
	if firstName == "" && lastName == "" {
		http.Error(w, "judge name required", http.StatusBadRequest)
		return
	}
	orgID := strings.TrimSpace(r.FormValue("organization_id"))
	orgRole := ""
	if orgID != "" {
		orgRole = "judge"
	}
	person, err := a.store.createPerson(PersonInput{
		FirstName:        firstName,
		LastName:         lastName,
		Email:            strings.TrimSpace(r.FormValue("email")),
		Phone:            strings.TrimSpace(r.FormValue("phone")),
		Specialties:      strings.TrimSpace(r.FormValue("specialties")),
		Qualifications:   strings.TrimSpace(r.FormValue("qualifications")),
		Notes:            strings.TrimSpace(r.FormValue("notes")),
		IsJudge:          true,
		OrganizationID:   orgID,
		OrganizationRole: orgRole,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, "/admin/judges/"+person.ID, http.StatusSeeOther)
}

func (a *app) handleAdminJudgeProfile(w http.ResponseWriter, r *http.Request) {
	data, err := a.adminJudgeProfileData(r.PathValue("personID"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	a.render(w, r, "admin_judge.html", data)
}

func adminDashboardSection(raw string) string {
	switch strings.TrimSpace(raw) {
	case "shows", "clubs", "people", "judges", "references":
		return strings.TrimSpace(raw)
	default:
		return "overview"
	}
}

func adminDashboardSections(active string) []accountSectionView {
	return []accountSectionView{
		{ID: "overview", Label: "Overview", Href: "/admin?section=overview", Current: active == "overview"},
		{ID: "shows", Label: "Shows", Href: "/admin?section=shows", Current: active == "shows"},
		{ID: "clubs", Label: "Clubs", Href: "/admin?section=clubs", Current: active == "clubs"},
		{ID: "people", Label: "People", Href: "/admin?section=people", Current: active == "people"},
		{ID: "judges", Label: "Judges", Href: "/admin?section=judges", Current: active == "judges"},
		{ID: "references", Label: "References", Href: "/admin?section=references", Current: active == "references"},
	}
}

func adminReferenceLinks() []adminReferenceLink {
	return []adminReferenceLink{
		{Label: "Standards", Href: "/standards", Description: "Reference materials and source documents."},
		{Label: "Taxonomy", Href: "/taxonomy", Description: "Browse taxons and related show entries."},
		{Label: "Leaderboard", Href: "/leaderboard?org=all&season=" + time.Now().Format("2006"), Description: "Current season results across organizations."},
		{Label: "Browse", Href: "/browse", Description: "Search entries, classes, and public history."},
		{Label: "Role Management", Href: "/admin/roles", Description: "Platform-level role assignment surface."},
	}
}

func (a *app) adminJudgePersonViews() []adminJudgePersonView {
	type judgeBucket struct {
		Person       *Person
		Affiliations map[string]struct{}
		ShowsByKey   map[string]adminJudgeShowView
	}

	now := dateOnly(time.Now())
	buckets := map[string]*judgeBucket{}
	for _, person := range a.store.allPersons() {
		if person == nil {
			continue
		}
		if person.IsJudge {
			buckets[person.ID] = &judgeBucket{
				Person:       person,
				Affiliations: make(map[string]struct{}),
				ShowsByKey:   make(map[string]adminJudgeShowView),
			}
		}
		for _, link := range a.store.personOrganizationsByPerson(person.ID) {
			if link == nil || !strings.Contains(strings.ToLower(strings.TrimSpace(link.Role)), "judge") {
				continue
			}
			bucket := buckets[person.ID]
			if bucket == nil {
				bucket = &judgeBucket{
					Person:       person,
					Affiliations: make(map[string]struct{}),
					ShowsByKey:   make(map[string]adminJudgeShowView),
				}
				buckets[person.ID] = bucket
			}
			orgName := link.OrganizationID
			if org, ok := a.store.organizationByID(link.OrganizationID); ok && org != nil {
				orgName = org.Name
			}
			roleLabel := strings.TrimSpace(link.Role)
			if roleLabel == "" {
				roleLabel = "judge"
			}
			bucket.Affiliations[orgName+" · "+roleLabel] = struct{}{}
		}
	}

	for _, show := range a.store.allShows() {
		if show == nil {
			continue
		}
		statusLabel := strings.Title(strings.TrimSpace(show.Status))
		if showDate, ok := parseShowDate(show.Date); ok {
			if showDate.Before(now) {
				statusLabel = "Completed"
			} else {
				statusLabel = "Scheduled"
			}
		}
		for _, assignment := range a.store.judgesByShow(show.ID) {
			person, _ := a.store.personByID(assignment.PersonID)
			if person == nil {
				continue
			}
			bucket := buckets[person.ID]
			if bucket == nil {
				bucket = &judgeBucket{
					Person:       person,
					Affiliations: make(map[string]struct{}),
					ShowsByKey:   make(map[string]adminJudgeShowView),
				}
				buckets[person.ID] = bucket
			}
			bucket.ShowsByKey[show.ID] = adminJudgeShowView{
				ShowName:    show.Name,
				ShowHref:    "/admin/shows/" + show.ID,
				ShowDate:    strings.TrimSpace(show.Date),
				StatusLabel: statusLabel,
				RoleLabel:   "Assigned judge",
			}
		}
	}

	for _, show := range a.store.allShows() {
		if show == nil {
			continue
		}
		statusLabel := "Completed"
		if showDate, ok := parseShowDate(show.Date); ok && !showDate.Before(now) {
			statusLabel = "Scheduled"
		}
		for _, entry := range a.store.entriesByShow(show.ID) {
			for _, scorecard := range a.store.scorecardsByEntry(entry.ID) {
				person, _ := a.store.personByID(scorecard.JudgeID)
				if person == nil {
					continue
				}
				bucket := buckets[person.ID]
				if bucket == nil {
					bucket = &judgeBucket{
						Person:       person,
						Affiliations: make(map[string]struct{}),
						ShowsByKey:   make(map[string]adminJudgeShowView),
					}
					buckets[person.ID] = bucket
				}
				showView := adminJudgeShowView{
					ShowName:    show.Name,
					ShowHref:    "/admin/shows/" + show.ID,
					ShowDate:    strings.TrimSpace(show.Date),
					StatusLabel: statusLabel,
					RoleLabel:   "Scored entries",
				}
				if existing, ok := bucket.ShowsByKey[show.ID]; ok && existing.RoleLabel == "Assigned judge" {
					continue
				}
				bucket.ShowsByKey[show.ID] = showView
			}
		}
	}

	out := make([]adminJudgePersonView, 0, len(buckets))
	for _, bucket := range buckets {
		if bucket == nil || bucket.Person == nil {
			continue
		}
		shows := make([]adminJudgeShowView, 0, len(bucket.ShowsByKey))
		for _, show := range bucket.ShowsByKey {
			shows = append(shows, show)
		}
		sort.Slice(shows, func(i, j int) bool {
			leftDate, leftOK := parseShowDate(shows[i].ShowDate)
			rightDate, rightOK := parseShowDate(shows[j].ShowDate)
			if leftOK && rightOK && !leftDate.Equal(rightDate) {
				return leftDate.After(rightDate)
			}
			return shows[i].ShowName < shows[j].ShowName
		})
		affiliations := make([]string, 0, len(bucket.Affiliations))
		for label := range bucket.Affiliations {
			affiliations = append(affiliations, label)
		}
		sort.Strings(affiliations)
		fullName := strings.TrimSpace(bucket.Person.FirstName + " " + bucket.Person.LastName)
		if fullName == "" {
			fullName = bucket.Person.Initials
		}
		out = append(out, adminJudgePersonView{
			PersonID:       bucket.Person.ID,
			PersonName:     fullName,
			PersonHref:     "/admin/judges/" + bucket.Person.ID,
			Email:          strings.TrimSpace(bucket.Person.Email),
			Phone:          strings.TrimSpace(bucket.Person.Phone),
			Initials:       strings.TrimSpace(bucket.Person.Initials),
			Specialties:    strings.TrimSpace(bucket.Person.Specialties),
			Qualifications: strings.TrimSpace(bucket.Person.Qualifications),
			Notes:          strings.TrimSpace(bucket.Person.Notes),
			Affiliations:   affiliations,
			Shows:          shows,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].PersonName == out[j].PersonName {
			return out[i].Email < out[j].Email
		}
		return out[i].PersonName < out[j].PersonName
	})
	return out
}

func (a *app) adminJudgeProfileData(personID string) (adminJudgeProfileData, error) {
	var judge *adminJudgePersonView
	for _, item := range a.adminJudgePersonViews() {
		if item.PersonID == personID {
			copy := item
			judge = &copy
			break
		}
	}
	if judge == nil {
		return adminJudgeProfileData{}, fmt.Errorf("judge not found")
	}

	var upcoming []adminJudgeShowView
	var past []adminJudgeShowView
	now := dateOnly(time.Now())
	for _, show := range judge.Shows {
		if showDate, ok := parseShowDate(show.ShowDate); ok && showDate.Before(now) {
			past = append(past, show)
			continue
		}
		upcoming = append(upcoming, show)
	}

	firstPickByEntry := make(map[string]adminJudgePickView)
	for _, show := range a.store.allShows() {
		if show == nil {
			continue
		}
		for _, entry := range a.store.entriesByShow(show.ID) {
			if entry == nil || entry.Placement != 1 || entry.Suppressed {
				continue
			}
			var picked bool
			for _, scorecard := range a.store.scorecardsByEntry(entry.ID) {
				if scorecard != nil && scorecard.JudgeID == personID {
					picked = true
					break
				}
			}
			if !picked {
				continue
			}
			class, _ := a.store.classByID(entry.ClassID)
			person, _ := a.store.personByID(entry.PersonID)
			view := adminJudgePickView{
				EntryName: entry.Name,
				EntryHref: "/entries/" + entry.ID,
			}
			view.ShowName = show.Name
			view.ShowHref = "/shows/" + show.Slug
			view.ShowDate = strings.TrimSpace(show.Date)
			if class != nil {
				view.ClassLabel = strings.TrimSpace(class.ClassNumber + " · " + class.Title)
			}
			if person != nil {
				view.EntrantLabel = publicPersonLabel(person)
			}
			if media := a.store.mediaByEntry(entry.ID); len(media) > 0 {
				view.MediaPath = "/media/" + media[0].ID
			}
			firstPickByEntry[entry.ID] = view
		}
	}
	firstPicks := make([]adminJudgePickView, 0, len(firstPickByEntry))
	for _, pick := range firstPickByEntry {
		firstPicks = append(firstPicks, pick)
	}
	sort.Slice(firstPicks, func(i, j int) bool {
		if firstPicks[i].ShowDate == firstPicks[j].ShowDate {
			return firstPicks[i].EntryName < firstPicks[j].EntryName
		}
		return firstPicks[i].ShowDate > firstPicks[j].ShowDate
	})

	return adminJudgeProfileData{
		Title:         "Judge Profile: " + judge.PersonName,
		CurrentPath:   "/admin/judges/" + personID,
		Sections:      adminDashboardSections("judges"),
		Judge:         *judge,
		UpcomingShows: upcoming,
		PastShows:     past,
		FirstPicks:    firstPicks,
	}, nil
}

func clubAdminSections(organizationID, active string) []accountSectionView {
	base := "/admin/clubs/" + organizationID
	return []accountSectionView{
		{ID: "overview", Label: "Overview", Href: base, Current: active == "overview"},
		{ID: "invites", Label: "Invites", Href: base + "?section=invites#club-invites", Current: active == "invites"},
	}
}

func organizationInviteRoleOptions() []inviteRoleOptionView {
	roles := []string{"organization_admin", "show_intake_operator", "show_judge_support", "photographer", "judge", "entrant"}
	out := make([]inviteRoleOptionView, 0, len(roles))
	for _, role := range roles {
		def, ok := flowershowAuthorityBundles[role]
		if !ok {
			continue
		}
		out = append(out, inviteRoleOptionView{
			Role:        role,
			Label:       def.DisplayName,
			Description: formatPermissionList(def.Capabilities),
		})
	}
	return out
}

func organizationLevelOptions() []string {
	return []string{"society", "district", "region", "province", "country", "global"}
}

func (a *app) adminSearchHits(query string) []adminSearchHit {
	query = strings.TrimSpace(strings.ToLower(query))
	if query == "" {
		return nil
	}

	var hits []adminSearchHit
	addHit := func(kind, title, meta, href string) {
		title = strings.TrimSpace(title)
		meta = strings.TrimSpace(meta)
		if title == "" || href == "" {
			return
		}
		hits = append(hits, adminSearchHit{
			TypeLabel: kind,
			Title:     title,
			Meta:      meta,
			Href:      href,
		})
	}

	for _, show := range a.store.allShows() {
		if show == nil {
			continue
		}
		blob := strings.ToLower(strings.Join([]string{show.Name, show.Location, show.Season, show.Date}, " "))
		if strings.Contains(blob, query) {
			addHit("Show", show.Name, strings.TrimSpace(show.Date+" · "+show.Location), "/admin/shows/"+show.ID)
		}
		for _, cls := range a.store.classesByShowID(show.ID) {
			if cls == nil {
				continue
			}
			blob = strings.ToLower(strings.Join([]string{cls.ClassNumber, cls.Title, cls.Description, show.Name}, " "))
			if strings.Contains(blob, query) {
				addHit("Class", strings.TrimSpace(cls.ClassNumber+" · "+cls.Title), show.Name, "/shows/"+show.Slug+"/classes/"+cls.ID)
			}
		}
	}
	for _, org := range a.store.allOrganizations() {
		if org == nil {
			continue
		}
		blob := strings.ToLower(strings.Join([]string{org.Name, org.Level}, " "))
		if strings.Contains(blob, query) {
			addHit("Club", org.Name, org.Level, "/admin/clubs/"+org.ID)
		}
	}
	for _, person := range a.store.allPersons() {
		if person == nil {
			continue
		}
		fullName := strings.TrimSpace(person.FirstName + " " + person.LastName)
		blob := strings.ToLower(strings.Join([]string{fullName, person.Email}, " "))
		if strings.Contains(blob, query) {
			meta := person.Email
			if meta == "" {
				meta = "Person profile"
			}
			addHit("Person", fullName, meta, "/people/"+person.ID)
		}
	}
	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].TypeLabel == hits[j].TypeLabel {
			return hits[i].Title < hits[j].Title
		}
		return hits[i].TypeLabel < hits[j].TypeLabel
	})
	return hits
}

func (a *app) clubAdminData(organizationID, activeSection string, user *UserIdentity, notice string) (clubAdminData, error) {
	org, ok := a.store.organizationByID(organizationID)
	if !ok {
		return clubAdminData{}, fmt.Errorf("organization not found")
	}

	data := clubAdminData{
		Title:             "Club Admin: " + org.Name,
		CurrentPath:       "/admin/clubs/" + org.ID,
		OrganizationID:    org.ID,
		Organization:      org,
		CurrentUser:       user,
		ActiveSection:     activeSection,
		Sections:          clubAdminSections(org.ID, activeSection),
		InviteRoleOptions: organizationInviteRoleOptions(),
		Notice:            strings.TrimSpace(notice),
	}

	for _, show := range a.store.allShows() {
		if show != nil && show.OrganizationID == org.ID {
			data.ManagedShows = append(data.ManagedShows, show)
		}
	}

	roleAssignments := map[string][]string{}
	acceptedInviteRoles := map[string][]string{}
	if a.authority != nil {
		if roles, err := a.authority.AllRoleAssignments(context.Background()); err == nil {
			for _, role := range roles {
				if role == nil || role.OrganizationID != org.ID {
					continue
				}
				roleAssignments[role.SubjectID] = append(roleAssignments[role.SubjectID], role.Role)
			}
		}
	}
	for _, invite := range a.store.organizationInvitesByOrganization(org.ID) {
		if invite == nil || invite.Status != "accepted" {
			continue
		}
		key := normalizeAuthIdentifier(invite.Email)
		if key == "" {
			continue
		}
		acceptedInviteRoles[key] = append(acceptedInviteRoles[key], invite.PermissionRoles...)
	}

	for _, person := range a.store.allPersons() {
		if person == nil {
			continue
		}
		for _, link := range a.store.personOrganizationsByPerson(person.ID) {
			if link == nil || link.OrganizationID != org.ID {
				continue
			}
			data.Members = append(data.Members, clubMemberView{
				FullName:         strings.TrimSpace(person.FirstName + " " + person.LastName),
				Email:            person.Email,
				OrganizationRole: link.Role,
				PermissionRoles: append([]string(nil), func() []string {
					if roles := roleAssignments[person.ID]; len(roles) > 0 {
						return roles
					}
					return acceptedInviteRoles[normalizeAuthIdentifier(person.Email)]
				}()...),
			})
		}
	}

	for _, invite := range a.store.organizationInvitesByOrganization(org.ID) {
		if invite == nil {
			continue
		}
		labels := make([]string, 0, len(invite.PermissionRoles))
		for _, role := range invite.PermissionRoles {
			if def, ok := flowershowAuthorityBundles[role]; ok {
				labels = append(labels, def.DisplayName)
			} else {
				labels = append(labels, role)
			}
		}
		claimedLabel := ""
		if !invite.ClaimedAt.IsZero() {
			claimedLabel = invite.ClaimedAt.Local().Format("2006-01-02 15:04")
		}
		data.Invites = append(data.Invites, organizationInviteView{
			ID:               invite.ID,
			FullName:         strings.TrimSpace(invite.FirstName + " " + invite.LastName),
			Email:            invite.Email,
			OrganizationRole: invite.OrganizationRole,
			PermissionLabels: labels,
			StatusLabel:      invite.Status,
			ClaimedLabel:     claimedLabel,
		})
	}

	return data, nil
}

func (a *app) handleAdminClubDetail(w http.ResponseWriter, r *http.Request) {
	user, _ := a.currentUser(r)
	activeSection := strings.TrimSpace(r.URL.Query().Get("section"))
	if activeSection != "invites" {
		activeSection = "overview"
	}
	data, err := a.clubAdminData(r.PathValue("organizationID"), activeSection, user, r.URL.Query().Get("notice"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	a.render(w, r, "admin_club.html", data)
}

func (a *app) handleAdminClubInviteCreate(w http.ResponseWriter, r *http.Request) {
	user, _ := a.currentUser(r)
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	orgID := r.PathValue("organizationID")
	_, err := a.store.createOrganizationInvite(OrganizationInviteInput{
		OrganizationID:   orgID,
		FirstName:        r.FormValue("first_name"),
		LastName:         r.FormValue("last_name"),
		Email:            r.FormValue("email"),
		OrganizationRole: r.FormValue("organization_role"),
		PermissionRoles:  r.Form["permission_roles"],
		InvitedBySubject: func() string {
			if user != nil {
				return user.SubjectID
			}
			return ""
		}(),
		InvitedByName: func() string {
			if user == nil {
				return ""
			}
			if strings.TrimSpace(user.Name) != "" {
				return user.Name
			}
			return user.Email
		}(),
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	fullName := strings.TrimSpace(strings.Join([]string{r.FormValue("first_name"), r.FormValue("last_name")}, " "))
	if fullName == "" {
		fullName = strings.TrimSpace(r.FormValue("email"))
	}
	notice := "Invite sent to " + fullName
	if email := strings.TrimSpace(r.FormValue("email")); email != "" {
		notice += " · " + email
	}
	http.Redirect(w, r, "/admin/clubs/"+orgID+"?section=invites&notice="+url.QueryEscape(notice)+"#club-invites", http.StatusSeeOther)
}

func (a *app) handleAdminClubNew(w http.ResponseWriter, r *http.Request) {
	a.render(w, r, "admin_club_new.html", adminClubCreateData{
		Title:         "New Club",
		CurrentPath:   "/admin/clubs/new",
		Organizations: a.store.allOrganizations(),
	})
}

func (a *app) handleAdminClubCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	user, _ := a.currentUser(r)
	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		http.Error(w, "club name is required", http.StatusBadRequest)
		return
	}
	parentID := strings.TrimSpace(r.FormValue("parent_id"))
	level := a.inferOrganizationLevel(parentID)
	org, err := a.store.createOrganization(Organization{
		Name:     name,
		Level:    level,
		ParentID: parentID,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	firstName := strings.TrimSpace(r.FormValue("admin_first_name"))
	lastName := strings.TrimSpace(r.FormValue("admin_last_name"))
	email := strings.TrimSpace(r.FormValue("admin_email"))
	if email != "" {
		_, err = a.store.createPerson(PersonInput{
			FirstName:        firstName,
			LastName:         lastName,
			Email:            email,
			OrganizationID:   org.ID,
			OrganizationRole: "admin",
		})
		if err != nil && !strings.Contains(strings.ToLower(err.Error()), "exists") {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		_, err = a.store.createOrganizationInvite(OrganizationInviteInput{
			OrganizationID:   org.ID,
			FirstName:        firstName,
			LastName:         lastName,
			Email:            email,
			OrganizationRole: "admin",
			PermissionRoles:  []string{"organization_admin"},
			InvitedBySubject: func() string {
				if user != nil {
					return user.SubjectID
				}
				return ""
			}(),
			InvitedByName: func() string {
				if user == nil {
					return ""
				}
				if strings.TrimSpace(user.Name) != "" {
					return user.Name
				}
				return user.Email
			}(),
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}

	notice := "Club created"
	if email != "" {
		fullName := strings.TrimSpace(strings.Join([]string{firstName, lastName}, " "))
		if fullName == "" {
			fullName = email
		}
		notice = "Club created. Invite sent to " + fullName + " · " + email
	}
	http.Redirect(w, r, "/admin/clubs/"+org.ID+"?notice="+url.QueryEscape(notice), http.StatusSeeOther)
}

func (a *app) inferOrganizationLevel(parentID string) string {
	parentID = strings.TrimSpace(parentID)
	if parentID == "" {
		return "society"
	}
	parent, ok := a.store.organizationByID(parentID)
	if !ok || parent == nil {
		return "society"
	}
	order := []string{"society", "district", "region", "province", "country", "global"}
	for i, level := range order {
		if level != parent.Level {
			continue
		}
		if i == 0 {
			return "society"
		}
		return order[i-1]
	}
	return "society"
}

// --- Show CRUD ---

func (a *app) handleAdminShowNew(w http.ResponseWriter, r *http.Request) {
	a.render(w, r, "admin_show_new.html", map[string]any{
		"Title":         "New Show",
		"CurrentPath":   "/admin/shows/new",
		"Orgs":          a.store.allOrganizations(),
		"DefaultSeason": time.Now().Format("2006"),
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
	Title                string
	CurrentPath          string
	ShowID               string
	Show                 *Show
	Schedule             *ShowSchedule
	ScheduleEdition      *StandardEdition
	Divisions            []*divisionView
	Entries              []*entryView
	Persons              []*Person
	EntrantCandidates    []*personLookupView
	RecentEntries        []*entryView
	EntriesNeedingPhotos []*entryView
	BoardDivisions       []*boardDivisionView
	BoardStats           boardStats
	Classes              []*ShowClass
	Awards               []*AwardDefinition
	Rubrics              []*JudgingRubric
	RubricViews          []*rubricView
	Orgs                 []*Organization
	Standards            []*StandardDocument
	StandardViews        []*standardView
	StandardEditions     []*StandardEdition
	Sources              []*SourceDocument
	Judges               []*showJudgeView
	AvailableJudges      []*Person
	ShowCredits          []*showCreditView
	ClassRuleViews       []*classRuleView
	CitationTargets      []citationTargetOption
}

func (a *app) handleAdminShowDetail(w http.ResponseWriter, r *http.Request) {
	showID := r.PathValue("showID")
	data, err := a.adminShowDetailData(showID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	a.render(w, r, "show_admin.html", data)
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
	boardDivisions, boardStats := a.boardDataForShow(show.ID, entries)
	var showCredits []*showCreditView
	for _, credit := range a.store.showCreditsByShow(show.ID) {
		person, _ := a.store.personByID(credit.PersonID)
		showCredits = append(showCredits, &showCreditView{
			Credit: credit,
			Person: person,
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
		Title:                "Admin: " + show.Name,
		CurrentPath:          "/admin/shows/" + show.ID,
		ShowID:               show.ID,
		Show:                 show,
		Schedule:             sched,
		ScheduleEdition:      scheduleEdition,
		Divisions:            divisions,
		Entries:              entries,
		Persons:              a.store.allPersons(),
		EntrantCandidates:    a.personLookupViewsForShow(show.ID, ""),
		RecentEntries:        a.recentEntryViews(entries, 8),
		EntriesNeedingPhotos: a.entriesNeedingPhotos(entries),
		BoardDivisions:       boardDivisions,
		BoardStats:           boardStats,
		Classes:              a.store.classesByShowID(show.ID),
		Awards:               awards,
		Rubrics:              a.store.allRubrics(),
		RubricViews:          rubricViews,
		Orgs:                 orgs,
		Standards:            a.store.allStandardDocuments(),
		StandardViews:        standardViews,
		StandardEditions:     a.store.allStandardEditions(),
		Sources:              a.store.allSourceDocuments(),
		Judges:               a.judgeViewsForShow(show.ID),
		AvailableJudges:      a.availableJudgesForShow(show.ID),
		ShowCredits:          showCredits,
		ClassRuleViews:       classRuleViews,
		CitationTargets:      a.citationTargetsForShow(show.ID, classRuleViews),
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
	a.publishAdminSections(showID, "setup")
	a.respondAdminSectionOrRedirect(w, r, showID, "setup")
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
		a.publishAdminSections(showID, "setup", "governance", "scoring", "board")
		a.respondAdminSectionOrRedirect(w, r, showID, "setup")
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
	a.publishAdminSections(showID, "setup", "governance", "scoring", "board")
	a.respondAdminSectionOrRedirect(w, r, showID, "setup")
}

func (a *app) handleAdminJudgeAssign(w http.ResponseWriter, r *http.Request) {
	showID := r.PathValue("showID")
	r.ParseForm()
	if _, err := a.store.assignJudgeToShow(showID, r.FormValue("person_id")); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	a.sseBroker.publish(showID, "show-updated", `<div class="toast">Judge assigned</div>`)
	a.publishAdminSections(showID, "setup", "scoring")
	a.respondAdminSectionOrRedirect(w, r, showID, "setup")
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
	a.publishAdminSections(showID, "setup", "intake", "floor", "board", "scoring", "governance")
	a.respondAdminSectionOrRedirect(w, r, showID, "setup")
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
	a.publishAdminSections(showID, "setup", "intake", "floor", "board", "scoring", "governance")
	a.respondAdminSectionOrRedirect(w, r, showID, "setup")
}

func (a *app) handleAdminClassCreate(w http.ResponseWriter, r *http.Request) {
	showID := r.PathValue("showID")
	r.ParseForm()
	specimenCount, _ := strconv.Atoi(r.FormValue("specimen_count"))
	sortOrder, _ := strconv.Atoi(r.FormValue("sort_order"))
	_, err := a.store.createClass(ShowClassInput{
		SectionID:     r.FormValue("section_id"),
		ClassNumber:   r.FormValue("class_number"),
		SortOrder:     sortOrder,
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
	a.publishAdminSections(showID, "setup", "intake", "floor", "board", "scoring", "governance")
	a.respondAdminSectionOrRedirect(w, r, showID, "setup")
}

func (a *app) handleAdminClassUpdate(w http.ResponseWriter, r *http.Request) {
	classID := r.PathValue("classID")
	r.ParseForm()
	specimenCount, _ := strconv.Atoi(r.FormValue("specimen_count"))
	sortOrder, _ := strconv.Atoi(r.FormValue("sort_order"))
	_, err := a.store.updateClass(classID, ShowClassInput{
		SectionID:       r.FormValue("section_id"),
		ClassNumber:     r.FormValue("class_number"),
		SortOrder:       sortOrder,
		Title:           r.FormValue("title"),
		Domain:          r.FormValue("domain"),
		Description:     r.FormValue("description"),
		SpecimenCount:   specimenCount,
		ScheduleNotes:   r.FormValue("schedule_notes"),
		MeasurementRule: r.FormValue("measurement_rule"),
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	showID := r.FormValue("show_id")
	a.sseBroker.publish(showID, "show-updated", `<div class="toast">Class updated</div>`)
	a.publishAdminSections(showID, "setup", "intake", "floor", "board", "scoring", "governance")
	a.respondAdminSectionOrRedirect(w, r, showID, "setup")
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
	a.publishAdminSections(showID, "intake", "floor", "board", "scoring", "governance")
	a.publishShowSummary(showID)
	a.respondAdminSectionOrRedirect(w, r, showID, "intake")
}

func (a *app) handleAdminEntryMove(w http.ResponseWriter, r *http.Request) {
	entryID := r.PathValue("entryID")
	r.ParseForm()
	entry, err := a.store.moveEntry(entryID, r.FormValue("class_id"), r.FormValue("reason"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	a.sseBroker.publish(entry.ShowID, "show-updated", `<div class="toast">Entry moved to a different class</div>`)
	a.publishAdminSections(entry.ShowID, "setup", "intake", "floor", "board", "scoring", "governance")
	a.publishShowSummary(entry.ShowID)
	a.respondAdminSectionOrRedirect(w, r, entry.ShowID, "floor")
}

func (a *app) handleAdminEntryReassign(w http.ResponseWriter, r *http.Request) {
	entryID := r.PathValue("entryID")
	r.ParseForm()
	entry, err := a.store.reassignEntry(entryID, r.FormValue("person_id"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	a.sseBroker.publish(entry.ShowID, "show-updated", `<div class="toast">Entrant assignment updated</div>`)
	a.publishAdminSections(entry.ShowID, "intake", "floor", "board", "scoring")
	a.respondAdminSectionOrRedirect(w, r, entry.ShowID, "floor")
}

func (a *app) handleAdminEntryDelete(w http.ResponseWriter, r *http.Request) {
	entryID := r.PathValue("entryID")
	entry, ok := a.store.entryByID(entryID)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if err := a.store.deleteEntry(entryID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	a.sseBroker.publish(entry.ShowID, "show-updated", `<div class="toast">Entry deleted</div>`)
	a.publishAdminSections(entry.ShowID, "intake", "floor", "board", "scoring", "governance")
	a.publishShowSummary(entry.ShowID)
	a.respondAdminSectionOrRedirect(w, r, entry.ShowID, "floor")
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
		a.publishAdminSections(entry.ShowID, "floor", "board")
		a.publishShowSummary(entry.ShowID)
		a.respondAdminSectionOrRedirect(w, r, entry.ShowID, "floor")
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
		a.publishAdminSections(entry.ShowID, "intake", "floor", "board", "scoring", "governance")
		a.publishShowSummary(entry.ShowID)
		a.respondAdminSectionOrRedirect(w, r, entry.ShowID, "floor")
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
		a.publishAdminSections(entry.ShowID, "intake", "floor", "board")
		a.publishShowSummary(entry.ShowID)
		a.respondAdminSectionOrRedirect(w, r, entry.ShowID, "floor")
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
		a.publishAdminSections(entry.ShowID, "intake", "floor", "board")
		a.publishShowSummary(entry.ShowID)
		a.respondAdminSectionOrRedirect(w, r, entry.ShowID, "floor")
		return
	}
	redirect(w, r)
}

func (a *app) handleAdminShowCreditCreate(w http.ResponseWriter, r *http.Request) {
	showID := r.PathValue("showID")
	r.ParseForm()
	sortOrder, _ := strconv.Atoi(r.FormValue("sort_order"))
	_, err := a.store.createShowCredit(ShowCreditInput{
		ShowID:      showID,
		PersonID:    r.FormValue("person_id"),
		DisplayName: r.FormValue("display_name"),
		CreditLabel: r.FormValue("credit_label"),
		Notes:       r.FormValue("notes"),
		SortOrder:   sortOrder,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	a.sseBroker.publish(showID, "show-updated", `<div class="toast">Show credit added</div>`)
	a.publishAdminSections(showID, "setup")
	a.publishShowSummary(showID)
	a.respondAdminSectionOrRedirect(w, r, showID, "setup")
}

func (a *app) handleAdminShowCreditDelete(w http.ResponseWriter, r *http.Request) {
	creditID := r.PathValue("creditID")
	r.ParseForm()
	showID := r.FormValue("show_id")
	if err := a.store.deleteShowCredit(creditID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	a.sseBroker.publish(showID, "show-updated", `<div class="toast">Show credit removed</div>`)
	a.publishAdminSections(showID, "setup")
	a.respondAdminSectionOrRedirect(w, r, showID, "setup")
}

// --- Persons ---

func (a *app) handleAdminPersons(w http.ResponseWriter, r *http.Request) {
	a.render(w, r, "admin_persons.html", adminPersonsData{
		Title:       "People",
		CurrentPath: "/admin/persons",
		Persons:     a.store.allPersons(),
		Orgs:        a.store.allOrganizations(),
	})
}

func (a *app) handleAdminPersonCreate(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	_, err := a.store.createPerson(PersonInput{
		FirstName:        r.FormValue("first_name"),
		LastName:         r.FormValue("last_name"),
		Email:            r.FormValue("email"),
		OrganizationID:   r.FormValue("organization_id"),
		OrganizationRole: r.FormValue("organization_role"),
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
		a.publishAdminSections(showID, "governance", "setup", "scoring")
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
		a.publishAdminSections(showID, "governance", "setup", "scoring")
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
		a.publishAdminSections(showID, "governance", "setup")
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
		a.publishAdminSections(showID, "governance", "setup")
		a.respondAdminSectionOrRedirect(w, r, showID, "governance")
		return
	}
	for _, show := range a.store.allShows() {
		if sched, ok := a.store.scheduleByShowID(show.ID); ok && sched.EffectiveStandardEditionID == r.FormValue("standard_edition_id") {
			a.publishAdminSections(show.ID, "governance", "setup")
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
	a.publishAdminSections(entry.ShowID, "scoring", "floor", "board", "intake")
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
		a.publishAdminSections(showID, "scoring", "floor", "board", "intake")
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
