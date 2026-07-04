package main

import (
	"context"
	"log"
	"net/http"

	"github.com/takayoshiotake/shiroyagi/internal/auth"
	"github.com/takayoshiotake/shiroyagi/internal/config"
	httpserver "github.com/takayoshiotake/shiroyagi/internal/http"
)

func main() {
	ctx := context.Background()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
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
