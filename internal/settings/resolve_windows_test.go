//go:build windows

package settings

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

func TestSaveDirResolvesJunctionAndMissingSuffix(t *testing.T) {
	root := t.TempDir()
	denied := filepath.Join(root, "moved-startup")
	if err := os.Mkdir(denied, 0o700); err != nil {
		t.Fatal(err)
	}
	old := ExtraDenied
	ExtraDenied = append(append([]string{}, old...), denied)
	t.Cleanup(func() { ExtraDenied = old })
	link := filepath.Join(root, "innocent-folder")
	// Directory junctions need no administrator or Developer Mode privileges.
	if out, err := exec.Command("cmd.exe", "/c", "mklink", "/J", link, denied).CombinedOutput(); err != nil {
		t.Fatalf("create isolated junction: %v %s", err, out)
	}
	defer os.Remove(link)
	for _, path := range []string{link, filepath.Join(link, "not-created-yet")} {
		if ValidSaveDir(path) {
			t.Fatalf("accepted Startup alias %q", path)
		}
	}
	if !ValidSaveDir(filepath.Join(root, "safe", "not-created-yet")) {
		t.Fatal("refused safe missing suffix")
	}
}

func TestSaveDirResolvesShortName(t *testing.T) {
	root := t.TempDir()
	denied := filepath.Join(root, "Long denied directory")
	if err := os.Mkdir(denied, 0o700); err != nil {
		t.Fatal(err)
	}
	p, _ := windows.UTF16PtrFromString(denied)
	b := make([]uint16, 32768)
	n, err := windows.GetShortPathName(p, &b[0], uint32(len(b)))
	if err != nil {
		t.Fatal(err)
	}
	short := windows.UTF16ToString(b[:n])
	old := ExtraDenied
	ExtraDenied = append(append([]string{}, old...), denied)
	t.Cleanup(func() { ExtraDenied = old })
	if ValidSaveDir(filepath.Join(short, "future")) {
		t.Fatal("accepted short-name alias")
	}
}

func TestLoadRefusesUnsafeDefaultFolder(t *testing.T) {
	for _, dir := range []string{`\\server\redirected-downloads\CopyApp`, filepath.Join(os.Getenv("SystemRoot"), "CopyApp"), `C:\Windows \Temp`} {
		if _, err := Load(t.TempDir(), dir); err == nil {
			t.Fatalf("unsafe default bypassed validation: %q", dir)
		}
	}
}
