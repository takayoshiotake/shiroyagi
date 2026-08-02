package httpserver

import (
	"fmt"
	"log"
	"mime"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/takayoshiotake/shiroyagi/internal/auth"
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

	renderMessageForm(w, session, account, composeMessageForm{Mode: composeModeNew})
}

func (s *Server) handleSendMessage(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadRequestBytes)
	mediaType, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if mediaType == "multipart/form-data" {
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			http.Error(w, "invalid or oversized message form", http.StatusRequestEntityTooLarge)
			return
		}
		defer r.MultipartForm.RemoveAll()
	} else if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	form, ok := parseComposeMessageForm(r)
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
	errors := composeFormErrors(form)
	if !ok || len(errors) > 0 {
		s.renderComposeFormError(w, session, account, form, errors, "Check the highlighted fields.", http.StatusUnprocessableEntity)
		return
	}
	uploadedAttachments, err := readUploadedAttachments(r)
	if err != nil {
		s.renderComposeFormError(w, session, account, form, map[string]string{"attachments": err.Error()}, "The attachments could not be added.", http.StatusUnprocessableEntity)
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
		From:        account.EmailAddress,
		To:          form.To,
		Cc:          form.Cc,
		Subject:     form.Subject,
		Body:        form.Body,
		Attachments: uploadedAttachments,
	}
	if form.Mode == composeModeReply || form.Mode == composeModeReplyAll {
		if original.MessageID == "" {
			renderIMAPError(w, account, "Could not prepare reply: original message has no Message-ID.")
			return
		}
		message.InReplyTo = original.MessageID
		message.References = replyReferences(original)
	}
	if form.Mode == composeModeForward && len(original.Attachments) > 0 {
		imapAccount, err := s.imapReaderAccount(session.Subject, account)
		if err != nil {
			log.Printf("prepare imap account %s for forwarded attachments: %v", account.ID, err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		attachments, err := mailimap.GetAttachments(r.Context(), imapAccount, form.Mailbox, form.UID)
		if err != nil {
			log.Printf("load forwarded attachments account=%s mailbox=%q uid=%d: %v", account.ID, form.Mailbox, form.UID, err)
			renderIMAPError(w, account, "Could not load original attachments: "+err.Error())
			return
		}
		message.Attachments = append(message.Attachments, smtpAttachments(attachments)...)
	}
	if err := mailsmtp.Send(r.Context(), smtpAccount, message); err != nil {
		log.Printf("send %s message account=%s mailbox=%q uid=%d: %v", form.Mode, account.ID, form.Mailbox, form.UID, err)
		renderSMTPError(w, account, "Could not send "+form.sentNoun()+": "+err.Error())
		return
	}

	warning := s.applyComposeSideEffect(r, session.Subject, account, form)
	renderComposeSent(w, session, account, original, form, warning)
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

func renderComposeSent(w http.ResponseWriter, session auth.UserSession, account mailaccount.Detail, original mailimap.Message, form composeMessageForm, warning string) {
	renderComposeStatus(w, sentStatusView(session, account, original, form, warning))
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
		return form, false
	}
	if form.requiresOriginal() {
		uid, err := strconv.ParseUint(r.FormValue("uid"), 10, 32)
		if err != nil || uid == 0 || form.Mailbox == "" {
			return form, false
		}
		form.UID = uint32(uid)
	}
	if form.To == "" || form.Subject == "" || form.Body == "" {
		return form, false
	}
	return form, true
}

func composeFormErrors(form composeMessageForm) map[string]string {
	errors := make(map[string]string)
	if strings.TrimSpace(form.To) == "" {
		errors["to"] = "Enter at least one recipient."
	}
	if strings.TrimSpace(form.Subject) == "" {
		errors["subject"] = "Enter a subject."
	}
	if strings.TrimSpace(form.Body) == "" {
		errors["body"] = "Enter a message."
	}
	return errors
}

func (s *Server) renderComposeFormError(w http.ResponseWriter, session auth.UserSession, account mailaccount.Detail, form composeMessageForm, errors map[string]string, message string, status int) {
	heading := "Compose message"
	submitLabel := "Send"
	backURL := mailboxURL(account.ID, defaultMailbox)
	attachmentHelp := "Up to 10 files and 25 MiB total."
	if form.Mode == composeModeReply || form.Mode == composeModeReplyAll {
		if form.Mode == composeModeReplyAll {
			heading, submitLabel = "Reply all", "Send reply all"
		} else {
			heading, submitLabel = "Reply message", "Send reply"
		}
		attachmentHelp = "Original attachments are not included. You can add up to 10 files and 25 MiB total."
	} else if form.Mode == composeModeForward {
		heading, submitLabel = "Forward message", "Send forward"
		attachmentHelp = "Original attachments will be included when available. The 25 MiB total limit includes them."
	}
	if form.requiresOriginal() && form.Mailbox != "" && form.UID != 0 {
		backURL = messageURL(account.ID, form.Mailbox, form.UID)
	}
	if !form.Mode.valid() {
		form.Mode = composeModeNew
	}
	view := newComposePageView(session, account, heading, submitLabel, backURL, form)
	view.AttachmentHelp = attachmentHelp
	view.Errors = errors
	view.FormError = message
	if err := renderUI(w, status, "compose-page", view); err != nil {
		log.Printf("render compose validation error: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
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

func renderMessageForm(w http.ResponseWriter, session auth.UserSession, account mailaccount.Detail, form composeMessageForm) {
	view := newComposePageView(session, account, "Compose message", "Send", mailboxURL(account.ID, defaultMailbox), form)
	renderComposePage(w, view)
}

func renderSMTPError(w http.ResponseWriter, account mailaccount.Detail, message string) {
	renderProtocolError(w, "SMTP", account, message)
}
