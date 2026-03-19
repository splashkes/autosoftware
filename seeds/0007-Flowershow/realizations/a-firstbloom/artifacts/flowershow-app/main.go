package main

import (
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
	store           flowershowStore
	templates       map[string]*template.Template
	serviceToken    string
	sseBroker       *sseBroker
	auth            authProvider
	media           mediaStore
	sessionSecret   []byte
	bootstrapAdmins map[string]bool
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
		store:           store,
		templates:       parseTemplates(),
		serviceToken:    strings.TrimSpace(os.Getenv("AS_SERVICE_TOKEN")),
		sseBroker:       newSSEBroker(),
		auth:            auth,
		media:           media,
		sessionSecret:   newSessionSecret(),
		bootstrapAdmins: bootstrapAdminMap(),
	}

	mux := http.NewServeMux()

	// Assets & health
	mux.Handle("GET /assets/", a.assetHandler())
	mux.HandleFunc("GET /healthz", a.handleHealth)
	mux.HandleFunc("GET /v1/contracts", a.handleContractsList)
	mux.HandleFunc("GET /v1/contracts/{seed_id}/{realization_id}", a.handleContractDetail)

	// Public pages
	mux.HandleFunc("GET /", a.handleHome)
	mux.HandleFunc("GET /account", a.requireSignedInPage(a.handleAccount))
	mux.HandleFunc("GET /profile", a.requireSignedInPage(a.handleAccount))
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

	// Admin shows
	mux.HandleFunc("GET /admin/shows/new", a.requireAdmin(a.handleAdminShowNew))
	mux.HandleFunc("POST /admin/shows", a.requireAdmin(a.handleAdminShowCreate))
	mux.HandleFunc("GET /admin/shows/{showID}", a.requireAdmin(a.handleAdminShowDetail))
	mux.HandleFunc("GET /admin/shows/{showID}/fragments/{section}", a.requireAdmin(a.handleAdminShowFragment))
	mux.HandleFunc("POST /admin/shows/{showID}", a.requireAdmin(a.handleAdminShowUpdate))
	mux.HandleFunc("POST /admin/shows/{showID}/judges", a.requireAdmin(a.handleAdminJudgeAssign))
	mux.HandleFunc("GET /admin/shows/{showID}/stream", a.requireAdmin(a.handleAdminShowStream))

	// Admin schedule management
	mux.HandleFunc("POST /admin/shows/{showID}/schedule", a.requireAdmin(a.handleAdminScheduleCreate))
	mux.HandleFunc("POST /admin/shows/{showID}/divisions", a.requireAdmin(a.handleAdminDivisionCreate))
	mux.HandleFunc("POST /admin/shows/{showID}/sections", a.requireAdmin(a.handleAdminSectionCreate))
	mux.HandleFunc("POST /admin/shows/{showID}/classes", a.requireAdmin(a.handleAdminClassCreate))

	// Admin entries
	mux.HandleFunc("POST /admin/shows/{showID}/entries", a.requireAdmin(a.handleAdminEntryCreate))
	mux.HandleFunc("POST /admin/entries/{entryID}/placement", a.requireAdmin(a.handleAdminEntryPlacement))
	mux.HandleFunc("POST /admin/entries/{entryID}/visibility", a.requireAdmin(a.handleAdminEntryVisibility))
	mux.HandleFunc("POST /admin/entries/{entryID}/media", a.requireAdmin(a.handleMediaUpload))
	mux.HandleFunc("POST /admin/media/{mediaID}/delete", a.requireAdmin(a.handleMediaDelete))

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
	mux.HandleFunc("GET /v1/projections/0007-Flowershow/shows", a.handleAPIShowsDirectory)
	mux.HandleFunc("GET /v1/projections/0007-Flowershow/shows/{id}", a.handleAPIShowDetail)
	mux.HandleFunc("GET /v1/projections/0007-Flowershow/shows/{id}/workspace", a.handleAPIShowWorkspace)
	mux.HandleFunc("GET /v1/projections/0007-Flowershow/entries", a.handleAPIEntries)
	mux.HandleFunc("GET /v1/projections/0007-Flowershow/entries/{id}", a.handleAPIEntryDetail)
	mux.HandleFunc("GET /v1/projections/0007-Flowershow/classes", a.handleAPIClasses)
	mux.HandleFunc("GET /v1/projections/0007-Flowershow/classes/{id}", a.handleAPIClassDetail)
	mux.HandleFunc("GET /v1/projections/0007-Flowershow/taxonomy", a.handleAPITaxonomy)
	mux.HandleFunc("GET /v1/projections/0007-Flowershow/leaderboard", a.handleAPILeaderboard)
	mux.HandleFunc("GET /v1/projections/0007-Flowershow/ledger/{objectID}", a.handleAPILedger)
	mux.HandleFunc("GET /v1/projections/0007-Flowershow/admin/dashboard", a.handleAPIAdminDashboard)

	mux.HandleFunc("POST /v1/commands/0007-Flowershow/shows.create", a.handleAPICommand)
	mux.HandleFunc("POST /v1/commands/0007-Flowershow/shows.update", a.handleAPICommand)
	mux.HandleFunc("POST /v1/commands/0007-Flowershow/schedules.upsert", a.handleAPICommand)
	mux.HandleFunc("POST /v1/commands/0007-Flowershow/judges.assign", a.handleAPICommand)
	mux.HandleFunc("POST /v1/commands/0007-Flowershow/divisions.create", a.handleAPICommand)
	mux.HandleFunc("POST /v1/commands/0007-Flowershow/sections.create", a.handleAPICommand)
	mux.HandleFunc("POST /v1/commands/0007-Flowershow/entries.create", a.handleAPICommand)
	mux.HandleFunc("POST /v1/commands/0007-Flowershow/entries.update", a.handleAPICommand)
	mux.HandleFunc("POST /v1/commands/0007-Flowershow/entries.set_placement", a.handleAPICommand)
	mux.HandleFunc("POST /v1/commands/0007-Flowershow/entries.set_visibility", a.handleAPICommand)
	mux.HandleFunc("POST /v1/commands/0007-Flowershow/classes.create", a.handleAPICommand)
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
	mux.HandleFunc("POST /v1/commands/0007-Flowershow/roles.assign", a.handleAPICommand)

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
		if err := http.Serve(ln, requestLog(mux)); err != nil && !errors.Is(err, net.ErrClosed) {
			log.Fatal(err)
		}
		return
	}

	log.Printf("flowershow listening on http://%s", addr)
	if err := http.ListenAndServe(addr, requestLog(mux)); err != nil {
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

var templateFuncMap = template.FuncMap{
	"bp":           func() string { return globalBasePath },
	"assetVersion": func() string { return assetVersion },
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
}

func parseTemplates() map[string]*template.Template {
	pages := []string{
		"templates/home.html",
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

func (a *app) render(w http.ResponseWriter, name string, data any) {
	t, ok := a.templates[name]
	if !ok {
		log.Printf("render: template %q not found", name)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.ExecuteTemplate(w, "base", data); err != nil {
		log.Printf("render %s: %v", name, err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
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

func (a *app) isServiceToken(r *http.Request) bool {
	if a.serviceToken == "" {
		return false
	}
	auth := r.Header.Get("Authorization")
	return strings.TrimPrefix(auth, "Bearer ") == a.serviceToken
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

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
