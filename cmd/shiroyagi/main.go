package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/takayoshiotake/shiroyagi/internal/auth"
	"github.com/takayoshiotake/shiroyagi/internal/config"
	"github.com/takayoshiotake/shiroyagi/internal/db"
	httpserver "github.com/takayoshiotake/shiroyagi/internal/http"
	"github.com/takayoshiotake/shiroyagi/internal/mailaccount"
	"github.com/takayoshiotake/shiroyagi/internal/version"
)

func main() {
	if len(os.Args) == 2 && (os.Args[1] == "--version" || os.Args[1] == "-v") {
		fmt.Println(version.String())
		return
	}

	ctx := context.Background()
	log.Printf("start %s", version.String())

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

	server := httpserver.New(authClient, auth.NewSessionStore(), cfg.MailCrypto, mailaccount.NewStore(database))

	log.Println("listening on :8080")
	if err := http.ListenAndServe(":8080", server.Routes()); err != nil {
		log.Fatalf("listen: %v", err)
	}
}
