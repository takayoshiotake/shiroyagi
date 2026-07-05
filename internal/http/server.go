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
	accounts   *mailaccount.Store
}

func New(authClient *auth.Client, sessions *auth.SessionStore, mailCrypto config.MailCryptoConfig, accounts *mailaccount.Store) *Server {
	return &Server{
		authClient: authClient,
		sessions:   sessions,
		mailCrypto: mailCrypto,
		accounts:   accounts,
	}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.requireSession(s.handleIndex))
	mux.HandleFunc("/mail-accounts", s.requireSession(s.handleMailAccounts))
	mux.HandleFunc("/mail-accounts/", s.requireSession(s.handleMailAccount))
	mux.HandleFunc("/mail-accounts/new", s.requireSession(s.handleNewMailAccount))

	mux.HandleFunc("/signin", s.handleSignIn)
	mux.HandleFunc("/auth/login", s.handleAuthLogin)
	mux.HandleFunc("/auth/callback", s.handleAuthCallback)
	mux.HandleFunc("/auth/logout", s.handleAuthLogout)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintln(w, "ok")
	})
	return mux
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

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
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
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
