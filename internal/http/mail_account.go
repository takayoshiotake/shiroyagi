package httpserver

import (
	"fmt"
	"html"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/takayoshiotake/shiroyagi/internal/crypto"
	"github.com/takayoshiotake/shiroyagi/internal/mailaccount"
)

const aadFieldIMAPPassword = "imap_password"

func (s *Server) handleMailAccounts(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleListMailAccounts(w, r)
	case http.MethodPost:
		s.handleCreateMailAccount(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleMailAccount(w http.ResponseWriter, r *http.Request) {
	id, action, ok := parseMailAccountAction(r.URL.Path)
	if !ok {
		http.NotFound(w, r)
		return
	}

	switch action {
	case "edit":
		switch r.Method {
		case http.MethodGet:
			s.handleEditMailAccount(w, r, id)
		case http.MethodPost:
			s.handleUpdateMailAccount(w, r, id)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	case "delete":
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.handleDeleteMailAccount(w, r, id)
	default:
		http.NotFound(w, r)
	}
}

func parseMailAccountAction(path string) (string, string, bool) {
	rest := strings.TrimPrefix(path, "/mail-accounts/")
	parts := strings.Split(rest, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
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
    <li>%s <a href="/mail-accounts/%s/edit">Edit</a></li>`,
				html.EscapeString(account.EmailAddress),
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

func (s *Server) handleEditMailAccount(w http.ResponseWriter, r *http.Request, id string) {
	session, _ := sessionFromContext(r.Context())
	account, found, err := s.accounts.Get(r.Context(), session.Subject, id)
	if err != nil {
		log.Printf("get mail account: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if !found {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = fmt.Fprintf(w, `<!doctype html>
<html>
<head><title>Edit mail account - Shiroyagi</title></head>
<body>
  <h1>Edit mail account</h1>

  <form method="post" action="/mail-accounts/%s/edit">
    <p>
      <strong>Email address</strong><br>
      %s
    </p>
    <p>
      <label>IMAP Host<br>
        <input name="imap_host" value="%s" required>
      </label>
    </p>
    <p>
      <label>IMAP Port<br>
        <input name="imap_port" value="%d" required>
      </label>
    </p>
    <p>
      <label>IMAP protocol<br>
        <select name="imap_security" required>
          <option value="imaps"%s>IMAPS</option>
          <option value="imap"%s>IMAP</option>
        </select>
      </label>
    </p>
    <p>
      <label>IMAP username<br>
        <input name="imap_username" value="%s" required>
      </label>
    </p>
    <p>
      <label>New password<br>
        <input type="password" name="password">
      </label>
    </p>
    <button type="submit">Save</button>
  </form>

  <form method="post" action="/mail-accounts/%s/delete">
    <button type="submit">Delete</button>
  </form>

  <p><a href="/mail-accounts">Back</a></p>
</body>
</html>`,
		html.EscapeString(account.ID),
		html.EscapeString(account.EmailAddress),
		html.EscapeString(account.IMAPHost),
		account.IMAPPort,
		selected(account.IMAPSecurity == "imaps"),
		selected(account.IMAPSecurity == "imap"),
		html.EscapeString(account.IMAPUsername),
		html.EscapeString(account.ID),
	)
}

func (s *Server) handleCreateMailAccount(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	emailAddress, imapHost, imapPort, imapSecurity, imapUsername, password, ok := parseMailAccountForm(r, true)
	if !ok {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	session, _ := sessionFromContext(r.Context())

	kek, err := os.ReadFile(s.mailCrypto.KEKFile)
	if err != nil {
		log.Printf("read mail account KEK: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	// TODO: Check IMAP connectivity before saving the account.
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

	if err := s.insertMailAccount(r, kek, session.Subject, emailAddress, imapHost, imapPort, imapSecurity, imapUsername, password); err != nil {
		log.Printf("insert mail account: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/mail-accounts", http.StatusSeeOther)
}

func (s *Server) handleUpdateMailAccount(w http.ResponseWriter, r *http.Request, id string) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	imapHost, imapPort, imapSecurity, imapUsername, password, ok := parseMailAccountUpdateForm(r)
	if !ok {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	session, _ := sessionFromContext(r.Context())
	account, found, err := s.accounts.Get(r.Context(), session.Subject, id)
	if err != nil {
		log.Printf("get mail account: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if !found {
		http.NotFound(w, r)
		return
	}

	encryptedPassword := account.EncryptedIMAPPassword
	if password != "" {
		kek, err := os.ReadFile(s.mailCrypto.KEKFile)
		if err != nil {
			log.Printf("read mail account KEK: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		decrypter, err := crypto.OpenEnvelope(kek, crypto.Envelope{
			WrappedDEK: account.WrappedDEK,
			KEKVersion: account.KEKVersion,
		}, envelopeAAD(session.Subject, account.ID))
		if err != nil {
			log.Printf("open mail account envelope: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		encryptedPassword, err = decrypter.EncryptWithAAD([]byte(password), fieldAAD(session.Subject, account.ID, aadFieldIMAPPassword))
		if err != nil {
			log.Printf("encrypt imap password: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
	}

	if err := s.accounts.Update(r.Context(), mailaccount.Account{
		ID:                    account.ID,
		UserID:                session.Subject,
		IMAPHost:              imapHost,
		IMAPPort:              imapPort,
		IMAPSecurity:          imapSecurity,
		IMAPUsername:          imapUsername,
		EncryptedIMAPPassword: encryptedPassword,
	}); err != nil {
		log.Printf("update mail account: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/mail-accounts", http.StatusSeeOther)
}

func (s *Server) handleDeleteMailAccount(w http.ResponseWriter, r *http.Request, id string) {
	session, _ := sessionFromContext(r.Context())
	if err := s.accounts.Delete(r.Context(), session.Subject, id); err != nil {
		log.Printf("delete mail account: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/mail-accounts", http.StatusSeeOther)
}

func parseMailAccountForm(r *http.Request, requirePassword bool) (string, string, int, string, string, string, bool) {
	emailAddress := r.FormValue("email_address")
	if emailAddress == "" {
		return "", "", 0, "", "", "", false
	}
	imapHost, imapPort, imapSecurity, imapUsername, password, ok := parseMailAccountUpdateForm(r)
	if !ok {
		return "", "", 0, "", "", "", false
	}
	if requirePassword && password == "" {
		return "", "", 0, "", "", "", false
	}
	return emailAddress, imapHost, imapPort, imapSecurity, imapUsername, password, true
}

func parseMailAccountUpdateForm(r *http.Request) (string, int, string, string, string, bool) {
	imapHost := r.FormValue("imap_host")
	imapPort, err := strconv.Atoi(r.FormValue("imap_port"))
	if err != nil {
		return "", 0, "", "", "", false
	}
	imapSecurity := r.FormValue("imap_security")
	if imapSecurity != "imaps" && imapSecurity != "imap" {
		return "", 0, "", "", "", false
	}
	imapUsername := r.FormValue("imap_username")
	password := r.FormValue("password")
	if imapHost == "" || imapPort <= 0 || imapPort > 65535 || imapUsername == "" {
		return "", 0, "", "", "", false
	}
	return imapHost, imapPort, imapSecurity, imapUsername, password, true
}

func (s *Server) insertMailAccount(r *http.Request, kek []byte, userID, emailAddress, imapHost string, imapPort int, imapSecurity, imapUsername, password string) error {
	accountID, err := mailaccount.NewID()
	if err != nil {
		return fmt.Errorf("create account id: %w", err)
	}

	encrypter, err := crypto.NewEnvelope(kek, s.mailCrypto.KEKVersion, envelopeAAD(userID, accountID))
	if err != nil {
		return fmt.Errorf("create envelope: %w", err)
	}
	encryptedPassword, err := encrypter.EncryptWithAAD([]byte(password), fieldAAD(userID, accountID, aadFieldIMAPPassword))
	if err != nil {
		return fmt.Errorf("encrypt imap password: %w", err)
	}
	envelope := encrypter.Envelope()

	if err := s.accounts.Insert(r.Context(), mailaccount.Account{
		ID:                    accountID,
		UserID:                userID,
		EmailAddress:          emailAddress,
		IMAPHost:              imapHost,
		IMAPPort:              imapPort,
		IMAPSecurity:          imapSecurity,
		IMAPUsername:          imapUsername,
		EncryptedIMAPPassword: encryptedPassword,
		WrappedDEK:            envelope.WrappedDEK,
		KEKVersion:            envelope.KEKVersion,
	}); err != nil {
		return fmt.Errorf("insert account: %w", err)
	}
	return nil
}

func envelopeAAD(userID, accountID string) []byte {
	return []byte(userID + ":" + accountID)
}

func fieldAAD(userID, accountID, field string) []byte {
	return []byte(userID + ":" + accountID + ":" + field)
}

func selected(ok bool) string {
	if ok {
		return " selected"
	}
	return ""
}

func (s *Server) handleNewMailAccount(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	fmt.Fprint(w, `<!doctype html>
<html>
<head><title>Add mail account - Shiroyagi</title></head>
<body>
  <h1>Add mail account</h1>

  <form method="post" action="/mail-accounts">
    <p>
      <label>Email address<br>
        <input type="email" name="email_address" required>
      </label>
    </p>
    <p>
      <label>IMAP Host<br>
        <input name="imap_host" required>
      </label>
    </p>
    <p>
      <label>IMAP Port<br>
        <input name="imap_port" value="993" required>
      </label>
    </p>
    <p>
      <label>IMAP protocol<br>
        <select name="imap_security" required>
          <option value="imaps" selected>IMAPS</option>
          <option value="imap">IMAP</option>
        </select>
      </label>
    </p>
    <p>
      <label>IMAP username<br>
        <input name="imap_username" required>
      </label>
    </p>
    <p>
      <label>Password<br>
        <input type="password" name="password" required>
      </label>
    </p>
    <button type="submit">Save</button>
  </form>

  <p><a href="/mail-accounts">Back</a></p>
</body>
</html>`)
}
