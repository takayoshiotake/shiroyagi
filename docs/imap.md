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
- non-TLS `IMAP` LOGIN is allowed for `localhost`; other hosts require
  `IMAP_ALLOW_INSECURE_AUTH=true`
- `\Answered` is added to the original message after a successful reply
- `$Forwarded` is added to the original message after a successful forward
- message lists and message detail pages show reply and forward status from
  IMAP flags

Errors are logged by the HTTP handlers and shown on the IMAP error page without
logging mail passwords.

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
