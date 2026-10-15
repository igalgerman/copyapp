//go:build windows

package pc

import (
	"os"
	"strings"

	"golang.org/x/sys/windows"
)

const (
	mbOK            = 0x0
	mbYesNo         = 0x4
	mbIconQuestion  = 0x20
	mbIconWarning   = 0x30
	mbIconInfo      = 0x40
	mbDefButton2    = 0x100
	mbSetForeground = 0x10000
	mbTopmost       = 0x40000
	mbRTLReading    = 0x2000
	mbRight         = 0x80000
	idYes           = 6
	beepOK          = mbOK
	beepWarning     = mbIconWarning
)

var procMessageBeep = windows.NewLazySystemDLL("user32.dll").NewProc("MessageBeep")

// skipPrompts: COPYAPP_SKIP_PROMPTS=yes|no answers every question without a
// window (tests, e2e). Information boxes are skipped.
func skipPrompts() (answer, skip bool) {
	switch strings.ToLower(os.Getenv("COPYAPP_SKIP_PROMPTS")) {
	case "yes":
		return true, true
	case "no":
		return false, true
	}
	return false, false
}

func box(title, text string, flags uint32, rtl bool) int32 {
	flags |= mbTopmost | mbSetForeground
	if rtl {
		flags |= mbRTLReading | mbRight
	}
	t, _ := windows.UTF16PtrFromString(strings.ReplaceAll(text, "\x00", ""))
	c, _ := windows.UTF16PtrFromString(title)
	r, _ := windows.MessageBox(0, t, c, flags)
	return r
}

// Ask shows a native Yes/No box and blocks until it is answered. defaultNo
// makes No the default button (for destructive questions).
func Ask(title, text string, rtl, defaultNo bool) bool {
	if a, skip := skipPrompts(); skip {
		return a
	}
	flags := uint32(mbYesNo | mbIconQuestion)
	if defaultNo {
		flags = mbYesNo | mbIconWarning | mbDefButton2
	}
	return box(title, text, flags, rtl) == idYes
}

// Info shows a native information box (skipped under COPYAPP_SKIP_PROMPTS).
func Info(title, text string, rtl bool) {
	if _, skip := skipPrompts(); skip {
		return
	}
	box(title, text, mbOK|mbIconInfo, rtl)
}

// Beep plays the default "OK" sound (or the warning sound).
func Beep(ok bool) {
	t := uintptr(beepOK)
	if !ok {
		t = beepWarning
	}
	procMessageBeep.Call(t)
}

const folderScript = `$ErrorActionPreference='Stop'
Add-Type -AssemblyName System.Windows.Forms
[System.Windows.Forms.Application]::EnableVisualStyles()
$o=New-Object System.Windows.Forms.Form -Property @{TopMost=$true;ShowInTaskbar=$false}
$d=New-Object System.Windows.Forms.FolderBrowserDialog
$d.Description=$env:CA_TITLE
$d.ShowNewFolderButton=$true
if($env:CA_INIT -and (Test-Path -LiteralPath $env:CA_INIT)){$d.SelectedPath=$env:CA_INIT}
if($d.ShowDialog($o) -eq [System.Windows.Forms.DialogResult]::OK){$b=[Text.Encoding]::UTF8.GetBytes($d.SelectedPath);$s=[Console]::OpenStandardOutput();$s.Write($b,0,$b.Length)}`

const fileScript = `$ErrorActionPreference='Stop'
Add-Type -AssemblyName System.Windows.Forms
[System.Windows.Forms.Application]::EnableVisualStyles()
$o=New-Object System.Windows.Forms.Form -Property @{TopMost=$true;ShowInTaskbar=$false}
$d=New-Object System.Windows.Forms.OpenFileDialog
$d.Title=$env:CA_TITLE
$d.CheckFileExists=$true
$d.Multiselect=$false
if($d.ShowDialog($o) -eq [System.Windows.Forms.DialogResult]::OK){$b=[Text.Encoding]::UTF8.GetBytes($d.FileName);$s=[Console]::OpenStandardOutput();$s.Write($b,0,$b.Length)}`

// pick runs a picker script. COPYAPP_TEST_PICK, when set, is returned
// instead of showing a dialog (e2e). "" means cancelled.
func pick(script, title, initial string) (string, error) {
	if v, ok := os.LookupEnv("COPYAPP_TEST_PICK"); ok {
		return v, nil
	}
	cmd := psCmd(script, true)
	cmd.Env = append(os.Environ(), "CA_TITLE="+title, "CA_INIT="+initial)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// ChooseFolder shows the native folder picker, starting at initial.
func ChooseFolder(title, initial string) (string, error) { return pick(folderScript, title, initial) }

// ChooseFile shows the native open-file dialog.
func ChooseFile(title string) (string, error) { return pick(fileScript, title, "") }
