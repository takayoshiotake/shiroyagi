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
)

func (s *Server) handleNewForwardMessage(w http.ResponseWriter, r *http.Request) {
	forwardContext, ok := s.loadForwardContext(w, r)
	if !ok {
		return
	}

	form := composeMessageForm{
		Mode:    composeModeForward,
		Mailbox: forwardContext.mailbox,
		UID:     forwardContext.original.UID,
		Subject: forwardSubject(forwardContext.original.Subject),
		Body:    forwardedBody(forwardContext.original),
	}
	renderForwardMessageForm(w, forwardContext.account, forwardContext.mailbox, forwardContext.original, form)
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

func renderForwardMessageForm(w http.ResponseWriter, account mailaccount.Detail, mailbox string, original mailimap.Message, form composeMessageForm) {
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

  <form method="post" action="/mail-accounts/%s/send">
    <input type="hidden" name="mode" value="forward">
    <input type="hidden" name="mailbox" value="%s">
    <input type="hidden" name="uid" value="%d">
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
		html.EscapeString(mailbox),
		original.UID,
		html.EscapeString(form.To),
		html.EscapeString(form.Cc),
		html.EscapeString(form.Subject),
		html.EscapeString(form.Body),
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
