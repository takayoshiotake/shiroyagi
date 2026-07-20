package httpserver

import (
	"fmt"
	"html"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	mailimap "github.com/takayoshiotake/shiroyagi/internal/imap"
	"github.com/takayoshiotake/shiroyagi/internal/mailaccount"
	mailsmtp "github.com/takayoshiotake/shiroyagi/internal/smtp"
)

type forwardMessageForm struct {
	To      string
	Cc      string
	Subject string
	Body    string
}

func (s *Server) handleNewForwardMessage(w http.ResponseWriter, r *http.Request) {
	forwardContext, ok := s.loadForwardContext(w, r)
	if !ok {
		return
	}

	form := forwardMessageForm{
		Subject: forwardSubject(forwardContext.original.Subject),
		Body:    forwardedBody(forwardContext.original),
	}
	renderForwardMessageForm(w, forwardContext.account, forwardContext.mailbox, forwardContext.original, form)
}

func (s *Server) handleSendForwardMessage(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	form, ok := parseForwardMessageForm(r)
	if !ok {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	forwardContext, ok := s.loadForwardContext(w, r)
	if !ok {
		return
	}

	session, _ := sessionFromContext(r.Context())
	smtpAccount, err := s.smtpSenderAccount(session.Subject, forwardContext.account)
	if err != nil {
		log.Printf("prepare smtp account %s: %v", forwardContext.account.ID, err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	message := mailsmtp.Message{
		From:    forwardContext.account.EmailAddress,
		To:      form.To,
		Cc:      form.Cc,
		Subject: form.Subject,
		Body:    form.Body,
	}
	if err := mailsmtp.Send(r.Context(), smtpAccount, message); err != nil {
		log.Printf("send forward message account=%s mailbox=%q uid=%d: %v", forwardContext.account.ID, forwardContext.mailbox, forwardContext.original.UID, err)
		renderSMTPError(w, forwardContext.account, "Could not send forward: "+err.Error())
		return
	}

	var forwardedWarning string
	imapAccount, err := s.imapReaderAccount(session.Subject, forwardContext.account)
	if err != nil {
		log.Printf("prepare imap account %s for forwarded flag: %v", forwardContext.account.ID, err)
		forwardedWarning = "Forward sent, but the original message could not be marked as forwarded."
	} else if err := mailimap.MarkForwarded(r.Context(), imapAccount, forwardContext.mailbox, forwardContext.original.UID); err != nil {
		log.Printf("mark forwarded account=%s mailbox=%q uid=%d: %v", forwardContext.account.ID, forwardContext.mailbox, forwardContext.original.UID, err)
		forwardedWarning = "Forward sent, but the original message could not be marked as forwarded."
	}

	renderForwardSent(w, forwardContext.account, forwardContext.mailbox, forwardContext.original, form, forwardedWarning)
}

type forwardContext struct {
	account  mailaccount.Detail
	mailbox  string
	original mailimap.Message
}

func (s *Server) loadForwardContext(w http.ResponseWriter, r *http.Request) (forwardContext, bool) {
	id := r.PathValue("id")
	mailbox := r.PathValue("mailbox")
	if mailbox == "" {
		http.NotFound(w, r)
		return forwardContext{}, false
	}

	uid, err := strconv.ParseUint(r.PathValue("uid"), 10, 32)
	if err != nil || uid == 0 {
		http.NotFound(w, r)
		return forwardContext{}, false
	}

	session, _ := sessionFromContext(r.Context())
	account, found, err := s.accounts.Get(r.Context(), session.Subject, id)
	if err != nil {
		log.Printf("get mail account for forward: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return forwardContext{}, false
	}
	if !found {
		http.NotFound(w, r)
		return forwardContext{}, false
	}
	if !account.HasIMAP {
		renderIMAPError(w, account, "IMAP account is not configured.")
		return forwardContext{}, false
	}
	if !account.HasSMTP {
		renderSMTPError(w, account, "SMTP account is not configured.")
		return forwardContext{}, false
	}

	imapAccount, err := s.imapReaderAccount(session.Subject, account)
	if err != nil {
		log.Printf("prepare imap account %s for forward: %v", account.ID, err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return forwardContext{}, false
	}

	original, err := mailimap.GetMessage(r.Context(), imapAccount, mailbox, uint32(uid))
	if err != nil {
		log.Printf("get original message for forward account=%s mailbox=%q uid=%d: %v", account.ID, mailbox, uid, err)
		renderIMAPError(w, account, "Could not load original message: "+err.Error())
		return forwardContext{}, false
	}

	return forwardContext{
		account:  account,
		mailbox:  mailbox,
		original: original,
	}, true
}

func parseForwardMessageForm(r *http.Request) (forwardMessageForm, bool) {
	form := forwardMessageForm{
		To:      r.FormValue("to"),
		Cc:      r.FormValue("cc"),
		Subject: r.FormValue("subject"),
		Body:    r.FormValue("body"),
	}
	if form.To == "" || form.Subject == "" || form.Body == "" {
		return forwardMessageForm{}, false
	}
	return form, true
}

func renderForwardMessageForm(w http.ResponseWriter, account mailaccount.Detail, mailbox string, original mailimap.Message, form forwardMessageForm) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = fmt.Fprintf(w, `<!doctype html>
<html>
<head><title>Forward message - Shiroyagi</title></head>
<body>
  <p><a href="/mail-accounts/%s/mailboxes/%s/messages/%d">Back to message</a></p>
  <h1>Forward message</h1>

  <p>
    <strong>From</strong><br>
    %s
  </p>
  <h2>Original message</h2>
  <dl>
    <dt>From</dt><dd>%s</dd>
    <dt>To</dt><dd>%s</dd>
    <dt>Cc</dt><dd>%s</dd>
    <dt>Reply-To</dt><dd>%s</dd>
    <dt>Date</dt><dd>%s</dd>
    <dt>Message-ID</dt><dd>%s</dd>
    <dt>In-Reply-To</dt><dd>%s</dd>
    <dt>References</dt><dd>%s</dd>
  </dl>

  <form method="post" action="/mail-accounts/%s/mailboxes/%s/messages/%d/forward">
    <p>
      <label>To<br>
        <input name="to" value="%s" required>
      </label>
    </p>
    <p>
      <label>Cc<br>
        <input name="cc" value="%s">
      </label>
    </p>
    <p>
      <label>Subject<br>
        <input name="subject" value="%s" required>
      </label>
    </p>
    <p>
      <label>Body<br>
        <textarea name="body" rows="18" cols="72" required>%s</textarea>
      </label>
    </p>
    <button type="submit">Send forward</button>
  </form>
</body>
</html>`,
		html.EscapeString(account.ID),
		url.PathEscape(mailbox),
		original.UID,
		html.EscapeString(account.EmailAddress),
		html.EscapeString(messageHeaderValue(original.From)),
		html.EscapeString(messageHeaderValue(original.To)),
		html.EscapeString(messageHeaderValue(original.Cc)),
		html.EscapeString(messageHeaderValue(original.ReplyTo)),
		html.EscapeString(formatIMAPTime(original.Date)),
		html.EscapeString(messageHeaderValue(original.MessageID)),
		html.EscapeString(messageHeaderValue(original.InReplyTo)),
		html.EscapeString(messageHeaderValue(original.References)),
		html.EscapeString(account.ID),
		url.PathEscape(mailbox),
		original.UID,
		html.EscapeString(form.To),
		html.EscapeString(form.Cc),
		html.EscapeString(form.Subject),
		html.EscapeString(form.Body),
	)
}

func renderForwardSent(w http.ResponseWriter, account mailaccount.Detail, mailbox string, original mailimap.Message, form forwardMessageForm, forwardedWarning string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = fmt.Fprintf(w, `<!doctype html>
<html>
<head><title>Forward sent - Shiroyagi</title></head>
<body>
  <h1>Forward sent</h1>
  <p><strong>%s</strong> to <strong>%s</strong></p>`,
		html.EscapeString(account.EmailAddress),
		html.EscapeString(form.To),
	)
	if form.Cc != "" {
		_, _ = fmt.Fprintf(w, `
  <p><strong>Cc</strong> %s</p>`, html.EscapeString(form.Cc))
	}
	if forwardedWarning != "" {
		_, _ = fmt.Fprintf(w, `
  <p>%s</p>`, html.EscapeString(forwardedWarning))
	}
	_, _ = fmt.Fprintf(w, `
  <p><a href="/mail-accounts/%s/mailboxes/%s/messages/%d">Back to message</a></p>
  <p><a href="/mail-accounts/%s/mailboxes/%s">Back to %s</a></p>
</body>
</html>`,
		html.EscapeString(account.ID),
		url.PathEscape(mailbox),
		original.UID,
		html.EscapeString(account.ID),
		url.PathEscape(mailbox),
		html.EscapeString(mailbox),
	)
}

func forwardSubject(subject string) string {
	trimmed := strings.TrimSpace(subject)
	if trimmed == "" {
		return "Fwd: (no subject)"
	}
	lower := strings.ToLower(trimmed)
	if strings.HasPrefix(lower, "fwd:") || strings.HasPrefix(lower, "fw:") {
		return trimmed
	}
	return "Fwd: " + trimmed
}

func forwardedBody(original mailimap.Message) string {
	headerLines := []string{
		"From: " + messageHeaderValue(original.From),
		"Date: " + formatIMAPTime(original.Date),
		"Subject: " + subjectOrUntitled(original.Subject),
		"To: " + messageHeaderValue(original.To),
	}
	if original.Cc != "" {
		headerLines = append(headerLines, "Cc: "+original.Cc)
	}

	return "\n\n---------- Forwarded message ---------\n" + strings.Join(headerLines, "\n") + "\n\n" + original.Body
}
