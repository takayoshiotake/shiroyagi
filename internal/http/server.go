package httpserver

import (
	"fmt"
	"html"
	"net/http"

	"github.com/takayoshiotake/shiroyagi/internal/auth"
	"github.com/takayoshiotake/shiroyagi/internal/config"
	"github.com/takayoshiotake/shiroyagi/internal/mailaccount"
)

const (
	oauthStateCookie = "shiroyagi_oauth_state"
	oauthNonceCookie = "shiroyagi_oauth_nonce"
	forceLoginCookie = "shiroyagi_force_login"
	sessionCookie    = "shiroyagi_session"
)

type Server struct {
	authClient *auth.Client
	sessions   *auth.SessionStore
	mailCrypto config.MailCryptoConfig
	imapConfig config.IMAPConfig
	smtpConfig config.SMTPConfig
	accounts   *mailaccount.Store
}

func New(authClient *auth.Client, sessions *auth.SessionStore, mailCrypto config.MailCryptoConfig, imapConfig config.IMAPConfig, smtpConfig config.SMTPConfig, accounts *mailaccount.Store) *Server {
	return &Server{
		authClient: authClient,
		sessions:   sessions,
		mailCrypto: mailCrypto,
		imapConfig: imapConfig,
		smtpConfig: smtpConfig,
		accounts:   accounts,
	}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.requireSession(s.handleIndex))

	mux.HandleFunc("GET /mail-accounts", s.requireSession(s.handleListMailAccounts))
	mux.HandleFunc("GET /mail-accounts/new", s.requireSession(s.handleNewMailAccount))
	mux.HandleFunc("POST /mail-accounts/create", s.requireSession(s.handleCreateMailAccount))
	mux.HandleFunc("POST /mail-accounts/{id}/delete", s.requireSession(s.handleDeleteMailAccount))
	mux.HandleFunc("GET /mail-accounts/{id}/send", s.requireSession(s.handleNewTestMessage))
	mux.HandleFunc("POST /mail-accounts/{id}/send", s.requireSession(s.handleSendTestMessage))
	mux.HandleFunc("GET /mail-accounts/{id}/mailboxes/{mailbox}", s.requireSession(s.handleMailbox))
	mux.HandleFunc("GET /mail-accounts/{id}/mailboxes/{mailbox}/messages/{uid}", s.requireSession(s.handleMessage))
	mux.HandleFunc("GET /mail-accounts/{id}/mailboxes/{mailbox}/messages/{uid}/reply", s.requireSession(s.handleNewReplyMessage))
	mux.HandleFunc("POST /mail-accounts/{id}/mailboxes/{mailbox}/messages/{uid}/reply", s.requireSession(s.handleSendReplyMessage))
	mux.HandleFunc("GET /mail-accounts/{id}/mailboxes/{mailbox}/messages/{uid}/reply-all", s.requireSession(s.handleNewReplyAllMessage))
	mux.HandleFunc("POST /mail-accounts/{id}/mailboxes/{mailbox}/messages/{uid}/reply-all", s.requireSession(s.handleSendReplyAllMessage))
	mux.HandleFunc("GET /mail-accounts/{id}/mailboxes/{mailbox}/messages/{uid}/forward", s.requireSession(s.handleNewForwardMessage))
	mux.HandleFunc("POST /mail-accounts/{id}/mailboxes/{mailbox}/messages/{uid}/forward", s.requireSession(s.handleSendForwardMessage))

	mux.HandleFunc("GET /mail-accounts/{id}/imap/edit", s.requireSession(s.handleEditIMAPAccount))
	mux.HandleFunc("POST /mail-accounts/{id}/imap/save", s.requireSession(s.handleSaveIMAPAccount))
	mux.HandleFunc("POST /mail-accounts/{id}/imap/delete", s.requireSession(s.handleDeleteIMAPAccount))

	mux.HandleFunc("GET /mail-accounts/{id}/smtp/edit", s.requireSession(s.handleEditSMTPAccount))
	mux.HandleFunc("POST /mail-accounts/{id}/smtp/save", s.requireSession(s.handleSaveSMTPAccount))
	mux.HandleFunc("POST /mail-accounts/{id}/smtp/delete", s.requireSession(s.handleDeleteSMTPAccount))

	mux.HandleFunc("GET /signin", s.handleSignIn)
	mux.HandleFunc("GET /auth/login", s.handleAuthLogin)
	mux.HandleFunc("GET /auth/callback", s.handleAuthCallback)
	mux.HandleFunc("GET /auth/logout", s.handleAuthLogout)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintln(w, "ok")
	})
	return mux
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	session, _ := sessionFromContext(r.Context())

	displayName := session.Name
	if displayName == "" {
		displayName = session.PreferredUsername
	}
	if displayName == "" {
		displayName = session.Email
	}
	if displayName == "" {
		displayName = session.Subject
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = fmt.Fprintf(w, `<!doctype html>
<html lang="ja">
<head><meta charset="utf-8"><title>Shiroyagi</title></head>
<body>
  <h1>Shiroyagi</h1>
  <p>Signed in as %s</p>
  <p><a href="/mail-accounts">Mail accounts</a></p>
  <p><a href="/auth/logout">Sign out</a></p>
</body>
</html>`, html.EscapeString(displayName))
}

func (s *Server) handleSignIn(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.currentSession(r); ok {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = fmt.Fprint(w, `<!doctype html>
<html lang="ja">
<head><meta charset="utf-8"><title>Sign in - Shiroyagi</title></head>
<body>
  <h1>Shiroyagi</h1>
  <p><a href="/auth/login">Sign in with Keycloak</a></p>
</body>
</html>`)
}
