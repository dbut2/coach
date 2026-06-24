package service

import (
	"context"
	"strings"
	"sync"

	"github.com/a-h/templ"
)

type sseEvent struct {
	name string
	html string
}

type hub struct {
	mu   sync.Mutex
	subs map[string]map[chan sseEvent]struct{}
}

func newHub() *hub {
	return &hub{subs: map[string]map[chan sseEvent]struct{}{}}
}

func (h *hub) subscribe(userID string) (chan sseEvent, func()) {
	ch := make(chan sseEvent, 8)
	h.mu.Lock()
	if h.subs[userID] == nil {
		h.subs[userID] = map[chan sseEvent]struct{}{}
	}
	h.subs[userID][ch] = struct{}{}
	h.mu.Unlock()
	return ch, func() {
		h.mu.Lock()
		delete(h.subs[userID], ch)
		h.mu.Unlock()
	}
}

func (h *hub) broadcast(userID, name, html string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.subs[userID] {
		select {
		case ch <- sseEvent{name: name, html: html}:
		default: // slow client, drop rather than block generation
		}
	}
}

// renderHTML draws a templ component to a string for SSE delivery.
func renderHTML(c templ.Component) string {
	var b strings.Builder
	if err := c.Render(context.Background(), &b); err != nil {
		return ""
	}
	return b.String()
}
