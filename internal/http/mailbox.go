package httpserver

import (
	"errors"
	"fmt"
	"html"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/takayoshiotake/shiroyagi/internal/crypto"
	mailimap "github.com/takayoshiotake/shiroyagi/internal/imap"
	"github.com/takayoshiotake/shiroyagi/internal/mailaccount"
)

const (
	defaultMailbox      = "INBOX"
	mailboxMessageLimit = 100
)

func (s *Server) handleMailbox(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	mailbox := r.PathValue("mailbox")
	if mailbox == "" {
		http.NotFound(w, r)
		return
	}

	session, _ := sessionFromContext(r.Context())
	account, found, err := s.accounts.Get(r.Context(), session.Subject, id)
	if err != nil {
		log.Printf("get mail account for imap mailbox: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if !found {
		http.NotFound(w, r)
		return
	}
	if !account.HasIMAP {
		renderIMAPError(w, account, "IMAP account is not configured.")
		return
	}

	imapAccount, err := s.imapReaderAccount(session.Subject, account)
	if err != nil {
		log.Printf("prepare imap account %s: %v", account.ID, err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	messages, err := mailimap.ListMessages(r.Context(), imapAccount, mailbox, mailboxMessageLimit)
	if err != nil {
		log.Printf("list mailbox account=%s mailbox=%q: %v", account.ID, mailbox, err)
		renderIMAPError(w, account, "Could not load mailbox: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = fmt.Fprintf(w, `<!doctype html>
<html>
<head><title>%s - Shiroyagi</title></head>
<body>
  <h1>%s</h1>
  <p><strong>%s</strong></p>`,
		html.EscapeString(mailbox),
		html.EscapeString(mailbox),
		html.EscapeString(account.EmailAddress),
	)

	if len(messages) == 0 {
		_, _ = fmt.Fprint(w, `
  <p>No messages.</p>`)
	} else {
		_, _ = fmt.Fprint(w, `
  <table>
    <thead>
      <tr><th>Status</th><th>Date</th><th>From</th><th>Subject</th><th>Size</th></tr>
    </thead>
    <tbody>`)
		for _, message := range messages {
			_, _ = fmt.Fprintf(w, `
      <tr>
        <td>%s</td>
        <td>%s</td>
        <td>%s</td>
        <td><a href="/mail-accounts/%s/mailboxes/%s/messages/%d">%s</a></td>
        <td>%d</td>
      </tr>`,
				html.EscapeString(messageStatus(message)),
				html.EscapeString(formatIMAPTime(message.Date)),
				html.EscapeString(message.From),
				html.EscapeString(account.ID),
				url.PathEscape(mailbox),
				message.UID,
				html.EscapeString(subjectOrUntitled(message.Subject)),
				message.Size,
			)
		}
		_, _ = fmt.Fprint(w, `
    </tbody>
  </table>`)
	}

	_, _ = fmt.Fprint(w, `
  <p><a href="/mail-accounts">Back</a></p>
</body>
</html>`)
}

func messageStatus(message mailimap.MessageSummary) string {
	statuses := make([]string, 0, 2)
	if message.Answered {
		statuses = append(statuses, "Replied")
	}
	if message.Forwarded {
		statuses = append(statuses, "Forwarded")
	}
	return strings.Join(statuses, ", ")
}

func messageDetailStatus(message mailimap.Message) string {
	statuses := make([]string, 0, 2)
	if message.Answered {
		statuses = append(statuses, "Replied")
	}
	if message.Forwarded {
		statuses = append(statuses, "Forwarded")
	}
	if len(statuses) == 0 {
		return "(none)"
	}
	return strings.Join(statuses, ", ")
}

func messageHeaderValue(value string) string {
	if value == "" {
		return "(none)"
	}
	return value
}

func (s *Server) handleMessage(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	mailbox := r.PathValue("mailbox")
	if mailbox == "" {
		http.NotFound(w, r)
		return
	}

	uid, err := strconv.ParseUint(r.PathValue("uid"), 10, 32)
	if err != nil || uid == 0 {
		http.NotFound(w, r)
		return
	}

	session, _ := sessionFromContext(r.Context())
	account, found, err := s.accounts.Get(r.Context(), session.Subject, id)
	if err != nil {
		log.Printf("get mail account for imap message: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if !found {
		http.NotFound(w, r)
		return
	}
	if !account.HasIMAP {
		renderIMAPError(w, account, "IMAP account is not configured.")
		return
	}

	imapAccount, err := s.imapReaderAccount(session.Subject, account)
	if err != nil {
		log.Printf("prepare imap account %s: %v", account.ID, err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	message, err := mailimap.GetMessage(r.Context(), imapAccount, mailbox, uint32(uid))
	if err != nil {
		if errors.Is(err, mailimap.ErrMessageNotFound) {
			http.NotFound(w, r)
			return
		}
		log.Printf("get message account=%s mailbox=%q uid=%d: %v", account.ID, mailbox, uid, err)
		renderIMAPError(w, account, "Could not load message: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = fmt.Fprintf(w, `<!doctype html>
<html>
<head><title>%s - Shiroyagi</title></head>
<body>
  <p><a href="/mail-accounts/%s/mailboxes/%s">Back to %s</a></p>
  <h1>%s</h1>
  <dl>
    <dt>Status</dt><dd>%s</dd>
    <dt>From</dt><dd>%s</dd>
    <dt>To</dt><dd>%s</dd>
    <dt>Cc</dt><dd>%s</dd>
    <dt>Reply-To</dt><dd>%s</dd>
    <dt>Date</dt><dd>%s</dd>
    <dt>Message-ID</dt><dd>%s</dd>
    <dt>In-Reply-To</dt><dd>%s</dd>
    <dt>References</dt><dd>%s</dd>
  </dl>
  <p>
    <a href="/mail-accounts/%s/mailboxes/%s/messages/%d/reply">Reply</a>
    <a href="/mail-accounts/%s/mailboxes/%s/messages/%d/reply-all">Reply all</a>
    <a href="/mail-accounts/%s/mailboxes/%s/messages/%d/forward">Forward</a>
  </p>`,
		html.EscapeString(subjectOrUntitled(message.Subject)),
		html.EscapeString(account.ID),
		url.PathEscape(mailbox),
		html.EscapeString(mailbox),
		html.EscapeString(subjectOrUntitled(message.Subject)),
		html.EscapeString(messageDetailStatus(message)),
		html.EscapeString(message.From),
		html.EscapeString(messageHeaderValue(message.To)),
		html.EscapeString(messageHeaderValue(message.Cc)),
		html.EscapeString(messageHeaderValue(message.ReplyTo)),
		html.EscapeString(formatIMAPTime(message.Date)),
		html.EscapeString(messageHeaderValue(message.MessageID)),
		html.EscapeString(messageHeaderValue(message.InReplyTo)),
		html.EscapeString(messageHeaderValue(message.References)),
		html.EscapeString(account.ID),
		url.PathEscape(mailbox),
		message.UID,
		html.EscapeString(account.ID),
		url.PathEscape(mailbox),
		message.UID,
		html.EscapeString(account.ID),
		url.PathEscape(mailbox),
		message.UID,
	)
	renderAttachmentList(w, account, mailbox, message)
	_, _ = fmt.Fprintf(w, `
  <pre>%s</pre>
</body>
</html>`, html.EscapeString(message.Body))
}

func (s *Server) imapReaderAccount(userID string, account mailaccount.Detail) (mailimap.Account, error) {
	password, err := s.decryptIMAPPassword(userID, account)
	if err != nil {
		return mailimap.Account{}, err
	}
	return mailimap.Account{
		Host:              account.IMAPHost,
		Port:              account.IMAPPort,
		Security:          account.IMAPSecurity,
		Username:          account.IMAPUsername,
		Password:          password,
		AllowInsecureAuth: s.imapConfig.AllowInsecureAuth,
	}, nil
}

func (s *Server) decryptIMAPPassword(userID string, account mailaccount.Detail) (string, error) {
	kek, err := os.ReadFile(s.mailCrypto.KEKFile)
	if err != nil {
		return "", fmt.Errorf("read mail account KEK: %w", err)
	}
	decrypter, err := crypto.OpenEnvelope(kek, crypto.Envelope{
		WrappedDEK: account.IMAPWrappedDEK,
		KEKVersion: account.IMAPKEKVersion,
	}, envelopeAAD(userID, account.ID))
	if err != nil {
		return "", fmt.Errorf("open imap account envelope: %w", err)
	}

	plaintext, err := decrypter.DecryptWithAAD(account.EncryptedIMAPPassword, fieldAAD(userID, account.ID, aadFieldIMAPPassword))
	if err != nil {
		return "", fmt.Errorf("decrypt imap password: %w", err)
	}
	return string(plaintext), nil
}

func renderIMAPError(w http.ResponseWriter, account mailaccount.Detail, message string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusBadGateway)
	_, _ = fmt.Fprintf(w, `<!doctype html>
<html>
<head><title>IMAP error - Shiroyagi</title></head>
<body>
  <h1>IMAP error</h1>
  <p><strong>%s</strong></p>
  <p>%s</p>
  <p><a href="/mail-accounts/%s/imap/edit">Edit IMAP</a></p>
  <p><a href="/mail-accounts">Back</a></p>
</body>
</html>`,
		html.EscapeString(account.EmailAddress),
		html.EscapeString(message),
		html.EscapeString(account.ID),
	)
}

func subjectOrUntitled(subject string) string {
	if subject == "" {
		return "(no subject)"
	}
	return subject
}

func formatIMAPTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Local().Format("2006-01-02 15:04")
}
