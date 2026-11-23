# Architecture

## Development

```text
Browser
  ├── http://localhost:8080 -> Web
  └── http://keycloak.localhost:8081 -> Keycloak

Web
  ├── PostgreSQL
  └── Stalwart (IMAP and SMTP, local delivery for example.test)
```

The development Compose profiles add Dovecot for IMAP compatibility checks
and Mailpit for inspecting sent messages. See the
[optional mail services](../README.md#optional-mail-services) for connection
settings.

## Production

```text
Internet
  -> Caddy
  -> Web

Web
  -> External OIDC Provider
  -> External on-premise mail server
  -> PostgreSQL
```
