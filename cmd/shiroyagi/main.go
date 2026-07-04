package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/takayoshiotake/shiroyagi/internal/auth"
	"github.com/takayoshiotake/shiroyagi/internal/config"
	"github.com/takayoshiotake/shiroyagi/internal/db"
	httpserver "github.com/takayoshiotake/shiroyagi/internal/http"
)

func main() {
	ctx := context.Background()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	database, err := db.Open(cfg.Database)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer database.Close()

	if err := db.Wait(ctx, database, 30*time.Second); err != nil {
		log.Fatalf("wait for database: %v", err)
	}
	if err := db.Migrate(ctx, database); err != nil {
		log.Fatalf("migrate database: %v", err)
	}

	authClient, err := auth.NewClient(ctx, cfg)
	if err != nil {
		log.Fatalf("create auth client: %v", err)
	}

	server := httpserver.New(authClient, auth.NewSessionStore())

	log.Println("listening on :8080")
	if err := http.ListenAndServe(":8080", server.Routes()); err != nil {
		log.Fatalf("listen: %v", err)
	}
}
