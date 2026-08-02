package httpserver

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/mail"
	"strings"

	"github.com/takayoshiotake/shiroyagi/internal/auth"
	"github.com/takayoshiotake/shiroyagi/internal/mailaccount"
)

const (
	aadFieldIMAPPassword = "imap_password"
	aadFieldSMTPPassword = "smtp_password"
)

type createMailAccountForm struct {
	EmailAddress string
}

type accountSettingsPageView struct {
	Title          string
	CurrentAccount mailAccountView
	User           mailUserView
	Settings       []accountSettingsItemView
	Protocol       *protocolSettingsView
	Confirmation   *removalConfirmationView
	NewEmail       string
	Errors         map[string]string
	FormError      string
}

type removalConfirmationView struct {
	Heading      string
	Description  string
	Action       string
	CancelURL    string
	ConfirmLabel string
}

type accountSettingsItemView struct {
	Email     string
	InboxURL  string
	IMAPURL   string
	SMTPURL   string
	DeleteURL string
}

type protocolSettingsView struct {
	Name         string
	Email        string
	FormAction   string
	RemoveAction string
	HasConfig    bool
	Host         string
	Port         string
	Security     string
	Username     string
	Errors       map[string]string
	FormError    string
}

func (s *Server) buildAccountSettingsPage(ctx context.Context, session auth.UserSession, selectedID string) (accountSettingsPageView, error) {
	summaries, err := s.accounts.ListSummaries(ctx, session.Subject)
	if err != nil {
		return accountSettingsPageView{}, err
	}

	view := accountSettingsPageView{
		Title:    "Mail accounts",
		User:     mailUserView{Initial: initial(userDisplayName(session))},
		Settings: make([]accountSettingsItemView, 0, len(summaries)),
	}
	if selectedID == "" && len(summaries) > 0 {
		selectedID = summaries[0].ID
	}
	for _, account := range summaries {
		accountView := newMailAccountView(account.ID, account.EmailAddress, account.ID == selectedID)
		if accountView.Current {
			view.CurrentAccount = accountView
		}
		view.Settings = append(view.Settings, accountSettingsItemView{
			Email:     account.EmailAddress,
			InboxURL:  accountView.InboxURL,
			IMAPURL:   "/mail-accounts/" + account.ID + "/imap/edit",
			SMTPURL:   "/mail-accounts/" + account.ID + "/smtp/edit",
			DeleteURL: "/mail-accounts/" + account.ID + "/delete",
		})
	}
	return view, nil
}

func renderAccountSettings(w http.ResponseWriter, name string, view accountSettingsPageView) {
	renderAccountSettingsStatus(w, http.StatusOK, name, view)
}

func renderAccountSettingsStatus(w http.ResponseWriter, status int, name string, view accountSettingsPageView) {
	if err := renderUI(w, status, name, view); err != nil {
		log.Printf("render account settings template %s: %v", name, err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}

func (s *Server) renderRemovalConfirmation(w http.ResponseWriter, r *http.Request, kind string) {
	id := r.PathValue("id")
	session, _ := sessionFromContext(r.Context())
	account, found, err := s.accounts.Get(r.Context(), session.Subject, id)
	if err != nil {
		log.Printf("get mail account for removal confirmation: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if !found {
		http.NotFound(w, r)
		return
	}
	view, err := s.buildAccountSettingsPage(r.Context(), session, account.ID)
	if err != nil {
		log.Printf("prepare removal confirmation: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	view.Title = "Confirm removal"
	confirmation := removalConfirmationView{CancelURL: "/settings/mail-accounts"}
	switch kind {
	case "IMAP":
		confirmation.Heading = "Remove IMAP settings?"
		confirmation.Description = "Incoming mail settings for " + account.EmailAddress + " will be removed. The mail account and SMTP settings will remain."
		confirmation.Action = "/mail-accounts/" + account.ID + "/imap/delete"
		confirmation.CancelURL = "/mail-accounts/" + account.ID + "/imap/edit"
		confirmation.ConfirmLabel = "Remove IMAP"
	case "SMTP":
		confirmation.Heading = "Remove SMTP settings?"
		confirmation.Description = "Outgoing mail settings for " + account.EmailAddress + " will be removed. The mail account and IMAP settings will remain."
		confirmation.Action = "/mail-accounts/" + account.ID + "/smtp/delete"
		confirmation.CancelURL = "/mail-accounts/" + account.ID + "/smtp/edit"
		confirmation.ConfirmLabel = "Remove SMTP"
	default:
		confirmation.Heading = "Delete mail account?"
		confirmation.Description = account.EmailAddress + " and its saved IMAP and SMTP settings will be permanently deleted."
		confirmation.Action = "/mail-accounts/" + account.ID + "/delete"
		confirmation.ConfirmLabel = "Delete account"
	}
	view.Confirmation = &confirmation
	renderAccountSettings(w, "account-removal-page", view)
}

func (s *Server) handleConfirmDeleteMailAccount(w http.ResponseWriter, r *http.Request) {
	s.renderRemovalConfirmation(w, r, "account")
}

func (s *Server) handleConfirmDeleteIMAPAccount(w http.ResponseWriter, r *http.Request) {
	s.renderRemovalConfirmation(w, r, "IMAP")
}

func (s *Server) handleConfirmDeleteSMTPAccount(w http.ResponseWriter, r *http.Request) {
	s.renderRemovalConfirmation(w, r, "SMTP")
}

func (s *Server) handleMailEntry(w http.ResponseWriter, r *http.Request) {
	session, _ := sessionFromContext(r.Context())
	accounts, err := s.accounts.ListSummaries(r.Context(), session.Subject)
	if err != nil {
		log.Printf("list mail accounts for mail entry: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, mailEntryURL(accounts), http.StatusFound)
}

func mailEntryURL(accounts []mailaccount.Summary) string {
	for _, account := range accounts {
		if account.HasIMAP {
			return mailboxURL(account.ID, defaultMailbox)
		}
	}
	return "/settings/mail-accounts"
}

func (s *Server) handleListMailAccounts(w http.ResponseWriter, r *http.Request) {
	session, _ := sessionFromContext(r.Context())
	view, err := s.buildAccountSettingsPage(r.Context(), session, "")
	if err != nil {
		log.Printf("list mail accounts: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	renderAccountSettings(w, "account-settings-page", view)
}

func (s *Server) handleNewMailAccount(w http.ResponseWriter, r *http.Request) {
	session, _ := sessionFromContext(r.Context())
	view, err := s.buildAccountSettingsPage(r.Context(), session, "")
	if err != nil {
		log.Printf("prepare new mail account page: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	view.Title = "Add mail account"
	renderAccountSettings(w, "account-new-page", view)
}

func (s *Server) handleCreateMailAccount(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	session, _ := sessionFromContext(r.Context())
	emailAddress := strings.TrimSpace(r.FormValue("email_address"))
	if !validEmailAddress(emailAddress) {
		s.renderNewMailAccountError(w, r, session, emailAddress, http.StatusUnprocessableEntity, "Enter a valid email address.")
		return
	}

	found, err := s.accounts.ExistsByUserAndEmail(r.Context(), session.Subject, emailAddress)
	if err != nil {
		log.Printf("check mail account existence: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if found {
		s.renderNewMailAccountError(w, r, session, emailAddress, http.StatusConflict, "This mail account already exists.")
		return
	}

	if _, err := s.insertMailAccount(r, session.Subject, createMailAccountForm{EmailAddress: emailAddress}); err != nil {
		log.Printf("insert mail account: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/settings/mail-accounts", http.StatusSeeOther)
}

func validEmailAddress(value string) bool {
	address, err := mail.ParseAddress(value)
	return err == nil && address.Address == value
}

func (s *Server) renderNewMailAccountError(w http.ResponseWriter, r *http.Request, session auth.UserSession, email string, status int, message string) {
	view, err := s.buildAccountSettingsPage(r.Context(), session, "")
	if err != nil {
		log.Printf("prepare new mail account error page: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	view.Title = "Add mail account"
	view.NewEmail = email
	view.Errors = map[string]string{"email_address": message}
	view.FormError = "Check the highlighted field."
	renderAccountSettingsStatus(w, status, "account-new-page", view)
}

func (s *Server) handleDeleteMailAccount(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	session, _ := sessionFromContext(r.Context())
	if err := s.accounts.Delete(r.Context(), session.Subject, id); err != nil {
		log.Printf("delete mail account: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/settings/mail-accounts", http.StatusSeeOther)
}

func (s *Server) insertMailAccount(r *http.Request, userID string, form createMailAccountForm) (string, error) {
	accountID, err := mailaccount.NewID()
	if err != nil {
		return "", fmt.Errorf("create account id: %w", err)
	}

	if err := s.accounts.Insert(r.Context(), mailaccount.Account{
		ID:           accountID,
		UserID:       userID,
		EmailAddress: form.EmailAddress,
	}); err != nil {
		return "", fmt.Errorf("insert account: %w", err)
	}
	return accountID, nil
}

func envelopeAAD(userID, accountID string) []byte {
	return []byte(userID + ":" + accountID)
}

func fieldAAD(userID, accountID, field string) []byte {
	return []byte(userID + ":" + accountID + ":" + field)
}
