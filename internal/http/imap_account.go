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

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = fmt.Fprintf(w, `<!doctype html>
<html>
<head><title>Edit IMAP account - Shiroyagi</title></head>
<body>
  <h1>Edit IMAP account</h1>

  <p>
    <strong>Email address</strong><br>
    %s
  </p>

  <form method="post" action="/mail-accounts/%s/imap/save">
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
  <form method="post" action="/mail-accounts/%s/imap/delete">
    <button type="submit">Remove IMAP</button>
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
	)
}

func (s *Server) handleSaveIMAPAccount(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
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
		IMAPHost:     r.FormValue("imap_host"),
		IMAPSecurity: r.FormValue("imap_security"),
		IMAPUsername: r.FormValue("imap_username"),
		IMAPPassword: r.FormValue("imap_password"),
	}
	imapPort, err := strconv.Atoi(r.FormValue("imap_port"))
	if err != nil {
		return imapAccountForm{}, false
	}
	form.IMAPPort = imapPort
	if form.IMAPSecurity != "imaps" && form.IMAPSecurity != "imap" {
		return imapAccountForm{}, false
	}
	if form.IMAPHost == "" || form.IMAPPort <= 0 || form.IMAPPort > 65535 || form.IMAPUsername == "" {
		return imapAccountForm{}, false
	}
	return form, true
}
