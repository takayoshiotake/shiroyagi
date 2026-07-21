# IMAP

Current:

- mailbox route parameter, with the current UI linking to `INBOX`
- latest 100 messages
- read-only mailbox selection for listing and reading messages
- read-write mailbox selection when updating message flags
- message list via `FETCH ENVELOPE`, `FLAGS`, `UID`, `INTERNALDATE`, and
  `RFC822.SIZE`
- message body via `BODY.PEEK[]`
- inline `text/plain` body display, with `text/html` as a fallback
- attachment metadata display and authenticated downloads by MIME part ID
- non-TLS `IMAP` LOGIN is allowed for `localhost`; other hosts require
  `IMAP_ALLOW_INSECURE_AUTH=true`
- `\Answered` is added to the original message after a successful reply
- `$Forwarded` is added to the original message after a successful forward
- message lists and message detail pages show reply and forward status from
  IMAP flags

Errors are logged by the HTTP handlers and shown on the IMAP error page without
logging mail passwords.

Attachments are treated as opaque bytes and are never previewed, executed, or
extracted. Message parsing is bounded to 50 MiB, 100 MIME parts, and 10 nested
MIME levels; each decoded attachment is limited to 25 MiB. The original
filename is display-only. Downloads use a sanitized filename,
`application/octet-stream`, attachment disposition, and `nosniff`.

Initial protocol scope:

- LOGIN
- EXAMINE
- FETCH
- SELECT
- STORE
- LOGOUT

`$Forwarded` is an IMAP keyword and depends on server support. If SMTP sending
succeeds but a flag update fails, the sent message is kept and the result page
shows a warning.

Future:

- LIST
- SEARCH
- IDLE
