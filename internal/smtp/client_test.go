package smtp

import (
	"strings"
	"testing"
)

func TestBuildMessage(t *testing.T) {
	msg := buildMessage(Message{
		From:    "sender@example.test",
		To:      "recipient@example.test",
		Subject: "Hello",
		Body:    "line 1\nline 2",
	})
	got := string(msg)
	for _, want := range []string{
		"From: sender@example.test\r\n",
		"To: recipient@example.test\r\n",
		"Subject: Hello\r\n",
		"Content-Type: text/plain; charset=UTF-8\r\n",
		"\r\nline 1\r\nline 2",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("message missing %q in:\n%s", want, got)
		}
	}
}

func TestNormalizeBody(t *testing.T) {
	got := normalizeBody("a\r\nb\rc\nd")
	want := "a\r\nb\r\nc\r\nd"
	if got != want {
		t.Fatalf("normalizeBody() = %q, want %q", got, want)
	}
}

func TestValidateMessageRejectsInvalidAddress(t *testing.T) {
	err := validateMessage(Message{
		From:    "sender@example.test",
		To:      "bad\naddress@example.test",
		Subject: "Hello",
		Body:    "Body",
	})
	if err == nil {
		t.Fatal("validateMessage() error = nil, want error")
	}
}

func TestSendRequiresAuth(t *testing.T) {
	err := Send(t.Context(), Account{
		Host:     "localhost",
		Port:     1025,
		Security: SecurityPlain,
	}, Message{
		From:    "sender@example.test",
		To:      "recipient@example.test",
		Subject: "Hello",
		Body:    "Body",
	})
	if err == nil {
		t.Fatal("Send() error = nil, want error")
	}
}
