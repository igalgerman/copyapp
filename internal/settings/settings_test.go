package settings

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaults(t *testing.T) {
	s := Defaults(`C:\D\CopyApp`)
	if s.Language != "he" || s.ReceiveMode != ModeClipboard || s.SaveDir != `C:\D\CopyApp` || s.MaxBytes != 2<<30 || s.AutostartAsked {
		t.Fatalf("%+v", s)
	}
}

func TestLoadMissingGivesDefaultsAndMigratesLang(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "lang.txt"), []byte("ru\n"), 0o600)
	st, err := Load(dir, dir)
	if err != nil {
		t.Fatal(err)
	}
	if g := st.Get(); g.Language != "ru" || g.SaveDir != dir {
		t.Fatalf("%+v", g)
	}
	if _, err := os.Stat(filepath.Join(dir, FileName)); err == nil {
		t.Fatal("Load must not write the file")
	}
}

func TestUpdatePersistsAndNormalizes(t *testing.T) {
	dir := t.TempDir()
	st, _ := Load(dir, dir)
	if _, err := st.Update(func(s *Settings) {
		s.ReceiveMode = ModeFolder
		s.MaxBytes = 500 << 20
		s.AutostartAsked = true
		s.Language = "en"
	}); err != nil {
		t.Fatal(err)
	}
	st2, err := Load(dir, dir)
	if err != nil {
		t.Fatal(err)
	}
	g := st2.Get()
	if g.ReceiveMode != ModeFolder || g.MaxBytes != 500<<20 || !g.AutostartAsked || g.Language != "en" || g.SaveDir != dir {
		t.Fatalf("%+v", g)
	}
	// Garbage is repaired on load.
	os.WriteFile(filepath.Join(dir, FileName), []byte(`{"language":"xx","receiveMode":"zip","maxBytes":-5,"saveDir":""}`), 0o600)
	st3, _ := Load(dir, dir)
	if g := st3.Get(); g.Language != "he" || g.ReceiveMode != ModeClipboard || g.MaxBytes != DefaultMaxBytes || g.SaveDir != dir {
		t.Fatalf("%+v", g)
	}
	// Corrupt JSON: defaults, not an error that stops the app.
	os.WriteFile(filepath.Join(dir, FileName), []byte(`{`), 0o600)
	if st4, err := Load(dir, dir); err != nil || st4.Get().MaxBytes != DefaultMaxBytes {
		t.Fatalf("corrupt: %v", err)
	}
}

func TestValidSaveDir(t *testing.T) {
	sys := os.Getenv("SystemRoot")
	if sys == "" {
		sys = `C:\Windows`
	}
	for _, ok := range []string{`C:\Users\me\Downloads\CopyApp`, `C:\`, filepath.Join(t.TempDir(), "Phone stuff"), t.TempDir()} {
		if !ValidSaveDir(ok) {
			t.Errorf("refused %q", ok)
		}
	}
	for _, bad := range []string{"", "relative", `\no-drive`, `\\server\share\x`, `\\?\C:\x`, `\\.\PhysicalDrive0`,
		`C:\x\file.txt:stream`, `C:\x\NUL`, `C:\x\com1.txt`, sys, filepath.Join(sys, "System32"), "C:\\x\x00y",
		filepath.Join(os.Getenv("APPDATA"), `Microsoft\Windows\Start Menu\Programs\Startup`),
		filepath.Join(os.Getenv("ProgramData"), `Microsoft\Windows\Start Menu\Programs\StartUp`)} {
		if ValidSaveDir(bad) {
			t.Errorf("accepted %q", bad)
		}
	}
}

func TestTamperedFileFallsBack(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, FileName), []byte(`{"saveDir":"\\\\evil\\share","maxBytes":99999999999}`), 0o600)
	st, err := Load(dir, `C:\def`)
	if err != nil {
		t.Fatal(err)
	}
	if g := st.Get(); g.SaveDir != `C:\def` || g.MaxBytes != DefaultMaxBytes {
		t.Fatalf("%+v", g)
	}
}

func TestValidSize(t *testing.T) {
	for _, v := range []int64{200 << 20, 500 << 20, 1 << 30, 2 << 30} {
		if !ValidSize(v) {
			t.Errorf("%d", v)
		}
	}
	if ValidSize(123) || ValidSize(4<<30) {
		t.Fatal("arbitrary sizes accepted")
	}
}

func TestMemoryStoreDoesNotWrite(t *testing.T) {
	st := NewMemory(Settings{MaxBytes: 1024, SaveDir: "d"})
	if g := st.Get(); g.MaxBytes != 1024 || g.Language != "he" || g.ReceiveMode != ModeClipboard {
		t.Fatalf("%+v", g)
	}
	if _, err := st.Update(func(s *Settings) { s.ReceiveMode = ModeFolder }); err != nil || st.Get().ReceiveMode != ModeFolder {
		t.Fatal(err)
	}
}
