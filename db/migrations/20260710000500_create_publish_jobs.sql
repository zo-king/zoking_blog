-- +goose Up
create table if not exists publish_jobs (
    id uuid primary key default gen_random_uuid(),
    post_id uuid null references posts(id) on delete set null,
    job_type text not null default 'post',
    status text not null default 'requested',
    trigger_source text not null default 'admin',
    requested_by uuid null references users(id) on delete set null,
    run_at timestamptz not null default now(),
    started_at timestamptz null,
    finished_at timestamptz null,
    snapshot_key text not null default '',
    release_key text not null default '',
    content_path text not null default '',
    output_path text not null default '',
    manifest_json jsonb null,
    log_json jsonb null,
    error_code text not null default '',
    error_message text not null default '',
    retry_count integer not null default 0,
    canceled_at timestamptz null,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    deleted_at timestamptz null
);

create index if not exists publish_jobs_status_run_idx on publish_jobs(status, run_at);
create index if not exists publish_jobs_post_created_idx on publish_jobs(post_id, created_at desc);
create index if not exists publish_jobs_release_key_idx on publish_jobs(release_key);

create table if not exists publish_releases (
    id uuid primary key default gen_random_uuid(),
    job_id uuid not null references publish_jobs(id) on delete cascade,
    release_key text not null unique,
    status text not null default 'active',
    post_id uuid null references posts(id) on delete set null,
    output_path text not null,
    manifest_json jsonb not null,
    is_active boolean not null default true,
    promoted_at timestamptz null,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    deleted_at timestamptz null
);

create unique index if not exists publish_releases_active_unique_idx on publish_releases(is_active) where is_active = true and deleted_at is null;
create index if not exists publish_releases_created_idx on publish_releases(created_at desc);

-- +goose Down
drop table if exists publish_releases;
drop table if exists publish_jobs;
