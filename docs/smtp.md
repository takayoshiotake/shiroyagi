# SMTP

## Development check

Use the [development mail settings](../README.md#development-mail) to configure
authenticated SMTP submission to Stalwart. Send to a local test address such as
`alice@example.test`, then open the recipient's inbox over IMAP to verify
delivery. Stalwart rejects external recipients in this environment.

Messages with attachments are sent as `multipart/mixed`, with attachments
base64 encoded and marked with attachment disposition. A compose request may
contain at most 10 files and 25 MiB of attachment data in total. Replying does
not include attachments from the original message. Forwarding includes the
original attachments as opaque bytes, plus any files explicitly selected in
the forward form.

## Security modes

- `Plain` connects without TLS. This is intended for the development Stalwart
  service or for deployments where TLS is terminated before Shiroyagi reaches
  the SMTP server. SMTP AUTH over this mode is rejected by default unless the
  server is reached as `localhost`. Set `SMTP_ALLOW_INSECURE_AUTH=true` to
  explicitly allow SMTP AUTH over non-TLS `Plain` connections.
- `STARTTLS` connects in plaintext first, then requires the SMTP server to
  advertise and complete STARTTLS before authentication or message submission.
- `SMTPS` connects with TLS from the start.

SMTP authentication is required by Shiroyagi. Stalwart verifies the credentials
of the test account configured for SMTP.
