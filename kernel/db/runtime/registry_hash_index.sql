create table if not exists runtime_registry_hash_index (
  content_hash text primary key,
  resource_kind text not null,
  canonical_url text not null,
  permalink_url text not null,
  updated_at timestamptz not null default now()
);

create index if not exists runtime_registry_hash_index_updated_idx
  on runtime_registry_hash_index (updated_at desc);
