package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"strings"

	"as/kernel/internal/materializer"
	"as/kernel/internal/realizations"
)

func main() {
	repoRoot, err := repoRootFromEnvOrWD()
	if err != nil {
		log.Fatal(err)
	}

	service, err := materializer.NewService(repoRoot, remoteClient())
	if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /v1/realizations", func(w http.ResponseWriter, r *http.Request) {
		entries, err := service.ListRealizations(r.Context())
		if err != nil {
			respondError(w, http.StatusInternalServerError, err)
			return
		}

		respondJSON(w, http.StatusOK, map[string]any{"realizations": entries})
	})
	mux.HandleFunc("GET /v1/materializations", func(w http.ResponseWriter, r *http.Request) {
		reference := strings.TrimSpace(r.URL.Query().Get("reference"))
		if reference == "" {
			respondError(w, http.StatusBadRequest, errors.New("reference is required"))
			return
		}

		result, err := service.Materialize(r.Context(), reference)
		if err != nil {
			status := http.StatusInternalServerError
			if errors.Is(err, materializer.ErrReferenceNotFound) {
				status = http.StatusNotFound
			}
			respondError(w, status, err)
			return
		}

		respondJSON(w, http.StatusOK, result)
	})

	addr := envOrDefault("AS_MATERIALIZER_ADDR", ":8091")
	log.Printf("materializerd listening on %s (repo root %s)", addr, repoRoot)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}

func repoRootFromEnvOrWD() (string, error) {
	if root := strings.TrimSpace(os.Getenv("AS_REPO_ROOT")); root != "" {
		return root, nil
	}

	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	return realizations.FindRepoRoot(wd)
}

func remoteClient() *materializer.RemoteRegistryClient {
	baseURL := strings.TrimSpace(os.Getenv("AS_REMOTE_REGISTRY_URL"))
	if baseURL == "" {
		return nil
	}

	return &materializer.RemoteRegistryClient{BaseURL: baseURL}
}

func envOrDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}

	return fallback
}

func respondJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func respondError(w http.ResponseWriter, status int, err error) {
	respondJSON(w, status, map[string]string{"error": err.Error()})
}
