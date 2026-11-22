-- Headliner — accounts, synced progress and friends (docs/ONLINE.md Part B, §B6).
--
-- Run once in the Supabase SQL editor (docs/SUPABASE_SETUP.md, step 5).
-- Every table has Row Level Security: a player reads and writes only their
-- own rows; friends see each other's profile card (public_profiles) and
-- nothing else; nobody but the owner ever reads a progress document.
-- Friendships change only through the functions at the bottom.

-- ---------------------------------------------------------------------------
-- Friend codes: HL-XXXX-XXXX from an alphabet without 0/O, 1/I/L.

create or replace function public.gen_friend_code() returns text
language plpgsql volatile security definer set search_path = public as $$
declare
  alphabet constant text := '23456789ABCDEFGHJKMNPQRSTUVWXYZ';
  code text;
begin
  loop
    code := 'HL-';
    for i in 1..8 loop
      if i = 5 then code := code || '-'; end if;
      code := code || substr(alphabet, 1 + floor(random() * length(alphabet))::int, 1);
    end loop;
    exit when not exists (select 1 from public.profiles where friend_code = code);
  end loop;
  return code;
end $$;

-- ---------------------------------------------------------------------------
-- profiles: the card a friend sees, plus the friend code.

create table if not exists public.profiles (
  id            uuid primary key references auth.users (id) on delete cascade,
  display_name  text not null check (char_length(display_name) between 1 and 20
                                     and display_name = btrim(display_name)),
  friend_code   text not null unique,
  character     int  not null default 0,
  level         int  not null default 1,
  title         text not null default '' check (char_length(title) <= 40),
  xp            int  not null default 0,
  fame          int  not null default 0,
  tier          int  not null default 0,
  battle_wins   int  not null default 0,
  battle_losses int  not null default 0,
  battle_draws  int  not null default 0,
  updated_at    timestamptz not null default now()
);

-- The friend code is the server's: set on insert, never changed.
create or replace function public.profiles_guard() returns trigger
language plpgsql security definer set search_path = public as $$
begin
  if tg_op = 'INSERT' then
    new.friend_code := public.gen_friend_code();
  else
    new.friend_code := old.friend_code;
    new.id := old.id;
  end if;
  new.updated_at := now();
  return new;
end $$;

drop trigger if exists profiles_guard on public.profiles;
create trigger profiles_guard before insert or update on public.profiles
  for each row execute function public.profiles_guard();

alter table public.profiles enable row level security;
drop policy if exists profiles_own_select on public.profiles;
drop policy if exists profiles_own_insert on public.profiles;
drop policy if exists profiles_own_update on public.profiles;
create policy profiles_own_select on public.profiles for select to authenticated using (id = auth.uid());
create policy profiles_own_insert on public.profiles for insert to authenticated with check (id = auth.uid());
create policy profiles_own_update on public.profiles for update to authenticated
  using (id = auth.uid()) with check (id = auth.uid());

-- ---------------------------------------------------------------------------
-- progress_docs: the linked profile's whole progress, owner-only.

create table if not exists public.progress_docs (
  user_id        uuid primary key references auth.users (id) on delete cascade,
  doc            jsonb not null check (octet_length(doc::text) < 8000000),
  schema_version int  not null,
  updated_at     timestamptz not null default now()
);

alter table public.progress_docs enable row level security;
drop policy if exists progress_own on public.progress_docs;
create policy progress_own on public.progress_docs for all to authenticated
  using (user_id = auth.uid()) with check (user_id = auth.uid());

-- ---------------------------------------------------------------------------
-- friendships: a asked b (or a blocked b). One row per pair.

create table if not exists public.friendships (
  a          uuid not null references auth.users (id) on delete cascade,
  b          uuid not null references auth.users (id) on delete cascade,
  status     text not null check (status in ('pending', 'accepted', 'blocked')),
  created_at timestamptz not null default now(),
  primary key (a, b),
  check (a <> b)
);
create unique index if not exists friendships_pair on public.friendships (least(a, b), greatest(a, b));

alter table public.friendships enable row level security;
drop policy if exists friendships_mine on public.friendships;
-- You see your own rows, except that someone blocked you.
create policy friendships_mine on public.friendships for select to authenticated
  using (a = auth.uid() or (b = auth.uid() and status <> 'blocked'));
-- No insert/update/delete policies: only the functions below change rows.

-- ---------------------------------------------------------------------------
-- public_profiles: card fields of yourself and your accepted friends.

create or replace view public.public_profiles with (security_barrier = true) as
select p.id, p.display_name, p.character, p.level, p.title, p.fame, p.tier,
       p.battle_wins, p.battle_losses, p.battle_draws, p.updated_at
from public.profiles p
where p.id = auth.uid()
   or exists (select 1 from public.friendships f
              where f.status = 'accepted'
                and ((f.a = auth.uid() and f.b = p.id) or (f.b = auth.uid() and f.a = p.id)));

-- ---------------------------------------------------------------------------
-- Friend functions (run as the database owner, always for auth.uid()).

-- request_friend: 'sent', 'accepted' (they had already asked you),
-- 'already', 'self' or 'not_found'. A block looks like 'sent'.
create or replace function public.request_friend(code text) returns text
language plpgsql security definer set search_path = public as $$
declare
  me uuid := auth.uid();
  them uuid;
  existing public.friendships;
begin
  if me is null then raise exception 'not signed in'; end if;
  select id into them from public.profiles where friend_code = upper(btrim(code));
  if them is null then return 'not_found'; end if;
  if them = me then return 'self'; end if;
  select * into existing from public.friendships
    where (a = me and b = them) or (a = them and b = me);
  if found then
    if existing.status = 'blocked' then return 'sent'; end if;   -- never reveal a block
    if existing.status = 'accepted' then return 'already'; end if;
    if existing.a = them then                                     -- they asked first
      update public.friendships set status = 'accepted' where a = them and b = me;
      return 'accepted';
    end if;
    return 'already';
  end if;
  insert into public.friendships (a, b, status) values (me, them, 'pending');
  return 'sent';
end $$;

-- respond_friend: accept or decline a request someone sent you.
create or replace function public.respond_friend(other uuid, accept boolean) returns void
language plpgsql security definer set search_path = public as $$
begin
  if accept then
    update public.friendships set status = 'accepted'
      where a = other and b = auth.uid() and status = 'pending';
  else
    delete from public.friendships where a = other and b = auth.uid() and status = 'pending';
  end if;
end $$;

-- remove_friend: unfriend, cancel your request, or unblock.
create or replace function public.remove_friend(other uuid) returns void
language plpgsql security definer set search_path = public as $$
begin
  delete from public.friendships
    where (a = auth.uid() and b = other)
       or (a = other and b = auth.uid() and status <> 'blocked');
end $$;

-- block_user: ends any friendship or request and stops new ones.
create or replace function public.block_user(other uuid) returns void
language plpgsql security definer set search_path = public as $$
begin
  if auth.uid() is null or other = auth.uid() then return; end if;
  delete from public.friendships
    where (a = auth.uid() and b = other) or (a = other and b = auth.uid() and status <> 'blocked');
  if not exists (select 1 from public.friendships where a = other and b = auth.uid()) then
    insert into public.friendships (a, b, status) values (auth.uid(), other, 'blocked');
  end if;
end $$;

-- friend_list: everyone you're connected to. Accepted friends come with
-- their card; requests and blocks with the name only.
create or replace function public.friend_list()
returns table (other uuid, status text, incoming boolean, display_name text, "character" int,
               level int, title text, fame int, tier int,
               battle_wins int, battle_losses int, battle_draws int)
language sql stable security definer set search_path = public as $$
  select o.id, f.status, f.b = auth.uid(), p.display_name,
         case when f.status = 'accepted' then p.character end,
         case when f.status = 'accepted' then p.level end,
         case when f.status = 'accepted' then p.title end,
         case when f.status = 'accepted' then p.fame end,
         case when f.status = 'accepted' then p.tier end,
         case when f.status = 'accepted' then p.battle_wins end,
         case when f.status = 'accepted' then p.battle_losses end,
         case when f.status = 'accepted' then p.battle_draws end
  from public.friendships f
  cross join lateral (select case when f.a = auth.uid() then f.b else f.a end as id) o
  join public.profiles p on p.id = o.id
  where f.a = auth.uid() or (f.b = auth.uid() and f.status <> 'blocked')
  order by f.status, p.display_name
$$;

-- delete_my_account: removes the account and, by cascade, every row of it.
create or replace function public.delete_my_account() returns void
language plpgsql security definer set search_path = public, auth as $$
begin
  if auth.uid() is null then raise exception 'not signed in'; end if;
  delete from auth.users where id = auth.uid();
end $$;

-- ---------------------------------------------------------------------------
-- Who may call what: signed-in players only; nothing for anonymous visitors.

revoke all on public.profiles, public.progress_docs, public.friendships, public.public_profiles from anon;
grant select, insert, update on public.profiles to authenticated;
grant select, insert, update, delete on public.progress_docs to authenticated;
grant select on public.friendships, public.public_profiles to authenticated;

revoke all on function public.gen_friend_code(), public.profiles_guard(), public.request_friend(text),
  public.respond_friend(uuid, boolean), public.remove_friend(uuid), public.block_user(uuid),
  public.friend_list(), public.delete_my_account() from public, anon;
grant execute on function public.request_friend(text), public.respond_friend(uuid, boolean),
  public.remove_friend(uuid), public.block_user(uuid), public.friend_list(),
  public.delete_my_account() to authenticated;
