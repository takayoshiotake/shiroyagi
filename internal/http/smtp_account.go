package httpserver

import (
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/takayoshiotake/shiroyagi/internal/auth"
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

	protocol := &protocolSettingsView{
		Name:         "SMTP",
		Email:        account.EmailAddress,
		FormAction:   "/mail-accounts/" + account.ID + "/smtp/save",
		RemoveAction: "/mail-accounts/" + account.ID + "/smtp/delete",
		HasConfig:    account.HasSMTP,
		Host:         account.SMTPHost,
		Port:         portValue(account.HasSMTP, account.SMTPPort),
		Security:     account.SMTPSecurity,
		Username:     account.SMTPUsername,
	}
	s.renderSMTPSettings(w, r, session, account, protocol, http.StatusOK)
}

func (s *Server) handleSaveSMTPAccount(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := r.ParseForm(); err != nil {
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
	form, ok := parseSMTPForm(r)
	errors := smtpFormErrors(r, !account.HasSMTP)
	if !ok || len(errors) > 0 {
		protocol := &protocolSettingsView{
			Name:         "SMTP",
			Email:        account.EmailAddress,
			FormAction:   "/mail-accounts/" + account.ID + "/smtp/save",
			RemoveAction: "/mail-accounts/" + account.ID + "/smtp/delete",
			HasConfig:    account.HasSMTP,
			Host:         strings.TrimSpace(r.FormValue("smtp_host")),
			Port:         strings.TrimSpace(r.FormValue("smtp_port")),
			Security:     r.FormValue("smtp_security"),
			Username:     strings.TrimSpace(r.FormValue("smtp_username")),
			Errors:       errors,
			FormError:    "Check the highlighted fields.",
		}
		s.renderSMTPSettings(w, r, session, account, protocol, http.StatusUnprocessableEntity)
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
		SMTPHost:     strings.TrimSpace(r.FormValue("smtp_host")),
		SMTPSecurity: r.FormValue("smtp_security"),
		SMTPUsername: strings.TrimSpace(r.FormValue("smtp_username")),
		SMTPPassword: r.FormValue("smtp_password"),
	}
	smtpPort, err := strconv.Atoi(r.FormValue("smtp_port"))
	if err != nil {
		return form, false
	}
	form.SMTPPort = smtpPort
	if form.SMTPSecurity != "starttls" && form.SMTPSecurity != "smtps" && form.SMTPSecurity != "plain" {
		return form, false
	}
	if form.SMTPHost == "" || form.SMTPPort <= 0 || form.SMTPPort > 65535 || form.SMTPUsername == "" {
		return form, false
	}
	return form, true
}

func smtpFormErrors(r *http.Request, requirePassword bool) map[string]string {
	errors := make(map[string]string)
	if strings.TrimSpace(r.FormValue("smtp_host")) == "" {
		errors["host"] = "Enter the SMTP host."
	}
	port, err := strconv.Atoi(strings.TrimSpace(r.FormValue("smtp_port")))
	if err != nil || port <= 0 || port > 65535 {
		errors["port"] = "Enter a port from 1 to 65535."
	}
	security := r.FormValue("smtp_security")
	if security != "starttls" && security != "smtps" && security != "plain" {
		errors["security"] = "Select a supported security mode."
	}
	if strings.TrimSpace(r.FormValue("smtp_username")) == "" {
		errors["username"] = "Enter the SMTP username."
	}
	if requirePassword && r.FormValue("smtp_password") == "" {
		errors["password"] = "Enter the SMTP password."
	}
	return errors
}

func (s *Server) renderSMTPSettings(w http.ResponseWriter, r *http.Request, session auth.UserSession, account mailaccount.Detail, protocol *protocolSettingsView, status int) {
	view, err := s.buildAccountSettingsPage(r.Context(), session, account.ID)
	if err != nil {
		log.Printf("prepare smtp account page: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	view.Title = "SMTP settings"
	view.Protocol = protocol
	renderAccountSettingsStatus(w, status, "account-protocol-page", view)
}
