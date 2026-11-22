package online

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/S95F/musicTutor/internal/progress"
	"github.com/S95F/musicTutor/internal/scoring"
)

// fakeSupabase is just enough of Supabase's auth and data API.
type fakeSupabase struct {
	mu         sync.Mutex
	challenge  string
	refreshes  int
	revoked    bool
	doc        json.RawMessage
	schema     int
	card       map[string]any
	down       bool
	userID     string
	lastBearer string
}

func (f *fakeSupabase) handler(t *testing.T) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		if f.down {
			w.WriteHeader(http.StatusServiceUnavailable) // a paused project
			return
		}
		if r.Header.Get("apikey") != "anon" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		body, _ := io.ReadAll(r.Body)
		var in map[string]any
		_ = json.Unmarshal(body, &in)
		f.lastBearer = strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		tokens := func(access string) {
			json.NewEncoder(w).Encode(map[string]any{"access_token": access, "refresh_token": "rt-" + access,
				"expires_in": 3600, "user": map[string]any{"id": f.userID, "email": "alex@example.com"}})
		}
		switch {
		case r.URL.Path == "/auth/v1/token" && r.URL.Query().Get("grant_type") == "pkce":
			sum := sha256.Sum256([]byte(in["code_verifier"].(string)))
			if in["auth_code"] != "the-code" || base64.RawURLEncoding.EncodeToString(sum[:]) != f.challenge {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			tokens("a1")
		case r.URL.Path == "/auth/v1/token" && r.URL.Query().Get("grant_type") == "refresh_token":
			if f.revoked {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte(`{"error_description":"Invalid Refresh Token"}`))
				return
			}
			f.refreshes++
			tokens("a2")
		case r.URL.Path == "/auth/v1/logout":
			w.WriteHeader(http.StatusNoContent)
		case r.URL.Path == "/rest/v1/progress_docs" && r.Method == http.MethodGet:
			if f.doc == nil {
				w.Write([]byte("[]"))
				return
			}
			json.NewEncoder(w).Encode([]map[string]any{{"doc": f.doc, "schema_version": f.schema}})
		case r.URL.Path == "/rest/v1/progress_docs" && r.Method == http.MethodPost:
			b, _ := json.Marshal(in["doc"])
			f.doc, f.schema = b, int(in["schema_version"].(float64))
			w.WriteHeader(http.StatusCreated)
		case r.URL.Path == "/rest/v1/profiles" && r.Method == http.MethodPost:
			in["friend_code"] = "HL-7K3Q-9PXM"
			f.card = in
			json.NewEncoder(w).Encode([]map[string]any{in})
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL)
			w.WriteHeader(http.StatusNotFound)
		}
	})
}

func newFake(t *testing.T) (*fakeSupabase, *httptest.Server) {
	f := &fakeSupabase{userID: "user-1"}
	srv := httptest.NewServer(f.handler(t))
	t.Cleanup(srv.Close)
	return f, srv
}

// browserFor plays the system browser: it reads the sign-in URL and comes
// back to the app's callback with a code.
func browserFor(f *fakeSupabase) func(string) error {
	return func(u string) error {
		q, _ := url.Parse(u)
		f.mu.Lock()
		f.challenge = q.Query().Get("code_challenge")
		f.mu.Unlock()
		if q.Query().Get("provider") != "google" || q.Query().Get("code_challenge_method") != "s256" {
			return errors.New("bad sign-in URL " + u)
		}
		go func() {
			resp, err := http.Get(q.Query().Get("redirect_to") + "?code=the-code")
			if err == nil {
				resp.Body.Close()
			}
		}()
		return nil
	}
}

func TestSignInWithGoogleEndToEnd(t *testing.T) {
	f, srv := newFake(t)
	store := &MemoryStore{}
	c := &Client{Project: Project{URL: srv.URL, AnonKey: "anon"}, Store: store, OpenBrowser: browserFor(f)}
	if err := c.SignInWithGoogle(context.Background()); err != nil {
		t.Fatal(err)
	}
	saved, err := store.Load()
	if err != nil || saved.RefreshToken != "rt-a1" || saved.UserID != "user-1" || saved.Email != "alex@example.com" {
		t.Fatalf("saved %+v, %v", saved, err)
	}
	// A new run of the app picks the account up from the store alone.
	c2 := &Client{Project: c.Project, Store: store}
	if _, ok := c2.SignedIn(); !ok {
		t.Fatal("the saved account was not found on the next run")
	}
	if _, err := c2.session(context.Background()); err != nil || f.refreshes != 1 {
		t.Fatalf("refresh on the next run: %v (%d refreshes)", err, f.refreshes)
	}
	if s, _ := store.Load(); s.RefreshToken != "rt-a2" {
		t.Error("the rotated refresh token was not saved")
	}
	// Revoked elsewhere: signed out here too, quietly.
	f.revoked = true
	c3 := &Client{Project: c.Project, Store: store}
	if _, err := c3.session(context.Background()); !errors.Is(err, ErrNoToken) {
		t.Errorf("revoked token: %v", err)
	}
	if _, ok := c3.SignedIn(); ok {
		t.Error("a revoked account still looks signed in")
	}
}

func TestOfflineAndPausedProjectsAreJustOffline(t *testing.T) {
	f, srv := newFake(t)
	f.down = true
	c := &Client{Project: Project{URL: srv.URL, AnonKey: "anon"}, Store: &MemoryStore{}}
	c.Store.Save(Saved{RefreshToken: "rt", UserID: "user-1"})
	if _, _, err := c.GetDoc(context.Background()); !errors.Is(err, ErrOffline) {
		t.Errorf("paused project: %v", err)
	}
	srv.Close()
	if _, _, err := c.GetDoc(context.Background()); !errors.Is(err, ErrOffline) {
		t.Errorf("no network: %v", err)
	}
	if _, _, err := (&Client{}).GetDoc(context.Background()); !errors.Is(err, ErrOffline) {
		t.Errorf("no project configured: %v", err)
	}
	if _, ok := (&Client{Store: &MemoryStore{}}).SignedIn(); ok {
		t.Error("signed in with nothing saved")
	}
}

func openStore(t *testing.T, name string, xp int) (*progress.Store, string) {
	t.Helper()
	st, err := progress.OpenFile(filepath.Join(t.TempDir(), "progress.json"))
	if err != nil {
		t.Fatal(err)
	}
	id, _ := st.CreateProfile(name)
	st.AddXP(xp)
	return st, id
}

func TestTwoLaptopsSyncTheSameProfile(t *testing.T) {
	f, srv := newFake(t)
	project := Project{URL: srv.URL, AnonKey: "anon"}
	signedIn := func() *Client {
		s := &MemoryStore{}
		s.Save(Saved{RefreshToken: "rt", UserID: "user-1", Email: "alex@example.com"})
		return &Client{Project: project, Store: s}
	}

	// Laptop 1 links its profile and uploads it.
	st1, id1 := openStore(t, "Alex", 500)
	st1.Link("user-1", "alex@example.com", id1)
	s1 := NewSyncer(signedIn(), st1)
	if err := s1.SyncNow(context.Background()); err != nil {
		t.Fatal(err)
	}
	if f.card["display_name"] != "Alex" || f.card["xp"].(float64) != 500 {
		t.Errorf("the card sent: %v", f.card)
	}

	// Laptop 2 has its own progress, more XP, and chooses to merge.
	st2, id2 := openStore(t, "Alex", 800)
	s2 := NewSyncer(signedIn(), st2)
	if err := s2.SyncNow(context.Background()); !errors.Is(err, ErrNotLinked) {
		t.Fatalf("unlinked sync: %v", err)
	}
	if remote, ok, err := s2.RemoteProfile(context.Background()); !ok || err != nil || remote.XP.TotalXP != 500 {
		t.Fatalf("the account's profile as laptop 2 sees it: %v %v", ok, err)
	}
	st2.Link("user-1", "alex@example.com", id2)
	if err := s2.SyncNow(context.Background()); err != nil {
		t.Fatal(err)
	}
	// Laptop 1 picks up the merge: XP is the larger.
	if err := s1.SyncNow(context.Background()); err != nil {
		t.Fatal(err)
	}
	if c, _ := st1.Card(id1); c.XP != 800 {
		t.Errorf("laptop 1 has %d XP after syncing, want 800", c.XP)
	}
	if s1.Status() == "" || s1.Offline() {
		t.Errorf("status %q offline %v", s1.Status(), s1.Offline())
	}

	// Offline: play goes on, the status says so.
	f.mu.Lock()
	f.down = true
	f.mu.Unlock()
	st1.AddXP(10)
	if err := s1.SyncNow(context.Background()); !errors.Is(err, ErrOffline) || !s1.Offline() ||
		!strings.Contains(s1.Status(), "Offline") {
		t.Errorf("offline sync: %v, status %q", err, s1.Status())
	}
	if c, _ := st1.Card(id1); c.XP != 810 {
		t.Error("an offline sync touched the local profile")
	}
}

func TestADocFromANewerHeadlinerIsLeftAlone(t *testing.T) {
	f, srv := newFake(t)
	f.doc, f.schema = json.RawMessage(`{"name":"Alex","xp":{"totalXP":99999}}`), progress.SchemaVersion+1
	st, id := openStore(t, "Alex", 100)
	st.Link("user-1", "", id)
	ms := &MemoryStore{}
	ms.Save(Saved{RefreshToken: "rt", UserID: "user-1"})
	s := NewSyncer(&Client{Project: Project{URL: srv.URL, AnonKey: "anon"}, Store: ms}, st)
	if err := s.SyncNow(context.Background()); !errors.Is(err, progress.ErrNewerDoc) {
		t.Errorf("newer doc: %v", err)
	}
	if c, _ := st.Card(id); c.XP != 100 {
		t.Error("a newer doc was merged in")
	}
	if f.schema != progress.SchemaVersion+1 {
		t.Error("the newer doc was overwritten")
	}
}

func TestNamesAndCodes(t *testing.T) {
	for name, ok := range map[string]bool{"Alex": true, "": false, " Alex": false, "Alex ": false,
		"Twenty characters ok": true, "Twenty-one characters": false} {
		if (ValidName(name) == nil) != ok {
			t.Errorf("ValidName(%q) ok=%v", name, !ok)
		}
	}
	for in, want := range map[string]string{"hl 7k3q 9pxm": "HL-7K3Q-9PXM", "HL-7K3Q-9PXM": "HL-7K3Q-9PXM",
		"7k3q9pxm": "HL-7K3Q-9PXM"} {
		if got := NormalizeCode(in); got != want {
			t.Errorf("NormalizeCode(%q) = %q, want %q", in, got, want)
		}
	}
	long := CardFrom(progress.CardOf{Name: "  A name that is far too long  ", Level: 3, Title: scoring.Title(3)})
	if ValidName(long.DisplayName) != nil {
		t.Errorf("CardFrom made an invalid name %q", long.DisplayName)
	}
}

func TestTheCredentialManagerKeepsTheAccount(t *testing.T) {
	store := SystemStore()
	if _, ok := store.(*MemoryStore); ok {
		t.Skip("no system credential store on this OS")
	}
	cs := CredStore{Target: "Headliner/test-" + time.Now().Format("150405.000000")}
	defer cs.Clear()
	want := Saved{RefreshToken: "rt-test", UserID: "u", Email: "alex@example.com"}
	if err := cs.Save(want); err != nil {
		t.Fatal(err)
	}
	got, err := cs.Load()
	if err != nil || got != want {
		t.Fatalf("round trip: %+v, %v", got, err)
	}
	if err := cs.Clear(); err != nil {
		t.Fatal(err)
	}
	if _, err := cs.Load(); !errors.Is(err, ErrNoToken) {
		t.Errorf("after Clear: %v", err)
	}
}
