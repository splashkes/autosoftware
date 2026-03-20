package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

// --- Mock registry API ---

func newMockRegistry() *httptest.Server {
	mux := http.NewServeMux()
	realizationHash := strings.Repeat("1", 64)
	commandCreateHash := strings.Repeat("2", 64)
	commandUpdateHash := strings.Repeat("3", 64)
	projectionHash := strings.Repeat("4", 64)
	objectHash := strings.Repeat("5", 64)
	schemaHash := strings.Repeat("6", 64)
	noteCreatedAt := time.Date(2026, time.March, 19, 12, 0, 0, 0, time.UTC)
	noteUpdatedAt := time.Date(2026, time.March, 20, 9, 30, 0, 0, time.UTC)

	rows := []RegistryRow{
		{
			RowID:         1,
			ChangeSetID:   "cs-note-create",
			Reference:     "0001-notepad/a-go-htmx",
			SeedID:        "0001-notepad",
			RealizationID: "a-go-htmx",
			RowOrder:      1,
			RowType:       "object.create",
			ObjectID:      "note-1",
			Payload: map[string]any{
				"id":          "note-1",
				"object_type": "note",
				"slug":        "hello-world",
			},
			AcceptedAt: noteCreatedAt,
		},
		{
			RowID:         2,
			ChangeSetID:   "cs-note-create",
			Reference:     "0001-notepad/a-go-htmx",
			SeedID:        "0001-notepad",
			RealizationID: "a-go-htmx",
			RowOrder:      2,
			RowType:       "claim.create",
			ObjectID:      "note-1",
			ClaimID:       "claim-note-create",
			Payload: map[string]any{
				"claim_id":   "claim-note-create",
				"object_id":  "note-1",
				"claim_type": "note.created",
				"title":      "Hello World",
				"edited_by":  "actor-1",
			},
			AcceptedAt: noteCreatedAt,
		},
		{
			RowID:         3,
			ChangeSetID:   "cs-actor-create",
			Reference:     "0001-notepad/a-go-htmx",
			SeedID:        "0001-notepad",
			RealizationID: "a-go-htmx",
			RowOrder:      1,
			RowType:       "object.create",
			ObjectID:      "actor-1",
			Payload: map[string]any{
				"id":          "actor-1",
				"object_type": "actor",
				"slug":        "sam-editor",
			},
			AcceptedAt: noteCreatedAt,
		},
		{
			RowID:         4,
			ChangeSetID:   "cs-actor-create",
			Reference:     "0001-notepad/a-go-htmx",
			SeedID:        "0001-notepad",
			RealizationID: "a-go-htmx",
			RowOrder:      2,
			RowType:       "claim.create",
			ObjectID:      "actor-1",
			ClaimID:       "claim-actor-create",
			Payload: map[string]any{
				"claim_id":     "claim-actor-create",
				"object_id":    "actor-1",
				"claim_type":   "actor.created",
				"display_name": "Sam Editor",
			},
			AcceptedAt: noteCreatedAt,
		},
		{
			RowID:         5,
			ChangeSetID:   "cs-note-update",
			Reference:     "0001-notepad/a-go-htmx",
			SeedID:        "0001-notepad",
			RealizationID: "a-go-htmx",
			RowOrder:      1,
			RowType:       "claim.create",
			ObjectID:      "note-1",
			ClaimID:       "claim-note-update",
			Payload: map[string]any{
				"claim_id":   "claim-note-update",
				"object_id":  "note-1",
				"claim_type": "note.updated",
				"title":      "Hello World Revised",
				"edited_by":  "actor-1",
			},
			AcceptedAt: noteUpdatedAt,
		},
	}

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
		seedID := r.URL.Query().Get("seed_id")
		items := []RealizationSummary{
			{Reference: "0001-notepad/a-go-htmx", SeedID: "0001-notepad", RealizationID: "a-go-htmx", Summary: "Shared notepad", Status: "draft", SurfaceKind: "interactive", CommandCount: 2, ProjectionCount: 1},
			{Reference: "0004-events/a-web-mvp", SeedID: "0004-events", RealizationID: "a-web-mvp", Summary: "Event listings", Status: "draft", SurfaceKind: "interactive", CommandCount: 2, ProjectionCount: 4},
		}
		if seedID != "" {
			var filtered []RealizationSummary
			for _, item := range items {
				if item.SeedID == seedID {
					filtered = append(filtered, item)
				}
			}
			items = filtered
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
					Objects: []ResourceLink{{Kind: "shared_note"}, {Kind: "actor"}},
					Relations: []GraphRelation{{
						Kind:        "note_edited_by",
						Summary:     "Tracks who last edited the shared note.",
						Cardinality: "many_to_one",
						Visibility:  "mixed",
						FromObjects: []ResourceLink{{Kind: "shared_note"}},
						ToObjects:   []ResourceLink{{Kind: "actor"}},
					}},
					Commands:     []ResourceLink{{Name: "notes.create"}, {Name: "notes.update"}},
					Projections:  []ResourceLink{{Name: "notes.room"}},
					Self:         "/v1/registry/realization?reference=0001-notepad%2Fa-go-htmx",
					CanonicalURL: "https://registry.autosoftware.app/contracts/0001-notepad/a-go-htmx",
					PermalinkURL: "https://registry.autosoftware.app/reg/" + realizationHash,
					ContentHash:  realizationHash,
				},
			})
			return
		}
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
	})

	mux.HandleFunc("GET /v1/registry/commands", func(w http.ResponseWriter, r *http.Request) {
		seedID := r.URL.Query().Get("seed_id")
		items := []CommandSummary{
			{Reference: "0001-notepad/a-go-htmx", SeedID: "0001-notepad", RealizationID: "a-go-htmx", Name: "notes.create", Path: "/v1/commands/0001-notepad/notes.create"},
			{Reference: "0001-notepad/a-go-htmx", SeedID: "0001-notepad", RealizationID: "a-go-htmx", Name: "notes.update", Path: "/v1/commands/0001-notepad/notes.update"},
		}
		if seedID != "" {
			var filtered []CommandSummary
			for _, item := range items {
				if item.SeedID == seedID {
					filtered = append(filtered, item)
				}
			}
			items = filtered
		}
		json.NewEncoder(w).Encode(map[string]any{"commands": items})
	})

	mux.HandleFunc("GET /v1/registry/command", func(w http.ResponseWriter, r *http.Request) {
		ref := r.URL.Query().Get("reference")
		name := r.URL.Query().Get("name")
		if ref == "0001-notepad/a-go-htmx" && (name == "notes.create" || name == "notes.update") {
			summary := "Create a note"
			path := "/v1/commands/0001-notepad/notes.create"
			self := "/v1/registry/command?reference=0001-notepad%2Fa-go-htmx&name=notes.create"
			if name == "notes.update" {
				summary = "Update a note"
				path = "/v1/commands/0001-notepad/notes.update"
				self = "/v1/registry/command?reference=0001-notepad%2Fa-go-htmx&name=notes.update"
			}
			contentHash := commandCreateHash
			if name == "notes.update" {
				contentHash = commandUpdateHash
			}
			json.NewEncoder(w).Encode(map[string]any{
				"command": CommandDetail{
					Reference: ref, SeedID: "0001-notepad", RealizationID: "a-go-htmx",
					Name: name, Summary: summary, Path: path,
					AuthModes: []string{"anonymous"}, Idempotency: "required", Consistency: "read_your_writes",
					Self:         self,
					CanonicalURL: "/actions/0001-notepad/a-go-htmx/" + name,
					PermalinkURL: "/reg/" + contentHash,
					ContentHash:  contentHash,
				},
			})
			return
		}
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
	})

	mux.HandleFunc("GET /v1/registry/projections", func(w http.ResponseWriter, r *http.Request) {
		seedID := r.URL.Query().Get("seed_id")
		items := []ProjectionSummary{
			{Reference: "0001-notepad/a-go-htmx", SeedID: "0001-notepad", RealizationID: "a-go-htmx", Name: "notes.room", Path: "/v1/projections/0001-notepad/notes.room", AuthModes: []string{"anonymous", "session"}, Freshness: "materialized"},
		}
		if seedID != "" {
			var filtered []ProjectionSummary
			for _, item := range items {
				if item.SeedID == seedID {
					filtered = append(filtered, item)
				}
			}
			items = filtered
		}
		json.NewEncoder(w).Encode(map[string]any{"projections": items})
	})

	mux.HandleFunc("GET /v1/registry/projection", func(w http.ResponseWriter, r *http.Request) {
		ref := r.URL.Query().Get("reference")
		name := r.URL.Query().Get("name")
		if ref == "0001-notepad/a-go-htmx" && name == "notes.room" {
			json.NewEncoder(w).Encode(map[string]any{
				"projection": ProjectionDetail{
					Reference: ref, SeedID: "0001-notepad", RealizationID: "a-go-htmx",
					Name: "notes.room", Summary: "Note room view", Path: "/v1/projections/0001-notepad/notes.room",
					AuthModes: []string{"anonymous", "session"},
					Freshness: "materialized",
					DataViews: []DataView{
						{AuthModes: []string{"anonymous"}, Sections: []string{"shared_metadata", "public_payload"}, Summary: "Anonymous readers get the public note."},
						{AuthModes: []string{"session"}, Sections: []string{"shared_metadata", "public_payload", "private_payload"}, Summary: "Session readers get the fuller note."},
					},
					Self:         "/v1/registry/projection?reference=0001-notepad%2Fa-go-htmx&name=notes.room",
					CanonicalURL: "/read-models/0001-notepad/a-go-htmx/notes.room",
					PermalinkURL: "/reg/" + projectionHash,
					ContentHash:  projectionHash,
				},
			})
			return
		}
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
	})

	mux.HandleFunc("GET /v1/registry/objects", func(w http.ResponseWriter, r *http.Request) {
		seedID := r.URL.Query().Get("seed_id")
		items := []ObjectSummary{
			{SeedID: "0001-notepad", Kind: "shared_note", Summary: "A shared note", RealizationCount: 1, CommandCount: 2, ProjectionCount: 1, Self: "/v1/registry/object?seed_id=0001-notepad&kind=shared_note"},
			{SeedID: "0001-notepad", Kind: "actor", Summary: "A note actor", RealizationCount: 1, CommandCount: 0, ProjectionCount: 0, Self: "/v1/registry/object?seed_id=0001-notepad&kind=actor"},
			{SeedID: "0004-events", Kind: "event", Summary: "An event", RealizationCount: 1, CommandCount: 2, ProjectionCount: 3, Self: "/v1/registry/object?seed_id=0004-events&kind=event"},
		}
		if seedID != "" {
			var filtered []ObjectSummary
			for _, item := range items {
				if item.SeedID == seedID {
					filtered = append(filtered, item)
				}
			}
			items = filtered
		}
		json.NewEncoder(w).Encode(map[string]any{"objects": items})
	})

	mux.HandleFunc("GET /v1/registry/object", func(w http.ResponseWriter, r *http.Request) {
		seedID := r.URL.Query().Get("seed_id")
		kind := r.URL.Query().Get("kind")
		if seedID == "0001-notepad" && kind == "shared_note" {
			json.NewEncoder(w).Encode(map[string]any{
				"object": ObjectDetail{
					SeedID: seedID, Kind: kind, Summary: "A shared note",
					Capabilities: []string{"search_documents"},
					DataLayout: DataLayout{
						SharedMetadata: DataSection{
							Summary: "Stable note identity.",
							Fields:  []DataField{{Name: "note_id", Type: "string", Summary: "Stable note id."}},
						},
						PublicPayload: DataSection{
							Summary: "Public note content.",
							Fields:  []DataField{{Name: "title", Type: "string", Summary: "Public title."}},
						},
						PrivatePayload: DataSection{
							Summary: "Private note content.",
							Fields:  []DataField{{Name: "draft_body", Type: "string", Summary: "Draft-only body."}},
						},
					},
					OutgoingRelations: []GraphRelation{{
						Kind:        "note_edited_by",
						Summary:     "Tracks which actor last edited the note.",
						Cardinality: "many_to_one",
						Visibility:  "mixed",
						ToObjects:   []ResourceLink{{Kind: "actor"}},
						Attributes:  []DataField{{Name: "edited_at", Type: "RFC3339 timestamp", Summary: "Timestamp for the latest edit."}},
					}},
					IncomingRelations: []GraphRelation{{
						Kind:        "note_forked_from",
						Summary:     "Tracks notes that fork from this one.",
						Cardinality: "one_to_many",
						Visibility:  "public",
						FromObjects: []ResourceLink{{Kind: "shared_note"}},
					}},
					Schemas: []ResourceLink{{Ref: "seeds/0001-notepad/design.md"}},
					Realizations: []ObjectRealization{
						{Reference: "0001-notepad/a-go-htmx", SeedID: "0001-notepad", RealizationID: "a-go-htmx", Summary: "Shared notepad", Status: "draft", SurfaceKind: "interactive"},
					},
					Self:         "/v1/registry/object?seed_id=0001-notepad&kind=shared_note",
					CanonicalURL: "/objects/0001-notepad/shared_note",
					PermalinkURL: "/reg/" + objectHash,
					ContentHash:  objectHash,
				},
			})
			return
		}
		if seedID == "0001-notepad" && kind == "actor" {
			json.NewEncoder(w).Encode(map[string]any{
				"object": ObjectDetail{
					SeedID: seedID, Kind: kind, Summary: "A note actor",
					DataLayout: DataLayout{
						SharedMetadata: DataSection{
							Summary: "Stable actor identity.",
							Fields:  []DataField{{Name: "actor_id", Type: "string", Summary: "Stable actor id."}},
						},
						PublicPayload: DataSection{
							Summary: "Public actor display fields.",
							Fields:  []DataField{{Name: "display_name", Type: "string", Summary: "Visible actor name."}},
						},
					},
					IncomingRelations: []GraphRelation{{
						Kind:        "note_edited_by",
						Summary:     "Tracks notes edited by this actor.",
						Cardinality: "one_to_many",
						Visibility:  "mixed",
						FromObjects: []ResourceLink{{Kind: "shared_note"}},
					}},
					Schemas: []ResourceLink{{Ref: "seeds/0001-notepad/design.md"}},
					Realizations: []ObjectRealization{
						{Reference: "0001-notepad/a-go-htmx", SeedID: "0001-notepad", RealizationID: "a-go-htmx", Summary: "Shared notepad", Status: "draft", SurfaceKind: "interactive"},
					},
					Self:         "/v1/registry/object?seed_id=0001-notepad&kind=actor",
					CanonicalURL: "/objects/0001-notepad/actor",
					PermalinkURL: "/reg/" + objectHash,
					ContentHash:  objectHash,
				},
			})
			return
		}
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
	})

	mux.HandleFunc("GET /v1/registry/rows", func(w http.ResponseWriter, r *http.Request) {
		seedID := r.URL.Query().Get("seed_id")
		after, _ := strconv.ParseInt(r.URL.Query().Get("after"), 10, 64)
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		if limit <= 0 {
			limit = len(rows)
		}
		filtered := make([]RegistryRow, 0, len(rows))
		for _, row := range rows {
			if seedID != "" && row.SeedID != seedID {
				continue
			}
			if after > 0 && row.RowID <= after {
				continue
			}
			filtered = append(filtered, row)
			if len(filtered) == limit {
				break
			}
		}
		json.NewEncoder(w).Encode(map[string]any{"rows": filtered})
	})

	mux.HandleFunc("GET /v1/registry/schemas", func(w http.ResponseWriter, r *http.Request) {
		seedID := r.URL.Query().Get("seed_id")
		items := []SchemaSummary{
			{Ref: "seeds/0001-notepad/design.md", Path: "seeds/0001-notepad/design.md", ObjectUseCount: 1, CommandInputCount: 0, CommandResultCount: 0, Self: "/v1/registry/schema?ref=seeds%2F0001-notepad%2Fdesign.md"},
		}
		if seedID != "" && seedID != "0001-notepad" {
			items = nil
		}
		json.NewEncoder(w).Encode(map[string]any{"schemas": items})
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
					Self:         "/v1/registry/schema?ref=seeds%2F0001-notepad%2Fdesign.md",
					CanonicalURL: "https://registry.autosoftware.app/schemas/seeds/0001-notepad/design.md",
					PermalinkURL: "https://registry.autosoftware.app/reg/" + schemaHash,
					ContentHash:  schemaHash,
				},
			})
			return
		}
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
	})

	mux.HandleFunc("GET /v1/registry/lookup", func(w http.ResponseWriter, r *http.Request) {
		hash := r.URL.Query().Get("sha256")
		switch hash {
		case realizationHash:
			json.NewEncoder(w).Encode(map[string]any{"lookup": HashLookupDetail{
				ContentHash:  hash,
				ResourceKind: "realization",
				CanonicalURL: "https://registry.autosoftware.app/contracts/0001-notepad/a-go-htmx",
				PermalinkURL: "https://registry.autosoftware.app/reg/" + hash,
				RedirectURL:  "https://registry.autosoftware.app/@sha256-" + hash + "/contracts/0001-notepad/a-go-htmx",
			}})
		case commandCreateHash:
			json.NewEncoder(w).Encode(map[string]any{"lookup": HashLookupDetail{
				ContentHash:  hash,
				ResourceKind: "command",
				CanonicalURL: "https://registry.autosoftware.app/actions/0001-notepad/a-go-htmx/notes.create",
				PermalinkURL: "https://registry.autosoftware.app/reg/" + hash,
				RedirectURL:  "https://registry.autosoftware.app/@sha256-" + hash + "/actions/0001-notepad/a-go-htmx/notes.create",
			}})
		case commandUpdateHash:
			json.NewEncoder(w).Encode(map[string]any{"lookup": HashLookupDetail{
				ContentHash:  hash,
				ResourceKind: "command",
				CanonicalURL: "https://registry.autosoftware.app/actions/0001-notepad/a-go-htmx/notes.update",
				PermalinkURL: "https://registry.autosoftware.app/reg/" + hash,
				RedirectURL:  "https://registry.autosoftware.app/@sha256-" + hash + "/actions/0001-notepad/a-go-htmx/notes.update",
			}})
		case projectionHash:
			json.NewEncoder(w).Encode(map[string]any{"lookup": HashLookupDetail{
				ContentHash:  hash,
				ResourceKind: "projection",
				CanonicalURL: "https://registry.autosoftware.app/read-models/0001-notepad/a-go-htmx/notes.room",
				PermalinkURL: "https://registry.autosoftware.app/reg/" + hash,
				RedirectURL:  "https://registry.autosoftware.app/@sha256-" + hash + "/read-models/0001-notepad/a-go-htmx/notes.room",
			}})
		case objectHash:
			json.NewEncoder(w).Encode(map[string]any{"lookup": HashLookupDetail{
				ContentHash:  hash,
				ResourceKind: "object",
				CanonicalURL: "https://registry.autosoftware.app/objects/0001-notepad/shared_note",
				PermalinkURL: "https://registry.autosoftware.app/reg/" + hash,
				RedirectURL:  "https://registry.autosoftware.app/@sha256-" + hash + "/objects/0001-notepad/shared_note",
			}})
		case schemaHash:
			json.NewEncoder(w).Encode(map[string]any{"lookup": HashLookupDetail{
				ContentHash:  hash,
				ResourceKind: "schema",
				CanonicalURL: "https://registry.autosoftware.app/schemas/seeds/0001-notepad/design.md",
				PermalinkURL: "https://registry.autosoftware.app/reg/" + hash,
				RedirectURL:  "https://registry.autosoftware.app/@sha256-" + hash + "/schemas/seeds/0001-notepad/design.md",
			}})
		default:
			http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		}
	})

	return httptest.NewServer(mux)
}

func newTestApp(registryURL string) *App {
	app := &App{registry: NewRegistryClient(registryURL)}
	app.loadTemplates()
	return app
}

func newTestMux(app *App) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", app.handleHealthz)
	mux.HandleFunc("GET /{$}", app.handleHome)
	mux.HandleFunc("GET /systems", app.handleSystems)
	mux.HandleFunc("GET /systems/", app.handleSystemDetail)
	mux.HandleFunc("GET /registry-internals", app.handleRegistryInternals)
	mux.HandleFunc("GET /contracts", app.handleRealizations)
	mux.HandleFunc("GET /contracts/", app.handleRealizationDetail)
	mux.HandleFunc("GET /actions", app.handleCommands)
	mux.HandleFunc("GET /actions/", app.handleCommandDetail)
	mux.HandleFunc("GET /read-models", app.handleProjections)
	mux.HandleFunc("GET /read-models/", app.handleProjectionDetail)
	mux.HandleFunc("GET /realizations", app.handleRealizations)
	mux.HandleFunc("GET /realizations/", app.handleRealizationDetail)
	mux.HandleFunc("GET /commands", app.handleCommands)
	mux.HandleFunc("GET /commands/", app.handleCommandDetail)
	mux.HandleFunc("GET /projections", app.handleProjections)
	mux.HandleFunc("GET /projections/", app.handleProjectionDetail)
	mux.HandleFunc("GET /objects", app.handleObjects)
	mux.HandleFunc("GET /objects/", app.handleObjectDetail)
	mux.HandleFunc("GET /schemas", app.handleSchemas)
	mux.HandleFunc("GET /schemas/", app.handleSchemaDetail)
	mux.HandleFunc("GET /schemas/detail", app.handleSchemaDetail)
	mux.HandleFunc("GET /reg/", app.handlePermalinkLookup)
	mux.HandleFunc("GET /assets/style.css", app.handleStyleCSS)
	return app.permalinkMiddleware(mux)
}

// --- Read-only enforcement ---

func TestNormalizeRegistryBaseURL(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "base host", in: "https://autosoftware.app", want: "https://autosoftware.app"},
		{name: "registry prefix", in: "https://autosoftware.app/v1/registry", want: "https://autosoftware.app"},
		{name: "catalog route", in: "https://autosoftware.app/v1/registry/catalog", want: "https://autosoftware.app"},
		{name: "trailing slash", in: "https://autosoftware.app/", want: "https://autosoftware.app"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeRegistryBaseURL(tt.in); got != tt.want {
				t.Fatalf("normalizeRegistryBaseURL(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestCanonicalInstanceKindUsesCatalogKindsWithoutOvermatching(t *testing.T) {
	knownKinds := []string{"show_class", "award_definition", "organization"}

	if got := canonicalInstanceKind("class", knownKinds); got != "show_class" {
		t.Fatalf("canonicalInstanceKind(class) = %q, want %q", got, "show_class")
	}
	if got := canonicalInstanceKind("award", knownKinds); got != "award_definition" {
		t.Fatalf("canonicalInstanceKind(award) = %q, want %q", got, "award_definition")
	}
	if got := canonicalInstanceKind("organization_invite", knownKinds); got != "organization_invite" {
		t.Fatalf("canonicalInstanceKind(organization_invite) = %q, want unchanged", got)
	}
}

func TestHealthzReturnsOK(t *testing.T) {
	mock := newMockRegistry()
	defer mock.Close()
	app := newTestApp(mock.URL)
	mux := newTestMux(app)

	req := httptest.NewRequest("GET", "/healthz", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("healthz returned %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"status":"ok"`) {
		t.Fatalf("healthz body = %q, want status ok", rec.Body.String())
	}
}

func TestNoMutationRoutes(t *testing.T) {
	mock := newMockRegistry()
	defer mock.Close()
	app := newTestApp(mock.URL)
	mux := newTestMux(app)

	methods := []string{"POST", "PUT", "DELETE", "PATCH"}
	paths := []string{
		"/", "/systems", "/systems/test", "/registry-internals",
		"/contracts", "/contracts/test", "/realizations", "/realizations/test",
		"/actions", "/actions/ref/name", "/commands", "/commands/ref/name",
		"/read-models", "/read-models/ref/name", "/projections", "/projections/ref/name",
		"/objects", "/objects/seed/kind", "/objects/seed/kind/instances/object-id",
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

func TestSystemsPageRendersSystems(t *testing.T) {
	mock := newMockRegistry()
	defer mock.Close()
	app := newTestApp(mock.URL)
	mux := newTestMux(app)

	req := httptest.NewRequest("GET", "/systems", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("systems returned %d", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "Notepad") {
		t.Error("systems page missing humanized system title")
	}
	if !strings.Contains(body, "/systems/0001-notepad") {
		t.Error("systems page missing system detail link")
	}
}

func TestSystemDetailPageRendersGroupedResources(t *testing.T) {
	mock := newMockRegistry()
	defer mock.Close()
	app := newTestApp(mock.URL)
	mux := newTestMux(app)

	req := httptest.NewRequest("GET", "/systems/0001-notepad", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("system detail returned %d", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "Shared notepad") {
		t.Error("system detail missing summary")
	}
	if !strings.Contains(body, "notes.create") {
		t.Error("system detail missing action link")
	}
	if !strings.Contains(body, "notes.room") {
		t.Error("system detail missing read model link")
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
	if !strings.Contains(body, "a-go-htmx") {
		t.Error("realizations page missing notepad realization")
	}
	if !strings.Contains(body, "a-web-mvp") {
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
	if !strings.Contains(body, "a-go-htmx") {
		t.Error("search should include matching notepad realization")
	}
	if strings.Contains(body, "0004-events") {
		t.Error("search should not include non-matching events realization")
	}
}

func TestRealizationDetailPage(t *testing.T) {
	mock := newMockRegistry()
	defer mock.Close()
	app := newTestApp(mock.URL)
	mux := newTestMux(app)

	req := httptest.NewRequest("GET", "/realizations/0001-notepad/a-go-htmx", nil)
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
	if !strings.Contains(body, "note_edited_by") || !strings.Contains(body, "Graph Relations") {
		t.Error("detail page missing graph relation section")
	}
}

func TestRealizationDetailPageAcceptsEncodedReferencePath(t *testing.T) {
	mock := newMockRegistry()
	defer mock.Close()
	app := newTestApp(mock.URL)
	mux := newTestMux(app)

	req := httptest.NewRequest("GET", "/realizations/0001-notepad%2Fa-go-htmx", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("encoded realization detail returned %d", rec.Code)
	}
}

func TestRealizationDetailPageAcceptsTrailingSlash(t *testing.T) {
	mock := newMockRegistry()
	defer mock.Close()
	app := newTestApp(mock.URL)
	mux := newTestMux(app)

	req := httptest.NewRequest("GET", "/realizations/0001-notepad/a-go-htmx/", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("trailing-slash realization detail returned %d", rec.Code)
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

	req := httptest.NewRequest("GET", "/commands/0001-notepad/a-go-htmx/notes.create", nil)
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

func TestCommandDetailPageAcceptsEncodedReferencePath(t *testing.T) {
	mock := newMockRegistry()
	defer mock.Close()
	app := newTestApp(mock.URL)
	mux := newTestMux(app)

	req := httptest.NewRequest("GET", "/commands/0001-notepad%2Fa-go-htmx/notes.create", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("encoded command detail returned %d", rec.Code)
	}
}

func TestCommandDetailPageAcceptsTrailingSlash(t *testing.T) {
	mock := newMockRegistry()
	defer mock.Close()
	app := newTestApp(mock.URL)
	mux := newTestMux(app)

	req := httptest.NewRequest("GET", "/commands/0001-notepad/a-go-htmx/notes.create/", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("trailing-slash command detail returned %d", rec.Code)
	}
}

func TestCommandDetailPermalinkPage(t *testing.T) {
	mock := newMockRegistry()
	defer mock.Close()
	app := newTestApp(mock.URL)
	mux := newTestMux(app)

	hash := strings.Repeat("2", 64)
	req := httptest.NewRequest("GET", "/@sha256-"+hash+"/commands/0001-notepad/a-go-htmx/notes.create", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("command permalink returned %d", rec.Code)
	}
	if got := rec.Header().Get("ETag"); got != `"sha256-`+hash+`"` {
		t.Fatalf("unexpected ETag %q", got)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `<link rel="canonical" href="https://registry.autosoftware.app/actions/0001-notepad/a-go-htmx/notes.create">`) {
		t.Fatal("command permalink page missing canonical link tag")
	}
	if !strings.Contains(body, "/@sha256-"+hash+"/actions/0001-notepad/a-go-htmx/notes.create") {
		if !strings.Contains(body, "/reg/"+hash) {
			t.Fatal("command permalink page missing rendered short permalink")
		}
	}
}

func TestRegistryShortPermalinkRedirect(t *testing.T) {
	mock := newMockRegistry()
	defer mock.Close()
	app := newTestApp(mock.URL)
	mux := newTestMux(app)

	hash := strings.Repeat("2", 64)
	req := httptest.NewRequest("GET", "/reg/"+hash, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("short permalink returned %d", rec.Code)
	}
	if got, want := rec.Header().Get("Location"), "https://registry.autosoftware.app/@sha256-"+hash+"/actions/0001-notepad/a-go-htmx/notes.create"; got != want {
		t.Fatalf("short permalink redirect = %q, want %q", got, want)
	}
}

func TestCanonicalHostRedirectsLegacyMountRequests(t *testing.T) {
	mock := newMockRegistry()
	defer mock.Close()
	app := newTestApp(mock.URL)
	mux := newTestMux(app)

	req := httptest.NewRequest("GET", "http://autosoftware.app/systems?q=registry", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("legacy host request returned %d", rec.Code)
	}
	if got, want := rec.Header().Get("Location"), "https://registry.autosoftware.app/systems?q=registry"; got != want {
		t.Fatalf("legacy host redirect = %q, want %q", got, want)
	}
}

func TestCommandDetailPermalinkMismatchReturnsNotFound(t *testing.T) {
	mock := newMockRegistry()
	defer mock.Close()
	app := newTestApp(mock.URL)
	mux := newTestMux(app)

	badHash := strings.Repeat("9", 64)
	req := httptest.NewRequest("GET", "/@sha256-"+badHash+"/commands/0001-notepad/a-go-htmx/notes.create", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for mismatched command permalink, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "does not match the current accepted registry state") {
		t.Fatal("mismatched command permalink should explain the hash mismatch")
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

	req := httptest.NewRequest("GET", "/projections/0001-notepad/a-go-htmx/notes.room", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("projection detail returned %d", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "notes.room") {
		t.Error("projection detail missing name")
	}
	if !strings.Contains(body, "anonymous, session") {
		t.Error("projection detail missing auth modes")
	}
	if !strings.Contains(body, "materialized") {
		t.Error("projection detail missing freshness")
	}
	if !strings.Contains(body, "Visible Data By Auth Mode") || !strings.Contains(body, "Shared Metadata") {
		t.Error("projection detail missing data-view breakdown")
	}
}

func TestProjectionDetailPageAcceptsEncodedReferencePath(t *testing.T) {
	mock := newMockRegistry()
	defer mock.Close()
	app := newTestApp(mock.URL)
	mux := newTestMux(app)

	req := httptest.NewRequest("GET", "/projections/0001-notepad%2Fa-go-htmx/notes.room", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("encoded projection detail returned %d", rec.Code)
	}
}

func TestProjectionDetailPageAcceptsTrailingSlash(t *testing.T) {
	mock := newMockRegistry()
	defer mock.Close()
	app := newTestApp(mock.URL)
	mux := newTestMux(app)

	req := httptest.NewRequest("GET", "/projections/0001-notepad/a-go-htmx/notes.room/", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("trailing-slash projection detail returned %d", rec.Code)
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
	if !strings.Contains(body, "/contracts/0001-notepad/a-go-htmx") {
		t.Error("object detail missing contract link")
	}
	if !strings.Contains(body, "Data Layout") || !strings.Contains(body, "note_id") || !strings.Contains(body, "Private Payload") {
		t.Error("object detail missing grouped data layout")
	}
	if !strings.Contains(body, "Objects This Object Points To") || !strings.Contains(body, "note_edited_by") || !strings.Contains(body, "Objects That Point Here") {
		t.Error("object detail missing graph relation sections")
	}
	if !strings.Contains(body, "Raw Instances") || !strings.Contains(body, "note-1") || !strings.Contains(body, "/objects/0001-notepad/shared_note/instances/note-1") {
		t.Error("object detail missing raw instance browsing")
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

func TestObjectInstanceDetailPage(t *testing.T) {
	mock := newMockRegistry()
	defer mock.Close()
	app := newTestApp(mock.URL)
	mux := newTestMux(app)

	req := httptest.NewRequest("GET", "/objects/0001-notepad/shared_note/instances/note-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("instance detail returned %d", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "Hello World Revised") {
		t.Error("instance detail missing latest claim payload")
	}
	if !strings.Contains(body, "claim-note-update") {
		t.Error("instance detail missing claim id")
	}
	if !strings.Contains(body, "/objects/0001-notepad/actor/instances/actor-1") {
		t.Error("instance detail missing related instance link")
	}
}

func TestObjectInstanceDetailAliasRedirectsToCanonicalKind(t *testing.T) {
	mock := newMockRegistry()
	defer mock.Close()
	app := newTestApp(mock.URL)
	mux := newTestMux(app)

	req := httptest.NewRequest("GET", "/objects/0001-notepad/note/instances/note-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("alias instance detail returned %d", rec.Code)
	}
	if location := rec.Header().Get("Location"); location != "/objects/0001-notepad/shared_note/instances/note-1" {
		t.Fatalf("alias instance detail redirected to %q", location)
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
	if !strings.Contains(body, `/schemas/seeds%2F0001-notepad%2Fdesign.md`) {
		t.Error("schemas page missing singly-escaped schema detail link")
	}
	if strings.Contains(body, `%252F`) {
		t.Error("schemas page should not double-escape schema refs")
	}
}

func TestSchemaDetailPage(t *testing.T) {
	mock := newMockRegistry()
	defer mock.Close()
	app := newTestApp(mock.URL)
	mux := newTestMux(app)

	req := httptest.NewRequest("GET", "/schemas/seeds%2F0001-notepad%2Fdesign.md", nil)
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
	if !strings.Contains(body, `href="/schemas/seeds%2F0001-notepad%2Fdesign.md"`) {
		t.Error("schema detail missing singly-escaped canonical link")
	}
	if !strings.Contains(body, `https://github.com/splashkes/autosoftware/blob/main/seeds/0001-notepad/design.md`) {
		t.Error("schema detail missing GitHub source link")
	}
	if !strings.Contains(body, `href="/contracts/0001-notepad/a-go-htmx"`) {
		t.Error("schema detail missing human-readable contract link")
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

func TestBrowseSchemaPath(t *testing.T) {
	got := string(browseSchemaPath("seeds/0001-notepad/design.md"))
	want := "https://registry.autosoftware.app/schemas/seeds/0001-notepad/design.md"
	if got != want {
		t.Fatalf("browseSchemaPath() = %q, want %q", got, want)
	}
}

func TestRepoSourceURLPreservesAnchor(t *testing.T) {
	got := string(repoSourceURL("genesis/design.md#initial-objects-and-claims"))
	want := "https://github.com/splashkes/autosoftware/blob/main/genesis/design.md#initial-objects-and-claims"
	if got != want {
		t.Fatalf("repoSourceURL() = %q, want %q", got, want)
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
