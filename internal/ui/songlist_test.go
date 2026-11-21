package ui

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/S95F/musicTutor/internal/progress"
)

// tabIndex is where pane p sits among the SONGS filter tabs.
func tabIndex(b *Browser, p browserPane) int {
	for i, q := range b.paneOrder() {
		if q == p {
			return i
		}
	}
	return -1
}

func TestSongTabsAreAllRecentWrittenLibrary(t *testing.T) {
	b := browserLibFixture(t, stubLibrary{})
	var got []string
	for _, p := range b.paneOrder() {
		got = append(got, paneTitles[p])
	}
	if strings.Join(got, ",") != "All,Recent,Written here,Library" {
		t.Errorf("the tabs read %v", got)
	}
}

func TestAllTabListsEachFileOnce(t *testing.T) {
	dir := t.TempDir()
	browserTree(t, dir, "a.gp", "b.gtab")
	a, bb := filepath.Join(dir, "a.gp"), filepath.Join(dir, "b.gtab")
	sh := NewShell(Services{Opener: &browserFakeOpener{}, Prefs: &browserFakePrefs{
		recents: []string{a, bb, a},
		created: []string{bb},
	}, Library: stubLibrary{pieces: []PieceInfo{{Path: a, Title: "A song"}, {Path: filepath.Join(dir, "c.gp"), Name: "c.gp"}}}}, nil)
	b := NewBrowser(sh)
	waitForLibrary(t, b)

	if n := len(b.panes[paneRecent]); n != 2 {
		t.Errorf("the recent tab has %d rows for 2 files", n)
	}
	all := b.panes[paneAll]
	if len(all) != 3 {
		t.Fatalf("All has %v, want a, b and c once each", browserNames(all))
	}
	if all[0].path != a || all[0].from != paneRecent || all[2].from != paneLibrary {
		t.Errorf("All is %v, want recent first, then the library", browserNames(all))
	}
}

func TestSongsOpenOnRecentElseLibrary(t *testing.T) {
	dir := t.TempDir()
	browserTree(t, dir, "a.gp")
	withRecent := NewBrowser(NewShell(Services{Prefs: &browserFakePrefs{recents: []string{filepath.Join(dir, "a.gp")}}, Library: stubLibrary{}}, nil))
	if withRecent.focus != paneRecent {
		t.Errorf("with a recent song the list opens on tab %d, want Recent", withRecent.focus)
	}
	empty := NewBrowser(NewShell(Services{Prefs: &browserFakePrefs{}, Library: stubLibrary{}}, nil))
	if empty.focus != paneLibrary {
		t.Errorf("with no recent songs the list opens on tab %d, want Library", empty.focus)
	}
}

func TestSongRowsShowNoFolder(t *testing.T) {
	dir := t.TempDir()
	browserTree(t, dir, "Dire Straits-Sultans Of Swing-10-29-2026.gp")
	p := filepath.Join(dir, "Dire Straits-Sultans Of Swing-10-29-2026.gp")
	b := browserFixture(t, &browserFakePrefs{recents: []string{p}}, &browserFakeOpener{})
	e := b.panes[paneRecent][0]
	if got := b.songTitle(e); got != "Sultans of Swing" {
		t.Errorf("the row is titled %q, want the cleaned title", got)
	}
	if sub, _ := b.songSub(e); strings.Contains(sub, dir) || sub == "" {
		t.Errorf("the row's second line is %q: no folder, but something", sub)
	}
}

func TestEmptyTabOffersItsButton(t *testing.T) {
	b := browserFixture(t, &browserFakePrefs{}, &browserFakeOpener{})
	b.SetOpenDialog(func(string) {})
	b.SetNewPiece(func() {})
	if a, ok := b.emptyAction(); !ok || a.id != "open" {
		t.Errorf("an empty Recent tab offers %q, want Open song", a.id)
	}
	b.focusPane(paneCreated)
	if a, ok := b.emptyAction(); !ok || a.id != "new" {
		t.Errorf("an empty Written here tab offers %q, want Write new", a.id)
	}
	if l := b.layout(); l.empty.w == 0 || l.empty.y < l.list.y {
		t.Errorf("the empty tab's button sits at %+v, want it inside the list %+v", l.empty, l.list)
	}
}

func TestStartScreenHasNoQuitOrFooter(t *testing.T) {
	b := browserFixture(t, &browserFakePrefs{}, &browserFakeOpener{})
	if b.nested() {
		t.Fatal("the root start screen thinks it is nested")
	}
	for _, m := range browserMenu {
		if m.label == "Quit" {
			t.Error("Quit is back in the header")
		}
	}
}

func cardBrowser(t *testing.T, name string) *Browser {
	t.Helper()
	st := tempStore(t, name)
	return NewBrowser(NewShell(Services{Prefs: &browserFakePrefs{}, Progress: st}, nil))
}

func TestCardShowsEveryBadgeEarnedFirst(t *testing.T) {
	b := cardBrowser(t, "Aarsh")
	ms := b.cardMedals()
	if len(ms) != len(progress.Badges) {
		t.Fatalf("the card knows %d badges, want all %d", len(ms), len(progress.Badges))
	}
	for _, m := range ms {
		if m.earned {
			t.Errorf("a new player already has %s", m.def.Name)
		}
		if _, ok := badgeIcon[m.def.ID]; !ok {
			t.Errorf("badge %s has no medal icon", m.def.Name)
		}
	}
}

func TestBKeyOpensTheBadges(t *testing.T) {
	b := cardBrowser(t, "Aarsh")
	if err := b.handleKey(ebiten.KeyB); err != nil {
		t.Fatal(err)
	}
	if !b.badgesOpen {
		t.Error("B did not open the badges")
	}
	noCard := browserFixture(t, &browserFakePrefs{}, &browserFakeOpener{})
	noCard.handleKey(ebiten.KeyB)
	if noCard.badgesOpen {
		t.Error("B opened badges with no progress store")
	}
}

func TestStreakLineLeadsWithDaysPractised(t *testing.T) {
	s := streakLine(progress.Streak{Days: 3, TotalDays: 12, Freezes: 2, TodayDone: true})
	if s != "12 days practised · 3-day streak · 2 freezes" {
		t.Errorf("streak line %q", s)
	}
}
