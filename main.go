package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
)

// serveWs handles websocket requests from the browser.
func serveWs(hub *Hub, w http.ResponseWriter, r *http.Request) {
	// 1. Authenticate Token
	tokenString := r.URL.Query().Get("token")
	username, err := VerifyToken(tokenString)
	if err != nil {
		http.Error(w, "unauthorized_access", http.StatusUnauthorized)
		return
	}

	// 2. Extract Room parameter (default to 'general' if empty)
	room := strings.TrimSpace(r.URL.Query().Get("room"))
	if room == "" {
		room = "general"
	}

	// 3. Upgrade HTTP connection to WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Upgrade Error:", err)
		return
	}

	// 4. Initialize client with matching lowercase struct fields from Phase 3
	client := &Client{
		hub:      hub,
		conn:     conn,
		send:     make(chan []byte, 256),
		Username: username,
		room:     room,
	}
	client.hub.register <- client

	// 5. Spin up background pumps
	go client.writePump()
	go client.readPump()
}

func handleLogin(w http.ResponseWriter, r *http.Request) {
	// 1. Enable CORS so phones/other devices can hit this endpoint without browser blocks
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	// 2. Read input from URL parameters or fallback to form data
	username := r.URL.Query().Get("username")
	if username == "" {
		username = r.FormValue("username")
	}
	username = strings.TrimSpace(username)

	if username == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "username_required"})
		return
	}

	// 3. Issue the temporary session token
	token, err := CreateToken(username)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// 4. Return the payloads
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"token":    token,
		"username": username,
	})
}

func main() {
	initDB()
	if db != nil {
		defer db.Close()
	}

	hub := NewHub()
	go hub.Run()

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

	http.HandleFunc("./upload", HandleMediaUpload)
	file_server := http.FileServer(http.Dir("./upolads"))
	http.Handle("/static/uploads", http.StripPrefix("/static/uploads", file_server))

	const port = ":8080"
	log.Printf("Chat server started on http://localhost%s\n", port)

	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatal("ListenAndServe Error: ", err)
	}
}
