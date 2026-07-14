-- +goose Up
alter table posts drop constraint if exists posts_status_allowed;

alter table posts
    add constraint posts_status_allowed
    check (status in ('draft', 'offline', 'archived', 'published', 'publishing'))
    not valid;

alter table posts validate constraint posts_status_allowed;

-- +goose Down
-- +goose StatementBegin
do $$
begin
    if exists (
        select 1
        from publish_jobs
        where status in ('requested', 'queued', 'snapshotting', 'building', 'verifying', 'promoting')
          and deleted_at is null
    ) then
        raise exception 'cannot roll back publishing status while publish jobs are active';
    end if;
end
$$;
-- +goose StatementEnd

alter table posts drop constraint if exists posts_status_allowed;

update posts set status = 'published' where status = 'publishing';

alter table posts
    add constraint posts_status_allowed
    check (status in ('draft', 'offline', 'archived', 'published'))
    not valid;

alter table posts validate constraint posts_status_allowed;
