package httpserver

import (
	"fmt"
	"log"
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
	mux.HandleFunc("GET /static/{path...}", handleStaticAsset)
	mux.HandleFunc("GET /{$}", s.requireSession(s.handleIndex))

	mux.HandleFunc("GET /mail-accounts", s.requireSession(s.handleMailEntry))
	mux.HandleFunc("GET /settings/mail-accounts", s.requireSession(s.handleListMailAccounts))
	mux.HandleFunc("GET /mail-accounts/new", s.requireSession(s.handleNewMailAccount))
	mux.HandleFunc("POST /mail-accounts/create", s.requireSession(s.handleCreateMailAccount))
	mux.HandleFunc("POST /mail-accounts/{id}/delete", s.requireSession(s.handleDeleteMailAccount))
	mux.HandleFunc("GET /mail-accounts/{id}/delete", s.requireSession(s.handleConfirmDeleteMailAccount))
	mux.HandleFunc("GET /mail-accounts/{id}/send", s.requireSession(s.handleNewMessage))
	mux.HandleFunc("POST /mail-accounts/{id}/send", s.requireSession(s.handleSendMessage))
	mux.HandleFunc("GET /mail-accounts/{id}/mailboxes/{mailbox}", s.requireSession(s.handleMailbox))
	mux.HandleFunc("GET /mail-accounts/{id}/mailboxes/{mailbox}/messages/{uid}", s.requireSession(s.handleMessage))
	mux.HandleFunc("GET /mail-accounts/{id}/mailboxes/{mailbox}/messages/{uid}/attachments/{part}", s.requireSession(s.handleAttachmentDownload))
	mux.HandleFunc("GET /mail-accounts/{id}/mailboxes/{mailbox}/messages/{uid}/reply", s.requireSession(s.handleNewReplyMessage))
	mux.HandleFunc("GET /mail-accounts/{id}/mailboxes/{mailbox}/messages/{uid}/reply-all", s.requireSession(s.handleNewReplyAllMessage))
	mux.HandleFunc("GET /mail-accounts/{id}/mailboxes/{mailbox}/messages/{uid}/forward", s.requireSession(s.handleNewForwardMessage))

	mux.HandleFunc("GET /mail-accounts/{id}/imap/edit", s.requireSession(s.handleEditIMAPAccount))
	mux.HandleFunc("POST /mail-accounts/{id}/imap/save", s.requireSession(s.handleSaveIMAPAccount))
	mux.HandleFunc("POST /mail-accounts/{id}/imap/delete", s.requireSession(s.handleDeleteIMAPAccount))
	mux.HandleFunc("GET /mail-accounts/{id}/imap/delete", s.requireSession(s.handleConfirmDeleteIMAPAccount))

	mux.HandleFunc("GET /mail-accounts/{id}/smtp/edit", s.requireSession(s.handleEditSMTPAccount))
	mux.HandleFunc("POST /mail-accounts/{id}/smtp/save", s.requireSession(s.handleSaveSMTPAccount))
	mux.HandleFunc("POST /mail-accounts/{id}/smtp/delete", s.requireSession(s.handleDeleteSMTPAccount))
	mux.HandleFunc("GET /mail-accounts/{id}/smtp/delete", s.requireSession(s.handleConfirmDeleteSMTPAccount))

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
	http.Redirect(w, r, "/mail-accounts", http.StatusFound)
}

func (s *Server) handleSignIn(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.currentSession(r); ok {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	if err := renderUI(w, http.StatusOK, "sign-in-page", signInPageView{Title: "Sign in"}); err != nil {
		log.Printf("render sign-in page: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}
