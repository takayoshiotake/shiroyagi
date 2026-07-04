package httpserver

import (
	"context"
	"net/http"

	"github.com/takayoshiotake/shiroyagi/internal/auth"
)

type sessionContextKey struct{}

func (s *Server) requireSession(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, ok := s.currentSession(r)
		if !ok {
			http.Redirect(w, r, "/signin", http.StatusFound)
			return
		}

		ctx := context.WithValue(r.Context(), sessionContextKey{}, session)
		next(w, r.WithContext(ctx))
	}
}

func (s *Server) currentSession(r *http.Request) (auth.UserSession, bool) {
	cookie, err := r.Cookie(sessionCookie)
	if err != nil {
		return auth.UserSession{}, false
	}
	return s.sessions.Get(cookie.Value)
}

func sessionFromContext(ctx context.Context) (auth.UserSession, bool) {
	session, ok := ctx.Value(sessionContextKey{}).(auth.UserSession)
	return session, ok
}
