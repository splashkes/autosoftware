package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"as/kernel/internal/config"
	"as/kernel/internal/http/server"
	"as/kernel/internal/interactions"
	"as/kernel/internal/materializer"
	runtimedb "as/kernel/internal/runtime"
	"as/kernel/internal/telemetry"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	repoRoot, err := config.RepoRootFromEnvOrWD()
	if err != nil {
		log.Fatal(err)
	}

	service, err := materializer.NewService(repoRoot, remoteClient())
	if err != nil {
		log.Fatal(err)
	}

	var runtimeService *interactions.RuntimeService
	runtimeConfig := config.LoadRuntimeConfigFromEnv()
	if runtimeConfig.Enabled() {
		pool, err := runtimedb.OpenPool(ctx, runtimeConfig.DatabaseURL)
		if err != nil {
			log.Fatal(err)
		}
		defer pool.Close()
		if runtimeConfig.AutoMigrate {
			if err := runtimedb.RunMigrations(ctx, pool, repoRoot); err != nil {
				log.Fatal(err)
			}
		}
		runtimeService = interactions.NewRuntimeService(pool)
		telemetry.NewServiceMonitor("materializerd", runtimeService).Start(ctx)
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

	addr := config.EnvOrDefault("AS_MATERIALIZER_ADDR", "127.0.0.1:8091")
	log.Printf("materializerd listening on %s (repo root %s)", addr, repoRoot)
	httpServer := &http.Server{Addr: addr, Handler: server.DefaultMiddlewareStack(mux)}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdownCtx)
	}()
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func remoteClient() *materializer.RemoteRegistryClient {
	baseURL := strings.TrimSpace(config.EnvOrDefault("AS_REMOTE_REGISTRY_URL", ""))
	if baseURL == "" {
		return nil
	}

	return &materializer.RemoteRegistryClient{BaseURL: baseURL}
}

func respondJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func respondError(w http.ResponseWriter, status int, err error) {
	respondJSON(w, status, map[string]string{"error": err.Error()})
}
