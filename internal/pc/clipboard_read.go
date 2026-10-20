//go:build windows

package pc

import (
	"context"
	"errors"
	"time"

	"github.com/igalgerman/copyapp/internal/store"
	"golang.design/x/clipboard"
)

var ErrSensitiveClipboard = errors.New("pc: sensitive clipboard")

// ReadClipboard preserves access failures instead of presenting a temporarily
// locked clipboard as empty. The entire attempt, including retries, is bounded.
func (p *PC) ReadClipboard() (store.Item, error) {
	c, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()
	return readClipboard(c, func(ctx context.Context) (bool, error) { return clipboard.Sensitive(ctx) }, clipboard.Read)
}

func readClipboard(c context.Context, sensitive func(context.Context) (bool, error), read func(context.Context, clipboard.Format, ...clipboard.Option) ([]byte, error)) (store.Item, error) {
	var last error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			t := time.NewTimer(time.Duration(attempt) * 40 * time.Millisecond)
			select {
			case <-c.Done():
				t.Stop()
				return store.Item{}, c.Err()
			case <-t.C:
			}
		}
		secret, err := sensitive(c)
		if err != nil {
			last = err
			if errors.Is(err, clipboard.ErrUnavailable) {
				continue
			}
			return store.Item{}, err // fail closed when secret status is unknown
		}
		if secret {
			return store.Item{}, ErrSensitiveClipboard
		}
		img, err := read(c, clipboard.FmtImage)
		if err == nil && len(img) > 0 {
			return store.Item{Kind: "image", Name: "clipboard.png", Mime: "image/png", Data: img}, nil
		}
		if err != nil && !errors.Is(err, clipboard.ErrNoData) {
			last = err
			if errors.Is(err, clipboard.ErrUnavailable) {
				continue
			}
			return store.Item{}, err
		}
		text, err := read(c, clipboard.FmtText)
		if err == nil && len(text) > 0 {
			return store.Item{Kind: "text", Name: "clipboard.txt", Mime: "text/plain; charset=utf-8", Data: text}, nil
		}
		if err == nil || errors.Is(err, clipboard.ErrNoData) {
			return store.Item{}, clipboard.ErrNoData
		}
		last = err
		if !errors.Is(err, clipboard.ErrUnavailable) {
			return store.Item{}, err
		}
	}
	return store.Item{}, last
}
