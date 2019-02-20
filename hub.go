package main

import (
	"log"
	"sync"
	"time"
)

// Hub for publish/subscribe mechanism and broadcasts to all other ws connections
type Hub struct {
	connectionsMx sync.RWMutex                    // the mutex to protect connections
	connections   map[string]map[*connection]bool // Map of conversations/groups that contains user connections using bool for exist state.
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
	log.Println("New hub created")
	return h
}

func (h *Hub) handleMessages(msg *WebSocketMessage) {
	log.Printf("Number of connections in group %s: %d", msg.GroupID, len(h.connections[msg.GroupID]))
	for c := range h.connections[msg.GroupID] {
		h.connectionsMx.RLock()
		select {
		case c.sendChannel <- msg.RawMessage: //Send message to sendChannel

		case <-time.After(1 * time.Second):
			log.Println("Connection dead: shutting down...", c)
			h.removeNewConnection(c, msg.GroupID)
		}
		h.connectionsMx.RUnlock()
	}
}

func (h *Hub) addNewConnection(conn *connection, groupID string) {
	h.connectionsMx.Lock()
	defer h.connectionsMx.Unlock()
	defer log.Println("Attached new connection to hub")
	// Create conversation/group if it doesn't exist
	if h.connections[groupID] == nil {
		h.connections[groupID] = make(map[*connection]bool)
		log.Printf("Created connection ID %s \n", groupID)
	}
	h.connections[groupID][conn] = true
}

func (h *Hub) removeNewConnection(conn *connection, group string) {
	h.connectionsMx.Lock()
	defer h.connectionsMx.Unlock()
	if _, exists := h.connections[group][conn]; exists {
		delete(h.connections[group], conn)
		close(conn.sendChannel)
		log.Printf("Removed connection ID %s from hub\n", group)
	}
}
