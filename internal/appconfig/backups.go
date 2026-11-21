package appconfig

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"
)

// Save safety (ONLINE.md §A7): before an update, a schema migration, an
// import or a restore, progress.json and config.json are copied to
// backups/<timestamp>/; the last 15 are kept.

// SaveFiles are the files a backup holds.
var SaveFiles = []string{"progress.json", fileName}

const (
	backupsDir  = "backups"
	keepBackups = 15
	stampLayout = "20060102-150405"
)

// Backup is one backup folder.
type Backup struct {
	Name   string // the folder name: <timestamp>[-<reason>]
	Time   time.Time
	Reason string
	Path   string
}

// BackupSaves copies the saves in dir to dir/backups/<timestamp>-<reason>
// and drops the oldest beyond 15. It returns the backup's path, or "" if
// there was nothing to back up.
func BackupSaves(dir, reason string) (string, error) {
	var have []string
	for _, f := range SaveFiles {
		if st, err := os.Stat(filepath.Join(dir, f)); err == nil && st.Mode().IsRegular() {
			have = append(have, f)
		}
	}
	if len(have) == 0 {
		return "", nil
	}
	name := time.Now().Format(stampLayout)
	if reason != "" {
		name += "-" + reason
	}
	dst := filepath.Join(dir, backupsDir, name)
	for i := 2; ; i++ { // two backups in one second keep apart
		if _, err := os.Stat(dst); os.IsNotExist(err) {
			break
		}
		dst = filepath.Join(dir, backupsDir, fmt.Sprintf("%s-%d", name, i))
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return "", err
	}
	for _, f := range have {
		if err := copyFile(filepath.Join(dir, f), filepath.Join(dst, f)); err != nil {
			return "", fmt.Errorf("backing up %s: %w", f, err)
		}
	}
	pruneBackups(dir)
	return dst, nil
}

// Backups lists dir's backups, newest first.
func Backups(dir string) []Backup {
	entries, err := os.ReadDir(filepath.Join(dir, backupsDir))
	if err != nil {
		return nil
	}
	var out []Backup
	for _, e := range entries {
		if !e.IsDir() || len(e.Name()) < len(stampLayout) {
			continue
		}
		t, err := time.ParseInLocation(stampLayout, e.Name()[:len(stampLayout)], time.Local)
		if err != nil {
			continue
		}
		reason := strings.TrimPrefix(e.Name()[len(stampLayout):], "-")
		out = append(out, Backup{Name: e.Name(), Time: t, Reason: reason, Path: filepath.Join(dir, backupsDir, e.Name())})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name > out[j].Name })
	return out
}

func pruneBackups(dir string) {
	bs := Backups(dir)
	for i := keepBackups; i < len(bs); i++ {
		_ = os.RemoveAll(bs[i].Path)
	}
}

// RestoreBackup puts a backup's saves back, backing up the current ones
// first.
func RestoreBackup(dir, name string) error {
	src := filepath.Join(dir, backupsDir, filepath.Base(name))
	if _, err := os.Stat(src); err != nil {
		return fmt.Errorf("no backup %s", name)
	}
	if _, err := BackupSaves(dir, "before-restore"); err != nil {
		return err
	}
	for _, f := range SaveFiles {
		from := filepath.Join(src, f)
		if _, err := os.Stat(from); err != nil {
			continue
		}
		if err := copyFile(from, filepath.Join(dir, f)); err != nil {
			return err
		}
	}
	return nil
}

// ExportSaves writes the saves in dir to a zip at path.
func ExportSaves(dir, path string) error {
	out, err := os.Create(path)
	if err != nil {
		return err
	}
	zw := zip.NewWriter(out)
	n := 0
	for _, f := range SaveFiles {
		b, err := os.ReadFile(filepath.Join(dir, f))
		if err != nil {
			continue
		}
		w, err := zw.Create(f)
		if err == nil {
			_, err = w.Write(b)
		}
		if err != nil {
			zw.Close()
			out.Close()
			return err
		}
		n++
	}
	if err := zw.Close(); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	if n == 0 {
		_ = os.Remove(path)
		return errors.New("there are no saves to export yet")
	}
	return nil
}

// ImportSaves reads an exported zip: each save in it must be a readable
// Headliner file. The current saves are backed up, then replaced.
func ImportSaves(dir, path string) error {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return fmt.Errorf("%s is not a Headliner export: %w", filepath.Base(path), err)
	}
	defer zr.Close()
	found := map[string][]byte{}
	for _, zf := range zr.File {
		name := filepath.Base(zf.Name)
		if zf.Name != name || !isSaveFile(name) {
			continue
		}
		rc, err := zf.Open()
		if err != nil {
			return err
		}
		b, err := io.ReadAll(io.LimitReader(rc, 64<<20))
		rc.Close()
		if err != nil {
			return err
		}
		var probe struct {
			Version int `json:"version"`
		}
		if err := json.Unmarshal(b, &probe); err != nil || probe.Version < 1 {
			return fmt.Errorf("%s in the export is damaged", name)
		}
		found[name] = b
	}
	if found["progress.json"] == nil {
		return fmt.Errorf("%s has no progress in it", filepath.Base(path))
	}
	if _, err := BackupSaves(dir, "before-import"); err != nil {
		return err
	}
	for name, b := range found {
		if err := writeAtomic(filepath.Join(dir, name), b); err != nil {
			return err
		}
	}
	return nil
}

func isSaveFile(name string) bool { return slices.Contains(SaveFiles, name) }

func writeAtomic(path string, b []byte) error {
	tmp := path + ".importing"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// QuarantineConfig moves an unreadable config.json aside to
// config.json.bad-<timestamp> so a fresh one can be saved without losing
// it; it returns the new name.
func QuarantineConfig() (string, error) {
	p, err := Path()
	if err != nil {
		return "", err
	}
	bad := p + ".bad-" + time.Now().Format(stampLayout)
	return bad, os.Rename(p, bad)
}
