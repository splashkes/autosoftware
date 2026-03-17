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
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	adminCookieName = "as_event_admin"
	maxFormMemory   = 1 << 20
)

//go:embed assets/*
var assets embed.FS

type eventStatus string

const (
	statusDraft     eventStatus = "draft"
	statusPublished eventStatus = "published"
	statusCanceled  eventStatus = "canceled"
	statusArchived  eventStatus = "archived"
)

type eventRecord struct {
	ID            string
	Slug          string
	Title         string
	Summary       string
	Description   string
	Venue         string
	VenueNote     string
	Neighborhood  string
	Location      string
	Category      string
	OrganizerName string
	OrganizerRole string
	OrganizerURL  string
	CoverImageURL string
	FeaturedBlurb string
	ExternalURL   string
	Timezone      string
	Tags          []string
	ShareCount    int
	SaveCount     int
	CrowdLabel    string
	AllDay        bool
	Status        eventStatus
	Start         time.Time
	End           time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
	RevisionCount int
}

type eventInput struct {
	Title         string
	Summary       string
	Description   string
	Venue         string
	VenueNote     string
	Neighborhood  string
	Location      string
	Category      string
	OrganizerName string
	OrganizerRole string
	OrganizerURL  string
	CoverImageURL string
	FeaturedBlurb string
	Tags          string
	CrowdLabel    string
	ExternalURL   string
	Timezone      string
	AllDay        bool
	StartDate     string
	EndDate       string
	StartTime     string
	EndTime       string
}

type app struct {
	store         eventStore
	templates     *template.Template
	adminPassword string
	serviceToken  string
}

func main() {
	addr := envOrDefault("AS_ADDR", "127.0.0.1:8096")
	store, err := newEventStore(envOrDefault("AS_RUNTIME_DATABASE_URL", ""))
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()
	a := &app{
		store:         store,
		templates:     parseTemplates(),
		adminPassword: envOrDefault("AS_ADMIN_PASSWORD", "admin"),
		serviceToken:  strings.TrimSpace(os.Getenv("AS_SERVICE_TOKEN")),
	}

	mux := http.NewServeMux()
	mux.Handle("GET /assets/", a.assetHandler())
	mux.HandleFunc("GET /healthz", a.handleHealth)
	mux.HandleFunc("GET /", a.handleHome)
	mux.HandleFunc("GET /calendar", a.handleCalendar)
	mux.HandleFunc("GET /events/", a.handleEventDetail)

	mux.HandleFunc("GET /admin/login", a.handleAdminLogin)
	mux.HandleFunc("POST /admin/login", a.handleAdminLoginPost)
	mux.HandleFunc("POST /admin/logout", a.handleAdminLogoutPost)
	mux.HandleFunc("GET /admin", a.requireAdmin(a.handleAdminDashboard))
	mux.HandleFunc("GET /admin/events/new", a.requireAdmin(a.handleAdminNew))
	mux.HandleFunc("POST /admin/events", a.requireAdmin(a.handleAdminCreatePost))
	mux.HandleFunc("GET /admin/events/", a.requireAdmin(a.handleAdminEventRoutes))
	mux.HandleFunc("POST /admin/events/", a.requireAdmin(a.handleAdminEventRoutes))

	mux.HandleFunc("GET /v1/projections/0004-event-listings/admin/events", a.handleWorkspaceProjection)
	mux.HandleFunc("GET /v1/projections/0004-event-listings/events/by-id/", a.handleRecordProjection)
	mux.HandleFunc("GET /v1/projections/0004-event-listings/events", a.handleDirectoryProjection)
	mux.HandleFunc("GET /v1/projections/0004-event-listings/calendar", a.handleCalendarProjection)
	mux.HandleFunc("GET /v1/projections/0004-event-listings/events/", a.handleDetailProjection)
	mux.HandleFunc("POST /v1/commands/0004-event-listings/events.create", a.handleCreateCommand)
	mux.HandleFunc("POST /v1/commands/0004-event-listings/events.update", a.handleUpdateCommand)
	mux.HandleFunc("POST /v1/commands/0004-event-listings/events.publish", a.handlePublishCommand)
	mux.HandleFunc("POST /v1/commands/0004-event-listings/events.unpublish", a.handleUnpublishCommand)
	mux.HandleFunc("POST /v1/commands/0004-event-listings/events.cancel", a.handleCancelCommand)
	mux.HandleFunc("POST /v1/commands/0004-event-listings/events.archive", a.handleArchiveCommand)

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
		log.Printf("event listings ledger web listening on unix:%s", addr)
		if err := http.Serve(ln, requestLog(mux)); err != nil && !errors.Is(err, net.ErrClosed) {
			log.Fatal(err)
		}
		return
	}

	log.Printf("event listings ledger web listening on http://%s", addr)
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
		"seed":   "0004-event-listings",
	})
}

type homePageData struct {
	Title          string
	CurrentPath    string
	Filters        directoryFilters
	Events         []*eventView
	Featured       *eventView
	Spotlights     []*eventView
	Categories     []string
	Locations      []string
	Stats          statsView
	Flash          string
	MonthLabel     string
	MonthURL       string
	CalendarURL    string
	AdminAvailable bool
}

type eventView struct {
	ID             string   `json:"id"`
	EventID        string   `json:"event_id"`
	Slug           string   `json:"slug"`
	Title          string   `json:"title"`
	Summary        string   `json:"summary"`
	Description    string   `json:"description"`
	Venue          string   `json:"venue"`
	VenueNote      string   `json:"venue_note"`
	Neighborhood   string   `json:"neighborhood"`
	Location       string   `json:"location"`
	Category       string   `json:"category"`
	OrganizerName  string   `json:"organizer_name"`
	OrganizerRole  string   `json:"organizer_role"`
	OrganizerURL   string   `json:"organizer_url,omitempty"`
	CoverImageURL  string   `json:"cover_image_url,omitempty"`
	FeaturedBlurb  string   `json:"featured_blurb,omitempty"`
	ExternalURL    string   `json:"external_url,omitempty"`
	Timezone       string   `json:"timezone"`
	Tags           []string `json:"tags,omitempty"`
	ShareCount     int      `json:"share_count"`
	SaveCount      int      `json:"save_count"`
	CrowdLabel     string   `json:"crowd_label,omitempty"`
	AllDay         bool     `json:"all_day"`
	Status         string   `json:"status"`
	PublicURL      string   `json:"public_url"`
	AbsoluteURL    string   `json:"absolute_url"`
	StartISO       string   `json:"start"`
	EndISO         string   `json:"end"`
	RangeLabel     string   `json:"range_label"`
	StatusLabel    string   `json:"status_label"`
	LocationLabel  string   `json:"location_label"`
	ThemeClass     string   `json:"theme_class"`
	DayName        string   `json:"day_name"`
	MonthShort     string   `json:"month_short"`
	DayNumber      string   `json:"day_number"`
	TimeBadge      string   `json:"time_badge"`
	DiscoveryNote  string   `json:"discovery_note"`
	Atmosphere     string   `json:"atmosphere"`
	ShareLabel     string   `json:"share_label"`
	SaveLabel      string   `json:"save_label"`
	OrganizerLabel string   `json:"organizer_label"`
	LedgerURL      string   `json:"ledger_url"`
	RecordAPIURL   string   `json:"record_api_url"`
	HandleAPIURL   string   `json:"handle_api_url"`
	LedgerAPIURL   string   `json:"ledger_api_url"`
	WorkspaceAPI   string   `json:"workspace_api_url,omitempty"`
	RevisionCount  int      `json:"revision_count"`
}

type statsView struct {
	Upcoming     int
	Canceled     int
	Categories   int
	OrganizerURL string
}

func (a *app) handleHome(w http.ResponseWriter, r *http.Request) {
	filters := parseDirectoryFilters(r)
	events := a.publicDirectory(filters)
	categories, locations := a.filterOptions()
	nowMonth := time.Now().Format("2006-01")
	data := homePageData{
		Title:          "Event Listings",
		CurrentPath:    r.URL.Path,
		Filters:        filters,
		Events:         events,
		Featured:       firstEvent(events),
		Spotlights:     takeEvents(events, 3),
		Categories:     categories,
		Locations:      locations,
		Stats:          a.stats(),
		Flash:          r.URL.Query().Get("flash"),
		MonthLabel:     monthLabel(nowMonth),
		MonthURL:       "/calendar?month=" + nowMonth,
		CalendarURL:    "/calendar",
		AdminAvailable: true,
	}
	a.render(w, "home", data)
}

type calendarDayView struct {
	Date      string
	DayNumber int
	InMonth   bool
	Events    []*eventView
}

type calendarPageData struct {
	Title       string
	CurrentPath string
	Month       string
	MonthLabel  string
	PrevMonth   string
	NextMonth   string
	Highlights  []*eventView
	Days        []calendarDayView
}

func (a *app) handleCalendar(w http.ResponseWriter, r *http.Request) {
	month := r.URL.Query().Get("month")
	if month == "" {
		month = time.Now().Format("2006-01")
	}
	monthStart, err := time.Parse("2006-01", month)
	if err != nil {
		http.Error(w, "invalid month", http.StatusBadRequest)
		return
	}
	data := calendarPageData{
		Title:       "Event Calendar",
		CurrentPath: r.URL.Path,
		Month:       month,
		MonthLabel:  monthLabel(month),
		PrevMonth:   monthStart.AddDate(0, -1, 0).Format("2006-01"),
		NextMonth:   monthStart.AddDate(0, 1, 0).Format("2006-01"),
		Highlights:  a.calendarHighlights(month, 3),
		Days:        a.calendarDays(month),
	}
	a.render(w, "calendar", data)
}

type detailPageData struct {
	Title       string
	CurrentPath string
	Event       *eventView
	Warning     string
	IsAdmin     bool
	Related     []*eventView
}

func (a *app) handleEventDetail(w http.ResponseWriter, r *http.Request) {
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/events/"), "/")
	if strings.HasSuffix(path, "/ledger") {
		a.handleEventLedger(w, r, strings.TrimSuffix(path, "/ledger"))
		return
	}
	event, ok := a.store.bySlug(path)
	if !ok {
		http.NotFound(w, r)
		return
	}
	isAdmin := a.isAdmin(r)
	if event.Status == statusDraft && !isAdmin {
		http.NotFound(w, r)
		return
	}
	warning := ""
	if event.Status == statusCanceled {
		warning = "This event remains listed, but it has been canceled."
	}
	if event.Status == statusArchived {
		warning = "This event has been archived and no longer appears in default upcoming discovery."
	}
	a.render(w, "detail", detailPageData{
		Title:       event.Title,
		CurrentPath: r.URL.Path,
		Event:       toEventView(event),
		Warning:     warning,
		IsAdmin:     isAdmin,
		Related:     a.relatedEvents(event.ID, event.Category, 3),
	})
}

type ledgerPageData struct {
	Title       string
	CurrentPath string
	Event       *eventView
	Claims      []eventClaimView
	IsAdmin     bool
}

type eventClaimView struct {
	ID                string
	ClaimType         string
	Summary           string
	AcceptedAt        string
	AcceptedBy        string
	SupersedesClaimID string
	Status            string
	PayloadJSON       string
}

func (a *app) handleEventLedger(w http.ResponseWriter, r *http.Request, slug string) {
	event, ok := a.store.bySlug(strings.Trim(slug, "/"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	isAdmin := a.isAdmin(r)
	if event.Status == statusDraft && !isAdmin {
		http.NotFound(w, r)
		return
	}
	claims, err := a.store.ledgerByID(event.ID)
	if err != nil {
		http.Error(w, "could not load event ledger", http.StatusBadGateway)
		return
	}
	a.render(w, "ledger", ledgerPageData{
		Title:       event.Title + " Ledger",
		CurrentPath: r.URL.Path,
		Event:       toEventView(event),
		Claims:      toClaimViews(claims),
		IsAdmin:     isAdmin,
	})
}

type adminDashboardData struct {
	Title          string
	CurrentPath    string
	Events         []*eventView
	Flash          string
	OrganizerCount int
}

func (a *app) handleAdminDashboard(w http.ResponseWriter, r *http.Request) {
	events := a.store.all()
	views := make([]*eventView, 0, len(events))
	for _, event := range events {
		views = append(views, toEventView(event))
	}
	a.render(w, "admin_dashboard", adminDashboardData{
		Title:          "Organizer Workspace",
		CurrentPath:    "/admin",
		Events:         views,
		Flash:          r.URL.Query().Get("flash"),
		OrganizerCount: len(views),
	})
}

type adminFormData struct {
	Title       string
	CurrentPath string
	Mode        string
	Flash       string
	Error       string
	Event       eventFormView
}

type eventFormView struct {
	ID            string
	Slug          string
	Title         string
	Summary       string
	Description   string
	Venue         string
	VenueNote     string
	Neighborhood  string
	Location      string
	Category      string
	OrganizerName string
	OrganizerRole string
	OrganizerURL  string
	CoverImageURL string
	FeaturedBlurb string
	Tags          string
	CrowdLabel    string
	ExternalURL   string
	Timezone      string
	AllDay        bool
	StartDate     string
	EndDate       string
	StartTime     string
	EndTime       string
	Status        string
}

func blankEventForm() eventFormView {
	return eventFormView{
		Timezone:      "America/Toronto",
		OrganizerRole: "Independent organizer",
		StartDate:     time.Now().Format("2006-01-02"),
		EndDate:       time.Now().Format("2006-01-02"),
		StartTime:     "18:00",
		EndTime:       "20:00",
	}
}

func formFromEvent(event *eventRecord) eventFormView {
	loc, _ := time.LoadLocation(event.Timezone)
	start := event.Start.In(loc)
	end := event.End.In(loc)
	view := eventFormView{
		ID:            event.ID,
		Slug:          event.Slug,
		Title:         event.Title,
		Summary:       event.Summary,
		Description:   event.Description,
		Venue:         event.Venue,
		VenueNote:     event.VenueNote,
		Neighborhood:  event.Neighborhood,
		Location:      event.Location,
		Category:      event.Category,
		OrganizerName: event.OrganizerName,
		OrganizerRole: event.OrganizerRole,
		OrganizerURL:  event.OrganizerURL,
		CoverImageURL: event.CoverImageURL,
		FeaturedBlurb: event.FeaturedBlurb,
		Tags:          strings.Join(event.Tags, ", "),
		CrowdLabel:    event.CrowdLabel,
		ExternalURL:   event.ExternalURL,
		Timezone:      event.Timezone,
		AllDay:        event.AllDay,
		StartDate:     start.Format("2006-01-02"),
		EndDate:       end.Format("2006-01-02"),
		StartTime:     start.Format("15:04"),
		EndTime:       end.Format("15:04"),
		Status:        string(event.Status),
	}
	if event.AllDay {
		view.StartTime = ""
		view.EndTime = ""
	}
	return view
}

func (a *app) handleAdminNew(w http.ResponseWriter, _ *http.Request) {
	a.render(w, "admin_form", adminFormData{
		Title:       "New Event",
		CurrentPath: "/admin/events/new",
		Mode:        "create",
		Event:       blankEventForm(),
	})
}

func (a *app) handleAdminCreatePost(w http.ResponseWriter, r *http.Request) {
	input, err := parseEventInput(r)
	if err != nil {
		a.render(w, "admin_form", adminFormData{
			Title:       "New Event",
			CurrentPath: "/admin/events/new",
			Mode:        "create",
			Error:       err.Error(),
			Event:       formFromInput(input),
		})
		return
	}
	event, err := a.store.create(input)
	if err != nil {
		a.render(w, "admin_form", adminFormData{
			Title:       "New Event",
			CurrentPath: "/admin/events/new",
			Mode:        "create",
			Error:       err.Error(),
			Event:       formFromInput(input),
		})
		return
	}
	http.Redirect(w, r, "/admin?flash="+url.QueryEscape("Draft created for "+event.Title), http.StatusSeeOther)
}

func (a *app) handleAdminEventRoutes(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/admin/events/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 2 && parts[1] == "edit" && r.Method == http.MethodGet {
		a.handleAdminEdit(w, r, parts[0])
		return
	}
	if len(parts) == 1 && r.Method == http.MethodPost {
		a.handleAdminUpdatePost(w, r, parts[0])
		return
	}
	if len(parts) == 2 && parts[1] == "state" && r.Method == http.MethodPost {
		a.handleAdminStatePost(w, r, parts[0])
		return
	}
	http.NotFound(w, r)
}

func (a *app) handleAdminEdit(w http.ResponseWriter, _ *http.Request, id string) {
	event, ok := a.store.byID(id)
	if !ok {
		http.Error(w, "event not found", http.StatusNotFound)
		return
	}
	a.render(w, "admin_form", adminFormData{
		Title:       "Edit Event",
		CurrentPath: "/admin",
		Mode:        "edit",
		Event:       formFromEvent(event),
	})
}

func (a *app) handleAdminUpdatePost(w http.ResponseWriter, r *http.Request, id string) {
	input, err := parseEventInput(r)
	if err != nil {
		inputView := formFromInput(input)
		inputView.ID = id
		if existing, ok := a.store.byID(id); ok {
			inputView.Slug = existing.Slug
			inputView.Status = string(existing.Status)
		}
		a.render(w, "admin_form", adminFormData{
			Title:       "Edit Event",
			CurrentPath: "/admin",
			Mode:        "edit",
			Error:       err.Error(),
			Event:       inputView,
		})
		return
	}
	event, err := a.store.update(id, input)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	http.Redirect(w, r, "/admin?flash="+url.QueryEscape("Updated "+event.Title+" without changing "+event.Slug), http.StatusSeeOther)
}

func (a *app) handleAdminStatePost(w http.ResponseWriter, r *http.Request, id string) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	action := r.PostForm.Get("action")
	var target eventStatus
	switch action {
	case "publish":
		target = statusPublished
	case "unpublish":
		target = statusDraft
	case "cancel":
		target = statusCanceled
	case "archive":
		target = statusArchived
	default:
		http.Error(w, "invalid action", http.StatusBadRequest)
		return
	}
	event, err := a.store.setStatus(id, target)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	http.Redirect(w, r, "/admin?flash="+url.QueryEscape(event.Title+" is now "+string(event.Status)), http.StatusSeeOther)
}

func (a *app) handleAdminLogin(w http.ResponseWriter, r *http.Request) {
	if a.isAdmin(r) {
		http.Redirect(w, r, "/admin", http.StatusSeeOther)
		return
	}
	a.render(w, "admin_login", map[string]any{
		"Title":       "Organizer Login",
		"CurrentPath": "/admin/login",
		"Error":       r.URL.Query().Get("error"),
	})
}

func (a *app) handleAdminLoginPost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	if r.PostForm.Get("password") != a.adminPassword {
		http.Redirect(w, r, "/admin/login?error="+url.QueryEscape("Incorrect password"), http.StatusSeeOther)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     adminCookieName,
		Value:    "ok",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

func (a *app) handleAdminLogoutPost(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     adminCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})
	http.Redirect(w, r, "/", http.StatusSeeOther)
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

type directoryFilters struct {
	Query    string `json:"q,omitempty"`
	Category string `json:"category,omitempty"`
	Location string `json:"location,omitempty"`
	From     string `json:"from,omitempty"`
	To       string `json:"to,omitempty"`
}

type workspaceFilters struct {
	Query  string `json:"q,omitempty"`
	Status string `json:"status,omitempty"`
}

func parseDirectoryFilters(r *http.Request) directoryFilters {
	q := r.URL.Query()
	return directoryFilters{
		Query:    strings.TrimSpace(q.Get("q")),
		Category: strings.TrimSpace(q.Get("category")),
		Location: strings.TrimSpace(q.Get("location")),
		From:     strings.TrimSpace(q.Get("from")),
		To:       strings.TrimSpace(q.Get("to")),
	}
}

func parseWorkspaceFilters(r *http.Request) workspaceFilters {
	q := r.URL.Query()
	return workspaceFilters{
		Query:  strings.TrimSpace(q.Get("q")),
		Status: strings.TrimSpace(q.Get("status")),
	}
}

func (a *app) publicDirectory(filters directoryFilters) []*eventView {
	all := a.store.all()
	results := make([]*eventView, 0, len(all))
	for _, event := range all {
		if !isPublic(event) {
			continue
		}
		if !isUpcoming(event) {
			continue
		}
		if !matchesFilters(event, filters) {
			continue
		}
		results = append(results, toEventView(event))
	}
	return results
}

func (a *app) workspaceEvents(filters workspaceFilters) []*eventView {
	all := a.store.all()
	results := make([]*eventView, 0, len(all))
	for _, event := range all {
		if !matchesWorkspaceFilters(event, filters) {
			continue
		}
		results = append(results, toEventView(event))
	}
	return results
}

func (a *app) filterOptions() ([]string, []string) {
	all := a.store.all()
	catSet := map[string]struct{}{}
	locSet := map[string]struct{}{}
	for _, event := range all {
		if event.Category != "" {
			catSet[event.Category] = struct{}{}
		}
		if event.Location != "" {
			locSet[event.Location] = struct{}{}
		}
	}
	categories := make([]string, 0, len(catSet))
	for category := range catSet {
		categories = append(categories, category)
	}
	locations := make([]string, 0, len(locSet))
	for location := range locSet {
		locations = append(locations, location)
	}
	sort.Strings(categories)
	sort.Strings(locations)
	return categories, locations
}

func (a *app) stats() statsView {
	all := a.store.all()
	categories := map[string]struct{}{}
	var upcoming, canceled int
	for _, event := range all {
		if isPublic(event) && isUpcoming(event) {
			upcoming++
		}
		if event.Status == statusCanceled {
			canceled++
		}
		if event.Category != "" {
			categories[event.Category] = struct{}{}
		}
	}
	return statsView{
		Upcoming:     upcoming,
		Canceled:     canceled,
		Categories:   len(categories),
		OrganizerURL: "/admin",
	}
}

func (a *app) calendarDays(month string) []calendarDayView {
	start, err := time.Parse("2006-01", month)
	if err != nil {
		return nil
	}
	firstDay := time.Date(start.Year(), start.Month(), 1, 0, 0, 0, 0, time.UTC)
	gridStart := firstDay.AddDate(0, 0, -int(firstDay.Weekday()))
	nextMonth := firstDay.AddDate(0, 1, 0)

	all := a.store.all()
	days := make([]calendarDayView, 0, 42)
	for i := 0; i < 42; i++ {
		day := gridStart.AddDate(0, 0, i)
		view := calendarDayView{
			Date:      day.Format("2006-01-02"),
			DayNumber: day.Day(),
			InMonth:   day.Month() == firstDay.Month(),
		}
		for _, event := range all {
			if !isPublic(event) || !overlapsCalendarDay(event, day) {
				continue
			}
			if day.Before(firstDay) || !day.Before(nextMonth) {
				// Show overflow days too; they are already visually muted.
			}
			view.Events = append(view.Events, toEventView(event))
		}
		sort.Slice(view.Events, func(i, j int) bool {
			return view.Events[i].StartISO < view.Events[j].StartISO
		})
		days = append(days, view)
	}
	return days
}

func (a *app) calendarHighlights(month string, limit int) []*eventView {
	start, err := time.Parse("2006-01", month)
	if err != nil {
		return nil
	}
	end := start.AddDate(0, 1, 0)
	items := make([]*eventView, 0, limit)
	for _, event := range a.store.all() {
		if !isPublic(event) {
			continue
		}
		if event.End.Before(start) || !event.Start.Before(end) {
			continue
		}
		items = append(items, toEventView(event))
		if len(items) == limit {
			break
		}
	}
	return items
}

func (a *app) relatedEvents(id, category string, limit int) []*eventView {
	items := make([]*eventView, 0, limit)
	fallback := make([]*eventView, 0, limit)
	for _, event := range a.store.all() {
		if event.ID == id || !isPublic(event) || !isUpcoming(event) {
			continue
		}
		view := toEventView(event)
		if strings.EqualFold(event.Category, category) && len(items) < limit {
			items = append(items, view)
			continue
		}
		if len(fallback) < limit {
			fallback = append(fallback, view)
		}
	}
	for _, item := range fallback {
		if len(items) == limit {
			break
		}
		items = append(items, item)
	}
	return items
}

func (a *app) handleDirectoryProjection(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"filters":   parseDirectoryFilters(r),
		"events":    a.publicDirectory(parseDirectoryFilters(r)),
		"discovery": eventProjectionDiscovery(),
	})
}

func (a *app) handleCalendarProjection(w http.ResponseWriter, r *http.Request) {
	month := r.URL.Query().Get("month")
	if month == "" {
		month = time.Now().Format("2006-01")
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"month":     month,
		"days":      a.calendarDays(month),
		"discovery": eventProjectionDiscovery(),
	})
}

func (a *app) handleWorkspaceProjection(w http.ResponseWriter, r *http.Request) {
	if !a.isProjectionAuthorized(r) {
		writeJSONError(w, http.StatusUnauthorized, "authentication required for organizer workspace projection")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"filters":   parseWorkspaceFilters(r),
		"events":    a.workspaceEvents(parseWorkspaceFilters(r)),
		"discovery": eventProjectionDiscovery(),
	})
}

func (a *app) handleRecordProjection(w http.ResponseWriter, r *http.Request) {
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/v1/projections/0004-event-listings/events/by-id/"), "/")
	if path == "" {
		http.NotFound(w, r)
		return
	}
	if strings.HasSuffix(path, "/ledger") {
		eventID := strings.Trim(strings.TrimSuffix(path, "/ledger"), "/")
		event, ok := a.store.byID(eventID)
		if !ok || !a.canReadEvent(r, event) {
			http.NotFound(w, r)
			return
		}
		claims, err := a.store.ledgerByID(event.ID)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"event":     toEventView(event),
			"claims":    claims,
			"discovery": eventProjectionDiscovery(),
		})
		return
	}
	event, ok := a.store.byID(path)
	if !ok || !a.canReadEvent(r, event) {
		http.NotFound(w, r)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"event":     toEventView(event),
		"discovery": eventProjectionDiscovery(),
	})
}

func (a *app) handleDetailProjection(w http.ResponseWriter, r *http.Request) {
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/v1/projections/0004-event-listings/events/"), "/")
	if strings.HasSuffix(path, "/ledger") {
		eventRef := strings.Trim(strings.TrimSuffix(path, "/ledger"), "/")
		event, ok := a.lookupEventRef(eventRef)
		if !ok || !a.canReadEvent(r, event) {
			http.NotFound(w, r)
			return
		}
		claims, err := a.store.ledgerByID(event.ID)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"event":     toEventView(event),
			"claims":    claims,
			"discovery": eventProjectionDiscovery(),
		})
		return
	}
	event, ok := a.lookupEventRef(path)
	if !ok || !a.canReadEvent(r, event) {
		http.NotFound(w, r)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"event":     toEventView(event),
		"discovery": eventProjectionDiscovery(),
	})
}

func (a *app) handleCreateCommand(w http.ResponseWriter, r *http.Request) {
	if !a.isCommandAuthorized(r) {
		writeJSONError(w, http.StatusUnauthorized, "authentication required for event command")
		return
	}
	var payload eventCommandPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	event, err := a.store.create(payload.toInput())
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"event": toEventView(event),
	})
}

func (a *app) handleUpdateCommand(w http.ResponseWriter, r *http.Request) {
	if !a.isCommandAuthorized(r) {
		writeJSONError(w, http.StatusUnauthorized, "authentication required for event command")
		return
	}
	var payload eventCommandPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	event, ok := a.lookupEvent(payload.EventID, payload.Slug)
	if !ok {
		http.NotFound(w, r)
		return
	}
	updated, err := a.store.update(event.ID, payload.toInput())
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"event": toEventView(updated),
	})
}

func (a *app) handlePublishCommand(w http.ResponseWriter, r *http.Request) {
	if !a.isCommandAuthorized(r) {
		writeJSONError(w, http.StatusUnauthorized, "authentication required for event command")
		return
	}
	var payload struct {
		EventID string `json:"event_id"`
		Slug    string `json:"slug"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	event, ok := a.lookupEvent(payload.EventID, payload.Slug)
	if !ok {
		http.NotFound(w, r)
		return
	}
	published, err := a.store.setStatus(event.ID, statusPublished)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"event": toEventView(published),
	})
}

func (a *app) handleUnpublishCommand(w http.ResponseWriter, r *http.Request) {
	a.handleStateCommand(w, r, statusDraft)
}

func (a *app) handleCancelCommand(w http.ResponseWriter, r *http.Request) {
	a.handleStateCommand(w, r, statusCanceled)
}

func (a *app) handleArchiveCommand(w http.ResponseWriter, r *http.Request) {
	a.handleStateCommand(w, r, statusArchived)
}

func (a *app) handleStateCommand(w http.ResponseWriter, r *http.Request, target eventStatus) {
	if !a.isCommandAuthorized(r) {
		writeJSONError(w, http.StatusUnauthorized, "authentication required for event command")
		return
	}
	var payload struct {
		EventID string `json:"event_id"`
		Slug    string `json:"slug"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	event, ok := a.lookupEvent(payload.EventID, payload.Slug)
	if !ok {
		http.NotFound(w, r)
		return
	}
	updated, err := a.store.setStatus(event.ID, target)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"event": toEventView(updated),
	})
}

func (a *app) lookupEvent(id, slug string) (*eventRecord, bool) {
	if id != "" {
		return a.store.byID(id)
	}
	if slug != "" {
		return a.store.bySlug(slug)
	}
	return nil, false
}

func (a *app) lookupEventRef(ref string) (*eventRecord, bool) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil, false
	}
	if event, ok := a.store.byID(ref); ok {
		return event, true
	}
	return a.store.bySlug(ref)
}

type eventCommandPayload struct {
	EventID       string `json:"event_id,omitempty"`
	Slug          string `json:"slug,omitempty"`
	Title         string `json:"title"`
	Summary       string `json:"summary"`
	Description   string `json:"description"`
	Venue         string `json:"venue"`
	VenueNote     string `json:"venue_note"`
	Neighborhood  string `json:"neighborhood"`
	Location      string `json:"location"`
	Category      string `json:"category"`
	OrganizerName string `json:"organizer_name"`
	OrganizerRole string `json:"organizer_role"`
	OrganizerURL  string `json:"organizer_url"`
	CoverImageURL string `json:"cover_image_url"`
	FeaturedBlurb string `json:"featured_blurb"`
	Tags          string `json:"tags"`
	CrowdLabel    string `json:"crowd_label"`
	ExternalURL   string `json:"external_url"`
	Timezone      string `json:"timezone"`
	AllDay        bool   `json:"all_day"`
	StartDate     string `json:"start_date"`
	EndDate       string `json:"end_date"`
	StartTime     string `json:"start_time"`
	EndTime       string `json:"end_time"`
}

func (p eventCommandPayload) toInput() eventInput {
	return eventInput{
		Title:         p.Title,
		Summary:       p.Summary,
		Description:   p.Description,
		Venue:         p.Venue,
		VenueNote:     p.VenueNote,
		Neighborhood:  p.Neighborhood,
		Location:      p.Location,
		Category:      p.Category,
		OrganizerName: p.OrganizerName,
		OrganizerRole: p.OrganizerRole,
		OrganizerURL:  p.OrganizerURL,
		CoverImageURL: p.CoverImageURL,
		FeaturedBlurb: p.FeaturedBlurb,
		Tags:          p.Tags,
		CrowdLabel:    p.CrowdLabel,
		ExternalURL:   p.ExternalURL,
		Timezone:      p.Timezone,
		AllDay:        p.AllDay,
		StartDate:     p.StartDate,
		EndDate:       p.EndDate,
		StartTime:     p.StartTime,
		EndTime:       p.EndTime,
	}
}

func parseEventInput(r *http.Request) (eventInput, error) {
	if err := r.ParseMultipartForm(maxFormMemory); err != nil && !errors.Is(err, http.ErrNotMultipart) {
		return eventInput{}, errors.New("could not parse form")
	}
	input := eventInput{
		Title:         r.FormValue("title"),
		Summary:       r.FormValue("summary"),
		Description:   r.FormValue("description"),
		Venue:         r.FormValue("venue"),
		VenueNote:     r.FormValue("venue_note"),
		Neighborhood:  r.FormValue("neighborhood"),
		Location:      r.FormValue("location"),
		Category:      r.FormValue("category"),
		OrganizerName: r.FormValue("organizer_name"),
		OrganizerRole: r.FormValue("organizer_role"),
		OrganizerURL:  r.FormValue("organizer_url"),
		CoverImageURL: r.FormValue("cover_image_url"),
		FeaturedBlurb: r.FormValue("featured_blurb"),
		Tags:          r.FormValue("tags"),
		CrowdLabel:    r.FormValue("crowd_label"),
		ExternalURL:   r.FormValue("external_url"),
		Timezone:      r.FormValue("timezone"),
		AllDay:        r.FormValue("all_day") == "on",
		StartDate:     r.FormValue("start_date"),
		EndDate:       r.FormValue("end_date"),
		StartTime:     r.FormValue("start_time"),
		EndTime:       r.FormValue("end_time"),
	}
	if strings.TrimSpace(input.Title) == "" {
		return input, errors.New("title is required")
	}
	if strings.TrimSpace(input.Summary) == "" {
		return input, errors.New("summary is required")
	}
	if strings.TrimSpace(input.Description) == "" {
		return input, errors.New("description is required")
	}
	if strings.TrimSpace(input.Category) == "" {
		return input, errors.New("category is required")
	}
	if strings.TrimSpace(input.OrganizerName) == "" {
		return input, errors.New("organizer name is required")
	}
	if strings.TrimSpace(input.Location) == "" {
		return input, errors.New("location is required")
	}
	if strings.TrimSpace(input.Timezone) == "" {
		input.Timezone = "America/Toronto"
	}
	if input.ExternalURL != "" {
		if _, err := url.ParseRequestURI(input.ExternalURL); err != nil {
			return input, errors.New("external URL must be a valid absolute URL")
		}
	}
	if input.OrganizerURL != "" {
		if _, err := url.ParseRequestURI(input.OrganizerURL); err != nil {
			return input, errors.New("organizer URL must be a valid absolute URL")
		}
	}
	if input.CoverImageURL != "" {
		if _, err := url.ParseRequestURI(input.CoverImageURL); err != nil {
			return input, errors.New("cover image URL must be a valid absolute URL")
		}
	}
	if _, _, err := parseSchedule(input); err != nil {
		return input, err
	}
	return input, nil
}

func parseSchedule(input eventInput) (time.Time, time.Time, error) {
	locationName := strings.TrimSpace(input.Timezone)
	if locationName == "" {
		locationName = "America/Toronto"
	}
	loc, err := time.LoadLocation(locationName)
	if err != nil {
		return time.Time{}, time.Time{}, errors.New("timezone must be valid")
	}
	if input.StartDate == "" || input.EndDate == "" {
		return time.Time{}, time.Time{}, errors.New("start and end dates are required")
	}
	if input.AllDay {
		start, err := time.ParseInLocation("2006-01-02", input.StartDate, loc)
		if err != nil {
			return time.Time{}, time.Time{}, errors.New("start date must be valid")
		}
		endDate, err := time.ParseInLocation("2006-01-02", input.EndDate, loc)
		if err != nil {
			return time.Time{}, time.Time{}, errors.New("end date must be valid")
		}
		end := time.Date(endDate.Year(), endDate.Month(), endDate.Day(), 23, 59, 0, 0, loc)
		if end.Before(start) {
			return time.Time{}, time.Time{}, errors.New("end date must be on or after start date")
		}
		return start, end, nil
	}

	if input.StartTime == "" || input.EndTime == "" {
		return time.Time{}, time.Time{}, errors.New("start and end times are required unless the event is all day")
	}
	start, err := time.ParseInLocation("2006-01-02 15:04", input.StartDate+" "+input.StartTime, loc)
	if err != nil {
		return time.Time{}, time.Time{}, errors.New("start date/time must be valid")
	}
	end, err := time.ParseInLocation("2006-01-02 15:04", input.EndDate+" "+input.EndTime, loc)
	if err != nil {
		return time.Time{}, time.Time{}, errors.New("end date/time must be valid")
	}
	if !end.After(start) {
		return time.Time{}, time.Time{}, errors.New("end must be after start")
	}
	return start, end, nil
}

func isPublic(event *eventRecord) bool {
	return event.Status == statusPublished || event.Status == statusCanceled
}

func isUpcoming(event *eventRecord) bool {
	loc, err := time.LoadLocation(event.Timezone)
	if err != nil {
		return false
	}
	now := time.Now().In(loc)
	endDay := time.Date(event.End.In(loc).Year(), event.End.In(loc).Month(), event.End.In(loc).Day(), 23, 59, 0, 0, loc)
	return !endDay.Before(now)
}

func matchesFilters(event *eventRecord, filters directoryFilters) bool {
	if filters.Query != "" {
		haystack := strings.ToLower(strings.Join([]string{
			event.Title, event.Summary, event.Description, event.Venue, event.Location, event.Category,
		}, " "))
		if !strings.Contains(haystack, strings.ToLower(filters.Query)) {
			return false
		}
	}
	if filters.Category != "" && !strings.EqualFold(filters.Category, event.Category) {
		return false
	}
	if filters.Location != "" && !strings.Contains(strings.ToLower(event.Location), strings.ToLower(filters.Location)) {
		return false
	}
	from, to := parseDateRange(filters)
	if !from.IsZero() || !to.IsZero() {
		if !overlapsDateRange(event, from, to) {
			return false
		}
	}
	return true
}

func matchesWorkspaceFilters(event *eventRecord, filters workspaceFilters) bool {
	if filters.Status != "" && !strings.EqualFold(filters.Status, string(event.Status)) {
		return false
	}
	if filters.Query == "" {
		return true
	}
	haystack := strings.ToLower(strings.Join([]string{
		event.ID,
		event.Slug,
		event.Title,
		event.Summary,
		event.Description,
		event.Venue,
		event.Location,
		event.Category,
		string(event.Status),
	}, " "))
	return strings.Contains(haystack, strings.ToLower(filters.Query))
}

func parseDateRange(filters directoryFilters) (time.Time, time.Time) {
	var from, to time.Time
	if filters.From != "" {
		from, _ = time.Parse("2006-01-02", filters.From)
	}
	if filters.To != "" {
		to, _ = time.Parse("2006-01-02", filters.To)
	}
	return from, to
}

func overlapsDateRange(event *eventRecord, from, to time.Time) bool {
	loc, err := time.LoadLocation(event.Timezone)
	if err != nil {
		return false
	}
	start := event.Start.In(loc)
	end := event.End.In(loc)
	eventStart := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.UTC)
	eventEnd := time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, time.UTC)
	if !from.IsZero() && eventEnd.Before(from) {
		return false
	}
	if !to.IsZero() && eventStart.After(to) {
		return false
	}
	return true
}

func overlapsCalendarDay(event *eventRecord, day time.Time) bool {
	loc, err := time.LoadLocation(event.Timezone)
	if err != nil {
		return false
	}
	eventStart := event.Start.In(loc)
	eventEnd := event.End.In(loc)
	dayInLoc := day.In(loc)
	startDay := time.Date(eventStart.Year(), eventStart.Month(), eventStart.Day(), 0, 0, 0, 0, loc)
	endDay := time.Date(eventEnd.Year(), eventEnd.Month(), eventEnd.Day(), 0, 0, 0, 0, loc)
	target := time.Date(dayInLoc.Year(), dayInLoc.Month(), dayInLoc.Day(), 0, 0, 0, 0, loc)
	return !target.Before(startDay) && !target.After(endDay)
}

func toEventView(event *eventRecord) *eventView {
	loc := loadLocationOrUTC(event.Timezone)
	start := event.Start.In(loc)
	end := event.End.In(loc)
	publicURL := "/events/" + event.Slug
	recordAPIURL := "/v1/projections/0004-event-listings/events/by-id/" + url.PathEscape(event.ID)
	handleAPIURL := "/v1/projections/0004-event-listings/events/" + url.PathEscape(event.Slug)
	return &eventView{
		ID:             event.ID,
		EventID:        event.ID,
		Slug:           event.Slug,
		Title:          event.Title,
		Summary:        event.Summary,
		Description:    event.Description,
		Venue:          event.Venue,
		VenueNote:      event.VenueNote,
		Neighborhood:   event.Neighborhood,
		Location:       event.Location,
		Category:       event.Category,
		OrganizerName:  event.OrganizerName,
		OrganizerRole:  event.OrganizerRole,
		OrganizerURL:   event.OrganizerURL,
		CoverImageURL:  event.CoverImageURL,
		FeaturedBlurb:  event.FeaturedBlurb,
		ExternalURL:    event.ExternalURL,
		Timezone:       event.Timezone,
		Tags:           append([]string(nil), event.Tags...),
		ShareCount:     event.ShareCount,
		SaveCount:      event.SaveCount,
		CrowdLabel:     event.CrowdLabel,
		AllDay:         event.AllDay,
		Status:         string(event.Status),
		PublicURL:      publicURL,
		AbsoluteURL:    publicURL,
		StartISO:       event.Start.Format(time.RFC3339),
		EndISO:         event.End.Format(time.RFC3339),
		RangeLabel:     formatRange(start, end, event.AllDay, event.Timezone),
		StatusLabel:    strings.Title(string(event.Status)),
		LocationLabel:  joinNonEmpty(" · ", event.Venue, event.Location),
		ThemeClass:     eventThemeClass(event),
		DayName:        strings.ToUpper(start.Format("Mon")),
		MonthShort:     strings.ToUpper(start.Format("Jan")),
		DayNumber:      start.Format("2"),
		TimeBadge:      eventTimeBadge(start, end, event.AllDay),
		DiscoveryNote:  eventDiscoveryNote(event),
		Atmosphere:     eventAtmosphere(event),
		ShareLabel:     formatMetricLabel(event.ShareCount, "share"),
		SaveLabel:      formatMetricLabel(event.SaveCount, "save"),
		OrganizerLabel: joinNonEmpty(" · ", event.OrganizerName, event.OrganizerRole),
		LedgerURL:      publicURL + "/ledger",
		RecordAPIURL:   recordAPIURL,
		HandleAPIURL:   handleAPIURL,
		LedgerAPIURL:   recordAPIURL + "/ledger",
		WorkspaceAPI:   "/v1/projections/0004-event-listings/admin/events",
		RevisionCount:  event.RevisionCount,
	}
}

func eventProjectionDiscovery() map[string]string {
	return map[string]string{
		"workspace":              "/v1/projections/0004-event-listings/admin/events",
		"record_template":        "/v1/projections/0004-event-listings/events/by-id/{event_id}",
		"public_detail_template": "/v1/projections/0004-event-listings/events/{slug}",
		"ledger_template":        "/v1/projections/0004-event-listings/events/by-id/{event_id}/ledger",
		"create_command":         "/v1/commands/0004-event-listings/events.create",
		"update_command":         "/v1/commands/0004-event-listings/events.update",
		"publish_command":        "/v1/commands/0004-event-listings/events.publish",
		"unpublish_command":      "/v1/commands/0004-event-listings/events.unpublish",
		"cancel_command":         "/v1/commands/0004-event-listings/events.cancel",
		"archive_command":        "/v1/commands/0004-event-listings/events.archive",
	}
}

func toClaimViews(claims []eventClaim) []eventClaimView {
	views := make([]eventClaimView, 0, len(claims))
	for _, claim := range claims {
		payloadJSON := ""
		if claim.Payload != nil {
			if body, err := json.MarshalIndent(claim.Payload, "", "  "); err == nil {
				payloadJSON = string(body)
			}
		}
		views = append(views, eventClaimView{
			ID:                claim.ID,
			ClaimType:         claim.ClaimType,
			Summary:           claim.Summary,
			AcceptedAt:        claim.AcceptedAt.Format(time.RFC3339),
			AcceptedBy:        claim.AcceptedBy,
			SupersedesClaimID: claim.SupersedesClaimID,
			Status:            claim.Status,
			PayloadJSON:       payloadJSON,
		})
	}
	return views
}

func firstEvent(events []*eventView) *eventView {
	if len(events) == 0 {
		return nil
	}
	return events[0]
}

func takeEvents(events []*eventView, limit int) []*eventView {
	if limit <= 0 || len(events) == 0 {
		return nil
	}
	if len(events) < limit {
		limit = len(events)
	}
	return events[:limit]
}

func formFromInput(input eventInput) eventFormView {
	return eventFormView{
		Title:         input.Title,
		Summary:       input.Summary,
		Description:   input.Description,
		Venue:         input.Venue,
		VenueNote:     input.VenueNote,
		Neighborhood:  input.Neighborhood,
		Location:      input.Location,
		Category:      input.Category,
		OrganizerName: input.OrganizerName,
		OrganizerRole: input.OrganizerRole,
		OrganizerURL:  input.OrganizerURL,
		CoverImageURL: input.CoverImageURL,
		FeaturedBlurb: input.FeaturedBlurb,
		Tags:          input.Tags,
		CrowdLabel:    input.CrowdLabel,
		ExternalURL:   input.ExternalURL,
		Timezone:      input.Timezone,
		AllDay:        input.AllDay,
		StartDate:     input.StartDate,
		EndDate:       input.EndDate,
		StartTime:     input.StartTime,
		EndTime:       input.EndTime,
	}
}

func uniqueSlug(base, id string) string {
	if base == "" {
		base = "event"
	}
	return base + "-" + id
}

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	lastDash := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
			lastDash = false
		case r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		default:
			if !lastDash {
				b.WriteRune('-')
				lastDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

func formatRange(start, end time.Time, allDay bool, timezone string) string {
	if allDay {
		if sameDay(start, end) {
			return start.Format("Mon Jan 2, 2006") + " · All day · " + timezone
		}
		return start.Format("Mon Jan 2") + " to " + end.Format("Mon Jan 2, 2006") + " · All day · " + timezone
	}
	if sameDay(start, end) {
		return start.Format("Mon Jan 2, 2006 · 3:04 PM") + " to " + end.Format("3:04 PM") + " · " + timezone
	}
	return start.Format("Mon Jan 2, 2006 · 3:04 PM") + " to " + end.Format("Mon Jan 2, 2006 · 3:04 PM") + " · " + timezone
}

func eventTimeBadge(start, end time.Time, allDay bool) string {
	if allDay {
		if sameDay(start, end) {
			return "ALL DAY"
		}
		return "MULTI-DAY"
	}
	return strings.ToUpper(start.Format("3:04 PM"))
}

func eventThemeClass(event *eventRecord) string {
	switch {
	case event.Status == statusCanceled:
		return "ember"
	case strings.EqualFold(event.Category, "Workshop"):
		return "marine"
	case strings.EqualFold(event.Category, "Community"):
		return "sunset"
	case strings.EqualFold(event.Category, "Volunteer"):
		return "grove"
	default:
		return "midnight"
	}
}

func eventDiscoveryNote(event *eventRecord) string {
	if strings.TrimSpace(event.FeaturedBlurb) != "" {
		return event.FeaturedBlurb
	}
	switch {
	case strings.EqualFold(event.Category, "Community"):
		return "Good pick for a group text: social, local, and easy to say yes to after work."
	case strings.EqualFold(event.Category, "Workshop"):
		return "More useful than passive: this is the kind of event people attend to leave with sharper notes and new contacts."
	case strings.EqualFold(event.Category, "Volunteer"):
		return "Shows up best when people want a concrete plan and a clear meeting point."
	case event.Status == statusCanceled:
		return "Kept visible so nobody wastes a trip or assumes it quietly vanished."
	default:
		return "A focused local listing with enough detail to decide quickly and share confidently."
	}
}

func eventAtmosphere(event *eventRecord) string {
	switch {
	case strings.EqualFold(event.Category, "Community"):
		return "Casual evening energy"
	case strings.EqualFold(event.Category, "Workshop"):
		return "Small-room skill building"
	case strings.EqualFold(event.Category, "Volunteer"):
		return "Morning with a purpose"
	default:
		return "Local scene pick"
	}
}

func normalizeTags(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == '\n'
	})
	seen := map[string]struct{}{}
	tags := make([]string, 0, len(parts))
	for _, part := range parts {
		tag := strings.TrimSpace(part)
		if tag == "" {
			continue
		}
		lower := strings.ToLower(tag)
		if _, ok := seen[lower]; ok {
			continue
		}
		seen[lower] = struct{}{}
		tags = append(tags, tag)
	}
	return tags
}

func seededShareCount(id, category string) int {
	base, _ := strconv.Atoi(id)
	switch strings.ToLower(strings.TrimSpace(category)) {
	case "community":
		return 58 + base*9
	case "workshop":
		return 32 + base*7
	case "volunteer":
		return 21 + base*5
	default:
		return 14 + base*4
	}
}

func seededSaveCount(id, category string) int {
	base, _ := strconv.Atoi(id)
	switch strings.ToLower(strings.TrimSpace(category)) {
	case "community":
		return 86 + base*11
	case "workshop":
		return 48 + base*8
	case "volunteer":
		return 24 + base*6
	default:
		return 18 + base*5
	}
}

func formatMetricLabel(value int, singular string) string {
	if value == 1 {
		return "1 " + singular
	}
	return strconv.Itoa(value) + " " + singular + "s"
}

func joinNonEmpty(sep string, values ...string) string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			parts = append(parts, value)
		}
	}
	return strings.Join(parts, sep)
}

func sameDay(a, b time.Time) bool {
	return a.Year() == b.Year() && a.Month() == b.Month() && a.Day() == b.Day()
}

func monthLabel(month string) string {
	t, err := time.Parse("2006-01", month)
	if err != nil {
		return month
	}
	return t.Format("January 2006")
}

func envOrDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func requestLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s (%s)", r.Method, r.URL.Path, time.Since(start).Round(time.Millisecond))
	})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeJSONError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]any{
		"error": map[string]string{
			"message": strings.TrimSpace(message),
		},
	})
}

func (a *app) isProjectionAuthorized(r *http.Request) bool {
	return a.isAdmin(r) || a.hasServiceToken(r)
}

func (a *app) isCommandAuthorized(r *http.Request) bool {
	return a.isProjectionAuthorized(r)
}

func (a *app) hasServiceToken(r *http.Request) bool {
	if strings.TrimSpace(a.serviceToken) == "" || r == nil {
		return false
	}
	if token := strings.TrimSpace(r.Header.Get("X-AS-Service-Token")); token != "" {
		return token == a.serviceToken
	}
	authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
	if !strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
		return false
	}
	return strings.TrimSpace(authHeader[len("Bearer "):]) == a.serviceToken
}

func (a *app) canReadEvent(r *http.Request, event *eventRecord) bool {
	if event == nil {
		return false
	}
	if event.Status != statusDraft {
		return true
	}
	return a.isProjectionAuthorized(r)
}

func (a *app) render(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := a.templates.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func parseTemplates() *template.Template {
	funcs := template.FuncMap{
		"today": func() string { return time.Now().Format("2006-01-02") },
		"coverStyle": func(event *eventView) template.CSS {
			if event == nil || strings.TrimSpace(event.CoverImageURL) == "" {
				return ""
			}
			return template.CSS("background-image: linear-gradient(180deg, rgba(17, 23, 28, 0.12), rgba(17, 23, 28, 0.48)), url('" + template.URL(event.CoverImageURL) + "'); background-size: cover; background-position: center;")
		},
		"mailtoLink": func(event *eventView) string {
			if event == nil {
				return "#"
			}
			subject := url.QueryEscape("You should check out: " + event.Title)
			body := url.QueryEscape(event.Summary + "\n\n" + event.AbsoluteURL)
			return "mailto:?subject=" + subject + "&body=" + body
		},
	}
	return template.Must(template.New("pages").Funcs(funcs).Parse(frameTemplate + homeTemplate + calendarTemplate + detailTemplate + ledgerTemplate + adminLoginTemplate + adminDashboardTemplate + adminFormTemplate))
}

func loadLocationOrUTC(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		return time.UTC
	}
	return loc
}

const frameTemplate = `
{{define "frame_start"}}
<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>{{.Title}}</title>
  <link rel="stylesheet" href="/assets/app.css">
</head>
<body>
  <div class="shell">
    <nav class="nav">
      <a class="brand" href="/">
        <span class="brand-mark">Autosoftware Seed 0004</span>
        <span class="brand-name">Field Guide Events</span>
      </a>
      <div class="nav-links">
        <a class="text-link" href="/">Directory</a>
        <a class="text-link" href="/calendar">Calendar</a>
        <a class="text-link" href="/admin">Organizer</a>
      </div>
    </nav>
{{end}}

{{define "frame_end"}}
    <footer class="footer">
      Public event discovery with stable URLs, explicit time semantics, and organizer-first publishing controls.
    </footer>
  </div>
  <script src="/assets/app.js"></script>
</body>
</html>
{{end}}
`

const homeTemplate = `
{{define "home"}}
  {{template "frame_start" .}}
  <section class="landing">
    <div class="hero-card landing-copy">
      <p class="section-kicker">City picks for people who actually go out</p>
      <h1>Find the nights, workshops, and local scenes worth texting your friends about.</h1>
      <p>Field Guide Events should feel less like a spreadsheet of listings and more like a trustworthy local guide: quick to scan, easy to share, and clear about whether something is worth leaving home for.</p>
      <div class="hero-actions">
        <a class="pill-link" href="#browse">Browse picks</a>
        <a class="pill-link alt" href="{{.MonthURL}}">Open {{.MonthLabel}}</a>
      </div>
      <div class="hero-stats">
        <div class="stat"><strong>{{.Stats.Upcoming}}</strong>live picks this week</div>
        <div class="stat"><strong>{{.Stats.Categories}}</strong>ways to spend the week</div>
        <div class="stat"><strong>{{.Stats.Canceled}}</strong>clear status updates</div>
      </div>
    </div>
    <aside class="hero-side hero-card">
      <p class="section-kicker subtle">This week’s rhythm</p>
      <h2>Fast answers to the real questions.</h2>
      <ul class="scene-points">
        <li>What is actually happening soon?</li>
        <li>Which listing feels social versus useful?</li>
        <li>What can I forward without adding explanation?</li>
      </ul>
      <a class="pill-link alt" href="/admin">Organizer workspace</a>
    </aside>
  </section>

  {{if .Flash}}<div class="notice">{{.Flash}}</div>{{end}}

  {{if .Featured}}
  <section class="feature-band theme-{{.Featured.ThemeClass}}">
    <a class="feature-poster" href="{{.Featured.PublicURL}}" style="{{coverStyle .Featured}}">
      <div class="poster-meta">{{.Featured.Category}} · {{.Featured.Atmosphere}}</div>
      <div class="poster-date">
        <span>{{.Featured.MonthShort}}</span>
        <strong>{{.Featured.DayNumber}}</strong>
      </div>
      <h2>{{.Featured.Title}}</h2>
      <p>{{.Featured.LocationLabel}}</p>
    </a>
    <div class="feature-copy">
      <p class="section-kicker">Featured pick</p>
      <h2>{{.Featured.Summary}}</h2>
      <p class="feature-note">{{.Featured.DiscoveryNote}}</p>
      <p class="host-line">Hosted by {{.Featured.OrganizerLabel}}</p>
      <dl class="summary-list feature-facts">
        <div><dt>When</dt><dd>{{.Featured.RangeLabel}}</dd></div>
        <div><dt>Where</dt><dd>{{.Featured.LocationLabel}}</dd></div>
        <div><dt>Signals</dt><dd>{{.Featured.SaveLabel}} · {{.Featured.ShareLabel}}</dd></div>
        <div><dt>Best with</dt><dd>{{.Featured.CrowdLabel}}</dd></div>
      </dl>
      <div class="hero-actions">
        <a class="pill-link" href="{{.Featured.PublicURL}}">Open event page</a>
        <a class="pill-link alt" href="{{.CalendarURL}}">View full calendar</a>
      </div>
    </div>
  </section>
  {{end}}

  <section class="section-head" id="browse">
    <div>
      <h2>Quick picks</h2>
      <p>Events should scan like recommendations, not database rows.</p>
    </div>
  </section>

  <section class="spotlight-grid">
    {{range .Spotlights}}
    <article class="spotlight-card theme-{{.ThemeClass}}" style="{{coverStyle .}}">
      <div class="spotlight-top">
        <div class="mini-date">
          <span>{{.MonthShort}}</span>
          <strong>{{.DayNumber}}</strong>
        </div>
        <div>
          <p class="section-kicker subtle">{{.Category}} · {{.TimeBadge}}</p>
          <h3><a href="{{.PublicURL}}">{{.Title}}</a></h3>
        </div>
      </div>
      <p>{{.DiscoveryNote}}</p>
      <div class="spotlight-foot">
        <span>{{.LocationLabel}} · {{.SaveLabel}}</span>
        <a class="text-link" href="{{.PublicURL}}">Why go?</a>
      </div>
    </article>
    {{end}}
  </section>

  <section class="section-head">
    <div>
      <h2>Dial the map in</h2>
      <p>Search by keyword, then narrow by date range, category, or location.</p>
    </div>
    <a class="pill-link alt" href="{{.CalendarURL}}">Switch to calendar</a>
  </section>

  <section class="panel">
    <form method="get" action="/">
      <div class="filters">
        <label>Keyword
          <input type="search" name="q" value="{{.Filters.Query}}" placeholder="market, workshop, cleanup">
        </label>
        <label>Category
          <select name="category">
            <option value="">All categories</option>
            {{range .Categories}}
              <option value="{{.}}" {{if eq $.Filters.Category .}}selected{{end}}>{{.}}</option>
            {{end}}
          </select>
        </label>
        <label>Location
          <select name="location">
            <option value="">All locations</option>
            {{range .Locations}}
              <option value="{{.}}" {{if eq $.Filters.Location .}}selected{{end}}>{{.}}</option>
            {{end}}
          </select>
        </label>
        <label>From
          <input type="date" name="from" value="{{.Filters.From}}">
        </label>
        <label>To
          <input type="date" name="to" value="{{.Filters.To}}">
        </label>
      </div>
      <div class="nav-actions">
        <input type="submit" value="Apply filters">
        <a class="text-link" href="/">Reset</a>
      </div>
    </form>
  </section>

  <section class="section-head">
    <div>
      <h2>Browse everything upcoming</h2>
      <p>Still structured, but now with a little more attitude.</p>
    </div>
  </section>

  <section class="event-grid discovery-grid">
    {{if .Events}}
      {{range .Events}}
      <article class="event-card discovery-card theme-{{.ThemeClass}}">
        <div class="event-poster" style="{{coverStyle .}}">
          <div class="mini-date">
            <span>{{.MonthShort}}</span>
            <strong>{{.DayNumber}}</strong>
          </div>
          <div class="poster-copy">
            <span class="badge {{.Status}}">{{.StatusLabel}}</span>
            <span class="eyebrow">{{.Category}} · {{.TimeBadge}}</span>
          </div>
        </div>
        <div>
          <h3><a href="{{.PublicURL}}">{{.Title}}</a></h3>
          <p class="muted">{{.Summary}}</p>
        </div>
        <dl class="summary-list">
          <div><dt>When</dt><dd>{{.RangeLabel}}</dd></div>
          <div><dt>Where</dt><dd>{{.LocationLabel}}</dd></div>
          <div><dt>Hosted by</dt><dd>{{.OrganizerName}}</dd></div>
          <div><dt>Signals</dt><dd>{{.SaveLabel}} · {{.ShareLabel}}</dd></div>
        </dl>
        <p class="card-note">{{.DiscoveryNote}}</p>
        <a class="pill-link" href="{{.PublicURL}}">View details</a>
      </article>
      {{end}}
    {{else}}
      <div class="empty">No published or canceled upcoming events match the current filters.</div>
    {{end}}
  </section>
  {{template "frame_end" .}}
{{end}}
`

const calendarTemplate = `
{{define "calendar"}}
  {{template "frame_start" .}}
  <section class="section-head calendar-hero">
    <div>
      <p class="section-kicker">Plan the month, then pick the nights that feel alive</p>
      <h2>{{.MonthLabel}}</h2>
      <p>Use the grid for timing, but use the highlights to decide what is actually worth your attention.</p>
    </div>
    <div class="nav-actions">
      <a class="pill-link alt" href="/calendar?month={{.PrevMonth}}">Previous</a>
      <a class="pill-link alt" href="/calendar?month={{.NextMonth}}">Next</a>
    </div>
  </section>

  {{if .Highlights}}
  <section class="spotlight-grid calendar-highlights">
    {{range .Highlights}}
    <article class="spotlight-card theme-{{.ThemeClass}}" style="{{coverStyle .}}">
      <div class="spotlight-top">
        <div class="mini-date">
          <span>{{.MonthShort}}</span>
          <strong>{{.DayNumber}}</strong>
        </div>
        <div>
          <p class="section-kicker subtle">{{.Category}} · {{.TimeBadge}}</p>
          <h3><a href="{{.PublicURL}}">{{.Title}}</a></h3>
        </div>
      </div>
      <p>{{.Summary}}</p>
      <div class="spotlight-foot">
        <span>{{.LocationLabel}} · {{.SaveLabel}}</span>
        <a class="text-link" href="{{.PublicURL}}">Open event</a>
      </div>
    </article>
    {{end}}
  </section>
  {{end}}

  <section class="calendar-card">
    <div class="calendar-grid">
      <div class="calendar-head">Sun</div>
      <div class="calendar-head">Mon</div>
      <div class="calendar-head">Tue</div>
      <div class="calendar-head">Wed</div>
      <div class="calendar-head">Thu</div>
      <div class="calendar-head">Fri</div>
      <div class="calendar-head">Sat</div>
      {{range .Days}}
      <div class="calendar-day {{if not .InMonth}}muted{{end}}">
        <div class="calendar-number">{{.DayNumber}}</div>
        {{range .Events}}
          <a class="calendar-event {{.Status}}" href="{{.PublicURL}}">
            {{.Title}}
            <small>{{if .AllDay}}All day{{else}}{{.RangeLabel}}{{end}}</small>
          </a>
        {{end}}
      </div>
      {{end}}
    </div>
  </section>
  {{template "frame_end" .}}
{{end}}
`

const detailTemplate = `
{{define "detail"}}
  {{template "frame_start" .}}
  {{if .Warning}}
    <div class="notice {{if eq .Event.Status "canceled"}}warning{{end}}">{{.Warning}}</div>
  {{end}}
  <section class="detail-layout">
    <article class="hero-card detail-story theme-{{.Event.ThemeClass}}" style="{{coverStyle .Event}}">
      <div class="detail-banner">
        <div class="mini-date inverted">
          <span>{{.Event.MonthShort}}</span>
          <strong>{{.Event.DayNumber}}</strong>
        </div>
        <div>
          <div class="eyebrow">
            <span class="badge {{.Event.Status}}">{{.Event.StatusLabel}}</span>
            <span>{{.Event.Category}} · {{.Event.Atmosphere}}</span>
          </div>
          <h1 class="detail-title">{{.Event.Title}}</h1>
        </div>
      </div>
      <p class="detail-lead">{{.Event.Summary}}</p>
      <div class="pull-quote">{{.Event.DiscoveryNote}}</div>
      <div class="prose">{{.Event.Description}}</div>
      <div class="detail-meta-strip">
        <span>{{.Event.OrganizerLabel}}</span>
        <span>{{.Event.CrowdLabel}}</span>
        <span>{{.Event.SaveLabel}} · {{.Event.ShareLabel}}</span>
      </div>
      {{if .Event.Tags}}
      <div class="tag-row">
        {{range .Event.Tags}}<span class="tag">{{.}}</span>{{end}}
      </div>
      {{end}}
      <div class="hero-actions">
        {{if .Event.ExternalURL}}
          <a class="pill-link" href="{{.Event.ExternalURL}}" target="_blank" rel="noreferrer">Open official event link</a>
        {{end}}
        <a class="pill-link alt" href="/calendar">See this in the calendar</a>
        <button class="alt" type="button" data-copy="{{.Event.AbsoluteURL}}">Copy link</button>
        <a class="pill-link alt" href="{{mailtoLink .Event}}">Email this</a>
      </div>
    </article>
    <aside class="panel detail-sidebar">
      <h2>Before you send this to friends</h2>
      <dl class="meta-list">
        <div><dt>When</dt><dd>{{.Event.RangeLabel}}</dd></div>
        <div><dt>Venue</dt><dd>{{.Event.Venue}}</dd></div>
        <div><dt>Venue note</dt><dd>{{.Event.VenueNote}}</dd></div>
        <div><dt>Neighborhood</dt><dd>{{.Event.Neighborhood}}</dd></div>
        <div><dt>Location</dt><dd>{{.Event.Location}}</dd></div>
        <div><dt>Organizer</dt><dd>{{.Event.OrganizerLabel}}</dd></div>
        <div><dt>Timezone</dt><dd>{{.Event.Timezone}}</dd></div>
        <div><dt>Status</dt><dd>{{.Event.StatusLabel}}</dd></div>
        <div><dt>Best fit</dt><dd>{{.Event.Atmosphere}}</dd></div>
      </dl>
      {{if .IsAdmin}}
        <p class="muted">Stable public URL: <code>{{.Event.PublicURL}}</code></p>
      {{end}}
      <div class="notice">
        <strong>Governed object</strong><br>
        <span class="muted"><code>{{.Event.ID}}</code> · {{.Event.RevisionCount}} accepted changes · <a href="{{.Event.LedgerURL}}">Open event ledger</a></span>
      </div>
      <div class="nav-actions">
        <a class="pill-link alt" href="/">Back to directory</a>
        <a class="pill-link alt" href="/calendar">Calendar</a>
        <a class="pill-link alt" href="{{.Event.LedgerURL}}">Ledger</a>
      </div>
    </aside>
  </section>

  {{if .Related}}
  <section class="section-head">
    <div>
      <h2>Keep the night moving</h2>
      <p>If this one is close, these are the next listings worth checking.</p>
    </div>
  </section>
  <section class="spotlight-grid">
    {{range .Related}}
    <article class="spotlight-card theme-{{.ThemeClass}}" style="{{coverStyle .}}">
      <div class="spotlight-top">
        <div class="mini-date">
          <span>{{.MonthShort}}</span>
          <strong>{{.DayNumber}}</strong>
        </div>
        <div>
          <p class="section-kicker subtle">{{.Category}} · {{.TimeBadge}}</p>
          <h3><a href="{{.PublicURL}}">{{.Title}}</a></h3>
        </div>
      </div>
      <p>{{.Summary}}</p>
      <div class="spotlight-foot">
        <span>{{.LocationLabel}} · {{.SaveLabel}}</span>
        <a class="text-link" href="{{.PublicURL}}">Open event</a>
      </div>
    </article>
    {{end}}
  </section>
  {{end}}
  {{template "frame_end" .}}
{{end}}
`

const ledgerTemplate = `
{{define "ledger"}}
  {{template "frame_start" .}}
  <section class="section-head">
    <div>
      <h2>{{.Event.Title}} ledger</h2>
      <p>Stable object <code>{{.Event.ID}}</code> with {{.Event.RevisionCount}} accepted changes materialized into the current event view.</p>
    </div>
    <div class="nav-actions">
      <a class="pill-link alt" href="{{.Event.PublicURL}}">Event page</a>
      <a class="pill-link alt" href="/admin">Organizer workspace</a>
    </div>
  </section>

  <section class="panel">
    <dl class="meta-list">
      <div><dt>Object ID</dt><dd><code>{{.Event.ID}}</code></dd></div>
      <div><dt>Stable handle</dt><dd><code>{{.Event.Slug}}</code></dd></div>
      <div><dt>Current status</dt><dd>{{.Event.StatusLabel}}</dd></div>
      <div><dt>Current URL</dt><dd><code>{{.Event.PublicURL}}</code></dd></div>
    </dl>
  </section>

  <section class="panel">
    <h3>Accepted claim history</h3>
    <div class="stack">
      {{range .Claims}}
      <article class="panel" style="margin-top:1rem;">
        <div class="eyebrow">
          <span class="badge">{{.ClaimType}}</span>
          <span>{{.AcceptedAt}} · {{.AcceptedBy}}</span>
        </div>
        <p><strong>{{.Summary}}</strong></p>
        {{if .SupersedesClaimID}}<p class="muted">Supersedes <code>{{.SupersedesClaimID}}</code></p>{{end}}
        <p class="muted">Claim ID <code>{{.ID}}</code></p>
        {{if .PayloadJSON}}<pre>{{.PayloadJSON}}</pre>{{end}}
      </article>
      {{end}}
    </div>
  </section>
  {{template "frame_end" .}}
{{end}}
`

const adminLoginTemplate = `
{{define "admin_login"}}
  {{template "frame_start" .}}
  <section class="login-card panel" style="max-width:520px;margin:40px auto;">
    <h2>Organizer login</h2>
    <p class="muted">Use the local development password to manage event drafts and public state transitions.</p>
    {{if .Error}}<div class="notice warning">{{.Error}}</div>{{end}}
    <form method="post" action="/admin/login">
      <label>Password
        <input type="password" name="password" autocomplete="current-password">
      </label>
      <input type="submit" value="Sign in">
    </form>
  </section>
  {{template "frame_end" .}}
{{end}}
`

const adminDashboardTemplate = `
{{define "admin_dashboard"}}
  {{template "frame_start" .}}
  {{if .Flash}}<div class="notice">{{.Flash}}</div>{{end}}
  <section class="section-head">
    <div>
      <h2>Organizer workspace</h2>
      <p>Create drafts, preserve stable URLs, then publish, unpublish, cancel, or archive without changing the slug.</p>
    </div>
    <div class="nav-actions">
      <a class="pill-link" href="/admin/events/new">Create event</a>
      <form class="inline-form" method="post" action="/admin/logout"><button class="alt" type="submit">Log out</button></form>
    </div>
  </section>

  <section class="panel">
    <table class="table">
      <thead>
        <tr>
          <th>Event</th>
          <th>Public URL</th>
          <th>Status</th>
          <th>Schedule</th>
          <th>Actions</th>
        </tr>
      </thead>
      <tbody>
        {{range .Events}}
        <tr>
          <td>
            <strong>{{.Title}}</strong><br>
            <span class="muted">{{.Category}} · {{.Location}} · {{.OrganizerName}}</span><br>
            <span class="muted"><code>{{.ID}}</code> · {{.RevisionCount}} accepted changes</span>
          </td>
          <td><a href="{{.PublicURL}}">{{.Slug}}</a></td>
          <td><span class="badge {{.Status}}">{{.StatusLabel}}</span></td>
          <td>{{.RangeLabel}}</td>
          <td class="actions">
            <a class="pill-link alt" href="/admin/events/{{.ID}}/edit">Edit</a>
            <a class="pill-link alt" href="{{.LedgerURL}}">Ledger</a>
            {{if eq .Status "draft"}}
              <form class="inline-form" method="post" action="/admin/events/{{.ID}}/state"><input type="hidden" name="action" value="publish"><button type="submit">Publish</button></form>
            {{else}}
              <form class="inline-form" method="post" action="/admin/events/{{.ID}}/state"><input type="hidden" name="action" value="unpublish"><button class="alt" type="submit">Unpublish</button></form>
            {{end}}
            {{if ne .Status "canceled"}}
              <form class="inline-form" method="post" action="/admin/events/{{.ID}}/state"><input type="hidden" name="action" value="cancel"><button class="alt" type="submit">Cancel</button></form>
            {{end}}
            {{if ne .Status "archived"}}
              <form class="inline-form" method="post" action="/admin/events/{{.ID}}/state"><input type="hidden" name="action" value="archive"><button class="alt" type="submit">Archive</button></form>
            {{end}}
          </td>
        </tr>
        {{end}}
      </tbody>
    </table>
  </section>
  {{template "frame_end" .}}
{{end}}
`

const adminFormTemplate = `
{{define "admin_form"}}
  {{template "frame_start" .}}
  <section class="section-head">
    <div>
      <h2>{{if eq .Mode "create"}}Create event draft{{else}}Edit event{{end}}</h2>
      <p>{{if eq .Mode "create"}}Drafts stay private until published.{{else}}Edits keep the same public URL.{{end}}</p>
    </div>
    <a class="pill-link alt" href="/admin">Back to workspace</a>
  </section>
  {{if .Error}}<div class="notice warning">{{.Error}}</div>{{end}}
  <section class="panel">
    <form method="post" action="{{if eq .Mode "create"}}/admin/events{{else}}/admin/events/{{.Event.ID}}{{end}}">
      {{if .Event.Slug}}
        <div class="notice">Stable public URL: <code>/events/{{.Event.Slug}}</code></div>
      {{end}}
      <div class="grid-two" style="grid-template-columns:repeat(2,minmax(0,1fr));">
        <label>Title
          <input type="text" name="title" value="{{.Event.Title}}" required>
        </label>
        <label>Category
          <input type="text" name="category" value="{{.Event.Category}}" required>
        </label>
        <label>Venue
          <input type="text" name="venue" value="{{.Event.Venue}}">
        </label>
        <label>Neighborhood
          <input type="text" name="neighborhood" value="{{.Event.Neighborhood}}" placeholder="West End, Queens Quay, Downtown">
        </label>
        <label>Location
          <input type="text" name="location" value="{{.Event.Location}}" required>
        </label>
        <label>Timezone
          <input type="text" name="timezone" value="{{.Event.Timezone}}" required>
        </label>
        <label>External URL
          <input type="url" name="external_url" value="{{.Event.ExternalURL}}" placeholder="https://example.com">
        </label>
      </div>
      <div class="grid-two" style="grid-template-columns:repeat(2,minmax(0,1fr));">
        <label>Organizer name
          <input type="text" name="organizer_name" value="{{.Event.OrganizerName}}" required>
        </label>
        <label>Organizer role
          <input type="text" name="organizer_role" value="{{.Event.OrganizerRole}}" placeholder="Independent organizer, community nonprofit">
        </label>
        <label>Organizer URL
          <input type="url" name="organizer_url" value="{{.Event.OrganizerURL}}" placeholder="https://example.com/organizer">
        </label>
        <label>Cover image URL
          <input type="url" name="cover_image_url" value="{{.Event.CoverImageURL}}" placeholder="https://images.example.com/event.jpg">
        </label>
      </div>
      <label>Summary
        <input type="text" name="summary" value="{{.Event.Summary}}" required>
      </label>
      <label>Featured blurb
        <input type="text" name="featured_blurb" value="{{.Event.FeaturedBlurb}}" placeholder="Why this is worth going to or sharing">
      </label>
      <label>Description
        <textarea name="description" required>{{.Event.Description}}</textarea>
      </label>
      <label>Venue note
        <textarea name="venue_note" placeholder="Arrival details, room feel, what the venue is like">{{.Event.VenueNote}}</textarea>
      </label>
      <div class="grid-two" style="grid-template-columns:repeat(2,minmax(0,1fr));">
        <label>Tags
          <input type="text" name="tags" value="{{.Event.Tags}}" placeholder="music, free, outdoors, family-friendly">
        </label>
        <label>Crowd label
          <input type="text" name="crowd_label" value="{{.Event.CrowdLabel}}" placeholder="Best with 2-4 friends">
        </label>
      </div>
      <label class="checkbox">
        <input type="checkbox" name="all_day" {{if .Event.AllDay}}checked{{end}}>
        <span>All-day event</span>
      </label>
      <div class="grid-two" style="grid-template-columns:repeat(2,minmax(0,1fr));">
        <label>Start date
          <input type="date" name="start_date" value="{{.Event.StartDate}}" required>
        </label>
        <label>End date
          <input type="date" name="end_date" value="{{.Event.EndDate}}" required>
        </label>
        <label>Start time
          <input type="time" name="start_time" value="{{.Event.StartTime}}">
        </label>
        <label>End time
          <input type="time" name="end_time" value="{{.Event.EndTime}}">
        </label>
      </div>
      <div class="nav-actions">
        <input type="submit" value="{{if eq .Mode "create"}}Save draft{{else}}Update event{{end}}">
        <a class="text-link" href="/admin">Cancel</a>
      </div>
    </form>
  </section>
  {{template "frame_end" .}}
{{end}}
`
