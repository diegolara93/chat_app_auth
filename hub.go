package main

// ChatMessage represents a message sent to the chat
type ChatMessage struct {
	Content  string `json:"content"`
	Username string `json:"username,omitempty"`
	RoomID   string `json:"room_id"`
}

// Hub maintains the set of active clients and broadcasts messages to the
// clients
type Hub struct {
	// registered clients
	clients map[*Client]bool

	// Map of roomID to clients in that room
	rooms map[string]map[*Client]bool

	// Inbound messages from the clients
	broadcast chan ChatMessage

	// register requests from the clients
	register chan *Client

	// unregister requests from clients
	unregister chan *Client

	// requests to join a room
	joinRoom chan *ClientRoomAction

	// requests to leave a room
	leaveRoom chan *ClientRoomAction
}

type ClientRoomAction struct {
	Client *Client
	RoomID string
}

func newHub() *Hub {
	return &Hub{
		broadcast:  make(chan ChatMessage),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[*Client]bool),
		rooms:      make(map[string]map[*Client]bool),
		joinRoom:   make(chan *ClientRoomAction),
		leaveRoom:  make(chan *ClientRoomAction),
	}
}

func (h *Hub) run() {
	for {
		select {
		case client := <-h.register:
			h.clients[client] = true

		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)

				// Remove client from all rooms
				for roomID, clients := range h.rooms {
					if _, inRoom := clients[client]; inRoom {
						delete(h.rooms[roomID], client)
					}
				}

				close(client.send)
			}

		case action := <-h.joinRoom:
			// Create room if it doesn't exist
			if _, ok := h.rooms[action.RoomID]; !ok {
				h.rooms[action.RoomID] = make(map[*Client]bool)
			}
			// Add client to room
			h.rooms[action.RoomID][action.Client] = true

			// Set client's current room
			action.Client.currentRoom = action.RoomID

		case action := <-h.leaveRoom:
			// Remove client from room
			if room, ok := h.rooms[action.RoomID]; ok {
				delete(room, action.Client)
			}

			if action.Client.currentRoom == action.RoomID {
				action.Client.currentRoom = ""
			}

		case message := <-h.broadcast:
			// If room specified, only send to clients in that room
			if message.RoomID != "" {
				if roomClients, ok := h.rooms[message.RoomID]; ok {
					for client := range roomClients {
						client.send <- message
					}
				}
			} else {
				// uhh this isnt needed anymore but change later
				for client := range h.clients {
					client.send <- message
				}
			}
		}
	}
}
