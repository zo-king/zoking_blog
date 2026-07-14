-- +goose Up
create table if not exists categories (
    id uuid primary key default gen_random_uuid(),
    name text not null,
    slug text not null,
    description text not null default '',
    parent_id uuid null references categories(id) on delete set null,
    sort_order integer not null default 0,
    enabled boolean not null default true,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    deleted_at timestamptz null
);

create unique index if not exists categories_slug_active_idx on categories(slug) where deleted_at is null;
create index if not exists categories_parent_idx on categories(parent_id);
create index if not exists categories_enabled_idx on categories(enabled);

create table if not exists tags (
    id uuid primary key default gen_random_uuid(),
    name text not null,
    slug text not null,
    description text not null default '',
    color text not null default '',
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    deleted_at timestamptz null
);

create unique index if not exists tags_slug_active_idx on tags(slug) where deleted_at is null;
create index if not exists tags_color_idx on tags(color);

create table if not exists post_categories (
    post_id uuid not null references posts(id) on delete cascade,
    category_id uuid not null references categories(id) on delete cascade,
    created_at timestamptz not null default now(),
    primary key (post_id, category_id)
);

create index if not exists post_categories_category_idx on post_categories(category_id);

create table if not exists post_tags (
    post_id uuid not null references posts(id) on delete cascade,
    tag_id uuid not null references tags(id) on delete cascade,
    created_at timestamptz not null default now(),
    primary key (post_id, tag_id)
);

create index if not exists post_tags_tag_idx on post_tags(tag_id);

-- +goose Down
drop table if exists post_tags;
drop table if exists post_categories;
drop table if exists tags;
drop table if exists categories;
