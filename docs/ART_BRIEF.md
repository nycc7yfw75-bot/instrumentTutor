# HEADLINER — Art brief for the performance scene

For an artist, Claude Design, or a later Claude Code session. Everything here is self-contained: make the files below, drop them into the folders named, edit one line in each `manifest.json`, and the game uses them. No code changes are needed (CAREER.md §7).

## 1. The scene

A **side view of a small stage**, like a pixel-art game screenshot. Your guitarist stands in the left third of the stage, facing right; the crowd fills the right side, facing left, back rows behind and a front row in front. A city skyline shows through windows (small venues) or behind the stage (stadium).

- **Base resolution: 480 × 200 pixels.** The game scales it up by whole numbers (×2, ×3, ×4) with nearest-neighbour filtering. Never draw at any other size.
- **1 px = 1 art pixel.** No anti-aliasing, no soft brushes, no gradients except hard-edged stepped bands, no sub-pixel detail.
- **At most 32 colours per scene** (all layers together).
- Transparent PNG (RGBA) for every layer except the sky.
- Layers, back to front: sky → city skyline → venue interior → lights → crowd back rows → stage + amp/drum props → guitarist → crowd front row → effects.

## 2. Palette

Base tokens (from docs/REDESIGN.md), used for UI-adjacent detail, outlines and shadows:

| Token | Hex | Use in the scene |
|---|---|---|
| bg | #111116 | deepest shadow, night sky top |
| surface | #191920 | stage edges, dark props |
| raised | #25252F | amps, cabinets |
| line | #393843 | window frames, stage lip |
| text | #F4F0E8 | highlights, phone screens |
| text2 | #A5A3AF | "BOO" text, glass |
| text3 | #6F6D7A | stars in the sky, cymbals |
| brand | #F3A33B | lit windows, confetti, earned stars |
| miss | #E56A6A | the studio REC light, flame base |

Per-tier accent ramp (the venue's lights; build 3–4 shades of each):

| Tier | Accent | Feel |
|---|---|---|
| Bedroom | warm lamp orange #E8954A | a desk lamp, fairy lights |
| Jam room | teal #3FB8AF | strip lights, egg-box foam |
| Pub gigs | amber / wood #D49A45, #6B4226 | wood panels, warm bulbs |
| Opening act | magenta stage #D14FA0 | a theatre rig, haze |
| Stadium | white / blue floods #CFE0FF, #4A7BD8 | huge beams, screens |
| Recording studio | cool grey-blue #8FA3B8 | glass, a red REC light |

## 3. The files to make

Each row replaces one placeholder. "Frames" are laid out left to right in a row; each animation is its own row, in the order given. Put the PNG in the folder shown and change that sprite's `"image"` in the folder's `manifest.json` from `placeholder:…` to the file name.

### Venues — `internal/ui/assets/career/scenes/<tier>/` (tier = bedroom, jamroom, pub, opening, stadium, studio)

| Sprite | Size | Frames | Replaces | Notes |
|---|---|---|---|---|
| sky | 480×200 | 1 | placeholder:sky:<tier> | night sky behind everything (opaque) |
| interior | 480×200 | 1 | placeholder:interior:<tier> | walls and floor; **leave windows transparent** so the city shows; stadium = stands either side, open sky |
| lights | 480×200 | 4 (row "cycle") | placeholder:lights:<tier> | lamps/beams in the tier accent; the 4 frames cycle; brighter at higher excitement (the game speeds the cycle) |
| stage | 480×200 | 1 | placeholder:stage:<tier> | stage platform (top at y=160; bedroom: the floor at y=168), amp, drum kit; studio: control-room glass + amp |

### Cities — `internal/ui/assets/career/cities/<region>/` (south-asia, east-asia, europe, north-america, latin-america, oceania, africa)

| Sprite | Size | Frames | Replaces | Notes |
|---|---|---|---|---|
| skyline | 480×200 | 1 | placeholder:skyline:<region> | silhouettes in the region's palette, a few lit windows; **no real landmarks** (no recognisable towers, bridges or buildings) |

### Guitarists — `internal/ui/assets/career/characters/<preset>/` (rae, theo, mina, sol)

One sprite sheet per preset, frames **32×48**, anchor at the feet (16, 48), standing at (140, 168) in the scene. About 48 px tall. Keep each character to one palette so it can be recoloured later.

| Row | Animation | Frames |
|---|---|---|
| 0 | idle | 2 |
| 1 | strum (synced to the song's beat) | 4 |
| 2 | cheer / rock pose | 2 |
| 3 | bow | 3 |
| 4 | slump (booed off) | 3 |

Presets: **Rae** — short dark hair, denim jacket, sunburst guitar. **Theo** — long curly hair, band tee, black guitar. **Mina** — shaved sides, leather vest, red guitar. **Sol** — beanie and flannel, natural-wood guitar. Different silhouettes, skin tones, hair and outfits. Original characters only.

### Crowd — `internal/ui/assets/career/crowd/`

Six members (`member0` … `member5`), each a sheet of **16×24** frames, anchor at the feet (8, 24). Varied skin tones, hair and shirts.

| Row | Animation | Frames | When |
|---|---|---|---|
| 0 | idle (sway) | 2 | Cold |
| 1 | nod | 2 | Warm |
| 2 | jump | 2 | Hot |
| 3 | arms up | 2 | On fire |
| 4 | arms crossed | 1 | tough crowd, low excitement |
| 5 | cheer | 2 | streaks, song end |
| 6 | boo | 2 | booed off |

### Effects — `internal/ui/assets/career/fx/`

| Sprite | Size | Frames | Notes |
|---|---|---|---|
| phone | 4×4 | 2 | a raised phone light (On fire) |
| beam | 32×120 | 1 | a stadium light beam, translucent |
| confetti | 4×4 | 3 | falling confetti piece |
| boo | 24×10 | 3 | the "BOO" puff, growing |
| flames | 16×16 | 3 | TOUGH CROWD flames, looping |
| star | 16×16 | 2 rows: unlit, lit | gig rating stars |
| rec | 6×6 | 2 | the studio's REC light, on / off |

### Crowd audio — `internal/ui/assets/career/audio/`

| Sound | File | Notes |
|---|---|---|
| bed | WAV, 10–20 s, seamless loop | crowd murmur → cheering; the game sets its volume by excitement |
| cheer | WAV, 2–4 s | end of song, streak cheer |
| boo | WAV, 2–4 s | booed off |

Audio candidates (verify the licence on the page before use): "Crowd Cheering" by SoundsExciting, Freesound, listed CC0 — https://freesound.org/people/SoundsExciting/sounds/365132/. City parallax layers that may suit the skylines: "FREE Pixel Art – City Parallax Background" by karsiori on itch.io, listed CC0 — https://karsiori.itch.io/free-pixel-art-city-background-pack (resize to the 480×200 base without smoothing).

## 4. The manifest format

Each folder has a `manifest.json`:

```json
{
  "group": "characters/rae",
  "sprites": [
    {"name": "guitarist", "image": "guitarist.png", "frameW": 32, "frameH": 48,
     "anims": [{"name": "idle", "frames": 2, "row": 0}, {"name": "strum", "frames": 4, "row": 1}],
     "anchor": [16, 48], "layer": "guitarist", "pos": [140, 168]}
  ],
  "sounds": [{"name": "bed", "file": "bed.wav", "loop": true, "levelDB": -18}]
}
```

- `image`: a PNG in the same folder, or `placeholder:<kind>` for the art drawn in code.
- `frameW`/`frameH`: one frame; `anims`: rows of frames.
- `anchor`: the point of a frame placed at `pos` (base-resolution pixels).
- `layer`: one of backdrop, city, interior, lights, crowd-back, stage, guitarist, crowd-front, fx.

## 5. Licences

Allowed: CC0, CC-BY 3.0/4.0 (credited), or art the owner provides or commissions. Not allowed: "personal use only", NonCommercial, "free" with no licence text, AI-image gallery licences, anything without a licence you can link. Add every non-original file to `internal/ui/assets/CREDITS.md` (title, author, source URL, licence, date checked) before it is committed.
