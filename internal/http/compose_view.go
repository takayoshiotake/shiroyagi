package httpserver

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/takayoshiotake/shiroyagi/internal/auth"
	mailimap "github.com/takayoshiotake/shiroyagi/internal/imap"
	"github.com/takayoshiotake/shiroyagi/internal/mailaccount"
)

type composePageView struct {
	Title          string
	User           mailUserView
	Heading        string
	BackURL        string
	From           string
	FormAction     string
	Mode           composeMode
	Mailbox        string
	UID            uint32
	To             string
	Cc             string
	Subject        string
	Body           string
	SubmitLabel    string
	AttachmentHelp string
	Original       *composeOriginalView
	Errors         map[string]string
	FormError      string
}

type composeOriginalView struct {
	From       string
	To         string
	Cc         string
	ReplyTo    string
	Date       string
	MessageID  string
	InReplyTo  string
	References string
}

type composeStatusPageView struct {
	Title          string
	User           mailUserView
	Heading        string
	From           string
	To             string
	Cc             string
	Warning        string
	PrimaryURL     string
	PrimaryLabel   string
	SecondaryURL   string
	SecondaryLabel string
}

type protocolErrorPageView struct {
	Title        string
	User         mailUserView
	Heading      string
	AccountEmail string
	Message      string
	EditURL      string
	EditLabel    string
}

type signInPageView struct {
	Title string
	User  mailUserView
}

func newComposePageView(session auth.UserSession, account mailaccount.Detail, heading string, submitLabel string, backURL string, form composeMessageForm) composePageView {
	return composePageView{
		Title:          heading,
		User:           mailUserView{Initial: initial(userDisplayName(session))},
		Heading:        heading,
		BackURL:        backURL,
		From:           account.EmailAddress,
		FormAction:     "/mail-accounts/" + url.PathEscape(account.ID) + "/send",
		Mode:           form.Mode,
		Mailbox:        form.Mailbox,
		UID:            form.UID,
		To:             form.To,
		Cc:             form.Cc,
		Subject:        form.Subject,
		Body:           form.Body,
		SubmitLabel:    submitLabel,
		AttachmentHelp: "Up to 10 files and 25 MiB total.",
	}
}

func newComposeOriginalView(message mailimap.Message) composeOriginalView {
	return composeOriginalView{
		From:       messageHeaderValue(message.From),
		To:         messageHeaderValue(message.To),
		Cc:         messageHeaderValue(message.Cc),
		ReplyTo:    messageHeaderValue(message.ReplyTo),
		Date:       formatIMAPTime(message.Date),
		MessageID:  messageHeaderValue(message.MessageID),
		InReplyTo:  messageHeaderValue(message.InReplyTo),
		References: messageHeaderValue(message.References),
	}
}

func renderComposePage(w http.ResponseWriter, view composePageView) {
	if err := renderUI(w, http.StatusOK, "compose-page", view); err != nil {
		log.Printf("render compose page: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}

func renderComposeStatus(w http.ResponseWriter, view composeStatusPageView) {
	if err := renderUI(w, http.StatusOK, "compose-status-page", view); err != nil {
		log.Printf("render compose status page: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}

func renderProtocolError(w http.ResponseWriter, protocol string, account mailaccount.Detail, message string) {
	view := protocolErrorPageView{
		Title:        protocol + " error",
		User:         mailUserView{Initial: "?"},
		Heading:      protocol + " error",
		AccountEmail: account.EmailAddress,
		Message:      message,
		EditURL:      "/mail-accounts/" + url.PathEscape(account.ID) + "/" + strings.ToLower(protocol) + "/edit",
		EditLabel:    "Edit " + protocol + " settings",
	}
	if err := renderUI(w, http.StatusBadGateway, "protocol-error-page", view); err != nil {
		log.Printf("render %s error page: %v", protocol, err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}

func originalMessageURL(accountID string, mailbox string, uid uint32) string {
	return messageURL(accountID, mailbox, uid)
}

func sentStatusView(session auth.UserSession, account mailaccount.Detail, original mailimap.Message, form composeMessageForm, warning string) composeStatusPageView {
	view := composeStatusPageView{
		Title:   form.sentTitle(),
		User:    mailUserView{Initial: initial(userDisplayName(session))},
		Heading: form.sentTitle(),
		From:    account.EmailAddress,
		To:      form.To,
		Cc:      form.Cc,
		Warning: warning,
	}
	if form.requiresOriginal() {
		view.PrimaryURL = originalMessageURL(account.ID, form.Mailbox, original.UID)
		view.PrimaryLabel = "Back to message"
		view.SecondaryURL = mailboxURL(account.ID, form.Mailbox)
		view.SecondaryLabel = "Back to " + form.Mailbox
	} else {
		view.PrimaryURL = "/mail-accounts/" + url.PathEscape(account.ID) + "/send"
		view.PrimaryLabel = "Send another message"
		view.SecondaryURL = mailboxURL(account.ID, defaultMailbox)
		view.SecondaryLabel = "Back to Inbox"
	}
	return view
}

func attachmentHelpForForward(count int) string {
	if count == 0 {
		return "The original message has no attachments. You can add up to 10 files and 25 MiB total."
	}
	return fmt.Sprintf("The %d original attachment(s) will be included automatically. The 25 MiB total limit includes them.", count)
}
