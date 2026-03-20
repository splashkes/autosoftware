create table if not exists runtime_registry_change_sets (
  change_set_id text primary key,
  reference text not null,
  seed_id text not null,
  realization_id text not null,
  idempotency_key text not null default '',
  accepted_by text not null default 'system',
  metadata jsonb not null default '{}'::jsonb,
  accepted_at timestamptz not null default now()
);

create index if not exists runtime_registry_change_sets_reference_idx
  on runtime_registry_change_sets (reference, accepted_at desc);

create unique index if not exists runtime_registry_change_sets_reference_idempotency_idx
  on runtime_registry_change_sets (reference, idempotency_key)
  where idempotency_key <> '';

create table if not exists runtime_registry_rows (
  row_id bigint generated always as identity primary key,
  change_set_id text not null references runtime_registry_change_sets(change_set_id) on delete cascade,
  reference text not null,
  seed_id text not null,
  realization_id text not null,
  row_order integer not null default 0,
  row_type text not null,
  object_id text not null default '',
  claim_id text not null default '',
  payload jsonb not null default '{}'::jsonb,
  accepted_at timestamptz not null default now()
);

create index if not exists runtime_registry_rows_reference_row_idx
  on runtime_registry_rows (reference, row_id asc);

create index if not exists runtime_registry_rows_change_set_idx
  on runtime_registry_rows (change_set_id, row_order asc, row_id asc);

create unique index if not exists runtime_registry_rows_change_set_order_idx
  on runtime_registry_rows (change_set_id, row_order);

create index if not exists runtime_registry_rows_object_idx
  on runtime_registry_rows (reference, object_id, row_id asc)
  where object_id <> '';

create index if not exists runtime_registry_rows_claim_idx
  on runtime_registry_rows (reference, claim_id, row_id asc)
  where claim_id <> '';
