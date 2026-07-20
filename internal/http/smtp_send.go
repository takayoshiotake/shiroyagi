package httpserver

import (
	"fmt"
	"html"
	"log"
	"net/http"
	"os"

	"github.com/takayoshiotake/shiroyagi/internal/crypto"
	"github.com/takayoshiotake/shiroyagi/internal/mailaccount"
	mailsmtp "github.com/takayoshiotake/shiroyagi/internal/smtp"
)

type testMessageForm struct {
	To      string
	Subject string
	Body    string
}

func (s *Server) handleNewTestMessage(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	session, _ := sessionFromContext(r.Context())
	account, found, err := s.accounts.Get(r.Context(), session.Subject, id)
	if err != nil {
		log.Printf("get mail account for smtp send: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if !found {
		http.NotFound(w, r)
		return
	}
	if !account.HasSMTP {
		renderSMTPError(w, account, "SMTP account is not configured.")
		return
	}

	renderTestMessageForm(w, account, testMessageForm{})
}

func (s *Server) handleSendTestMessage(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	form, ok := parseTestMessageForm(r)
	if !ok {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	session, _ := sessionFromContext(r.Context())
	account, found, err := s.accounts.Get(r.Context(), session.Subject, id)
	if err != nil {
		log.Printf("get mail account for smtp send: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if !found {
		http.NotFound(w, r)
		return
	}
	if !account.HasSMTP {
		renderSMTPError(w, account, "SMTP account is not configured.")
		return
	}

	smtpAccount, err := s.smtpSenderAccount(session.Subject, account)
	if err != nil {
		log.Printf("prepare smtp account %s: %v", account.ID, err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	message := mailsmtp.Message{
		From:    account.EmailAddress,
		To:      form.To,
		Subject: form.Subject,
		Body:    form.Body,
	}
	if err := mailsmtp.Send(r.Context(), smtpAccount, message); err != nil {
		log.Printf("send test message account=%s: %v", account.ID, err)
		renderSMTPError(w, account, "Could not send message: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = fmt.Fprintf(w, `<!doctype html>
<html>
<head><title>Message sent - Shiroyagi</title></head>
<body>
  <h1>Message sent</h1>
  <p><strong>%s</strong> to <strong>%s</strong></p>
  <p><a href="/mail-accounts/%s/send">Send another message</a></p>
  <p><a href="/mail-accounts">Back</a></p>
</body>
</html>`,
		html.EscapeString(account.EmailAddress),
		html.EscapeString(form.To),
		html.EscapeString(account.ID),
	)
}

func (s *Server) smtpSenderAccount(userID string, account mailaccount.Detail) (mailsmtp.Account, error) {
	password, err := s.decryptSMTPPassword(userID, account)
	if err != nil {
		return mailsmtp.Account{}, err
	}
	return mailsmtp.Account{
		Host:              account.SMTPHost,
		Port:              account.SMTPPort,
		Security:          account.SMTPSecurity,
		Username:          account.SMTPUsername,
		Password:          password,
		AllowInsecureAuth: s.smtpConfig.AllowInsecureAuth,
	}, nil
}

func (s *Server) decryptSMTPPassword(userID string, account mailaccount.Detail) (string, error) {
	kek, err := os.ReadFile(s.mailCrypto.KEKFile)
	if err != nil {
		return "", fmt.Errorf("read mail account KEK: %w", err)
	}
	decrypter, err := crypto.OpenEnvelope(kek, crypto.Envelope{
		WrappedDEK: account.SMTPWrappedDEK,
		KEKVersion: account.SMTPKEKVersion,
	}, envelopeAAD(userID, account.ID))
	if err != nil {
		return "", fmt.Errorf("open smtp account envelope: %w", err)
	}

	plaintext, err := decrypter.DecryptWithAAD(account.EncryptedSMTPPassword, fieldAAD(userID, account.ID, aadFieldSMTPPassword))
	if err != nil {
		return "", fmt.Errorf("decrypt smtp password: %w", err)
	}
	return string(plaintext), nil
}

func parseTestMessageForm(r *http.Request) (testMessageForm, bool) {
	form := testMessageForm{
		To:      r.FormValue("to"),
		Subject: r.FormValue("subject"),
		Body:    r.FormValue("body"),
	}
	if form.To == "" || form.Subject == "" || form.Body == "" {
		return testMessageForm{}, false
	}
	return form, true
}

func renderTestMessageForm(w http.ResponseWriter, account mailaccount.Detail, form testMessageForm) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = fmt.Fprintf(w, `<!doctype html>
<html>
<head><title>Send test message - Shiroyagi</title></head>
<body>
  <h1>Send test message</h1>

  <p>
    <strong>From</strong><br>
    %s
  </p>

  <form method="post" action="/mail-accounts/%s/send">
    <p>
      <label>To<br>
        <input type="email" name="to" value="%s" required>
      </label>
    </p>
    <p>
      <label>Subject<br>
        <input name="subject" value="%s" required>
      </label>
    </p>
    <p>
      <label>Body<br>
        <textarea name="body" rows="12" cols="72" required>%s</textarea>
      </label>
    </p>
    <button type="submit">Send</button>
  </form>

  <p><a href="/mail-accounts">Back</a></p>
</body>
</html>`,
		html.EscapeString(account.EmailAddress),
		html.EscapeString(account.ID),
		html.EscapeString(form.To),
		html.EscapeString(form.Subject),
		html.EscapeString(form.Body),
	)
}

func renderSMTPError(w http.ResponseWriter, account mailaccount.Detail, message string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusBadGateway)
	_, _ = fmt.Fprintf(w, `<!doctype html>
<html>
<head><title>SMTP error - Shiroyagi</title></head>
<body>
  <h1>SMTP error</h1>
  <p><strong>%s</strong></p>
  <p>%s</p>
  <p><a href="/mail-accounts/%s/smtp/edit">Edit SMTP</a></p>
  <p><a href="/mail-accounts">Back</a></p>
</body>
</html>`,
		html.EscapeString(account.EmailAddress),
		html.EscapeString(message),
		html.EscapeString(account.ID),
	)
}
