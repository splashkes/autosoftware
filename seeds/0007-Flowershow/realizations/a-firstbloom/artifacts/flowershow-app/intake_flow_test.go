package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestIntakePhotosPageRenders verifies the GET handler returns 200 and
// includes the seeded class title on the sticky bar.
func TestIntakePhotosPageRenders(t *testing.T) {
	a := testApp()
	req := httptest.NewRequest("GET", "/admin/shows/show_spring2025/intake/photos", nil)
	addAdminSession(t, a, req)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /admin/shows/{showID}/intake/photos", a.requireCapabilityPage("entries.manage", a.handleIntakeSequentialPhotos))

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	// First seeded class is "One Hybrid Tea Bloom" (class_01).
	if !strings.Contains(body, "One Hybrid Tea Bloom") {
		t.Fatalf("expected first class title in intake page, body=%s", body)
	}
	if !strings.Contains(body, "data-intake-shell") {
		t.Fatal("expected intake shell marker in rendered page")
	}
	if !strings.Contains(body, "Add photo to next entry") {
		t.Fatal("expected primary capture button copy")
	}
	if !strings.Contains(body, "/admin/shows/show_spring2025/intake/photos/anon-entry") {
		t.Fatal("expected anon-entry url to be embedded in intake shell")
	}
}

// TestIntakePhotosPageHonoursClassQueryParam verifies ?class=... selects the
// correct class on the sticky bar.
func TestIntakePhotosPageHonoursClassQueryParam(t *testing.T) {
	a := testApp()
	req := httptest.NewRequest("GET", "/admin/shows/show_spring2025/intake/photos?class=class_03", nil)
	addAdminSession(t, a, req)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /admin/shows/{showID}/intake/photos", a.requireCapabilityPage("entries.manage", a.handleIntakeSequentialPhotos))

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "One Floribunda Spray") {
		t.Fatalf("expected class_03 title, body=%s", body)
	}
}

// TestIntakeAnonymousEntryCreation verifies the POST endpoint creates an
// anonymous entry with empty PersonID and returns the expected JSON shape.
func TestIntakeAnonymousEntryCreation(t *testing.T) {
	a := testApp()
	before := len(a.store.entriesByShow("show_spring2025"))

	req := httptest.NewRequest("POST", "/admin/shows/show_spring2025/intake/photos/anon-entry?class=class_01", nil)
	addAdminSession(t, a, req)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /admin/shows/{showID}/intake/photos/anon-entry", a.requireCapabilityPage("entries.manage", a.handleIntakeNewAnonymousEntry))

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", w.Code, w.Body.String())
	}

	var resp intakeAnonEntryResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.EntryID == "" {
		t.Fatal("expected entry_id in response")
	}
	if resp.ClassID != "class_01" {
		t.Fatalf("expected class_id=class_01, got %q", resp.ClassID)
	}
	if resp.UploadURL != "/admin/entries/"+resp.EntryID+"/media" {
		t.Fatalf("unexpected upload_url %q", resp.UploadURL)
	}
	if !strings.HasPrefix(resp.EntryLabel, "Anonymous") {
		t.Fatalf("expected anonymous label, got %q", resp.EntryLabel)
	}

	created, ok := a.store.entryByID(resp.EntryID)
	if !ok {
		t.Fatal("created entry not retrievable")
	}
	if strings.TrimSpace(created.PersonID) != "" {
		t.Fatalf("expected empty person id, got %q", created.PersonID)
	}
	if strings.TrimSpace(created.Name) != "" {
		t.Fatalf("expected empty entry name, got %q", created.Name)
	}
	if created.ClassID != "class_01" {
		t.Fatalf("entry attached to wrong class: %q", created.ClassID)
	}
	if created.ShowID != "show_spring2025" {
		t.Fatalf("entry attached to wrong show: %q", created.ShowID)
	}

	if got := len(a.store.entriesByShow("show_spring2025")); got != before+1 {
		t.Fatalf("expected one new entry, before=%d after=%d", before, got)
	}
}

// TestIntakeAnonymousEntryRejectsUnknownClass verifies the handler refuses
// requests for classes that do not exist.
func TestIntakeAnonymousEntryRejectsUnknownClass(t *testing.T) {
	a := testApp()
	req := httptest.NewRequest("POST", "/admin/shows/show_spring2025/intake/photos/anon-entry?class=class_does_not_exist", nil)
	addAdminSession(t, a, req)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /admin/shows/{showID}/intake/photos/anon-entry", a.requireCapabilityPage("entries.manage", a.handleIntakeNewAnonymousEntry))

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for unknown class, got %d", w.Code)
	}
}

// TestEntryDetailGalleryRendersAllMedia attaches three media items to an
// entry, signs in as admin, fetches the public entry detail page, and asserts
// that all three thumbnails plus the delete and star controls are present.
func TestEntryDetailGalleryRendersAllMedia(t *testing.T) {
	a := testApp()
	now := time.Now().UTC()
	for i, name := range []string{"alpha.jpg", "beta.jpg", "gamma.jpg"} {
		_, err := a.store.attachMedia(Media{
			EntryID:   "entry_01",
			MediaType: "photo",
			URL:       "/media/test_" + name,
			FileName:  name,
			IsCover:   i == 0,
			CreatedAt: now.Add(time.Duration(i) * time.Second),
		})
		if err != nil {
			t.Fatalf("attach media: %v", err)
		}
	}

	req := httptest.NewRequest("GET", "/entries/entry_01", nil)
	addAdminSession(t, a, req)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /entries/{entryID}", a.handleEntryDetail)

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, `data-entry-photo-gallery`) {
		t.Fatal("expected admin gallery section markers")
	}
	if c := strings.Count(body, `data-entry-photo-tile`); c < 3 {
		t.Fatalf("expected at least 3 photo tiles, got %d", c)
	}
	if c := strings.Count(body, `data-entry-photo-delete`); c < 3 {
		t.Fatalf("expected at least 3 delete buttons, got %d", c)
	}
	if c := strings.Count(body, `data-entry-photo-cover`); c < 3 {
		t.Fatalf("expected at least 3 cover buttons, got %d", c)
	}
	if !strings.Contains(body, `entry-photo-tile-cover`) {
		t.Fatal("expected the cover tile to be styled with cover class")
	}
	if !strings.Contains(body, `entry-photo-tile-star-active`) {
		t.Fatal("expected the cover star to be active")
	}
	if !strings.Contains(body, `?thumb=1`) {
		t.Fatal("expected thumbnail query param on photo tiles")
	}
}

// TestIntakePhotosPageAcceptsScopedIntakeOperator verifies that the intake
// page is reachable for show_intake_operator badges (same role helper-share
// links assume), not just full admins.
func TestIntakePhotosPageAcceptsScopedIntakeOperator(t *testing.T) {
	a := testApp()
	req := httptest.NewRequest("GET", "/admin/shows/show_spring2025/intake/photos", nil)
	addRoleSession(t, a, req, UserRoleInput{
		SubjectID:  "sub_intake_helper",
		CognitoSub: "sub_intake_helper",
		ShowID:     "show_spring2025",
		Role:       "show_intake_operator",
	}, UserIdentity{
		SubjectID:  "sub_intake_helper",
		CognitoSub: "sub_intake_helper",
		Email:      "helper@example.com",
	})

	mux := http.NewServeMux()
	mux.HandleFunc("GET /admin/shows/{showID}/intake/photos", a.requireCapabilityPage("entries.manage", a.handleIntakeSequentialPhotos))

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected scoped helper to render intake page, got %d", w.Code)
	}
}

// TestIntakePhotosJSAssetIsServed verifies the new intake JS asset is
// embedded and reachable via the /assets/ static handler.
func TestIntakePhotosJSAssetIsServed(t *testing.T) {
	a := testApp()
	req := httptest.NewRequest("GET", "/assets/js/intake-photos.js", nil)
	w := httptest.NewRecorder()
	a.assetHandler().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "Sequential photo intake") {
		t.Fatal("expected intake JS content")
	}
}

// TestEntryDetailGalleryHiddenForAnonymousVisitors makes sure the admin
// gallery is not exposed to logged-out users browsing the public entry page.
func TestEntryDetailGalleryHiddenForAnonymousVisitors(t *testing.T) {
	a := testApp()
	if _, err := a.store.attachMedia(Media{
		EntryID:   "entry_01",
		MediaType: "photo",
		URL:       "/media/test.jpg",
		FileName:  "test.jpg",
		CreatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("attach media: %v", err)
	}

	req := httptest.NewRequest("GET", "/entries/entry_01", nil)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /entries/{entryID}", a.handleEntryDetail)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if strings.Contains(body, "data-entry-photo-delete") {
		t.Fatal("admin delete buttons must not render for anonymous visitors")
	}
}
