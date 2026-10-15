package server

import (
	"net/http"
	"testing"
)

func TestEventStreamsAreCapped(t *testing.T) {
	e := newEnv(t, 0)
	var cs []*client
	for i := 0; i < maxClients+3; i++ {
		cs = append(cs, e.s.subscribe())
	}
	e.s.mu.Lock()
	n := len(e.s.clients)
	e.s.mu.Unlock()
	if n != maxClients {
		t.Fatalf("%d streams open", n)
	}
	for i, c := range cs {
		select {
		case <-c.quit:
			if i >= 3 {
				t.Fatalf("newer stream %d closed", i)
			}
		default:
			if i < 3 {
				t.Fatalf("oldest stream %d still open", i)
			}
		}
	}
	for _, c := range cs {
		e.s.unsubscribe(c)
	}
}

func TestConcurrentUploadsAreCapped(t *testing.T) {
	e := newEnv(t, 0)
	c := e.pair()
	e.s.uploads.Add(maxUploads) // two uploads running
	w := e.sendJSON(c, map[string]string{"text": "third"})
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("third upload: %d %s", w.Code, w.Body)
	}
	e.s.uploads.Add(-maxUploads)
	if w := e.sendJSON(c, map[string]string{"text": "ok now"}); w.Code != 200 {
		t.Fatalf("after: %d %s", w.Code, w.Body)
	}
}
