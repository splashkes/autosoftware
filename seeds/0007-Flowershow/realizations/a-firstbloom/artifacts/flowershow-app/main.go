package main

import (
	"embed"
	"encoding/json"
	"errors"
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
)

const (
	adminCookieName = "as_flowershow_admin"
	maxFormMemory   = 1 << 20
)

//go:embed templates/*
var templates embed.FS

//go:embed assets/*
var assets embed.FS

type app struct {
	store         flowershowStore
	templates     map[string]*template.Template
	adminPassword string
	serviceToken  string
	sseBroker     *sseBroker
}

func main() {
	addr := envOrDefault("AS_ADDR", "127.0.0.1:8097")
	store, err := newFlowershowStore(envOrDefault("AS_RUNTIME_DATABASE_URL", ""))
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()

	a := &app{
		store:         store,
		templates:     parseTemplates(),
		adminPassword: envOrDefault("AS_ADMIN_PASSWORD", "admin"),
		serviceToken:  strings.TrimSpace(os.Getenv("AS_SERVICE_TOKEN")),
		sseBroker:     newSSEBroker(),
	}

	mux := http.NewServeMux()

	// Assets & health
	mux.Handle("GET /assets/", a.assetHandler())
	mux.HandleFunc("GET /healthz", a.handleHealth)

	// Public pages
	mux.HandleFunc("GET /", a.handleHome)
	mux.HandleFunc("GET /shows/{slug}", a.handleShowDetail)
	mux.HandleFunc("GET /shows/{slug}/classes", a.handleClassBrowse)
	mux.HandleFunc("GET /shows/{slug}/classes/{classID}", a.handleClassDetail)
	mux.HandleFunc("GET /shows/{slug}/rules", a.handleShowRules)
	mux.HandleFunc("GET /entries/{entryID}", a.handleEntryDetail)
	mux.HandleFunc("GET /taxonomy", a.handleTaxonomyBrowse)
	mux.HandleFunc("GET /taxonomy/{taxonID}", a.handleTaxonDetail)
	mux.HandleFunc("GET /leaderboard", a.handleLeaderboard)
	mux.HandleFunc("GET /standards", a.handleStandards)

	// Admin auth
	mux.HandleFunc("GET /admin/login", a.handleAdminLogin)
	mux.HandleFunc("POST /admin/login", a.handleAdminLoginPost)
	mux.HandleFunc("POST /admin/logout", a.handleAdminLogout)

	// Admin pages
	mux.HandleFunc("GET /admin", a.requireAdmin(a.handleAdminDashboard))

	// Admin shows
	mux.HandleFunc("GET /admin/shows/new", a.requireAdmin(a.handleAdminShowNew))
	mux.HandleFunc("POST /admin/shows", a.requireAdmin(a.handleAdminShowCreate))
	mux.HandleFunc("GET /admin/shows/{showID}", a.requireAdmin(a.handleAdminShowDetail))
	mux.HandleFunc("POST /admin/shows/{showID}", a.requireAdmin(a.handleAdminShowUpdate))
	mux.HandleFunc("GET /admin/shows/{showID}/stream", a.requireAdmin(a.handleAdminShowStream))

	// Admin schedule management
	mux.HandleFunc("POST /admin/shows/{showID}/schedule", a.requireAdmin(a.handleAdminScheduleCreate))
	mux.HandleFunc("POST /admin/shows/{showID}/divisions", a.requireAdmin(a.handleAdminDivisionCreate))
	mux.HandleFunc("POST /admin/shows/{showID}/sections", a.requireAdmin(a.handleAdminSectionCreate))
	mux.HandleFunc("POST /admin/shows/{showID}/classes", a.requireAdmin(a.handleAdminClassCreate))

	// Admin entries
	mux.HandleFunc("POST /admin/shows/{showID}/entries", a.requireAdmin(a.handleAdminEntryCreate))
	mux.HandleFunc("POST /admin/entries/{entryID}/placement", a.requireAdmin(a.handleAdminEntryPlacement))

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
	mux.HandleFunc("GET /v1/projections/0007-Flowershow/entries", a.handleAPIEntries)
	mux.HandleFunc("GET /v1/projections/0007-Flowershow/classes", a.handleAPIClasses)
	mux.HandleFunc("GET /v1/projections/0007-Flowershow/taxonomy", a.handleAPITaxonomy)
	mux.HandleFunc("GET /v1/projections/0007-Flowershow/leaderboard", a.handleAPILeaderboard)
	mux.HandleFunc("GET /v1/projections/0007-Flowershow/ledger/{objectID}", a.handleAPILedger)
	mux.HandleFunc("GET /v1/projections/0007-Flowershow/admin/dashboard", a.handleAPIAdminDashboard)

	mux.HandleFunc("POST /v1/commands/0007-Flowershow/shows.create", a.handleAPICommand)
	mux.HandleFunc("POST /v1/commands/0007-Flowershow/shows.update", a.handleAPICommand)
	mux.HandleFunc("POST /v1/commands/0007-Flowershow/entries.create", a.handleAPICommand)
	mux.HandleFunc("POST /v1/commands/0007-Flowershow/entries.update", a.handleAPICommand)
	mux.HandleFunc("POST /v1/commands/0007-Flowershow/entries.set_placement", a.handleAPICommand)
	mux.HandleFunc("POST /v1/commands/0007-Flowershow/classes.create", a.handleAPICommand)
	mux.HandleFunc("POST /v1/commands/0007-Flowershow/persons.create", a.handleAPICommand)
	mux.HandleFunc("POST /v1/commands/0007-Flowershow/awards.create", a.handleAPICommand)
	mux.HandleFunc("POST /v1/commands/0007-Flowershow/awards.compute", a.handleAPICommand)
	mux.HandleFunc("POST /v1/commands/0007-Flowershow/taxons.create", a.handleAPICommand)
	mux.HandleFunc("POST /v1/commands/0007-Flowershow/rubrics.create", a.handleAPICommand)
	mux.HandleFunc("POST /v1/commands/0007-Flowershow/criteria.create", a.handleAPICommand)
	mux.HandleFunc("POST /v1/commands/0007-Flowershow/scorecards.submit", a.handleAPICommand)
	mux.HandleFunc("POST /v1/commands/0007-Flowershow/standards.create", a.handleAPICommand)
	mux.HandleFunc("POST /v1/commands/0007-Flowershow/editions.create", a.handleAPICommand)
	mux.HandleFunc("POST /v1/commands/0007-Flowershow/sources.create", a.handleAPICommand)
	mux.HandleFunc("POST /v1/commands/0007-Flowershow/citations.create", a.handleAPICommand)
	mux.HandleFunc("POST /v1/commands/0007-Flowershow/rules.create", a.handleAPICommand)
	mux.HandleFunc("POST /v1/commands/0007-Flowershow/overrides.create", a.handleAPICommand)

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

var templateFuncMap = template.FuncMap{
	"bp": func() string { return globalBasePath },
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
		"templates/class_browse.html",
		"templates/class_detail.html",
		"templates/entry_detail.html",
		"templates/taxonomy_browse.html",
		"templates/taxon_detail.html",
		"templates/leaderboard.html",
		"templates/login.html",
		"templates/standards.html",
		"templates/show_rules.html",
		"templates/show_admin.html",
		"templates/admin_dashboard.html",
		"templates/admin_show_new.html",
		"templates/admin_persons.html",
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

func (a *app) isAdmin(r *http.Request) bool {
	cookie, err := r.Cookie(adminCookieName)
	return err == nil && cookie.Value == "ok"
}

func (a *app) requireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !a.isAdmin(r) {
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
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
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
