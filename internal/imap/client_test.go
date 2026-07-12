package imap

import "testing"

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
