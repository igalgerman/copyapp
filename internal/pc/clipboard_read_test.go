//go:build windows

package pc

import (
	"context"
	"errors"
	"testing"

	"golang.design/x/clipboard"
)

func TestClipboardTransientFailureThenText(t *testing.T) {
	checks := 0
	it, err := readClipboard(context.Background(), func(context.Context) (bool, error) {
		checks++
		if checks == 1 { return false, clipboard.ErrUnavailable }
		return false, nil
	}, func(_ context.Context, f clipboard.Format, _ ...clipboard.Option) ([]byte, error) {
		if f == clipboard.FmtImage { return nil, clipboard.ErrNoData }
		return []byte("שלום"), nil
	})
	if err != nil || it.Kind != "text" || string(it.Data) != "שלום" || checks != 2 {
		t.Fatalf("item=%+v error=%v checks=%d", it, err, checks)
	}
}

func TestClipboardUnknownSensitivityFailsClosed(t *testing.T) {
	reads := 0
	_, err := readClipboard(context.Background(), func(context.Context) (bool, error) {
		return false, clipboard.ErrUnavailable
	}, func(context.Context, clipboard.Format, ...clipboard.Option) ([]byte, error) {
		reads++
		return []byte("must not send"), nil
	})
	if !errors.Is(err, clipboard.ErrUnavailable) || reads != 0 { t.Fatalf("error=%v reads=%d", err, reads) }
}

func TestClipboardImageFailureIsNotEmpty(t *testing.T) {
	_, err := readClipboard(context.Background(), func(context.Context) (bool, error) { return false, nil },
		func(context.Context, clipboard.Format, ...clipboard.Option) ([]byte, error) { return nil, clipboard.ErrUnavailable })
	if !errors.Is(err, clipboard.ErrUnavailable) { t.Fatalf("error=%v", err) }
}

func TestClipboardSecretNeverReads(t *testing.T) {
	_, err := readClipboard(context.Background(), func(context.Context) (bool, error) { return true, nil },
		func(context.Context, clipboard.Format, ...clipboard.Option) ([]byte, error) { t.Fatal("read secret"); return nil, nil })
	if !errors.Is(err, ErrSensitiveClipboard) { t.Fatalf("error=%v", err) }
}
