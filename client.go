package main

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait = 10 * time.Second
	pongWait  = 60 * time.Second

	pingPeriod = (pongWait * 9) / 10

	maxMessageSize = 32 * 1024
)

var ratelimitBucket = make(chan struct{}, 20)

func init() {
	go func() {
		ticker := time.NewTicker(time.Second)
		for range ticker.C {
			select {
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
		if origin == "" || getEnv("ALLOW_ANY_ORIGIN", "") == "1" {
			return true
		}
		originURL, err := url.Parse(origin)
		return err == nil && strings.EqualFold(originURL.Host, r.Host)
	},
}

type Client struct {
	hub  *Hub
	conn *websocket.Conn

	// Buffered channel of outbound messages.
	send chan []byte

	// Username attached to messages sent by this client.
	Username string
	UserKey  string
	roomName string
	roomKey  string
	private  bool
}

type clientPayload struct {
	Type      string `json:"type"`
	Target    string `json:"target"`
	Content   string `json:"content"`
	MediaType string `json:"media_type"`
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
		_, payload, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				break
			}
			break
		}
		select {
		case <-ratelimitBucket:
		default:
			continue
		}

		msg, ok := c.parseMessage(payload)
		if !ok {
			continue
		}
		msgBytes, err := json.Marshal(msg)
		if err != nil {
			continue
		}

		if msg.Type == "dm" {
			c.hub.direct <- DirectMessage{
				Recipient: msg.Target,
				Payload:   msgBytes,
				Message:   msg,
				Sender:    c,
			}
			continue
		}

		c.hub.broadcast <- MessageContainer{
			Room:    c.roomKey,
			Payload: msgBytes,
			Message: msg,
		}
	}
}

func (c *Client) parseMessage(raw []byte) (Message, bool) {
	var incoming clientPayload
	if err := json.Unmarshal(raw, &incoming); err != nil {
		incoming = clientPayload{
			Type:      roomMessageType(c.private),
			Target:    c.roomName,
			Content:   string(raw),
			MediaType: "text",
		}
	}

	mediaType := strings.ToLower(strings.TrimSpace(incoming.MediaType))
	if mediaType != "image" && mediaType != "video" {
		mediaType = "text"
	}

	content := strings.TrimSpace(incoming.Content)
	if content == "" {
		return Message{}, false
	}

	msgType := strings.ToLower(strings.TrimSpace(incoming.Type))
	msg := Message{
		Username:  c.Username,
		Content:   content,
		Timestamp: time.Now().UTC(),
		MediaType: mediaType,
	}

	if msgType == "dm" || msgType == "direct" {
		target := strings.TrimSpace(incoming.Target)
		if target == "" {
			return Message{}, false
		}
		msg.Type = "dm"
		msg.Target = target
		return msg, true
	}

	msg.Type = roomMessageType(c.private)
	msg.Target = c.roomName
	return msg, true
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
