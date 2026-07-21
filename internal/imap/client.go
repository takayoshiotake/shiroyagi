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
	"unicode"
	"unicode/utf8"

	goimap "github.com/emersion/go-imap"
	goimapclient "github.com/emersion/go-imap/client"
)

const (
	SecurityIMAP  = "imap"
	SecurityIMAPS = "imaps"

	defaultLimit    = 100
	bodyLimit       = 2 << 20
	messageLimit    = 50 << 20
	attachmentLimit = 25 << 20
	maxMIMEParts    = 100
	maxMIMEDepth    = 10
	forwardedFlag   = "$Forwarded"
)

type Account struct {
	Host              string
	Port              int
	Security          string
	Username          string
	Password          string
	AllowInsecureAuth bool
}

type MessageSummary struct {
	UID       uint32
	Subject   string
	From      string
	Date      time.Time
	Size      int64
	Answered  bool
	Forwarded bool
}

type Message struct {
	UID         uint32
	Subject     string
	From        string
	ReplyTo     string
	To          string
	Cc          string
	Date        time.Time
	Body        string
	MessageID   string
	InReplyTo   string
	References  string
	Answered    bool
	Forwarded   bool
	Attachments []Attachment
}

type Attachment struct {
	PartID      string
	Filename    string
	ContentType string
	Size        int64
	Data        []byte
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
		goimap.FetchFlags,
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
			UID:       msg.Uid,
			Size:      int64(msg.Size),
			Date:      msg.InternalDate,
			Answered:  hasFlag(msg.Flags, goimap.AnsweredFlag),
			Forwarded: hasFlag(msg.Flags, forwardedFlag),
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
	bodyBytes, envelope, flags, fetchedUID, err := fetchRawMessage(ctx, account, mailbox, uid)
	if err != nil {
		return Message{}, err
	}
	parsed, err := parseMessageContent(bodyBytes)
	if err != nil {
		return Message{}, wrapError("parse mailbox message body", account, mailbox, uid, err)
	}
	headers, err := extractHeaders(bodyBytes)
	if err != nil {
		return Message{}, wrapError("parse mailbox message headers", account, mailbox, uid, err)
	}

	message := Message{
		UID:         fetchedUID,
		Body:        parsed.body,
		Attachments: attachmentMetadata(parsed.attachments),
		InReplyTo:   headers.Get("In-Reply-To"),
		References:  headers.Get("References"),
		Answered:    hasFlag(flags, goimap.AnsweredFlag),
		Forwarded:   hasFlag(flags, forwardedFlag),
	}
	if envelope != nil {
		message.Subject = envelope.Subject
		message.From = formatAddresses(envelope.From)
		message.ReplyTo = firstAddress(envelope.ReplyTo)
		message.To = formatAddresses(envelope.To)
		message.Cc = formatAddresses(envelope.Cc)
		message.Date = envelope.Date
		message.MessageID = envelope.MessageId
		if message.InReplyTo == "" {
			message.InReplyTo = envelope.InReplyTo
		}
	}
	if message.MessageID == "" {
		message.MessageID = headers.Get("Message-ID")
	}
	return message, nil
}

func GetAttachment(ctx context.Context, account Account, mailbox string, uid uint32, partID string) (Attachment, error) {
	bodyBytes, _, _, _, err := fetchRawMessage(ctx, account, mailbox, uid)
	if err != nil {
		return Attachment{}, err
	}
	parsed, err := parseMessageContent(bodyBytes)
	if err != nil {
		return Attachment{}, wrapError("parse mailbox attachment", account, mailbox, uid, err)
	}
	for _, attachment := range parsed.attachments {
		if attachment.PartID == partID {
			return attachment, nil
		}
	}
	return Attachment{}, ErrAttachmentNotFound
}

func GetAttachments(ctx context.Context, account Account, mailbox string, uid uint32) ([]Attachment, error) {
	bodyBytes, _, _, _, err := fetchRawMessage(ctx, account, mailbox, uid)
	if err != nil {
		return nil, err
	}
	parsed, err := parseMessageContent(bodyBytes)
	if err != nil {
		return nil, wrapError("parse mailbox attachments", account, mailbox, uid, err)
	}
	return parsed.attachments, nil
}

func fetchRawMessage(ctx context.Context, account Account, mailbox string, uid uint32) ([]byte, *goimap.Envelope, []string, uint32, error) {
	client, err := connect(ctx, account)
	if err != nil {
		return nil, nil, nil, 0, err
	}
	defer client.Close()
	defer client.Logout()

	if _, err := client.Select(mailbox, true); err != nil {
		return nil, nil, nil, 0, wrapError("select mailbox", account, mailbox, uid, err)
	}

	bodySection := &goimap.BodySectionName{Peek: true}
	seqSet := new(goimap.SeqSet)
	seqSet.AddNum(uid)
	messages, err := uidFetchMessages(client, seqSet, []goimap.FetchItem{
		bodySection.FetchItem(),
		goimap.FetchEnvelope,
		goimap.FetchFlags,
		goimap.FetchUid,
	})
	if err != nil {
		return nil, nil, nil, 0, wrapError("fetch mailbox message", account, mailbox, uid, err)
	}
	if len(messages) == 0 {
		return nil, nil, nil, 0, ErrMessageNotFound
	}

	msg := messages[0]
	bodyReader := msg.GetBody(bodySection)
	if bodyReader == nil {
		return nil, nil, nil, 0, wrapError("fetch mailbox message body", account, mailbox, uid, errors.New("empty body"))
	}

	bodyBytes, err := readLimited(bodyReader, messageLimit)
	if err != nil {
		return nil, nil, nil, 0, wrapError("read mailbox message body", account, mailbox, uid, err)
	}
	return bodyBytes, msg.Envelope, msg.Flags, msg.Uid, nil
}

var ErrMessageNotFound = errors.New("message not found")
var ErrAttachmentNotFound = errors.New("attachment not found")

func MarkAnswered(ctx context.Context, account Account, mailbox string, uid uint32) error {
	return markMessageFlag(ctx, account, mailbox, uid, goimap.AnsweredFlag, "mark mailbox message answered")
}

func MarkForwarded(ctx context.Context, account Account, mailbox string, uid uint32) error {
	return markMessageFlag(ctx, account, mailbox, uid, forwardedFlag, "mark mailbox message forwarded")
}

func markMessageFlag(ctx context.Context, account Account, mailbox string, uid uint32, flag string, op string) error {
	client, err := connect(ctx, account)
	if err != nil {
		return err
	}
	defer client.Close()
	defer client.Logout()

	if _, err := client.Select(mailbox, false); err != nil {
		return wrapError("select mailbox", account, mailbox, uid, err)
	}

	seqSet := new(goimap.SeqSet)
	seqSet.AddNum(uid)
	item := goimap.FormatFlagsOp(goimap.AddFlags, true)
	if err := client.UidStore(seqSet, item, []interface{}{flag}, nil); err != nil {
		return wrapError(op, account, mailbox, uid, err)
	}
	return nil
}

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
		if !account.AllowInsecureAuth && !isLocalhost(account.Host) {
			return nil, wrapError("validate imap auth", account, "", 0, errors.New("insecure IMAP auth is disabled"))
		}
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

func isLocalhost(host string) bool {
	normalized := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(host)), ".")
	if normalized == "localhost" {
		return true
	}
	ip := net.ParseIP(normalized)
	return ip != nil && ip.IsLoopback()
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
		if formatted := formatAddress(address); formatted != "" {
			parts = append(parts, formatted)
		}
	}
	return strings.Join(parts, ", ")
}

func firstAddress(addresses []*goimap.Address) string {
	if len(addresses) == 0 {
		return ""
	}
	return formatAddress(addresses[0])
}

func formatAddress(address *goimap.Address) string {
	if address == nil || address.MailboxName == "" || address.HostName == "" {
		return ""
	}
	email := address.Address()
	switch {
	case address.PersonalName != "" && email != "":
		return fmt.Sprintf("%s <%s>", address.PersonalName, email)
	case email != "":
		return email
	case address.PersonalName != "":
		return address.PersonalName
	default:
		return ""
	}
}

func hasFlag(flags []string, want string) bool {
	for _, flag := range flags {
		if strings.EqualFold(flag, want) {
			return true
		}
	}
	return false
}

func reverseSummaries(summaries []MessageSummary) {
	for i, j := 0, len(summaries)-1; i < j; i, j = i+1, j-1 {
		summaries[i], summaries[j] = summaries[j], summaries[i]
	}
}

func extractTextBody(raw []byte) (string, error) {
	parsed, err := parseMessageContent(raw)
	return parsed.body, err
}

func extractHeaders(raw []byte) (mail.Header, error) {
	msg, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	return msg.Header, nil
}

type parsedMessageContent struct {
	body        string
	plainBody   string
	htmlBody    string
	attachments []Attachment
	parts       int
}

func parseMessageContent(raw []byte) (parsedMessageContent, error) {
	msg, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		return parsedMessageContent{}, err
	}
	parsed := parsedMessageContent{}
	if err := parseEntity(msg.Header, msg.Body, "", 0, &parsed); err != nil {
		return parsedMessageContent{}, err
	}
	switch {
	case parsed.plainBody != "":
		parsed.body = parsed.plainBody
	case parsed.htmlBody != "":
		parsed.body = parsed.htmlBody
	default:
		parsed.body = "(no text body)"
	}
	return parsed, nil
}

func parseEntity(header mail.Header, body io.Reader, partID string, depth int, parsed *parsedMessageContent) error {
	if depth > maxMIMEDepth {
		return fmt.Errorf("MIME nesting exceeds %d levels", maxMIMEDepth)
	}
	parsed.parts++
	if parsed.parts > maxMIMEParts {
		return fmt.Errorf("MIME message exceeds %d parts", maxMIMEParts)
	}

	mediaType, params, err := parseContentType(header.Get("Content-Type"))
	if err != nil {
		return err
	}

	if strings.HasPrefix(mediaType, "multipart/") {
		boundary := params["boundary"]
		if boundary == "" {
			return fmt.Errorf("missing multipart boundary")
		}
		reader := multipart.NewReader(body, boundary)
		partNumber := 0
		for {
			part, err := reader.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				return err
			}
			partNumber++
			childID := strconv.Itoa(partNumber)
			if partID != "" {
				childID = partID + "." + childID
			}
			if err := parseEntity(mail.Header(part.Header), part, childID, depth+1, parsed); err != nil {
				return err
			}
		}
		return nil
	}

	if partID == "" {
		partID = "1"
	}
	filename := attachmentFilename(header, params)
	if isAttachment(header.Get("Content-Disposition")) || filename != "" {
		decoded, err := decodeTransferEncodingLimited(header.Get("Content-Transfer-Encoding"), body, attachmentLimit)
		if err != nil {
			return fmt.Errorf("read attachment part %s: %w", partID, err)
		}
		if filename == "" {
			filename = "(unnamed attachment)"
		}
		parsed.attachments = append(parsed.attachments, Attachment{
			PartID: partID, Filename: filename, ContentType: mediaType,
			Size: int64(len(decoded)), Data: decoded,
		})
		return nil
	}

	if mediaType != "text/plain" && mediaType != "text/html" {
		return nil
	}

	decoded, err := decodeTransferEncodingLimited(header.Get("Content-Transfer-Encoding"), body, bodyLimit)
	if err != nil {
		return err
	}
	if mediaType == "text/plain" && parsed.plainBody == "" {
		parsed.plainBody = string(decoded)
	}
	if mediaType == "text/html" && parsed.htmlBody == "" {
		parsed.htmlBody = string(decoded)
	}
	return nil
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
	return decodeTransferEncodingLimited(encoding, body, bodyLimit)
}

func decodeTransferEncodingLimited(encoding string, body io.Reader, limit int64) ([]byte, error) {
	var reader io.Reader = body
	switch strings.ToLower(strings.TrimSpace(encoding)) {
	case "base64":
		reader = base64.NewDecoder(base64.StdEncoding, body)
	case "quoted-printable":
		reader = quotedprintable.NewReader(body)
	}
	return readLimited(reader, limit)
}

func readLimited(reader io.Reader, limit int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("content exceeds %d byte limit", limit)
	}
	return data, nil
}

func attachmentFilename(header mail.Header, contentTypeParams map[string]string) string {
	_, dispositionParams, _ := mime.ParseMediaType(header.Get("Content-Disposition"))
	filename := dispositionParams["filename"]
	if filename == "" {
		filename = contentTypeParams["name"]
	}
	if decoded, err := new(mime.WordDecoder).DecodeHeader(filename); err == nil {
		filename = decoded
	}
	if !utf8.ValidString(filename) {
		filename = strings.ToValidUTF8(filename, "�")
	}
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || r == '\u061c' || r == '\u200e' || r == '\u200f' ||
			(r >= '\u202a' && r <= '\u202e') || (r >= '\u2066' && r <= '\u2069') {
			return '�'
		}
		return r
	}, filename)
}

func attachmentMetadata(attachments []Attachment) []Attachment {
	metadata := make([]Attachment, len(attachments))
	for i, attachment := range attachments {
		attachment.Data = nil
		metadata[i] = attachment
	}
	return metadata
}

func isAttachment(disposition string) bool {
	if disposition == "" {
		return false
	}
	dispositionType, _, _ := strings.Cut(disposition, ";")
	return strings.EqualFold(strings.TrimSpace(dispositionType), "attachment")
}
