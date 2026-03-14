create table if not exists runtime_search_documents (
  document_id text primary key,
  subject_kind text not null,
  subject_id text not null,
  scope text not null default 'public',
  language text,
  title text,
  summary text,
  body_text text not null default '',
  facets jsonb not null default '{}'::jsonb,
  ranking jsonb not null default '{}'::jsonb,
  published_at timestamptz,
  sort_at timestamptz,
  updated_at timestamptz not null default now(),
  unique (subject_kind, subject_id, scope)
);

create index if not exists runtime_search_documents_scope_idx
  on runtime_search_documents (scope, updated_at desc);

create index if not exists runtime_search_documents_sort_idx
  on runtime_search_documents (scope, sort_at desc);
