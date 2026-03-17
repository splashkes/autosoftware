package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type eventStore interface {
	create(eventInput) (*eventRecord, error)
	update(string, eventInput) (*eventRecord, error)
	setStatus(string, eventStatus) (*eventRecord, error)
	byID(string) (*eventRecord, bool)
	bySlug(string) (*eventRecord, bool)
	all() []*eventRecord
	ledgerByID(string) ([]eventClaim, error)
	Close()
}

type postgresEventStore struct {
	pool *pgxpool.Pool
}

type eventClaim struct {
	ID                string      `json:"id"`
	EventID           string      `json:"event_id"`
	ClaimType         string      `json:"claim_type"`
	AcceptedAt        time.Time   `json:"accepted_at"`
	AcceptedBy        string      `json:"accepted_by"`
	SupersedesClaimID string      `json:"supersedes_claim_id,omitempty"`
	Summary           string      `json:"summary"`
	Status            string      `json:"status,omitempty"`
	Snapshot          any         `json:"snapshot,omitempty"`
	Payload           interface{} `json:"payload,omitempty"`
}

type eventClaimPayload struct {
	Status   string         `json:"status,omitempty"`
	Snapshot *eventSnapshot `json:"snapshot,omitempty"`
}

type eventSnapshot struct {
	Slug          string   `json:"slug"`
	Title         string   `json:"title"`
	Summary       string   `json:"summary"`
	Description   string   `json:"description"`
	Venue         string   `json:"venue"`
	VenueNote     string   `json:"venue_note"`
	Neighborhood  string   `json:"neighborhood"`
	Location      string   `json:"location"`
	Category      string   `json:"category"`
	OrganizerName string   `json:"organizer_name"`
	OrganizerRole string   `json:"organizer_role"`
	OrganizerURL  string   `json:"organizer_url"`
	CoverImageURL string   `json:"cover_image_url"`
	FeaturedBlurb string   `json:"featured_blurb"`
	ExternalURL   string   `json:"external_url"`
	Timezone      string   `json:"timezone"`
	Tags          []string `json:"tags"`
	ShareCount    int      `json:"share_count"`
	SaveCount     int      `json:"save_count"`
	CrowdLabel    string   `json:"crowd_label"`
	AllDay        bool     `json:"all_day"`
	Start         string   `json:"start"`
	End           string   `json:"end"`
}

type storedEvent struct {
	Event             *eventRecord
	RevisionCount     int
	LastSnapshotClaim string
	LastStatusClaim   string
}

func newEventStore(databaseURL string) (eventStore, error) {
	if strings.TrimSpace(databaseURL) == "" {
		return nil, errors.New("AS_RUNTIME_DATABASE_URL is required for the ledger-backed event realization")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("open event store database: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping event store database: %w", err)
	}

	store := &postgresEventStore{pool: pool}
	if err := store.migrate(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	if err := store.seedIfEmpty(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return store, nil
}

func (s *postgresEventStore) Close() {
	if s != nil && s.pool != nil {
		s.pool.Close()
	}
}

func (s *postgresEventStore) migrate(ctx context.Context) error {
	if s == nil || s.pool == nil {
		return errors.New("event store unavailable")
	}
	_, err := s.pool.Exec(ctx, `
create table if not exists as_event_listings_objects (
  event_id text primary key,
  slug text not null unique,
  created_at timestamptz not null default now(),
  created_by text not null default 'system'
);

create table if not exists as_event_listings_claims (
  claim_id text primary key,
  event_id text not null references as_event_listings_objects(event_id) on delete cascade,
  claim_seq bigint generated always as identity unique,
  claim_type text not null,
  accepted_at timestamptz not null default now(),
  accepted_by text not null,
  supersedes_claim_id text references as_event_listings_claims(claim_id),
  payload jsonb not null default '{}'::jsonb
);

create index if not exists as_event_listings_claims_event_idx
  on as_event_listings_claims (event_id, claim_seq desc);

create table if not exists as_event_listings_materialized_events (
  event_id text primary key references as_event_listings_objects(event_id) on delete cascade,
  slug text not null unique,
  title text not null,
  summary text not null,
  description text not null,
  venue text not null,
  venue_note text not null default '',
  neighborhood text not null default '',
  location text not null,
  category text not null,
  organizer_name text not null default '',
  organizer_role text not null default '',
  organizer_url text not null default '',
  cover_image_url text not null default '',
  featured_blurb text not null default '',
  external_url text not null default '',
  timezone text not null,
  tags text[] not null default '{}',
  share_count integer not null default 0,
  save_count integer not null default 0,
  crowd_label text not null default '',
  all_day boolean not null default false,
  status text not null,
  start_at timestamptz not null,
  end_at timestamptz not null,
  created_at timestamptz not null,
  updated_at timestamptz not null,
  revision_count integer not null default 0,
  last_snapshot_claim_id text references as_event_listings_claims(claim_id),
  last_status_claim_id text references as_event_listings_claims(claim_id)
);
`)
	if err != nil {
		return fmt.Errorf("migrate event store: %w", err)
	}
	return nil
}

func (s *postgresEventStore) seedIfEmpty(ctx context.Context) error {
	var count int
	if err := s.pool.QueryRow(ctx, `select count(*) from as_event_listings_objects`).Scan(&count); err != nil {
		return fmt.Errorf("count event objects: %w", err)
	}
	if count > 0 {
		return nil
	}

	now := time.Now().UTC()
	seeds := []struct {
		Input  eventInput
		Status eventStatus
	}{
		{
			Input: eventInput{
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
			},
			Status: statusPublished,
		},
		{
			Input: eventInput{
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
			},
			Status: statusPublished,
		},
		{
			Input: eventInput{
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
			},
			Status: statusCanceled,
		},
		{
			Input: eventInput{
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
			},
			Status: statusDraft,
		},
		{
			Input: eventInput{
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
			},
			Status: statusArchived,
		},
	}

	for _, seed := range seeds {
		event, err := s.createAs(ctx, seed.Input, "seed")
		if err != nil {
			return fmt.Errorf("seed event %q: %w", seed.Input.Title, err)
		}
		if seed.Status != statusDraft {
			if _, err := s.setStatusAs(ctx, event.ID, seed.Status, "seed"); err != nil {
				return fmt.Errorf("seed status for %q: %w", seed.Input.Title, err)
			}
		}
	}
	return nil
}

func (s *postgresEventStore) create(input eventInput) (*eventRecord, error) {
	return s.createAs(context.Background(), input, "organizer")
}

func (s *postgresEventStore) createAs(ctx context.Context, input eventInput, acceptedBy string) (*eventRecord, error) {
	start, end, err := parseSchedule(input)
	if err != nil {
		return nil, err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin create event: %w", err)
	}
	defer tx.Rollback(ctx)

	eventID := newLedgerID("evt")
	slug, err := s.allocateSlug(ctx, tx, strings.TrimSpace(input.Title))
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	event := &eventRecord{
		ID:            eventID,
		Slug:          slug,
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
		ShareCount:    seededShareCount(eventID, input.Category),
		SaveCount:     seededSaveCount(eventID, input.Category),
		CrowdLabel:    strings.TrimSpace(input.CrowdLabel),
		AllDay:        input.AllDay,
		Status:        statusDraft,
		Start:         start,
		End:           end,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	hydrateEventDefaults(event)

	if _, err := tx.Exec(ctx, `
insert into as_event_listings_objects (event_id, slug, created_at, created_by)
values ($1, $2, $3, $4)
`, event.ID, event.Slug, event.CreatedAt, nonEmpty(acceptedBy, "organizer")); err != nil {
		return nil, fmt.Errorf("insert event object: %w", err)
	}

	snapshotClaimID, err := s.insertSnapshotClaim(ctx, tx, event, "", acceptedBy)
	if err != nil {
		return nil, err
	}
	statusClaimID, err := s.insertStatusClaim(ctx, tx, event.ID, event.Status, "", acceptedBy)
	if err != nil {
		return nil, err
	}
	if err := s.upsertMaterialized(ctx, tx, event, 2, snapshotClaimID, statusClaimID); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit create event: %w", err)
	}
	return cloneEvent(event), nil
}

func (s *postgresEventStore) update(id string, input eventInput) (*eventRecord, error) {
	return s.updateAs(context.Background(), id, input, "organizer")
}

func (s *postgresEventStore) updateAs(ctx context.Context, id string, input eventInput, acceptedBy string) (*eventRecord, error) {
	start, end, err := parseSchedule(input)
	if err != nil {
		return nil, err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin update event: %w", err)
	}
	defer tx.Rollback(ctx)

	current, err := s.loadStoredEvent(ctx, tx, id)
	if err != nil {
		return nil, err
	}
	event := current.Event
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
	hydrateEventDefaults(event)

	snapshotClaimID, err := s.insertSnapshotClaim(ctx, tx, event, current.LastSnapshotClaim, acceptedBy)
	if err != nil {
		return nil, err
	}
	if err := s.upsertMaterialized(ctx, tx, event, current.RevisionCount+1, snapshotClaimID, current.LastStatusClaim); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit update event: %w", err)
	}
	return cloneEvent(event), nil
}

func (s *postgresEventStore) setStatus(id string, status eventStatus) (*eventRecord, error) {
	return s.setStatusAs(context.Background(), id, status, "organizer")
}

func (s *postgresEventStore) setStatusAs(ctx context.Context, id string, status eventStatus, acceptedBy string) (*eventRecord, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin set status: %w", err)
	}
	defer tx.Rollback(ctx)

	current, err := s.loadStoredEvent(ctx, tx, id)
	if err != nil {
		return nil, err
	}
	current.Event.Status = status
	current.Event.UpdatedAt = time.Now().UTC()

	statusClaimID, err := s.insertStatusClaim(ctx, tx, id, status, current.LastStatusClaim, acceptedBy)
	if err != nil {
		return nil, err
	}
	if err := s.upsertMaterialized(ctx, tx, current.Event, current.RevisionCount+1, current.LastSnapshotClaim, statusClaimID); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit set status: %w", err)
	}
	return cloneEvent(current.Event), nil
}

func (s *postgresEventStore) byID(id string) (*eventRecord, bool) {
	event, err := s.fetchEvent(context.Background(), `select
  event_id, slug, title, summary, description, venue, venue_note, neighborhood,
  location, category, organizer_name, organizer_role, organizer_url, cover_image_url,
  featured_blurb, external_url, timezone, tags, share_count, save_count, crowd_label,
  all_day, status, start_at, end_at, created_at, updated_at, revision_count
from as_event_listings_materialized_events
where event_id = $1`, id)
	if err != nil {
		return nil, false
	}
	return event, true
}

func (s *postgresEventStore) bySlug(slug string) (*eventRecord, bool) {
	event, err := s.fetchEvent(context.Background(), `select
  event_id, slug, title, summary, description, venue, venue_note, neighborhood,
  location, category, organizer_name, organizer_role, organizer_url, cover_image_url,
  featured_blurb, external_url, timezone, tags, share_count, save_count, crowd_label,
  all_day, status, start_at, end_at, created_at, updated_at, revision_count
from as_event_listings_materialized_events
where slug = $1`, slug)
	if err != nil {
		return nil, false
	}
	return event, true
}

func (s *postgresEventStore) all() []*eventRecord {
	rows, err := s.pool.Query(context.Background(), `select
  event_id, slug, title, summary, description, venue, venue_note, neighborhood,
  location, category, organizer_name, organizer_role, organizer_url, cover_image_url,
  featured_blurb, external_url, timezone, tags, share_count, save_count, crowd_label,
  all_day, status, start_at, end_at, created_at, updated_at, revision_count
from as_event_listings_materialized_events
order by start_at asc, title asc`)
	if err != nil {
		return nil
	}
	defer rows.Close()

	items := make([]*eventRecord, 0)
	for rows.Next() {
		event, err := scanEvent(rows)
		if err != nil {
			continue
		}
		items = append(items, event)
	}
	return items
}

func (s *postgresEventStore) ledgerByID(id string) ([]eventClaim, error) {
	rows, err := s.pool.Query(context.Background(), `
select claim_id, event_id, claim_type, accepted_at, accepted_by, coalesce(supersedes_claim_id, ''), payload
from as_event_listings_claims
where event_id = $1
order by claim_seq asc
`, id)
	if err != nil {
		return nil, fmt.Errorf("query event ledger: %w", err)
	}
	defer rows.Close()

	var claims []eventClaim
	for rows.Next() {
		var (
			claim eventClaim
			raw   []byte
		)
		if err := rows.Scan(&claim.ID, &claim.EventID, &claim.ClaimType, &claim.AcceptedAt, &claim.AcceptedBy, &claim.SupersedesClaimID, &raw); err != nil {
			return nil, fmt.Errorf("scan event ledger: %w", err)
		}
		var payload eventClaimPayload
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &payload); err != nil {
				return nil, fmt.Errorf("decode event claim payload: %w", err)
			}
		}
		claim.Status = payload.Status
		claim.Snapshot = payload.Snapshot
		claim.Payload = payload
		claim.Summary = summarizeClaim(claim.ClaimType, payload)
		claims = append(claims, claim)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate event ledger: %w", err)
	}
	return claims, nil
}

func (s *postgresEventStore) fetchEvent(ctx context.Context, query string, arg string) (*eventRecord, error) {
	row := s.pool.QueryRow(ctx, query, arg)
	event, err := scanEvent(row)
	if err != nil {
		return nil, err
	}
	return event, nil
}

func (s *postgresEventStore) loadStoredEvent(ctx context.Context, tx pgx.Tx, id string) (*storedEvent, error) {
	row := tx.QueryRow(ctx, `select
  event_id, slug, title, summary, description, venue, venue_note, neighborhood,
  location, category, organizer_name, organizer_role, organizer_url, cover_image_url,
  featured_blurb, external_url, timezone, tags, share_count, save_count, crowd_label,
  all_day, status, start_at, end_at, created_at, updated_at, revision_count,
  coalesce(last_snapshot_claim_id, ''), coalesce(last_status_claim_id, '')
from as_event_listings_materialized_events
where event_id = $1
for update`, id)
	var (
		event             eventRecord
		tags              []string
		revisionCount     int
		lastSnapshotClaim string
		lastStatusClaim   string
		status            string
	)
	err := row.Scan(
		&event.ID,
		&event.Slug,
		&event.Title,
		&event.Summary,
		&event.Description,
		&event.Venue,
		&event.VenueNote,
		&event.Neighborhood,
		&event.Location,
		&event.Category,
		&event.OrganizerName,
		&event.OrganizerRole,
		&event.OrganizerURL,
		&event.CoverImageURL,
		&event.FeaturedBlurb,
		&event.ExternalURL,
		&event.Timezone,
		&tags,
		&event.ShareCount,
		&event.SaveCount,
		&event.CrowdLabel,
		&event.AllDay,
		&status,
		&event.Start,
		&event.End,
		&event.CreatedAt,
		&event.UpdatedAt,
		&revisionCount,
		&lastSnapshotClaim,
		&lastStatusClaim,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errors.New("event not found")
	}
	if err != nil {
		return nil, fmt.Errorf("load stored event: %w", err)
	}
	event.Tags = append([]string(nil), tags...)
	event.Status = eventStatus(status)
	hydrateEventDefaults(&event)
	return &storedEvent{
		Event:             &event,
		RevisionCount:     revisionCount,
		LastSnapshotClaim: lastSnapshotClaim,
		LastStatusClaim:   lastStatusClaim,
	}, nil
}

func (s *postgresEventStore) allocateSlug(ctx context.Context, tx pgx.Tx, title string) (string, error) {
	base := slugify(title)
	if base == "" {
		base = "event"
	}
	slug := base
	for i := 2; ; i++ {
		var exists bool
		if err := tx.QueryRow(ctx, `select exists(select 1 from as_event_listings_objects where slug = $1)`, slug).Scan(&exists); err != nil {
			return "", fmt.Errorf("check existing slug: %w", err)
		}
		if !exists {
			return slug, nil
		}
		slug = fmt.Sprintf("%s-%d", base, i)
	}
}

func (s *postgresEventStore) insertSnapshotClaim(ctx context.Context, tx pgx.Tx, event *eventRecord, supersedes, acceptedBy string) (string, error) {
	claimID := newLedgerID("claim")
	payload, err := json.Marshal(eventClaimPayload{Snapshot: snapshotFromRecord(event)})
	if err != nil {
		return "", fmt.Errorf("marshal snapshot claim payload: %w", err)
	}
	_, err = tx.Exec(ctx, `
insert into as_event_listings_claims (claim_id, event_id, claim_type, accepted_at, accepted_by, supersedes_claim_id, payload)
values ($1, $2, 'event.snapshot', $3, $4, nullif($5, ''), $6::jsonb)
`, claimID, event.ID, time.Now().UTC(), nonEmpty(acceptedBy, "organizer"), supersedes, string(payload))
	if err != nil {
		return "", fmt.Errorf("insert snapshot claim: %w", err)
	}
	return claimID, nil
}

func (s *postgresEventStore) insertStatusClaim(ctx context.Context, tx pgx.Tx, eventID string, status eventStatus, supersedes, acceptedBy string) (string, error) {
	claimID := newLedgerID("claim")
	payload, err := json.Marshal(eventClaimPayload{Status: string(status)})
	if err != nil {
		return "", fmt.Errorf("marshal status claim payload: %w", err)
	}
	_, err = tx.Exec(ctx, `
insert into as_event_listings_claims (claim_id, event_id, claim_type, accepted_at, accepted_by, supersedes_claim_id, payload)
values ($1, $2, 'event.status', $3, $4, nullif($5, ''), $6::jsonb)
`, claimID, eventID, time.Now().UTC(), nonEmpty(acceptedBy, "organizer"), supersedes, string(payload))
	if err != nil {
		return "", fmt.Errorf("insert status claim: %w", err)
	}
	return claimID, nil
}

func (s *postgresEventStore) upsertMaterialized(ctx context.Context, tx pgx.Tx, event *eventRecord, revisionCount int, snapshotClaimID, statusClaimID string) error {
	_, err := tx.Exec(ctx, `
insert into as_event_listings_materialized_events (
  event_id, slug, title, summary, description, venue, venue_note, neighborhood,
  location, category, organizer_name, organizer_role, organizer_url, cover_image_url,
  featured_blurb, external_url, timezone, tags, share_count, save_count, crowd_label,
  all_day, status, start_at, end_at, created_at, updated_at, revision_count,
  last_snapshot_claim_id, last_status_claim_id
)
values (
  $1, $2, $3, $4, $5, $6, $7, $8,
  $9, $10, $11, $12, $13, $14,
  $15, $16, $17, $18, $19, $20, $21,
  $22, $23, $24, $25, $26, $27, $28,
  nullif($29, ''), nullif($30, '')
)
on conflict (event_id)
do update set
  slug = excluded.slug,
  title = excluded.title,
  summary = excluded.summary,
  description = excluded.description,
  venue = excluded.venue,
  venue_note = excluded.venue_note,
  neighborhood = excluded.neighborhood,
  location = excluded.location,
  category = excluded.category,
  organizer_name = excluded.organizer_name,
  organizer_role = excluded.organizer_role,
  organizer_url = excluded.organizer_url,
  cover_image_url = excluded.cover_image_url,
  featured_blurb = excluded.featured_blurb,
  external_url = excluded.external_url,
  timezone = excluded.timezone,
  tags = excluded.tags,
  share_count = excluded.share_count,
  save_count = excluded.save_count,
  crowd_label = excluded.crowd_label,
  all_day = excluded.all_day,
  status = excluded.status,
  start_at = excluded.start_at,
  end_at = excluded.end_at,
  created_at = excluded.created_at,
  updated_at = excluded.updated_at,
  revision_count = excluded.revision_count,
  last_snapshot_claim_id = excluded.last_snapshot_claim_id,
  last_status_claim_id = excluded.last_status_claim_id
`, event.ID, event.Slug, event.Title, event.Summary, event.Description, event.Venue, event.VenueNote, event.Neighborhood,
		event.Location, event.Category, event.OrganizerName, event.OrganizerRole, event.OrganizerURL, event.CoverImageURL,
		event.FeaturedBlurb, event.ExternalURL, event.Timezone, event.Tags, event.ShareCount, event.SaveCount, event.CrowdLabel,
		event.AllDay, string(event.Status), event.Start.UTC(), event.End.UTC(), event.CreatedAt.UTC(), event.UpdatedAt.UTC(), revisionCount,
		snapshotClaimID, statusClaimID)
	if err != nil {
		return fmt.Errorf("upsert materialized event: %w", err)
	}
	return nil
}

func scanEvent(row interface{ Scan(...any) error }) (*eventRecord, error) {
	var (
		event  eventRecord
		tags   []string
		status string
	)
	err := row.Scan(
		&event.ID,
		&event.Slug,
		&event.Title,
		&event.Summary,
		&event.Description,
		&event.Venue,
		&event.VenueNote,
		&event.Neighborhood,
		&event.Location,
		&event.Category,
		&event.OrganizerName,
		&event.OrganizerRole,
		&event.OrganizerURL,
		&event.CoverImageURL,
		&event.FeaturedBlurb,
		&event.ExternalURL,
		&event.Timezone,
		&tags,
		&event.ShareCount,
		&event.SaveCount,
		&event.CrowdLabel,
		&event.AllDay,
		&status,
		&event.Start,
		&event.End,
		&event.CreatedAt,
		&event.UpdatedAt,
		&event.RevisionCount,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	if err != nil {
		return nil, fmt.Errorf("scan event: %w", err)
	}
	event.Tags = append([]string(nil), tags...)
	event.Status = eventStatus(status)
	hydrateEventDefaults(&event)
	return &event, nil
}

func cloneEvent(in *eventRecord) *eventRecord {
	if in == nil {
		return nil
	}
	copy := *in
	copy.Tags = append([]string(nil), in.Tags...)
	return &copy
}

func hydrateEventDefaults(event *eventRecord) {
	if strings.TrimSpace(event.OrganizerName) == "" {
		event.OrganizerName = "Field Guide Team"
	}
	if strings.TrimSpace(event.OrganizerRole) == "" {
		event.OrganizerRole = "Local organizer"
	}
	if strings.TrimSpace(event.CrowdLabel) == "" {
		event.CrowdLabel = "Easy to share"
	}
	if len(event.Tags) == 0 {
		event.Tags = normalizeTags(strings.Join([]string{event.Category, event.Location}, ","))
	}
	if event.ShareCount == 0 {
		event.ShareCount = seededShareCount(event.ID, event.Category)
	}
	if event.SaveCount == 0 {
		event.SaveCount = seededSaveCount(event.ID, event.Category)
	}
	if strings.TrimSpace(event.FeaturedBlurb) == "" {
		event.FeaturedBlurb = eventDiscoveryNote(event)
	}
}

func snapshotFromRecord(event *eventRecord) *eventSnapshot {
	return &eventSnapshot{
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
		ExternalURL:   event.ExternalURL,
		Timezone:      event.Timezone,
		Tags:          append([]string(nil), event.Tags...),
		ShareCount:    event.ShareCount,
		SaveCount:     event.SaveCount,
		CrowdLabel:    event.CrowdLabel,
		AllDay:        event.AllDay,
		Start:         event.Start.Format(time.RFC3339),
		End:           event.End.Format(time.RFC3339),
	}
}

func summarizeClaim(claimType string, payload eventClaimPayload) string {
	switch claimType {
	case "event.snapshot":
		if payload.Snapshot == nil {
			return "Captured a new event snapshot."
		}
		return "Recorded a full event snapshot for " + payload.Snapshot.Title + "."
	case "event.status":
		if payload.Status == "" {
			return "Changed event state."
		}
		return "Changed lifecycle state to " + payload.Status + "."
	default:
		return "Accepted event ledger change."
	}
}

func newLedgerID(prefix string) string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("%s_%d", prefix, time.Now().UTC().UnixNano())
	}
	return prefix + "_" + hex.EncodeToString(buf)
}

func nonEmpty(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}
