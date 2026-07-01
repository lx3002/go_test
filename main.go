package main

import (
	"encoding/json"
	"log"
	"net"
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

	// 2. Extract room settings. Private rooms require a shared key.
	room := normalizeRoomName(r.URL.Query().Get("room"))
	private := privateFlag(r.URL.Query().Get("private"))
	roomSecret := strings.TrimSpace(r.URL.Query().Get("room_key"))
	if private && roomSecret == "" {
		http.Error(w, "private_room_key_required", http.StatusBadRequest)
		return
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
		UserKey:  normalizeUserKey(username),
		roomName: room,
		roomKey:  roomChannelKey(room, private, roomSecret),
		private:  private,
	}
	client.hub.register <- client

	// 5. Spin up background pumps
	go client.writePump()
	go client.readPump()
}

func handleLogin(w http.ResponseWriter, r *http.Request) {
	// 1. Enable CORS so phones/other devices can hit this endpoint without browser blocks
	setCORS(w)
	if handleOptions(w, r) {
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
	if len(username) > 32 {
		username = username[:32]
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

func handleConfig(w http.ResponseWriter, r *http.Request) {
	setCORS(w)
	if handleOptions(w, r) {
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"upload_max_bytes": MaxUploadSize,
		"message_types":    []string{"room", "private_room", "dm"},
		"media_types":      []string{"text", "image", "video"},
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
	http.HandleFunc("/config", handleConfig)
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		serveWs(hub, w, r)
	})

	http.HandleFunc("/upload", HandleMediaUpload)
	fileServer := http.FileServer(http.Dir(UploadDirectory))
	http.Handle("/static/uploads/", http.StripPrefix("/static/uploads/", fileServer))

	addr := getEnv("CHAT_ADDR", "0.0.0.0:8080")
	logServerURLs(addr)

	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatal("ListenAndServe Error: ", err)
	}
}

func setCORS(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
}

func handleOptions(w http.ResponseWriter, r *http.Request) bool {
	if r.Method != http.MethodOptions {
		return false
	}
	w.WriteHeader(http.StatusNoContent)
	return true
}

func logServerURLs(addr string) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		log.Printf("Chat server started on http://%s\n", addr)
		return
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		log.Printf("Chat server started on http://localhost:%s\n", port)
		for _, url := range localNetworkURLs(port) {
			log.Printf("Available on your network at %s\n", url)
		}
		return
	}
	log.Printf("Chat server started on http://%s:%s\n", host, port)
}

func localNetworkURLs(port string) []string {
	var urls []string
	interfaces, err := net.Interfaces()
	if err != nil {
		return urls
	}
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}
			ip := ipNet.IP.To4()
			if ip == nil {
				continue
			}
			urls = append(urls, "http://"+ip.String()+":"+port)
		}
	}
	return urls
}
