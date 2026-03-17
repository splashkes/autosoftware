package interactions

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	runtimedb "as/kernel/internal/runtime"
)

func TestRuntimeServiceEnforceRateLimitBlocksAfterLimit(t *testing.T) {
	dsn := os.Getenv("AS_RUNTIME_DATABASE_URL")
	if dsn == "" {
		t.Skip("AS_RUNTIME_DATABASE_URL is required")
	}

	ctx := context.Background()
	pool, err := runtimedb.OpenPool(ctx, dsn)
	if err != nil {
		t.Fatalf("open pool: %v", err)
	}
	defer pool.Close()

	if _, err := pool.Exec(ctx, `
		delete from runtime_rate_limit_buckets
		where namespace = 'api.registry'
		  and subject_key = 'ip:198.51.100.99'
	`); err != nil {
		t.Fatalf("delete buckets: %v", err)
	}

	service := NewRuntimeService(pool)
	service.now = func() time.Time {
		return time.Date(2026, time.March, 17, 12, 34, 45, 0, time.UTC)
	}

	first, err := service.EnforceRateLimit(ctx, EnforceRateLimitInput{
		Namespace:     "api.registry",
		SubjectKey:    "ip:198.51.100.99",
		Action:        "/v1/registry/catalog",
		Limit:         2,
		Window:        time.Minute,
		BlockDuration: time.Minute,
	})
	if err != nil {
		t.Fatalf("first enforce: %v", err)
	}
	if !first.Allowed || first.HitCount != 1 {
		t.Fatalf("expected first call allowed with hit_count=1, got allowed=%v hit_count=%d", first.Allowed, first.HitCount)
	}

	second, err := service.EnforceRateLimit(ctx, EnforceRateLimitInput{
		Namespace:     "api.registry",
		SubjectKey:    "ip:198.51.100.99",
		Action:        "/v1/registry/catalog",
		Limit:         2,
		Window:        time.Minute,
		BlockDuration: time.Minute,
	})
	if err != nil {
		t.Fatalf("second enforce: %v", err)
	}
	if !second.Allowed || second.HitCount != 2 {
		t.Fatalf("expected second call allowed with hit_count=2, got allowed=%v hit_count=%d", second.Allowed, second.HitCount)
	}

	third, err := service.EnforceRateLimit(ctx, EnforceRateLimitInput{
		Namespace:     "api.registry",
		SubjectKey:    "ip:198.51.100.99",
		Action:        "/v1/registry/catalog",
		Limit:         2,
		Window:        time.Minute,
		BlockDuration: time.Minute,
	})
	if err == nil {
		t.Fatal("expected third call to be rate limited")
	}
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("expected rate-limited error, got %T %v", err, err)
	}
	if third.Allowed {
		t.Fatal("expected third call to be blocked")
	}
	if third.HitCount != 3 {
		t.Fatalf("expected third hit_count=3, got %d", third.HitCount)
	}
	if third.RetryAfter <= 0 {
		t.Fatalf("expected retry_after > 0, got %s", third.RetryAfter)
	}
}
