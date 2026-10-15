package server

import (
	"net/http"
	"sort"
	"sync"
	"time"
)

// Transfer is one upload (phone -> PC, Dir "in") or file download
// (PC -> phone, Dir "out") in progress. Total is -1 when unknown; for
// uploads it is the request size, multipart framing included.
type Transfer struct {
	ID      int64     `json:"id"`
	Dir     string    `json:"dir"`
	Name    string    `json:"name"`
	Total   int64     `json:"total"`
	Done    int64     `json:"done"`
	Started time.Time `json:"started"`
}

type transfers struct {
	mu   sync.Mutex
	next int64
	m    map[int64]*Transfer
}

type transfer struct {
	reg *transfers
	id  int64
}

func (r *transfers) start(dir, name string, total int64) *transfer {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.m == nil {
		r.m = map[int64]*Transfer{}
	}
	r.next++
	if total < 0 {
		total = -1
	}
	r.m[r.next] = &Transfer{ID: r.next, Dir: dir, Name: name, Total: total, Started: time.Now()}
	return &transfer{reg: r, id: r.next}
}

func (t *transfer) add(n int64) {
	if n <= 0 {
		return
	}
	t.reg.mu.Lock()
	if x := t.reg.m[t.id]; x != nil {
		x.Done += n
	}
	t.reg.mu.Unlock()
}

func (t *transfer) setName(name string) {
	t.reg.mu.Lock()
	if x := t.reg.m[t.id]; x != nil {
		x.Name = name
	}
	t.reg.mu.Unlock()
}

func (t *transfer) end() {
	t.reg.mu.Lock()
	delete(t.reg.m, t.id)
	t.reg.mu.Unlock()
}

// Transfers lists the transfers in progress, oldest first.
func (s *Server) Transfers() []Transfer {
	r := &s.transfers
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]Transfer, 0, len(r.m))
	for _, x := range r.m {
		out = append(out, *x)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// countWriter reports download progress.
type countWriter struct {
	http.ResponseWriter
	tr *transfer
}

func (c *countWriter) Write(p []byte) (int, error) {
	n, err := c.ResponseWriter.Write(p)
	c.tr.add(int64(n))
	return n, err
}

func (c *countWriter) Unwrap() http.ResponseWriter { return c.ResponseWriter }
