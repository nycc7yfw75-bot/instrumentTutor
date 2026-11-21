package update

import (
	"bufio"
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

// Release integrity (ONLINE.md §A5.6): SHA256SUMS is signed with an
// Ed25519 key in minisign's format (algorithm "Ed": the signature covers
// the file itself, plus minisign's global signature over the signature
// and its trusted comment). The private key lives only in the repo's
// Actions secrets; the public key is built into the app (PublicKey). The
// exe is not code-signed: this proves an update came from the owner's
// release pipeline.

const sigAlg = "Ed"

// ErrBadSignature is any signature or checksum that does not match.
var ErrBadSignature = errors.New("the update's signature does not match: it may have been tampered with")

// PublicKey is a minisign public key: its 8-byte key id and the key.
type PublicKey struct {
	ID  [8]byte
	Key ed25519.PublicKey
}

// SecretKey is the signing side (used only by tools/releasesign in CI).
type SecretKey struct {
	ID  [8]byte
	Key ed25519.PrivateKey
}

// GenerateKey makes a new key pair.
func GenerateKey() (PublicKey, SecretKey, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return PublicKey{}, SecretKey{}, err
	}
	var id [8]byte
	if _, err := rand.Read(id[:]); err != nil {
		return PublicKey{}, SecretKey{}, err
	}
	return PublicKey{ID: id, Key: pub}, SecretKey{ID: id, Key: priv}, nil
}

// String is the public key as minisign writes it (the base64 line).
func (p PublicKey) String() string {
	b := append([]byte(sigAlg), p.ID[:]...)
	return base64.StdEncoding.EncodeToString(append(b, p.Key...))
}

// ParsePublicKey reads a minisign public key: the base64 line, or the
// whole .pub file with its comment line.
func ParsePublicKey(text string) (PublicKey, error) {
	var p PublicKey
	line := lastLine(text)
	b, err := base64.StdEncoding.DecodeString(line)
	if err != nil || len(b) != 2+8+ed25519.PublicKeySize || string(b[:2]) != sigAlg {
		return p, errors.New("not a minisign Ed25519 public key")
	}
	copy(p.ID[:], b[2:10])
	p.Key = ed25519.PublicKey(append([]byte(nil), b[10:]...))
	return p, nil
}

// String is the secret key as one base64 line (not minisign's encrypted
// format: it is only ever kept in an Actions secret).
func (s SecretKey) String() string {
	b := append([]byte("HS"), s.ID[:]...)
	return base64.StdEncoding.EncodeToString(append(b, s.Key.Seed()...))
}

// ParseSecretKey reads SecretKey.String's line.
func ParseSecretKey(text string) (SecretKey, error) {
	var s SecretKey
	b, err := base64.StdEncoding.DecodeString(strings.TrimSpace(text))
	if err != nil || len(b) != 2+8+ed25519.SeedSize || string(b[:2]) != "HS" {
		return s, errors.New("not a Headliner signing key")
	}
	copy(s.ID[:], b[2:10])
	s.Key = ed25519.NewKeyFromSeed(b[10:])
	return s, nil
}

// Public is the secret key's public half.
func (s SecretKey) Public() PublicKey {
	return PublicKey{ID: s.ID, Key: s.Key.Public().(ed25519.PublicKey)}
}

// Sign signs msg as a minisign .sig file.
func Sign(s SecretKey, msg []byte, trusted string) string {
	sig := ed25519.Sign(s.Key, msg)
	blob := append(append([]byte(sigAlg), s.ID[:]...), sig...)
	global := ed25519.Sign(s.Key, append(append([]byte(nil), sig...), trusted...))
	return "untrusted comment: signature from headliner release key\n" +
		base64.StdEncoding.EncodeToString(blob) + "\n" +
		"trusted comment: " + trusted + "\n" +
		base64.StdEncoding.EncodeToString(global) + "\n"
}

// Verify checks a minisign .sig over msg with the public key.
func Verify(p PublicKey, msg []byte, sigFile string) error {
	var lines []string
	sc := bufio.NewScanner(strings.NewReader(sigFile))
	for sc.Scan() {
		if l := strings.TrimSpace(sc.Text()); l != "" {
			lines = append(lines, l)
		}
	}
	if len(lines) != 4 || !strings.HasPrefix(lines[0], "untrusted comment:") || !strings.HasPrefix(lines[2], "trusted comment: ") {
		return ErrBadSignature
	}
	blob, err := base64.StdEncoding.DecodeString(lines[1])
	if err != nil || len(blob) != 2+8+ed25519.SignatureSize || string(blob[:2]) != sigAlg {
		return ErrBadSignature
	}
	if !bytes.Equal(blob[2:10], p.ID[:]) {
		return fmt.Errorf("%w (signed with a different key)", ErrBadSignature)
	}
	sig := blob[10:]
	if !ed25519.Verify(p.Key, msg, sig) {
		return ErrBadSignature
	}
	global, err := base64.StdEncoding.DecodeString(lines[3])
	trusted := strings.TrimPrefix(lines[2], "trusted comment: ")
	if err != nil || !ed25519.Verify(p.Key, append(append([]byte(nil), sig...), trusted...), global) {
		return ErrBadSignature
	}
	return nil
}

// SumFor finds name's SHA-256 in a SHA256SUMS file ("<hex>  <name>").
func SumFor(sums []byte, name string) ([]byte, error) {
	sc := bufio.NewScanner(bytes.NewReader(sums))
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) == 2 && strings.TrimPrefix(f[1], "*") == name {
			h, err := hex.DecodeString(f[0])
			if err != nil || len(h) != sha256.Size {
				return nil, ErrBadSignature
			}
			return h, nil
		}
	}
	return nil, fmt.Errorf("%s is not in SHA256SUMS", name)
}

// CheckFile verifies a file's SHA-256 digest against its line in a signed
// SHA256SUMS: the signature first, then the hash.
func CheckFile(p PublicKey, sums []byte, sig string, name string, digest []byte) error {
	if err := Verify(p, sums, sig); err != nil {
		return err
	}
	want, err := SumFor(sums, name)
	if err != nil {
		return err
	}
	if !bytes.Equal(digest, want) {
		return fmt.Errorf("%w (%s's checksum is wrong)", ErrBadSignature, name)
	}
	return nil
}

func lastLine(text string) string {
	lines := strings.Split(strings.TrimSpace(text), "\n")
	return strings.TrimSpace(lines[len(lines)-1])
}
