-- +goose Up
create extension if not exists pgcrypto;
create extension if not exists citext;
create extension if not exists pg_trgm;

create table if not exists users (
    id uuid primary key default gen_random_uuid(),
    email citext not null,
    username citext not null,
    password_hash text not null,
    display_name text not null default '',
    avatar_url text not null default '',
    bio text not null default '',
    status text not null default 'active',
    last_login_at timestamptz null,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    deleted_at timestamptz null
);

create unique index if not exists users_email_active_idx on users(email) where deleted_at is null;
create unique index if not exists users_username_active_idx on users(username) where deleted_at is null;
create index if not exists users_status_idx on users(status);

create table if not exists roles (
    id uuid primary key default gen_random_uuid(),
    code text not null unique,
    name text not null,
    description text not null default '',
    is_system boolean not null default false,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

create table if not exists permissions (
    id uuid primary key default gen_random_uuid(),
    code text not null unique,
    name text not null,
    resource text not null,
    action text not null,
    created_at timestamptz not null default now()
);

create table if not exists user_roles (
    user_id uuid not null references users(id) on delete cascade,
    role_id uuid not null references roles(id) on delete cascade,
    created_at timestamptz not null default now(),
    primary key (user_id, role_id)
);

create table if not exists role_permissions (
    role_id uuid not null references roles(id) on delete cascade,
    permission_id uuid not null references permissions(id) on delete cascade,
    created_at timestamptz not null default now(),
    primary key (role_id, permission_id)
);

create table if not exists refresh_tokens (
    id uuid primary key default gen_random_uuid(),
    user_id uuid not null references users(id) on delete cascade,
    token_hash text not null unique,
    device_id text not null default '',
    user_agent text not null default '',
    ip_hash text not null default '',
    expires_at timestamptz not null,
    revoked_at timestamptz null,
    created_at timestamptz not null default now()
);

create index if not exists refresh_tokens_user_expires_idx on refresh_tokens(user_id, expires_at);
create index if not exists refresh_tokens_revoked_idx on refresh_tokens(revoked_at);

create table if not exists posts (
    id uuid primary key default gen_random_uuid(),
    title text not null,
    slug text not null,
    summary text not null default '',
    content_md text not null default '',
    status text not null default 'draft',
    visibility text not null default 'public',
    allow_comment boolean not null default true,
    published_at timestamptz null,
    author_id uuid null references users(id) on delete set null,
    seo_title text not null default '',
    seo_description text not null default '',
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    deleted_at timestamptz null
);

create unique index if not exists posts_slug_active_idx on posts(slug) where deleted_at is null;
create index if not exists posts_status_published_idx on posts(status, published_at desc);
create index if not exists posts_author_created_idx on posts(author_id, created_at desc);

create table if not exists site_settings (
    id uuid primary key default gen_random_uuid(),
    key text not null unique,
    value_json jsonb not null,
    description text not null default '',
    is_public boolean not null default false,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

create table if not exists audit_logs (
    id uuid primary key default gen_random_uuid(),
    actor_id uuid null references users(id) on delete set null,
    action text not null,
    resource_type text not null,
    resource_id uuid null,
    before_json jsonb null,
    after_json jsonb null,
    ip_hash text not null default '',
    user_agent text not null default '',
    result text not null default 'success',
    request_id text not null default '',
    created_at timestamptz not null default now()
);

create index if not exists audit_logs_actor_created_idx on audit_logs(actor_id, created_at desc);
create index if not exists audit_logs_resource_idx on audit_logs(resource_type, resource_id);
create index if not exists audit_logs_action_created_idx on audit_logs(action, created_at desc);

-- +goose Down
drop table if exists audit_logs;
drop table if exists site_settings;
drop table if exists posts;
drop table if exists refresh_tokens;
drop table if exists role_permissions;
drop table if exists user_roles;
drop table if exists permissions;
drop table if exists roles;
drop table if exists users;
