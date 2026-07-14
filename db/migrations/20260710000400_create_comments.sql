-- +goose Up
create table if not exists comments (
    id uuid primary key default gen_random_uuid(),
    post_id uuid not null references posts(id) on delete cascade,
    parent_id uuid null references comments(id) on delete cascade,
    author_name text not null,
    author_email_hash text not null default '',
    author_website text not null default '',
    content text not null,
    status text not null default 'pending',
    ip_hash text not null default '',
    user_agent text not null default '',
    reviewed_by uuid null references users(id) on delete set null,
    reviewed_at timestamptz null,
    spam_reason text not null default '',
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    deleted_at timestamptz null
);

create index if not exists comments_post_status_created_idx on comments(post_id, status, created_at desc);
create index if not exists comments_status_created_idx on comments(status, created_at desc);
create index if not exists comments_parent_idx on comments(parent_id);
create index if not exists comments_reviewed_by_idx on comments(reviewed_by);

-- +goose Down
drop table if exists comments;
