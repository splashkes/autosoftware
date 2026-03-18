package main

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
)

type sseBroker struct {
	mu          sync.RWMutex
	subscribers map[string]map[chan string]struct{} // showID -> set of channels
}

func newSSEBroker() *sseBroker {
	return &sseBroker{
		subscribers: make(map[string]map[chan string]struct{}),
	}
}

func (b *sseBroker) subscribe(showID string) chan string {
	b.mu.Lock()
	defer b.mu.Unlock()
	ch := make(chan string, 16)
	if b.subscribers[showID] == nil {
		b.subscribers[showID] = make(map[chan string]struct{})
	}
	b.subscribers[showID][ch] = struct{}{}
	return ch
}

func (b *sseBroker) unsubscribe(showID string, ch chan string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if subs, ok := b.subscribers[showID]; ok {
		delete(subs, ch)
		if len(subs) == 0 {
			delete(b.subscribers, showID)
		}
	}
	close(ch)
}

func (b *sseBroker) publish(showID, eventName, html string) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	subs, ok := b.subscribers[showID]
	if !ok {
		return
	}
	// Format as SSE with event name
	var sb strings.Builder
	sb.WriteString("event: " + eventName + "\n")
	for _, line := range strings.Split(html, "\n") {
		sb.WriteString("data: " + line + "\n")
	}
	sb.WriteString("\n")
	msg := sb.String()
	for ch := range subs {
		select {
		case ch <- msg:
		default:
			// Drop if subscriber is too slow
		}
	}
}

func (a *app) handleAdminShowStream(w http.ResponseWriter, r *http.Request) {
	showID := r.PathValue("showID")

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	ch := a.sseBroker.subscribe(showID)
	defer a.sseBroker.unsubscribe(showID, ch)

	// Send initial keepalive
	fmt.Fprintf(w, ": connected\n\n")
	flusher.Flush()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			fmt.Fprint(w, msg)
			flusher.Flush()
			log.Printf("SSE sent to show %s", showID)
		}
	}
}
