-- +goose Up
create table if not exists publish_previews (
    id uuid primary key default gen_random_uuid(),
    preview_key text not null unique,
    scope text not null,
    status text not null default 'building',
    post_id uuid references posts(id) on delete set null,
    page_id uuid references pages(id) on delete set null,
    requested_by uuid references users(id) on delete set null,
    output_path text not null default '',
    entry_path text not null default '',
    url text not null default '',
    target_url text not null default '',
    settings_hash text not null default '',
    content_hash text not null default '',
    manifest_json jsonb not null default '{}'::jsonb,
    log_json jsonb not null default '[]'::jsonb,
    error_code text not null default '',
    error_message text not null default '',
    started_at timestamptz,
    finished_at timestamptz,
    expires_at timestamptz,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    deleted_at timestamptz
);

create index if not exists idx_publish_previews_status_created_at on publish_previews(status, created_at desc);
create index if not exists idx_publish_previews_expires_at on publish_previews(expires_at);
create index if not exists idx_publish_previews_post_id on publish_previews(post_id);
create index if not exists idx_publish_previews_page_id on publish_previews(page_id);

-- +goose Down
drop table if exists publish_previews;
