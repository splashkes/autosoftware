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
  reference text primary key,
  seed_id text not null,
  execution_id text references runtime_realization_executions(execution_id) on delete set null,
  activated_by_principal_id text,
  activated_by_session_id text,
  request_id text,
  metadata jsonb not null default '{}'::jsonb,
  activated_at timestamptz not null default now()
);

alter table runtime_realization_activation
  drop constraint if exists runtime_realization_activation_pkey;

alter table runtime_realization_activation
  add constraint runtime_realization_activation_pkey primary key (reference);

create index if not exists runtime_realization_activation_seed_idx
  on runtime_realization_activation (seed_id, activated_at desc);

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

create index if not exists runtime_realization_route_bindings_reference_idx
  on runtime_realization_route_bindings (reference, status, updated_at desc);

create table if not exists runtime_process_samples (
  sample_id text primary key,
  scope_kind text not null,
  service_name text,
  execution_id text references runtime_realization_executions(execution_id) on delete cascade,
  seed_id text,
  reference text,
  pid integer,
  cpu_percent double precision,
  rss_bytes bigint,
  virtual_bytes bigint,
  open_fds integer,
  log_bytes bigint,
  metadata jsonb not null default '{}'::jsonb,
  observed_at timestamptz not null default now()
);

create index if not exists runtime_process_samples_scope_idx
  on runtime_process_samples (scope_kind, observed_at desc);

create index if not exists runtime_process_samples_execution_idx
  on runtime_process_samples (execution_id, observed_at desc);

create index if not exists runtime_process_samples_service_idx
  on runtime_process_samples (service_name, observed_at desc);

create table if not exists runtime_service_events (
  event_id text primary key,
  service_name text not null,
  event_name text not null,
  severity text not null default 'info',
  message text,
  boot_id text,
  pid integer,
  request_id text,
  metadata jsonb not null default '{}'::jsonb,
  occurred_at timestamptz not null default now()
);

create index if not exists runtime_service_events_service_idx
  on runtime_service_events (service_name, occurred_at desc);

create index if not exists runtime_service_events_name_idx
  on runtime_service_events (event_name, occurred_at desc);

create table if not exists runtime_realization_suspensions (
  suspension_id text primary key,
  seed_id text not null,
  reference text not null,
  execution_id text references runtime_realization_executions(execution_id) on delete set null,
  route_subdomain text,
  route_path_prefix text,
  reason_code text not null,
  message text not null default '',
  remediation_target text not null default 'main',
  remediation_hint text not null default '',
  status text not null default 'active',
  metadata jsonb not null default '{}'::jsonb,
  created_at timestamptz not null default now(),
  cleared_at timestamptz
);

create unique index if not exists runtime_realization_suspensions_active_reference_idx
  on runtime_realization_suspensions (reference)
  where status = 'active' and cleared_at is null;

create index if not exists runtime_realization_suspensions_active_route_idx
  on runtime_realization_suspensions (status, route_subdomain, route_path_prefix, created_at desc);

create index if not exists runtime_realization_suspensions_execution_idx
  on runtime_realization_suspensions (execution_id, created_at desc);
