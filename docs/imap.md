# IMAP

Current:

- mailbox route parameter, with the current UI linking to `INBOX`
- latest 100 messages
- read-only mailbox selection
- message list via `FETCH ENVELOPE`, `UID`, `INTERNALDATE`, and `RFC822.SIZE`
- message body via `BODY.PEEK[]`
- inline `text/plain` body display, with `text/html` as a fallback

Errors are logged by the HTTP handlers and shown on the IMAP error page without
logging mail passwords.

Initial protocol scope:

- LOGIN
- EXAMINE
- FETCH
- LOGOUT

Future:

- LIST
- SEARCH
- IDLE
