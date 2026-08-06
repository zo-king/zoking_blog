-- +goose Up
-- Per-post view / like counters for the reader site (plugin: 浏览量/点赞).
create table if not exists post_metrics (
    post_id uuid primary key references posts(id) on delete cascade,
    views bigint not null default 0,
    likes bigint not null default 0,
    updated_at timestamptz not null default now()
);

-- One row per visitor who liked a post (visitor identified by a keyed HMAC).
-- Enables toggle + dedupe without storing raw IPs.
create table if not exists post_likes (
    post_id uuid not null references posts(id) on delete cascade,
    visitor_hash text not null,
    created_at timestamptz not null default now(),
    primary key (post_id, visitor_hash)
);

-- Deduplicates view increments to at most one per visitor per UTC day.
create table if not exists post_view_marks (
    post_id uuid not null references posts(id) on delete cascade,
    visitor_hash text not null,
    view_day date not null default ((now() at time zone 'utc')::date),
    primary key (post_id, visitor_hash, view_day)
);

create index if not exists post_likes_post_idx on post_likes(post_id);
create index if not exists post_likes_created_at_idx on post_likes(created_at);
create index if not exists post_view_marks_post_idx on post_view_marks(post_id);
create index if not exists post_view_marks_view_day_idx on post_view_marks(view_day);

-- +goose Down
drop table if exists post_view_marks;
drop table if exists post_likes;
drop table if exists post_metrics;
