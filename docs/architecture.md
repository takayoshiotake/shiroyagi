# Architecture

## Development

```text
Browser
  ├── http://localhost:8080 -> Web
  ├── http://localhost:8081 -> Keycloak
  └── http://localhost:8025 -> Mailpit UI

Web
  ├── PostgreSQL
  ├── Dovecot   (IMAPS)
  └── Mailpit   (SMTP)
```

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
