package main

import (
	"encoding/json"
	"github.com/gorilla/websocket"
	"log"
	"net/http"
	"sync"
)

var upgrader = &websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type wsHandler struct {
	hub *Hub
}

type connection struct {
	sendChannel chan []byte // Buffered channel of outbound messages
	hub         *Hub        // The hub
}

type WebSocketMessage struct {
	Type       string `json:"type"`
	UID        string `json:"uid"`
	OID        string `json:"oid"`
	GroupID    string `json:"groupid"`
	RawMessage []byte `json:"message"`
}

// reader goroutine that subscribes messages
func (c *connection) reader(wg *sync.WaitGroup, wsConn *websocket.Conn) {
	defer wg.Done()
	for {
		data := make(map[string]interface{})
		_, msg, err := wsConn.ReadMessage() // listening to websocket connection
		if err != nil {
			log.Println("Reader connection lost found: ", err)
			break // breaks to finish wait group
		}
		json.Unmarshal(msg, &data)
		// messageType := data["type"].(string)
		wsMessage := WebSocketMessage{
			Type:       data["type"].(string),
			UID:        data["uid"].(string),
			OID:        data["oid"].(string),
			GroupID:    data["groupid"].(string),
			RawMessage: msg,
		}
		if wsMessage.Type == "INIT" {
			log.Println("Initialize new ws connection")
			c.hub.addNewConnection(c, wsMessage.UID)
		} else {
			log.Printf("Read connection. Message Type: %s, Message: %s", wsMessage.Type, wsMessage.RawMessage)
			if !c.hub.connections[wsMessage.GroupID][c] {
				c.hub.addNewConnection(c, wsMessage.GroupID)
			}
			// if participant connection exists
			if len(c.hub.connections[wsMessage.OID]) > 0 {
				for otherConnection := range c.hub.connections[wsMessage.OID] {
					c.hub.addNewConnection(otherConnection, wsMessage.GroupID) //add other participant in group chat
				}
			}
			// goroutine to send message to correct sendChannel
			go c.hub.handleMessages(&wsMessage)
		}
	}
}

// writer go routine that publishes messages
func (c *connection) writer(wg *sync.WaitGroup, wsConn *websocket.Conn) {
	defer wg.Done()
	for message := range c.sendChannel { // Listening to send channel
		err := wsConn.WriteMessage(websocket.TextMessage, message)
		if err != nil {
			break // breaks to finish wait group
		}
		log.Printf("Write connection. Message Type: %d, Message: %s", websocket.TextMessage, message)
	}
}

// Implicit ServeHTTP method signature for websocket handler https://www.alexedwards.net/blog/a-recap-of-request-handling
func (wsh wsHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	// Upgrade http connection to websocket connection
	wsConn, err := upgrader.Upgrade(w, req, nil)
	if err != nil {
		log.Printf("error upgrading %s", err)
		return
	}
	log.Println("Upgrade connection for", req.RemoteAddr)
	// Creating new connection
	c := &connection{sendChannel: make(chan []byte, 256), hub: wsh.hub}
	// c.hub.addDefaultConnection(c)
	// defer c.hub.removeDefaultConnection(c)
	defer log.Println("Connection lost from hub. Removing Connection...")
	var wg sync.WaitGroup
	wg.Add(2)
	go c.writer(&wg, wsConn) // Call publisher goroutine
	go c.reader(&wg, wsConn) //Call subscribe goroutine
	wg.Wait()                // Waits until all wait groups are done.
	log.Println("Finished read/write goroutines. I'm done waiting.")
	log.Println("Closing websocket...")
	wsConn.Close() // Closes websocket once client connection is lost
}
