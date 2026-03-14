package main

import (
	"context"
	"log"
	"net/http"

	"as/kernel/internal/config"
	jsontransport "as/kernel/internal/http/json"
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
	if !runtimeConfig.Enabled() {
		log.Fatal("AS_RUNTIME_DATABASE_URL is required")
	}

	ctx := context.Background()
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
	api := jsontransport.NewRuntimeAPI(service, func(ctx context.Context) ([]string, error) {
		return runtimedb.AppliedMigrations(ctx, pool)
	})
	contractsAPI := jsontransport.NewContractsAPI(repoRoot)
	growthAPI := jsontransport.NewGrowthAPI(repoRoot, service)

	mux := http.NewServeMux()
	contractsAPI.Register(mux)
	api.Register(mux)
	growthAPI.Register(mux)

	handler := server.DefaultMiddlewareStack(
		server.SessionResolutionMiddleware(server.RuntimeSessionResolver{Lookup: service}, mux),
	)

	addr := config.EnvOrDefault("AS_APID_ADDR", "127.0.0.1:8092")
	log.Printf("apid listening on %s (repo root %s)", addr, repoRoot)
	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatal(err)
	}
}
