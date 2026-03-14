create table if not exists runtime_upload_blobs (
  upload_id text primary key,
  storage_key text not null unique,
  content_type text,
  byte_size bigint not null default 0,
  sha256 text,
  status text not null default 'staged',
  visibility text not null default 'private',
  uploader_principal_id text,
  session_id text,
  metadata jsonb not null default '{}'::jsonb,
  created_at timestamptz not null default now(),
  finalized_at timestamptz
);

create index if not exists runtime_upload_blobs_status_idx
  on runtime_upload_blobs (status, created_at desc);

create table if not exists runtime_file_references (
  file_reference_id text primary key,
  upload_id text not null references runtime_upload_blobs(upload_id) on delete cascade,
  subject_kind text not null,
  subject_id text not null,
  usage_kind text not null,
  metadata jsonb not null default '{}'::jsonb,
  created_at timestamptz not null default now()
);

create unique index if not exists runtime_file_references_unique_idx
  on runtime_file_references (upload_id, subject_kind, subject_id, usage_kind);
