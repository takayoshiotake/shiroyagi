package httpserver

import (
	"fmt"
	"html"
	"log"
	"net/http"
	"net/url"

	"github.com/takayoshiotake/shiroyagi/internal/mailaccount"
)

const (
	aadFieldIMAPPassword = "imap_password"
	aadFieldSMTPPassword = "smtp_password"
)

type createMailAccountForm struct {
	EmailAddress string
}

func (s *Server) handleListMailAccounts(w http.ResponseWriter, r *http.Request) {
	session, _ := sessionFromContext(r.Context())
	accounts, err := s.accounts.ListSummaries(r.Context(), session.Subject)
	if err != nil {
		log.Printf("list mail accounts: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = fmt.Fprint(w, `<!doctype html>
<html>
<head><title>Mail accounts - Shiroyagi</title></head>
<body>
  <h1>Mail accounts</h1>
  <p><a href="/mail-accounts/new">New mail account</a></p>`)

	if len(accounts) == 0 {
		_, _ = fmt.Fprint(w, `
  <p>No mail accounts.</p>`)
	} else {
		_, _ = fmt.Fprint(w, `
  <ul>`)
		for _, account := range accounts {
			_, _ = fmt.Fprintf(w, `
    <li>
      %s
      <a href="/mail-accounts/%s/mailboxes/%s">Inbox</a>
      <a href="/mail-accounts/%s/imap/edit">IMAP</a>
      <a href="/mail-accounts/%s/smtp/edit">SMTP</a>
      <form method="post" action="/mail-accounts/%s/delete">
        <button type="submit">Delete</button>
      </form>
    </li>`,
				html.EscapeString(account.EmailAddress),
				html.EscapeString(account.ID),
				url.PathEscape(defaultMailbox),
				html.EscapeString(account.ID),
				html.EscapeString(account.ID),
				html.EscapeString(account.ID),
			)
		}
		_, _ = fmt.Fprint(w, `
  </ul>`)
	}

	_, _ = fmt.Fprint(w, `
  <p><a href="/">Back</a></p>
</body>
</html>`)
}

func (s *Server) handleNewMailAccount(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	fmt.Fprint(w, `<!doctype html>
<html>
<head><title>Add mail account - Shiroyagi</title></head>
<body>
  <h1>Add mail account</h1>

  <form method="post" action="/mail-accounts/create">
    <p>
      <label>Email address<br>
        <input type="email" name="email_address" required>
      </label>
    </p>
    <button type="submit">Save</button>
  </form>

  <p><a href="/mail-accounts">Back</a></p>
</body>
</html>`)
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

	http.Redirect(w, r, "/mail-accounts", http.StatusSeeOther)
}

func (s *Server) handleDeleteMailAccount(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	session, _ := sessionFromContext(r.Context())
	if err := s.accounts.Delete(r.Context(), session.Subject, id); err != nil {
		log.Printf("delete mail account: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/mail-accounts", http.StatusSeeOther)
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
