# UI inventory (REDESIGN §2)

Every screen, overlay, prompt and message the app draws, with where it is
drawn, its controls and its keys. This is the Stage 6 checklist: after the
redesign, every control below must still exist and do what it did.

Key tables are not copied here: each screen's full key list is generated
from the binding table named in its row, and the help overlay (`?`/F1)
shows it. Stage 6 walks those tables key by key.

All screens sit in the `Shell` stack (`shell.go: Shell.Draw`), which draws
the top screen only. Esc on any screen pops it (the start screen quits).

## Screens

| Screen | Drawn by | Controls | Keys |
|---|---|---|---|
| Start screen (piece browser) | `browserdraw.go: Browser.Draw`, `drawPanes`, `drawPane`, `drawPaneNote`, `drawActions`, `drawStatus` | Action buttons: Open a file… (primary), Write a new piece, Edit selected, Settings, Help; three panes (Recent, Created here, Library) with selectable rows; Quit chip (top left) | `browser.go: browserBindings` (O, N, E, S, H, ?, arrows, Enter, Tab between panes, Esc/Q quit) |
| Getting-started strip | `hint.go: Browser.drawHint` | Three steps (interface, calibration, open a piece) with ticks; show/hide (H) button | H |
| Practice view | `ui.go: App.Draw`, `drawHUD`, `drawTab`; `transport.go: drawTransport`, `drawTrackStrip`, `drawTimeline`, `drawStateChip`; `menu.go: drawTempo`, `drawMenuItem`; `sections.go: drawSections`; `windlane.go: drawWindGrid` (wind tracks) | Back chip; text menu (play, loop, click, ramp, count-in, wait, tempo ‹ BPM ›, tuner, mixer, settings, help); track chips (click mute, right-click solo); notation (click/drag seek, shift-drag loop, wheel zoom, loop edges drag); timeline (click/drag seek, sections) | `ui.go: practiceBindings` (Space, arrows, Home/End, [ ], A/B/L, shift+B, R, C, M, T, V, W, I, D, P, S, F5, Tab, 1–9, shift+1–9, +/−, ?, Esc, Q) |
| Settings | `settings.go: Settings.Draw`, `drawItemRow` | Profile: player + rename; capture ‹ ›; playback ‹ ›; round-trip offset: calibrate now, ‹ › fine-tune; tap calibrate: tap now; instrument sounds: browse, clear; count-in − +; audio/visual sync − + (when measurable); Back chip | `settings.go: settingsBindings` (up/down, left/right, Enter, ?, Esc) |
| Name prompt | `nameprompt.go: NamePrompt.Draw` | Text field, Save button, Back chip | typing, Backspace, Enter save, Esc later |
| Editor — grid | `editor.go: Editor.Draw`, `drawGrid`, `drawBar`, `drawRhythm`, `drawFirstSteps`; `editortoolbar.go: drawChrome`, `drawValueButton` | Toolbar: dot, triplet, tie, rest, add beat, delete beat, add bar, undo, redo, add track, instrument, tuning, capo, meter, tempo, title, view (grid/text), save, practice, help; grid cells | `editor.go: editorBindings` |
| Editor — text view (.gtab) | `editortext.go: drawTextPane`, `drawGtabLegend` | Text area with cursor and selection; same toolbar | `editortext.go: textBindings` |
| Editor — wind ladder | `editorwind.go: drawPicker`, `drawWindSystemLines` | Pitch picker; same toolbar | `editorwind.go: windBindings` |
| Glyph sheet (dev) | `glyphsheet.go: GlyphSheet.Draw` | none (reference sheet) | Esc |

## Overlays, prompts and banners

| Item | Drawn by | Controls | Keys |
|---|---|---|---|
| Help overlay (every screen) | `help.go: drawHelpOverlay` | Close on click | ?/F1 toggles, Esc closes |
| Tooltip | `tooltip.go: tips.draw` | — (hover) | — |
| Tuner | `live.go: App.drawTuner` | — | T |
| Mixer | `mixer.go: drawMixer`, `paintMixer` | Sliders: master, metronome, backing track, each track; drag or click | V, up/down select, left/right adjust, Esc |
| Import-warnings list | `warnlist.go: drawWarnList`, `paintWarnList` | Scrollable list, close | I, Esc |
| Warning banner (practice) | `live.go: App.drawWarning` | Dismiss | D |
| Live HUD stats line | `live.go: App.drawLiveHUD` | — | — |
| Hit/Close/Miss legend | `live.go: App.drawLegend` (when no game layer) | — | — |
| Scoring HUD row | `game.go: drawGameHUD`; `gamehud.go: exciteView.draw`, `xpBarView.draw`, `xpToast.draw`, `levelCard.draw` | Click the "Unscored"/"Add your name" message | Space/click skips level card |
| Timing ghosts | `ghosts.go: drawGhosts`, `drawGhost` | — | — |
| Pass review | `review.go: drawReview`, `paintReview`, `paintTimingStrip`, `paintDotTip`, `paintReviewTab` | Dots (hover, click), Loop this again, Close | P, ←/→, Esc |
| State chip (COUNT-IN n / WAITING) | `transport.go: drawStateChip` | — | — |
| BPM entry | `ui.go: drawBPMEntry` | Digits field | shift+B opens; digits, Enter, Esc |
| Editor entry prompt (save name, title, tempo, meter…) | `editorchrome.go: drawEntry` | Text field | Enter apply, Esc cancel |
| Editor unsaved-changes prompt | `editor.go: drawLeavePrompt` | Save / Discard / Cancel | keys per `editorBindings` |
| Level-up card | `gamehud.go: levelCard.draw` | — | Space/click |

## Messages, empty and error states

| Item | Where |
|---|---|
| Browser error line ("cannot open … for editing") | `browserdraw.go: drawStatus` (`b.errMsg`) |
| Browser empty panes | `browserdraw.go: drawPaneNote` |
| Settings notices, calibration failure advice, save errors | `settings.go: items` (`reserveNote`), `saveErrLine`, `calFailureAdvice` |
| Audio unavailable text | `settings.go: audioUnavailableText` |
| Practice status line (Paused/Playing · Pass n · Live) | `ui.go: statusLine` |
| BPM / reload messages | `ui.go: drawHUD` (`bpmMessage`, `reloadPrompt`) |
| Input hint (no input chosen) | `inputhint.go` |
| Footer shortcut bar | `theme.go: drawFooter` (every screen) |
| Native file dialogs and their fallbacks | `cmd/musictutor/dialogs.go` (OS dialogs, not drawn by the app) |
