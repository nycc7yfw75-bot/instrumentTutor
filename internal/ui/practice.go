package ui

import (
	"fmt"
	"image/color"
	"math"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/S95F/musicTutor/internal/score"
)

// The practice screen's layout (docs/REDESIGN.md §6), top to bottom:
// header 64, toolbar 48, track bar 44, the tab in everything left, the
// timeline zone 72; a 32px margin all round and no fixed canvas.

const (
	pgMargin   = sp8
	zHeaderH   = 64.0
	zToolbarH  = 48.0
	zTrackBarH = 44.0
	zTimelineH = 72.0
	zItemH     = 40.0 // toolbar item hit height (≥ 40, MOTION §A6)
	zTrackTagH = 32.0
	zTlTrackH  = 24.0
	zRulerH    = 28.0 // bar numbers above the staff
	zNameW     = 24.0 // string names at the left of the staff
)

// stageBandH is the performance scene's band (CAREER.md §6.1). It takes the
// toolbar's place while a gig is drawn; 0 everywhere else.
var stageBandH float64

// toolbarSlot is the height between the header and the track bar.
func toolbarSlot() float64 {
	if stageBandH > 0 {
		return stageBandH
	}
	return zToolbarH
}

func zToolbarY() float64  { return zHeaderH }
func zTrackBarY() float64 { return zHeaderH + toolbarSlot() }
func zBodyY() float64     { return zTrackBarY() + zTrackBarH }

// timelineZoneY is the top of the bottom zone: the timeline and its line.
func timelineZoneY() float64 { return screenH - pgMargin - zTimelineH }

// bannerRect is the warning strip: under the track bar, over the tab zone.
func bannerRect() rect {
	return rect{pgMargin, zBodyY() + sp1, screenW - 2*pgMargin, bannerH}
}

// tabZone is the tab's share of the screen: what the bands above and
// below leave.
func (a *App) tabZone() rect {
	top := zBodyY()
	if a.warningVisible() {
		b := bannerRect()
		top = b.y + b.h + sp2
	}
	bottom := timelineZoneY() - sp2
	return rect{0, top, screenW, bottom - top}
}

// stringGap scales with the room: clamp(36, (tab height − 48) / 6, 64);
// a neck of 7 or more strings divides by its own count so it still fits.
func (a *App) stringGap() float64 {
	g := (a.tabZone().h - 48) / float64(max(6, a.laneLines()))
	return math.Max(36, math.Min(64, math.Floor(g)))
}

// tabTop is the top string's y: the staff sits centred in its zone, under
// the bar-number ruler.
func (a *App) tabTop() float64 {
	z := a.tabZone()
	staff := float64(a.laneLines()-1) * a.stringGap()
	return math.Round(z.y + zRulerH/2 + (z.h-staff)/2)
}

// fretSize is the fret numbers' size: 0.45 × the string gap, 16..28.
func (a *App) fretSize() float64 {
	return math.Max(16, math.Min(28, math.Round(0.45*a.stringGap())))
}

// tabLeft and tabRight bound the staff: inside the margins, after the
// string names.
func tabLeft() float64  { return pgMargin + zNameW }
func tabRight() float64 { return screenW - pgMargin }

// playheadXAt is the playhead's x on the tab.
func playheadXAt() float64 { return math.Round(tabLeft() + (tabRight()-tabLeft())*playheadX) }

// ---- Header ----

var (
	reDateSuffix = regexp.MustCompile(`[-_ ]\d{1,2}-\d{1,2}-\d{2,4}$`)
	reDupSuffix  = regexp.MustCompile(`\s*\(\d+\)$`)
	smallWords   = map[string]bool{"a": true, "an": true, "and": true, "as": true, "at": true, "but": true,
		"by": true, "for": true, "in": true, "of": true, "on": true, "or": true, "the": true, "to": true}
)

// CleanTitle is a piece's title as people read it (§6.1): its title
// metadata, or the file name without extension, date suffix or "(1)", in
// title case with the short words lower case. A "Band-Song" file name keeps
// only the song.
func CleanTitle(sc *score.Score, path string) string { return cleanPieceTitle(sc.Title, path) }

// cleanPieceTitle is CleanTitle for a title the caller already has (the
// library's metadata), or "" to derive it from the file name.
func cleanPieceTitle(title, path string) string {
	t := strings.TrimSpace(title)
	if t == "" {
		t = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		for {
			s := reDupSuffix.ReplaceAllString(reDateSuffix.ReplaceAllString(t, ""), "")
			if s == t {
				break
			}
			t = s
		}
		if i := strings.Index(t, "-"); i > 0 && i < len(t)-1 && strings.Contains(t, " ") {
			t = t[i+1:] // "Dire Straits-Sultans Of Swing": the song
		} else {
			t = strings.ReplaceAll(t, "-", " ") // "little-wing": one name
		}
		t = strings.ReplaceAll(t, "_", " ")
	}
	return titleCase(strings.TrimSpace(t))
}

func titleCase(s string) string {
	words := strings.Fields(s)
	for i, w := range words {
		lw := strings.ToLower(w)
		switch {
		case i > 0 && i < len(words)-1 && smallWords[lw]:
			words[i] = lw
		case w == strings.ToUpper(w) && len(w) > 1:
			// AC/DC, PM: an acronym stays as written.
		default:
			words[i] = strings.ToUpper(lw[:1]) + lw[1:]
		}
	}
	return strings.Join(words, " ")
}

// SetPiecePath tells the view its file, for the title fallback.
func (a *App) SetPiecePath(path string) { a.piecePath = path }

func (a *App) songTitle() string {
	if t := CleanTitle(a.sc, a.piecePath); t != "" {
		return t
	}
	return "HEADLINER"
}

// songSub is the line under the title: the track being practised (the importers
// read no artist).
func (a *App) songSub() string {
	return a.trackName(a.track)
}

// headerPills are the state pills, left to right.
func (a *App) headerPills() []struct {
	label string
	kind  pillKind
} {
	type pill = struct {
		label string
		kind  pillKind
	}
	var out []pill
	if a.live {
		if a.noSignal() {
			out = append(out, pill{"NO SIGNAL", pillWarn})
		} else {
			out = append(out, pill{"LIVE", pillLive})
		}
	}
	if _, _, on := a.eng.Loop(); on {
		out = append(out, pill{fmt.Sprintf("PASS %d", a.eng.PassCount()), pillOutline})
	}
	state := "PAUSED"
	switch in, left := a.eng.CountingIn(); {
	case in:
		state = fmt.Sprintf("COUNT-IN %d", left)
	case a.eng.Waiting():
		state = "WAITING"
	case a.eng.Playing():
		state = "PLAYING"
	}
	return append(out, pill{state, pillOutline})
}

func (a *App) drawPracticeHeader(dst *ebiten.Image) {
	x := drawEscButton(dst, a.backChip(), &a.anim, a.ptr)
	pills := a.headerPills()
	right := screenW - pgMargin
	m := &a.chrome
	for i := len(pills) - 1; i >= 0; i-- {
		p := pills[i]
		right -= pillKindW(p.label, p.kind)
		if p.kind == pillLive || p.kind == pillWarn {
			// LIVE ↔ NO SIGNAL: a 200ms replacement crossfade (MOTION §A).
			m.live.set(p.label)
			k := m.live.step(motionDt())
			if k < 1 && m.live.prev != "" {
				old := pillLive
				if m.live.prev == "NO SIGNAL" {
					old = pillWarn
				}
				ow := pillKindW(m.live.prev, old)
				drawPillA(dst, right+pillKindW(p.label, p.kind)-ow, 20, m.live.prev, old, 1-k, true)
			}
			drawPillA(dst, right, 20, p.label, p.kind, k, m.liveDotOn())
		} else {
			drawPill(dst, right, 20, p.label, p.kind)
		}
		right -= sp2
	}
	budget := right - sp6 - x
	drawTextScaled(dst, truncateWScaled(a.songTitle(), budget, dispL), x, 6, dispL, colNote)
	drawText(dst, truncateW(a.songSub(), budget), x, 42, colDim)
}

// ---- No signal ----

// noSignalAfter is how long a flat input while playing live turns the
// LIVE pill into NO SIGNAL (§6.7).
const (
	noSignalAfter = 3.0
	noSignalDB    = -55.0
)

// noSignal reports a flat input meter for 3s while playing live.
func (a *App) noSignal() bool { return a.flatFor >= noSignalAfter }

// trackSignal advances the flat-input clock; once a frame.
func (a *App) trackSignal() {
	dt := uiFrameSeconds()
	a.chrome.clock += dt
	lv := math.Max(-60, math.Min(0, a.levelDB))
	a.chrome.peak.update(int(math.Round((lv+60)/60*hudMeterBlocks)), dt)
	if a.live && a.eng.Playing() && a.levelDB < noSignalDB {
		a.flatFor += uiFrameSeconds()
		return
	}
	a.flatFor = 0
}

// ---- Track bar ----

// instrumentOf names a track's instrument and its icon, from its kind and
// GM program, falling back to its name.
func instrumentOf(tr *score.Track) (string, iconID) {
	name := strings.ToLower(tr.Name)
	has := func(ws ...string) bool {
		for _, w := range ws {
			if strings.Contains(name, w) {
				return true
			}
		}
		return false
	}
	switch {
	case tr.Drums:
		return "Drums", icDrums
	case has("vocal", "voice", "vox", "choir", "sing"):
		return "Vocals", icMic
	case tr.Wind != nil:
		return "Horn", icHorn
	case has("bass") || (tr.Program >= 32 && tr.Program <= 39):
		return "Bass", icBass
	case has("piano", "key", "organ", "synth") || tr.Program <= 7 && tr.Program > 0 || (tr.Program >= 16 && tr.Program <= 23):
		return "Keys", icKeys
	case tr.Program >= 52 && tr.Program <= 54:
		return "Vocals", icMic
	}
	return "Guitar", icGuitar
}

var instrumentWords = []string{"vocals", "vocal", "voice", "vox", "guitars", "guitar", "gtr", "bass", "drums", "drum", "kit",
	"keys", "keyboard", "piano", "organ", "synth", "horn"}

// shortTrackNames are the tags' names (§6.3): the instrument, and where two
// tracks share one, "Guitar · Rhythm" from what else their name says.
func (a *App) shortTrackNames() []string {
	out := make([]string, len(a.sc.Tracks))
	count := map[string]int{}
	for i, tr := range a.sc.Tracks {
		out[i], _ = instrumentOf(tr)
		count[out[i]]++
	}
	for i, tr := range a.sc.Tracks {
		if count[out[i]] < 2 {
			continue
		}
		rest := tr.Name
		if j := strings.LastIndex(rest, "|"); j >= 0 {
			rest = rest[j+1:] // "Player | Model | Lead Guitar": the part
		}
		for _, w := range instrumentWords {
			rest = regexp.MustCompile(`(?i)\b`+w+`\b`).ReplaceAllString(rest, "")
		}
		f := strings.Fields(rest)
		if len(f) > 2 {
			f = f[:2] // a tag stays short; the tooltip has the full name
		}
		rest = strings.Join(f, " ")
		if rest == "" {
			rest = fmt.Sprintf("%d", i+1)
		}
		out[i] += " · " + titleCase(rest)
	}
	return out
}

// trackTagW is a tag's width: icon, name, then the YOU pill or the
// speaker.
func (a *App) trackTagW(i int, name string) float64 {
	w := iconSize + sp1
	if i == a.track {
		return w + textW(name) + sp2 + pillW("YOU") + sp2
	}
	return w + textWSmall(name) + sp1 + iconSize
}

// trackOrder is your track first, then the rest in file order.
func (a *App) trackOrder() []int {
	order := []int{a.track}
	for i := range a.sc.Tracks {
		if i != a.track {
			order = append(order, i)
		}
	}
	return order
}

func (a *App) drawTrackTags(dst *ebiten.Image, l practiceLayout, p pointer) {
	y := zTrackBarY() + (zTrackBarH-sizeDisplayS)/2 - 1
	drawSectionHeader(dst, pgMargin, y, 0, "TRACKS", 0)
	names := a.shortTrackNames()
	for i, r := range l.tracks {
		if r.empty() {
			continue
		}
		av := a.anim.step(fmt.Sprintf("track:%d", i), p.over(r), p.down, motionDt())
		_, icon := instrumentOf(a.sc.Tracks[i])
		muted := a.mutedAudibly(i)
		alpha := 1.0
		if muted {
			alpha = 0.4
		}
		ink := withAlpha(lerpCol(colBarline, colNote, av.hover), alpha)
		cy := r.y + r.h/2
		x := r.x
		if i == a.track {
			drawPixelIcon(dst, icon, x, cy-iconSize/2, colNote)
			x += iconSize + sp1
			drawText(dst, names[i], x, cy-uiTextH/2, colNote)
			x += textW(names[i]) + sp2
			drawPill(dst, x, cy-pillH/2, "YOU", pillYou)
		} else {
			drawPixelIcon(dst, icon, x, cy-iconSize/2, ink)
			x += iconSize + sp1
			drawTextSmall(dst, names[i], x, cy-uiTextH/2+1, ink)
			x += textWSmall(names[i]) + sp1
			switch {
			case a.solo == i+1:
				drawPixelIcon(dst, icSolo, x, cy-iconSize/2, colNote)
			case muted:
				drawPixelIcon(dst, icSpeakerMuted, x, cy-iconSize/2, ink)
			default:
				drawPixelIcon(dst, icSpeakerOn, x, cy-iconSize/2, ink)
			}
		}
		tip := a.sc.Tracks[i].Name
		if i < 9 {
			tip = fmt.Sprintf("%s — click mute, right-click solo", tip)
		}
		key := ""
		if i < 9 {
			key = fmt.Sprintf("%d", i+1)
		}
		a.tip.offer(fmt.Sprintf("track:%d", i), tip, key, r, p.over(r), motionDt())
	}
	if n := a.hiddenTracks(l); n > 0 {
		// "+2 more" after the last tag that fits; its tooltip names the keys.
		last := 0.0
		for _, r := range l.tracks {
			last = math.Max(last, r.x+r.w)
		}
		s := fmt.Sprintf("+%d more", n)
		r := rect{last + sp4, zTrackBarY(), textWSmall(s), zTrackBarH}
		drawTextSmall(dst, s, r.x, r.y+(r.h-uiTextH)/2+1, colHint)
		a.tip.offer("tracks:more", a.hiddenTracksHint(len(l.tracks)-n, n), "", r, p.over(r), motionDt())
	}
}

// ---- HUD (right of the track bar, §6.7) ----

const (
	hudMeterBlocks = 12
	hudW           = 360.0 // room kept for the HUD at the track bar's right
)

// hudCounts is "12 · 3 · 1" in hit, close and miss.
func (a *App) drawPracticeHUD(dst *ebiten.Image) {
	cy := zTrackBarY() + zTrackBarH/2
	right := screenW - pgMargin
	if !a.live {
		s := "Not listening"
		drawText(dst, s, right-textW(s), cy-uiTextH/2, colBarline)
		return
	}
	acc := a.hudAccuracy()
	right -= textWScaled(acc, dispM)
	drawTextScaled(dst, acc, right, cy-sizeDisplayM/2-1, dispM, colNote)
	right -= sp4
	parts := []struct {
		s   string
		col color.RGBA
	}{{fmt.Sprint(a.stats.Hit), colHit}, {" · ", colBarline}, {fmt.Sprint(a.stats.Close), colClose}, {" · ", colBarline}, {fmt.Sprint(a.stats.Miss), colMiss}}
	for i := len(parts) - 1; i >= 0; i-- {
		right -= textWMonoSized(parts[i].s, sizeUILarge)
		drawMonoSized(dst, parts[i].s, right, cy-11, sizeUILarge, parts[i].col)
	}
	right -= sp4
	lv := math.Max(-60, math.Min(0, a.levelDB))
	meterW := float64(hudMeterBlocks)*(meterBlockW+meterGap) - meterGap
	right -= meterW
	drawSegMeter(dst, right, cy-meterBlockH/2, hudMeterBlocks, (lv+60)/60)
	drawPeakBlock(dst, right, cy-meterBlockH/2, hudMeterBlocks, a.chrome.peak.block, int(math.Round((lv+60)/60*hudMeterBlocks)))
	meter := rect{right, cy - 12, meterW, 24}
	hint, hintCol := a.inputHint()
	if a.dropped > 0 {
		hint, hintCol = strings.TrimSpace(fmt.Sprintf("dropped %d samples   %s", a.dropped, hint)), colMiss
	}
	inCol := colBarline
	if hint != "" {
		inCol = hintCol // the label warns; the tooltip says why
	}
	right -= sp2 + textWScaled("IN", dispS)
	drawTextScaled(dst, "IN", right, cy-sizeDisplayS/2-1, dispS, inCol)
	if hint == "" {
		hint = "Input level"
	}
	a.tip.offer("hud:meter", hint, "", rect{right, meter.y, meter.x + meter.w - right, meter.h}, a.ptr.over(rect{right, meter.y, meter.x + meter.w - right, meter.h}), motionDt())
}

// ---- Messages over the bottom of the tab ----

func (a *App) drawPracticeMessages(dst *ebiten.Image) {
	y := timelineZoneY() - sp2 - uiTextH
	if msg := a.reloadPrompt(); msg != "" {
		drawText(dst, msg, centreX(msg, 0, screenW), y, colClose)
		y -= uiTextH + sp1
	}
	if msg := a.bpmMessage(); msg != "" {
		drawText(dst, msg, centreX(msg, 0, screenW), y, colDim)
	}
}

// ---- Banner (§5) ----

// bannerItems splits the warning into its conditions: the banner shows the
// first and offers the rest as "N more".
func (a *App) bannerItems() []string {
	if a.warnMsg == "" {
		return nil
	}
	var out []string
	for _, s := range strings.Split(a.warnMsg, "; ") {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, sentenceCase(s))
		}
	}
	return out
}

func (a *App) drawPracticeBanner(dst *ebiten.Image) {
	items := a.bannerItems()
	if len(items) == 0 {
		return
	}
	more := len(items) - 1 + len(a.importWarns)
	drawBanner(dst, bannerRect(), items[0], more, a.ptr)
}

// ---- Fret-number font ----

func drawMonoSized(dst *ebiten.Image, s string, x, y, size float64, col color.RGBA) {
	drawFace(dst, s, faceOf(srcMono, size), math.Round(x), math.Round(y), col)
}

func textWMonoSized(s string, size float64) float64 { return textAdvance(s, faceOf(srcMono, size)) }

func drawFretText(dst *ebiten.Image, s string, x, y, size float64, col color.RGBA) {
	drawFace(dst, s, faceOf(srcMonoBold, size), math.Round(x), math.Round(y), col)
}

func textWFret(s string, size float64) float64 { return textAdvance(s, faceOf(srcMonoBold, size)) }

// drawPracticeDividers draws the 1px lines between toolbar groups.
func drawToolbarDivider(dst *ebiten.Image, x float64) {
	y := zToolbarY() + 14
	vector.FillRect(dst, float32(math.Round(x)), float32(y), 1, zToolbarH-28, colPanelEdge, false)
}

// tabMid is the middle of the tab zone, where the tuner centres.
func (a *App) tabMid() float64 { z := a.tabZone(); return z.y + z.h/2 }

// practiceTabZone is the tab zone without a banner, for overlays placed
// before an App is at hand.
func practiceTabZone() rect {
	return rect{0, zBodyY(), screenW, timelineZoneY() - sp2 - zBodyY()}
}

// hudAccuracy is the HUD's accuracy: the scored pass when the game layer
// is on, else the practice stats; "—" until there is something to grade.
func (a *App) hudAccuracy() string {
	if a.game.sess != nil {
		pass := a.game.sess.Pass()
		if !a.game.passOn {
			pass = a.game.lastSummary
		}
		if pct, ok := pass.AccuracyPct(); ok {
			return fmt.Sprintf("%d%%", pct)
		}
		return "—"
	}
	if a.stats.Hit+a.stats.Close+a.stats.Miss > 0 {
		return fmt.Sprintf("%.0f%%", a.stats.Accuracy()*100)
	}
	return "—"
}
