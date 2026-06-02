// Package sse implements a Server-Sent Events broker for real-time push.
package sse

import (
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// Event represents an SSE event to be sent to clients.
type Event struct {
	UserID int64  // target user (0 = broadcast)
	Type   string // event type (e.g. "notification", "parse_progress")
	Data   string // JSON payload
}

// Client represents a connected SSE client.
type Client struct {
	UserID int64
	Ch     chan Event
	Done   chan struct{}
}

// Broker manages SSE client connections and event distribution.
type Broker struct {
	mu         sync.RWMutex
	clients    map[int64][]*Client // userID -> clients (supports multiple connections per user)
	register   chan *Client
	unregister chan *Client
	publish    chan Event
	done       chan struct{}
	keepalive  time.Duration
}

// NewBroker creates and starts a new SSE broker.
func NewBroker() *Broker {
	b := &Broker{
		clients:    make(map[int64][]*Client),
		register:   make(chan *Client, 64),
		unregister: make(chan *Client, 64),
		publish:    make(chan Event, 256),
		done:       make(chan struct{}),
		keepalive:  30 * time.Second,
	}
	go b.run()
	slog.Info("SSE broker started")
	return b
}

// Subscribe registers a new SSE client for the given user.
func (b *Broker) Subscribe(userID int64) *Client {
	c := &Client{
		UserID: userID,
		Ch:     make(chan Event, 32),
		Done:   make(chan struct{}),
	}
	b.register <- c
	return c
}

// Unsubscribe removes a client from the broker.
func (b *Broker) Unsubscribe(c *Client) {
	b.unregister <- c
}

// Publish sends an event to the target user (or broadcast if UserID == 0).
func (b *Broker) Publish(event Event) {
	select {
	case b.publish <- event:
	default:
		slog.Warn("SSE publish channel full, dropping event",
			"user_id", event.UserID,
			"type", event.Type,
		)
	}
}

// Shutdown stops the broker.
func (b *Broker) Shutdown() {
	close(b.done)
}

// ClientCount returns the total number of connected clients.
func (b *Broker) ClientCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	count := 0
	for _, clients := range b.clients {
		count += len(clients)
	}
	return count
}

func (b *Broker) run() {
	keepaliveTicker := time.NewTicker(b.keepalive)
	defer keepaliveTicker.Stop()

	for {
		select {
		case <-b.done:
			// Close all client channels
			b.mu.Lock()
			for _, clients := range b.clients {
				for _, c := range clients {
					close(c.Ch)
				}
			}
			b.clients = make(map[int64][]*Client)
			b.mu.Unlock()
			return

		case c := <-b.register:
			b.mu.Lock()
			b.clients[c.UserID] = append(b.clients[c.UserID], c)
			b.mu.Unlock()
			slog.Debug("SSE client registered", "user_id", c.UserID)

		case c := <-b.unregister:
			b.mu.Lock()
			b.removeClient(c)
			b.mu.Unlock()
			close(c.Done)
			slog.Debug("SSE client unregistered", "user_id", c.UserID)

		case event := <-b.publish:
			b.mu.RLock()
			if event.UserID == 0 {
				// Broadcast to all
				for _, clients := range b.clients {
					for _, c := range clients {
						b.sendToClient(c, event)
					}
				}
			} else {
				// Send to specific user
				for _, c := range b.clients[event.UserID] {
					b.sendToClient(c, event)
				}
			}
			b.mu.RUnlock()

		case <-keepaliveTicker.C:
			// Send keepalive to all clients
			keepalive := Event{Type: "keepalive", Data: `{"ts":"` + time.Now().Format(time.RFC3339) + `"}`}
			b.mu.RLock()
			for _, clients := range b.clients {
				for _, c := range clients {
					b.sendToClient(c, keepalive)
				}
			}
			b.mu.RUnlock()
		}
	}
}

func (b *Broker) sendToClient(c *Client, event Event) {
	select {
	case c.Ch <- event:
	default:
		// Client buffer full, skip
		slog.Debug("SSE client buffer full, skipping event",
			"user_id", c.UserID,
			"type", event.Type,
		)
	}
}

func (b *Broker) removeClient(c *Client) {
	clients := b.clients[c.UserID]
	for i, existing := range clients {
		if existing == c {
			b.clients[c.UserID] = append(clients[:i], clients[i+1:]...)
			break
		}
	}
	if len(b.clients[c.UserID]) == 0 {
		delete(b.clients, c.UserID)
	}
}

// FormatSSE formats an event as an SSE message string.
func FormatSSE(event Event) string {
	msg := ""
	if event.Type != "" {
		msg += fmt.Sprintf("event: %s\n", event.Type)
	}
	msg += fmt.Sprintf("data: %s\n\n", event.Data)
	return msg
}
