package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// loadAdminShowPanel renders /admin/shows/{showID} for an admin session and
// returns the body. Tests use the demo-seeded show_spring2025 which already
// has classes and entries.
func loadAdminShowPanel(t *testing.T, a *app, showID string) string {
	t.Helper()
	registerSplitsRenderStore(a.store)

	req := httptest.NewRequest("GET", "/admin/shows/"+showID, nil)
	addAdminSession(t, a, req)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /admin/shows/{showID}", a.requireCapabilityPage("shows.workspace.read", a.handleAdminShowDetail))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("admin show detail returned %d: %s", w.Code, w.Body.String())
	}
	return w.Body.String()
}

func TestSplitsUIRendersSplitButton(t *testing.T) {
	a := testApp()
	body := loadAdminShowPanel(t, a, "show_spring2025")

	if !strings.Contains(body, "data-split-button") {
		t.Fatal("expected admin show panel to expose split button trigger")
	}
	if !strings.Contains(body, `data-class-id="class_01"`) {
		t.Fatal("expected per-class data-class-id wiring on intake card")
	}
	if !strings.Contains(body, "data-splittable-class=\"class_01\"") {
		t.Fatal("expected splittable wrapper around class panel")
	}
	if !strings.Contains(body, "data-split-action-bar") {
		t.Fatal("expected hidden split action bar to be present")
	}
}

func TestSplitsUIRendersExistingSplitsGrouped(t *testing.T) {
	a := testApp()
	registerSplitsRenderStore(a.store)

	split, err := a.store.createClassSplit(ClassSplitInput{
		ClassID: "class_01",
		Label:   "Yellow",
	})
	if err != nil {
		t.Fatalf("create split: %v", err)
	}
	if split == nil || split.SplitCode != "a" {
		t.Fatalf("expected first split to be code 'a', got %+v", split)
	}

	if err := a.store.moveEntryToSplit("entry_01", split.ID); err != nil {
		t.Fatalf("move entry to split: %v", err)
	}

	body := loadAdminShowPanel(t, a, "show_spring2025")

	if !strings.Contains(body, "Split a") {
		t.Fatal("expected split-a header to render")
	}
	if !strings.Contains(body, "Yellow") {
		t.Fatal("expected split label to render")
	}
	if !strings.Contains(body, `data-split-group="`+split.ID+`"`) {
		t.Fatal("expected split group container with data-split-group attr")
	}
	if !strings.Contains(body, "(unassigned)") {
		t.Fatal("expected (unassigned) bucket header when splits exist")
	}
	if !strings.Contains(body, "data-split-delete-button") {
		t.Fatal("expected per-split delete affordance")
	}
	if !strings.Contains(body, "data-split-move-select") {
		t.Fatal("expected per-entry move-to-split select when splits exist")
	}
	// Per-split judging row (C5 surface) must render once per split.
	if !strings.Contains(body, "data-split-judging-row") {
		t.Fatal("expected per-split judging row for split judging surface")
	}
}

func TestModeToggleEmitsBothViews(t *testing.T) {
	a := testApp()
	body := loadAdminShowPanel(t, a, "show_spring2025")

	if !strings.Contains(body, `data-mode="fast"`) {
		t.Fatal("expected default workspace mode to be 'fast' on body element")
	}
	if !strings.Contains(body, "data-mode-toggle-button=\"fast\"") {
		t.Fatal("expected fast-mode toggle button")
	}
	if !strings.Contains(body, "data-mode-toggle-button=\"update\"") {
		t.Fatal("expected update-mode toggle button")
	}
	if !strings.Contains(body, "data-mode-toggle-reset") {
		t.Fatal("expected reset-to-defaults link")
	}
	// Update-mode-only DOM must be present so JS can switch without re-fetching.
	if !strings.Contains(body, "data-mode-update-only") {
		t.Fatal("expected update-mode-only DOM to be inlined for instant switching")
	}
	// Per-section toggles for the persistent collapsibles.
	for _, sec := range []string{"name_match", "ranking", "comment", "photo_detail"} {
		if !strings.Contains(body, `data-entry-section-toggle="`+sec+`"`) {
			t.Fatalf("expected section-toggle button for %q", sec)
		}
	}
	// Each entry should expose collapsible <details> panels for the same sections.
	for _, sec := range []string{"name_match", "ranking", "comment", "photo_detail"} {
		if !strings.Contains(body, `data-entry-section="`+sec+`"`) {
			t.Fatalf("expected per-entry section <details> for %q", sec)
		}
	}
}

func TestSplitsFragmentEndpointReturnsSplitsForClass(t *testing.T) {
	a := testApp()
	registerSplitsRenderStore(a.store)

	split, err := a.store.createClassSplit(ClassSplitInput{
		ClassID: "class_01",
		Label:   "Double",
	})
	if err != nil {
		t.Fatalf("create split: %v", err)
	}
	if err := a.store.moveEntryToSplit("entry_01", split.ID); err != nil {
		t.Fatalf("move entry to split: %v", err)
	}

	req := httptest.NewRequest("GET", "/admin/classes/class_01/splits/fragment", nil)
	addAdminSession(t, a, req)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /admin/classes/{classID}/splits/fragment", a.requireCapabilityPage("shows.workspace.read", a.handleAdminClassSplitsFragment))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("fragment returned %d: %s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if !strings.Contains(body, "data-splittable-class=\"class_01\"") {
		t.Fatal("fragment should target class_01 wrapper")
	}
	if !strings.Contains(body, "Double") {
		t.Fatal("fragment should include the split's label")
	}
}

// TestModeToggleAssetsAreEmbedded protects against //go:embed regressions
// hiding the new JS files from the runtime asset handler.
func TestModeToggleAssetsAreEmbedded(t *testing.T) {
	a := testApp()
	for _, path := range []string{"/assets/js/admin-mode-toggle.js", "/assets/js/class-splits-ui.js"} {
		req := httptest.NewRequest("GET", path, nil)
		w := httptest.NewRecorder()
		a.assetHandler().ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected %s to be served, got %d body=%s", path, w.Code, w.Body.String())
		}
	}
}
