-- +goose Up
create table if not exists media_assets (
    id uuid primary key default gen_random_uuid(),
    filename text not null,
    original_name text not null,
    mime_type text not null,
    size_bytes bigint not null default 0,
    width integer not null default 0,
    height integer not null default 0,
    storage_driver text not null default 'local',
    storage_bucket text not null default '',
    storage_key text not null,
    public_url text not null,
    checksum text not null default '',
    uploaded_by uuid null references users(id) on delete set null,
    status text not null default 'ready',
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    deleted_at timestamptz null
);

create unique index if not exists media_assets_storage_key_idx on media_assets(storage_key);
create index if not exists media_assets_uploaded_by_created_idx on media_assets(uploaded_by, created_at desc);
create index if not exists media_assets_mime_type_idx on media_assets(mime_type);
create index if not exists media_assets_status_idx on media_assets(status);

create table if not exists media_usages (
    id uuid primary key default gen_random_uuid(),
    media_id uuid not null references media_assets(id) on delete cascade,
    resource_type text not null,
    resource_id uuid not null,
    usage_type text not null,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    deleted_at timestamptz null
);

create unique index if not exists media_usages_unique_idx on media_usages(media_id, resource_type, resource_id, usage_type) where deleted_at is null;
create index if not exists media_usages_resource_idx on media_usages(resource_type, resource_id);

-- +goose Down
drop table if exists media_usages;
drop table if exists media_assets;
