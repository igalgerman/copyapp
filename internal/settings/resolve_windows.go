//go:build windows

package settings

import (
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

func canonicalExisting(path string) (string, error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return "", err
	}
	h, err := windows.CreateFile(p, windows.FILE_READ_ATTRIBUTES,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return "", err
	}
	defer windows.CloseHandle(h)
	b := make([]uint16, 32768)
	n, err := windows.GetFinalPathNameByHandle(h, &b[0], uint32(len(b)), 0)
	if err != nil {
		return "", err
	}
	if n >= uint32(len(b)) {
		return "", windows.ERROR_FILENAME_EXCED_RANGE
	}
	resolved := windows.UTF16ToString(b[:n])
	if strings.HasPrefix(resolved, `\\?\UNC\`) {
		resolved = `\\` + strings.TrimPrefix(resolved, `\\?\UNC\`)
	} else {
		resolved = strings.TrimPrefix(resolved, `\\?\`)
	}
	return filepath.Clean(resolved), nil
}
