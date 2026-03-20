package main

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"time"
)

const maxFormMemory = 1 << 20

//go:embed templates/*
var templates embed.FS

//go:embed assets/*
var assets embed.FS

type app struct {
	store         flowershowStore
	authority     runtimeAuthorityResolver
	templates     map[string]*template.Template
	serviceToken  string
	sseBroker     *sseBroker
	auth          authProvider
	media         mediaStore
	sessions      authStateStore
	allowTestAuth bool
}

type refreshingStore interface {
	Refresh(context.Context) error
}

func main() {
	addr := envOrDefault("AS_ADDR", "127.0.0.1:8097")
	store, err := newFlowershowStore(envOrDefault("AS_RUNTIME_DATABASE_URL", ""))
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()
	auth, err := newAuthProviderFromEnv()
	if err != nil {
		log.Fatal(err)
	}
	media, err := newMediaStore()
	if err != nil {
		log.Fatal(err)
	}

	a := &app{
		store:         store,
		authority:     newRuntimeAuthorityResolver(store),
		templates:     parseTemplates(),
		serviceToken:  strings.TrimSpace(os.Getenv("AS_SERVICE_TOKEN")),
		sseBroker:     newSSEBroker(),
		auth:          auth,
		media:         media,
		sessions:      newAuthStateStore(store, auth),
		allowTestAuth: strings.TrimSpace(os.Getenv("AS_ALLOW_TEST_AUTH")) == "1",
	}
	if a.authority != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := a.authority.Init(ctx, store); err != nil {
			log.Printf("flowershow runtime authority init failed: %v", err)
		}
		cancel()
	}

	mux := http.NewServeMux()

	// Assets & health
	mux.Handle("GET /assets/", a.assetHandler())
	mux.HandleFunc("GET /healthz", a.handleHealth)
	mux.HandleFunc("GET /v1/contracts", a.handleContractsList)
	mux.HandleFunc("GET /v1/contracts/{seed_id}/{realization_id}", a.handleContractDetail)
	if a.allowTestAuth {
		mux.HandleFunc("POST /__test/session", a.handleTestSessionCreate)
	}

	// Public pages
	mux.HandleFunc("GET /", a.handleHome)
	mux.HandleFunc("GET /clubs", a.handleClubs)
	mux.HandleFunc("GET /classes", a.handleClassesIndex)
	mux.HandleFunc("GET /account", a.requireSignedInPage(a.handleAccount))
	mux.HandleFunc("GET /profile", a.requireSignedInPage(a.handleAccount))
	mux.HandleFunc("POST /account/profile", a.requireSignedInPage(a.handleAccountProfileUpdate))
	mux.HandleFunc("POST /account/agent-tokens", a.requireSignedInPage(a.handleAccountTokenCreate))
	mux.HandleFunc("POST /account/agent-tokens/{tokenID}/revoke", a.requireSignedInPage(a.handleAccountTokenRevoke))
	mux.HandleFunc("GET /shows/{slug}", a.handleShowDetail)
	mux.HandleFunc("GET /shows/{slug}/classes", a.handleClassBrowse)
	mux.HandleFunc("GET /shows/{slug}/classes/{classID}", a.handleClassDetail)
	mux.HandleFunc("GET /shows/{slug}/summary", a.handleShowSummary)
	mux.HandleFunc("GET /shows/{slug}/summary/stream", a.handleShowSummaryStream)
	mux.HandleFunc("GET /shows/{slug}/rules", a.handleShowRules)
	mux.HandleFunc("GET /entries/{entryID}", a.handleEntryDetail)
	mux.HandleFunc("GET /people/{personID}", a.handlePersonDetail)
	mux.HandleFunc("GET /browse", a.handleBrowse)
	mux.HandleFunc("GET /taxonomy", a.handleTaxonomyBrowse)
	mux.HandleFunc("GET /taxonomy/{taxonID}", a.handleTaxonDetail)
	mux.HandleFunc("GET /leaderboard", a.handleLeaderboard)
	mux.HandleFunc("GET /standards", a.handleStandards)
	mux.HandleFunc("GET /media/{mediaID}", a.handleMediaOpen)

	// Admin auth
	mux.HandleFunc("GET /admin/login", a.handleAdminLogin)
	mux.HandleFunc("POST /admin/logout", a.handleAdminLogout)
	mux.HandleFunc("POST /auth/login/back", a.handleAdminLoginBack)
	mux.HandleFunc("POST /auth/login/password", a.handleAdminPasswordLogin)
	mux.HandleFunc("POST /auth/login/email-code", a.handleAdminEmailCodeStart)
	mux.HandleFunc("POST /auth/login/email-code/verify", a.handleAdminEmailCodeVerify)
	mux.HandleFunc("POST /auth/login/forgot-password", a.handleAdminForgotPasswordStart)
	mux.HandleFunc("POST /auth/login/forgot-password/confirm", a.handleAdminForgotPasswordConfirm)
	mux.HandleFunc("GET /auth/login", a.handleCognitoLogin)
	mux.HandleFunc("GET /auth/callback", a.handleCognitoCallback)
	mux.HandleFunc("POST /auth/logout", a.handleCognitoLogout)

	// Admin pages
	mux.HandleFunc("GET /admin", a.requireAdmin(a.handleAdminDashboard))
	mux.HandleFunc("GET /admin/roles", a.requireAdmin(a.handleRoleManagement))
	mux.HandleFunc("POST /admin/roles", a.requireAdmin(a.handleRoleAssign))
	mux.HandleFunc("GET /admin/clubs/new", a.requireAdmin(a.handleAdminClubNew))
	mux.HandleFunc("POST /admin/clubs", a.requireAdmin(a.handleAdminClubCreate))
	mux.HandleFunc("GET /admin/clubs/{organizationID}", a.requireCapabilityPage("organization.manage", a.handleAdminClubDetail))
	mux.HandleFunc("POST /admin/clubs/{organizationID}/invites", a.requireCapabilityPage("organization.invites.manage", a.handleAdminClubInviteCreate))

	// Admin shows
	mux.HandleFunc("GET /admin/shows/new", a.requireAdmin(a.handleAdminShowNew))
	mux.HandleFunc("POST /admin/shows", a.requireAdmin(a.handleAdminShowCreate))
	mux.HandleFunc("GET /admin/shows/{showID}", a.requireCapabilityPage("shows.workspace.read", a.handleAdminShowDetail))
	mux.HandleFunc("GET /admin/shows/{showID}/fragments/{section}", a.requireCapabilityPage("shows.workspace.read", a.handleAdminShowFragment))
	mux.HandleFunc("POST /admin/shows/{showID}", a.requireCapabilityPage("shows.manage", a.handleAdminShowUpdate))
	mux.HandleFunc("POST /admin/shows/{showID}/judges", a.requireCapabilityPage("judges.manage", a.handleAdminJudgeAssign))
	mux.HandleFunc("GET /admin/shows/{showID}/stream", a.requireCapabilityPage("shows.workspace.read", a.handleAdminShowStream))

	// Admin schedule management
	mux.HandleFunc("POST /admin/shows/{showID}/schedule", a.requireCapabilityPage("schedule.manage", a.handleAdminScheduleCreate))
	mux.HandleFunc("POST /admin/shows/{showID}/divisions", a.requireCapabilityPage("schedule.manage", a.handleAdminDivisionCreate))
	mux.HandleFunc("POST /admin/shows/{showID}/sections", a.requireCapabilityPage("schedule.manage", a.handleAdminSectionCreate))
	mux.HandleFunc("POST /admin/shows/{showID}/classes", a.requireCapabilityPage("classes.manage", a.handleAdminClassCreate))
	mux.HandleFunc("POST /admin/classes/{classID}", a.requireCapabilityPage("classes.manage", a.handleAdminClassUpdate))

	// Admin entries
	mux.HandleFunc("POST /admin/shows/{showID}/entries", a.requireCapabilityPage("entries.manage", a.handleAdminEntryCreate))
	mux.HandleFunc("POST /admin/entries/{entryID}/move", a.requireCapabilityPage("entries.manage", a.handleAdminEntryMove))
	mux.HandleFunc("POST /admin/entries/{entryID}/entrant", a.requireCapabilityPage("entries.manage", a.handleAdminEntryReassign))
	mux.HandleFunc("POST /admin/entries/{entryID}/delete", a.requireCapabilityPage("entries.manage", a.handleAdminEntryDelete))
	mux.HandleFunc("POST /admin/entries/{entryID}/placement", a.requireCapabilityPage("entries.manage", a.handleAdminEntryPlacement))
	mux.HandleFunc("POST /admin/entries/{entryID}/visibility", a.requireCapabilityPage("entries.manage", a.handleAdminEntryVisibility))
	mux.HandleFunc("POST /admin/entries/{entryID}/media", a.requireCapabilityPage("media.manage", a.handleMediaUpload))
	mux.HandleFunc("POST /admin/media/{mediaID}/delete", a.requireCapabilityPage("media.manage", a.handleMediaDelete))
	mux.HandleFunc("POST /admin/shows/{showID}/credits", a.requireCapabilityPage("show_credits.manage", a.handleAdminShowCreditCreate))
	mux.HandleFunc("POST /admin/credits/{creditID}/delete", a.requireCapabilityPage("show_credits.manage", a.handleAdminShowCreditDelete))

	// Admin persons
	mux.HandleFunc("GET /admin/persons", a.requireAdmin(a.handleAdminPersons))
	mux.HandleFunc("POST /admin/persons", a.requireAdmin(a.handleAdminPersonCreate))

	// Admin awards
	mux.HandleFunc("POST /admin/awards", a.requireAdmin(a.handleAdminAwardCreate))
	mux.HandleFunc("POST /admin/awards/{awardID}/compute", a.requireAdmin(a.handleAdminAwardCompute))

	// Admin standards & rules
	mux.HandleFunc("POST /admin/standards", a.requireAdmin(a.handleAdminStandardCreate))
	mux.HandleFunc("POST /admin/editions", a.requireAdmin(a.handleAdminEditionCreate))
	mux.HandleFunc("POST /admin/sources", a.requireAdmin(a.handleAdminSourceCreate))
	mux.HandleFunc("POST /admin/citations", a.requireAdmin(a.handleAdminCitationCreate))
	mux.HandleFunc("POST /admin/ingestions", a.requireAdmin(a.handleAdminIngestionImport))
	mux.HandleFunc("POST /admin/rules", a.requireAdmin(a.handleAdminRuleCreate))
	mux.HandleFunc("POST /admin/overrides", a.requireAdmin(a.handleAdminOverrideCreate))

	// Admin rubrics & scoring
	mux.HandleFunc("POST /admin/rubrics", a.requireAdmin(a.handleAdminRubricCreate))
	mux.HandleFunc("POST /admin/criteria", a.requireAdmin(a.handleAdminCriterionCreate))
	mux.HandleFunc("POST /admin/scorecards", a.requireAdmin(a.handleAdminScorecardSubmit))
	mux.HandleFunc("POST /admin/classes/{classID}/compute-placements", a.requireAdmin(a.handleAdminComputePlacements))

	// JSON API
	mux.HandleFunc("GET /v1/projections/0007-Flowershow/account", a.handleAPIAccount)
	mux.HandleFunc("GET /v1/projections/0007-Flowershow/organizations", a.handleAPIOrganizationsDirectory)
	mux.HandleFunc("GET /v1/projections/0007-Flowershow/clubs/{id}/workspace", a.handleAPIClubWorkspace)
	mux.HandleFunc("GET /v1/projections/0007-Flowershow/shows", a.handleAPIShowsDirectory)
	mux.HandleFunc("GET /v1/projections/0007-Flowershow/shows/{id}", a.handleAPIShowDetail)
	mux.HandleFunc("GET /v1/projections/0007-Flowershow/shows/{id}/workspace", a.handleAPIShowWorkspace)
	mux.HandleFunc("GET /v1/projections/0007-Flowershow/shows/{id}/board", a.handleAPIShowBoard)
	mux.HandleFunc("GET /v1/projections/0007-Flowershow/shows/{id}/people.lookup", a.handleAPIShowPeopleLookup)
	mux.HandleFunc("GET /v1/projections/0007-Flowershow/entries", a.handleAPIEntries)
	mux.HandleFunc("GET /v1/projections/0007-Flowershow/entries/{id}", a.handleAPIEntryDetail)
	mux.HandleFunc("GET /v1/projections/0007-Flowershow/classes", a.handleAPIClasses)
	mux.HandleFunc("GET /v1/projections/0007-Flowershow/classes/{id}", a.handleAPIClassDetail)
	mux.HandleFunc("GET /v1/projections/0007-Flowershow/taxonomy", a.handleAPITaxonomy)
	mux.HandleFunc("GET /v1/projections/0007-Flowershow/leaderboard", a.handleAPILeaderboard)
	mux.HandleFunc("GET /v1/projections/0007-Flowershow/ledger/{objectID}", a.handleAPILedger)
	mux.HandleFunc("GET /v1/projections/0007-Flowershow/admin/dashboard", a.handleAPIAdminDashboard)

	mux.HandleFunc("POST /v1/commands/0007-Flowershow/organization.create", a.handleAPICommand)
	mux.HandleFunc("POST /v1/commands/0007-Flowershow/shows.create", a.handleAPICommand)
	mux.HandleFunc("POST /v1/commands/0007-Flowershow/shows.update", a.handleAPICommand)
	mux.HandleFunc("POST /v1/commands/0007-Flowershow/clubs.invites.create", a.handleAPICommand)
	mux.HandleFunc("POST /v1/commands/0007-Flowershow/schedules.upsert", a.handleAPICommand)
	mux.HandleFunc("POST /v1/commands/0007-Flowershow/judges.assign", a.handleAPICommand)
	mux.HandleFunc("POST /v1/commands/0007-Flowershow/divisions.create", a.handleAPICommand)
	mux.HandleFunc("POST /v1/commands/0007-Flowershow/sections.create", a.handleAPICommand)
	mux.HandleFunc("POST /v1/commands/0007-Flowershow/entries.create", a.handleAPICommand)
	mux.HandleFunc("POST /v1/commands/0007-Flowershow/entries.update", a.handleAPICommand)
	mux.HandleFunc("POST /v1/commands/0007-Flowershow/entries.move", a.handleAPICommand)
	mux.HandleFunc("POST /v1/commands/0007-Flowershow/entries.delete", a.handleAPICommand)
	mux.HandleFunc("POST /v1/commands/0007-Flowershow/entries.reassign_entrant", a.handleAPICommand)
	mux.HandleFunc("POST /v1/commands/0007-Flowershow/entries.set_placement", a.handleAPICommand)
	mux.HandleFunc("POST /v1/commands/0007-Flowershow/entries.set_visibility", a.handleAPICommand)
	mux.HandleFunc("POST /v1/commands/0007-Flowershow/classes.create", a.handleAPICommand)
	mux.HandleFunc("POST /v1/commands/0007-Flowershow/classes.update", a.handleAPICommand)
	mux.HandleFunc("POST /v1/commands/0007-Flowershow/classes.reorder", a.handleAPICommand)
	mux.HandleFunc("POST /v1/commands/0007-Flowershow/classes.compute_placements", a.handleAPICommand)
	mux.HandleFunc("POST /v1/commands/0007-Flowershow/persons.create", a.handleAPICommand)
	mux.HandleFunc("POST /v1/commands/0007-Flowershow/awards.create", a.handleAPICommand)
	mux.HandleFunc("POST /v1/commands/0007-Flowershow/awards.compute", a.handleAPICommand)
	mux.HandleFunc("POST /v1/commands/0007-Flowershow/taxons.create", a.handleAPICommand)
	mux.HandleFunc("POST /v1/commands/0007-Flowershow/media.attach", a.handleAPICommand)
	mux.HandleFunc("POST /v1/commands/0007-Flowershow/media.delete", a.handleAPICommand)
	mux.HandleFunc("POST /v1/commands/0007-Flowershow/rubrics.create", a.handleAPICommand)
	mux.HandleFunc("POST /v1/commands/0007-Flowershow/criteria.create", a.handleAPICommand)
	mux.HandleFunc("POST /v1/commands/0007-Flowershow/scorecards.submit", a.handleAPICommand)
	mux.HandleFunc("POST /v1/commands/0007-Flowershow/standards.create", a.handleAPICommand)
	mux.HandleFunc("POST /v1/commands/0007-Flowershow/editions.create", a.handleAPICommand)
	mux.HandleFunc("POST /v1/commands/0007-Flowershow/sources.create", a.handleAPICommand)
	mux.HandleFunc("POST /v1/commands/0007-Flowershow/citations.create", a.handleAPICommand)
	mux.HandleFunc("POST /v1/commands/0007-Flowershow/ingestions.import", a.handleAPICommand)
	mux.HandleFunc("POST /v1/commands/0007-Flowershow/rules.create", a.handleAPICommand)
	mux.HandleFunc("POST /v1/commands/0007-Flowershow/overrides.create", a.handleAPICommand)
	mux.HandleFunc("POST /v1/commands/0007-Flowershow/show_credits.create", a.handleAPICommand)
	mux.HandleFunc("POST /v1/commands/0007-Flowershow/show_credits.delete", a.handleAPICommand)
	mux.HandleFunc("POST /v1/commands/0007-Flowershow/roles.assign", a.handleAPICommand)

	handler := requestLog(a.storeRefreshMiddleware(mux))

	if strings.HasPrefix(addr, "/") || strings.HasPrefix(addr, ".") {
		if err := os.MkdirAll(filepath.Dir(addr), 0755); err != nil {
			log.Fatal(err)
		}
		os.Remove(addr)
		ln, err := net.Listen("unix", addr)
		if err != nil {
			log.Fatal(err)
		}
		defer ln.Close()
		defer os.Remove(addr)
		go func() {
			sig := make(chan os.Signal, 1)
			signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
			<-sig
			ln.Close()
			os.Remove(addr)
			os.Exit(0)
		}()
		log.Printf("flowershow listening on unix:%s", addr)
		if err := http.Serve(ln, handler); err != nil && !errors.Is(err, net.ErrClosed) {
			log.Fatal(err)
		}
		return
	}

	log.Printf("flowershow listening on http://%s", addr)
	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatal(err)
	}
}

func (a *app) assetHandler() http.Handler {
	sub, err := fs.Sub(assets, "assets")
	if err != nil {
		panic(err)
	}
	return http.StripPrefix("/assets/", http.FileServer(http.FS(sub)))
}

func (a *app) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"seed":   "0007-Flowershow",
	})
}

var globalBasePath = strings.TrimSuffix(strings.TrimSpace(os.Getenv("AS_PATH_PREFIX")), "/")
var assetVersion = time.Now().UTC().Format("20060102150405")

const githubRepoBlobBase = "https://github.com/splashkes/autosoftware/blob/main/"

type agentAccessLink struct {
	Label string
	Href  string
}

func requestBasePath(r *http.Request) string {
	if r != nil {
		if forwarded := strings.TrimSuffix(strings.TrimSpace(r.Header.Get("X-Forwarded-Prefix")), "/"); forwarded != "" {
			return forwarded
		}
	}
	return globalBasePath
}

var templateFuncMap = template.FuncMap{
	"assetVersion": func() string { return assetVersion },
	"githubSourceURL": func(path string) string {
		path = strings.TrimSpace(path)
		path = strings.TrimPrefix(path, "/")
		return githubRepoBlobBase + path
	},
	"agentRegistryLinks": func(data any) []agentAccessLink {
		return agentRegistryLinks(data)
	},
	"placementLabel": func(p int) string {
		switch p {
		case 1:
			return "1st"
		case 2:
			return "2nd"
		case 3:
			return "3rd"
		default:
			return ""
		}
	},
	"placementClass": func(p int) string {
		switch p {
		case 1:
			return "placement-first"
		case 2:
			return "placement-second"
		case 3:
			return "placement-third"
		default:
			return ""
		}
	},
	"initials": func(p *Person) string {
		if p == nil {
			return ""
		}
		return p.Initials
	},
	"publicPersonLabel": func(p *Person) string {
		if p == nil {
			return ""
		}
		switch strings.TrimSpace(p.PublicDisplayMode) {
		case "full_name":
			if full := strings.TrimSpace(p.FirstName + " " + p.LastName); full != "" {
				return full
			}
		case "first_name_last_initial":
			firstName := strings.TrimSpace(p.FirstName)
			lastRunes := []rune(strings.TrimSpace(p.LastName))
			if firstName != "" && len(lastRunes) > 0 {
				return firstName + " " + strings.ToUpper(string(lastRunes[:1])) + "."
			}
			if firstName != "" {
				return firstName
			}
		}
		return p.Initials
	},
	"seq": func(start, end int) []int {
		var s []int
		for i := start; i <= end; i++ {
			s = append(s, i)
		}
		return s
	},
	"cycleDelay": func(i int) string {
		return fmt.Sprintf("%ds", i*4)
	},
	"statusBadge": func(status string) template.HTML {
		colors := map[string]string{
			"draft":     "badge-gray",
			"published": "badge-green",
			"completed": "badge-gold",
			"archived":  "badge-muted",
		}
		cls := colors[status]
		if cls == "" {
			cls = "badge-gray"
		}
		return template.HTML(`<span class="badge ` + cls + `">` + template.HTMLEscapeString(status) + `</span>`)
	},
	"entriesForClass": func(entries []*entryView, classID string) []*entryView {
		out := make([]*entryView, 0)
		for _, entry := range entries {
			if entry != nil && entry.Entry != nil && entry.Entry.ClassID == classID {
				out = append(out, entry)
			}
		}
		return out
	},
	"primaryMedia": func(media []*Media) *Media {
		for _, item := range media {
			if item != nil && item.MediaType == "photo" {
				return item
			}
		}
		if len(media) > 0 {
			return media[0]
		}
		return nil
	},
	"entryWorkflowBadges": func(entry *entryView) []string {
		if entry == nil || entry.Entry == nil {
			return nil
		}
		out := make([]string, 0, 4)
		if len(entry.Media) == 0 {
			out = append(out, "needs photo")
		}
		if entry.Entry.Suppressed {
			out = append(out, "suppressed")
		}
		if entry.Entry.Placement > 0 {
			switch entry.Entry.Placement {
			case 1:
				out = append(out, "1st")
			case 2:
				out = append(out, "2nd")
			case 3:
				out = append(out, "3rd")
			}
		} else if entry.Entry.Points > 0 {
			out = append(out, "scored")
		}
		return out
	},
	"entryCountWithPhotosMissing": func(entries []*entryView) int {
		count := 0
		for _, entry := range entries {
			if entry != nil && len(entry.Media) == 0 {
				count++
			}
		}
		return count
	},
	"entryCountPlaced": func(entries []*entryView) int {
		count := 0
		for _, entry := range entries {
			if entry != nil && entry.Entry != nil && entry.Entry.Placement > 0 {
				count++
			}
		}
		return count
	},
}

func agentRegistryLinks(data any) []agentAccessLink {
	currentPath := templateCurrentPath(data)
	showID := templateStringField(data, "ShowID")
	if showID == "" {
		showID = templateNestedObjectID(data, "Show")
	}
	classID := templateStringField(data, "ClassID")
	if classID == "" {
		classID = templateNestedObjectID(data, "Class")
	}
	entryID := templateStringField(data, "EntryID")
	if entryID == "" {
		entryID = templateNestedObjectID(data, "Entry")
	}
	taxonID := templateStringField(data, "TaxonID")
	if taxonID == "" {
		taxonID = templateNestedObjectID(data, "Taxon")
	}
	personID := templateStringField(data, "PersonID")
	if personID == "" {
		personID = templateNestedObjectID(data, "Person")
	}
	organizationID := templateStringField(data, "OrganizationID")
	if organizationID == "" {
		organizationID = templateNestedObjectID(data, "Organization")
	}

	links := make([]agentAccessLink, 0, 8)
	seen := make(map[string]struct{})
	add := func(label, href string) {
		href = strings.TrimSpace(href)
		if href == "" {
			return
		}
		if _, ok := seen[href]; ok {
			return
		}
		seen[href] = struct{}{}
		links = append(links, agentAccessLink{Label: label, Href: href})
	}

	switch {
	case currentPath == "/" || currentPath == "/browse":
		add("Shows directory projection", "/v1/projections/0007-Flowershow/shows")
	case currentPath == "/account" || currentPath == "/profile":
		add("Account projection", "/v1/projections/0007-Flowershow/account")
	case currentPath == "/taxonomy":
		add("Taxonomy projection", "/v1/projections/0007-Flowershow/taxonomy")
	case currentPath == "/leaderboard":
		add("Leaderboard projection", "/v1/projections/0007-Flowershow/leaderboard")
	case currentPath == "/admin":
		add("Admin dashboard projection", "/v1/projections/0007-Flowershow/admin/dashboard")
	case strings.HasPrefix(currentPath, "/admin/"):
		add("Admin dashboard projection", "/v1/projections/0007-Flowershow/admin/dashboard")
	}

	if organizationID != "" {
		add("Club workspace projection", "/v1/projections/0007-Flowershow/clubs/"+organizationID+"/workspace")
		add("Organization ledger", "/v1/projections/0007-Flowershow/ledger/"+organizationID)
	}

	if showID != "" {
		add("Show projection", "/v1/projections/0007-Flowershow/shows/"+showID)
		add("Show workspace projection", "/v1/projections/0007-Flowershow/shows/"+showID+"/workspace")
		add("Show board projection", "/v1/projections/0007-Flowershow/shows/"+showID+"/board")
		add("Show ledger", "/v1/projections/0007-Flowershow/ledger/"+showID)
	}
	if classID != "" {
		add("Class projection", "/v1/projections/0007-Flowershow/classes/"+classID)
		add("Class ledger", "/v1/projections/0007-Flowershow/ledger/"+classID)
	}
	if entryID != "" {
		add("Entry projection", "/v1/projections/0007-Flowershow/entries/"+entryID)
		add("Entry ledger", "/v1/projections/0007-Flowershow/ledger/"+entryID)
	}
	if taxonID != "" {
		add("Taxonomy projection", "/v1/projections/0007-Flowershow/taxonomy")
		add("Taxon ledger", "/v1/projections/0007-Flowershow/ledger/"+taxonID)
	}
	if personID != "" {
		add("Person ledger", "/v1/projections/0007-Flowershow/ledger/"+personID)
	}
	if len(links) == 0 {
		add("Shows directory projection", "/v1/projections/0007-Flowershow/shows")
		add("Contract detail", flowershowContractSelf)
	}

	return links
}

func templateCurrentPath(data any) string {
	return templateStringField(data, "CurrentPath")
}

func templateNestedObjectID(data any, fieldName string) string {
	value, ok := templateFieldValue(data, fieldName)
	if !ok {
		return ""
	}
	if id, ok := templateValueStringField(value, "ID"); ok {
		return id
	}
	return ""
}

func templateStringField(data any, fieldName string) string {
	value, ok := templateFieldValue(data, fieldName)
	if !ok {
		return ""
	}
	if str, ok := templateValueAsString(value); ok {
		return strings.TrimSpace(str)
	}
	return ""
}

func templateFieldValue(data any, fieldName string) (reflect.Value, bool) {
	value := reflect.ValueOf(data)
	for value.IsValid() && value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return reflect.Value{}, false
		}
		value = value.Elem()
	}
	if !value.IsValid() {
		return reflect.Value{}, false
	}
	switch value.Kind() {
	case reflect.Struct:
		field := value.FieldByName(fieldName)
		if !field.IsValid() {
			return reflect.Value{}, false
		}
		return field, true
	case reflect.Map:
		for _, key := range value.MapKeys() {
			if key.Kind() == reflect.String && key.String() == fieldName {
				return value.MapIndex(key), true
			}
		}
	}
	return reflect.Value{}, false
}

func templateValueStringField(value reflect.Value, fieldName string) (string, bool) {
	for value.IsValid() && value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return "", false
		}
		value = value.Elem()
	}
	for value.IsValid() && value.Kind() == reflect.Interface {
		if value.IsNil() {
			return "", false
		}
		value = value.Elem()
	}
	if !value.IsValid() || value.Kind() != reflect.Struct {
		return "", false
	}
	field := value.FieldByName(fieldName)
	if !field.IsValid() {
		return "", false
	}
	return templateValueAsString(field)
}

func templateValueAsString(value reflect.Value) (string, bool) {
	for value.IsValid() && value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return "", false
		}
		value = value.Elem()
	}
	if !value.IsValid() {
		return "", false
	}
	if value.Kind() == reflect.Interface {
		return templateValueAsString(value.Elem())
	}
	if value.Kind() != reflect.String {
		return "", false
	}
	return value.String(), true
}

func parseTemplates() map[string]*template.Template {
	pages := []string{
		"templates/home.html",
		"templates/clubs.html",
		"templates/classes.html",
		"templates/show_detail.html",
		"templates/show_summary.html",
		"templates/class_browse.html",
		"templates/class_detail.html",
		"templates/entry_detail.html",
		"templates/browse.html",
		"templates/person_detail.html",
		"templates/taxonomy_browse.html",
		"templates/taxon_detail.html",
		"templates/leaderboard.html",
		"templates/login.html",
		"templates/account.html",
		"templates/standards.html",
		"templates/show_rules.html",
		"templates/show_admin.html",
		"templates/admin_dashboard.html",
		"templates/admin_club.html",
		"templates/admin_club_new.html",
		"templates/admin_show_new.html",
		"templates/admin_persons.html",
		"templates/admin_roles.html",
	}

	base := template.Must(template.New("base").Funcs(templateFuncMap).ParseFS(
		templates, "templates/base.html", "templates/partials/*.html",
	))

	ts := make(map[string]*template.Template, len(pages))
	for _, page := range pages {
		clone := template.Must(template.Must(base.Clone()).ParseFS(templates, page))
		name := page[len("templates/"):]
		ts[name] = clone
	}
	return ts
}

func (a *app) render(w http.ResponseWriter, r *http.Request, name string, data any) {
	t, ok := a.templates[name]
	if !ok {
		log.Printf("render: template %q not found", name)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.ExecuteTemplate(w, "base", a.withChrome(r, data)); err != nil {
		log.Printf("render %s: %v", name, err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (a *app) withChrome(r *http.Request, data any) map[string]any {
	out := map[string]any{}
	appendFields(out, reflect.ValueOf(data))
	var (
		user *UserIdentity
		ok   bool
	)
	if r != nil {
		user, ok = a.currentUser(r)
	}
	out["BasePath"] = requestBasePath(r)
	out["NavSignedIn"] = ok
	out["NavUserLabel"] = ""
	out["NavUserHref"] = "/account"
	out["NavShowAdmin"] = false
	if ok && user != nil {
		label := strings.TrimSpace(user.Name)
		if label == "" {
			label = strings.TrimSpace(user.Email)
		}
		out["NavUserLabel"] = label
		out["NavShowAdmin"] = a.userIsAdmin(*user)
	}
	return out
}

func appendFields(dst map[string]any, value reflect.Value) {
	for value.IsValid() && value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return
		}
		value = value.Elem()
	}
	if !value.IsValid() {
		return
	}
	switch value.Kind() {
	case reflect.Map:
		for _, key := range value.MapKeys() {
			if key.Kind() != reflect.String {
				continue
			}
			dst[key.String()] = value.MapIndex(key).Interface()
		}
	case reflect.Struct:
		valueType := value.Type()
		for i := 0; i < value.NumField(); i++ {
			field := valueType.Field(i)
			if !field.IsExported() {
				continue
			}
			dst[field.Name] = value.Field(i).Interface()
		}
	}
}

func (a *app) requireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := a.currentUser(r); !ok {
			http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
			return
		}
		if !a.isAdmin(r) {
			http.Redirect(w, r, "/account?notice=admin_required", http.StatusSeeOther)
			return
		}
		next(w, r)
	}
}

func (a *app) requireSignedInPage(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := a.currentUser(r); !ok {
			http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
			return
		}
		next(w, r)
	}
}

func (a *app) requireCapabilityPage(capability string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := a.currentUser(r)
		if !ok {
			http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
			return
		}
		if capability == "" || a.userHasCapability(r.Context(), *user, capability, a.authorityScopesForRequest(r)...) {
			next(w, r)
			return
		}
		http.Redirect(w, r, "/account?notice=access_denied", http.StatusSeeOther)
	}
}

func (a *app) isServiceToken(r *http.Request) bool {
	if a.serviceToken == "" {
		return false
	}
	return bearerTokenFromRequest(r) == a.serviceToken
}

func (a *app) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !a.isAdmin(r) && !a.isServiceToken(r) {
			a.writeAPIError(w, r, http.StatusUnauthorized, "unauthorized", "Authentication required.", "Use an admin session or a Bearer service token to access this endpoint.", nil)
			return
		}
		next(w, r)
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func requestLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}

func (a *app) storeRefreshMiddleware(next http.Handler) http.Handler {
	refresher, ok := a.store.(refreshingStore)
	if !ok {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/assets/") || r.URL.Path == "/healthz" {
			next.ServeHTTP(w, r)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		if err := refresher.Refresh(ctx); err != nil {
			http.Error(w, "Store refresh failed.", http.StatusServiceUnavailable)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
