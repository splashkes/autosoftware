package jsontransport

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestContractsAPIListsAndReturnsContracts(t *testing.T) {
	repoRoot := t.TempDir()
	writeHTTPRepoFile(t, filepath.Join(repoRoot, "genesis", "README.md"), "# Genesis\n")
	writeHTTPRepoFile(t, filepath.Join(repoRoot, "kernel", "README.md"), "# Kernel\n")
	writeHTTPRepoFile(t, filepath.Join(repoRoot, "seeds", "README.md"), "# Seeds\n")

	realizationDir := filepath.Join(repoRoot, "seeds", "1234-demo", "realizations", "a-test")
	writeHTTPRepoFile(t, filepath.Join(repoRoot, "seeds", "1234-demo", "brief.md"), "# Brief\n")
	writeHTTPRepoFile(t, filepath.Join(repoRoot, "seeds", "1234-demo", "design.md"), "# Design\n")
	writeHTTPRepoFile(t, filepath.Join(realizationDir, "README.md"), "# Demo\n")
	writeHTTPRepoFile(t, filepath.Join(realizationDir, "realization.yaml"), ""+
		"realization_id: a-test\n"+
		"seed_id: 1234-demo\n"+
		"approach_id: a-approach\n"+
		"summary: Demo realization.\n"+
		"status: draft\n")
	writeHTTPRepoFile(t, filepath.Join(realizationDir, "interaction_contract.yaml"), ""+
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
		"  - kind: demo_item\n"+
		"    summary: Demo item.\n"+
		"    schema_ref: ../../design.md#demo-item\n"+
		"    capabilities:\n"+
		"      - sessions\n"+
		"commands:\n"+
		"  - name: demo_items.create\n"+
		"    summary: Create a demo item.\n"+
		"    path: /v1/commands/1234-demo/demo_items.create\n"+
		"    object_kinds:\n"+
		"      - demo_item\n"+
		"    auth_modes:\n"+
		"      - session\n"+
		"    idempotency: required\n"+
		"    input_schema_ref: ../../design.md#demo-input\n"+
		"    result_schema_ref: README.md#demo\n"+
		"    capabilities:\n"+
		"      - sessions\n"+
		"    projection: demo_items.detail\n"+
		"    consistency: read_your_writes\n"+
		"projections:\n"+
		"  - name: demo_items.detail\n"+
		"    summary: Demo projection.\n"+
		"    path: /v1/projections/1234-demo/demo-items/{item_id}\n"+
		"    object_kinds:\n"+
		"      - demo_item\n"+
		"    auth_modes:\n"+
		"      - session\n"+
		"    capabilities:\n"+
		"      - sessions\n"+
		"    freshness: materialized\n"+
		"consistency:\n"+
		"  write_visibility: read_your_writes\n"+
		"  projection_freshness: materialized\n")

	mux := http.NewServeMux()
	NewContractsAPI(repoRoot).Register(mux)

	listReq := httptest.NewRequest(http.MethodGet, "/v1/contracts", nil)
	listRec := httptest.NewRecorder()
	mux.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list status = %d, body = %s", listRec.Code, listRec.Body.String())
	}

	var listPayload struct {
		Contracts []struct {
			Reference string `json:"reference"`
			Self      string `json:"self"`
		} `json:"contracts"`
	}
	if err := json.Unmarshal(listRec.Body.Bytes(), &listPayload); err != nil {
		t.Fatalf("unmarshal list response: %v", err)
	}
	if len(listPayload.Contracts) != 1 {
		t.Fatalf("expected 1 contract, got %d", len(listPayload.Contracts))
	}
	if listPayload.Contracts[0].Self != "/v1/contracts/1234-demo/a-test" {
		t.Fatalf("unexpected self path %q", listPayload.Contracts[0].Self)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/v1/contracts/1234-demo/a-test", nil)
	getRec := httptest.NewRecorder()
	mux.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("get status = %d, body = %s", getRec.Code, getRec.Body.String())
	}

	var getPayload struct {
		Reference string `json:"reference"`
		Contract  struct {
			SurfaceKind string `json:"surface_kind"`
		} `json:"contract"`
	}
	if err := json.Unmarshal(getRec.Body.Bytes(), &getPayload); err != nil {
		t.Fatalf("unmarshal get response: %v", err)
	}
	if getPayload.Reference != "1234-demo/a-test" {
		t.Fatalf("unexpected reference %q", getPayload.Reference)
	}
	if getPayload.Contract.SurfaceKind != "interactive" {
		t.Fatalf("unexpected surface kind %q", getPayload.Contract.SurfaceKind)
	}
}

func writeHTTPRepoFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
