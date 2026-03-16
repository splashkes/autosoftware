package realizations

import (
	"path/filepath"
	"testing"
)

func TestLoadRealizationMetaClassifiesReadiness(t *testing.T) {
	repoRoot := setupGrowthTestRepo(t)

	entries, err := Discover(repoRoot)
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}

	definedEntry, ok := ResolveByReference(entries, "1234-demo/a-defined")
	if !ok {
		t.Fatal("expected to resolve defined realization")
	}
	runnableEntry, ok := ResolveByReference(entries, "1234-demo/b-runnable")
	if !ok {
		t.Fatal("expected to resolve runnable realization")
	}

	definedMeta, err := LoadRealizationMeta(repoRoot, definedEntry)
	if err != nil {
		t.Fatalf("LoadRealizationMeta(defined) error = %v", err)
	}
	if definedMeta.Readiness.Stage != "defined" {
		t.Fatalf("expected defined readiness stage, got %q", definedMeta.Readiness.Stage)
	}
	if definedMeta.Readiness.CanRun {
		t.Fatal("expected defined realization to be non-runnable")
	}
	if !definedMeta.Readiness.HasContract {
		t.Fatal("expected defined realization to have an interaction contract")
	}
	if definedMeta.Readiness.HasRuntime {
		t.Fatal("expected defined realization to have no runtime artifact")
	}

	runnableMeta, err := LoadRealizationMeta(repoRoot, runnableEntry)
	if err != nil {
		t.Fatalf("LoadRealizationMeta(runnable) error = %v", err)
	}
	if runnableMeta.Readiness.Stage != "runnable" {
		t.Fatalf("expected runnable readiness stage, got %q", runnableMeta.Readiness.Stage)
	}
	if !runnableMeta.Readiness.CanRun {
		t.Fatal("expected runnable realization to be runnable")
	}
	if !runnableMeta.Readiness.CanLaunchLocal {
		t.Fatal("expected runnable realization to be launchable on the local backend")
	}
	if !runnableMeta.Readiness.HasRuntime {
		t.Fatal("expected runnable realization to have a runtime artifact")
	}
	if got, want := runnableMeta.RuntimeArtifact, "seeds/1234-demo/realizations/b-runnable/artifacts/runtime.yaml"; got != want {
		t.Fatalf("expected runtime artifact %q, got %q", want, got)
	}
}

func TestLoadGrowthContextCollectsSeedPacket(t *testing.T) {
	repoRoot := setupGrowthTestRepo(t)

	packet, err := LoadGrowthContext(repoRoot, "1234-demo/b-runnable")
	if err != nil {
		t.Fatalf("LoadGrowthContext() error = %v", err)
	}

	if packet.Reference != "1234-demo/b-runnable" {
		t.Fatalf("expected packet reference %q, got %q", "1234-demo/b-runnable", packet.Reference)
	}
	if packet.Readiness.Stage != "runnable" {
		t.Fatalf("expected runnable readiness, got %q", packet.Readiness.Stage)
	}
	if len(packet.SeedDocs) < 5 {
		t.Fatalf("expected several seed docs, got %d", len(packet.SeedDocs))
	}
	if !containsContextKind(packet.ApproachDocs, "approach_manifest") {
		t.Fatal("expected approach manifest in growth context")
	}
	if !containsContextKind(packet.RealizationDocs, "interaction_contract") {
		t.Fatal("expected interaction contract in realization docs")
	}
	if !containsContextKind(packet.RuntimeDocs, "runtime_manifest") {
		t.Fatal("expected runtime manifest in growth context")
	}
	if !containsContextKind(packet.RuntimeDocs, "artifact") {
		t.Fatal("expected runtime artifact preview in growth context")
	}
}

func setupGrowthTestRepo(t *testing.T) string {
	t.Helper()

	repoRoot := t.TempDir()
	writeRepoFile(t, filepath.Join(repoRoot, "genesis", "README.md"), "# Genesis\n")
	writeRepoFile(t, filepath.Join(repoRoot, "kernel", "README.md"), "# Kernel\n")
	writeRepoFile(t, filepath.Join(repoRoot, "seeds", "README.md"), "# Seeds\n")

	seedDir := filepath.Join(repoRoot, "seeds", "1234-demo")
	writeRepoFile(t, filepath.Join(seedDir, "README.md"), "# Demo Seed\n")
	writeRepoFile(t, filepath.Join(seedDir, "brief.md"), "# Brief\n")
	writeRepoFile(t, filepath.Join(seedDir, "design.md"), "# Design\n")
	writeRepoFile(t, filepath.Join(seedDir, "acceptance.md"), "# Acceptance\n")
	writeRepoFile(t, filepath.Join(seedDir, "decision_log.md"), "# Decision Log\n")
	writeRepoFile(t, filepath.Join(seedDir, "notes.md"), "# Notes\n")
	writeRepoFile(t, filepath.Join(seedDir, "seed.yaml"), ""+
		"seed_id: 1234-demo\n"+
		"version: 1\n"+
		"summary: Demo growth seed.\n"+
		"status: proposed\n")
	writeRepoFile(t, filepath.Join(seedDir, "approaches", "README.md"), "# Approaches\n")
	writeRepoFile(t, filepath.Join(seedDir, "approaches", "a-test.yaml"), ""+
		"approach_id: a-test\n"+
		"summary: Demo approach.\n"+
		"status: active\n")

	writeGrowthRealizationFixture(t, seedDir, "a-defined", false)
	writeGrowthRealizationFixture(t, seedDir, "b-runnable", true)

	return repoRoot
}

func writeGrowthRealizationFixture(t *testing.T, seedDir, realizationID string, withRuntime bool) {
	t.Helper()

	realizationDir := filepath.Join(seedDir, "realizations", realizationID)
	manifest := "" +
		"realization_id: " + realizationID + "\n" +
		"seed_id: 1234-demo\n" +
		"approach_id: a-test\n" +
		"summary: Demo realization " + realizationID + ".\n" +
		"status: draft\n"
	if withRuntime {
		manifest += "artifacts:\n  - artifacts/runtime.yaml\n  - artifacts/README.md\n"
	}

	writeRepoFile(t, filepath.Join(realizationDir, "README.md"), "# Realization\n")
	writeRepoFile(t, filepath.Join(realizationDir, "notes.md"), "# Notes\n")
	writeRepoFile(t, filepath.Join(realizationDir, "realization.yaml"), manifest)
	writeRepoFile(t, filepath.Join(realizationDir, "interaction_contract.yaml"), ""+
		"contract_version: v1\n"+
		"surface_kind: interactive\n"+
		"seed_id: 1234-demo\n"+
		"realization_id: "+realizationID+"\n"+
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
		"    capabilities:\n"+
		"      - sessions\n"+
		"    freshness: materialized\n"+
		"consistency:\n"+
		"  write_visibility: read_your_writes\n"+
		"  projection_freshness: materialized\n")

	if withRuntime {
		writeRepoFile(t, filepath.Join(realizationDir, "artifacts", "runtime.yaml"), ""+
			"kind: runtime_manifest\n"+
			"version: 1\n"+
			"runtime: go\n"+
			"entrypoint: artifacts/app\n"+
			"working_directory: artifacts\n"+
			"run:\n"+
			"  command: go\n"+
			"  args:\n"+
			"    - run\n"+
			"    - ./cmd/app\n")
		writeRepoFile(t, filepath.Join(realizationDir, "artifacts", "README.md"), "# Runtime Artifact\n")
	}
}

func containsContextKind(files []ContextFile, kind string) bool {
	for _, file := range files {
		if file.Kind == kind {
			return true
		}
	}
	return false
}
