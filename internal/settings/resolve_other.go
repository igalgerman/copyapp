//go:build !windows

package settings

import "path/filepath"

func canonicalExisting(path string) (string, error) { return filepath.EvalSymlinks(path) }
