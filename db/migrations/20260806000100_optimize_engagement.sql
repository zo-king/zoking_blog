-- +goose Up
create index if not exists post_likes_created_at_idx on post_likes(created_at);
create index if not exists post_view_marks_view_day_idx on post_view_marks(view_day);

-- +goose Down
drop index if exists post_view_marks_view_day_idx;
drop index if exists post_likes_created_at_idx;
