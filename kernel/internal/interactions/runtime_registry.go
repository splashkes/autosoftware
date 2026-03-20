package interactions

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

func (s *RuntimeService) AppendRegistryChangeSet(ctx context.Context, input AppendRegistryChangeSetInput) (AppendedRegistryChangeSet, error) {
	pool, err := expectReady(s)
	if err != nil {
		return AppendedRegistryChangeSet{}, err
	}
	input.Reference = strings.TrimSpace(input.Reference)
	input.SeedID = strings.TrimSpace(input.SeedID)
	input.RealizationID = strings.TrimSpace(input.RealizationID)
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	input.AcceptedBy = strings.TrimSpace(input.AcceptedBy)
	if input.Reference == "" || input.SeedID == "" || input.RealizationID == "" {
		return AppendedRegistryChangeSet{}, errors.New("reference, seed_id, and realization_id are required")
	}
	if len(input.Rows) == 0 {
		return AppendedRegistryChangeSet{}, errors.New("at least one row is required")
	}
	for i, row := range input.Rows {
		if strings.TrimSpace(row.RowType) == "" {
			return AppendedRegistryChangeSet{}, fmt.Errorf("rows[%d].row_type is required", i)
		}
	}
	if input.ChangeSetID == "" {
		input.ChangeSetID = newID("chg")
	}
	if input.AcceptedBy == "" {
		input.AcceptedBy = "system"
	}
	acceptedAt := s.nowUTC()

	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return AppendedRegistryChangeSet{}, wrapErr("begin registry change set tx", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	row := tx.QueryRow(ctx, `
		insert into runtime_registry_change_sets (
		  change_set_id, reference, seed_id, realization_id, idempotency_key, accepted_by, metadata, accepted_at
		)
		values ($1, $2, $3, $4, $5, $6, $7::jsonb, $8)
		on conflict (change_set_id)
		do update set
		  change_set_id = runtime_registry_change_sets.change_set_id
		returning change_set_id, reference, seed_id, realization_id, idempotency_key, accepted_by, metadata::text, accepted_at
	`, input.ChangeSetID, input.Reference, input.SeedID, input.RealizationID, input.IdempotencyKey, input.AcceptedBy, jsonBytes(input.Metadata), acceptedAt)
	changeSet, err := scanRegistryChangeSet(row)
	if err != nil {
		return AppendedRegistryChangeSet{}, wrapErr("append registry change set", err)
	}

	rows := make([]RegistryRow, 0, len(input.Rows))
	for i, item := range input.Rows {
		row := tx.QueryRow(ctx, `
			insert into runtime_registry_rows (
			  change_set_id, reference, seed_id, realization_id, row_order, row_type, object_id, claim_id, payload, accepted_at
			)
			values ($1, $2, $3, $4, $5, $6, $7, $8, $9::jsonb, $10)
			on conflict (change_set_id, row_order)
			do update set
			  change_set_id = runtime_registry_rows.change_set_id
			returning row_id, change_set_id, reference, seed_id, realization_id, row_order, row_type, object_id, claim_id, payload::text, accepted_at
		`, changeSet.ChangeSetID, changeSet.Reference, changeSet.SeedID, changeSet.RealizationID, i, strings.TrimSpace(item.RowType), strings.TrimSpace(item.ObjectID), strings.TrimSpace(item.ClaimID), jsonBytes(item.Payload), changeSet.AcceptedAt)
		appended, err := scanRegistryRow(row)
		if err != nil {
			return AppendedRegistryChangeSet{}, wrapErr("append registry row", err)
		}
		rows = append(rows, appended)
	}

	if err := tx.Commit(ctx); err != nil {
		return AppendedRegistryChangeSet{}, wrapErr("commit registry change set", err)
	}
	return AppendedRegistryChangeSet{
		ChangeSet: changeSet,
		Rows:      rows,
	}, nil
}

func (s *RuntimeService) GetRegistryChangeSet(ctx context.Context, changeSetID string) (RegistryChangeSet, error) {
	pool, err := expectReady(s)
	if err != nil {
		return RegistryChangeSet{}, err
	}
	row := pool.QueryRow(ctx, `
		select change_set_id, reference, seed_id, realization_id, idempotency_key, accepted_by, metadata::text, accepted_at
		from runtime_registry_change_sets
		where change_set_id = $1
	`, strings.TrimSpace(changeSetID))
	item, err := scanRegistryChangeSet(row)
	return item, wrapErr("get registry change set", err)
}

func (s *RuntimeService) ListRegistryChangeSets(ctx context.Context, reference string, limit int) ([]RegistryChangeSet, error) {
	pool, err := expectReady(s)
	if err != nil {
		return nil, err
	}
	limit = clampLimit(limit, 100, 500)
	reference = strings.TrimSpace(reference)
	rows, err := pool.Query(ctx, `
		select change_set_id, reference, seed_id, realization_id, idempotency_key, accepted_by, metadata::text, accepted_at
		from runtime_registry_change_sets
		where ($1 = '' or reference = $1)
		order by accepted_at desc, change_set_id desc
		limit $2
	`, reference, limit)
	if err != nil {
		return nil, wrapErr("list registry change sets", err)
	}
	defer rows.Close()

	var items []RegistryChangeSet
	for rows.Next() {
		item, err := scanRegistryChangeSet(rows)
		if err != nil {
			return nil, wrapErr("scan registry change set", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, wrapErr("iterate registry change sets", err)
	}
	return items, nil
}

func (s *RuntimeService) ListRegistryRows(ctx context.Context, input ListRegistryRowsInput) ([]RegistryRow, error) {
	pool, err := expectReady(s)
	if err != nil {
		return nil, err
	}
	return s.listRegistryRowsWithQuery(ctx, pool, input)
}

type registryQueryer interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

func (s *RuntimeService) listRegistryRowsWithQuery(ctx context.Context, queryer registryQueryer, input ListRegistryRowsInput) ([]RegistryRow, error) {
	input.Reference = strings.TrimSpace(input.Reference)
	input.SeedID = strings.TrimSpace(input.SeedID)
	input.RealizationID = strings.TrimSpace(input.RealizationID)
	input.Limit = clampLimit(input.Limit, 500, 5000)
	query := `
		select row_id, change_set_id, reference, seed_id, realization_id, row_order, row_type, object_id, claim_id, payload::text, accepted_at
		from runtime_registry_rows
		where ($1 = '' or reference = $1)
		  and ($2 = '' or seed_id = $2)
		  and ($3 = '' or realization_id = $3)
		  and row_id > $4
	`
	args := []any{input.Reference, input.SeedID, input.RealizationID, input.AfterRowID}
	query += `
		order by row_id asc
		limit $5
	`
	args = append(args, input.Limit)
	rows, err := queryer.Query(ctx, query, args...)
	if err != nil {
		return nil, wrapErr("list registry rows", err)
	}
	defer rows.Close()

	items := make([]RegistryRow, 0, input.Limit)
	for rows.Next() {
		item, err := scanRegistryRow(rows)
		if err != nil {
			return nil, wrapErr("scan registry row", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, wrapErr("iterate registry rows", err)
	}
	return items, nil
}

func (s *RuntimeService) GetRegistryRow(ctx context.Context, rowID int64) (RegistryRow, error) {
	pool, err := expectReady(s)
	if err != nil {
		return RegistryRow{}, err
	}
	row := pool.QueryRow(ctx, `
		select row_id, change_set_id, reference, seed_id, realization_id, row_order, row_type, object_id, claim_id, payload::text, accepted_at
		from runtime_registry_rows
		where row_id = $1
	`, rowID)
	item, err := scanRegistryRow(row)
	return item, wrapErr("get registry row", err)
}

func scanRegistryChangeSet(row rowScanner) (RegistryChangeSet, error) {
	var item RegistryChangeSet
	var metadata string
	err := row.Scan(
		&item.ChangeSetID,
		&item.Reference,
		&item.SeedID,
		&item.RealizationID,
		&item.IdempotencyKey,
		&item.AcceptedBy,
		&metadata,
		&item.AcceptedAt,
	)
	if err != nil {
		return RegistryChangeSet{}, rowNotFound(err)
	}
	item.Metadata = parseJSON(metadata)
	return item, nil
}

func scanRegistryRow(row rowScanner) (RegistryRow, error) {
	var item RegistryRow
	var payload string
	err := row.Scan(
		&item.RowID,
		&item.ChangeSetID,
		&item.Reference,
		&item.SeedID,
		&item.RealizationID,
		&item.RowOrder,
		&item.RowType,
		&item.ObjectID,
		&item.ClaimID,
		&payload,
		&item.AcceptedAt,
	)
	if err != nil {
		return RegistryRow{}, rowNotFound(err)
	}
	item.Payload = parseJSON(payload)
	return item, nil
}
