//go:build windows

package pc

import (
	"bytes"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The clipboard tests replace the user's clipboard, so they only run when
// COPYAPP_CLIPBOARD_TEST=1. The previous text content is restored.
func clip(t *testing.T) *PC {
	t.Helper()
	if os.Getenv("COPYAPP_CLIPBOARD_TEST") != "1" {
		t.Skip("set COPYAPP_CLIPBOARD_TEST=1 to run (touches the real clipboard)")
	}
	p, err := New()
	if err != nil {
		t.Fatal(err)
	}
	old := p.ReadText()
	t.Cleanup(func() {
		if old != "" {
			p.SetText(old)
		}
	})
	return p
}

func TestTextRoundTrip(t *testing.T) {
	p := clip(t)
	want := "copyapp test é中"
	if err := p.SetText(want); err != nil {
		t.Fatal(err)
	}
	if got := p.ReadText(); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestImageRoundTrip(t *testing.T) {
	p := clip(t)
	img := image.NewNRGBA(image.Rect(0, 0, 3, 2))
	img.Set(1, 1, color.NRGBA{R: 200, G: 10, B: 30, A: 255})
	var buf bytes.Buffer
	png.Encode(&buf, img)
	if err := p.SetImagePNG(buf.Bytes()); err != nil {
		t.Fatal(err)
	}
	got := p.ReadImagePNG()
	dec, err := png.Decode(bytes.NewReader(got))
	if err != nil {
		t.Fatalf("read back is not PNG: %v", err)
	}
	if dec.Bounds().Dx() != 3 || dec.Bounds().Dy() != 2 {
		t.Fatalf("size %v", dec.Bounds())
	}
	r, g, b, _ := dec.At(1, 1).RGBA()
	if r>>8 != 200 || g>>8 != 10 || b>>8 != 30 {
		t.Fatalf("pixel mismatch %d %d %d", r>>8, g>>8, b>>8)
	}
}

func TestSetImageRejectsNonPNG(t *testing.T) {
	p := &PC{} // no clipboard access needed: validation happens first
	if err := p.SetImagePNG([]byte("GIF89a not a png")); err == nil {
		t.Fatal("non-PNG accepted")
	}
}

func TestLanIPs(t *testing.T) {
	ips := LanIPs()
	if len(ips) == 0 {
		t.Skip("no private IPv4 address (not connected?)")
	}
	for _, ip := range ips {
		if ip.To4() == nil || !ip.IsPrivate() || ip.IsLoopback() {
			t.Fatalf("unexpected address %v", ip)
		}
	}
}

func TestReceivedDir(t *testing.T) {
	d := ReceivedDir()
	if !strings.HasSuffix(d, filepath.Join("Downloads", "CopyApp")) && filepath.Base(d) != "CopyApp" {
		t.Fatalf("unexpected dir %q", d)
	}
}

// Shows a real toast; run with COPYAPP_TOAST_TEST=1.
func TestToastRuns(t *testing.T) {
	if os.Getenv("COPYAPP_TOAST_TEST") != "1" {
		t.Skip("set COPYAPP_TOAST_TEST=1 to show a real toast")
	}
	out, err := toastCmd("CopyApp test", `<b>&'" $env:PATH ok`).CombinedOutput()
	if err != nil {
		t.Fatalf("%v: %s", err, out)
	}
}

func TestPSQuote(t *testing.T) {
	// Notify passes text via environment, never via the script; make sure the
	// script itself has no interpolation of caller data.
	if strings.Contains(toastScript, "%s") {
		t.Fatal("toast script must not be formatted with caller data")
	}
}

func TestSetImageRejectsHugePNG(t *testing.T) {
	var b bytes.Buffer
	png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 2, 2)))
	p := b.Bytes()
	binary.BigEndian.PutUint32(p[16:], 10000)
	binary.BigEndian.PutUint32(p[20:], 5001)
	binary.BigEndian.PutUint32(p[29:], crc32.ChecksumIEEE(p[12:29]))
	if err := (&PC{}).SetImagePNG(p); !errors.Is(err, ErrTooManyPixels) {
		t.Fatalf("want ErrTooManyPixels, got %v", err)
	}
}

// Smoke test against the real netsh: must not fail or hang; prints the result.
func TestWiFiNameRuns(t *testing.T) {
	t.Logf("WiFiName() = %q", WiFiName())
}

func TestDecodeOEM(t *testing.T) {
	if got := decodeOEM([]byte("SSID : Дом")); got != "SSID : Дом" {
		t.Fatalf("utf-8 passthrough: %q", got)
	}
	if got := decodeOEM([]byte("SSID : Home")); got != "SSID : Home" {
		t.Fatalf("ascii: %q", got)
	}
}
