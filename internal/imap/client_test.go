package imap

import (
	"strings"
	"testing"
)

func TestExtractTextBodyPlain(t *testing.T) {
	body, err := extractTextBody([]byte("Subject: Hi\r\nContent-Type: text/plain\r\n\r\nhello\r\n"))
	if err != nil {
		t.Fatalf("extractTextBody() error = %v", err)
	}
	if body != "hello\r\n" {
		t.Fatalf("extractTextBody() = %q, want %q", body, "hello\r\n")
	}
}

func TestExtractTextBodyMultipartPrefersPlainText(t *testing.T) {
	raw := []byte("Subject: Hi\r\nContent-Type: multipart/alternative; boundary=frontier\r\n\r\n" +
		"--frontier\r\nContent-Type: text/html\r\n\r\n<p>html</p>\r\n" +
		"--frontier\r\nContent-Type: text/plain\r\n\r\nplain\r\n" +
		"--frontier--\r\n")

	body, err := extractTextBody(raw)
	if err != nil {
		t.Fatalf("extractTextBody() error = %v", err)
	}
	if body != "plain" {
		t.Fatalf("extractTextBody() = %q, want %q", body, "plain")
	}
}

func TestExtractTextBodyBase64(t *testing.T) {
	raw := []byte("Subject: Hi\r\nContent-Type: text/plain\r\nContent-Transfer-Encoding: base64\r\n\r\naGVsbG8=\r\n")

	body, err := extractTextBody(raw)
	if err != nil {
		t.Fatalf("extractTextBody() error = %v", err)
	}
	if body != "hello" {
		t.Fatalf("extractTextBody() = %q, want %q", body, "hello")
	}
}

func TestParseMessageContentListsAndDecodesAttachments(t *testing.T) {
	raw := []byte("Subject: Files\r\nContent-Type: multipart/mixed; boundary=mix\r\n\r\n" +
		"--mix\r\nContent-Type: text/plain\r\n\r\nhello\r\n" +
		"--mix\r\nContent-Type: application/pdf\r\nContent-Disposition: attachment; filename=report.pdf\r\nContent-Transfer-Encoding: base64\r\n\r\ncGRm\r\n" +
		"--mix--\r\n")

	parsed, err := parseMessageContent(raw)
	if err != nil {
		t.Fatalf("parseMessageContent() error = %v", err)
	}
	if parsed.body != "hello" {
		t.Fatalf("body = %q, want hello", parsed.body)
	}
	if len(parsed.attachments) != 1 {
		t.Fatalf("attachments = %#v, want one", parsed.attachments)
	}
	attachment := parsed.attachments[0]
	if attachment.PartID != "2" || attachment.Filename != "report.pdf" || attachment.ContentType != "application/pdf" || string(attachment.Data) != "pdf" {
		t.Fatalf("attachment = %#v", attachment)
	}
}

func TestParseMessageContentKeepsOriginalFilenameForDisplay(t *testing.T) {
	raw := []byte("Content-Type: application/octet-stream; name=\"../../invoice.exe\"\r\n" +
		"Content-Disposition: attachment; filename=\"../../invoice.exe\"\r\n\r\ndata")
	parsed, err := parseMessageContent(raw)
	if err != nil {
		t.Fatalf("parseMessageContent() error = %v", err)
	}
	if got := parsed.attachments[0].Filename; got != "../../invoice.exe" {
		t.Fatalf("filename = %q, want original filename", got)
	}
}

func TestParseMessageContentDoesNotRenderMalformedAttachmentAsBody(t *testing.T) {
	raw := []byte("Content-Type: text/html\r\n" +
		"Content-Disposition: attachment; filename=\"unterminated\r\n\r\n<script>bad()</script>")
	parsed, err := parseMessageContent(raw)
	if err != nil {
		t.Fatalf("parseMessageContent() error = %v", err)
	}
	if parsed.body != "(no text body)" || len(parsed.attachments) != 1 {
		t.Fatalf("parsed = %#v", parsed)
	}
}

func TestHasFlagIsCaseInsensitive(t *testing.T) {
	if !hasFlag([]string{"\\Seen", "$forwarded"}, "$Forwarded") {
		t.Fatal("hasFlag() = false, want true")
	}
}

func TestIsLocalhost(t *testing.T) {
	for _, host := range []string{"localhost", "LOCALHOST.", "127.0.0.1", "::1"} {
		if !isLocalhost(host) {
			t.Fatalf("isLocalhost(%q) = false, want true", host)
		}
	}
	if isLocalhost("mailpit") {
		t.Fatal("isLocalhost(mailpit) = true, want false")
	}
}

func TestValidateIMAPAuthRejectsPlainNonLocalhostWithoutOptIn(t *testing.T) {
	_, err := connect(t.Context(), Account{
		Host:     "mailpit",
		Port:     2143,
		Security: SecurityIMAP,
		Username: "dev@example.test",
		Password: "dev",
	})
	if err == nil {
		t.Fatal("connect() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "insecure IMAP auth is disabled") {
		t.Fatalf("connect() error = %q, want insecure auth error", err)
	}
}
