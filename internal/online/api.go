package online

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Card is what friends see (ONLINE.md §B5): never song libraries or
// practice history.
type Card struct {
	DisplayName  string `json:"display_name"`
	Character    int    `json:"character"`
	Level        int    `json:"level"`
	Title        string `json:"title"`
	XP           int    `json:"xp"`
	Fame         int    `json:"fame"`
	Tier         int    `json:"tier"`
	BattleWins   int    `json:"battle_wins"`
	BattleLosses int    `json:"battle_losses"`
	BattleDraws  int    `json:"battle_draws"`
}

// Me is the signed-in account's own row.
type Me struct {
	Card
	ID         string `json:"id"`
	FriendCode string `json:"friend_code"`
}

// ValidName checks a display name: 1–20 characters, no spaces at either end.
func ValidName(s string) error {
	n := len([]rune(s))
	switch {
	case strings.TrimSpace(s) != s:
		return errors.New("no spaces at the start or end of a name")
	case n == 0:
		return errors.New("a name can't be empty")
	case n > 20:
		return errors.New("a name can be at most 20 characters")
	}
	return nil
}

// PutCard creates or updates the account's card; the server keeps the
// friend code.
func (c *Client) PutCard(ctx context.Context, card Card) (Me, error) {
	if err := ValidName(card.DisplayName); err != nil {
		return Me{}, err
	}
	s, err := c.session(ctx)
	if err != nil {
		return Me{}, err
	}
	row := struct {
		ID string `json:"id"`
		Card
	}{s.UserID, card}
	var out []Me
	err = c.do(ctx, http.MethodPost, "/rest/v1/profiles?on_conflict=id", s.AccessToken, row, &out,
		http.Header{"Prefer": {"resolution=merge-duplicates,return=representation"}})
	if err != nil {
		return Me{}, err
	}
	if len(out) == 0 {
		return Me{}, errors.New("the card was not saved")
	}
	return out[0], nil
}

// MyCard reads the account's own row (ok false before the first PutCard).
func (c *Client) MyCard(ctx context.Context) (Me, bool, error) {
	s, err := c.session(ctx)
	if err != nil {
		return Me{}, false, err
	}
	var out []Me
	if err := c.do(ctx, http.MethodGet, "/rest/v1/profiles?select=*&id=eq."+url.QueryEscape(s.UserID),
		s.AccessToken, nil, &out, nil); err != nil {
		return Me{}, false, err
	}
	if len(out) == 0 {
		return Me{}, false, nil
	}
	return out[0], true, nil
}

// Doc is the synced progress document.
type Doc struct {
	Doc           json.RawMessage `json:"doc"`
	SchemaVersion int             `json:"schema_version"`
	UpdatedAt     time.Time       `json:"updated_at"`
}

// GetDoc reads the account's progress (ok false when there is none yet).
func (c *Client) GetDoc(ctx context.Context) (Doc, bool, error) {
	s, err := c.session(ctx)
	if err != nil {
		return Doc{}, false, err
	}
	var out []Doc
	if err := c.do(ctx, http.MethodGet, "/rest/v1/progress_docs?select=doc,schema_version,updated_at&user_id=eq."+
		url.QueryEscape(s.UserID), s.AccessToken, nil, &out, nil); err != nil {
		return Doc{}, false, err
	}
	if len(out) == 0 {
		return Doc{}, false, nil
	}
	return out[0], true, nil
}

// PutDoc saves the account's progress.
func (c *Client) PutDoc(ctx context.Context, doc json.RawMessage, schema int) error {
	s, err := c.session(ctx)
	if err != nil {
		return err
	}
	row := map[string]any{"user_id": s.UserID, "doc": doc, "schema_version": schema,
		"updated_at": c.now().UTC().Format(time.RFC3339)}
	return c.do(ctx, http.MethodPost, "/rest/v1/progress_docs?on_conflict=user_id", s.AccessToken, row, nil,
		http.Header{"Prefer": {"resolution=merge-duplicates,return=minimal"}})
}

// Friend is one connection: an accepted friend (with their card), a
// request either way, or someone you blocked.
type Friend struct {
	ID       string `json:"other"`
	Status   string `json:"status"` // pending, accepted, blocked
	Incoming bool   `json:"incoming"`
	Name     string `json:"display_name"`
	Card     *Card  `json:"-"`

	Character    *int    `json:"character"`
	Level        *int    `json:"level"`
	Title        *string `json:"title"`
	Fame         *int    `json:"fame"`
	Tier         *int    `json:"tier"`
	BattleWins   *int    `json:"battle_wins"`
	BattleLosses *int    `json:"battle_losses"`
	BattleDraws  *int    `json:"battle_draws"`
}

func deref[T any](p *T) T {
	var z T
	if p == nil {
		return z
	}
	return *p
}

// Friends lists the account's connections.
func (c *Client) Friends(ctx context.Context) ([]Friend, error) {
	var out []Friend
	if err := c.rpc(ctx, "friend_list", map[string]any{}, &out); err != nil {
		return nil, err
	}
	for i := range out {
		f := &out[i]
		if f.Status == "accepted" {
			f.Card = &Card{DisplayName: f.Name, Character: deref(f.Character), Level: deref(f.Level),
				Title: deref(f.Title), Fame: deref(f.Fame), Tier: deref(f.Tier), BattleWins: deref(f.BattleWins),
				BattleLosses: deref(f.BattleLosses), BattleDraws: deref(f.BattleDraws)}
		}
	}
	return out, nil
}

// AddFriend sends a request to a friend code: "sent", "accepted" (they
// had asked you), "already", "self" or "not_found".
func (c *Client) AddFriend(ctx context.Context, code string) (string, error) {
	var out string
	err := c.rpc(ctx, "request_friend", map[string]any{"code": NormalizeCode(code)}, &out)
	return out, err
}

// Answer accepts or declines a request someone sent.
func (c *Client) Answer(ctx context.Context, other string, accept bool) error {
	return c.rpc(ctx, "respond_friend", map[string]any{"other": other, "accept": accept}, nil)
}

// RemoveFriend unfriends, cancels a request, or unblocks.
func (c *Client) RemoveFriend(ctx context.Context, other string) error {
	return c.rpc(ctx, "remove_friend", map[string]any{"other": other}, nil)
}

// Block ends a friendship or request and stops new ones.
func (c *Client) Block(ctx context.Context, other string) error {
	return c.rpc(ctx, "block_user", map[string]any{"other": other}, nil)
}

// DeleteAccount removes the account and everything stored for it online;
// local saves stay.
func (c *Client) DeleteAccount(ctx context.Context) error {
	if err := c.rpc(ctx, "delete_my_account", map[string]any{}, nil); err != nil {
		return err
	}
	c.forget()
	return nil
}

func (c *Client) rpc(ctx context.Context, fn string, args, out any) error {
	s, err := c.session(ctx)
	if err != nil {
		return err
	}
	return c.post(ctx, "/rest/v1/rpc/"+fn, s.AccessToken, args, out)
}

// NormalizeCode tidies a typed friend code: "hl 7k3q 9pxm" → "HL-7K3Q-9PXM".
func NormalizeCode(s string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(s) {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	t := strings.TrimPrefix(b.String(), "HL")
	if len(t) != 8 {
		return strings.ToUpper(strings.TrimSpace(s))
	}
	return "HL-" + t[:4] + "-" + t[4:]
}

// ShareText is what "Share my code" copies.
func ShareText(code, releases string) string {
	return "Add me on Headliner: " + code + " — download: " + releases
}
