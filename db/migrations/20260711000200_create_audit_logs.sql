-- +goose Up
alter table audit_logs add column if not exists actor_email text not null default '';
alter table audit_logs add column if not exists route text not null default '';
alter table audit_logs add column if not exists method text not null default '';
alter table audit_logs add column if not exists status_code integer not null default 200;
alter table audit_logs add column if not exists error_code text not null default '';
alter table audit_logs add column if not exists details_json jsonb not null default '{}'::jsonb;
alter table audit_logs add column if not exists ip_hash_version smallint not null default 1;

create index if not exists idx_audit_logs_actor_created_at on audit_logs(actor_id, created_at desc);
create index if not exists idx_audit_logs_resource on audit_logs(resource_type, resource_id, created_at desc);
create index if not exists idx_audit_logs_action_created_at on audit_logs(action, created_at desc);
create index if not exists idx_audit_logs_request_id on audit_logs(request_id);

-- +goose Down
drop index if exists idx_audit_logs_request_id;
drop index if exists idx_audit_logs_action_created_at;
drop index if exists idx_audit_logs_resource;
drop index if exists idx_audit_logs_actor_created_at;
alter table audit_logs drop column if exists ip_hash_version;
alter table audit_logs drop column if exists details_json;
alter table audit_logs drop column if exists error_code;
alter table audit_logs drop column if exists status_code;
alter table audit_logs drop column if exists method;
alter table audit_logs drop column if exists route;
alter table audit_logs drop column if exists actor_email;
