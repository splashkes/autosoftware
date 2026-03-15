package main

import (
	"path/filepath"
	"testing"
	"time"
)

func newTestStore(t *testing.T) *eventStore {
	t.Helper()
	return newEventStore(filepath.Join(t.TempDir(), "events.json"))
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

func TestStorePersistsCreatedEvent(t *testing.T) {
	dataFile := filepath.Join(t.TempDir(), "events.json")
	store := newEventStore(dataFile)
	created, err := store.create(eventInput{
		Title:         "Persistence Check",
		Summary:       "Verifies disk-backed storage.",
		Description:   "Created to prove the event store survives process restarts.",
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

	reloaded := newEventStore(dataFile)
	found, ok := reloaded.byID(created.ID)
	if !ok {
		t.Fatal("expected created event to persist to disk")
	}
	if found.Title != "Persistence Check" {
		t.Fatalf("unexpected persisted title: %q", found.Title)
	}
}
