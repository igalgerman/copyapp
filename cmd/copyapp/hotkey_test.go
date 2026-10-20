//go:build windows

package main

import (
	"testing"
	"time"

	"golang.design/x/hotkey"
)

func TestConsumeHotkeyReleasesAndClosedListener(t *testing.T) {
	down, up := make(chan hotkey.Event), make(chan hotkey.Event)
	sent, done := make(chan struct{}, 20), make(chan struct{})
	go func() { consumeHotkey(down, up, func() { sent <- struct{}{} }); close(done) }()
	for i := 0; i < 20; i++ {
		down <- hotkey.Event{}
		select { case <-sent: case <-time.After(time.Second): t.Fatal("missed press", i) }
		up <- hotkey.Event{}
	}
	close(down)
	select { case <-done: case <-time.After(time.Second): t.Fatal("closed listener did not return") }
}

// Independent SDK queues can contain down1,up1,down2 simultaneously. Treating
// the release as belonging to the second down silently discards real presses.
func TestConsumeHotkeyBurstDoesNotLosePresses(t *testing.T) {
	down, up := make(chan hotkey.Event, 3), make(chan hotkey.Event, 1)
	down <- hotkey.Event{}
	up <- hotkey.Event{}
	down <- hotkey.Event{}
	down <- hotkey.Event{}
	close(down)
	count := 0
	consumeHotkey(down, up, func() { count++ })
	if count != 3 { t.Fatalf("burst triggered %d sends, want 3", count) }
}
