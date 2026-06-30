package main

import (
	"encoding/json"
	"html"

	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

const (
	
	writeWait = 10 * time.Second
	
	pongWait = 60 * time.Second
	
	pingPeriod = (pongWait * 9) / 10
	
	maxMessageSize = 1024
)
var ratelimitBucket = make(chan struct {}, 5)

func init(){
	go func() {
		ticker := time.NewTicker(time.Second)
		for range ticker.C{
			select{
			case ratelimitBucket <- struct{}{}:
			default:
			}
		}
	}()
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		return origin == "" || strings.Contains(origin, r.Host)
	},
}

type Client struct {
	hub *Hub
	
	conn *websocket.Conn
	// Buffered channel of outbound messages.
	send chan []byte
	// Username attached to messages sent by this client.
	Username string
	// create a room field to track which room the client is in
	room   string
}


func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, text, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				break
			}
			
		}
		select{
		case <- ratelimitBucket:
		default:
			continue
		}

		safeContent := html.EscapeString(string(text))

		content := strings.TrimSpace(string(text))
		if content == "" {
			continue
		}

		msg := Message{
			Username:  c.Username,
			Content:   content,
			Timestamp: time.Now().UTC(),
			Type: "room",
			target: c.room,
			medmediatype: "text",
		}
		msgBytes, _ := json.Marshal(msg)
		

		// send the marshaled message to the hub's broadcast channel
		c.hub.broadcast <- MessageContainer{
			Room:    c.room,
			Payload: msgBytes,
		}
	}
}

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
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
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
