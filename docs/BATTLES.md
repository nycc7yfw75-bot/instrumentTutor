# HEADLINER — Guitar Battles

Save as `docs/BATTLES.md` in the repo. Re-read at the start of every stage and after any context compaction.

**Prerequisites (stop and tell me if any is missing):** `docs/REDESIGN.md` landed (tokens, pixel kit) · `docs/SCORING.md` landed (`internal/scoring`, per-profile `progress.json`, best-pass replays, tap calibration, ghosts). Battles reuse all of it — do not duplicate scoring logic. **All battle motion follows `docs/MOTION.md` Part B2** (setup, VS intro, turn banner, scoreboard, result card chain, lead-change crown, crowd meter, final results, confirms); where this file and MOTION.md disagree on motion, MOTION.md wins.

---

## 0. Scope

Build: a Battles mode, offline only. 2–4 participants take turns on the same laptop: any mix of local human profiles and bots. Format is mirrored trade-bars. Three bot types. Profile management. Sound check. Battle-riff pack. XP bonus and win/loss records.

Do NOT build: online play, power-ups/attacks, venues/career events, cosmetics, separate battle rating (Elo), changes to detection, matching, playback timing or file formats.

Honesty rules: never show a score for a turn that wasn't actually judged; bots are labelled as bots; never fake audio the engine can't produce (see §5.3).

---

## 1. Concepts

| Term | Meaning |
|---|---|
| Battle | One match: material + participants + settings + results |
| Participant | A human **profile**, a **rival** bot, a **ghost** (a profile's stored best pass) or a **slider bot** |
| Section | The bar range being battled (a song section or a battle riff) |
| Phrase | `phraseBars` consecutive bars of the section (default 4) |
| Exchange | Every participant plays the same phrase once, in turn order (mirrored) |
| Battle length | `exchanges` (default 4) → phrase k covers bars `start + (k−1)·phraseBars …` |
| Turn | One participant playing one phrase |

If the section is shorter than `phraseBars × exchanges`, reduce exchanges to fit and say so at setup. If a phrase has fewer than 8 judgeable notes, warn at setup and suggest longer phrases.

---

## 2. Stages (one commit each; tests + `CGO_ENABLED=1 go build ./cmd/musictutor` pass after each)

1. **Profiles + sound check**
2. **Battle core** (pure package, unit-tested) → **CHECKPOINT A**: show the test table output; wait.
3. **Bots**
4. **Battle UI** (setup, turns, results) → **CHECKPOINT B**: screenshots at 1366×768 and 1920×1080 via `HEADLINER_SHOT`; wait.
5. **Battle-riff pack**
6. **Records, XP, validation + report**

---

## 3. Stage 1 — Profiles and sound check

- Profiles live in `progress.json` (already per-profile from SCORING.md). Add UI: **Profiles** in Settings and in battle setup — create (name, max 20 chars), rename, delete (confirm dialog naming the profile and stating its XP/records will be lost; Cancel has default focus; button reads "Delete profile" — MOTION §A6), set active.
- Each profile shows: pixel guitar icon tinted by a per-profile colour picked from 6 fixed, non-token-clashing tints (not `brand`, not verdict colours — add them as tokens `p1..p6`), name, level + title, battle record.
- **Sound check** (per human participant, before their first turn): "Play your open strings" — 3 s window showing the existing input level as the segmented meter. Pass = level rose above the detector's existing gate on ≥ 3 distinct onsets. Fail → "Can't hear you — turn your guitar volume up" + retry. This checks level only; it does not change calibration or detection.
- Calibration is per device pair (shared rig), done once via Tap calibrate. If the rig is uncalibrated, battles still run but every human turn is **Unscored** and the battle can't produce a winner — block "Start battle" with a button to Tap calibrate.

---

## 4. Stage 2 — Battle core (`internal/battle`)

Imports `internal/scoring`, `internal/practice`, `internal/score` only. No UI/engine/audio. Constants in `internal/battle/tuning.go`. Deterministic: all randomness via a seeded RNG passed in.

### 4.1 Setup
`Setup{Material, StartBar, PhraseBars=4, Exchanges=4, Participants[2..4], TurnOrder}`. Each human participant has their own `TempoScale` (handicap), fixed for the battle; bots use 1.0 unless set.

Turn order: setup order for exchange 1; each later exchange rotates the order by one so nobody always goes first.

### 4.2 Turn score

```
accuracy    = scoring accuracy of the turn (Σ noteScore / judgedNotes, SCORING.md §4.1–4.2)
battleTempo = sqrt(min(tempoScale, 1.0))        -- handicap curve: 60% speed counts as 0.77
turnScore   = round(1000 × accuracy × battleTempo)
```

Battle total = Σ turnScores. Winner = highest total. Ties broken by: (1) smaller |mean timing offset| across the battle, (2) smaller timing spread, (3) declared a draw.

Note: XP still uses SCORING.md's linear tempo factor; only battle comparison uses the square root.

### 4.3 Crowd meter (visual only — never affects the winner)

- 2 participants: position `c ∈ [−1, 1]`, starts 0. After each exchange: `c += clamp((sA − sB) / 1000 × 1.5, −0.4, 0.4)`, then clamp to [−1, 1].
- 3–4 participants: crowd share = each participant's running total ÷ sum of totals, drawn as a split bar.

### 4.4 Tests
Phrase splitting and shortening; turn-order rotation; turnScore with handicaps (assert 60% @ 95% = 736, 100% @ 75% = 750); tie-break chain; crowd clamp; unscored turn blocks a winner; determinism with a fixed seed.

---

## 5. Stage 3 — Bots

### 5.1 Simulation model
A bot turn produces synthetic `NoteResult`-equivalents for each judgeable note of the phrase, from a profile:

```
BotProfile{ HitP, CloseP, MissP          // sum to 1 at difficulty 1.0
            TimingBiasMs, TimingSpreadMs  // offset ~ Normal(bias, spread), clamped to ±150
            SpeedChoke                    // extra MissP per note/sec above 4 (fast passages)
            ChordPenalty }                // extra MissP on chords
```

Per note: draw verdict (with choke/penalty adjustments, renormalised), draw timing offset, then run it through the **same** `internal/scoring` functions humans use. Seeded RNG per battle.

### 5.2 The three opponent types (all selectable in setup)

**Named rivals** — level-gated roster (the player's active profile level), each with a pixel portrait (16×16, 2×) and a one-line bio:

| Rival | Unlocks at level | Hit / Close / Miss | Bias / spread | Habit |
|---|---|---|---|---|
| Basement Benny | 1 | 60 / 20 / 20 | +40 / 55 ms | Drags, sloppy |
| Rusher Rex | 1 | 70 / 15 / 15 | −35 / 40 ms | Rushes everything |
| Metronome Mira | 3 | 72 / 18 / 10 | 0 / 20 ms | Dead on time, misses notes |
| Velvet Vik | 6 | 80 / 12 / 8 | +15 / 30 ms | Laid back |
| Clockwork Kai | 10 | 85 / 10 / 5 | 0 / 15 ms | Machine-tight, chokes on chords |
| Shredder Sol | 15 | 88 / 8 / 4 | −20 / 25 ms | Fast, chokes less on speed |
| Ghost-Note Gwen | 20 | 92 / 6 / 2 | +5 / 15 ms | Clean, rarely rattled |
| The Headliner | 30 | 96 / 3 / 1 | 0 / 10 ms | Final boss |

Locked rivals show as silhouettes with "Unlocks at level N". These are all original characters — no real musicians' names or likenesses.

**Your own ghost** — replays the stored best-pass per-note results for (piece, track, bar range) from a chosen profile (SCORING.md §5). Available only when a stored replay covers every phrase in the battle; otherwise shown disabled with "Play this section once to create a ghost". No randomness.

**Difficulty slider bot** — "Session Player" with Easy / Medium / Hard / Expert mapping to the Benny / Velvet Vik / Ghost-Note Gwen / The Headliner parameter sets (no portrait, generic pixel amp icon).

### 5.3 How a bot turn looks and sounds
- The phrase plays through the existing synth on the battled track's voice, exactly as written (the engine plays the score; do not add per-note timing shifts to playback).
- If the engine already supports silencing individual events cheaply and safely, silence notes the bot "missed"; if not, don't touch the engine — play the phrase as written and show the misses visually only. Say which in your report.
- The bot's verdicts and timing ghosts animate on the tab at each note's time, using the same rendering as a human turn.
- Label on screen during the turn: rival name + "BOT".

---

## 6. Stage 4 — Battle UI

Follow REDESIGN tokens, pixel kit, notched corners. Every screen keyboard-operable; Esc backs out (mid-battle Esc asks "Abandon battle? No record will be saved").

### 6.1 Entry
Start screen gets a **Battles** item (pixel crossed-picks icon) next to Open song. Available from level 1.

### 6.2 Setup screen (one screen, top to bottom)
1. **Material**: tabs "Your songs" / "Battle riffs"; pick piece → track → section (existing section markers; else bar range picker).
2. **Participants** (2–4 slots): each slot = Profile ▾ / Rival ▾ / Ghost ▾ / Session Player ▾. Humans pick their tempo (`‹ 80% ›`, 50–100% in 5% steps) — show the handicap value beside it ("counts as ×0.89").
3. **Rules**: Phrase bars `‹ 4 ›` (2–8), Exchanges `‹ 4 ›` (1–8), warnings from §1 inline.
4. **Start battle** (primary button). Disabled with a reason when invalid (uncalibrated rig, <2 participants, duplicate profile in two slots, ghost unavailable).

### 6.3 Battle screen
- **Top: scoreboard** — one column per participant: portrait/icon, name, BOT tag if bot, running total (display font), and a 4-dot exchange tracker.
- **Crowd meter** under it (§4.3): 2 players = 24-block pixel tug-of-war bar with a pixel crowd marker; 3–4 players = split bar in profile tints.
- **Middle: tab** — the current phrase only, using the practice tab renderer (verdicts, live ghosts).
- **Turn flow:**
  1. Banner: "EXCHANGE 2 / 4 — PLAYER: Aaditya" (+ "pass the guitar" if the previous turn was a different human).
  2. Human: "Ready" (Space) → sound check if first turn → count-in 1 bar → play phrase → result card.
  3. Bot: 1 s "Rusher Rex is up…" → plays → result card.
  4. **Result card**: turn accuracy, tightness label, turnScore, then crowd swing and total — chained, never simultaneous (MOTION §B2.5). Space jumps to the end; auto-advance 3 s after the last step.
  - Battle start plays the VS intro (MOTION §B2.2); lead changes move the crown (§B2.6).
- No pausing a turn mid-phrase except Esc (abandon prompt). Wait mode and looping are off in battles.

### 6.4 Results screen
Choreographed per MOTION §B2.8 (rows → totals → trophy drop → XP → buttons; Space skips). Winner banner (pixel trophy), final totals, per-exchange table (rows = exchanges, columns = participants, cells = turnScore + accuracy), tiebreak note if used, XP awarded per human profile, buttons: **Rematch** (same setup), **New battle**, **Done**. Then "Review a turn" opens SCORING.md's pass review for any human turn.

---

## 7. Stage 5 — Battle-riff pack

- Use the `gtab-writer` skill (installed plugin) to write 8 original riffs into the app's pieces folder under `battles/`: two each at Easy, Medium, Hard, Expert. 16 bars each (4 phrases × 4 bars), single practised track + optional backing bass, section comments every 4 bars. Single-note lines; at most one chord per phrase.
- Validate every file with `./musictutor render -o check.wav <file>`; delete check.wav after.
- The "Battle riffs" tab lists exactly these files, sorted by difficulty.

---

## 8. Stage 6 — Records, XP, validation

### 8.1 XP (human profiles only)
- Every scored human turn is a pass under SCORING.md §4.5 (normal XP, linear tempo factor, repeat factor applies per phrase range).
- Winner bonus: `+25 × (number of other participants)`. Draw: `+10` each. Losing earns no bonus — never negative.
- Credibility gate from SCORING.md applies per turn.

### 8.2 Records (per profile)
- Overall wins / losses / draws; per opponent (each rival, each other profile, ghost, session-player level); best battle score; last 50 battles (date, material, participants, totals, winner).
- Shown on the profile card and a **Records** tab in the Battles menu.

### 8.3 Validation
1. All tests pass; build succeeds.
2. Manual script for me (plain steps, non-programmer), ending your report:
   - Create two profiles; run a 2-human battle at different tempos; confirm the handicap shows and totals add up.
   - Battle Basement Benny at level 1; confirm locked rivals show silhouettes.
   - Play a section once, then battle your own ghost on it.
   - Run a 4-participant battle (2 humans + 2 bots); confirm turn order rotates and the split crowd bar works.
   - Force a tie (e.g. two ghosts of the same pass) and confirm the tiebreak / draw.
   - Esc mid-battle: no record saved.
   - Quit and reopen: records and XP persist per profile.
3. Run the MOTION.md §C checks for Part B2.
4. Report: every file added/changed, whether bot misses are audible (§5.3), anything skipped and why. Don't claim untested behaviour works.
