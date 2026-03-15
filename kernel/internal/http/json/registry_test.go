package jsontransport

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestRegistryAPIListsCatalogObjectsAndSchemas(t *testing.T) {
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
			Objects int `json:"objects"`
			Schemas int `json:"schemas"`
		} `json:"summary"`
		Objects []struct {
			Self string `json:"self"`
		} `json:"objects"`
	}
	if err := json.Unmarshal(catalogRec.Body.Bytes(), &catalogPayload); err != nil {
		t.Fatalf("unmarshal catalog: %v", err)
	}
	if catalogPayload.Summary.Objects != 1 || catalogPayload.Summary.Schemas != 3 {
		t.Fatalf("unexpected summary %+v", catalogPayload.Summary)
	}
	if catalogPayload.Objects[0].Self != "/v1/registry/object?seed_id=1234-demo&kind=ticket" {
		t.Fatalf("unexpected object self %q", catalogPayload.Objects[0].Self)
	}

	objectReq := httptest.NewRequest(http.MethodGet, "/v1/registry/object?seed_id=1234-demo&kind=ticket", nil)
	objectRec := httptest.NewRecorder()
	mux.ServeHTTP(objectRec, objectReq)
	if objectRec.Code != http.StatusOK {
		t.Fatalf("object status = %d, body = %s", objectRec.Code, objectRec.Body.String())
	}

	var objectPayload struct {
		Object struct {
			Kind    string `json:"kind"`
			Schemas []struct {
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

	schemaReq := httptest.NewRequest(http.MethodGet, "/v1/registry/schema?ref=seeds%2F1234-demo%2Fdesign.md%23ticket-input", nil)
	schemaRec := httptest.NewRecorder()
	mux.ServeHTTP(schemaRec, schemaReq)
	if schemaRec.Code != http.StatusOK {
		t.Fatalf("schema status = %d, body = %s", schemaRec.Code, schemaRec.Body.String())
	}

	var schemaPayload struct {
		Schema struct {
			Ref           string `json:"ref"`
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
