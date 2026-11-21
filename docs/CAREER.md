# HEADLINER — Landing Page, Performance Mode & Career Tour

Save as `docs/CAREER.md`. Re-read at the start of every stage and after any context compaction.

**Prerequisites (stop and tell me if missing):** `docs/REDESIGN.md` (tokens, pixel kit), `docs/MOTION.md` (motion tokens + engine), `docs/SCORING.md` (scoring, excitement, XP, levels, per-profile `progress.json`, tap calibration). Build this **before** `docs/BATTLES.md`.

Where this file and MOTION.md disagree on motion, MOTION.md's rules win; this file adds the career-specific choreography in §9.

---

## 0. What this builds

1. A new **landing page** with three doors: **Practice · Perform · Battles**.
2. **Mastery**: a per-profile record of which songs/tracks are mastered.
3. **Performance mode**: one-take gigs at full speed in a **side-view pixel-art scene** — your guitarist on stage, a crowd that reacts to excitement.
4. **The career tour**: Bedroom → Jam room → Pub gigs → Opening act → Stadium, through real cities with fictional venues, starting in a hometown you pick. Recording-studio gigs between tiers.
5. **Gig ratings** (1–5 stars), **tough crowds** (can boo you off), **Fame**.
6. An **asset pipeline** so all art and crowd audio are drop-in files with a licence record — placeholders first, real art swapped in later.

Out of scope: cosmetics shop/spending Fame, character customisation (presets only now), online anything, new detection/scoring rules, note highway.

---

## 1. Landing page

Replaces the current start screen as the first screen. The current start screen (song library) becomes the **Practice** screen, unchanged apart from a Back button.

Layout (REDESIGN tokens, 32px margins):
- **Top strip:** HEADLINER wordmark left. Right: active profile name, `LV 7 · Garage Player`, 12-block XP bar, Fame (pixel star + number), Settings and Help icons.
- **Three doors** across the middle, notched panels, each with a pixel illustration (placeholder until art lands), a title and one line:
  - **Perform** — largest, centre, the view's one filled `brand` element: "Your career: <current venue>, <city>" + next unmet requirement ("Master 1 more song to play the Jam Room").
  - **Practice** — "Learn and master songs".
  - **Battles** — shown only once BATTLES.md is implemented. Until then the door is absent (no "coming soon").
- Keys: P = Practice, G = Perform (gig), B = Battles, S = Settings, ? = Help, Esc/Q = quit. Add them to the help bindings table.
- First launch only: the **career setup** (§5.1) runs before the landing page.

---

## 2. Mastery

- A song+track is **mastered** for a profile when that profile completes **one full run (first bar to last bar, no loop) at 100% tempo with ≥ 90% accuracy** in Practice, in a credible session (SCORING.md credibility gate).
- Stored per profile in `progress.json`: `mastered[pieceID+track] = {date, accuracy, tightness}`. Never removed automatically.
- Practice screen song rows show a pixel "MASTERED" badge; the pass review after a qualifying run shows a one-off "Song mastered" moment (§9).
- Difficulty rating (Easy/Medium/Hard/Expert, SCORING.md §4.5) is shown next to every song.

---

## 3. Performance rules

- **Only mastered songs** can be put in a gig's setlist.
- **One take, 100% tempo.** No loop, slow-down, pause, wait mode, seeking or count-in toggle (a fixed 1-bar count-in before each song). Esc opens "Leave the stage? This gig won't count" (Cancel default focus).
- Scoring, excitement, ghosts and verdicts work exactly as in SCORING.md. Ghosts stay on (they're feedback, not help).
- The practised track is the one the song was mastered on.
- Between songs in a set: a 4-second break (crowd cheer, next song card); Space skips it.

### 3.1 Gig rating

```
excitementAvg = time-weighted average excitement over the set (0–100)
gigScore      = 0.75 × accuracy + 0.25 × (excitementAvg / 100)      -- accuracy over the whole set
stars: ≥ 0.60 ★ · ≥ 0.70 ★★ · ≥ 0.80 ★★★ · ≥ 0.88 ★★★★ · ≥ 0.94 ★★★★★ ; below 0.60 = 0 stars
```
Best stars per gig are kept per profile. Constants in one tuning file.

### 3.2 Tough crowds

- A gig's crowd type is **rolled once, the first time that profile attempts that gig**, then stored for good.
- Odds, per profile: `p` starts at **20%**; every roll that comes up normal adds **+10%** (capped at 100%); a tough roll resets `p` to **20%**. Store `p` in the profile.
- Recording sessions have no crowd and are never rolled.
- First attempt: revealed on stage at the start (§9 "Tough crowd reveal"). Every later visit: the gig card shows **TOUGH CROWD** with animated pixel flames.
- Tough-crowd rules: excitement starts at **30** instead of 50, drains **1.5×** as fast, and after a **10-second grace period** at the start of each song, excitement reaching **0** gets you **booed off**.
- Booed off: the set stops; the gig scores **0 stars**; **XP for notes already played is kept**; Fame 0; retry any time (it stays a tough crowd).
- Normal crowds can never boo you off.

### 3.3 Rewards

- **XP:** every played note earns XP exactly as in Practice (SCORING.md §4.5, tempo factor 1.0), plus a gig bonus `50 × stars × tierNumber` on completion.
- **Fame:** earned only from gigs: `stars × tierFame × (1.5 if tough crowd)`, tierFame = Bedroom 10 · Jam 20 · Pub 40 · Opening 80 · Stadium 160 · Recording = the tier it sits in. Fame is awarded only when a gig's **best** stars improve (the improvement × the formula), so replaying a cleared gig can't farm Fame. Fame is shown, not spent, in this phase.

---

## 4. The tour

### 4.1 Tiers

| # | Tier | Gigs | Setlist | Book it by (all required) | Crowd |
|---|---|---|---|---|---|
| 1 | Bedroom | 4 + 1 recording | 1 song | Level 1 · 1 mastered song | Friends on the floor (3–5 people) |
| 2 | Jam room | 4 + 1 recording | 2 songs | Level 5 · 3 mastered · Tier 1 cleared | Other musicians, ~10 |
| 3 | Pub gigs | 4 + 1 recording | 3 songs | Level 10 · 5 mastered · Tier 2 cleared | Bar crowd, ~40 |
| 4 | Opening act | 4 + 1 recording | 4 songs | Level 20 · 8 mastered (≥ 2 Hard+) · Tier 3 cleared | Theatre/club crowd, ~300 visible as rows |
| 5 | Stadium | 4 + 1 recording | 6 songs + encore | Level 30 · 12 mastered (≥ 3 Hard+) · Tier 4 cleared | Stadium, crowd as layered silhouettes + lights |

- **Tier cleared** = any **3 gigs of that tier at ≥ 3 stars** (the recording session counts as a gig).
- Locked tiers/gigs show their requirements with ticks for the ones met.
- All numbers are constants in the tuning file.

### 4.2 Cities

- First launch: choose a **hometown** from this list (and a preset character, §6): Mumbai, Delhi, Bangalore · Tokyo, Seoul, Singapore · London, Berlin, Paris, Dublin · New York, Los Angeles, Austin, Toronto · São Paulo, Mexico City, Buenos Aires · Sydney, Melbourne · Lagos, Cape Town. Each has a `region`.
- Route: Bedroom + Jam room in the hometown · Pub gigs in the hometown + 2 cities from the same region · Opening acts in 3 cities from other regions · Stadiums in 3 world cities from other regions (seeded per profile so the route is stable).
- **Venue names are fictional**, generated from city-flavoured templates ("The Copper Kettle", "Basement 9", "Riverside Hall"); never a real venue, brand, band or landmark name. A gig card reads: "Pub gigs · The Copper Kettle · Bangalore".
- City identity comes from the backdrop seen through windows/behind the stage (§6.2): region palette + a skyline layer. No real landmarks drawn.

### 4.3 Perform screen (the tour map)

- Header: city name (display font), tier name, tier progress ("2 / 3 gigs at ★★★"), Fame.
- Body: the tour as a horizontal pixel road; tiers are stops, the current one expanded into its gig cards: venue, city, setlist size, best stars (pixel stars), **TOUGH CROWD** flames if revealed, lock + requirements if locked.
- Selecting a gig → **setlist picker**: mastered songs only (with difficulty and best accuracy), choose N in order; "Play gig" (primary). Remember the last setlist per gig.
- Recording session card: studio icon, "No crowd · judged on accuracy and tightness".

---

## 5. Career setup and progress data

### 5.1 First launch
Choose: profile name (default "Player 1"), hometown (list §4.2), guitarist (4 presets §6.3). Editable later in Settings → Career (hometown change resets nothing but the route's display names; confirm dialog).

### 5.2 `progress.json` additions (per profile)
`career{hometown, routeSeed, character, toughP, gigs{gigID: {crowd: "normal"|"tough"|null, bestStars, attempts, lastSetlist}}, tiersCleared[], fame}`, `mastered{...}`. Versioned migration from SCORING.md's schema; never drop existing data.

---

## 6. The performance scene (side view)

### 6.1 Layout
- Screen splits: **scene band on top (~42% of height)**, the tab below (the rest), HUD in the track bar between them. The tab must keep at least 36px string spacing at 1280×720; if it can't, the scene band shrinks first.
- The scene renders to an offscreen image at a **base resolution of 480×200**, then scales by the **largest whole-number factor** that fits the band width, nearest filtering, centred. Any leftover width is filled by extending the scene's edge layers (walls/sky), never black bars.
- Layers back to front: backdrop (sky/skyline or wall) → venue interior → lights → crowd back rows → stage + amp/drum props → **your guitarist** (left third, facing right) → crowd front row (right side, facing left) → effects (light beams, confetti, boo text).

### 6.2 What the scene reacts to

| Signal | Scene response |
|---|---|
| Excitement band Cold / Warm / Hot / On fire | Crowd animation state: idle sway / head-nod / jumping / jumping + arms up + phone-light sprites; venue lights brighten and colour-cycle faster per band |
| Note hit streak of 10 | One crowd member does a cheer pose (2 frames) |
| Excitement low (< 15) on a tough crowd | Crowd arms-crossed idle; "BOO" pixel text puffs occasionally |
| Song playing | Guitarist strum loop, tempo-synced via the same position value the tab reads; idle pose between songs |
| Song end | Guitarist "bow" frames; crowd cheer state for 2 s |
| Booed off | Guitarist slump frames; crowd boo; lights dim (§9) |
| Recording session | No crowd; studio glass, a red "REC" light that's on while playing |

All crowd/guitarist motion is **authored sprite frames** played as stepped animation (MOTION §A2), crowd members phase-offset so they never move in sync. Nothing here adds reads to the audio thread; it only reads values the UI already has (excitement, position, verdict events).

### 6.3 Characters
4 preset guitarists (different silhouettes, skin tones, hair, outfits), each with frames: idle (2), strum (4), cheer/rock pose (2), bow (3), slump (3). Same pixel scale as the scene (character ~48 px tall at base res). Customisation comes in a later phase; keep the character as one sprite sheet + palette so recolouring is possible later.

### 6.4 Crowd audio
- Crowd bed (ambient murmur/cheer loop) volume follows excitement band; cheer stingers at song end, streak cheers (rate-limited, max one per 8 s), boo at booed off.
- Played through the existing engine's backing/mixer path as its own **Crowd** channel in the mixer, default −18 dB under the song; never routed into the input; never affects detection.
- Crowd audio is off in recording sessions.

---

## 7. Asset pipeline (art and audio are swappable files)

- `internal/ui/assets/career/` (embedded) with one folder per asset group: `scenes/<tier>/`, `characters/<preset>/`, `crowd/`, `fx/`, `cities/<region>/`, `audio/`.
- Each group has a `manifest.json`: image file, frame size, frame count per animation, anchor point, layer, base-res position. The renderer reads manifests only — swapping art never needs code changes.
- **Stage 1 ships placeholder art drawn in code** (simple, consistent, palette-correct pixel shapes) so every system works before real art exists. Placeholders follow the same manifest format.
- `assets/CREDITS.md`: for every non-original file — title, author, source URL, licence, date checked.
- **Allowed licences:** CC0, CC-BY 3.0/4.0 (credited in CREDITS.md and an in-app Credits screen), or art the user provides/commissions. **Not allowed:** "personal use only", NC/NonCommercial, "free" with no licence text, AI-image gallery licences, anything you can't link to a licence. Check the licence on the asset's own download page at the time you add it; if unclear, don't use it and list it in your report.
- Known candidates (verify the licence on the page yourself before use):
  - City skyline parallax layers — "FREE Pixel Art – City Parallax Background" by karsiori on itch.io, listed as CC0: https://karsiori.itch.io/free-pixel-art-city-background-pack
  - Crowd cheering (medium crowd, cheering/clapping/woo-ing, 13 s) — "Crowd Cheering" by SoundsExciting on Freesound, CC0: https://freesound.org/people/SoundsExciting/sounds/365132/
  - More CC0 backgrounds to browse: https://opengameart.org/content/cc0-backgrounds
- Side-view venue interiors, the guitarist sprites and crowd sprites: no reliable free side-view sets were found. Keep the code placeholders and leave `docs/ART_BRIEF.md` (§8) for the user to commission or generate them.

---

## 8. `docs/ART_BRIEF.md` (write this file as part of Stage 1)

A self-contained brief an artist, Claude Design or a later Claude Code session can produce art from: base resolution 480×200, palette (REDESIGN tokens + a per-tier accent ramp: Bedroom warm lamp orange, Jam room teal, Pub amber/wood, Opening act magenta stage, Stadium white/blue floods), pixel scale rules (1px = 1 art pixel, no anti-aliasing, max 32 colours per scene), every asset with exact frame counts and sizes from §6, the manifest format from §7, and a list of the placeholder files each asset replaces.

---

## 9. Choreography (MOTION.md tokens; Space skips any sequence to its end state; reduced motion shows end states)

- **Gig intro (≤ 2.5 s):** scene lights off → venue name + city card fades in (`overlayIn`) → lights come up stepped (`steps4`) → guitarist walks from the wing to the mark (whole-pixel steps, `travel` timing) → crowd cheer swell (audio + crowd jump frames) → "TOUGH CROWD" reveal if this is the first attempt at a tough gig: flames sprite pops (`steps3`) beside the card, crowd switches to arms-crossed, card holds 1 s → count-in.
- **Between songs:** song title card in/out (`toastIn`/`toastOut`), crowd cheer state, 4 s break.
- **Gig end (peak-end, ≤ 2.5 s):** guitarist bow → stars reveal one at a time (`steps3` pop each, 150 ms apart; only earned stars light) → gig bonus XP and Fame roll (`roll`) → "New best" pill if improved → buttons (Back to tour = primary).
- **Booed off:** excitement hits 0 → crowd boo frames + "BOO" puffs (`steps3`) → lights dim stepped → guitarist slump → "Booed off — 0 stars. XP kept: +N" card → Retry / Back to tour. Never a shake, never red flashes on the tab.
- **Tier cleared (rare):** after the gig end, a full-screen moment: tier name card, next tier's venue silhouette slides in (`travel`), "Booked: Jam Room, Mumbai", Fame total rolls. ≤ 2.5 s.
- **Song mastered (rare):** in Practice's pass review: "MASTERED" pixel stamp pops (`steps3`) with a secondary spark burst, then "Now available for gigs".
- **Tour map:** moving between tiers slides the road (`move`, `easeInOut`); key navigation moves instantly.
- **Tough-crowd flames** on gig cards: ambient 3-frame loop, phase-offset per card, paused off-screen.

---

## 10. Stages (one commit each; tests + `CGO_ENABLED=1 go build ./cmd/musictutor` pass after each)

1. **Data + placeholders**: mastery, career data, tough-crowd odds, gig rating, Fame, tier rules as a pure package `internal/career` (unit-tested), placeholder art in code, manifests, `ART_BRIEF.md`, `CREDITS.md`. → **CHECKPOINT A**: test output; wait.
2. **Landing page + career setup + Practice relabel.**
3. **Perform screen (tour map, gig cards, setlist picker).**
4. **Performance mode + scene + crowd audio.** → **CHECKPOINT B**: screenshots (`HEADLINER_SHOT`) of the landing page, tour map, a normal gig, a tough gig, booed-off card, gig end — at 1366×768 and 1920×1080; wait.
5. **Choreography (§9).**
6. **Validation + report.**

### Tests to write first (stage 1)
Mastery only from a full credible run ≥ 90% at 100%; tough-odds sequence (20, 30, 40… reset to 20 after tough; never rolled twice for one gig; never for recording); gig stars thresholds; booed-off keeps XP and gives 0 stars/0 Fame; Fame only on best-star improvement (replaying a cleared gig gives 0 Fame); tier clear = 3 gigs ≥ 3★ including recording; route generation stable for a seed and never uses a real venue name list; schema migration keeps all SCORING.md data.

### Validation (stage 6) — write these as plain steps for me at the end of your report
- Fresh profile: pick hometown + character; landing shows three doors (Battles hidden until implemented).
- Master one song in Practice (full run ≥ 90% at 100%): "Song mastered" moment; it appears in the setlist picker.
- Play a Bedroom gig: intro, crowd reacts as excitement changes, stars at the end, XP + Fame.
- Force a tough crowd (a dev flag `HEADLINER_FORCE_TOUGH=1`): reveal on first attempt, flames on the card after; let excitement hit 0 → booed off, XP kept, 0 stars.
- Clear Tier 1 (dev flag `HEADLINER_GRANT=tier1` may pre-fill gig results for testing only; it must not exist in normal runs).
- Mixer shows a Crowd channel; crowd is silent in a recording session.
- Report every file changed, every asset added with its licence, and anything left as placeholder.
