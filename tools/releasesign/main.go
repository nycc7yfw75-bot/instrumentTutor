// Command releasesign makes and signs a release's SHA256SUMS (ONLINE.md
// §A5.5–A5.6).
//
//	releasesign -keygen -out <secret-key-file>   new key pair; prints the public key
//	releasesign -sums <file>...                  writes SHA256SUMS for the files
//	releasesign -sign SHA256SUMS                 writes SHA256SUMS.sig with the key
//	                                             in $HEADLINER_SIGNING_KEY
//	releasesign -verify SHA256SUMS               checks SHA256SUMS.sig with the built-in key
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/S95F/musicTutor/internal/update"
)

func main() {
	keygen := flag.Bool("keygen", false, "make a new key pair")
	out := flag.String("out", "", "with -keygen: where to write the secret key")
	sums := flag.Bool("sums", false, "write SHA256SUMS for the files given")
	sign := flag.String("sign", "", "sign this SHA256SUMS")
	verify := flag.String("verify", "", "verify this SHA256SUMS against its .sig")
	flag.Parse()
	var err error
	switch {
	case *keygen:
		err = runKeygen(*out)
	case *sums:
		err = runSums(flag.Args())
	case *sign != "":
		err = runSign(*sign)
	case *verify != "":
		err = runVerify(*verify)
	default:
		flag.Usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "releasesign:", err)
		os.Exit(1)
	}
}

func runKeygen(out string) error {
	if out == "" {
		return fmt.Errorf("-out is required: the secret key must go to a file, never the terminal")
	}
	if _, err := os.Stat(out); err == nil {
		return fmt.Errorf("%s exists; not overwriting a key", out)
	}
	pub, sec, err := update.GenerateKey()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(out), 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(out, []byte(sec.String()+"\n"), 0o600); err != nil {
		return err
	}
	fmt.Println(pub.String())
	return nil
}

func runSums(files []string) error {
	var b strings.Builder
	for _, f := range files {
		h, err := hashFile(f)
		if err != nil {
			return err
		}
		fmt.Fprintf(&b, "%s  %s\n", h, filepath.Base(f))
	}
	return os.WriteFile("SHA256SUMS", []byte(b.String()), 0o644)
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func runSign(path string) error {
	key := os.Getenv("HEADLINER_SIGNING_KEY")
	if key == "" {
		return fmt.Errorf("HEADLINER_SIGNING_KEY is not set")
	}
	sec, err := update.ParseSecretKey(key)
	if err != nil {
		return err
	}
	pub, err := update.ParsePublicKey(update.ReleasePublicKey)
	if err != nil || pub.String() != sec.Public().String() {
		return fmt.Errorf("the signing key does not match the public key built into the app")
	}
	msg, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	trusted := fmt.Sprintf("timestamp:%d\tfile:%s", time.Now().Unix(), filepath.Base(path))
	return os.WriteFile(path+".sig", []byte(update.Sign(sec, msg, trusted)), 0o644)
}

func runVerify(path string) error {
	pub, err := update.ParsePublicKey(update.ReleasePublicKey)
	if err != nil {
		return err
	}
	msg, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	sig, err := os.ReadFile(path + ".sig")
	if err != nil {
		return err
	}
	if err := update.Verify(pub, msg, string(sig)); err != nil {
		return err
	}
	fmt.Println("signature OK")
	return nil
}
