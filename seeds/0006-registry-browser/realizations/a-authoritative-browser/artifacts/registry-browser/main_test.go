package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// --- Mock registry API ---

func newMockRegistry() *httptest.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /v1/registry/catalog", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"mode": "catalog_projection",
			"summary": CatalogSummary{
				Realizations: 2,
				Contracts:    2,
				Objects:      3,
				Schemas:      1,
				Commands:     4,
				Projections:  5,
			},
			"realizations": []RealizationSummary{
				{Reference: "0001-notepad/a-go-htmx", SeedID: "0001-notepad", RealizationID: "a-go-htmx", Summary: "Shared notepad", Status: "draft", SurfaceKind: "interactive", CommandCount: 2, ProjectionCount: 1, Self: "/v1/registry/realization?reference=0001-notepad%2Fa-go-htmx"},
				{Reference: "0004-events/a-web-mvp", SeedID: "0004-events", RealizationID: "a-web-mvp", Summary: "Event listings", Status: "draft", SurfaceKind: "interactive", CommandCount: 2, ProjectionCount: 4, Self: "/v1/registry/realization?reference=0004-events%2Fa-web-mvp"},
			},
			"commands":    []CommandSummary{},
			"projections": []ProjectionSummary{},
			"objects":     []ObjectSummary{},
			"schemas":     []SchemaSummary{},
			"discovery": map[string]string{
				"catalog":      "/v1/registry/catalog",
				"realizations": "/v1/registry/realizations",
				"objects":      "/v1/registry/objects",
			},
		})
	})

	mux.HandleFunc("GET /v1/registry/realizations", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		items := []RealizationSummary{
			{Reference: "0001-notepad/a-go-htmx", SeedID: "0001-notepad", RealizationID: "a-go-htmx", Summary: "Shared notepad", Status: "draft", SurfaceKind: "interactive", CommandCount: 2, ProjectionCount: 1},
			{Reference: "0004-events/a-web-mvp", SeedID: "0004-events", RealizationID: "a-web-mvp", Summary: "Event listings", Status: "draft", SurfaceKind: "interactive", CommandCount: 2, ProjectionCount: 4},
		}
		if q != "" {
			var filtered []RealizationSummary
			for _, item := range items {
				if strings.Contains(strings.ToLower(item.Summary), strings.ToLower(q)) {
					filtered = append(filtered, item)
				}
			}
			items = filtered
		}
		json.NewEncoder(w).Encode(map[string]any{"realizations": items})
	})

	mux.HandleFunc("GET /v1/registry/realization", func(w http.ResponseWriter, r *http.Request) {
		ref := r.URL.Query().Get("reference")
		if ref == "0001-notepad/a-go-htmx" {
			json.NewEncoder(w).Encode(map[string]any{
				"realization": RealizationDetail{
					Reference: "0001-notepad/a-go-htmx", SeedID: "0001-notepad", RealizationID: "a-go-htmx",
					Summary: "Shared notepad", Status: "draft", SurfaceKind: "interactive",
					AuthModes: []string{"anonymous"}, Capabilities: []string{"search_documents"},
					Objects:     []ResourceLink{{Kind: "shared_note"}},
					Commands:    []ResourceLink{{Name: "notes.create"}, {Name: "notes.update"}},
					Projections: []ResourceLink{{Name: "notes.room"}},
					Self:        "/v1/registry/realization?reference=0001-notepad%2Fa-go-htmx",
				},
			})
			return
		}
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
	})

	mux.HandleFunc("GET /v1/registry/commands", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"commands": []CommandSummary{
				{Reference: "0001-notepad/a-go-htmx", SeedID: "0001-notepad", RealizationID: "a-go-htmx", Name: "notes.create", Path: "/v1/commands/0001-notepad/notes.create"},
				{Reference: "0001-notepad/a-go-htmx", SeedID: "0001-notepad", RealizationID: "a-go-htmx", Name: "notes.update", Path: "/v1/commands/0001-notepad/notes.update"},
			},
		})
	})

	mux.HandleFunc("GET /v1/registry/command", func(w http.ResponseWriter, r *http.Request) {
		ref := r.URL.Query().Get("reference")
		name := r.URL.Query().Get("name")
		if ref == "0001-notepad/a-go-htmx" && name == "notes.create" {
			json.NewEncoder(w).Encode(map[string]any{
				"command": CommandDetail{
					Reference: ref, SeedID: "0001-notepad", RealizationID: "a-go-htmx",
					Name: "notes.create", Summary: "Create a note", Path: "/v1/commands/0001-notepad/notes.create",
					AuthModes: []string{"anonymous"}, Idempotency: "required", Consistency: "read_your_writes",
					Self: "/v1/registry/command?reference=0001-notepad%2Fa-go-htmx&name=notes.create",
				},
			})
			return
		}
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
	})

	mux.HandleFunc("GET /v1/registry/projections", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"projections": []ProjectionSummary{
				{Reference: "0001-notepad/a-go-htmx", SeedID: "0001-notepad", RealizationID: "a-go-htmx", Name: "notes.room", Path: "/v1/projections/0001-notepad/notes.room", Freshness: "materialized"},
			},
		})
	})

	mux.HandleFunc("GET /v1/registry/projection", func(w http.ResponseWriter, r *http.Request) {
		ref := r.URL.Query().Get("reference")
		name := r.URL.Query().Get("name")
		if ref == "0001-notepad/a-go-htmx" && name == "notes.room" {
			json.NewEncoder(w).Encode(map[string]any{
				"projection": ProjectionDetail{
					Reference: ref, SeedID: "0001-notepad", RealizationID: "a-go-htmx",
					Name: "notes.room", Summary: "Note room view", Path: "/v1/projections/0001-notepad/notes.room",
					Freshness: "materialized",
					Self:      "/v1/registry/projection?reference=0001-notepad%2Fa-go-htmx&name=notes.room",
				},
			})
			return
		}
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
	})

	mux.HandleFunc("GET /v1/registry/objects", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"objects": []ObjectSummary{
				{SeedID: "0001-notepad", Kind: "shared_note", Summary: "A shared note", RealizationCount: 1, CommandCount: 2, ProjectionCount: 1, Self: "/v1/registry/object?seed_id=0001-notepad&kind=shared_note"},
				{SeedID: "0004-events", Kind: "event", Summary: "An event", RealizationCount: 1, CommandCount: 2, ProjectionCount: 3, Self: "/v1/registry/object?seed_id=0004-events&kind=event"},
			},
		})
	})

	mux.HandleFunc("GET /v1/registry/object", func(w http.ResponseWriter, r *http.Request) {
		seedID := r.URL.Query().Get("seed_id")
		kind := r.URL.Query().Get("kind")
		if seedID == "0001-notepad" && kind == "shared_note" {
			json.NewEncoder(w).Encode(map[string]any{
				"object": ObjectDetail{
					SeedID: seedID, Kind: kind, Summary: "A shared note",
					Capabilities: []string{"search_documents"},
					Schemas:      []ResourceLink{{Ref: "seeds/0001-notepad/design.md"}},
					Realizations: []ObjectRealization{
						{Reference: "0001-notepad/a-go-htmx", SeedID: "0001-notepad", RealizationID: "a-go-htmx", Summary: "Shared notepad", Status: "draft", SurfaceKind: "interactive"},
					},
					Self: "/v1/registry/object?seed_id=0001-notepad&kind=shared_note",
				},
			})
			return
		}
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
	})

	mux.HandleFunc("GET /v1/registry/schemas", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"schemas": []SchemaSummary{
				{Ref: "seeds/0001-notepad/design.md", Path: "seeds/0001-notepad/design.md", ObjectUseCount: 1, CommandInputCount: 0, CommandResultCount: 0, Self: "/v1/registry/schema?ref=seeds%2F0001-notepad%2Fdesign.md"},
			},
		})
	})

	mux.HandleFunc("GET /v1/registry/schema", func(w http.ResponseWriter, r *http.Request) {
		ref := r.URL.Query().Get("ref")
		if ref == "seeds/0001-notepad/design.md" {
			json.NewEncoder(w).Encode(map[string]any{
				"schema": SchemaDetail{
					Ref: ref, Path: ref,
					ObjectUses: []SchemaObjectUse{
						{Reference: "0001-notepad/a-go-htmx", SeedID: "0001-notepad", RealizationID: "a-go-htmx", Kind: "shared_note", Summary: "A shared note"},
					},
					Self: "/v1/registry/schema?ref=seeds%2F0001-notepad%2Fdesign.md",
				},
			})
			return
		}
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
	})

	return httptest.NewServer(mux)
}

func newTestApp(registryURL string) *App {
	app := &App{registry: NewRegistryClient(registryURL)}
	app.loadTemplates()
	return app
}

func newTestMux(app *App) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", app.handleHome)
	mux.HandleFunc("GET /realizations", app.handleRealizations)
	mux.HandleFunc("GET /realizations/", app.handleRealizationDetail)
	mux.HandleFunc("GET /commands", app.handleCommands)
	mux.HandleFunc("GET /commands/", app.handleCommandDetail)
	mux.HandleFunc("GET /projections", app.handleProjections)
	mux.HandleFunc("GET /projections/", app.handleProjectionDetail)
	mux.HandleFunc("GET /objects", app.handleObjects)
	mux.HandleFunc("GET /objects/", app.handleObjectDetail)
	mux.HandleFunc("GET /schemas", app.handleSchemas)
	mux.HandleFunc("GET /schemas/detail", app.handleSchemaDetail)
	mux.HandleFunc("GET /assets/style.css", app.handleStyleCSS)
	return mux
}

// --- Read-only enforcement ---

func TestNoMutationRoutes(t *testing.T) {
	mock := newMockRegistry()
	defer mock.Close()
	app := newTestApp(mock.URL)
	mux := newTestMux(app)

	methods := []string{"POST", "PUT", "DELETE", "PATCH"}
	paths := []string{
		"/", "/realizations", "/realizations/test",
		"/commands", "/commands/ref/name",
		"/projections", "/projections/ref/name",
		"/objects", "/objects/seed/kind",
		"/schemas", "/schemas/detail?ref=test",
	}

	for _, method := range methods {
		for _, path := range paths {
			req := httptest.NewRequest(method, path, nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)
			if rec.Code != http.StatusMethodNotAllowed && rec.Code != http.StatusNotFound {
				t.Errorf("%s %s returned %d, expected 405 or 404", method, path, rec.Code)
			}
		}
	}
}

// --- Home page ---

func TestHomePageRendersCountsAndAgentGuidance(t *testing.T) {
	mock := newMockRegistry()
	defer mock.Close()
	app := newTestApp(mock.URL)
	mux := newTestMux(app)

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("home returned %d", rec.Code)
	}

	body := rec.Body.String()

	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/html") {
		t.Errorf("home page content-type = %q, want text/html", ct)
	}
	if !strings.Contains(body, "Registry") {
		t.Error("home page missing expected HTML content")
	}

	// Should show counts
	if !strings.Contains(body, ">2<") {
		t.Error("home page missing realization count of 2")
	}
	if !strings.Contains(body, ">3<") {
		t.Error("home page missing object count of 3")
	}

	// Should contain agent guidance
	if !strings.Contains(body, "For Agents") {
		t.Error("home page missing agent guidance section")
	}
	if !strings.Contains(body, "GET /v1/registry/catalog") {
		t.Error("home page missing catalog API route in agent guidance")
	}
	if !strings.Contains(body, "Do not scrape") {
		t.Error("home page missing scraping warning")
	}
}

func TestHomePageRegistryDown(t *testing.T) {
	app := newTestApp("http://127.0.0.1:1") // nothing listening
	mux := newTestMux(app)

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502 when registry is down, got %d", rec.Code)
	}
}

// --- Realizations ---

func TestRealizationsListPage(t *testing.T) {
	mock := newMockRegistry()
	defer mock.Close()
	app := newTestApp(mock.URL)
	mux := newTestMux(app)

	req := httptest.NewRequest("GET", "/realizations", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("realizations returned %d", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "0001-notepad/a-go-htmx") {
		t.Error("realizations page missing notepad realization")
	}
	if !strings.Contains(body, "0004-events/a-web-mvp") {
		t.Error("realizations page missing events realization")
	}
	if !strings.Contains(body, "GET /v1/registry/realizations") {
		t.Error("realizations page missing API route badge")
	}
}

func TestRealizationsSearchFilters(t *testing.T) {
	mock := newMockRegistry()
	defer mock.Close()
	app := newTestApp(mock.URL)
	mux := newTestMux(app)

	req := httptest.NewRequest("GET", "/realizations?q=notepad", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("realizations search returned %d", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "0001-notepad/a-go-htmx") {
		t.Error("search should include matching notepad realization")
	}
	if strings.Contains(body, "0004-events/a-web-mvp") {
		t.Error("search should not include non-matching events realization")
	}
}

func TestRealizationDetailPage(t *testing.T) {
	mock := newMockRegistry()
	defer mock.Close()
	app := newTestApp(mock.URL)
	mux := newTestMux(app)

	req := httptest.NewRequest("GET", "/realizations/0001-notepad%2Fa-go-htmx", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("realization detail returned %d", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "a-go-htmx") {
		t.Error("detail page missing realization ID")
	}
	if !strings.Contains(body, "Shared notepad") {
		t.Error("detail page missing summary")
	}
	if !strings.Contains(body, "notes.create") {
		t.Error("detail page missing command link")
	}
	if !strings.Contains(body, "notes.room") {
		t.Error("detail page missing projection link")
	}
}

func TestRealizationDetailNotFound(t *testing.T) {
	mock := newMockRegistry()
	defer mock.Close()
	app := newTestApp(mock.URL)
	mux := newTestMux(app)

	req := httptest.NewRequest("GET", "/realizations/nonexistent", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for missing realization, got %d", rec.Code)
	}
}

// --- Commands ---

func TestCommandsListPage(t *testing.T) {
	mock := newMockRegistry()
	defer mock.Close()
	app := newTestApp(mock.URL)
	mux := newTestMux(app)

	req := httptest.NewRequest("GET", "/commands", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("commands returned %d", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "notes.create") {
		t.Error("commands page missing notes.create")
	}
	if !strings.Contains(body, "notes.update") {
		t.Error("commands page missing notes.update")
	}
}

func TestCommandDetailPage(t *testing.T) {
	mock := newMockRegistry()
	defer mock.Close()
	app := newTestApp(mock.URL)
	mux := newTestMux(app)

	req := httptest.NewRequest("GET", "/commands/0001-notepad%2Fa-go-htmx/notes.create", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("command detail returned %d", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "notes.create") {
		t.Error("command detail missing name")
	}
	if !strings.Contains(body, "Create a note") {
		t.Error("command detail missing summary")
	}
	if !strings.Contains(body, "required") {
		t.Error("command detail missing idempotency")
	}
}

// --- Projections ---

func TestProjectionsListPage(t *testing.T) {
	mock := newMockRegistry()
	defer mock.Close()
	app := newTestApp(mock.URL)
	mux := newTestMux(app)

	req := httptest.NewRequest("GET", "/projections", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("projections returned %d", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "notes.room") {
		t.Error("projections page missing notes.room")
	}
}

func TestProjectionDetailPage(t *testing.T) {
	mock := newMockRegistry()
	defer mock.Close()
	app := newTestApp(mock.URL)
	mux := newTestMux(app)

	req := httptest.NewRequest("GET", "/projections/0001-notepad%2Fa-go-htmx/notes.room", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("projection detail returned %d", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "notes.room") {
		t.Error("projection detail missing name")
	}
	if !strings.Contains(body, "materialized") {
		t.Error("projection detail missing freshness")
	}
}

// --- Objects ---

func TestObjectsListPage(t *testing.T) {
	mock := newMockRegistry()
	defer mock.Close()
	app := newTestApp(mock.URL)
	mux := newTestMux(app)

	req := httptest.NewRequest("GET", "/objects", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("objects returned %d", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "shared_note") {
		t.Error("objects page missing shared_note")
	}
	if !strings.Contains(body, "event") {
		t.Error("objects page missing event")
	}
	// Facet filter links
	if !strings.Contains(body, "0001-notepad") {
		t.Error("objects page missing seed facet for 0001-notepad")
	}
}

func TestObjectDetailPage(t *testing.T) {
	mock := newMockRegistry()
	defer mock.Close()
	app := newTestApp(mock.URL)
	mux := newTestMux(app)

	req := httptest.NewRequest("GET", "/objects/0001-notepad/shared_note", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("object detail returned %d", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "shared_note") {
		t.Error("object detail missing kind")
	}
	if !strings.Contains(body, "A shared note") {
		t.Error("object detail missing summary")
	}
	if !strings.Contains(body, "0001-notepad/a-go-htmx") {
		t.Error("object detail missing realization link")
	}
}

func TestObjectDetailNotFound(t *testing.T) {
	mock := newMockRegistry()
	defer mock.Close()
	app := newTestApp(mock.URL)
	mux := newTestMux(app)

	req := httptest.NewRequest("GET", "/objects/fake-seed/fake-kind", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for missing object, got %d", rec.Code)
	}
}

// --- Schemas ---

func TestSchemasListPage(t *testing.T) {
	mock := newMockRegistry()
	defer mock.Close()
	app := newTestApp(mock.URL)
	mux := newTestMux(app)

	req := httptest.NewRequest("GET", "/schemas", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("schemas returned %d", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "seeds/0001-notepad/design.md") {
		t.Error("schemas page missing design.md schema")
	}
}

func TestSchemaDetailPage(t *testing.T) {
	mock := newMockRegistry()
	defer mock.Close()
	app := newTestApp(mock.URL)
	mux := newTestMux(app)

	req := httptest.NewRequest("GET", "/schemas/detail?ref=seeds/0001-notepad/design.md", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("schema detail returned %d", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "seeds/0001-notepad/design.md") {
		t.Error("schema detail missing ref")
	}
	if !strings.Contains(body, "shared_note") {
		t.Error("schema detail missing object use")
	}
}

func TestSchemaDetailMissingRef(t *testing.T) {
	mock := newMockRegistry()
	defer mock.Close()
	app := newTestApp(mock.URL)
	mux := newTestMux(app)

	req := httptest.NewRequest("GET", "/schemas/detail", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing ref, got %d", rec.Code)
	}
}

// --- Static assets ---

func TestStyleCSSServed(t *testing.T) {
	mock := newMockRegistry()
	defer mock.Close()
	app := newTestApp(mock.URL)
	mux := newTestMux(app)

	req := httptest.NewRequest("GET", "/assets/style.css", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("style.css returned %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "text/css") {
		t.Errorf("style.css content-type = %q, want text/css", ct)
	}
	if rec.Body.Len() == 0 {
		t.Error("style.css body is empty")
	}
}

// --- Helpers ---

func TestBuildQuery(t *testing.T) {
	tests := []struct {
		pairs []string
		want  string
	}{
		{[]string{}, ""},
		{[]string{"q", ""}, ""},
		{[]string{"q", "hello"}, "?q=hello"},
		{[]string{"seed_id", "0001", "q", "test"}, "?q=test&seed_id=0001"},
		{[]string{"a", "1", "b", "", "c", "3"}, "?a=1&c=3"},
	}

	for _, tt := range tests {
		got := buildQuery(tt.pairs...)
		if got != tt.want {
			t.Errorf("buildQuery(%v) = %q, want %q", tt.pairs, got, tt.want)
		}
	}
}

func TestUniqueStrings(t *testing.T) {
	items := []ObjectSummary{
		{SeedID: "a"}, {SeedID: "b"}, {SeedID: "a"}, {SeedID: "c"}, {SeedID: "b"},
	}
	got := uniqueStrings(items, func(o ObjectSummary) string { return o.SeedID })
	if len(got) != 3 || got[0] != "a" || got[1] != "b" || got[2] != "c" {
		t.Errorf("uniqueStrings = %v, want [a b c]", got)
	}
}

// --- Content checks ---

func TestAllPagesShowAPIRoute(t *testing.T) {
	mock := newMockRegistry()
	defer mock.Close()
	app := newTestApp(mock.URL)
	mux := newTestMux(app)

	pages := []struct {
		path     string
		apiRoute string
	}{
		{"/realizations", "/v1/registry/realizations"},
		{"/commands", "/v1/registry/commands"},
		{"/projections", "/v1/registry/projections"},
		{"/objects", "/v1/registry/objects"},
		{"/schemas", "/v1/registry/schemas"},
	}

	for _, p := range pages {
		req := httptest.NewRequest("GET", p.path, nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("%s returned %d", p.path, rec.Code)
			continue
		}
		if !strings.Contains(rec.Body.String(), p.apiRoute) {
			t.Errorf("%s missing API route badge %q", p.path, p.apiRoute)
		}
	}
}

func TestFooterShowsReadOnlyNotice(t *testing.T) {
	mock := newMockRegistry()
	defer mock.Close()
	app := newTestApp(mock.URL)
	mux := newTestMux(app)

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, "Read-only") {
		t.Error("footer missing read-only notice")
	}
	if !strings.Contains(body, "authoritative API") {
		t.Error("footer missing authoritative API link")
	}
}
