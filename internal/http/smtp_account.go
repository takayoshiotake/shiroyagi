package httpserver

import (
	"fmt"
	"html"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/takayoshiotake/shiroyagi/internal/crypto"
	"github.com/takayoshiotake/shiroyagi/internal/mailaccount"
)

type smtpAccountForm struct {
	SMTPHost     string
	SMTPPort     int
	SMTPSecurity string
	SMTPUsername string
	SMTPPassword string
}

func (s *Server) handleEditSMTPAccount(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
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
<head><title>Edit SMTP account - Shiroyagi</title></head>
<body>
  <h1>Edit SMTP account</h1>

  <p>
    <strong>Email address</strong><br>
    %s
  </p>

  <form method="post" action="/mail-accounts/%s/smtp/save">
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
  <form method="post" action="/mail-accounts/%s/smtp/delete">
    <button type="submit">Remove SMTP</button>
  </form>

  <p><a href="/mail-accounts">Back</a></p>
</body>
</html>`,
		html.EscapeString(account.EmailAddress),
		html.EscapeString(account.ID),
		html.EscapeString(account.SMTPHost),
		portValue(account.HasSMTP, account.SMTPPort),
		selected(account.SMTPSecurity == "starttls"),
		selected(account.SMTPSecurity == "smtps"),
		selected(account.SMTPSecurity == "plain"),
		html.EscapeString(account.SMTPUsername),
		html.EscapeString(account.ID),
	)
}

func (s *Server) handleSaveSMTPAccount(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
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
	wrappedDEK := account.SMTPWrappedDEK
	kekVersion := account.SMTPKEKVersion
	if form.SMTPPassword != "" {
		kek, err := os.ReadFile(s.mailCrypto.KEKFile)
		if err != nil {
			log.Printf("read mail account KEK: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		var encrypter interface {
			EncryptWithAAD([]byte, []byte) ([]byte, error)
		}
		if account.HasSMTP {
			decrypter, err := crypto.OpenEnvelope(kek, crypto.Envelope{
				WrappedDEK: account.SMTPWrappedDEK,
				KEKVersion: account.SMTPKEKVersion,
			}, envelopeAAD(session.Subject, account.ID))
			if err != nil {
				log.Printf("open smtp account envelope: %v", err)
				http.Error(w, "internal server error", http.StatusInternalServerError)
				return
			}
			encrypter = decrypter
		} else {
			newEnvelope, err := crypto.NewEnvelope(kek, s.mailCrypto.KEKVersion, envelopeAAD(session.Subject, account.ID))
			if err != nil {
				log.Printf("create smtp account envelope: %v", err)
				http.Error(w, "internal server error", http.StatusInternalServerError)
				return
			}
			envelope := newEnvelope.Envelope()
			wrappedDEK = envelope.WrappedDEK
			kekVersion = envelope.KEKVersion
			encrypter = newEnvelope
		}
		encryptedPassword, err = encrypter.EncryptWithAAD([]byte(form.SMTPPassword), fieldAAD(session.Subject, account.ID, aadFieldSMTPPassword))
		if err != nil {
			log.Printf("encrypt smtp password: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
	}

	if err := s.accounts.SaveSMTP(r.Context(), mailaccount.Account{
		ID:                    account.ID,
		UserID:                session.Subject,
		HasSMTP:               true,
		SMTPHost:              form.SMTPHost,
		SMTPPort:              form.SMTPPort,
		SMTPSecurity:          form.SMTPSecurity,
		SMTPUsername:          form.SMTPUsername,
		EncryptedSMTPPassword: encryptedPassword,
		SMTPWrappedDEK:        wrappedDEK,
		SMTPKEKVersion:        kekVersion,
	}); err != nil {
		log.Printf("save smtp account: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/mail-accounts/"+id+"/smtp/edit", http.StatusSeeOther)
}

func (s *Server) handleDeleteSMTPAccount(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	session, _ := sessionFromContext(r.Context())
	if err := s.accounts.DeleteSMTP(r.Context(), session.Subject, id); err != nil {
		log.Printf("delete smtp account: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/mail-accounts/"+id+"/smtp/edit", http.StatusSeeOther)
}

func parseSMTPForm(r *http.Request) (smtpAccountForm, bool) {
	form := smtpAccountForm{
		SMTPHost:     r.FormValue("smtp_host"),
		SMTPSecurity: r.FormValue("smtp_security"),
		SMTPUsername: r.FormValue("smtp_username"),
		SMTPPassword: r.FormValue("smtp_password"),
	}
	smtpPort, err := strconv.Atoi(r.FormValue("smtp_port"))
	if err != nil {
		return smtpAccountForm{}, false
	}
	form.SMTPPort = smtpPort
	if form.SMTPSecurity != "starttls" && form.SMTPSecurity != "smtps" && form.SMTPSecurity != "plain" {
		return smtpAccountForm{}, false
	}
	if form.SMTPHost == "" || form.SMTPPort <= 0 || form.SMTPPort > 65535 || form.SMTPUsername == "" {
		return smtpAccountForm{}, false
	}
	return form, true
}
