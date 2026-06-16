package main

import (
	"database/sql"
	"log"
	"os"
     _ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	"github.com/joho/godotenv"
)

var db *sql.DB

func init() {
	godotenv.Load()
}

func initDB() {
	connStr := os.Getenv("DATABASE_URL")
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
		id SERIAL PRIMARY KEY AUTO_INCREMENT,
		username TEXT NOT NULL,
		content TEXT NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);`

	if _, err = db.Exec(query); err != nil {
		log.Printf("couldn't create messages table: %v", err)
		db.Close()
		db = nil
	}
}

func saveMessage(username, content string) {
	if db == nil {
		return
	}
	if _, err := db.Exec("INSERT INTO messages (username, content) VALUES ($1, $2)", username, content); err != nil {
		log.Printf("couldn't save message: %v", err)
	}
}
