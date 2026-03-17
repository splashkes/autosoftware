package runtime

import (
	"context"
	"fmt"
	"strings"

	"as/kernel/internal/registry"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type RegistryHashIndex struct {
	pool *pgxpool.Pool
}

func NewRegistryHashIndex(pool *pgxpool.Pool) *RegistryHashIndex {
	if pool == nil {
		return nil
	}
	return &RegistryHashIndex{pool: pool}
}

func (idx *RegistryHashIndex) SyncCatalogReader(ctx context.Context, reader registry.CatalogReader) error {
	catalog, err := reader.Catalog()
	if err != nil {
		return fmt.Errorf("load registry catalog for hash index: %w", err)
	}
	return idx.SyncCatalog(ctx, catalog)
}

func (idx *RegistryHashIndex) SyncCatalog(ctx context.Context, catalog registry.Catalog) error {
	if idx == nil || idx.pool == nil {
		return fmt.Errorf("registry hash index database pool is required")
	}

	tx, err := idx.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin registry hash index sync: %w", err)
	}
	defer tx.Rollback(ctx)

	for _, record := range catalogHashRecords(catalog) {
		if _, err := tx.Exec(ctx, `
			insert into runtime_registry_hash_index (
				content_hash,
				resource_kind,
				canonical_url,
				permalink_url,
				updated_at
			)
			values ($1, $2, $3, $4, now())
			on conflict (content_hash)
			do update set
				resource_kind = excluded.resource_kind,
				canonical_url = excluded.canonical_url,
				permalink_url = excluded.permalink_url,
				updated_at = excluded.updated_at
		`, record.ContentHash, record.ResourceKind, record.CanonicalURL, record.PermalinkURL); err != nil {
			return fmt.Errorf("upsert registry hash index %s: %w", record.ContentHash, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit registry hash index sync: %w", err)
	}
	return nil
}

func (idx *RegistryHashIndex) Resolve(ctx context.Context, contentHash string) (registry.HashLookupRecord, error) {
	if idx == nil || idx.pool == nil {
		return registry.HashLookupRecord{}, fmt.Errorf("registry hash index database pool is required")
	}

	contentHash = strings.ToLower(strings.TrimSpace(contentHash))
	if !registry.IsSHA256Hex(contentHash) {
		return registry.HashLookupRecord{}, registry.ErrHashLookupNotFound
	}

	var record registry.HashLookupRecord
	err := idx.pool.QueryRow(ctx, `
		select content_hash, resource_kind, canonical_url, permalink_url
		from runtime_registry_hash_index
		where content_hash = $1
	`, contentHash).Scan(
		&record.ContentHash,
		&record.ResourceKind,
		&record.CanonicalURL,
		&record.PermalinkURL,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return registry.HashLookupRecord{}, registry.ErrHashLookupNotFound
		}
		return registry.HashLookupRecord{}, fmt.Errorf("resolve registry hash %s: %w", contentHash, err)
	}
	return record, nil
}

func catalogHashRecords(catalog registry.Catalog) []registry.HashLookupRecord {
	records := make([]registry.HashLookupRecord, 0, len(catalog.Realizations)+len(catalog.Commands)+len(catalog.Projections)+len(catalog.Objects)+len(catalog.Schemas))
	for _, item := range catalog.Realizations {
		records = appendCatalogHashRecord(records, registry.RealizationLocator(item))
	}
	for _, item := range catalog.Commands {
		records = appendCatalogHashRecord(records, registry.CommandLocator(item))
	}
	for _, item := range catalog.Projections {
		records = appendCatalogHashRecord(records, registry.ProjectionLocator(item))
	}
	for _, item := range catalog.Objects {
		records = appendCatalogHashRecord(records, registry.ObjectLocator(item))
	}
	for _, item := range catalog.Schemas {
		records = appendCatalogHashRecord(records, registry.SchemaLocator(item))
	}
	return records
}

func appendCatalogHashRecord(records []registry.HashLookupRecord, locator registry.ResourceLocator) []registry.HashLookupRecord {
	if strings.TrimSpace(locator.ContentHash) == "" || strings.TrimSpace(locator.CanonicalURL) == "" {
		return records
	}
	return append(records, registry.HashLookupRecord{
		ContentHash:  locator.ContentHash,
		ResourceKind: locator.ResourceKind,
		CanonicalURL: locator.CanonicalURL,
		PermalinkURL: locator.PermalinkURL,
	})
}
