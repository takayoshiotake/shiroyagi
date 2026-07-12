package imap

import (
	"os"
	"strconv"
	"testing"
)

func TestIntegrationListMessages(t *testing.T) {
	if os.Getenv("SHIROYAGI_IMAP_INTEGRATION") == "" {
		t.Skip("set SHIROYAGI_IMAP_INTEGRATION=1 to run")
	}

	port, err := strconv.Atoi(os.Getenv("SHIROYAGI_IMAP_PORT"))
	if err != nil {
		t.Fatalf("parse SHIROYAGI_IMAP_PORT: %v", err)
	}

	messages, err := ListMessages(t.Context(), Account{
		Host:     os.Getenv("SHIROYAGI_IMAP_HOST"),
		Port:     port,
		Security: os.Getenv("SHIROYAGI_IMAP_SECURITY"),
		Username: os.Getenv("SHIROYAGI_IMAP_USERNAME"),
		Password: os.Getenv("SHIROYAGI_IMAP_PASSWORD"),
	}, "INBOX", 100)
	if err != nil {
		t.Fatalf("ListMessages() error = %v", err)
	}
	if len(messages) == 0 {
		t.Fatal("ListMessages() returned no messages")
	}
}
