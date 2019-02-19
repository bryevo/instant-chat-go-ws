package main

import (
	"log"
	"sync"
	"time"
)

// Hub for publish/subscribe mechanism and broadcasts to all other ws connections
type Hub struct {
	connectionsMx sync.RWMutex                    // the mutex to protect connections
	connections   map[string]map[*connection]bool // Registered connections.
	logMx         sync.RWMutex
	log           [][]byte
}

// newHub creates new hub which as a goroutine for publish and subscribe
func newHub() *Hub {
	// initialize Hub with
	h := &Hub{
		connectionsMx: sync.RWMutex{},
		connections:   make(map[string]map[*connection]bool),
	}
	log.Printf("Addr in init %p", h)
	log.Println("New hub created")
	return h
}
func (h *Hub) handleMessages(msg *WebSocketMessage) {
	log.Printf("Number of connections in group %s: %d", msg.GroupID, len(h.connections[msg.GroupID]))
	for c := range h.connections[msg.GroupID] {
		h.connectionsMx.RLock()
		select {

		case c.sendChannel <- msg.RawMessage:

		case <-time.After(1 * time.Second):
			log.Println("shutting down connection ", c)
			h.removeDefaultConnection(c)
		}
		h.connectionsMx.RUnlock()
	}
}

func (h *Hub) addDefaultConnection(conn *connection) {
	h.connectionsMx.Lock()
	defer h.connectionsMx.Unlock()
	h.connections["lobby"][conn] = true
	log.Println("Added connection to hub")
}

func (h *Hub) removeDefaultConnection(conn *connection) {
	h.connectionsMx.Lock()
	defer h.connectionsMx.Unlock()
	if _, ok := h.connections["lobby"][conn]; ok {
		delete(h.connections["lobby"], conn)
		close(conn.sendChannel)
		log.Println("Removed connection from hub")
	}
}

func (h *Hub) addNewConnection(conn *connection, group string) {
	h.connectionsMx.Lock()
	defer h.connectionsMx.Unlock()
	defer log.Println("Added new connection to hub")
	if h.connections[group] == nil {
		h.connections[group] = make(map[*connection]bool)
		log.Printf("Created %s connection to hub\n", group)
	}
	h.connections[group][conn] = true
}

func (h *Hub) removeNewConnection(conn *connection, group string) {
	h.connectionsMx.Lock()
	defer h.connectionsMx.Unlock()
	if _, ok := h.connections[group][conn]; ok {
		delete(h.connections[group], conn)
		close(conn.sendChannel)
		log.Printf("Removed %s connection from hub\n", group)
	}
}
