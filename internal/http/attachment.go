package httpserver

import (
	"errors"
	"fmt"
	"html"
	"io"
	"log"
	"mime"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"unicode"

	mailimap "github.com/takayoshiotake/shiroyagi/internal/imap"
	"github.com/takayoshiotake/shiroyagi/internal/mailaccount"
	mailsmtp "github.com/takayoshiotake/shiroyagi/internal/smtp"
)

const (
	maxUploadRequestBytes = 30 << 20
	maxUploadTotalBytes   = 25 << 20
	maxUploadFileBytes    = 25 << 20
	maxUploadFiles        = 10
)

func (s *Server) handleAttachmentDownload(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	mailbox := r.PathValue("mailbox")
	uid, err := strconv.ParseUint(r.PathValue("uid"), 10, 32)
	if err != nil || uid == 0 || mailbox == "" || !validPartID(r.PathValue("part")) {
		http.NotFound(w, r)
		return
	}

	session, _ := sessionFromContext(r.Context())
	account, found, err := s.accounts.Get(r.Context(), session.Subject, id)
	if err != nil {
		log.Printf("get mail account for attachment download: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if !found || !account.HasIMAP {
		http.NotFound(w, r)
		return
	}
	imapAccount, err := s.imapReaderAccount(session.Subject, account)
	if err != nil {
		log.Printf("prepare imap account %s for attachment download: %v", account.ID, err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	attachment, err := mailimap.GetAttachment(r.Context(), imapAccount, mailbox, uint32(uid), r.PathValue("part"))
	if err != nil {
		if errors.Is(err, mailimap.ErrMessageNotFound) || errors.Is(err, mailimap.ErrAttachmentNotFound) {
			http.NotFound(w, r)
			return
		}
		log.Printf("download attachment account=%s mailbox=%q uid=%d part=%q: %v", account.ID, mailbox, uid, r.PathValue("part"), err)
		renderIMAPError(w, account, "Could not download attachment: "+err.Error())
		return
	}

	writeAttachmentDownload(w, attachment)
}

func writeAttachmentDownload(w http.ResponseWriter, attachment mailimap.Attachment) {
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": safeDownloadFilename(attachment.Filename)}))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Content-Length", strconv.Itoa(len(attachment.Data)))
	_, _ = w.Write(attachment.Data)
}

func readUploadedAttachments(r *http.Request) ([]mailsmtp.Attachment, error) {
	if r.MultipartForm == nil {
		return nil, nil
	}
	files := r.MultipartForm.File["attachments"]
	if len(files) > maxUploadFiles {
		return nil, fmt.Errorf("too many attachments: maximum is %d", maxUploadFiles)
	}
	attachments := make([]mailsmtp.Attachment, 0, len(files))
	var total int64
	for _, header := range files {
		file, err := header.Open()
		if err != nil {
			return nil, err
		}
		data, readErr := io.ReadAll(io.LimitReader(file, maxUploadFileBytes+1))
		closeErr := file.Close()
		if readErr != nil {
			return nil, readErr
		}
		if closeErr != nil {
			return nil, closeErr
		}
		if len(data) > maxUploadFileBytes {
			return nil, fmt.Errorf("attachment %q exceeds 25 MiB limit", displayAttachmentFilename(header.Filename))
		}
		total += int64(len(data))
		if total > maxUploadTotalBytes {
			return nil, errors.New("attachments exceed 25 MiB total limit")
		}
		attachments = append(attachments, mailsmtp.Attachment{
			Filename: safeDownloadFilename(header.Filename), ContentType: header.Header.Get("Content-Type"), Data: data,
		})
	}
	return attachments, nil
}

func validPartID(value string) bool {
	if value == "" || len(value) > 64 {
		return false
	}
	for i, r := range value {
		if r == '.' && i > 0 {
			continue
		}
		if r < '0' || r > '9' {
			return false
		}
	}
	return !strings.HasSuffix(value, ".") && !strings.Contains(value, "..")
}

func displayAttachmentFilename(value string) string {
	if value == "" {
		return "(unnamed attachment)"
	}
	return strings.Map(func(r rune) rune {
		if unsafeFilenameRune(r) {
			return '�'
		}
		return r
	}, value)
}

func safeDownloadFilename(value string) string {
	value = strings.ReplaceAll(value, "\\", "/")
	value = path.Base(value)
	value = strings.Trim(displayAttachmentFilename(value), " .")
	value = strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r == ':' || unsafeFilenameRune(r) {
			return '_'
		}
		return r
	}, value)
	if value == "" || value == "." || value == ".." {
		value = "attachment"
	}
	if strings.HasPrefix(value, ".") || strings.HasPrefix(value, "-") || strings.HasPrefix(value, "~") || strings.HasPrefix(value, "|") {
		value = "attachment-" + strings.TrimLeft(value, ".-~| ")
	}
	runes := []rune(value)
	if len(runes) > 180 {
		value = string(runes[:180])
	}
	if reservedWindowsFilename(strings.ToLower(strings.TrimSuffix(value, path.Ext(value)))) {
		value = "attachment-" + value
	}
	if dangerousAttachmentExtension(strings.ToLower(path.Ext(value))) {
		value += ".download"
	}
	return value
}

func unsafeFilenameRune(r rune) bool {
	return unicode.IsControl(r) || r == '\u061c' || r == '\u200e' || r == '\u200f' ||
		(r >= '\u202a' && r <= '\u202e') || (r >= '\u2066' && r <= '\u2069')
}

func reservedWindowsFilename(name string) bool {
	switch name {
	case "con", "prn", "aux", "nul", "com1", "com2", "com3", "com4", "com5", "com6", "com7", "com8", "com9", "lpt1", "lpt2", "lpt3", "lpt4", "lpt5", "lpt6", "lpt7", "lpt8", "lpt9":
		return true
	default:
		return false
	}
}

func dangerousAttachmentExtension(extension string) bool {
	switch extension {
	case ".app", ".bat", ".cmd", ".com", ".dmg", ".exe", ".jar", ".js", ".msi", ".pkg", ".ps1", ".scr", ".sh", ".vbs":
		return true
	default:
		return false
	}
}

func smtpAttachments(attachments []mailimap.Attachment) []mailsmtp.Attachment {
	result := make([]mailsmtp.Attachment, 0, len(attachments))
	for _, attachment := range attachments {
		result = append(result, mailsmtp.Attachment{
			Filename: safeDownloadFilename(attachment.Filename), ContentType: attachment.ContentType, Data: attachment.Data,
		})
	}
	return result
}

func renderAttachmentList(w http.ResponseWriter, account mailaccount.Detail, mailbox string, message mailimap.Message) {
	if len(message.Attachments) == 0 {
		return
	}
	_, _ = fmt.Fprint(w, "\n  <h2>Attachments</h2>\n  <ul>")
	for _, attachment := range message.Attachments {
		_, _ = fmt.Fprintf(w, `
    <li><a href="/mail-accounts/%s/mailboxes/%s/messages/%d/attachments/%s">%s</a> (%s, %d bytes)</li>`,
			html.EscapeString(account.ID), url.PathEscape(mailbox), message.UID, url.PathEscape(attachment.PartID),
			html.EscapeString(displayAttachmentFilename(attachment.Filename)), html.EscapeString(attachment.ContentType), attachment.Size)
	}
	_, _ = fmt.Fprint(w, "\n  </ul>")
}
