package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"as/kernel/internal/config"
	jsontransport "as/kernel/internal/http/json"
	"as/kernel/internal/http/server"
	"as/kernel/internal/interactions"
	"as/kernel/internal/registry"
	runtimedb "as/kernel/internal/runtime"
	"as/kernel/internal/telemetry"
)

func main() {
	repoRoot, err := config.RepoRootFromEnvOrWD()
	if err != nil {
		log.Fatal(err)
	}

	runtimeConfig := config.LoadRuntimeConfigFromEnv()
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

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

	mux := newMux(repoRoot, runtimeService, appliedMigrations)
	if runtimeService != nil {
		telemetry.NewServiceMonitor("registryd", runtimeService).Start(ctx)
	}

	var handler http.Handler = mux
	if runtimeService != nil {
		handler = server.SessionResolutionMiddleware(server.RuntimeSessionResolver{Lookup: runtimeService},
			server.RateLimitMiddleware(runtimeService, rateLimitOptions(runtimeConfig), mux),
		)
	}
	handler = server.DefaultMiddlewareStack(handler)

	addr := config.EnvOrDefault("AS_REGISTRYD_ADDR", "127.0.0.1:8093")
	log.Printf("registryd listening on %s (repo root %s, runtime %t)", addr, repoRoot, runtimeService != nil)
	server := &http.Server{Addr: addr, Handler: handler}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func newMux(repoRoot string, runtimeService *interactions.RuntimeService, appliedMigrations func(context.Context) ([]string, error)) *http.ServeMux {
	mux := http.NewServeMux()
	jsontransport.NewRegistryCatalogAPI(registry.NewCatalogReader(repoRoot)).Register(mux)
	jsontransport.NewRegistryLedgerAPI(runtimeService).Register(mux)
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
	return mux
}

func rateLimitOptions(runtimeConfig config.RuntimeConfig) server.RateLimitOptions {
	return server.RateLimitOptions{
		Enabled:             runtimeConfig.RateLimitsEnabled,
		Window:              time.Duration(runtimeConfig.RateLimitWindow) * time.Second,
		BlockDuration:       time.Duration(runtimeConfig.RateLimitBlock) * time.Second,
		AnonymousReadLimit:  int64(runtimeConfig.AnonymousRead),
		AnonymousWriteLimit: int64(runtimeConfig.AnonymousWrite),
		SessionReadLimit:    int64(runtimeConfig.SessionRead),
		SessionWriteLimit:   int64(runtimeConfig.SessionWrite),
		InternalLimit:       int64(runtimeConfig.Internal),
		WorkerLimit:         int64(runtimeConfig.Worker),
		FeedbackLimit:       int64(runtimeConfig.Feedback),
	}
}
