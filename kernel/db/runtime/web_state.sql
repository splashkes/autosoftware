create table if not exists runtime_handles (
  handle_id text primary key,
  namespace text not null,
  handle text not null,
  subject_kind text not null,
  subject_id text not null,
  status text not null default 'active',
  redirect_to_handle_id text references runtime_handles(handle_id),
  metadata jsonb not null default '{}'::jsonb,
  created_at timestamptz not null default now(),
  retired_at timestamptz,
  unique (namespace, handle)
);

create index if not exists runtime_handles_subject_idx
  on runtime_handles (subject_kind, subject_id, status);

create table if not exists runtime_access_links (
  access_link_id text primary key,
  token_hash text not null unique,
  subject_kind text not null,
  subject_id text not null,
  bound_principal_id text,
  scope jsonb not null default '{}'::jsonb,
  status text not null default 'active',
  max_uses integer,
  use_count integer not null default 0,
  expires_at timestamptz,
  last_used_at timestamptz,
  created_by_principal_id text,
  metadata jsonb not null default '{}'::jsonb,
  created_at timestamptz not null default now(),
  revoked_at timestamptz
);

create index if not exists runtime_access_links_subject_idx
  on runtime_access_links (subject_kind, subject_id, status);

create table if not exists runtime_publications (
  publication_id text primary key,
  subject_kind text not null,
  subject_id text not null,
  status text not null,
  visibility text not null default 'private',
  publish_at timestamptz,
  unpublish_at timestamptz,
  starts_at timestamptz,
  ends_at timestamptz,
  timezone text,
  all_day boolean not null default false,
  metadata jsonb not null default '{}'::jsonb,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  unique (subject_kind, subject_id)
);

create index if not exists runtime_publications_visibility_idx
  on runtime_publications (visibility, status, publish_at, starts_at, ends_at);

create table if not exists runtime_state_transitions (
  transition_id text primary key,
  subject_kind text not null,
  subject_id text not null,
  machine text not null,
  from_state text,
  to_state text not null,
  actor_principal_id text,
  actor_session_id text,
  request_id text,
  reason text,
  metadata jsonb not null default '{}'::jsonb,
  occurred_at timestamptz not null default now()
);

create index if not exists runtime_state_transitions_subject_idx
  on runtime_state_transitions (subject_kind, subject_id, occurred_at desc);

create table if not exists runtime_activity_events (
  activity_id text primary key,
  subject_kind text not null,
  subject_id text not null,
  actor_principal_id text,
  actor_session_id text,
  request_id text,
  name text not null,
  visibility text not null default 'internal',
  data jsonb not null default '{}'::jsonb,
  occurred_at timestamptz not null default now()
);

create index if not exists runtime_activity_events_subject_idx
  on runtime_activity_events (subject_kind, subject_id, occurred_at desc);

create table if not exists runtime_jobs (
  job_id text primary key,
  queue text not null default 'default',
  kind text not null,
  dedupe_key text,
  status text not null default 'pending',
  priority integer not null default 100,
  run_at timestamptz not null,
  locked_at timestamptz,
  locked_by text,
  attempts integer not null default 0,
  max_attempts integer not null default 10,
  payload jsonb not null default '{}'::jsonb,
  last_error text,
  created_at timestamptz not null default now(),
  finished_at timestamptz
);

create unique index if not exists runtime_jobs_dedupe_idx
  on runtime_jobs (queue, dedupe_key)
  where dedupe_key is not null and status in ('pending', 'running');

create index if not exists runtime_jobs_run_idx
  on runtime_jobs (status, run_at, priority desc);

create table if not exists runtime_outbox_messages (
  message_id text primary key,
  subject_kind text,
  subject_id text,
  recipient_principal_id text,
  recipient_address text,
  channel text not null,
  template text not null,
  dedupe_key text,
  status text not null default 'pending',
  enqueue_after timestamptz not null default now(),
  payload jsonb not null default '{}'::jsonb,
  created_at timestamptz not null default now(),
  sent_at timestamptz,
  canceled_at timestamptz
);

create unique index if not exists runtime_outbox_messages_dedupe_idx
  on runtime_outbox_messages (channel, dedupe_key)
  where dedupe_key is not null;

create index if not exists runtime_outbox_messages_status_idx
  on runtime_outbox_messages (status, enqueue_after);

create table if not exists runtime_outbox_attempts (
  attempt_id text primary key,
  message_id text not null references runtime_outbox_messages(message_id) on delete cascade,
  provider text,
  status text not null,
  response_code text,
  response_body text,
  attempted_at timestamptz not null default now()
);

create index if not exists runtime_outbox_attempts_message_idx
  on runtime_outbox_attempts (message_id, attempted_at desc);

create table if not exists runtime_idempotency_keys (
  key_id text primary key,
  namespace text not null,
  idempotency_key text not null,
  request_fingerprint text,
  response_status integer,
  response_body jsonb not null default '{}'::jsonb,
  created_at timestamptz not null default now(),
  expires_at timestamptz,
  unique (namespace, idempotency_key)
);

create index if not exists runtime_idempotency_keys_expires_idx
  on runtime_idempotency_keys (expires_at);
