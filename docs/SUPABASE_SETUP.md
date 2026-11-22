# Setting up Supabase for Headliner (once)

Supabase is the free online service that keeps accounts, synced progress and
friend lists. You set it up once (about 30 minutes); after that it runs by itself.

You'll need: a Google account, and the project folder open in VS Code (for two
files you'll copy from).

**Never share** the database password, the `service_role` key or any key called
**secret**. The only things you send me are the **Project URL** and the
**anon / publishable key**, which are public by design (every installed copy of
Headliner carries them; the database rules decide what anyone can see).

---

## 1. Create the project

1. Go to https://supabase.com and click **Start your project**. Sign in with GitHub.
2. Click **New project**.
   - Organization: your personal one (Free plan).
   - Project name: `headliner`
   - Database password: click **Generate a password**, then save it in your
     password manager. (You won't need it day to day.)
   - Region: the one nearest you (e.g. **South Asia (Mumbai)**).
3. Click **Create new project**. Wait a minute or two until the dashboard says
   the project is ready.

## 2. Copy the two public values

1. In the left sidebar, open **Project Settings** (the gear) → **API Keys**
   (on some dashboards: **Data API** / **API**).
2. Copy these two into a note:
   - **Project URL**, like `https://abcdefghijklmnop.supabase.co`
     (shown under Data API, or at the top of the API page).
   - The **anon public** key (a long `eyJ…` text) or, on newer dashboards, the
     **Publishable key** (`sb_publishable_…`). Either works.

## 3. Google sign-in

Google needs to know Headliner is allowed to ask "Sign in with Google".

1. In Supabase: **Authentication** → **Sign In / Providers** → **Google**.
   Copy the **Callback URL (for OAuth)** shown there, like
   `https://abcdefghijklmnop.supabase.co/auth/v1/callback`. Leave the page open.
2. In another tab go to https://console.cloud.google.com and sign in.
3. Top bar: project picker → **New project** → name `Headliner` → **Create**,
   then make sure it's selected.
4. Search for **Google Auth Platform** (or "OAuth consent screen") and open it.
   Click **Get started**:
   - App name: `Headliner`; User support email: yours → **Next**.
   - Audience: **External** → **Next**.
   - Contact email: yours → **Next** → agree → **Create**.
5. Still in Google Auth Platform → **Audience** → **Publish app** → **Confirm**.
   (Headliner only asks for name and email, so Google doesn't need to review it.
   Without this, only people you list as "test users" can sign in.)
6. **Clients** → **Create client**:
   - Application type: **Web application**; Name: `Headliner`.
   - **Authorized redirect URIs** → **Add URI** → paste the Callback URL from step 1.
   - **Create**. Copy the **Client ID** and **Client secret** it shows.
7. Back in Supabase's Google page: switch **Enable Sign in with Google** on,
   paste the Client ID and Client secret, **Save**.

## 4. Email sign-in and where sign-in returns to

1. **Authentication** → **Sign In / Providers** → **Email**: it's on already.
   Leave "Confirm email" on. Save if you changed anything.
2. **Authentication** → **URL Configuration**:
   - **Site URL**: `http://127.0.0.1:47821/callback`
   - **Redirect URLs** → **Add URL**, three times:
     - `http://127.0.0.1:47821/callback`
     - `http://127.0.0.1:47822/callback`
     - `http://127.0.0.1:47823/callback`
   - **Save**.

   (After you sign in in the browser, it hands back to Headliner on your own
   laptop at one of these addresses. Nothing outside your laptop can use them.)

## 5. Create the tables

1. **SQL Editor** (left sidebar) → **New query**.
2. In VS Code open `supabase/migrations/20261122000000_accounts_and_friends.sql`,
   select all (Ctrl+A), copy, paste into the editor.
3. Click **Run**. You should see **Success. No rows returned**.
   (If Supabase asks to confirm a "destructive operation", click confirm: the
   script only removes old copies of its own rules before re-adding them.)
4. Check: **Table Editor** shows `profiles`, `progress_docs` and `friendships`,
   each with a shield/RLS badge saying it's enabled.

## 6. Test the security rules

1. **SQL Editor** → **New query**.
2. Copy all of `supabase/tests/rls_test.sql` from VS Code, paste, **Run**.
3. **Success. No rows returned** = every rule works. (The test makes three
   pretend players and deletes them again; nothing is left behind.)
   If you see an error starting with `FAIL:`, send me the whole message.

## 7. Send me the two public values

Send the **Project URL** and the **anon / publishable key** from step 2. I put
them into the release builds (as GitHub secrets) and into your dev build's
settings file. That's it.

---

## Good to know

- **The free project pauses after about a week with no sign-ins.** Headliner
  then just says "Offline — will sync later" and everything else keeps working.
  To wake it: log in at supabase.com, open the project, click **Restore project**
  (a couple of minutes).
- **Email sign-in links** come from Supabase's own mail service, which sends only
  a few per hour on the free plan. Google sign-in has no such limit.
- Open the email link **on the same laptop** where Headliner is waiting.
