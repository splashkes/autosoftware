package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"as/kernel/internal/config"
	"as/kernel/internal/execution"
	"as/kernel/internal/interactions"
	runtimedb "as/kernel/internal/runtime"
	"as/kernel/internal/telemetry"
	"github.com/jackc/pgx/v5/pgxpool"
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
		RegistryBaseURL:    capabilityBaseURL("AS_EXECD_REGISTRY_BASE_URL", "AS_REGISTRYD_ADDR", "127.0.0.1:8093"),
		PublicAPIBaseURL:   capabilityBaseURL("AS_EXECD_PUBLIC_API_BASE_URL", "AS_APID_ADDR", "127.0.0.1:8092"),
		InternalAPIBaseURL: capabilityBaseURL("AS_EXECD_INTERNAL_API_BASE_URL", "AS_APID_ADDR", "127.0.0.1:8092"),
		RuntimeDatabaseURL: runtimeConfig.DatabaseURL,
	}, config.EnvOrDefault("AS_EXECD_WORKER", "execd-local"), true)
	worker.Budgets.MaxRSSBytes = int64Env("AS_EXECUTION_MAX_RSS_BYTES", worker.Budgets.MaxRSSBytes)
	worker.Budgets.MaxLogBytes = int64Env("AS_EXECUTION_MAX_LOG_BYTES", worker.Budgets.MaxLogBytes)
	worker.Budgets.MaxCPUPercent = float64Env("AS_EXECUTION_MAX_CPU_PERCENT", worker.Budgets.MaxCPUPercent)
	worker.Budgets.MaxOpenFDs = intEnv("AS_EXECUTION_MAX_OPEN_FDS", worker.Budgets.MaxOpenFDs)
	worker.Budgets.ConsecutiveBreaches = intEnv("AS_EXECUTION_BUDGET_STRIKES", worker.Budgets.ConsecutiveBreaches)
	worker.Budgets.RemediationTarget = config.EnvOrDefault("AS_EXECUTION_REMEDIATION_TARGET", worker.Budgets.RemediationTarget)
	worker.Budgets.RemediationHint = config.EnvOrDefault("AS_EXECUTION_REMEDIATION_HINT", worker.Budgets.RemediationHint)
	worker.LaunchHealthTimeout = time.Duration(intEnv("AS_EXECUTION_HEALTH_TIMEOUT_SECONDS", int(worker.LaunchHealthTimeout/time.Second))) * time.Second

	telemetry.NewServiceMonitor("execd", service).Start(ctx)

	if err := worker.ReconcileStartup(ctx); err != nil {
		log.Fatal(err)
	}

	go runWorkerLoop(ctx, worker, pool)

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

func runWorkerLoop(ctx context.Context, worker *execution.LocalWorker, pool *pgxpool.Pool) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	wakeCh := startRuntimeJobWakeListener(ctx, pool, "realization-execution")
	for {
		if err := worker.Tick(ctx); err != nil && ctx.Err() == nil {
			log.Printf("execd worker tick failed: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-wakeCh:
		case <-ticker.C:
		}
	}
}

func startRuntimeJobWakeListener(ctx context.Context, pool *pgxpool.Pool, queue string) <-chan struct{} {
	wakeCh := make(chan struct{}, 1)
	go func() {
		for ctx.Err() == nil {
			if err := listenForRuntimeJobs(ctx, pool, queue, wakeCh); err != nil && ctx.Err() == nil {
				log.Printf("execd runtime job listener failed: %v", err)
			}
			if ctx.Err() != nil {
				return
			}
			timer := time.NewTimer(time.Second)
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
		}
	}()
	return wakeCh
}

func listenForRuntimeJobs(ctx context.Context, pool *pgxpool.Pool, queue string, wakeCh chan<- struct{}) error {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, "listen "+interactions.RuntimeJobsNotifyChannel); err != nil {
		return err
	}
	defer func() {
		unlistenCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_, _ = conn.Exec(unlistenCtx, "unlisten "+interactions.RuntimeJobsNotifyChannel)
	}()

	for ctx.Err() == nil {
		notification, err := conn.Conn().WaitForNotification(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		if notification == nil {
			continue
		}
		payload := strings.TrimSpace(notification.Payload)
		if payload != "" && payload != queue {
			continue
		}
		select {
		case wakeCh <- struct{}{}:
		default:
		}
	}
	return nil
}

func intEnv(key string, fallback int) int {
	value, err := strconv.Atoi(strings.TrimSpace(config.EnvOrDefault(key, "")))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func int64Env(key string, fallback int64) int64 {
	value, err := strconv.ParseInt(strings.TrimSpace(config.EnvOrDefault(key, "")), 10, 64)
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func float64Env(key string, fallback float64) float64 {
	value, err := strconv.ParseFloat(strings.TrimSpace(config.EnvOrDefault(key, "")), 64)
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func httpBaseURLFromAddr(addr string) string {
	trimmed := strings.TrimSpace(addr)
	if trimmed == "" {
		return ""
	}
	if strings.Contains(trimmed, "://") {
		return strings.TrimRight(trimmed, "/")
	}
	if strings.HasPrefix(trimmed, ":") {
		trimmed = "127.0.0.1" + trimmed
	}
	return "http://" + trimmed
}

func capabilityBaseURL(explicitKey, addrKey, fallbackAddr string) string {
	if explicit := strings.TrimSpace(config.EnvOrDefault(explicitKey, "")); explicit != "" {
		return httpBaseURLFromAddr(explicit)
	}
	return httpBaseURLFromAddr(config.EnvOrDefault(addrKey, fallbackAddr))
}
