package main

import (
	"log"
	"sync"
	// "time"
	"bytes"
)

// Hub for publish/subscribe mechanism and broadcasts to all other ws connections
type Hub struct {
	connectionsMx sync.RWMutex                    // the mutex to protect connections
	connections   map[string]map[*connection]bool // Registered connections.
	logMx         sync.RWMutex
	log           [][]byte
}

// newHub creates new hub which as a goroutine for publish and subscribe
func (h *Hub) newHub() {
	// initialize Hub with
	log.Printf("Addr in init %p", h)
	// goroutine to iterate through all
	go func() {
		// for {
		// 	// msg := <-h.broadcastChannel["2"] //listening on read channel
		// 	log.Printf("Addr in loop %p", h)
		// 	h.connectionsMx.RLock()
		// 	log.Printf("Number of connections: %d", len(h.connections["2"]))
		// 	for c := range h.connections["2"] {

		// 		//  select blocks/waits on multiple communication operations and executes case thats ready
		// 		// https://tour.golang.org/concurrency/5
		// 		select {
		// 		case c.sendChannel <- msg:

		// 		// Somehow the reader died so stop trying after for 1 second.
		// 		case <-time.After(1 * time.Second):
		// 			log.Println("shutting down connection ", c)
		// 			h.removeDefaultConnection(c)
		// 		}
		// 	}
		// 	h.connectionsMx.RUnlock()
		// }
	}()
	log.Println("New hub created")
}
func (h *Hub) handleMessages(msg *WebSocketMessage) {
	log.Printf("Number of connections in group %s: %d", msg.GroupID, len(h.connections[msg.GroupID]))
	for c := range h.connections[msg.GroupID] {
		h.connectionsMx.RLock()
		buff := bytes.NewBufferString(msg.UID + ": ")
		buff.Write(msg.Message)
		c.sendChannel <- []byte(buff.String())
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
