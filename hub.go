package main

import (
	"log"
	"strings"
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

func (h *Hub) handleNewMessages(msg *WebSocketMessage) {
	log.Printf("Number of connections in group %s: %d", msg.GroupID, len(h.connections[msg.GroupID]))
	for c := range h.connections[msg.GroupID] {
		h.connectionsMx.RLock()
		select {
		case c.sendChannel <- *msg: //Send message to sendChannel

		case <-time.After(1 * time.Second):
			log.Println("Connection dead: shutting down...", c)
			h.removeConnectionFromHub(c)
		}
		h.connectionsMx.RUnlock()
	}
}

func (h *Hub) getOldMessages(msg *WebSocketMessage, c *connection) {
	convo, err := c.redis.ZRange(msg.GroupID, 0, -1).Result()
	if err != nil {
		log.Println("Error getting redis values: ", err)
		return
	}
	log.Println("Retrieved old messages from redis")
	// parseable stringified json array of objects
	msg.RawMessage = []byte("[" + strings.Join(convo, ",") + "]")
	h.connectionsMx.RLock()
	select {
	case c.sendChannel <- *msg: //Send message to sendChannel
	case <-time.After(1 * time.Second):
		log.Println("Connection dead: shutting down...", c)
		h.removeConnectionFromHub(c)
	}
	h.connectionsMx.RUnlock()
}

func (h *Hub) addNewConnection(conn *connection, groupID string) {
	h.connectionsMx.Lock()
	defer h.connectionsMx.Unlock()
	defer log.Printf("Attached new connection ID %s to hub", groupID)
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
		log.Printf("Length of h.connection[%s]: %d", group, len(h.connections[group]))
	}
}

func (h *Hub) removeConnectionFromHub(conn *connection) {
	h.connectionsMx.Lock()
	defer h.connectionsMx.Unlock()
	close(conn.sendChannel)
	for _, g := range conn.groups {
		if _, exists := h.connections[g][conn]; exists {
			delete(h.connections[g], conn)
			log.Printf("Removed connection ID %s from hub\n", g)
			log.Printf("Length of h.connection[%s]: %d", g, len(h.connections[g]))
		}
	}
}
