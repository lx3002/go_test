package main

import (
	"encoding/json"
	"strings"
	"time"
)

type Message struct {
	Username  string    `json:"username"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
	Target    string    `json:"target,omitempty"`
	Type      string    `json:"type"`
	MediaType string    `json:"media_type"`
}

func (m Message) Marshal() ([]byte, error) {
	return json.Marshal(m)
}

func systemMessage(content string) Message {
	return Message{
		Username:  "System",
		Content:   strings.TrimSpace(content),
		Timestamp: time.Now().UTC(),
		Type:      "system",
		MediaType: "text",
	}
}
