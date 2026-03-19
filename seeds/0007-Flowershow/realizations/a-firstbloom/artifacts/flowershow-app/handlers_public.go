package main

import (
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

// --- Home Page ---

type accountRoleView struct {
	RoleLabel  string
	ScopeLabel string
}

type accountData struct {
	Title              string
	CurrentPath        string
	User               *UserIdentity
	IsAdmin            bool
	Notice             accountNotice
	Roles              []accountRoleView
	Shows              []*Show
	AgentTokens        []accountAgentTokenView
	TokenProfiles      []accountTokenProfileView
	SelectedProfileID  string
	DefaultExpiryDays  int
	IssuedAgentToken   *issuedAgentTokenView
	AccountPermissions []accountPermissionView
}

type accountPermissionView struct {
	Key         string
	Label       string
	Description string
}

type accountTokenProfileView struct {
	ID          string
	Label       string
	Description string
	Selected    bool
	Permissions []accountPermissionView
}

type accountAgentTokenView struct {
	ID                string
	Label             string
	TokenPrefix       string
	PermissionProfile string
	StatusLabel       string
	CreatedLabel      string
	ExpiresLabel      string
	LastUsedLabel     string
	Permissions       []accountPermissionView
}

type issuedAgentTokenView struct {
	Label             string
	Secret            string
	TokenPrefix       string
	PermissionProfile string
	ExpiresLabel      string
	Permissions       []accountPermissionView
}

func (a *app) handleAccount(w http.ResponseWriter, r *http.Request) {
	user, ok := a.currentUser(r)
	if !ok {
		http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
		return
	}
	a.render(w, "account.html", a.buildAccountData(*user, accountNoticeMessage(r.URL.Query().Get("notice")), nil, "", 14))
}

func (a *app) buildAccountData(user UserIdentity, notice accountNotice, issued *IssuedAgentToken, selectedProfile string, defaultExpiryDays int) accountData {
	if defaultExpiryDays < agentTokenMinDays || defaultExpiryDays > agentTokenMaxDays {
		defaultExpiryDays = 14
	}

	showLabels := make(map[string]string)
	for _, show := range a.store.allShows() {
		showLabels[show.ID] = show.Name
	}
	orgLabels := make(map[string]string)
	for _, org := range a.store.allOrganizations() {
		orgLabels[org.ID] = org.Name
	}

	roles := make([]accountRoleView, 0)
	for _, role := range a.roleAssignmentsForUser(user) {
		scopeLabel := "Across the platform"
		switch {
		case role.ShowID != "" && showLabels[role.ShowID] != "":
			scopeLabel = showLabels[role.ShowID]
		case role.OrganizationID != "" && orgLabels[role.OrganizationID] != "":
			scopeLabel = orgLabels[role.OrganizationID]
		case role.ShowID != "":
			scopeLabel = role.ShowID
		case role.OrganizationID != "":
			scopeLabel = role.OrganizationID
		}
		roles = append(roles, accountRoleView{
			RoleLabel:  role.Role,
			ScopeLabel: scopeLabel,
		})
	}
	if user.Email != "" && a.bootstrapAdmins[strings.ToLower(user.Email)] {
		roles = append(roles, accountRoleView{
			RoleLabel:  "admin",
			ScopeLabel: "Deployment allowlist",
		})
	}

	profiles := a.availableAgentTokenProfiles(user)
	if selectedProfile == "" && len(profiles) > 0 {
		selectedProfile = profiles[0].ID
	}
	profileViews := make([]accountTokenProfileView, 0, len(profiles))
	accountPermissionMap := make(map[string]accountPermissionView)
	for _, profile := range profiles {
		permissionViews := make([]accountPermissionView, 0, len(profile.Permissions))
		for _, permission := range profile.Permissions {
			item := agentPermissionLookup(permission)
			view := accountPermissionView{
				Key:         item.Key,
				Label:       item.Label,
				Description: item.Description,
			}
			permissionViews = append(permissionViews, view)
			if _, ok := accountPermissionMap[view.Key]; !ok {
				accountPermissionMap[view.Key] = view
			}
		}
		profileViews = append(profileViews, accountTokenProfileView{
			ID:          profile.ID,
			Label:       profile.Label,
			Description: profile.Description,
			Selected:    profile.ID == selectedProfile,
			Permissions: permissionViews,
		})
	}

	accountPermissions := make([]accountPermissionView, 0, len(accountPermissionMap))
	for _, view := range accountPermissionMap {
		accountPermissions = append(accountPermissions, view)
	}
	sortAccountPermissions(accountPermissions)

	tokenViews := make([]accountAgentTokenView, 0)
	for _, token := range a.store.listAgentTokensBySubject(user.CognitoSub) {
		statusLabel := "Active"
		switch {
		case token.RevokedAt != nil:
			statusLabel = "Revoked"
		case !token.ExpiresAt.After(time.Now().UTC()):
			statusLabel = "Expired"
		}
		lastUsedLabel := "Not used yet"
		if token.LastUsedAt != nil {
			lastUsedLabel = token.LastUsedAt.Local().Format("2006-01-02 15:04")
		}
		tokenViews = append(tokenViews, accountAgentTokenView{
			ID:                token.ID,
			Label:             token.Label,
			TokenPrefix:       token.TokenPrefix,
			PermissionProfile: agentProfileLabel(token.PermissionProfile),
			StatusLabel:       statusLabel,
			CreatedLabel:      token.CreatedAt.Local().Format("2006-01-02 15:04"),
			ExpiresLabel:      token.ExpiresAt.Local().Format("2006-01-02 15:04"),
			LastUsedLabel:     lastUsedLabel,
			Permissions:       permissionViewsForKeys(token.Permissions),
		})
	}

	var issuedView *issuedAgentTokenView
	if issued != nil && issued.Token != nil {
		issuedView = &issuedAgentTokenView{
			Label:             issued.Token.Label,
			Secret:            issued.Secret,
			TokenPrefix:       issued.Token.TokenPrefix,
			PermissionProfile: agentProfileLabel(issued.Token.PermissionProfile),
			ExpiresLabel:      issued.Token.ExpiresAt.Local().Format("2006-01-02 15:04"),
			Permissions:       permissionViewsForKeys(issued.Token.Permissions),
		}
	}

	return accountData{
		Title:              "Your Profile",
		CurrentPath:        "/account",
		User:               &user,
		IsAdmin:            a.userIsAdmin(user),
		Notice:             notice,
		Roles:              roles,
		Shows:              a.store.allShows(),
		AgentTokens:        tokenViews,
		TokenProfiles:      profileViews,
		SelectedProfileID:  selectedProfile,
		DefaultExpiryDays:  defaultExpiryDays,
		IssuedAgentToken:   issuedView,
		AccountPermissions: accountPermissions,
	}
}

func permissionViewsForKeys(keys []string) []accountPermissionView {
	views := make([]accountPermissionView, 0, len(keys))
	for _, key := range keys {
		item := agentPermissionLookup(key)
		views = append(views, accountPermissionView{
			Key:         item.Key,
			Label:       item.Label,
			Description: item.Description,
		})
	}
	sortAccountPermissions(views)
	return views
}

func sortAccountPermissions(items []accountPermissionView) {
	sort.Slice(items, func(i, j int) bool {
		return items[i].Label < items[j].Label
	})
}

func agentProfileLabel(profileID string) string {
	if profile, ok := agentPermissionProfiles[profileID]; ok {
		return profile.Label
	}
	return profileID
}

func (a *app) handleAccountTokenCreate(w http.ResponseWriter, r *http.Request) {
	user, ok := a.currentUser(r)
	if !ok {
		http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
		return
	}
	if err := r.ParseForm(); err != nil {
		a.render(w, "account.html", a.buildAccountData(*user, accountNotice{Message: err.Error(), Kind: "error"}, nil, "", 14))
		return
	}
	selectedProfile := strings.TrimSpace(r.FormValue("permission_profile"))
	profile, ok := a.agentTokenProfileForUser(*user, selectedProfile)
	if !ok {
		a.render(w, "account.html", a.buildAccountData(*user, accountNotice{Message: "Choose one of the available permission profiles for this account.", Kind: "error"}, nil, selectedProfile, 14))
		return
	}
	expiryDays, err := strconv.Atoi(strings.TrimSpace(r.FormValue("expires_in_days")))
	if err != nil {
		a.render(w, "account.html", a.buildAccountData(*user, accountNotice{Message: "Enter a valid expiry in days between 1 and 60.", Kind: "error"}, nil, selectedProfile, 14))
		return
	}
	issued, err := a.store.issueAgentToken(AgentTokenIssueInput{
		OwnerCognitoSub:   user.CognitoSub,
		OwnerEmail:        user.Email,
		OwnerName:         user.Name,
		Label:             r.FormValue("label"),
		PermissionProfile: profile.ID,
		Permissions:       profile.Permissions,
		ExpiresInDays:     expiryDays,
	})
	if err != nil {
		a.render(w, "account.html", a.buildAccountData(*user, accountNotice{Message: err.Error(), Kind: "error"}, nil, selectedProfile, expiryDays))
		return
	}
	a.render(w, "account.html", a.buildAccountData(*user, accountNotice{
		Message: "Agent token issued.",
		Kind:    "info",
	}, issued, selectedProfile, expiryDays))
}

func (a *app) handleAccountTokenRevoke(w http.ResponseWriter, r *http.Request) {
	user, ok := a.currentUser(r)
	if !ok {
		http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
		return
	}
	tokenID := r.PathValue("tokenID")
	if _, err := a.store.revokeAgentToken(tokenID, user.CognitoSub); err != nil {
		a.render(w, "account.html", a.buildAccountData(*user, accountNotice{Message: err.Error(), Kind: "error"}, nil, "", 14))
		return
	}
	http.Redirect(w, r, "/account?notice=agent_token_revoked#agent-tokens", http.StatusSeeOther)
}

type homeData struct {
	Title       string
	CurrentPath string
	Shows       []*Show
	Orgs        []*Organization
}

func (a *app) handleHome(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	a.render(w, "home.html", homeData{
		Title:       "Flowershow",
		CurrentPath: "/",
		Shows:       a.store.allShows(),
		Orgs:        a.store.allOrganizations(),
	})
}

// --- Show Detail ---

type showDetailData struct {
	Title       string
	CurrentPath string
	Show        *Show
	Schedule    *ShowSchedule
	Divisions   []*divisionView
	Entries     []*entryView
	Awards      []*AwardDefinition
	Org         *Organization
}

type divisionView struct {
	Division *Division
	Sections []*sectionView
}

type sectionView struct {
	Section *Section
	Classes []*ShowClass
}

type entryView struct {
	Entry  *Entry
	Person *Person
	Class  *ShowClass
	Media  []*Media
}

func (a *app) handleShowDetail(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	show, ok := a.store.showBySlug(slug)
	if !ok {
		http.NotFound(w, r)
		return
	}

	org, _ := a.store.organizationByID(show.OrganizationID)
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
		if !isPublicEntry(e) {
			continue
		}
		person, _ := a.store.personByID(e.PersonID)
		cls, _ := a.store.classByID(e.ClassID)
		entries = append(entries, &entryView{
			Entry:  e,
			Person: person,
			Class:  cls,
			Media:  a.store.mediaByEntry(e.ID),
		})
	}

	awards := a.store.awardsByOrganization(show.OrganizationID)

	a.render(w, "show_detail.html", showDetailData{
		Title:       show.Name,
		CurrentPath: "/shows/" + slug,
		Show:        show,
		Schedule:    sched,
		Divisions:   divisions,
		Entries:     entries,
		Awards:      awards,
		Org:         org,
	})
}

// --- Class Browse ---

type classBrowseData struct {
	Title       string
	CurrentPath string
	Show        *Show
	Divisions   []*divisionView
}

func (a *app) handleClassBrowse(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	show, ok := a.store.showBySlug(slug)
	if !ok {
		http.NotFound(w, r)
		return
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

	a.render(w, "class_browse.html", classBrowseData{
		Title:       "Classes — " + show.Name,
		CurrentPath: "/shows/" + slug + "/classes",
		Show:        show,
		Divisions:   divisions,
	})
}

// --- Class Detail ---

type classDetailData struct {
	Title       string
	CurrentPath string
	Show        *Show
	Class       *ShowClass
	Section     *Section
	Division    *Division
	Entries     []*entryView
}

func (a *app) handleClassDetail(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	classID := r.PathValue("classID")
	show, ok := a.store.showBySlug(slug)
	if !ok {
		http.NotFound(w, r)
		return
	}

	cls, ok := a.store.classByID(classID)
	if !ok {
		http.NotFound(w, r)
		return
	}

	sec, _ := a.store.sectionByID(cls.SectionID)
	var div *Division
	if sec != nil {
		div, _ = a.store.divisionByID(sec.DivisionID)
	}

	var entries []*entryView
	for _, e := range a.store.entriesByClass(classID) {
		if !isPublicEntry(e) {
			continue
		}
		person, _ := a.store.personByID(e.PersonID)
		entries = append(entries, &entryView{
			Entry:  e,
			Person: person,
			Class:  cls,
			Media:  a.store.mediaByEntry(e.ID),
		})
	}

	a.render(w, "class_detail.html", classDetailData{
		Title:       cls.Title + " — " + show.Name,
		CurrentPath: "/shows/" + slug + "/classes/" + classID,
		Show:        show,
		Class:       cls,
		Section:     sec,
		Division:    div,
		Entries:     entries,
	})
}

// --- Entry Detail ---

type entryDetailData struct {
	Title       string
	CurrentPath string
	Entry       *Entry
	Person      *Person
	Class       *ShowClass
	Show        *Show
	Taxons      []*Taxon
	Scorecards  []*scorecardView
	Media       []*Media
}

type scorecardView struct {
	Scorecard *EntryScorecard
	Judge     *Person
	Scores    []*EntryCriterionScore
}

func (a *app) handleEntryDetail(w http.ResponseWriter, r *http.Request) {
	entryID := r.PathValue("entryID")
	entry, ok := a.store.entryByID(entryID)
	if !ok || !isPublicEntry(entry) {
		http.NotFound(w, r)
		return
	}

	person, _ := a.store.personByID(entry.PersonID)
	cls, _ := a.store.classByID(entry.ClassID)
	show, _ := a.store.showByID(entry.ShowID)

	var taxons []*Taxon
	for _, ref := range entry.TaxonRefs {
		if t, ok := a.store.taxonByID(ref); ok {
			taxons = append(taxons, t)
		}
	}

	var scorecardViews []*scorecardView
	for _, sc := range a.store.scorecardsByEntry(entryID) {
		judge, _ := a.store.personByID(sc.JudgeID)
		scores := a.store.criterionScoresByScorecard(sc.ID)
		scorecardViews = append(scorecardViews, &scorecardView{
			Scorecard: sc,
			Judge:     judge,
			Scores:    scores,
		})
	}

	a.render(w, "entry_detail.html", entryDetailData{
		Title:       entry.Name,
		CurrentPath: "/entries/" + entryID,
		Entry:       entry,
		Person:      person,
		Class:       cls,
		Show:        show,
		Taxons:      taxons,
		Scorecards:  scorecardViews,
		Media:       a.store.mediaByEntry(entry.ID),
	})
}

// --- Person History ---

type personDetailData struct {
	Title       string
	CurrentPath string
	Person      *Person
	Entries     []*entryView
	EntryCount  int
	FirstCount  int
	TotalPoints float64
}

func (a *app) handlePersonDetail(w http.ResponseWriter, r *http.Request) {
	personID := r.PathValue("personID")
	person, ok := a.store.personByID(personID)
	if !ok {
		http.NotFound(w, r)
		return
	}

	var entries []*entryView
	var firstCount int
	var totalPoints float64
	for _, entry := range a.store.entriesByPerson(personID) {
		if !isPublicEntry(entry) {
			continue
		}
		cls, _ := a.store.classByID(entry.ClassID)
		totalPoints += entry.Points
		if entry.Placement == 1 {
			firstCount++
		}
		entries = append(entries, &entryView{
			Entry:  entry,
			Person: person,
			Class:  cls,
			Media:  a.store.mediaByEntry(entry.ID),
		})
	}

	a.render(w, "person_detail.html", personDetailData{
		Title:       "History — " + person.Initials,
		CurrentPath: "/people/" + personID,
		Person:      person,
		Entries:     entries,
		EntryCount:  len(entries),
		FirstCount:  firstCount,
		TotalPoints: totalPoints,
	})
}

// --- Taxonomy ---

type taxonomyData struct {
	Title       string
	CurrentPath string
	Taxons      []*Taxon
}

func (a *app) handleTaxonomyBrowse(w http.ResponseWriter, r *http.Request) {
	a.render(w, "taxonomy_browse.html", taxonomyData{
		Title:       "Taxonomy",
		CurrentPath: "/taxonomy",
		Taxons:      a.store.allTaxons(),
	})
}

type taxonDetailData struct {
	Title       string
	CurrentPath string
	Taxon       *Taxon
	Children    []*Taxon
	Entries     []*entryView
}

func (a *app) handleTaxonDetail(w http.ResponseWriter, r *http.Request) {
	taxonID := r.PathValue("taxonID")
	taxon, ok := a.store.taxonByID(taxonID)
	if !ok {
		http.NotFound(w, r)
		return
	}

	// Find child taxons
	var children []*Taxon
	for _, t := range a.store.allTaxons() {
		if t.ParentID == taxonID {
			children = append(children, t)
		}
	}

	// Find entries referencing this taxon
	var entries []*entryView
	for _, show := range a.store.allShows() {
		for _, e := range a.store.entriesByShow(show.ID) {
			if !isPublicEntry(e) {
				continue
			}
			for _, ref := range e.TaxonRefs {
				if ref == taxonID {
					person, _ := a.store.personByID(e.PersonID)
					cls, _ := a.store.classByID(e.ClassID)
					entries = append(entries, &entryView{
						Entry:  e,
						Person: person,
						Class:  cls,
						Media:  a.store.mediaByEntry(e.ID),
					})
					break
				}
			}
		}
	}

	a.render(w, "taxon_detail.html", taxonDetailData{
		Title:       taxon.Name,
		CurrentPath: "/taxonomy/" + taxonID,
		Taxon:       taxon,
		Children:    children,
		Entries:     entries,
	})
}

// --- Leaderboard ---

type leaderboardData struct {
	Title       string
	CurrentPath string
	Entries     []LeaderboardEntry
	Boards      []*leaderboardBoard
	OrgName     string
	Season      string
	Orgs        []*Organization
	SelectedOrg string
}

func (a *app) handleLeaderboard(w http.ResponseWriter, r *http.Request) {
	orgs := a.store.allOrganizations()
	orgID := r.URL.Query().Get("org")
	season := r.URL.Query().Get("season")
	if season == "" {
		season = "2025"
	}

	var orgName string
	var entries []LeaderboardEntry
	var boards []*leaderboardBoard
	if orgID == "" {
		orgID = "all"
	}
	if orgID == "all" {
		orgName = "All Organizations"
		boards = a.leaderboardBoards(season)
	} else {
		if org, ok := a.store.organizationByID(orgID); ok {
			orgName = org.Name
		}
		entries = a.store.leaderboard(orgID, season)
	}

	a.render(w, "leaderboard.html", leaderboardData{
		Title:       "Leaderboard",
		CurrentPath: "/leaderboard",
		Entries:     entries,
		Boards:      boards,
		OrgName:     orgName,
		Season:      season,
		Orgs:        orgs,
		SelectedOrg: orgID,
	})
}

// --- Winner Summary ---

func (a *app) handleShowSummary(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	data, err := a.winnerSummaryDataBySlug(slug)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	a.render(w, "show_summary.html", data)
}

func (a *app) handleShowSummaryStream(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	show, ok := a.store.showBySlug(slug)
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}
	ch := a.sseBroker.subscribe(show.ID)
	defer a.sseBroker.unsubscribe(show.ID, ch)
	_, _ = w.Write([]byte(": connected\n\n"))
	flusher.Flush()
	for {
		select {
		case <-r.Context().Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			_, _ = w.Write([]byte(msg))
			flusher.Flush()
		}
	}
}

// --- Browse ---

func (a *app) handleBrowse(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	orgID := r.URL.Query().Get("org")
	taxonID := r.URL.Query().Get("taxon")
	judgeID := r.URL.Query().Get("judge")
	domain := r.URL.Query().Get("domain")
	var judges []*Person
	seenJudges := map[string]bool{}
	for _, show := range a.store.allShows() {
		for _, assignment := range a.store.judgesByShow(show.ID) {
			if seenJudges[assignment.PersonID] {
				continue
			}
			if judge, ok := a.store.personByID(assignment.PersonID); ok {
				judges = append(judges, judge)
				seenJudges[assignment.PersonID] = true
			}
		}
	}
	a.render(w, "browse.html", browseData{
		Title:           "Browse",
		CurrentPath:     "/browse",
		Query:           query,
		SelectedOrgID:   orgID,
		SelectedTaxonID: taxonID,
		SelectedJudgeID: judgeID,
		SelectedDomain:  domain,
		Orgs:            a.store.allOrganizations(),
		Taxons:          a.store.allTaxons(),
		Judges:          judges,
		Results:         a.browseResults(query, orgID, taxonID, judgeID, domain),
	})
}

// --- Standards ---

type standardsData struct {
	Title       string
	CurrentPath string
	Standards   []*standardView
}

func (a *app) handleStandards(w http.ResponseWriter, r *http.Request) {
	a.render(w, "standards.html", standardsData{
		Title:       "Standards",
		CurrentPath: "/standards",
		Standards:   a.standardViews(),
	})
}

// --- Show Rules ---

type showRulesData struct {
	Title       string
	CurrentPath string
	Show        *Show
	Rules       []effectiveRule
	Schedule    *ShowSchedule
}

func (a *app) handleShowRules(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	show, ok := a.store.showBySlug(slug)
	if !ok {
		http.NotFound(w, r)
		return
	}

	sched, _ := a.store.scheduleByShowID(show.ID)
	var rules []effectiveRule
	if sched != nil && sched.EffectiveStandardEditionID != "" {
		for _, cls := range a.store.classesByShowID(show.ID) {
			classRules := a.store.effectiveRulesForClass(cls.ID, sched.EffectiveStandardEditionID)
			rules = append(rules, classRules...)
		}
	}

	a.render(w, "show_rules.html", showRulesData{
		Title:       "Rules — " + show.Name,
		CurrentPath: "/shows/" + slug + "/rules",
		Show:        show,
		Rules:       rules,
		Schedule:    sched,
	})
}
