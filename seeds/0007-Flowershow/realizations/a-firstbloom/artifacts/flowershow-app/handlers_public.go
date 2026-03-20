package main

import (
	"context"
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
	Title                string
	CurrentPath          string
	User                 *UserIdentity
	IsAdmin              bool
	ActiveSection        string
	Sections             []accountSectionView
	Notice               accountNotice
	Roles                []accountRoleView
	Shows                []*Show
	AgentTokens          []accountAgentTokenView
	TokenProfiles        []accountTokenProfileView
	SelectedProfileID    string
	DefaultExpiryDays    int
	IssuedAgentToken     *issuedAgentTokenView
	AccountPermissions   []accountPermissionView
	ManagedClubs         []managedClubView
	ClubMemberships      []accountClubMembershipView
	Entries              []accountEntryView
	Profile              accountProfileEditorView
	CurrentSeason        string
	CurrentSeasonPoints  float64
	CurrentSeasonEntries int
}

type accountSectionView struct {
	ID      string
	Label   string
	Href    string
	Current bool
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

type managedClubView struct {
	ID        string
	Name      string
	Level     string
	AdminHref string
}

type accountClubMembershipView struct {
	ID        string
	Name      string
	Level     string
	RoleLabel string
	AdminHref string
}

type accountEntryView struct {
	EntryID      string
	ShowName     string
	ShowHref     string
	ClassLabel   string
	EntryName    string
	EntryHref    string
	PointsLabel  string
	Placement    string
	CreatedLabel string
	Suppressed   bool
}

type accountProfileEditorView struct {
	PersonID                   string
	FirstName                  string
	LastName                   string
	Email                      string
	PublicDisplayMode          string
	InitialsSample             string
	FirstNameLastInitialSample string
	FullNameSample             string
	HasStoredProfile           bool
}

func (a *app) handleAccount(w http.ResponseWriter, r *http.Request) {
	user, ok := a.currentUser(r)
	if !ok {
		http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
		return
	}
	section := accountSection(r.URL.Query().Get("section"))
	a.render(w, r, "account.html", a.buildAccountData(*user, section, accountNoticeMessage(r.URL.Query().Get("notice")), nil, "", 14))
}

func (a *app) buildAccountData(user UserIdentity, section string, notice accountNotice, issued *IssuedAgentToken, selectedProfile string, defaultExpiryDays int) accountData {
	if defaultExpiryDays < agentTokenMinDays || defaultExpiryDays > agentTokenMaxDays {
		defaultExpiryDays = 14
	}
	section = accountSection(section)

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

	currentSeason, seasonPoints, seasonEntries := a.accountSeasonStats(user)
	clubMemberships := a.clubMembershipsForUser(user)
	profile := a.accountProfileForUser(user)
	entries := a.accountEntriesForUser(user)
	sections := accountSections(section, a.userIsAdmin(user))

	return accountData{
		Title:                "Your Profile",
		CurrentPath:          "/account",
		User:                 &user,
		IsAdmin:              a.userIsAdmin(user),
		ActiveSection:        section,
		Sections:             sections,
		Notice:               notice,
		Roles:                roles,
		Shows:                a.store.allShows(),
		AgentTokens:          tokenViews,
		TokenProfiles:        profileViews,
		SelectedProfileID:    selectedProfile,
		DefaultExpiryDays:    defaultExpiryDays,
		IssuedAgentToken:     issuedView,
		AccountPermissions:   accountPermissions,
		ManagedClubs:         a.managedClubsForUser(user),
		ClubMemberships:      clubMemberships,
		Entries:              entries,
		Profile:              profile,
		CurrentSeason:        currentSeason,
		CurrentSeasonPoints:  seasonPoints,
		CurrentSeasonEntries: seasonEntries,
	}
}

func (a *app) accountProfileForUser(user UserIdentity) accountProfileEditorView {
	profile := accountProfileEditorView{
		Email:             user.Email,
		PublicDisplayMode: "initials",
	}
	if person, ok := a.store.personByEmail(user.Email); ok && person != nil {
		profile.PersonID = person.ID
		profile.FirstName = person.FirstName
		profile.LastName = person.LastName
		profile.Email = person.Email
		profile.HasStoredProfile = true
		if strings.TrimSpace(person.PublicDisplayMode) != "" {
			profile.PublicDisplayMode = person.PublicDisplayMode
		}
	} else if strings.TrimSpace(user.Name) != "" {
		parts := strings.Fields(user.Name)
		if len(parts) > 0 {
			profile.FirstName = parts[0]
		}
		if len(parts) > 1 {
			profile.LastName = strings.Join(parts[1:], " ")
		}
	}
	profile.InitialsSample = sampleInitials(profile.FirstName, profile.LastName)
	profile.FirstNameLastInitialSample = sampleFirstNameLastInitial(profile.FirstName, profile.LastName)
	fullName := strings.TrimSpace(profile.FirstName + " " + profile.LastName)
	if fullName == "" {
		fullName = "Your full name"
	}
	profile.FullNameSample = fullName
	return profile
}

func sampleInitials(firstName, lastName string) string {
	firstRunes := []rune(strings.TrimSpace(firstName))
	lastRunes := []rune(strings.TrimSpace(lastName))
	initials := ""
	if len(firstRunes) > 0 {
		initials += string(firstRunes[:1])
	}
	if len(lastRunes) > 0 {
		initials += string(lastRunes[:1])
	}
	if initials == "" {
		initials = "AB"
	}
	return strings.ToUpper(initials)
}

func publicPersonLabel(person *Person) string {
	if person == nil {
		return ""
	}
	switch strings.TrimSpace(person.PublicDisplayMode) {
	case "full_name":
		if full := strings.TrimSpace(person.FirstName + " " + person.LastName); full != "" {
			return full
		}
	case "first_name_last_initial":
		if label := sampleFirstNameLastInitial(person.FirstName, person.LastName); label != "" {
			return label
		}
	}
	return person.Initials
}

func sampleFirstNameLastInitial(firstName, lastName string) string {
	firstName = strings.TrimSpace(firstName)
	lastRunes := []rune(strings.TrimSpace(lastName))
	switch {
	case firstName != "" && len(lastRunes) > 0:
		return firstName + " " + strings.ToUpper(string(lastRunes[:1])) + "."
	case firstName != "":
		return firstName
	default:
		return "Alex B."
	}
}

func placementLabelText(placement int) string {
	switch placement {
	case 1:
		return "1st"
	case 2:
		return "2nd"
	case 3:
		return "3rd"
	default:
		return ""
	}
}

func (a *app) accountEntriesForUser(user UserIdentity) []accountEntryView {
	person, ok := a.store.personByEmail(user.Email)
	if !ok || person == nil {
		return nil
	}
	out := make([]accountEntryView, 0)
	for _, entry := range a.store.entriesByPerson(person.ID) {
		show, _ := a.store.showByID(entry.ShowID)
		class, _ := a.store.classByID(entry.ClassID)
		view := accountEntryView{
			EntryID:      entry.ID,
			EntryName:    entry.Name,
			EntryHref:    "/entries/" + entry.ID,
			Suppressed:   entry.Suppressed,
			CreatedLabel: entry.CreatedAt.Local().Format("2006-01-02"),
		}
		if show != nil {
			view.ShowName = show.Name
			view.ShowHref = "/shows/" + show.Slug
		}
		if class != nil {
			view.ClassLabel = strings.TrimSpace(class.ClassNumber + " · " + class.Title)
		}
		if entry.Points > 0 {
			view.PointsLabel = strconv.FormatFloat(entry.Points, 'f', -1, 64) + " pts"
		}
		if entry.Placement > 0 {
			view.Placement = placementLabelText(entry.Placement)
		}
		out = append(out, view)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedLabel == out[j].CreatedLabel {
			return out[i].EntryName < out[j].EntryName
		}
		return out[i].CreatedLabel > out[j].CreatedLabel
	})
	return out
}

func (a *app) managedClubsForUser(user UserIdentity) []managedClubView {
	items := make([]managedClubView, 0)
	for _, org := range a.store.allOrganizations() {
		if org == nil {
			continue
		}
		if !a.userHasCapability(context.Background(), user, "organization.manage", authorityScope{Kind: "organization", ID: org.ID}) {
			continue
		}
		items = append(items, managedClubView{
			ID:        org.ID,
			Name:      org.Name,
			Level:     org.Level,
			AdminHref: "/admin/clubs/" + org.ID,
		})
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].Name < items[j].Name
	})
	return items
}

func (a *app) clubMembershipsForUser(user UserIdentity) []accountClubMembershipView {
	person, ok := a.store.personByEmail(user.Email)
	if !ok || person == nil {
		return nil
	}
	items := make([]accountClubMembershipView, 0)
	for _, link := range a.store.personOrganizationsByPerson(person.ID) {
		if link == nil {
			continue
		}
		org, ok := a.store.organizationByID(link.OrganizationID)
		if !ok || org == nil {
			continue
		}
		view := accountClubMembershipView{
			ID:        org.ID,
			Name:      org.Name,
			Level:     org.Level,
			RoleLabel: strings.Title(strings.TrimSpace(link.Role)),
		}
		if a.userHasCapability(context.Background(), user, "organization.manage", authorityScope{Kind: "organization", ID: org.ID}) {
			view.AdminHref = "/admin/clubs/" + org.ID
		}
		items = append(items, view)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Name == items[j].Name {
			return items[i].RoleLabel < items[j].RoleLabel
		}
		return items[i].Name < items[j].Name
	})
	return items
}

func (a *app) accountSeasonStats(user UserIdentity) (string, float64, int) {
	person, ok := a.store.personByEmail(user.Email)
	if !ok || person == nil {
		return time.Now().Format("2006"), 0, 0
	}
	season := time.Now().Format("2006")
	var points float64
	var entries int
	for _, entry := range a.store.entriesByPerson(person.ID) {
		show, ok := a.store.showByID(entry.ShowID)
		if !ok || show == nil || strings.TrimSpace(show.Season) != season {
			continue
		}
		entries++
		points += entry.Points
	}
	return season, points, entries
}

func accountSection(raw string) string {
	switch strings.TrimSpace(raw) {
	case "shows", "clubs", "entries", "access":
		return strings.TrimSpace(raw)
	case "tokens":
		return "access"
	default:
		return "overview"
	}
}

func accountSections(active string, isAdmin bool) []accountSectionView {
	items := []accountSectionView{
		{ID: "overview", Label: "Overview", Href: "/account?section=overview", Current: active == "overview"},
		{ID: "shows", Label: "Shows", Href: "/account?section=shows", Current: active == "shows"},
		{ID: "clubs", Label: "Clubs", Href: "/account?section=clubs", Current: active == "clubs"},
		{ID: "entries", Label: "Entries", Href: "/account?section=entries", Current: active == "entries"},
		{ID: "access", Label: "Access Tokens", Href: "/account?section=access#agent-tokens", Current: active == "access"},
	}
	if isAdmin {
		items = append(items, accountSectionView{ID: "admin", Label: "Admin Workspace", Href: "/admin", Current: false})
	}
	return items
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
		a.render(w, r, "account.html", a.buildAccountData(*user, "access", accountNotice{Message: err.Error(), Kind: "error"}, nil, "", 14))
		return
	}
	selectedProfile := strings.TrimSpace(r.FormValue("permission_profile"))
	profile, ok := a.agentTokenProfileForUser(*user, selectedProfile)
	if !ok {
		a.render(w, r, "account.html", a.buildAccountData(*user, "access", accountNotice{Message: "Choose one of the available permission profiles for this account.", Kind: "error"}, nil, selectedProfile, 14))
		return
	}
	expiryDays, err := strconv.Atoi(strings.TrimSpace(r.FormValue("expires_in_days")))
	if err != nil {
		a.render(w, r, "account.html", a.buildAccountData(*user, "access", accountNotice{Message: "Enter a valid expiry in days between 1 and 60.", Kind: "error"}, nil, selectedProfile, 14))
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
		a.render(w, r, "account.html", a.buildAccountData(*user, "access", accountNotice{Message: err.Error(), Kind: "error"}, nil, selectedProfile, expiryDays))
		return
	}
	a.render(w, r, "account.html", a.buildAccountData(*user, "access", accountNotice{
		Message: "Agent token issued.",
		Kind:    "info",
	}, issued, selectedProfile, expiryDays))
}

func (a *app) handleAccountProfileUpdate(w http.ResponseWriter, r *http.Request) {
	user, ok := a.currentUser(r)
	if !ok {
		http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
		return
	}
	if err := r.ParseForm(); err != nil {
		a.render(w, r, "account.html", a.buildAccountData(*user, "overview", accountNotice{Message: err.Error(), Kind: "error"}, nil, "", 14))
		return
	}
	firstName := strings.TrimSpace(r.FormValue("first_name"))
	lastName := strings.TrimSpace(r.FormValue("last_name"))
	email := strings.TrimSpace(r.FormValue("email"))
	if email == "" {
		email = user.Email
	}
	displayMode := strings.TrimSpace(r.FormValue("public_display_mode"))
	switch displayMode {
	case "full_name", "first_name_last_initial":
	default:
		displayMode = "initials"
	}
	input := PersonInput{
		FirstName:         firstName,
		LastName:          lastName,
		Email:             email,
		PublicDisplayMode: displayMode,
	}
	person, found := a.store.personByEmail(user.Email)
	if found && person != nil {
		if _, err := a.store.updatePerson(person.ID, input); err != nil {
			a.render(w, r, "account.html", a.buildAccountData(*user, "overview", accountNotice{Message: err.Error(), Kind: "error"}, nil, "", 14))
			return
		}
	} else {
		if _, err := a.store.createPerson(input); err != nil {
			a.render(w, r, "account.html", a.buildAccountData(*user, "overview", accountNotice{Message: err.Error(), Kind: "error"}, nil, "", 14))
			return
		}
	}
	updatedUser := *user
	updatedUser.Email = email
	if full := strings.TrimSpace(firstName + " " + lastName); full != "" {
		updatedUser.Name = full
	}
	if err := a.setUserSession(w, r, updatedUser); err != nil {
		a.render(w, r, "account.html", a.buildAccountData(*user, "overview", accountNotice{Message: err.Error(), Kind: "error"}, nil, "", 14))
		return
	}
	http.Redirect(w, r, "/account?section=overview&notice=profile_updated", http.StatusSeeOther)
}

func (a *app) handleAccountTokenRevoke(w http.ResponseWriter, r *http.Request) {
	user, ok := a.currentUser(r)
	if !ok {
		http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
		return
	}
	tokenID := r.PathValue("tokenID")
	if _, err := a.store.revokeAgentToken(tokenID, user.CognitoSub); err != nil {
		a.render(w, r, "account.html", a.buildAccountData(*user, "access", accountNotice{Message: err.Error(), Kind: "error"}, nil, "", 14))
		return
	}
	http.Redirect(w, r, "/account?section=access&notice=agent_token_revoked#agent-tokens", http.StatusSeeOther)
}

func (a *app) handleHome(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	upcoming, past := a.homeShowCards(time.Now())
	totalEntries := 0
	for _, show := range a.store.allShows() {
		totalEntries += len(a.store.entriesByShow(show.ID))
	}
	a.render(w, r, "home.html", homeData{
		Title:         "Flowershow",
		CurrentPath:   "/",
		UpcomingShows: upcoming,
		PastShows:     past,
		Clubs:         a.clubCards(time.Now()),
		TotalShows:    len(a.store.allShows()),
		TotalEntries:  totalEntries,
		TotalMembers:  len(a.store.allPersons()),
	})
}

func (a *app) handleClubs(w http.ResponseWriter, r *http.Request) {
	a.render(w, r, "clubs.html", clubsData{
		Title:       "Clubs",
		CurrentPath: "/clubs",
		Clubs:       a.clubCards(time.Now()),
	})
}

func (a *app) handleClubDetail(w http.ResponseWriter, r *http.Request) {
	data, ok := a.clubDetailData(strings.TrimSpace(r.PathValue("organizationID")), time.Now())
	if !ok {
		http.NotFound(w, r)
		return
	}
	a.render(w, r, "club_detail.html", data)
}

func (a *app) handleClassesIndex(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	a.render(w, r, "classes.html", classesIndexData{
		Title:       "Classes",
		CurrentPath: "/classes",
		Query:       query,
		Domains:     a.classesIndexDomains(query),
	})
}

// --- Show Detail ---

type showDetailData struct {
	Title       string
	CurrentPath string
	ShowID      string
	Show        *Show
	Schedule    *ShowSchedule
	Divisions   []*divisionView
	Entries     []*entryView
	Awards      []*AwardDefinition
	ShowCredits []*showCreditView
	Org         *Organization
}

type divisionView struct {
	Division *Division
	Sections []*sectionView
}

type sectionView struct {
	Section    *Section
	Classes    []*ShowClass
	ClassCards []*publicClassListItem
}

type entryView struct {
	Entry     *Entry
	Person    *Person
	Class     *ShowClass
	Media     []*Media
	LeadMedia *Media
}

type publicClassListItem struct {
	Class      *ShowClass
	EntryCount int
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
	entryCountByClass := map[string]int{}

	var divisions []*divisionView
	var entries []*entryView
	for _, e := range a.store.entriesByShow(show.ID) {
		if !isPublicEntry(e) {
			continue
		}
		person, _ := a.store.personByID(e.PersonID)
		cls, _ := a.store.classByID(e.ClassID)
		media := a.store.mediaByEntry(e.ID)
		var leadMedia *Media
		if len(media) > 0 {
			leadMedia = media[0]
		}
		if cls != nil {
			entryCountByClass[cls.ID]++
		}
		entries = append(entries, &entryView{
			Entry:     e,
			Person:    person,
			Class:     cls,
			Media:     media,
			LeadMedia: leadMedia,
		})
	}
	if sched != nil {
		for _, div := range a.store.divisionsBySchedule(sched.ID) {
			dv := &divisionView{Division: div}
			for _, sec := range a.store.sectionsByDivision(div.ID) {
				sv := &sectionView{
					Section: sec,
					Classes: a.store.classesBySection(sec.ID),
				}
				for _, class := range sv.Classes {
					sv.ClassCards = append(sv.ClassCards, &publicClassListItem{
						Class:      class,
						EntryCount: entryCountByClass[class.ID],
					})
				}
				dv.Sections = append(dv.Sections, sv)
			}
			divisions = append(divisions, dv)
		}
	}
	var showCredits []*showCreditView
	for _, credit := range a.store.showCreditsByShow(show.ID) {
		person, _ := a.store.personByID(credit.PersonID)
		showCredits = append(showCredits, &showCreditView{
			Credit: credit,
			Person: person,
		})
	}

	awards := a.store.awardsByOrganization(show.OrganizationID)

	a.render(w, r, "show_detail.html", showDetailData{
		Title:       show.Name,
		CurrentPath: "/shows/" + slug,
		ShowID:      show.ID,
		Show:        show,
		Schedule:    sched,
		Divisions:   divisions,
		Entries:     entries,
		Awards:      awards,
		ShowCredits: showCredits,
		Org:         org,
	})
}

// --- Class Browse ---

type classBrowseData struct {
	Title       string
	CurrentPath string
	ShowID      string
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

	a.render(w, r, "class_browse.html", classBrowseData{
		Title:       "Classes — " + show.Name,
		CurrentPath: "/shows/" + slug + "/classes",
		ShowID:      show.ID,
		Show:        show,
		Divisions:   divisions,
	})
}

// --- Class Detail ---

type classDetailData struct {
	Title       string
	CurrentPath string
	ShowID      string
	ClassID     string
	Show        *Show
	Class       *ShowClass
	Section     *Section
	Division    *Division
	Entries     []*entryView
	HasMedia    bool
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
	hasMedia := false
	for _, e := range a.store.entriesByClass(classID) {
		if !isPublicEntry(e) {
			continue
		}
		person, _ := a.store.personByID(e.PersonID)
		media := a.store.mediaByEntry(e.ID)
		var leadMedia *Media
		if len(media) > 0 {
			leadMedia = media[0]
			hasMedia = true
		}
		entries = append(entries, &entryView{
			Entry:     e,
			Person:    person,
			Class:     cls,
			Media:     media,
			LeadMedia: leadMedia,
		})
	}

	a.render(w, r, "class_detail.html", classDetailData{
		Title:       cls.Title + " — " + show.Name,
		CurrentPath: "/shows/" + slug + "/classes/" + classID,
		ShowID:      show.ID,
		ClassID:     cls.ID,
		Show:        show,
		Class:       cls,
		Section:     sec,
		Division:    div,
		Entries:     entries,
		HasMedia:    hasMedia,
	})
}

// --- Entry Detail ---

type entryDetailData struct {
	Title       string
	CurrentPath string
	EntryID     string
	ShowID      string
	ClassID     string
	PersonID    string
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

	a.render(w, r, "entry_detail.html", entryDetailData{
		Title:       entry.Name,
		CurrentPath: "/entries/" + entryID,
		EntryID:     entry.ID,
		ShowID:      entry.ShowID,
		ClassID:     entry.ClassID,
		PersonID:    entry.PersonID,
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

	a.render(w, r, "person_detail.html", personDetailData{
		Title:       "History — " + publicPersonLabel(person),
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
	a.render(w, r, "taxonomy_browse.html", taxonomyData{
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

	a.render(w, r, "taxon_detail.html", taxonDetailData{
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

	a.render(w, r, "leaderboard.html", leaderboardData{
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
	a.render(w, r, "show_summary.html", data)
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
	a.render(w, r, "browse.html", browseData{
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
	a.render(w, r, "standards.html", standardsData{
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

	a.render(w, r, "show_rules.html", showRulesData{
		Title:       "Rules — " + show.Name,
		CurrentPath: "/shows/" + slug + "/rules",
		Show:        show,
		Rules:       rules,
		Schedule:    sched,
	})
}
