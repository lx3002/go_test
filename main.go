package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
)

// serveWs handles websocket requests from the browser.
func serveWs(hub *Hub, w http.ResponseWriter, r *http.Request) {
	tokenString := r.URL.Query().Get("token")
	username, err := VerifyToken(tokenString)
	if err != nil {
		http.Error(w, "unauthorized_access", http.StatusUnauthorized)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Upgrade Error:", err)
		return
	}

	client := &Client{hub: hub, conn: conn, send: make(chan []byte, 256), Username: username}
	client.hub.register <- client

	go client.writePump()
	go client.readPump()
}

func handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	username := strings.TrimSpace(r.FormValue("username"))
	if username == "" {
		http.Error(w, "username_required", http.StatusBadRequest)
		return
	}

	token, err := CreateToken(username)
	if err != nil {
		http.Error(w, "token_error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"token": token, "username": username})
}

func main() {
	initDB()
	if db != nil {
		defer db.Close()
	}

	hub := newHub()
	go hub.run()

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		http.ServeFile(w, r, "index.html")
	})

	http.HandleFunc("/login", handleLogin)
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		serveWs(hub, w, r)
	})

	const port = ":8080"
	log.Printf("Chat server started on http://localhost%s\n", port)

	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatal("ListenAndServe Error: ", err)
	}
}
