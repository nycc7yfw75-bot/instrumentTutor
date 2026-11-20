# HEADLINER — Scoring, Excitement, XP, Levels and Timing Ghosts

Save as `docs/SCORING.md` in the repo. Re-read at the start of every stage and after any context compaction. This is the spec; numbers here are the defaults, all kept as named constants in one file so they can be tuned.

---

## 0. Scope

Build: per-note scoring with timing, accuracy %, tightness (rush/drag), excitement bar, XP, levels with titles, timing ghosts (live + review), tap calibration, persistence of progress.

Also build: a minimal **player profile store** (see §5) with one default profile, so Battles can add 2–4 local players later without migrating data. No profile-switching UI yet beyond renaming the default profile in Settings.

Do NOT build: venues, career ladder, venue gating logic, cosmetics/unlocks, achievements, streaks, leaderboards, note highway. Store the player's level so venues can read it later; nothing gates on it yet.

Do NOT change: pitch detection, the matching window, hit/close/miss pitch rules, playback, the audio engine, file formats. Scoring reads results the scorer already produces.

Visual work follows `docs/REDESIGN.md` tokens and pixel kit if that redesign has landed; otherwise use the existing theme and keep everything easy to restyle (no literal colours). **All motion follows `docs/MOTION.md` Part B1** (excitement bar, ghosts, XP bar, XP toast, level up, pass review, tap calibration) and its tokens; where this file and MOTION.md disagree on motion, MOTION.md wins.

---

## 1. What already exists (verify, don't assume)

- `internal/practice/practice.go`: `NoteResult{Event, OutFrame, Verdict, Matched, ErrCents, ErrFrames}`; `Stats{Hit, Close, Miss}` with `Accuracy() = (hit + 0.5·close)/n`. Match window defaults to 150 ms (`defaultWindowMillis`). Hit/Close are **pitch** tiers (cents), not timing.
- `ErrFrames = detOut − exp.outFrame` — timing error per matched note, in frames.
- `internal/pitch/detector.go` has onset detection (`onsetJumpDB`, flux onset).
- Latency offset per device pair in `internal/appconfig` (`LatencyOffsets`, `SetOffset`, confidence).

**First task:** read the scorer and confirm whether `detOut` is the note's **onset** (pick attack) frame or the moment pitch was **recognised**. If it's pitch-recognition time, timing is biased late, more so on low strings (82 Hz needs more cycles). In that case, carry the onset frame from the detector through to `NoteResult` (add a field, e.g. `OnsetErrFrames`) and use it for all timing below. Report what you found before building on it.

---

## 2. Stage order (one commit per stage; `go test ./...` and `CGO_ENABLED=1 go build ./cmd/musictutor` pass after each)

0. **Tap calibration** (prerequisite — ghosts are meaningless without it)
1. **Scoring core** (pure package, fully unit-tested, no UI)
2. **Persistence** (progress file)
3. **HUD: accuracy, excitement, XP bar, level**
4. **Live ghosts**
5. **Pass review screen**
6. **Validation + report**

→ **CHECKPOINT after stage 1** (report the onset finding, show the test table output) and **after stage 4** (screenshots). Wait for my go-ahead each time.

---

## 3. Stage 0 — Tap calibration

The user's interface (Marshall CODE 25 over USB) plays computer audio but does not return it over USB, so loopback calibration fails.

- New Settings row under Calibration: **Tap calibrate**. Plays 16 clicks at 90 BPM through the playback device. The user plucks a palm-muted string on each click. The app uses the detector's onset frames for those plucks.
- Discard the first 4 taps (settling). Offset = median of (onset − click) over the remaining 12. Confidence from spread: IQR ≤ 10 ms → 0.9, ≤ 20 ms → 0.7, ≤ 35 ms → 0.5, else refuse and ask to retry.
- Save via the existing `SetOffset` for the current device pair with that confidence, so every existing code path picks it up.
- Plus a manual fine-tune: `‹ ±ms ›` in 2 ms steps, range ±150 ms, saved to the same offset.
- Show the result in plain words: "Offset 34 ms · steady" / "Too uneven — try again with muted strings".

---

## 4. Stage 1 — Scoring core

New package `internal/scoring` importing only `internal/practice` and `internal/score`. Pure functions + a `Session` accumulator. No UI, audio or engine imports. All constants in `internal/scoring/tuning.go`.

### 4.1 Per-note score

```
pitchCredit:  Hit = 1.0   Close = 0.5   Miss = 0
timingTier (|offset| after latency offset is applied):
  Perfect  ≤ 35 ms   × 1.00
  Good     ≤ 70 ms   × 0.85
  Loose    ≤ 150 ms  × 0.60   (150 = existing match window)
noteScore = pitchCredit × timingMultiplier       (Miss → 0)
```

- Notes the scorer can't fairly judge are **excluded** from every total (count neither for nor against): dead notes, bends, and anything the existing chord logic marks as unjudged. Chords: one judgement per chord (pitch per existing chord scoring; timing from the strum onset).
- Offset sign: positive = late (dragging), negative = early (rushing).

### 4.2 Accuracy

`accuracy = Σ noteScore / judgedNotes`, shown as a whole percent. Computed per pass, per loop section, and per song run. Never shown when judgedNotes < 8 (show "—").

### 4.3 Tightness

Over Hit and Close notes only:
- `mean` signed offset (ms) and `spread` (standard deviation, ms).
- Label: |mean| < 10 ms → "Locked in"; mean ≥ 10 → "Dragging +N ms"; mean ≤ −10 → "Rushing −N ms". Spread label: < 20 "tight", < 40 "loose", else "uneven".

### 4.4 Excitement (0–100)

- Starts at 50 when playback starts. Never below 0, never above 100.
- On each judged note: Perfect hit +4, Good hit +3, Loose hit +2, Close (any timing) +1, Miss +0.
- Streak bonus: every 10 consecutive non-miss notes, +5.
- Drain: −3 per second **of transport playing time** (frame-derived, never wall clock). Paused, waiting (wait mode) or count-in → no drain.
- Misses never subtract directly; the bar falls only by drain.
- Bands for the UI: 0–24 Cold · 25–49 Warm · 50–74 Hot · 75–100 On fire.

### 4.5 XP

Awarded when a **pass completes** (one loop pass, or a full run to the end), never mid-pass, never negative.

```
difficulty  = Easy 1.0 | Medium 1.25 | Hard 1.5 | Expert 2.0
              (from the practised track at full tempo: notes/sec < 2 Easy, < 4 Medium, < 7 Hard, else Expert;
               computed once per piece+track from score events, cached)
tempoFactor = min(playbackScale, 1.0)            -- 60% speed earns 60%
passXP      = 10  +  Σ noteScore × difficulty × tempoFactor     -- Σ noteScore = judgedNotes × accuracy
repeatFactor= max(0.25, 1 − 0.15 × (timesThisSectionPassedToday − 1))
xpAwarded   = round(passXP × repeatFactor)
```

- A pass with judgedNotes < 8 earns nothing.
- "Section" = the loop's bar range, or the whole song for a full run; counts reset at local midnight.
- **Credibility gate:** XP and excitement only count in live sessions where the input is calibrated (stored offset with confidence ≥ 0.5) and no capture samples were dropped during the pass. Otherwise accuracy and ghosts still show, the HUD says "Unscored — calibrate to earn XP", and nothing is saved to XP.

### 4.6 Levels

```
xpToNext(level) = round(100 × level^1.5)     -- L1→2: 100, L5→6: ~1118, L10→11: ~3162
```

Titles (shown with the level):

| Levels | Title |
|---|---|
| 1–4 | Bedroom Player |
| 5–9 | Garage Player |
| 10–14 | Open-Mic Regular |
| 15–19 | Bar Band |
| 20–24 | Club Act |
| 25–29 | Touring Musician |
| 30–39 | Festival Act |
| 40+ | Headliner |

Level and title are stored for the future venue ladder; nothing is gated now.

### 4.7 Tests (write these first)

- Table tests for noteScore across every pitch × timing tier, including exclusions.
- `TestMissNeverReducesXP`: note that `judgedNotes × accuracy = Σ noteScore`, so a miss adds 0 to XP. Assert: inserting a miss at any position leaves the pass's XP unchanged (accuracy % drops — that's honest and intended), XP is never negative, and stored total XP only ever increases.
- Excitement: drain freezes while paused/waiting/count-in; caps at 0 and 100; streak bonus at exactly 10, 20.
- Tightness sign and labels; spread labels.
- Repeat factor and midnight reset with an injected clock.
- Level curve and title boundaries.
- Credibility gate: uncalibrated session awards 0 XP and leaves stored XP untouched.

---

## 5. Stage 2 — Persistence

- `progress.json` beside the existing `config.json` (same directory helper). Do not touch `config.json`.
- Atomic write (temp file + rename), versioned schema (`"version": 1`).
- **Everything is stored per profile.** Top level: `{"version":1, "activeProfile":"<id>", "profiles":{"<id>":{...}}}`. Create one profile on first run: id = random 8-char, name "Player 1" (renamable in Settings). Every read/write goes through the active profile — no global XP anywhere.
- Each profile holds: name, created date, total XP, level, per-piece+track bests (best accuracy, best tightness mean, best tempo), today's section pass counts, a capped log of the last 200 passes (date, piece, track, bars, tempo, accuracy, tightness, xp).
- **Best-pass replays** (needed later by Battles' "your own ghost" opponent): for each (piece, track, bar range) keep the per-note results of that profile's best-accuracy pass — note index, verdict, timing offset ms. Cap at 300 stored ranges per profile (drop least-recently-used).
- Piece identity: file path for now (hashing comes later); store the title alongside.
- Corrupt file → rename aside to `progress.json.bad-<timestamp>` and start fresh; never crash a session over it.

---

## 6. Stage 3 — HUD

In the practice view's HUD area (track bar right side, per REDESIGN §6.7):

- **Accuracy %** (display font) and the existing hit/close/miss counts.
- **Excitement**: 20-block pixel segmented bar; blocks fill with the band colour (Cold `text3` → Warm `text2` → Hot `hit` → On fire `hit` + 2-frame stepped shimmer). Band name in pixel font beside it. Never amber, never red.
- **Level + XP**: "LV 7 · Garage Player" + thin 12-block XP bar to next level.
- XP gain: "+42 XP" toast after each completed pass, queued until the pass ends (MOTION §B1.5). Level up: the choreographed level card (MOTION §B1.6), never while playing.
- Excitement bar motion (block pops, stepped drain, band sweep, On-fire flicker, +5 streak): MOTION §B1.2.
- Unscored state replaces excitement + XP with "Unscored — calibrate to earn XP" (clickable → opens Tap calibrate).

---

## 7. Stage 4 — Live ghosts

On the tab, for each judged note whose timing tier is **not** Perfect:

- Draw a **ghost** at the x-position where the note was actually played (expected x + offset converted with the tab's current px-per-second at the current tempo scale), on the same string.
- Ghost = the fret number in `text2` at 40% opacity inside a 1px dashed notched outline, joined to the real note by a 1px `text3` line.
- Direction without colour: early ghost gets a small pixel `‹` on its left; late ghost a `›` on its right.
- Perfect notes get no ghost (keeps the tab clean; the verdict fill already says "good").
- Misses get no ghost (there's no onset to place). Wrong-pitch notes still get a timing ghost if they had an onset.
- Ghosts appear instantly when judged and fade out across the 2 bars after the playhead passes (MOTION §B1.3).
- Ghost drawing must not read engine state beyond what the tab renderer already reads.

---

## 8. Stage 5 — Pass review

Opens automatically when a full run ends, and on demand for the last completed loop pass via **P** (check the practice view's help bindings first; if P is taken, pick an unused key, add it to the help table, and say which in your report).

- Overlay over the practice view, `surface` panel:
  - Top: pass accuracy %, tightness ("Dragging +18 ms · loose"), XP earned, excitement peak.
  - Middle: **timing strip** — one dot per judged note, x = time across the pass, y = offset (−150 … +150 ms), horizontal bands at ±35 and ±70 ms. Dots: Hit filled, Close outlined, Miss as a small `×` on the zero line. Hover a dot → bar, beat, string, fret, offset.
  - Bottom: the tab for the pass with **all** ghosts shown persistently; ←/→ scroll bars; click a dot to jump the tab to that note.
  - Buttons: "Loop this again" (re-arms the same loop), "Close" (Esc).
- Read-only: reviewing never changes XP or stats.

---

## 9. Stage 6 — Validation

1. All tests pass; build succeeds.
2. Manual check script for me (write it at the end of your report, plain steps for a non-programmer):
   - Run Tap calibrate; confirm the offset is saved and the HUD stops saying Unscored.
   - Play a 4-bar loop at 70%: accuracy shows, excitement moves and drains when you stop playing, ghosts appear on late/early notes, "+N XP" toast after the pass.
   - Pause mid-pass: excitement freezes.
   - Play the same loop 6 times: XP per pass shrinks, never below 25%.
   - Quit and reopen: level and XP persist.
   - Open the pass review; hover dots; jump to a note.
3. Run the MOTION.md §C checks for Part B1 (especially audio isolation: identical verdicts and offsets with animations on/off).
4. Report: the onset finding from §1, every file added/changed, constants file location, anything you couldn't do and why. Do not claim untested behaviour works.
