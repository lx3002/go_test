package main

import "time"

type Message struct {
	Username  string    `json:"username"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
	target    string     `json:"target"`
    content   string     `json:"content"`
	mediatype string      `json:"media_type"`
	
}
