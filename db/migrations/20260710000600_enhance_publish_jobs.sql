-- +goose Up
alter table if exists publish_jobs
    add column if not exists log_json jsonb null,
    add column if not exists canceled_at timestamptz null;

-- +goose Down
alter table if exists publish_jobs
    drop column if exists canceled_at,
    drop column if exists log_json;
