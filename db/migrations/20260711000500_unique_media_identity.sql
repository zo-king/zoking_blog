-- +goose Up
create unique index if not exists media_assets_active_checksum_unique_idx
    on media_assets(lower(checksum))
    where deleted_at is null and status <> 'deleted' and checksum <> '';

create unique index if not exists media_assets_active_public_url_unique_idx
    on media_assets(public_url)
    where deleted_at is null and status <> 'deleted' and public_url <> '';

-- +goose Down
drop index if exists media_assets_active_public_url_unique_idx;
drop index if exists media_assets_active_checksum_unique_idx;
