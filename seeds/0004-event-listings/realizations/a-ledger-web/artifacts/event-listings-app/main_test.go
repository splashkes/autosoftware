package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func newTestStore(t *testing.T) eventStore {
	t.Helper()
	return newMemoryEventStore()
}

func newTestApp(t *testing.T) *app {
	t.Helper()
	return &app{
		store:         newTestStore(t),
		adminPassword: "password",
		serviceToken:  "test-token",
	}
}

func newTestMux(a *app) *http.ServeMux {
	mux := http.NewServeMux()
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
	return mux
}

func TestSlugStaysStableAcrossEdits(t *testing.T) {
	store := newTestStore(t)
	created, err := store.create(eventInput{
		Title:       "City Studio Tour",
		Summary:     "A public studio crawl.",
		Description: "Visit studios across the arts district.",
		Venue:       "Arts District",
		Location:    "Toronto",
		Category:    "Arts",
		Timezone:    "America/Toronto",
		StartDate:   "2026-04-01",
		EndDate:     "2026-04-01",
		StartTime:   "10:00",
		EndTime:     "17:00",
	})
	if err != nil {
		t.Fatalf("create event: %v", err)
	}

	updated, err := store.update(created.ID, eventInput{
		Title:       "City Studio Tour Revised",
		Summary:     "A public studio crawl with updated stops.",
		Description: "Visit studios across the arts district with refreshed route notes.",
		Venue:       "Arts District",
		Location:    "Toronto",
		Category:    "Arts",
		Timezone:    "America/Toronto",
		StartDate:   "2026-04-01",
		EndDate:     "2026-04-01",
		StartTime:   "11:00",
		EndTime:     "18:00",
	})
	if err != nil {
		t.Fatalf("update event: %v", err)
	}

	if updated.Slug != created.Slug {
		t.Fatalf("slug changed across edit: got %q want %q", updated.Slug, created.Slug)
	}
}

func TestDirectoryExcludesDraftAndArchived(t *testing.T) {
	a := &app{store: newTestStore(t)}
	events := a.publicDirectory(directoryFilters{})
	if len(events) == 0 {
		t.Fatal("expected seeded public events in directory")
	}

	for _, event := range events {
		if event.Status == string(statusDraft) || event.Status == string(statusArchived) {
			t.Fatalf("unexpected non-public event in directory: %s", event.Status)
		}
	}
}

func TestCalendarIncludesMultiDayEventOnCoveredDay(t *testing.T) {
	a := &app{store: newTestStore(t)}
	created, err := a.store.create(eventInput{
		Title:       "Three Day Summit",
		Summary:     "A multi-day summit.",
		Description: "Talks and workshops across three days.",
		Venue:       "Summit Hall",
		Location:    "Ottawa",
		Category:    "Conference",
		Timezone:    "America/Toronto",
		AllDay:      true,
		StartDate:   "2026-05-11",
		EndDate:     "2026-05-13",
	})
	if err != nil {
		t.Fatalf("create event: %v", err)
	}
	if _, err := a.store.setStatus(created.ID, statusPublished); err != nil {
		t.Fatalf("publish event: %v", err)
	}

	days := a.calendarDays("2026-05")
	found := false
	for _, day := range days {
		if day.Date != "2026-05-12" {
			continue
		}
		for _, event := range day.Events {
			if event.Title == "Three Day Summit" {
				found = true
			}
		}
	}

	if !found {
		t.Fatal("expected multi-day event to appear on overlapping calendar day")
	}
}

func TestOverlapsDateRange(t *testing.T) {
	start := time.Date(2026, 6, 10, 18, 0, 0, 0, loadLocationOrUTC("America/Toronto"))
	end := time.Date(2026, 6, 12, 22, 0, 0, 0, loadLocationOrUTC("America/Toronto"))
	event := &eventRecord{
		Timezone: "America/Toronto",
		Start:    start,
		End:      end,
	}

	from, _ := time.Parse("2006-01-02", "2026-06-11")
	to, _ := time.Parse("2006-01-02", "2026-06-11")
	if !overlapsDateRange(event, from, to) {
		t.Fatal("expected date filter overlap on event middle day")
	}
}

func TestLedgerRecordsSnapshotAndStatusClaims(t *testing.T) {
	store := newTestStore(t)
	created, err := store.create(eventInput{
		Title:         "Persistence Check",
		Summary:       "Verifies ledger-backed storage.",
		Description:   "Created to prove the event store emits claims for object state.",
		Venue:         "Archive Hall",
		Location:      "Toronto",
		Category:      "QA",
		OrganizerName: "Codex QA",
		Timezone:      "America/Toronto",
		StartDate:     "2026-04-02",
		EndDate:       "2026-04-02",
		StartTime:     "18:00",
		EndTime:       "19:30",
	})
	if err != nil {
		t.Fatalf("create event: %v", err)
	}

	if _, err := store.setStatus(created.ID, statusPublished); err != nil {
		t.Fatalf("publish event: %v", err)
	}
	published, ok := store.byID(created.ID)
	if !ok {
		t.Fatal("expected published event to remain addressable")
	}
	if published.RevisionCount < 3 {
		t.Fatalf("expected revision count to reflect snapshot+status history, got %d", published.RevisionCount)
	}

	claims, err := store.ledgerByID(created.ID)
	if err != nil {
		t.Fatalf("ledgerByID: %v", err)
	}
	if len(claims) < 3 {
		t.Fatalf("expected at least 3 claims for created+published event, got %d", len(claims))
	}
	if claims[0].ClaimType != "event.snapshot" {
		t.Fatalf("expected first claim to be snapshot, got %q", claims[0].ClaimType)
	}
	if claims[len(claims)-1].ClaimType != "event.status" || claims[len(claims)-1].Status != string(statusPublished) {
		t.Fatalf("expected last claim to publish the event, got %+v", claims[len(claims)-1])
	}
}

func TestWorkspaceProjectionRequiresAuth(t *testing.T) {
	a := newTestApp(t)
	mux := newTestMux(a)

	req := httptest.NewRequest(http.MethodGet, "/v1/projections/0004-event-listings/admin/events", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without auth, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/projections/0004-event-listings/admin/events", nil)
	req.AddCookie(&http.Cookie{Name: adminCookieName, Value: "ok"})
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 with admin session, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"status":"draft"`) {
		t.Fatalf("expected workspace projection to include draft events, got %s", rec.Body.String())
	}
}

func TestLedgerRecordProjectionAllowsServiceTokenForDrafts(t *testing.T) {
	a := newTestApp(t)
	mux := newTestMux(a)

	var draftID string
	for _, event := range a.store.all() {
		if event.Status == statusDraft {
			draftID = event.ID
			break
		}
	}
	if draftID == "" {
		t.Fatal("expected seeded draft event")
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/projections/0004-event-listings/events/by-id/"+draftID+"/ledger", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for anonymous draft ledger read, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/projections/0004-event-listings/events/by-id/"+draftID+"/ledger", nil)
	req.Header.Set("X-AS-Service-Token", a.serviceToken)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 with service token, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"claim_type":"event.snapshot"`) {
		t.Fatalf("expected ledger projection to return claim history, got %s", rec.Body.String())
	}
}

func TestCommandEndpointsRequireAuthAndSupportServiceTokenFlow(t *testing.T) {
	a := newTestApp(t)
	mux := newTestMux(a)

	payload := map[string]any{
		"title":          "Ledger API Contract Event",
		"summary":        "Created through the declared command surface.",
		"description":    "Used to prove alternate clients can create and publish through the same ledger-backed contract.",
		"venue":          "Ledger Hall",
		"location":       "Toronto",
		"category":       "QA",
		"organizer_name": "Contract Test",
		"timezone":       "America/Toronto",
		"start_date":     "2026-04-08",
		"end_date":       "2026-04-08",
		"start_time":     "18:00",
		"end_time":       "20:00",
	}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal create payload: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/commands/0004-event-listings/events.create", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without auth, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/v1/commands/0004-event-listings/events.create", bytes.NewReader(body))
	req.Header.Set("X-AS-Service-Token", a.serviceToken)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for authorized create, got %d body=%s", rec.Code, rec.Body.String())
	}

	var created struct {
		Event struct {
			ID string `json:"id"`
		} `json:"event"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("unmarshal create response: %v", err)
	}
	if created.Event.ID == "" {
		t.Fatal("expected created event id")
	}

	publishBody, err := json.Marshal(map[string]string{"event_id": created.Event.ID})
	if err != nil {
		t.Fatalf("marshal publish payload: %v", err)
	}
	req = httptest.NewRequest(http.MethodPost, "/v1/commands/0004-event-listings/events.publish", bytes.NewReader(publishBody))
	req.Header.Set("X-AS-Service-Token", a.serviceToken)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for publish, got %d body=%s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/projections/0004-event-listings/events/by-id/"+created.Event.ID, nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected anonymous read of published record, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestToEventViewDoesNotBakeEnvironmentHost(t *testing.T) {
	start := time.Date(2026, 4, 10, 18, 0, 0, 0, loadLocationOrUTC("America/Toronto"))
	end := start.Add(2 * time.Hour)

	view := toEventView(&eventRecord{
		ID:       "evt_test",
		Slug:     "spring-market",
		Title:    "Spring Market",
		Summary:  "A spring market.",
		Status:   statusPublished,
		Timezone: "America/Toronto",
		Start:    start,
		End:      end,
	})

	if strings.Contains(view.AbsoluteURL, "://") {
		t.Fatalf("expected share URL to stay relative, got %q", view.AbsoluteURL)
	}
	if view.AbsoluteURL != "/events/spring-market" {
		t.Fatalf("unexpected share URL: %q", view.AbsoluteURL)
	}
}
