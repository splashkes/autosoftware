create table if not exists runtime_realization_executions (
  execution_id text primary key,
  reference text not null,
  seed_id text not null,
  realization_id text not null,
  backend text not null,
  mode text not null default 'preview',
  status text not null,
  route_subdomain text,
  route_path_prefix text,
  preview_path_prefix text,
  upstream_addr text,
  execution_package_ref text,
  launched_by_principal_id text,
  launched_by_session_id text,
  request_id text,
  metadata jsonb not null default '{}'::jsonb,
  started_at timestamptz not null default now(),
  healthy_at timestamptz,
  stopped_at timestamptz,
  last_error text
);

create index if not exists runtime_realization_executions_reference_idx
  on runtime_realization_executions (reference, started_at desc);

create index if not exists runtime_realization_executions_seed_status_idx
  on runtime_realization_executions (seed_id, status, started_at desc);

create table if not exists runtime_realization_execution_events (
  event_id text primary key,
  execution_id text not null references runtime_realization_executions(execution_id) on delete cascade,
  name text not null,
  data jsonb not null default '{}'::jsonb,
  occurred_at timestamptz not null default now()
);

create index if not exists runtime_realization_execution_events_execution_idx
  on runtime_realization_execution_events (execution_id, occurred_at desc);

create table if not exists runtime_realization_activation (
  seed_id text primary key,
  reference text not null,
  execution_id text references runtime_realization_executions(execution_id) on delete set null,
  activated_by_principal_id text,
  activated_by_session_id text,
  request_id text,
  metadata jsonb not null default '{}'::jsonb,
  activated_at timestamptz not null default now()
);

create index if not exists runtime_realization_activation_reference_idx
  on runtime_realization_activation (reference, activated_at desc);

create table if not exists runtime_realization_route_bindings (
  binding_id text primary key,
  execution_id text not null references runtime_realization_executions(execution_id) on delete cascade,
  seed_id text not null,
  reference text not null,
  binding_kind text not null,
  subdomain text,
  path_prefix text,
  upstream_addr text not null,
  status text not null default 'active',
  metadata jsonb not null default '{}'::jsonb,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

create index if not exists runtime_realization_route_bindings_active_idx
  on runtime_realization_route_bindings (status, binding_kind, seed_id, updated_at desc);

create index if not exists runtime_realization_route_bindings_execution_idx
  on runtime_realization_route_bindings (execution_id, status, updated_at desc);
