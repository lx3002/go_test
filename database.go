package main

import (
	"database/sql"
	"log"
	"os"
	"strings"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

var db *sql.DB

func init() {
	godotenv.Load()
}

func initDB() {
	connStr := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if connStr == "" {
		log.Println("DATABASE_URL is not set; message persistence is disabled")
		return
	}

	var err error
	db, err = sql.Open("postgres", connStr)
	if err != nil {
		log.Printf("couldn't open database: %v", err)
		return
	}

	if err = db.Ping(); err != nil {
		log.Printf("couldn't connect to database: %v", err)
		db.Close()
		db = nil
		return
	}

	query := `
	CREATE TABLE IF NOT EXISTS messages (
		id INTEGER GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
		username TEXT NOT NULL,
		content TEXT NOT NULL,
		message_type TEXT NOT NULL DEFAULT 'room',
		target TEXT NOT NULL DEFAULT 'general',
		media_type TEXT NOT NULL DEFAULT 'text',
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);`

	if _, err = db.Exec(query); err != nil {
		log.Printf("couldn't create messages table: %v", err)
		db.Close()
		db = nil
		return
	}

	alterStatements := []string{
		"ALTER TABLE messages ADD COLUMN IF NOT EXISTS message_type TEXT NOT NULL DEFAULT 'room'",
		"ALTER TABLE messages ADD COLUMN IF NOT EXISTS target TEXT NOT NULL DEFAULT 'general'",
		"ALTER TABLE messages ADD COLUMN IF NOT EXISTS media_type TEXT NOT NULL DEFAULT 'text'",
	}
	for _, statement := range alterStatements {
		if _, err = db.Exec(statement); err != nil {
			log.Printf("couldn't migrate messages table: %v", err)
			db.Close()
			db = nil
			return
		}
	}
}

func saveMessage(message Message) {
	if db == nil {
		return
	}
	_, err := db.Exec(
		"INSERT INTO messages (username, content, message_type, target, media_type) VALUES ($1, $2, $3, $4, $5)",
		message.Username,
		message.Content,
		message.Type,
		message.Target,
		message.MediaType,
	)
	if err != nil {
		log.Printf("couldn't save message: %v", err)
	}
}
