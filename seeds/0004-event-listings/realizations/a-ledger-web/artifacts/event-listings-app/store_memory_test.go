package main

import (
	"errors"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type memoryEventStore struct {
	mu     sync.RWMutex
	nextID int
	order  []string
	events map[string]*eventRecord
	ledger map[string][]eventClaim
}

func newMemoryEventStore() eventStore {
	s := &memoryEventStore{
		nextID: 1,
		events: make(map[string]*eventRecord),
		ledger: make(map[string][]eventClaim),
	}

	now := time.Now().UTC()
	s.mustSeed(eventInput{
		Title:         "Harbour Lights Night Market",
		Summary:       "An evening market with local food, craft stalls, and live jazz.",
		Description:   "Browse independent makers, snack at harbour-side food stands, and stay for an outdoor jazz set after sunset.",
		Venue:         "Pier Warehouse",
		VenueNote:     "Covered indoor-outdoor market hall with plenty of room to wander and snack between sets.",
		Neighborhood:  "Queens Quay",
		Location:      "Toronto",
		Category:      "Community",
		OrganizerName: "Harbour Collective",
		OrganizerRole: "Independent local organizer",
		OrganizerURL:  "https://example.com/harbour-collective",
		CoverImageURL: "https://picsum.photos/seed/harbour-night-market/1200/900",
		FeaturedBlurb: "Go for the easy social energy, the food lineups, and the kind of event that turns into a low-effort group plan.",
		Tags:          "night market, live music, food, makers",
		CrowdLabel:    "Best with 2-4 friends",
		ExternalURL:   "https://example.com/night-market",
		Timezone:      "America/Toronto",
		StartDate:     now.AddDate(0, 0, 3).Format("2006-01-02"),
		EndDate:       now.AddDate(0, 0, 3).Format("2006-01-02"),
		StartTime:     "18:30",
		EndTime:       "22:00",
	}, statusPublished)
	s.mustSeed(eventInput{
		Title:         "Design Systems Field Day",
		Summary:       "A two-day workshop on documentation, component audits, and accessibility reviews.",
		Description:   "Small-team sessions on component inventory, naming conventions, content patterns, and documentation debt remediation.",
		Venue:         "Foundry Hall",
		VenueNote:     "A bright former industrial space with breakout rooms and long shared tables.",
		Neighborhood:  "Downtown Ottawa",
		Location:      "Ottawa",
		Category:      "Workshop",
		OrganizerName: "Systems Practice",
		OrganizerRole: "Product design studio",
		OrganizerURL:  "https://example.com/systems-practice",
		CoverImageURL: "https://picsum.photos/seed/design-systems-field-day/1200/900",
		FeaturedBlurb: "This is for people who want useful notes, sharper thinking, and the right kind of post-session conversations.",
		Tags:          "design systems, ux, accessibility, workshop",
		CrowdLabel:    "Best solo or with one teammate",
		ExternalURL:   "https://example.com/design-field-day",
		Timezone:      "America/Toronto",
		StartDate:     now.AddDate(0, 0, 7).Format("2006-01-02"),
		EndDate:       now.AddDate(0, 0, 8).Format("2006-01-02"),
		AllDay:        true,
	}, statusPublished)
	s.mustSeed(eventInput{
		Title:         "Riverfront Cleanup Rally",
		Summary:       "Volunteer-led cleanup with loaner gloves, coffee, and route captains.",
		Description:   "Meet at the south gate, collect supplies, and move in teams along the riverfront trail before lunch.",
		Venue:         "South Gate Plaza",
		VenueNote:     "Meet at the volunteer tent for gloves, route maps, and coffee before heading out.",
		Neighborhood:  "Bayfront",
		Location:      "Hamilton",
		Category:      "Volunteer",
		OrganizerName: "Friends of the Waterfront",
		OrganizerRole: "Community nonprofit",
		CoverImageURL: "https://picsum.photos/seed/riverfront-cleanup/1200/900",
		FeaturedBlurb: "Worth forwarding when your group wants a clear plan, a useful morning, and low friction.",
		Tags:          "cleanup, volunteer, outdoors, community",
		CrowdLabel:    "Easy to join last-minute",
		Timezone:      "America/Toronto",
		StartDate:     now.AddDate(0, 0, 10).Format("2006-01-02"),
		EndDate:       now.AddDate(0, 0, 10).Format("2006-01-02"),
		StartTime:     "09:00",
		EndTime:       "12:30",
	}, statusCanceled)
	s.mustSeed(eventInput{
		Title:         "Autumn Venue Crawl",
		Summary:       "Draft route for venue managers comparing neighborhood spaces.",
		Description:   "Internal draft itinerary used to compare capacity, transit access, and accessibility notes across partner venues.",
		Venue:         "Multiple venues",
		Neighborhood:  "West End",
		Location:      "Toronto",
		Category:      "Industry",
		OrganizerName: "Venue Operators Circle",
		OrganizerRole: "Industry roundtable",
		CoverImageURL: "https://picsum.photos/seed/venue-crawl/1200/900",
		FeaturedBlurb: "Draft internal walk-through for comparing room feel, logistics, and accessibility.",
		Tags:          "venues, operations, industry",
		CrowdLabel:    "Invite-only working session",
		Timezone:      "America/Toronto",
		StartDate:     now.AddDate(0, 0, 14).Format("2006-01-02"),
		EndDate:       now.AddDate(0, 0, 14).Format("2006-01-02"),
		StartTime:     "13:00",
		EndTime:       "17:00",
	}, statusDraft)
	s.mustSeed(eventInput{
		Title:         "Last Season Recap Gala",
		Summary:       "Archived gala page kept reachable for direct links and retrospectives.",
		Description:   "An archived page preserving the event record and venue notes after the live season wrapped.",
		Venue:         "Civic Exchange",
		VenueNote:     "Formal seated room used for end-of-season speeches, donor tables, and live music.",
		Neighborhood:  "Market District",
		Location:      "Kingston",
		Category:      "Celebration",
		OrganizerName: "Civic Exchange Society",
		OrganizerRole: "Arts fundraiser team",
		CoverImageURL: "https://picsum.photos/seed/recap-gala/1200/900",
		FeaturedBlurb: "Archived so direct links and sponsor recaps still have a stable home.",
		Tags:          "gala, archive, fundraiser",
		CrowdLabel:    "Archived season wrap-up",
		Timezone:      "America/Toronto",
		StartDate:     now.AddDate(0, -1, -4).Format("2006-01-02"),
		EndDate:       now.AddDate(0, -1, -4).Format("2006-01-02"),
		StartTime:     "19:00",
		EndTime:       "22:00",
	}, statusArchived)
	return s
}

func (s *memoryEventStore) Close() {}

func (s *memoryEventStore) mustSeed(input eventInput, status eventStatus) {
	event, err := s.create(input)
	if err != nil {
		panic(err)
	}
	if status != statusDraft {
		if _, err := s.setStatus(event.ID, status); err != nil {
			panic(err)
		}
	}
}

func (s *memoryEventStore) create(input eventInput) (*eventRecord, error) {
	start, end, err := parseSchedule(input)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	id := strconv.Itoa(s.nextID)
	s.nextID++
	now := time.Now().UTC()
	event := &eventRecord{
		ID:            id,
		Slug:          uniqueSlug(slugify(input.Title), id),
		Title:         strings.TrimSpace(input.Title),
		Summary:       strings.TrimSpace(input.Summary),
		Description:   strings.TrimSpace(input.Description),
		Venue:         strings.TrimSpace(input.Venue),
		VenueNote:     strings.TrimSpace(input.VenueNote),
		Neighborhood:  strings.TrimSpace(input.Neighborhood),
		Location:      strings.TrimSpace(input.Location),
		Category:      strings.TrimSpace(input.Category),
		OrganizerName: strings.TrimSpace(input.OrganizerName),
		OrganizerRole: strings.TrimSpace(input.OrganizerRole),
		OrganizerURL:  strings.TrimSpace(input.OrganizerURL),
		CoverImageURL: strings.TrimSpace(input.CoverImageURL),
		FeaturedBlurb: strings.TrimSpace(input.FeaturedBlurb),
		ExternalURL:   strings.TrimSpace(input.ExternalURL),
		Timezone:      strings.TrimSpace(input.Timezone),
		Tags:          normalizeTags(input.Tags),
		ShareCount:    seededShareCount(id, input.Category),
		SaveCount:     seededSaveCount(id, input.Category),
		CrowdLabel:    strings.TrimSpace(input.CrowdLabel),
		AllDay:        input.AllDay,
		Status:        statusDraft,
		Start:         start,
		End:           end,
		CreatedAt:     now,
		UpdatedAt:     now,
		RevisionCount: 2,
	}
	hydrateEventDefaults(event)
	s.events[id] = event
	s.order = append(s.order, id)
	s.ledger[id] = []eventClaim{
		{
			ID:         "claim_snapshot_" + id,
			EventID:    id,
			ClaimType:  "event.snapshot",
			AcceptedAt: now,
			AcceptedBy: "test",
			Summary:    "Recorded a full event snapshot for " + event.Title + ".",
			Snapshot:   snapshotFromRecord(event),
		},
		{
			ID:         "claim_status_" + id,
			EventID:    id,
			ClaimType:  "event.status",
			AcceptedAt: now,
			AcceptedBy: "test",
			Summary:    "Changed lifecycle state to draft.",
			Status:     string(statusDraft),
		},
	}
	return cloneEvent(event), nil
}

func (s *memoryEventStore) update(id string, input eventInput) (*eventRecord, error) {
	start, end, err := parseSchedule(input)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	event, ok := s.events[id]
	if !ok {
		return nil, errors.New("event not found")
	}
	event.Title = strings.TrimSpace(input.Title)
	event.Summary = strings.TrimSpace(input.Summary)
	event.Description = strings.TrimSpace(input.Description)
	event.Venue = strings.TrimSpace(input.Venue)
	event.VenueNote = strings.TrimSpace(input.VenueNote)
	event.Neighborhood = strings.TrimSpace(input.Neighborhood)
	event.Location = strings.TrimSpace(input.Location)
	event.Category = strings.TrimSpace(input.Category)
	event.OrganizerName = strings.TrimSpace(input.OrganizerName)
	event.OrganizerRole = strings.TrimSpace(input.OrganizerRole)
	event.OrganizerURL = strings.TrimSpace(input.OrganizerURL)
	event.CoverImageURL = strings.TrimSpace(input.CoverImageURL)
	event.FeaturedBlurb = strings.TrimSpace(input.FeaturedBlurb)
	event.ExternalURL = strings.TrimSpace(input.ExternalURL)
	event.Timezone = strings.TrimSpace(input.Timezone)
	event.Tags = normalizeTags(input.Tags)
	event.CrowdLabel = strings.TrimSpace(input.CrowdLabel)
	event.AllDay = input.AllDay
	event.Start = start
	event.End = end
	event.UpdatedAt = time.Now().UTC()
	event.RevisionCount++
	hydrateEventDefaults(event)
	s.ledger[id] = append(s.ledger[id], eventClaim{
		ID:         "claim_snapshot_" + strconv.Itoa(len(s.ledger[id])+1),
		EventID:    id,
		ClaimType:  "event.snapshot",
		AcceptedAt: event.UpdatedAt,
		AcceptedBy: "test",
		Summary:    "Recorded a full event snapshot for " + event.Title + ".",
		Snapshot:   snapshotFromRecord(event),
	})
	return cloneEvent(event), nil
}

func (s *memoryEventStore) setStatus(id string, status eventStatus) (*eventRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	event, ok := s.events[id]
	if !ok {
		return nil, errors.New("event not found")
	}
	event.Status = status
	event.UpdatedAt = time.Now().UTC()
	event.RevisionCount++
	s.ledger[id] = append(s.ledger[id], eventClaim{
		ID:         "claim_status_" + strconv.Itoa(len(s.ledger[id])+1),
		EventID:    id,
		ClaimType:  "event.status",
		AcceptedAt: event.UpdatedAt,
		AcceptedBy: "test",
		Summary:    "Changed lifecycle state to " + string(status) + ".",
		Status:     string(status),
	})
	return cloneEvent(event), nil
}

func (s *memoryEventStore) byID(id string) (*eventRecord, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	event, ok := s.events[id]
	if !ok {
		return nil, false
	}
	return cloneEvent(event), true
}

func (s *memoryEventStore) bySlug(slug string) (*eventRecord, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, id := range s.order {
		event := s.events[id]
		if event.Slug == slug {
			return cloneEvent(event), true
		}
	}
	return nil, false
}

func (s *memoryEventStore) all() []*eventRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()

	items := make([]*eventRecord, 0, len(s.order))
	for _, id := range s.order {
		items = append(items, cloneEvent(s.events[id]))
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Start.Equal(items[j].Start) {
			return items[i].Title < items[j].Title
		}
		return items[i].Start.Before(items[j].Start)
	})
	return items
}

func (s *memoryEventStore) ledgerByID(id string) ([]eventClaim, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	claims := s.ledger[id]
	out := make([]eventClaim, len(claims))
	copy(out, claims)
	return out, nil
}
