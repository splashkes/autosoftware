create table if not exists runtime_auth_providers (
  provider_id text primary key,
  kind text not null,
  issuer text,
  client_id text,
  status text not null default 'active',
  config jsonb not null default '{}'::jsonb,
  created_at timestamptz not null default now(),
  disabled_at timestamptz
);

create table if not exists runtime_auth_identities (
  identity_id text primary key,
  provider_id text not null,
  principal_id text not null,
  provider_subject text not null,
  profile jsonb not null default '{}'::jsonb,
  linked_at timestamptz not null default now(),
  last_seen_at timestamptz,
  unique (provider_id, provider_subject)
);

create index if not exists runtime_auth_identities_principal_idx
  on runtime_auth_identities (principal_id, linked_at desc);

create table if not exists runtime_auth_challenges (
  challenge_id text primary key,
  challenge_kind text not null,
  provider_id text,
  principal_id text,
  session_id text,
  delivery_target text,
  verifier_hash text,
  scope jsonb not null default '{}'::jsonb,
  status text not null default 'pending',
  expires_at timestamptz,
  used_at timestamptz,
  metadata jsonb not null default '{}'::jsonb,
  created_at timestamptz not null default now()
);

create index if not exists runtime_auth_challenges_status_idx
  on runtime_auth_challenges (status, expires_at);
