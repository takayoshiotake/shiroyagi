package httpserver

import (
	"net/http/httptest"
	"strings"
	"testing"

	mailimap "github.com/takayoshiotake/shiroyagi/internal/imap"
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

func TestParseReplyMessageForm(t *testing.T) {
	req := httptest.NewRequest("POST", "/mail-accounts/account-1/mailboxes/INBOX/messages/1/reply", strings.NewReader("to=to%40example.test&cc=cc%40example.test&subject=Re%3A+Hello&body=Body"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if err := req.ParseForm(); err != nil {
		t.Fatalf("ParseForm() error = %v", err)
	}

	form, ok := parseReplyMessageForm(req)
	if !ok {
		t.Fatal("parseReplyMessageForm() ok = false, want true")
	}
	if form.To != "to@example.test" || form.Cc != "cc@example.test" || form.Subject != "Re: Hello" || form.Body != "Body" {
		t.Fatalf("parseReplyMessageForm() = %+v", form)
	}
}

func TestReplyRecipientPrefersReplyToAddress(t *testing.T) {
	got := replyRecipient(mailimap.Message{
		From:    "Alice Example <alice@example.test>",
		ReplyTo: "Replies <reply@example.test>",
	})
	want := "reply@example.test"
	if got != want {
		t.Fatalf("replyRecipient() = %q, want %q", got, want)
	}
}

func TestReplySubject(t *testing.T) {
	if got := replySubject("Hello"); got != "Re: Hello" {
		t.Fatalf("replySubject() = %q, want Re: Hello", got)
	}
	if got := replySubject("Re: Hello"); got != "Re: Hello" {
		t.Fatalf("replySubject() = %q, want unchanged subject", got)
	}
}

func TestReplyAllRecipientsExcludesSelfAndDuplicates(t *testing.T) {
	to, cc := replyAllRecipients(mailimap.Message{
		From: "Alice Example <alice@example.test>",
		To:   "Dev User <dev@example.test>, Bob <bob@example.test>",
		Cc:   "bob@example.test, Carol <carol@example.test>",
	}, "dev@example.test")

	if to != "alice@example.test" {
		t.Fatalf("replyAllRecipients() to = %q, want alice@example.test", to)
	}
	if cc != "bob@example.test, carol@example.test" {
		t.Fatalf("replyAllRecipients() cc = %q, want bob@example.test, carol@example.test", cc)
	}
}

func TestReplyReferencesAppendsOriginalMessageID(t *testing.T) {
	got := replyReferences(mailimap.Message{
		MessageID:  "<child@example.test>",
		References: "<root@example.test>",
	})
	want := "<root@example.test> <child@example.test>"
	if got != want {
		t.Fatalf("replyReferences() = %q, want %q", got, want)
	}
}
