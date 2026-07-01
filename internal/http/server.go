package httpserver

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"html"
	"net/http"
	"time"

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
	mux.HandleFunc("/login", s.handleLogin)
	mux.HandleFunc("/auth/callback", s.handleAuthCallback)
	mux.HandleFunc("/logout", s.handleLogout)
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
  <p><a href="/login">Sign in with Keycloak</a></p>
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
  <p><a href="/logout">Sign out</a></p>
</body>
</html>`, html.EscapeString(displayName))
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	state, err := randomToken()
	if err != nil {
		http.Error(w, "failed to create login state", http.StatusInternalServerError)
		return
	}
	nonce, err := randomToken()
	if err != nil {
		http.Error(w, "failed to create login nonce", http.StatusInternalServerError)
		return
	}

	s.setTransientCookie(w, oauthStateCookie, state)
	s.setTransientCookie(w, oauthNonceCookie, nonce)

	forceReauth := false
	if forceLogin, err := r.Cookie(forceLoginCookie); err == nil && forceLogin.Value == "1" {
		forceReauth = true
		s.clearCookie(w, forceLoginCookie)
	}

	authURL := s.authClient.AuthCodeURL(state, nonce, forceReauth)
	http.Redirect(w, r, authURL, http.StatusFound)
}

func (s *Server) handleAuthCallback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if oauthErr := r.URL.Query().Get("error"); oauthErr != "" {
		http.Error(w, "OIDC login failed", http.StatusBadRequest)
		return
	}

	expectedState, err := r.Cookie(oauthStateCookie)
	if err != nil || expectedState.Value == "" || r.URL.Query().Get("state") != expectedState.Value {
		http.Error(w, "invalid OIDC state", http.StatusBadRequest)
		return
	}
	expectedNonce, err := r.Cookie(oauthNonceCookie)
	if err != nil || expectedNonce.Value == "" {
		http.Error(w, "missing OIDC nonce", http.StatusBadRequest)
		return
	}
	s.clearCookie(w, oauthStateCookie)
	s.clearCookie(w, oauthNonceCookie)

	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "missing OIDC code", http.StatusBadRequest)
		return
	}

	session, err := s.authClient.ExchangeCode(r.Context(), code, expectedNonce.Value)
	if err != nil {
		http.Error(w, "failed to exchange OIDC code", http.StatusBadGateway)
		return
	}

	if reauthAfter, ok := s.sessions.ReauthAfterTime(session.Subject); ok {
		if session.AuthTime == 0 || session.AuthTime < reauthAfter {
			http.Error(w, "fresh Keycloak authentication required", http.StatusUnauthorized)
			return
		}
		s.sessions.ClearReauthAfter(session.Subject)
	}

	sessionID, err := randomToken()
	if err != nil {
		http.Error(w, "failed to create session", http.StatusInternalServerError)
		return
	}

	s.sessions.Put(sessionID, session)
	s.setSessionCookie(w, sessionID)

	http.Redirect(w, r, "/", http.StatusFound)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookie); err == nil {
		if session, ok := s.sessions.Get(cookie.Value); ok {
			s.sessions.RequireReauthAfter(session.Subject, time.Now().Unix())
		}
		s.sessions.Delete(cookie.Value)
	}
	s.clearCookie(w, sessionCookie)
	s.setTransientCookie(w, forceLoginCookie, "1")
	http.Redirect(w, r, "/", http.StatusFound)
}

func (s *Server) currentSession(r *http.Request) (auth.UserSession, bool) {
	cookie, err := r.Cookie(sessionCookie)
	if err != nil {
		return auth.UserSession{}, false
	}
	return s.sessions.Get(cookie.Value)
}

func (s *Server) setTransientCookie(w http.ResponseWriter, name, value string) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		MaxAge:   300,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func (s *Server) setSessionCookie(w http.ResponseWriter, value string) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    value,
		Path:     "/",
		MaxAge:   int((12 * time.Hour).Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func (s *Server) clearCookie(w http.ResponseWriter, name string) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func randomToken() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", errors.New("read random bytes")
	}
	return base64.RawURLEncoding.EncodeToString(b[:]), nil
}
