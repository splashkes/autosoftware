package realizations

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadInteractionContractValidatesManifestLinkage(t *testing.T) {
	repoRoot := t.TempDir()
	writeRepoFile(t, filepath.Join(repoRoot, "genesis", "README.md"), "# Genesis\n")
	writeRepoFile(t, filepath.Join(repoRoot, "kernel", "README.md"), "# Kernel\n")
	writeRepoFile(t, filepath.Join(repoRoot, "seeds", "README.md"), "# Seeds\n")

	realizationDir := filepath.Join(repoRoot, "seeds", "1234-demo", "realizations", "a-test")
	writeRepoFile(t, filepath.Join(repoRoot, "seeds", "1234-demo", "brief.md"), "# Brief\n")
	writeRepoFile(t, filepath.Join(repoRoot, "seeds", "1234-demo", "design.md"), "# Design\n")
	writeRepoFile(t, filepath.Join(repoRoot, "seeds", "1234-demo", "README.md"), "# Seed\n")
	writeRepoFile(t, filepath.Join(realizationDir, "README.md"), "# Demo\n")
	writeRepoFile(t, filepath.Join(realizationDir, "realization.yaml"), ""+
		"realization_id: a-test\n"+
		"seed_id: 1234-demo\n"+
		"approach_id: a-approach\n"+
		"summary: Demo realization.\n"+
		"status: draft\n")
	writeRepoFile(t, filepath.Join(realizationDir, "interaction_contract.yaml"), ""+
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
		"  - service_token\n"+
		"capabilities:\n"+
		"  - name: sessions\n"+
		"    summary: Session plumbing.\n"+
		"  - name: state_transitions\n"+
		"    summary: State change audit trail.\n"+
		"domain_objects:\n"+
		"  - kind: demo_item\n"+
		"    summary: Demo item.\n"+
		"    schema_ref: ../../design.md#demo-item\n"+
		"    capabilities:\n"+
		"      - sessions\n"+
		"      - state_transitions\n"+
		"commands:\n"+
		"  - name: demo_items.create\n"+
		"    summary: Create a demo item.\n"+
		"    path: /v1/commands/1234-demo/demo_items.create\n"+
		"    object_kinds:\n"+
		"      - demo_item\n"+
		"    auth_modes:\n"+
		"      - session\n"+
		"      - service_token\n"+
		"    idempotency: required\n"+
		"    input_schema_ref: ../../design.md#demo-input\n"+
		"    result_schema_ref: README.md#demo\n"+
		"    capabilities:\n"+
		"      - sessions\n"+
		"      - state_transitions\n"+
		"    projection: demo_items.detail\n"+
		"    consistency: read_your_writes\n"+
		"projections:\n"+
		"  - name: demo_items.detail\n"+
		"    summary: Demo read model.\n"+
		"    path: /v1/projections/1234-demo/demo-items/{item_id}\n"+
		"    object_kinds:\n"+
		"      - demo_item\n"+
		"    auth_modes:\n"+
		"      - session\n"+
		"      - service_token\n"+
		"    capabilities:\n"+
		"      - sessions\n"+
		"      - state_transitions\n"+
		"    freshness: materialized\n"+
		"consistency:\n"+
		"  write_visibility: read_your_writes\n"+
		"  projection_freshness: materialized\n")

	entries, err := Discover(repoRoot)
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 realization, got %d", len(entries))
	}

	loaded, err := LoadInteractionContract(repoRoot, entries[0])
	if err != nil {
		t.Fatalf("LoadInteractionContract() error = %v", err)
	}

	if loaded.Contract.SurfaceKind != "interactive" {
		t.Fatalf("expected interactive surface, got %q", loaded.Contract.SurfaceKind)
	}
	if len(loaded.Contract.Commands) != 1 {
		t.Fatalf("expected 1 command, got %d", len(loaded.Contract.Commands))
	}
}

func TestDiscoverContractsRequiresInteractionContract(t *testing.T) {
	repoRoot := t.TempDir()
	writeRepoFile(t, filepath.Join(repoRoot, "genesis", "README.md"), "# Genesis\n")
	writeRepoFile(t, filepath.Join(repoRoot, "kernel", "README.md"), "# Kernel\n")
	writeRepoFile(t, filepath.Join(repoRoot, "seeds", "README.md"), "# Seeds\n")

	realizationDir := filepath.Join(repoRoot, "seeds", "1234-demo", "realizations", "a-test")
	writeRepoFile(t, filepath.Join(realizationDir, "realization.yaml"), ""+
		"realization_id: a-test\n"+
		"seed_id: 1234-demo\n"+
		"approach_id: a-approach\n"+
		"summary: Demo realization.\n"+
		"status: draft\n")

	_, err := DiscoverContracts(repoRoot)
	if err == nil {
		t.Fatal("expected missing interaction contract error")
	}
	if !errors.Is(err, ErrInteractionContractMissing) {
		t.Fatalf("expected ErrInteractionContractMissing, got %v", err)
	}
}

func TestRepositoryRealizationsDeclareInteractionContracts(t *testing.T) {
	repoRoot, err := FindRepoRoot(".")
	if err != nil {
		t.Fatalf("FindRepoRoot() error = %v", err)
	}

	contracts, err := DiscoverContracts(repoRoot)
	if err != nil {
		t.Fatalf("DiscoverContracts() error = %v", err)
	}
	if len(contracts) == 0 {
		t.Fatal("expected at least one realization contract")
	}
}

func writeRepoFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
