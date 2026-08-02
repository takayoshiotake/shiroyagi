package httpserver

import (
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/takayoshiotake/shiroyagi/internal/auth"
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
	session, _ := sessionFromContext(r.Context())
	renderForwardMessageForm(w, session, forwardContext.account, forwardContext.mailbox, forwardContext.original, form)
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

func renderForwardMessageForm(w http.ResponseWriter, session auth.UserSession, account mailaccount.Detail, mailbox string, original mailimap.Message, form composeMessageForm) {
	view := newComposePageView(session, account, "Forward message", "Send forward", messageURL(account.ID, mailbox, original.UID), form)
	originalView := newComposeOriginalView(original)
	view.Original = &originalView
	view.AttachmentHelp = attachmentHelpForForward(len(original.Attachments))
	renderComposePage(w, view)
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
