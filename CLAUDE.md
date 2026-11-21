# CLAUDE.md

Private fork of `S95F/instrumentTutor`, being turned into a guitar career game.
The full build plan lives at `D:\AARSH\Projects\GuitarCareer\musicTutor → Guitar Career Game Build Plan.md`.

## Owner

The owner is a guitarist, not a programmer. Test steps for them must be written as
"click this, play that, you should see X". No jargon.

## Build and test (Windows, run from Git Bash)

```
go test ./...
CGO_ENABLED=1 go build ./cmd/musictutor
```

- cgo build needs gcc from MSYS2 (`/c/msys64/ucrt64/bin`), already on PATH.
- Before every rebuild: `cp musictutor.exe musictutor-good.exe` (known-good rollback).
- Never break existing tests. Run the full suite before calling a change done.

## Git

- Work on branch `career-mode`. `origin` = private repo (nycc7yfw75-bot/instrumentTutor),
  `upstream` = the original developer's repo. Never push to `upstream`, never open PRs upstream.
- Commit only after the owner has tested on the guitar and said it works.

## Game ground rules (apply to every feature)

1. **A miss never subtracts.** Hits add, misses add zero. No booing, no lost points, no badge taken away.
2. **The streak counts practice time, not accuracy.** 5+ minutes of playing counts as a day, even with detection off.
3. **Scores only count from trustworthy sessions:** listening on, timing offset set, no dropped audio.
   Otherwise show "unscored" and still log practice time.
4. **Chords, bends and dead notes never produce misses.**
5. **Slow practice earns more:** up to 1.5x score at half speed.
6. **Everything can be switched off,** and nothing in the game layer may crash a practice session.

Not doing: XP for minutes played, leaderboards, subscriptions, 3D graphics, bundled songs.

## Design docs to follow

- `docs/PROGRESS.md` — scoring, combo, streaks, badges (Phase 2). Build as written.
- `docs/HIGHWAY.md` — note highway (Phase 4). Build as written; toggle key is **Y** (V = mixer).

## Owner's setup

- Audio: Marshall CODE 25 over USB, used as both input and output. It cannot do loopback calibration.
- Save + pieces folder (ONLINE.md §A2–A3): dev builds (`go build`) use `C:\Users\Dell\AppData\Roaming\headliner-dev` (pieces in `pieces\`); installed releases use `...\Roaming\headliner`. Both started as copies of the old `...\Roaming\musictutor`, which is kept untouched.
- Release signing key: `C:\Users\Dell\.headliner-signing\release.key` (never commit; it is the repo secret HEADLINER_SIGNING_KEY).
- Main song: `Dire Straits-Sultans Of Swing-10-29-2026.gp` (fingerstyle/legato — expect false misses).
- Reference test song (simple picked notes): `First steps on guitar.gtab`.
- Career profile stage/band name: editable in the app (no default needed).
- Setlists for live shows: chosen by the player, not suggested.
