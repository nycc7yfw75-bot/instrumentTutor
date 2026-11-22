package online

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/S95F/musicTutor/internal/progress"
)

// Sync (ONLINE.md §B4): the linked profile goes up and comes down on
// launch, a few seconds after each save (a pass, a gig, a battle), and on
// exit, always in the background. Merges follow progress.MergeProfiles
// and are backed up first. Nothing here ever blocks play.

// SyncDelay is how long after a save the sync waits for more changes.
var SyncDelay = 5 * time.Second

// Syncer keeps the linked profile in step with the account.
type Syncer struct {
	C  *Client
	St *progress.Store

	kick chan struct{}
	run  sync.Mutex // one sync at a time

	mu      sync.Mutex
	status  string
	offline bool
	last    time.Time
	onDone  func() // tells the UI the profile may have changed
}

// NewSyncer makes a syncer and hooks it to the store's saves.
func NewSyncer(c *Client, st *progress.Store) *Syncer {
	s := &Syncer{C: c, St: st, kick: make(chan struct{}, 1)}
	if st != nil {
		st.OnSave(s.Kick)
	}
	return s
}

// OnDone sets a function run after each sync that changed the profile.
func (s *Syncer) OnDone(fn func()) { s.mu.Lock(); s.onDone = fn; s.mu.Unlock() }

// Kick asks for a sync soon.
func (s *Syncer) Kick() {
	select {
	case s.kick <- struct{}{}:
	default:
	}
}

// Run syncs once now, then whenever kicked (debounced), until ctx ends.
func (s *Syncer) Run(ctx context.Context) {
	s.syncLogged(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.kick:
		}
		// Gather a burst of saves into one sync.
		t := time.NewTimer(SyncDelay)
	wait:
		for {
			select {
			case <-ctx.Done():
				t.Stop()
				return
			case <-s.kick:
			case <-t.C:
				break wait
			}
		}
		s.syncLogged(ctx)
	}
}

func (s *Syncer) syncLogged(ctx context.Context) {
	sctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	_ = s.SyncNow(sctx)
}

// Flush syncs one last time (on exit), giving up after a few seconds.
func (s *Syncer) Flush() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = s.SyncNow(ctx)
}

// Status is a line for the Account screen.
func (s *Syncer) Status() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.status != "" {
		return s.status
	}
	if !s.last.IsZero() {
		return "Synced " + ago(time.Since(s.last))
	}
	return ""
}

// Offline reports the last sync could not reach the server.
func (s *Syncer) Offline() bool { s.mu.Lock(); defer s.mu.Unlock(); return s.offline }

func (s *Syncer) setStatus(st string, offline bool) {
	s.mu.Lock()
	s.status, s.offline = st, offline
	s.mu.Unlock()
}

// ErrNotLinked: signed in, but no profile here is linked yet.
var ErrNotLinked = errors.New("no profile is linked to your account yet")

// SyncNow syncs the linked profile: download, merge, upload, and the
// friends' card.
func (s *Syncer) SyncNow(ctx context.Context) error {
	if s.St == nil {
		return nil
	}
	s.run.Lock()
	defer s.run.Unlock()
	acct, signed := s.C.SignedIn()
	link, linked := s.St.Linked()
	switch {
	case !signed:
		s.setStatus("", false)
		return ErrNoToken
	case !linked || link.UserID != acct.UserID:
		s.setStatus("", false)
		return ErrNotLinked
	}
	changed, err := s.sync(ctx, link)
	switch {
	case errors.Is(err, ErrOffline):
		s.setStatus("Offline — will sync later", true)
	case errors.Is(err, ErrNoToken):
		s.setStatus("Signed out: sign in again to sync", false)
	case err != nil:
		s.setStatus("Sync problem: "+err.Error(), false)
	default:
		now := time.Now()
		s.St.MarkSynced(now)
		s.mu.Lock()
		s.status, s.offline, s.last = "", false, now
		done := s.onDone
		s.mu.Unlock()
		if changed && done != nil {
			done()
		}
	}
	return err
}

func (s *Syncer) sync(ctx context.Context, link progress.AccountLink) (bool, error) {
	changed := false
	doc, found, err := s.C.GetDoc(ctx)
	if err != nil {
		return false, err
	}
	if found {
		remote, err := progress.DecodeProfile(doc.Doc, doc.SchemaVersion)
		if err != nil {
			return false, err
		}
		if changed, err = s.St.ApplyRemote(link.ProfileID, remote); err != nil {
			return false, err
		}
	}
	mine, err := s.St.ProfileJSON(link.ProfileID)
	if err != nil {
		return changed, err
	}
	if err := s.C.PutDoc(ctx, mine, progress.SchemaVersion); err != nil {
		return changed, err
	}
	if card, ok := s.St.Card(link.ProfileID); ok {
		if _, err := s.C.PutCard(ctx, CardFrom(card)); err != nil {
			return changed, err
		}
	}
	return changed, nil
}

// CardFrom turns a local profile's card into the online one (a name that
// is too long or padded is tidied, never refused).
func CardFrom(c progress.CardOf) Card {
	n := strings.TrimSpace(c.Name)
	if r := []rune(n); len(r) > 20 {
		n = strings.TrimSpace(string(r[:20]))
	}
	if n == "" {
		n = "Player"
	}
	return Card{DisplayName: n, Character: c.Character, Level: c.Level, Title: c.Title, XP: c.XP,
		Fame: c.Fame, Tier: c.Tier, BattleWins: c.Wins, BattleLosses: c.Losses, BattleDraws: c.Draws}
}

// RemoteProfile reads the account's synced profile, if any (the second
// device's choice: download it or merge it).
func (s *Syncer) RemoteProfile(ctx context.Context) (*progress.Profile, bool, error) {
	doc, found, err := s.C.GetDoc(ctx)
	if err != nil || !found {
		return nil, false, err
	}
	p, err := progress.DecodeProfile(doc.Doc, doc.SchemaVersion)
	return p, err == nil, err
}

func ago(d time.Duration) string {
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%d min ago", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("%d h ago", int(d.Hours()))
	}
	return fmt.Sprintf("%d days ago", int(d.Hours()/24))
}
