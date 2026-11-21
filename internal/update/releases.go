package update

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Channels (Settings → Updates).
const (
	ChannelStable = "stable"
	ChannelBeta   = "beta"
)

// Asset is one file on a release.
type Asset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
	Size int64  `json:"size"`
}

// Release is a GitHub release as the API lists it.
type Release struct {
	Tag        string  `json:"tag_name"`
	Name       string  `json:"name"`
	Body       string  `json:"body"`
	Draft      bool    `json:"draft"`
	Prerelease bool    `json:"prerelease"`
	Assets     []Asset `json:"assets"`
}

// Version is the release's tag as a version.
func (r Release) Version() (Version, error) { return ParseVersion(r.Tag) }

// Asset finds a file on the release by name.
func (r Release) Asset(name string) (Asset, bool) {
	for _, a := range r.Assets {
		if a.Name == name {
			return a, true
		}
	}
	return Asset{}, false
}

// ZipName is the portable zip the updater installs.
func ZipName(v Version) string { return "headliner-" + v.String() + "-windows-amd64.zip" }

// Asset names next to the zip.
const (
	SumsName = "SHA256SUMS"
	SigName  = "SHA256SUMS.sig"
)

// Newest picks the newest release newer than current that the channel
// may see: Stable never sees a pre-release or a pre-release tag; Beta
// sees both. Drafts and releases missing the zip or its signature are
// skipped. ok is false when nothing is newer.
func Newest(releases []Release, current Version, channel string) (Release, bool) {
	var best Release
	var bestV Version
	found := false
	for _, r := range releases {
		v, err := r.Version()
		if err != nil || r.Draft {
			continue
		}
		if channel != ChannelBeta && (r.Prerelease || !v.Stable()) {
			continue
		}
		if v.Compare(current) <= 0 {
			continue
		}
		if _, ok := r.Asset(ZipName(v)); !ok {
			continue
		}
		if _, ok := r.Asset(SumsName); !ok {
			continue
		}
		if _, ok := r.Asset(SigName); !ok {
			continue
		}
		if !found || v.Compare(bestV) > 0 {
			best, bestV, found = r, v, true
		}
	}
	return best, found
}

// Client reads releases from GitHub. Responses are cached on disk with
// their ETag, so a check that finds nothing new costs no rate limit.
type Client struct {
	Repo     string // owner/name
	HTTP     *http.Client
	CacheDir string
	BaseURL  string // tests; default https://api.github.com
}

type releaseCache struct {
	ETag     string    `json:"etag"`
	Fetched  time.Time `json:"fetched"`
	Releases []Release `json:"releases"`
}

// ErrOffline is a check that could not reach GitHub: it is skipped
// quietly.
var ErrOffline = errors.New("could not reach GitHub")

// Releases lists the repo's releases, newest first as GitHub sends them.
func (c Client) Releases(ctx context.Context) ([]Release, error) {
	if c.Repo == "" || strings.Count(c.Repo, "/") != 1 {
		return nil, fmt.Errorf("no release repo set")
	}
	base := c.BaseURL
	if base == "" {
		base = "https://api.github.com"
	}
	hc := c.HTTP
	if hc == nil {
		hc = &http.Client{Timeout: 20 * time.Second}
	}
	var cache releaseCache
	cachePath := ""
	if c.CacheDir != "" {
		cachePath = filepath.Join(c.CacheDir, "releases.json")
		if b, err := os.ReadFile(cachePath); err == nil {
			_ = json.Unmarshal(b, &cache)
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/repos/"+c.Repo+"/releases?per_page=30", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "headliner-updater")
	if cache.ETag != "" {
		req.Header.Set("If-None-Match", cache.ETag)
	}
	resp, err := hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrOffline, err)
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusNotModified:
		return cache.Releases, nil
	case resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests:
		if cache.Releases != nil {
			return cache.Releases, nil // rate limited: what we knew
		}
		return nil, fmt.Errorf("%w: GitHub asked us to wait (rate limit)", ErrOffline)
	case resp.StatusCode != http.StatusOK:
		return nil, fmt.Errorf("GitHub answered %s", resp.Status)
	}
	var rs []Release
	if err := json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(&rs); err != nil {
		return nil, fmt.Errorf("reading the release list: %w", err)
	}
	if cachePath != "" {
		cache = releaseCache{ETag: resp.Header.Get("ETag"), Fetched: time.Now(), Releases: rs}
		if b, err := json.Marshal(cache); err == nil {
			_ = os.MkdirAll(c.CacheDir, 0o755)
			_ = os.WriteFile(cachePath, b, 0o644)
		}
	}
	return rs, nil
}

// Download fetches url into path, reporting progress (done, total; total
// is 0 when unknown). Only HTTPS GitHub release assets are fetched.
func (c Client) Download(ctx context.Context, url, path string, progress func(done, total int64)) error {
	if !allowedAssetURL(url, c.BaseURL) {
		return fmt.Errorf("refusing to download from %s", url)
	}
	hc := c.HTTP
	if hc == nil {
		hc = &http.Client{Timeout: 15 * time.Minute}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "headliner-updater")
	resp, err := hc.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrOffline, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("downloading %s: %s", filepath.Base(path), resp.Status)
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	var done int64
	buf := make([]byte, 256<<10)
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, err := f.Write(buf[:n]); err != nil {
				f.Close()
				return err
			}
			done += int64(n)
			if progress != nil {
				progress(done, resp.ContentLength)
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			f.Close()
			return rerr
		}
	}
	return f.Close()
}

// allowedAssetURL: GitHub release downloads over HTTPS (or the test
// server).
func allowedAssetURL(url, testBase string) bool {
	if testBase != "" && strings.HasPrefix(url, testBase) {
		return true
	}
	return strings.HasPrefix(url, "https://github.com/") && strings.Contains(url, "/releases/download/")
}
