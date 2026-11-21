package ui

// BuildInfo is what the app was built as (ONLINE.md §A4): the About
// screen shows it, the wordmark carries a DEV pill on dev builds, and the
// updater reads the channel and repo.
type BuildInfo struct {
	Version string // "v1.2.0", or "dev"
	Channel string // "stable" or "beta"; "" on a dev build
	Repo    string // owner/name the releases come from
	Dev     bool
}

// Build is set once at start-up.
var Build = BuildInfo{Version: "dev", Dev: true}
