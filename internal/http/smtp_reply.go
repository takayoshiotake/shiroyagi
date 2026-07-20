package httpserver

import (
	"fmt"
	"html"
	"log"
	"net/http"
	stdmail "net/mail"
	"net/url"
	"strconv"
	"strings"

	mailimap "github.com/takayoshiotake/shiroyagi/internal/imap"
	"github.com/takayoshiotake/shiroyagi/internal/mailaccount"
)

func (s *Server) handleNewReplyMessage(w http.ResponseWriter, r *http.Request) {
	s.handleNewReply(w, r, replyModeReply)
}

func (s *Server) handleNewReplyAllMessage(w http.ResponseWriter, r *http.Request) {
	s.handleNewReply(w, r, replyModeReplyAll)
}

func (s *Server) handleNewReply(w http.ResponseWriter, r *http.Request, mode replyMode) {
	replyContext, ok := s.loadReplyContext(w, r)
	if !ok {
		return
	}

	form := composeMessageForm{
		Mode:    composeMode(mode),
		Mailbox: replyContext.mailbox,
		UID:     replyContext.original.UID,
		To:      replyRecipient(replyContext.original),
		Subject: replySubject(replyContext.original.Subject),
		Body:    quotedReplyBody(replyContext.original),
	}
	if mode == replyModeReplyAll {
		form.To, form.Cc = replyAllRecipients(replyContext.original, replyContext.account.EmailAddress)
	}

	renderReplyMessageForm(w, replyContext.account, replyContext.mailbox, replyContext.original, mode, form)
}

type replyMode string

const (
	replyModeReply    replyMode = "reply"
	replyModeReplyAll replyMode = "reply-all"
)

func (mode replyMode) title() string {
	if mode == replyModeReplyAll {
		return "Reply all"
	}
	return "Reply message"
}

func (mode replyMode) action() string {
	if mode == replyModeReplyAll {
		return "reply-all"
	}
	return "reply"
}

func (mode replyMode) submitLabel() string {
	if mode == replyModeReplyAll {
		return "Send reply all"
	}
	return "Send reply"
}

type replyContext struct {
	account  mailaccount.Detail
	mailbox  string
	original mailimap.Message
}

func (s *Server) loadReplyContext(w http.ResponseWriter, r *http.Request) (replyContext, bool) {
	id := r.PathValue("id")
	mailbox := r.PathValue("mailbox")
	if mailbox == "" {
		http.NotFound(w, r)
		return replyContext{}, false
	}

	uid, err := strconv.ParseUint(r.PathValue("uid"), 10, 32)
	if err != nil || uid == 0 {
		http.NotFound(w, r)
		return replyContext{}, false
	}

	session, _ := sessionFromContext(r.Context())
	account, found, err := s.accounts.Get(r.Context(), session.Subject, id)
	if err != nil {
		log.Printf("get mail account for reply: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return replyContext{}, false
	}
	if !found {
		http.NotFound(w, r)
		return replyContext{}, false
	}
	if !account.HasIMAP {
		renderIMAPError(w, account, "IMAP account is not configured.")
		return replyContext{}, false
	}
	if !account.HasSMTP {
		renderSMTPError(w, account, "SMTP account is not configured.")
		return replyContext{}, false
	}

	imapAccount, err := s.imapReaderAccount(session.Subject, account)
	if err != nil {
		log.Printf("prepare imap account %s for reply: %v", account.ID, err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return replyContext{}, false
	}

	original, err := mailimap.GetMessage(r.Context(), imapAccount, mailbox, uint32(uid))
	if err != nil {
		log.Printf("get original message for reply account=%s mailbox=%q uid=%d: %v", account.ID, mailbox, uid, err)
		renderIMAPError(w, account, "Could not load original message: "+err.Error())
		return replyContext{}, false
	}

	return replyContext{
		account:  account,
		mailbox:  mailbox,
		original: original,
	}, true
}

func renderReplyMessageForm(w http.ResponseWriter, account mailaccount.Detail, mailbox string, original mailimap.Message, mode replyMode, form composeMessageForm) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = fmt.Fprintf(w, `<!doctype html>
<html>
<head><title>%s - Shiroyagi</title></head>
<body>
  <p><a href="/mail-accounts/%s/mailboxes/%s/messages/%d">Back to message</a></p>
  <h1>%s</h1>

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
    <input type="hidden" name="mode" value="%s">
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
        <textarea name="body" rows="16" cols="72" required>%s</textarea>
      </label>
    </p>
    <button type="submit">%s</button>
  </form>
</body>
</html>`,
		html.EscapeString(mode.title()),
		html.EscapeString(account.ID),
		url.PathEscape(mailbox),
		original.UID,
		html.EscapeString(mode.title()),
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
		html.EscapeString(mode.action()),
		html.EscapeString(mailbox),
		original.UID,
		html.EscapeString(form.To),
		html.EscapeString(form.Cc),
		html.EscapeString(form.Subject),
		html.EscapeString(form.Body),
		html.EscapeString(mode.submitLabel()),
	)
}

func replyRecipient(original mailimap.Message) string {
	if original.ReplyTo != "" {
		return replyAddress(original.ReplyTo)
	}
	return replyAddress(original.From)
}

func replyAddress(value string) string {
	addresses, err := stdmail.ParseAddressList(value)
	if err != nil || len(addresses) == 0 {
		return value
	}
	return addresses[0].Address
}

func replyAllRecipients(original mailimap.Message, selfAddress string) (string, string) {
	to := replyRecipient(original)
	excluded := map[string]bool{}
	addExcludedAddress(excluded, selfAddress)
	addExcludedAddress(excluded, to)

	ccAddresses := make([]string, 0)
	ccAddresses = appendReplyAllAddresses(ccAddresses, excluded, original.To)
	ccAddresses = appendReplyAllAddresses(ccAddresses, excluded, original.Cc)
	return to, strings.Join(ccAddresses, ", ")
}

func appendReplyAllAddresses(out []string, excluded map[string]bool, value string) []string {
	addresses, err := stdmail.ParseAddressList(value)
	if err != nil {
		return out
	}
	for _, address := range addresses {
		key := strings.ToLower(address.Address)
		if key == "" || excluded[key] {
			continue
		}
		out = append(out, address.Address)
		excluded[key] = true
	}
	return out
}

func addExcludedAddress(excluded map[string]bool, value string) {
	addresses, err := stdmail.ParseAddressList(value)
	if err != nil {
		return
	}
	for _, address := range addresses {
		if address.Address != "" {
			excluded[strings.ToLower(address.Address)] = true
		}
	}
}

func replySubject(subject string) string {
	trimmed := strings.TrimSpace(subject)
	if trimmed == "" {
		return "Re: (no subject)"
	}
	if strings.HasPrefix(strings.ToLower(trimmed), "re:") {
		return trimmed
	}
	return "Re: " + trimmed
}

func quotedReplyBody(original mailimap.Message) string {
	intro := "On " + formatIMAPTime(original.Date) + ", " + original.From + " wrote:"
	lines := strings.Split(original.Body, "\n")
	quoted := make([]string, 0, len(lines)+3)
	quoted = append(quoted, "", "", intro)
	for _, line := range lines {
		quoted = append(quoted, "> "+strings.TrimRight(line, "\r"))
	}
	return strings.Join(quoted, "\n")
}

func replyReferences(original mailimap.Message) string {
	references := strings.TrimSpace(original.References)
	if references == "" {
		return original.MessageID
	}
	if strings.Contains(references, original.MessageID) {
		return references
	}
	return references + " " + original.MessageID
}
