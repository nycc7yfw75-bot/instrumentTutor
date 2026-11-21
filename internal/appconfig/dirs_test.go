package appconfig

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func asBuild(t *testing.T, dev bool) {
	t.Helper()
	SetDevBuild(dev)
	t.Cleanup(func() { SetDevBuild(false) })
}

func TestTheMusictutorFolderIsCopiedNeverMoved(t *testing.T) {
	asBuild(t, false)
	base := t.TempDir()
	old := filepath.Join(base, legacyDirName)
	writeFile(t, filepath.Join(old, "progress.json"), `{"version":2}`)
	writeFile(t, filepath.Join(old, "pieces", "Sultans.gp"), "tab")

	from, err := seedDir(base)
	if err != nil || from != old {
		t.Fatalf("seedDir = %q, %v", from, err)
	}
	if got := readFile(t, filepath.Join(base, releaseDirName, "pieces", "Sultans.gp")); got != "tab" {
		t.Errorf("the copy holds %q", got)
	}
	if got := readFile(t, filepath.Join(old, "progress.json")); got != `{"version":2}` {
		t.Error("the old folder's save changed")
	}
	if _, err := os.Stat(filepath.Join(old, MigratedNote)); err != nil {
		t.Error("no note left in the old folder")
	}
	if _, err := os.Stat(filepath.Join(base, releaseDirName, MigratedNote)); err == nil {
		t.Error("the note was copied into the new folder")
	}

	// A second run leaves the (now changed) new folder alone.
	writeFile(t, filepath.Join(base, releaseDirName, "progress.json"), "newer")
	if from, _ := seedDir(base); from != "" {
		t.Errorf("copied again from %q", from)
	}
	if readFile(t, filepath.Join(base, releaseDirName, "progress.json")) != "newer" {
		t.Error("an existing Headliner folder was overwritten")
	}
}

func TestADevBuildStartsFromTheInstalledGame(t *testing.T) {
	asBuild(t, true)
	base := t.TempDir()
	writeFile(t, filepath.Join(base, legacyDirName, "progress.json"), "musictutor")
	writeFile(t, filepath.Join(base, releaseDirName, "progress.json"), "headliner")
	if _, err := seedDir(base); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, filepath.Join(base, devDirName, "progress.json")); got != "headliner" {
		t.Errorf("the dev folder started from %q, want the installed game's", got)
	}
	if _, err := os.Stat(filepath.Join(base, legacyDirName, MigratedNote)); err == nil {
		t.Error("a dev copy marked the musictutor folder as migrated")
	}
	p, _ := Path()
	if filepath.Base(filepath.Dir(p)) != devDirName {
		t.Errorf("a dev build reads %s", p)
	}
}

func TestNothingToCopyStartsEmpty(t *testing.T) {
	asBuild(t, false)
	base := t.TempDir()
	if from, err := seedDir(base); from != "" || err != nil {
		t.Errorf("seedDir on a fresh machine = %q, %v", from, err)
	}
	if _, err := os.Stat(filepath.Join(base, releaseDirName)); err == nil {
		t.Error("a folder was made with nothing to copy")
	}
}

func TestRememberedPathsFollowTheCopy(t *testing.T) {
	asBuild(t, false)
	base := t.TempDir()
	t.Setenv(EnvConfigDir, "")
	t.Setenv(LegacyEnvConfigDir, "")
	t.Setenv("AppData", base)
	t.Setenv("XDG_CONFIG_HOME", base)
	old := filepath.Join(base, legacyDirName, "pieces", "riff.gtab")
	gone := filepath.Join(base, legacyDirName, "pieces", "only-there.gtab")
	writeFile(t, old, "x")
	writeFile(t, filepath.Join(base, releaseDirName, "pieces", "riff.gtab"), "x")
	c := Config{Recents: []string{old, gone}, Pieces: map[string]PieceSetup{old: {Track: 2}}}
	c.rehomePaths()
	want := filepath.Join(base, releaseDirName, "pieces", "riff.gtab")
	if c.Recents[0] != want || c.Recents[1] != gone {
		t.Errorf("recents %q", c.Recents)
	}
	if c.Pieces[want].Track != 2 {
		t.Errorf("the piece's setup didn't follow: %+v", c.Pieces)
	}
}

func TestTheOldConfigVariableStillWorks(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(EnvConfigDir, "")
	t.Setenv(LegacyEnvConfigDir, dir)
	if p, _ := Path(); p != filepath.Join(dir, fileName) {
		t.Errorf("Path = %q with only MUSICTUTOR_CONFIG_DIR set", p)
	}
}
