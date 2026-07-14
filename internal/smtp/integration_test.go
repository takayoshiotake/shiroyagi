package smtp

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"
)

func TestSendIntegration(t *testing.T) {
	if os.Getenv("SHIROYAGI_SMTP_INTEGRATION") != "1" {
		t.Skip("set SHIROYAGI_SMTP_INTEGRATION=1 to run")
	}

	port, err := strconv.Atoi(envOrDefault("SHIROYAGI_SMTP_PORT", "1025"))
	if err != nil {
		t.Fatalf("invalid SHIROYAGI_SMTP_PORT: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = Send(ctx, Account{
		Host:     envOrDefault("SHIROYAGI_SMTP_HOST", "localhost"),
		Port:     port,
		Security: envOrDefault("SHIROYAGI_SMTP_SECURITY", SecurityPlain),
		Username: envOrDefault("SHIROYAGI_SMTP_USERNAME", "dev@example.test"),
		Password: envOrDefault("SHIROYAGI_SMTP_PASSWORD", "dev"),
	}, Message{
		From:    envOrDefault("SHIROYAGI_SMTP_FROM", "dev@example.test"),
		To:      envOrDefault("SHIROYAGI_SMTP_TO", "recipient@example.test"),
		Subject: "Shiroyagi SMTP integration test",
		Body:    "This message was sent by the Shiroyagi SMTP integration test.",
	})
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}
}

func envOrDefault(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
