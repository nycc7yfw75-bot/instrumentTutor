// Package online is Headliner's account side (ONLINE.md Part B): sign-in
// through Supabase, the synced profile, and friends. Everything here is
// optional: with no project configured, offline, or signed out, the game
// plays exactly as before.
package online

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// Set at build time by the release workflow:
//
//	-X github.com/S95F/musicTutor/internal/online.projectURL=https://….supabase.co
//	-X github.com/S95F/musicTutor/internal/online.anonKey=…
//
// The anon (publishable) key is public by design; the database's row
// rules are what protect the data. The service_role key never appears
// in the app.
var (
	projectURL = ""
	anonKey    = ""
)

// Project is the Supabase project the app talks to.
type Project struct {
	URL     string `json:"url"`
	AnonKey string `json:"anonKey"`
}

// Configured reports a usable project.
func (p Project) Configured() bool {
	ok := strings.HasPrefix(p.URL, "https://") || strings.HasPrefix(p.URL, "http://127.0.0.1") ||
		strings.HasPrefix(p.URL, "http://localhost") // a local Supabase, and tests
	return ok && p.AnonKey != ""
}

// LoadProject finds the project: built into a release, else (dev builds)
// HEADLINER_SUPABASE_URL / HEADLINER_SUPABASE_ANON_KEY, else online.json in
// the save folder.
func LoadProject(saveDir string) Project {
	if projectURL != "" && anonKey != "" {
		return Project{URL: baseURL(projectURL), AnonKey: anonKey}
	}
	if u, k := os.Getenv("HEADLINER_SUPABASE_URL"), os.Getenv("HEADLINER_SUPABASE_ANON_KEY"); u != "" && k != "" {
		return Project{URL: baseURL(u), AnonKey: k}
	}
	var p Project
	if b, err := os.ReadFile(filepath.Join(saveDir, "online.json")); err == nil {
		_ = json.Unmarshal(b, &p)
		p.URL = baseURL(p.URL)
	}
	return p
}

// baseURL is the project's address with any trailing "/" or the API path
// people copy from the dashboard ("…supabase.co/rest/v1/") taken off.
func baseURL(u string) string {
	u = strings.TrimRight(strings.TrimSpace(u), "/")
	u = strings.TrimSuffix(u, "/rest/v1")
	return strings.TrimRight(u, "/")
}
