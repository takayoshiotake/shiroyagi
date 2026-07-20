# SMTP

Development uses Mailpit.

Postfix is not required initially.

## Development check

Use Mailpit as the SMTP server during development.

When the Go application runs on the host:

```text
Host: localhost
Port: 1025
Protocol: Plain
Username: dev@example.test
Password: dev
```

When the Go application runs in compose:

```text
Host: mailpit
Port: 1025
Protocol: Plain
Username: dev@example.test
Password: dev
```

After saving the SMTP settings for a mail account, open `Send` from the
mail account list, enter a recipient, subject, and body, then send the message.
The sent message should appear in the Mailpit Web UI at
http://localhost:8025.

## Security modes

- `Plain` connects without TLS. This is intended for the development Mailpit
  service or for deployments where TLS is terminated before Shiroyagi reaches
  the SMTP server. SMTP AUTH over this mode is rejected by default unless the
  server is reached as `localhost`. Set `SMTP_ALLOW_INSECURE_AUTH=true` to
  explicitly allow SMTP AUTH over non-TLS `Plain` connections.
- `STARTTLS` connects in plaintext first, then requires the SMTP server to
  advertise and complete STARTTLS before authentication or message submission.
- `SMTPS` connects with TLS from the start.

SMTP authentication is required by Shiroyagi. The development Mailpit service is
configured to accept SMTP AUTH for any username and password so the authenticated
send path can be verified locally.
