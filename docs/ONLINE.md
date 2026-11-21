# HEADLINER — Release, Auto-Update, Accounts & Async Battles

Save as `docs/ONLINE.md`. Re-read the part you're working on at the start of every stage and after any context compaction.

This file has four parts, built at different times:

| Part | What | Build after |
|---|---|---|
| **A. Release & updates** | Windows installer, GitHub Releases, auto-update (Stable/Beta), save safety, dev/play separation, rename to Headliner | SCORING.md — so the owner can start playing with saved progress straight away |
| **B. Accounts & sync** | Supabase login (Google + email link), local-first profile sync, friends via invite codes | CAREER.md |
| **C. Async battles** | Friend challenges + friend leaderboards, played separately on each device | BATTLES.md + Part B |
| **D. Mac port** | Outline only — a separate brief later | Part C working on Windows |

Global rules for every part:
- **Never lose a save.** Every update, migration and sync writes a backup first. Anything that can't be read is quarantined, never overwritten.
- **Offline always works.** Practice, career and local battles never need the internet or an account. Online features degrade to a calm "Offline — will sync later".
- **Song files never leave the device.** Commercial tabs are not ours to distribute. Only fingerprints (§C1) and titles travel.
- Secrets: nothing secret is ever committed or embedded. The Supabase **anon** key is public by design; security comes from Row Level Security. The **service_role** key never appears in the app or repo.

---

## PART A — Release & updates

### A1. Repo and licence
- Public fork (the owner's GitHub). Keep the upstream MIT `LICENSE`; add `NOTICE.md`: "Headliner is based on musicTutor by S95F (MIT)". An in-app **About** screen shows version, licence, credits (`assets/CREDITS.md`) and the upstream notice.
- Default branch `main` = what Stable builds come from. Work happens on feature branches.

### A2. The rename (deferred from REDESIGN)
- Binary: `headliner.exe`. Window/app name already "HEADLINER".
- Config dir: `%APPDATA%\headliner\`. On first run of a Headliner build: if `%APPDATA%\musictutor\` exists and `headliner\` doesn't, **copy** (not move) everything across, verify file-by-file, then leave a `MIGRATED_TO_HEADLINER.txt` in the old folder. Never delete the old folder automatically.
- Env vars: `HEADLINER_CONFIG_DIR` (keep honouring `MUSICTUTOR_CONFIG_DIR` as a fallback).
- Go module path can stay as-is (internal); don't churn imports.

### A3. Dev vs play separation (so the owner's real saves are safe while developing)
- Builds made locally with `go build` (no release ldflags) are **dev builds**: they default to `%APPDATA%\headliner-dev\` and show a small "DEV" pill in the header.
- Installed release builds use `%APPDATA%\headliner\`.
- The owner plays with the installed Stable/Beta build and develops in the repo; the two never share a save folder unless `HEADLINER_CONFIG_DIR` says so.

### A4. Versioning
- Semver tags: `v1.2.0` = Stable, `v1.3.0-beta.1` = Beta.
- Version injected at build: `-ldflags "-X main.version=<tag> -X main.channel=<stable|beta> -H windowsgui"`.
- `CHANGELOG.md` updated with every release; the release notes come from it.

### A5. CI — GitHub Actions (`.github/workflows/release.yml`)
On a pushed tag `v*`:
1. `windows-latest` runner; `actions/setup-go` (Go 1.26); `msys2/setup-msys2` with `mingw-w64-ucrt-x86_64-gcc` on PATH; `CGO_ENABLED=1`.
2. `go test ./...` (must pass; the release fails otherwise).
3. Build `headliner.exe` with the ldflags above.
4. Build the installer with **Inno Setup** (`iscc`, from Chocolatey on the runner):
   - **Per-user install**, no admin: `%LOCALAPPDATA%\Programs\Headliner\`. (So the updater can replace files without UAC prompts.)
   - Start-menu shortcut + optional desktop shortcut; uninstaller that **does not** delete saves (say so in the uninstall dialog).
   - Installs the app's bundled SoundFont/assets if any.
5. Produce assets: `Headliner-Setup-<ver>.exe`, `headliner-<ver>-windows-amd64.zip` (portable, used by the updater), `SHA256SUMS`, `SHA256SUMS.sig`.
6. **Integrity signing (free, not code-signing):** sign `SHA256SUMS` with an Ed25519 key (minisign format). Private key in the repo's Actions secrets; the public key is embedded in the app. This proves updates came from the owner's pipeline even though the exe isn't code-signed.
7. Create the GitHub Release (pre-release for beta tags) with notes from `CHANGELOG.md`.

Also a `ci.yml` on every push/PR: test + build only.

### A6. The updater (inside the app)
- Checks on launch, and from Settings → Updates → "Check now". Never while a song, gig or battle is running; updates only from the landing page.
- Source: `https://api.github.com/repos/<owner>/<repo>/releases` (unauthenticated; cache the response; respect rate limits; offline = skip silently).
- Channel: Settings → Updates → **Stable** (default) / **Beta**. Beta sees pre-releases too; Stable never does.
- If newer: dialog "Headliner 1.3.0 is available" + notes, buttons **Update now** (primary) · **Later** · **Skip this version**.
- Update now:
  1. Download the zip + `SHA256SUMS` + `.sig` to a temp folder (progress bar).
  2. Verify the signature with the embedded public key, then the zip's SHA-256. Any mismatch → abort, delete, tell the user plainly.
  3. **Back up saves** (A7).
  4. Unpack next to the install as `Headliner.new\`, then swap: rename the current install to `Headliner.prev\`, `Headliner.new\` → install folder (a tiny helper exe or a restart step handles the running-exe lock).
  5. Relaunch.
- **Rollback:** if the new version fails to reach the landing page twice in a row, the launcher restores `Headliner.prev\` and shows "Update rolled back". Keep exactly one previous version.
- The updater only ever downloads from that repo's release assets over HTTPS.

### A7. Save safety
- Before any update, any schema migration, and any cloud sync that changes local data: copy `progress.json` + `config.json` to `backups/<timestamp>/`; keep the last 15.
- Load: unreadable/corrupt file → rename to `*.bad-<timestamp>`, offer "Restore the latest backup" (lists timestamps).
- A save from a **newer** schema than the app understands → read-only mode with a message; never downgrade-write (as PROGRESS.md specifies).
- Settings → Data: **Export progress** (zip of saves) / **Import progress** (validates, backs up current first) / **Open save folder**.

### A8. Friend-facing docs
- `docs/INSTALL_FOR_FRIENDS.md` — plain steps: what you need (electric guitar + USB audio interface or USB amp; headphones), download the Setup from the Releases page, the SmartScreen "More info → Run anyway" step (with what it looks like, explaining it's unsigned, not unsafe), first launch → pick hometown/character → Settings → choose input/output → Tap calibrate → play. Troubleshooting: no input, "Unscored — calibrate", Defender quarantine.
- README section "Download Headliner" linking the latest release.

### A9. Part A tests and checks
- Semver compare incl. beta ordering; channel filtering; signature + hash verification (good, tampered zip, tampered sums, wrong key); backup rotation (keeps 15); corrupt-save quarantine; config-dir migration (copy, verify, never delete); dev builds default to the dev folder.
- Manual (write these steps for the owner at the end): tag `v0.1.0-beta.1` → CI produces the installer → install on the laptop → play and save → tag `v0.1.0-beta.2` → app offers the update → update → saves intact → simulate a broken release (a build that exits on start) → automatic rollback works.

→ **CHECKPOINT A**: after the first successful tagged release and a real update cycle on the owner's laptop. Wait.

---

## PART B — Accounts & sync (Supabase)

### B1. Setup the owner does once (write exact steps in `docs/SUPABASE_SETUP.md`)
Create a Supabase project (free tier) → enable **Google** and **Email (magic link)** auth providers → set the redirect URL allow-list to `http://127.0.0.1/*` → run the SQL migration from `supabase/migrations/` (in the repo) → copy the project URL + anon key into `internal/online/config.go` via CI secrets → done. Note in the doc: free-tier projects **pause after about a week without activity**; the app must treat a paused/unreachable backend as "offline" and the owner can resume it from the dashboard.

### B2. Sign-in flow (desktop)
- Settings → Account → **Sign in** (and a small "Sign in to battle friends" link on the Battles door).
- OAuth with PKCE: open the system browser to Supabase's auth URL; a loopback listener on `127.0.0.1:<random free port>/callback` receives the code; exchange it; show "Signed in as …". Email link uses the same redirect.
- Store the refresh token in **Windows Credential Manager** (dependency allowed: a small, maintained wincred library — explain the choice in the report). Never store tokens in plain files.
- Sign out; **Delete account** (removes server rows for the user; local saves stay; confirm dialog, Cancel default focus).

### B3. Profiles: local-first
- Guest play is the default: a local profile works fully offline.
- On first sign-in: "Link *Player 1* to your account?" → uploads that profile. One account ↔ one linked profile. Other local profiles (hot-seat friends) stay local-only.
- Signing in on a second device downloads the account's profile and merges with any local one only if the user chooses.

### B4. Sync
- The linked profile syncs on launch, after each pass/gig/battle, and on exit (debounced, background, never blocking play).
- Server keeps the profile's progress document + derived public fields.
- **Merge rules (deterministic, unit-tested):** XP, Fame, level = max · mastered = union (earliest date wins) · gig best stars = max · gig crowd type = first recorded wins · tough-crowd odds `p` = from the side with more gig rolls · pass/gig/battle logs = union by ID, capped · best-pass replays = higher accuracy wins · records = recomputed from the merged battle log.
- Conflicts never delete data; a backup (A7) is written before applying a merge.

### B5. Friends (invite-only)
- Each account gets a friend code like `HL-7K3Q-9PXM`. Add friend = enter a code → request → the other accepts. No player search, no public directory.
- "Share my code" copies: "Add me on Headliner: HL-7K3Q-9PXM — download: <releases link>".
- Friends see the **profile card only**: display name, character, level + title, Fame, career tier, battle record, leaderboard scores. Never song libraries or practice history.
- Remove friend; block (hides and prevents requests).

### B6. Database (Supabase Postgres, with Row Level Security on every table)
- `profiles` (id = auth uid, display_name ≤ 20 chars, friend_code unique, character, level, title, xp, fame, tier, updated_at)
- `progress_docs` (user_id, doc jsonb, schema_version, updated_at) — owner-only read/write
- `friendships` (a, b, status pending|accepted|blocked, created_at)
- `public_profiles` view exposing only card fields to accepted friends
- Battle tables in Part C.
- RLS: users read/write only their own rows; friends read `public_profiles`; nobody reads anyone's `progress_docs` but the owner. Write SQL tests (pgTAP or simple SQL assertions) for these policies.

### B7. Part B tests and checks
Merge-rule table tests; token storage round-trip; offline/paused backend behaviour (app stays usable, shows offline); RLS tests; display-name validation. Manual: sign in with Google on the owner's laptop, link profile, sign in on a second Windows machine/account (or a friend's), add each other by code, see each other's cards, sign out/in, delete a test account.

→ **CHECKPOINT B**: after the manual run with two real accounts. Wait.

---

## PART C — Async battles (friends, played separately)

### C1. What can be battled
- **Battle-riff pack** (original, ships with the app): always available to everyone.
- **Songs both players own:** matched by a **fingerprint** of the battled material — the structural hash from PROGRESS.md (tick timing + pitch per note, excluding string/fret and file bytes) over the practised track's bar range. The server stores only `{fingerprint, title, track name, bar range, bars, note count}`. If the recipient has no local piece with that fingerprint, the challenge says: "You need your own copy of *<title>* (track: Lead Guitar) to accept" — the app never sends the file.

### C2. Challenges
- From a friend's card or the Battles screen: **Challenge** → pick material (riff or a section of a song) → the BATTLES.md format (mirrored trade-bars, phrase bars × exchanges) → the sender plays **all their phrases in one sitting** at a tempo they choose (handicap curve from BATTLES.md applies) → results upload.
- Uploaded result per phrase: per-note verdicts + timing offsets, accuracy, tightness, turnScore, tempo, scoring version, app version, calibration confidence. No audio.
- Recipient: an inbox badge on the Battles door (checked on launch and every 60 s while on the Battles screen; no push notifications). They have **7 days**. Accepting runs the same phrases; after each of their phrases a result card shows **both** players' phrase results side by side, and "Watch their phrase" replays the opponent's notes as a ghost on the tab (same renderer as the own-ghost bot).
- Winner computed by the shared `internal/battle` code on the recipient's device and verified by the sender's device on next sync (same inputs → same result). Both get BATTLES.md XP/records.
- Expiry: after 7 days unaccepted → "Expired", no win/loss recorded. Declining is allowed, also no record.
- Only credible sessions (calibrated, no dropped samples) can send or answer; otherwise the play button explains why and links to Tap calibrate.
- Scoring-version mismatch: if the two scoring versions differ in their major number, the challenge can't be accepted until both update ("Update Headliner to answer this challenge").

### C3. Friend leaderboards
- Per material (riff, or song section by fingerprint + bar range): your friends' best **100%-tempo** runs, top 10, with accuracy, tightness and date. Any credible full run of that material at 100% submits automatically if it beats your previous best.
- Shown on the riff/section's card and in Battles → Leaderboards. "Challenge the leader" shortcut.

### C4. Tables
`challenges` (id, from, to, material json, format json, status, created_at, expires_at) · `challenge_runs` (challenge_id, user_id, results jsonb, submitted_at) · `leaderboard_entries` (user_id, material_key, best json, updated_at). RLS: challenge rows readable by its two participants only; leaderboard entries readable by the owner's accepted friends.

### C5. Fair play (friends-only, light touch)
Client-side scoring is trusted among friends. Still: results include app + scoring versions and calibration confidence; impossible values (accuracy > 1, negative counts, more notes than the material has) are rejected server-side by a check constraint/function.

### C6. Part C tests and checks
Fingerprint stability across .gp/.mid/.musicxml/.gtab of the same material (use the repo's fixture corpus); challenge lifecycle (send → accept → finish; expire; decline); deterministic winner on both devices; leaderboard best-only submission; RLS tests. Manual with a friend: send a riff challenge, they answer a day later, both see the same winner; battle a section of a song both own; try a song the friend doesn't have and see the "you need your own copy" message.

→ **CHECKPOINT C** after the manual friend test. Wait.

---

## PART D — Mac port (outline only; write `docs/MAC_PORT.md` as a plan, don't build)

- macOS CI runner, cgo build for arm64 + amd64 (universal binary), `.app` bundle in a `.dmg`, **unsigned** for now: friends approve it once via System Settings → Privacy & Security → "Open Anyway" (document this with the current macOS wording).
- Updater variant that swaps the `.app` bundle; same signature/hash checks.
- Audio: the app's miniaudio backend on CoreAudio; file dialogs; keyboard shortcuts (Cmd vs Ctrl); config dir `~/Library/Application Support/headliner`.
- Needs a tester with a Mac and a guitar interface before release; list what must be verified (devices, calibration, detection, timing).
