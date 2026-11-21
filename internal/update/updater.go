package update

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// ReleasePublicKey verifies every update. Its private half is only in the
// repo's Actions secrets (HEADLINER_SIGNING_KEY). Generated with
// `go run ./tools/releasesign -keygen`.
const ReleasePublicKey = "RWRQW2eI4xDw5RfNTV9MbvVExgrYrzTnP+ourNlThtzn5vgtKXUw1juD"

// Updater checks for and installs updates for one install.
type Updater struct {
	Client     Client
	Current    Version
	Channel    string
	Key        PublicKey
	InstallDir string // the folder headliner.exe runs from
	TempDir    string // downloads; default os.TempDir()
}

// Check finds the newest release this install may update to.
func (u Updater) Check(ctx context.Context) (Release, bool, error) {
	rs, err := u.Client.Releases(ctx)
	if err != nil {
		return Release{}, false, err
	}
	r, ok := Newest(rs, u.Current, u.Channel)
	return r, ok, nil
}

// Progress reports a step ("Downloading", "Checking", "Backing up",
// "Unpacking") and, while downloading, bytes done of total.
type Progress func(step string, done, total int64)

// Prepare downloads r's zip and its signed checksums, verifies them, runs
// backup, and unpacks the zip beside the install. After it returns nil
// the caller hands over (Handoff) and quits. Any mismatch deletes what
// was downloaded.
func (u Updater) Prepare(ctx context.Context, r Release, backup func() error, progress Progress) error {
	if progress == nil {
		progress = func(string, int64, int64) {}
	}
	v, err := r.Version()
	if err != nil {
		return err
	}
	zipA, ok1 := r.Asset(ZipName(v))
	sumsA, ok2 := r.Asset(SumsName)
	sigA, ok3 := r.Asset(SigName)
	if !ok1 || !ok2 || !ok3 {
		return fmt.Errorf("release %s is missing its files", r.Tag)
	}
	tmpBase := u.TempDir
	if tmpBase == "" {
		tmpBase = os.TempDir()
	}
	dir, err := os.MkdirTemp(tmpBase, "headliner-update-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)

	sumsPath, sigPath, zipPath := filepath.Join(dir, SumsName), filepath.Join(dir, SigName), filepath.Join(dir, zipA.Name)
	progress("Downloading", 0, zipA.Size)
	if err := u.Client.Download(ctx, sumsA.URL, sumsPath, nil); err != nil {
		return err
	}
	if err := u.Client.Download(ctx, sigA.URL, sigPath, nil); err != nil {
		return err
	}
	if err := u.Client.Download(ctx, zipA.URL, zipPath, func(d, t int64) {
		if t <= 0 {
			t = zipA.Size
		}
		progress("Downloading", d, t)
	}); err != nil {
		return err
	}

	progress("Checking", 0, 0)
	sums, err := os.ReadFile(sumsPath)
	if err != nil {
		return err
	}
	sig, err := os.ReadFile(sigPath)
	if err != nil {
		return err
	}
	digest, err := sha256File(zipPath)
	if err != nil {
		return err
	}
	if err := CheckFile(u.Key, sums, string(sig), zipA.Name, digest); err != nil {
		return err
	}

	progress("Backing up", 0, 0)
	if backup != nil {
		if err := backup(); err != nil {
			return fmt.Errorf("backing up your saves: %w", err)
		}
	}
	progress("Unpacking", 0, 0)
	_, err = Stage(zipPath, u.InstallDir)
	return err
}

func sha256File(path string) ([]byte, error) {
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
