package ui

import (
	"context"
	"errors"
	"fmt"
	"image/color"
	"math"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/S95F/musicTutor/internal/online"
	"github.com/S95F/musicTutor/internal/progress"
)

// The Account screen (ONLINE.md §B2–B5), from Settings → ACCOUNT: sign in
// with Google or an email link; link a profile (or, on a second laptop,
// download or merge the account's one); the sync status; the friend code
// and the friends list; sign out; delete the account. Every network call
// runs in the background: the screen never waits on the internet.

// AccountDeps is what the screen needs from the app.
type AccountDeps struct {
	Client   *online.Client
	Syncer   *online.Syncer
	Store    *progress.Store
	Copy     func(string) error // to the clipboard
	Releases string             // the download link in "Share my code"
}

// AccountScreen is the Account screen.
type AccountScreen struct {
	sh   *Shell
	d    AccountDeps
	ptr  pointer
	anim animator

	field     string // what is being typed: the email or a friend code
	cancelRun context.CancelFunc

	mu      sync.Mutex
	busy    string // "Waiting for the browser…", "Sending…"
	msg     string
	msgErr  bool
	me      *online.Me
	friends []online.Friend
	remote  *progress.Profile // the account's profile, before linking
	checked bool              // remote looked up
	loading bool

	confirmDelete bool
	confirmFocus  int // 0 Cancel (default), 1 Delete account
	linkChoice    int // which local profile to link
}

// NewAccountScreen opens the screen.
func NewAccountScreen(sh *Shell, d AccountDeps) *AccountScreen {
	s := &AccountScreen{sh: sh, d: d}
	s.refresh()
	return s
}

func (s *AccountScreen) say(msg string, isErr bool) {
	s.mu.Lock()
	s.msg, s.msgErr = msg, isErr
	s.mu.Unlock()
}

// friendly turns an error into a sentence.
func friendly(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, online.ErrOffline):
		return "Offline: can't reach the server. Everything else keeps working; try again later."
	case errors.Is(err, online.ErrNoToken):
		return "You're signed out. Sign in again."
	}
	return err.Error()
}

// background runs fn off the UI thread with a busy line.
func (s *AccountScreen) background(busy string, fn func(ctx context.Context) error) {
	s.mu.Lock()
	if s.busy != "" {
		s.mu.Unlock()
		return
	}
	s.busy, s.msg = busy, ""
	s.mu.Unlock()
	ctx, cancel := context.WithCancel(context.Background())
	s.cancelRun = cancel
	go func() {
		err := fn(ctx)
		cancel()
		s.mu.Lock()
		s.busy = ""
		if err != nil && !errors.Is(err, context.Canceled) {
			s.msg, s.msgErr = friendly(err), true
		}
		s.mu.Unlock()
	}()
}

// refresh reloads the card, friends and (before linking) the account's
// profile.
func (s *AccountScreen) refresh() {
	if s.d.Client == nil {
		return
	}
	if _, ok := s.d.Client.SignedIn(); !ok {
		return
	}
	s.mu.Lock()
	if s.loading {
		s.mu.Unlock()
		return
	}
	s.loading = true
	s.mu.Unlock()
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		var me *online.Me
		if m, ok, err := s.d.Client.MyCard(ctx); err == nil && ok {
			me = &m
		}
		fr, ferr := s.d.Client.Friends(ctx)
		var remote *progress.Profile
		checked := false
		if !s.linked() && s.d.Syncer != nil {
			if p, ok, err := s.d.Syncer.RemoteProfile(ctx); err == nil {
				checked = true
				if ok {
					remote = p
				}
			}
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		s.loading = false
		if me != nil {
			s.me = me
		}
		if ferr == nil {
			s.friends = fr
		}
		if checked {
			s.remote, s.checked = remote, true
		}
		if ferr != nil && errors.Is(ferr, online.ErrOffline) {
			s.msg, s.msgErr = friendly(ferr), true
		}
	}()
}

// linked reports this laptop's profile linked to the signed-in account.
func (s *AccountScreen) linked() bool {
	if s.d.Store == nil || s.d.Client == nil {
		return false
	}
	acct, ok := s.d.Client.SignedIn()
	link, lok := s.d.Store.Linked()
	return ok && lok && link.UserID == acct.UserID
}

// accountButton is one clickable thing on the page.
type accountButton struct {
	r     rect
	label string
	kind  btnKind
	act   func()
}

type accountPage struct {
	x, w    float64
	lines   []accountLine
	buttons []accountButton
	field   rect // the text field, if any (w 0: none)
	fieldOf string
}

type accountLine struct {
	y    float64
	x    float64
	text string
	col  color.RGBA
	big  bool
}

func (s *AccountScreen) Update() error {
	s.ptr = readPointer()
	if s.confirmDelete {
		return s.updateConfirm()
	}
	page := s.page()
	if page.field.w > 0 {
		for _, r := range ebiten.AppendInputChars(nil) {
			if r >= 32 && utf8.RuneCountInString(s.field) < 80 {
				s.field += string(r)
			}
		}
		if backspaceHeld() && s.field != "" {
			_, n := utf8.DecodeLastRuneInString(s.field)
			s.field = s.field[:len(s.field)-n]
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyEnter) && len(page.buttons) > 0 {
			for _, b := range page.buttons {
				if b.kind == kitPrimary && b.act != nil {
					b.act()
					break
				}
			}
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) || escButtonPressed(escBackChip, s.ptr) {
		if s.cancelRun != nil {
			s.cancelRun()
		}
		return errQuit
	}
	if s.ptr.pressed {
		for _, b := range page.buttons {
			if s.ptr.over(b.r) && b.act != nil {
				b.act()
				break
			}
		}
	}
	return nil
}

func (s *AccountScreen) updateConfirm() error {
	switch {
	case inpututil.IsKeyJustPressed(ebiten.KeyEscape):
		s.confirmDelete = false
	case inpututil.IsKeyJustPressed(ebiten.KeyTab), inpututil.IsKeyJustPressed(ebiten.KeyLeft), inpututil.IsKeyJustPressed(ebiten.KeyRight):
		s.confirmFocus = 1 - s.confirmFocus
	case inpututil.IsKeyJustPressed(ebiten.KeyEnter):
		s.confirmed(s.confirmFocus == 1)
	}
	if s.ptr.pressed {
		card, cancel, del := s.confirmRects()
		_ = card
		switch {
		case s.ptr.over(cancel):
			s.confirmed(false)
		case s.ptr.over(del):
			s.confirmed(true)
		}
	}
	return nil
}

func (s *AccountScreen) confirmed(del bool) {
	s.confirmDelete = false
	if !del {
		return
	}
	s.background("Deleting your account…", func(ctx context.Context) error {
		if err := s.d.Client.DeleteAccount(ctx); err != nil {
			return err
		}
		if s.d.Store != nil {
			_ = s.d.Store.Unlink()
		}
		s.mu.Lock()
		s.me, s.friends, s.remote, s.checked = nil, nil, nil, false
		s.mu.Unlock()
		s.say("Your account and everything stored online are deleted. Your progress on this laptop is untouched.", false)
		return nil
	})
}

func (s *AccountScreen) confirmRects() (card, cancel, del rect) {
	w := math.Min(screenW-2*pgMargin, 560)
	card = rect{math.Round((screenW - w) / 2), math.Round(screenH/2 - 110), w, 220}
	dw := kitButtonW("Delete account", kitDestructive, false)
	cw := kitButtonW("Cancel", kitSecondary, false)
	y := card.y + card.h - sp6 - btnH
	del = rect{card.x + card.w - sp6 - dw, y, dw, btnH}
	cancel = rect{del.x - sp3 - cw, y, cw, btnH}
	return
}

// page lays out what the screen shows now.
func (s *AccountScreen) page() accountPage {
	var p accountPage
	p.w = math.Min(screenW-2*pgMargin, 900)
	p.x = math.Round((screenW - p.w) / 2)
	y := float64(zHeaderH) + sp2
	line := func(text string, col color.RGBA) {
		p.lines = append(p.lines, accountLine{y: y, x: p.x, text: text, col: col})
		y += uiTextH + 6
	}
	heading := func(text string) {
		y += sp3
		p.lines = append(p.lines, accountLine{y: y, x: p.x, text: text, col: colNote, big: true})
		y += sizeDisplayM + sp3
	}
	row := func(btns ...accountButton) {
		x := p.x
		for i := range btns {
			btns[i].r = rect{x, y, kitButtonW(btns[i].label, btns[i].kind, false), btnH}
			x += btns[i].r.w + sp2
			p.buttons = append(p.buttons, btns[i])
		}
		y += btnH + sp3
	}
	field := func(label, placeholder string, btns ...accountButton) {
		fw := math.Min(360, p.w*0.5)
		p.field = rect{p.x, y, fw, btnH}
		p.fieldOf = placeholder
		x := p.x + fw + sp2
		for i := range btns {
			btns[i].r = rect{x, y, kitButtonW(btns[i].label, btns[i].kind, false), btnH}
			x += btns[i].r.w + sp2
			p.buttons = append(p.buttons, btns[i])
		}
		y += btnH + sp3
		_ = label
	}

	s.mu.Lock()
	busy, msg, msgErr := s.busy, s.msg, s.msgErr
	me, friends, remote, checked := s.me, s.friends, s.remote, s.checked
	s.mu.Unlock()

	c := s.d.Client
	switch {
	case c == nil || !c.Project.Configured():
		line("Accounts aren't set up in this copy of Headliner yet.", colDim)
		line("Everything else works without one.", colDim)
	case !s.signedIn():
		line("Sign in to keep your progress in step between laptops and to add friends.", colDim)
		line("Playing never needs an account: everything works offline.", colDim)
		heading("Sign in")
		if busy != "" {
			line(busy, colNote)
			row(accountButton{label: "Cancel", kind: kitSecondary, act: func() {
				if s.cancelRun != nil {
					s.cancelRun()
				}
			}})
			break
		}
		row(accountButton{label: "Sign in with Google", kind: kitPrimary, act: s.signInGoogle})
		y += sp2
		line("Or get a sign-in link by email (open it on this laptop):", colDim)
		field("Email", "your email", accountButton{label: "Email me a link", kind: kitSecondary, act: s.signInEmail})
	default:
		acct, _ := c.SignedIn()
		line("Signed in as "+acct.Email, colNote)
		if !s.linked() {
			s.linkSection(&y, &p, line, heading, row, remote, checked, busy)
			break
		}
		link, _ := s.d.Store.Linked()
		name := ""
		if card, ok := s.d.Store.Card(link.ProfileID); ok {
			name = card.Name
		}
		status := ""
		if s.d.Syncer != nil {
			status = s.d.Syncer.Status()
		}
		if status == "" {
			status = "Syncing…"
		}
		line(name+" is linked to this account  ·  "+status, colDim)
		row(accountButton{label: "Sync now", kind: kitSecondary, act: func() {
			s.background("Syncing…", func(ctx context.Context) error {
				err := s.d.Syncer.SyncNow(ctx)
				s.refresh()
				return err
			})
		}})

		heading("Friends")
		if me != nil {
			line("Your friend code:  "+me.FriendCode, colNote)
			row(accountButton{label: "Copy my code to share", kind: kitSecondary, act: func() {
				if s.d.Copy != nil && s.d.Copy(online.ShareText(me.FriendCode, s.d.Releases)) == nil {
					s.say("Copied. Paste it to a friend: they add you with that code.", false)
				}
			}})
		}
		field("Friend code", "HL-XXXX-XXXX", accountButton{label: "Add friend", kind: kitPrimary, act: s.addFriend})
		if len(friends) == 0 {
			line("No friends yet. Swap codes with someone who has Headliner.", colDim)
		}
		for _, f := range friends {
			s.friendRow(&y, &p, f)
		}

		y += sp4
		row(accountButton{label: "Sign out", kind: kitSecondary, act: s.signOut},
			accountButton{label: "Delete account", kind: kitDestructive, act: func() { s.confirmDelete, s.confirmFocus = true, 0 }})
	}
	if busy != "" && s.signedIn() {
		line(busy, colNote)
	}
	if msg != "" {
		col := colDim
		if msgErr {
			col = colClose
		}
		line(msg, col)
	}
	return p
}

func (s *AccountScreen) signedIn() bool {
	if s.d.Client == nil {
		return false
	}
	_, ok := s.d.Client.SignedIn()
	return ok
}

// linkSection: the first sign-in on this laptop (§B3).
func (s *AccountScreen) linkSection(y *float64, p *accountPage, line func(string, color.RGBA), heading func(string),
	row func(...accountButton), remote *progress.Profile, checked bool, busy string) {
	heading("Link a profile")
	if busy != "" {
		line(busy, colNote)
		return
	}
	if !checked {
		line("Looking up your account…", colDim)
		row(accountButton{label: "Try again", kind: kitSecondary, act: s.refresh})
		return
	}
	ps := s.d.Store.Profiles()
	if len(ps) == 0 {
		line("Make a profile first (Settings → Profiles).", colDim)
		return
	}
	s.linkChoice = max(0, min(len(ps)-1, s.linkChoice))
	local := ps[s.linkChoice]
	pick := accountButton{label: "Another profile", kind: kitSecondary, act: func() { s.linkChoice = (s.linkChoice + 1) % len(ps) }}
	if remote != nil {
		line(fmt.Sprintf("Your account already has %s's progress (level %d).", remote.Name, remote.XP.Level()), colNote)
		line("Download it as a new profile on this laptop, or merge it into one you have here.", colDim)
		btns := []accountButton{
			{label: "Download as a new profile", kind: kitPrimary, act: func() { s.download(remote) }},
			{label: "Merge into " + local.Name, kind: kitSecondary, act: func() { s.link(local.ID, local.Name) }},
		}
		if len(ps) > 1 {
			btns = append(btns, pick)
		}
		row(btns...)
		return
	}
	line(fmt.Sprintf("Link %s to your account?", local.Name), colNote)
	line("Its progress goes online and stays in step on every laptop you sign in on. Other profiles here stay local.", colDim)
	btns := []accountButton{{label: "Link " + local.Name, kind: kitPrimary, act: func() { s.link(local.ID, local.Name) }}}
	if len(ps) > 1 {
		btns = append(btns, pick)
	}
	row(btns...)
}

func (s *AccountScreen) friendRow(y *float64, p *accountPage, f online.Friend) {
	r := rect{p.x, *y, p.w, 44}
	text, sub := f.Name, ""
	var btns []accountButton
	switch {
	case f.Status == "accepted" && f.Card != nil:
		c := f.Card
		sub = fmt.Sprintf("LV %d · %s · Fame %d · tier %d · %dW %dL %dD", c.Level, c.Title, c.Fame, c.Tier+1,
			c.BattleWins, c.BattleLosses, c.BattleDraws)
		btns = []accountButton{{label: "Remove", kind: kitSecondary, act: func() { s.friendAct("Removing…", f.ID, false) }},
			{label: "Block", kind: kitSecondary, act: func() { s.block(f.ID) }}}
	case f.Status == "pending" && f.Incoming:
		sub = "wants to be friends"
		btns = []accountButton{{label: "Accept", kind: kitPrimary, act: func() { s.answer(f.ID, true) }},
			{label: "Decline", kind: kitSecondary, act: func() { s.answer(f.ID, false) }},
			{label: "Block", kind: kitSecondary, act: func() { s.block(f.ID) }}}
	case f.Status == "pending":
		sub = "request sent, waiting for them"
		btns = []accountButton{{label: "Cancel request", kind: kitSecondary, act: func() { s.friendAct("Cancelling…", f.ID, false) }}}
	case f.Status == "blocked":
		sub = "blocked"
		btns = []accountButton{{label: "Unblock", kind: kitSecondary, act: func() { s.friendAct("Unblocking…", f.ID, false) }}}
	}
	p.lines = append(p.lines, accountLine{y: r.y + 2, x: r.x + sp3, text: text, col: colNote},
		accountLine{y: r.y + 2 + uiTextH + 2, x: r.x + sp3, text: sub, col: colDim})
	x := r.x + r.w
	for i := len(btns) - 1; i >= 0; i-- {
		w := kitButtonW(btns[i].label, btns[i].kind, false)
		x -= w
		btns[i].r = rect{x, r.y + (r.h-btnH)/2, w, btnH}
		x -= sp2
		p.buttons = append(p.buttons, btns[i])
	}
	*y += r.h + sp1
}

func (s *AccountScreen) signInGoogle() {
	s.background("Finish signing in in your browser…", func(ctx context.Context) error {
		if err := s.d.Client.SignInWithGoogle(ctx); err != nil {
			return err
		}
		s.afterSignIn()
		return nil
	})
}

func (s *AccountScreen) signInEmail() {
	email := strings.TrimSpace(s.field)
	s.field = ""
	s.background("Sending the link…", func(ctx context.Context) error {
		err := s.d.Client.SignInWithEmail(ctx, email, func() {
			s.mu.Lock()
			s.busy = "Check your email (" + email + ") and open the link on this laptop…"
			s.mu.Unlock()
		})
		if err != nil {
			return err
		}
		s.afterSignIn()
		return nil
	})
}

func (s *AccountScreen) afterSignIn() {
	s.say("Signed in.", false)
	s.refresh()
}

func (s *AccountScreen) link(profileID, name string) {
	acct, ok := s.d.Client.SignedIn()
	if !ok {
		return
	}
	s.background("Linking "+name+"…", func(ctx context.Context) error {
		if err := s.d.Store.Link(acct.UserID, acct.Email, profileID); err != nil {
			return err
		}
		if err := s.d.Syncer.SyncNow(ctx); err != nil {
			return err
		}
		s.say(name+" is linked and synced.", false)
		s.refresh()
		return nil
	})
}

func (s *AccountScreen) download(remote *progress.Profile) {
	acct, ok := s.d.Client.SignedIn()
	if !ok {
		return
	}
	s.background("Downloading "+remote.Name+"…", func(ctx context.Context) error {
		id, err := s.d.Store.AddProfileFrom(remote)
		if err != nil {
			return err
		}
		if err := s.d.Store.Link(acct.UserID, acct.Email, id); err != nil {
			return err
		}
		_ = s.d.Store.SetActive(id)
		if err := s.d.Syncer.SyncNow(ctx); err != nil {
			return err
		}
		s.say(remote.Name+" is on this laptop now, linked and playing.", false)
		s.refresh()
		return nil
	})
}

func (s *AccountScreen) addFriend() {
	code := s.field
	s.field = ""
	if strings.TrimSpace(code) == "" {
		s.say("Type your friend's code first, like HL-7K3Q-9PXM.", true)
		return
	}
	s.background("Sending the request…", func(ctx context.Context) error {
		res, err := s.d.Client.AddFriend(ctx, code)
		if err != nil {
			return err
		}
		switch res {
		case "sent":
			s.say("Request sent. They'll see it next time they open Account.", false)
		case "accepted":
			s.say("They'd already asked you: you're friends now.", false)
		case "already":
			s.say("You're already friends (or a request is waiting).", false)
		case "self":
			s.say("That's your own code.", true)
		default:
			s.say("No one has that code. Check it with your friend.", true)
		}
		s.refresh()
		return nil
	})
}

func (s *AccountScreen) answer(id string, accept bool) {
	s.background("Answering…", func(ctx context.Context) error {
		if err := s.d.Client.Answer(ctx, id, accept); err != nil {
			return err
		}
		s.refresh()
		return nil
	})
}

func (s *AccountScreen) friendAct(busy, id string, _ bool) {
	s.background(busy, func(ctx context.Context) error {
		if err := s.d.Client.RemoveFriend(ctx, id); err != nil {
			return err
		}
		s.refresh()
		return nil
	})
}

func (s *AccountScreen) block(id string) {
	s.background("Blocking…", func(ctx context.Context) error {
		if err := s.d.Client.Block(ctx, id); err != nil {
			return err
		}
		s.refresh()
		return nil
	})
}

func (s *AccountScreen) signOut() {
	s.background("Signing out…", func(ctx context.Context) error {
		if s.d.Syncer != nil {
			_ = s.d.Syncer.SyncNow(ctx) // send what's here first
		}
		_ = s.d.Client.SignOut(ctx)
		if s.d.Store != nil {
			_ = s.d.Store.Unlink()
		}
		s.mu.Lock()
		s.me, s.friends, s.remote, s.checked = nil, nil, nil, false
		s.mu.Unlock()
		s.say("Signed out. Your progress stays on this laptop.", false)
		return nil
	})
}

func (s *AccountScreen) Draw(dst *ebiten.Image) {
	dst.Fill(colBG)
	s.anim.tick()
	x := drawEscButton(dst, escBackChip, &s.anim, s.ptr)
	drawTextScaled(dst, "Account", x, 14, dispL, colNote)
	page := s.page()
	for _, l := range page.lines {
		if l.big {
			drawTextScaled(dst, l.text, l.x, l.y, dispM, l.col)
		} else {
			drawText(dst, truncateW(l.text, page.w), l.x, l.y, l.col)
		}
	}
	if page.field.w > 0 {
		text := s.field
		col := colNote
		if text == "" {
			text, col = page.fieldOf, colBarline
		}
		notchedFill(dst, page.field, colHover)
		drawText(dst, truncateW(text, page.field.w-2*sp3), page.field.x+sp3, page.field.y+(page.field.h-uiTextH)/2, col)
		cx := page.field.x + sp3 + math.Min(textW(s.field), page.field.w-2*sp3) + 2
		vector.FillRect(dst, float32(cx), float32(page.field.y+8), 2, float32(page.field.h-16), colNote, false)
	}
	for _, b := range page.buttons {
		drawKitButton(dst, b.r, b.label, -1, b.kind, btnState{hover: s.ptr.over(b.r)})
	}
	if s.confirmDelete {
		vector.FillRect(dst, 0, 0, float32(screenW), float32(screenH), withAlpha(colBG, 0.85), false)
		card, cancel, del := s.confirmRects()
		y := drawDialogCard(dst, card, "Delete your account?")
		for _, l := range []string{
			"This deletes your account and everything stored online: the synced",
			"progress, your friend code and your friends. Your progress on this",
			"laptop stays. This can't be undone.",
		} {
			drawText(dst, l, card.x+sp6, y, colDim)
			y += uiTextH + 4
		}
		drawKitButton(dst, cancel, "Cancel", -1, kitSecondary, btnState{hover: s.ptr.over(cancel), focus: s.confirmFocus == 0})
		drawKitButton(dst, del, "Delete account", -1, kitDestructive, btnState{hover: s.ptr.over(del), focus: s.confirmFocus == 1})
	}
}

// ShotState fills the screen for the dev screenshot tool (tools/uishot):
// "link" (signed in, nothing linked yet) or "friends".
func (s *AccountScreen) ShotState(kind string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.loading = true // no network in a screenshot
	switch kind {
	case "link":
		s.checked = true
	case "friends":
		s.me = &online.Me{FriendCode: "HL-7K3Q-9PXM"}
		lv, t, fame, tier, w, l, d := 12, "Pub Regular", 340, 2, 7, 3, 1
		s.friends = []online.Friend{
			{ID: "1", Status: "pending", Incoming: true, Name: "Priya"},
			{ID: "2", Status: "accepted", Name: "Sam", Card: &online.Card{DisplayName: "Sam", Level: lv, Title: t,
				Fame: fame, Tier: tier, BattleWins: w, BattleLosses: l, BattleDraws: d}},
			{ID: "3", Status: "pending", Name: "Jordan"},
		}
	}
}
