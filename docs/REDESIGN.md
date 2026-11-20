# HEADLINER — Visual Redesign Brief (cosmetics only)

Save this file as `docs/REDESIGN.md` in the repo. Re-read it at the start of every stage and whenever your context is compacted. It is the single source of truth for this redesign.

**Motion is specified separately in `docs/MOTION.md`.** Implement its Part A (tokens, global rules, chrome motion, layers/Esc, reduced motion, hit areas) as part of this redesign: tokens and the tween/spring engine in Stage 1, chrome motion with the components in Stage 2, and screen-specific motion in Stages 3–5. Where this file and MOTION.md disagree on motion, MOTION.md wins.

---

## 0. Scope — read this twice

This pass changes **how the app looks**, not **what it does**.

IN scope:
- Colours, fonts, sizes, spacing, layout, icons, borders, corners, motion.
- Rearranging existing controls and existing information on screen.
- Removing duplicated or redundant on-screen text.
- Renaming the app in user-visible text to HEADLINER.
- Displaying data the app already has, more clearly (e.g. hit/close/miss counts it already computes; title/artist/tempo if the importers already parse them).

OUT of scope — do NOT build any of these, not even as placeholders:
- Career mode, venues, levels, XP, fame, achievements, badges, streaks, mastery stars, setlists, "resume from last bar", song artwork.
- Any change to scoring, detection, timing windows, playback, the audio engine, the tablature data model, file formats, or importers.
- New persisted data or new config fields — with one exception: the `Reduce motion` display setting defined in `docs/MOTION.md` §A4. Anything else: stop and ask.
- Renaming the Go module, the binary, the config directory, or env vars. User-visible strings only.
- New keyboard shortcuts that change behaviour. Every existing shortcut and mouse action must keep doing exactly what it does now.

If a design idea below needs something out of scope, skip that part and note it in your report. Never fake it.

---

## 1. Stack facts (don't assume web tech)

- Go + Ebitengine, immediate-mode drawing. No HTML, CSS, React, Electron, or icon fonts. No Lucide.
- UI code lives in `internal/ui/` (`ui.go`, `transport.go`, `browser.go`, `browserdraw.go`, `settings.go`, `editor*.go`, `live.go`, `theme.go`, `font.go`, `anim.go`, `tooltip.go`, `help.go`, `hint.go`, `glyph*.go`, `windlane.go`, `playhead.go`, `pointer.go`). Find any others with `grep`.
- Build: `CGO_ENABLED=1 go build ./cmd/musictutor` (Windows, Git Bash). Tests: `go test ./...`.
- Drawing must never block or slow the audio/render goroutine. Decorative animation runs off the UI frame clock only; it must never read or wait on engine locks more than today's code does.
- Add no third-party dependencies except font files (see §3). Embed fonts and pixel assets with `go:embed`.

---

## 2. Process — stages, commits, checkpoints

Work on a new branch: `git checkout -b ui-redesign`.

Before Stage 1, write `docs/UI_INVENTORY.md`: every screen, overlay, dialog, banner, toast, prompt and wizard in the app (start screen, practice view, editor grid, editor text view, wind editor, settings, calibration, tuner, mixer, help overlay, save-name prompt, BPM entry, warning banner, quit/unsaved prompt, file-dialog fallbacks, empty states, error lines). For each: file + function that draws it, every control on it, its shortcut. This inventory is your checklist for Stage 6.

**Screenshot tool (build this first, it's the only non-visual code you add):** a dev-only env var `HEADLINER_SHOT=<dir>` that, when set, saves the current frame as PNG to `<dir>/<screen>-<width>x<height>.png` each time F12 is pressed. Use `ebiten.Image.ReadPixels` in `Draw`. Off by default, no effect when unset. Use it to self-review every stage at 1280×720, 1366×768, 1600×900 and 1920×1080.

Stages (one commit per stage, message `ui: stage N — <name>`):

1. **Tokens and fonts** — §3 and §4 as code; replace every hard-coded colour, size and font in `internal/ui` with tokens. Zero visual-layout changes yet beyond colours and fonts.
2. **Pixel kit** — §5 components and the icon set, plus a hidden dev screen (opened only via `HEADLINER_KIT=1`) that shows every component and icon in all states.
   → **CHECKPOINT A: stop. Report, attach screenshot paths of the kit screen and the start + practice screens at 1366×768. Wait for my go-ahead.**
3. **Practice screen** — §6.
   → **CHECKPOINT B: stop. Same report + screenshots at all four sizes. Wait.**
4. **Start screen** — §7.
5. **Everything else** — §8 (editor, settings, tuner, mixer, help, dialogs, banners, prompts, empty/error states).
6. **Validation** — §9. Fix every failure, then final report.

After each stage: `go test ./...` and the build must pass. Update a test only if it asserts an old pixel position, size, colour or label text; never weaken a test that asserts behaviour. List every test you changed and why.

---

## 3. Design tokens

Put all of these in one place (e.g. `internal/ui/tokens.go`). Nothing in `internal/ui` may use a literal colour, font size or spacing value after Stage 1.

### Colour

| Token | Hex | Used for — and nothing else |
|---|---|---|
| `bg` | `#111116` | Window background |
| `surface` | `#191920` | Overlays, cards, tooltips, timeline track |
| `raised` | `#25252F` | Hovered rows, pressed controls, input fields |
| `line` | `#393843` | Separators, tab strings, borders |
| `text` | `#F4F0E8` | Primary text, fret numbers, playhead |
| `text2` | `#A5A3AF` | Secondary text, inactive menu items |
| `text3` | `#6F6D7A` | Disabled, other tracks, bar numbers |
| `brand` | `#F3A33B` | HEADLINER amber: wordmark, primary button, active menu item + underline, LIVE pill, current section on the timeline, focus ring |
| `hit` | `#55D69A` | Hit verdicts, meter low segments |
| `close` | `#E6C25E` | Close verdicts, meter mid segments |
| `miss` | `#E56A6A` | Miss verdicts, meter clip segment, destructive actions |

Hard rules:
- **Brand amber never appears inside the tab area.** The tab area uses only `text`, `text2`, `text3`, `line` and the three verdict colours. This keeps "close" (yellow) and "brand" (amber) from ever being confused, and keeps the playhead from looking like a verdict.
- No colour carries meaning alone. Verdicts also differ by shape (§6.5). Active menu items also have an underline.
- No gradients, no glows except the playhead's soft halo, no neon, no pure black, no pure white.

### Type

Exactly three families, embedded TTFs, licence files committed next to them:

| Role | Family | Sizes (px) |
|---|---|---|
| Display — wordmark, screen titles, song title, pills, section labels, HUD numbers, BPM | **Pixel Operator** (CC0). Fallback if unavailable: **Silkscreen** (OFL) | 16, 24, 32, 48 only. Pixel fonts blur at other sizes. Draw at integer pixel positions. |
| UI — menu, lists, labels, tooltips, settings, body | **Inter** (OFL) | 13, 14, 16, 20 |
| Numeric — fret numbers, BPM digits in fields, timers, bar numbers, counters | **JetBrains Mono** (OFL), Bold for fret numbers | 13, 14, 16, 20–28 (tab scales) |

Casing: Title Case for screen titles and menu labels; pixel-font UPPERCASE only for pills and section labels (LIVE, PAUSED, VERSE 1, TRACKS). Song titles in proper title case ("Sultans of Swing" — lowercase short words).

### Spacing and shape

- 4px base unit; use 4, 8, 12, 16, 24, 32, 48.
- Outer window margin: 32. Section gaps: 24. Control height: 36 (dense) / 44 (primary).
- **Corners: no rounded corners.** Buttons, pills, cards, tooltips and the timeline use a **2px stepped notch** at each corner (the corner pixel removed, drawn as a pixel staircase). This is the single most important retro detail — apply it consistently.
- Borders 1px `line`, used sparingly. Prefer spacing and `surface` fills over outlines. Most things need no container at all.

### Motion

All durations, curves, pixel-step rules and per-element motion: `docs/MOTION.md` (tokens §1, Part A). Two rules repeated here because they shape everything:
- Anything triggered by a key or per note gets no transition; transitions live on the pointer path only.
- Pixel bitmaps move and scale in whole pixels only; vector surfaces may ease.
- Nothing animates in the tab area except the playhead (linear) and verdicts (instant).

---

## 4. Branding

- User-visible app name: **HEADLINER** everywhere (window title, start screen, help, dialogs, error text, usage text shown in-app). Do not rename module/binary/config/env vars.
- Window title format: `HEADLINER — <Song Title>` on the practice screen, `HEADLINER` elsewhere. Never show a file name with extension or date suffix as a title.
- Wordmark: "HEADLINER" in the display font at 32px, 2px letter spacing, `brand` colour, with a small 16×16 pixel spotlight mark to its left (cone of light, 3 shades of amber). Drawn from a hand-authored bitmap, not text effects.
- Tagline under the wordmark on the start screen only: "Open mic to arena" in UI 13px `text2`.

---

## 5. Pixel kit (Stage 2)

### Icons

One icon family only: hand-authored **16×16 pixel bitmaps**, drawn at 2× (32px) with nearest-neighbour filtering, 1-colour (tinted at draw time) unless stated. Author them as PNGs in `internal/ui/assets/icons/` or as Go byte grids — whichever is easier to review. No smoothing, no sub-pixel positions.

Required set: back, play, pause, skip-start, prev-bar, next-bar, loop, metronome, ramp (stairs), count-in (1-2-3), wait (hourglass), tuner (fork), mixer (faders), settings (cog), help (?), speaker-on, speaker-muted, solo (S), guitar, bass, drums, mic (vocals), keys, horn (wind), open (folder), new (page+), edit (pencil), spotlight (amber, multicolour), warning (!).

Transport, menu, track and start-screen icons all come from this set. Delete or stop using any previous glyph-font icons for these.

### Components

- **Button** — notched corners; primary = `brand` fill + `bg` text in display font (one filled primary per view, MOTION §A7); secondary = no fill, `text2` → `text` on hover; destructive = `miss` text. Height 36/44, hit zone ≥ 40×40. Press = 1px down shift (MOTION §A5). Tooltip on hover shows label + shortcut keycap.
- **Menu item** — icon + label (UI 14), no box. Inactive `text2`; hover `text` + 2px underline growing from centre (MOTION §A5); active `brand` text + full `brand` underline — instant when toggled by key.
- **Pill** — display font 16 uppercase, notched, 24px tall. Variants: filled `brand` (LIVE), outline `line` (PAUSED/PLAYING), outline `close` (NO SIGNAL).
- **Keycap** — notched 1px `line` box, display font 16, `text2`; used in tooltips and the help overlay.
- **Segmented meter** — N blocks of 8×12px, 2px gaps; fills `hit` → `close` → `miss` for the top block. Used for input level.
- **Tooltip** — `surface` fill, notched, UI 13 label + keycaps, 8px offset; timing and warm state per MOTION §A5.
- **List row** — 48px tall, no box; hover `raised` fill; selected = 3px `brand` bar on the left edge + `text` colour.
- **Section header** — display font 16 uppercase `text3`, 2px letter spacing, optional count right-aligned in `text3`.
- **Banner** — full-width strip under the header, `surface` fill, warning icon in `close`, message in UI 14, dismiss "×" right. Multi-warning banners show "N more" as a link that opens a list overlay (the list already exists as data — just present it).

---

## 6. Practice screen (Stage 3)

Layout top to bottom, filling the window (no fixed canvas, no side gutters, 32px outer margin):

| Zone | Height | Content |
|---|---|---|
| Header | 64 | Back (icon + "Back", `text2`) · song title (display 32) with artist (UI 14 `text2`) beneath if known · right: pills (LIVE/NO SIGNAL, PAUSED/PLAYING) |
| Toolbar | 48 | Groups separated by 24px + a 1px `line` divider: **Transport** (icons only) · **Practice** (Loop, Click, Ramp, Count-in, Wait) · **Tempo** control · right-aligned **Tools** (Tuner, Mixer, Settings, Help) |
| Track bar | 44 | Section header "TRACKS" left, then track tags (§6.3); input meter + HUD right |
| Tab | everything left | §6.4 |
| Timeline | 72 | §6.6 |

### 6.1 Header
- Song title: from the piece's title metadata; fall back to the file name with extension, date suffixes like `-10-29-2026` and "(1)"-style duplicates stripped. Proper title case.
- Artist: show only if the importer already provides it. If not, show the practised track's name instead (e.g. "Lead Guitar").

### 6.2 Toolbar
- Practice items: icon + label; Tools: icon + label on ≥1600px wide, icon-only below with tooltips.
- Tempo control: `‹` icon · BPM in display font 24 · `· 60%` in UI 14 `text2` · `›` icon. Each arrow a 40×40 hit area. Clicking the number opens the existing exact-BPM entry. Remove tempo from anywhere else on screen.
- No key letters printed under or beside labels; shortcuts live in tooltips only.

### 6.3 Track bar
- Your track first: instrument icon tinted `text`, name in UI 14 `text`, "YOU" pill (outline `brand`). Then the others as compact tags: instrument icon + short name in UI 13 `text3` + speaker-on/speaker-muted icon. Solo shown as an `S` icon in `text`.
- Short names: derive instrument from the existing GM program / track type ("Vocals", "Guitar", "Bass", "Drums", "Keys"); if two collide, append the original name trimmed to fit ("Guitar · Rhythm"). Never show a word cut with "…" if a shorter derived name exists; full name in the tooltip.
- Never print "Muted by solo". Muted state = speaker-muted icon + 40% opacity.
- Clicking, right-clicking, keys 1–9, shift+1–9, Tab keep their current behaviour exactly.

### 6.4 Tablature
- String spacing scales with available height: `clamp(36px, (tabHeight − 48) / 6, 64px)`. The staff is vertically centred in the tab zone.
- String lines span the content area only (32px margins), not the window edge. Low E line 2px, others 1px, all `line`.
- String names (e B G D A E, or the track's actual tuning) at the left in display font 16 `text3`.
- Bar lines 1px `text3`; bar numbers in a thin ruler above the staff, JetBrains Mono 13 `text3`, left-aligned at each bar line.
- Fret numbers: JetBrains Mono Bold, size = 0.45 × string spacing (min 16, max 28), `text`. Each number sits on a `bg`-coloured knockout rectangle so the string line doesn't run through it.
- **Past notes** (before the playhead and not yet judged): `text2`. **Upcoming**: `text`. Notes inside the next beat: `text` with a 1px `text2` notched outline.
- Ties: replace "(5)" brackets with a small arc from the previous note (2px, `text2`). Hammer/pull/slide/bend/vibrato/dead keep their existing meaning; draw them at 2px `text2`, ending 6px short of the numbers. Keep musical meaning identical — rendering only.
- Playhead: 2px `text` vertical line + 6px soft halo at 15% opacity + a 6×4px pixel triangle at the top of the staff. Never red, never amber.

### 6.5 Verdict feedback
- Hit: notched filled square behind the number, `hit` at 25% opacity, number in `hit`.
- Close: notched **outlined** square (2px `close`), number in `close`.
- Miss: number in `miss` with a 2px pixel "×" in the top-right corner of its knockout.
- Shapes differ, so verdicts don't rely on colour.

### 6.6 Timeline
- One horizontal track, 24px tall, `surface` fill, notched. Each song section is a solid segment; section types get tints of `raised`/`line` (verse, chorus, solo, intro/outro distinguishable by lightness), the **current** section is `brand` at 35% with its label in `brand`.
- Labels: display font 16 uppercase, shown only when the full label fits; otherwise nothing (no "Ch…", no "…"). Hover shows a tooltip with section name, bar range and duration.
- Below it, one line: `VERSE 1` (display 16, `brand`) · `Bar 14 / 212` · `0:40 / 9:36` (JetBrains Mono 14, `text2`).
- No tick marks. Remove the "Click or drag to move · Shift-drag to loop a section" text (put it in the tooltip and the help overlay).
- Loop region: `text` 2px brackets at its ends (hit zone ≥ 24px wide), region tinted `text` at 8%.
- All existing click/drag/shift-drag behaviour unchanged.

### 6.7 HUD and input
- Right side of the track bar: segmented input meter (12 blocks) labelled "IN" (display 16 `text3`), then counters `12 · 3 · 1` in JetBrains Mono 16 coloured `hit`/`close`/`miss`, then accuracy `85%` in display 24 `text`.
- Accuracy and counters only show values the scorer already produces. When not listening, the whole HUD is replaced by "Not listening" in `text3`.
- If the app already tracks input level and the meter has been flat while playing for ≥3s in live mode, swap the LIVE pill for NO SIGNAL (outline `close`). Use only data already available in `live.go`.
- Delete the floating "Hit Close Miss" legend.

### 6.8 Footer
- Delete the permanent shortcut bar. The existing `?`/F1 help overlay is the full reference (§8). Tooltips carry per-control shortcuts.

---

## 7. Start screen (Stage 4)

Fix what's broken; don't invent a career hub (that comes later).

- Header: wordmark + spotlight left, tagline under it. Right: Settings and Help as menu items. **Remove "Quit" from the header** (Esc/Q and the window close button still quit — behaviour unchanged).
- Remove "6 pieces to hand" and the "getting started" label + "show (H)" button. Present the existing getting-started checklist as a single notched `surface` card with three steps and a dismiss "×" (H still toggles it — same state, new look).
- Actions: one primary button "Open song" (O). "Write new" (N) and "Edit" (E) as secondary buttons beside it. No key letters under labels.
- Replace the three equal columns with **one list** titled "SONGS" and filter tabs **All · Recent · Written here · Library** (same underlying lists as today — this is a presentation change; "All" is the union with duplicates removed). Default tab: Recent if non-empty, else Library.
- De-duplicate rows that point at the same file path. (Only by path — do not add hashing.)
- Row: instrument icon · title (UI 16 `text`, from metadata, cleaned as in §6.1) · secondary line (UI 13 `text2`): whatever metadata the library already computes ("4/4 · 90 BPM · 4 bars · drop D") — use the same format for every tab. **No Windows paths on rows**; show the folder path once in a tooltip on hover.
- Empty tab: one centred line in `text3` + the relevant button.
- No panel boxes, no glowing focus border. Focus = selected row style (§5).
- Find and remove the bright-green horizontal line that crosses the screen (~y=530 at 1842×1027). If it isn't in our drawing code, note where it comes from.
- Remove the permanent footer shortcut bar here too.

---

## 8. Everything else (Stage 5)

Apply tokens, fonts, pixel kit and the rules above to every item in `docs/UI_INVENTORY.md`. Specifics:

- **Settings**: group existing rows under section headers — AUDIO (capture, playback, calibration, sync trim), PLAYBACK (count-in, SoundFont). Only existing settings. Row = label left (UI 14), control right; adjusters as `‹ value ›` with pixel arrows. Calibrate = secondary button.
- **Tuner overlay**: note name huge in display font 48 centred, cents offset as a horizontal segmented meter (21 blocks, centre block `hit` when in tune ±5 cents, sides `close`/`miss` by distance), target string name below. Same data as today.
- **Mixer overlay**: one column per existing channel/track: pixel instrument icon, vertical segmented fader, value in JetBrains Mono, mute/solo icons. Same controls as today.
- **Help overlay**: `surface` panel, notched; groups PLAYBACK · NAVIGATION · TRACKS · LOOP · TEMPO · TOOLS · APP; each row = keycaps + description (UI 14). Generated from the existing help bindings tables — don't hand-copy them.
- **Editor (grid, text view, wind ladder)**: tokens, fonts, notched toolbar buttons, pixel icons where an icon already exists; keep every tool, key and layout behaviour. Notation glyphs (noteheads, ties, flags) stay vector — they're notation, not UI.
- **Prompts and dialogs** (save-name, BPM entry, unsaved-changes, errors): centred notched `surface` card, title in display 24, body UI 14, buttons per §5, Esc/Enter behaviour unchanged.
- **Banners and status lines**: §5 banner. Error text in plain language as it exists today — restyle, don't rewrite meaning.

---

## 9. Validation (Stage 6)

Do all of this, then report results honestly. Do not claim anything works that you didn't check.

1. `go test ./...` passes. `CGO_ENABLED=1 go build ./cmd/musictutor` succeeds.
2. Screenshots via `HEADLINER_SHOT` of every inventory screen at 1280×720, 1366×768, 1600×900, 1920×1080. Check each for: overlaps, clipped text, any "…" truncation where a shorter label exists, side gutters, literal colours outside tokens, any non-pixel icon, any rounded corner.
3. Grep check: no hex colour literals or `color.RGBA{` literals in `internal/ui` outside the tokens file; no font-size literals outside tokens.
4. Walk `docs/UI_INVENTORY.md`: every control still exists and does what it did. Every shortcut listed in the existing help tables still works.
5. Run the MOTION.md §C checks that apply to Part A.
6. Self-critique against this list and fix before reporting:
   - Is anything still boxed that doesn't need a box?
   - Is any information shown twice on one screen?
   - Does any screen still look like a developer utility?
   - Is the tab the most prominent thing on the practice screen at every size?
   - Is amber used anywhere inside the tab area? (must be no)
   - Is any icon not from the pixel set?
7. Final report: what changed per screen, tests changed and why, anything skipped because it was out of scope or impossible, screenshot paths.

Commit, push the branch, and stop.
