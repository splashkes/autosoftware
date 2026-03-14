create table if not exists runtime_rate_limit_buckets (
  bucket_id text primary key,
  namespace text not null,
  subject_key text not null,
  window_started_at timestamptz not null,
  window_ends_at timestamptz not null,
  hit_count bigint not null default 0,
  blocked_until timestamptz,
  metadata jsonb not null default '{}'::jsonb,
  updated_at timestamptz not null default now(),
  unique (namespace, subject_key, window_started_at, window_ends_at)
);

create index if not exists runtime_rate_limit_buckets_lookup_idx
  on runtime_rate_limit_buckets (namespace, subject_key, window_ends_at desc);

create table if not exists runtime_guard_decisions (
  decision_id text primary key,
  namespace text not null,
  request_id text,
  session_id text,
  principal_id text,
  subject_key text,
  action text not null,
  outcome text not null,
  reason text,
  metadata jsonb not null default '{}'::jsonb,
  created_at timestamptz not null default now()
);

create index if not exists runtime_guard_decisions_lookup_idx
  on runtime_guard_decisions (namespace, outcome, created_at desc);

create table if not exists runtime_risk_events (
  risk_event_id text primary key,
  namespace text not null,
  subject_key text,
  request_id text,
  session_id text,
  principal_id text,
  kind text not null,
  severity text not null,
  status text not null default 'open',
  data jsonb not null default '{}'::jsonb,
  created_at timestamptz not null default now(),
  resolved_at timestamptz
);

create index if not exists runtime_risk_events_status_idx
  on runtime_risk_events (namespace, status, created_at desc);
