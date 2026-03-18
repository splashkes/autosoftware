package main

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func testApp() *app {
	dir, err := os.MkdirTemp("", "flowershow-media-test-*")
	if err != nil {
		panic(err)
	}
	return &app{
		store:           newMemoryStore(),
		templates:       parseTemplates(),
		adminPassword:   "admin",
		serviceToken:    "test-token",
		sseBroker:       newSSEBroker(),
		media:           &localMediaStore{dir: dir},
		sessionSecret:   []byte("test-secret"),
		bootstrapAdmins: map[string]bool{},
	}
}

func TestHealthEndpoint(t *testing.T) {
	a := testApp()
	req := httptest.NewRequest("GET", "/healthz", nil)
	w := httptest.NewRecorder()
	a.handleHealth(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var body map[string]any
	json.Unmarshal(w.Body.Bytes(), &body)
	if body["status"] != "ok" {
		t.Fatalf("expected ok, got %v", body["status"])
	}
	if body["seed"] != "0007-Flowershow" {
		t.Fatalf("expected 0007-Flowershow, got %v", body["seed"])
	}
}

func TestHomePageLoads(t *testing.T) {
	a := testApp()
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	a.handleHome(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Flowershow") {
		t.Fatal("home page missing title")
	}
	if !strings.Contains(body, "Spring Rose Show") {
		t.Fatal("home page missing seeded show")
	}
}

func TestShowDetailBySlug(t *testing.T) {
	a := testApp()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /shows/{slug}", a.handleShowDetail)

	req := httptest.NewRequest("GET", "/shows/spring-rose-show-2025", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Spring Rose Show 2025") {
		t.Fatal("show detail missing name")
	}
	if !strings.Contains(body, "Horticulture Specimens") {
		t.Fatal("show detail missing division")
	}
}

func TestShowDetailNotFound(t *testing.T) {
	a := testApp()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /shows/{slug}", a.handleShowDetail)

	req := httptest.NewRequest("GET", "/shows/nonexistent", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestClassBrowse(t *testing.T) {
	a := testApp()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /shows/{slug}/classes", a.handleClassBrowse)

	req := httptest.NewRequest("GET", "/shows/spring-rose-show-2025/classes", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Hybrid Tea") {
		t.Fatal("class browse missing section")
	}
}

func TestEntryDetail(t *testing.T) {
	a := testApp()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /entries/{entryID}", a.handleEntryDetail)

	req := httptest.NewRequest("GET", "/entries/entry_01", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Peace") {
		t.Fatal("entry detail missing name")
	}
	// Privacy check: should show initials, not full name in public view
	if !strings.Contains(body, "MC") {
		t.Fatal("entry detail missing initials")
	}
}

func TestTaxonomyBrowse(t *testing.T) {
	a := testApp()
	req := httptest.NewRequest("GET", "/taxonomy", nil)
	w := httptest.NewRecorder()
	a.handleTaxonomyBrowse(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Rose") {
		t.Fatal("taxonomy missing Rose")
	}
}

func TestLeaderboard(t *testing.T) {
	a := testApp()
	req := httptest.NewRequest("GET", "/leaderboard?org=org_demo1&season=2025", nil)
	w := httptest.NewRecorder()
	a.handleLeaderboard(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Leaderboard") {
		t.Fatal("leaderboard page missing title")
	}
}

func TestAdminLoginFlow(t *testing.T) {
	a := testApp()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /admin/login", a.handleAdminLogin)
	mux.HandleFunc("POST /admin/login", a.handleAdminLoginPost)
	mux.HandleFunc("GET /admin", a.requireAdmin(a.handleAdminDashboard))

	// GET login page
	req := httptest.NewRequest("GET", "/admin/login", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	// POST wrong password
	req = httptest.NewRequest("POST", "/admin/login", strings.NewReader("password=wrong"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 (re-render with error), got %d", w.Code)
	}

	// POST correct password
	req = httptest.NewRequest("POST", "/admin/login", strings.NewReader("password=admin"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", w.Code)
	}

	// Access admin with cookie
	req = httptest.NewRequest("GET", "/admin", nil)
	req.AddCookie(&http.Cookie{Name: adminCookieName, Value: "ok"})
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestAdminRequiresAuth(t *testing.T) {
	a := testApp()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /admin", a.requireAdmin(a.handleAdminDashboard))

	req := httptest.NewRequest("GET", "/admin", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 redirect, got %d", w.Code)
	}
}

func TestAdminAllowsCognitoSessionWithAdminRole(t *testing.T) {
	a := testApp()
	_, err := a.store.assignUserRole(UserRoleInput{
		CognitoSub: "sub_admin",
		Role:       "admin",
	})
	if err != nil {
		t.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /admin", a.requireAdmin(a.handleAdminDashboard))

	req := httptest.NewRequest("GET", "/admin", nil)
	w := httptest.NewRecorder()
	if err := a.setUserSession(w, UserIdentity{CognitoSub: "sub_admin", Email: "admin@example.com"}); err != nil {
		t.Fatal(err)
	}
	for _, cookie := range w.Result().Cookies() {
		req.AddCookie(cookie)
	}

	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestAPIShowsDirectory(t *testing.T) {
	a := testApp()
	req := httptest.NewRequest("GET", "/v1/projections/0007-Flowershow/shows", nil)
	w := httptest.NewRecorder()
	a.handleAPIShowsDirectory(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var shows []*Show
	json.Unmarshal(w.Body.Bytes(), &shows)
	if len(shows) == 0 {
		t.Fatal("expected seeded shows")
	}
}

func TestCommandEndpointsRequireAuth(t *testing.T) {
	a := testApp()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/commands/0007-Flowershow/shows.create", a.handleAPICommand)

	req := httptest.NewRequest("POST", "/v1/commands/0007-Flowershow/shows.create",
		strings.NewReader(`{"name":"Test"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestCommandEndpointsAcceptServiceToken(t *testing.T) {
	a := testApp()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/commands/0007-Flowershow/shows.create", a.handleAPICommand)

	req := httptest.NewRequest("POST", "/v1/commands/0007-Flowershow/shows.create",
		strings.NewReader(`{"name":"Token Show","organization_id":"org_demo1","season":"2025"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		body, _ := io.ReadAll(w.Body)
		t.Fatalf("expected 201, got %d: %s", w.Code, body)
	}
}

func TestProjectionsReturnJSON(t *testing.T) {
	a := testApp()

	tests := []struct {
		name    string
		handler http.HandlerFunc
		path    string
	}{
		{"shows", a.handleAPIShowsDirectory, "/v1/projections/0007-Flowershow/shows"},
		{"taxonomy", a.handleAPITaxonomy, "/v1/projections/0007-Flowershow/taxonomy"},
		{"leaderboard", a.handleAPILeaderboard, "/v1/projections/0007-Flowershow/leaderboard?org_id=org_demo1&season=2025"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			w := httptest.NewRecorder()
			tt.handler(w, req)
			if w.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d", w.Code)
			}
			ct := w.Header().Get("Content-Type")
			if !strings.HasPrefix(ct, "application/json") {
				t.Fatalf("expected JSON content-type, got %s", ct)
			}
		})
	}
}

func TestLedgerProjection(t *testing.T) {
	a := testApp()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/projections/0007-Flowershow/ledger/{objectID}", a.handleAPILedger)

	// Without auth
	req := httptest.NewRequest("GET", "/v1/projections/0007-Flowershow/ledger/show_spring2025", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}

	// With service token
	req = httptest.NewRequest("GET", "/v1/projections/0007-Flowershow/ledger/show_spring2025", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestFullAPIFlow(t *testing.T) {
	a := testApp()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/commands/0007-Flowershow/persons.create", a.handleAPICommand)
	mux.HandleFunc("POST /v1/commands/0007-Flowershow/entries.create", a.handleAPICommand)
	mux.HandleFunc("POST /v1/commands/0007-Flowershow/entries.set_placement", a.handleAPICommand)

	auth := "Bearer test-token"

	// Create person
	req := httptest.NewRequest("POST", "/v1/commands/0007-Flowershow/persons.create",
		strings.NewReader(`{"first_name":"Alice","last_name":"Brown"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", auth)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create person: expected 201, got %d", w.Code)
	}
	var person Person
	json.Unmarshal(w.Body.Bytes(), &person)

	// Create entry
	body := `{"show_id":"show_spring2025","class_id":"class_01","person_id":"` + person.ID + `","name":"Test Rose"}`
	req = httptest.NewRequest("POST", "/v1/commands/0007-Flowershow/entries.create",
		strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", auth)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create entry: expected 201, got %d", w.Code)
	}
	var entry Entry
	json.Unmarshal(w.Body.Bytes(), &entry)

	// Set placement
	body = `{"id":"` + entry.ID + `","placement":1,"points":6}`
	req = httptest.NewRequest("POST", "/v1/commands/0007-Flowershow/entries.set_placement",
		strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", auth)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("set placement: expected 200, got %d", w.Code)
	}
}

func TestStoreMemoryBasics(t *testing.T) {
	s := newMemoryStore()

	// Shows exist from seed
	shows := s.allShows()
	if len(shows) == 0 {
		t.Fatal("expected seeded shows")
	}

	// Persons exist from seed
	persons := s.allPersons()
	if len(persons) != 3 {
		t.Fatalf("expected 3 persons, got %d", len(persons))
	}

	// Create new show
	show, err := s.createShow(ShowInput{
		Name:           "Test Show",
		OrganizationID: "org_demo1",
		Season:         "2025",
	})
	if err != nil {
		t.Fatal(err)
	}
	if show.Slug != "test-show" {
		t.Fatalf("expected slug test-show, got %s", show.Slug)
	}

	// Create entry
	entry, err := s.createEntry(EntryInput{
		ShowID:   show.ID,
		ClassID:  "class_01",
		PersonID: "person_01",
		Name:     "Test Entry",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Set placement
	if err := s.setPlacement(entry.ID, 1, 6); err != nil {
		t.Fatal(err)
	}
	e, ok := s.entryByID(entry.ID)
	if !ok || e.Placement != 1 || e.Points != 6 {
		t.Fatal("placement not set correctly")
	}

	// Ledger
	claims, err := s.ledgerByObjectID(show.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(claims) == 0 {
		t.Fatal("expected ledger claims for show")
	}

	// Leaderboard
	lb := s.leaderboard("org_demo1", "2025")
	if len(lb) == 0 {
		t.Fatal("expected leaderboard entries")
	}

	// Award compute
	results, err := s.computeAward("award_hp")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Fatal("expected award results")
	}
}

func TestMediaUploadAndRender(t *testing.T) {
	a := testApp()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /admin/entries/{entryID}/media", a.requireAdmin(a.handleMediaUpload))
	mux.HandleFunc("GET /entries/{entryID}", a.handleEntryDetail)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("media", "rose.jpg")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write([]byte("fake image bytes")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("POST", "/admin/entries/entry_01/media", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.AddCookie(&http.Cookie{Name: adminCookieName, Value: "ok"})
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", w.Code)
	}

	media := a.store.mediaByEntry("entry_01")
	if len(media) != 1 {
		t.Fatalf("expected 1 media record, got %d", len(media))
	}
	if _, err := os.Stat(media[0].StorageKey); err != nil {
		t.Fatalf("expected uploaded media on disk: %v", err)
	}

	req = httptest.NewRequest("GET", "/entries/entry_01", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "/media/"+media[0].ID) {
		t.Fatal("entry detail missing uploaded media")
	}
}

func TestEffectiveRulesForClass(t *testing.T) {
	s := newMemoryStore()

	// Create standard and edition
	doc, _ := s.createStandardDocument(StandardDocument{Name: "OJES", DomainScope: "horticulture"})
	ed, _ := s.createStandardEdition(StandardEdition{
		StandardDocumentID: doc.ID,
		EditionLabel:       "2019",
		PublicationYear:    2019,
		Status:             "current",
	})

	// Create rule
	rule, _ := s.createStandardRule(StandardRule{
		StandardEditionID: ed.ID,
		Domain:            "horticulture",
		RuleType:          "presentation",
		SubjectLabel:      "Hybrid Tea Display",
		Body:              "Must be displayed in a clear container",
	})

	// Create override
	s.createClassRuleOverride(ClassRuleOverride{
		ShowClassID:        "class_01",
		BaseStandardRuleID: rule.ID,
		OverrideType:       "narrow",
		Body:               "Container must be exactly 6 inches",
		Rationale:          "Local venue constraint",
	})

	// Create local-only override
	s.createClassRuleOverride(ClassRuleOverride{
		ShowClassID:  "class_01",
		OverrideType: "local_only",
		Body:         "Must include exhibitor tag",
	})

	effective := s.effectiveRulesForClass("class_01", ed.ID)
	if len(effective) != 2 {
		t.Fatalf("expected 2 effective rules, got %d", len(effective))
	}

	// Check we have both override and local_only sources
	sources := map[string]bool{}
	for _, r := range effective {
		sources[r.Source] = true
	}
	if !sources["override"] {
		t.Fatal("missing override rule")
	}
	if !sources["local_only"] {
		t.Fatal("missing local_only rule")
	}
}

func TestScorecardAndPlacement(t *testing.T) {
	s := newMemoryStore()

	// Create rubric with criteria
	rubric, _ := s.createRubric(JudgingRubric{Domain: "horticulture", Title: "Test Rubric"})
	crit1, _ := s.createCriterion(JudgingCriterion{JudgingRubricID: rubric.ID, Name: "Form", MaxPoints: 50, SortOrder: 1})
	crit2, _ := s.createCriterion(JudgingCriterion{JudgingRubricID: rubric.ID, Name: "Color", MaxPoints: 50, SortOrder: 2})

	// Submit scorecards for entry_01 and entry_02
	s.submitScorecard(EntryScorecard{
		EntryID:  "entry_01",
		JudgeID:  "person_03",
		RubricID: rubric.ID,
	}, []EntryCriterionScore{
		{CriterionID: crit1.ID, Score: 45},
		{CriterionID: crit2.ID, Score: 40},
	})

	s.submitScorecard(EntryScorecard{
		EntryID:  "entry_02",
		JudgeID:  "person_03",
		RubricID: rubric.ID,
	}, []EntryCriterionScore{
		{CriterionID: crit1.ID, Score: 35},
		{CriterionID: crit2.ID, Score: 30},
	})

	// Verify scorecards
	scs := s.scorecardsByEntry("entry_01")
	if len(scs) == 0 {
		t.Fatal("expected scorecard")
	}
	if scs[0].TotalScore != 85 {
		t.Fatalf("expected total 85, got %f", scs[0].TotalScore)
	}

	// Compute placements
	s.computePlacementsFromScores("class_01")

	e1, _ := s.entryByID("entry_01")
	e2, _ := s.entryByID("entry_02")
	if e1.Placement != 1 {
		t.Fatalf("expected entry_01 1st, got %d", e1.Placement)
	}
	if e2.Placement != 2 {
		t.Fatalf("expected entry_02 2nd, got %d", e2.Placement)
	}
}
