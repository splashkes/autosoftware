package execution

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWaitForHealthyRejects404(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()

	addr := strings.TrimPrefix(server.URL, "http://")
	ctx, cancel := context.WithTimeout(context.Background(), 700*time.Millisecond)
	defer cancel()

	if err := WaitForHealthy(ctx, addr); err == nil {
		t.Fatal("expected WaitForHealthy to reject a 404-only server")
	}
}

func TestWaitForHealthyAccepts200(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	addr := strings.TrimPrefix(server.URL, "http://")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := WaitForHealthy(ctx, addr); err != nil {
		t.Fatalf("expected WaitForHealthy to accept a 200 server, got %v", err)
	}
}

func TestBuildLocalSpecInjectsRuntimeDatabaseURL(t *testing.T) {
	repoRoot := t.TempDir()
	for _, dir := range []string{
		filepath.Join(repoRoot, "genesis"),
		filepath.Join(repoRoot, "kernel"),
		filepath.Join(repoRoot, "seeds", "1234-demo", "realizations", "a-runtime", "artifacts", "app"),
	} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}

	manifestPath := filepath.Join(repoRoot, "seeds", "1234-demo", "realizations", "a-runtime", "realization.yaml")
	runtimePath := filepath.Join(repoRoot, "seeds", "1234-demo", "realizations", "a-runtime", "artifacts", "runtime.yaml")
	entrypointPath := filepath.Join(repoRoot, "seeds", "1234-demo", "realizations", "a-runtime", "artifacts", "app", "main.go")
	if err := os.WriteFile(manifestPath, []byte(strings.TrimSpace(`
realization_id: a-runtime
seed_id: 1234-demo
approach_id: a-runtime
summary: Runtime demo.
status: draft
artifacts:
  - artifacts/runtime.yaml
`)), 0644); err != nil {
		t.Fatalf("write realization manifest: %v", err)
	}
	if err := os.WriteFile(runtimePath, []byte(strings.TrimSpace(`
kind: runtime
version: 1
runtime: go
entrypoint: artifacts/app/main.go
working_directory: artifacts/app
run:
  command: prebuilt
`)), 0644); err != nil {
		t.Fatalf("write runtime manifest: %v", err)
	}
	if err := os.WriteFile(entrypointPath, []byte("package main\n"), 0644); err != nil {
		t.Fatalf("write entrypoint: %v", err)
	}
	prebuiltDir := filepath.Join(repoRoot, "materialized", "realizations", "1234-demo", "a-runtime", "runtime", "prebuilt", DOKSRuntimeGOOS+"-"+DOKSRuntimeGOARCH)
	if err := os.MkdirAll(prebuiltDir, 0755); err != nil {
		t.Fatalf("mkdir prebuilt dir: %v", err)
	}
	prebuiltBinary := filepath.Join(prebuiltDir, "launch-test")
	if err := os.WriteFile(prebuiltBinary, []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
		t.Fatalf("write prebuilt binary: %v", err)
	}
	metadata, err := json.Marshal(goPrebuiltMetadata{
		Runtime:          "go",
		GOOS:             DOKSRuntimeGOOS,
		GOARCH:           DOKSRuntimeGOARCH,
		Fingerprint:      "test",
		Binary:           filepath.Base(prebuiltBinary),
		Entrypoint:       "artifacts/app/main.go",
		WorkingDirectory: "artifacts/app",
		BuiltAt:          time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("marshal prebuilt metadata: %v", err)
	}
	if err := os.WriteFile(filepath.Join(prebuiltDir, goPrebuiltMetadataFile), append(metadata, '\n'), 0644); err != nil {
		t.Fatalf("write prebuilt metadata: %v", err)
	}

	spec, err := BuildLocalSpec(repoRoot, "1234-demo/a-runtime", "exec_demo_123", CapabilityURLs{
		RuntimeDatabaseURL: "postgres://runtime.example/as",
	})
	if err != nil {
		t.Fatalf("BuildLocalSpec: %v", err)
	}

	joined := strings.Join(spec.Environment, "\n")
	if !strings.Contains(joined, "AS_RUNTIME_DATABASE_URL=postgres://runtime.example/as") {
		t.Fatalf("expected runtime database URL in environment, got %q", joined)
	}
	if got, want := spec.Command, prebuiltBinary; got != want {
		t.Fatalf("expected prebuilt command %q, got %q", want, got)
	}
}
