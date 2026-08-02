package httpserver

import (
	"bytes"
	"mime/multipart"
	"net/http/httptest"
	"strings"
	"testing"

	mailimap "github.com/takayoshiotake/shiroyagi/internal/imap"
	"github.com/takayoshiotake/shiroyagi/internal/mailaccount"
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

func TestSMTPSecuritySelectedDefaultsToPlain(t *testing.T) {
	if smtpSecuritySelected("", "plain") != " selected" {
		t.Fatal("smtpSecuritySelected() did not default empty security to plain")
	}
	if smtpSecuritySelected("", "starttls") != "" {
		t.Fatal("smtpSecuritySelected() selected starttls for empty security")
	}
	if smtpSecuritySelected("starttls", "starttls") != " selected" {
		t.Fatal("smtpSecuritySelected() did not keep saved starttls selection")
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

func TestParseComposeMessageFormDefaultsToNew(t *testing.T) {
	req := httptest.NewRequest("POST", "/mail-accounts/account-1/send", strings.NewReader("to=to%40example.test&subject=Hello&body=Body"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if err := req.ParseForm(); err != nil {
		t.Fatalf("ParseForm() error = %v", err)
	}

	form, ok := parseComposeMessageForm(req)
	if !ok {
		t.Fatal("parseComposeMessageForm() ok = false, want true")
	}
	if form.Mode != composeModeNew || form.To != "to@example.test" || form.Subject != "Hello" || form.Body != "Body" {
		t.Fatalf("parseComposeMessageForm() = %+v", form)
	}
}

func TestParseComposeMessageFormReadsReplyMetadata(t *testing.T) {
	req := httptest.NewRequest("POST", "/mail-accounts/account-1/send", strings.NewReader("mode=reply&mailbox=INBOX&uid=42&to=to%40example.test&cc=cc%40example.test&subject=Re%3A+Hello&body=Body"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if err := req.ParseForm(); err != nil {
		t.Fatalf("ParseForm() error = %v", err)
	}

	form, ok := parseComposeMessageForm(req)
	if !ok {
		t.Fatal("parseComposeMessageForm() ok = false, want true")
	}
	if form.Mode != composeModeReply || form.Mailbox != "INBOX" || form.UID != 42 || form.To != "to@example.test" || form.Cc != "cc@example.test" || form.Subject != "Re: Hello" || form.Body != "Body" {
		t.Fatalf("parseComposeMessageForm() = %+v", form)
	}
}

func TestParseComposeMessageFormReadsForwardMetadata(t *testing.T) {
	req := httptest.NewRequest("POST", "/mail-accounts/account-1/send", strings.NewReader("mode=forward&mailbox=INBOX&uid=42&to=to%40example.test&cc=cc%40example.test&subject=Fwd%3A+Hello&body=Body"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if err := req.ParseForm(); err != nil {
		t.Fatalf("ParseForm() error = %v", err)
	}

	form, ok := parseComposeMessageForm(req)
	if !ok {
		t.Fatal("parseComposeMessageForm() ok = false, want true")
	}
	if form.Mode != composeModeForward || form.Mailbox != "INBOX" || form.UID != 42 || form.To != "to@example.test" || form.Cc != "cc@example.test" || form.Subject != "Fwd: Hello" || form.Body != "Body" {
		t.Fatalf("parseComposeMessageForm() = %+v", form)
	}
}

func TestParseComposeMessageFormRequiresOriginalMetadata(t *testing.T) {
	req := httptest.NewRequest("POST", "/mail-accounts/account-1/send", strings.NewReader("mode=forward&to=to%40example.test&subject=Fwd%3A+Hello&body=Body"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if err := req.ParseForm(); err != nil {
		t.Fatalf("ParseForm() error = %v", err)
	}

	if _, ok := parseComposeMessageForm(req); ok {
		t.Fatal("parseComposeMessageForm() ok = true, want false")
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

func TestForwardSubject(t *testing.T) {
	if got := forwardSubject("Hello"); got != "Fwd: Hello" {
		t.Fatalf("forwardSubject() = %q, want Fwd: Hello", got)
	}
	if got := forwardSubject("Fwd: Hello"); got != "Fwd: Hello" {
		t.Fatalf("forwardSubject() = %q, want unchanged subject", got)
	}
	if got := forwardSubject("FW: Hello"); got != "FW: Hello" {
		t.Fatalf("forwardSubject() = %q, want unchanged subject", got)
	}
}

func TestMessageStatusIncludesForwarded(t *testing.T) {
	got := messageStatus(mailimap.MessageSummary{Answered: true, Forwarded: true})
	if got != "Replied, Forwarded" {
		t.Fatalf("messageStatus() = %q, want Replied, Forwarded", got)
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

func TestForwardedBodyIncludesHeadersAndBody(t *testing.T) {
	got := forwardedBody(mailimap.Message{
		Subject: "Hello",
		From:    "Alice Example <alice@example.test>",
		To:      "Dev User <dev@example.test>",
		Cc:      "Carol Example <carol@example.test>",
		Body:    "Original body",
	})
	for _, want := range []string{
		"---------- Forwarded message ---------",
		"From: Alice Example <alice@example.test>",
		"Subject: Hello",
		"To: Dev User <dev@example.test>",
		"Cc: Carol Example <carol@example.test>",
		"Original body",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("forwardedBody() missing %q in:\n%s", want, got)
		}
	}
}

func TestSafeDownloadFilenameSeparatesOriginalDisplayName(t *testing.T) {
	original := "../../invoice.exe"
	if got := displayAttachmentFilename(original); got != original {
		t.Fatalf("display filename = %q, want original %q", got, original)
	}
	if got := safeDownloadFilename(original); got != "invoice.exe.download" {
		t.Fatalf("download filename = %q, want invoice.exe.download", got)
	}
}

func TestWriteAttachmentDownloadForcesOpaqueDownload(t *testing.T) {
	recorder := httptest.NewRecorder()
	writeAttachmentDownload(recorder, mailimap.Attachment{Filename: "../../invoice.exe", Data: []byte("data")})
	response := recorder.Result()
	if got := response.Header.Get("Content-Type"); got != "application/octet-stream" {
		t.Fatalf("Content-Type = %q", got)
	}
	if got := response.Header.Get("Content-Disposition"); !strings.Contains(got, "attachment") || !strings.Contains(got, "invoice.exe.download") {
		t.Fatalf("Content-Disposition = %q", got)
	}
	if got := response.Header.Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("X-Content-Type-Options = %q", got)
	}
	if got := response.Header.Get("Cache-Control"); got != "private, no-store" {
		t.Fatalf("Cache-Control = %q", got)
	}
	if recorder.Body.String() != "data" {
		t.Fatalf("body = %q", recorder.Body.String())
	}
}

func TestRenderAttachmentListShowsEscapedOriginalFilename(t *testing.T) {
	recorder := httptest.NewRecorder()
	renderAttachmentList(recorder, mailaccount.Detail{ID: "account-1"}, "INBOX", mailimap.Message{
		UID: 42,
		Attachments: []mailimap.Attachment{{
			PartID: "2", Filename: "../<original>.txt", ContentType: "text/plain", Size: 4,
		}},
	})
	body := recorder.Body.String()
	for _, want := range []string{"../&lt;original&gt;.txt", "/messages/42/attachments/2", "text/plain", "4 bytes"} {
		if !strings.Contains(body, want) {
			t.Fatalf("attachment list missing %q in %s", want, body)
		}
	}
	if strings.Contains(body, "../<original>.txt") {
		t.Fatalf("attachment list contains unescaped filename: %s", body)
	}
}

func TestReadUploadedAttachments(t *testing.T) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("attachments", "report.txt")
	if err != nil {
		t.Fatalf("CreateFormFile() error = %v", err)
	}
	_, _ = part.Write([]byte("report"))
	if err := writer.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	req := httptest.NewRequest("POST", "/mail-accounts/account-1/send", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if err := req.ParseMultipartForm(1024); err != nil {
		t.Fatalf("ParseMultipartForm() error = %v", err)
	}
	defer req.MultipartForm.RemoveAll()

	attachments, err := readUploadedAttachments(req)
	if err != nil {
		t.Fatalf("readUploadedAttachments() error = %v", err)
	}
	if len(attachments) != 1 || attachments[0].Filename != "report.txt" || string(attachments[0].Data) != "report" {
		t.Fatalf("attachments = %#v", attachments)
	}
}
