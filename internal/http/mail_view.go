package httpserver

import (
	"context"
	"fmt"
	"net/mail"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/takayoshiotake/shiroyagi/internal/auth"
	mailimap "github.com/takayoshiotake/shiroyagi/internal/imap"
	"github.com/takayoshiotake/shiroyagi/internal/mailaccount"
)

type mailPageView struct {
	Title          string
	Mailbox        string
	MailboxURL     string
	CurrentAccount mailAccountView
	Accounts       []mailAccountView
	User           mailUserView
	Messages       []mailMessageListItemView
	SelectedUID    uint32
	Selected       *mailMessageDetailView
	OOBRows        []mailMessageListItemView
}

type mailAccountView struct {
	Name       string
	Email      string
	InboxURL   string
	ComposeURL string
	Current    bool
}

type mailUserView struct {
	Initial string
}

type mailMessageListItemView struct {
	UID         uint32
	URL         string
	From        string
	Subject     string
	DateTime    string
	DisplayDate string
	Status      string
	Selected    bool
}

type mailMessageDetailView struct {
	UID           uint32
	Subject       string
	DisplayDate   string
	SenderName    string
	SenderEmail   string
	SenderInitial string
	To            string
	Cc            string
	ReplyTo       string
	MessageID     string
	InReplyTo     string
	References    string
	Status        string
	Body          string
	ReplyURL      string
	ReplyAllURL   string
	ForwardURL    string
	Attachments   []mailAttachmentView
}

type mailAttachmentView struct {
	Filename    string
	ContentType string
	Size        string
	URL         string
}

type mailPanelErrorView struct {
	Error string
}

func (s *Server) buildMailPage(
	ctx context.Context,
	session auth.UserSession,
	account mailaccount.Detail,
	mailbox string,
	selectedUID uint32,
	loadSelected bool,
) (mailPageView, error) {
	accounts, err := s.accounts.ListSummaries(ctx, session.Subject)
	if err != nil {
		return mailPageView{}, fmt.Errorf("list mail accounts: %w", err)
	}
	imapAccount, err := s.imapReaderAccount(session.Subject, account)
	if err != nil {
		return mailPageView{}, fmt.Errorf("prepare imap account: %w", err)
	}
	messages, err := mailimap.ListMessages(ctx, imapAccount, mailbox, mailboxMessageLimit)
	if err != nil {
		return mailPageView{}, err
	}

	view := mailPageView{
		Title:          mailbox,
		Mailbox:        mailbox,
		MailboxURL:     mailboxURL(account.ID, mailbox),
		CurrentAccount: newMailAccountView(account.ID, account.EmailAddress, true),
		Accounts:       make([]mailAccountView, 0, len(accounts)),
		User:           mailUserView{Initial: initial(userDisplayName(session))},
		Messages:       make([]mailMessageListItemView, 0, len(messages)),
		SelectedUID:    selectedUID,
	}
	for _, listedAccount := range accounts {
		view.Accounts = append(view.Accounts, newMailAccountView(
			listedAccount.ID,
			listedAccount.EmailAddress,
			listedAccount.ID == account.ID,
		))
	}
	for _, message := range messages {
		view.Messages = append(view.Messages, newMailMessageListItemView(
			account.ID,
			mailbox,
			message,
			message.UID == selectedUID,
		))
	}

	if loadSelected {
		message, err := mailimap.GetMessage(ctx, imapAccount, mailbox, selectedUID)
		if err != nil {
			return mailPageView{}, err
		}
		detail := newMailMessageDetailView(account.ID, mailbox, message)
		view.Selected = &detail
		view.Title = detail.Subject
	}
	return view, nil
}

func newMailAccountView(id string, email string, current bool) mailAccountView {
	return mailAccountView{
		Name:       accountDisplayName(email),
		Email:      email,
		InboxURL:   mailboxURL(id, defaultMailbox),
		ComposeURL: "/mail-accounts/" + url.PathEscape(id) + "/send",
		Current:    current,
	}
}

func newMailMessageListItemView(accountID string, mailbox string, message mailimap.MessageSummary, selected bool) mailMessageListItemView {
	return mailMessageListItemView{
		UID:         message.UID,
		URL:         messageURL(accountID, mailbox, message.UID),
		From:        message.From,
		Subject:     subjectOrUntitled(message.Subject),
		DateTime:    message.Date.Format(time.RFC3339),
		DisplayDate: formatMessageListTime(message.Date),
		Status:      messageStatus(message),
		Selected:    selected,
	}
}

func newMailMessageDetailView(accountID string, mailbox string, message mailimap.Message) mailMessageDetailView {
	senderName, senderEmail := splitAddress(message.From)
	baseURL := messageURL(accountID, mailbox, message.UID)
	attachments := make([]mailAttachmentView, 0, len(message.Attachments))
	for _, attachment := range message.Attachments {
		attachments = append(attachments, mailAttachmentView{
			Filename:    displayAttachmentFilename(attachment.Filename),
			ContentType: attachment.ContentType,
			Size:        formatByteSize(attachment.Size),
			URL:         baseURL + "/attachments/" + url.PathEscape(attachment.PartID),
		})
	}
	return mailMessageDetailView{
		UID:           message.UID,
		Subject:       subjectOrUntitled(message.Subject),
		DisplayDate:   formatIMAPTime(message.Date),
		SenderName:    senderName,
		SenderEmail:   senderEmail,
		SenderInitial: initial(senderName),
		To:            messageHeaderValue(message.To),
		Cc:            messageHeaderValue(message.Cc),
		ReplyTo:       messageHeaderValue(message.ReplyTo),
		MessageID:     messageHeaderValue(message.MessageID),
		InReplyTo:     messageHeaderValue(message.InReplyTo),
		References:    messageHeaderValue(message.References),
		Status:        messageDetailStatus(message),
		Body:          message.Body,
		ReplyURL:      baseURL + "/reply",
		ReplyAllURL:   baseURL + "/reply-all",
		ForwardURL:    baseURL + "/forward",
		Attachments:   attachments,
	}
}

func (view *mailPageView) selectOOBRows(previousUID uint32) {
	seen := make(map[uint32]bool)
	for _, uid := range []uint32{previousUID, view.SelectedUID} {
		if uid == 0 || seen[uid] {
			continue
		}
		seen[uid] = true
		for _, message := range view.Messages {
			if message.UID == uid {
				view.OOBRows = append(view.OOBRows, message)
				break
			}
		}
	}
}

func mailboxURL(accountID string, mailbox string) string {
	return "/mail-accounts/" + url.PathEscape(accountID) + "/mailboxes/" + url.PathEscape(mailbox)
}

func messageURL(accountID string, mailbox string, uid uint32) string {
	return mailboxURL(accountID, mailbox) + "/messages/" + strconv.FormatUint(uint64(uid), 10)
}

func accountDisplayName(email string) string {
	name, _, found := strings.Cut(email, "@")
	if found && name != "" {
		return name
	}
	return email
}

func userDisplayName(session auth.UserSession) string {
	for _, value := range []string{session.Name, session.PreferredUsername, session.Email, session.Subject} {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return "User"
}

func splitAddress(value string) (string, string) {
	addresses, err := mail.ParseAddressList(value)
	if err == nil && len(addresses) > 0 {
		name := addresses[0].Name
		if name == "" {
			name = addresses[0].Address
		}
		return name, addresses[0].Address
	}
	if value == "" {
		return "Unknown sender", ""
	}
	return value, ""
}

func initial(value string) string {
	for _, r := range strings.TrimSpace(value) {
		return string(unicode.ToUpper(r))
	}
	return "?"
}

func formatMessageListTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	local := value.Local()
	now := time.Now()
	if sameDay(local, now) {
		return local.Format("15:04")
	}
	if sameDay(local, now.AddDate(0, 0, -1)) {
		return "Yesterday"
	}
	if local.Year() == now.Year() {
		return local.Format("Jan 2")
	}
	return local.Format("2006-01-02")
}

func sameDay(a time.Time, b time.Time) bool {
	aYear, aMonth, aDay := a.Date()
	bYear, bMonth, bDay := b.Date()
	return aYear == bYear && aMonth == bMonth && aDay == bDay
}

func formatByteSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d bytes", size)
	}
	if size < unit*unit {
		return fmt.Sprintf("%.1f KiB", float64(size)/unit)
	}
	return fmt.Sprintf("%.1f MiB", float64(size)/(unit*unit))
}

func isHTMXRequest(rHeader string, historyRestoreHeader string) bool {
	return strings.EqualFold(rHeader, "true") && !strings.EqualFold(historyRestoreHeader, "true")
}
