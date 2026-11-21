package update

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func mustV(t *testing.T, s string) Version {
	t.Helper()
	v, err := ParseVersion(s)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestVersionsSortBySemverWithBetasBeforeTheRelease(t *testing.T) {
	order := []string{"v0.9.9", "v1.0.0-beta.1", "v1.0.0-beta.2", "v1.0.0-beta.10", "v1.0.0-rc.1", "v1.0.0", "v1.0.1", "v1.2.0", "v1.10.0"}
	for i := 0; i+1 < len(order); i++ {
		a, b := mustV(t, order[i]), mustV(t, order[i+1])
		if a.Compare(b) >= 0 || b.Compare(a) <= 0 {
			t.Errorf("%s should come before %s", order[i], order[i+1])
		}
	}
	if mustV(t, "1.2.3").Compare(mustV(t, "v1.2.3")) != 0 {
		t.Error("the v prefix changed the order")
	}
	for _, bad := range []string{"", "1.2", "v1.x.0", "1.2.3-", "latest"} {
		if _, err := ParseVersion(bad); err == nil {
			t.Errorf("%q parsed", bad)
		}
	}
}

func release(tag string, pre bool) Release {
	v, _ := ParseVersion(tag)
	return Release{Tag: tag, Prerelease: pre, Assets: []Asset{{Name: ZipName(v)}, {Name: SumsName}, {Name: SigName}}}
}

func TestStableNeverSeesABetaAndBetaSeesBoth(t *testing.T) {
	rs := []Release{release("v1.3.0-beta.1", true), release("v1.2.0", false), release("v1.1.0", false), {Tag: "v9.0.0", Draft: true}}
	cur := mustV(t, "v1.1.0")
	if r, ok := Newest(rs, cur, ChannelStable); !ok || r.Tag != "v1.2.0" {
		t.Errorf("stable picked %q, %v", r.Tag, ok)
	}
	if r, ok := Newest(rs, cur, ChannelBeta); !ok || r.Tag != "v1.3.0-beta.1" {
		t.Errorf("beta picked %q, %v", r.Tag, ok)
	}
	if _, ok := Newest(rs, mustV(t, "v1.2.0"), ChannelStable); ok {
		t.Error("stable on the newest release still offered one")
	}
	// A beta tag not marked pre-release is still hidden from Stable.
	if _, ok := Newest([]Release{release("v2.0.0-beta.1", false)}, cur, ChannelStable); ok {
		t.Error("stable was offered a beta tag")
	}
	// A release without its signature is never offered.
	noSig := Release{Tag: "v1.5.0", Assets: []Asset{{Name: ZipName(mustV(t, "v1.5.0"))}, {Name: SumsName}}}
	if _, ok := Newest([]Release{noSig}, cur, ChannelStable); ok {
		t.Error("an unsigned release was offered")
	}
}

func signedSums(t *testing.T, sec SecretKey, files map[string][]byte) ([]byte, string) {
	t.Helper()
	var b strings.Builder
	for name, data := range files {
		h := sha256.Sum256(data)
		fmt.Fprintf(&b, "%s  %s\n", hex.EncodeToString(h[:]), name)
	}
	sums := []byte(b.String())
	return sums, Sign(sec, sums, "timestamp:1\tfile:SHA256SUMS")
}

func TestSignaturesCatchTampering(t *testing.T) {
	pub, sec, _ := GenerateKey()
	zipData := []byte("the update")
	sums, sig := signedSums(t, sec, map[string][]byte{"h.zip": zipData})
	digest := func(b []byte) []byte { h := sha256.Sum256(b); return h[:] }

	if err := CheckFile(pub, sums, sig, "h.zip", digest(zipData)); err != nil {
		t.Fatalf("a good update was refused: %v", err)
	}
	if err := CheckFile(pub, sums, sig, "h.zip", digest([]byte("the update!"))); !errors.Is(err, ErrBadSignature) {
		t.Errorf("a tampered zip: %v", err)
	}
	tampered := append([]byte("0000  other.zip\n"), sums...)
	if err := CheckFile(pub, tampered, sig, "h.zip", digest(zipData)); !errors.Is(err, ErrBadSignature) {
		t.Errorf("tampered sums: %v", err)
	}
	other, _, _ := GenerateKey()
	if err := CheckFile(other, sums, sig, "h.zip", digest(zipData)); !errors.Is(err, ErrBadSignature) {
		t.Errorf("the wrong key: %v", err)
	}
	// The trusted comment is covered too.
	lines := strings.Split(sig, "\n")
	lines[2] = "trusted comment: timestamp:2"
	if err := Verify(pub, sums, strings.Join(lines, "\n")); !errors.Is(err, ErrBadSignature) {
		t.Errorf("an edited trusted comment: %v", err)
	}
	// Keys survive their text form.
	p2, _ := ParsePublicKey("untrusted comment: x\n" + pub.String())
	s2, err := ParseSecretKey(sec.String())
	if err != nil || p2.String() != pub.String() || s2.Public().String() != pub.String() {
		t.Error("keys don't round-trip")
	}
}

func TestTheBuiltInKeyParses(t *testing.T) {
	if _, err := ParsePublicKey(ReleasePublicKey); err != nil {
		t.Fatal(err)
	}
}

func makeZip(t *testing.T, path string, files map[string]string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	for name, body := range files {
		w, _ := zw.Create(name)
		w.Write([]byte(body))
	}
	zw.Close()
	f.Close()
}

func TestStageUnpacksBesideTheInstallAndNowhereElse(t *testing.T) {
	root := t.TempDir()
	install := filepath.Join(root, "Headliner")
	good := filepath.Join(root, "good.zip")
	makeZip(t, good, map[string]string{ExeName: "new exe", "assets/x.txt": "x"})
	dir, err := Stage(good, install)
	if err != nil || dir != install+".new" {
		t.Fatalf("Stage = %q, %v", dir, err)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "assets", "x.txt")); string(b) != "x" {
		t.Error("a file is missing from the staged update")
	}
	evil := filepath.Join(root, "evil.zip")
	makeZip(t, evil, map[string]string{ExeName: "x", "../outside.txt": "gotcha"})
	if _, err := Stage(evil, install); err == nil {
		t.Error("a zip writing outside its folder was unpacked")
	}
	if _, err := os.Stat(filepath.Join(root, "outside.txt")); err == nil {
		t.Error("a file landed outside the folder")
	}
	noExe := filepath.Join(root, "noexe.zip")
	makeZip(t, noExe, map[string]string{"readme.txt": "x"})
	if _, err := Stage(noExe, install); err == nil {
		t.Error("an update with no exe was staged")
	}
}

func TestSwapKeepsOnePreviousVersionAndRollsBack(t *testing.T) {
	root := t.TempDir()
	install := filepath.Join(root, "Headliner")
	write := func(dir, body string) {
		os.MkdirAll(dir, 0o755)
		os.WriteFile(filepath.Join(dir, ExeName), []byte(body), 0o644)
	}
	read := func(dir string) string { b, _ := os.ReadFile(filepath.Join(dir, ExeName)); return string(b) }
	write(install, "v1")
	os.WriteFile(filepath.Join(install, "unins000.exe"), []byte("uninstaller"), 0o644)
	write(install+".prev", "v0")
	write(install+".new", "v2")
	if err := swapIn(install); err != nil {
		t.Fatal(err)
	}
	if read(install) != "v2" || read(install+".prev") != "v1" {
		t.Errorf("after the swap: install %q, prev %q", read(install), read(install+".prev"))
	}
	if _, err := os.Stat(install + ".new"); err == nil {
		t.Error(".new is still there")
	}
	if b, _ := os.ReadFile(filepath.Join(install, "unins000.exe")); string(b) != "uninstaller" {
		t.Error("the uninstaller was lost in the swap")
	}
	if err := rollBack(install); err != nil {
		t.Fatal(err)
	}
	if read(install) != "v1" {
		t.Errorf("rolled back to %q, want v1", read(install))
	}
}

func TestReleasesAreCachedWithTheirETag(t *testing.T) {
	hits, notModified := 0, 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if r.Header.Get("If-None-Match") == `"abc"` {
			notModified++
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", `"abc"`)
		json.NewEncoder(w).Encode([]Release{release("v1.2.0", false)})
	}))
	defer srv.Close()
	c := Client{Repo: "owner/headliner", BaseURL: srv.URL, CacheDir: t.TempDir()}
	for i := 0; i < 2; i++ {
		rs, err := c.Releases(context.Background())
		if err != nil || len(rs) != 1 || rs[0].Tag != "v1.2.0" {
			t.Fatalf("check %d: %v %v", i, rs, err)
		}
	}
	if hits != 2 || notModified != 1 {
		t.Errorf("%d requests, %d answered from the cache", hits, notModified)
	}
	srv.Close()
	if _, err := (Client{Repo: "owner/headliner", BaseURL: srv.URL}).Releases(context.Background()); !errors.Is(err, ErrOffline) {
		t.Errorf("offline check: %v", err)
	}
}

func TestPrepareVerifiesBeforeUnpacking(t *testing.T) {
	pub, sec, _ := GenerateKey()
	root := t.TempDir()
	v := mustV(t, "v1.2.0")
	zipPath := filepath.Join(root, "src.zip")
	makeZip(t, zipPath, map[string]string{ExeName: "new"})
	zipData, _ := os.ReadFile(zipPath)
	sums, sig := signedSums(t, sec, map[string][]byte{ZipName(v): zipData})
	serve := map[string][]byte{"/" + ZipName(v): zipData, "/" + SumsName: sums, "/" + SigName: []byte(sig)}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(serve[r.URL.Path]) }))
	defer srv.Close()
	r := Release{Tag: "v1.2.0", Assets: []Asset{{Name: ZipName(v), URL: srv.URL + "/" + ZipName(v)},
		{Name: SumsName, URL: srv.URL + "/" + SumsName}, {Name: SigName, URL: srv.URL + "/" + SigName}}}
	install := filepath.Join(root, "Headliner")
	backedUp := false
	u := Updater{Client: Client{BaseURL: srv.URL}, Key: pub, InstallDir: install, TempDir: root}
	if err := u.Prepare(context.Background(), r, func() error { backedUp = true; return nil }, nil); err != nil {
		t.Fatal(err)
	}
	if !backedUp {
		t.Error("the saves were not backed up")
	}
	if _, err := os.Stat(filepath.Join(install+".new", ExeName)); err != nil {
		t.Error("nothing was staged")
	}

	// A tampered zip: nothing staged, nothing backed up.
	os.RemoveAll(install + ".new")
	serve["/"+ZipName(v)] = append(zipData, 0)
	backedUp = false
	if err := u.Prepare(context.Background(), r, func() error { backedUp = true; return nil }, nil); !errors.Is(err, ErrBadSignature) {
		t.Errorf("tampered download: %v", err)
	}
	if backedUp {
		t.Error("backed up before the check failed")
	}
	if _, err := os.Stat(install + ".new"); err == nil {
		t.Error("a tampered update was staged")
	}
	if _, err := (Client{}).Releases(context.Background()); err == nil {
		t.Error("no repo, yet a check ran")
	}
	if err := (Client{}).Download(context.Background(), "http://evil.example/x.zip", filepath.Join(root, "x"), nil); err == nil {
		t.Error("downloaded from outside GitHub")
	}
}
