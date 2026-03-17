package interactions

import (
	"context"
	"errors"
	"strings"
	"time"
)

func (s *RuntimeService) RecordProcessSample(ctx context.Context, input RecordProcessSampleInput) (ProcessSample, error) {
	pool, err := expectReady(s)
	if err != nil {
		return ProcessSample{}, err
	}
	if strings.TrimSpace(input.ScopeKind) == "" {
		return ProcessSample{}, errors.New("scope_kind is required")
	}
	if strings.TrimSpace(input.SampleID) == "" {
		input.SampleID = newID("ps")
	}
	observedAt := s.nowUTC()
	if input.ObservedAt != nil && !input.ObservedAt.IsZero() {
		observedAt = input.ObservedAt.UTC()
	}

	row := pool.QueryRow(ctx, `
		insert into runtime_process_samples (
		  sample_id, scope_kind, service_name, execution_id, seed_id, reference,
		  pid, cpu_percent, rss_bytes, virtual_bytes, open_fds, log_bytes, metadata, observed_at
		)
		values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13::jsonb, $14)
		returning sample_id, scope_kind, service_name, execution_id, seed_id, reference,
		          pid, cpu_percent, rss_bytes, virtual_bytes, open_fds, log_bytes, metadata::text, observed_at
	`, input.SampleID, input.ScopeKind, nullString(input.ServiceName), nullString(input.ExecutionID),
		nullString(input.SeedID), nullString(input.Reference), nullInt32(input.PID), nullFloat64(input.CPUPercent),
		nullInt64(input.RSSBytes), nullInt64(input.VirtualBytes), nullInt32(input.OpenFDs), nullInt64(input.LogBytes),
		jsonBytes(input.Metadata), observedAt)
	item, err := scanProcessSample(row)
	return item, wrapErr("record process sample", err)
}

func (s *RuntimeService) ListProcessSamples(ctx context.Context, input ListProcessSamplesInput) ([]ProcessSample, error) {
	pool, err := expectReady(s)
	if err != nil {
		return nil, err
	}
	limit := clampLimit(input.Limit, 50, 500)
	rows, err := pool.Query(ctx, `
		select sample_id, scope_kind, service_name, execution_id, seed_id, reference,
		       pid, cpu_percent, rss_bytes, virtual_bytes, open_fds, log_bytes, metadata::text, observed_at
		from runtime_process_samples
		where ($1 = '' or scope_kind = $1)
		  and ($2 = '' or service_name = $2)
		  and ($3 = '' or execution_id = $3)
		  and ($4 = '' or reference = $4)
		order by observed_at desc
		limit $5
	`, strings.TrimSpace(input.ScopeKind), strings.TrimSpace(input.ServiceName), strings.TrimSpace(input.ExecutionID),
		strings.TrimSpace(input.Reference), limit)
	if err != nil {
		return nil, wrapErr("list process samples", err)
	}
	defer rows.Close()

	var items []ProcessSample
	for rows.Next() {
		item, err := scanProcessSample(rows)
		if err != nil {
			return nil, wrapErr("scan process sample", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, wrapErr("iterate process samples", err)
	}
	return items, nil
}

func (s *RuntimeService) RecordServiceEvent(ctx context.Context, input RecordServiceEventInput) (ServiceEvent, error) {
	pool, err := expectReady(s)
	if err != nil {
		return ServiceEvent{}, err
	}
	if strings.TrimSpace(input.ServiceName) == "" || strings.TrimSpace(input.EventName) == "" {
		return ServiceEvent{}, errors.New("service_name and event_name are required")
	}
	if strings.TrimSpace(input.EventID) == "" {
		input.EventID = newID("svc")
	}
	severity := statusOrDefault(input.Severity, "info")
	occurredAt := s.nowUTC()
	if input.OccurredAt != nil && !input.OccurredAt.IsZero() {
		occurredAt = input.OccurredAt.UTC()
	}

	row := pool.QueryRow(ctx, `
		insert into runtime_service_events (
		  event_id, service_name, event_name, severity, message, boot_id, pid, request_id, metadata, occurred_at
		)
		values ($1, $2, $3, $4, $5, $6, $7, $8, $9::jsonb, $10)
		returning event_id, service_name, event_name, severity, message, boot_id, pid, request_id, metadata::text, occurred_at
	`, input.EventID, input.ServiceName, input.EventName, severity, nullString(input.Message), nullString(input.BootID),
		nullInt32(input.PID), nullString(input.RequestID), jsonBytes(input.Metadata), occurredAt)
	item, err := scanServiceEvent(row)
	return item, wrapErr("record service event", err)
}

func (s *RuntimeService) ListServiceEvents(ctx context.Context, input ListServiceEventsInput) ([]ServiceEvent, error) {
	pool, err := expectReady(s)
	if err != nil {
		return nil, err
	}
	limit := clampLimit(input.Limit, 50, 500)
	rows, err := pool.Query(ctx, `
		select event_id, service_name, event_name, severity, message, boot_id, pid, request_id, metadata::text, occurred_at
		from runtime_service_events
		where ($1 = '' or service_name = $1)
		  and ($2 = '' or event_name = $2)
		order by occurred_at desc
		limit $3
	`, strings.TrimSpace(input.ServiceName), strings.TrimSpace(input.EventName), limit)
	if err != nil {
		return nil, wrapErr("list service events", err)
	}
	defer rows.Close()

	var items []ServiceEvent
	for rows.Next() {
		item, err := scanServiceEvent(rows)
		if err != nil {
			return nil, wrapErr("scan service event", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, wrapErr("iterate service events", err)
	}
	return items, nil
}

func (s *RuntimeService) UpsertRealizationSuspension(ctx context.Context, input UpsertRealizationSuspensionInput) (RealizationSuspension, error) {
	pool, err := expectReady(s)
	if err != nil {
		return RealizationSuspension{}, err
	}
	if strings.TrimSpace(input.SeedID) == "" || strings.TrimSpace(input.Reference) == "" || strings.TrimSpace(input.ReasonCode) == "" {
		return RealizationSuspension{}, errors.New("seed_id, reference, and reason_code are required")
	}
	if strings.TrimSpace(input.SuspensionID) == "" {
		input.SuspensionID = newID("susp")
	}
	if strings.TrimSpace(input.Status) == "" {
		input.Status = "active"
	}
	if strings.TrimSpace(input.RemediationTarget) == "" {
		input.RemediationTarget = "main"
	}
	createdAt := s.nowUTC()
	if input.CreatedAt != nil && !input.CreatedAt.IsZero() {
		createdAt = input.CreatedAt.UTC()
	}

	row := pool.QueryRow(ctx, `
		insert into runtime_realization_suspensions (
		  suspension_id, seed_id, reference, execution_id, route_subdomain, route_path_prefix,
		  reason_code, message, remediation_target, remediation_hint, status, metadata, created_at, cleared_at
		)
		values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12::jsonb, $13, null)
		on conflict (reference) where status = 'active' and cleared_at is null
		do update set
		  execution_id = excluded.execution_id,
		  route_subdomain = excluded.route_subdomain,
		  route_path_prefix = excluded.route_path_prefix,
		  reason_code = excluded.reason_code,
		  message = excluded.message,
		  remediation_target = excluded.remediation_target,
		  remediation_hint = excluded.remediation_hint,
		  status = excluded.status,
		  metadata = excluded.metadata,
		  created_at = excluded.created_at,
		  cleared_at = null
		returning suspension_id, seed_id, reference, execution_id, route_subdomain, route_path_prefix,
		          reason_code, message, remediation_target, remediation_hint, status, metadata::text, created_at, cleared_at
	`, input.SuspensionID, input.SeedID, input.Reference, nullString(input.ExecutionID), nullString(input.RouteSubdomain),
		nullString(input.RoutePathPrefix), input.ReasonCode, input.Message, input.RemediationTarget, input.RemediationHint,
		input.Status, jsonBytes(input.Metadata), createdAt)
	item, err := scanRealizationSuspension(row)
	return item, wrapErr("upsert realization suspension", err)
}

func (s *RuntimeService) ClearRealizationSuspension(ctx context.Context, reference string) error {
	pool, err := expectReady(s)
	if err != nil {
		return err
	}
	_, err = pool.Exec(ctx, `
		update runtime_realization_suspensions
		set status = 'cleared',
		    cleared_at = now()
		where reference = $1 and status = 'active' and cleared_at is null
	`, strings.TrimSpace(reference))
	return wrapErr("clear realization suspension", err)
}

func (s *RuntimeService) GetActiveRealizationSuspension(ctx context.Context, reference string) (RealizationSuspension, error) {
	pool, err := expectReady(s)
	if err != nil {
		return RealizationSuspension{}, err
	}
	row := pool.QueryRow(ctx, `
		select suspension_id, seed_id, reference, execution_id, route_subdomain, route_path_prefix,
		       reason_code, message, remediation_target, remediation_hint, status, metadata::text, created_at, cleared_at
		from runtime_realization_suspensions
		where reference = $1 and status = 'active' and cleared_at is null
		order by created_at desc
		limit 1
	`, strings.TrimSpace(reference))
	item, err := scanRealizationSuspension(row)
	return item, wrapErr("get active realization suspension", err)
}

func (s *RuntimeService) GetRealizationSuspensionByExecution(ctx context.Context, executionID string) (RealizationSuspension, error) {
	pool, err := expectReady(s)
	if err != nil {
		return RealizationSuspension{}, err
	}
	row := pool.QueryRow(ctx, `
		select suspension_id, seed_id, reference, execution_id, route_subdomain, route_path_prefix,
		       reason_code, message, remediation_target, remediation_hint, status, metadata::text, created_at, cleared_at
		from runtime_realization_suspensions
		where execution_id = $1 and status = 'active' and cleared_at is null
		order by created_at desc
		limit 1
	`, strings.TrimSpace(executionID))
	item, err := scanRealizationSuspension(row)
	return item, wrapErr("get realization suspension by execution", err)
}

func (s *RuntimeService) ListRealizationSuspensions(ctx context.Context, input ListRealizationSuspensionsInput) ([]RealizationSuspension, error) {
	pool, err := expectReady(s)
	if err != nil {
		return nil, err
	}
	limit := clampLimit(input.Limit, 50, 200)
	rows, err := pool.Query(ctx, `
		select suspension_id, seed_id, reference, execution_id, route_subdomain, route_path_prefix,
		       reason_code, message, remediation_target, remediation_hint, status, metadata::text, created_at, cleared_at
		from runtime_realization_suspensions
		where ($1 = '' or reference = $1)
		  and ($2 = '' or execution_id = $2)
		  and (not $3 or (status = 'active' and cleared_at is null))
		order by created_at desc
		limit $4
	`, strings.TrimSpace(input.Reference), strings.TrimSpace(input.ExecutionID), input.ActiveOnly, limit)
	if err != nil {
		return nil, wrapErr("list realization suspensions", err)
	}
	defer rows.Close()

	var items []RealizationSuspension
	for rows.Next() {
		item, err := scanRealizationSuspension(rows)
		if err != nil {
			return nil, wrapErr("scan realization suspension", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, wrapErr("iterate realization suspensions", err)
	}
	return items, nil
}

func (s *RuntimeService) PruneOperationalTelemetry(ctx context.Context, ttl time.Duration) error {
	pool, err := expectReady(s)
	if err != nil {
		return err
	}
	if ttl <= 0 {
		return errors.New("ttl must be positive")
	}
	cutoff := s.nowUTC().Add(-ttl)

	tx, err := pool.Begin(ctx)
	if err != nil {
		return wrapErr("begin prune operational telemetry", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `delete from runtime_process_samples where observed_at < $1`, cutoff); err != nil {
		return wrapErr("prune process samples", err)
	}
	if _, err := tx.Exec(ctx, `delete from runtime_realization_execution_logs where occurred_at < $1`, cutoff); err != nil {
		return wrapErr("prune realization execution logs", err)
	}
	if _, err := tx.Exec(ctx, `delete from runtime_service_events where occurred_at < $1`, cutoff); err != nil {
		return wrapErr("prune service events", err)
	}
	if _, err := tx.Exec(ctx, `
		delete from runtime_realization_suspensions
		where cleared_at is not null and cleared_at < $1
	`, cutoff); err != nil {
		return wrapErr("prune cleared realization suspensions", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return wrapErr("commit prune operational telemetry", err)
	}
	return nil
}
