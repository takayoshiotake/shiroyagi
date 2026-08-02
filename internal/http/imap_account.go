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

type imapAccountForm struct {
	IMAPHost     string
	IMAPPort     int
	IMAPSecurity string
	IMAPUsername string
	IMAPPassword string
}

func (s *Server) handleEditIMAPAccount(w http.ResponseWriter, r *http.Request) {
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
		Name:         "IMAP",
		Email:        account.EmailAddress,
		FormAction:   "/mail-accounts/" + account.ID + "/imap/save",
		RemoveAction: "/mail-accounts/" + account.ID + "/imap/delete",
		HasConfig:    account.HasIMAP,
		Host:         account.IMAPHost,
		Port:         portValue(account.HasIMAP, account.IMAPPort),
		Security:     account.IMAPSecurity,
		Username:     account.IMAPUsername,
	}
	s.renderIMAPSettings(w, r, session, account, protocol, http.StatusOK)
}

func (s *Server) handleSaveIMAPAccount(w http.ResponseWriter, r *http.Request) {
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
	form, ok := parseIMAPForm(r)
	errors := imapFormErrors(r, !account.HasIMAP)
	if !ok || len(errors) > 0 {
		protocol := &protocolSettingsView{
			Name:         "IMAP",
			Email:        account.EmailAddress,
			FormAction:   "/mail-accounts/" + account.ID + "/imap/save",
			RemoveAction: "/mail-accounts/" + account.ID + "/imap/delete",
			HasConfig:    account.HasIMAP,
			Host:         strings.TrimSpace(r.FormValue("imap_host")),
			Port:         strings.TrimSpace(r.FormValue("imap_port")),
			Security:     r.FormValue("imap_security"),
			Username:     strings.TrimSpace(r.FormValue("imap_username")),
			Errors:       errors,
			FormError:    "Check the highlighted fields.",
		}
		s.renderIMAPSettings(w, r, session, account, protocol, http.StatusUnprocessableEntity)
		return
	}

	encryptedPassword := account.EncryptedIMAPPassword
	wrappedDEK := account.IMAPWrappedDEK
	kekVersion := account.IMAPKEKVersion
	if form.IMAPPassword != "" {
		kek, err := os.ReadFile(s.mailCrypto.KEKFile)
		if err != nil {
			log.Printf("read mail account KEK: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		var encrypter interface {
			EncryptWithAAD([]byte, []byte) ([]byte, error)
		}
		if account.HasIMAP {
			decrypter, err := crypto.OpenEnvelope(kek, crypto.Envelope{
				WrappedDEK: account.IMAPWrappedDEK,
				KEKVersion: account.IMAPKEKVersion,
			}, envelopeAAD(session.Subject, account.ID))
			if err != nil {
				log.Printf("open imap account envelope: %v", err)
				http.Error(w, "internal server error", http.StatusInternalServerError)
				return
			}
			encrypter = decrypter
		} else {
			newEnvelope, err := crypto.NewEnvelope(kek, s.mailCrypto.KEKVersion, envelopeAAD(session.Subject, account.ID))
			if err != nil {
				log.Printf("create imap account envelope: %v", err)
				http.Error(w, "internal server error", http.StatusInternalServerError)
				return
			}
			envelope := newEnvelope.Envelope()
			wrappedDEK = envelope.WrappedDEK
			kekVersion = envelope.KEKVersion
			encrypter = newEnvelope
		}
		encryptedPassword, err = encrypter.EncryptWithAAD([]byte(form.IMAPPassword), fieldAAD(session.Subject, account.ID, aadFieldIMAPPassword))
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
		IMAPWrappedDEK:        wrappedDEK,
		IMAPKEKVersion:        kekVersion,
	}); err != nil {
		log.Printf("save imap account: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/mail-accounts/"+id+"/imap/edit", http.StatusSeeOther)
}

func (s *Server) handleDeleteIMAPAccount(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	session, _ := sessionFromContext(r.Context())
	if err := s.accounts.DeleteIMAP(r.Context(), session.Subject, id); err != nil {
		log.Printf("delete imap account: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/mail-accounts/"+id+"/imap/edit", http.StatusSeeOther)
}

func parseIMAPForm(r *http.Request) (imapAccountForm, bool) {
	form := imapAccountForm{
		IMAPHost:     strings.TrimSpace(r.FormValue("imap_host")),
		IMAPSecurity: r.FormValue("imap_security"),
		IMAPUsername: strings.TrimSpace(r.FormValue("imap_username")),
		IMAPPassword: r.FormValue("imap_password"),
	}
	imapPort, err := strconv.Atoi(r.FormValue("imap_port"))
	if err != nil {
		return form, false
	}
	form.IMAPPort = imapPort
	if form.IMAPSecurity != "imaps" && form.IMAPSecurity != "imap" {
		return form, false
	}
	if form.IMAPHost == "" || form.IMAPPort <= 0 || form.IMAPPort > 65535 || form.IMAPUsername == "" {
		return form, false
	}
	return form, true
}

func imapFormErrors(r *http.Request, requirePassword bool) map[string]string {
	errors := make(map[string]string)
	if strings.TrimSpace(r.FormValue("imap_host")) == "" {
		errors["host"] = "Enter the IMAP host."
	}
	port, err := strconv.Atoi(strings.TrimSpace(r.FormValue("imap_port")))
	if err != nil || port <= 0 || port > 65535 {
		errors["port"] = "Enter a port from 1 to 65535."
	}
	security := r.FormValue("imap_security")
	if security != "imaps" && security != "imap" {
		errors["security"] = "Select a supported security mode."
	}
	if strings.TrimSpace(r.FormValue("imap_username")) == "" {
		errors["username"] = "Enter the IMAP username."
	}
	if requirePassword && r.FormValue("imap_password") == "" {
		errors["password"] = "Enter the IMAP password."
	}
	return errors
}

func (s *Server) renderIMAPSettings(w http.ResponseWriter, r *http.Request, session auth.UserSession, account mailaccount.Detail, protocol *protocolSettingsView, status int) {
	view, err := s.buildAccountSettingsPage(r.Context(), session, account.ID)
	if err != nil {
		log.Printf("prepare imap account page: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	view.Title = "IMAP settings"
	view.Protocol = protocol
	renderAccountSettingsStatus(w, status, "account-protocol-page", view)
}
