create table if not exists runtime_threads (
  thread_id text primary key,
  subject_kind text not null,
  subject_id text not null,
  thread_kind text not null default 'conversation',
  status text not null default 'open',
  visibility text not null default 'shared',
  title text,
  metadata jsonb not null default '{}'::jsonb,
  created_at timestamptz not null default now(),
  closed_at timestamptz
);

create index if not exists runtime_threads_subject_idx
  on runtime_threads (subject_kind, subject_id, status, created_at desc);

create table if not exists runtime_thread_participants (
  participant_id text primary key,
  thread_id text not null references runtime_threads(thread_id) on delete cascade,
  principal_id text,
  role text not null default 'participant',
  status text not null default 'active',
  delivery_policy jsonb not null default '{}'::jsonb,
  metadata jsonb not null default '{}'::jsonb,
  joined_at timestamptz not null default now(),
  left_at timestamptz
);

create index if not exists runtime_thread_participants_thread_idx
  on runtime_thread_participants (thread_id, status, joined_at);

create index if not exists runtime_thread_participants_principal_idx
  on runtime_thread_participants (principal_id, status);

create table if not exists runtime_messages (
  message_id text primary key,
  thread_id text not null references runtime_threads(thread_id) on delete cascade,
  author_principal_id text,
  author_session_id text,
  request_id text,
  kind text not null default 'message',
  visibility text not null default 'shared',
  body_format text not null default 'plain_text',
  body text not null,
  metadata jsonb not null default '{}'::jsonb,
  created_at timestamptz not null default now(),
  edited_at timestamptz,
  deleted_at timestamptz
);

create index if not exists runtime_messages_thread_idx
  on runtime_messages (thread_id, created_at desc);

create table if not exists runtime_message_cursors (
  cursor_id text primary key,
  thread_id text not null references runtime_threads(thread_id) on delete cascade,
  principal_id text not null,
  last_read_message_id text,
  last_read_at timestamptz,
  updated_at timestamptz not null default now(),
  unique (thread_id, principal_id)
);

create index if not exists runtime_message_cursors_principal_idx
  on runtime_message_cursors (principal_id, updated_at desc);

create table if not exists runtime_assignments (
  assignment_id text primary key,
  subject_kind text not null,
  subject_id text not null,
  principal_id text not null,
  role text not null default 'assignee',
  status text not null default 'active',
  assigned_by_principal_id text,
  metadata jsonb not null default '{}'::jsonb,
  created_at timestamptz not null default now(),
  ended_at timestamptz
);

create index if not exists runtime_assignments_subject_idx
  on runtime_assignments (subject_kind, subject_id, status, created_at desc);

create index if not exists runtime_assignments_principal_idx
  on runtime_assignments (principal_id, status, created_at desc);

create table if not exists runtime_subscriptions (
  subscription_id text primary key,
  subject_kind text not null,
  subject_id text not null,
  principal_id text not null,
  subscription_kind text not null default 'watch',
  channel text,
  status text not null default 'active',
  delivery_policy jsonb not null default '{}'::jsonb,
  metadata jsonb not null default '{}'::jsonb,
  created_at timestamptz not null default now(),
  ended_at timestamptz
);

create unique index if not exists runtime_subscriptions_unique_active_idx
  on runtime_subscriptions (subject_kind, subject_id, principal_id, subscription_kind, channel)
  where status = 'active';

create index if not exists runtime_subscriptions_principal_idx
  on runtime_subscriptions (principal_id, status, created_at desc);

create table if not exists runtime_notification_preferences (
  preference_id text primary key,
  principal_id text not null,
  topic text not null,
  channel text not null,
  status text not null default 'enabled',
  frequency text not null default 'immediate',
  quiet_hours jsonb not null default '{}'::jsonb,
  metadata jsonb not null default '{}'::jsonb,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  unique (principal_id, topic, channel)
);

create index if not exists runtime_notification_preferences_principal_idx
  on runtime_notification_preferences (principal_id, status, updated_at desc);
