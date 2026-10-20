package server

import "time"

// ClipboardSendStatus describes the PC's last attempt. Published means placed
// in the outbox with an SSE connection present, never a phone clipboard ACK.
type ClipboardSendStatus struct {
	At     time.Time `json:"at"`
	Source string    `json:"source"`
	Result string    `json:"result"`
}

func (s *Server) PhoneConnected() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.clients) > 0
}
