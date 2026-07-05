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
	HasIMAP      bool
	IMAPHost     string
	IMAPPort     int
	IMAPSecurity string
	IMAPUsername string
	IMAPPassword string
	HasSMTP      bool
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
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.handleEditMailAccount(w, r, id)
	case "imap":
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.handleSaveIMAPAccount(w, r, id)
	case "smtp":
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.handleSaveSMTPAccount(w, r, id)
	case "delete-imap":
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.handleDeleteIMAPAccount(w, r, id)
	case "delete-smtp":
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.handleDeleteSMTPAccount(w, r, id)
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

  <p>
    <strong>Email address</strong><br>
    %s
  </p>

  <h2>IMAP settings</h2>
  <form method="post" action="/mail-accounts/%s/imap">
    <p>
      <label>IMAP Host<br>
        <input name="imap_host" value="%s">
      </label>
    </p>
    <p>
      <label>IMAP Port<br>
        <input name="imap_port" value="%s">
      </label>
    </p>
    <p>
      <label>IMAP protocol<br>
        <select name="imap_security">
          <option value="imaps"%s>IMAPS</option>
          <option value="imap"%s>IMAP</option>
        </select>
      </label>
    </p>
    <p>
      <label>IMAP username<br>
        <input name="imap_username" value="%s">
      </label>
    </p>
    <p>
      <label>New password<br>
        <input type="password" name="imap_password">
      </label>
    </p>
    <button type="submit">Save IMAP</button>
  </form>
  <form method="post" action="/mail-accounts/%s/delete-imap">
    <button type="submit">Remove IMAP</button>
  </form>

  <h2>SMTP settings</h2>
  <form method="post" action="/mail-accounts/%s/smtp">
    <p>
      <label>SMTP Host<br>
        <input name="smtp_host" value="%s">
      </label>
    </p>
    <p>
      <label>SMTP Port<br>
        <input name="smtp_port" value="%s">
      </label>
    </p>
    <p>
      <label>SMTP protocol<br>
        <select name="smtp_security">
          <option value="starttls"%s>STARTTLS</option>
          <option value="smtps"%s>SMTPS</option>
          <option value="plain"%s>Plain</option>
        </select>
      </label>
    </p>
    <p>
      <label>SMTP username<br>
        <input name="smtp_username" value="%s">
      </label>
    </p>
    <p>
      <label>New SMTP password<br>
        <input type="password" name="smtp_password">
      </label>
    </p>
    <button type="submit">Save SMTP</button>
  </form>
  <form method="post" action="/mail-accounts/%s/delete-smtp">
    <button type="submit">Remove SMTP</button>
  </form>

  <form method="post" action="/mail-accounts/%s/delete">
    <button type="submit">Delete mail account</button>
  </form>

  <p><a href="/mail-accounts">Back</a></p>
</body>
</html>`,
		html.EscapeString(account.EmailAddress),
		html.EscapeString(account.ID),
		html.EscapeString(account.IMAPHost),
		portValue(account.HasIMAP, account.IMAPPort),
		selected(account.IMAPSecurity == "imaps"),
		selected(account.IMAPSecurity == "imap"),
		html.EscapeString(account.IMAPUsername),
		html.EscapeString(account.ID),
		html.EscapeString(account.ID),
		html.EscapeString(account.SMTPHost),
		portValue(account.HasSMTP, account.SMTPPort),
		selected(account.SMTPSecurity == "starttls"),
		selected(account.SMTPSecurity == "smtps"),
		selected(account.SMTPSecurity == "plain"),
		html.EscapeString(account.SMTPUsername),
		html.EscapeString(account.ID),
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

	emailAddress := r.FormValue("email_address")
	if emailAddress == "" {
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

	if _, err := s.insertMailAccount(r, kek, session.Subject, mailAccountForm{EmailAddress: emailAddress}); err != nil {
		log.Printf("insert mail account: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/mail-accounts", http.StatusSeeOther)
}

func (s *Server) handleSaveIMAPAccount(w http.ResponseWriter, r *http.Request, id string) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	form, ok := parseIMAPForm(r)
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
	if !account.HasIMAP && form.IMAPPassword == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	encryptedPassword := account.EncryptedIMAPPassword
	if form.IMAPPassword != "" {
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
		encryptedPassword, err = decrypter.EncryptWithAAD([]byte(form.IMAPPassword), fieldAAD(session.Subject, account.ID, aadFieldIMAPPassword))
		if err != nil {
			log.Printf("encrypt imap password: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
	}

	if err := s.accounts.SaveIMAP(r.Context(), mailaccount.Account{
		ID:                    account.ID,
		UserID:                session.Subject,
		HasIMAP:               true,
		IMAPHost:              form.IMAPHost,
		IMAPPort:              form.IMAPPort,
		IMAPSecurity:          form.IMAPSecurity,
		IMAPUsername:          form.IMAPUsername,
		EncryptedIMAPPassword: encryptedPassword,
	}); err != nil {
		log.Printf("save imap account: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/mail-accounts/"+id+"/edit", http.StatusSeeOther)
}

func (s *Server) handleSaveSMTPAccount(w http.ResponseWriter, r *http.Request, id string) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	form, ok := parseSMTPForm(r)
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
	if !account.HasSMTP && form.SMTPPassword == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	encryptedPassword := account.EncryptedSMTPPassword
	if form.SMTPPassword != "" {
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
		encryptedPassword, err = decrypter.EncryptWithAAD([]byte(form.SMTPPassword), fieldAAD(session.Subject, account.ID, aadFieldSMTPPassword))
		if err != nil {
			log.Printf("encrypt smtp password: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
	}

	if err := s.accounts.SaveSMTP(r.Context(), mailaccount.Account{
		ID:                    account.ID,
		UserID:                session.Subject,
		HasSMTP:               form.HasSMTP,
		SMTPHost:              form.SMTPHost,
		SMTPPort:              form.SMTPPort,
		SMTPSecurity:          form.SMTPSecurity,
		SMTPUsername:          form.SMTPUsername,
		EncryptedSMTPPassword: encryptedPassword,
	}); err != nil {
		log.Printf("save smtp account: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/mail-accounts/"+id+"/edit", http.StatusSeeOther)
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

func (s *Server) handleDeleteIMAPAccount(w http.ResponseWriter, r *http.Request, id string) {
	session, _ := sessionFromContext(r.Context())
	if err := s.accounts.DeleteIMAP(r.Context(), session.Subject, id); err != nil {
		log.Printf("delete imap account: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/mail-accounts/"+id+"/edit", http.StatusSeeOther)
}

func (s *Server) handleDeleteSMTPAccount(w http.ResponseWriter, r *http.Request, id string) {
	session, _ := sessionFromContext(r.Context())
	if err := s.accounts.DeleteSMTP(r.Context(), session.Subject, id); err != nil {
		log.Printf("delete smtp account: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/mail-accounts/"+id+"/edit", http.StatusSeeOther)
}

func parseIMAPForm(r *http.Request) (mailAccountForm, bool) {
	form := mailAccountForm{
		IMAPHost:     r.FormValue("imap_host"),
		IMAPSecurity: r.FormValue("imap_security"),
		IMAPUsername: r.FormValue("imap_username"),
		IMAPPassword: r.FormValue("imap_password"),
	}
	imapPort, err := strconv.Atoi(r.FormValue("imap_port"))
	if err != nil {
		return mailAccountForm{}, false
	}
	form.HasIMAP = true
	form.IMAPPort = imapPort
	if form.IMAPSecurity != "imaps" && form.IMAPSecurity != "imap" {
		return mailAccountForm{}, false
	}
	if form.IMAPHost == "" || form.IMAPPort <= 0 || form.IMAPPort > 65535 || form.IMAPUsername == "" {
		return mailAccountForm{}, false
	}
	return form, true
}

func parseSMTPForm(r *http.Request) (mailAccountForm, bool) {
	form := mailAccountForm{
		HasSMTP:      true,
		SMTPHost:     r.FormValue("smtp_host"),
		SMTPSecurity: r.FormValue("smtp_security"),
		SMTPUsername: r.FormValue("smtp_username"),
		SMTPPassword: r.FormValue("smtp_password"),
	}
	smtpPort, err := strconv.Atoi(r.FormValue("smtp_port"))
	if err != nil {
		return mailAccountForm{}, false
	}
	form.SMTPPort = smtpPort
	if form.SMTPSecurity != "starttls" && form.SMTPSecurity != "smtps" && form.SMTPSecurity != "plain" {
		return mailAccountForm{}, false
	}
	if form.SMTPHost == "" || form.SMTPPort <= 0 || form.SMTPPort > 65535 || form.SMTPUsername == "" {
		return mailAccountForm{}, false
	}
	return form, true
}

func (s *Server) insertMailAccount(r *http.Request, kek []byte, userID string, form mailAccountForm) (string, error) {
	accountID, err := mailaccount.NewID()
	if err != nil {
		return "", fmt.Errorf("create account id: %w", err)
	}

	encrypter, err := crypto.NewEnvelope(kek, s.mailCrypto.KEKVersion, envelopeAAD(userID, accountID))
	if err != nil {
		return "", fmt.Errorf("create envelope: %w", err)
	}
	var encryptedIMAPPassword []byte
	if form.HasIMAP {
		encryptedIMAPPassword, err = encrypter.EncryptWithAAD([]byte(form.IMAPPassword), fieldAAD(userID, accountID, aadFieldIMAPPassword))
		if err != nil {
			return "", fmt.Errorf("encrypt imap password: %w", err)
		}
	}
	var encryptedSMTPPassword []byte
	if form.HasSMTP {
		encryptedSMTPPassword, err = encrypter.EncryptWithAAD([]byte(form.SMTPPassword), fieldAAD(userID, accountID, aadFieldSMTPPassword))
		if err != nil {
			return "", fmt.Errorf("encrypt smtp password: %w", err)
		}
	}
	envelope := encrypter.Envelope()

	if err := s.accounts.Insert(r.Context(), mailaccount.Account{
		ID:                    accountID,
		UserID:                userID,
		EmailAddress:          form.EmailAddress,
		HasIMAP:               form.HasIMAP,
		IMAPHost:              form.IMAPHost,
		IMAPPort:              form.IMAPPort,
		IMAPSecurity:          form.IMAPSecurity,
		IMAPUsername:          form.IMAPUsername,
		EncryptedIMAPPassword: encryptedIMAPPassword,
		HasSMTP:               form.HasSMTP,
		SMTPHost:              form.SMTPHost,
		SMTPPort:              form.SMTPPort,
		SMTPSecurity:          form.SMTPSecurity,
		SMTPUsername:          form.SMTPUsername,
		EncryptedSMTPPassword: encryptedSMTPPassword,
		WrappedDEK:            envelope.WrappedDEK,
		KEKVersion:            envelope.KEKVersion,
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

func selected(ok bool) string {
	if ok {
		return " selected"
	}
	return ""
}

func portValue(ok bool, port int) string {
	if !ok {
		return ""
	}
	return strconv.Itoa(port)
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
    <button type="submit">Save</button>
  </form>

  <p><a href="/mail-accounts">Back</a></p>
</body>
</html>`)
}
