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
	"as/kernel/internal/execution"
	"as/kernel/internal/interactions"
	runtimedb "as/kernel/internal/runtime"
)

func main() {
	repoRoot, err := config.RepoRootFromEnvOrWD()
	if err != nil {
		log.Fatal(err)
	}

	runtimeConfig := config.LoadRuntimeConfigFromEnv()
	if !runtimeConfig.Enabled() {
		log.Fatal("AS_RUNTIME_DATABASE_URL is required")
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

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

	service := interactions.NewRuntimeService(pool)
	worker := execution.NewLocalWorker(repoRoot, service, execution.CapabilityURLs{
		RegistryBaseURL:    "http://" + config.EnvOrDefault("AS_REGISTRYD_ADDR", "127.0.0.1:8093"),
		PublicAPIBaseURL:   "http://" + config.EnvOrDefault("AS_APID_ADDR", "127.0.0.1:8092"),
		InternalAPIBaseURL: "http://" + config.EnvOrDefault("AS_APID_ADDR", "127.0.0.1:8092"),
	}, config.EnvOrDefault("AS_EXECD_WORKER", "execd-local"), true)

	if err := worker.ReconcileStartup(ctx); err != nil {
		log.Fatal(err)
	}

	go runWorkerLoop(ctx, worker)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		if err := service.Ping(r.Context()); err != nil {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": "ok",
			"worker": "localprocess",
		})
	})

	addr := config.EnvOrDefault("AS_EXECD_ADDR", "127.0.0.1:8094")
	log.Printf("execd listening on %s (repo root %s)", addr, repoRoot)
	server := &http.Server{Addr: addr, Handler: mux}
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

func runWorkerLoop(ctx context.Context, worker *execution.LocalWorker) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		if err := worker.Tick(ctx); err != nil && ctx.Err() == nil {
			log.Printf("execd worker tick failed: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
