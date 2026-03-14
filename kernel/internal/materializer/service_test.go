package materializer

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestServiceMaterializeMergesLocalAndRemote(t *testing.T) {
	repoRoot := t.TempDir()
	writeFile(t, filepath.Join(repoRoot, "genesis", "realizations", "alpha", "realization.yaml"), ""+
		"realization_id: alpha\n"+
		"seed_id: 0000-genesis\n"+
		"approach_id: a-foundation\n"+
		"summary: Local foundation.\n"+
		"status: accepted\n"+
		"artifacts:\n"+
		"  - artifacts/registry_init.yaml\n")
	writeFile(t, filepath.Join(repoRoot, "genesis", "realizations", "alpha", "README.md"), "# Alpha\n")
	writeFile(t, filepath.Join(repoRoot, "genesis", "realizations", "alpha", "artifacts", "registry_init.yaml"), "kind: registry\n")
	mustMkdir(t, filepath.Join(repoRoot, "kernel"))
	mustMkdir(t, filepath.Join(repoRoot, "seeds"))

	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/realizations":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"realizations": []RealizationOption{{
					Reference:     "0000-genesis/alpha",
					SeedID:        "0000-genesis",
					RealizationID: "alpha",
					Summary:       "Remote alpha.",
					Status:        "accepted",
					Sources:       SourceFlags{Remote: true},
				}},
			})
		case "/v1/materializations":
			_ = json.NewEncoder(w).Encode(Materialization{
				Reference:     "0000-genesis/alpha",
				SeedID:        "0000-genesis",
				RealizationID: "alpha",
				Summary:       "Remote alpha.",
				Status:        "accepted",
				GeneratedAt:   time.Date(2026, 3, 14, 0, 0, 0, 0, time.UTC),
				Sources:       SourceFlags{Remote: true},
				Local: &LocalSource{
					Files: []FilePreview{{
						Path:    "remote/snapshot.json",
						Kind:    "artifact",
						Preview: "{\"remote\":true}",
					}},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer remote.Close()

	service, err := NewService(repoRoot, &RemoteRegistryClient{BaseURL: remote.URL})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	service.Now = func() time.Time {
		return time.Date(2026, 3, 14, 12, 0, 0, 0, time.UTC)
	}

	result, err := service.Materialize(context.Background(), "0000-genesis/alpha")
	if err != nil {
		t.Fatalf("Materialize() error = %v", err)
	}

	if !result.Sources.Local || !result.Sources.Remote {
		t.Fatalf("expected both sources, got %+v", result.Sources)
	}
	if result.PersistedPath == "" {
		t.Fatal("expected persisted path")
	}

	target := filepath.Join(repoRoot, filepath.FromSlash(result.PersistedPath))
	if _, err := os.Stat(target); err != nil {
		t.Fatalf("persisted file missing: %v", err)
	}
}

func TestListRealizationsIncludesRemoteOnlyEntries(t *testing.T) {
	repoRoot := t.TempDir()
	mustMkdir(t, filepath.Join(repoRoot, "genesis"))
	mustMkdir(t, filepath.Join(repoRoot, "kernel"))
	mustMkdir(t, filepath.Join(repoRoot, "seeds"))

	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"realizations": []RealizationOption{{
				Reference:     "9000-remote/omega",
				SeedID:        "9000-remote",
				RealizationID: "omega",
				Summary:       "Remote only.",
				Status:        "published",
			}},
		})
	}))
	defer remote.Close()

	service, err := NewService(repoRoot, &RemoteRegistryClient{BaseURL: remote.URL})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	options, err := service.ListRealizations(context.Background())
	if err != nil {
		t.Fatalf("ListRealizations() error = %v", err)
	}
	if len(options) != 1 {
		t.Fatalf("expected 1 option, got %d", len(options))
	}
	if !options[0].Sources.Remote {
		t.Fatalf("expected remote source flag, got %+v", options[0].Sources)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}
