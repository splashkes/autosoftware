package realizations

import (
	"path/filepath"
	"testing"
)

func TestValidateLocalRuntimeManifestRejectsKernelOwnedEnvironment(t *testing.T) {
	repoRoot := t.TempDir()
	realizationRoot := filepath.Join(repoRoot, "seeds", "1234-demo", "realizations", "a-runtime")

	manifest := RuntimeManifest{
		Runtime:          "go",
		Entrypoint:       "artifacts/app/main.go",
		WorkingDirectory: "artifacts/app",
		Run: RuntimeManifestRun{
			Command: "go",
			Args:    []string{"run", "."},
		},
		Environment: map[string]string{
			"AS_ADDR": "127.0.0.1:9000",
		},
	}

	if err := ValidateLocalRuntimeManifest(repoRoot, realizationRoot, manifest); err == nil {
		t.Fatal("expected reserved kernel environment key to be rejected")
	}
}

func TestValidateLocalRuntimeManifestAllowsAuthorOwnedEnvironment(t *testing.T) {
	repoRoot := t.TempDir()
	realizationRoot := filepath.Join(repoRoot, "seeds", "1234-demo", "realizations", "a-runtime")

	manifest := RuntimeManifest{
		Runtime:          "go",
		Entrypoint:       "artifacts/app/main.go",
		WorkingDirectory: "artifacts/app",
		Run: RuntimeManifestRun{
			Command: "go",
			Args:    []string{"run", "."},
		},
		Environment: map[string]string{
			"AS_DATA_FILE": "data/events.json",
		},
	}

	if err := ValidateLocalRuntimeManifest(repoRoot, realizationRoot, manifest); err != nil {
		t.Fatalf("expected author-owned environment to pass validation, got %v", err)
	}
}
