package ui

import (
	"fmt"
	"image/color"
	"math"
	"sync"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/S95F/musicTutor/internal/pitch"
	"github.com/S95F/musicTutor/internal/practice"
)

type noteKey struct {
	start int64
	str   int
}

type feed struct {
	mu            sync.Mutex
	results       []practice.NoteResult
	tunerNote     pitch.Note
	tunerSounding bool
	status        func() (levelDB float64, dropped int64)
	waitCtl       bool
	warning       string

	warnPosted bool
}

type liveUI struct {
	feed feed

	frame     int64
	tunerView bool
	wait      bool

	live          bool
	waitCtl       bool
	levelDB       float64
	dropped       int64
	tunerNote     pitch.Note
	tunerSounding bool
	stats         practice.Stats
	verdicts      map[noteKey]practice.Verdict

	warnMsg   string
	warnShown bool

	warnAsserted bool

	stallLastFrames int64
	stallCount      int
	stallPrev       string
	stalled         bool
}

const (
	audioStallWarning = "no sound is coming out — playback is stalled; press S for settings and check the playback device"
	audioStallAfter   = 150
)

func (a *App) checkAudioStalled() {
	if !a.eng.Playing() {
		a.stallCount = 0
		return
	}
	f := a.eng.TotalFrames()
	if f != a.stallLastFrames {
		a.stallLastFrames = f
		a.stallCount = 0
		if a.stalled {
			a.stalled = false
			a.SetLiveWarning(a.stallPrev)
		}
		return
	}
	if a.stalled {
		return
	}
	a.stallCount++
	if a.stallCount >= audioStallAfter {
		a.stalled = true
		a.stallPrev = a.warnMsg
		a.SetLiveWarning(audioStallWarning)
	}
}

func (a *App) OfferResults(rs []practice.NoteResult) {
	if len(rs) == 0 {
		return
	}
	a.feed.mu.Lock()
	a.feed.results = append(a.feed.results, rs...)
	a.feed.mu.Unlock()
}

func (a *App) OfferTuner(n pitch.Note, sounding bool) {
	a.feed.mu.Lock()
	a.feed.tunerNote, a.feed.tunerSounding = n, sounding
	a.feed.mu.Unlock()
}

func (a *App) SetLiveStatus(fn func() (levelDB float64, dropped int64)) {
	a.feed.mu.Lock()
	a.feed.status = fn
	a.feed.mu.Unlock()
}

func (a *App) SetWaitControl(enabled bool) {
	a.feed.mu.Lock()
	a.feed.waitCtl = enabled
	a.feed.mu.Unlock()
}

func (a *App) SetLiveWarning(msg string) {
	a.feed.mu.Lock()
	a.feed.warning, a.feed.warnPosted = msg, true
	a.feed.mu.Unlock()
}

func (a *App) warningVisible() bool { return a.warnShown && a.warnMsg != "" }

func (a *App) dismissWarning() { a.warnShown = false }

func (a *App) syncLive() {
	a.feed.mu.Lock()
	rs := a.feed.results
	a.feed.results = nil
	a.tunerNote, a.tunerSounding = a.feed.tunerNote, a.feed.tunerSounding
	status := a.feed.status
	a.waitCtl = a.feed.waitCtl
	warn, posted := a.feed.warning, a.feed.warnPosted
	a.feed.warnPosted = false
	a.feed.mu.Unlock()

	switch {
	case warn != a.warnMsg:
		a.warnMsg, a.warnShown = warn, warn != ""
	case posted && !a.warnAsserted && warn != "":
		a.warnShown = true
	}
	a.warnAsserted = posted

	if len(rs) > 0 && a.verdicts == nil {
		a.verdicts = make(map[noteKey]practice.Verdict)
	}
	for _, r := range rs {
		a.verdicts[noteKey{r.Event.Start, r.Event.String}] = r.Verdict
		switch r.Verdict {
		case practice.VerdictHit:
			a.stats.Hit++
		case practice.VerdictClose:
			a.stats.Close++
		default:
			a.stats.Miss++
		}
	}

	a.gameResults(rs)

	a.live = status != nil
	if status != nil {
		a.levelDB, a.dropped = status()
	}
	a.updateInputHint()
}

func (a *App) verdictAt(start int64, str int, pos int64) (practice.Verdict, bool) {
	if start > pos {
		return 0, false
	}
	v, ok := a.verdicts[noteKey{start, str}]
	return v, ok
}

func (a *App) toggleWait() {
	a.wait = !a.wait
	a.eng.SetWaitMode(a.wait)
}

func verdictColor(v practice.Verdict) color.RGBA {
	switch v {
	case practice.VerdictHit:
		return colHit
	case practice.VerdictClose:
		return colClose
	default:
		return colMiss
	}
}

func (a *App) waitingKeys() map[noteKey]bool {
	evs, _, ok := a.eng.WaitingOn()
	if !ok || len(evs) == 0 {
		return nil
	}
	m := make(map[noteKey]bool, len(evs))
	for _, ev := range evs {
		m[noteKey{ev.Start, ev.String}] = true
	}
	return m
}

func (a *App) pulseCol() color.RGBA {
	p := 0.5 + 0.5*math.Sin(float64(a.frame)*0.15)
	return lerpRGBA(colWaitLo, colWaitHi, p)
}

func lerpRGBA(c0, c1 color.RGBA, t float64) color.RGBA {
	l := func(x, y uint8) uint8 { return uint8(float64(x) + t*(float64(y)-float64(x)) + 0.5) }
	return color.RGBA{l(c0.R, c1.R), l(c0.G, c1.G), l(c0.B, c1.B), 255}
}

var keyNames = [12]string{"C", "C#", "D", "D#", "E", "F", "F#", "G", "G#", "A", "A#", "B"}

func keyName(key int) string {
	return fmt.Sprintf("%s%d", keyNames[((key%12)+12)%12], key/12-1)
}

// drawTuner is the tuner (REDESIGN §8): the note huge in the display font,
// the cents offset as a 21-block segmented meter (the centre block lights
// hit when within ±5 cents; others close, then miss, by distance), and the
// nearest open string below.
func (a *App) drawTuner(screen *ebiten.Image) {
	if !a.tunerSounding {
		s, scale := a.tunerIdleLine()
		drawTextScaled(screen, s, centreXScaled(s, 0, screenW, scale), a.tabMid()-30, scale, colString)
		return
	}
	n := a.tunerNote
	inTune := math.Abs(n.Cents) <= 5
	col := colNote
	if inTune {
		col = colHit
	}
	mid := math.Round(a.tabMid())
	name := a.displayName(n.Key)
	drawTextScaled(screen, name, math.Round(centreXScaled(name, 0, screenW, dispX)), mid-96, dispX, col)

	const blocks, bw, bh, gap = tunerBlocks, 16.0, 24.0, 4.0
	w := blocks*(bw+gap) - gap
	x0 := math.Round((screenW - w) / 2)
	y0 := mid - 8
	lit := tunerBlock(n.Cents)
	for i := 0; i < blocks; i++ {
		r := rect{x0 + float64(i)*(bw+gap), y0, bw, bh}
		switch {
		case i == lit:
			notchedFill(screen, r, tunerBlockCol(i, inTune))
		case i == blocks/2:
			notchedStroke(screen, r, 1, colDim) // the target
		default:
			vector.FillRect(screen, float32(r.x), float32(r.y), bw, bh, colBlockOff, false)
		}
	}
	drawMonoSized(screen, "-50", x0-sp3-textWMonoSized("-50", sizeUISmall), y0+4, sizeUISmall, colBarline)
	drawMonoSized(screen, "+50", x0+w+sp3, y0+4, sizeUISmall, colBarline)
	cents := fmt.Sprintf("%+.0f cents", n.Cents)
	drawMonoSized(screen, cents, math.Round((screenW-textWMonoSized(cents, sizeUI))/2), y0+bh+sp3, sizeUI, colDim)
	if s := a.nearestString(n.Key); s != "" {
		drawText(screen, s, math.Round(centreX(s, 0, screenW)), y0+bh+sp3+uiTextH+sp2, colDim)
	}
}

const tunerBlocks = 21

// tunerBlock is the meter block for a cents offset: 10 is in the middle,
// each block 5 cents.
func tunerBlock(cents float64) int {
	return max(0, min(tunerBlocks-1, tunerBlocks/2+int(math.Round(cents/5))))
}

// tunerBlockCol colours the lit block by its distance from the centre.
func tunerBlockCol(i int, inTune bool) color.RGBA {
	switch d := i - tunerBlocks/2; {
	case d == 0 && inTune:
		return colHit
	case d >= -3 && d <= 3:
		return colClose
	}
	return colMiss
}

// nearestString names the open string closest to a key on the practised
// track ("Closest string: A"), or "" for a wind track.
func (a *App) nearestString(key int) string {
	tr := a.displayed()
	if tr.Wind != nil || len(tr.Tuning) == 0 {
		return ""
	}
	best, bestD := 0, 1<<30
	for i, k := range tr.Tuning {
		d := key - k
		if d < 0 {
			d = -d
		}
		if d < bestD {
			best, bestD = i, d
		}
	}
	return "Closest string: " + keyName(tr.Tuning[best])
}

func (a *App) tunerIdleLine() (string, float64) {
	if !a.live {
		return "no live input — " + a.liveRemedy(), dispL
	}
	return "listening...", dispX
}

func (a *App) liveRemedy() string {
	if a.settings != nil {
		return "choose your capture device in settings (S)"
	}
	return "quit and re-run with: musictutor play -listen <file>"
}
