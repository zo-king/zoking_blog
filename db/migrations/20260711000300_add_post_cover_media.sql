-- +goose Up
alter table posts
    add column if not exists cover_media_id uuid null references media_assets(id) on delete set null;

create index if not exists posts_cover_media_id_idx on posts(cover_media_id)
    where cover_media_id is not null;

-- +goose Down
drop index if exists posts_cover_media_id_idx;
alter table posts drop column if exists cover_media_id;
