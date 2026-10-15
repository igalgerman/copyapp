//go:build !windows

package secret

import "os"

// Restrict limits the file to its owner.
func Restrict(path string) error { return os.Chmod(path, 0o600) }
