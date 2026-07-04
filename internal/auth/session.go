package auth

import (
	"sync"
	"time"
)

type UserSession struct {
	Subject           string
	Email             string
	Name              string
	PreferredUsername string
	AuthTime          int64
	CreatedAt         time.Time
}

type SessionStore struct {
	mu          sync.RWMutex
	sessions    map[string]UserSession
	reauthAfter map[string]int64
}

func NewSessionStore() *SessionStore {
	return &SessionStore{
		sessions:    make(map[string]UserSession),
		reauthAfter: make(map[string]int64),
	}
}

func (s *SessionStore) Put(id string, session UserSession) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[id] = session
}

func (s *SessionStore) Get(id string) (UserSession, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	session, ok := s.sessions[id]
	return session, ok
}

func (s *SessionStore) Delete(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, id)
}

func (s *SessionStore) RequireReauthAfter(subject string, unixTime int64) {
	if subject == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reauthAfter[subject] = unixTime
}

func (s *SessionStore) ReauthAfterTime(subject string) (int64, bool) {
	if subject == "" {
		return 0, false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	unixTime, ok := s.reauthAfter[subject]
	return unixTime, ok
}

func (s *SessionStore) ClearReauthAfter(subject string) {
	if subject == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.reauthAfter, subject)
}
