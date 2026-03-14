package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func RunMigrations(ctx context.Context, pool *pgxpool.Pool, repoRoot string) error {
	if pool == nil {
		return fmt.Errorf("runtime database pool is required")
	}
	if strings.TrimSpace(repoRoot) == "" {
		return fmt.Errorf("repo root is required")
	}

	files, err := runtimeMigrationFiles(repoRoot)
	if err != nil {
		return err
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin runtime migration transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
		create table if not exists as_kernel_runtime_migrations (
		  filename text primary key,
		  checksum text not null default '',
		  applied_at timestamptz not null default now()
		);
	`); err != nil {
		return fmt.Errorf("ensure runtime migrations table: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		alter table as_kernel_runtime_migrations
		add column if not exists checksum text not null default ''
	`); err != nil {
		return fmt.Errorf("upgrade runtime migrations table: %w", err)
	}

	applied := map[string]string{}
	rows, err := tx.Query(ctx, `select filename, checksum from as_kernel_runtime_migrations`)
	if err != nil {
		return fmt.Errorf("load applied runtime migrations: %w", err)
	}
	for rows.Next() {
		var filename string
		var checksum string
		if err := rows.Scan(&filename, &checksum); err != nil {
			rows.Close()
			return fmt.Errorf("scan applied runtime migration: %w", err)
		}
		applied[filename] = checksum
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("iterate applied runtime migrations: %w", err)
	}
	rows.Close()

	for _, path := range files {
		filename := filepath.Base(path)
		sqlBytes, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read runtime migration %s: %w", filename, err)
		}

		statement := strings.TrimSpace(string(sqlBytes))
		if statement == "" {
			continue
		}
		checksum := migrationChecksum(sqlBytes)
		if applied[filename] == checksum {
			continue
		}

		if _, err := tx.Exec(ctx, statement); err != nil {
			return fmt.Errorf("apply runtime migration %s: %w", filename, err)
		}

		if _, err := tx.Exec(ctx, `
			insert into as_kernel_runtime_migrations (filename, checksum, applied_at)
			values ($1, $2, $3)
			on conflict (filename)
			do update set
			  checksum = excluded.checksum,
			  applied_at = excluded.applied_at
		`, filename, checksum, time.Now().UTC()); err != nil {
			return fmt.Errorf("record runtime migration %s: %w", filename, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit runtime migrations: %w", err)
	}

	return nil
}

func AppliedMigrations(ctx context.Context, pool *pgxpool.Pool) ([]string, error) {
	if pool == nil {
		return nil, fmt.Errorf("runtime database pool is required")
	}

	rows, err := pool.Query(ctx, `
		select filename
		from as_kernel_runtime_migrations
		order by filename
	`)
	if err != nil {
		return nil, fmt.Errorf("list applied runtime migrations: %w", err)
	}
	defer rows.Close()

	var filenames []string
	for rows.Next() {
		var filename string
		if err := rows.Scan(&filename); err != nil {
			return nil, fmt.Errorf("scan applied runtime migration: %w", err)
		}
		filenames = append(filenames, filename)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate applied runtime migrations: %w", err)
	}

	return filenames, nil
}

func runtimeMigrationFiles(repoRoot string) ([]string, error) {
	pattern := filepath.Join(repoRoot, "kernel", "db", "runtime", "*.sql")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("glob runtime migrations: %w", err)
	}
	sort.Strings(files)
	return files, nil
}

func migrationChecksum(payload []byte) string {
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}
