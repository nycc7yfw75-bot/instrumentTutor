//go:build !windows

package online

// SystemStore is the platform's token store: on systems other than
// Windows, the session's memory only (Headliner ships for Windows).
func SystemStore() TokenStore { return &MemoryStore{} }
