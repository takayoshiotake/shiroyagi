package httpserver

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMailAccountAAD(t *testing.T) {
	gotEnvelopeAAD := string(envelopeAAD("user-1", "account-1"))
	wantEnvelopeAAD := "user-1:account-1"
	if gotEnvelopeAAD != wantEnvelopeAAD {
		t.Fatalf("envelopeAAD() = %q, want %q", gotEnvelopeAAD, wantEnvelopeAAD)
	}

	gotFieldAAD := string(fieldAAD("user-1", "account-1", aadFieldIMAPPassword))
	wantFieldAAD := "user-1:account-1:imap_password"
	if gotFieldAAD != wantFieldAAD {
		t.Fatalf("fieldAAD() = %q, want %q", gotFieldAAD, wantFieldAAD)
	}

	gotSMTPFieldAAD := string(fieldAAD("user-1", "account-1", aadFieldSMTPPassword))
	wantSMTPFieldAAD := "user-1:account-1:smtp_password"
	if gotSMTPFieldAAD != wantSMTPFieldAAD {
		t.Fatalf("fieldAAD() = %q, want %q", gotSMTPFieldAAD, wantSMTPFieldAAD)
	}
}

func TestSelected(t *testing.T) {
	if selected(true) != " selected" {
		t.Fatalf("selected(true) = %q, want selected attribute", selected(true))
	}
	if selected(false) != "" {
		t.Fatalf("selected(false) = %q, want empty string", selected(false))
	}
}

func TestParseSMTPFormRequiresAuth(t *testing.T) {
	req := httptest.NewRequest("POST", "/mail-accounts/account-1/smtp/save", strings.NewReader("smtp_host=localhost&smtp_port=1025&smtp_security=plain"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if err := req.ParseForm(); err != nil {
		t.Fatalf("ParseForm() error = %v", err)
	}

	if _, ok := parseSMTPForm(req); ok {
		t.Fatal("parseSMTPForm() ok = true, want false")
	}
}

func TestParseSMTPForm(t *testing.T) {
	req := httptest.NewRequest("POST", "/mail-accounts/account-1/smtp/save", strings.NewReader("smtp_host=localhost&smtp_port=1025&smtp_security=plain&smtp_username=dev%40example.test&smtp_password=dev"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if err := req.ParseForm(); err != nil {
		t.Fatalf("ParseForm() error = %v", err)
	}

	form, ok := parseSMTPForm(req)
	if !ok {
		t.Fatal("parseSMTPForm() ok = false, want true")
	}
	if form.SMTPUsername != "dev@example.test" || form.SMTPPassword != "dev" {
		t.Fatalf("parseSMTPForm() auth = %q/%q", form.SMTPUsername, form.SMTPPassword)
	}
}

func TestParseTestMessageForm(t *testing.T) {
	req := httptest.NewRequest("POST", "/mail-accounts/account-1/send", strings.NewReader("to=to%40example.test&subject=Hello&body=Body"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if err := req.ParseForm(); err != nil {
		t.Fatalf("ParseForm() error = %v", err)
	}

	form, ok := parseTestMessageForm(req)
	if !ok {
		t.Fatal("parseTestMessageForm() ok = false, want true")
	}
	if form.To != "to@example.test" || form.Subject != "Hello" || form.Body != "Body" {
		t.Fatalf("parseTestMessageForm() = %+v", form)
	}
}
