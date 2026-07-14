-- +goose Up
create table if not exists series (
    id uuid primary key default gen_random_uuid(),
    name text not null,
    slug text not null,
    description text not null default '',
    cover_media_id uuid null references media_assets(id) on delete set null,
    sort_order integer not null default 0,
    enabled boolean not null default true,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    deleted_at timestamptz null,
    constraint series_sort_order_nonnegative check (sort_order >= 0)
);

create unique index if not exists series_slug_active_idx
    on series(slug) where deleted_at is null;
create index if not exists series_enabled_sort_idx
    on series(enabled, sort_order, name);
create index if not exists series_cover_media_id_idx
    on series(cover_media_id) where cover_media_id is not null;

alter table posts
    add column if not exists series_id uuid null,
    add column if not exists series_order integer null;

alter table posts
    add constraint posts_series_pair_check check (
        (series_id is null and series_order is null)
        or (series_id is not null and series_order is not null and series_order > 0)
    ),
    add constraint posts_series_id_fkey foreign key (series_id)
        references series(id) on delete restrict;

create index if not exists posts_series_id_idx on posts(series_id);
create unique index if not exists posts_series_order_active_idx
    on posts(series_id, series_order)
    where deleted_at is null and series_id is not null;

-- +goose Down
drop index if exists posts_series_order_active_idx;
drop index if exists posts_series_id_idx;
alter table posts drop constraint if exists posts_series_id_fkey;
alter table posts drop constraint if exists posts_series_pair_check;
alter table posts drop column if exists series_order;
alter table posts drop column if exists series_id;
drop index if exists series_cover_media_id_idx;
drop index if exists series_enabled_sort_idx;
drop index if exists series_slug_active_idx;
drop table if exists series;
