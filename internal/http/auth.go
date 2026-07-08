package httpserver

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/http"
	"time"
)

func (s *Server) handleAuthLogin(w http.ResponseWriter, r *http.Request) {
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

func (s *Server) handleAuthLogout(w http.ResponseWriter, r *http.Request) {
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
