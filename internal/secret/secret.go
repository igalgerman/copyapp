// Package secret holds the per-launch local secret that authorizes this PC's
// own tools (second instance, pairing page) against the loopback routes, plus
// at-rest protection helpers (owner-only file ACLs, Windows DPAPI).
package secret

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// LocalKeyFile is the file in the data directory that holds the local secret.
const LocalKeyFile = "local.key"

// NewLocalKey creates a fresh 32-byte secret, stores it hex-encoded in
// dir\local.key readable only by the current user, and returns it.
func NewLocalKey(dir string) (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	key := hex.EncodeToString(b[:])
	if err := WriteOwnerOnly(filepath.Join(dir, LocalKeyFile), []byte(key)); err != nil {
		return "", err
	}
	return key, nil
}

// ReadLocalKey returns the secret written by the running instance.
func ReadLocalKey(dir string) (string, error) {
	b, err := os.ReadFile(filepath.Join(dir, LocalKeyFile))
	if err != nil {
		return "", err
	}
	k := strings.TrimSpace(string(b))
	if len(k) != 64 {
		return "", errors.New("secret: malformed local key")
	}
	return k, nil
}

// Equal compares secrets in constant time; empty never matches.
func Equal(a, b string) bool {
	return a != "" && subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// WriteOwnerOnly replaces path with data. The new file is restricted to the
// current user before any data is written to it.
func WriteOwnerOnly(path string, data []byte) error {
	tmp := path + ".tmp"
	os.Remove(tmp)
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	err = Restrict(tmp)
	if err == nil {
		_, err = f.Write(data)
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(tmp, path)
	}
	if err != nil {
		os.Remove(tmp)
	}
	return err
}
