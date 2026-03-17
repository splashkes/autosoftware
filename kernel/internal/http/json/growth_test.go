package jsontransport

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"as/kernel/internal/realizations"
)

func TestGrowthAPISeedPacketEndpoint(t *testing.T) {
	repoRoot := setupGrowthAPITestRepo(t)
	api := NewGrowthAPI(repoRoot, nil)

	mux := http.NewServeMux()
	api.Register(mux)

	req := httptest.NewRequest(http.MethodGet, "/v1/projections/realization-growth/seed-packet?reference=1234-demo/a-test", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", rec.Code, rec.Body.String())
	}

	var payload struct {
		Packet struct {
			Reference string `json:"reference"`
			Readiness struct {
				Stage string `json:"stage"`
			} `json:"readiness"`
		} `json:"packet"`
		Operations []struct {
			ID string `json:"id"`
		} `json:"operations"`
		Profiles []struct {
			ID string `json:"id"`
		} `json:"profiles"`
		Targets []struct {
			ID string `json:"id"`
		} `json:"targets"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if payload.Packet.Reference != "1234-demo/a-test" {
		t.Fatalf("expected reference %q, got %q", "1234-demo/a-test", payload.Packet.Reference)
	}
	if payload.Packet.Readiness.Stage != "defined" {
		t.Fatalf("expected defined stage, got %q", payload.Packet.Readiness.Stage)
	}
	if len(payload.Operations) != 3 {
		t.Fatalf("expected 3 operations, got %d", len(payload.Operations))
	}
	if len(payload.Profiles) != 4 {
		t.Fatalf("expected 4 profiles, got %d", len(payload.Profiles))
	}
	if len(payload.Targets) != 4 {
		t.Fatalf("expected 4 targets, got %d", len(payload.Targets))
	}
}

func TestGrowthAPIHandleGrowRequiresRuntimeService(t *testing.T) {
	repoRoot := setupGrowthAPITestRepo(t)
	api := NewGrowthAPI(repoRoot, nil)

	req := httptest.NewRequest(http.MethodPost, "/v1/commands/realizations.grow", strings.NewReader(`{"reference":"1234-demo/a-test"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	api.handleGrow(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d with body %s", rec.Code, rec.Body.String())
	}
}

func TestTargetReferenceForGrowthSupportsNewVariants(t *testing.T) {
	packet := realizations.GrowthContext{
		Reference: "1234-demo/a-test",
		SeedID:    "1234-demo",
	}

	got, err := targetReferenceForGrowth(packet, GrowthCommandInput{
		CreateNew:        true,
		NewRealizationID: "b-ornate-pass",
	})
	if err != nil {
		t.Fatalf("targetReferenceForGrowth() error = %v", err)
	}
	if got != "1234-demo/b-ornate-pass" {
		t.Fatalf("expected target reference %q, got %q", "1234-demo/b-ornate-pass", got)
	}
}

func TestBuildGrowthPromptSortsSourceFiles(t *testing.T) {
	packet := realizations.GrowthContext{
		Reference:   "1234-demo/a-test",
		SeedID:      "1234-demo",
		SeedSummary: "Demo seed.",
		Readiness: realizations.ReadinessInfo{
			Label: "Defined",
		},
		SeedDocs: []realizations.ContextFile{
			{Path: "seeds/1234-demo/design.md"},
			{Path: "seeds/1234-demo/brief.md"},
		},
		RealizationDocs: []realizations.ContextFile{
			{Path: "seeds/1234-demo/realizations/a-test/realization.yaml"},
		},
	}

	prompt := buildGrowthPrompt(packet, "1234-demo/a-test", "grow", "minimal", "runnable_mvp", "Stay small.")
	if !strings.Contains(prompt, "developer_instructions: Stay small.") {
		t.Fatalf("expected developer instructions in prompt, got %q", prompt)
	}
	if !strings.Contains(prompt, "source_files: seeds/1234-demo/brief.md, seeds/1234-demo/design.md, seeds/1234-demo/realizations/a-test/realization.yaml") {
		t.Fatalf("expected sorted source files in prompt, got %q", prompt)
	}
}

func setupGrowthAPITestRepo(t *testing.T) string {
	t.Helper()

	repoRoot := t.TempDir()
	writeGrowthRepoFile(t, filepath.Join(repoRoot, "genesis", "README.md"), "# Genesis\n")
	writeGrowthRepoFile(t, filepath.Join(repoRoot, "kernel", "README.md"), "# Kernel\n")
	writeGrowthRepoFile(t, filepath.Join(repoRoot, "seeds", "README.md"), "# Seeds\n")

	seedDir := filepath.Join(repoRoot, "seeds", "1234-demo")
	writeGrowthRepoFile(t, filepath.Join(seedDir, "README.md"), "# Demo Seed\n")
	writeGrowthRepoFile(t, filepath.Join(seedDir, "brief.md"), "# Brief\n")
	writeGrowthRepoFile(t, filepath.Join(seedDir, "design.md"), "# Design\n")
	writeGrowthRepoFile(t, filepath.Join(seedDir, "acceptance.md"), "# Acceptance\n")
	writeGrowthRepoFile(t, filepath.Join(seedDir, "decision_log.md"), "# Decision Log\n")
	writeGrowthRepoFile(t, filepath.Join(seedDir, "notes.md"), "# Notes\n")
	writeGrowthRepoFile(t, filepath.Join(seedDir, "seed.yaml"), ""+
		"seed_id: 1234-demo\n"+
		"version: 1\n"+
		"summary: Demo growth seed.\n"+
		"status: proposed\n")
	writeGrowthRepoFile(t, filepath.Join(seedDir, "approaches", "README.md"), "# Approaches\n")
	writeGrowthRepoFile(t, filepath.Join(seedDir, "approaches", "a-test.yaml"), ""+
		"approach_id: a-test\n"+
		"summary: Demo approach.\n"+
		"status: active\n")

	realizationDir := filepath.Join(seedDir, "realizations", "a-test")
	writeGrowthRepoFile(t, filepath.Join(realizationDir, "README.md"), "# Realization\n")
	writeGrowthRepoFile(t, filepath.Join(realizationDir, "notes.md"), "# Notes\n")
	writeGrowthRepoFile(t, filepath.Join(realizationDir, "realization.yaml"), ""+
		"realization_id: a-test\n"+
		"seed_id: 1234-demo\n"+
		"approach_id: a-test\n"+
		"summary: Demo realization.\n"+
		"status: draft\n")
	writeGrowthRepoFile(t, filepath.Join(realizationDir, "interaction_contract.yaml"), ""+
		"contract_version: v1\n"+
		"surface_kind: interactive\n"+
		"seed_id: 1234-demo\n"+
		"realization_id: a-test\n"+
		"summary: Demo contract.\n"+
		"links:\n"+
		"  seed_design: ../../design.md\n"+
		"  seed_brief: ../../brief.md\n"+
		"  seed_acceptance: ../../acceptance.md\n"+
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
		"    summary: Demo detail.\n"+
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

	return repoRoot
}

func writeGrowthRepoFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
