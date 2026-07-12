package imap

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"strconv"
	"strings"
	"time"

	goimap "github.com/emersion/go-imap"
	goimapclient "github.com/emersion/go-imap/client"
)

const (
	SecurityIMAP  = "imap"
	SecurityIMAPS = "imaps"

	defaultLimit = 100
	bodyLimit    = 2 << 20
)

type Account struct {
	Host     string
	Port     int
	Security string
	Username string
	Password string
}

type MessageSummary struct {
	UID     uint32
	Subject string
	From    string
	Date    time.Time
	Size    int64
}

type Message struct {
	UID     uint32
	Subject string
	From    string
	Date    time.Time
	Body    string
}

type OperationError struct {
	Op       string
	Host     string
	Port     int
	Security string
	Mailbox  string
	UID      uint32
	Err      error
}

func (e *OperationError) Error() string {
	location := net.JoinHostPort(e.Host, strconv.Itoa(e.Port))
	if e.Mailbox != "" && e.UID != 0 {
		return fmt.Sprintf("%s imap host=%s security=%s mailbox=%q uid=%d: %v", e.Op, location, e.Security, e.Mailbox, e.UID, e.Err)
	}
	if e.Mailbox != "" {
		return fmt.Sprintf("%s imap host=%s security=%s mailbox=%q: %v", e.Op, location, e.Security, e.Mailbox, e.Err)
	}
	return fmt.Sprintf("%s imap host=%s security=%s: %v", e.Op, location, e.Security, e.Err)
}

func (e *OperationError) Unwrap() error {
	return e.Err
}

func ListMessages(ctx context.Context, account Account, mailbox string, limit uint32) ([]MessageSummary, error) {
	if limit == 0 {
		limit = defaultLimit
	}

	client, err := connect(ctx, account)
	if err != nil {
		return nil, err
	}
	defer client.Close()
	defer client.Logout()

	selected, err := client.Select(mailbox, true)
	if err != nil {
		return nil, wrapError("select mailbox", account, mailbox, 0, err)
	}
	if selected.Messages == 0 {
		return nil, nil
	}

	start := uint32(1)
	if selected.Messages > limit {
		start = selected.Messages - limit + 1
	}
	seqSet := new(goimap.SeqSet)
	seqSet.AddRange(start, selected.Messages)

	messages, err := fetchMessages(client, seqSet, []goimap.FetchItem{
		goimap.FetchEnvelope,
		goimap.FetchInternalDate,
		goimap.FetchRFC822Size,
		goimap.FetchUid,
	})
	if err != nil {
		return nil, wrapError("fetch mailbox messages", account, mailbox, 0, err)
	}

	summaries := make([]MessageSummary, 0, len(messages))
	for _, msg := range messages {
		summary := MessageSummary{
			UID:  msg.Uid,
			Size: int64(msg.Size),
			Date: msg.InternalDate,
		}
		if msg.Envelope != nil {
			summary.Subject = msg.Envelope.Subject
			summary.From = formatAddresses(msg.Envelope.From)
			if !msg.Envelope.Date.IsZero() {
				summary.Date = msg.Envelope.Date
			}
		}
		summaries = append(summaries, summary)
	}
	reverseSummaries(summaries)
	return summaries, nil
}

func GetMessage(ctx context.Context, account Account, mailbox string, uid uint32) (Message, error) {
	client, err := connect(ctx, account)
	if err != nil {
		return Message{}, err
	}
	defer client.Close()
	defer client.Logout()

	if _, err := client.Select(mailbox, true); err != nil {
		return Message{}, wrapError("select mailbox", account, mailbox, uid, err)
	}

	bodySection := &goimap.BodySectionName{Peek: true}
	seqSet := new(goimap.SeqSet)
	seqSet.AddNum(uid)
	messages, err := uidFetchMessages(client, seqSet, []goimap.FetchItem{
		bodySection.FetchItem(),
		goimap.FetchEnvelope,
		goimap.FetchUid,
	})
	if err != nil {
		return Message{}, wrapError("fetch mailbox message", account, mailbox, uid, err)
	}
	if len(messages) == 0 {
		return Message{}, ErrMessageNotFound
	}

	msg := messages[0]
	bodyReader := msg.GetBody(bodySection)
	if bodyReader == nil {
		return Message{}, wrapError("fetch mailbox message body", account, mailbox, uid, errors.New("empty body"))
	}

	bodyBytes, err := io.ReadAll(bodyReader)
	if err != nil {
		return Message{}, wrapError("read mailbox message body", account, mailbox, uid, err)
	}
	body, err := extractTextBody(bodyBytes)
	if err != nil {
		return Message{}, wrapError("parse mailbox message body", account, mailbox, uid, err)
	}

	message := Message{
		UID:  msg.Uid,
		Body: body,
	}
	if msg.Envelope != nil {
		message.Subject = msg.Envelope.Subject
		message.From = formatAddresses(msg.Envelope.From)
		message.Date = msg.Envelope.Date
	}
	return message, nil
}

var ErrMessageNotFound = errors.New("message not found")

func connect(ctx context.Context, account Account) (*goimapclient.Client, error) {
	select {
	case <-ctx.Done():
		return nil, wrapError("prepare imap request", account, "", 0, ctx.Err())
	default:
	}

	address := net.JoinHostPort(account.Host, strconv.Itoa(account.Port))

	var (
		client *goimapclient.Client
		err    error
	)
	switch account.Security {
	case SecurityIMAPS:
		client, err = goimapclient.DialTLS(address, &tls.Config{ServerName: account.Host})
	case SecurityIMAP:
		dialer := &net.Dialer{Timeout: 10 * time.Second}
		conn, dialErr := dialer.Dial("tcp", address)
		if dialErr != nil {
			return nil, wrapError("connect imap server", account, "", 0, dialErr)
		}
		client, err = goimapclient.New(conn)
	default:
		return nil, wrapError("validate imap security", account, "", 0, fmt.Errorf("unsupported value %q", account.Security))
	}
	if err != nil {
		return nil, wrapError("connect imap server", account, "", 0, err)
	}

	if err := client.Login(account.Username, account.Password); err != nil {
		client.Close()
		return nil, wrapError("login imap server", account, "", 0, err)
	}
	return client, nil
}

func wrapError(op string, account Account, mailbox string, uid uint32, err error) error {
	if err == nil {
		return nil
	}
	return &OperationError{
		Op:       op,
		Host:     account.Host,
		Port:     account.Port,
		Security: account.Security,
		Mailbox:  mailbox,
		UID:      uid,
		Err:      err,
	}
}

func fetchMessages(client *goimapclient.Client, seqSet *goimap.SeqSet, items []goimap.FetchItem) ([]*goimap.Message, error) {
	messages := make(chan *goimap.Message, 128)
	done := make(chan error, 1)
	go func() {
		done <- client.Fetch(seqSet, items, messages)
	}()

	var out []*goimap.Message
	for message := range messages {
		out = append(out, message)
	}
	if err := <-done; err != nil {
		return nil, err
	}
	return out, nil
}

func uidFetchMessages(client *goimapclient.Client, seqSet *goimap.SeqSet, items []goimap.FetchItem) ([]*goimap.Message, error) {
	messages := make(chan *goimap.Message, 16)
	done := make(chan error, 1)
	go func() {
		done <- client.UidFetch(seqSet, items, messages)
	}()

	var out []*goimap.Message
	for message := range messages {
		out = append(out, message)
	}
	if err := <-done; err != nil {
		return nil, err
	}
	return out, nil
}

func formatAddresses(addresses []*goimap.Address) string {
	if len(addresses) == 0 {
		return ""
	}
	parts := make([]string, 0, len(addresses))
	for _, address := range addresses {
		if address == nil || address.MailboxName == "" || address.HostName == "" {
			continue
		}
		email := address.Address()
		switch {
		case address.PersonalName != "" && email != "":
			parts = append(parts, fmt.Sprintf("%s <%s>", address.PersonalName, email))
		case email != "":
			parts = append(parts, email)
		case address.PersonalName != "":
			parts = append(parts, address.PersonalName)
		}
	}
	return strings.Join(parts, ", ")
}

func reverseSummaries(summaries []MessageSummary) {
	for i, j := 0, len(summaries)-1; i < j; i, j = i+1, j-1 {
		summaries[i], summaries[j] = summaries[j], summaries[i]
	}
}

func extractTextBody(raw []byte) (string, error) {
	msg, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	body, ok, err := extractEntityText(msg.Header, msg.Body)
	if err != nil {
		return "", err
	}
	if !ok {
		return "(no text body)", nil
	}
	return body, nil
}

func extractEntityText(header mail.Header, body io.Reader) (string, bool, error) {
	mediaType, params, err := parseContentType(header.Get("Content-Type"))
	if err != nil {
		return "", false, err
	}

	if strings.HasPrefix(mediaType, "multipart/") {
		boundary := params["boundary"]
		if boundary == "" {
			return "", false, fmt.Errorf("missing multipart boundary")
		}
		reader := multipart.NewReader(body, boundary)
		var htmlCandidate string
		for {
			part, err := reader.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				return "", false, err
			}
			partBody, ok, err := extractEntityText(mail.Header(part.Header), part)
			if err != nil {
				return "", false, err
			}
			if ok {
				partType, _, _ := parseContentType(part.Header.Get("Content-Type"))
				if partType == "text/plain" {
					return partBody, true, nil
				}
				if htmlCandidate == "" {
					htmlCandidate = partBody
				}
			}
		}
		if htmlCandidate != "" {
			return htmlCandidate, true, nil
		}
		return "", false, nil
	}

	if mediaType != "text/plain" && mediaType != "text/html" {
		return "", false, nil
	}
	if isAttachment(header.Get("Content-Disposition")) {
		return "", false, nil
	}

	decoded, err := decodeTransferEncoding(header.Get("Content-Transfer-Encoding"), body)
	if err != nil {
		return "", false, err
	}
	return string(decoded), true, nil
}

func parseContentType(value string) (string, map[string]string, error) {
	if value == "" {
		return "text/plain", nil, nil
	}
	mediaType, params, err := mime.ParseMediaType(value)
	if err != nil {
		return "", nil, err
	}
	return strings.ToLower(mediaType), params, nil
}

func decodeTransferEncoding(encoding string, body io.Reader) ([]byte, error) {
	switch strings.ToLower(strings.TrimSpace(encoding)) {
	case "base64":
		return io.ReadAll(io.LimitReader(base64.NewDecoder(base64.StdEncoding, body), bodyLimit))
	case "quoted-printable":
		return io.ReadAll(io.LimitReader(quotedprintable.NewReader(body), bodyLimit))
	default:
		return io.ReadAll(io.LimitReader(body, bodyLimit))
	}
}

func isAttachment(disposition string) bool {
	if disposition == "" {
		return false
	}
	mediaType, _, err := mime.ParseMediaType(disposition)
	if err != nil {
		return false
	}
	return strings.EqualFold(mediaType, "attachment")
}
