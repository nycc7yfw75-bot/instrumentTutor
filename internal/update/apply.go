package update

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Installing an update (ONLINE.md §A6.4–A6.5). The verified zip is
// unpacked beside the install as <install>.new. The running app then
// hands over to a copy of ITSELF (the old, known-good version) in the
// temp folder, run as `--apply-update`, and quits. That helper waits for
// the app to close, renames <install> to <install>.prev and <install>.new
// to <install>, and starts the new version, watching for it to reach the
// landing page (it writes OKFile). If the new version fails to get there
// twice in a row, the helper puts <install>.prev back and starts it with
// RolledBackEnv set. Exactly one previous version is kept.

const (
	// ApplyFlag is the helper's first argument.
	ApplyFlag = "--apply-update"
	// OKEnv tells a freshly updated app where to write when it reaches
	// the landing page.
	OKEnv = "HEADLINER_UPDATE_OK_FILE"
	// RolledBackEnv tells the restored app which version was rolled back.
	RolledBackEnv = "HEADLINER_ROLLED_BACK"
	// ExeName is the app's exe inside the install folder.
	ExeName = "headliner.exe"

	newSuffix  = ".new"
	prevSuffix = ".prev"
	badSuffix  = ".bad"
)

// Timings the helper uses (tests shorten them).
var (
	closeWait   = 60 * time.Second // for the app to quit
	landingWait = 90 * time.Second // for the new version to reach the landing page
	tries       = 2
)

// Stage unpacks the zip into <install>.new, replacing any earlier
// attempt. Nothing may unpack outside that folder.
func Stage(zipPath, installDir string) (string, error) {
	newDir := installDir + newSuffix
	if err := os.RemoveAll(newDir); err != nil {
		return "", err
	}
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", err
	}
	defer zr.Close()
	hasExe := false
	for _, f := range zr.File {
		name := filepath.FromSlash(f.Name)
		target := filepath.Join(newDir, name)
		if !strings.HasPrefix(target, newDir+string(filepath.Separator)) {
			return "", fmt.Errorf("the update has a file outside its folder: %s", f.Name)
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return "", err
			}
			continue
		}
		if strings.EqualFold(name, ExeName) {
			hasExe = true
		}
		if err := unzipFile(f, target); err != nil {
			return "", err
		}
	}
	if !hasExe {
		_ = os.RemoveAll(newDir)
		return "", fmt.Errorf("the update has no %s", ExeName)
	}
	return newDir, nil
}

func unzipFile(f *zip.File, target string) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	out, err := os.Create(target)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, rc); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// Handoff starts the helper: a copy of the running exe in the temp
// folder. The caller quits straight after.
func Handoff(installDir, okFile string) error {
	self, err := os.Executable()
	if err != nil {
		return err
	}
	helper := filepath.Join(os.TempDir(), "headliner-updater.exe")
	_ = os.Remove(helper)
	if err := copyPath(self, helper); err != nil {
		return err
	}
	cmd := exec.Command(helper, ApplyFlag, installDir, strconv.Itoa(os.Getpid()), okFile)
	cmd.Dir = os.TempDir()
	return cmd.Start()
}

// RunHelper is the helper's whole job; args follow ApplyFlag. It returns
// a message for the log.
func RunHelper(args []string) string {
	if len(args) != 3 {
		return "update helper: want <install> <pid> <ok-file>"
	}
	installDir, okFile := args[0], args[2]
	if pid, err := strconv.Atoi(args[1]); err == nil {
		waitForExit(pid)
	}
	if err := swapIn(installDir); err != nil {
		launch(installDir, "", "")
		return "update not installed: " + err.Error()
	}
	if watchLaunch(installDir, okFile) {
		return "updated"
	}
	if err := rollBack(installDir); err != nil {
		return "the new version would not start and rolling back failed: " + err.Error()
	}
	launch(installDir, RolledBackEnv, "1")
	return "rolled back"
}

func waitForExit(pid int) {
	p, err := os.FindProcess(pid)
	if err != nil {
		return
	}
	done := make(chan struct{})
	go func() { _, _ = p.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(closeWait):
	}
}

// swapIn: <install> → <install>.prev, <install>.new → <install>. A
// running exe can hold the folder for a moment after it quits, so each
// rename is retried.
func swapIn(installDir string) error {
	newDir, prev := installDir+newSuffix, installDir+prevSuffix
	if _, err := os.Stat(newDir); err != nil {
		return fmt.Errorf("nothing staged: %w", err)
	}
	if err := retry(func() error { return os.RemoveAll(prev) }); err != nil {
		return err
	}
	if err := retry(func() error { return os.Rename(installDir, prev) }); err != nil {
		return err
	}
	if err := retry(func() error { return os.Rename(newDir, installDir) }); err != nil {
		_ = retry(func() error { return os.Rename(prev, installDir) })
		return err
	}
	carryOver(prev, installDir)
	return nil
}

// carryOver copies top-level files the installer made (its uninstaller,
// unins000.exe and .dat) from the previous version into the new one when
// the update didn't bring them.
func carryOver(prev, installDir string) {
	entries, err := os.ReadDir(prev)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !e.Type().IsRegular() || strings.EqualFold(e.Name(), ExeName) {
			continue
		}
		if _, err := os.Stat(filepath.Join(installDir, e.Name())); err == nil {
			continue
		}
		_ = copyPath(filepath.Join(prev, e.Name()), filepath.Join(installDir, e.Name()))
	}
}

func rollBack(installDir string) error {
	prev, bad := installDir+prevSuffix, installDir+badSuffix
	if _, err := os.Stat(prev); err != nil {
		return errors.New("no previous version kept")
	}
	_ = retry(func() error { return os.RemoveAll(bad) })
	if err := retry(func() error { return os.Rename(installDir, bad) }); err != nil {
		return err
	}
	return retry(func() error { return os.Rename(prev, installDir) })
}

func retry(fn func() error) error {
	var err error
	for i := 0; i < 40; i++ {
		if err = fn(); err == nil {
			return nil
		}
		time.Sleep(250 * time.Millisecond)
	}
	return err
}

// watchLaunch starts the new version up to `tries` times and reports
// whether it reached the landing page.
func watchLaunch(installDir, okFile string) bool {
	for i := 0; i < tries; i++ {
		_ = os.Remove(okFile)
		cmd := launch(installDir, OKEnv, okFile)
		if cmd == nil {
			continue
		}
		exited := make(chan struct{})
		go func() { _ = cmd.Wait(); close(exited) }()
		deadline := time.After(landingWait)
		for ok := false; !ok; {
			select {
			case <-exited:
				if fileExists(okFile) {
					return true // reached the landing page, then the player quit
				}
				ok = true
			case <-deadline:
				if fileExists(okFile) {
					return true
				}
				_ = cmd.Process.Kill()
				<-exited
				ok = true
			case <-time.After(200 * time.Millisecond):
				if fileExists(okFile) {
					return true
				}
			}
		}
	}
	return false
}

func launch(installDir, env, val string) *exec.Cmd {
	cmd := exec.Command(filepath.Join(installDir, ExeName))
	cmd.Dir = installDir
	cmd.Env = os.Environ()
	if env != "" {
		cmd.Env = append(cmd.Env, env+"="+val)
	}
	if err := cmd.Start(); err != nil {
		return nil
	}
	return cmd
}

// MarkLanded is called when the app reaches the landing page: a freshly
// updated version tells the helper it started fine.
func MarkLanded() {
	if f := os.Getenv(OKEnv); f != "" {
		_ = os.WriteFile(f, []byte(time.Now().Format(time.RFC3339)), 0o644)
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func copyPath(src, dst string) error {
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
