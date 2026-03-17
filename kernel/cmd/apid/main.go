package main

import (
	"context"
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
	hashIndex := runtimedb.NewRegistryHashIndex(pool)
	registryReader := registry.NewCatalogReader(repoRoot)
	if err := hashIndex.SyncCatalogReader(ctx, registryReader); err != nil {
		log.Fatal(err)
	}
	api := jsontransport.NewRuntimeAPI(service, func(ctx context.Context) ([]string, error) {
		return runtimedb.AppliedMigrations(ctx, pool)
	})
	executionAPI := jsontransport.NewExecutionAPI(repoRoot, service)
	operationsAPI := jsontransport.NewOperationsAPI(service)
	contractsAPI := jsontransport.NewContractsAPI(repoRoot)
	growthAPI := jsontransport.NewGrowthAPI(repoRoot, service)
	registryAPI := jsontransport.NewRegistryCatalogAPI(registryReader, hashIndex)

	mux := http.NewServeMux()
	contractsAPI.Register(mux)
	api.Register(mux)
	executionAPI.Register(mux)
	operationsAPI.Register(mux)
	growthAPI.Register(mux)
	registryAPI.Register(mux)

	telemetry.NewServiceMonitor("apid", service).Start(ctx)

	handler := server.DefaultMiddlewareStack(
		server.SessionResolutionMiddleware(server.RuntimeSessionResolver{Lookup: service},
			server.RateLimitMiddleware(service, rateLimitOptions(runtimeConfig), mux),
		),
	)

	addr := config.EnvOrDefault("AS_APID_ADDR", "127.0.0.1:8092")
	log.Printf("apid listening on %s (repo root %s)", addr, repoRoot)
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
