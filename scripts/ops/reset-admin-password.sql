do $reset$
declare
  v_account text;
  v_password text;
  v_user_id uuid;
  v_user_email text;
  v_username text;
  v_hash text;
  v_matches integer;
  v_revoked integer;
begin
  select
    convert_from(decode(account_b64, 'base64'), 'UTF8'),
    convert_from(decode(password_b64, 'base64'), 'UTF8')
  into v_account, v_password
  from zoking_password_reset_input;

  if octet_length(v_password) < 16 or octet_length(v_password) > 72 then
    raise exception 'password length is outside the allowed range';
  end if;

  select count(*)
  into v_matches
  from users u
  where u.deleted_at is null
    and u.status = 'active'
    and (lower(u.username::text) = lower(v_account) or lower(u.email::text) = lower(v_account))
    and exists (
      select 1
      from user_roles ur
      join roles r on r.id = ur.role_id
      where ur.user_id = u.id and r.code = 'super_admin'
    );

  if v_matches <> 1 then
    raise exception 'expected exactly one active super administrator, found %', v_matches;
  end if;

  select u.id, u.email::text, u.username::text
  into v_user_id, v_user_email, v_username
  from users u
  where u.deleted_at is null
    and u.status = 'active'
    and (lower(u.username::text) = lower(v_account) or lower(u.email::text) = lower(v_account))
    and exists (
      select 1
      from user_roles ur
      join roles r on r.id = ur.role_id
      where ur.user_id = u.id and r.code = 'super_admin'
    );

  v_hash := crypt(v_password, gen_salt('bf', 12));
  update users
  set password_hash = v_hash, updated_at = now()
  where id = v_user_id;

  delete from refresh_tokens where user_id = v_user_id;
  get diagnostics v_revoked = row_count;

  if not exists (
    select 1 from users where id = v_user_id and crypt(v_password, password_hash) = password_hash
  ) then
    raise exception 'password hash verification failed';
  end if;

  insert into audit_logs (
    actor_id, actor_email, action, resource_type, resource_id,
    after_json, result, request_id, details_json
  ) values (
    v_user_id, v_user_email, 'admin.password.reset.ops', 'user', v_user_id,
    jsonb_build_object('password_reset', true, 'refresh_tokens_revoked', v_revoked),
    'success', 'local-console-password-reset', jsonb_build_object('source', 'local-console')
  );

  raise notice 'password reset complete for account %; refresh tokens revoked: %', v_username, v_revoked;
end
$reset$;
commit;
