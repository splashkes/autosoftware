create table if not exists runtime_client_incidents (
  incident_id text primary key,
  request_id text,
  session_id text,
  seed_id text,
  realization_id text,
  route text,
  method text,
  page_url text,
  referrer text,
  user_agent text,
  kind text not null,
  severity text not null,
  message text not null,
  stack text,
  component_stack text,
  source text,
  tags jsonb not null default '{}'::jsonb,
  data jsonb not null default '{}'::jsonb,
  created_at timestamptz not null default now()
);

create index if not exists runtime_client_incidents_realization_idx
  on runtime_client_incidents (realization_id, created_at desc);

create index if not exists runtime_client_incidents_seed_idx
  on runtime_client_incidents (seed_id, created_at desc);

create index if not exists runtime_client_incidents_request_idx
  on runtime_client_incidents (request_id);

create table if not exists runtime_request_events (
  event_id text primary key,
  request_id text,
  session_id text,
  seed_id text,
  realization_id text,
  route text,
  method text,
  name text not null,
  status_code integer not null default 0,
  latency_ms bigint not null default 0,
  data jsonb not null default '{}'::jsonb,
  created_at timestamptz not null default now()
);

create index if not exists runtime_request_events_realization_idx
  on runtime_request_events (realization_id, created_at desc);

create table if not exists runtime_test_runs (
  test_run_id text primary key,
  seed_id text,
  realization_id text,
  suite text not null,
  status text not null,
  request_id text,
  session_id text,
  summary jsonb not null default '{}'::jsonb,
  started_at timestamptz not null,
  finished_at timestamptz
);

create index if not exists runtime_test_runs_realization_idx
  on runtime_test_runs (realization_id, started_at desc);

create table if not exists runtime_agent_reviews (
  review_id text primary key,
  seed_id text,
  realization_id text,
  reviewer text,
  status text not null,
  summary text,
  findings jsonb not null default '[]'::jsonb,
  request_id text,
  session_id text,
  created_at timestamptz not null default now()
);

create index if not exists runtime_agent_reviews_realization_idx
  on runtime_agent_reviews (realization_id, created_at desc);
