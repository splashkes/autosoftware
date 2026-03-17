create table if not exists runtime_realization_execution_logs (
  log_id text primary key,
  execution_id text not null references runtime_realization_executions(execution_id) on delete cascade,
  source text not null,
  stream text not null default '',
  message text not null default '',
  occurred_at timestamptz not null default now()
);

create index if not exists runtime_realization_execution_logs_execution_idx
  on runtime_realization_execution_logs (execution_id, occurred_at desc);

create index if not exists runtime_realization_execution_logs_source_idx
  on runtime_realization_execution_logs (source, occurred_at desc);
