# Routes

Shiroyagi currently exposes server-rendered Web routes. These routes return full
HTML pages, HTML fragments for HTMX swaps, redirects after form submissions, file
downloads, health-check text, or error responses.

The existing routes are not JSON APIs. If machine-readable APIs are added later,
they should use a separate namespace such as `/api/v1/...` and be documented
separately from these Web routes.

## Route Types

| Type | Meaning |
| --- | --- |
| Page | A full HTML page rendered by the server. |
| Fragment | An HTML fragment intended for an HTMX target. |
| Redirect | A redirect response, usually after a successful form submission or navigation entry point. |
| Download | A binary file response with download headers. |
| Text | A plain text operational response. |
| Error | A not found response, plain text error, or rendered HTML error page. |

## Request Handling Policy

- Standard browser requests should receive full pages, redirects, downloads, or
  standard error responses.
- HTMX requests are detected with `HX-Request: true`.
- `HX-History-Restore-Request: true` is treated as a normal browser request so
  the client receives a full page for history restoration.
- Routes that vary on HTMX headers should include `Vary` for `HX-Request`,
  `HX-Target`, and `HX-History-Restore-Request`.
- Form POST routes should remain usable as standard HTML form submissions.
- Successful POST routes should prefer `303 See Other` when redirecting after a
  state change.

## Web Routes

| Method | Path | Purpose | Normal Response | HTMX Response | Success Response |
| --- | --- | --- | --- | --- | --- |
| GET | `/static/{path...}` | Serve embedded static assets. | Asset response | Same as normal | Asset response |
| GET | `/` | Entry point after sign-in. | Redirect | Redirect | `302 Found` to `/mail-accounts` |
| GET | `/mail-accounts` | Choose the first configured IMAP account, or settings when none are ready. | Redirect | Redirect | `302 Found` to mailbox or `/settings/mail-accounts` |
| GET | `/settings/mail-accounts` | Show mail account settings. | Page | Page | `account-settings-page` |
| GET | `/mail-accounts/new` | Show add-account form. | Page | Page | `account-new-page` |
| POST | `/mail-accounts/create` | Create a mail account record. | Page on validation error; error otherwise | Same as normal | `303 See Other` to `/settings/mail-accounts` |
| GET | `/mail-accounts/{id}/delete` | Confirm mail account deletion. | Page | Page | `account-removal-page` |
| POST | `/mail-accounts/{id}/delete` | Delete a mail account and saved protocol settings. | Error on failure | Same as normal | `303 See Other` to `/settings/mail-accounts` |
| GET | `/mail-accounts/{id}/send` | Show compose form for a new message. | Page | Page | `compose-page` |
| POST | `/mail-accounts/{id}/send` | Send a new, reply, reply-all, or forwarded message. | Page on validation, protocol error, or sent status | Same as normal | `compose-status-page` |
| GET | `/mail-accounts/{id}/mailboxes/{mailbox}` | Show a mailbox and its latest messages. | Page | Fragment or page based on `HX-Target` | `mail-page`, `message-list-panel`, or `message-empty-response` |
| GET | `/mail-accounts/{id}/mailboxes/{mailbox}/messages/{uid}` | Show a message. | Page | Fragment when targeting `message-detail` | `mail-page` or `message-detail-response` |
| GET | `/mail-accounts/{id}/mailboxes/{mailbox}/messages/{uid}/attachments/{part}` | Download a message attachment. | Download | Download | Attachment bytes |
| GET | `/mail-accounts/{id}/mailboxes/{mailbox}/messages/{uid}/reply` | Show reply compose form. | Page | Page | `compose-page` |
| GET | `/mail-accounts/{id}/mailboxes/{mailbox}/messages/{uid}/reply-all` | Show reply-all compose form. | Page | Page | `compose-page` |
| GET | `/mail-accounts/{id}/mailboxes/{mailbox}/messages/{uid}/forward` | Show forward compose form. | Page | Page | `compose-page` |
| GET | `/mail-accounts/{id}/imap/edit` | Show IMAP settings form. | Page | Page | `account-protocol-page` |
| POST | `/mail-accounts/{id}/imap/save` | Save IMAP settings. | Page on validation error; error otherwise | Same as normal | `303 See Other` to the IMAP edit page |
| GET | `/mail-accounts/{id}/imap/delete` | Confirm IMAP settings removal. | Page | Page | `account-removal-page` |
| POST | `/mail-accounts/{id}/imap/delete` | Remove IMAP settings from a mail account. | Error on failure | Same as normal | `303 See Other` to the IMAP edit page |
| GET | `/mail-accounts/{id}/smtp/edit` | Show SMTP settings form. | Page | Page | `account-protocol-page` |
| POST | `/mail-accounts/{id}/smtp/save` | Save SMTP settings. | Page on validation error; error otherwise | Same as normal | `303 See Other` to the SMTP edit page |
| GET | `/mail-accounts/{id}/smtp/delete` | Confirm SMTP settings removal. | Page | Page | `account-removal-page` |
| POST | `/mail-accounts/{id}/smtp/delete` | Remove SMTP settings from a mail account. | Error on failure | Same as normal | `303 See Other` to the SMTP edit page |
| GET | `/signin` | Show sign-in page, or leave it when already signed in. | Page or redirect | Page or redirect | `sign-in-page` or `302 Found` to `/` |
| GET | `/auth/login` | Start the OIDC authorization flow. | Redirect | Redirect | `302 Found` to the identity provider |
| GET | `/auth/callback` | Complete the OIDC authorization flow and create a session. | Redirect or error | Redirect or error | `302 Found` to `/` |
| GET | `/auth/logout` | Clear the local session and require fresh authentication. | Redirect | Redirect | `302 Found` to `/` |
| GET | `/healthz` | Report basic process health. | Text | Text | `ok` |

All routes except static assets, sign-in, OIDC auth routes, and `healthz` require
an application session. Unauthenticated requests are redirected to `/signin`.

## HTMX Responses

Only the mailbox and message routes currently vary their successful response by
HTMX headers.

| Route | HTMX Target | Response Template | Notes |
| --- | --- | --- | --- |
| `GET /mail-accounts/{id}/mailboxes/{mailbox}` | `message-list-panel` | `message-list-panel` | Keeps the previously selected message when possible. |
| `GET /mail-accounts/{id}/mailboxes/{mailbox}` | `message-detail` | `message-empty-response` | Clears the detail panel and updates selected rows out of band. |
| `GET /mail-accounts/{id}/mailboxes/{mailbox}` | Other or empty | `mail-page` | Falls back to the full mail page. |
| `GET /mail-accounts/{id}/mailboxes/{mailbox}/messages/{uid}` | `message-detail` | `message-detail-response` | Replaces the message detail panel and updates selected rows out of band. |
| `GET /mail-accounts/{id}/mailboxes/{mailbox}/messages/{uid}` | Other or empty | `mail-page` | Falls back to the full mail page. |

HTMX error responses for mailbox and message loading use scoped fragments when
possible:

| Situation | Response Template | Status |
| --- | --- | --- |
| Mailbox loading fails during an HTMX request. | `message-list-error` | `502 Bad Gateway` |
| Message loading fails during an HTMX request. | `message-panel-error` | `502 Bad Gateway` |

## Error Responses

Error handling is currently mixed by route and failure type:

- Missing or invalid path values return `404 Not Found`.
- Unexpected server-side failures usually return a plain text `500 Internal
  Server Error`.
- OIDC callback failures return plain text `400`, `401`, or `502` responses.
- IMAP and SMTP connection or protocol failures are rendered as
  `protocol-error-page` with `502 Bad Gateway`.
- Form validation failures are rendered by the relevant page template with
  `422 Unprocessable Entity` or `409 Conflict`.
- Attachment downloads return `404 Not Found` for missing messages, attachments,
  accounts, or IMAP configuration.

Future cleanup should prefer rendered HTML errors for user-facing browser
workflows, scoped HTML fragments for HTMX workflows, and plain text only for
operational endpoints or unrecoverable server errors.

## Future JSON API

JSON APIs are not part of the current route surface. If added later:

- Use a separate namespace such as `/api/v1/...`.
- Document request and response schemas separately from this file.
- Keep server-rendered Web routes stable for browser workflows.
- Do not rename existing HTML routes into `/api/...`.
