package jsontransport

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRegistryAPIListsCatalogObservabilityRoutes(t *testing.T) {
	repoRoot := t.TempDir()
	writeRegistryHTTPRepoFile(t, filepath.Join(repoRoot, "genesis", "README.md"), "# Genesis\n")
	writeRegistryHTTPRepoFile(t, filepath.Join(repoRoot, "kernel", "README.md"), "# Kernel\n")
	writeRegistryHTTPRepoFile(t, filepath.Join(repoRoot, "seeds", "README.md"), "# Seeds\n")

	seedDir := filepath.Join(repoRoot, "seeds", "1234-demo")
	realizationDir := filepath.Join(seedDir, "realizations", "a-test")
	writeRegistryHTTPRepoFile(t, filepath.Join(seedDir, "brief.md"), "# Brief\n")
	writeRegistryHTTPRepoFile(t, filepath.Join(seedDir, "design.md"), "# Design\n")
	writeRegistryHTTPRepoFile(t, filepath.Join(realizationDir, "README.md"), "# Demo\n")
	writeRegistryHTTPRepoFile(t, filepath.Join(realizationDir, "realization.yaml"), ""+
		"realization_id: a-test\n"+
		"seed_id: 1234-demo\n"+
		"approach_id: a-approach\n"+
		"summary: Demo realization.\n"+
		"status: draft\n")
	writeRegistryHTTPRepoFile(t, filepath.Join(realizationDir, "interaction_contract.yaml"), ""+
		"contract_version: v1\n"+
		"surface_kind: interactive\n"+
		"seed_id: 1234-demo\n"+
		"realization_id: a-test\n"+
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
		"consistency:\n"+
		"  write_visibility: read_your_writes\n"+
		"  projection_freshness: materialized\n")

	mux := http.NewServeMux()
	NewRegistryAPI(repoRoot).Register(mux)

	catalogReq := httptest.NewRequest(http.MethodGet, "/v1/registry/catalog", nil)
	catalogRec := httptest.NewRecorder()
	mux.ServeHTTP(catalogRec, catalogReq)
	if catalogRec.Code != http.StatusOK {
		t.Fatalf("catalog status = %d, body = %s", catalogRec.Code, catalogRec.Body.String())
	}

	var catalogPayload struct {
		Summary struct {
			Objects      int `json:"objects"`
			Schemas      int `json:"schemas"`
			Realizations int `json:"realizations"`
		} `json:"summary"`
		Realizations []struct {
			Self         string `json:"self"`
			CanonicalURL string `json:"canonical_url"`
			PermalinkURL string `json:"permalink_url"`
			ContentHash  string `json:"content_hash"`
		} `json:"realizations"`
		Commands []struct {
			Self         string `json:"self"`
			CanonicalURL string `json:"canonical_url"`
			PermalinkURL string `json:"permalink_url"`
			ContentHash  string `json:"content_hash"`
		} `json:"commands"`
		Projections []struct {
			AuthModes    []string `json:"auth_modes"`
			Self         string   `json:"self"`
			CanonicalURL string   `json:"canonical_url"`
			PermalinkURL string   `json:"permalink_url"`
			ContentHash  string   `json:"content_hash"`
		} `json:"projections"`
		Objects []struct {
			Self         string `json:"self"`
			CanonicalURL string `json:"canonical_url"`
			PermalinkURL string `json:"permalink_url"`
			ContentHash  string `json:"content_hash"`
		} `json:"objects"`
	}
	if err := json.Unmarshal(catalogRec.Body.Bytes(), &catalogPayload); err != nil {
		t.Fatalf("unmarshal catalog: %v", err)
	}
	if catalogPayload.Summary.Objects != 1 || catalogPayload.Summary.Schemas != 3 || catalogPayload.Summary.Realizations != 1 {
		t.Fatalf("unexpected summary %+v", catalogPayload.Summary)
	}
	if catalogPayload.Realizations[0].Self != "/v1/registry/realization?reference=1234-demo%2Fa-test" {
		t.Fatalf("unexpected realization self %q", catalogPayload.Realizations[0].Self)
	}
	if catalogPayload.Realizations[0].CanonicalURL != "/contracts/1234-demo/a-test" {
		t.Fatalf("unexpected realization canonical url %q", catalogPayload.Realizations[0].CanonicalURL)
	}
	if !strings.HasPrefix(catalogPayload.Realizations[0].PermalinkURL, "/@sha256-") || !strings.HasSuffix(catalogPayload.Realizations[0].PermalinkURL, catalogPayload.Realizations[0].CanonicalURL) {
		t.Fatalf("unexpected realization permalink %q", catalogPayload.Realizations[0].PermalinkURL)
	}
	if catalogPayload.Realizations[0].ContentHash == "" {
		t.Fatal("expected realization content hash")
	}
	if catalogPayload.Commands[0].Self != "/v1/registry/command?reference=1234-demo%2Fa-test&name=tickets.create" {
		t.Fatalf("unexpected command self %q", catalogPayload.Commands[0].Self)
	}
	if catalogPayload.Commands[0].CanonicalURL != "/actions/1234-demo/a-test/tickets.create" {
		t.Fatalf("unexpected command canonical url %q", catalogPayload.Commands[0].CanonicalURL)
	}
	if catalogPayload.Commands[0].ContentHash == "" {
		t.Fatal("expected command content hash")
	}
	if catalogPayload.Projections[0].Self != "/v1/registry/projection?reference=1234-demo%2Fa-test&name=tickets.detail" {
		t.Fatalf("unexpected projection self %q", catalogPayload.Projections[0].Self)
	}
	if len(catalogPayload.Projections[0].AuthModes) != 1 || catalogPayload.Projections[0].AuthModes[0] != "session" {
		t.Fatalf("unexpected projection auth modes %+v", catalogPayload.Projections[0].AuthModes)
	}
	if catalogPayload.Projections[0].CanonicalURL != "/read-models/1234-demo/a-test/tickets.detail" {
		t.Fatalf("unexpected projection canonical url %q", catalogPayload.Projections[0].CanonicalURL)
	}
	if catalogPayload.Objects[0].Self != "/v1/registry/object?seed_id=1234-demo&kind=ticket" {
		t.Fatalf("unexpected object self %q", catalogPayload.Objects[0].Self)
	}
	if catalogPayload.Objects[0].CanonicalURL != "/objects/1234-demo/ticket" {
		t.Fatalf("unexpected object canonical url %q", catalogPayload.Objects[0].CanonicalURL)
	}

	realizationReq := httptest.NewRequest(http.MethodGet, "/v1/registry/realization?reference=1234-demo%2Fa-test", nil)
	realizationRec := httptest.NewRecorder()
	mux.ServeHTTP(realizationRec, realizationReq)
	if realizationRec.Code != http.StatusOK {
		t.Fatalf("realization status = %d, body = %s", realizationRec.Code, realizationRec.Body.String())
	}

	var realizationPayload struct {
		Realization struct {
			Reference    string `json:"reference"`
			CanonicalURL string `json:"canonical_url"`
			PermalinkURL string `json:"permalink_url"`
			ContentHash  string `json:"content_hash"`
			Commands     []struct {
				Self string `json:"self"`
			} `json:"commands"`
		} `json:"realization"`
	}
	if err := json.Unmarshal(realizationRec.Body.Bytes(), &realizationPayload); err != nil {
		t.Fatalf("unmarshal realization: %v", err)
	}
	if realizationPayload.Realization.Reference != "1234-demo/a-test" {
		t.Fatalf("unexpected realization reference %q", realizationPayload.Realization.Reference)
	}
	if realizationPayload.Realization.Commands[0].Self != "/v1/registry/command?reference=1234-demo%2Fa-test&name=tickets.create" {
		t.Fatalf("unexpected realization command self %q", realizationPayload.Realization.Commands[0].Self)
	}
	if realizationPayload.Realization.CanonicalURL != "/contracts/1234-demo/a-test" {
		t.Fatalf("unexpected realization detail canonical url %q", realizationPayload.Realization.CanonicalURL)
	}
	if realizationPayload.Realization.ContentHash != catalogPayload.Realizations[0].ContentHash {
		t.Fatalf("realization detail hash %q does not match catalog hash %q", realizationPayload.Realization.ContentHash, catalogPayload.Realizations[0].ContentHash)
	}
	if realizationPayload.Realization.PermalinkURL != catalogPayload.Realizations[0].PermalinkURL {
		t.Fatalf("realization detail permalink %q does not match catalog permalink %q", realizationPayload.Realization.PermalinkURL, catalogPayload.Realizations[0].PermalinkURL)
	}

	commandReq := httptest.NewRequest(http.MethodGet, "/v1/registry/command?reference=1234-demo%2Fa-test&name=tickets.create", nil)
	commandRec := httptest.NewRecorder()
	mux.ServeHTTP(commandRec, commandReq)
	if commandRec.Code != http.StatusOK {
		t.Fatalf("command status = %d, body = %s", commandRec.Code, commandRec.Body.String())
	}

	var commandPayload struct {
		Command struct {
			Name           string `json:"name"`
			ProjectionSelf string `json:"projection_self"`
			CanonicalURL   string `json:"canonical_url"`
			PermalinkURL   string `json:"permalink_url"`
			ContentHash    string `json:"content_hash"`
		} `json:"command"`
	}
	if err := json.Unmarshal(commandRec.Body.Bytes(), &commandPayload); err != nil {
		t.Fatalf("unmarshal command: %v", err)
	}
	if commandPayload.Command.Name != "tickets.create" {
		t.Fatalf("unexpected command name %q", commandPayload.Command.Name)
	}
	if commandPayload.Command.ProjectionSelf != "/v1/registry/projection?reference=1234-demo%2Fa-test&name=tickets.detail" {
		t.Fatalf("unexpected projection self %q", commandPayload.Command.ProjectionSelf)
	}
	if commandPayload.Command.CanonicalURL != "/actions/1234-demo/a-test/tickets.create" {
		t.Fatalf("unexpected command detail canonical url %q", commandPayload.Command.CanonicalURL)
	}
	if commandPayload.Command.ContentHash != catalogPayload.Commands[0].ContentHash {
		t.Fatalf("command detail hash %q does not match catalog hash %q", commandPayload.Command.ContentHash, catalogPayload.Commands[0].ContentHash)
	}

	objectReq := httptest.NewRequest(http.MethodGet, "/v1/registry/object?seed_id=1234-demo&kind=ticket", nil)
	objectRec := httptest.NewRecorder()
	mux.ServeHTTP(objectRec, objectReq)
	if objectRec.Code != http.StatusOK {
		t.Fatalf("object status = %d, body = %s", objectRec.Code, objectRec.Body.String())
	}

	var objectPayload struct {
		Object struct {
			Kind         string `json:"kind"`
			CanonicalURL string `json:"canonical_url"`
			PermalinkURL string `json:"permalink_url"`
			ContentHash  string `json:"content_hash"`
			Schemas      []struct {
				Self string `json:"self"`
			} `json:"schemas"`
			Commands []struct {
				Name string `json:"name"`
			} `json:"commands"`
		} `json:"object"`
	}
	if err := json.Unmarshal(objectRec.Body.Bytes(), &objectPayload); err != nil {
		t.Fatalf("unmarshal object: %v", err)
	}
	if objectPayload.Object.Kind != "ticket" {
		t.Fatalf("unexpected object kind %q", objectPayload.Object.Kind)
	}
	if len(objectPayload.Object.Commands) != 1 {
		t.Fatalf("expected 1 command, got %d", len(objectPayload.Object.Commands))
	}
	if objectPayload.Object.Schemas[0].Self != "/v1/registry/schema?ref=seeds%2F1234-demo%2Fdesign.md%23ticket" {
		t.Fatalf("unexpected schema self %q", objectPayload.Object.Schemas[0].Self)
	}
	if objectPayload.Object.CanonicalURL != "/objects/1234-demo/ticket" {
		t.Fatalf("unexpected object detail canonical url %q", objectPayload.Object.CanonicalURL)
	}
	if objectPayload.Object.ContentHash != catalogPayload.Objects[0].ContentHash {
		t.Fatalf("object detail hash %q does not match catalog hash %q", objectPayload.Object.ContentHash, catalogPayload.Objects[0].ContentHash)
	}

	schemaReq := httptest.NewRequest(http.MethodGet, "/v1/registry/schema?ref=seeds%2F1234-demo%2Fdesign.md%23ticket-input", nil)
	schemaRec := httptest.NewRecorder()
	mux.ServeHTTP(schemaRec, schemaReq)
	if schemaRec.Code != http.StatusOK {
		t.Fatalf("schema status = %d, body = %s", schemaRec.Code, schemaRec.Body.String())
	}

	var schemaPayload struct {
		Schema struct {
			Ref           string `json:"ref"`
			CanonicalURL  string `json:"canonical_url"`
			PermalinkURL  string `json:"permalink_url"`
			ContentHash   string `json:"content_hash"`
			CommandInputs []struct {
				Name string `json:"name"`
			} `json:"command_inputs"`
		} `json:"schema"`
	}
	if err := json.Unmarshal(schemaRec.Body.Bytes(), &schemaPayload); err != nil {
		t.Fatalf("unmarshal schema: %v", err)
	}
	if schemaPayload.Schema.Ref != "seeds/1234-demo/design.md#ticket-input" {
		t.Fatalf("unexpected schema ref %q", schemaPayload.Schema.Ref)
	}
	if len(schemaPayload.Schema.CommandInputs) != 1 {
		t.Fatalf("expected 1 input use, got %d", len(schemaPayload.Schema.CommandInputs))
	}
	if schemaPayload.Schema.CanonicalURL != "/schemas/detail?ref=seeds%2F1234-demo%2Fdesign.md%23ticket-input" {
		t.Fatalf("unexpected schema detail canonical url %q", schemaPayload.Schema.CanonicalURL)
	}
	if schemaPayload.Schema.ContentHash == "" {
		t.Fatal("expected schema content hash")
	}
}

func TestRegistryAPIRejectsMutatingMethods(t *testing.T) {
	repoRoot := t.TempDir()
	writeRegistryHTTPRepoFile(t, filepath.Join(repoRoot, "genesis", "README.md"), "# Genesis\n")
	writeRegistryHTTPRepoFile(t, filepath.Join(repoRoot, "kernel", "README.md"), "# Kernel\n")
	writeRegistryHTTPRepoFile(t, filepath.Join(repoRoot, "seeds", "README.md"), "# Seeds\n")

	seedDir := filepath.Join(repoRoot, "seeds", "1234-demo")
	realizationDir := filepath.Join(seedDir, "realizations", "a-test")
	writeRegistryHTTPRepoFile(t, filepath.Join(seedDir, "brief.md"), "# Brief\n")
	writeRegistryHTTPRepoFile(t, filepath.Join(seedDir, "design.md"), "# Design\n")
	writeRegistryHTTPRepoFile(t, filepath.Join(realizationDir, "README.md"), "# Demo\n")
	writeRegistryHTTPRepoFile(t, filepath.Join(realizationDir, "realization.yaml"), ""+
		"realization_id: a-test\n"+
		"seed_id: 1234-demo\n"+
		"approach_id: a-approach\n"+
		"summary: Demo realization.\n"+
		"status: draft\n")
	writeRegistryHTTPRepoFile(t, filepath.Join(realizationDir, "interaction_contract.yaml"), ""+
		"contract_version: v1\n"+
		"surface_kind: interactive\n"+
		"seed_id: 1234-demo\n"+
		"realization_id: a-test\n"+
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
		"consistency:\n"+
		"  write_visibility: read_your_writes\n"+
		"  projection_freshness: materialized\n")

	mux := http.NewServeMux()
	NewRegistryAPI(repoRoot).Register(mux)

	paths := []string{
		"/v1/registry/catalog",
		"/v1/registry/realizations",
		"/v1/registry/realization?reference=1234-demo%2Fa-test",
		"/v1/registry/commands",
		"/v1/registry/command?reference=1234-demo%2Fa-test&name=tickets.create",
		"/v1/registry/projections",
		"/v1/registry/projection?reference=1234-demo%2Fa-test&name=tickets.detail",
		"/v1/registry/objects",
		"/v1/registry/object?seed_id=1234-demo&kind=ticket",
		"/v1/registry/schemas",
		"/v1/registry/schema?ref=seeds%2F1234-demo%2Fdesign.md%23ticket",
	}
	methods := []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete}

	for _, path := range paths {
		for _, method := range methods {
			req := httptest.NewRequest(method, path, strings.NewReader(`{}`))
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)
			if rec.Code != http.StatusMethodNotAllowed {
				t.Fatalf("%s %s status = %d, want %d", method, path, rec.Code, http.StatusMethodNotAllowed)
			}
		}
	}
}

func writeRegistryHTTPRepoFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
