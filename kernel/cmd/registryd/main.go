package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	"as/kernel/internal/config"
	"as/kernel/internal/http/server"
	"as/kernel/internal/interactions"
	runtimedb "as/kernel/internal/runtime"
)

func main() {
	repoRoot, err := config.RepoRootFromEnvOrWD()
	if err != nil {
		log.Fatal(err)
	}

	runtimeConfig := config.LoadRuntimeConfigFromEnv()
	ctx := context.Background()

	var runtimeService *interactions.RuntimeService
	var appliedMigrations func(context.Context) ([]string, error)

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
		appliedMigrations = func(ctx context.Context) ([]string, error) {
			return runtimedb.AppliedMigrations(ctx, pool)
		}
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	mux.HandleFunc("GET /v1/registry/status", func(w http.ResponseWriter, r *http.Request) {
		payload := map[string]interface{}{
			"status":   "ok",
			"registry": "scaffolded",
		}
		if runtimeService != nil {
			if err := runtimeService.Ping(r.Context()); err != nil {
				http.Error(w, err.Error(), http.StatusServiceUnavailable)
				return
			}
			payload["runtime_db"] = "connected"
			if appliedMigrations != nil {
				migrations, err := appliedMigrations(r.Context())
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				payload["applied_migrations"] = migrations
			}
		} else {
			payload["runtime_db"] = "disabled"
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(payload); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	var handler http.Handler = mux
	if runtimeService != nil {
		handler = server.SessionResolutionMiddleware(server.RuntimeSessionResolver{Lookup: runtimeService}, mux)
	}
	handler = server.DefaultMiddlewareStack(handler)

	addr := config.EnvOrDefault("AS_REGISTRYD_ADDR", ":8093")
	log.Printf("registryd listening on %s (repo root %s, runtime %t)", addr, repoRoot, runtimeService != nil)
	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatal(err)
	}
}
