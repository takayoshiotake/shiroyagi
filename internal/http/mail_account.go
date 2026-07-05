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

const (
	aadFieldIMAPPassword = "imap_password"
	aadFieldSMTPPassword = "smtp_password"
)

type mailAccountForm struct {
	EmailAddress string
	IMAPHost     string
	IMAPPort     int
	IMAPSecurity string
	IMAPUsername string
	IMAPPassword string
	SMTPHost     string
	SMTPPort     int
	SMTPSecurity string
	SMTPUsername string
	SMTPPassword string
}

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
        <input type="password" name="imap_password">
      </label>
    </p>
    <p>
      <label>SMTP Host<br>
        <input name="smtp_host" value="%s" required>
      </label>
    </p>
    <p>
      <label>SMTP Port<br>
        <input name="smtp_port" value="%d" required>
      </label>
    </p>
    <p>
      <label>SMTP protocol<br>
        <select name="smtp_security" required>
          <option value="starttls"%s>STARTTLS</option>
          <option value="smtps"%s>SMTPS</option>
          <option value="plain"%s>Plain</option>
        </select>
      </label>
    </p>
    <p>
      <label>SMTP username<br>
        <input name="smtp_username" value="%s" required>
      </label>
    </p>
    <p>
      <label>New SMTP password<br>
        <input type="password" name="smtp_password">
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
		html.EscapeString(account.SMTPHost),
		account.SMTPPort,
		selected(account.SMTPSecurity == "starttls"),
		selected(account.SMTPSecurity == "smtps"),
		selected(account.SMTPSecurity == "plain"),
		html.EscapeString(account.SMTPUsername),
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

	form, ok := parseMailAccountForm(r, true)
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
	found, err := s.accounts.ExistsByUserAndEmail(r.Context(), session.Subject, form.EmailAddress)
	if err != nil {
		log.Printf("check mail account existence: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if found {
		http.Error(w, "mail account already exists", http.StatusConflict)
		return
	}

	if err := s.insertMailAccount(r, kek, session.Subject, form); err != nil {
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

	form, ok := parseMailAccountUpdateForm(r)
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

	encryptedIMAPPassword := account.EncryptedIMAPPassword
	encryptedSMTPPassword := account.EncryptedSMTPPassword
	if form.IMAPPassword != "" || form.SMTPPassword != "" {
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
		if form.IMAPPassword != "" {
			encryptedIMAPPassword, err = decrypter.EncryptWithAAD([]byte(form.IMAPPassword), fieldAAD(session.Subject, account.ID, aadFieldIMAPPassword))
			if err != nil {
				log.Printf("encrypt imap password: %v", err)
				http.Error(w, "internal server error", http.StatusInternalServerError)
				return
			}
		}
		if form.SMTPPassword != "" {
			encryptedSMTPPassword, err = decrypter.EncryptWithAAD([]byte(form.SMTPPassword), fieldAAD(session.Subject, account.ID, aadFieldSMTPPassword))
			if err != nil {
				log.Printf("encrypt smtp password: %v", err)
				http.Error(w, "internal server error", http.StatusInternalServerError)
				return
			}
		}
	}

	if err := s.accounts.Update(r.Context(), mailaccount.Account{
		ID:                    account.ID,
		UserID:                session.Subject,
		IMAPHost:              form.IMAPHost,
		IMAPPort:              form.IMAPPort,
		IMAPSecurity:          form.IMAPSecurity,
		IMAPUsername:          form.IMAPUsername,
		EncryptedIMAPPassword: encryptedIMAPPassword,
		SMTPHost:              form.SMTPHost,
		SMTPPort:              form.SMTPPort,
		SMTPSecurity:          form.SMTPSecurity,
		SMTPUsername:          form.SMTPUsername,
		EncryptedSMTPPassword: encryptedSMTPPassword,
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

func parseMailAccountForm(r *http.Request, requirePassword bool) (mailAccountForm, bool) {
	form, ok := parseMailAccountUpdateForm(r)
	if !ok {
		return mailAccountForm{}, false
	}
	form.EmailAddress = r.FormValue("email_address")
	if form.EmailAddress == "" {
		return mailAccountForm{}, false
	}
	if requirePassword && (form.IMAPPassword == "" || form.SMTPPassword == "") {
		return mailAccountForm{}, false
	}
	return form, true
}

func parseMailAccountUpdateForm(r *http.Request) (mailAccountForm, bool) {
	imapPort, err := strconv.Atoi(r.FormValue("imap_port"))
	if err != nil {
		return mailAccountForm{}, false
	}
	smtpPort, err := strconv.Atoi(r.FormValue("smtp_port"))
	if err != nil {
		return mailAccountForm{}, false
	}

	form := mailAccountForm{
		IMAPHost:     r.FormValue("imap_host"),
		IMAPPort:     imapPort,
		IMAPSecurity: r.FormValue("imap_security"),
		IMAPUsername: r.FormValue("imap_username"),
		IMAPPassword: r.FormValue("imap_password"),
		SMTPHost:     r.FormValue("smtp_host"),
		SMTPPort:     smtpPort,
		SMTPSecurity: r.FormValue("smtp_security"),
		SMTPUsername: r.FormValue("smtp_username"),
		SMTPPassword: r.FormValue("smtp_password"),
	}
	if form.IMAPSecurity != "imaps" && form.IMAPSecurity != "imap" {
		return mailAccountForm{}, false
	}
	if form.SMTPSecurity != "starttls" && form.SMTPSecurity != "smtps" && form.SMTPSecurity != "plain" {
		return mailAccountForm{}, false
	}
	if form.IMAPHost == "" || form.IMAPPort <= 0 || form.IMAPPort > 65535 || form.IMAPUsername == "" {
		return mailAccountForm{}, false
	}
	if form.SMTPHost == "" || form.SMTPPort <= 0 || form.SMTPPort > 65535 || form.SMTPUsername == "" {
		return mailAccountForm{}, false
	}
	return form, true
}

func (s *Server) insertMailAccount(r *http.Request, kek []byte, userID string, form mailAccountForm) error {
	accountID, err := mailaccount.NewID()
	if err != nil {
		return fmt.Errorf("create account id: %w", err)
	}

	encrypter, err := crypto.NewEnvelope(kek, s.mailCrypto.KEKVersion, envelopeAAD(userID, accountID))
	if err != nil {
		return fmt.Errorf("create envelope: %w", err)
	}
	encryptedIMAPPassword, err := encrypter.EncryptWithAAD([]byte(form.IMAPPassword), fieldAAD(userID, accountID, aadFieldIMAPPassword))
	if err != nil {
		return fmt.Errorf("encrypt imap password: %w", err)
	}
	encryptedSMTPPassword, err := encrypter.EncryptWithAAD([]byte(form.SMTPPassword), fieldAAD(userID, accountID, aadFieldSMTPPassword))
	if err != nil {
		return fmt.Errorf("encrypt smtp password: %w", err)
	}
	envelope := encrypter.Envelope()

	if err := s.accounts.Insert(r.Context(), mailaccount.Account{
		ID:                    accountID,
		UserID:                userID,
		EmailAddress:          form.EmailAddress,
		IMAPHost:              form.IMAPHost,
		IMAPPort:              form.IMAPPort,
		IMAPSecurity:          form.IMAPSecurity,
		IMAPUsername:          form.IMAPUsername,
		EncryptedIMAPPassword: encryptedIMAPPassword,
		SMTPHost:              form.SMTPHost,
		SMTPPort:              form.SMTPPort,
		SMTPSecurity:          form.SMTPSecurity,
		SMTPUsername:          form.SMTPUsername,
		EncryptedSMTPPassword: encryptedSMTPPassword,
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
        <input type="password" name="imap_password" required>
      </label>
    </p>
    <p>
      <label>SMTP Host<br>
        <input name="smtp_host" required>
      </label>
    </p>
    <p>
      <label>SMTP Port<br>
        <input name="smtp_port" value="587" required>
      </label>
    </p>
    <p>
      <label>SMTP protocol<br>
        <select name="smtp_security" required>
          <option value="starttls" selected>STARTTLS</option>
          <option value="smtps">SMTPS</option>
          <option value="plain">Plain</option>
        </select>
      </label>
    </p>
    <p>
      <label>SMTP username<br>
        <input name="smtp_username" required>
      </label>
    </p>
    <p>
      <label>SMTP password<br>
        <input type="password" name="smtp_password" required>
      </label>
    </p>
    <button type="submit">Save</button>
  </form>

  <p><a href="/mail-accounts">Back</a></p>
</body>
</html>`)
}
