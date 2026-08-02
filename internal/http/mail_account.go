package httpserver

import (
	"context"
	"fmt"
	"log"
	"net/http"

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
	if err := renderUI(w, http.StatusOK, name, view); err != nil {
		log.Printf("render account settings template %s: %v", name, err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
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

	emailAddress := r.FormValue("email_address")
	if emailAddress == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	session, _ := sessionFromContext(r.Context())

	found, err := s.accounts.ExistsByUserAndEmail(r.Context(), session.Subject, emailAddress)
	if err != nil {
		log.Printf("check mail account existence: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if found {
		http.Error(w, "mail account already exists", http.StatusConflict)
		return
	}

	if _, err := s.insertMailAccount(r, session.Subject, createMailAccountForm{EmailAddress: emailAddress}); err != nil {
		log.Printf("insert mail account: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/settings/mail-accounts", http.StatusSeeOther)
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
