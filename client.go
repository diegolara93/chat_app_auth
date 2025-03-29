package main

import (
	"bytes"
	"encoding/json"
	"log"
	"time"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

const (
	// Time allowed to write a message to the peer
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer
	maxMessageSize = 512
)

var (
	newLine = []byte{'\n'}
	space   = []byte{' '}
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// Client is the middleman between the websocket connection and the hub
type Client struct {
	hub *Hub

	// The websocket connection
	conn *websocket.Conn

	// Buffered channel of outbound messages
	send chan ChatMessage

	// A user pointer to allow multiple sockets for a single user
	user *User

	// The current room the client is in
	currentRoom string
}

// readPump pumps messages from the ws connection to the hub
//
// The application runs readPump in a per-connection goroutine. The application
// Ensures that there is at most one reader on a connection by executing all
// reads from this specific goroutine
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()
	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error { c.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			}
			break
		}
		message = bytes.TrimSpace(bytes.Replace(message, newLine, space, -1))

		// Try to parse as a ChatMessage
		var chatMsg ChatMessage
		if err := json.Unmarshal(message, &chatMsg); err != nil {
			// If parsing fails, create a simple message with the content
			chatMsg = ChatMessage{
				Content: string(message),
			}
		}

		// Set username if available
		if c.user != nil {
			chatMsg.Username = c.user.Username
		}

		// If no room specified in message but client is in a room, use that
		if chatMsg.RoomID == "" && c.currentRoom != "" {
			chatMsg.RoomID = c.currentRoom
		}

		c.hub.broadcast <- chatMsg
	}
}

// writePump pumps messages from the hub to the ws connection
//
// A goroutine running writePump is started for each connection. The
// application ensures that there is at most one writer to a connection by
// executing all writes from this specific goroutine.
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// the hub closed the channel
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}

			// Convert the ChatMessage to JSON
			messageJSON, err := json.Marshal(message)
			if err != nil {
				log.Printf("Error marshaling message: %v", err)
				return
			}

			w.Write(messageJSON)

			// Add queued chat messages to the current ws message
			n := len(c.send)
			for i := 0; i < n; i++ {
				nextMsg := <-c.send
				nextMsgJSON, err := json.Marshal(nextMsg)
				if err != nil {
					continue
				}
				w.Write(newLine)
				w.Write(nextMsgJSON)
			}

			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// joinRoom makes the client join a chat room
func (c *Client) joinRoom(roomID string) {
	// If client is already in a room, leave it first
	if c.currentRoom != "" {
		c.leaveRoom(c.currentRoom)
	}

	// Join the new room
	c.hub.joinRoom <- &ClientRoomAction{
		Client: c,
		RoomID: roomID,
	}
}

// leaveRoom makes the client leave a chat room
func (c *Client) leaveRoom(roomID string) {
	c.hub.leaveRoom <- &ClientRoomAction{
		Client: c,
		RoomID: roomID,
	}
}

func serveWs(hub *Hub, c echo.Context, db *gorm.DB) error {
	conn, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		log.Println(err)
		return err
	}

	// Get optional room ID from query params
	roomID := c.QueryParam("room_id")

	client := &Client{
		hub:         hub,
		conn:        conn,
		send:        make(chan ChatMessage, 256),
		currentRoom: roomID,
		user:        nil,
	}

	// Check for authentication
	if err = Authorize(c, db); err == nil {
		username := GetUsername(c)
		var user User
		if err := db.Where("username = ?", username).First(&user).Error; err == nil {
			client.user = &user
		}
	}

	client.hub.register <- client

	// Join room if specified
	if roomID != "" {
		client.joinRoom(roomID)
	}

	// Allow collection of memory referenced by the caller by doing all work in
	// new goroutines
	go client.writePump()
	go client.readPump()
	return nil
}
