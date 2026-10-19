package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/4damrr/tr4cker/internal/db"
	"github.com/4damrr/tr4cker/internal/web"
)

func main() {
	addr := envOr("ADDR", ":8080")
	dbPath := envOr("DB_PATH", "data/tr4cker.db")

	conn, err := db.Open(dbPath)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer conn.Close()

	handler, err := web.New(conn)
	if err != nil {
		log.Fatalf("web: %v", err)
	}

	server := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("tr4cker listening on %s (db %s)", addr, dbPath)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
