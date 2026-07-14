-- +goose Up
create unique index if not exists publish_jobs_post_active_unique_idx
    on publish_jobs(post_id)
    where post_id is not null
      and status in ('requested', 'queued', 'snapshotting', 'building', 'verifying', 'promoting')
      and deleted_at is null;

create unique index if not exists publish_jobs_page_active_unique_idx
    on publish_jobs(page_id)
    where page_id is not null
      and status in ('requested', 'queued', 'snapshotting', 'building', 'verifying', 'promoting')
      and deleted_at is null;

-- +goose Down
drop index if exists publish_jobs_page_active_unique_idx;
drop index if exists publish_jobs_post_active_unique_idx;
