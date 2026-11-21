package appconfig

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Where Headliner keeps its files (ONLINE.md §A2–A3). Installed release
// builds use %APPDATA%\headliner; builds made with a plain `go build` are
// dev builds and use %APPDATA%\headliner-dev, so developing never touches
// the saves the owner plays with. HEADLINER_CONFIG_DIR (or the old
// MUSICTUTOR_CONFIG_DIR) overrides both.

const (
	releaseDirName = "headliner"
	devDirName     = "headliner-dev"
	// MigratedNote is left in the musictutor folder once it is copied.
	MigratedNote  = "MIGRATED_TO_HEADLINER.txt"
	copyingSuffix = ".copying"
)

var dirName = releaseDirName

// SetDevBuild picks the dev folder (true) or the release one. Call it
// once, before anything reads the config.
func SetDevBuild(dev bool) {
	if dev {
		dirName = devDirName
	} else {
		dirName = releaseDirName
	}
}

// IsDevBuild reports the dev folder is in use.
func IsDevBuild() bool { return dirName == devDirName }

func envDir() string {
	if d := os.Getenv(EnvConfigDir); d != "" {
		return d
	}
	return os.Getenv(LegacyEnvConfigDir)
}

// Dir is the folder config.json, progress.json and pieces live in.
func Dir() (string, error) {
	p, err := Path()
	if err != nil {
		return "", err
	}
	return filepath.Dir(p), nil
}

// seedSources are the folders a missing one is copied from, best first:
// a dev folder starts as a copy of the installed game's, else of the old
// musictutor folder.
func seedSources() []string {
	if dirName == devDirName {
		return []string{releaseDirName, legacyDirName}
	}
	return []string{legacyDirName}
}

// Prepare readies the folder before anything reads it: the old
// guitartutor folder is renamed to musictutor, and a missing Headliner
// folder is filled with a verified copy of an earlier one. The earlier
// folder is never changed (only a note is left in it) and never deleted.
// It returns the folder copied from, or "".
func Prepare() (copiedFrom string, err error) {
	if envDir() != "" {
		return "", nil
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("appconfig: locating user config dir: %w", err)
	}
	migrateOldDir(base)
	return seedDir(base)
}

func seedDir(base string) (string, error) {
	dst := filepath.Join(base, dirName)
	if _, err := os.Stat(dst); err == nil {
		return "", nil
	}
	for _, name := range seedSources() {
		src := filepath.Join(base, name)
		if st, err := os.Stat(src); err != nil || !st.IsDir() {
			continue
		}
		tmp := dst + copyingSuffix
		_ = os.RemoveAll(tmp) // a copy cut short last time
		// If the copy fails, keep using the old folder rather than start
		// with nothing.
		fail := func(err error) (string, error) {
			_ = os.RemoveAll(tmp)
			_ = os.Setenv(EnvConfigDir, src)
			return "", err
		}
		if err := copyTree(src, tmp); err != nil {
			return fail(fmt.Errorf("appconfig: copying %s: %w", src, err))
		}
		if err := sameTree(src, tmp); err != nil {
			return fail(fmt.Errorf("appconfig: checking the copy of %s: %w", src, err))
		}
		if err := os.Rename(tmp, dst); err != nil {
			return fail(err)
		}
		if name == legacyDirName && dirName == releaseDirName {
			note := "Your musicTutor files were copied to " + dst + " for Headliner.\r\n" +
				"Nothing here was changed or deleted. Once Headliner has your progress, you may delete this folder.\r\n"
			_ = os.WriteFile(filepath.Join(src, MigratedNote), []byte(note), 0o644)
		}
		return src, nil
	}
	return "", nil
}

// skipCopy leaves out the migration note and half-made copies.
func skipCopy(rel string) bool {
	return rel == MigratedNote || strings.HasSuffix(rel, copyingSuffix)
}

// copyTree copies every regular file and folder under src into dst.
func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		if rel != "." && skipCopy(rel) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		target := filepath.Join(dst, rel)
		switch {
		case d.IsDir():
			return os.MkdirAll(target, 0o755)
		case d.Type().IsRegular():
			return copyFile(path, target)
		}
		return nil // links and the like stay behind
	})
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// sameTree checks, file by file, that dst holds what src holds.
func sameTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		if rel == "." || !d.Type().IsRegular() || skipCopy(rel) {
			if d.IsDir() && rel != "." && skipCopy(rel) {
				return filepath.SkipDir
			}
			return nil
		}
		a, err := fileSum(path)
		if err != nil {
			return err
		}
		b, err := fileSum(filepath.Join(dst, rel))
		if err != nil {
			return err
		}
		if !bytes.Equal(a, b) {
			return fmt.Errorf("%s differs after copying", rel)
		}
		return nil
	})
}

func fileSum(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return nil, err
	}
	return h.Sum(nil), nil
}
