-- Headliner - Row Level Security tests (docs/ONLINE.md B6, B7).
--
-- Paste into the Supabase SQL editor and Run, after the migration. It makes
-- three throwaway users (Ava, Ben, Cal), plays out who may see what, and
-- UNDOES EVERYTHING at the end: nothing is left in your database.
--
-- The whole test is one block, so the editor can't split it up, and it needs
-- no helper tables or schemas of its own.
--
-- Passed: one row, "All security tests passed".
-- Failed: an error starting "FAIL:" naming the rule that broke.

do $test$
declare
  ava constant uuid := '00000000-0000-4000-8000-00000000000a';
  ben constant uuid := '00000000-0000-4000-8000-00000000000b';
  cal constant uuid := '00000000-0000-4000-8000-00000000000c';
  code_a text;
  code_b text;
  n int;
begin
  begin -- everything in here is rolled back at the end
    insert into auth.users (id, email) values
      (ava, 'ava@test.invalid'), (ben, 'ben@test.invalid'), (cal, 'cal@test.invalid');

    -- 1. Each makes a profile; the server picks the friend code.
    perform set_config('request.jwt.claims', json_build_object('sub', ava, 'role', 'authenticated')::text, true);
    execute 'set local role authenticated';
    insert into public.profiles (id, display_name, friend_code) values (auth.uid(), 'Ava', 'HL-MINE-MINE');
    select friend_code into code_a from public.profiles where id = auth.uid();
    assert code_a ~ '^HL-[2-9A-HJKMNP-Z]{4}-[2-9A-HJKMNP-Z]{4}$', 'FAIL: friend code was not made by the server';
    insert into public.progress_docs (user_id, doc, schema_version) values (auth.uid(), '{"xp": 10}', 2);
    execute 'reset role';

    perform set_config('request.jwt.claims', json_build_object('sub', ben, 'role', 'authenticated')::text, true);
    execute 'set local role authenticated';
    insert into public.profiles (id, display_name) values (auth.uid(), 'Ben');
    select friend_code into code_b from public.profiles where id = auth.uid();
    -- 2. Ben sees only his own profile row and none of Ava's progress.
    assert (select count(*) from public.profiles) = 1, 'FAIL: Ben can read another profile row';
    assert (select count(*) from public.progress_docs) = 0, 'FAIL: Ben can read Ava''s progress';
    update public.progress_docs set doc = '{"xp": 0}' where user_id = ava;
    assert (select count(*) from public.public_profiles) = 1, 'FAIL: Ben sees a card before any friendship';
    -- 3. Friendships can't be written directly.
    begin
      insert into public.friendships (a, b, status) values (auth.uid(), ava, 'accepted');
      raise exception 'FAIL: Ben wrote a friendship directly';
    exception when insufficient_privilege then null;
    end;
    -- 4. A name over 20 characters is refused.
    begin
      update public.profiles set display_name = 'A name far too long to keep' where id = auth.uid();
      raise exception 'FAIL: a 27-character name was accepted';
    exception when check_violation then null;
    end;
    execute 'reset role';
    assert (select doc->>'xp' from public.progress_docs where user_id = ava) = '10', 'FAIL: Ben changed Ava''s progress';

    perform set_config('request.jwt.claims', json_build_object('sub', cal, 'role', 'authenticated')::text, true);
    execute 'set local role authenticated';
    insert into public.profiles (id, display_name) values (auth.uid(), 'Cal');
    execute 'reset role';

    -- 5. Ava asks Ben; Ben sees the request with her name only, then accepts.
    perform set_config('request.jwt.claims', json_build_object('sub', ava, 'role', 'authenticated')::text, true);
    execute 'set local role authenticated';
    assert public.request_friend(code_b) = 'sent', 'FAIL: request not sent';
    assert public.request_friend('HL-NOPE-NOPE') = 'not_found', 'FAIL: unknown code not reported';
    assert public.request_friend(code_a) = 'self', 'FAIL: asking yourself allowed';
    execute 'reset role';

    perform set_config('request.jwt.claims', json_build_object('sub', ben, 'role', 'authenticated')::text, true);
    execute 'set local role authenticated';
    assert (select count(*) from public.friend_list() where incoming and status = 'pending'
      and display_name = 'Ava' and level is null) = 1, 'FAIL: Ben doesn''t see Ava''s request (name only)';
    assert (select count(*) from public.public_profiles where display_name = 'Ava') = 0,
      'FAIL: a pending request already shows the card';
    perform public.respond_friend(ava, true);
    assert (select count(*) from public.public_profiles where display_name = 'Ava') = 1,
      'FAIL: Ben can''t see his friend''s card';
    assert (select count(*) from public.progress_docs) = 0, 'FAIL: a friend can read progress';
    execute 'reset role';

    -- 6. Cal sees neither; Ava blocks Cal; Cal's request goes nowhere and he
    --    can't tell he's blocked.
    perform set_config('request.jwt.claims', json_build_object('sub', cal, 'role', 'authenticated')::text, true);
    execute 'set local role authenticated';
    assert (select count(*) from public.public_profiles) = 1, 'FAIL: Cal sees strangers'' cards';
    execute 'reset role';

    perform set_config('request.jwt.claims', json_build_object('sub', ava, 'role', 'authenticated')::text, true);
    execute 'set local role authenticated';
    perform public.block_user(cal);
    execute 'reset role';

    perform set_config('request.jwt.claims', json_build_object('sub', cal, 'role', 'authenticated')::text, true);
    execute 'set local role authenticated';
    assert public.request_friend(code_a) = 'sent', 'FAIL: a block was revealed';
    assert (select count(*) from public.friendships) = 0, 'FAIL: Cal can see he was blocked';
    execute 'reset role';

    perform set_config('request.jwt.claims', json_build_object('sub', ava, 'role', 'authenticated')::text, true);
    execute 'set local role authenticated';
    assert (select count(*) from public.friend_list() where status = 'pending') = 0,
      'FAIL: a blocked user''s request got through';
    execute 'reset role';

    -- 7. Anonymous visitors see nothing.
    perform set_config('request.jwt.claims', '', true);
    execute 'set local role anon';
    begin
      select count(*) into n from public.profiles;
      raise exception 'FAIL: anonymous visitors can read profiles';
    exception when insufficient_privilege then null;
    end;
    execute 'reset role';

    -- 8. Deleting an account removes every row of it.
    perform set_config('request.jwt.claims', json_build_object('sub', ben, 'role', 'authenticated')::text, true);
    execute 'set local role authenticated';
    perform public.delete_my_account();
    execute 'reset role';
    assert (select count(*) from public.profiles where display_name = 'Ben') = 0,
      'FAIL: Ben''s profile outlived his account';
    assert (select count(*) from public.friendships where a = ben or b = ben) = 0,
      'FAIL: Ben''s friendships outlived his account';

    raise exception 'headliner-tests-passed'; -- undo everything above
  exception when others then
    if sqlerrm <> 'headliner-tests-passed' then
      raise; -- a FAIL (or any other error): show it
    end if;
  end;
end
$test$;

select 'All security tests passed' as result;
