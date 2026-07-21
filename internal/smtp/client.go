package smtp

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
	"net"
	"net/mail"
	stdsmtp "net/smtp"
	"net/textproto"
	"strconv"
	"strings"
	"time"
)

const (
	SecurityPlain    = "plain"
	SecuritySTARTTLS = "starttls"
	SecuritySMTPS    = "smtps"
)

type Account struct {
	Host              string
	Port              int
	Security          string
	Username          string
	Password          string
	AllowInsecureAuth bool
}

type Message struct {
	From        string
	To          string
	Cc          string
	Subject     string
	Body        string
	InReplyTo   string
	References  string
	Attachments []Attachment
}

type Attachment struct {
	Filename    string
	ContentType string
	Data        []byte
}

type OperationError struct {
	Op       string
	Host     string
	Port     int
	Security string
	Err      error
}

func (e *OperationError) Error() string {
	location := net.JoinHostPort(e.Host, strconv.Itoa(e.Port))
	return fmt.Sprintf("%s smtp host=%s security=%s: %v", e.Op, location, e.Security, e.Err)
}

func (e *OperationError) Unwrap() error {
	return e.Err
}

func Send(ctx context.Context, account Account, message Message) error {
	if err := validateMessage(message); err != nil {
		return wrapError("validate smtp message", account, err)
	}
	payload, err := buildMessage(message)
	if err != nil {
		return wrapError("build smtp message", account, err)
	}

	client, err := connect(ctx, account)
	if err != nil {
		return err
	}
	defer client.Close()

	if account.Username == "" {
		return wrapError("validate smtp auth", account, errors.New("username is required"))
	}
	if account.Password == "" {
		return wrapError("validate smtp auth", account, errors.New("password is required"))
	}
	auth := smtpAuth(account)
	if err := client.Auth(auth); err != nil {
		return wrapError("authenticate smtp server", account, err)
	}

	if err := client.Mail(message.From); err != nil {
		return wrapError("set smtp sender", account, err)
	}
	recipients, err := messageRecipients(message)
	if err != nil {
		return wrapError("validate smtp recipients", account, err)
	}
	for _, recipient := range recipients {
		if err := client.Rcpt(recipient); err != nil {
			return wrapError("set smtp recipient", account, err)
		}
	}

	writer, err := client.Data()
	if err != nil {
		return wrapError("start smtp data", account, err)
	}
	if _, err := writer.Write(payload); err != nil {
		_ = writer.Close()
		return wrapError("write smtp data", account, err)
	}
	if err := writer.Close(); err != nil {
		return wrapError("finish smtp data", account, err)
	}
	if err := client.Quit(); err != nil {
		return wrapError("quit smtp server", account, err)
	}
	return nil
}

func smtpAuth(account Account) stdsmtp.Auth {
	if account.Security == SecurityPlain && account.AllowInsecureAuth {
		return insecurePlainAuth{
			username: account.Username,
			password: account.Password,
		}
	}
	return stdsmtp.PlainAuth("", account.Username, account.Password, account.Host)
}

type insecurePlainAuth struct {
	username string
	password string
}

func (a insecurePlainAuth) Start(*stdsmtp.ServerInfo) (string, []byte, error) {
	response := []byte("\x00" + a.username + "\x00" + a.password)
	return "PLAIN", response, nil
}

func (a insecurePlainAuth) Next([]byte, bool) ([]byte, error) {
	return nil, nil
}

func connect(ctx context.Context, account Account) (*stdsmtp.Client, error) {
	select {
	case <-ctx.Done():
		return nil, wrapError("prepare smtp request", account, ctx.Err())
	default:
	}

	address := net.JoinHostPort(account.Host, strconv.Itoa(account.Port))
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	switch account.Security {
	case SecuritySMTPS:
		tlsDialer := tls.Dialer{
			NetDialer: dialer,
			Config:    &tls.Config{ServerName: account.Host},
		}
		conn, err := tlsDialer.DialContext(ctx, "tcp", address)
		if err != nil {
			return nil, wrapError("connect smtp server", account, err)
		}
		client, err := stdsmtp.NewClient(conn, account.Host)
		if err != nil {
			_ = conn.Close()
			return nil, wrapError("connect smtp server", account, err)
		}
		return client, nil
	case SecurityPlain, SecuritySTARTTLS:
		conn, err := dialer.DialContext(ctx, "tcp", address)
		if err != nil {
			return nil, wrapError("connect smtp server", account, err)
		}
		client, err := stdsmtp.NewClient(conn, account.Host)
		if err != nil {
			_ = conn.Close()
			return nil, wrapError("connect smtp server", account, err)
		}
		if account.Security == SecuritySTARTTLS {
			if ok, _ := client.Extension("STARTTLS"); !ok {
				_ = client.Close()
				return nil, wrapError("start smtp tls", account, errors.New("server does not advertise STARTTLS"))
			}
			if err := client.StartTLS(&tls.Config{ServerName: account.Host}); err != nil {
				_ = client.Close()
				return nil, wrapError("start smtp tls", account, err)
			}
		}
		return client, nil
	default:
		return nil, wrapError("validate smtp security", account, fmt.Errorf("unsupported value %q", account.Security))
	}
}

func buildMessage(message Message) ([]byte, error) {
	headers := []string{
		"From: " + message.From,
		"To: " + message.To,
	}
	if message.Cc != "" {
		headers = append(headers, "Cc: "+sanitizeHeaderValue(message.Cc))
	}
	headers = append(headers, "Subject: "+sanitizeHeaderValue(message.Subject))
	if message.InReplyTo != "" {
		headers = append(headers, "In-Reply-To: "+sanitizeHeaderValue(message.InReplyTo))
	}
	if message.References != "" {
		headers = append(headers, "References: "+sanitizeHeaderValue(message.References))
	}
	headers = append(headers, "MIME-Version: 1.0")
	if len(message.Attachments) == 0 {
		headers = append(headers,
			"Content-Type: text/plain; charset=UTF-8",
			"Content-Transfer-Encoding: 8bit",
		)
		return []byte(strings.Join(headers, "\r\n") + "\r\n\r\n" + normalizeBody(message.Body)), nil
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	headers = append(headers, "Content-Type: "+mime.FormatMediaType("multipart/mixed", map[string]string{"boundary": writer.Boundary()}))
	textHeader := make(textproto.MIMEHeader)
	textHeader.Set("Content-Type", "text/plain; charset=UTF-8")
	textHeader.Set("Content-Transfer-Encoding", "8bit")
	part, err := writer.CreatePart(textHeader)
	if err != nil {
		return nil, err
	}
	if _, err := io.WriteString(part, normalizeBody(message.Body)); err != nil {
		return nil, err
	}
	for _, attachment := range message.Attachments {
		partHeader := make(textproto.MIMEHeader)
		partHeader.Set("Content-Type", safeContentType(attachment.ContentType))
		partHeader.Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": attachment.Filename}))
		partHeader.Set("Content-Transfer-Encoding", "base64")
		part, err := writer.CreatePart(partHeader)
		if err != nil {
			return nil, err
		}
		if _, err := io.WriteString(part, encodeMIMEBase64(attachment.Data)); err != nil {
			return nil, err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return []byte(strings.Join(headers, "\r\n") + "\r\n\r\n" + body.String()), nil
}

func safeContentType(value string) string {
	mediaType, _, err := mime.ParseMediaType(value)
	if err != nil || !strings.Contains(mediaType, "/") {
		return "application/octet-stream"
	}
	return strings.ToLower(mediaType)
}

func encodeMIMEBase64(data []byte) string {
	encoded := base64.StdEncoding.EncodeToString(data)
	var result strings.Builder
	for len(encoded) > 76 {
		result.WriteString(encoded[:76])
		result.WriteString("\r\n")
		encoded = encoded[76:]
	}
	result.WriteString(encoded)
	result.WriteString("\r\n")
	return result.String()
}

func normalizeBody(body string) string {
	body = strings.ReplaceAll(body, "\r\n", "\n")
	body = strings.ReplaceAll(body, "\r", "\n")
	return strings.ReplaceAll(body, "\n", "\r\n")
}

func validateMessage(message Message) error {
	if message.From == "" {
		return errors.New("from is required")
	}
	if _, err := mail.ParseAddress(message.From); err != nil {
		return fmt.Errorf("invalid from address: %w", err)
	}
	if message.To == "" {
		return errors.New("to is required")
	}
	if _, err := messageRecipients(message); err != nil {
		return err
	}
	var attachmentBytes int64
	for _, attachment := range message.Attachments {
		if attachment.Filename == "" {
			return errors.New("attachment filename is required")
		}
		attachmentBytes += int64(len(attachment.Data))
		if attachmentBytes > 25<<20 {
			return errors.New("attachments exceed 25 MiB total limit")
		}
	}
	return nil
}

func messageRecipients(message Message) ([]string, error) {
	recipients, err := parseAddressList(message.To)
	if err != nil {
		return nil, fmt.Errorf("invalid to address: %w", err)
	}
	if message.Cc != "" {
		ccRecipients, err := parseAddressList(message.Cc)
		if err != nil {
			return nil, fmt.Errorf("invalid cc address: %w", err)
		}
		recipients = append(recipients, ccRecipients...)
	}
	if len(recipients) == 0 {
		return nil, errors.New("recipient is required")
	}
	return recipients, nil
}

func parseAddressList(value string) ([]string, error) {
	addresses, err := mail.ParseAddressList(value)
	if err != nil {
		return nil, err
	}
	recipients := make([]string, 0, len(addresses))
	for _, address := range addresses {
		recipients = append(recipients, address.Address)
	}
	return recipients, nil
}

func sanitizeHeaderValue(value string) string {
	replacer := strings.NewReplacer("\r", " ", "\n", " ")
	return replacer.Replace(value)
}

func wrapError(op string, account Account, err error) error {
	if err == nil {
		return nil
	}
	return &OperationError{
		Op:       op,
		Host:     account.Host,
		Port:     account.Port,
		Security: account.Security,
		Err:      err,
	}
}
