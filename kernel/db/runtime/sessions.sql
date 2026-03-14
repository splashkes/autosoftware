create table if not exists runtime_principals (
  principal_id text primary key,
  kind text not null,
  display_name text,
  status text not null default 'active',
  profile jsonb not null default '{}'::jsonb,
  created_at timestamptz not null default now(),
  deactivated_at timestamptz
);

create index if not exists runtime_principals_kind_idx
  on runtime_principals (kind, created_at desc);

create table if not exists runtime_principal_identifiers (
  identifier_id text primary key,
  principal_id text not null references runtime_principals(principal_id) on delete cascade,
  identifier_type text not null,
  value text not null,
  normalized_value text,
  is_primary boolean not null default false,
  is_verified boolean not null default false,
  verified_at timestamptz,
  metadata jsonb not null default '{}'::jsonb,
  created_at timestamptz not null default now(),
  unique (identifier_type, normalized_value)
);

create index if not exists runtime_principal_identifiers_principal_idx
  on runtime_principal_identifiers (principal_id, identifier_type);

create table if not exists runtime_principal_memberships (
  membership_id text primary key,
  principal_id text not null references runtime_principals(principal_id) on delete cascade,
  scope_kind text not null,
  scope_id text not null,
  role text not null,
  status text not null default 'active',
  metadata jsonb not null default '{}'::jsonb,
  created_at timestamptz not null default now(),
  ended_at timestamptz
);

create index if not exists runtime_principal_memberships_scope_idx
  on runtime_principal_memberships (scope_kind, scope_id, status);

create index if not exists runtime_principal_memberships_principal_idx
  on runtime_principal_memberships (principal_id, status);

create table if not exists runtime_sessions (
  session_id text primary key,
  principal_id text references runtime_principals(principal_id) on delete set null,
  status text not null default 'active',
  auth_context jsonb not null default '{}'::jsonb,
  user_agent text,
  ip_address text,
  started_at timestamptz not null default now(),
  last_seen_at timestamptz,
  expires_at timestamptz,
  ended_at timestamptz
);

create index if not exists runtime_sessions_principal_idx
  on runtime_sessions (principal_id, status, started_at desc);

create table if not exists runtime_session_events (
  event_id text primary key,
  session_id text not null references runtime_sessions(session_id) on delete cascade,
  name text not null,
  data jsonb not null default '{}'::jsonb,
  created_at timestamptz not null default now()
);

create index if not exists runtime_session_events_session_idx
  on runtime_session_events (session_id, created_at desc);

create table if not exists runtime_principal_consents (
  consent_id text primary key,
  principal_id text not null references runtime_principals(principal_id) on delete cascade,
  policy text not null,
  version text not null,
  accepted_at timestamptz not null default now(),
  evidence jsonb not null default '{}'::jsonb
);

create index if not exists runtime_principal_consents_principal_idx
  on runtime_principal_consents (principal_id, policy, accepted_at desc);
