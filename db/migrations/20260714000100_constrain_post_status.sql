-- +goose Up
alter table posts
    add constraint posts_status_allowed
    check (status in ('draft', 'offline', 'archived', 'published'))
    not valid;

alter table posts validate constraint posts_status_allowed;

-- +goose Down
alter table posts drop constraint if exists posts_status_allowed;
