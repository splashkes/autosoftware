package registry

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadCatalogBuildsObjectsAndSchemas(t *testing.T) {
	repoRoot := t.TempDir()
	writeRegistryRepoFile(t, filepath.Join(repoRoot, "genesis", "README.md"), "# Genesis\n")
	writeRegistryRepoFile(t, filepath.Join(repoRoot, "kernel", "README.md"), "# Kernel\n")
	writeRegistryRepoFile(t, filepath.Join(repoRoot, "seeds", "README.md"), "# Seeds\n")

	seedDir := filepath.Join(repoRoot, "seeds", "1234-demo")
	realizationA := filepath.Join(seedDir, "realizations", "a-test")
	realizationB := filepath.Join(seedDir, "realizations", "b-test")

	writeRegistryRepoFile(t, filepath.Join(seedDir, "brief.md"), "# Brief\n")
	writeRegistryRepoFile(t, filepath.Join(seedDir, "design.md"), "# Design\n## ticket\n## ticket-input\n## ticket-result\n")
	writeRegistryRepoFile(t, filepath.Join(realizationA, "README.md"), "# A\n")
	writeRegistryRepoFile(t, filepath.Join(realizationB, "README.md"), "# B\n")

	writeRegistryRepoFile(t, filepath.Join(realizationA, "realization.yaml"), ""+
		"realization_id: a-test\n"+
		"seed_id: 1234-demo\n"+
		"approach_id: a-approach\n"+
		"summary: A realization.\n"+
		"status: draft\n")
	writeRegistryRepoFile(t, filepath.Join(realizationB, "realization.yaml"), ""+
		"realization_id: b-test\n"+
		"seed_id: 1234-demo\n"+
		"approach_id: b-approach\n"+
		"summary: B realization.\n"+
		"status: accepted\n")

	for _, dir := range []string{realizationA, realizationB} {
		writeRegistryRepoFile(t, filepath.Join(dir, "interaction_contract.yaml"), ""+
			"contract_version: v1\n"+
			"surface_kind: interactive\n"+
			"seed_id: 1234-demo\n"+
			"realization_id: "+filepath.Base(dir)+"\n"+
			"summary: Demo contract.\n"+
			"links:\n"+
			"  seed_design: ../../design.md\n"+
			"  seed_brief: ../../brief.md\n"+
			"  realization_readme: README.md\n"+
			"auth_modes:\n"+
			"  - session\n"+
			"capabilities:\n"+
			"  - name: sessions\n"+
			"    summary: Session plumbing.\n"+
			"domain_objects:\n"+
			"  - kind: ticket\n"+
			"    summary: Ticket.\n"+
			"    schema_ref: ../../design.md#ticket\n"+
			"    capabilities:\n"+
			"      - sessions\n"+
			"    data_layout:\n"+
			"      shared_metadata:\n"+
			"        summary: Stable ticket identity.\n"+
			"        fields:\n"+
			"          - name: ticket_id\n"+
			"            type: string\n"+
			"            summary: Stable ticket id.\n"+
			"      public_payload:\n"+
			"        summary: Public ticket content.\n"+
			"        fields:\n"+
			"          - name: subject\n"+
			"            type: string\n"+
			"            summary: Public subject.\n"+
			"domain_relations:\n"+
			"  - kind: ticket_replies_to\n"+
			"    summary: Connects a ticket to the ticket it replies to.\n"+
			"    from_kinds:\n"+
			"      - ticket\n"+
			"    to_kinds:\n"+
			"      - ticket\n"+
			"    cardinality: many_to_one\n"+
			"    visibility: mixed\n"+
			"    schema_ref: ../../design.md#ticket\n"+
			"    capabilities:\n"+
			"      - sessions\n"+
			"    attributes:\n"+
			"      - name: reply_role\n"+
			"        type: string\n"+
			"        summary: Describes the parent-child ticket relation.\n"+
			"commands:\n"+
			"  - name: tickets.create\n"+
			"    summary: Create ticket.\n"+
			"    path: /v1/commands/1234-demo/tickets.create\n"+
			"    object_kinds:\n"+
			"      - ticket\n"+
			"    auth_modes:\n"+
			"      - session\n"+
			"    idempotency: required\n"+
			"    input_schema_ref: ../../design.md#ticket-input\n"+
			"    result_schema_ref: ../../design.md#ticket-result\n"+
			"    capabilities:\n"+
			"      - sessions\n"+
			"    projection: tickets.detail\n"+
			"    consistency: read_your_writes\n"+
			"projections:\n"+
			"  - name: tickets.detail\n"+
			"    summary: Ticket detail.\n"+
			"    path: /v1/projections/1234-demo/tickets/{ticket_id}\n"+
			"    object_kinds:\n"+
			"      - ticket\n"+
			"    auth_modes:\n"+
			"      - session\n"+
			"    capabilities:\n"+
			"      - sessions\n"+
			"    freshness: materialized\n"+
			"    data_views:\n"+
			"      - auth_modes:\n"+
			"          - session\n"+
			"        sections:\n"+
			"          - shared_metadata\n"+
			"          - public_payload\n"+
			"        summary: Session callers get the ticket payload.\n"+
			"consistency:\n"+
			"  write_visibility: read_your_writes\n"+
			"  projection_freshness: materialized\n")
	}

	catalog, err := LoadCatalog(repoRoot)
	if err != nil {
		t.Fatalf("LoadCatalog() error = %v", err)
	}

	if catalog.Summary.Realizations != 2 {
		t.Fatalf("expected 2 realizations, got %d", catalog.Summary.Realizations)
	}
	if catalog.Summary.Objects != 1 {
		t.Fatalf("expected 1 object, got %d", catalog.Summary.Objects)
	}
	if catalog.Summary.Relations != 2 {
		t.Fatalf("expected 2 relations, got %d", catalog.Summary.Relations)
	}
	if catalog.Summary.Schemas != 3 {
		t.Fatalf("expected 3 schemas, got %d", catalog.Summary.Schemas)
	}
	if len(catalog.Realizations) != 2 {
		t.Fatalf("expected 2 catalog realizations, got %d", len(catalog.Realizations))
	}
	if len(catalog.Commands) != 2 {
		t.Fatalf("expected 2 catalog commands, got %d", len(catalog.Commands))
	}
	if len(catalog.Projections) != 2 {
		t.Fatalf("expected 2 catalog projections, got %d", len(catalog.Projections))
	}

	object, ok := GetObject(catalog, "1234-demo", "ticket")
	if !ok {
		t.Fatal("expected ticket object")
	}
	if len(object.Realizations) != 2 {
		t.Fatalf("expected 2 object realizations, got %d", len(object.Realizations))
	}
	if len(object.Commands) != 2 {
		t.Fatalf("expected 2 command uses, got %d", len(object.Commands))
	}
	if len(object.Projections) != 2 {
		t.Fatalf("expected 2 projection uses, got %d", len(object.Projections))
	}
	if len(object.OutgoingRelations) != 2 || len(object.IncomingRelations) != 2 {
		t.Fatalf("expected relation graph on object, got outgoing=%d incoming=%d", len(object.OutgoingRelations), len(object.IncomingRelations))
	}
	if object.OutgoingRelations[0].Kind != "ticket_replies_to" || object.OutgoingRelations[0].Visibility != "mixed" {
		t.Fatalf("unexpected outgoing relation %+v", object.OutgoingRelations[0])
	}
	if len(object.Projections[0].AuthModes) != 1 || object.Projections[0].AuthModes[0] != "session" {
		t.Fatalf("unexpected projection auth modes %+v", object.Projections[0].AuthModes)
	}
	if !contains(object.SchemaRefs, "seeds/1234-demo/design.md#ticket") {
		t.Fatalf("expected canonical schema ref, got %+v", object.SchemaRefs)
	}
	if got := object.DataLayout.SharedMetadata.Fields[0].Name; got != "ticket_id" {
		t.Fatalf("expected shared metadata field ticket_id, got %q", got)
	}
	if len(object.Projections[0].DataViews) != 1 || object.Projections[0].DataViews[0].Sections[1] != "public_payload" {
		t.Fatalf("unexpected projection data views %+v", object.Projections[0].DataViews)
	}

	schema, ok := GetSchema(catalog, "seeds/1234-demo/design.md#ticket-input")
	if !ok {
		t.Fatal("expected input schema")
	}
	if len(schema.CommandInputs) != 2 {
		t.Fatalf("expected 2 command input uses, got %d", len(schema.CommandInputs))
	}
	realization, ok := GetRealization(catalog, "1234-demo/a-test")
	if !ok {
		t.Fatal("expected realization detail")
	}
	if len(realization.Relations) != 1 || realization.Relations[0].Kind != "ticket_replies_to" {
		t.Fatalf("unexpected realization relations %+v", realization.Relations)
	}
	if len(realization.CommandNames) != 1 || realization.CommandNames[0] != "tickets.create" {
		t.Fatalf("unexpected realization commands %+v", realization.CommandNames)
	}
	command, ok := GetCommand(catalog, "1234-demo/a-test", "tickets.create")
	if !ok || command.InputSchemaRef != "seeds/1234-demo/design.md#ticket-input" {
		t.Fatalf("unexpected command %+v", command)
	}
}

func TestFilterHelpers(t *testing.T) {
	catalog := Catalog{
		Objects: []CatalogObject{{
			SeedID:     "1234-demo",
			Kind:       "ticket",
			Summary:    "Support ticket",
			SchemaRefs: []string{"seeds/1234-demo/design.md#ticket"},
			OutgoingRelations: []CatalogRelation{{
				Kind:        "ticket_replies_to",
				Summary:     "Reply chain edge",
				Cardinality: "many_to_one",
				Visibility:  "mixed",
			}},
		}},
		Schemas: []CatalogSchema{{
			Ref:  "seeds/1234-demo/design.md#ticket",
			Path: "seeds/1234-demo/design.md",
			ObjectUses: []CatalogSchemaObjectUse{{
				SeedID: "1234-demo",
				Kind:   "ticket",
			}},
		}},
	}

	if got := FilterObjects(catalog.Objects, "1234-demo", "", "support"); len(got) != 1 {
		t.Fatalf("expected object query match, got %d", len(got))
	}
	if got := FilterObjects(catalog.Objects, "1234-demo", "", "reply"); len(got) != 1 {
		t.Fatalf("expected relation-backed object query match, got %d", len(got))
	}
	if got := FilterSchemas(catalog.Schemas, "1234-demo", "ticket"); len(got) != 1 {
		t.Fatalf("expected schema query match, got %d", len(got))
	}
	if got := FilterRealizations([]CatalogRealization{{
		Reference:     "1234-demo/a-test",
		SeedID:        "1234-demo",
		RealizationID: "a-test",
		Summary:       "Support ticket UI",
		ObjectKinds:   []string{"ticket"},
	}}, "1234-demo", "ticket"); len(got) != 1 {
		t.Fatalf("expected realization query match, got %d", len(got))
	}
}

func TestCatalogReaderReturnsNotFoundErrors(t *testing.T) {
	repoRoot := t.TempDir()
	writeRegistryRepoFile(t, filepath.Join(repoRoot, "genesis", "README.md"), "# Genesis\n")
	writeRegistryRepoFile(t, filepath.Join(repoRoot, "kernel", "README.md"), "# Kernel\n")
	writeRegistryRepoFile(t, filepath.Join(repoRoot, "seeds", "README.md"), "# Seeds\n")

	reader := NewCatalogReader(repoRoot)
	if _, err := reader.GetObject("missing-seed", "missing-kind"); !errors.Is(err, ErrCatalogObjectNotFound) {
		t.Fatalf("expected ErrCatalogObjectNotFound, got %v", err)
	}
	if _, err := reader.GetSchema("missing-ref"); !errors.Is(err, ErrCatalogSchemaNotFound) {
		t.Fatalf("expected ErrCatalogSchemaNotFound, got %v", err)
	}
	if _, err := reader.GetRealization("missing-reference"); !errors.Is(err, ErrCatalogRealizationNotFound) {
		t.Fatalf("expected ErrCatalogRealizationNotFound, got %v", err)
	}
	if _, err := reader.GetCommand("missing-reference", "missing-command"); !errors.Is(err, ErrCatalogCommandNotFound) {
		t.Fatalf("expected ErrCatalogCommandNotFound, got %v", err)
	}
	if _, err := reader.GetProjection("missing-reference", "missing-projection"); !errors.Is(err, ErrCatalogProjectionNotFound) {
		t.Fatalf("expected ErrCatalogProjectionNotFound, got %v", err)
	}
}

func writeRegistryRepoFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
