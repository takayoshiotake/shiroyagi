package httpserver

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/takayoshiotake/shiroyagi/internal/crypto"
	"github.com/takayoshiotake/shiroyagi/internal/mailaccount"
)

func (s *Server) handleMailAccounts(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		// s.handleListMailAccounts(w, r)
	case http.MethodPost:
		s.handleCreateMailAccount(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
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

	emailAddress := r.FormValue("email_address")
	imapHost := r.FormValue("imap_host")
	imapPort, err := strconv.Atoi(r.FormValue("imap_port"))
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	imapSecurity := r.FormValue("imap_security")
	if imapSecurity != "imaps" && imapSecurity != "imap" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	imapUsername := r.FormValue("imap_username")
	password := r.FormValue("password")
	if emailAddress == "" || imapHost == "" || imapPort <= 0 || imapPort > 65535 || imapUsername == "" || password == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	session, _ := sessionFromContext(r.Context())
	accountID, err := mailaccount.NewID()
	if err != nil {
		log.Printf("create mail account id: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	kek, err := os.ReadFile(s.mailCrypto.KEKFile)
	if err != nil {
		log.Printf("read mail account KEK: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	// TODO: Check IMAP connectivity before saving the account.
	encrypter, err := crypto.NewEnvelope(kek, s.mailCrypto.KEKVersion, mailaccount.EncryptionAAD(session.Subject, accountID))
	if err != nil {
		log.Printf("create mail account envelope: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	encryptedPassword, err := encrypter.Encrypt([]byte(password))
	if err != nil {
		log.Printf("encrypt mail account password: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	envelope := encrypter.Envelope()
	if err := s.accounts.Create(r.Context(), mailaccount.Account{
		ID:                    accountID,
		UserID:                session.Subject,
		EmailAddress:          emailAddress,
		IMAPHost:              imapHost,
		IMAPPort:              imapPort,
		IMAPSecurity:          imapSecurity,
		IMAPUsername:          imapUsername,
		EncryptedIMAPPassword: encryptedPassword,
		WrappedDEK:            envelope.WrappedDEK,
		KEKVersion:            envelope.KEKVersion,
	}); err != nil {
		if errors.Is(err, mailaccount.ErrDuplicateAccount) {
			http.Error(w, "mail account already exists", http.StatusConflict)
			return
		}
		log.Printf("create mail account: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
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

  <p><a href="/">Back</a></p>
</body>
</html>`)
}
