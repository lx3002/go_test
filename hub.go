package main

import (
	"strings"
)

// MessageContainer wraps the message bytes with the destination room
type MessageContainer struct {
	Room    string
	Payload []byte
	Message Message
}

type DirectMessage struct {
	Recipient string
	Payload   []byte
	Message   Message
	Sender    *Client
}

type Hub struct {
	rooms      map[string]map[*Client]bool
	broadcast  chan MessageContainer
	direct     chan DirectMessage
	register   chan *Client
	unregister chan *Client
	clients    map[*Client]bool
	users      map[string]map[*Client]bool
}

func NewHub() *Hub {
	return &Hub{
		broadcast:  make(chan MessageContainer),
		direct:     make(chan DirectMessage),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		rooms:      make(map[string]map[*Client]bool),
		clients:    make(map[*Client]bool),
		users:      make(map[string]map[*Client]bool),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.addClient(client)

		case client := <-h.unregister:
			h.removeClient(client)

		case container := <-h.broadcast:
			saveMessage(container.Message)
			if clients, ok := h.rooms[container.Room]; ok {
				for client := range clients {
					h.sendToClient(client, container.Payload)
				}
			}

		case direct := <-h.direct:
			saveMessage(direct.Message)
			recipients := make(map[*Client]bool)
			delivered := false
			if clients, ok := h.users[normalizeUserKey(direct.Recipient)]; ok {
				for client := range clients {
					recipients[client] = true
					delivered = true
				}
			}
			if direct.Sender != nil {
				if clients, ok := h.users[direct.Sender.UserKey]; ok {
					for client := range clients {
						recipients[client] = true
					}
				}
			}
			for client := range recipients {
				h.sendToClient(client, direct.Payload)
			}
			if !delivered && direct.Sender != nil {
				notice := systemMessage("User is not online: " + strings.TrimSpace(direct.Recipient))
				payload, err := notice.Marshal()
				if err == nil {
					h.sendToClient(direct.Sender, payload)
				}
			}
		}
	}
}

func (h *Hub) addClient(client *Client) {
	if h.rooms[client.roomKey] == nil {
		h.rooms[client.roomKey] = make(map[*Client]bool)
	}
	h.rooms[client.roomKey][client] = true
	h.clients[client] = true

	if h.users[client.UserKey] == nil {
		h.users[client.UserKey] = make(map[*Client]bool)
	}
	h.users[client.UserKey][client] = true
}

func (h *Hub) removeClient(client *Client) {
	if _, ok := h.clients[client]; !ok {
		return
	}

	delete(h.clients, client)
	if clients, ok := h.rooms[client.roomKey]; ok {
		delete(clients, client)
		if len(clients) == 0 {
			delete(h.rooms, client.roomKey)
		}
	}
	if clients, ok := h.users[client.UserKey]; ok {
		delete(clients, client)
		if len(clients) == 0 {
			delete(h.users, client.UserKey)
		}
	}
	close(client.send)
}

func (h *Hub) sendToClient(client *Client, payload []byte) {
	select {
	case client.send <- payload:
	default:
		h.removeClient(client)
	}
}
