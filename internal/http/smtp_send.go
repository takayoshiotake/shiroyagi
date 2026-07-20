package httpserver

import (
	"fmt"
	"html"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"

	"github.com/takayoshiotake/shiroyagi/internal/crypto"
	mailimap "github.com/takayoshiotake/shiroyagi/internal/imap"
	"github.com/takayoshiotake/shiroyagi/internal/mailaccount"
	mailsmtp "github.com/takayoshiotake/shiroyagi/internal/smtp"
)

type composeMode string

const (
	composeModeNew      composeMode = "new"
	composeModeReply    composeMode = "reply"
	composeModeReplyAll composeMode = "reply-all"
	composeModeForward  composeMode = "forward"
)

type composeMessageForm struct {
	Mode    composeMode
	Mailbox string
	UID     uint32
	To      string
	Cc      string
	Subject string
	Body    string
}

func (s *Server) handleNewMessage(w http.ResponseWriter, r *http.Request) {
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

	renderMessageForm(w, account, composeMessageForm{Mode: composeModeNew})
}

func (s *Server) handleSendMessage(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	form, ok := parseComposeMessageForm(r)
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

	original, ok := s.loadComposeOriginal(w, r, session.Subject, account, form)
	if !ok {
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
		Cc:      form.Cc,
		Subject: form.Subject,
		Body:    form.Body,
	}
	if form.Mode == composeModeReply || form.Mode == composeModeReplyAll {
		if original.MessageID == "" {
			renderIMAPError(w, account, "Could not prepare reply: original message has no Message-ID.")
			return
		}
		message.InReplyTo = original.MessageID
		message.References = replyReferences(original)
	}
	if err := mailsmtp.Send(r.Context(), smtpAccount, message); err != nil {
		log.Printf("send %s message account=%s mailbox=%q uid=%d: %v", form.Mode, account.ID, form.Mailbox, form.UID, err)
		renderSMTPError(w, account, "Could not send "+form.sentNoun()+": "+err.Error())
		return
	}

	warning := s.applyComposeSideEffect(r, session.Subject, account, form)
	renderComposeSent(w, account, original, form, warning)
}

func (s *Server) loadComposeOriginal(w http.ResponseWriter, r *http.Request, userID string, account mailaccount.Detail, form composeMessageForm) (mailimap.Message, bool) {
	if !form.requiresOriginal() {
		return mailimap.Message{}, true
	}
	if !account.HasIMAP {
		renderIMAPError(w, account, "IMAP account is not configured.")
		return mailimap.Message{}, false
	}

	imapAccount, err := s.imapReaderAccount(userID, account)
	if err != nil {
		log.Printf("prepare imap account %s for %s: %v", account.ID, form.Mode, err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return mailimap.Message{}, false
	}

	original, err := mailimap.GetMessage(r.Context(), imapAccount, form.Mailbox, form.UID)
	if err != nil {
		log.Printf("get original message for %s account=%s mailbox=%q uid=%d: %v", form.Mode, account.ID, form.Mailbox, form.UID, err)
		renderIMAPError(w, account, "Could not load original message: "+err.Error())
		return mailimap.Message{}, false
	}
	return original, true
}

func (s *Server) applyComposeSideEffect(r *http.Request, userID string, account mailaccount.Detail, form composeMessageForm) string {
	if form.Mode != composeModeReply && form.Mode != composeModeReplyAll && form.Mode != composeModeForward {
		return ""
	}

	imapAccount, err := s.imapReaderAccount(userID, account)
	if err != nil {
		log.Printf("prepare imap account %s for %s flag: %v", account.ID, form.Mode, err)
		return form.flagWarning()
	}
	switch form.Mode {
	case composeModeReply, composeModeReplyAll:
		if err := mailimap.MarkAnswered(r.Context(), imapAccount, form.Mailbox, form.UID); err != nil {
			log.Printf("mark answered account=%s mailbox=%q uid=%d: %v", account.ID, form.Mailbox, form.UID, err)
			return form.flagWarning()
		}
	case composeModeForward:
		if err := mailimap.MarkForwarded(r.Context(), imapAccount, form.Mailbox, form.UID); err != nil {
			log.Printf("mark forwarded account=%s mailbox=%q uid=%d: %v", account.ID, form.Mailbox, form.UID, err)
			return form.flagWarning()
		}
	}
	return ""
}

func renderComposeSent(w http.ResponseWriter, account mailaccount.Detail, original mailimap.Message, form composeMessageForm, warning string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = fmt.Fprintf(w, `<!doctype html>
<html>
<head><title>%s - Shiroyagi</title></head>
<body>
  <h1>%s</h1>
  <p><strong>%s</strong> to <strong>%s</strong></p>`,
		html.EscapeString(form.sentTitle()),
		html.EscapeString(form.sentTitle()),
		html.EscapeString(account.EmailAddress),
		html.EscapeString(form.To),
	)
	if form.Cc != "" {
		_, _ = fmt.Fprintf(w, `
  <p><strong>Cc</strong> %s</p>`, html.EscapeString(form.Cc))
	}
	if warning != "" {
		_, _ = fmt.Fprintf(w, `
  <p>%s</p>`, html.EscapeString(warning))
	}
	if form.requiresOriginal() {
		_, _ = fmt.Fprintf(w, `
  <p><a href="/mail-accounts/%s/mailboxes/%s/messages/%d">Back to message</a></p>
  <p><a href="/mail-accounts/%s/mailboxes/%s">Back to %s</a></p>`,
			html.EscapeString(account.ID),
			url.PathEscape(form.Mailbox),
			original.UID,
			html.EscapeString(account.ID),
			url.PathEscape(form.Mailbox),
			html.EscapeString(form.Mailbox),
		)
	} else {
		_, _ = fmt.Fprintf(w, `
  <p><a href="/mail-accounts/%s/send">Send another message</a></p>
  <p><a href="/mail-accounts">Back</a></p>`,
			html.EscapeString(account.ID),
		)
	}
	_, _ = fmt.Fprint(w, `
</body>
</html>`)
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

func parseComposeMessageForm(r *http.Request) (composeMessageForm, bool) {
	mode := composeMode(r.FormValue("mode"))
	if mode == "" {
		mode = composeModeNew
	}
	form := composeMessageForm{
		Mode:    mode,
		Mailbox: r.FormValue("mailbox"),
		To:      r.FormValue("to"),
		Cc:      r.FormValue("cc"),
		Subject: r.FormValue("subject"),
		Body:    r.FormValue("body"),
	}
	if !form.Mode.valid() {
		return composeMessageForm{}, false
	}
	if form.requiresOriginal() {
		uid, err := strconv.ParseUint(r.FormValue("uid"), 10, 32)
		if err != nil || uid == 0 || form.Mailbox == "" {
			return composeMessageForm{}, false
		}
		form.UID = uint32(uid)
	}
	if form.To == "" || form.Subject == "" || form.Body == "" {
		return composeMessageForm{}, false
	}
	return form, true
}

func (mode composeMode) valid() bool {
	return mode == composeModeNew || mode == composeModeReply || mode == composeModeReplyAll || mode == composeModeForward
}

func (form composeMessageForm) requiresOriginal() bool {
	return form.Mode == composeModeReply || form.Mode == composeModeReplyAll || form.Mode == composeModeForward
}

func (form composeMessageForm) sentTitle() string {
	switch form.Mode {
	case composeModeReply, composeModeReplyAll:
		return "Reply sent"
	case composeModeForward:
		return "Forward sent"
	default:
		return "Message sent"
	}
}

func (form composeMessageForm) sentNoun() string {
	switch form.Mode {
	case composeModeReply, composeModeReplyAll:
		return "reply"
	case composeModeForward:
		return "forward"
	default:
		return "message"
	}
}

func (form composeMessageForm) flagWarning() string {
	switch form.Mode {
	case composeModeReply, composeModeReplyAll:
		return "Reply sent, but the original message could not be marked as answered."
	case composeModeForward:
		return "Forward sent, but the original message could not be marked as forwarded."
	default:
		return ""
	}
}

func renderMessageForm(w http.ResponseWriter, account mailaccount.Detail, form composeMessageForm) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = fmt.Fprintf(w, `<!doctype html>
<html>
<head><title>Compose message - Shiroyagi</title></head>
<body>
  <h1>Compose message</h1>

  <p>
    <strong>From</strong><br>
    %s
  </p>

  <form method="post" action="/mail-accounts/%s/send">
    <input type="hidden" name="mode" value="new">
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
