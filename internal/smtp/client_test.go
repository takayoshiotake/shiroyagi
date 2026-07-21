package smtp

import (
	"bytes"
	"encoding/base64"
	"io"
	"mime"
	"mime/multipart"
	"net/mail"
	"net/smtp"
	"strings"
	"testing"
)

func TestBuildMessage(t *testing.T) {
	msg := Message{
		From:    "sender@example.test",
		To:      "recipient@example.test",
		Subject: "Hello",
		Body:    "line 1\nline 2",
	}
	encoded, err := buildMessage(msg)
	if err != nil {
		t.Fatalf("buildMessage() error = %v", err)
	}
	got := string(encoded)
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

func TestBuildMessageAddsReplyHeaders(t *testing.T) {
	msg := Message{
		From:       "sender@example.test",
		To:         "recipient@example.test",
		Cc:         "copy@example.test",
		Subject:    "Re: Hello",
		Body:       "reply",
		InReplyTo:  "<original@example.test>",
		References: "<root@example.test> <original@example.test>",
	}
	encoded, err := buildMessage(msg)
	if err != nil {
		t.Fatalf("buildMessage() error = %v", err)
	}
	got := string(encoded)
	for _, want := range []string{
		"Cc: copy@example.test\r\n",
		"In-Reply-To: <original@example.test>\r\n",
		"References: <root@example.test> <original@example.test>\r\n",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("message missing %q in:\n%s", want, got)
		}
	}
}

func TestBuildMessageAddsMultipartAttachment(t *testing.T) {
	encoded, err := buildMessage(Message{
		From: "sender@example.test", To: "recipient@example.test", Subject: "Files", Body: "body",
		Attachments: []Attachment{{Filename: "report.pdf", ContentType: "application/pdf", Data: []byte("pdf")}},
	})
	if err != nil {
		t.Fatalf("buildMessage() error = %v", err)
	}
	message, err := mail.ReadMessage(bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("ReadMessage() error = %v", err)
	}
	_, params, err := mime.ParseMediaType(message.Header.Get("Content-Type"))
	if err != nil {
		t.Fatalf("ParseMediaType() error = %v", err)
	}
	reader := multipart.NewReader(message.Body, params["boundary"])
	if _, err := reader.NextPart(); err != nil {
		t.Fatalf("read text part: %v", err)
	}
	attachment, err := reader.NextPart()
	if err != nil {
		t.Fatalf("read attachment part: %v", err)
	}
	if attachment.FileName() != "report.pdf" || attachment.Header.Get("Content-Type") != "application/pdf" {
		t.Fatalf("attachment headers = %#v", attachment.Header)
	}
	data, err := io.ReadAll(base64.NewDecoder(base64.StdEncoding, attachment))
	if err != nil {
		t.Fatalf("read attachment: %v", err)
	}
	if string(data) != "pdf" {
		t.Fatalf("attachment data = %q, want pdf", data)
	}
}

func TestMessageRecipientsIncludesCc(t *testing.T) {
	got, err := messageRecipients(Message{
		To: "to@example.test",
		Cc: "Copy One <copy1@example.test>, copy2@example.test",
	})
	if err != nil {
		t.Fatalf("messageRecipients() error = %v", err)
	}
	want := []string{"to@example.test", "copy1@example.test", "copy2@example.test"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("messageRecipients() = %#v, want %#v", got, want)
	}
}

func TestSMTPAuthAllowsPlainModeWithoutTLS(t *testing.T) {
	auth := smtpAuth(Account{
		Host:              "mailpit",
		Security:          SecurityPlain,
		Username:          "dev@example.test",
		Password:          "dev",
		AllowInsecureAuth: true,
	})
	mechanism, response, err := auth.Start(nil)
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if mechanism != "PLAIN" {
		t.Fatalf("mechanism = %q, want PLAIN", mechanism)
	}
	if string(response) != "\x00dev@example.test\x00dev" {
		t.Fatalf("response = %q, want PLAIN auth response", response)
	}
}

func TestSMTPAuthRejectsPlainModeWithoutInsecureAuth(t *testing.T) {
	auth := smtpAuth(Account{
		Host:     "mailpit",
		Security: SecurityPlain,
		Username: "dev@example.test",
		Password: "dev",
	})
	_, _, err := auth.Start(&smtp.ServerInfo{Name: "mailpit"})
	if err == nil {
		t.Fatal("Start() error = nil, want error")
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
