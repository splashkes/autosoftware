package interactions

import (
	"context"
	"errors"
	"strings"
)

func (s *RuntimeService) CreateRealizationExecution(ctx context.Context, input CreateRealizationExecutionInput) (RealizationExecution, error) {
	pool, err := expectReady(s)
	if err != nil {
		return RealizationExecution{}, err
	}
	if strings.TrimSpace(input.Reference) == "" || strings.TrimSpace(input.SeedID) == "" || strings.TrimSpace(input.RealizationID) == "" {
		return RealizationExecution{}, errors.New("reference, seed_id, and realization_id are required")
	}
	if strings.TrimSpace(input.Backend) == "" || strings.TrimSpace(input.Status) == "" {
		return RealizationExecution{}, errors.New("backend and status are required")
	}
	if strings.TrimSpace(input.ExecutionID) == "" {
		input.ExecutionID = newID("exec")
	}
	if strings.TrimSpace(input.Mode) == "" {
		input.Mode = "preview"
	}
	startedAt := s.nowUTC()
	if input.StartedAt != nil && !input.StartedAt.IsZero() {
		startedAt = input.StartedAt.UTC()
	}

	row := pool.QueryRow(ctx, `
		insert into runtime_realization_executions (
		  execution_id, reference, seed_id, realization_id, backend, mode, status,
		  route_subdomain, route_path_prefix, preview_path_prefix, upstream_addr,
		  execution_package_ref, launched_by_principal_id, launched_by_session_id,
		  request_id, metadata, started_at
		)
		values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16::jsonb, $17)
		returning execution_id, reference, seed_id, realization_id, backend, mode, status,
		          route_subdomain, route_path_prefix, preview_path_prefix, upstream_addr,
		          execution_package_ref, launched_by_principal_id, launched_by_session_id,
		          request_id, metadata::text, started_at, healthy_at, stopped_at, last_error
	`, input.ExecutionID, input.Reference, input.SeedID, input.RealizationID, input.Backend, input.Mode, input.Status,
		nullString(input.RouteSubdomain), nullString(input.RoutePathPrefix), nullString(input.PreviewPathPrefix),
		nullString(input.UpstreamAddr), nullString(input.ExecutionPackageRef), nullString(input.LaunchedByPrincipalID),
		nullString(input.LaunchedBySessionID), nullString(input.RequestID), jsonBytes(input.Metadata), startedAt)
	item, err := scanRealizationExecution(row)
	return item, wrapErr("create realization execution", err)
}

func (s *RuntimeService) UpdateRealizationExecution(ctx context.Context, executionID string, input UpdateRealizationExecutionInput) (RealizationExecution, error) {
	pool, err := expectReady(s)
	if err != nil {
		return RealizationExecution{}, err
	}
	executionID = strings.TrimSpace(executionID)
	if executionID == "" {
		return RealizationExecution{}, errors.New("execution_id is required")
	}

	row := pool.QueryRow(ctx, `
		update runtime_realization_executions
		set status = case when $2 <> '' then $2 else status end,
		    upstream_addr = case when $3 <> '' then $3 else upstream_addr end,
		    preview_path_prefix = case when $4 <> '' then $4 else preview_path_prefix end,
		    healthy_at = case
		      when $5::timestamptz is not null then $5::timestamptz
		      when $2 = 'healthy' and healthy_at is null then now()
		      else healthy_at
		    end,
		    stopped_at = case
		      when $6::timestamptz is not null then $6::timestamptz
		      when $2 in ('stopped', 'failed') and stopped_at is null then now()
		      else stopped_at
		    end,
		    last_error = case when $7 <> '' then $7 else last_error end,
		    metadata = case
		      when $8::jsonb = '{}'::jsonb then metadata
		      else metadata || $8::jsonb
		    end
		where execution_id = $1
		returning execution_id, reference, seed_id, realization_id, backend, mode, status,
		          route_subdomain, route_path_prefix, preview_path_prefix, upstream_addr,
		          execution_package_ref, launched_by_principal_id, launched_by_session_id,
		          request_id, metadata::text, started_at, healthy_at, stopped_at, last_error
	`, executionID, strings.TrimSpace(input.Status), strings.TrimSpace(input.UpstreamAddr), strings.TrimSpace(input.PreviewPathPrefix),
		nullTimeValue(input.HealthyAt), nullTimeValue(input.StoppedAt), strings.TrimSpace(input.LastError), jsonBytes(input.Metadata))
	item, err := scanRealizationExecution(row)
	return item, wrapErr("update realization execution", err)
}

func (s *RuntimeService) GetRealizationExecution(ctx context.Context, executionID string) (RealizationExecution, error) {
	pool, err := expectReady(s)
	if err != nil {
		return RealizationExecution{}, err
	}
	row := pool.QueryRow(ctx, `
		select execution_id, reference, seed_id, realization_id, backend, mode, status,
		       route_subdomain, route_path_prefix, preview_path_prefix, upstream_addr,
		       execution_package_ref, launched_by_principal_id, launched_by_session_id,
		       request_id, metadata::text, started_at, healthy_at, stopped_at, last_error
		from runtime_realization_executions
		where execution_id = $1
	`, strings.TrimSpace(executionID))
	item, err := scanRealizationExecution(row)
	return item, wrapErr("get realization execution", err)
}

func (s *RuntimeService) ListRealizationExecutions(ctx context.Context, reference string, limit int) ([]RealizationExecution, error) {
	pool, err := expectReady(s)
	if err != nil {
		return nil, err
	}
	limit = clampLimit(limit, 50, 200)
	reference = strings.TrimSpace(reference)

	rows, err := pool.Query(ctx, `
		select execution_id, reference, seed_id, realization_id, backend, mode, status,
		       route_subdomain, route_path_prefix, preview_path_prefix, upstream_addr,
		       execution_package_ref, launched_by_principal_id, launched_by_session_id,
		       request_id, metadata::text, started_at, healthy_at, stopped_at, last_error
		from runtime_realization_executions
		where ($1 = '' or reference = $1)
		order by started_at desc
		limit $2
	`, reference, limit)
	if err != nil {
		return nil, wrapErr("list realization executions", err)
	}
	defer rows.Close()

	var items []RealizationExecution
	for rows.Next() {
		item, err := scanRealizationExecution(rows)
		if err != nil {
			return nil, wrapErr("scan realization execution", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, wrapErr("iterate realization executions", err)
	}
	return items, nil
}

func (s *RuntimeService) RecordRealizationExecutionEvent(ctx context.Context, input RecordRealizationExecutionEventInput) (RealizationExecutionEvent, error) {
	pool, err := expectReady(s)
	if err != nil {
		return RealizationExecutionEvent{}, err
	}
	if strings.TrimSpace(input.ExecutionID) == "" || strings.TrimSpace(input.Name) == "" {
		return RealizationExecutionEvent{}, errors.New("execution_id and name are required")
	}
	if strings.TrimSpace(input.EventID) == "" {
		input.EventID = newID("evt")
	}
	occurredAt := s.nowUTC()
	if input.OccurredAt != nil && !input.OccurredAt.IsZero() {
		occurredAt = input.OccurredAt.UTC()
	}
	row := pool.QueryRow(ctx, `
		insert into runtime_realization_execution_events (event_id, execution_id, name, data, occurred_at)
		values ($1, $2, $3, $4::jsonb, $5)
		returning event_id, execution_id, name, data::text, occurred_at
	`, input.EventID, input.ExecutionID, input.Name, jsonBytes(input.Data), occurredAt)
	item, err := scanRealizationExecutionEvent(row)
	return item, wrapErr("record realization execution event", err)
}

func (s *RuntimeService) ListRealizationExecutionEvents(ctx context.Context, executionID string, limit int) ([]RealizationExecutionEvent, error) {
	pool, err := expectReady(s)
	if err != nil {
		return nil, err
	}
	limit = clampLimit(limit, 50, 200)
	rows, err := pool.Query(ctx, `
		select event_id, execution_id, name, data::text, occurred_at
		from runtime_realization_execution_events
		where execution_id = $1
		order by occurred_at desc
		limit $2
	`, strings.TrimSpace(executionID), limit)
	if err != nil {
		return nil, wrapErr("list realization execution events", err)
	}
	defer rows.Close()

	var items []RealizationExecutionEvent
	for rows.Next() {
		item, err := scanRealizationExecutionEvent(rows)
		if err != nil {
			return nil, wrapErr("scan realization execution event", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, wrapErr("iterate realization execution events", err)
	}
	return items, nil
}

func (s *RuntimeService) ActivateRealization(ctx context.Context, input ActivateRealizationInput) (RealizationActivation, error) {
	pool, err := expectReady(s)
	if err != nil {
		return RealizationActivation{}, err
	}
	if strings.TrimSpace(input.SeedID) == "" || strings.TrimSpace(input.Reference) == "" || strings.TrimSpace(input.ExecutionID) == "" {
		return RealizationActivation{}, errors.New("seed_id, reference, and execution_id are required")
	}
	row := pool.QueryRow(ctx, `
		insert into runtime_realization_activation (
		  seed_id, reference, execution_id, activated_by_principal_id,
		  activated_by_session_id, request_id, metadata, activated_at
		)
		values ($1, $2, $3, $4, $5, $6, $7::jsonb, now())
		on conflict (seed_id)
		do update set
		  reference = excluded.reference,
		  execution_id = excluded.execution_id,
		  activated_by_principal_id = excluded.activated_by_principal_id,
		  activated_by_session_id = excluded.activated_by_session_id,
		  request_id = excluded.request_id,
		  metadata = excluded.metadata,
		  activated_at = excluded.activated_at
		returning seed_id, reference, execution_id, activated_by_principal_id,
		          activated_by_session_id, request_id, metadata::text, activated_at
	`, input.SeedID, input.Reference, input.ExecutionID, nullString(input.ActivatedByPrincipalID),
		nullString(input.ActivatedBySessionID), nullString(input.RequestID), jsonBytes(input.Metadata))
	item, err := scanRealizationActivation(row)
	return item, wrapErr("activate realization", err)
}

func (s *RuntimeService) GetRealizationActivation(ctx context.Context, seedID string) (RealizationActivation, error) {
	pool, err := expectReady(s)
	if err != nil {
		return RealizationActivation{}, err
	}
	row := pool.QueryRow(ctx, `
		select seed_id, reference, execution_id, activated_by_principal_id,
		       activated_by_session_id, request_id, metadata::text, activated_at
		from runtime_realization_activation
		where seed_id = $1
	`, strings.TrimSpace(seedID))
	item, err := scanRealizationActivation(row)
	return item, wrapErr("get realization activation", err)
}

func (s *RuntimeService) DeleteRealizationActivation(ctx context.Context, seedID string) error {
	pool, err := expectReady(s)
	if err != nil {
		return err
	}
	_, err = pool.Exec(ctx, `delete from runtime_realization_activation where seed_id = $1`, strings.TrimSpace(seedID))
	return wrapErr("delete realization activation", err)
}

func (s *RuntimeService) ReplaceRealizationRouteBindings(ctx context.Context, executionID string, bindings []RealizationRouteBindingInput) ([]RealizationRouteBinding, error) {
	pool, err := expectReady(s)
	if err != nil {
		return nil, err
	}
	executionID = strings.TrimSpace(executionID)
	if executionID == "" {
		return nil, errors.New("execution_id is required")
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return nil, wrapErr("begin replace realization route bindings", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `delete from runtime_realization_route_bindings where execution_id = $1`, executionID); err != nil {
		return nil, wrapErr("delete realization route bindings", err)
	}

	items := make([]RealizationRouteBinding, 0, len(bindings))
	for _, input := range bindings {
		if strings.TrimSpace(input.BindingID) == "" {
			input.BindingID = newID("rb")
		}
		if strings.TrimSpace(input.Status) == "" {
			input.Status = "active"
		}
		row := tx.QueryRow(ctx, `
			insert into runtime_realization_route_bindings (
			  binding_id, execution_id, seed_id, reference, binding_kind,
			  subdomain, path_prefix, upstream_addr, status, metadata, created_at, updated_at
			)
			values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10::jsonb, now(), now())
			returning binding_id, execution_id, seed_id, reference, binding_kind,
			          subdomain, path_prefix, upstream_addr, status, metadata::text,
			          created_at, updated_at
		`, input.BindingID, executionID, input.SeedID, input.Reference, input.BindingKind,
			nullString(input.Subdomain), nullString(input.PathPrefix), input.UpstreamAddr,
			input.Status, jsonBytes(input.Metadata))
		item, err := scanRealizationRouteBinding(row)
		if err != nil {
			return nil, wrapErr("insert realization route binding", err)
		}
		items = append(items, item)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, wrapErr("commit realization route bindings", err)
	}
	return items, nil
}

func (s *RuntimeService) DeleteStableRouteBindingsForSeed(ctx context.Context, seedID string) error {
	pool, err := expectReady(s)
	if err != nil {
		return err
	}
	_, err = pool.Exec(ctx, `
		delete from runtime_realization_route_bindings
		where seed_id = $1 and binding_kind in ('stable_subdomain', 'stable_path')
	`, strings.TrimSpace(seedID))
	return wrapErr("delete stable route bindings", err)
}

func (s *RuntimeService) DeleteRealizationRouteBindings(ctx context.Context, executionID string) error {
	pool, err := expectReady(s)
	if err != nil {
		return err
	}
	_, err = pool.Exec(ctx, `delete from runtime_realization_route_bindings where execution_id = $1`, strings.TrimSpace(executionID))
	return wrapErr("delete realization route bindings", err)
}

func (s *RuntimeService) ListRealizationRouteBindings(ctx context.Context, activeOnly bool) ([]RealizationRouteBinding, error) {
	pool, err := expectReady(s)
	if err != nil {
		return nil, err
	}
	rows, err := pool.Query(ctx, `
		select binding_id, execution_id, seed_id, reference, binding_kind,
		       subdomain, path_prefix, upstream_addr, status, metadata::text,
		       created_at, updated_at
		from runtime_realization_route_bindings
		where (not $1 or status = 'active')
		order by char_length(coalesce(path_prefix, '')) desc, updated_at desc
	`, activeOnly)
	if err != nil {
		return nil, wrapErr("list realization route bindings", err)
	}
	defer rows.Close()

	var items []RealizationRouteBinding
	for rows.Next() {
		item, err := scanRealizationRouteBinding(rows)
		if err != nil {
			return nil, wrapErr("scan realization route binding", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, wrapErr("iterate realization route bindings", err)
	}
	return items, nil
}

func (s *RuntimeService) ResetBackendExecutions(ctx context.Context, backend, reason string) error {
	pool, err := expectReady(s)
	if err != nil {
		return err
	}
	backend = strings.TrimSpace(backend)
	if backend == "" {
		return errors.New("backend is required")
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return wrapErr("begin reset backend executions", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
		update runtime_realization_executions
		set status = 'stopped',
		    stopped_at = now(),
		    last_error = case when $2 <> '' then $2 else last_error end
		where backend = $1 and status in ('launch_requested', 'starting', 'healthy')
	`, backend, strings.TrimSpace(reason)); err != nil {
		return wrapErr("update backend executions", err)
	}
	if _, err := tx.Exec(ctx, `
		delete from runtime_realization_route_bindings
		where execution_id in (
		  select execution_id from runtime_realization_executions where backend = $1
		)
	`, backend); err != nil {
		return wrapErr("delete backend route bindings", err)
	}
	if _, err := tx.Exec(ctx, `
		delete from runtime_realization_activation
		where execution_id in (
		  select execution_id from runtime_realization_executions where backend = $1
		)
	`, backend); err != nil {
		return wrapErr("delete backend activations", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return wrapErr("commit reset backend executions", err)
	}
	return nil
}
