package main

import (
	"bytes"
	"encoding/json"
	"github.com/go-redis/redis"
	"github.com/gorilla/websocket"
	"log"
	"net/http"
	"sync"
	"time"
)

var upgrader = &websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type connection struct {
	sendChannel chan WebSocketMessage // Buffered channel of outbound messages
	hub         *Hub                  // The hub
	redis       *redis.Client         // The hub
	groups      []string
}

type WebSocketMessage struct {
	Type       string `json:"type"`
	UID        string `json:"uid"`
	OID        string `json:"oid"`
	GroupID    string `json:"groupid"`
	Timestamp  string `json:"timestamp"`
	RawMessage []byte `json:"message"`
}

// reader goroutine that subscribes messages
func (c *connection) reader(wg *sync.WaitGroup, wsConn *websocket.Conn) {
	defer wg.Done()
	for {
		data := make(map[string]interface{})
		_, msg, err := wsConn.ReadMessage() // listening to websocket connection
		if err != nil {
			log.Println("Reader connection lost: ", err)
			break // breaks to finish wait group
		}
		json.Unmarshal(msg, &data)
		wsMessage := WebSocketMessage{
			Type:       data["type"].(string),
			UID:        data["uid"].(string),
			OID:        data["oid"].(string),
			GroupID:    data["groupid"].(string),
			Timestamp:  data["timestamp"].(string),
			RawMessage: msg,
		}

		if wsMessage.Type == "INIT" {
			log.Println("Initialize new ws connection")
			c.hub.addNewConnection(c, wsMessage.UID)
			c.groups = append(c.groups, wsMessage.UID)
		} else {
			log.Printf("Read connection. Message Type: %s, Message: %s", wsMessage.Type, wsMessage.RawMessage)

			// if user does not exist in group
			if !c.hub.connections[wsMessage.GroupID][c] {
				log.Println("Add me to the group")
				c.hub.addNewConnection(c, wsMessage.GroupID)
				c.groups = append(c.groups, wsMessage.GroupID)
			}

			// if the requested other user connection exists
			if len(c.hub.connections[wsMessage.OID]) > 0 {
				for otherConnection := range c.hub.connections[wsMessage.OID] {
					if !c.hub.connections[wsMessage.GroupID][otherConnection] {
						log.Println("I want to add other person to the group")
						c.hub.addNewConnection(otherConnection, wsMessage.GroupID) //add other user to the group
					}
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
	var cachedMessage []byte
	for wsMessage := range c.sendChannel { // Listening to send channel
		err := wsConn.WriteMessage(websocket.TextMessage, wsMessage.RawMessage)
		if err != nil {
			break // breaks to finish wait group
		}
		log.Printf("Write connection. Message Type: %d, Message: %s", websocket.TextMessage, wsMessage.RawMessage)
		if bytes.Compare(cachedMessage, wsMessage.RawMessage) != 0 {
			err := c.redis.ZAdd(wsMessage.GroupID, redis.Z{Score: 0, Member: string(wsMessage.RawMessage)}).Err()
			if err != nil {
				log.Println("Set redis value error: ", err)
			}
		}
		cachedMessage = wsMessage.RawMessage
	}
}

// ping goroutine that pings client every 60 seconds. Theres no response after 5 seconds close the connection
func (c *connection) ping(wg *sync.WaitGroup, wsConn *websocket.Conn) {
	ticker := time.NewTicker(60 * time.Second)
	defer wg.Done()
	defer ticker.Stop()
	for { // Listening to send channel
		<-ticker.C
		// wsConn.SetWriteDeadline(time.Now().Add(900 * time.Second)) // Time to respond
		err := wsConn.WriteMessage(websocket.PingMessage, nil)
		if err != nil {
			log.Printf("Pong failed")
			c.hub.removeConnectionFromHub(c)
			break // breaks to finish wait group
		}
		log.Printf("Ping client connection %v", wsConn.RemoteAddr())
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
	pong, err := wsh.redis.Ping().Result()
	log.Println(pong, err)

	// Creating new connection
	c := &connection{sendChannel: make(chan WebSocketMessage, 256), hub: wsh.hub, redis: wsh.redis}
	var wg sync.WaitGroup
	wg.Add(3)
	go c.writer(&wg, wsConn) // Call publisher goroutine
	go c.reader(&wg, wsConn) //Call subscribe goroutine
	go c.ping(&wg, wsConn)   //Call subscribe goroutine
	wg.Wait()                // Waits until all wait groups are done.
	log.Println("Finished ws goroutines. I'm done waiting.")
	log.Println("Closed websocket.")
	wsConn.Close() // Closes websocket once client connection is lost
}
