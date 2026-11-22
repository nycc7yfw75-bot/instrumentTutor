-- Headliner — Row Level Security tests (docs/ONLINE.md §B6, §B7).
--
-- Paste into the Supabase SQL editor and Run, after the migration. It makes
-- three throwaway users (Ava, Ben, Cal), plays out who may see what, and
-- ROLLS EVERYTHING BACK at the end: nothing is left in your database.
-- "Success. No rows returned" = every test passed. Any failure stops with an
-- error starting "FAIL:".

begin;

insert into auth.users (id, email) values
  ('00000000-0000-4000-8000-00000000000a', 'ava@test.invalid'),
  ('00000000-0000-4000-8000-00000000000b', 'ben@test.invalid'),
  ('00000000-0000-4000-8000-00000000000c', 'cal@test.invalid');

create schema headliner_test;
grant usage on schema headliner_test to authenticated, anon;
create table headliner_test.codes (who text primary key, code text);
grant all on headliner_test.codes to authenticated;

-- Act as a user: as('a') / as('b') / as('c').
create or replace function headliner_test.act(who text) returns void language plpgsql as $$
begin
  perform set_config('request.jwt.claims',
    json_build_object('sub', '00000000-0000-4000-8000-00000000000' || who, 'role', 'authenticated')::text, true);
  execute 'set local role authenticated';
end $$;

create or replace function headliner_test.check(ok boolean, what text) returns void language plpgsql as $$
begin
  if not coalesce(ok, false) then raise exception 'FAIL: %', what; end if;
end $$;
grant execute on all functions in schema headliner_test to authenticated, anon;

-- 1. Each makes a profile; the server picks the friend code.
select headliner_test.act('a');
insert into public.profiles (id, display_name, friend_code) values (auth.uid(), 'Ava', 'HL-MINE-MINE');
insert into headliner_test.codes select 'a', friend_code from public.profiles where id = auth.uid();
select headliner_test.check((select code from headliner_test.codes where who = 'a') ~ '^HL-[2-9A-HJKMNP-Z]{4}-[2-9A-HJKMNP-Z]{4}$',
  'friend code was not made by the server');
insert into public.progress_docs (user_id, doc, schema_version) values (auth.uid(), '{"xp": 10}', 2);
reset role;

select headliner_test.act('b');
insert into public.profiles (id, display_name) values (auth.uid(), 'Ben');
insert into headliner_test.codes select 'b', friend_code from public.profiles where id = auth.uid();
-- 2. Ben sees only his own profile row and none of Ava's progress.
select headliner_test.check((select count(*) from public.profiles) = 1, 'Ben can read another profile row');
select headliner_test.check((select count(*) from public.progress_docs) = 0, 'Ben can read Ava''s progress');
update public.progress_docs set doc = '{"xp": 0}' where user_id = '00000000-0000-4000-8000-00000000000a';
select headliner_test.check((select count(*) from public.public_profiles) = 1, 'Ben sees a card before any friendship');
-- 3. Friendships can't be written directly.
do $$ begin
  insert into public.friendships (a, b, status)
    values (auth.uid(), '00000000-0000-4000-8000-00000000000a', 'accepted');
  raise exception 'FAIL: Ben wrote a friendship directly';
exception when insufficient_privilege then null;
end $$;
-- 4. A name over 20 characters is refused.
do $$ begin
  update public.profiles set display_name = 'A name far too long to keep' where id = auth.uid();
  raise exception 'FAIL: a 27-character name was accepted';
exception when check_violation then null;
end $$;
reset role;
select headliner_test.check((select doc->>'xp' from public.progress_docs
  where user_id = '00000000-0000-4000-8000-00000000000a') = '10', 'Ben changed Ava''s progress');

select headliner_test.act('c');
insert into public.profiles (id, display_name) values (auth.uid(), 'Cal');
insert into headliner_test.codes select 'c', friend_code from public.profiles where id = auth.uid();
reset role;

-- 5. Ava asks Ben; Ben sees the request with her name only, then accepts.
select headliner_test.act('a');
select headliner_test.check(public.request_friend((select code from headliner_test.codes where who = 'b')) = 'sent', 'request not sent');
select headliner_test.check(public.request_friend('HL-NOPE-NOPE') = 'not_found', 'unknown code not reported');
select headliner_test.check(public.request_friend((select code from headliner_test.codes where who = 'a')) = 'self', 'asking yourself allowed');
reset role;
select headliner_test.act('b');
select headliner_test.check((select count(*) from public.friend_list() where incoming and status = 'pending'
  and display_name = 'Ava' and level is null) = 1, 'Ben doesn''t see Ava''s request (name only)');
select headliner_test.check((select count(*) from public.public_profiles where display_name = 'Ava') = 0,
  'a pending request already shows the card');
select public.respond_friend('00000000-0000-4000-8000-00000000000a', true);
select headliner_test.check((select count(*) from public.public_profiles where display_name = 'Ava') = 1,
  'Ben can''t see his friend''s card');
select headliner_test.check((select count(*) from public.progress_docs) = 0, 'a friend can read progress');
reset role;

-- 6. Cal sees neither; Ava blocks Cal; Cal's request goes nowhere and he
--    can't tell he's blocked.
select headliner_test.act('c');
select headliner_test.check((select count(*) from public.public_profiles) = 1, 'Cal sees strangers'' cards');
reset role;
select headliner_test.act('a');
select public.block_user('00000000-0000-4000-8000-00000000000c');
reset role;
select headliner_test.act('c');
select headliner_test.check(public.request_friend((select code from headliner_test.codes where who = 'a')) = 'sent', 'a block was revealed');
select headliner_test.check((select count(*) from public.friendships) = 0, 'Cal can see he was blocked');
reset role;
select headliner_test.act('a');
select headliner_test.check((select count(*) from public.friend_list() where status = 'pending') = 0, 'a blocked user''s request got through');
reset role;

-- 7. Anonymous visitors see nothing.
set local role anon;
do $$ begin
  perform count(*) from public.profiles;
  raise exception 'FAIL: anonymous visitors can read profiles';
exception when insufficient_privilege then null;
end $$;
reset role;

-- 8. Deleting an account removes every row of it.
select headliner_test.act('b');
select public.delete_my_account();
reset role;
select headliner_test.check((select count(*) from public.profiles where display_name = 'Ben') = 0, 'Ben''s profile outlived his account');
select headliner_test.check((select count(*) from public.friendships
  where a = '00000000-0000-4000-8000-00000000000b' or b = '00000000-0000-4000-8000-00000000000b') = 0,
  'Ben''s friendships outlived his account');

rollback;
