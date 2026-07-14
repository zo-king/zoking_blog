-- +goose Up
create table if not exists achievements (
    id uuid primary key default gen_random_uuid(),
    kind text not null,
    title text not null,
    organization text not null default '',
    summary text not null default '',
    occurred_at date not null,
    ended_at date null,
    external_url text not null default '',
    credential_id text not null default '',
    image_media_id uuid null references media_assets(id) on delete set null,
    sort_order integer not null default 0,
    status text not null default 'draft',
    published_at timestamptz null,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    deleted_at timestamptz null,
    constraint achievements_kind_allowed check (kind in ('award', 'certificate', 'project')),
    constraint achievements_occurred_at_minimum check (occurred_at >= date '2024-01-01'),
    constraint achievements_date_range_valid check (ended_at is null or ended_at >= occurred_at),
    constraint achievements_sort_order_nonnegative check (sort_order >= 0),
    constraint achievements_status_allowed check (status in ('draft', 'published', 'archived'))
);

create index if not exists achievements_timeline_idx
    on achievements(occurred_at desc, sort_order asc, id asc)
    where deleted_at is null;
create index if not exists achievements_status_timeline_idx
    on achievements(status, occurred_at desc, sort_order asc, id asc)
    where deleted_at is null;
create index if not exists achievements_image_media_id_idx
    on achievements(image_media_id) where image_media_id is not null;

-- +goose Down
drop index if exists achievements_image_media_id_idx;
drop index if exists achievements_status_timeline_idx;
drop index if exists achievements_timeline_idx;
drop table if exists achievements;
