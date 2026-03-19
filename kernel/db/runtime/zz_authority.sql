create table if not exists runtime_authority_bundles (
  bundle_id text primary key,
  display_name text,
  capabilities text[] not null default '{}'::text[],
  status text not null default 'active',
  metadata jsonb not null default '{}'::jsonb,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  retired_at timestamptz
);

create index if not exists runtime_authority_bundles_status_idx
  on runtime_authority_bundles (status, updated_at desc);

create table if not exists runtime_authority_grants (
  grant_id text primary key,
  grantor_principal_id text references runtime_principals(principal_id) on delete set null,
  grantee_principal_id text not null references runtime_principals(principal_id) on delete cascade,
  bundle_id text not null references runtime_authority_bundles(bundle_id) on delete restrict,
  capabilities_snapshot text[] not null default '{}'::text[],
  scope_kind text not null,
  scope_id text not null,
  delegation_mode text not null default 'none',
  basis text not null default 'delegated',
  status text not null default 'accepted',
  effective_at timestamptz,
  expires_at timestamptz,
  supersedes_grant_id text references runtime_authority_grants(grant_id) on delete set null,
  reason text,
  evidence_refs text[] not null default '{}'::text[],
  metadata jsonb not null default '{}'::jsonb,
  created_at timestamptz not null default now()
);

create index if not exists runtime_authority_grants_grantee_idx
  on runtime_authority_grants (grantee_principal_id, created_at desc);

create index if not exists runtime_authority_grants_grantor_idx
  on runtime_authority_grants (grantor_principal_id, created_at desc);

create index if not exists runtime_authority_grants_scope_idx
  on runtime_authority_grants (scope_kind, scope_id, created_at desc);

create index if not exists runtime_authority_grants_supersedes_idx
  on runtime_authority_grants (supersedes_grant_id);
