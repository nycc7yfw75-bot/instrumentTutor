# HEADLINER — Motion & Interaction Spec

Save as `docs/MOTION.md`. It is referenced by `docs/REDESIGN.md` (Part A), `docs/SCORING.md` and `docs/BATTLES.md` (Part B). Re-read the relevant part before any stage that draws motion.

Built on the UX & Animation rules: every interaction answers, in order — **should it move · where should the eye go · how should it move · can people find, hit and choose it · can they finish.** A later answer never rescues an earlier one.

**The headline rule for a guitar game:** the player is playing an instrument. Anything that fires per note, many times a second, or from a key gets **no transition** — it shows on the next frame. Animation is spent on moments that happen a few times a session (overlays, turn changes, band changes) and choreography on the rare ones (level up, battle start, battle results). That's where "more animation" pays off; everywhere else it's latency in front of a musician.

---

## 0. Review of the current briefs (what this spec fixes)

Interactions in scope: menu hover/toggle, buttons, tooltips, overlays (help, tuner, mixer, settings, dialogs, pass review), banners, list rows, filter tabs, timeline seek/drag/loop, track mute/solo, LIVE/NO SIGNAL, input meter, verdicts, playhead, ghosts, accuracy/counters, excitement bar, XP bar, XP toast, level up, tap calibration, battle setup, battle intro, turn banner, bot turn, result card, crowd meter, lead change, final results, confirm dialogs.

| Severity | Principle | Where | Before | After |
|---|---|---|---|---|
| broken | Jakob's law (Esc closes the top layer) | All overlays | Esc behaviour with an overlay open unspecified; Esc on practice = leave song | Esc closes the top-most layer only; leaving the song needs Esc with no layer open (§A3) |
| friction | Frequency of use | REDESIGN §3, §5 menu items | Active-state transition 140ms, including key toggles (C, M, R, W…) | Key-triggered changes instant; transitions only on pointer hover/press (§A1) |
| friction | Staging | SCORING §6 | XP / level-up toasts can fire mid-performance | Queued until the pass ends or the transport stops (§B1.5) |
| friction | Staging | BATTLES §6.3 result card | Score and crowd swing animate together | Chained: card → accuracy → score → crowd → total (§B2.5) |
| friction | Reduced motion | All briefs | Not addressed | `Reduce motion` setting, defaulting from Windows "Show animations" (§A4) |
| friction | Solid pixel drawing | REDESIGN §5 | Pixel bitmaps could be scaled/eased like vector UI → blur | Bitmaps move/scale in whole pixels only; squash/stretch as authored frames (§A2) |
| friction | Frequency of use | SCORING accuracy % | Would re-render/roll on every note | Instant during play; digit roll only on summaries (§B1.1) |
| friction | Fitts's law / hit area | REDESIGN §6.2, §6.6 | Tempo arrows 36×36; loop handles unspecified | ≥ 40×40 pointer targets; loop handles 24px wide hit zone (§A6) |
| polish | Von Restorff | REDESIGN §3 brand use | Several filled amber elements can co-exist | One filled `brand` element per view (§A7) |
| polish | Tooltip timing | REDESIGN §5 | 400ms delay, no duration, no warm state | 400ms first delay, 125ms in, instant for 300ms after one closes (§A5) |
| polish | Errors: recover | REDESIGN §7 `del` forget | Removes a recent with no way back | Row collapses + Undo toast 5s, if the existing forget is reversible in code (§A5) |

---

## 1. Tokens (one file, e.g. `internal/ui/motion.go`)

### Durations (ms)

| Token | Value | Used for |
|---|---|---|
| `press` | 100 | Pressed state |
| `hover` | 150 | Hover colour/underline |
| `tooltipIn` / `tooltipOut` | 125 / 90 | Tooltips |
| `popIn` / `popOut` | 200 / 140 | Small popovers, dropdowns |
| `overlayIn` / `overlayOut` | 260 / 180 | Overlays, dialogs, pass review |
| `toastIn` / `toastOut` | 240 / 170 | Toasts, banners |
| `move` | 250 | Moving/resizing in place: bars filling, rows collapsing, underline sliding, crowd marker |
| `travel` | 350 | Something going somewhere: crown hop, trophy drop, VS cards |
| `roll` | 300 (400 for big totals) | Digit rolls |
| `momentStep` | ≤ 500 | Any single step of a choreographed moment |
| `momentMax` | 2500 | Whole choreographed moment, end to end |
| `staggerCopy` | 40 | Copies of one element (rows, dots, cards), first 6 only |
| `staggerPiece` | 90 | Distinct pieces (heading → copy → buttons) |

### Curves

| Token | Value | Used for |
|---|---|---|
| `easeOut` | cubic-bezier(0.32, 0.72, 0, 1) | Anything appearing |
| `easeIn` | cubic-bezier(0.55, 0, 1, 0.45) | Anything leaving (on the shorter exit duration) |
| `easeInOut` | cubic-bezier(0.77, 0, 0.175, 1) | Moving/resizing while on screen |
| `linear` | — | Time itself: playhead, sweeps that replay time, countdowns |
| `spring` | critically damped, ~500ms settle, no bounce | Drags released (loop handles, mixer faders, scroll fling) |

Implement a cubic-bezier evaluator and a critically damped spring in `anim.go`; every animation in the app uses these tokens — no inline numbers.

### Pixel steps
Pixel-identity elements (icons, sprites, portraits, pixel text pops, meter blocks) animate in **steps**, not eases: `steps2` = 2 frames held 3 frames each (~100ms), `steps3` = 3×3 frames (~150ms), `steps4` = 4×3 frames (~200ms).

---

## PART A — Global rules and app chrome (implement during REDESIGN)

### A1. Should it move?
- **Many times a session or from the keyboard → no transition.** Key toggles (C, M, R, W, L, 1–9, Tab, arrows), list selection by keys, mute/solo, verdict fills, ghost appearance, counters, meter updates, playhead jumps from seeks. The new state shows on the next frame.
- Hover and press feedback keep their `hover`/`press` transitions at any frequency, but **only on the pointer path**: when a key and the pointer share a style, the transition lives on the hover rule only.
- **Nothing at rest on first paint plays an entrance.** No staggered list on launch, no HUD fly-in when a song opens.
- Every motion must answer a question: where did this come from, did my action register, what changed, what caused it. If it answers none, cut it.

### A2. How pixel art moves
- Bitmaps (icons, portraits, sprites, wordmark mark) are drawn at **integer scale and integer positions** with nearest filtering. Never tween their scale; never place them at fractional pixels mid-animation (round every frame).
- Entrances for bitmaps: fade (opacity is fine) and/or a whole-pixel translate (e.g. 4px → 0 in 1px steps).
- Squash/stretch and "pops" on pixel sprites are **authored frames** (normal / squashed / stretched), played with `steps2`–`steps4`. Keep volume: a squash frame is wider by as many pixels as it's shorter.
- Vector surfaces (notched cards, bars, lines, Inter/JetBrains text) may fade and translate smoothly; cards may scale 0.97 → 1 on entrance.

### A3. Layers and Esc
- Layer stack: practice/start screen → overlay (help, tuner, mixer, settings, pass review) → dialog (confirm, BPM entry, save name) → tooltip.
- Esc closes the top-most layer only. Leaving a song (Esc "back") happens only when no layer is open.
- Closed overlays are not drawn and receive no input during or after their exit.

### A4. Reduced motion
- New setting **Display → Reduce motion** (the one new config field this brief allows). Default: on if Windows "Show animations in Windows" is off (`SystemParametersInfo SPI_GETCLIENTAREAANIMATION`); if that call isn't available, default off.
- When on: every transition, fade, roll, stagger, choreography and ambient loop is skipped — end states appear at once, with identical information. The playhead keeps moving (it's time), meters keep updating, and any "working…" indicator keeps moving.
- Dev-only `HEADLINER_SLOWMO=0.1` scales the UI animation clock to 10% for scrubbing (never touches the audio clock).

### A5. Chrome motion

| Element | Motion |
|---|---|
| Menu item hover | Underline grows from centre, `hover` `easeOut`; text `text2`→`text` same timing. Leave: `tooltipOut`-length reverse. |
| Menu item toggled by click | Underline completes to full + text to `brand`, `hover`. By key: instant. |
| Button press | Vector button shifts down 1px (pixel press), `press`; release glides back. No scale (dense toolbar). Primary button same. |
| Tooltip | First one after 400ms hover; fades in + 4px from the trigger side, `tooltipIn` `easeOut`. Moving to a neighbouring control within 300ms of one closing shows the next instantly (warm state). |
| Overlay / dialog | Scrim `bg` at 40% fades with it. Panel: opacity 0→1 + translateY 8px→0 + scale 0.97→1, `overlayIn` `easeOut`; exit opacity + 4px down, `overlayOut` `easeIn`. Panel and scrim share one duration. |
| Banner | Slides 8px down from the header + fade, `toastIn`; dismiss `toastOut`. |
| List row hover | Background `raised`, `hover`. Keyboard selection moves instantly. |
| Filter tabs (start screen) | Underline slides to the clicked tab, `move` `easeInOut`; content swaps instantly. By key: underline jumps. |
| Section label change ("VERSE 1" → "CHORUS") | Replacement: old fades out while new fades in, stacked in one spot, 200ms — never sequential. |
| Timeline drag / loop handles | 1:1 with the pointer, no easing. On release past song bounds: rubber-band back with `spring`. Click-to-seek: playhead jumps instantly. |
| Mixer faders | Drag 1:1; click on track snaps to nearest 5% with `spring`. Keyboard steps instant. |
| LIVE pill dot | Ambient 2-step pulse (`steps2`) every 1s (not tied to the beat) while signal is present; static when NO SIGNAL or reduced motion. |
| LIVE ↔ NO SIGNAL | Replacement crossfade 200ms. No shake (it's a passive warning, not a failed action). |
| Input meter | Live, no easing. Peak-hold block holds 600ms, then falls 1 block per 50ms (stepped). |
| Getting-started card dismiss | Card collapses (height via transform) + fades, `move` `easeInOut`; list below closes the gap on the same clock. |
| Recent row forget (`del`) | Only if the existing forget is reversible in code: row collapses `move`, Undo toast for 5s, the forget commits when the toast closes. Otherwise: instant, no toast. |
| Error on an action (e.g. tap calibration "too uneven", invalid BPM entry) | Exaggeration: the message/field shakes translateX 0,−10,10,−6,6,−3,0 px over 360ms, once, on submit only. |

### A6. Find, hit, choose
- Pointer targets ≥ 40×40 (tempo arrows, track speaker icons, close ×, timeline loop handles' hit zone ≥ 24px wide even though the bracket is 2px). Make the hit zone bigger than the visual, never the visual.
- Destructive confirms: **Cancel / Keep playing takes default focus**; the destructive button names the verb ("Delete profile", "Abandon battle") and is never larger than Cancel.

### A7. Where the eye goes
- One filled `brand` element per view (start: Open song; practice: LIVE pill; overlays: their primary action). Other amber is text/lines only.
- One primary motion at a time. When an action sets off several, chain them, each starting as the previous lands.
- Ambient loops pause when their element isn't visible.

---

## PART B — Scoring and Battles choreography (implement inside SCORING.md and BATTLES.md stages)

All of Part A applies. Nothing here may run on, block, or add reads to the audio/render goroutine: animations read only the per-frame values the UI already receives. Verify scorer output is identical with animations on and off (§C).

### B1. Scoring HUD

**B1.1 Accuracy % and counters** — during play: instant updates, no roll (they change several times a second). In summaries (pass review, result cards): digits roll, only the digits that change, up when the value rises, `roll` `easeOut`, tabular figures.

**B1.2 Excitement bar (20 blocks)** — the HUD's signature element.
- **Rise:** when the value crosses into a new block, that block lights with a `steps2` pop (drawn 1px taller for 2 steps). This is per-note response feedback, so it stays ≤ 100ms and never eases.
- **Drain:** the top lit block goes dark stepped; no fade, no red, no shake — misses are never dramatised.
- **Band change** (Cold → Warm → Hot → On fire, a few times per song): lit blocks re-tint in a left-to-right sweep (`staggerCopy` 8ms per block, ≤ 160ms total) and the band label swaps by replacement crossfade 200ms. This is the bar's one "moment" — keep it the only motion at that instant.
- **On fire (75+):** ambient flicker on the top 3 lit blocks — sine-driven, each block phase-offset, 600ms period, alternating between two authored tints. Stops the instant the band drops. Off with reduced motion.
- **Streak bonus (+5 every 10):** a small pixel "+5" rises 8px in whole-pixel steps and fades, 300ms, above the bar. Secondary motion: starts with the block pop, ends within 150ms of it, moves ≤ ⅓ of anything else.
- Frozen while paused/waiting/count-in (no drain, no flicker).

**B1.3 Ghosts** — appear instantly when the note is judged (per-note frequency). Fade out `steps3` across the 2 bars after the playhead passes. Reduced motion: disappear at the 2-bar mark with no fade.

**B1.4 XP bar** — grows from its last value with a left-anchored fill, `move` `easeInOut`; never animates backwards except the level-up reset (B1.6), which happens behind the level card.

**B1.5 XP toast (after a loop pass — happens often, so it's quiet)** — queued until the pass ends. Fades in + rises 8px, `toastIn`; "+42 XP" rolls up from 0 over `roll`; holds 1.2s; exits fade + 4px, `toastOut`. Never during a turn or while the transport is playing a new pass's first bar — if the next pass has already started, show it at the next transport stop instead.

**B1.6 Level up (rare — choreographed, ≤ `momentMax`; Space/click jumps to the end state; never shown while playing)**

```
TIMING (ms from start)
   0  XP bar fills to full ............................ move, easeInOut
 250  bar flashes brand, steps2; resets (behind scrim)
 300  scrim 40% in ................................... 200, easeOut
 350  level card in: fade + translateY 16→0 + 0.97→1 . 350, easeOut
 600  "LEVEL 8" pixel text pops ........................ steps3
 700  pixel spark burst: 8 sprites, whole-pixel travel ≤ 12px, steps4 (secondary: ends ≤ 150ms after the pop)
 900  title line fades in (only if the title changed: "Garage Player"), 200; if unchanged, show "Level 8" alone
2300  card + scrim exit ................................ overlayOut, easeIn
```
One `TIMING` object + one config per element in code, as above.

**B1.7 Pass review overlay**
- Opens with the overlay motion (A5). Header stats arrive as pieces, `staggerPiece` 90ms (accuracy → tightness → XP → excitement peak).
- **Timing strip:** when auto-opened at the end of a full run (rare), dots draw in a left-to-right sweep over 600ms `linear` — it replays the run's time, so the motion means something. Opened manually (P): all dots appear at once.
- Hover a dot: tooltip (A5) + dot grows by 1px ring (vector). Dots have ≥ 24px hit zones.
- Click a dot: the tab scrolls to that note, `travel` `easeInOut`, and the note's outline blinks `steps2` twice.

**B1.8 Tap calibration**
- 16 tap blocks; each block lights instantly on its tap. A visual click marker pulses on each click (`steps2`) so the user can follow it with eyes as well as ears.
- Result: the offset number rolls in, `roll`. Failure ("too uneven"): shake (A5 error rule) + retry button gets focus.

### B2. Battles

**B2.1 Setup** — adding a participant slot expands it, `move` `easeInOut`; removing collapses it. Choosing a rival swaps the portrait by replacement (old fades while new fades in, 200ms, stacked). Locked rivals: silhouettes, no motion.

**B2.2 Battle intro — VS screen (rare, ≤ 2.5s, Space skips)**
```
   0  scrim in, 200
 100  participant cards slide in from their side: translateX 48→0, travel, easeOut, staggerPiece 90 between cards
 450  "VS" pixel text: steps3 pop
 650  material line ("Sultans of Swing · Verse 1 · 4×4 bars") fades in, 200
1800  everything exits, overlayOut → battle screen (no entrance on its own static parts)
```
With 3–4 participants the cards enter from left and right alternately.

**B2.3 Turn banner** — "EXCHANGE 2 / 4 — AADITYA" fades in + translateY −12→0, `toastIn`; "Pass the guitar" sub-line follows 90ms later (only if the previous turn was a different human). On Ready: banner exits `toastOut`, then the count-in starts (chained, not overlapped).

**B2.4 Scoreboard**
- Active participant: a `brand` underline under their name — the battle screen's main amber accent (the LIVE pill switches to its outline style in battles). It slides to the next participant, `move` `easeInOut`, as the turn changes.
- Exchange tracker dots fill `steps2` when a turn completes.
- Bot portrait idle: 2-frame head-bob, sine-timed to the song's tempo using the same position value the tab already reads; paused when not playing; off with reduced motion.

**B2.5 Result card after each turn (≤ 1.4s, Space jumps to end; auto-advance 3s after the last step; input never blocked)**
```
   0  card in: fade + 12px rise ......................... toastIn, easeOut
 240  accuracy rolls ...................................... roll
 540  turn score rolls (big) ............................. 400, easeOut
 940  (only when this turn completes an exchange) crowd marker moves toward the exchange leader ... travel, easeInOut
      crowd silhouettes on that side bob steps2 for 300ms (secondary)
1290  scoreboard total for this participant rolls ......... roll
```

**B2.6 Lead change (a few times per battle)** — the pixel crown hops from the old leader's column to the new one along an arc (bulge 15% of the distance), `travel`; on landing plays authored frames squash → stretch → normal (`steps3`). If the leader is unchanged, no crown motion.

**B2.7 Crowd meter**
- 2 players: the tug-of-war marker only moves at exchange ends (B2.5), never mid-turn.
- 3–4 players: split bar segments resize together, `move` `easeInOut`, at exchange ends.

**B2.8 Final results (rare — peak-end; ≤ 2.5s; Space skips to end)**
```
   0  scrim + "RESULTS" header ............................ overlayIn
 260  per-exchange rows: fade + 8px, staggerCopy 40 (first 6 rows; the rest arrive with the 6th)
 600  totals roll ........................................... 500, easeOut
1100  winner: pixel trophy drops straight down onto the winner's column (single axis, no arc), travel, easeOut;
      landing frames squash → normal (steps3); sparkle burst secondary (steps4, ends within 150ms of landing)
      losers' columns recede to 50% opacity, move
1500  XP lines roll per human profile, staggerPiece 90 between them
1900  buttons fade in, 200 — Rematch is the view's one filled brand button
```
Draw: no trophy; both/all tied names get a "DRAW" pill (`steps3` pop) instead.

**B2.9 Confirms** — Abandon battle and Delete profile follow A6: default focus on Keep playing / Cancel; destructive verb named; not larger than Cancel.

---

## C. Verify (run every check, report results)

- [ ] **Scrub** with `HEADLINER_SLOWMO=0.1`: every animation's first frame responds, last frame settles with no pop, and only one thing leads at a time.
- [ ] **Interrupt** each transition halfway (hover off, close an overlay while it opens, press Space mid-choreography): it turns around or jumps to the end state; input is never blocked.
- [ ] **Hammer the keyboard** through the practice screen and battle setup: nothing lags behind the keys; no key toggle animates.
- [ ] **Esc stack:** with each overlay and dialog open, Esc closes only the top layer; the song is never left accidentally.
- [ ] **Reduced motion on:** every transition, roll, sweep, loop and choreography is skipped; all information identical; playhead still moves.
- [ ] **Pixel crispness:** screenshot mid-animation (`HEADLINER_SHOT`) — no blurred or fractional-position bitmaps.
- [ ] **Audio isolation:** run the same scripted session (or the existing practice/scorer tests) with animations on and off — identical verdicts and timing offsets; no new engine reads in draw code.
- [ ] **No performance distraction:** XP toasts and level-ups never appear while a pass is playing.
- [ ] **Tenth use:** watch the XP toast, result card and turn banner at full speed ten times in a row; shorten anything that starts to feel slow, and note what you changed.
