-- +goose Up
alter table publish_jobs add column if not exists settings_hash text not null default '';

-- +goose Down
alter table publish_jobs drop column if exists settings_hash;
