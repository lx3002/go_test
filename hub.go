package main

import (
	"encoding/json"
)

// MessageContainer wraps the message bytes with the destination room
type MessageContainer struct {
	Room    string
	Payload []byte
}

type Hub struct {
	
	rooms      map[string]map[*Client]bool
	broadcast  chan MessageContainer
	register   chan *Client
	unregister chan *Client
	client    map[*Client] bool
	users    map[string]*Client
}

func NewHub() *Hub {
	return &Hub{
		broadcast:  make(chan MessageContainer),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		rooms:      make(map[string]map[*Client]bool),
		client :    make(map[*Client]bool),
		users:      make(map[string]*Client),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			
			if h.rooms[client.room] == nil {
				h.rooms[client.room] = make(map[*Client]bool)
			}
			h.rooms[client.room][client] = true
			h.users[client.Username] = client

		case client := <-h.unregister:
			if clients, ok := h.rooms[client.room]; ok {
				if _, exists := clients[client]; exists {
					delete(clients, client)
					close(client.send)
					
					if len(clients) == 0 {
						delete(h.rooms, client.room)
					}
				}
			}
			delete(h.users, client.Username)

		case container := <-h.broadcast:
			
			var msg Message
			if err := json.Unmarshal(container.Payload, &msg); err == nil {
				saveMessage(msg.Username, msg.Content) 
			}

			// Broadcast ONLY to clients in the specified room
			if clients, ok := h.rooms[container.Room]; ok {
				for client := range clients {
					select {
					case client.send <- container.Payload:
					default:
						close(client.send)
						delete(clients, client)
					}
				}
			}
		}
	}
}