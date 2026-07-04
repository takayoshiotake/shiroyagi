package httpserver

import (
	"fmt"
	"html"
	"net/http"

	"github.com/takayoshiotake/shiroyagi/internal/auth"
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
}

func New(authClient *auth.Client, sessions *auth.SessionStore) *Server {
	return &Server{
		authClient: authClient,
		sessions:   sessions,
	}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
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

	session, ok := s.currentSession(r)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if !ok {
		_, _ = fmt.Fprint(w, `<!doctype html>
<html lang="ja">
<head><meta charset="utf-8"><title>Shiroyagi</title></head>
<body>
  <h1>Shiroyagi</h1>
  <p><a href="/auth/login">Sign in with Keycloak</a></p>
</body>
</html>`)
		return
	}

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

	_, _ = fmt.Fprintf(w, `<!doctype html>
<html lang="ja">
<head><meta charset="utf-8"><title>Shiroyagi</title></head>
<body>
  <h1>Shiroyagi</h1>
  <p>Signed in as %s</p>
  <p><a href="/auth/logout">Sign out</a></p>
</body>
</html>`, html.EscapeString(displayName))
}

func (s *Server) currentSession(r *http.Request) (auth.UserSession, bool) {
	cookie, err := r.Cookie(sessionCookie)
	if err != nil {
		return auth.UserSession{}, false
	}
	return s.sessions.Get(cookie.Value)
}
