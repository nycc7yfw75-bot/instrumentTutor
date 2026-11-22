package online

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Signing in (ONLINE.md §B2): OAuth with PKCE. The app listens on
// 127.0.0.1 (one of CallbackPorts), opens the system browser at
// Supabase's sign-in page, and gets the code back on that listener; the
// code and the secret verifier are exchanged for a session. The email
// link comes back the same way. The ports are fixed (not random) because
// Supabase checks the return address against an exact allow-list
// (docs/SUPABASE_SETUP.md step 4).
var CallbackPorts = []int{47821, 47822, 47823}

// ErrOffline is any failure to reach the project: no network, or the free
// project paused. The game carries on and says "Offline".
var ErrOffline = errors.New("offline")

// Session is a signed-in account.
type Session struct {
	AccessToken  string
	RefreshToken string
	Expires      time.Time
	UserID       string
	Email        string
}

// Client talks to one Supabase project.
type Client struct {
	Project Project
	HTTP    *http.Client
	Store   TokenStore
	// OpenBrowser opens a URL in the system browser.
	OpenBrowser func(string) error
	Now         func() time.Time

	mu   sync.Mutex
	sess *Session
}

func (c *Client) now() time.Time {
	if c.Now != nil {
		return c.Now()
	}
	return time.Now()
}

func (c *Client) http() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return &http.Client{Timeout: 20 * time.Second}
}

// SignedIn is the account, if any (from this run or the saved token).
func (c *Client) SignedIn() (Saved, bool) {
	c.mu.Lock()
	if c.sess != nil {
		s := Saved{RefreshToken: c.sess.RefreshToken, UserID: c.sess.UserID, Email: c.sess.Email}
		c.mu.Unlock()
		return s, true
	}
	c.mu.Unlock()
	if c.Store == nil {
		return Saved{}, false
	}
	s, err := c.Store.Load()
	return s, err == nil
}

// pkce is a verifier and its S256 challenge.
func pkce() (verifier, challenge string) {
	b := make([]byte, 48)
	_, _ = rand.Read(b)
	verifier = base64.RawURLEncoding.EncodeToString(b)
	sum := sha256.Sum256([]byte(verifier))
	return verifier, base64.RawURLEncoding.EncodeToString(sum[:])
}

// listen opens the first free callback port.
func listen() (net.Listener, string, error) {
	var last error
	for _, p := range CallbackPorts {
		l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", p))
		if err == nil {
			return l, fmt.Sprintf("http://127.0.0.1:%d/callback", p), nil
		}
		last = err
	}
	return nil, "", fmt.Errorf("no free port to sign in on (%v)", last)
}

const callbackPage = `<!doctype html><meta charset="utf-8"><title>Headliner</title>
<body style="background:#141418;color:#eee;font:18px sans-serif;text-align:center;padding-top:80px">
<h1 style="color:#f2a541">%s</h1><p>%s</p></body>`

// waitForCode serves the callback until a code (or an error) arrives.
func waitForCode(ctx context.Context, l net.Listener) (string, error) {
	type result struct {
		code string
		err  error
	}
	got := make(chan result, 1)
	srv := &http.Server{ReadHeaderTimeout: 10 * time.Second}
	srv.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/callback" {
			http.NotFound(w, r)
			return
		}
		q := r.URL.Query()
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if code := q.Get("code"); code != "" {
			fmt.Fprintf(w, callbackPage, "You're signed in", "You can close this tab and go back to Headliner.")
			select {
			case got <- result{code: code}:
			default:
			}
			return
		}
		msg := q.Get("error_description")
		if msg == "" {
			msg = q.Get("error")
		}
		if msg == "" {
			msg = "the sign-in page came back without a code"
		}
		fmt.Fprintf(w, callbackPage, "Sign-in didn't work", "Go back to Headliner and try again.")
		select {
		case got <- result{err: errors.New(msg)}:
		default:
		}
	})
	go func() { _ = srv.Serve(l) }()
	defer srv.Close()
	select {
	case r := <-got:
		return r.code, r.err
	case <-ctx.Done():
		return "", errors.New("sign-in timed out")
	}
}

// SignInWithGoogle runs the browser sign-in and waits (up to ten minutes)
// for it to finish.
func (c *Client) SignInWithGoogle(ctx context.Context) error {
	l, redirect, err := listen()
	if err != nil {
		return err
	}
	verifier, challenge := pkce()
	q := url.Values{"provider": {"google"}, "redirect_to": {redirect},
		"code_challenge": {challenge}, "code_challenge_method": {"s256"}}
	if c.OpenBrowser == nil {
		l.Close()
		return errors.New("no browser to sign in with")
	}
	if err := c.OpenBrowser(c.Project.URL + "/auth/v1/authorize?" + q.Encode()); err != nil {
		l.Close()
		return err
	}
	return c.finish(ctx, l, verifier)
}

// SignInWithEmail sends a sign-in link to email, then waits (up to ten
// minutes) for it to be opened on this computer.
func (c *Client) SignInWithEmail(ctx context.Context, email string, sent func()) error {
	email = strings.TrimSpace(email)
	if !strings.Contains(email, "@") {
		return errors.New("that doesn't look like an email address")
	}
	l, redirect, err := listen()
	if err != nil {
		return err
	}
	verifier, challenge := pkce()
	body := map[string]any{"email": email, "create_user": true,
		"code_challenge": challenge, "code_challenge_method": "s256"}
	if err := c.post(ctx, "/auth/v1/otp?redirect_to="+url.QueryEscape(redirect), "", body, nil); err != nil {
		l.Close()
		return err
	}
	if sent != nil {
		sent()
	}
	return c.finish(ctx, l, verifier)
}

func (c *Client) finish(ctx context.Context, l net.Listener, verifier string) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	code, err := waitForCode(ctx, l)
	if err != nil {
		return err
	}
	var tok tokenResponse
	if err := c.post(ctx, "/auth/v1/token?grant_type=pkce", "",
		map[string]string{"auth_code": code, "code_verifier": verifier}, &tok); err != nil {
		return err
	}
	return c.adopt(tok)
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	User         struct {
		ID    string `json:"id"`
		Email string `json:"email"`
	} `json:"user"`
}

func (c *Client) adopt(tok tokenResponse) error {
	if tok.AccessToken == "" || tok.RefreshToken == "" || tok.User.ID == "" {
		return errors.New("the sign-in answer was incomplete")
	}
	s := &Session{AccessToken: tok.AccessToken, RefreshToken: tok.RefreshToken, UserID: tok.User.ID,
		Email: tok.User.Email, Expires: c.now().Add(time.Duration(tok.ExpiresIn) * time.Second)}
	c.mu.Lock()
	c.sess = s
	c.mu.Unlock()
	if c.Store != nil {
		return c.Store.Save(Saved{RefreshToken: s.RefreshToken, UserID: s.UserID, Email: s.Email})
	}
	return nil
}

// session returns a live session, refreshing it when it is about to
// expire (refresh tokens rotate: the new one is saved).
func (c *Client) session(ctx context.Context) (*Session, error) {
	if !c.Project.Configured() {
		return nil, ErrOffline
	}
	c.mu.Lock()
	s := c.sess
	c.mu.Unlock()
	if s != nil && c.now().Before(s.Expires.Add(-time.Minute)) {
		return s, nil
	}
	refresh := ""
	if s != nil {
		refresh = s.RefreshToken
	} else if c.Store != nil {
		saved, err := c.Store.Load()
		if err != nil {
			return nil, ErrNoToken
		}
		refresh = saved.RefreshToken
	}
	if refresh == "" {
		return nil, ErrNoToken
	}
	var tok tokenResponse
	err := c.post(ctx, "/auth/v1/token?grant_type=refresh_token", "", map[string]string{"refresh_token": refresh}, &tok)
	var he *HTTPError
	if errors.As(err, &he) && (he.Status == http.StatusBadRequest || he.Status == http.StatusUnauthorized) {
		// The token was revoked (signed out elsewhere, account deleted).
		c.forget()
		return nil, ErrNoToken
	}
	if err != nil {
		return nil, err
	}
	if err := c.adopt(tok); err != nil {
		return nil, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.sess, nil
}

func (c *Client) forget() {
	c.mu.Lock()
	c.sess = nil
	c.mu.Unlock()
	if c.Store != nil {
		_ = c.Store.Clear()
	}
}

// SignOut ends the session here and on the server.
func (c *Client) SignOut(ctx context.Context) error {
	if s, err := c.session(ctx); err == nil {
		_ = c.post(ctx, "/auth/v1/logout", s.AccessToken, nil, nil)
	}
	c.forget()
	return nil
}

// HTTPError is an answer other than success.
type HTTPError struct {
	Status int
	Msg    string
}

func (e *HTTPError) Error() string { return fmt.Sprintf("the server said %d: %s", e.Status, e.Msg) }

// do sends a request; a network failure or an unavailable project is
// ErrOffline.
func (c *Client) do(ctx context.Context, method, path, token string, body any, out any, extra http.Header) error {
	if !c.Project.Configured() {
		return ErrOffline
	}
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.Project.URL+path, rd)
	if err != nil {
		return err
	}
	req.Header.Set("apikey", c.Project.AnonKey)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range extra {
		req.Header[k] = v
	}
	resp, err := c.http().Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrOffline, err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	switch {
	case resp.StatusCode >= 500:
		// 5xx, including a paused free project: try again later.
		return fmt.Errorf("%w: the server answered %d", ErrOffline, resp.StatusCode)
	case resp.StatusCode >= 300:
		var e struct {
			Msg     string `json:"msg"`
			Message string `json:"message"`
			Error   string `json:"error_description"`
		}
		_ = json.Unmarshal(data, &e)
		msg := e.Msg + e.Message + e.Error
		if msg == "" {
			msg = strings.TrimSpace(string(data))
		}
		return &HTTPError{Status: resp.StatusCode, Msg: msg}
	}
	if out != nil && len(bytes.TrimSpace(data)) > 0 {
		return json.Unmarshal(data, out)
	}
	return nil
}

func (c *Client) post(ctx context.Context, path, token string, body, out any) error {
	return c.do(ctx, http.MethodPost, path, token, body, out, nil)
}
