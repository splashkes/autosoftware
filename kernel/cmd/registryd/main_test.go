package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRegistrydRejectsMutatingMethods(t *testing.T) {
	repoRoot := t.TempDir()
	writeRegistrydFile(t, filepath.Join(repoRoot, "genesis", "README.md"), "# Genesis\n")
	writeRegistrydFile(t, filepath.Join(repoRoot, "kernel", "README.md"), "# Kernel\n")
	writeRegistrydFile(t, filepath.Join(repoRoot, "seeds", "README.md"), "# Seeds\n")

	seedDir := filepath.Join(repoRoot, "seeds", "1234-demo")
	realizationDir := filepath.Join(seedDir, "realizations", "a-test")
	writeRegistrydFile(t, filepath.Join(seedDir, "brief.md"), "# Brief\n")
	writeRegistrydFile(t, filepath.Join(seedDir, "design.md"), "# Design\n")
	writeRegistrydFile(t, filepath.Join(realizationDir, "README.md"), "# Demo\n")
	writeRegistrydFile(t, filepath.Join(realizationDir, "realization.yaml"), ""+
		"realization_id: a-test\n"+
		"seed_id: 1234-demo\n"+
		"approach_id: a-approach\n"+
		"summary: Demo realization.\n"+
		"status: draft\n")
	writeRegistrydFile(t, filepath.Join(realizationDir, "interaction_contract.yaml"), ""+
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

	mux := newMux(repoRoot, nil, nil)
	paths := []string{
		"/healthz",
		"/v1/registry/status",
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

func writeRegistrydFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
