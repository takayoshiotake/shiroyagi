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
	for _, summary := range messages {
		if summary.Subject != "Attachment fixture" {
			continue
		}
		message, err := GetMessage(t.Context(), Account{
			Host: os.Getenv("SHIROYAGI_IMAP_HOST"), Port: port, Security: os.Getenv("SHIROYAGI_IMAP_SECURITY"),
			Username: os.Getenv("SHIROYAGI_IMAP_USERNAME"), Password: os.Getenv("SHIROYAGI_IMAP_PASSWORD"),
		}, "INBOX", summary.UID)
		if err != nil {
			t.Fatalf("GetMessage() attachment fixture error = %v", err)
		}
		if len(message.Attachments) != 1 || message.Attachments[0].Filename != "../original-report.txt" {
			t.Fatalf("attachment fixture metadata = %#v", message.Attachments)
		}
		attachment, err := GetAttachment(t.Context(), Account{
			Host: os.Getenv("SHIROYAGI_IMAP_HOST"), Port: port, Security: os.Getenv("SHIROYAGI_IMAP_SECURITY"),
			Username: os.Getenv("SHIROYAGI_IMAP_USERNAME"), Password: os.Getenv("SHIROYAGI_IMAP_PASSWORD"),
		}, "INBOX", summary.UID, message.Attachments[0].PartID)
		if err != nil {
			t.Fatalf("GetAttachment() error = %v", err)
		}
		if string(attachment.Data) != "This is the attachment fixture.\n" {
			t.Fatalf("attachment data = %q", attachment.Data)
		}
		return
	}
	t.Fatal("Attachment fixture not found")
}
