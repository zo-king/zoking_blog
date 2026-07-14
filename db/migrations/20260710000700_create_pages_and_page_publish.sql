-- +goose Up
create table if not exists pages (
    id uuid primary key default gen_random_uuid(),
    title text not null,
    slug text not null,
    summary text not null default '',
    content_md text not null default '',
    status text not null default 'draft',
    visibility text not null default 'public',
    show_in_menu boolean not null default false,
    menu_weight integer not null default 0,
    menu_icon text not null default '',
    allow_comment boolean not null default false,
    published_at timestamptz null,
    author_id uuid null references users(id) on delete set null,
    seo_title text not null default '',
    seo_description text not null default '',
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    deleted_at timestamptz null
);

create unique index if not exists pages_slug_active_idx on pages(slug) where deleted_at is null;
create index if not exists pages_status_menu_idx on pages(status, show_in_menu, menu_weight);
create index if not exists pages_author_created_idx on pages(author_id, created_at desc);

alter table publish_jobs add column if not exists page_id uuid null references pages(id) on delete set null;
create index if not exists publish_jobs_page_idx on publish_jobs(page_id);

alter table publish_releases add column if not exists page_id uuid null references pages(id) on delete set null;
create index if not exists publish_releases_page_idx on publish_releases(page_id);

-- +goose Down
drop index if exists publish_releases_page_idx;
alter table publish_releases drop column if exists page_id;

drop index if exists publish_jobs_page_idx;
alter table publish_jobs drop column if exists page_id;

drop table if exists pages;
