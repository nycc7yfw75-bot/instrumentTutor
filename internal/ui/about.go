package ui

import (
	_ "embed"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// The About & updates screen (ONLINE.md §A1, §A6, §A7), opened from
// Settings: the version and licence, the update channel and Check now,
// the save tools (export, import, open the folder, restore a backup) and
// the credits.

//go:embed assets/CREDITS.md
var creditsMD string

// UpstreamNotice is the upstream credit (NOTICE.md).
const UpstreamNotice = "Headliner is based on musicTutor by S95F (MIT licence)."

// BackupEntry is one saved backup the player can restore.
type BackupEntry struct {
	Name   string
	When   time.Time
	Reason string
}

// SystemHooks is what the About screen does outside the UI. Any may be
// nil; its control is then hidden.
type SystemHooks struct {
	Channel    func() string // "stable" or "beta"
	SetChannel func(string)
	CheckNow   func()
	Status     func() string // the updater's last word, e.g. "Up to date"
	Export     func() (string, error)
	Import     func() error // closes Headliner when it worked
	OpenFolder func()
	Backups    func() []BackupEntry
	Restore    func(name string) error // closes Headliner when it worked
}

// AboutScreen is the About & updates screen.
type AboutScreen struct {
	sh    *Shell
	hooks SystemHooks
	ptr   pointer
	anim  animator
	msg   string
	err   bool

	restoring bool // the backup list is open
}

// NewAboutScreen makes the screen.
func NewAboutScreen(sh *Shell, h SystemHooks) *AboutScreen { return &AboutScreen{sh: sh, hooks: h} }

type aboutLayout struct {
	x, w                float64
	stable, beta, check rect
	export, imp, folder rect
	restore             rect
	statusY, msgY       float64
	creditsY            float64
	backups             []rect
	cancel              rect
}

func (s *AboutScreen) layout() aboutLayout {
	var l aboutLayout
	l.w = math.Min(screenW-2*pgMargin, 900)
	l.x = math.Round((screenW - l.w) / 2)
	y := float64(zHeaderH) + sp2 + 2*uiTextH + sp6 + sizeDisplayM + sp3
	bw := func(s string) float64 { return kitButtonW(s, kitSecondary, false) }
	l.stable = rect{l.x, y, bw("Stable"), btnH}
	l.beta = rect{l.stable.x + l.stable.w + sp2, y, bw("Beta"), btnH}
	l.check = rect{l.beta.x + l.beta.w + sp6, y, bw("Check now"), btnH}
	l.statusY = y + btnH + sp2
	y = l.statusY + uiTextH + sp6 + sizeDisplayM + sp3
	x := l.x
	for i, r := range []*rect{&l.export, &l.imp, &l.folder, &l.restore} {
		label := [4]string{"Export progress", "Import progress", "Open save folder", "Restore a backup"}[i]
		*r = rect{x, y, bw(label), btnH}
		x += r.w + sp2
	}
	l.msgY = y + btnH + sp2
	l.creditsY = l.msgY + uiTextH + sp6
	// The backup list, over the page.
	if s.hooks.Backups != nil {
		bs := s.hooks.Backups()
		top := float64(zHeaderH) + 80
		for i := range bs {
			l.backups = append(l.backups, rect{l.x + sp6, top + 48 + float64(i)*32, l.w - 2*sp6, 28})
		}
		l.cancel = rect{l.x + sp6, top + 48 + float64(len(bs))*32 + sp3, bw("Cancel"), btnH}
	}
	return l
}

func (s *AboutScreen) say(msg string, isErr bool) { s.msg, s.err = msg, isErr }

func (s *AboutScreen) Update() error {
	s.ptr = readPointer()
	l := s.layout()
	if s.restoring {
		if inpututil.IsKeyJustPressed(ebiten.KeyEscape) || (s.ptr.pressed && s.ptr.over(l.cancel)) {
			s.restoring = false
			return nil
		}
		if s.ptr.pressed {
			bs := s.hooks.Backups()
			for i, r := range l.backups {
				if s.ptr.over(r) && i < len(bs) {
					s.restoring = false
					if err := s.hooks.Restore(bs[i].Name); err != nil {
						s.say("Could not restore: "+err.Error(), true)
					}
				}
			}
		}
		return nil
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) || escButtonPressed(escBackChip, s.ptr) {
		return errQuit
	}
	if !s.ptr.pressed {
		return nil
	}
	switch {
	case s.hooks.SetChannel != nil && s.ptr.over(l.stable):
		s.hooks.SetChannel("stable")
	case s.hooks.SetChannel != nil && s.ptr.over(l.beta):
		s.hooks.SetChannel("beta")
	case s.hooks.CheckNow != nil && s.ptr.over(l.check):
		s.hooks.CheckNow()
	case s.hooks.Export != nil && s.ptr.over(l.export):
		if path, err := s.hooks.Export(); err != nil {
			s.say("Could not export: "+err.Error(), true)
		} else if path != "" {
			s.say("Saved your progress to "+path, false)
		}
	case s.hooks.Import != nil && s.ptr.over(l.imp):
		if err := s.hooks.Import(); err != nil {
			s.say("Could not import: "+err.Error(), true)
		}
	case s.hooks.OpenFolder != nil && s.ptr.over(l.folder):
		s.hooks.OpenFolder()
	case s.hooks.Backups != nil && s.hooks.Restore != nil && s.ptr.over(l.restore):
		if len(s.hooks.Backups()) == 0 {
			s.say("There are no backups yet: one is made before every update and import.", false)
		} else {
			s.restoring = true
		}
	}
	return nil
}

func (s *AboutScreen) Draw(dst *ebiten.Image) {
	dst.Fill(colBG)
	s.anim.tick()
	l := s.layout()
	x := drawEscButton(dst, escBackChip, &s.anim, s.ptr)
	drawTextScaled(dst, "About & updates", x, 14, dispL, colNote)

	y := float64(zHeaderH) + sp2
	ver := "Version " + Build.Version
	switch {
	case Build.Dev:
		ver = "Dev build: your saves here are kept apart from the installed game's"
	case Build.Channel != "":
		ver += " · " + strings.ToUpper(Build.Channel[:1]) + Build.Channel[1:] + " channel"
	}
	drawText(dst, ver, l.x, y, colNote)
	drawText(dst, UpstreamNotice, l.x, y+uiTextH+4, colDim)

	y += 2*uiTextH + sp6
	drawTextScaled(dst, "Updates", l.x, y, dispM, colNote)
	if s.hooks.SetChannel != nil && s.hooks.Channel != nil {
		ch := s.hooks.Channel()
		kind := func(on bool) btnKind {
			if on {
				return kitPrimary
			}
			return kitSecondary
		}
		drawKitButton(dst, l.stable, "Stable", -1, kind(ch != "beta"), btnState{hover: s.ptr.over(l.stable)})
		drawKitButton(dst, l.beta, "Beta", -1, kind(ch == "beta"), btnState{hover: s.ptr.over(l.beta)})
	}
	if s.hooks.CheckNow != nil {
		drawKitButton(dst, l.check, "Check now", -1, kitSecondary, btnState{hover: s.ptr.over(l.check)})
	}
	status := "Updates are switched off in dev builds."
	if s.hooks.Status != nil {
		status = s.hooks.Status()
	}
	drawText(dst, status, l.x, l.statusY, colDim)

	drawTextScaled(dst, "Your data", l.x, l.export.y-sizeDisplayM-sp3, dispM, colNote)
	for _, b := range []struct {
		r     rect
		label string
		on    bool
	}{{l.export, "Export progress", s.hooks.Export != nil}, {l.imp, "Import progress", s.hooks.Import != nil},
		{l.folder, "Open save folder", s.hooks.OpenFolder != nil}, {l.restore, "Restore a backup", s.hooks.Restore != nil}} {
		if b.on {
			drawKitButton(dst, b.r, b.label, -1, kitSecondary, btnState{hover: s.ptr.over(b.r)})
		}
	}
	msg, col := s.msg, colDim
	if s.err {
		col = colClose
	}
	if msg == "" {
		msg = "Import and restore back up your current saves first, then close Headliner so it starts on them."
	}
	drawText(dst, truncateW(msg, l.w), l.x, l.msgY, col)

	drawTextScaled(dst, "Credits", l.x, l.creditsY, dispM, colNote)
	cy := l.creditsY + sizeDisplayM + sp3
	for _, line := range creditLines() {
		if cy > screenH-pgMargin-uiTextH {
			break
		}
		drawText(dst, truncateW(line, l.w), l.x, cy, colDim)
		cy += uiTextH + 4
	}

	if s.restoring {
		s.drawBackups(dst, l)
	}
}

func (s *AboutScreen) drawBackups(dst *ebiten.Image, l aboutLayout) {
	bs := s.hooks.Backups()
	top := float64(zHeaderH) + 80
	card := rect{l.x, top, l.w, 48 + float64(len(bs))*32 + sp3 + btnH + sp6}
	y := drawDialogCard(dst, card, "Restore which backup?")
	_ = y
	for i, b := range bs {
		r := l.backups[i]
		if s.ptr.over(r) {
			notchedFill(dst, r, colHover)
		}
		label := b.When.Format("Mon 2 Jan 2006, 15:04")
		if b.Reason != "" {
			label += "  ·  " + strings.ReplaceAll(b.Reason, "-", " ")
		}
		drawText(dst, label, r.x+sp3, r.y+(r.h-uiTextH)/2, colNote)
	}
	drawKitButton(dst, l.cancel, "Cancel", -1, kitSecondary, btnState{hover: s.ptr.over(l.cancel)})
}

// creditLines turns CREDITS.md's table into plain lines:
// "Pixel Operator — Jayvee Enaguas (CC0 1.0)".
func creditLines() []string {
	var out []string
	for _, line := range strings.Split(creditsMD, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "|") || strings.HasPrefix(line, "|---") || strings.HasPrefix(line, "| File") {
			continue
		}
		cells := strings.Split(strings.Trim(line, "|"), "|")
		if len(cells) < 5 {
			continue
		}
		lic := strings.TrimSpace(cells[4])
		if i := strings.Index(lic, " ("); i > 0 {
			lic = lic[:i]
		}
		out = append(out, fmt.Sprintf("%s — %s (%s)", strings.TrimSpace(cells[1]), strings.TrimSpace(cells[2]), lic))
	}
	return append(out, "Career scenes, characters and crowd sounds: placeholders made in code.")
}
