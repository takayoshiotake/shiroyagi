package httpserver

import (
	"errors"
	"fmt"
	"log"
	"net/http"
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
	setHTMXVary(w)
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

	previousUID := parsePreviousUID(r)
	selectedUID := uint32(0)
	if isHTMX(r) && r.Header.Get("HX-Target") == "message-list-panel" {
		selectedUID = previousUID
	}
	view, err := s.buildMailPage(r.Context(), session, account, mailbox, selectedUID, false)
	if err != nil {
		log.Printf("list mailbox account=%s mailbox=%q: %v", account.ID, mailbox, err)
		if isHTMX(r) {
			renderMailPartialError(w, "message-list-error", "Please try again. If the problem continues, check the IMAP account settings.")
			return
		}
		renderIMAPError(w, account, "Could not load mailbox: "+err.Error())
		return
	}
	if isHTMX(r) {
		switch r.Header.Get("HX-Target") {
		case "message-list-panel":
			renderMailTemplate(w, "message-list-panel", view)
		case "message-detail":
			view.selectOOBRows(previousUID)
			renderMailTemplate(w, "message-empty-response", view)
		default:
			renderMailTemplate(w, "mail-page", view)
		}
		return
	}
	renderMailTemplate(w, "mail-page", view)
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
	setHTMXVary(w)
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

	view, err := s.buildMailPage(r.Context(), session, account, mailbox, uint32(uid), true)
	if err != nil {
		if errors.Is(err, mailimap.ErrMessageNotFound) {
			http.NotFound(w, r)
			return
		}
		log.Printf("get message account=%s mailbox=%q uid=%d: %v", account.ID, mailbox, uid, err)
		if isHTMX(r) {
			renderMailPartialError(w, "message-panel-error", "Please try again. The current message content has been left unchanged when possible.")
			return
		}
		renderIMAPError(w, account, "Could not load message: "+err.Error())
		return
	}
	if isHTMX(r) && r.Header.Get("HX-Target") == "message-detail" {
		view.selectOOBRows(parsePreviousUID(r))
		renderMailTemplate(w, "message-detail-response", view)
		return
	}
	renderMailTemplate(w, "mail-page", view)
}

func isHTMX(r *http.Request) bool {
	return isHTMXRequest(r.Header.Get("HX-Request"), r.Header.Get("HX-History-Restore-Request"))
}

func setHTMXVary(w http.ResponseWriter) {
	w.Header().Add("Vary", "HX-Request")
	w.Header().Add("Vary", "HX-Target")
	w.Header().Add("Vary", "HX-History-Restore-Request")
}

func parsePreviousUID(r *http.Request) uint32 {
	uid, err := strconv.ParseUint(r.URL.Query().Get("previous_uid"), 10, 32)
	if err != nil {
		return 0
	}
	return uint32(uid)
}

func renderMailTemplate(w http.ResponseWriter, name string, data mailPageView) {
	if err := renderUI(w, http.StatusOK, name, data); err != nil {
		log.Printf("render mail template %s: %v", name, err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}

func renderMailPartialError(w http.ResponseWriter, name string, message string) {
	if err := renderUI(w, http.StatusBadGateway, name, mailPanelErrorView{Error: message}); err != nil {
		log.Printf("render mail error template %s: %v", name, err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
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
	renderProtocolError(w, "IMAP", account, message)
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
